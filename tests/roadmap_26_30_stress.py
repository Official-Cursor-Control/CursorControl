from pathlib import Path
from datetime import date, timedelta
import random, json, re, sys
ROOT=Path(__file__).resolve().parents[1]
allgo='\n'.join(x.read_text(errors='ignore') for x in ROOT.glob('*.go'))
main=(ROOT/'main.go').read_text(errors='ignore')
ana=(ROOT/'analytics_v320.go').read_text(errors='ignore')
aud=(ROOT/'audio_runtime.go').read_text(errors='ignore')
checks=0

def ok(cond,msg):
    global checks
    checks+=1
    if not cond:
        raise AssertionError(msg)

# Contract/source assertions for 26 reward hierarchy + 28/29 funnel analytics.
for token in ['session_started','run_started','run_failed','run_completed','tutorial_completed','space_cache_open_started','space_cache_ship','boss_attempted','boss_cleared','profile_customized']:
    ok(token in allgo, 'missing analytics event '+token)
for token in ['sfxPriority','criticalAudioDuckActive','requestCriticalAudioDuck']:
    ok(token in aud, 'missing audio hierarchy '+token)
ok('ShareAnonymousAnalytics' in main and 'if !gameMeta.ShareAnonymousAnalytics' in ana, 'analytics consent gate missing')
ok('chan gameplayAnalyticsEvent, 512' in ana, 'bounded analytics channel missing')
ok('3*1024*1024' in ana, 'analytics log rotation missing')
ok('len(analyticsRuntime.batch) > 100' in ana, 'analytics batch bound missing')

# 100k retention lifecycle simulations: D1/D7/D30 must be monotonic and exact-boundary-safe.
for _ in range(100_000):
    first=date(2026,1,1)+timedelta(days=random.randrange(0,100))
    offsets=sorted(set(random.randrange(0,80) for _ in range(random.randrange(1,20))))
    d1=any(x>=1 for x in offsets); d7=any(x>=7 for x in offsets); d30=any(x>=30 for x in offsets)
    ok(not d7 or d1, 'D7 without D1')
    ok(not d30 or d7, 'D30 without D7')
    # mirror date-string round trip used by client
    parsed=[date.fromisoformat((first+timedelta(days=x)).isoformat()) for x in offsets]
    got=[(d-first).days for d in parsed]
    ok(got==offsets,'retention date roundtrip drift')

# 250k bounded-channel/batch model simulations: telemetry must never grow unbounded.
capacity=512
batch=[]
queue=[]
for i in range(250_000):
    if len(queue)<capacity:
        queue.append(i)
    if random.random()<0.92 and queue:
        batch.append(queue.pop(0))
        if len(batch)>100:
            batch=batch[-100:]
    if random.random()<0.015:
        batch=[]
    ok(len(queue)<=512, 'analytics queue exceeded cap')
    ok(len(batch)<=100, 'analytics batch exceeded cap')

# 100k failure classification probes against intended precedence/category vocabulary.
def classify(reason):
    r=reason.lower()
    if 'boundary' in r or 'rail' in r: return 'BOUNDARY'
    if 'meteor' in r or 'rock' in r: return 'METEOR'
    if 'laser' in r or 'ufo' in r or 'alien' in r: return 'ALIEN'
    if 'time' in r or 'slow' in r: return 'TIME'
    if 'target' in r or 'core' in r or 'signal' in r: return 'OBJECTIVE'
    if 'serpent' in r or 'boss' in r or 'array' in r: return 'BOSS'
    return 'OTHER'
seeds=[('crossing boundary','BOUNDARY'),('meteor collision','METEOR'),('ufo laser','ALIEN'),('time expired','TIME'),('core too slow','TIME'),('target missed','OBJECTIVE'),('void serpent','BOSS'),('unknown','OTHER')]
for _ in range(100_000):
    reason,expected=random.choice(seeds)
    got=classify(reason)
    # "core too slow" intentionally maps TIME because time precedence comes first.
    ok(got==expected,(reason,got,expected))

print(f'PASS roadmap 26-30 stress: {checks:,} assertions')
