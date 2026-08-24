from pathlib import Path

root = Path(__file__).resolve().parents[1]
boss = (root / "survival_boss2.go").read_text(encoding="utf-8")
audio = (root / "audio_runtime.go").read_text(encoding="utf-8")

# Five hits remain the destruction contract.
assert "n.Hits++" in boss
assert "if n.Hits < 5" in boss
assert "playSurvivalBoss2HuntHitSound(n.Hits)" in boss
assert "playHitSound()" not in boss[boss.index("n.Hits++"):boss.index("if n.Hits < 5")], "Generic hit sound must not layer over combo cues"

# Remaining-hit flash is 4 -> 3 -> 2 -> 1 and never zero.
assert "FlashRemaining int" in boss
assert "FlashUntil     time.Time" in boss
assert "n.FlashRemaining = 5 - n.Hits" in boss
assert "now.Add(460 * time.Millisecond)" in boss
assert "n.FlashRemaining = 0" in boss
assert "rgb(255, 78, 78)" in boss and "rgb(215, 35, 45)" in boss, "Counter should flash red"

# All five supplied combo cues must be on the one-shot SFX bus.
for i in range(1, 6):
    assert f'"boss2_hunt_hit_{i}":' in audio
    assert f'"sboss2_hit_{i}.wav"' in audio
    wav = root / "assets" / "audio" / f"sboss2_hit_{i}.wav"
    assert wav.exists() and wav.stat().st_size > 10_000, f"Missing converted hit {i} audio asset"

print("v460 Void Serpent Hunt combo feedback regression: PASS")
