from pathlib import Path
import re
R=Path(__file__).resolve().parents[1]
M=(R/'afk_modules.go').read_text(errors='ignore')
E=(R/'afk_expeditions.go').read_text(errors='ignore')
A=(R/'afk_material_assets.go').read_text(errors='ignore')
G=(R/'main.go').read_text(errors='ignore')
B=(R/'build_identity.go').read_text(errors='ignore')
assets=R/'assets'/'ui'/'ship_materials'
expected={
 'pixel_wire.bgra':(234,279),
 'star_alloy.bgra':(208,196),
 'power_cell.bgra':(133,270),
 'quantum_glass.bgra':(218,263),
 'void_crystal.bgra':(181,257),
 'nova_core.bgra':(229,249),
}
checks=[]
def ck(name, cond): checks.append((name,bool(cond)))
ck('v461 identity', 'clientBuildVersion = "v461"' in B)
ck('six material sprites declared', A.count('{file:') == 6)
ck('material loader wired', 'loadAFKCraftMaterialAssets()' in G)
ck('module material inventory strip', 'drawAFKModuleMaterialStrip' in M and 'drawAFKCraftMaterialSprite' in M)
ck('exact expedition material preview', 'afkPendingExpeditionCraftComponent' in M and 'afkDeterministicCraftDrop' in M)
ck('large expedition reward popup', 'drawAFKExpeditionRewardPopup' in E and 'EXPEDITION COMPLETE' in E)
ck('reward popup shows all reward categories', all(x in E for x in ['STARBITS','NAV DATA','SHIP MATERIAL','CLAIM ALL REWARDS']))
ck('material collected with expedition claim', 'afkAwardPendingCraftComponent()' in E and 'afkCollectPendingCraftComponent()' in E)
ck('reward popup is modal', 'Reward popup is modal' in E and 'afkExpeditionRewardPopupClaimRect' in E)
for name,(w,h) in expected.items():
    p=assets/name
    ck(f'{name} exact BGRA size', p.exists() and p.stat().st_size==w*h*4)
    ck(f'{name} source PNG present', (assets/name.replace('.bgra','.png')).exists())
failed=[n for n,v in checks if not v]
print(f'{len(checks)-len(failed)}/{len(checks)} v461 material/reward assertions passed')
for n,v in checks:
    if not v: print('FAIL -',n)
if failed: raise SystemExit(1)
