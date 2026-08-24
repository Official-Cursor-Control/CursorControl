# v333 backend deployment status

Production Supabase project `fobeyfnqmbslywqapfkb` was updated during the v333 merge:

- `global_player_profiles` now includes `achievement_showcase`, `best_survival_wave`, `best_survival_kills`, `competitive_badge`, and `season_best` in addition to the existing fire/title/name-colour/frame fields.
- `player-profile` Edge Function was deployed as version 5 and now persists/returns showcase, fire cosmetics, title, name colour, profile frame, Survival profile stats, ship selection, and checkpoint data.

The runtime itself does not require this document; it ships in source only.
