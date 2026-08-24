from pathlib import Path
R=Path(__file__).resolve().parents[1]
t=(R/'afk_research_talents.go').read_text(errors='ignore')
a=(R/'afk_research_assets.go').read_text(errors='ignore')
m=(R/'main.go').read_text(errors='ignore')
b=(R/'build_identity.go').read_text(errors='ignore')
checks=[]
def ck(name, cond):
    checks.append((name,bool(cond))); print(('PASS' if cond else 'FAIL'), name)
ck('v438 identity', 'clientBuildVersion = "v438"' in b)
ck('2d world size', 'sx(2400, w), sy(1600, hgt)' in t)
ck('vertical branch spread', '{600, 1160}' in t and '{1510, 1280}' in t and '{2010, 1320}' in t)
ck('background loaded', 'pilot_network_background.bgra' in a and 'afkPilotNetworkBackground' in a)
ck('background moves with pan x', 'vp.Left + afkTalentPanX' in t)
ck('background moves with pan y', 'vp.Top + afkTalentPanY' in t)
ck('2d pan updates x', 'afkTalentPanX += x - afkTalentPanLastX' in m)
ck('2d pan updates y', 'afkTalentPanY += y - afkTalentPanLastY' in m)
ck('2d clamp y', 'afkTalentPanY < minY' in t)
ck('windows cursor helper', 'func shouldShowWindowsCursor' in m)
ck('overlay is interface cursor', 'overlayMode != OverlayNone' in m)
ck('hud outside arena cursor', 'return !pointInArena(p, w, hgt)' in m)
ck('setcursor uses helper', 'shouldShowWindowsCursor(h, p, w, hgt)' in m)
asset=R/'assets/ui/pilot_skills/pilot_network_background.bgra'
ck('background asset exact size', asset.exists() and asset.stat().st_size==2400*1600*4)
icons=list((R/'assets/ui/skill_tree_icons').glob('*.bgra'))
ck('all 29 skill assets retained', len(icons)==29)
print(f"\n{sum(v for _,v in checks)}/{len(checks)} v438 assertions passed")
raise SystemExit(0 if all(v for _,v in checks) else 1)
