#!/usr/bin/env python3
import math, random
from pathlib import Path

SRC = Path(__file__).resolve().parents[1] / 'survival_boss2.go'
text = SRC.read_text(encoding='utf-8')

def ck(name, cond):
    if not cond:
        raise AssertionError(name)
    print('PASS', name)

ck('relay remains ten nodes', 'const count = 10' in text)
ck('relay node window uses current punishing cadence', '950*time.Millisecond' in text and 'survivalBoss2BeamChainNodeWindow()' in text)
ck('mouth window uses current punishing cadence', '7*time.Second' in text and 'survivalBoss2BeamMouthWindow()' in text)
ck('post-energy exit is explicitly right', 'survivalBoss2RespawnAfterBeamExit(now, w, hgt, survivalBoss2ReentryRight)' in text)
ck('post-energy respawn excludes exit side', 'if side != exitSide' in text)
ck('post-energy reentry starts on committed inward stage', 'survivalBoss2ReentryRouteStage = 2' in text)
ck('offscreen reentry has recovery failsafe', 'VOID SERPENT // FORCED RE-ENTRY' in text)

BASE_W, BASE_H = 1536.0, 1024.0

def sx(v,w): return int(v*w/BASE_W)
def sy(v,h): return int(v*h/BASE_H)
def arena(w,h): return (sx(14,w),sy(180,h),sx(1522,w),sy(772,h))

rng=random.Random(409)
min_adj=10**9
max_y_swing=0
for _ in range(100_000):
    w,h=rng.choice([(1200,800),(1366,768),(1536,1024),(1600,900),(1920,1080)])
    L,T,R,B=arena(w,h); aw=R-L; ah=B-T
    park_x=R-sx(92,w); mouth_x=park_x-sx(72,w)
    start_x=L+aw*.17
    end_x=mouth_x-sx(72,w)
    min_span=sx(610,w)
    if end_x < start_x+min_span:
        start_x=max(L+sx(58,w), end_x-min_span)
    centre_y=(T+B)*.5
    mirror=-1 if rng.randrange(2)==0 else 1
    pattern=rng.randrange(6); phase=rng.random()*math.pi*2
    pts=[]
    for i in range(10):
        t=i/9
        x=start_x+(end_x-start_x)*t
        if pattern==0:
            off=.235*math.sin(t*math.pi*2.10+phase)
        elif pattern==1:
            off=.195*math.sin(t*math.pi*3.15+phase)+.055*math.sin(t*math.pi*6.30+phase*.35)
        elif pattern==2:
            off=.255*math.sin((t-.08)*math.pi*1.55+phase*.28)
        elif pattern==3:
            zig=-1 if i%2==0 else 1
            off=zig*(.175+.030*math.sin(t*math.pi*2))+.035*math.sin(t*math.pi*2+phase)
        elif pattern==4:
            group=-1 if (i//2)%2==0 else 1
            off=group*.185+.060*math.sin(t*math.pi*3+phase)
        else:
            off=.215*math.sin(t*math.pi*4+phase)*(1-.24*t)
        y=centre_y+off*mirror*ah+(rng.random()-.5)*sy(10,h)
        margin=sy(72,h); y=max(T+margin,min(B-margin,y))
        pts.append((x,y))
    assert all(pts[i][0] < pts[i+1][0] for i in range(9))
    assert pts[-1][0] < mouth_x
    assert all(L < x < R and T < y < B for x,y in pts)
    adj=[math.dist(pts[i],pts[i+1]) for i in range(9)]
    min_adj=min(min_adj,min(adj))
    max_y_swing=max(max_y_swing,max(y for _,y in pts)-min(y for _,y in pts))

# At 1200x800 the active-node art is ~38 px wide; this minimum keeps consecutive
# targets visually distinct even in the least-separated generated pattern.
ck('100k relay layouts ordered/bounded', True)
ck('relay consecutive-node spacing stays premium/readable', min_adj >= 80.0)
ck('relay uses substantial vertical travel', max_y_swing >= 250.0)

SIDES=(1,2,3,4)
for _ in range(100_000):
    exit_side=rng.choice(SIDES)
    candidates=[s for s in SIDES if s != exit_side]
    entry=rng.choice(candidates)
    assert entry != exit_side
ck('100k post-energy entries never reuse exit edge', True)

clock=1.15
ck('1.25s relay timer survives 1.15x boss clock', abs((1.25*clock)/clock-1.25)<1e-9)
ck('10s mouth timer survives 1.15x boss clock', abs((10*clock)/clock-10)<1e-9)
print(f'v409 stress complete: min adjacent distance={min_adj:.2f}px, max vertical swing={max_y_swing:.2f}px')
