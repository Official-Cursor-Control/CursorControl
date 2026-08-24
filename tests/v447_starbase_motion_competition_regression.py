from pathlib import Path
import re, sys
root = Path(__file__).resolve().parents[1]
checks=[]
def ok(name, cond):
    checks.append((name, bool(cond)))
main=(root/'main.go').read_text()
sing=(root/'afk_singularity.go').read_text()
decor=(root/'afk_facility_assets.go').read_text()
ui=(root/'ui_leaderboards_profiles.go').read_text()
bid=(root/'build_identity.go').read_text()

m=re.search(r'func backgroundMotionEnabled\(\) bool \{(.*?)\n\}', main, re.S)
ok('background toggle solely gates background scrolling', m and 'return gameMeta.MovingBackground' in m.group(1) and 'ReducedMotion' not in m.group(1).split('return')[1])
ok('reduced motion freezes singularity rotation', 'degree := 0' in sing and 'if !gameMeta.ReducedMotion' in sing and 'afkStarbaseVisualElapsedSeconds()*15.0' in sing)
ok('decor uses stable session jitter', 'func afkDecorJitter' in decor and 'gameMeta.Sessions' in decor)
ok('multiple moon flags', decor.count('drawGround("starbase_flag"') >= 4)
ok('multiple moon dishes', decor.count('drawGround("communications_dish"') >= 3)
ok('old tech-lab fixed prop coordinates removed', 'field.Left+sx(330, w)' not in decor and 'field.Left+sx(365, w)' not in decor)
ok('competition current removes rank', 'headers = []string{"#", "PLAYER", "EASY", "NORMAL", "HARD", "INSANE", "TOTAL POINTS"}' in ui)
ok('competition previous removes rank', 'headers = []string{"#", "PLAYER", "TOTAL POINTS", "AUTOMATIC REWARD"}' in ui)
ok('competition all-time removes rank', 'headers = []string{"#", "PLAYER", "WINS", "PODIUMS", "1ST", "2ND", "3RD"}' in ui)
mver=re.search(r'clientBuildVersion = \"v(\d+)\"', bid)
ok('build includes v447 changes', bool(mver and int(mver.group(1)) >= 447))
for name,res in checks:
    print(('PASS' if res else 'FAIL')+': '+name)
if not all(r for _,r in checks):
    sys.exit(1)
print(f'{sum(r for _,r in checks)}/{len(checks)} passed')
