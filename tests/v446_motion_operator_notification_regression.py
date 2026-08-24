from pathlib import Path
R=Path(__file__).resolve().parents[1]

def t(n): return (R/n).read_text(errors='ignore')
def ck(name, cond):
    if not cond: raise AssertionError(name)
    print('PASS', name)

build=t('build_identity.go'); main=t('main.go'); surv=t('survival.go')
b2=t('survival_boss2.go'); b3=t('survival_boss3.go'); sing=t('afk_singularity.go')
exp=t('afk_expeditions.go'); ops=t('afk_operators.go'); warp=t('endurance_warp.go')

ck('v446 identity', 'clientBuildVersion = "v446"' in build)
ck('shared background motion gate uses both toggles', 'return gameMeta.MovingBackground && !gameMeta.ReducedMotion' in main)
ck('Precision and Endurance background never disappears when motion disabled', 'if !gameMeta.MovingBackground {\n\t\treturn' not in main[main.index('func drawScrollingArenaBackground'):main.index('func drawArenaParticles')])
ck('Precision/Endurance scrolling gated', main[main.index('func drawScrollingArenaBackground'):main.index('func drawArenaParticles')].count('backgroundMotionEnabled()') >= 2)
ck('Survival sector scrolling gated', 'if backgroundMotionEnabled()' in surv[surv.index('func drawSurvivalArena'):])
ck('Void Serpent background scrolling gated', 'cropXMax > 0 && backgroundMotionEnabled()' in b2)
ck('Terminus background scrolling gated', 'if backgroundMotionEnabled()' in b3[b3.index('func terminusDrawScrollingBackground'):])
ck('Starbase background scrolling gated', 'if backgroundMotionEnabled()' in sing[sing.index('func drawAFKScrollingArtBackground'):])
ck('Expedition travel background remains visible but static', '!backgroundMotionEnabled()' in exp and 'drawAFKExpeditionBackgroundSlice' in exp[exp.index('func drawAFKExpeditionFlightBackground'):])

ck('Endurance rail slightly thicker', 'return 30.0' in warp[warp.index('func enduranceTrackWidth'):warp.index('func endurancePathCenterBounds')])
ck('Operator jobs substantially longer and staggered', all(x in ops for x in ['15 * 60, 7 * 60','23 * 60, 10 * 60','34 * 60, 14 * 60','48 * 60, 19 * 60','65 * 60, 25 * 60']))
ck('Operator job completion still never auto restarts', 'Nothing automatically restarts after the cooldown' in ops)
ck('Operator reminder persisted', 'AFKOperatorWaitNoticeSeenUnix' in main)
ck('Operator reminder detects actionable operator events', 'afkOperatorWaitNoticeEvent' in ops and 'afkOperatorCanLevel(i) || cooldownEnd <= now.Unix()' in ops)
ck('Operator reminder copy', 'YOUR OPERATORS ARE WAITING FOR YOU' in ops and 'OPEN OPERATIONS' in ops)
ck('Operator reminder opens Operator interface', 'afkOperatorPanelOpen = true' in ops[ops.index('func handleAFKOperatorWaitNotificationClick'):])
ck('Operator reminder drawn in Starbase', 'drawAFKOperatorWaitNotification(hdc, w, hgt)' in sing)
ck('Operator reminder clickable before world interaction', sing.index('handleAFKOperatorWaitNotificationClick') < sing.index('handleAFKPhysicalBuildingClick'))
ck('Expedition and Operator reminders stack rather than overlap', 'if afkExpeditionReturnNoticeVisible()' in ops and 'top += sy(104, hgt)' in ops)

print('\nPASS v446 motion/operator notification regression')
