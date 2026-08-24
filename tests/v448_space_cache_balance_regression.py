from pathlib import Path
import re, sys
root=Path(__file__).resolve().parents[1]
main=(root/'main.go').read_text()
bid=(root/'build_identity.go').read_text()
checks=[]
def ck(name, cond):
    checks.append((name,bool(cond)))
ck('build is v448', 'clientBuildVersion = "v448"' in bid)
fn=re.search(r'func resolveSpaceCacheReward\(\) \{(.*?)\n\}', main, re.S)
body=fn.group(1) if fn else ''
ck('cache EXP threshold is 85 percent', 'if r < 0.85 {' in body)
ck('coin band ends at 90 percent', '} else if r < 0.90 {' in body)
ck('documented split is 85 EXP 5 coins 10 ship', '85% EXP / 5% Space Coins / 10% spaceship' in body)
ck('spaceship branch preserved', 'ship := rollSpaceShip()' in body and 'updateShipPityAfterRoll(ship)' in body)
ck('EXP reward still grants 100 to 1000', 'exp := (1 + rand.Intn(10)) * 100' in body)
for n,r in checks: print(('PASS' if r else 'FAIL')+': '+n)
if not all(r for _,r in checks): sys.exit(1)
print(f'{sum(r for _,r in checks)}/{len(checks)} passed')
