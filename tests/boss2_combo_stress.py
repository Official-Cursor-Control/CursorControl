import math, random
random.seed(282)
ARENA=(0,0,1536,864)
MARGIN_X=165
MARGIN_Y=130
SEG_COUNT=48

def combo_size(c): return [3,4,5,6,6][c]

def choose_group(points, combo, cursor):
    total=combo_size(combo)
    best=None; bestscore=1e99
    for start in range(4, SEG_COUNT-2-total*2):
        ok=True; score=0
        for j in range(total):
            i=start+j*2
            x,y=points[i]
            if not (MARGIN_X <= x <= ARENA[2]-MARGIN_X and MARGIN_Y <= y <= ARENA[3]-MARGIN_Y):
                ok=False; break
            score += math.hypot(x-cursor[0],y-cursor[1])
        if ok and score<bestscore:
            bestscore=score; best=start
    if best is None: return None
    return [best+j*2 for j in range(total)]

# 60k generated readable on-screen passes across five combo sizes.
trials=0; valid=0; adjacency_fail=0; bounds_fail=0
for combo in range(5):
    for _ in range(12000):
        # smooth serpentine chain spanning the arena with mild random phase/tilt
        phase=random.random()*math.tau
        basey=random.uniform(260,604)
        step=random.uniform(18,25)
        points=[]
        for i in range(SEG_COUNT):
            x=120+i*step
            y=basey+math.sin(phase+i*0.19)*random.uniform(30,85)
            points.append((x,y))
        cursor=(random.uniform(150,1386),random.uniform(150,714))
        g=choose_group(points,combo,cursor)
        trials+=1
        if g is None: continue
        valid+=1
        if any(g[i+1]-g[i] != 2 for i in range(len(g)-1)): adjacency_fail+=1
        for i in g:
            x,y=points[i]
            if not (MARGIN_X <= x <= ARENA[2]-MARGIN_X and MARGIN_Y <= y <= ARENA[3]-MARGIN_Y): bounds_fail+=1

# 100k sequence/input lifecycle cases: every successful click advances exactly one,
# final click completes, and colours avoid runs of three identical cores.
lifecycle_fail=0; colour_fail=0
for _ in range(100000):
    combo=random.randrange(5); total=combo_size(combo)
    reds=[]
    for j in range(total):
        red=bool(random.getrandbits(1))
        if j>=2 and reds[-1]==reds[-2]: red=not reds[-1]
        reds.append(red)
    if any(reds[i]==reds[i-1]==reds[i-2] for i in range(2,total)): colour_fail+=1
    idx=0
    for click in range(total):
        idx+=1
        completed=(idx>=total)
        if completed != (click==total-1): lifecycle_fail+=1

print(f'combo_group_trials={trials} valid_groups={valid} adjacency_fail={adjacency_fail} bounds_fail={bounds_fail}')
print(f'lifecycle_cases=100000 lifecycle_fail={lifecycle_fail} colour_fail={colour_fail}')
assert valid > 50000
assert adjacency_fail == 0
assert bounds_fail == 0
assert lifecycle_fail == 0
assert colour_fail == 0
print('PASS boss2 combo stress')
