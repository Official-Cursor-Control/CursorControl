from pathlib import Path
import re
root=Path(__file__).resolve().parents[1]
exp=(root/'afk_expeditions.go').read_text()
main=(root/'main.go').read_text()
checks=[]
def ck(name,cond):
    checks.append((name,bool(cond)))
    print(('PASS' if cond else 'FAIL')+' - '+name)
ck('expedition mission grid uses two columns', 'col := index % 2' in exp and 'row := index / 2' in exp)
ck('expedition mission cards use a clipped scrollable tall-card layout', 'afkExpeditionMissionCardHeight' in exp and 'afkExpeditionMissionViewportRect' in exp and 'afkExpeditionScrollbarRects' in exp)
ck('mission cards reserve inner text width', 'innerW := r.Right - r.Left' in exp)
ck('mission names are ellipsised safely', 'fitTextEllipsis(hdc, hudTinyFont, d.Name, innerW)' in exp)
ck('mission subtitles are ellipsised safely', 'fitTextEllipsis(hdc, hudTinyFont, d.Subtitle, innerW)' in exp)
ck('mission stats include Star Cache chance', 'STAR CACHE %d%%' in exp)
ck('paid mission line shows explicit Starbit cost', 'COST %s STARBITS // REWARD %s' in exp)
ck('free mission line identifies free launch', 'FREE LAUNCH // REWARD %s STARBITS' in exp)
ck('utility layout has overlap detector', 'func hudUtilityLayoutConflicts' in main)
ck('bad utility layout automatically repairs', 'if hudUtilityLayoutConflicts(cfg)' in main)
ck('default bug button sits below profile row', 'Bug:        HUDLayoutRect{1324, 862, 1514, 908}' in main)
ck('default support button sits below bug', 'SupportDev: HUDLayoutRect{1324, 914, 1514, 960}' in main)
ck('default discord button stays inside design height', 'Discord:    HUDLayoutRect{1324, 966, 1514, 1012}' in main)
passed=sum(x for _,x in checks)
print(f'\n{passed}/{len(checks)} v426 responsive Starbase UI assertions passed')
raise SystemExit(0 if passed==len(checks) else 1)
