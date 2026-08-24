import random, math, statistics

BASE=[68.0,24.0,7.0,1.0]

def chances(red,gold):
    c=BASE[:]
    if red>=24:
        c=[0.0,0.0,99.0,1.0]
    elif red>=15:
        boost=(red-14)*2.1
        c[2]+=boost
        c[0]-=boost*.72
        c[1]-=boost*.28
    if gold>=99:
        return [0,0,0,100]
    if gold>=60:
        gb=min(9.0,(gold-59)*.23)
        c[3]+=gb
        for tier,share in enumerate([.70,.23,.07]):
            c[tier]=max(0,c[tier]-gb*share)
    total=sum(c)
    return [x*100/total for x in c]

def roll(red,gold):
    c=chances(red,gold); r=random.random()*100; a=0
    for t,v in enumerate(c):
        a+=v
        if r<a:return t
    return 3

# Base-distribution test far from pity.
N=1_000_000
counts=[0]*4
random.seed(320)
for _ in range(N):
    t=roll(0,0);counts[t]+=1
obs=[x/N*100 for x in counts]
for got,want,tol in zip(obs,BASE,[.25,.22,.10,.06]):
    assert abs(got-want)<tol,(obs,BASE)
print('PASS base_distribution',obs)

# 25-roll Red+ hard ceiling across 100k independent sequences.
random.seed(321)
max_red_gap=0; max_gold_gap=0
red=gold=0
for _ in range(2_000_000):
    t=roll(red,gold)
    if t>=2:
        max_red_gap=max(max_red_gap,red+1); red=0
    else:red+=1
    if t==3:
        max_gold_gap=max(max_gold_gap,gold+1); gold=0
    else:gold+=1
    assert red<=24,red
    assert gold<=99,gold
print('PASS pity_ceilings',max_red_gap,max_gold_gap)

# Verify probability vector is always sane over thousands of pity states.
for red in range(0,130):
    for gold in range(0,160):
        c=chances(red,gold)
        assert all(x>=0 for x in c),c
        assert abs(sum(c)-100)<1e-9,c
        if red>=24 and gold<99:
            assert c[0]==0 and c[1]==0
        if gold>=99:
            assert c==[0,0,0,100]
print('PASS probability_invariants')

# Duplicate compensation monotonicity.
comp=[100,175,300,900]
assert comp==sorted(comp) and len(set(comp))==4
print('PASS duplicate_compensation',comp)

# Monte-Carlo long-run ownership timing: collect all 11 unlockable ships.
# This is diagnostic only: ensure no pathological multi-thousand qualifying rolls.
tiers={0:[1,3,4],1:[5,6,7,10],2:[2,8,12],3:[9]}
random.seed(322)
trials=[]
for run in range(5000):
    red=gold=0; owned=set(); rolls=0
    while len(owned)<11 and rolls<2000:
        t=roll(red,gold); ship=random.choice(tiers[t]); rolls+=1
        owned.add(ship)
        if t>=2:red=0
        else:red+=1
        if t==3:gold=0
        else:gold+=1
    assert len(owned)==11
    trials.append(rolls)
print('PASS collection_trials', {'median':statistics.median(trials),'p90':sorted(trials)[4499],'p99':sorted(trials)[4949],'max':max(trials)})
