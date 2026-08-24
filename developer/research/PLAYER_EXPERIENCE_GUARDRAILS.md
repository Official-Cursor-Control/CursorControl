# Cursor Control player-experience guardrails (research-driven)

These are starting hypotheses, not universal truths. Cursor Control telemetry and structured player tests override generic industry averages once the sample is useful.

## Retention context
- GameAnalytics' January 2026 benchmark report explicitly uses D1/D7/D30 retention plus session behaviour as core comparison metrics; use the current report for market context rather than treating one historical shorthand as a universal target.
- GameAnalytics' Feb 2025 retention article still quotes its older benchmark of about 40% D1 / 15% D7 / 6.5% D28 for top-performing titles, and explicitly says genre matters. Those figures are useful as historical top-performance context, **not** as a launch promise for Cursor Control.
- Cursor Control has a small socially connected PC/community audience, so its own cohort trend is more valuable than mobile-wide averages once enough data exists.

Internal targets are therefore diagnostic guardrails, not launch promises:
- tutorial completion >= 85%
- first achievement >= 80%
- try 2+ modes >= 60%
- logged-in profile customisation >= 40%
- first cache >= 60%
- Boss 1 eventual clear 60–75%
- Boss 2 eventual clear 35–55%
- expert achievement ownership 2–10%
- legendary ownership <1%
- gold ship ownership initially 1–3%
- D1/D7/D30 are tracked, but changes are judged primarily against Cursor Control's own prior cohorts on comparable builds

## Challenge and flow
Published player-experience work repeatedly points to clear goals, immediate/unambiguous feedback and a challenge/skill balance as central flow conditions. A 2020 complex-mobile-game study found the best perceived challenge-skill balance around a player's level; very easy play produced less flow. Cursor Control therefore increases Survival *mechanical combinations* rather than simply inflating HP/speed.

## Achievement/social sequencing
Park et al. analysed 51,104 online-game players and found achievement features predictive from initial through advanced phases, while social features became most predictive at the highest level. This is why achievements/profile identity are built before deeper weekly/seasonal competition.

## Balance method
Pfau & El-Nasr combine player-reported preferences with large-scale gameplay data (over 4 million fights in their study). Cursor Control adopts the same principle: telemetry identifies where outcomes happen; player tests identify whether those outcomes feel fair/fun. Never balance from win-rate alone.

## Random rewards
Research on loot-box-like randomized rewards justifies conservative treatment. Cursor Control Space Caches remain gameplay-earned, publish odds, avoid fake near-miss presentation, use pity protection and provide a skill route to the prestige ship. No competitive power is attached to random cosmetics.

## UI / feedback
Critical gameplay feedback must win attention over progression decoration. Profiles/achievements/garage own long-term information; gameplay HUD owns immediate state. Visual/audio effects should reinforce a decision or outcome, not obscure the rail or collision space.

## Performance gates
No content feature is considered complete if it introduces repeatable severe frame hitches, uncontrolled allocation growth, runaway goroutines, or object-count escalation. Measure first; optimize measured hotspots, then re-run regression and stress tests.
