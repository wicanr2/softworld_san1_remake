#!/usr/bin/env python3
"""Normal new-game and battle selection phases, in both editions and themes."""
import ast
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import tempfile
import time
from PIL import Image, ImageChops

ROOT=Path('/src')
OUT=ROOT/os.environ.get('SAN1_SELECTION_BLINK_OUT','workplace/selection-blink-gui-20261010-r3')
assert OUT.resolve().is_relative_to((ROOT/'workplace').resolve())
assert not OUT.exists()
OUT.mkdir()
spec=importlib.util.spec_from_file_location('gui',ROOT/'tools/verify-window-inner.py')
gui=importlib.util.module_from_spec(spec);spec.loader.exec_module(gui);gui.OUT=OUT
shutil.copy2(ROOT/'workplace/san1-selection-blink-dev',OUT/'san1-window-check')
for p in (ROOT/'workplace/promo-source/v.1.1.1-20261010').glob('*-reference.png'):
    shutil.copyfile(p,OUT/p.name)
nodes=[n for n in ast.parse((ROOT/'tools/verify-hd-battle-branches-inner.py').read_text()).body
       if isinstance(n,ast.FunctionDef) and n.name in {'flip','source_face','await_prompt','reach_attack_turn'}]
exec(compile(ast.Module(body=nodes,type_ignores=[]),'<existing normal battle route>','exec'),globals())
PACK=ROOT/'workplace/hd-assets-mapcursor-v50-r1'
def theme(wid,high):
    gui.key(wid,'Escape');gui.choose_ready(wid,1,int(high));gui.key(wid,'Escape')
    gui.run(['xdotool','windowsize',wid,'2560' if high else '640','1632' if high else '408'])
    gui.run(['xdotool','windowmove',wid,'0','0']);gui.move(wid,320,200);time.sleep(.5)

def phases(wid,tag,region,high):
    scale=4 if high else 1
    with tempfile.TemporaryDirectory(prefix='san1-selection-frames-') as tmp:
        g=gui.geo(wid)
        gui.run(['ffmpeg','-nostdin','-v','error','-f','x11grab','-framerate','3',
                 '-video_size',f"{g['WIDTH']}x{g['HEIGHT']}",'-draw_mouse','0',
                 '-i',f":99+{g['X']},{g['Y']}",'-t','4','-threads','2',str(Path(tmp)/'%03d.png')])
        files=sorted(Path(tmp).glob('*.png'));assert len(files)>=6
        box=tuple(z*scale for z in region)
        first=Image.open(files[0]).convert('RGB').crop(box)
        other=None
        for p in files[1:]:
            im=Image.open(p).convert('RGB').crop(box)
            if im.tobytes()!=first.tobytes():other=p;break
        gui.check(tag+'-two-normal-phases',other is not None)
        diff=ImageChops.difference(first,Image.open(other).convert('RGB').crop(box)).getbbox()
        for name,p in [('first',files[0]),('second',other)]:shutil.copyfile(p,OUT/(tag+'-'+name+'.png'))
        gui.receipt.setdefault('selection_samples',[]).append({'tag':tag,'region':region,'scale':scale,
            'changed_bounds_within_region':diff,'frames_observed':len(files),
            'first_sha256':hashlib.sha256(files[0].read_bytes()).hexdigest(),
            'second_sha256':hashlib.sha256(other.read_bytes()).hexdigest()})

try:
    os.environ.update(DISPLAY=':99',LIBGL_ALWAYS_SOFTWARE='1',XDG_RUNTIME_DIR='/tmp')
    gui.start(['Xvfb',':99','-screen','0','2800x1900x24','-nolisten','tcp','-noreset','-ac'],'xvfb')
    gui.wait(['xdotool','getdisplaygeometry'])
    for edition in ['base','plus']:
        proc,wid=gui.launch(edition,hd_assets=PACK,tag=edition+'-main')
        gui.key(wid,'1','1','1','2','5')
        phases(wid,edition+'-main-original',(280,158,296,167),False)
        theme(wid,True);phases(wid,edition+'-main-hd',(280,158,296,167),True)
        gui.stop(proc)
        proc,wid=gui.launch(edition,hd_assets=PACK,tag=edition+'-battle')
        gui.key(wid,'1','1','1','6','5')
        waiting,rested=reach_attack_turn(wid,edition,edition)
        gui.key(wid,'2','Return','2')
        if waiting==14:gui.key(wid,'1','5','Return')
        gui.key(wid,'1','1','Return','2','Return','1','Return','Return','y',
                '0','Return','1','0','0','0','Return')
        await_prompt(wid,edition+'-camp','camp',2)
        gui.key(wid,'3','3','6','3','0');await_prompt(wid,edition+'-command','command',3)
        phases(wid,edition+'-battle-original',(56,36,432,364),False)
        theme(wid,True);phases(wid,edition+'-battle-hd',(56,36,432,364),True)
        gui.stop(proc)
    gui.receipt.update(passed=True,scope='normal current prefecture and main battle unit; both editions and themes',
        binary_sha256=hashlib.sha256((OUT/'san1-window-check').read_bytes()).hexdigest())
finally:
    for proc in reversed(gui.processes):gui.stop(proc)
    for log in gui.logs:log.close()
    (OUT/'receipt.json').write_text(json.dumps(gui.receipt,ensure_ascii=False,indent=2)+'\n')
