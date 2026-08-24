from pathlib import Path
import numpy as np
R=Path(__file__).resolve().parents[1]

def t(n): return (R/n).read_text(errors='ignore')
def ck(name, cond):
    if not cond: raise AssertionError(name)
    print('PASS', name)

build=t('build_identity.go'); ops=t('afk_operators.go'); assets=t('afk_operator_assets.go')
quips=t('afk_operator_quips.go'); audio=t('audio_runtime.go'); polish=t('polish.go')
prog=t('progression_achievements.go'); world=t('afk_world_buildings.go'); fac=t('afk_facility_assets.go')
bank=t('afk_starbit_bank.go'); hud=t('afk_bottom_hud.go'); sing=t('afk_singularity.go')

ck('v444 identity', 'clientBuildVersion = "v444"' in build)
ck('operator voice duck requested before quip', quips.index('requestOperatorVoiceDuck(hold)') < quips.index('playSFX(ev.SoundName)'))
ck('operator voice duck is strong', 'return 0.26' in polish and 'if duck < 0.20' in prog)
ck('operator music returns after voice', 'time.Sleep(wait + 90*time.Millisecond)' in audio and 'applyAudioVolumes()' in audio)

ck('core exact nudge', 'cx -= sx(13, w)' in world and 'y += sy(8, hgt)' in world)
ck('scout exact nudge', 'case afkWorldScout:' in world and 'y += sy(18, hgt)' in world)
ck('ops exact nudge', 'case afkWorldOps:' in world and world.count('y += sy(18, hgt)') >= 2)
ck('communications dish moved down 25', 'dishY := y + sy(25, hgt)' in fac)

ck('operator card copy simplified', 'START JOBS // EARN SERVICE XP // CLICK LEVEL UP' in ops and 'READY FOR WORK // CLICK TO START' not in ops)
ck('operator redundant work line removed', 'afkOperatorWorkText(i)' not in ops[ops.index('func drawAFKOperators'):])
ck('pets drawn after body', ops.index('drawn := drawAFKOperatorSprite') < ops.index('drawAFKOperatorPetSprite(hdc, pet, afkOperatorPetNova)'))
ck('pet movement enlarged', 'sy(15+float64((operatorIndex+petSlot)%3)*3' in ops and 'sy(13, hgt)' in ops and 'sx(36, w)' in ops)
ck('locked operators never animate pets', 'return drawAFKOperatorSprite(hdc, artBox, i, false)' in ops)
ck('Byte chair floats', 'bodyBox = afkOperatorAnimatedRect(bodyBox, 0, afkOperatorByteFloatY(now, hgt))' in ops)
ck('Flux spider patrols', 'afkOperatorFluxSpiderX(now, w)' in ops)

root=R/'assets'/'ui'/'operators'
for body in ['nova','vega','byte','astra','flux']:
    p=root/f'{body}.bgra'
    ck(f'{body} cleaned body canvas', p.exists() and p.stat().st_size == 512*896*4)
for pet,wh in {'nova_pet':(146,180),'vega_pet':(182,168),'byte_pet':(345,168),'astra_pet':(197,241),'flux_pet_1':(238,138),'flux_pet_2':(173,125)}.items():
    p=root/f'{pet}.bgra'; ck(f'{pet} reapplied asset', p.exists() and p.stat().st_size == wh[0]*wh[1]*4)

# Known original companion locations must be empty in cleaned body canvases.
placements=[('nova','nova_pet',(108,43),(146,180)),('vega','vega_pet',(48,84),(182,168)),('flux','flux_pet_1',(201,64),(238,138)),('flux','flux_pet_2',(107,688),(173,125))]
for body,pet,(x,y),(pw,ph) in placements:
    b=np.fromfile(root/f'{body}.bgra',dtype=np.uint8).reshape(896,512,4)
    p=np.fromfile(root/f'{pet}.bgra',dtype=np.uint8).reshape(ph,pw,4)
    mask=p[:,:,3]>0
    ck(f'{body} duplicate {pet} absent', np.count_nonzero(b[y:y+ph,x:x+pw,3][mask]) == 0)

ck('bank update pulse state', 'afkBankPulseStarted' in bank and 'afkBankProtected != oldProtected' in bank)
ck('bank pulse drawn strong yellow', 'drawAFKStarbitBankPulse' in hud and 'rgb(255, 226, 72)' in hud and '120*strength' in hud)

ck('unlocked ship flyby uses owned ships', 'afkChooseUnlockedFlybyShip' in world and 'shipUnlocked(id)' in world)
ck('flyby interval 2-5 minutes', '120 + rand.Intn(181)' in world)
ck('flyby reward 25-150 nav', '25 + rand.Intn(126)' in world and 'AFKNavigationData = saturatingAdd' in world)
ck('flyby is clickable before world buildings', 'handleAFKUnlockedShipFlybyClick' in sing and sing.index('handleAFKUnlockedShipFlybyClick') < sing.index('handleAFKPhysicalBuildingClick'))
ck('flyby teaches nav reward on hover', 'CLICK // NAV DATA' in world)
print('\nPASS v444 Starbase polish regression')
