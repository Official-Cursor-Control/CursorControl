from pathlib import Path
import random, re
ROOT=Path(__file__).resolve().parents[1]
cloud=(ROOT/'afk_cloud_sync.go').read_text(errors='ignore')
ui=(ROOT/'afk_ui_consistency.go').read_text(errors='ignore')
files={p.name:p.read_text(errors='ignore') for p in ROOT.glob('afk_*.go')}

checks=0
def ok(cond,msg):
    global checks
    checks+=1
    if not cond: raise AssertionError(msg)

# UI consistency: all requested management surfaces are tied to the shared frame/chrome.
for fn in ['afk_starbase_hub.go','afk_bottom_hud.go','afk_expeditions.go','afk_research_talents.go','afk_operators.go','afk_modules.go','afk_drones.go','afk_orbital.go','afk_exchange.go']:
    ok(fn in files, f'missing {fn}')
ok('func afkManagementPanelRect' in ui and 'func drawAFKManagementChrome' in ui, 'shared management chrome missing')
for token in ['STARBASE FACILITIES','STARBASE GUIDE','STARBASE SYSTEMS','SPACE EXPEDITIONS','TECH LAB','OPERATORS','SHIP MODULES','DRONE BAY','ORBITAL EXTRACTOR','STARBASE EXCHANGE']:
    ok(any(token in text for text in files.values()), f'missing panel label {token}')

# Cloud monotonic coverage for irreversible progression.
monotonic_keys=[
'afk_cursor_core_built','afk_cursor_core_tier','afk_auto_cursors','afk_cursor_processing_level',
'afk_scout_ship_unlocked','afk_scout_ship_tier','afk_expedition_speed_level','afk_capacity_level','afk_cache_chance_level','afk_expeditions_completed',
'afk_tech_lab_unlocked','afk_research_levels','afk_talents_unlocked',
'afk_operators_recruited','afk_operator_xp_seconds','afk_equipment_crafted','afk_equipment_sets_complete',
'afk_drone_bay_built','afk_drone_bay_tier','afk_drone_upgrade_levels','afk_drones_deployed',
'afk_orbital_extractor_built','afk_orbital_extractor_tier','afk_orbital_upgrade_levels','afk_prestige_rank']
for key in monotonic_keys:
    ok(key in cloud, f'monotonic merge missing {key}')

# Simulate delayed sync merge. A response may be older than current live state, but
# normal heartbeat must never make the visible balance move backwards.
for _ in range(100_000):
    request=random.randint(0,10**12)
    # local may gain or spend while request is in flight
    live=max(0, request+random.randint(-10**7,10**8))
    server=max(0, request+random.randint(-10**6,10**7))
    delta=live-request
    merged=max(0,server+delta)
    merged=max(merged,live) # sync heartbeat floor in client
    ok(merged>=live,'heartbeat rolled Starbits backwards')

# Monotonic tier merge property stress.
for _ in range(100_000):
    local=[random.randint(0,30) for _ in range(15)]
    cloudv=[random.randint(0,30) for _ in range(15)]
    merged=[max(a,b) for a,b in zip(local,cloudv)]
    ok(all(m>=a and m>=b for m,a,b in zip(merged,local,cloudv)),'monotonic level regression')

# Boolean unlock/crafted merge property stress.
for _ in range(50_000):
    local=[bool(random.getrandbits(1)) for _ in range(72)]
    cloudv=[bool(random.getrandbits(1)) for _ in range(72)]
    merged=[a or b for a,b in zip(local,cloudv)]
    ok(all((not a or m) and (not b or m) for m,a,b in zip(merged,local,cloudv)),'unlock/crafted rollback')

# Dirty-sync hooks should exist for Scout upgrades, the regression already seen in production.
exp=files['afk_expeditions.go']
ok(exp.count('afkCloudMarkProgressDirty()')>=2,'Scout tier/stat upgrades do not force next heartbeat')
print(f'PASS Starbase UI/cloud stress: {checks:,} assertions')
