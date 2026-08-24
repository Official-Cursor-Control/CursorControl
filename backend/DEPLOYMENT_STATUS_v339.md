# Cursor Control v339 cloud profile skins

- Production migration `profile_skins_cross_device_v339` applied to Supabase project.
- `global_player_profiles.unlocked_profile_skins` stores earned skin IDs 101..111.
- `global_player_profiles.selected_profile_skin` stores the equipped skin.
- Existing v335-v338 frame values were migrated into the explicit skin fields.
- `player-profile` Edge Function deployed as version 7.
- Version 7 mirrors explicit skin fields into legacy frame fields for backward compatibility.
