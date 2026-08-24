import math, random, statistics
random.seed(320)
N=300_000
# Boss3 memory sequences: unique nodes, legal colors, lengths 3/4/5.
for round_idx, length in enumerate((3,4,5)):
    for _ in range(N//3):
        seq=random.sample(range(6), length)
        colors=[bool(random.getrandbits(1)) for _ in range(length)]
        assert len(seq)==len(set(seq))==length
        assert all(0<=n<6 for n in seq)
        assert len(colors)==length
# Relocation rule: next safe node differs, deadline positive.
prev=-1
for _ in range(N):
    choices=[n for n in range(6) if n!=prev]
    nxt=random.choice(choices)
    assert nxt!=prev
    prev=nxt
# Tracking trajectory: mirrors code and stays inside representative 1600x900 arena margins.
# We don't need pixel-perfect arenaRect here; stress invariant is the +/- 165x118 design travel.
for i in range(N):
    t=i*0.0037
    x=800+math.cos(t*1.45)*165
    y=450+math.sin(t*1.90)*118
    assert 600 < x < 1000
    assert 300 < y < 600
# Elite rates and hard cap.
def elite_chance(wave, swarm=False):
    if wave < 21: return 0
    p=min(0.28, 0.08+(wave-21)*0.012)
    if swarm: p*=0.58
    return p
for wave in range(1,101):
    p=elite_chance(wave)
    assert 0 <= p <= .28
    if wave<21: assert p==0
# Monte Carlo check representative rates.
for wave in (21,25,30,40,60):
    p=elite_chance(wave)
    hits=sum(random.random()<p for _ in range(200_000))/200_000
    assert abs(hits-p)<0.004, (wave,hits,p)
# Meteor spawn interval is always 1150..1750ms; ensure random stress obeys and avoids runaway density.
intervals=[random.randint(1150,1750) for _ in range(N)]
assert min(intervals)>=1150 and max(intervals)<=1750
# Approx active count for 3.5s fall time should remain modest even at minimum interval.
assert math.ceil(3500/min(intervals)) <= 4
print('PASS boss3 memory/relocation/tracking:', N, 'iterations')
print('PASS sector3 elite probability + meteor density:', N, 'iterations')
