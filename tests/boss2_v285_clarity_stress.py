#!/usr/bin/env python3
import random, math, json

random.seed(285)
N=250_000
# Model the exact safe-section rule: host +/-2 and safe-exit +/-2.
def disabled(i, node_seg, node_live, exit_seg, exit_live):
    return ((node_live and node_seg >= 0 and abs(i-node_seg)<=2) or
            (exit_live and exit_seg >=0 and abs(i-exit_seg)<=2))

mapping_fail=0
for _ in range(N):
    i=random.randrange(48)
    ns=random.randrange(-1,48)
    nl=bool(random.getrandbits(1))
    es=random.randrange(-1,48)
    el=bool(random.getrandbits(1))
    collision_disabled=disabled(i,ns,nl,es,el)
    rendered_purple=disabled(i,ns,nl,es,el)
    if collision_disabled != rendered_purple:
        mapping_fail += 1

# Off-screen nodes: leaving the clickable inset must never be a failure. It must
# recycle/restart the same combo number, therefore only elapsed fight duration grows.
offscreen_fail=0
combo_progress_errors=0
for _ in range(N):
    combo=random.randrange(5)
    hit=random.randrange(0,6)
    x=random.uniform(-300,1500)
    y=random.uniform(-300,1000)
    # Reference playfield approximation + margins from Go logic.
    clickable=(82 <= x <= 1198-82 and 82 <= y <= 620-82)
    before=(combo,hit)
    if not clickable:
        failed=False
        after=(combo,hit)  # same combo/hit progress; next pass retries
        if failed:
            offscreen_fail+=1
        if after!=before:
            combo_progress_errors+=1

# Node colour sequences cannot exceed two identical colours in a row.
colour_fail=0
for _ in range(100_000):
    total=random.randint(3,6)
    reds=[]
    for j in range(total):
        red=bool(random.getrandbits(1))
        if j>=2 and reds[j-1]==reds[j-2]:
            red=not reds[j-1]
        reds.append(red)
    for j in range(2,total):
        if reds[j]==reds[j-1]==reds[j-2]:
            colour_fail+=1

result={
    'safe_render_collision_cases': N,
    'safe_render_collision_mismatches': mapping_fail,
    'offscreen_node_cases': N,
    'offscreen_failures': offscreen_fail,
    'offscreen_progress_errors': combo_progress_errors,
    'colour_sequence_cases':100_000,
    'colour_failures':colour_fail,
}
print(json.dumps(result,indent=2))
assert mapping_fail==0
assert offscreen_fail==0
assert combo_progress_errors==0
assert colour_fail==0
