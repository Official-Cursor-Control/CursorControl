#!/usr/bin/env python3
import random
random.seed(415)
SEG_COUNT=35
TOTAL=5
FIRST=2
LAST_EXCL=SEG_COUNT-2
USABLE=LAST_EXCL-FIRST

def pick_once():
    for _ in range(256):
        chosen=[]
        phase=random.randrange(max(1,USABLE//TOTAL))
        for band in range(TOTAL):
            lo=FIRST+band*USABLE//TOTAL
            hi=FIRST+(band+1)*USABLE//TOTAL
            if band>0: lo=max(FIRST,lo-1)
            if band<TOTAL-1: hi=min(LAST_EXCL,hi+1)
            if hi<=lo: continue
            perm=list(range(hi-lo)); random.shuffle(perm)
            p=-1
            for off in perm:
                c=lo+((off+phase)%(hi-lo))
                if all(abs(c-prev)>=4 for prev in chosen):
                    p=c; break
            if p>=0: chosen.append(p)
        if len(chosen)!=TOTAL: continue
        chosen.sort()
        if chosen[-1]-chosen[0] < 21: continue
        return chosen
    return None

seen=set()
for _ in range(500000):
    x=pick_once(); assert x is not None
    assert len(x)==5
    assert all(FIRST<=v<LAST_EXCL for v in x)
    assert all(abs(a-b)>=4 for i,a in enumerate(x) for b in x[i+1:])
    assert x[-1]-x[0]>=21
    for v in x:
        seen.add(v)
        assert v-1>=0 and v+1<SEG_COUNT
# Neck segments 0/1 can never host, eliminating the behind-head click bug.
assert 0 not in seen and 1 not in seen
# Ensure broad usable-body coverage remains random.
assert len(seen) >= 26, len(seen)
print('PASS v415 35-piece serpent Hunt stress', 'host_positions_seen=', len(seen), 'range=', (min(seen),max(seen)))
