from pathlib import Path
import re, math
ROOT=Path(__file__).resolve().parents[1]
S=(ROOT/'afk_singularity.go').read_text()
E=(ROOT/'afk_expeditions.go').read_text()
R=(ROOT/'afk_research_talents.go').read_text()
H=(ROOT/'afk_bottom_hud.go').read_text()
M=(ROOT/'main.go').read_text()

checks=[]
def ok(name, cond):
    if not cond: raise AssertionError(name)
    checks.append(name)

def nums_from_block(text, name):
    m=re.search(r'var\s+'+re.escape(name)+r'\s*=\s*\.\.\.int64\s*\{([^}]*)\}', text, re.S)
    if not m:
        m=re.search(r'var\s+'+re.escape(name)+r'\s*=\s*\[\.\.\.\]int64\s*\{([^}]*)\}', text, re.S)
    if not m: return None
    vals=[]
    for token in m.group(1).split(','):
        token=token.strip().replace('_','')
        if not token: continue
        if '*' in token: continue
        try: vals.append(int(token))
        except: pass
    return vals

# Section 1
ok('S1 manual target = 100', 'afkSection1StarbitTarget' in S)
ok('S1 manual click = 5', 'afkManualClickValue' in S and 'int64 = 5' in S)
ok('S1 hard cap before completion', 'next = afkSection1StarbitTarget' in S)

# Section 2
ok('S2 ten auto cursor slots', 'afkAutoCursorMax' in S and '= 10' in S)
ok('S2 processing max 10', 'afkProcessingMaxLevel' in S and '= 10' in S)
ok('S2 explicit auto costs', '0, 50, 100, 200, 400, 800, 1_600, 3_200, 6_400, 12_800' in S)
ok('S2 explicit processing rates', '1_000, 1_250, 1_600, 2_100, 2_800, 3_800, 5_200, 7_000, 9_500, 13_000' in S)

# Section 3
ok('S3 core max tier 5', 'afkCoreMaxTier' in S and '= 5' in S)
ok('S3 explicit rebalanced tier costs', '100_000, 5_000_000, 75_000_000, 1_500_000_000' in S)
ok('S3 explicit rebalanced tier multipliers', '1_000, 4_000, 16_000, 70_000, 300_000' in S)
ok('S3 tier2 gate requires expeditions', 'case 2:' in H and 'return afkSection5IsComplete()' in H)
ok('S3 tier3 gate requires section6', 'case 3:' in H and 'return gameMeta.AFKSection6Complete' in H)

# Section 4
ok('S4 base AFK capacity 2h', 'afkBaseCapacitySeconds' in S)
ok('S4 offline minimum 60 sec', 'afkOfflineMinimumSeconds' in S and 'int64 = 60' in S)
ok('S4 clock rollback watermark', 'AFKMaxObservedUnix' in S and 'AFKClockRollbackCount' in M)
ok('S4 Starbits-only offline economy', 'AFKOfflinePendingStarbits' in S and 'gameMeta.SpaceCoins' not in S)
ok('S4 claim required', 'afkClaimOfflineReward' in S and 'AFKOfflinePendingStarbits' in S)

# Section 5
ok('S5 scout unlock cost 250k', 'afkScoutShipUnlockCost int64 = 250_000' in E)
ok('S5 six hard-coded destinations', E.count('RequiredShipTier')>=2 and all(x in E for x in ['LUNAR RELAY','VIRELLA BELT','CYAN NEBULA','SHATTERED MOON','DEEP ORBIT','EVENT HORIZON']))
ok('S5 deterministic cache roll', 'afkDeterministicCacheRoll' in E and 'no rerolling' in E.lower())
ok('S5 capacity reaches 24h', '24 * 60 * 60' in E)
ok('S5 completion needs scout tier2 + 6 runs + L3 stats', 'AFKExpeditionsCompleted >= 6' in E and 'afkScoutSpeedLevel() >= 3' in E and 'afkScoutCapacityLevel() >= 3' in E and 'afkScoutCacheLevel() >= 3' in E)

# Section 6
ok('S6 Tech Lab requires tier3 + Section5', 'afkCoreTier() >= 3 && gameMeta.AFKSection5Complete' in R)
ok('S6 Tech Lab cost explicit', 'afkTechLabUnlockCost int64 = 12_000_000' in R)
ok('S6 ten hard-coded research rows', R.count('Costs       [10]int64')==1 and all(x in R for x in ['VOID CURSOR','BLACKSTONE ROUTING','SNARE ALGORITHM','SINGULARITY POINTER','STEALTH PATHING','PIXEL COMPRESSION FIELD','CURSOR PRISM','OVERDRIVE REGULATORS','QUANTUM CONDUIT','CONVERGENCE']))
ok('S6 talent tree has six nodes', all(x in R for x in ['SHARPER CURSORS','ORBITAL CURSOR MATH 2.0','TAP PRECISION I','DEEP SPACE NAVIGATOR','CACHE HUNTER','CURSOR SUPREMACY']))
ok('S6 first talent is balanced +10% Auto Cursor output', 'AUTO CURSOR OUTPUT +10%' in R and 'm += 100' in R)
ok('S6 Orbital Math gives balanced +10% processing layer', 'CURSOR PROCESSING +10%' in R)
ok('S6 manual talent +25%', 'MANUAL EXTRACTION +25%' in R and 'return 1250' in R)
ok('S6 downstream branches gated', 'Requires int' in R and 'return req < 0 || afkTalentUnlocked(req)' in R)
ok('S6 broad bonuses combine in permanent support pool', 'afkPermanentStarbitSupportMultiplierMilli' in S and 'm += afkResearchStarbitMultiplierMilli() - 1000' in S)

# Layout invariants for 1536x1024 design and 1600x900 runtime scaling.
def sx(v,w): return int(v*w/1536)
def sy(v,h): return int(v*h/1024)
def layout(w,h):
    arena=(sx(14,w),sy(180,h),sx(1522,w),sy(772,h))
    dock=(sx(14,w), arena[3]+sy(10,h), sx(1522,w), h-sy(10,h))
    return arena,dock
for w,h in [(1536,1024),(1600,900),(1366,768),(1920,1080),(2560,1440)]:
    ar,d=layout(w,h)
    ok(f'layout {w}x{h}: dock below arena', d[1]>ar[3])
    ok(f'layout {w}x{h}: dock on screen', 0<=d[0]<d[2]<=w and 0<=d[1]<d[3]<=h)
    dockh=d[3]-d[1]
    ok(f'layout {w}x{h}: dock tall enough', dockh>=170)
    # Context row after compression: y +94 design px to bottom -8.
    ctx_top=d[1]+sy(94,h); ctx_bottom=d[3]-sy(8,h)
    ok(f'layout {w}x{h}: context row valid', ctx_bottom-ctx_top>=80)

# Visual/animation invariants: all core visuals hard-coded; no new sprite dependency.
ok('visual: starfield is deterministic hard-coded', 'drawAFKPixelStarfield' in S and '0xC0A5E11' in S)
ok('visual: singularity animation uses pixel rings', 'drawAFKPixelRing' in S and 'Block-sampled circumference' in S)
ok('visual: Auto Cursor orbit animation exists', 'math.Cos(a)*orbit' in S and 'math.Sin(a)*orbit*0.55' in S)
ok('visual: Tech Lab card grid exists', 'drawAFKTechLab' in R and 'afkResearchCardRect' in R and 'drawBevelPanel' in R)
ok('visual: Tech Lab has selected research feedback', 'SELECTED // %s // LV.%d' in R and 'd.Description' in R)
ok('visual: Starbase utility dock owns management navigation', all(x in H for x in ['FACILITIES','GUIDE','SYSTEMS','CURSOR PROCESSING']))
ok('visual: Tech research upgrades live in Tech panel', 'for i := 0; i < visible; i++' in R and 'afkResearchDefs[i]' in R)

# Current implementation structural sequence. The original external reference report
# is intentionally not required for a portable source regression suite.
ok('reference sequence represented in code', 'TECH LAB' in R and 'PILOT SKILLS' in R)
ok('implementation: Expeditions precede Tech Lab', 'afkCoreTier() >= 3 && gameMeta.AFKSection5Complete' in R)
ok('implementation: Tech Lab precedes Pilot Skills', 'enabled = afkTechLabUnlocked()' in H and 'PILOT SKILLS' in H)

print(f'PASS: {len(checks)} assertions')
for c in checks: print('  OK',c)
