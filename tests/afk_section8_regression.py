from pathlib import Path
R=Path(__file__).resolve().parents[1]
M=(R/'afk_modules.go').read_text()
H=(R/'afk_bottom_hud.go').read_text()
S=(R/'afk_singularity.go').read_text()
E=(R/'afk_expeditions.go').read_text()
G=(R/'main.go').read_text()
checks={
'9 module sets':'afkModuleSetCount      = 9' in M,
'8 pieces per set':'afkModuleItemsPerSet   = 8' in M,
'72-item collection':'afkModuleItemCount     = afkModuleSetCount * afkModuleItemsPerSet' in M,
'6 craft components':'afkCraftComponentCount = 6' in M,
'full hard-coded recipe table':'var afkModuleRecipes = [afkModuleItemCount][afkCraftComponentCount]int{' in M,
'Section 8 persisted':'AFKSection8Complete' in G and 'AFKEquipmentCrafted' in G and 'AFKCraftComponents' in G,
'Modules HUD tab':'"MODULES"' in H,
'Module arena renderer':'drawAFKModules(hdc, w, hgt)' in S,
'Expeditions grant components':'afkAwardPendingCraftComponent()' in E,
'Component reward popup':'CONGRATS!' in E and 'CRAFTING COMPONENT' in E,
'All sets 500 Nav Data reward':'x500 NAV DATA' in H,
'Tier 5 gated behind Section 8':'return gameMeta.AFKSection8Complete' in H,
'No SpaceCoin use in module system':'SpaceCoins' not in M and 'SpaceCoin' not in M,
'All 9 named set defs': all(x in M for x in ['OPERATOR LOCKER','STARBIT GENERATOR','TECH LAB','HYDRO BAY','STAR LOUNGE','QUANTUM ARCADE','EXTRACTION DEPOT','VOID VAULT','ORBITAL ARSENAL']),
}
for k,v in checks.items():
    if not v: raise AssertionError(k)
print(f"PASS: {len(checks)}/{len(checks)} Section 8 assertions")
