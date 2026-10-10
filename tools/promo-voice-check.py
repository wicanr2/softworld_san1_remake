#!/usr/bin/env python3
"""Verify the complete original advice waveform in capture and mixed trailer."""
import argparse
import array
import hashlib
import json
import math
import cmath
from pathlib import Path
import subprocess

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--capture',type=Path,required=True)
p.add_argument('--video',type=Path,required=True)
p.add_argument('--out',type=Path,required=True)
p.add_argument('--full-search',action='store_true')
p.add_argument('--qa',type=Path,help='QA timeline when the opening edit changes shot positions')
a=p.parse_args()
sha=lambda f:hashlib.sha256(f.read_bytes()).hexdigest()
reference=a.capture/'adviser-361.pcm'
def decode(path,raw=False):
 command=['ffmpeg','-nostdin','-v','error']
 if raw:command+=['-f','s16le','-ar','48000','-ac','2']
 command+=['-i',str(path),'-vn','-ar','6000','-ac','1','-f','f32le','-']
 data=array.array('f');data.frombytes(subprocess.check_output(command));return data
ref=decode(reference,True)
mean=sum(ref)/len(ref);r=[x-mean for x in ref];energy=sum(x*x for x in r)
assert energy>0
def locate(path,begin,end):
    data=decode(path)
    offset=int(begin*6000)
    window=list(data[offset:int(end*6000)+len(r)])
    size=1
    while size<len(window)+len(r)-1:size*=2
    def fft(values,inverse=False):
        n=len(values);j=0
        for i in range(1,n):
            bit=n>>1
            while j&bit:j^=bit;bit>>=1
            j^=bit
            if i<j:values[i],values[j]=values[j],values[i]
        length=2
        while length<=n:
            step=cmath.exp((2j if inverse else -2j)*math.pi/length)
            for start in range(0,n,length):
                w=1;half=length//2
                for k in range(half):
                    u=values[start+k];v=values[start+k+half]*w
                    values[start+k],values[start+k+half]=u+v,u-v;w*=step
            length*=2
        if inverse:
            for i in range(n):values[i]/=n
    left=list(map(complex,window))+[0j]*(size-len(window))
    right=list(map(complex,reversed(r)))+[0j]*(size-len(r))
    fft(left);fft(right)
    for i in range(size):left[i]*=right[i]
    fft(left,True)
    sums=[0.0];squares=[0.0]
    for x in window:sums.append(sums[-1]+x);squares.append(squares[-1]+x*x)
    scores=[]
    for n in range(min(int((end-begin)*6000),len(window)-len(r)+1)):
        total=sums[n+len(r)]-sums[n]
        variance=squares[n+len(r)]-squares[n]-total*total/len(r)
        denominator=math.sqrt(max(0,variance)*energy)
        scores.append((left[n+len(r)-1].real/denominator if denominator else -1,n+offset))
    best=max(scores)
    return {'file':path.name,'sha256':sha(path),'start_seconds':best[1]/6000,
            'complete_reference_correlation':best[0],'reference_seconds':len(ref)/6000}
raw=locate(a.capture/'war-advice.mp4',12,16)
movie_begin = next(s['begin'] for s in json.loads(a.qa.read_text())['sequence']
                   if s['scene'] == 'war-advice') if a.qa else 27
clip_begin = json.loads((a.capture/'edit-plan.json').read_text())['war-advice']['start']
duration = float(json.loads(subprocess.check_output(['ffprobe','-v','error','-show_format','-of','json',str(a.video)]))['format']['duration'])
mixed=locate(a.video,0,duration) if a.full_search else locate(a.video,movie_begin,movie_begin+4)
print(json.dumps({'raw':raw,'mixed':mixed},ensure_ascii=False),flush=True)
assert raw['complete_reference_correlation']>.90,raw
assert mixed['complete_reference_correlation']>.80,mixed
assert abs(mixed['start_seconds']-(movie_begin+raw['start_seconds']-clip_begin))<.10, 'voice must align with the same captured shot'
result={'passed':True,'text':'兵者  貴神速','indices':[361,499,499],
 'reference_sha256':sha(reference),'method':'complete waveform Pearson correlation at 6 kHz; lossy AAC capture and mixed soundtrack',
 'raw_capture':raw,'mixed_trailer':mixed,'human_listening':False}
assert not a.out.exists()
a.out.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
print(json.dumps(result,ensure_ascii=False))
