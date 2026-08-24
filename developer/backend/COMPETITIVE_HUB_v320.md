# Competitive Hub v320 contract

Implement a Supabase Edge Function `competitive-hub` using the service role. The Windows client is already backward-compatible: Top 20 continues to use `global-top20`; Around Me and Weekly activate only if this function exists.

## GET
Query parameters:
- `scope=around_me|weekly`
- `difficulty=OVERALL|EASY|NORMAL|HARD|INSANE|ENDURANCE|SURVIVAL`

Authentication is required so Around Me can locate the caller without exposing private identifiers.

### `around_me`
Return at most 11 entries: caller, up to five immediately above, and up to five immediately below. Compute the caller's true position server-side, then fetch the rank window. Never fake a position using the Top 20 subset.

### `weekly`
Return the current server-week Top 20. The week boundary must use server UTC consistently (recommended Monday 00:00 UTC). Endurance ordering is distance, targets, accuracy. Standard ordering uses existing score/streak/accuracy semantics. Survival should use highest wave/boss checkpoint first, then kills, then remaining HP/run time as defined by the eventual Survival leaderboard contract.

Response:
```json
{
  "week_key": "2026-W34",
  "entries": [
    {
      "position": 1,
      "user_id": "...",
      "name": "PLAYER",
      "selected_name_colour": 4,
      "score": 12345,
      "streak": 40,
      "accuracy": 98.2,
      "distance": 1234.5,
      "targets_hit": 40,
      "rank": "MASTER I"
    }
  ]
}
```

## POST `claim_weekly_reward`
Only the server decides placement. Awards are cosmetic/status-only:
- #1: weekly champion badge + temporary champion title + temporary premium frame
- #2-3: podium badge + temporary premium frame
- #4-10: Top 10 badge
- #11-20: finalist badge (optional; no gameplay advantage)

One award per user/week via the `competitive_awards` unique constraint. Expiry should be server-time based. Never award score, HP, speed, hitbox, damage, cache odds, or other gameplay advantage.

## Weekly score update
Patch `submit-score` so every authenticated accepted run also upserts a row into `weekly_scores` for the current server week. Use the same anti-cheat/validation path as permanent scores; do not trust client-supplied week keys or placements.
