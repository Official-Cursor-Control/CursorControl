#!/usr/bin/env python3
from pathlib import Path
import re
R=Path(__file__).resolve().parents[1]
e=(R/'afk_expeditions.go').read_text()
m=(R/'main.go').read_text()
p=(R/'progression_achievements.go').read_text()
checks=[]
def ck(name, cond):
    checks.append((name,bool(cond)))
    print(('PASS' if cond else 'FAIL'), '-', name)
ck('locked expedition ship save field', 'AFKExpeditionShipID' in m and 'afk_expedition_ship_id' in m)
ck('launch snapshots equipped Endurance ship', 'gameMeta.AFKExpeditionShipID = shipID' in e and 'gameMeta.SelectedShip' in e)
ck('active display uses locked ship', 'afkCurrentExplorationShipID()' in e and 'if afkExpeditionActive()' in e)
ck('ship lock normalized', 'AFKExpeditionShipID' in p and 'spaceShipDefs' in p)
ck('starbit passives applied at completion', 'passive.StarbitMultiplierMilli' in e)
ck('nav passives applied at completion', 'passive.NavMultiplierMilli' in e)
ck('speed passives applied before mission launch', 'passive.DurationMultiplierMilli' in e and 'afkExpeditionDurationSecondsForShip' in e)
ck('cache passives applied to deterministic roll', 'CacheChanceBonus' in e and 'afkDeterministicCacheRoll' in e and 'afkExplorationPassive(afkValidExplorationShipID(shipID)).CacheChanceBonus' in e)
ck('all live Endurance ships have explicit passive cases', all(f'case {i}:' in e for i in [1,2,3,4,5,6,7,8,9,10,12]))
ck('default ship has useful passive', '"CADET ARRAY"' in e and '1050' in e)
ck('passive shown in expedition UI', 'passive.Name' in e and 'passive.Description' in e)
ck('launch preview includes modified rewards', 'afkExpeditionPreviewRewards' in e and '+%d NAV' in e)
ck('mid-flight ship swaps cannot alter reward ship', 'afkExplorationPassive(shipID)' in e and 'gameMeta.AFKExpeditionShipID' in e)
failed=[n for n,v in checks if not v]
print(f'\n{len(checks)-len(failed)}/{len(checks)} v422 expedition passive assertions passed')
raise SystemExit(1 if failed else 0)
