from pathlib import Path
D=Path('afk_drones.go').read_text(); S=Path('afk_singularity.go').read_text(); H=Path('afk_bottom_hud.go').read_text(); M=Path('main.go').read_text(); R=Path('afk_research_talents.go').read_text()
checks={
'S9 persisted': all(x in M for x in ['AFKSection9Complete','AFKDroneBayBuilt','AFKDroneBayTier','AFKDronesDeployed','AFKDroneUpgradeLevels']),
'core5 section8 gate': 'afkDronesAvailable() bool { return afkCoreTier() >= 5 && gameMeta.AFKSection8Complete }' in D,
'10 drone hard cap': 'afkDroneMaxCount' in D and '= 10' in D,
'3 drone hub tiers': 'afkDroneBayMaxTier' in D and '= 3' in D,
'five upgrade paths': all(x in D for x in ['DEPLOY DRONE','BIGGER PROPELLERS','TARGETING AI','OVERCHARGED CORES','NEURAL SYNC']),
'explicit deploy costs': 'afkDroneDeployCosts' in D and '1_400_000_000_000' in D,
'explicit upgrade tables': 'var afkDroneUpgradeCosts = [4][40]int64' in D,
'ascension tiers': 'afkDroneBayAscendCosts' in D and 'afkAscendDroneBay' in D,
'drones contribute separately': 'saturatingAdd(afkAutoCursorStarbitsPerSecondMilli(), afkDroneStarbitsPerSecondMilli())' in S,
'flux connected': 'afkOperatorDroneMultiplierMilli()' in D,
'arsenal module connected': 'afkModuleDroneOrbitalMultiplierMilli()' in D,
'advanced drone research connected': 'afkAdvancedDroneResearchMultiplierMilli()' in D,
'drone reticles': 'drawAFKDroneReticle' in D and 'drawAFKDroneWorld' in D,
'drone panel': 'drawAFKDrones(hdc, w, hgt)' in S,
'bottom dock tab': '"DRONE BAY"' in H and 'afkDronePanelOpen' in H,
'section9 objective chain': 'return afkSection9Objective()' in S,
'deploy ten objective': 'DEPLOY 10 DRONES' in D,
'reference-like research extension': all(x in R for x in ['OVERCHARGED CORES','NEURAL SYNC','ENGINEER TRAINING','OMEGA PROTOCOL','VOID PIERCERS']),
'advanced research hidden pre-drone': 'if afkDroneBayBuilt()' in R and 'return 10' in R,
'no SpaceCoin economy in drone system': 'SpaceCoin' not in D,
}
failed=[k for k,v in checks.items() if not v]
if failed:
    print('FAIL:',len(failed),'/',len(checks)); [print(' -',x) for x in failed]; raise SystemExit(1)
print(f'PASS: {len(checks)}/{len(checks)} Section 9 assertions')
