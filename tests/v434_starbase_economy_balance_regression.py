from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]
read = lambda name: (root/name).read_text(encoding='utf-8')
rank = read('afk_global_rank_bonus.go')
research = read('afk_research_talents.go')
ops = read('afk_operators.go')
mods = read('afk_modules.go')
drones = read('afk_drones.go')
orbital = read('afk_orbital.go')
exped = read('afk_expeditions.go')
endgame = read('afk_endgame.go')
build = read('build_identity.go')

sing = read('afk_singularity.go')
hud = read('afk_bottom_hud.go')

checks = []
def ok(name, cond):
    checks.append((name, bool(cond)))

ok('build v434', 'clientBuildVersion = "v434"' in build)
ok('rank +10 per step', 'int64(i)*100' in rank)
ok('rank documents Master III x2.70', 'Master III reaches x2.70' in rank)
ok('permanent supports combine additively', 'func afkPermanentStarbitSupportMultiplierMilli()' in sing and 'm += afkResearchStarbitMultiplierMilli() - 1000' in sing and 'm += afkPrestigeMultiplierMilli() - 1000' in sing)
ok('auto cursor no longer compounds broad layers', 'base = base * afkResearchStarbitMultiplierMilli() / 1000' not in sing and 'base = base * afkTalentStarbitMultiplierMilli() / 1000' not in sing)
ok('popup explains additive support total', 'PERMANENT SUPPORT TOTAL' in hud and 'addBonus("RESEARCH"' in hud)

# Tech Lab production max = 5 + 4 + 5 + 4 + 4 + 8 = 30%
for token in ['* 5 // +0.5%/level, +5% max', '* 4 // +0.4%/level, +4% max', '* 8 // +0.8%/level, +8% max']:
    ok('research token '+token, token in research)
ok('research manual max +40%', 'afkResearchLevel(2))*40' in research)
ok('research expedition cap -15%', 'if v < 850' in research)
ok('drone research 3x +1%/level', research.count('afkResearchLevel(10))*10') == 1 and research.count('afkResearchLevel(11))*10') == 1 and research.count('afkResearchLevel(13))*10') == 1)
ok('orbital research output +3%/level', 'afkResearchLevel(14))*30' in research)

ok('talent production additive +30 max', 'm += 100' in research and research.count('m += 100') == 3)
ok('talent manual +25%', 'return 1250' in research)
ok('talent expedition -10%', 'return 900' in research)
ok('talent cache +20%', 'return 1200' in research)

ok('operator starbits +2% level', 'int64(lvl)*20' in ops and 'ALL STARBITS +%d%%", lvl*2' in ops)
ok('operator expedition max -10%', 'v < 900' in ops)
ok('operator rewards +3% level', 'int64(lvl)*30' in ops)

ok('module all starbits +15', 'return 1150' in mods and 'ALL STARBITS +15%' in mods)
ok('module auto cursor +15', 'AUTO CURSOR OUTPUT +15%' in mods)
ok('module manual +40', 'return 1400' in mods)
ok('module cache +25', 'STAR CACHE REWARD +25%' in mods)
ok('module drone orbital +20', 'DRONE + ORBITAL OUTPUT +20%' in mods and 'return 1200' in mods)

ok('prestige +5 per rank', 'int64(afkPrestigeRank())*50' in endgame)
ok('prestige copy +5', '+5% ALL STARBIT PRODUCTION' in endgame)

for token in ['*10) / 1000', '*15) / 1000']:
    ok('drone scale '+token, token in drones)
ok('orbital output scaling reduced', 'afkOrbitalUpgradeLevel(0))*30' in orbital and 'afkOrbitalUpgradeLevel(3))*20' in orbital)
ok('orbital contribution capped 50%', 'cap := afkContinuousStarbitsPerSecondMilli() / 2' in orbital)

ok('nav passive max 18%', '+18% NAV DATA' in exped and '1280' not in exped.split('func afkExplorationPassive',1)[1].split('func afkValidExplorationShipID',1)[0])
ok('hybrid passive max 15 nav', '+15% STARBITS // +15% NAV // -8% TIME' in exped)
ok('expedition speed passives reduced', '-12% EXPEDITION TIME' in exped and '-20% EXPEDITION TIME' not in exped)

failed = [name for name, passed in checks if not passed]
for name, passed in checks:
    print(('PASS' if passed else 'FAIL') + ' - ' + name)
print(f'\n{len(checks)-len(failed)}/{len(checks)} checks passed')
if failed:
    raise SystemExit(1)
