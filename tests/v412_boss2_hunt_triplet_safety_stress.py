#!/usr/bin/env python3
import random
random.seed(412)
SEG_COUNT = 48
TOTAL = 5

def safe_for_node(seg, host, alive, restore_until, now):
    return (alive or now < restore_until) and abs(seg-host) <= 1

# Direct contract: host 16 opens 15/16/17 only.
for now, alive, restore in [(10.0, True, 0.0), (11.999, False, 12.0)]:
    got = [i for i in range(SEG_COUNT) if safe_for_node(i, 16, alive, restore, now)]
    assert got == [15,16,17], got
assert not safe_for_node(15, 16, False, 12.0, 12.0)
assert not safe_for_node(16, 16, False, 12.0, 12.0)
assert not safe_for_node(17, 16, False, 12.0, 12.0)

# Stress arbitrary legal hosts and exact two-second restoration of all three pieces.
for _ in range(200000):
    host = random.randint(2, SEG_COUNT-3)
    hit = random.uniform(0, 1000)
    for dt in (0.0, 0.4, 1.2, 1.999):
        now = hit + dt
        safe = {i for i in range(SEG_COUNT) if safe_for_node(i, host, False, hit+2.0, now)}
        assert safe == {host-1, host, host+1}
    now = hit + 2.0
    assert not any(safe_for_node(i, host, False, hit+2.0, now) for i in range(SEG_COUNT))

# Five nodes may coexist; safety is the union of each node's three-piece pocket.
for _ in range(100000):
    hosts = random.sample(range(2, SEG_COUNT-2), TOTAL)
    now = 50.0
    expected = set()
    for host in hosts:
        expected.update((host-1, host, host+1))
    got = {i for i in range(SEG_COUNT) if any(safe_for_node(i,h,True,0.0,now) for h in hosts)}
    assert got == expected

print('PASS v412 boss2 Hunt three-piece safety stress')
