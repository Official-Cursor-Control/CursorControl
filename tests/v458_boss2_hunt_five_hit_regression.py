from pathlib import Path

src = (Path(__file__).resolve().parents[1] / "survival_boss2.go").read_text(encoding="utf-8")

assert "Hits" in src and "int" in src[src.index("type SurvivalBoss2HuntNode struct"):src.index("}", src.index("type SurvivalBoss2HuntNode struct"))], "Hunt nodes need their own persistent hit counter"
assert "n.Hits++" in src, "Each correct Hunt click must increment that node's hit count"
assert "if n.Hits < 5" in src, "Hunt node must survive the first four valid hits"
assert src.index("if n.Hits < 5") < src.index("n.Alive = false", src.index("if n.Hits < 5")), "Node must not die before the five-hit gate"
assert 'CORE %d HIT %d/5' in src, "HUD status should expose per-core hit progress"
assert "n.RestoreUntil = now.Add(2 * time.Second)" in src, "Existing two-second escape pocket must remain after destruction"
assert "survivalBoss2ComboIndex++" in src, "Destroyed-core count must still advance only when the core dies"

print("v458 Void Serpent Hunt five-hit regression: PASS")
