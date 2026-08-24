from pathlib import Path
root=Path(__file__).resolve().parents[1]
exp=(root/'afk_expeditions.go').read_text()
main=(root/'main.go').read_text()
asset=root/'assets/backgrounds/starbase_expedition_hangar.bgra'
checks=[]
def ck(name,cond):
    checks.append((name,bool(cond)))
    print(('PASS' if cond else 'FAIL')+' - '+name)
ck('hangar background asset present', asset.exists() and asset.stat().st_size==930*1692*4)
ck('hangar asset loaded by runtime', 'starbase_expedition_hangar.bgra' in main and 'expeditionHangarBGRA' in main)
ck('mission viewport clips content', 'intersectClipRect.Call(hdc, uintptr(missionViewport.Left)' in exp)
ck('mission cards are tall and scrollable', 'afkExpeditionMissionCardHeight' in exp and 'afkExpeditionScroll' in exp)
ck('visible scrollbar exists', 'afkExpeditionScrollbarRects' in exp and 'drawBevelPanel(hdc, thumb' in exp)
ck('mouse wheel scroll wired', 'handleAFKExpeditionWheel' in exp and 'handleAFKExpeditionWheel(hiword(wParam)' in main)
ck('scrollbar drag wired', 'afkExpeditionScrollDragging' in main and 'afkExpeditionScrollbarDragTo' in main)
ck('right controls moved below flight viewport', 'r.Top + sy(178, hgt)' in exp and 'r.Top + sy(214, hgt)' in exp)
ck('launch snapshots visual loadout', 'AFKExpeditionFireColorID = gameMeta.SelectedFireColor' in exp and 'AFKExpeditionFireSizeID = gameMeta.SelectedFireSize' in exp)
ck('launch begins vertical transition', 'afkExpeditionLaunchVisualAt = now' in exp and 'bottomY - int32(float64(bottomY-spaceY)*p)' in exp)
ck('launch sound waits for transition', 'now.Sub(afkExpeditionLaunchVisualAt) >= afkExpeditionLaunchTransition' in exp)
ck('flight background moves right to left', 'view.Left - off' in exp and 'view.Left - off + vw' in exp)
ck('ship remains separate from background', 'drawAFKExpeditionFlightBackground(hdc, w, hgt)' in exp and 'drawAFKScoutShip(hdc, right, w, hgt)' in exp)
ck('fire only appears after launch transition', 'func afkExpeditionFireVisible' in exp and 'return flying' in exp)
ck('return transition restores hangar', 'afkExpeditionReturnVisualAt = now' in exp and 'spaceY + int32(float64(bottomY-spaceY)*p)' in exp)
ck('launched ship stays locked through claim state', 'afkExpeditionRewardPending() || afkSpaceCachePending() || gameMeta.AFKPendingComponentCount > 0' in exp)
passed=sum(x for _,x in checks)
print(f'\n{passed}/{len(checks)} v427 Expedition hangar/scroll assertions passed')
raise SystemExit(0 if passed==len(checks) else 1)
