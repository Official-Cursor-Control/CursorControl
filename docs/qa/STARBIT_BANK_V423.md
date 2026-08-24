# Cursor Control v423 — Starbit Bank rewrite

Starbits no longer use client/server balance reconciliation.

## Authority model

- `starbit_bank_accounts` is the protected account ledger.
- `bank_balance` is the 2-minute protected Bank.
- `unbanked_balance` is server-held live-session escrow.
- The Windows client never submits a Starbit balance to the Bank endpoint.
- The client submits monotonic session `earned_total` and `spent_total` counters.
- Supabase calculates deltas and is the only system that changes the authoritative balance.

## Banking

- Bank sync heartbeat: 10 seconds.
- Automatic Bank sweep: every 2 minutes.
- Device handoff immediately sweeps already-synced unbanked Starbits into the Bank.
- Clean Windows shutdown requests an immediate Bank sweep (best effort, 1.2-second cap).
- A new device hydrates from Bank + server escrow before Starbase economy is unlocked.

## Legacy isolation

- `afk-sync` protocol v5 owns progression/session state only.
- Legacy `afk_starbit_progress.starbits` is a compatibility mirror, never an authority.
- Offline Starbit production is disabled from v423 onward.
- Starbase Exchange spends directly from the Bank ledger and serialises against Bank heartbeats.
