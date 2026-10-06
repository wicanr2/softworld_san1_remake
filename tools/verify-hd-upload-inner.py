#!/usr/bin/env python3
"""以同一正常玩家路徑比較完整／局部紋理上傳。所有證據只留本機。"""
import argparse
import ast
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import time

ROOT = Path('/src')
PACK = ROOT / 'workplace/hd-assets-mapcursor-v50-r1'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def prepare(out):
    if out.exists():
        raise RuntimeError('輸出已存在')
    if out.parent.stat().st_uid != os.getuid():
        raise RuntimeError('輸出目錄擁有權不符')
    out.mkdir()
    (out/'trace.go').write_text((ROOT/'tools/hd-upload-trace.go').read_text().replace('//go:build ignore\n','',1))
    main = (ROOT / 'cmd/san1/main.go').read_text()
    main = main.replace('type app struct {', 'type app struct {\n\thdUploadTrace *hdUploadTrace', 1)
    for name in ['Update()', 'Draw(dst *ebiten.Image)', 'paint()']:
        needle = 'func (a *app) ' + name
        i = main.index('{', main.index(needle))
        stage = 'update' if name == 'Update()' else 'draw' if name.startswith('Draw') else 'paint'
        main = main[:i+1] + '\n\tdefer a.hdUploadMeasure("' + stage + '")()' + main[i+1:]
    bar = (ROOT / 'cmd/san1/windowbar.go').read_text()
    needle = '\tim := a.canvas.Output(a.hdTheme)'
    assert bar.count(needle) == 1
    bar = bar.replace(needle, '\tmeasure := a.hdUploadMeasure("output")\n' + needle + '\n\tmeasure()')
    partial = '\tr, pixels := a.upload.Changed(im)\n\tif !r.Empty() {\n\t\ta.screen.SubImage(r).(*ebiten.Image).WritePixels(pixels)\n\t}'
    assert bar.count(partial) == 1
    variants = {
        'full': bar.replace(partial, '\ta.screen.WritePixels(im.Pix)\n\ta.hdUploadPacket(len(im.Pix))'),
        'partial': bar.replace(partial, partial + '\n\ta.hdUploadPacket(len(pixels))'),
    }
    for strategy, text in variants.items():
        selected_main = main
        if strategy == 'full':
            # 對照恢復原有全幀重畫，其他更新與正式程式相同。
            selected_main = selected_main.replace(' && !(a.lure.of != nil && a.lure.step >= 0)', '')
            needle = 'func (a *app) updateBattle() error {'
            assert selected_main.count(needle) == 1
            selected_main = selected_main.replace(needle, needle+'\n\tdefer func() { a.dirty = true }()', 1)
        main_path = out/(strategy+'-main.go')
        main_path.write_text(selected_main)
        path = out / (strategy + '-windowbar.go')
        path.write_text(text)
        overlay = {'Replace': {
            str(ROOT/'cmd/san1/main.go'): str(main_path),
            str(ROOT/'cmd/san1/windowbar.go'): str(path),
            str(ROOT/'cmd/san1/zz_hd_upload_trace.go'): str(out/'trace.go'),
        }}
        overlay_path = out/(strategy+'-overlay.json')
        overlay_path.write_text(json.dumps(overlay, indent=2)+'\n')
        subprocess.run(['go', 'build', '-trimpath', '-overlay', str(overlay_path), '-o', str(out/strategy), './cmd/san1'], cwd=ROOT, check=True)
    subprocess.run(['go','run','./cmd/san1dump','-root','/orig/三國演義','-screen','title','-png',str(out/'title-reference.png')], cwd=ROOT, check=True, stdout=subprocess.DEVNULL)
    subprocess.run(['go','run','./tools/hd-battle-branches-reference.go','-out',str(out)], cwd=ROOT, check=True)
    inputs = {str(p.relative_to(ROOT)):sha(p) for p in [ROOT/'cmd/san1/main.go',ROOT/'cmd/san1/windowbar.go',ROOT/'internal/ui/pixelupload.go',ROOT/'tools/hd-upload-trace.go',ROOT/'tools/verify-hd-upload-inner.py',PACK/'manifest.json']}
    (out/'build.json').write_text(json.dumps({'inputs':inputs,'binaries':{s:sha(out/s) for s in variants},'method':'同一正式程式，對照恢復全幀重畫及整張上傳；候選按相位重畫及局部上傳'},ensure_ascii=False,indent=2)+'\n')


def play(out):
    spec = importlib.util.spec_from_file_location('window', ROOT/'tools/verify-window-inner.py')
    gui = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(gui)
    gui.OUT = out
    original_shot = gui.shot
    shot_counts = {}

    def unique_shot(wid, name):
        count = shot_counts.get(name, 0) + 1
        shot_counts[name] = count
        chosen = name if count == 1 else f'{name}-{count:04d}'
        if (out/(chosen+'.png')).exists():
            raise RuntimeError('截圖名稱已存在：'+chosen)
        return original_shot(wid, chosen)

    gui.shot = unique_shot
    namespace = {'gui':gui,'pack':PACK,'time':time}
    tree = ast.parse((ROOT/'tools/verify-hd-lure-inner.py').read_text())
    selected = [n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name in ['theme','await_face','command_ready']]
    exec(compile(ast.Module(body=selected,type_ignores=[]),'normal-lure-input','exec'),namespace)
    results = []
    gui.receipt.update(method='正常片頭、新局、合法出兵／紮寨及誘敵，無錄影',scope='兩版原貌／HD 與完整／局部上傳；不注入狀態、seed 或動畫相位')
    try:
        os.environ.update(DISPLAY=':99',LIBGL_ALWAYS_SOFTWARE='1',XDG_RUNTIME_DIR='/tmp')
        sock = Path('/tmp/.X11-unix')
        sock.mkdir(exist_ok=True)
        sock.chmod(0o1777)
        display = gui.start(['Xvfb',':99','-screen','0','2800x1900x24','-nolisten','tcp','-noreset','-ac'],'xvfb')
        gui.check('display-ready',gui.wait(['xdotool','getdisplaygeometry'])=='2800 1900' and display.poll() is None)
        build = json.loads((out/'build.json').read_text())
        gui.receipt['build'] = build
        for edition in ['base','plus']:
            for high in [False,True]:
                for strategy in ['full','partial']:
                    tag = edition+('-hd-' if high else '-original-')+strategy
                    link = out/'san1-window-check'
                    if link.exists() or link.is_symlink(): link.unlink()
                    link.symlink_to(strategy)
                    trace = out/(tag+'-trace.json')
                    os.environ['SAN1_HD_UPLOAD_TRACE'] = str(trace)
                    proc,wid = gui.launch(edition,hd_assets=PACK,tag=tag)
                    gui.key(wid,'1','1','1','2','5')
                    gui.key(wid,'2','Return','2','1','1','Return','4','Return','1','Return','1','Return','Return','y','4','0','0','Return','1','0','0','0','Return')
                    namespace['await_face'](wid,tag)
                    gui.key(wid,*(['space']*9))
                    gui.key(wid,'5','5','5','5','0')
                    namespace['command_ready'](wid,tag,False)
                    if high:
                        namespace['theme'](wid,True)
                        gui.dimensions(wid,tag+'-hd',2560,1632)
                    gui.key(wid,'6','1','4')
                    gui.shot(wid,tag+'-speech')
                    gui.key(wid,'space')
                    deadline = time.monotonic()+120
                    while not trace.exists() and time.monotonic()<deadline:
                        if proc.poll() is not None: raise RuntimeError(tag+' 程序停止')
                        time.sleep(.2)
                    data = json.loads(trace.read_text())
                    gui.check(tag+'-22-drawn-phases',[p['Step'] for p in data['phases']]==list(range(22)))
                    gui.check(tag+'-native-mode',data['high']==high)
                    after = gui.shot(wid,tag+'-after')
                    results.append({'edition':edition,'high':high,'strategy':strategy,'trace':trace.name,'sha256':sha(trace),'after':after.name,**data})
                    print(tag,round(data['duration_seconds'],3),data['upload_bytes'],flush=True)
                    gui.stop(proc)
        gui.receipt.update(passed=True,results=results)
    finally:
        gui.receipt.setdefault('passed',False)
        gui.receipt['results'] = results
        for proc in reversed(gui.processes): gui.stop(proc)
        for log in gui.logs: log.close()
        (out/'receipt.json').write_text(json.dumps(gui.receipt,ensure_ascii=False,indent=2)+'\n')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('operation',choices=['prepare','play'])
    parser.add_argument('--out',required=True)
    args = parser.parse_args()
    path = Path(args.out)
    if path.is_absolute() or not args.out.startswith('workplace/hd-upload-') or '..' in path.parts:
        raise ValueError('輸出須為 workplace/hd-upload-* 相對路徑')
    {'prepare':prepare,'play':play}[args.operation](ROOT/path)
