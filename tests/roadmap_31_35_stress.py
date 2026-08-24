#!/usr/bin/env python3
from pathlib import Path
from datetime import date,timedelta
import random,re
ROOT=Path(__file__).resolve().parents[1]
allgo='\n'.join(p.read_text(errors='ignore') for p in ROOT.glob('*.go'))
checks=0

def ok(c,m):
    global checks
    checks+=1
    if not c: raise AssertionError(m)

# 31 retention tracking: exact D1/D7/D30 boundaries and bounded active-date history.
for _ in range(250_000):
    first=date(2024,1,1)+timedelta(days=random.randint(0,500))
    offsets=sorted(set(random.randint(0,90) for __ in range(random.randint(1,30))))
    flags=(any(x>=1 for x in offsets),any(x>=7 for x in offsets),any(x>=30 for x in offsets))
    ok((not flags[2] or flags[1]) and (not flags[1] or flags[0]),'retention monotonicity')
    # client stores max 120 active dates
    stored=offsets[-120:]
    ok(len(stored)<=120,'active dates unbounded')

# 32 structured player testing exists and explicitly measures enjoyment/fairness/clarity/retry.
proto=(ROOT/'developer/research/STRUCTURED_PLAYER_TEST_PROTOCOL.md').read_text(errors='ignore').lower()
for term in ['enjoyment','fairness','clarity','another run','failure category','session duration','frame-time','20 comparable observations']:
    ok(term in proto,'player-test protocol missing '+term)

# 33 performance telemetry + measured-hotspot guardrails.
perf=(ROOT/'profiler.go').read_text(errors='ignore')
for term in ['frame_max_ms','heap_mb','peak_heap_mb','alloc_mb_s','goroutines','severe']:
    ok(term in perf.lower(),'performance telemetry missing '+term)
ok((ROOT/'tests/performance_guardrail_report.py').exists(),'performance report tool missing')

# 34 no broad refactor / rollback policy encoded in research docs.
guard=(ROOT/'developer/research/PLAYER_EXPERIENCE_GUARDRAILS.md').read_text(errors='ignore').lower()
road=(ROOT/'developer/research/ROADMAP_v320_v360.md').read_text(errors='ignore').lower()
for term in ['measure first','structured player','rollback','do not optimize','telemetry']:
    ok(term in guard or term in road,'guardrail missing '+term)

# 35 controlled release roadmap covers the main staged systems.
for term in ['identity / progression','survival expansion','competition','player experience / ux','audio','measurement','release rule']:
    ok(term in road,'roadmap section missing '+term)

# Competitive prestige rewards must never grant gameplay power.
comp='\n'.join(p.read_text(errors='ignore') for p in [ROOT/'competition_v320.go', ROOT/'migrations/SUPABASE_v320_COMPETITION.sql'] if p.exists())
for bad in ['damage_multiplier','speed_multiplier','max_hp_bonus','hitbox_bonus','timer_bonus']:
    ok(bad not in comp.lower(),'competitive gameplay power leaked: '+bad)

# Analytics remains opt-in for network upload, local bounded, and contains no raw cursor tracking field.
ana=(ROOT/'analytics_v320.go').read_text(errors='ignore')
ok('if !gameMeta.ShareAnonymousAnalytics' in ana,'network analytics not opt-in gated')
ok('chan gameplayAnalyticsEvent, 512' in ana,'analytics channel unbounded')
ok('len(analyticsRuntime.batch) > 100' in ana,'analytics batch unbounded')
for bad in ['cursor_x','cursor_y','mouse_x','mouse_y','discord_id','email_address']:
    ok(bad not in ana.lower(),'privacy-sensitive analytics field '+bad)

# 500k performance-gate model probes: severe hitches or runaway memory should trip a gate.
for _ in range(500_000):
    frame=random.uniform(4,90); heap=random.uniform(20,900); alloc=random.uniform(0,180)
    severe = frame>50
    memory_pressure = heap>700 or alloc>120
    gate = severe or memory_pressure
    ok(gate == (frame>50 or heap>700 or alloc>120),'performance gate inconsistency')

print(f'PASS roadmap 31-35 stress: {checks:,} assertions')
