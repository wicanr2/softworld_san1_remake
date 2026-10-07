#!/usr/bin/env python3
"""正常主選單讀取自然晚期存檔；輪詢圖片只存在容器暫存。"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import tempfile
import time
import traceback

ROOT = Path('/src')
PACK = ROOT/'workplace/hd-assets-mapcursor-v50-r1'
FIXTURE = ROOT/'workplace/credits-v44-fixture-r1'
NATIVE = ROOT/'workplace/hd-credits-v44-native-formal-r1/receipt.json'
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
digest = lambda data: hashlib.sha256(data).hexdigest()


def module(path):
    spec = importlib.util.spec_from_file_location('credits_gui', path)
    gui = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(gui)
    return gui


def crop(raw, width, x, y, w, h):
    return b''.join(raw[((y+r)*width+x)*3:((y+r)*width+x+w)*3] for r in range(h))


def run(out, editions):
    assert out.is_dir() and not (out/'receipt.json').exists()
    gui = module(out/'verify-window-inner.py')
    gui.OUT = out
    reference = json.loads(NATIVE.read_text())
    assert reference['passed'] and reference['all_full_canvas_pixels_compared'] and not reference['active_overlay']
    manifest = json.loads((PACK/'manifest.json').read_text())
    reference_assets = {n.split('/credits/',1)[1]:h for n,h in reference['inputs'].items()
                        if n.startswith('workplace/hd-assets-credits-v44-r4/credits/')}
    current_assets = {e['file'].removeprefix('credits/'):e['sha256'] for e in manifest['entries']
                      if e['file'].startswith('credits/')}
    assert reference_assets and reference_assets == current_assets
    assert all(sha(PACK/'credits'/n)==h for n,h in reference_assets.items())
    receipt = gui.receipt
    receipt.update(method='正常片頭、主選單讀檔、自然統一、三語／Theme、連續高清製作群及返回',
                   passed=False, state_injection=False, seed_injection=False, clock_injection=False,
                   audio=False, original_runtime_oracle=False, cases=[], samples=[], polls=[],
                   binary_sha256=sha(out/'san1-window-check'), pack_sha256=sha(PACK/'manifest.json'),
                   native_reference_sha256=sha(NATIVE), native_reference_file=str(NATIVE.relative_to(ROOT)),
                   reference_assets=reference_assets,
                   selection={'editions':editions,'locales':['zh-Hant','en','ja'],'continuous_runs':len(editions)},
                   environment={'cpus':8,'mesa_threads':2,'software_gl':True},
                   retention='具名代表完整畫面、選項操作及失敗末畫面；輪詢在容器暫存，沒有錄影')
    title = gui.rgb(out/'title-reference.png')
    menu = crop(title,640,40,27,568,160)
    menu_plate = crop(title,640,56,216,84,144)

    def save():
        (out/'receipt.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n')

    with tempfile.TemporaryDirectory(prefix='san1-hd-credits-') as transient:
        probe = Path(transient)/'poll.png'

        def poll(wid):
            g = gui.geo(wid)
            gui.run(['ffmpeg','-nostdin','-hide_banner','-loglevel','error','-threads','1','-y',
                     '-f','x11grab','-draw_mouse','0','-video_size',f"{g['WIDTH']}x{g['HEIGHT']}",
                     '-i',f":99+{g['X']},{g['Y']}",'-frames:v','1','-threads','1',str(probe)])
            return g, gui.rgb(probe)

        def keep(tag, g):
            destination = out/f'{tag}-{len(receipt["captures"]):04d}.png'
            assert not destination.exists()
            shutil.copyfile(probe,destination)
            receipt['captures'].append({'file':destination.name,'width':g['WIDTH'],'height':g['HEIGHT'],'sha256':sha(destination)})
            save()
            return destination

        def shot(wid, tag):
            g, _ = poll(wid)
            return keep(tag,g)

        gui.shot = shot

        def await_frame(wid, tag, predicate, seconds=20):
            began = time.monotonic()
            attempts = 0
            while time.monotonic()-began < seconds:
                g, raw = poll(wid)
                attempts += 1
                if predicate(g,raw):
                    p = keep(tag,g)
                    receipt['polls'].append({'name':tag,'attempts':attempts,'seconds':time.monotonic()-began,'kept':p.name})
                    save()
                    return p, raw
                time.sleep(.1)
            p = keep(tag+'-FAILED',g)
            receipt['polls'].append({'name':tag,'attempts':attempts,'failed_capture':p.name})
            save()
            raise RuntimeError(tag+' 畫面等待逾時')

        def leave():
            gui.run(['xdotool','mousemove','2700','1850'])

        def key(wid, value):
            gui.run(['xdotool','windowfocus','--sync',wid])
            gui.run(['xdotool','keydown','--clearmodifiers',value])
            try:
                time.sleep(.6)
            finally:
                gui.run(['xdotool','keyup',value])
            time.sleep(.25)
            receipt.setdefault('keys',[]).append(value)
            leave()

        def visible(g, raw):
            return raw[:12] == bytes([24,29,38])*4

        def hide(wid):
            gui.move(wid,616,16)
            gui.run(['xdotool','mousedown','1'])
            try:
                await_frame(wid,'toolbar-closed',lambda g,r:not visible(g,r),10)
            finally:
                gui.run(['xdotool','mouseup','1'])
            leave()

        def theme(wid, high):
            gui.choose_ready(wid,1,int(high))
            gui.run(['xdotool','windowsize',wid,'2560' if high else '640','1760' if high else '440'])
            gui.run(['xdotool','windowmove',wid,'0','0'])
            leave()

        def game(g, raw, high=False, bar=False):
            width = 2560 if high else 640
            height = 1632 if high else 408
            top = (128 if high else 32) if bar else 0
            if (g['WIDTH'],g['HEIGHT']) != (width,height+top):
                return None
            return crop(raw,width,0,top,width,height)

        def is_menu(g, raw):
            return ((g['WIDTH'],g['HEIGHT']) == (640,408)
                    and crop(raw,640,40,27,568,160) == menu
                    and crop(raw,640,56,216,84,144) == menu_plate)

        try:
            os.environ.update(DISPLAY=':99',LIBGL_ALWAYS_SOFTWARE='1',XDG_RUNTIME_DIR='/tmp')
            gui.start(['Xvfb',':99','-screen','0','2800x1900x24','-nolisten','tcp'],'xvfb')
            gui.wait(['xdotool','getdisplaygeometry'])
            for edition in editions:
                typed = json.loads((FIXTURE/edition/'receipt.json').read_text())
                assert typed['passed'] and typed['no_state_position_seed_clock_beat_injection']
                assert not typed['loaded_initial']['over'] and typed['loaded_initial']['owning_factions'] == 2
                case = {'edition':edition,'fixture_receipt_sha256':sha(FIXTURE/edition/'receipt.json'),'passed':False}
                receipt['cases'].append(case)
                saves = out/('saves-'+edition)
                shutil.copytree(FIXTURE/edition/'saves',saves)
                save_hashes = {str(p.relative_to(saves)):sha(p) for p in saves.rglob('*') if p.is_file()}
                assert save_hashes == {n.removeprefix('saves/'):h for n,h in typed['save_files_sha256'].items()}
                case['fixture_save_files'] = save_hashes
                rows = [r for r in reference['rows'] if r['edition']==edition and r['mode']=='new']
                assert len(rows)==470
                hall = next(r for r in rows if r['stage']=='hall')
                scrolls = {}
                for r in rows:
                    if r['stage']=='scroll':
                        scrolls.setdefault(r['native_rgb_sha256'],[]).append(r['scroll'])
                folder = '三國演義' if edition=='base' else '三國演義1加強版'
                proc = gui.start([str(out/'san1-window-check'),'-root','/orig/'+folder,'-edition',edition,
                                  '-ai',edition,'-scale','1','-music=false','-sound=false','-saves',str(saves),
                                  '-hd-assets',str(PACK)],edition)
                wid = gui.wait(['xdotool','search','--onlyvisible','--pid',str(proc.pid),'--name','三國演義 remake']).splitlines()[0]
                gui.run(['xdotool','windowmove',wid,'0','0'])
                gui.run(['xdotool','windowfocus','--sync',wid])
                leave()
                # 正常按空白鍵收片頭；每次先看主選單，避免把按鍵送進新局。
                began = time.monotonic()
                while time.monotonic()-began < 45:
                    g, raw = poll(wid)
                    if is_menu(g,raw):
                        case['initial_menu'] = keep(edition+'-initial-menu',g).name
                        break
                    key(wid,'space')
                else:
                    keep(edition+'-opening-FAILED',g)
                    raise RuntimeError('正常片頭未到主選單')
                gui.check(edition+'-default-original-hidden',True)
                key(wid,'2')
                initial_load, load_pixels = await_frame(wid,edition+'-load-panel',lambda g,r:(g['WIDTH'],g['HEIGHT'])==(640,408) and not is_menu(g,r))
                load_plate = crop(load_pixels,640,56,216,84,144)
                load_slot = crop(load_pixels,640,158,216,178,32)
                case['initial_load_panel'] = initial_load.name
                key(wid,'1')
                await_frame(wid,edition+'-natural-hall',lambda g,r:(page:=game(g,r)) is not None and digest(page)==hall['cpu_rgb_sha256'],120)
                gui.check(edition+'-loaded-natural-unification',True)
                key(wid,'Escape')
                await_frame(wid,edition+'-hall-toolbar',visible)
                for n, locale in enumerate(['zh-Hant','en','ja']):
                    gui.choose_ready(wid,0,n)
                    leave()
                    gui.check(edition+'-'+locale+'-window-language',('繁體中文','English','日本語')[n] in gui.run(['xdotool','getwindowname',wid]))
                    original, raw = await_frame(wid,edition+'-'+locale+'-hall-original',lambda g,r:(page:=game(g,r,bar=True)) is not None and digest(page)==hall['cpu_rgb_sha256'])
                    theme(wid,True)
                    high, _ = await_frame(wid,edition+'-'+locale+'-hall-hd',lambda g,r:(page:=game(g,r,True,True)) is not None and digest(page)==hall['native_rgb_sha256'])
                    theme(wid,False)
                    restored, _ = await_frame(wid,edition+'-'+locale+'-hall-restored',lambda g,r:(page:=game(g,r,bar=True)) is not None and digest(page)==hall['cpu_rgb_sha256'])
                    receipt['samples'].append({'edition':edition,'locale':locale,'original':original.name,'high':high.name,'restored':restored.name,
                                               'cpu_rgb_sha256':hall['cpu_rgb_sha256'],'native_rgb_sha256':hall['native_rgb_sha256']})
                    gui.check(edition+'-'+locale+'-complete-hall-and-restore',True)
                gui.choose_ready(wid,0,0)
                theme(wid,True)
                hide(wid)
                gui.run(['xdotool','windowsize',wid,'2560','1632'])
                gui.run(['xdotool','windowmove',wid,'0','0'])
                await_frame(wid,edition+'-hall-hd-hidden',lambda g,r:(page:=game(g,r,True)) is not None and digest(page)==hall['native_rgb_sha256'])
                key(wid,'space')
                def visible_scroll(g, raw):
                    page = game(g,raw,True)
                    candidates = scrolls.get(digest(page),[]) if page is not None else []
                    return bool(candidates) and all(400<=n<=740 for n in candidates)
                rolling, raw = await_frame(wid,edition+'-continuous-visible-scroll',visible_scroll,90)
                case['scroll'] = {'file':rolling.name,'candidates':scrolls[digest(raw)]}
                if edition == 'base':
                    entry = next(e for e in manifest['entries'] if e['edition']==edition and e['container']=='DATA3' and e['name']=='SCG16.IMG')
                    assert sha(PACK/entry['file']) == entry['sha256']
                    scene = gui.rgb(PACK/entry['file'])
                    assert len(scene)==704*384*3
                    ending, _ = await_frame(wid,edition+'-complete-ending-scene',lambda g,r:(g['WIDTH'],g['HEIGHT'])==(2560,1632) and crop(r,2560,1728,320,704,384)==scene,90)
                    case['ending'] = {'file':ending.name,'name':'SCG16.IMG','rect':[1728,320,704,384],'source_file':entry['file'],'sha256':entry['sha256']}
                else:
                    source_receipt = out/'ending-reference/receipt.json'
                    command = json.loads(source_receipt.read_text())
                    assert command['passed'] and not command['overlay'] and not command['normal_GUI']
                    target = next(r for r in command['rows'] if r['edition']==edition and r['high'])
                    source = out/'ending-reference'/target['file']
                    assert sha(source)==target['sha256']
                    expected = gui.rgb(source)
                    ending, _ = await_frame(wid,edition+'-complete-ending-command',lambda g,r:(g['WIDTH'],g['HEIGHT'])==(2560,1632) and r==expected,90)
                    case['ending'] = {'file':ending.name,'name':'normal-ended-command','rect':target['rect'],
                                      'reference_file':str(source.relative_to(ROOT)),'sha256':target['sha256'],
                                      'reference_receipt':str(source_receipt.relative_to(ROOT)),'reference_receipt_sha256':sha(source_receipt)}
                gui.check(edition+'-continuous-hd-completed',True)
                key(wid,'Escape')
                await_frame(wid,edition+'-ending-toolbar',visible)
                theme(wid,False)
                hide(wid)
                gui.run(['xdotool','windowsize',wid,'640','408'])
                gui.run(['xdotool','windowmove',wid,'0','0'])
                key(wid,'space')
                final_menu, _ = await_frame(wid,edition+'-returned-menu',is_menu,25)
                case['returned_menu'] = final_menu.name
                gui.check(edition+'-normal-return-to-menu',True)
                key(wid,'2')
                panel, _ = await_frame(wid,edition+'-returned-load-panel',lambda g,r:
                                      (g['WIDTH'],g['HEIGHT'])==(640,408)
                                      and crop(r,640,56,216,84,144)==load_plate
                                      and crop(r,640,158,216,178,32)==load_slot,15)
                case['returned_load_panel'] = panel.name
                gui.check(edition+'-menu-load-command-operates',True)
                key(wid,'shift+Escape')
                await_frame(wid,edition+'-load-cancelled-menu',is_menu)
                gui.check(edition+'-normal-load-cancel',True)
                gui.check(edition+'-source-saves-unchanged',save_hashes=={str(p.relative_to(saves)):sha(p) for p in saves.rglob('*') if p.is_file()})
                case['passed'] = True
                gui.stop(proc)
                save()
            receipt['passed'] = True
        except Exception as error:
            receipt['failure'] = {'message':str(error),'traceback':traceback.format_exc()}
            print(traceback.format_exc(),flush=True)
        finally:
            for process in reversed(gui.processes):
                gui.stop(process)
            for log in gui.logs:
                log.close()
            receipt['process_cleanup'] = all(p.poll() is not None for p in gui.processes)
            save()
    if not receipt['passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',required=True)
    parser.add_argument('--edition',choices=['base','plus'],action='append',help='預設兩版；補驗可指定一版')
    args = parser.parse_args()
    destination = (ROOT/args.out).resolve()
    assert destination.is_relative_to(ROOT/'workplace')
    assert destination.stat().st_uid == os.getuid() and destination.stat().st_gid == os.getgid()
    editions = args.edition or ['base','plus']
    assert len(set(editions)) == len(editions)
    run(destination, editions)
