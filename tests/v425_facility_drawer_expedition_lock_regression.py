from pathlib import Path
root=Path(__file__).resolve().parents[1]
hub=(root/'afk_starbase_hub.go').read_text()
exp=(root/'afk_expeditions.go').read_text()
main=(root/'main.go').read_text()
checks={
'facility drawer selection':'afkSelectFacilityDrawer' in hub,
'same facility closes':'afkFacilityDrawerIndex == i' in hub and 'afkFacilityDrawerPhase = 3' in hub,
'switch closes then opens':'afkFacilityDrawerPending = i' in hub and 'afkFacilityDrawerPhase = 2' in hub,
'drawer shows current tier':'afkFacilityTierLabel' in hub,
'drawer shows purpose':'afkFacilityPurpose' in hub,
'drawer shows current effect':'afkFacilityCurrentEffect' in hub,
'drawer shows next upgrade':'afkFacilityNextUpgrade' in hub,
'drawer upgrade action':'afkUpgradeFacilityFromDrawer' in hub,
'facility click no immediate management':'afkSelectFacilityDrawer(worldIndex)' in hub,
'expedition fire colour persisted':'AFKExpeditionFireColorID' in main,
'expedition fire size persisted':'AFKExpeditionFireSizeID' in main,
'launch snapshots colour':'AFKExpeditionFireColorID = gameMeta.SelectedFireColor' in exp,
'launch snapshots size':'AFKExpeditionFireSizeID = gameMeta.SelectedFireSize' in exp,
'in flight colour uses snapshot':'id = gameMeta.AFKExpeditionFireColorID' in exp,
'in flight size uses snapshot':'id = gameMeta.AFKExpeditionFireSizeID' in exp,
'active ship remains snapshot':'gameMeta.AFKExpeditionShipID' in exp,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL'),k)
if failed: raise SystemExit(1)
print(f'PASS v425 facility/expedition lock {len(checks)}/{len(checks)}')
