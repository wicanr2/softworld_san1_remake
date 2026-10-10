#!/usr/bin/env python3
"""Check complete generated voice waveforms in the local audition movie."""
import argparse
import array
import ast
import cmath
import hashlib
import json
import math
from pathlib import Path
import subprocess

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--samples', type=Path, required=True)
p.add_argument('--video', type=Path, required=True)
p.add_argument('--out', type=Path, required=True)
p.add_argument('--qa', type=Path, help='Full-trailer QA containing redubbing insertion positions')
a = p.parse_args()
sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
tool = Path(__file__).with_name('promo-voice-check.py')
functions = [n for n in ast.parse(tool.read_text()).body
             if isinstance(n, ast.FunctionDef) and n.name in {'decode', 'locate'}]
assert len(functions) == 2
exec(compile(ast.Module(body=functions, type_ignores=[]), str(tool), 'exec'), globals())
results = []
plan = [(row['file'], row['begin']) for row in json.loads(a.qa.read_text())['redubbing']['samples']] if a.qa else [
    ('poem-narration.wav', 5.3), ('adviser-a.wav', 12.8)]
for filename, begin in plan:
    reference = a.samples / filename
    ref = decode(reference)
    mean = sum(ref) / len(ref)
    r = [x - mean for x in ref]
    energy = sum(x * x for x in r)
    match = locate(a.video, begin - .15, begin + .15)
    assert match['complete_reference_correlation'] > .90, match
    assert abs(match['start_seconds'] - begin) < .05, match
    match.update(reference_file=filename, reference_sha256=sha(reference), expected_begin=begin)
    results.append(match)
assert not a.out.exists()
result = {'passed': True, 'scope': 'complete AI generated source waveforms and movie positions',
          'human_listening': 'pending user audition', 'production_approved': False,
          'movie_use_authorized': bool(a.qa), 'samples': results}
a.out.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(result, ensure_ascii=False))
