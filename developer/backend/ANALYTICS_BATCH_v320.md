# Analytics batch v320

The client records privacy-minimal local telemetry regardless of network state and defaults **ANONYMOUS ANALYTICS = OFF** for server sharing. Do not silently change that default.

If you deploy `analytics-batch`:
- require an authenticated access token;
- derive `user_id` from the token, never accept a client-supplied user id;
- accept at most 100 events/request and cap total JSON size;
- allow-list event names and scalar field keys;
- reject display names, email, Discord IDs, chat text, raw mouse coordinates/paths, IP-derived location fields, and arbitrary nested payloads;
- store server receive time separately from client time;
- aggregate reports; do not expose individual-player analytics to public clients.

Core funnel events emitted by the client include session_started, mode_selected, run_started, run_completed, run_failed, achievement_unlocked, space_cache_open_started, boss_attempted, tutorial_completed, profile_customized and analytics_consent_changed. Additional specific events should be introduced only when they answer a balancing question.
