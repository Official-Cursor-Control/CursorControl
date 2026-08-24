# Cursor Control v422 Codebase Audit

## Scope
- 63 top-level Windows Go source files reviewed by formatting/build/vet/static and regression tooling.
- 40 Python regression/stress/audit scripts executed against the current intended design.
- Full runtime asset/audio/UI QA executed against the packaged v422 runtime.

## New v422 behaviour verified
- Survival revive UX: right-side decision panel, remaining revives text, full Station HP, Nova Bomb clear, exactly 3 seconds immunity, click elsewhere in arena opens report immediately.
- Endurance ships now provide Space Expedition passives.
- Equipped Endurance ship is snapshotted on expedition launch so mid-flight ship changes do not alter the active mission.
- Passive effects apply to Starbit reward, NAV Data reward, expedition duration, and/or Space Cache chance depending on ship.
- Expedition UI displays selected/locked ship passive and launch preview uses modified rewards.

## Automated audit results
- Windows amd64 release build: PASS.
- Deterministic release build x3: PASS.
- gofmt: PASS.
- go vet: PASS with only 11 known Win32/Direct2D unsafe.Pointer interop warnings; no unexpected vet diagnostics.
- Current regression/stress scripts: 40/40 PASS.
- Full runtime QA: 656/656 PASS.
- Security/cloud regression: PASS.
- Starbase UI/cloud stress: PASS.
- Survival revive regression: PASS.
- Boss 1/2/3 and Sector 3 stress suites: PASS.
- Profile/achievement conflict audit: PASS.
- Architecture reconciliation: PASS.
- Runtime ZIP hygiene/assets/audio decode/button wiring/British English audit: PASS.

## Static safety review
- No explicit panic() calls in gameplay/source code.
- No log.Fatal/Fatalf calls.
- No TODO/FIXME/HACK markers left in top-level Go source.
- Profile synchronization remains coalesced through one buffered worker.
- Survival revive network completion returns through the main Windows message thread before mutating gameplay state.
- Starbase single-device cloud/session regression remains passing.

## QA maintenance
Several historical tests still asserted superseded design values (old Starbase core prices/multipliers, old navigation layout, 18-second announcements, and older Void Serpent timers). Those tests were updated to verify the current intentional design rather than obsolete historical constants.
