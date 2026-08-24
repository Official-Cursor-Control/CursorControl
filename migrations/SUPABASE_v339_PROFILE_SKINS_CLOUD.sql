ALTER TABLE public.global_player_profiles
  ADD COLUMN IF NOT EXISTS unlocked_profile_skins integer[] NOT NULL DEFAULT '{}'::integer[],
  ADD COLUMN IF NOT EXISTS selected_profile_skin integer NOT NULL DEFAULT 0;

UPDATE public.global_player_profiles
SET
  unlocked_profile_skins = (
    SELECT COALESCE(array_agg(DISTINCT v ORDER BY v), '{}'::integer[])
    FROM unnest(COALESCE(unlocked_profile_frames, '{}'::integer[])) AS v
    WHERE v BETWEEN 101 AND 111
  ),
  selected_profile_skin = CASE
    WHEN selected_profile_frame BETWEEN 101 AND 111 THEN selected_profile_frame
    ELSE 0
  END
WHERE cardinality(unlocked_profile_skins) = 0
  AND selected_profile_skin = 0;
