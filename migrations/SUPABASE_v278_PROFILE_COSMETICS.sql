-- Cursor Control v278 — profile cosmetic columns
-- Safe migration helper: adds the new fields to whichever public table already
-- stores the existing selected_ship / unlocked_ships profile state.
DO $$
DECLARE
  t record;
  found_count integer := 0;
BEGIN
  FOR t IN
    SELECT DISTINCT c.table_schema, c.table_name
    FROM information_schema.columns c
    WHERE c.table_schema = 'public'
      AND c.column_name IN ('selected_ship','unlocked_ships')
    GROUP BY c.table_schema, c.table_name
    HAVING count(DISTINCT c.column_name) = 2
  LOOP
    found_count := found_count + 1;
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS unlocked_fire_colors integer[] NOT NULL DEFAULT ''{}''::integer[]', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_fire_color integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS unlocked_fire_sizes integer[] NOT NULL DEFAULT ''{}''::integer[]', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_fire_size integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_title text NOT NULL DEFAULT ''ROOKIE PILOT''', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_name_colour integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS selected_profile_frame integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
  END LOOP;
  IF found_count = 0 THEN
    RAISE EXCEPTION 'Could not find the existing Cursor Control profile table (expected unlocked_ships + selected_ship columns).';
  END IF;
END $$;
