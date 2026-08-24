# Cursor Control v320 WIP — `player-profile` extension

The client keeps the complete v279 profile contract and adds three optional fields:

- `achievement_showcase`: up to three achievement IDs selected by the player.
- `best_survival_wave`: monotonic integer; keep the larger of cloud and incoming.
- `best_survival_kills`: monotonic integer; keep the larger of cloud and incoming.

## Showcase validation

The server should trim empty strings, remove duplicates, cap the array at three entries, and retain the player's ordering. The server does **not** need to infer achievement ownership yet; the client only offers locally unlocked achievements for selection.

A successful sync should include the authoritative values in addition to all v279 fields.

## Backward-compatible client rollout

The v320 client first attempts the extended payload. If the deployed Edge Function rejects the new contract, the client automatically retries the v279 payload. This prevents the new profile work from breaking existing ship/cosmetic/checkpoint cloud sync before this migration is deployed.
