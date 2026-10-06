#!/usr/bin/env python3
"""獨立回讀 PulseAudio WAV 與已保存的 remake PCM，不啟動遊戲。"""
import argparse
import array
import hashlib
import json
from pathlib import Path
import struct
import wave

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def samples(pcm):
    result = array.array('h', pcm)
    assert struct.pack('=H',1)==b'\x01\x00'
    assert len(result)%2==0
    assert result[::2]==result[1::2], '左右聲道與單聲道來源不符'
    return result[::2]

def positions(reference, query, offset=0):
    """量化容許最多兩個 16-bit 單位；完整片段另逐樣本驗證。"""
    stop=len(reference)-len(query)+1
    for value in range(query[0]-2,query[0]+3):
        at=offset
        while at<stop:
            try: at=reference.index(value,at,stop)
            except ValueError: break
            if all(abs(reference[at+i]-v)<=2 for i,v in enumerate(query)):
                yield at
            at+=1

def music_alignment(reference_pcm, captured_pcm):
    reference,captured=map(samples,(reference_pcm,captured_pcm))
    if len(captured)<48000: return None
    # 避開 monitor 啟動時的留白，完整比較仍包含所有錄音樣本。
    anchor=next((i for i in range(24000,min(len(captured)-128,96000))
                 if abs(captured[i])>100),None)
    if anchor is None:
        # 曲尾可能只有前半秒留下低音量波形。此回退只接受整段 bytes 相同，
        # 全靜音仍無法獨立定位；不放寬一般波形的容差或排除錄音幀。
        peak=max(range(len(captured)),key=lambda i:abs(captured[i]))
        if abs(captured[peak])<=2:return None
        anchor=min(peak,len(captured)-128)
        query=captured_pcm[anchor*4:(anchor+128)*4]
        at=reference_pcm.find(query)
        while at>=0:
            begin=at-anchor*4
            if begin>=0 and at%4==0 and reference_pcm.startswith(captured_pcm,begin):
                return dict(reference_start_frame=begin//4,frames=len(captured),
                            maximum_sample_delta=0,excluded_frames=0)
            at=reference_pcm.find(query,at+1)
        return None
    for at in positions(reference,captured[anchor:anchor+64]):
        begin=at-anchor
        if begin<0 or begin+len(captured)>len(reference):continue
        maximum=0
        for i,v in enumerate(captured):
            delta=abs(v-reference[begin+i])
            if delta>2:break
            maximum=max(maximum,delta)
        else:
            return dict(reference_start_frame=begin,frames=len(captured),
                        maximum_sample_delta=maximum,excluded_frames=0)
    return None

def sfx_matches(reference_pcm, captured_pcm):
    reference,captured=map(samples,(reference_pcm,captured_pcm))
    anchor=next(i for i,v in enumerate(reference) if abs(v)>100)
    result=[]
    end=0
    for at in sorted(positions(captured,reference[anchor:anchor+64])):
        begin=at-anchor
        if begin<end or begin<0 or begin+len(reference)>len(captured):continue
        maximum=max(abs(v-captured[begin+i]) for i,v in enumerate(reference))
        if maximum<=2:
            result.append(dict(start_frame=begin,frames=len(reference),maximum_sample_delta=maximum))
            end=begin+len(reference)
    # 片段以外也應全靜音，避免只挑少數碰巧相同的波形。
    nonzero_outside=0
    intervals=iter(result)
    active=next(intervals,None)
    for i,v in enumerate(captured):
        while active and i>=active['start_frame']+active['frames']:active=next(intervals,None)
        covered=active and active['start_frame']<=i<active['start_frame']+active['frames']
        if not covered and abs(v)>2:nonzero_outside+=1
    return dict(clips=result,nonzero_frames_outside_clips=nonzero_outside,
                captured_frames=len(captured),reference_frames=len(reference))

def read_wav(path):
    with wave.open(str(path)) as wav:
        assert (wav.getnchannels(),wav.getframerate(),wav.getsampwidth())==(2,48000,2)
        return wav.readframes(wav.getnframes())

def verify(out, reference_dir):
    receipt=json.loads((out/'receipt.json').read_text())
    music=(reference_dir/'music-reference.pcm').read_bytes()
    sfx=(reference_dir/'sfx-reference.pcm').read_bytes()
    if 'reference_files' in receipt:
        for name,digest in receipt['reference_files'].items():
            assert sha(reference_dir/name)==digest, 'PCM 或游標來源與擷取收據不符'
    proofs=[]
    controls=[]
    errors=[] if receipt.get('passed') else ['GUI 收據未完成；只回讀已保存樣本，不宣稱完整通過']
    for entry in receipt.get('audio',[]):
        path=out/entry['file']
        assert sha(path)==entry['sha256']
        pcm=read_wav(path)
        if entry['kind']=='music':
            matched=music_alignment(music,pcm)
            if not matched:
                values=samples(pcm)
                matched=dict(continuous=False,frames=len(values),zero_frames=sum(v==0 for v in values))
                errors.append(entry['file']+' 不符合完整連續參考')
                proofs.append(dict(file=entry['file'],kind=entry['kind'],passed=False,proof=matched))
                continue
            # 同一真實錄音插入一段重啟前奏，完整比較必須拒絕。
            middle=(len(pcm)//8)*4
            mutated=pcm[:middle]+music[:48000]+pcm[middle+48000:]
            rejected=music_alignment(music,mutated) is None
            assert rejected, '重啟負例未被拒絕'
            controls.append(dict(file=entry['file'],restart_rejected=rejected))
        elif entry['kind']=='sfx':
            matched=sfx_matches(sfx,pcm)
            count=len(matched['clips'])
            if count not in entry.get('expected_clip_counts',[entry.get('expected_clips')]) or matched['nonzero_frames_outside_clips']:
                errors.append(entry['file']+' 完整音效片段數或片段外樣本不符')
        else:
            values=samples(pcm)
            assert max(map(abs,values))<=2
            matched=dict(silent_frames=len(values))
        proofs.append(dict(file=entry['file'],kind=entry['kind'],passed=not any(e.startswith(entry['file']) for e in errors),proof=matched))
    selection=receipt.get('selection',dict(scope='all',editions=['base','plus'],locales=['zh-Hant','en','ja']))
    scope=selection['scope']
    expected=len(selection['editions'])*((2 if scope in ['all','music'] else 0)+
                                        (5*len(selection['locales']) if scope in ['all','sound'] else 0))
    if len(proofs)!=expected:
        errors.append(f'錄音 {len(proofs)}/{expected}，所選範圍尚未完成')
    listed={entry['file'] for entry in receipt.get('audio',[])}
    unlisted=[dict(file=p.name,sha256=sha(p)) for p in sorted(out.glob('*.wav')) if p.name not in listed]
    if unlisted:errors.append('有未登錄的 WAV，沒有音量或操作驗收')
    captures=receipt['captures']
    for capture in captures:
        path=out/capture['file']
        assert sha(path)==capture['sha256']
        data=path.read_bytes()
        assert data[:8]==b'\x89PNG\r\n\x1a\n'
        assert struct.unpack('>II',data[16:24])==(capture['width'],capture['height'])
    return dict(passed=not errors,errors=errors,method='標準 WAV 與完整 PCM 回讀，未呼叫 GUI 或遊戲播放器',
                receipt_sha256=sha(out/'receipt.json'),proofs=proofs,controls=controls,
                captures_checked=len(captures),expected_audio=expected,selection=selection,unlisted_wavs=unlisted,
                reference_sha256=dict(music=sha(reference_dir/'music-reference.pcm'),sfx=sha(reference_dir/'sfx-reference.pcm')))

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',required=True)
    parser.add_argument('--reference',required=True)
    args=parser.parse_args()
    root=Path('/src')
    result=verify(root/args.out,root/args.reference)
    path=root/args.out/'pcm-proof.json'
    with path.open('x') as f:json.dump(result,f,ensure_ascii=False,indent=2);f.write('\n')
    print(json.dumps(dict(passed=result['passed'],audio=len(result['proofs']),captures=result['captures_checked'],errors=result['errors']),ensure_ascii=False))
    raise SystemExit(0 if result['passed'] else 1)
