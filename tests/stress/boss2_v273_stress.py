import math, random, json
RUNS=500
FRAMES=900
COUNT=48
SPACING=63.0
arena=(0.0,0.0,1200.0,600.0)
# Entry geometry: head starts 110px right of arena, tail approx COUNT*spacing behind to the right.
head_x=arena[2]+110
tail_x=head_x+(COUNT*SPACING)
entry_ok = tail_x > arena[2] + arena[2] # at least another screen width offscreen
# approximate broad-turn path stress using same turn caps / speeds
mins=[]
finite=True
for r in range(RUNS):
    x,y=head_x,300.0
    ang=math.pi
    speed=250.0
    prev=ang
    minrad=1e9
    for f in range(FRAMES):
        t=f/60.0
        tx=600+math.cos(t*.50+r*.01)*610
        ty=300+math.sin(t*.72+r*.013)*200
        da=math.atan2(ty-y,tx-x)
        d=math.atan2(math.sin(da-ang),math.cos(da-ang))
        maxturn=.96/60
        d=max(-maxturn,min(maxturn,d))
        ang += d
        x += math.cos(ang)*speed/60
        y += math.sin(ang)*speed/60
        if abs(d)>1e-8:
            minrad=min(minrad,(speed/60)/abs(d))
        if not all(map(math.isfinite,(x,y,ang))): finite=False
    mins.append(minrad)
# node candidates: with 48 segments, visible middle body must leave candidates under common straight entries
node_candidate_trials=10000
node_ok=0
for _ in range(node_candidate_trials):
    hx=random.uniform(300,1000)
    # random horizontal-ish chain across arena
    pts=[(hx+i*SPACING, random.uniform(180,420)) for i in range(COUNT)]
    cand=[i for i,(x,y) in enumerate(pts) if 4<=i<COUNT-2 and 120<x<1080 and 120<y<480]
    if cand: node_ok+=1
out={
 'runs':RUNS,'frames_per_run':FRAMES,'segment_count':COUNT,
 'approx_serpent_length_px':COUNT*SPACING,
 'entry_tail_x_px':tail_x,'entry_tail_more_than_one_screen_off_right':entry_ok,
 'finite_movement_runs':finite,'min_turn_radius_px_approx':round(min(mins),2),
 'node_candidate_trials':node_candidate_trials,'node_candidate_success':node_ok
}
print(json.dumps(out,indent=2))
open('BOSS2_QA_REPORT_V273.json','w').write(json.dumps(out,indent=2)+'\n')
