-- Cursor Control v453: weekly/competition reset hotfix.
-- Intended cycle: Monday 00:00 Asia/Ho_Chi_Minh through Sunday 23:59:59.999.

create or replace function public.cc_precision_week_window(p_now timestamptz default now())
returns table(week_key text, starts_at timestamptz, ends_at timestamptz)
language plpgsql stable
as $$
declare
  local_now timestamp;
  local_start_date date;
  days_since_monday integer;
begin
  local_now := p_now at time zone 'Asia/Ho_Chi_Minh';
  days_since_monday := (extract(dow from local_now)::int + 6) % 7;
  local_start_date := local_now::date - days_since_monday;
  starts_at := local_start_date::timestamp at time zone 'Asia/Ho_Chi_Minh';
  ends_at := starts_at + interval '7 days';
  week_key := 'VN-' || to_char(local_start_date, 'YYYY-MM-DD');
  return next;
end;
$$;
