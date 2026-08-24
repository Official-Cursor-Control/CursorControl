# Cursor Control v315 Stability / Stress Pass

Basis: v314 CACHE_HUD_POSITION.

## Runtime changes

Only redundant outer goroutine wrappers were removed from Survival sound call sites where the central `playSFX` bus already dispatches asynchronously. This avoids creating two goroutines for one sound event. Boss 1 roar calls intentionally keep their outer goroutine because the lazy-load fallback can perform disk I/O.

No gameplay geometry, collision, pathing, boss timing, damage, reward, progression, save, network, or UI layout rules were changed.

## Test results

- `gofmt`: clean.
- Windows amd64 GUI cross-build: 5/5 successful and byte-identical SHA-256 outputs.
- `go vet`: no unexpected diagnostics; only the existing Win32 `unsafe.Pointer` interop warnings.
- Full automated regression harness: 394/394 PASS.
- Boss 2 combo stress: 60,000 combo groups + 100,000 lifecycle cases per pass, no failures.
- Boss 2 meteor stress: 100,000 trials per pass, no wall/click/round failures.
- Boss 2 re-entry stress: 6,000 runs per pass across 60/120/144 FPS and four entry sides; all re-entered and telegraphed.
- Boss 2 clarity stress: 250,000 collision/render cases + 250,000 off-screen-node cases + 100,000 colour-sequence cases per pass, zero failures.
- The Boss 2 suite was repeated multiple times during the audit, totalling several million simulated state/geometry cases.
- Audio: all 34 audio files completed three independent full FFmpeg decode passes (102/102 successful).
- Asset stability: all 209 runtime assets completed four additional SHA-256 reread passes (836/836 stable reads, zero mismatches), in addition to the full QA harness checks.
- Button hit-test primitive: 250,000 randomized rectangle/point cases passed.
- Runtime root hygiene: executable + `assets/` + `data/` only.

## QA tooling repair

`tests/qa_all.py` was still hard-coded to v300 paths and an obsolete assumption that wrappers must contain `playOneShotAsync`. It is now version-agnostic for the runtime executable and recognizes direct routing to the centralized asynchronous `playSFX` bus.
