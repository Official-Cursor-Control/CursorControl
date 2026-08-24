# Cursor Control v322 Architecture Reconciliation

This build is a zero-feature architecture pass. v321 is the behavioral reference.

## Ownership map

- `main.go` — Win32 bootstrap, common types/state, shared primitives, main frame/input dispatch.
- `build_identity.go` — single source of truth for client/build identity.
- `endurance_warp.go` — Endurance warp schedule, geometry, speed/ambient transition state.
- `endurance_render_d2d.go` — Direct2D resources and Endurance-only render path.
- `endurance_gameplay.go` — fixed-step Endurance simulation, hazards, aliens, power-ups, failure state.
- `progression_achievements.go` — ranks, EXP, achievements, profile cosmetic unlock progression.
- `online_sync.go` — authentication, profile synchronization, score submission and leaderboard persistence.
- `ui_modals.go` — profile/settings/achievement/tutorial/result modal screens.
- `ui_leaderboards_profiles.go` — leaderboard rendering/data adaptation and remote profile presentation.
- `ui_quick_garage_dev.go` — quick-access HUD, Garage, developer console and objective panels.
- `audio_runtime.go` — centralized runtime SFX bus and priority/ducking implementation.
- `analytics_v320.go` — anonymous aggregate/funnel analytics only; no gameplay authority.
- `competition_v320.go` — weekly/around-me competition contracts; no permanent PB authority.
- `survival*.go` — Survival director, enemies, bosses and sector-specific behavior.

## State ownership rules

1. Analytics observes state; it never decides gameplay.
2. Competition can award prestige cosmetics; it never modifies gameplay power.
3. Online synchronization mirrors persisted values; it must not manufacture achievements.
4. Each mode owns its transient gameplay state.
5. Shared progression/economy changes go through dedicated helpers, never UI-side direct mutation.
6. Rendering is presentation-only and never changes collision geometry.
7. Build/version identity comes only from `build_identity.go`.

## Acceptance rule

Any architecture change that alters v321 gameplay outcomes, save semantics, collision, reward math, or frame-time behavior is rejected unless explicitly documented as a bug fix.

## v336 profile skin fitting
Profile skins remain 1124x174 runtime assets. Source banner art is normalized with aspect-preserving height scaling and centre-rail extension rather than full-canvas stretching. Remote-profile identity content uses an inset skin-safe layout so avatar/rank content does not cover the skin perimeter.

## v337 profile skin fit/noise correction
- Global profile skins render outside the identity information rectangle so the PNG frame's inner edge aligns with the identity box instead of shrinking the banner inside it.
- The avatar/rank content is inset further from the left border so it cannot obscure the skin frame.
- All 11 profile skins use the Survival-sprite cleanup model: alpha < 8 is removed, only the connected foreground silhouette is retained, and runtime BGRA is premultiplied for Win32 AlphaBlend.

## v345 Global Profile identity layout
- Equipped profile skins are the identity container; the legacy bevel identity panel is not drawn beneath an equipped skin.
- Avatar/rank/text identity cluster is shifted 25 design pixels right.
- Username receives its additional -2 X / +3 Y adjustment while retaining the themed 3 px shadow.
- Title, rank name, and EXP receive the requested vertical spacing offsets.
- Rank-name text colour follows rank tier (Bronze/Silver/Gold/Platinum/Diamond/Master).
- Global Profile EDIT button is raised by 10 design pixels.

## v347 profile bounds / micro polish
- Global Profile banner fitting now uses the actual runtime identity rectangle and preserves aspect ratio when the modal is resolution-clamped, preventing skins from escaping the profile interface.
- Discord avatar is 1 px higher, uses a flat square frame with no bevel/drop shadow, and the avatar plus rank emblem are 10 px farther right without dragging the text column with them.
- Player display name is 1 px farther left and 5 px lower while retaining the themed 3D name treatment.


## v348 focused polish
- Survival locked-sector text spacing lowered 6 px.
- Survival middle HUD text clamped to its box.
- Survival HP-loss SFX added.
- Settings uses categorized, scroll-contained groups so customization can grow without overflowing the modal.
- Local profile restyled as a clean view-only Global-Profile-inspired layout.
- Achievement cards show stat-backed progress bars only after progress starts and hide them after completion.

## Starbase animation/physics rewrite
- Singularity rotation rewritten as software BGRA rotation with dedicated mutable DIB surface.
- Starbase background now follows the established two-tile elapsed-time scrolling structure.
- Starbit particles now have independent velocity, pairwise collisions, and circular wall reflection.

## Starbase physical-base presentation overhaul
- Major AFK systems now have persistent physical structures on the lunar floor: Cursor Core, Scout Pad, Tech Lab, Operations, Fabricator, Drone Bay, and Orbital system.
- Newly constructed systems trigger a short holographic build reveal and milestone banner; no construction animation mutates economy state.
- Physical structures are clickable navigation targets and open their existing management panels; top navigation remains a fallback.
- Cursor Core, Drone Bay, and Orbital structures visually evolve with their progression tiers.
- Scout Expeditions now have a physical landing pad and visible launch/return state.
- Active Starbit Cache is a physical glowing object in the world instead of a floating permanent HUD panel.
- Ambient maintenance drones, base energy packets, blinking lights, scanners, and beacons make the idle scene visibly alive without changing production.
- Welcome Back Starbit totals count upward visually before settling on the authoritative reward.

## Precision / Starbase visual micro-pass
- Precision arena background now renders at full 100% source opacity.
- Precision track uses a saturated bright-red collision edge with two-layer red outer glow while preserving the readable inner rail.
- Starbase physical facilities render at 150% of their prior size while remaining grounded to the moon strip.
- Cursor Core receives the requested additional 5 px left shift; Scout Pad receives the requested additional 5 px downward shift.
- Starbase telemetry values receive the requested additional 3 px downward alignment.


## Starbase facility glow / Survival handoff pass
- Constructed Starbase facilities render at 125% of the immediately previous visual scale (1.875x the original facility layout scale) while remaining grounded to the lunar strip.
- Every built facility receives a persistent light-purple breathing halo; hover feedback remains separate and stronger.
- Survival may open SELECT MODE during an active run; doing so abandons only the transient Survival run and returns to Waiting before mode selection.
- Selecting Starbase from Survival now normalizes Survival run state before opening the Starbase overlay, preventing the previous disappearing-HUD/no-overlay transition failure.

## v391 Void Serpent finale polish
- Finale keeps the Boss 2 scrolling background under all consciousness fades/head shots.
- Final Head 1 render scale reduced 50%; Final Head 2 reduced 25% from v390.
- Final mouth-core hits flash only Final Head 3 red for 120 ms per successful click.
- Head 3 final roar/node timing shortened: roar at 0.8 s, node at 1.1 s.
- Finale roars use three dedicated serpent samples at 50%, 75%, and 100% Effects Volume gain.
- Post-energy exit separates motion: body/tail preserve vertical downward stream while head retreats right.

## v392 Survival neutral special-enemy input

- Colour-specific mouse input is now reserved for standard red/blue Survival energy enemies (Scout and Heavy, including Splitter-created Scouts).
- All eight expansion enemy types accept either left or right click while keeping their own health, shield, phase and hazard mechanics.
- Monster Guide behaviour copy was updated to match the neutral-input rules.

## v437 Pilot Skills + Starbase polish

- Pilot Skills expanded from 6 to 14 fixed, server-synced skill IDs while preserving existing unlocked IDs 0-5.
- Skill points now use long-term Tech Lab milestones (5/12/20/30/40/50/60/70/80/90/100 visible research levels plus late Core tiers); already-spent legacy points are never revoked.
- The Pilot tree is a fixed left-to-right graph rendered in a larger pannable viewport. Empty-space drag pans the graph; nodes themselves remain fixed.
- Node visibility is prerequisite-local: owned = full-colour/steady gold, next available = grey art/pulsing gold, future = shared grey question-mark art.
- Skill details live in a slide-up bottom drawer and hidden nodes never disclose their identity/effect.
- New utility skills add controlled Overdrive duration, NAV rewards, cache chance/reward, expedition speed and Drone output without adding another compounding Starbit multiplier stack.
- Starbase moon floor is horizontally overscanned and shifted down 20 px; facility geometry follows the same surface anchor.
- Native Windows arrow cursor is restricted to Starbase. Other modes retain Cursor Control cursor presentation.

## v438 Pilot Skills navigation polish
- Pilot Skills is a 2400×1600 virtual world with true two-axis drag panning and clamped world bounds.
- The supplied Pilot Network background is a world-space BGRA asset and pans with the skill nodes rather than remaining screen-fixed.
- Skill nodes use wider horizontal and vertical spacing so branches remain readable at full progression depth.
- Skill icon BGRA assets are cleaned with an alpha fringe threshold and connected-silhouette pass to reduce detached glow/sparkle noise at gameplay scale.
- Native Windows cursor is shown for overlays, result/failure interfaces and HUD regions outside the gameplay arena; active gameplay arena control continues to use Cursor Control's custom cursor presentation.


## v452 intro revision

The startup video is based directly on `Intro 1.mp4`. Only the first second is visually altered: the original abstract construction mark is replaced by a vertically stacked geometric `K / O / N / G` line pattern. From the 1.000-second boundary onward, timing/composition follow Intro 1 unchanged. The supplied Intro 1 audio stream is preserved.


## v453 weekly boundary hotfix
- Precision Competition and weekly score windows reset at Monday 00:00 Asia/Ho_Chi_Minh, after the full Sunday has elapsed.
- Client fallback countdown uses the same Monday boundary.

## v465 Ship Module sprite/name binding
- Ship Module visible names are derived from the exact sprite filename at each fixed 9x8 save index so artwork and labels cannot drift apart.
- Module art rendering is capped at native crop size and 76% of the available art region for a sharper, smaller presentation.
- The Ship Modules subtitle line is intentionally omitted; recipes/save indices/materials remain unchanged.
