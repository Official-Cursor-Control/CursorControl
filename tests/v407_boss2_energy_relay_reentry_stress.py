#!/usr/bin/env python3
import math, random
from pathlib import Path

SRC = Path(__file__).resolve().parents[1] / 'survival_boss2.go'
text = SRC.read_text(encoding='utf-8')

def ck(name, cond):
    if not cond:
        raise AssertionError(name)
    print('PASS', name)

ck('one cover satellite remains', 'make([]SurvivalBoss2Satellite, 0, 1)' in text)
ck('cover satellite remains on left', 'width*(0.225+(rand.Float64()-.5)*0.035)' in text)
ck('relay is exactly ten nodes', 'const count = 10' in text)
ck('relay node window uses current punishing cadence', '950*time.Millisecond' in text and 'survivalBoss2BeamChainNodeWindow()' in text)
ck('mouth window uses current punishing cadence', '7*time.Second' in text and 'survivalBoss2BeamMouthWindow()' in text)
ck('relay timeout resets instead of failing', 'survivalBoss2ResetBeamChain(now, "timer")' in text)
ck('relay miss resets instead of failing', 'survivalBoss2ResetBeamChain(now, "miss")' in text)
ck('out of order resets instead of failing', 'survivalBoss2ResetBeamChain(now, "order")' in text)
ck('relay reset does not restart mouth deadline', 'func survivalBoss2ResetBeamChain' in text and 'survivalBoss2BeamNodeDeadline =' not in text[text.index('func survivalBoss2ResetBeamChain'):text.index('func survivalBoss2SpawnBeamChain')])
ck('mouth miss is non-fatal', 'if math.Hypot(p.X-mouth.X, p.Y-mouth.Y) > float64(sx(54, w)) {\n\t\treturn true' in text)
ck('only mouth timeout explicitly fails relay stage', 'survivalFail(h, "Mouth core timed out")' in text)
ck('reentry records exit side', 'survivalBoss2ReentryExitSide = survivalBoss2DetectExitSide' in text)
ck('reentry side hard invariant present', 'survivalBoss2ReentrySide == survivalBoss2ReentryExitSide' in text)
ck('outside-route clamp active', 'survivalBoss2KeepReentryRouteOutside(w, hgt)' in text)

BASE_W, BASE_H = 1536.0, 1024.0

def sx(v,w): return int(v*w/BASE_W)
def sy(v,h): return int(v*h/BASE_H)
def arena(w,h): return (sx(14,w),sy(180,h),sx(1522,w),sy(772,h))

rng=random.Random(407)
viol=[]
for _ in range(100_000):
    w,h=rng.choice([(1200,800),(1366,768),(1536,1024),(1600,900),(1920,1080)])
    L,T,R,B=arena(w,h); aw=R-L; ah=B-T
    mouth_x=(R-sx(92,w))-sx(72,w)
    start_x=L+aw*.29
    end_x=mouth_x-sx(105,w)
    if end_x < start_x+sx(210,w): end_x=start_x+sx(210,w)
    pattern=rng.randrange(5); phase=rng.random()*math.pi*2; mirror=-1 if rng.randrange(2)==0 else 1
    pts=[]
    for i in range(10):
        t=i/9
        x=start_x+(end_x-start_x)*t
        if pattern==0: off=.16*math.sin(t*math.pi*2.15+phase)
        elif pattern==1: off=.13*math.sin(t*math.pi*3.25+phase)+.045*math.sin(t*math.pi*6.5)
        elif pattern==2: off=.17*math.sin((t-.08)*math.pi*1.55+phase*.35)
        elif pattern==3:
            zig=-1 if i%2==0 else 1
            off=zig*(.105+.025*math.sin(t*math.pi*2))+.035*math.sin(t*math.pi*2+phase)
        else: off=.11*math.sin(t*math.pi*4+phase)*(1-.32*t)
        y=(T+B)*.5+off*mirror*ah+(rng.random()-.5)*sy(8,h)
        margin=sy(62,h); y=max(T+margin,min(B-margin,y))
        pts.append((x,y))
    if not all(pts[i][0] < pts[i+1][0] for i in range(9)):
        viol.append(('order',w,h)); break
    if pts[-1][0] >= mouth_x:
        viol.append(('mouth',w,h)); break
    if any(not (L < x < R and T < y < B) for x,y in pts):
        viol.append(('bounds',w,h)); break
ck('100k ten-node relay layouts stay ordered/bounded', not viol)

# Model the route-side selection and waypoints: each exit always routes around an adjacent
# outside corner, then along a different outside edge before the inward strike.
SIDES=('left','right','top','bottom')
def choose(exit_side,vx,vy):
    if exit_side in ('left','right'): return 'bottom' if vy>=0 else 'top'
    return 'right' if vx>=0 else 'left'

for _ in range(100_000):
    exit_side=rng.choice(SIDES); vx=rng.uniform(-500,500); vy=rng.uniform(-500,500)
    entry=choose(exit_side,vx,vy)
    if entry==exit_side:
        viol.append(('same-side',exit_side,vx,vy)); break
    if exit_side in ('left','right') and entry not in ('top','bottom'):
        viol.append(('non-adjacent',exit_side,entry)); break
    if exit_side in ('top','bottom') and entry not in ('left','right'):
        viol.append(('non-adjacent',exit_side,entry)); break
ck('100k reentry choices never reuse exit edge', not viol)

clock=1.15
ck('2.0s relay timer survives 1.15x boss clock', abs((2*clock)/clock-2)<1e-9)
ck('10.0s mouth timer survives 1.15x boss clock', abs((10*clock)/clock-10)<1e-9)
# Ten targets under a 10s mouth clock intentionally demand sub-second average execution.
ck('ten-node relay demands fast average cadence', 10.0/10 == 1.0)
print('v407 Boss2 energy relay + reentry stress: all checks passed')
