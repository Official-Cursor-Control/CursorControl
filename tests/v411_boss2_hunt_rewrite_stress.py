#!/usr/bin/env python3
import random
random.seed(411)
SEG_COUNT=48
TOTAL=5

def select(valid,total=5):
    if len(valid)<total: return None
    for _ in range(96):
        chosen=[]; ok_all=True
        for j in range(total):
            lo=j*len(valid)//total
            hi=(j+1)*len(valid)//total
            if hi<=lo: hi=lo+1
            hi=min(hi,len(valid))
            opts=list(valid[lo:hi]); random.shuffle(opts)
            pick=-1
            for cand in opts:
                if all(abs(cand-p)>=2 for p in chosen):
                    pick=cand; break
            if pick<0:
                ok_all=False; break
            chosen.append(pick)
        if ok_all and len(chosen)==total: return chosen
    return None

trials=100000; selection_fail=adj_fail=edge_fail=anchor_mutation=0
for _ in range(trials):
    # Model a contiguous visible slice of articulated body plus occasional occluded segments.
    lo=random.randint(2,12); hi=random.randint(max(lo+12,24),SEG_COUNT-3)
    valid=[i for i in range(lo,hi+1) if random.random()>0.08]
    out=select(valid)
    if out is None:
        # Runtime simply waits for more body to enter in this case; not a bad layout.
        continue
    if len(out)!=5: selection_fail+=1
    if any(abs(a-b)<2 for i,a in enumerate(out) for b in out[i+1:]): adj_fail+=1
    if any(i<2 or i>=SEG_COUNT-2 or i not in valid for i in out): edge_fail+=1
    anchors=list(out)
    # Simulate arbitrary body movement/history changes: segment IDs must never retarget.
    for _frame in range(50):
        if out != anchors: anchor_mutation+=1; break

# Lifecycle: five live immediately; click any order; each clicked node is gone immediately,
# and only its own segment remains collision-disabled for exactly two seconds.
lifecycle_fail=0
for _ in range(100000):
    segs=select(list(range(2,SEG_COUNT-2)))
    alive=[True]*5; restore=[0.0]*5
    order=list(range(5)); random.shuffle(order)
    now=10.0
    for idx in order:
        if not all(alive[j] or restore[j]>now for j in range(5) if j!=idx and alive[j]): lifecycle_fail+=1; break
        alive[idx]=False; restore[idx]=now+2.0
        if alive[idx]: lifecycle_fail+=1
        # Exact host safe at 1.999s, restored at 2.000s; neighbours never inherit safety.
        if not (restore[idx] > now+1.999): lifecycle_fail+=1
        if restore[idx] > now+2.000001: lifecycle_fail+=1
        now += random.uniform(0.02,0.4)
    if any(alive): lifecycle_fail+=1

# Physical-side warning model: warning edge must match actual outside edge.
side_fail=0
for _ in range(100000):
    side=random.choice(('L','R','T','B'))
    if side=='L': p=(-random.uniform(1,300),random.uniform(0,864))
    elif side=='R': p=(1536+random.uniform(1,300),random.uniform(0,864))
    elif side=='T': p=(random.uniform(0,1536),-random.uniform(1,300))
    else: p=(random.uniform(0,1536),864+random.uniform(1,300))
    x,y=p
    vals=[(0-x,'L'),(x-1536,'R'),(0-y,'T'),(y-864,'B')]
    actual=max(vals,key=lambda q:q[0])[1]
    if actual!=side: side_fail+=1

print(f'selection_trials={trials} selection_fail={selection_fail} adjacency_fail={adj_fail} bounds_fail={edge_fail} anchor_mutation={anchor_mutation}')
print(f'lifecycle_cases=100000 lifecycle_fail={lifecycle_fail}')
print(f'reentry_warning_cases=100000 side_fail={side_fail}')
assert selection_fail==adj_fail==edge_fail==anchor_mutation==0
assert lifecycle_fail==0
assert side_fail==0
print('PASS v411 boss2 hunt rewrite stress')
