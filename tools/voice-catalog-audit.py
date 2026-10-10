#!/usr/bin/env python3
"""Derive bounded voice cues from independently verified base/plus text-token tables."""
import argparse
import hashlib
import json
from pathlib import Path

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--evidence', type=Path, required=True)
p.add_argument('--mother', type=Path, required=True)
p.add_argument('--out', type=Path, required=True)
a = p.parse_args()
base = json.loads((a.evidence / 'voice-callers-base.json').read_text())
plus = json.loads((a.evidence / 'voice-callers-plus.json').read_text())
bt = json.loads((a.evidence / 'phrase-table-draft.json').read_text())
pt = json.loads((a.evidence / 'phrase-table-plus.json').read_text())
for evidence in [base, plus]:
    assert evidence['probe'] and evidence['tool'] == 'IDA Pro 9.4'
    assert len(evidence['caller_contexts']) == 111
    assert all(x['decoded_to_call'] for x in evidence['caller_contexts'])
assert base['input_sha256'] == bt['input_sha256']
assert plus['input_sha256'] == pt['input']['runtime_sha256']
assert [(x['index'], x['text']) for x in bt['entries']] == [(x['index'], x['text']) for x in pt['entries']]
assert len(bt['entries']) == 150 and [x['index'] for x in bt['entries']] == list(range(350, 500))
normalize = lambda s: ''.join(s.split())
parts = [(x['index'], normalize(x['text'])) for x in bt['entries']
         if x['text'] and not x['text'].strip().isdigit()]


def parse(text, used=()):
    if not text:
        return [used]
    if len(used) == 3:
        return []
    candidates = parse(text[2:], used + (-1,)) if text.startswith('%s') else []
    for index, phrase in parts:
        if text.startswith(phrase):
            candidates += parse(text[len(phrase):], used + (index,))
    return candidates


mother = json.loads(a.mother.read_text())
cues = {}
for key, text in mother.items():
    # Only explicit dialogue keys. Generic formatting keys are not speech cues.
    if not key.startswith('bub.'):
        continue
    options = parse(normalize(text))
    assert options, ('no original token decomposition', key)
    shortest = min(map(len, options))
    choices = set(x for x in options if len(x) == shortest)
    assert len(choices) == 1, ('ambiguous token decomposition', key, choices)
    tokens = list(choices.pop()) + [499] * (3 - shortest)
    assert tokens.count(-1) == text.count('%s') and tokens.count(-1) <= 1
    cues[key] = tokens
assert len(cues) == 102
assert cues['bub.warDeclare'] == [-1, 456, 499]
assert cues['bub.warReply'] == [-1, 457, 499]
assert cues['bub.recruitAsk'] == [390, -1, 391]
assert cues['bub.recruitYes'] == [393, -1, 394]
result = {'schema': 'san1-voice-catalog-v1', 'level': 'L0', 'editions': ['base', 'plus'],
    'scope': '102 normal-game dialogue templates; copy-protection excluded by spec/004 and spec/005',
    'name_token': -1, 'empty_token': 499, 'cues': cues,
    'inputs': {'base_runtime_sha256': base['input_sha256'], 'plus_runtime_sha256': plus['input_sha256'],
               'mother_sha256': hashlib.sha256(a.mother.read_bytes()).hexdigest()},
    'base': {'message_linear': 0x3273e, 'message_end_exclusive': 0x32df9,
             'code_segment': 0x3273, 'ds_base': 0x427e0, 'table_displacement': 0x9382,
             'caller_count': 111},
    'plus': {'message_linear': 0x2f366, 'message_end_exclusive': 0x2f9ab,
             'code_segment': 0x2f36, 'ds_base': pt['input']['ds_base'], 'table_displacement': 0x952e,
             'caller_count': 111},
    'evidence': ['docs/re/12-message-box.md', 'docs/re/09-speech.md#10-完整對白語音目錄研究'],
    'notes': 'Derived from the shared text/audio indices; no translated text dispatch or human naming of R499.'}
assert not a.out.exists()
a.out.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'templates': len(cues), 'base_and_plus_phrase_entries_equal': 150, 'passed': True}))
