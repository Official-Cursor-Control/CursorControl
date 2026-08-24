from pathlib import Path
import re
root=Path(__file__).resolve().parents[1]
orb=(root/'afk_orbital.go').read_text()
hud=(root/'afk_bottom_hud.go').read_text()
sing=(root/'afk_singularity.go').read_text()
main=(root/'main.go').read_text()
research=(root/'afk_research_talents.go').read_text()
checks=[]
def check(name, cond):
    checks.append((name,bool(cond)))

check('section10 save completion', 'AFKSection10Complete' in main)
check('orbital built save', 'AFKOrbitalExtractorBuilt' in main)
check('orbital tier save', 'AFKOrbitalExtractorTier' in main)
check('orbital burst counter', 'AFKOrbitalBurstsFired' in main)
check('unlock requires section9', 'gameMeta.AFKSection9Complete' in orb and 'afkDroneBayTier() >= 3' in orb)
check('build costs Starbits', 'afkSpendStarbits(afkOrbitalBuildCost)' in orb)
check('five hard-coded upgrades', re.search(r'afkOrbitalUpgradeCount\s*=\s*5', orb) is not None and '[5][20]int64' in orb)
check('three orbital tiers', re.search(r'afkOrbitalMaxTier\s*=\s*3', orb) is not None)
check('periodic cooldown', 'afkOrbitalCooldownSeconds' in orb and '60 - afkOrbitalUpgradeLevel(1)' in orb)
check('discrete active burst', 'afkProcessOrbitalLive(now)' in sing)
check('offline orbital disabled', 'afkOrbitalOfflineStarbits(paid)' not in sing)
check('effective hud rate', 'afkOrbitalEffectivePerSecondMilli()' in sing)
check('research speed hooked', 'afkOrbitalResearchSpeedMultiplierMilli()' in orb)
check('research output hooked', 'afkOrbitalResearchOutputMultiplierMilli()' in orb)
check('engineer training research exists', 'ENGINEER TRAINING' in research.upper())
check('void piercers research exists', 'VOID PIERCERS' in research.upper())
check('orbital module bonus hooked', 'afkModuleDroneOrbitalMultiplierMilli()' in orb)
check('orbital bottom tab', '"ORBITAL"' in hud)
check('current Starbase utility navigation', all(x in hud for x in ['FACILITIES','GUIDE','SYSTEMS','afkFacilityButtonRect']))
check('orbital bottom build action', 'BUILD ORBITAL EXTRACTOR // TIER 1' in hud)
check('orbital upgrade action', 'afkBuyOrbitalUpgrade' in hud)
check('orbital ascension action', 'afkAscendOrbital' in hud)
check('orbital panel renderer', 'drawAFKOrbital(hdc, w, hgt)' in sing)
check('orbital world renderer', 'drawAFKOrbitalWorld(hdc, w, hgt)' in sing)
check('hard-coded cannon pixels', 'Hard-coded 8-bit orbital cannon' in orb)
check('charge meter', 'Charge meter immediately under the cannon' in orb)
check('beam fire animation', '700*time.Millisecond' in orb)
check('objective unlock name', 'UNLOCK ORBITAL EXTRACTOR' in orb)
check('final endgame objective', 'AFK CORE LOOP COMPLETE' in orb)
check('no SpaceCoin mutation', 'SpaceCoins' not in orb and 'SpaceCoin' not in orb)

failed=[n for n,v in checks if not v]
print(f'{len(checks)-len(failed)}/{len(checks)} Section 10 assertions: ' + ('PASS' if not failed else 'FAIL'))
for n,v in checks:
    print(('PASS' if v else 'FAIL'), '-', n)
if failed: raise SystemExit(1)
