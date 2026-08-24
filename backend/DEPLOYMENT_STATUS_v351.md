# v351 live backend status

- `weekly_leaderboards_v351` migration applied to production.
- `submit-score` deployed as version 6; permanent PB/progression behavior retained and current-week rows are written on every submitted run.
- `competitive-hub` deployed as version 1 and serves the active weekly leaderboard.
- Weekly boundary: Monday 00:00 Asia/Ho_Chi_Minh (UTC+7). A new `week_key` starts automatically; historical rows are not deleted.
