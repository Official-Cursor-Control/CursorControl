from pathlib import Path
import re
ROOT=Path(__file__).resolve().parents[1]
assets=(ROOT/'afk_research_assets.go').read_text()
ui=(ROOT/'afk_research_talents.go').read_text()
build=(ROOT/'build_identity.go').read_text()
checks=[]
def ck(name, cond):
    checks.append((name, bool(cond)))
icons=['void_cursor','blackstone_routing','snare_algorithm','singularity_pointer','stealth_pathing','pixel_compression_field','cursor_prism','overdrive_regulators','quantum_conduit','convergence']
ck('v436 identity', 'clientBuildVersion = "v436"' in build and 'clientBuildNumber  = 380' in build)
ck('ten tech icon mappings', all(f'"{x}"' in assets for x in icons))
ck('tech lab visible count fixed at ten', re.search(r'func afkVisibleResearchCount\(\) int \{.*?return 10\s*\}', ui, re.S))
ck('bottom full-width info drawer', 'infoH := sy(178, hgt)' in ui and 'p.Left + sx(18, w)' in ui and 'p.Right - sx(18, w)' in ui)
ck('left to right root', '{.08, .50}' in ui)
ck('rightward tier two', '{.31, .30}' in ui and '{.31, .72}' in ui)
ck('rightward later tiers', '{.55, .20}' in ui and '{.82, .20}' in ui and '{.61, .58}' in ui)
ck('info includes effect', '"EFFECT"' in ui)
ck('info includes description', '"DESCRIPTION"' in ui)
ck('info includes requirement', '"REQUIREMENT"' in ui)
ck('info includes cost', '"COST"' in ui)
for x in icons:
    ck(f'asset exists {x}', (ROOT/'assets/ui/tech_lab_icons'/f'{x}.bgra').exists())
failed=[n for n,v in checks if not v]
for n,v in checks: print(('PASS' if v else 'FAIL'), n)
print(f'\n{sum(v for _,v in checks)}/{len(checks)} v436 assertions passed')
raise SystemExit(1 if failed else 0)
