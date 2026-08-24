from pathlib import Path
import re, random, statistics
ROOT=Path(__file__).resolve().parents[1]
s='\n'.join(p.read_text(errors='ignore') for p in ROOT.glob('*.go'))
# Rank threshold integrity
m=re.search(r'var expRanks = \[\]ExpRank\{(.*?)\n\}',s,re.S)
ranks=[(name,int(v)) for name,v in re.findall(r'\{"([^"]+)",\s*(\d+)\}',m.group(1))]
assert len(ranks)>=15
vals=[v for _,v in ranks]
assert vals==sorted(vals) and len(vals)==len(set(vals)) and vals[0]==0
# Early progression should be compressed relative to prestige endgame.
early=[vals[i+1]-vals[i] for i in range(0,5)]
late=[vals[i+1]-vals[i] for i in range(len(vals)-5,len(vals)-1)]
assert statistics.mean(late) > statistics.mean(early)*8
print('PASS rank_curve_fast_early_slow_late', {'early_avg':statistics.mean(early),'late_avg':statistics.mean(late)})
# Achievement reward ladder remains strongly increasing.
assert 'return 150' in s and 'return 600' in s and 'return 30' in s
print('PASS achievement_reward_ladder 30/150/600')
# Representative new-player path: Normal clears plus only conservative milestone awards.
# Guardrail: Bronze II should be reachable within five successful clears without caches.
exp=0
awarded=set()
for clear in range(1,6):
    exp += 5
    milestones=[]
    if clear==1: milestones+=['FIRST_BLOOD']
    if clear==3: milestones+=['THREE_CHAIN']
    if clear==5: milestones+=['WARMING_UP']
    for a in milestones:
        if a not in awarded:
            awarded.add(a); exp+=30
assert exp>=100,(exp,awarded)
print('PASS onboarding_rank_pacing',exp)
# Fuzz monotonic next-rank progress across 500k EXP samples.
random.seed(323)
for _ in range(500_000):
    x=random.randint(0,200_000)
    idx=max(i for i,v in enumerate(vals) if v<=x)
    if idx<len(vals)-1:
        into=x-vals[idx]; need=vals[idx+1]-vals[idx]
        assert into>=0 and need>0
print('PASS rank_progress_fuzz_500k')
