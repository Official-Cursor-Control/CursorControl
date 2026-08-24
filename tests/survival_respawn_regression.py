from pathlib import Path
import re

src = Path('survival_respawn.go').read_text()
main = Path('main.go').read_text()
exp = Path('afk_expeditions.go').read_text()
checks=[]

def ck(name, cond):
    checks.append((name, bool(cond)))
    print(('PASS' if cond else 'FAIL') + ' - ' + name)

# Root cause: afkEnsureDeviceID must execute before the afkCloudMu lock used to read session token.
post = src[src.index('func survivalRespawnPost'):src.index('func survivalRespawnPostUIMessage')]
ck('revive device ID lookup occurs before cloud mutex lock', post.index('afkEnsureDeviceID()') < post.index('afkCloudMu.Lock()'))
ck('revive request has hard HTTP timeout', 'Timeout: 6 * time.Second' in post)
ck('malformed server JSON is rejected', 'invalid revive server response' in post)
ck('revive errors use errors.New rather than dynamic fmt.Errorf', 'errors.New(out.Message)' in post and 'fmt.Errorf(out.Message)' not in post)

# Worker goroutines must not directly reset/start gameplay.
worker_area = src[src.index('func survivalRespawnRefreshAsync'):src.index('func survivalRespawnProcessMainThread')]
ck('workers communicate through PostMessage', 'survivalRespawnPostUIMessage()' in worker_area)
ck('workers do not call resetToWaiting', 'resetToWaiting(' not in worker_area)
ck('workers do not call startSurvivalGame', 'startSurvivalGame(' not in worker_area)
ck('main-thread handler owns checkpoint restart', 'resetToWaiting(h)' in src[src.index('func survivalRespawnProcessMainThread'):])
ck('window proc handles revive completion message', 'case WM_SURVIVAL_RESPAWN:' in main)
ck('stale revive responses are generation scoped', src.count('generation != survivalRespawnGeneration') >= 2)
ck('unavailable revive status is retryable', 'CLICK TO RETRY' in src and 'survivalRespawnRefreshAsync()' in src[src.index('func survivalRespawnHandleClick'):])

# Expedition behavior requested with the revive fix.
ship = exp[exp.index('func drawAFKScoutShip'):exp.index('func drawAFKExpeditionLayer')]
ck('idle/in-flight ship behavior is state dependent', 'inFlight := afkExpeditionActive()' in ship)
ck('thruster is drawn only while in flight', re.search(r'if inFlight \{\n\t\tfire := afkExpeditionThrusterColour\(\)', ship) is not None)
ck('ship movement loops across panel', 'math.Mod(t/7.5, 1.0)' in ship)
ck('ship is clipped through panel edges', 'intersectClipRect.Call' in ship)
ck('no decorative aura drawing path in scout renderer', 'drawGlow' not in ship and 'drawAura' not in ship and 'ellipse.Call' not in ship)


# v421 revive UX: right-side choice panel, instant decline-to-report, and safe revive benefits.
ck('revive panel uses right side of arena', 'ar.Right - margin - pw' in src)
ck('revive prompt wording matches requested interface', 'DO YOU WANT TO REVIVE?' in src and 'YOU HAVE %d REVIVES REMAINING.' in src)
ck('non-revive click in game area opens report immediately', 'finishSurvivalFailure(h)' in src[src.index('func survivalRespawnHandleClick'):src.index('func survivalReviveImmune')])
ck('revive restores maximum station HP', 'survivalHP = survivalMaxHP()' in src[src.index('func survivalRespawnActivateBenefits'):])
ck('revive activates nova bomb effect', 'survivalCollectPickup(survivalPickupBomb)' in src[src.index('func survivalRespawnActivateBenefits'):])
ck('revive grants exactly three seconds immunity', 'now.Add(3 * time.Second)' in src[src.index('func survivalRespawnActivateBenefits'):])
surv = Path('survival.go').read_text()
expansion = Path('survival_expansion.go').read_text()
boss3 = Path('survival_boss3.go').read_text()
ck('survival failure path respects revive immunity', 'if survivalReviveImmune(now)' in surv[surv.index('func survivalFail'):surv.index('func finishSurvivalFailure')])
ck('normal station impacts respect revive immunity', 'REVIVE IMMUNITY // IMPACT ABSORBED' in surv)
ck('pulse mine damage respects revive immunity', 'REVIVE IMMUNITY // PULSE ABSORBED' in expansion)
ck('Terminus damage respects revive immunity', 'survivalReviveImmune(now)' in boss3[boss3.index('func terminusKillOnMeteor'):boss3.index('func terminusDrawMeteors')] and 'survivalReviveImmune(now)' in boss3[boss3.index('func terminusDamagePlayer'):])
ck('Survival failure no longer auto-opens report on timer', 'setTimer.Call(h, TIMER_FAIL_RESET, 9000, 0)' not in surv)

passed=sum(v for _,v in checks)
print(f'\n{passed}/{len(checks)} revive/expedition assertions passed')
raise SystemExit(0 if passed == len(checks) else 1)
