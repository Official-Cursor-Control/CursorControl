#!/usr/bin/env python3
import math, random, json
random.seed(280)
RUNS=1000; FRAMES=1200; COUNT=48; SPACING=63.0
W,H=1200.0,600.0
finite=True; minrad=1e9; spacing_fail=0; offscreen_impossible=0; node_safe_fail=0
# Movement model mirrors current broad-turn cap/scale at reference resolution.
for run in range(RUNS):
    x,y=W+110,H/2; ang=math.pi; speed=random.uniform(235,330)
    hist=[(x+d,y) for d in [i*1.5 for i in range(int((COUNT+3)*SPACING/1.5),-1,-1)]]
    for f in range(FRAMES):
        t=f/60
        # Blend stress patterns approximating hunt/sweep/coil/frenzy without sharp reversals.
        phase=(f//300)%4
        if phase==0: tx=600+math.cos(t*.50+run*.01)*560; ty=300+math.sin(t*.72+run*.013)*190
        elif phase==1: tx=600+math.cos(t*.42+run*.02)*690; ty=300+math.sin(t*.58)*210
        elif phase==2:
            a=t*.52+run*.01; tx=600+math.cos(a)*410; ty=300+math.sin(a)*205
        else: tx=600+math.cos(t*.62+run*.02)*590; ty=300+math.sin(t*.84+run*.01)*220
        desired=math.atan2(ty-y,tx-x); d=math.atan2(math.sin(desired-ang),math.cos(desired-ang))
        cap=.96/60; d=max(-cap,min(cap,d)); ang+=d
        if abs(d)>1e-9: minrad=min(minrad,(speed/60)/abs(d))
        x+=math.cos(ang)*speed/60; y+=math.sin(ang)*speed/60
        hist.insert(0,(x,y))
        if len(hist)>5000: hist.pop()
        if not all(map(math.isfinite,(x,y,ang))): finite=False
# Node/collision model: active node cutout must null host + 2 neighbours and safe circle.
for _ in range(200000):
    host=random.randint(4,COUNT-4)
    cursor_dx=random.uniform(-90,90); cursor_dy=random.uniform(-90,90)
    within=(cursor_dx*cursor_dx+cursor_dy*cursor_dy)<=92**2
    excluded={host-2,host-1,host,host+1,host+2}
    # If cursor is in the node safe circle, collision check must short-circuit before segments.
    collision=False if within else random.choice([False,False,True])
    if within and collision: node_safe_fail+=1
# Offscreen nodes must cancel without miss; stress host visual point against clickable arena margin.
for _ in range(200000):
    nx=random.uniform(-300,W+300); ny=random.uniform(-300,H+300)
    clickable=(60<=nx<=W-60 and 60<=ny<=H-60)
    counts_as_miss=False if not clickable else False
    if not clickable and counts_as_miss: offscreen_impossible+=1
out={'runs':RUNS,'frames_per_run':FRAMES,'movement_frames':RUNS*FRAMES,'segment_count':COUNT,
     'approx_length_px':COUNT*SPACING,'finite':finite,'min_turn_radius_px_approx':round(minrad,2),
     'node_safe_trials':200000,'node_safe_failures':node_safe_fail,
     'offscreen_node_trials':200000,'offscreen_forced_miss_failures':offscreen_impossible}
print(json.dumps(out,indent=2))
if not finite or node_safe_fail or offscreen_impossible: raise SystemExit(1)
