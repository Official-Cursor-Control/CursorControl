# Cursor Control v279 — `player-profile` sync contract

The v279 Windows client sends these fields on `action: "sync"` and expects them back in the successful response:

- `unlocked_ships`, `selected_ship`
- `unlocked_fire_colors`, `selected_fire_color`
- `unlocked_fire_sizes`, `selected_fire_size`
- `unlocked_titles`, `selected_title`
- `unlocked_name_colours`, `selected_name_colour`
- `unlocked_profile_frames`, `selected_profile_frame`
- `survival_checkpoint`

## Merge rules

All `unlocked_*` collections are permanent unlocks. The server must merge local + cloud by **set union** before saving and returning them. Do not replace a non-empty cloud unlock collection with the incoming device collection.

Valid IDs:

- fire colours: `1..7` (`0` = free/default red)
- fire sizes: `1..2` (`0` = free/default small)
- name colours: `1..5` (`0` = default white)
- profile frames: `1..7` (`0` = default frame)

Profile titles are text values. Store the union of trimmed non-empty titles and always include `ROOKIE PILOT`.

Equipped selections must be accepted only when they are valid and owned in the merged collection (or are the free/default `0` value). `selected_title` must be present in merged `unlocked_titles`, otherwise retain the existing valid server selection or use `ROOKIE PILOT`.

For `survival_checkpoint`, keep the larger of cloud and incoming values.

A successful response must return the merged authoritative state so the client can immediately apply cosmetics earned on another device:

```json
{
  "ok": true,
  "unlocked_ships": [],
  "selected_ship": 0,
  "unlocked_fire_colors": [],
  "selected_fire_color": 0,
  "unlocked_fire_sizes": [],
  "selected_fire_size": 0,
  "unlocked_titles": ["ROOKIE PILOT"],
  "selected_title": "ROOKIE PILOT",
  "unlocked_name_colours": [],
  "selected_name_colour": 0,
  "unlocked_profile_frames": [],
  "selected_profile_frame": 0,
  "survival_checkpoint": 1
}
```

## Important

Run `SUPABASE_v279_PROFILE_COSMETICS.sql`, then update and redeploy the existing `player-profile` Edge Function. The Windows client changes are complete, but cross-device transfer cannot work until the deployed function accepts, merges, stores and returns these fields.
