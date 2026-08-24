# Controlled release roadmap

This WIP intentionally develops against the v319 stable gameplay baseline. It should not replace that rollback build until the complete regression/player-check checklist passes.

## Identity / progression foundation
Profiles, 95-achievement catalogue, showcase slots, six achievement prestige levels, profile cosmetics, CS-style ship rarity tiers, pity protection, rarity-scaled duplicate compensation, gold skill route, progression pacing guardrails.

## Survival expansion
Sector 3 elite enemies and meteor-drift event; Boss 3 Quantum Array with memory, movement and tracking tests; Sector 4 endless continuation. Each boss tests a distinct skill axis.

## Competition
Global Top 20 remains stable. Around Me and Weekly scopes are client-ready behind a backward-compatible `competitive-hub` endpoint. Server-authoritative weekly/seasonal awards are cosmetic only.

## Player experience / UX
Critical-focus presentation reduces nonessential live-feed prominence during boss/warp attention peaks. Long-term identity remains in Profile/Achievements/Garage rather than the collision arena.

## Audio
Priority-reserved SFX voices, per-effect concurrency caps, music ducking for critical cues, repeated-hit protection, rarity-sensitive reward stings.

## Measurement
Developer frame/memory telemetry remains the performance source of truth. Privacy-minimal local analytics adds funnel, failure, boss, progression and retention signals. Optional server analytics defaults OFF pending player consent/privacy handling.

## Release rule
Do not optimize or rebalance from external benchmarks alone. Use research for initial hypotheses, Cursor Control telemetry to locate outcomes, and structured player testing to decide whether those outcomes are enjoyable/fair.
