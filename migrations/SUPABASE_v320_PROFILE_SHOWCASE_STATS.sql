-- Cursor Control v320 WIP — public profile showcase + Survival profile stats
-- Run in Supabase SQL Editor before deploying the matching player-profile Edge Function.
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
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS achievement_showcase text[] NOT NULL DEFAULT ''{}''::text[]', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS best_survival_wave integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS best_survival_kills integer NOT NULL DEFAULT 0', t.table_schema, t.table_name);
  END LOOP;
  IF found_count = 0 THEN
    RAISE EXCEPTION 'Could not find Cursor Control profile table (expected unlocked_ships + selected_ship columns).';
  END IF;
END $$;

-- Competitive identity fields are server-owned presentation metadata. They are
-- returned on public profiles but must not grant gameplay power.
DO $$
DECLARE
  t record;
BEGIN
  FOR t IN
    SELECT c.table_schema, c.table_name
    FROM information_schema.columns c
    WHERE c.table_schema = 'public'
      AND c.column_name IN ('selected_ship','unlocked_ships')
    GROUP BY c.table_schema, c.table_name
    HAVING count(DISTINCT c.column_name) = 2
  LOOP
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS competitive_badge text NOT NULL DEFAULT ''''', t.table_schema, t.table_name);
    EXECUTE format('ALTER TABLE %I.%I ADD COLUMN IF NOT EXISTS season_best text NOT NULL DEFAULT ''''', t.table_schema, t.table_name);
  END LOOP;
END $$;
