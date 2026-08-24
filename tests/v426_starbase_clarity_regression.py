from pathlib import Path
root=Path(__file__).resolve().parents[1]
files={p.name:p.read_text(errors='ignore') for p in root.glob('*.go')}
alltext='\n'.join(files.values())
checks=[]
def ck(name, cond):
    print(('PASS' if cond else 'FAIL'), name)
    checks.append(bool(cond))

clarity=files.get('afk_clarity.go','')
hub=files.get('afk_starbase_hub.go','')
hud=files.get('afk_bottom_hud.go','')
guide=hub
ck('overall starbase level exists', 'func afkStarbaseLevel() int' in clarity and 'afkStarbaseMaxLevel = 7' in clarity)
ck('level has stage names', all(x in clarity for x in ['CALIBRATION','EXPLORATION','RESEARCH','OPERATIONS','INDUSTRIAL','ORBITAL','STELLAR']))
ck('level visible in telemetry', 'STARBASE LEVEL %d' in hud and 'afkStarbaseLevelName' in hud)
ck('objective strip includes level', 'STARBASE LV.%d' in hud)
ck('facilities header explains next unlock', 'NEXT UNLOCK:' in hub and 'afkStarbaseLevelNextUnlock' in hub)
ck('facility exact requirement helper', 'func afkFacilityRequirement' in clarity)
ck('scout lock explicit', 'CURSOR CORE TIER 2' in clarity and 'STARBASE LEVEL 2' in clarity)
ck('tech lock explicit', 'afkScoutDevelopmentSummary()' in clarity and 'CURSOR CORE TIER 3' in clarity)
ck('drone lock explicit', 'COMPLETE MODULE DEVELOPMENT' in clarity and 'CURSOR CORE TIER 5' in clarity)
ck('orbital lock explicit', 'DRONE BAY TO TIER 3' in clarity and 'DRONE NETWORK DEVELOPMENT' in clarity)
ck('drawer shows requirement', 'afkFacilityRequirement(idx)' in hub)
ck('selected facility owns bottom action', 'if afkFacilitiesPanelOpen {' in hud and 'afkFacilityContextAction(idx)' in hud)
ck('no forced core action under facilities', hud.index('if afkFacilitiesPanelOpen {') < hud.index('// Context-sensitive Core progression row.'))
ck('guide starts with progression', 'START HERE' in guide and 'STARBASE LEVEL' in guide and 'FACILITY TIERS' in guide and 'LOCKED FEATURES' in guide)
ck('guide explains tiers are separate', 'THESE ARE NOT STARBASE LEVELS' in guide)
ck('guide explains bank active only', 'BANK UPDATES EVERY 15 SECONDS WHILE STARBASE IS RUNNING' in guide)
ck('telemetry no AFK capacity', '{"AFK CAPACITY", afkFormatDuration(afkCapacitySeconds())' not in hud)
ck('systems no generic locked status', 'status = "LOCKED"' not in hud)
ck('guide has all 3 pages navigable', 'afkGuidePage < 2' in hub)
ck('context action supports all facilities', all(x in clarity for x in ['BUILD CURSOR CORE','BUILD SCOUT PAD','BUILD TECH LAB','OPEN OPERATIONS','OPEN FABRICATOR','BUILD DRONE BAY','BUILD ORBITAL EXTRACTOR']))

print(f"PASS v426 Starbase clarity {sum(checks)}/{len(checks)}" if all(checks) else f"FAIL v426 Starbase clarity {sum(checks)}/{len(checks)}")
raise SystemExit(0 if all(checks) else 1)
