from pathlib import Path
R = Path(__file__).resolve().parents[1]

def txt(name): return (R/name).read_text(errors='ignore')

def ck(name, cond):
    if not cond:
        raise AssertionError(name)
    print('PASS', name)

build = txt('build_identity.go')
exp = txt('afk_expeditions.go')
ops = txt('afk_operators.go')
quips = txt('afk_operator_quips.go')
mods = txt('afk_modules.go')
tal = txt('afk_research_talents.go')
hud = txt('afk_bottom_hud.go')
sing = txt('afk_singularity.go')
world = txt('afk_world_buildings.go')
main = txt('main.go')

ck('v441 identity', 'clientBuildVersion = "v441"' in build)
ck('expedition return notice exists', 'YOUR SHIP HAS RETURNED FROM AN EXPEDITION.' in exp and 'CLAIM YOUR REWARD' in exp)
ck('expedition notice slides from right', 'startLeft := w + sx(12, w)' in exp and 'targetLeft := w - width - sx(18, w)' in exp)
ck('expedition notice opens Scout Pad', 'afkExpeditionPanelOpen = true' in exp and 'handleAFKExpeditionReturnNotificationClick' in exp)
ck('recruitment triggers operator fill', 'afkStartOperatorLevelFill(i)' in ops)
ck('level-up fill still starts with quip', quips.index('afkStartOperatorLevelFill(operatorIndex)') < quips.index('playSFX(name)'))
ck('operator portraits moved farther down', 'r.Top + sy(48, hgt)' in ops and 'r.Top + sy(338, hgt)' in ops)
ck('material source explanation explicit', 'HOW TO GET MATERIALS' in mods and 'ANY ROUTE, BETTER ODDS ON LONGER ROUTES' in mods and 'DERELICT SIGNAL OR FARTHER ONLY' in mods)
ck('skill tree starts on Sharper Cursors', 'afkResetTalentPanToStart(w, hgt)' in hud and 'afkTalentNodeRect(w, hgt, 0)' in tal)
ck('skill pan instruction removed', 'DRAG EMPTY SPACE TO PAN IN ANY DIRECTION' not in tal)
ck('locked lines use background-blue family', 'afkRGBLerp(8, 54, 103, 18, 101, 173, pulse)' in tal)
ck('gold routes keep priority', 'afkRGBLerp(128, 88, 18, 250, 202, 68, pulse)' in tal and 'afkRGBLerp(186, 112, 16, 255, 226, 92, brightPulse)' in tal)
ck('drone skill gated by Drone Bay', 'case 12, 13' in tal and 'return afkDroneBayBuilt()' in tal and 'BUILD DRONE BAY' in tal)
ck('Scout skills have physical requirement', 'case 3, 4, 8, 9, 10, 11' in tal and 'return afkScoutUnlocked()' in tal and 'BUILD SCOUT PAD' in tal)
ck('click outside closes secondary Starbase interfaces', 'afkSecondaryManagementPanelOpen()' in hud and 'afkCloseManagementPanels()' in hud)
ck('facilities and guide outside click close', 'afkFacilitiesPanelOpen && !pointInRect' in hud and 'afkGuidePanelOpen && !pointInRect' in hud)
ck('facility rings are centered on building centers', 'afkWorldFacilityRingCenter' in world and 'drawAFKFacilityFloorRings' in world)
ck('legacy per-building x nudges removed', 'cx -= sx(25, w)' not in world and 'cx -= sx(10, w)' not in world)
ck('new Starbase wordmark loaded', 'starbaseLogoWordmarkBGRA = readExternalBytes("ui", "starbase_logo_wordmark.bgra")' in main)
ck('new Starbase wordmark drawn', 'drawRawBGRATrimmedFit(hdc, starbaseLogoWordmarkBGRA' in sing)
asset = R/'assets/ui/starbase_logo_wordmark.bgra'
ck('wordmark asset exact size', asset.exists() and asset.stat().st_size == 948*129*4)
print('\nPASS v441 Starbase polish regression')
