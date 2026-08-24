from pathlib import Path
R=Path(__file__).resolve().parents[1]
W=(R/'afk_world_buildings.go').read_text(); S=(R/'afk_singularity.go').read_text()
checks=[]
def ck(n,c): checks.append((n,bool(c)))
ck('seven physical progression structures','afkWorldBuildingCount' in W and 'afkWorldOrbital' in W)
ck('build transition celebration','afkRefreshPhysicalBuildingTransitions' in W and 'afkQueueMilestone' in W)
ck('core tier physical transform','drawAFKCoreBuilding' in W and 'tier >= 4' in W)
ck('scout landing pad','drawAFKScoutPadWorld' in W and 'afkExpeditionActive' in W)
ck('tech lab physical scanner','drawAFKTechBuilding' in W and 'scan :=' in W)
ck('operations building','drawAFKOpsBuilding' in W)
ck('module fabricator','drawAFKFabricatorBuilding' in W)
ck('drone bay hangar','drawAFKDroneBayBuilding' in W and 'drawAFKDroneSprite' in W)
ck('orbital physical structure','drawAFKOrbitalBuilding' in W)
ck('world building click navigation','handleAFKPhysicalBuildingClick' in W and 'afkTechPanelOpen=true' in W.replace(' ',''))
ck('physical cache','drawAFKPhysicalCache' in W and 'STAR CACHE' in W)
ck('ambient base life','drawAFKWorldAmbientActivity' in W)
ck('welcome reward count-up','formatAFKCountUpReward' in W and 'formatAFKCountUpReward(gameMeta.AFKOfflinePendingStarbits)' in S)
ck('physical world integrated','drawAFKPhysicalStarbaseWorld' in S)
ck('legacy core plinth removed from draw order','drawAFKMoonRockForeground(hdc, w, hgt)\n\tdrawAFKPhysicalStarbaseWorld' in S)
for n,v in checks: print(('PASS' if v else 'FAIL'),'-',n)
failed=[n for n,v in checks if not v]
print(f'\n{len(checks)-len(failed)}/{len(checks)} physical Starbase assertions passed')
if failed: raise SystemExit(1)
