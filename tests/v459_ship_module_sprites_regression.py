from pathlib import Path
R=Path(__file__).resolve().parents[1]
A=(R/'afk_module_assets.go').read_text()
M=(R/'afk_modules.go').read_text()
MAIN=(R/'main.go').read_text()
assets=R/'assets'/'ui'/'ship_modules'
checks={
    '72 module sprite declarations': A.count('{file: "') == 72,
    '72 module BGRA assets': len(list(assets.glob('*.bgra'))) == 72,
    '72 source PNG sprites': len(list(assets.glob('*.png'))) == 72,
    'module asset loader wired into initUI': 'loadAFKModuleAssets()' in MAIN,
    'module cards render authored sprites': 'drawAFKModuleSprite(hdc, art, set, item, crafted)' in M,
    'nine module sets unchanged': 'afkModuleSetCount      = 9' in M,
    'eight pieces per set unchanged': 'afkModuleItemsPerSet   = 8' in M,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ' - ' + name)
if not all(checks.values()):
    raise SystemExit(1)
