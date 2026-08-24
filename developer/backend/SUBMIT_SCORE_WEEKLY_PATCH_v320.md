# submit-score weekly patch v320

After the existing authenticated run has passed **all current validation / anti-cheat checks**, upsert the accepted run into `weekly_scores` in addition to the permanent PB logic.

Compute `week_key` on the server (ISO week, Monday 00:00 UTC). Never accept a client-supplied week key.

Use the same `user.id` and normalized difficulty already validated by `submit-score`. For each `(week_key,user_id,difficulty)`, compare against the existing weekly row with the same ordering used by the leaderboard:

- ENDURANCE: distance, then targets_hit, then accuracy.
- Standard difficulties: score, then streak, then accuracy.
- SURVIVAL: highest wave/checkpoint first once the Survival score contract is enabled.

Only overwrite when the new accepted run is better. This weekly write must never change permanent PBs, progression, HP, gameplay stats, cache odds, or unlock power.

Deploy order:
1. `SUPABASE_v320_COMPETITION.sql`
2. Patch/redeploy `submit-score`
3. Deploy `competitive-hub`
4. Test a logged-in weekly score and Around Me response before enabling competition announcements.
