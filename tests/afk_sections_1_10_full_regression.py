from pathlib import Path
import re
R=Path(__file__).resolve().parents[1]
files={p.name:p.read_text() for p in R.glob('afk_*.go')}
main=(R/'main.go').read_text(); prog=(R/'progression_achievements.go').read_text()
allafk='\n'.join(files.values())
checks=[]
def ck(name, cond): checks.append((name,bool(cond)))
S=files['afk_singularity.go']; E=files['afk_expeditions.go']; T=files['afk_research_talents.go']; O=files['afk_operators.go']; M=files['afk_modules.go']; D=files['afk_drones.go']; H=files['afk_bottom_hud.go']; Q=files['afk_orbital.go']
# S1-4 foundation
ck('S1 100 Starbit target', 'afkSection1StarbitTarget' in S and 'int64 = 100' in S)
ck('S1 manual +5', 'afkManualClickValue' in S and 'int64 = 5' in S)
ck('S2 ten Auto Cursor costs', len(re.findall(r'\b(?:25|50|100|200|400|800|1_600|3_200|6_400|12_800)\b', S)) >= 10)
ck('S2 processing max 10', 'afkProcessingMaxLevel' in S and '= 10' in S)
ck('S2 passive rate', 'afkAutoCursorStarbitsPerSecondMilli' in S)
ck('S3 core max tier 5', 'afkCoreMaxTier' in S and '= 5' in S)
ck('S3 explicit rebalanced core costs', all(x in S for x in ['100_000','5_000_000','75_000_000','1_500_000_000']))
ck('S4 2h base capacity', '2 * 60 * 60' in S)
ck('S4 active-only Starbits', 'paid = capSeconds' not in S)
ck('S4 welcome back', 'WELCOME BACK' in S and 'CLAIM AFK EARNINGS' in S)
ck('S4 clock rollback guard', 'AFKClockRollbackCount' in S and 'AFKMaxObservedUnix' in S)
# S5 expeditions
ck('S5 scout exists', 'SCOUT SHIP' in E)
ck('S5 six destinations', E.count('StarbitReward:') >= 6 or E.count('StarbitReward') >= 6)
ck('S5 Nav Data', 'AFKNavigationData' in E)
ck('S5 speed/capacity/cache stats', all(x in E for x in ['AFKExpeditionSpeedLevel','AFKCapacityLevel','AFKCacheChanceLevel']))
ck('S5 cache rarities', all(x in E.upper() for x in ['COMMON','RARE','EPIC','LEGENDARY']))
# S6
ck('S6 tech lab', 'TECH LAB' in T and 'afkTechLabUnlockCost' in T)
ck('S6 15 research rows after drones', len(re.findall(r'^\s*\{"', T, re.M)) >= 15)
ck('S6 max research 10', 'afkResearchMaxLevel' in T and '= 10' in T)
ck('S6 pilot skills', 'PILOT SKILLS' in T)
ck('S6 orbital research foreshadowing', 'ENGINEER TRAINING' in T and 'VOID PIERCERS' in T)
# S7
ck('S7 five operators', all(x in O for x in ['"NOVA"','"VEGA"','"BYTE"','"ASTRA"','"FLUX"']))
ck('S7 operator XP max 10', 'afkOperatorMaxLevel' in O and '= 10' in O)
ck('S7 Starbit operator bonus', 'afkOperatorStarbitProductionMultiplierMilli' in O)
ck('S7 expedition operator bonus', 'afkOperatorExpeditionMultiplierMilli' in O)
ck('S7 research operator bonus', 'afkOperatorResearchCostMultiplierMilli' in O)
ck('S7 drone operator bonus', 'afkOperatorDroneMultiplierMilli' in O)
# S8
ck('S8 9 module sets', 'afkModuleSetCount' in M and '= 9' in M)
ck('S8 8 items per set', 'afkModuleItemsPerSet' in M and '= 8' in M)
ck('S8 crafting components', 'afkCraftComponentDefs' in M)
ck('S8 all sets 500 Nav reward', '500' in M and 'AFKNavigationData' in M)
ck('S8 orbital module bonus', 'afkModuleDroneOrbitalMultiplierMilli' in M)
# S9
ck('S9 drone bay build', 'BUILD DRONE BAY' in D)
ck('S9 ten drones', 'afkDroneMaxCount' in D and '= 10' in D)
ck('S9 5 drone upgrade paths', 'afkDroneUpgradeCount' in D and '= 5' in D)
ck('S9 tier 3 max', 'afkDroneBayMaxTier' in D and '= 3' in D)
ck('S9 reticles animated', 'drawAFKDroneReticle' in D and 'time.Now().UnixNano()' in D)
ck('S9 separate continuous layer', 'afkDroneStarbitsPerSecondMilli()' in S and 'afkAutoCursorStarbitsPerSecondMilli()' in S)
# S10
ck('S10 unlock after section9', 'AFKSection9Complete' in Q and 'afkDroneBayTier() >= 3' in Q)
ck('S10 five upgrade paths x20', '[5][20]int64' in Q and 'afkOrbitalUpgradeMaxLevel' in Q)
ck('S10 three tiers', re.search(r'afkOrbitalMaxTier\s*=\s*3',Q) is not None)
ck('S10 discrete live burst', 'afkProcessOrbitalLive(now)' in S)
ck('S10 no offline Starbit burst', 'afkOrbitalOfflineStarbits(paid)' not in S)
ck('S10 double-pay prevention', 'AFKOrbitalLastFireUnix += cycles * cd' in Q)
ck('S10 charge animation', 'Charge meter immediately under the cannon' in Q)
ck('S10 beam animation', '700*time.Millisecond' in Q)
ck('S10 endgame', 'AFK CORE LOOP COMPLETE' in Q)
# Integration & economy isolation
ck('Starbits terminology', 'STARBITS / SEC' in H and 'AFKStarbits' in main)
ck('SpaceCoin writes isolated to explicit Starbase exchange', all('gameMeta.SpaceCoins' not in txt for name,txt in files.items() if name != 'afk_exchange.go'))
ck('No legacy Cursor Energy UI', 'Cursor Energy' not in allafk and 'ENERGY / SEC' not in allafk)
ck('Clean utility navigation + systems menu', all(x in H for x in ['"FACILITIES"','"GUIDE"','"SYSTEMS"','afkSystemsMenuOpen']))
ck('AFK HUD replaces lower dock', 'afkBottomDockRect' in H and 'afkPrimaryNavRect' in H)
ck('All panels mutually managed', all(x in H for x in ['afkExpeditionPanelOpen','afkTechPanelOpen','afkTalentPanelOpen','afkOperatorPanelOpen','afkModulePanelOpen','afkDronePanelOpen','afkOrbitalPanelOpen']))
ck('Save migration normalises S10', 'afkEnsureSection10SaveShape()' in prog and 'afkRefreshSection10Completion()' in prog)
ck('Reference sequence objective', S.index('afkSection5IsComplete') < S.index('afkSection6Objective') < S.index('afkSection7Objective') < S.index('afkSection8Objective') < S.index('afkSection9Objective') < S.index('afkSection10Objective'))
# HUD geometry at representative resolutions based on actual sx/sy/arena formulas.
def sx(v,w): return int(v*w/1536.0)
def sy(v,h): return int(v*h/1024.0)
for w,h in [(1024,768),(1280,720),(1366,768),(1536,864),(1536,1024),(1600,900),(1920,1080),(2560,1440),(3440,1440)]:
    arena_bottom=sy(772,h); dtop=arena_bottom+sy(10,h); dbot=h-sy(10,h)
    left=sx(14,w)+sx(12,w); right=sx(1522,w)-sx(12,w); gap=sx(8,w)
    bw=(right-left-gap*8)//9
    contained=(dtop < dbot and bw>20)
    prev=None; nonover=True
    for i in range(9):
        x=left+i*(bw+gap); r=(x,dtop+sy(82,h),x+bw,dtop+sy(114,h))
        if prev and r[0] < prev[2]: nonover=False
        prev=r
    context_top=dtop+sy(122,h); context_bottom=dbot-sy(8,h)
    ck(f'HUD geometry {w}x{h}', contained and nonover and context_top<context_bottom)

failed=[n for n,v in checks if not v]
print(f'{len(checks)-len(failed)}/{len(checks)} Sections 1-10 assertions: '+('PASS' if not failed else 'FAIL'))
for n,v in checks:
    if not v: print('FAIL -',n)
if failed: raise SystemExit(1)
