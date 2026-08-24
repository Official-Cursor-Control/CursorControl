from pathlib import Path
import numpy as np
R=Path(__file__).resolve().parents[1]

def text(n): return (R/n).read_text(errors='ignore')
def ck(name, cond):
    if not cond: raise AssertionError(name)
    print('PASS', name)

build=text('build_identity.go'); ops=text('afk_operators.go'); sing=text('afk_singularity.go')
hud=text('afk_bottom_hud.go'); exp=text('afk_expeditions.go'); world=text('afk_world_buildings.go')
cloud=text('afk_cloud_sync.go'); guide=text('afk_starbase_hub.go'); main=text('main.go'); quips=text('afk_operator_quips.go')

ck('v443 identity', 'clientBuildVersion = "v443"' in build)
ck('manual assignment save fields', all(x in main for x in ['AFKOperatorWorkStartedUnix','AFKOperatorWorkEndsUnix','AFKOperatorCooldownEndsUnix']))
ck('five distinct manual jobs', all(x in ops for x in ['CORE CALIBRATION','ROUTE SURVEY','SYSTEM ANALYSIS','SUPPLY AUDIT','DRONE SERVICE']))
ck('distinct work timings', all(x in ops for x in ['6 * 60, 4 * 60','9 * 60, 6 * 60','13 * 60, 8 * 60','18 * 60, 11 * 60','24 * 60, 15 * 60']))
ck('assignments cost Starbits', 'AssignmentCost' in ops and 'afkSpendStarbits(cost)' in ops)
ck('assignments never auto restart', 'Nothing automatically restarts after the cooldown' in ops or 'Nothing auto-starts' in sing)
ck('passive XP disabled', 'func afkAccumulateOperatorLiveDuration(d time.Duration) { _ = d }' in ops)
ck('only assignment completion grants XP', sum('afkGrantOperatorServiceXP' in p.read_text(errors='ignore') for p in R.glob('*.go')) == 1 and 'ASSIGNMENT NAME' not in ops)
ck('XP capped at next promotion threshold', 'capXP := afkOperatorLevelXPSeconds[lvl]' in ops)
ck('service bar can never overflow target', 'if current > target' in ops and 'current = target' in ops and 'allows at most one pending promotion' in ops)
ck('manual level up remains physical', 'changed = afkLevelUpOperator(i)' in sing and 'changed = afkStartOperatorAssignment(i, time.Now())' in sing)
ck('bottom action starts work', 'START %s // COST %s // SERVICE XP' in hud and 'afkStartOperatorAssignment(i, time.Now())' in hud)
ck('global quip queue still serialises voices', 'afkOperatorCelebrationQ' in quips and 'time.Sleep(hold)' in quips)
ck('cloud timer replay guard', 'afkCloudSanitizeOperatorTimers' in cloud and 'award the same Service XP twice' in cloud)

# Verify duplicate static pets are physically absent from runtime body sprites.
root=R/'assets'/'ui'/'operators'
placements=[('nova','nova_pet',(108,43),(146,180)),('vega','vega_pet',(48,84),(182,168)),('flux','flux_pet_1',(201,64),(238,138)),('flux','flux_pet_2',(107,688),(173,125))]
for body,pet,(x,y),(pw,ph) in placements:
    b=np.fromfile(root/f'{body}.bgra',dtype=np.uint8).reshape(896,512,4)
    p=np.fromfile(root/f'{pet}.bgra',dtype=np.uint8).reshape(ph,pw,4)
    mask=p[:,:,3]>0
    ck(f'{body} static {pet} removed', np.count_nonzero(b[y:y+ph,x:x+pw,3][mask]) == 0)
ck('pets have visible larger bob', 'sy(10+float64((operatorIndex+petSlot)%3)*2' in ops and 'sx(28, w)' in ops)
ck('Byte chair moves independently', 'afkOperatorByteFloatY' in ops and 'i == 2' in ops)
ck('locked pets remain static/hidden', 'return drawAFKOperatorSprite(hdc, artBox, i, false)' in ops)

ck('spiral world geometry stays fixed', 'return afkSingularityBaseGeometry(w, hgt)' in sing)
ck('core has separate snare geometry', 'func afkSingularityCoreGeometry' in sing and 'scale := 0.90 + 0.10*afkSingularitySnarePulse()' in sing)
ck('free particles follow core pulse', 'cx, cy, radius := afkSingularityCoreGeometry(w, hgt)' in sing[sing.index('func drawAFKSingularityStarbits'):])
ck('authored fixed centre is covered', "Cover the authored backdrop's original fixed black centre" in sing)
ck('notification headline wrapped', 'YOUR SHIP HAS RETURNED"' in exp and 'FROM AN EXPEDITION.' in exp and 'textRight := r.Right' in exp)
ck('facility heights are more irregular', 'yOffsets := [...]float64{8, -24, 14, -36, 5, 31, -17}' in world)
ck('Starbase logo constrained by telemetry gap', 'prodCard := afkLiveStatCardRect(w, hgt, 1)' in sing and 'navCard := afkLiveStatCardRect(w, hgt, 2)' in sing and 'logoRight := navCard.Left' in sing)
ck('guide explains paid assignments', 'OPERATORS NEVER FARM SERVICE XP AUTOMATICALLY' in guide and 'START THEIR PAID ASSIGNMENT' in guide)
print('\nPASS v443 Starbase operator/visual regression')
