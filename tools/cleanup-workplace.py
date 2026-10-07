#!/usr/bin/env python3
"""只清理有完整保留副本的工作目錄；由 cleanup-workplace.sh 在 Docker 執行。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import time


def confined(root, name):
    relative = Path(name)
    if relative.is_absolute() or '..' in relative.parts or relative.parts[:1] != ('workplace',):
        raise ValueError(f'路徑須位於 workplace/：{name}')
    if len(relative.parts) < 2:
        raise ValueError('不能清理 workplace 根目錄')
    path = root
    for part in relative.parts:
        path /= part
        if path.is_symlink():
            raise ValueError(f'拒絕符號連結：{path}')
    return path


def inventory(path):
    if not path.is_dir():
        raise ValueError(f'目錄不存在：{path}')
    files = {}
    allocated = 0
    for p in [path, *sorted(path.rglob('*'))]:
        info = p.lstat()
        if (info.st_uid, info.st_gid) != (os.getuid(), os.getgid()):
            raise ValueError(f'擁有權不符：{p}')
        if stat.S_ISDIR(info.st_mode):
            allocated += info.st_blocks*512
            continue
        if not stat.S_ISREG(info.st_mode):
            raise ValueError(f'拒絕連結或特殊檔：{p}')
        with p.open('rb') as source:
            digest = hashlib.file_digest(source, 'sha256').hexdigest()
        files[str(p.relative_to(path))] = {'size': info.st_size, 'sha256': digest}
        allocated += info.st_blocks*512
    if not files:
        raise ValueError(f'沒有可比對的檔案：{path}')
    return files, allocated


def prepare(root, pairs):
    items = []
    for pair in pairs:
        temporary, sep, retained = pair.partition('=')
        if not sep:
            raise ValueError('副本須寫成 暫存目錄=保留目錄')
        target, keep = confined(root, temporary), confined(root, retained)
        if target == keep or target in keep.parents or keep in target.parents:
            raise ValueError('暫存目錄與保留目錄不能重疊')
        items.append((temporary, retained, target, keep))
    if len({x[0] for x in items}) != len(items):
        raise ValueError('暫存目錄重複')
    for _, _, target, keep in items:
        for _, _, other, _ in items:
            if keep == other or other in keep.parents or keep in other.parents:
                raise ValueError('保留目錄不能同時列為清理目錄')
            if target != other and (target in other.parents or other in target.parents):
                raise ValueError('清理目錄不能互相包含')
    results = []
    for temporary, retained, target, keep in items:
        original, allocated = inventory(target)
        preserved, _ = inventory(keep)
        if original != preserved:
            raise ValueError(f'副本不完全相同，整批不清理：{temporary}')
        results.append({'temporary': temporary, 'retained': retained,
                        'allocated_bytes': allocated, 'files': original})
    return results


def run(root, pairs, report, apply=False):
    root = root.resolve(strict=True)
    destination = confined(root, report)
    if destination.exists() or not destination.parent.is_dir():
        raise ValueError('收據須為既有工作目錄中的新檔案')
    if destination.parent.stat().st_uid != os.getuid():
        raise ValueError('收據目錄擁有權不符')
    started = time.time_ns()
    document = {'passed': False, 'apply': apply, 'started_ns': started,
                'method': '全批次路徑、擁有權、檔案清單、長度及 SHA-256 檢查',
                'deleted': []}
    # 先保留唯一收據。即使預檢拒收，也留下原因，不覆寫其他批次。
    with destination.open('x') as f:
        json.dump(document, f, ensure_ascii=False, indent=2)

    def journal():
        temporary = destination.with_suffix(destination.suffix+'.tmp')
        with temporary.open('x') as f:
            json.dump(document, f, ensure_ascii=False, indent=2)
            f.write('\n')
            f.flush()
            os.fsync(f.fileno())
        temporary.replace(destination)

    try:
        document['copies'] = prepare(root, pairs)
        if any(confined(root, c['temporary']) in destination.parents for c in document['copies']):
            raise ValueError('收據不能放進待清理目錄')
        journal()
        if apply:
            # 再核對整批，以免預檢後來源已經改變。
            if prepare(root, pairs) != document['copies']:
                raise ValueError('預檢後檔案已改變')
            for copy in document['copies']:
                shutil.rmtree(confined(root, copy['temporary']))
                document['deleted'].append(copy['temporary'])
                journal()
            for copy in document['copies']:
                if inventory(confined(root, copy['retained']))[0] != copy['files']:
                    raise ValueError('清理後保留副本不符')
        document['passed'] = True
        document['finished_ns'] = time.time_ns()
        journal()
        return document
    except Exception as error:
        document['error'] = str(error)
        journal()
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path('/src'))
    parser.add_argument('--duplicate', action='append', required=True,
                        help='workplace/暫存=workplace/保留，每次一對')
    parser.add_argument('--report', required=True, help='workplace/ 內的新 JSON 收據')
    parser.add_argument('--apply', action='store_true', help='預設只驗證；加此參數才移除副本')
    args = parser.parse_args()
    result = run(args.root, args.duplicate, args.report, args.apply)
    print(json.dumps({'passed': result['passed'], 'apply': result['apply'],
                      'copies': len(result['copies']), 'deleted': result['deleted'],
                      'allocated_bytes': sum(c['allocated_bytes'] for c in result['copies']),
                      'report': args.report}, ensure_ascii=False))


if __name__ == '__main__':
    main()
