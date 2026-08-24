from pathlib import Path
files={p.name:p.read_text(errors='ignore') for p in Path('.').glob('*.go')}
alltext='\n'.join(files.values())
checks=[
('build is v432 or later', any(f'clientBuildVersion = "v{v}"' in alltext for v in range(432, 500))),
('starbase ESC returns through settings','starbaseSettingsReturn = true' in files['main.go'] and 'setOverlay(OverlaySettings)' in files['main.go']),
('settings returns to starbase','setOverlay(OverlayAFKSingularity)' in files['main.go'] and 'starbaseSettingsReturn = false' in files['main.go']),
('starbase draws under settings','overlayMode == OverlaySettings && starbaseSettingsReturn' in files['afk_singularity.go']),
('starbase timer survives settings','overlayMode == OverlaySettings && starbaseSettingsReturn' in files['main.go']),
('starbits sec popup exists','CURRENT STARBIT MULTIPLIERS' in files['afk_bottom_hud.go']),
('stat card removes misleading rank shorthand','{"STARBITS / SEC", afkStarbitsPerSecondText()' in files['afk_bottom_hud.go']),
('all stat cards consume clicks','for i := 0; i < 5; i++' in files['afk_bottom_hud.go']),
('global rank multiplier listed','GLOBAL RANK' in files['afk_bottom_hud.go']),
('overdrive multiplier listed','OVERDRIVE  //  x2.00' in files['afk_bottom_hud.go']),
('facility title includes tier','(TIER %d/%d)' in files['afk_starbase_hub.go']),
('requirement progress bars exist','drawAFKRequirementBar' in files['afk_starbase_hub.go']),
('progress red threshold','p < .33' in files['afk_starbase_hub.go']),
('progress orange threshold','p < .66' in files['afk_starbase_hub.go']),
('progress green on complete','cur >= target' in files['afk_starbase_hub.go']),
('scout NAV progress row','NAV DATA' in files['afk_starbase_hub.go'] and 'AFKNavigationData' in files['afk_starbase_hub.go']),
('scout expedition progress row','COMPLETED EXPEDITIONS' in files['afk_starbase_hub.go']),
('all built facilities hover including core','if pointInRect(cursorPos, r)' in files['afk_world_buildings.go']),
('click to manage conditional','if i != afkWorldCore' in files['afk_world_buildings.go']),
('revive starts frozen without fail animation','polishFailureStarted = time.Time{}' in files['survival.go']),
('revive hover repaints only panel','survivalRespawnPanelRect(w, hgt)' in files['main.go']),
('revive footer text','CLICK ANYWHERE TO CONTINUE' in files['survival_respawn.go']),
]
fail=0
for n,ok in checks:
 print(('PASS' if ok else 'FAIL')+' - '+n); fail+=not ok
print(f'\n{len(checks)-fail}/{len(checks)} v432 assertions passed')
raise SystemExit(1 if fail else 0)
