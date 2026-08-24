#!/usr/bin/env python3
import math, random, statistics, sys

L,T,R,B = 80.0,80.0,1200.0,640.0
BASE=250.0
TURN=1.10
DURATION=6.0
SIDES=('left','right','top','bottom')
FPS=(60,120,144)
RUNS_PER_SIDE_FPS=500
random.seed(281)

def norm(x,y):
    d=math.hypot(x,y)
    return (1.0,0.0) if d < 1e-9 else (x/d,y/d)

def one(side,fps):
    dt=1.0/fps
    target=(random.uniform(L+105,R-105), random.uniform(T+90,B-90))
    if side=='left':
        p=[L-random.uniform(1,90), random.uniform(T,B)]; a=random.uniform(math.radians(100),math.radians(260))
    elif side=='right':
        p=[R+random.uniform(1,90), random.uniform(T,B)]; a=random.uniform(math.radians(-80),math.radians(80))
    elif side=='top':
        p=[random.uniform(L,R), T-random.uniform(1,75)]; a=random.uniform(math.radians(190),math.radians(350))
    else:
        p=[random.uniform(L,R), B+random.uniform(1,75)]; a=random.uniform(math.radians(10),math.radians(170))
    speed=random.uniform(220,320)
    v=[math.cos(a)*speed, math.sin(a)*speed]
    entered=False; min_dist=1e9; max_turn=0.0; telegraphed=False
    side_now=side
    for k in range(int(DURATION/dt)+1):
        # Approximate the runtime's dynamic edge re-evaluation for the warning.
        if not telegraphed:
            left=L-p[0]; right=p[0]-R; top=T-p[1]; bottom=p[1]-B
            vals=[(left,'left'),(right,'right'),(top,'top'),(bottom,'bottom')]
            outside,side_now=max(vals,key=lambda z:z[0])
            inward={'left':v[0]>0,'right':v[0]<0,'top':v[1]>0,'bottom':v[1]<0}[side_now]
            if inward and 0 <= outside <= 220:
                telegraphed=True
        dx,dy=norm(target[0]-p[0],target[1]-p[1])
        desired=(dx*BASE*1.48,dy*BASE*1.48)
        cur_speed=math.hypot(*v); des_speed=math.hypot(*desired)
        ca=math.atan2(v[1],v[0]); da=math.atan2(desired[1],desired[0])
        delta=math.atan2(math.sin(da-ca),math.cos(da-ca))
        mt=TURN*dt
        delta=max(-mt,min(mt,delta)); max_turn=max(max_turn,abs(delta)/dt)
        na=ca+delta
        blend=min(1.0,dt*2.1); ns=cur_speed+(des_speed-cur_speed)*blend
        v=[math.cos(na)*ns,math.sin(na)*ns]
        p[0]+=v[0]*dt; p[1]+=v[1]*dt
        entered |= (L<=p[0]<=R and T<=p[1]<=B)
        min_dist=min(min_dist,math.hypot(p[0]-target[0],p[1]-target[1]))
    return entered,min_dist,max_turn,telegraphed

all_results=[]
for fps in FPS:
    for side in SIDES:
        rs=[one(side,fps) for _ in range(RUNS_PER_SIDE_FPS)]
        all_results += rs
        entered=sum(r[0] for r in rs); tele=sum(r[3] for r in rs)
        p95=sorted(r[1] for r in rs)[int(.95*len(rs))-1]
        print(f'{fps:3d} FPS {side:6s}: re-entered {entered}/{len(rs)}, telegraphed {tele}/{len(rs)}, p95 closest-to-lock {p95:.1f}px')

entered=sum(r[0] for r in all_results)
tele=sum(r[3] for r in all_results)
max_turn=max(r[2] for r in all_results)
within300=sum(r[1] <= 300 for r in all_results)
print(f'TOTAL: {len(all_results)} runs; re-entered={entered}; telegraphed={tele}; within300={within300}; max turn={max_turn:.3f} rad/s')
# Re-entry must always return to the arena and never exceed the configured broad-turn limit.
# Aim tolerance is intentionally forgiving because the player can bait the committed attack.
ok = entered == len(all_results) and max_turn <= TURN+1e-9 and tele >= int(len(all_results)*0.98) and within300 >= int(len(all_results)*0.90)
sys.exit(0 if ok else 1)
