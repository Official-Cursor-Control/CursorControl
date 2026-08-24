-- Cursor Control v279 — cross-device garage + profile cosmetics
-- Run in Supabase SQL Editor before deploying the updated player-profile function.
-- This safely extends whichever existing public table stores the Cursor Control
-- unlocked_ships + selected_ship player profile state.
DO $$
DECLARE
  t record;
  found_count integer := 0;
BEGIN
  FOR t IN
    SELECT c.table_schema, c.table_name
    FROM information_schema.columns c
    WHERE c.table_schema = 'public'
      AND c.column_name IN ('selected_ship','unlocked_ships')
    GROUP BY c.table_schema, c.table_name
    HAVING count(DISTINCT c.column_name) = 2
  LOOP
    found_count := found_count + 1;

    -- Thruster fire cosmetics.
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS unlocked_fire_colors integer[] NOT NULL DEFAULT ''{}''::integer[]', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_fire_color integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS unlocked_fire_sizes integer[] NOT NULL DEFAULT ''{}''::integer[]', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_fire_size integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);

    -- Profile customisation. Unlock collections are persisted independently of
    -- achievements so earned cosmetics transfer even when a new device has no
    -- local achievement history yet.
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS unlocked_titles text[] NOT NULL DEFAULT ARRAY[''ROOKIE PILOT'']::text[]', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_title text NOT NULL DEFAULT ''ROOKIE PILOT''', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS unlocked_name_colours integer[] NOT NULL DEFAULT ''{}''::integer[]', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_name_colour integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS unlocked_profile_frames integer[] NOT NULL DEFAULT ''{}''::integer[]', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_profile_frame integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
  END LOOP;

  IF found_count = 0 THEN
    RAISE EXCEPTION 'Could not find the Cursor Control profile table (expected unlocked_ships + selected_ship columns).';
  END IF;
END $$;
