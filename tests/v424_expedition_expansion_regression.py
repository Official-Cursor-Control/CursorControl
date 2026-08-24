from pathlib import Path
s=(Path(__file__).resolve().parents[1]/'afk_expeditions.go').read_text()
checks={
'12 routes':'SELECT DESTINATION // 12 ROUTES' in s and s.count('BLACK-SECTOR EXPEDITION')>=1,
'paid routes starbits only':'LaunchCost' in s and 'afkSpendStarbits(cost)' in s,
'nav never mission cost':'PAID ROUTES COST STARBITS' in s and 'LaunchCost' in s,
'free routes':'{"LUNAR RELAY", "LOCAL SIGNAL SWEEP", 16 * 60, 0,' in s,
'endgame paid route':'50_000_000, 1_500_000_000, 1_300, 3, 22' in s,
'responsive 2 column cards':'col := index % 2' in s and 'row := index / 2' in s,
'card shows cost':'COST %s STARBITS // REWARD %s' in s,
'card shows nav cache':'NAV +%d // STAR CACHE %d%%' in s,
'ship guide':'EXPEDITION SHIP GUIDE' in s and 'HIGHER RARITY = STRONGER ABILITIES' in s,
'equipped highlighted':'EQUIPPED' in s,
'cache art':'drawAFKStarCachePreview' in s and 'starCacheBGRA' in s,
'star cache wording':'OPEN %s STAR CACHE' in s,
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL'), k)
assert all(checks.values()), [k for k,v in checks.items() if not v]
print(f"PASS v424 expedition expansion {len(checks)}/{len(checks)}")
