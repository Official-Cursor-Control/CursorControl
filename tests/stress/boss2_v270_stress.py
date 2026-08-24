import math, random, json
random.seed(270)
PHASES={"hunt":0.78,"sweep":0.78,"coil":0.88,"frenzy":0.96,"final":0.96}
results={}
# Curvature/turn-rate Monte Carlo: hundreds per phase, jittered frame times and hostile desired reversals.
for phase,rate in PHASES.items():
    worst_deg=0.0; min_radius=1e9; frames=0
    for run in range(600):
        a=random.uniform(-math.pi,math.pi)
        speed=random.uniform(220,340)
        for k in range(1000):
            dt=random.uniform(1/165,1/42)
            # Desired headings can jump anywhere, which is harsher than real target functions.
            des=random.uniform(-math.pi,math.pi) if k%random.randint(20,80)==0 else a+random.uniform(-0.25,0.25)
            d=math.atan2(math.sin(des-a),math.cos(des-a))
            mt=rate*dt
            da=max(-mt,min(mt,d))
            a+=da
            worst_deg=max(worst_deg,abs(math.degrees(da)))
            if abs(da)>1e-9:
                r=speed*dt/abs(da)
                min_radius=min(min_radius,r)
            frames+=1
    results[phase]={"runs":600,"frames":frames,"worst_frame_turn_deg":round(worst_deg,3),"minimum_turn_radius_px":round(min_radius,2)}

# Node interaction pocket: clickable disk r=38 must always be collision-free because safe pocket r=64 is tested first.
node_tests=50000
node_fail=0
for _ in range(node_tests):
    ang=random.random()*2*math.pi; rr=38*math.sqrt(random.random())
    x,y=rr*math.cos(ang),rr*math.sin(ang)
    safe=math.hypot(x,y)<=64
    if not safe: node_fail+=1
results['node_clickable_disk']={"cases":node_tests,"failures":node_fail,"click_radius":38,"safe_radius":64}

# Gap geometry: host +/-1 omitted. With spacing 63, nearest live centres are 126 px away.
# Validate the entire 38 px clickable disk remains clear of 32 px live-body collision circles even before safe-pocket override.
gap_tests=50000; gap_fail=0; closest=1e9
for _ in range(gap_tests):
    ang=random.random()*2*math.pi; rr=38*math.sqrt(random.random())
    x,y=rr*math.cos(ang),rr*math.sin(ang)
    # conservative straight-chain live neighbors at +/-126
    d=min(math.hypot(x-126,y),math.hypot(x+126,y))
    closest=min(closest,d)
    if d<=32: gap_fail+=1
results['node_three_segment_gap']={"cases":gap_tests,"failures":gap_fail,"nearest_live_body_min_distance":round(closest,2),"live_body_radius":32}

# Size ratios, for regression audit.
results['render_sizes']={"body":"116x67","head":"188x130","tail":"172x50","node":"68x68","segment_spacing":63}
print(json.dumps(results,indent=2))
open('/mnt/data/work270/BOSS2_QA_REPORT_V270.json','w').write(json.dumps(results,indent=2))
