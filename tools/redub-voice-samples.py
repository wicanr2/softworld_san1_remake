#!/usr/bin/env python3
"""Create local AI redubbing auditions; these are not production game assets."""
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import time
import numpy as np
import soundfile as sf
import torch
from qwen_tts import Qwen3TTSModel

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--model', type=Path, required=True)
p.add_argument('--out', type=Path, required=True)
p.add_argument('--threads', type=int, default=8)
a = p.parse_args()
assert not a.out.exists() and a.out.parent.stat().st_uid == os.getuid()
source = json.loads((a.model / 'SOURCES.json').read_text())
assert source['passed'] and source['revision'] == '0c0e3051f131929182e2c023b9537f8b1c68adfe'
a.out.mkdir()
torch.set_num_threads(a.threads)
torch.set_num_interop_threads(2)
torch.manual_seed(20261010)
receipt = {'scope': 'disposable redubbing auditions; synthetic AI voices',
           'original_actor_reference': False, 'production_approved': False,
           'model': source['model'], 'model_revision': source['revision'],
           'packages': {n: importlib.metadata.version(n) for n in ['qwen-tts', 'torch', 'transformers', 'soundfile']},
           'seed': 20261010, 'samples': [], 'passed': False}
try:
    begin = time.monotonic()
    model = Qwen3TTSModel.from_pretrained(str(a.model), device_map='cpu',
                                        dtype=torch.float32, attn_implementation='sdpa',
                                        local_files_only=True)
    receipt['load_seconds'] = round(time.monotonic() - begin, 3)
    print(json.dumps({'model_loaded': True, 'seconds': receipt['load_seconds']}), flush=True)
    for tag, label, speaker, text, instruct in [
        ('adviser-a', '軍師聲線 A', 'Uncle_Fu', '兵者貴神速。',
         '沉穩、低沉的成熟男聲。像軍師向主公獻策，語速適中，字句清楚，語氣果斷。'),
        ('commander-b', '武將聲線 B', 'Dylan', '兵者貴神速。',
         '成年男性武將，聲音有力量，果斷而清晰，像在軍前下令，避免誇張吼叫。'),
        ('poem-narration', '卷首詞朗誦樣音', 'Uncle_Fu', '滾滾長江東逝水，浪花淘盡英雄。',
         '成熟男聲，開闊、沉著，朗誦古典詩詞，停頓自然，語調有歷史滄桑感。')]:
        begin = time.monotonic()
        waves, rate = model.generate_custom_voice(text=text, language='Chinese', speaker=speaker,
            instruct=instruct, max_new_tokens=256, do_sample=True, temperature=.8,
            top_p=.95, repetition_penalty=1.05)
        wave = np.asarray(waves[0], dtype=np.float32)
        duration, peak = len(wave) / rate, float(np.max(np.abs(wave)))
        rms = float(np.sqrt(np.mean(wave ** 2)))
        assert .5 < duration < 20 and .005 < peak < 1 and rms > .002
        target = a.out / (tag + '.wav')
        sf.write(target, wave, rate, subtype='PCM_16')
        entry = {'file': target.name, 'label': label, 'speaker_preset': speaker, 'text': text,
                 'instruction': instruct, 'sample_rate': rate, 'channels': 1,
                 'duration_seconds': duration, 'peak': peak, 'rms_db': 20 * np.log10(rms),
                 'generation_seconds': round(time.monotonic() - begin, 3),
                 'sha256': hashlib.sha256(target.read_bytes()).hexdigest(), 'human_review': 'pending'}
        receipt['samples'].append(entry)
        print(json.dumps(entry, ensure_ascii=False), flush=True)
        (a.out / 'receipt.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
    receipt['passed'] = True
finally:
    (a.out / 'receipt.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
