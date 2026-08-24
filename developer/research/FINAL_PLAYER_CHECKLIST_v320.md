# Cursor Control v320 — player verification checklist

Use this after the automated gates. Test naturally first; avoid developer commands until the normal-player pass is finished.

## 1. First-session / onboarding
- Start with a fresh local profile and confirm the first useful goal is obvious.
- Complete the tutorial without verbal coaching; note any unclear instruction.
- Confirm the first achievement and early EXP progress appear quickly enough to feel rewarding.
- Switch between Precision, Endurance and Survival and confirm each mode is discoverable.

## 2. Profile / identity
- Open your profile and verify avatar, name colour, EXP rank/badge, title, frame, mode statistics, equipped ship and achievement count.
- Cycle name colour/title/frame and verify persistence after restart.
- Select all three achievement showcase slots and verify they remain contained inside the profile layout.
- Open another player's global profile and verify their equipped identity cosmetics render correctly.

## 3. Achievements
- Scroll the full catalogue and check no title/reward text escapes its card.
- Verify visible versus hidden achievements behave correctly.
- Trigger several low/mid/high-tier achievements and confirm rewards and prestige styling feel proportionate.
- Confirm the rarest achievements feel aspirational rather than required for normal progression.

## 4. Space Garage / economy
- Verify blue, purple, red and gold rarity presentation.
- Inspect the final four spaceship effects for visual quality and readability during actual Endurance gameplay.
- Open caches repeatedly and confirm duplicate rewards scale by rarity.
- Confirm pity protection does not display fake near-miss behaviour.
- Verify the gold ship can be reached through the intended skill route as well as the rare cache route.
- Check the garage/cache panels at low/windowed resolutions for clipping.

## 5. Endurance
- Enable SETTINGS → ENDURANCE HITBOX and verify the green rectangle matches the body collision area.
- Complete normal rail movement, warp and UFO phases with the hitbox both ON and OFF.
- Confirm no cosmetics or profile rewards affect gameplay collision.
- Run `/Perf Reset`, play through warp/UFO, then `/Perf Save` and inspect severe >50 ms frames, memory growth and allocation rate.

## 6. Survival sectors / elites / events
- Play waves naturally and verify mechanical variety increases without abrupt unfair speed spikes.
- Encounter Armoured and Striker elites; verify their telegraphs, HP and rewards are readable.
- Verify only one elite is active where intended.
- Test Nebula Drift, Ion Surge and Supply Window events and rate clarity/fairness/enjoyment.
- Confirm Boss 1 and Boss 2 behaviour remains unchanged except intended polish.
- Test Boss 3 / Quantum Array memory, movement and tracking mechanics without developer explanation first.

## 7. Boss audio / feedback
- Verify critical boss warnings remain audible above music and repeated hit spam.
- Confirm music ducks smoothly for critical cues and restores afterwards.
- Spam Boss 1 core clicks and listen for clipping, missing final explosions or harsh stacking.
- Confirm high-priority HP damage/repair messages cannot be overwritten instantly by routine target messages.

## 8. Local / Global leaderboards
- Verify every row is vertically centred.
- Verify profile name colours render correctly.
- Check Global Top 20 still works even if the new competitive endpoint is unavailable.
- After backend deployment, test Around Me at top, middle and bottom rankings.
- Test Weekly standings and confirm permanent PBs are not reset.
- Confirm weekly rewards are cosmetic/status only.

## 9. Analytics / retention
- Confirm anonymous server analytics defaults OFF.
- Toggle it ON/OFF in Settings and verify the setting persists.
- Use `/Analytics Status` to inspect active days and D1/D7/D30 flags.
- Confirm local `analytics_local.jsonl` contains events but no display name, email, Discord ID or raw cursor path.
- Use `tests/analytics_funnel_report.py` after real sessions to inspect drop-off points.

## 10. Structured player test
For at least 20 comparable observations before balancing a non-obvious mechanic, record:
- enjoyment 1–7
- fairness 1–7
- clarity 1–7
- “I want another run” 1–7
- best moment
- annoying/unclear moment
- failure category
- retry behaviour
- session duration
- frame-time/hitch telemetry

## 11. Final stability pass
- Play at least one 30+ minute Endurance session and one 30+ minute Survival session.
- Alt-tab, resize/window/fullscreen where supported, open/close overlays repeatedly, and restart the game several times.
- Verify audio devices do not accumulate duplicate playback or stop after repeated mode changes.
- Verify local progression, server sync, coins, ships, profile cosmetics and achievements persist after restart/login.
- Run `/SelfTest`, `/DeepAudit`, `/TechAudit`, `/UIAudit`, `/AssetAudit` and save `/Perf` snapshots on the same hardware used for normal play.
