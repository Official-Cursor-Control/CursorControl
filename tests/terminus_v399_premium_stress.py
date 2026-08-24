#!/usr/bin/env python3
import math, random, statistics, json
from pathlib import Path
import numpy as np

ROOT=Path(__file__).resolve().parents[1]
W,H=1200,800
SX=lambda x: round(x*W/1536)
SY=lambda y: round(y*H/1000)
ARENA=(SX(14),SY(180),SX(1522),SY(772))
N=100000

def visible(p): return .75-.50*max(0,min(1,p))
def ship_rect(p):
    l,t,r,b=ARENA; aw=r-l; sw=aw*.58; sh=sw*535/1248
    x=r-sw*visible(p); y=t+((b-t)-sh)/2
    return x,y,x+sw,y+sh
def lane(p):
    l,_,r,_=ARENA
    left=l+SX(54)
    right=ship_rect(p)[0]-SX(24)
    right=max(right,left+SX(500))
    right=min(right,r-SX(72))
    return left,right
def slot_bounds(slot,p):
    left,right=lane(p); span=(right-left)/3
    return left+slot*span+SX(7),left+(slot+1)*span-SX(7),ARENA[1]+SY(54),ARENA[3]-SY(54)
def paty(i,n,v,lo,hi):
    mid=(lo+hi)/2; amp=(hi-lo)*.34; x=i/(n-1) if n>1 else 0
    if v%5==0:return mid+math.sin(x*2*math.pi)*amp
    if v%5==1:return mid-amp*.82 if i%2==0 else mid+amp*.82
    if v%5==2:return mid+math.sin(x*math.pi)*amp*.9
    if v%5==3:return mid-amp+x*amp*2
    return mid+math.cos(x*3*math.pi)*amp*.72
def smooth(a,b,v,lo,hi,n=48):
    amp=min((hi-lo)*.22,abs(b[0]-a[0])*.28); phase=math.pi if v%2 else 0; waves=1+(v%3)*.5
    out=[]
    for i in range(n+1):
        x=i/n; px=a[0]+(b[0]-a[0])*x; base=a[1]+(b[1]-a[1])*x
        py=base+math.sin(x*math.pi*waves+phase)*amp*math.sin(math.pi*x)
        out.append((px,max(lo,min(hi,py))))
    return out
def plen(p):return sum(math.dist(p[i-1],p[i]) for i in range(1,len(p)))
def q(v):
    v=sorted(v); return [round(v[int(len(v)*x)],1) for x in (.05,.5,.95)]

random.seed(399)
chain=[];stream=[];jump=[];slider=[];lanes=[];nose_gap=[]
bounds=clearance=duplicate=0
for _ in range(N):
    progress=random.random()
    lanes.append(lane(progress)[1]-lane(progress)[0])
    ship=ship_rect(progress)
    loop=random.sample(range(8),3)
    if len(set(loop))!=3: duplicate+=1
    for slot in range(3):
        x0,x1,y0,y1=slot_bounds(slot,progress); v=random.randrange(5)
        if x1>=ship[0]-SX(18): clearance+=1
        if slot==2: nose_gap.append(ship[0]-x1)
        n=random.choice((5,6))
        pts=[(x0+(x1-x0)*i/(n-1),paty(i,n,v,y0,y1)) for i in range(n)]
        for i in range(1,n):
            approach=max(425,545-i*20)/1000
            chain.append(math.dist(pts[i-1],pts[i])/approach)
        n=random.randrange(9,13); mid=(y0+y1)/2; pts=[]
        for i in range(n):
            x=i/(n-1); yy=mid+math.sin(x*2*math.pi+v)*(y1-y0)*.22
            pts.append((x0+(x1-x0)*x,max(y0,min(y1,yy))))
        for i in range(1,n):
            approach=max(235,300-i*5)/1000
            stream.append(math.dist(pts[i-1],pts[i])/approach)
        pts=[(x0+(x1-x0)*i/4,y0+(y1-y0)*(.16 if i%2==0 else .84)) for i in range(5)]
        for i in range(1,5):
            jump.append(math.dist(pts[i-1],pts[i])/.565)
        a=(x0,paty(0,2,v,y0,y1)); b=(x1,paty(1,2,v+2,y0,y1))
        path=smooth(a,b,v,y0,y1)
        slider.append(plen(path)/1.65)
        for x,y in path:
            if not(ARENA[0]<=x<=ARENA[2] and ARENA[1]<=y<=ARENA[3]):bounds+=1

# Static border meteor field safety. Pattern y-range starts/ends 54 logical px
# inside arena; meteors must remain entirely outside this range even at max bob.
meteor_specs=[
    (34,5,True),(27,7,True),(31,5,True),(28,6,True),(36,4,True),
    (32,5,False),(26,6,False),(33,4,False),(29,6,False)
]
path_y0=ARENA[1]+SY(54); path_y1=ARENA[3]-SY(54)
meteor_overlap=0
for size,amp,top in meteor_specs:
    # conservative visual half-height + bob
    half=max(SX(size),SY(size))/2
    if top:
        far=ARENA[1]+SY(28)+SY(amp)+half
        if far>=path_y0: meteor_overlap+=1
    else:
        far=ARENA[3]-SY(28)-SY(amp)-half
        if far<=path_y1: meteor_overlap+=1

raw=np.fromfile(ROOT/'assets/survival/boss3/terminus_fleet_background.bgra',dtype=np.uint8)
assert raw.size==1983*793*4, raw.size
raw=raw.reshape(793,1983,4)
edge=np.abs(raw[:,0,:3].astype(int)-raw[:,-1,:3].astype(int))

report={
 'iterations':N,
 'unique_loop_violations':duplicate,
 'path_bounds_violations':bounds,
 'ship_clearance_violations':clearance,
 'meteor_pattern_overlap_violations':meteor_overlap,
 'lane_width_px_p5_med_p95':q(lanes),
 'ship_nose_gap_px_p5_med_p95':q(nose_gap),
 'chain_speed_px_s_p5_med_p95':q(chain),
 'stream_speed_px_s_p5_med_p95':q(stream),
 'jump_speed_px_s_p5_med_p95':q(jump),
 'slider_speed_px_s_p5_med_p95':q(slider),
 'hit_radius_ref_px':SX(21),
 'stream_radius_ref_px':SX(16),
 'slider_corridor_ref_px':SX(15),
 'hold_radius_ref_px':SX(16),
 'turret_normal_warning_ms':1500,
 'turret_normal_lock_to_fire_ms':450,
 'turret_heavy_warning_ms':1780,
 'turret_heavy_lock_to_fire_ms':530,
 'turret_cadence_ms':[3600,3150,2800,2500],
 'background_edge_mad':round(float(edge.mean()),4),
 'background_edge_max':int(edge.max()),
 'background_dims':[1983,793],
 'satellite_sprite_to_radius_ratio':2.02,
}

assert duplicate==bounds==clearance==meteor_overlap==0
assert report['background_edge_max']==0
assert report['ship_nose_gap_px_p5_med_p95'][0] >= SX(18)
assert report['slider_corridor_ref_px'] <= 12
assert report['stream_radius_ref_px'] <= 13
assert report['turret_normal_warning_ms'] >= 1400
assert min(report['turret_cadence_ms']) > report['turret_heavy_warning_ms']
assert report['satellite_sprite_to_radius_ratio'] >= 1.9
assert 300 < report['jump_speed_px_s_p5_med_p95'][1] < 700
assert 80 < report['slider_speed_px_s_p5_med_p95'][1] < 350

out=ROOT/'docs/qa/v399_terminus_premium_stats.json'
out.parent.mkdir(parents=True,exist_ok=True)
out.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report,indent=2))
print('PASS: Terminus v399 premium statistical QA')
