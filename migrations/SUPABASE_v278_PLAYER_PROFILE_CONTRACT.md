# Cursor Control v278 — `player-profile` sync contract

The v278 Windows client now sends and accepts these additional fields through the existing `player-profile` Edge Function:

- `unlocked_fire_colors` — integer array; merge local + cloud as a union
- `selected_fire_color` — integer; return the saved equipped colour
- `unlocked_fire_sizes` — integer array; merge local + cloud as a union
- `selected_fire_size` — integer; return the saved equipped fire size
- `selected_title` — text
- `selected_name_colour` — integer
- `selected_profile_frame` — integer

The server must preserve the existing `unlocked_ships`, `selected_ship`, and `survival_checkpoint` behaviour.

For `action: "sync"`, union the two unlock arrays before saving/returning them. Valid fire-colour IDs are 1–7 (0 is the free default); valid fire-size IDs are 1–2 (0 is the free default). Reject or clamp out-of-range selected IDs rather than writing them.

The successful response should include the merged values so another device receives them immediately:

```json
{
  "ok": true,
  "unlocked_ships": [],
  "selected_ship": 0,
  "unlocked_fire_colors": [],
  "selected_fire_color": 0,
  "unlocked_fire_sizes": [],
  "selected_fire_size": 0,
  "selected_title": "ROOKIE PILOT",
  "selected_name_colour": 0,
  "selected_profile_frame": 0,
  "survival_checkpoint": 1
}
```

Run `SUPABASE_v278_PROFILE_COSMETICS.sql` first, then extend/deploy the existing `player-profile` function with these fields.
