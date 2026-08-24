from pathlib import Path
import re
R=Path(__file__).resolve().parents[1]
M=(R/'afk_modules.go').read_text()
A=(R/'afk_module_assets.go').read_text()
H=(R/'afk_bottom_hud.go').read_text()
B=(R/'build_identity.go').read_text()
files=re.findall(r'\{file: "([^"]+\.bgra)"', A)
checks={
    'v465 identity': 'clientBuildVersion = "v465"' in B and 'clientBuildNumber  = 405' in B,
    '72 sprite slots': len(files) == 72,
    'display names are sprite-bound': 'func afkModuleDisplayName(set, item int) string' in M and 'afkModuleSprites[idx].file' in M,
    'card uses sprite-bound name': 'name := afkModuleDisplayName(s, i)' in M,
    'craft button uses same name': 'name := afkModuleDisplayName(s, i)' in H,
    'Ship Modules subtitle removed': 'drawAFKManagementChrome(hdc, p, "SHIP MODULES", "",' in M,
    'module art reduced to 76 percent': '* 0.76 / float64(cropW)' in A and '* 0.76 / float64(cropH)' in A,
    'module art never enlarged above native': 'if scale > 1.0' in A and 'scale = 1.0' in A,
    'save index remains stable': 'return set*afkModuleItemsPerSet + item' in M,
}
for n,ok in checks.items(): print(('PASS' if ok else 'FAIL')+' - '+n)
raise SystemExit(0 if all(checks.values()) else 1)
