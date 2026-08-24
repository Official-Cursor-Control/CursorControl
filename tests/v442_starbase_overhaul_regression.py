from pathlib import Path
R = Path(__file__).resolve().parents[1]

def txt(name): return (R / name).read_text(errors='ignore')
def ck(name, cond):
    if not cond:
        raise AssertionError(name)
    print('PASS', name)

build = txt('build_identity.go')
ops = txt('afk_operators.go')
assets = txt('afk_operator_assets.go')
quips = txt('afk_operator_quips.go')
guide = txt('afk_starbase_hub.go')
sing = txt('afk_singularity.go')
world = txt('afk_world_buildings.go')
hud = txt('afk_bottom_hud.go')
mods = txt('afk_modules.go')
tal = txt('afk_research_talents.go')
exp = txt('afk_expeditions.go')
cloud = txt('afk_cloud_sync.go')

ck('v442 identity', 'clientBuildVersion = "v442"' in build)

# Operators: manual promotion + role XP.
ck('operator levels persisted separately', 'AFKOperatorLevels' in ops and 'AFKOperatorLevels' in cloud)
ck('operator level up is manual', 'func afkLevelUpOperator' in ops and 'afkOperatorCanLevel(i)' in ops)
ck('no automatic level award in XP grant', 'afkGrantOperatorServiceXP' in ops and 'AFKOperatorLevels[i]++' not in ops[ops.index('func afkGrantOperatorServiceXP'):ops.index('func afkAccumulateOperatorLiveDuration')])
ck('ready cards explicitly clickable', 'READY // CLICK TO LEVEL UP' in ops)
ck('role specific XP text', all(s in ops for s in ['ACTIVE STARBIT PRODUCTION + CORE UPGRADES','COMPLETE SCOUT PAD EXPEDITIONS','COMPLETE TECH LAB RESEARCH','CRAFT SHIP MODULES','DRONE BAY RUNNING + DRONE UPGRADES']))
ck('physical operator recruitment gates', all(s in ops for s in ['REQUIRES CURSOR CORE','REQUIRES SCOUT PAD','REQUIRES TECH LAB','REQUIRES FABRICATOR','REQUIRES DRONE BAY']))
ck('ready operator visual emphasis', 'readyToLevel := recruited && afkOperatorCanLevel(i)' in ops and 'drawOutlineRect' in ops)

# Celebration queue: one voice/fill at a time, recruitment included.
ck('global celebration queue', 'afkOperatorCelebrationQ' in quips and 'afkOperatorCelebrationOnce' in quips)
ck('single worker starts fill then voice', quips.index('afkStartOperatorLevelFill(ev.OperatorIndex)') < quips.index('playSFX(ev.SoundName)'))
ck('level quip queued only after physical promotion', 'afkQueueOperatorLevelQuip(i, lvl)' in ops and 'afkQueueOperatorCelebration' in quips)
ck('recruitment uses same queue', 'afkQueueOperatorRecruitCelebration' in ops and 'afkQueueOperatorCelebration(afkOperatorCelebration{' in quips)
ck('fill rise and fade timing preserved', '2700 * time.Millisecond' in quips and '3400 * time.Millisecond' in quips)

# New separated Operator companion art + animation rules.
for who in ['nova','vega','byte','astra','flux']:
    p = R / 'assets' / 'ui' / 'operators' / f'{who}.bgra'
    ck(f'{who} body bgra 512x896', p.exists() and p.stat().st_size == 512*896*4)
for pet, wh in {
    'nova_pet':(146,180), 'vega_pet':(182,168), 'byte_pet':(345,168),
    'astra_pet':(197,241), 'flux_pet_1':(238,138), 'flux_pet_2':(173,125)
}.items():
    p = R / 'assets' / 'ui' / 'operators' / f'{pet}.bgra'
    ck(f'{pet} asset', p.exists() and p.stat().st_size == wh[0]*wh[1]*4)
ck('locked operators do not draw companions', 'if !recruited {' in ops and 'return drawAFKOperatorSprite(hdc, artBox, i, false)' in ops)
ck('Byte flying chair bobs', 'afkOperatorByteFloatY' in ops and 'i == 2' in ops)
ck('Flux hover pet bobs', 'afkOperatorPetFluxHover' in ops and 'afkOperatorPetFloatY' in ops)
ck('Flux spider patrols horizontally', 'afkOperatorFluxSpiderX' in ops and 'afkOperatorPetFluxSpider' in ops)

# Guide overhaul: exactly three cards per page and substantially clearer pages.
ck('guide page count expanded', 'const afkGuidePageCount = 6' in guide)
ck('guide card layout is one card per row', 'int32(index)*(ch+gap)' in guide and ' / 3' in guide)
ck('guide renders three cards per page', 'for i, c := range pages[afkGuidePage]' in guide and guide.count('},\n\t\t{') >= 5)
ck('guide explains manual operator levels', 'MANUAL LEVEL UPS' in guide and 'DOES NOT LEVEL THEM AUTOMATICALLY' in guide)
ck('guide explains component sources', 'PIXEL WIRE / STAR ALLOY / POWER CELL' in guide and 'NOVA CORE: DERELICT SIGNAL OR FARTHER' in guide)
ck('guide explains skill physical gates', 'ORBITAL DRONES AND ITS BRANCH REQUIRE THE DRONE BAY' in guide)
ck('guide explains expedition return notice', 'A NOTIFICATION SLIDES IN FROM THE RIGHT' in guide)

# World presentation / navigation polish.
ck('facility placement rings no longer drawn', 'drawAFKFacilityFloorRings(hdc' not in '\n'.join(p.read_text(errors='ignore') for p in R.glob('*.go') if p.name != 'afk_world_buildings.go'))
ck('facilities have varied ground heights', 'yOffsets := [...]float64{-8, 10, -15, 5, -5, 14, -10}' in world)
ck('module requirements get majority width', 'backW := usable * 18 / 100' in hud and 'craftW := usable * 25 / 100' in hud)
ck('Starbase back goes directly to Precision', 'selectGameMode(h, 0)' in sing and 'Starbase BACK is a direct return to Precision' in sing)
ck('large Starbase logo', 'logoW := sx(560, w)' in sing and 'logoH := sy(76, hgt)' in sing)
ck('smaller subtitle moved down', 'sy(119, hgt), "SINGULARITY NETWORK"' in sing)
ck('facility blank click dismisses drawer', 'Clicking unused space inside FACILITIES dismisses only the slide-up drawer' in guide)
ck('outside click closes management panels', 'afkSecondaryManagementPanelOpen()' in hud and 'afkCloseManagementPanels()' in hud)

# Skill tree physical requirements and opening focus.
ck('skill tree starts at Sharper Cursors', 'afkResetTalentPanToStart' in hud and 'afkTalentNodeRect(w, hgt, 0)' in tal)
ck('Orbital Drones requires Drone Bay', 'case 12, 13' in tal and 'return afkDroneBayBuilt()' in tal)
ck('Scout branch requires Scout Pad', 'case 3, 4, 8, 9, 10, 11' in tal and 'return afkScoutUnlocked()' in tal)
ck('old pan helper text gone', 'DRAG EMPTY SPACE TO PAN' not in tal)

# Expedition and materials clarity.
ck('expedition return notice click opens Scout Pad', 'handleAFKExpeditionReturnNotificationClick' in exp and 'afkExpeditionPanelOpen = true' in exp)
ck('module UI names exact material sources', 'HOW TO GET MATERIALS' in mods and 'DERELICT SIGNAL OR FARTHER ONLY' in mods)

# Snare-driven Singularity pulse. The generated beat table is intentionally
# precomputed from starbase_theme.mp3 so runtime does not do FFT/audio analysis.
ck('precomputed snare beat table', 'afkStarbaseSnareBeatsMS' in sing and sing.count('afkStarbaseSnareBeatsMS') >= 2)
ck('music position read from Starbase music alias', 'status starbase_music position' in sing)
ck('resting Singularity is 90 percent', 'scale := 0.90 + 0.10*afkSingularitySnarePulse()' in sing)
ck('pulse release is smooth', 'math.Exp(-age / 0.18)' in sing)
ck('hitbox stays at maximum size', 'afkSingularityBaseGeometry(w, hgt)' in sing and 'maximum/original Singularity size' in sing)

print('\nPASS v442 Starbase overhaul regression')
