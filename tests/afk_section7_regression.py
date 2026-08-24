from pathlib import Path
import re
root=Path(__file__).resolve().parents[1]
ops=(root/'afk_operators.go').read_text()
hud=(root/'afk_bottom_hud.go').read_text()
sing=(root/'afk_singularity.go').read_text()
main=(root/'main.go').read_text()
exp=(root/'afk_expeditions.go').read_text()
res=(root/'afk_research_talents.go').read_text()
checks=[]
def ok(name, cond):
    checks.append((name,bool(cond)))

ok('5 hard-coded operators', ops.count('{"NOVA"')==1 and all(x in ops for x in ['"VEGA"','"BYTE"','"ASTRA"','"FLUX"']))
ok('operator unlock tied to core tier 4 + section6', 'afkCoreTier() >= 4 && gameMeta.AFKSection6Complete' in ops)
ok('explicit recruit costs', all(v in ops for v in ['25_000_000','60_000_000','150_000_000','400_000_000','1_200_000_000']))
ok('explicit level thresholds 1-10', 'afkOperatorLevelXPSeconds' in ops and '1440 * 60' in ops and 'afkOperatorMaxLevel = 10' in ops)
ok('operator saves', all(x in main for x in ['AFKSection7Complete','AFKOperatorsRecruited','AFKOperatorXPSeconds','AFKOperatorsRecruitedCount']))
ok('starbit operator multiplier connected', 'afkOperatorStarbitProductionMultiplierMilli()' in sing)
ok('expedition operator multiplier connected', 'afkOperatorExpeditionMultiplierMilli()' in exp)
ok('research cost operator multiplier connected', 'afkOperatorResearchCostMultiplierMilli()' in res)
ok('Astra cache/expedition multiplier connected', 'afkOperatorStarbitRewardMultiplierMilli()' in ops)
ok('future drone multiplier present but isolated', 'afkOperatorDroneMultiplierMilli' in ops and 'Section 9' in ops)
ok('live XP uses duration remainder', 'afkAccumulateOperatorLiveDuration(delta)' in sing and 'afkOperatorLiveRemainder' in ops)
ok('operator XP is active-session only', 'afkAccumulateOperatorXP(paid)' not in sing)
ok('section7 objective requires 3 recruits + level2', 'recruited >= 3' in ops and 'afkOperatorLevel(i) >= 2' in ops)
ok('tier4-to-tier5 gated by modules after operators', 'case 4:' in hud and 'return gameMeta.AFKSection8Complete' in hud)
ok('current bottom navigation exposes operator systems', all(x in hud for x in ['FACILITIES','GUIDE','SYSTEMS','afkOperatorPanelOpen']))
ok('operator panel is drawn', 'drawAFKOperators(hdc, w, hgt)' in sing)
ok('operator panel suppresses singularity helper', '!afkOperatorPanelOpen' in sing)
ok('operator cards use hard-coded pixel rectangles', 'drawAFKPixelOperator' in ops and ops.count('fillSolidRect') >= 9)
ok('recruit action exists in bottom context', 'RECRUIT %s // %s' in hud and 'afkRecruitOperator(i)' in hud)
ok('panel card selection exists', 'afkOperatorCardRect(w, hgt, i)' in sing)

failed=[n for n,v in checks if not v]
for n,v in checks:
    print(('PASS' if v else 'FAIL')+': '+n)
print(f'\n{len(checks)-len(failed)}/{len(checks)} assertions passed')
if failed:
    raise SystemExit(1)
