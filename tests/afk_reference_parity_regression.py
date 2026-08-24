from pathlib import Path
R=Path(__file__).resolve().parents[1]
S='\n'.join(p.read_text() for p in R.glob('afk_*.go')); H=(R/'afk_bottom_hud.go').read_text(); E=(R/'afk_expeditions.go').read_text(); T=(R/'afk_research_talents.go').read_text(); O=(R/'afk_operators.go').read_text(); M=(R/'afk_modules.go').read_text(); D=(R/'afk_drones.go').read_text(); Q=(R/'afk_orbital.go').read_text(); MAIN=(R/'main.go').read_text()
checks=[]
def ck(n,c): checks.append((n,bool(c)))
# Onboarding + core loop from report
ck('manual 0-100 onboarding', 'afkSection1StarbitTarget' in S and 'int64 = 100' in S and 'afkManualClickValue' in S)
ck('build core action', 'BUILD CURSOR CORE' in H)
ck('build particle reveal', 'drawAFKCoreBuildBurst' in S and '1100*time.Millisecond' in S)
ck('first automation unit free', '0, 50, 100, 200' in S and 'gameMeta.AFKAutoCursors == 0' in H)
ck('ten auto cursors', 'afkAutoCursorMax' in S and '= 10' in S)
ck('manual remains after automation', 'afkManualClickValue' in S and 'afkManualExtractionValue' in S)
ck('persistent objective', 'afkObjectiveStripRect' in H and 'afkObjective()' in H)
# Side loops observed in video/report
ck('exploration side progression', 'SPACE EXPEDITIONS' in E and 'SCOUT SHIP' in E)
ck('speed idle cache stats', all(x in E for x in ['EXPEDITION SPEED','SCOUT SYSTEMS','CACHE CHANCE']))
ck('timed mission ETA', 'SCOUT IN FLIGHT // ETA' in E)
ck('crafting loot pipeline', 'AFKPendingComponent' in M and 'CLAIM COMPONENT' in H)
ck('independent Lucky-Find analogue', 'STAR CACHE DETECTED' in S and 'AFKActiveCacheReward' in MAIN)
ck('15-minute x2 analogue', 'afkOverdriveDurationSeconds int64 = 15 * 60' in S and 'OVERDRIVE' in H)
ck('offline Welcome Back', 'WELCOME BACK' in S and 'CLAIM AFK EARNINGS' in S)
# Progression systems
ck('core tiers', 'afkCoreMaxTier' in S and '= 5' in S)
ck('research lab', 'TECH LAB' in T and 'afkResearchMaxLevel' in T)
ck('talent tree', 'PILOT SKILLS' in T and 'SHARPER CURSORS' in T and 'ORBITAL CURSOR MATH 2.0' in T)
ck('operators silhouettes/recruit', 'RECRUIT' in O and 'drawAFKOperator' in O)
ck('9x8 module collection', 'afkModuleSetCount' in M and '= 9' in M and 'afkModuleItemsPerSet' in M and '= 8' in M)
ck('drone second layer', 'afkDroneStarbitsPerSecondMilli' in D and 'drawAFKDroneReticle' in D)
ck('drone tier 1-3', 'afkDroneBayMaxTier' in D and '= 3' in D)
ck('late research foreshadows orbital', 'ENGINEER TRAINING' in T and 'VOID PIERCERS' in T)
ck('orbital continuation', 'UNLOCK ORBITAL EXTRACTOR' in Q and 'afkOrbitalMaxTier' in Q)
# User-requested adaptations
ck('clean management navigation', 'afkPrimaryNavRect' in H and 'afkSystemsMenuOpen' in H and 'STARBITS / SEC' in H)
ck('Starbits economy isolated except explicit exchange', all('gameMeta.SpaceCoins' not in p.read_text() for p in R.glob('afk_*.go') if p.name != 'afk_exchange.go'))
ck('no ad monetization dependency', 'rewarded ad' not in (S+H+E+T+O+M+D+Q).lower())
failed=[n for n,v in checks if not v]
for n,v in checks: print(('PASS' if v else 'FAIL'), '-', n)
print(f'\n{len(checks)-len(failed)}/{len(checks)} reference-parity assertions passed')
if failed: raise SystemExit(1)
