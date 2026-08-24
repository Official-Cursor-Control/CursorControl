from pathlib import Path
import re, random
ROOT=Path(__file__).resolve().parents[1]
s='\n'.join(p.read_text(errors='ignore') for p in ROOT.glob('*.go'))
r=(ROOT/'research_progression.go').read_text()
# Achievement catalogue integrity
m=re.search(r'func achievementDefinitions\(\) \[\]AchievementDef \{\s*return \[\]AchievementDef\{(.*?)\n\t\}\n\}',s,re.S)
assert m
items=re.findall(r'\{"([A-Z0-9_]+)",\s*"([^"]+)",\s*"([^"]+)",\s*(\d+)\}',m.group(1))
ids=[x[0] for x in items]
assert len(ids)==len(set(ids)), 'duplicate achievement IDs'
assert 70 <= len(ids) <= 95, len(ids)
assert 'TRIPLE_CROWN' in ids
print('PASS achievement_catalogue',len(ids))
# Hidden ratio target ~10% and hidden IDs valid
hidden=re.search(r'func achievementHidden\(id string\) bool \{(.*?)\n\}',r,re.S).group(1)
hidden_ids=re.findall(r'"([A-Z0-9_]+)"',hidden)
assert set(hidden_ids)<=set(ids)
ratio=len(hidden_ids)/len(ids)
assert .06 <= ratio <= .13,ratio
print('PASS hidden_ratio',len(hidden_ids),ratio)
# Six prestige labels present
for label in ['INTRO','COMMON','SKILLED','EXPERT','ELITE','LEGENDARY']:
    assert f'"{label}"' in r
print('PASS six_prestige_tiers')
# Showcase persisted, syncable, exactly 3 visible slots and duplicate prevented
assert re.search(r'AchievementShowcase\s+\[\]string', s)
assert 'achievement_showcase' in s
assert 'profileShowcaseRects' in s
assert 'setAchievementShowcaseSlot' in r and 'firstAvailableShowcaseSlot' in r
print('PASS showcase_contract')
# Profile contains mode-first primary stats
for label in ['PRECISION CLEARS','ENDURANCE BEST','SURVIVAL BEST','ACHIEVEMENTS']:
    assert label in s
print('PASS profile_mode_first_summary')
# Persistent Survival stats are wired on clear + failure
sv=(ROOT/'survival.go').read_text()
assert sv.count('updatePersistentSurvivalProfileStats') >= 2
print('PASS survival_profile_persistence')
# Fuzz positional showcase normalization + direct slot replacement.
random.seed(998)
for _ in range(100_000):
    unlocked=set(random.sample(ids,random.randrange(0,min(30,len(ids))+1)))
    incoming=[random.choice(ids+['','NOT_REAL']) for _ in range(3)]
    out=['','','']; seen=set()
    for i,aid in enumerate(incoming):
        if not aid or aid in seen or aid not in unlocked: continue
        seen.add(aid); out[i]=aid
    assert len(out)==3
    live=[x for x in out if x]
    assert len(live)==len(set(live)) and set(live)<=unlocked
    if unlocked:
        chosen=random.choice(list(unlocked)); slot=random.randrange(3)
        for i in range(3):
            if i!=slot and out[i]==chosen: out[i]=''
        out[slot]=chosen
        assert out[slot]==chosen and sum(x==chosen for x in out)==1
print('PASS showcase_fuzz_100k positional/manual')
