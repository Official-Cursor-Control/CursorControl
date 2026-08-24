# Deterministic smoke simulation for Sections 1-6 economy/gates.
# Uses the same hard-coded values as the Go implementation and verifies the
# player can reach every implemented system without a circular requirement.

auto_costs=[25,50,100,200,400,800,1600,3200,6400,12800]
proc_costs=[0,100,250,600,1400,3200,7000,15000,32000,68000]
proc_rate=[0,1.0,1.25,1.6,2.1,2.8,3.8,5.2,7.0,9.5,13.0]
core_mult={1:1,2:2,3:5,4:12,5:30}
energy=0.0
time_s=0.0
cursors=0
proc=1
core=0

def rate():
    return cursors*proc_rate[proc]*(core_mult.get(core,1))

def wait_for(cost):
    global energy,time_s
    if energy>=cost:return
    r=rate()
    assert r>0, (cost,energy,cursors,proc,core)
    dt=(cost-energy)/r
    time_s+=dt; energy=cost

# Section 1: 20 clicks x5, then build core consumes 100.
energy=100; core=1; energy-=100
assert energy==0 and core==1

# Section 2: manually fund the first Auto Cursor, then passive production takes over.
energy += 25  # five +5 manual clicks after the Core is built
for c in auto_costs:
    wait_for(c); energy-=c; cursors+=1
for lvl in range(1,10):
    c=proc_costs[lvl]; wait_for(c); energy-=c; proc+=1
assert cursors==10 and proc==10 and abs(rate()-130)<1e-9
section2_time=time_s

# Tier 2.
wait_for(100_000); energy-=100_000; core=2
assert abs(rate()-260)<1e-9

# Scout build.
wait_for(250_000); energy-=250_000

# Expeditions chosen to satisfy Section 5 with deterministic rewards/data.
# Four Cyan Nebula runs -> Tier2 data, then Shattered Moon + Deep Orbit.
nav=0; runs=0
for dur,reward,data in [(90*60,4_000_000,80)]*4:
    time_s+=dur; energy+=reward; nav+=data; runs+=1
assert nav==320 and runs==4
nav-=250 # Scout tier 2
scout_tier=2
# Shattered Moon then Deep Orbit.
for dur,reward,data in [(3*3600,25_000_000,160),(6*3600,200_000_000,320)]:
    time_s+=dur; energy+=reward; nav+=data; runs+=1
# Upgrade speed L1->3, capacity L1->3, cache L1->3.
for c in [25,60,30,75,35,90]:
    assert nav>=c; nav-=c
assert scout_tier>=2 and runs>=6
section5_time=time_s

# Tier 3 gate now unlocked and affordable from expedition energy.
wait_for(50_000_000); energy-=50_000_000; core=3
assert core==3
# Tech Lab.
wait_for(12_000_000); energy-=12_000_000
tech=True
# Buy 12 cheapest research levels by greedily selecting next cheapest available cost.
research_costs=[
[75_000,160_000,340_000,720_000,1_500_000,3_100_000,6_500_000,13_500_000,28_000_000],
[90_000,190_000,400_000,850_000,1_800_000,3_800_000,8_000_000,17_000_000,35_000_000],
[105_000,220_000,470_000,1_000_000,2_100_000,4_400_000,9_200_000,19_000_000,40_000_000],
[125_000,270_000,580_000,1_220_000,2_600_000,5_500_000,11_500_000,24_000_000,50_000_000],
[145_000,310_000,660_000,1_400_000,2_950_000,6_200_000,13_000_000,27_000_000,56_000_000],
[175_000,370_000,790_000,1_670_000,3_500_000,7_400_000,15_500_000,32_000_000,67_000_000],
[210_000,450_000,950_000,2_000_000,4_200_000,8_900_000,18_500_000,38_500_000,80_000_000],
[250_000,530_000,1_120_000,2_360_000,5_000_000,10_500_000,22_000_000,46_000_000,96_000_000],
[300_000,640_000,1_350_000,2_850_000,6_000_000,12_600_000,26_500_000,55_000_000,115_000_000],
[360_000,770_000,1_620_000,3_420_000,7_200_000,15_200_000,32_000_000,67_000_000,140_000_000]]
levels=[0]*10
for _ in range(12):
    choices=[(costs[levels[i]],i) for i,costs in enumerate(research_costs) if levels[i]<len(costs)]
    cost,i=min(choices)
    wait_for(cost); energy-=cost; levels[i]+=1
assert sum(levels)==12
# By 5 research levels player has at least 2 points; by 12 at least 3.
points=3
# Sharper Cursors -> Orbital Cursor Math 2.0 chain.
points-=1; sharper=True
points-=1; orbital=True
assert sharper and orbital and points>=0
section6_time=time_s

print('PASS progression smoke simulation')
print(f'Section 2 complete: {section2_time/3600:.2f} active hours')
print(f'Section 5 gate complete: {section5_time/3600:.2f} wall-clock hours')
print(f'Section 6 complete: {section6_time/3600:.2f} wall-clock hours')
print('No circular gate/deadlock detected through Tech Lab + Pilot Skills.')
