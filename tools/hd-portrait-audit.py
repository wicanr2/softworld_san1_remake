#!/usr/bin/env python3
"""稽核私人高清包的全部 256 肖像槽；須在 Docker 內執行。"""
import argparse
import collections
import hashlib
import json
import os
from pathlib import Path
import re
import struct

KEY = re.compile(r'^DATA3/F(?:[01][0-9]{2}|2[0-4][0-9]|25[0-5])\.FAC$')
ACCEPTED = {'accepted-for-local-pack', 'accepted-local'}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def local(root, name):
    path = (root / name).resolve()
    path.relative_to(root)
    return path


def png_size(path):
    with path.open('rb') as f:
        b = f.read(24)
    if len(b) != 24 or b[:8] != b'\x89PNG\r\n\x1a\n' or b[12:16] != b'IHDR':
        raise ValueError('PNG 表頭不符')
    return list(struct.unpack('>II', b[16:24]))


def canonical(key):
    if key.lower().startswith('f') and '/' not in key:
        key = 'DATA3/' + key.upper() + '.FAC'
    return key


def generations(root, names):
    by_file = collections.defaultdict(list)
    receipts = []
    for name in names:
        path = local(root, name)
        data = json.loads(path.read_text())
        receipts.append({'file': name, 'sha256': sha(path)})
        for entry in data.get('entries', data.get('records', [])):
            key = canonical(entry['key'])
            if not KEY.fullmatch(key):
                continue
            for candidate in entry.get('candidates', [entry]):
                value = {**data, **entry, **candidate}
                master = value.get('master_file', value.get('master_path', value.get('path')))
                if not master:
                    raise ValueError(f'{name}: {key} 缺生成原圖路徑')
                by_file[str(local(root, master))].append({
                    'key': key, 'receipt': name, 'value': value})
    return by_file, receipts


def audit(args):
    root = Path(args.root).resolve()
    inventory_path = local(root, args.inventory)
    inventory = json.loads(inventory_path.read_text())
    expected = {f'DATA3/F{n:03d}.FAC' for n in range(256)}
    sources, references = {}, {}
    require(len(inventory['editions']) == 2, '來源盤點須恰有兩版')
    for edition in inventory['editions']:
        name = edition['edition']
        entries = [x for x in edition['assets'] if KEY.fullmatch(x['key'])]
        if len(entries) != 256 or {x['key'] for x in entries} != expected:
            raise ValueError(f'{name}: 來源盤點不是完整 256 個獨立槽')
        sources[name] = {x['key']: x for x in entries}
        references[name] = edition['portrait_references']
    if set(sources) != {'base', 'plus'}:
        raise ValueError('來源盤點缺少兩版')
    pack = local(root, args.pack)
    manifest_path, preparation_path = pack / 'manifest.json', pack / 'preparation.json'
    manifest = json.loads(manifest_path.read_text())
    if (manifest['schema'], manifest['style'], manifest['scale']) != (1, 'b', 4):
        raise ValueError('素材包契約不符')
    problems = []
    entries, prepared, invalid = {}, {}, set()
    for field, target, values in [
            ('manifest', entries, manifest['entries']),
            ('preparation', prepared, json.loads(preparation_path.read_text()))]:
        for x in values:
            key = x.get('key', x.get('container', '') + '/' + x.get('name', ''))
            if not key.startswith('DATA3/F'):
                if re.match(r'[^/]+/F', key):
                    problems.append(f'{field}: 未知肖像鍵 {key}')
                continue
            if not KEY.fullmatch(key) or x['edition'] not in sources:
                problems.append(f'{field}: 未知肖像鍵 {x["edition"]}/{key}')
                continue
            identity = (x['edition'], key)
            if identity in target:
                problems.append(f'{field}: 重複 {x["edition"]}/{key}')
                invalid.add(identity)
            target[identity] = x
    metadata, receipts = generations(root, args.records)
    reviews = {}
    for name in args.reviews:
        path = local(root, name)
        receipts.append({'file': name, 'sha256': sha(path)})
        for x in json.loads(path.read_text())['entries']:
            key = canonical(x['key'])
            identity = (key, x['master_sha256'])
            if not KEY.fullmatch(key) or identity in reviews:
                raise ValueError(f'{name}: 未知或重複審查鍵 {key}')
            reviews[identity] = x
    result = []
    for key in sorted(expected):
        item = {'key': key, 'sources': {}, 'prepared': [], 'generation': [],
                'codex_reviewed': False, 'human_signoff': False, 'problems': []}
        for edition in ['base', 'plus']:
            source = sources[edition][key]
            item['sources'][edition] = {'sha256': source['source_sha256'],
                'width': source['width'], 'height': source['height'],
                'references': [x for x in references[edition] if x['portrait_key'] == key]}
            e, p = entries.get((edition, key)), prepared.get((edition, key))
            if e is None:
                if p is not None:
                    item['problems'].append(edition + ' 準備紀錄存在但包內缺圖')
                continue
            try:
                require((edition, key) not in invalid, '重複登錄')
                require(p is not None, '缺準備紀錄')
                require(e['source_sha256'] == source['source_sha256'], '來源雜湊不符')
                require([e['width'], e['height']] == [source['width']*4, source['height']*4], '登錄尺寸不符')
                require(not Path(e['file']).is_absolute(), '圖檔須用包內相對路徑')
                file = local(pack, e['file'])
                require(file.is_file() and file.stat().st_size <= 16 << 20, '檔案形態或大小不符')
                require(png_size(file) == [e['width'], e['height']], 'PNG 尺寸不符')
                require(sha(file) == e['sha256'], 'PNG 雜湊不符')
                require(all(e[k] == p[k] for k in ['source_sha256', 'file', 'sha256', 'width', 'height']), '準備紀錄不符')
                master = local(root, p['input'])
                require(sha(master) == p['input_sha256'], '生成原圖雜湊不符')
                mw, mh = png_size(master)
                require(abs(mw*source['height'] - mh*source['width']) <= max(source['width'], source['height']), '生成原圖比例不符')
                matches = metadata[str(master)]
                require(len(matches) == 1, '缺生成設定或重複原圖紀錄')
                m = matches[0]
                v = m['value']
                require(m['key'] == key, '生成設定的資源鍵不符')
                require(v.get('master_sha256', v.get('sha256')) == p['input_sha256'], '生成設定的原圖雜湊不符')
                require(v.get('prompt', '').strip(), '缺提示詞')
                require(v.get('tool', v.get('method', '')).strip(), '缺工具紀錄')
                require('seed' in v and ('model' in v or 'model_version' in v), '缺模型／seed 限制紀錄')
                if v.get('source_sha256'):
                    require(v['source_sha256'] == source['source_sha256'], '生成設定的來源雜湊不符')
                review = v.get('review', v.get('decision', ''))
                if isinstance(review, dict):
                    status = review.get('status', '')
                    by = review.get('by', '')
                    human = review.get('human_signoff', False)
                else:
                    status, by, human = review, v.get('reviewer', ''), v.get('human_signoff', False)
                override = reviews.get((key, p['input_sha256']))
                if override:
                    status, by, human = override['status'], override['by'], override['human_signoff']
                accepted = status in ACCEPTED and bool(by)
                item['prepared'].append({'edition': edition, 'file': e['file'], 'sha256': e['sha256']})
                item['generation'].append({'edition': edition, 'receipt': m['receipt'],
                    'master': p['input'], 'sha256': p['input_sha256'], 'dimensions': png_size(master),
                    'tool': v.get('tool', v.get('method')), 'model': v.get('model', v.get('model_version')),
                    'seed': v['seed'], 'reviewer': by, 'status': status,
                    'codex_reviewed': accepted, 'human_signoff': accepted and human is True})
            except (AssertionError, OSError, ValueError, KeyError) as err:
                item['problems'].append(edition + ': ' + str(err))
        item['prepared_both'] = len(item['prepared']) == 2 and not item['problems']
        item['codex_reviewed'] = item['prepared_both'] and all(x['codex_reviewed'] for x in item['generation'])
        item['human_signoff'] = item['prepared_both'] and all(x['human_signoff'] for x in item['generation'])
        result.append(item)
    summary = {'total': 256,
        'prepared_both': sum(x['prepared_both'] for x in result),
        'codex_reviewed': sum(x['codex_reviewed'] for x in result),
        'human_signoff': sum(x['human_signoff'] for x in result),
        'missing_keys': [x['key'] for x in result if not x['prepared_both']],
        'unreviewed_keys': [x['key'] for x in result if x['prepared_both'] and not x['codex_reviewed']],
        'technical_problems': problems + [x['key'] + ': ' + p for x in result for p in x['problems']]}
    output = local(root, args.out)
    require(output.parent.is_dir(), '輸出目錄須先存在')
    require((output.parent.stat().st_uid, output.parent.stat().st_gid) == (os.getuid(), os.getgid()), '輸出目錄擁有權不符')
    if output.exists():
        require((output.stat().st_uid, output.stat().st_gid) == (os.getuid(), os.getgid()), '輸出檔擁有權不符')
    report = {'schema': 1, 'rights': '原版衍生美術；公開再散布權未知，僅供本機私人驗收。',
        'scope': '兩版全部 256 DATA3 肖像槽；未宣稱原版 parity 或使用者逐張簽核。',
        'tool_sha256': sha(Path(__file__)), 'inventory_sha256': sha(inventory_path),
        'pack_sha256': sha(manifest_path), 'preparation_sha256': sha(preparation_path),
        'generation_receipts': receipts, 'summary': summary, 'entries': result}
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(f"肖像 {summary['prepared_both']}/256；Codex 已審查 {summary['codex_reviewed']}；使用者簽核 {summary['human_signoff']}")
    print(f"缺項 {len(summary['missing_keys'])}；未審查 {len(summary['unreviewed_keys'])}；技術問題 {len(summary['technical_problems'])}")
    print('收據 SHA-256', sha(output))
    if summary['technical_problems']:
        for problem in summary['technical_problems']:
            print(problem)
        return 1
    if args.require_complete and (summary['missing_keys'] or summary['unreviewed_keys']):
        return 3
    return 0


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--root', default='.')
    p.add_argument('--inventory', default='workplace/hd-inventory/inventory.json')
    p.add_argument('--pack', required=True)
    p.add_argument('--records', action='append', required=True)
    p.add_argument('--reviews', action='append', default=[])
    p.add_argument('--out', required=True)
    p.add_argument('--require-complete', action='store_true')
    args = p.parse_args()
    try:
        return audit(args)
    except (AssertionError, KeyError, OSError, ValueError) as err:
        p.exit(2, str(err) + '\n')


if __name__ == '__main__':
    raise SystemExit(main())
