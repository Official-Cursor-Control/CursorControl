from pathlib import Path

root = Path(__file__).resolve().parents[1]
allgo = "\n".join(p.read_text(errors="ignore") for p in root.glob("*.go"))

def ck(name, cond):
    if not cond:
        raise AssertionError(name)
    print("PASS", name)

build = (root / "build_identity.go").read_text()
ops = (root / "afk_operators.go").read_text()
assets = (root / "afk_operator_assets.go").read_text()
quips = (root / "afk_operator_quips.go").read_text()
audio = (root / "audio_runtime.go").read_text()
research_assets = (root / "afk_research_assets.go").read_text()

ck("v440 identity", 'clientBuildVersion = "v440"' in build)
ck("operator art loader", 'loadAFKOperatorAssets()' in allgo and 'assets/ui/operators' not in assets)
ck("five supplied operator sprites", all(name in assets for name in ['"nova"', '"vega"', '"byte"', '"astra"', '"flux"']))
ck("operator card sprite draw", 'drawAFKOperatorSprite(hdc, artBox, i, recruited)' in ops)
ck("unrecruited uses real dark silhouette", 'makeAFKOperatorLockedBGRA' in assets and 'data = s.locked' in assets)
ck("level threshold triggers quip", 'if afterLevel > beforeLevel' in ops and 'afkQueueOperatorLevelQuip(i, afterLevel)' in ops)
ck("quip playback serialized", 'afkOperatorQuipPlaybackMu' in quips)
ck("all 15 quips registered", sum(audio.count(f'afk_operator_{who}_quip_') for who in ['nova','vega','byte','astra','flux']) == 15)
ck("pilot network world background still loaded", 'pilot_network_background.bgra' in research_assets)
for who in ['nova','vega','byte','astra','flux']:
    ck(f"{who} bgra exists", (root / 'assets' / 'ui' / 'operators' / f'{who}.bgra').stat().st_size == 512*896*4)
for who in ['nova','vega','byte','astra','flux']:
    for n in range(1,4):
        ck(f"{who} quip {n}", (root / 'assets' / 'audio' / f'operator_{who}_quip_{n}.wav').exists())
ck("skill background exact world size", (root / 'assets' / 'ui' / 'pilot_skills' / 'pilot_network_background.bgra').stat().st_size == 2400*1600*4)
