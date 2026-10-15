#!/usr/bin/env python3
"""驗收候選包及收據後，完成使用者授權的本機同版號替換。僅在 Docker 執行。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import tempfile


def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for b in iter(lambda:f.read(1<<20), b''): h.update(b)
    return h.hexdigest()


p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--stage', type=Path, required=True)
p.add_argument('--release', type=Path, required=True)
a = p.parse_args()
root = Path('/src')
assert a.stage.is_relative_to(root/'workplace') and a.release.is_relative_to(root/'dist-all')
candidate = a.stage/'candidate'
m = json.loads((candidate/'SHA256SUMS.json').read_text())
baseline = json.loads((a.stage/'previous-packages/SHA256SUMS.json').read_text())
assert json.loads((a.release/'SHA256SUMS.json').read_text()) == baseline
assert m['version'] == a.release.name and m['startup_repackage']['same_version_authorized']
checked = json.loads((a.stage/'package-verification.json').read_text())
assert checked['passed'] and len(checked['packages']) == len(m['packages']) == 9
assert all(x['passed'] for x in checked['packages'])

# Go 原始碼及內嵌資料仍與實際建置提交一致。macOS image 沒有 git，
# 其 Go build info 不含 VCS 欄，來源由此檢查及來源檔雜湊固定。
subprocess.run(['git', 'diff', '--exit-code', m['source_commit'], '--', 'cmd', 'internal', 'go.mod', 'go.sum'], cwd=root, check=True)
source_files = subprocess.check_output(['git','ls-files','cmd','internal','go.mod','go.sum'], cwd=root, text=True).splitlines()
build = {'source_commit': m['source_commit'], 'go_source_and_embeds_match_commit': True,
         'source_sha256': {name:sha(root/name) for name in source_files}, 'binaries': []}
for platform in ['linux-amd64','windows-amd64','darwin-amd64','darwin-arm64']:
    binary = a.stage/'bin'/('san1-'+platform+('.exe' if platform.startswith('windows') else ''))
    b = binary.read_bytes()
    assert m['version'].encode() in b
    info = subprocess.check_output(['go','version','-m',str(binary)], text=True)
    assert 'go1.26.7' in info and 'GOOS='+platform.split('-')[0] in info
    assert 'GOARCH='+platform.split('-')[1] in info
    if platform.startswith('linux'):
        assert b[:4] == b'\x7fELF' and struct.unpack_from('<H',b,18)[0] == 62
    elif platform.startswith('windows'):
        offset = struct.unpack_from('<I',b,0x3c)[0]
        assert b[offset:offset+4] == b'PE\0\0' and struct.unpack_from('<H',b,offset+4)[0] == 0x8664
    else:
        assert b[:4] == b'\xcf\xfa\xed\xfe' and struct.unpack_from('<I',b,4)[0] == (0x01000007 if platform.endswith('amd64') else 0x0100000c)
    build['binaries'].append({'platform':platform,'sha256':sha(binary),'build_info':info})

linux = json.loads((a.stage/'final-linux-gui/receipt.json').read_text())
windows = json.loads((a.stage/'final-windows-gui-r2/receipt.json').read_text())
appimage = json.loads((a.stage/'final-appimage/appimage-smoke.json').read_text())
for receipt in [linux, windows]:
    assert receipt['passed'] and len(receipt['checks']) == 19 and all(x['passed'] for x in receipt['checks'])
assert linux['binary_sha256'] == sha(a.stage/'bin/san1-linux-amd64')
assert windows['binary_sha256'] == sha(a.stage/'bin/san1-windows-amd64.exe')
assert appimage['passed'] and len(appimage['edition_smoke']) == 2
assert appimage['binary_sha256'] == linux['binary_sha256']
for row in appimage['edition_smoke']:
    path = a.stage/'final-appimage'/row['capture']
    assert sha(path) == row['capture_sha256']
    subprocess.run(['convert',str(path),'null:'],check=True,capture_output=True)
long_animation = json.loads((a.stage/'final-windows-gui-r2/hd-animation.json').read_text())
assert long_animation['passed']
for folder, receipt in [('final-linux-gui',linux), ('final-windows-gui-r2',windows), ('final-windows-gui-r2',long_animation)]:
    captures = {x['file']:x for x in receipt['captures']}
    assert len(captures) == len(receipt['captures'])
    for name, row in captures.items():
        path = a.stage/folder/name
        assert sha(path) == row['sha256']
        subprocess.run(['convert',str(path),'null:'], check=True, capture_output=True)
for row in m['packages']:
    path = candidate/row['file']
    target = a.release/row['file']
    assert path.stat().st_uid == target.stat().st_uid == os.getuid()
    assert path.stat().st_gid == target.stat().st_gid == os.getgid()
    assert path.stat().st_size == row['bytes'] and sha(path) == row['sha256']
image = candidate/m['appimage']['file']
assert sha(image) == appimage['image_sha256']

# 再由正式 AppImage 自行擷取，獨立核對全部檔案，不能只看檔案數。
with tempfile.TemporaryDirectory(prefix='san1-startup-final-audit-') as tmp:
    subprocess.run([str(image),'--appimage-extract'], cwd=tmp, check=True, stdout=subprocess.DEVNULL)
    appdir = Path(tmp)/'squashfs-root'
    actual = {str(p.relative_to(appdir)):sha(p) for p in appdir.rglob('*') if p.is_file()}
    expected = {str(p.relative_to(a.stage/'AppDir')):sha(p) for p in (a.stage/'AppDir').rglob('*') if p.is_file()}
    assert actual == expected and len(actual) == 795
    appimage['full_content_readback'] = {'files':len(actual), 'hd_files':len([p for p in actual if p.startswith('hd-assets/')]),
        'game_files':len([p for p in actual if p.startswith('game/')]), 'passed':True}
    assert appimage['full_content_readback']['hd_files'] == 667
    assert appimage['full_content_readback']['game_files'] == 66

smoke = a.release/'smoke/startup-20261015'
assert not smoke.exists() and smoke.parent.stat().st_uid == os.getuid()
smoke.mkdir()
for folder in ['final-linux-gui','final-windows-gui-r2','final-appimage']:
    shutil.copytree(a.stage/folder,smoke/folder)
for name in ['package-verification.json','regression.jsonl']:
    shutil.copy2(a.stage/name,smoke/name)
for name in ['verify-startup.py','repackage-startup.py','appimage-smoke.py','finalize-startup.py']:
    shutil.copy2(root/'tools'/name,smoke/name)
(smoke/'build-receipt.json').write_text(json.dumps(build,ensure_ascii=False,indent=2)+'\n')
(smoke/'appimage-full-readback.json').write_text(json.dumps(appimage,ensure_ascii=False,indent=2)+'\n')
for folder in ['linux-package','windows-package']:
    for file in (a.stage/folder).glob('*/saves/**/REMAKE.JSON'):
        dest=smoke/'saved-games'/folder/file.parent.name/file.name
        dest.parent.mkdir(parents=True,exist_ok=True)
        shutil.copy2(file,dest)

m['full_local_smoke'] = {'linux-amd64':'base_plus_normal_gui_zero_args_save_load_passed',
    'windows-amd64':'base_plus_normal_gui_zero_args_save_load_passed_wine_not_native',
    'darwin-amd64':'not_run_native','darwin-arm64':'not_run_native'}
m['native_smoke'] = dict(baseline['native_smoke'], **{'linux-amd64':'base_plus_normal_gui_passed',
    'windows-amd64':'not_run','darwin-amd64':'not_run','darwin-arm64':'not_run'})
m['appimage']['verification'] = 'base_plus_actual_AppRun_and_795_files_passed'
m['startup_repackage']['verification'] = {'receipt':'smoke/startup-20261015/final-audit.json',
    'linux_checks':19,'windows_wine_checks':19,'appimage_editions':2,'hd_load_animation':'passed'}
m['complete_delivery'] = dict(baseline['complete_delivery'])
m['complete_delivery'].update(source_commit=m['source_commit'],passed=True,
    platforms=[dict(next(r for r in m['packages'] if r['file']==old['file']),platform=old['platform'])
               for old in baseline['complete_delivery']['platforms']])
m['complete_delivery']['verification'] = dict(baseline['complete_delivery']['verification'],
    linux_appimage='base_plus_actual_AppRun_and_full_readback_passed', windows='base_plus_normal_gui_zero_args_save_load_passed_wine_not_native')
audit = {'version':m['version'],'passed':True,'source_commit':m['source_commit'],
    'packages':m['packages'],'linux_checks':19,'windows_wine_checks':19,
    'appimage_full_content_readback':appimage['full_content_readback'],
    'macos_native':'not_run','windows_native':'not_run','public_release_modified':False}
# 所有檢查完成才替換正式封包，最後替換清單。
for row in m['packages']: os.replace(candidate/row['file'],a.release/row['file'])
temporary = a.release/'SHA256SUMS.json.tmp'
temporary.write_text(json.dumps(m,ensure_ascii=False,indent=2)+'\n')
os.replace(temporary,a.release/'SHA256SUMS.json')
assert all(sha(a.release/r['file']) == r['sha256'] for r in m['packages'])
(smoke/'final-audit.json').write_text(json.dumps(audit,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'version':m['version'],'passed':True,'packages':len(m['packages']), 'receipt':str(smoke/'final-audit.json')},ensure_ascii=False))
