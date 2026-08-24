-- Cursor Control v446: server-authoritative weekly Precision Competition.
-- Week boundary: Monday 00:00 Asia/Ho_Chi_Minh (Sunday is the final active day).

create table if not exists public.precision_competition_weekly (
  week_key text not null,
  user_id uuid not null references auth.users(id) on delete cascade,
  total_points bigint not null default 0,
  easy_points bigint not null default 0,
  normal_points bigint not null default 0,
  hard_points bigint not null default 0,
  insane_points bigint not null default 0,
  easy_streak integer not null default 0,
  normal_streak integer not null default 0,
  hard_streak integer not null default 0,
  insane_streak integer not null default 0,
  easy_completions integer not null default 0,
  normal_completions integer not null default 0,
  hard_completions integer not null default 0,
  insane_completions integer not null default 0,
  updated_at timestamptz not null default now(),
  primary key (week_key, user_id)
);
create index if not exists precision_competition_rank_idx
  on public.precision_competition_weekly
  (week_key, total_points desc, insane_points desc, hard_points desc, normal_points desc, easy_points desc, updated_at asc);
alter table public.precision_competition_weekly enable row level security;

create table if not exists public.precision_competition_awards (
  week_key text not null,
  user_id uuid not null references auth.users(id) on delete cascade,
  placement integer not null check (placement between 1 and 3),
  exp_reward integer not null default 0,
  spacecoins_reward integer not null default 0,
  claimed_at timestamptz,
  created_at timestamptz not null default now(),
  primary key (week_key, user_id)
);
create index if not exists precision_competition_awards_user_idx
  on public.precision_competition_awards(user_id, claimed_at, created_at desc);
alter table public.precision_competition_awards enable row level security;

create or replace function public.cc_precision_week_window(p_now timestamptz default now())
returns table(week_key text, starts_at timestamptz, ends_at timestamptz)
language plpgsql stable
as $$
declare
  local_now timestamp;
  local_start_date date;
begin
  local_now := p_now at time zone 'Asia/Ho_Chi_Minh';
  -- PostgreSQL DOW: Sunday=0, Monday=1. Normalize Monday to zero.
  local_start_date := local_now::date - ((extract(dow from local_now)::int + 6) % 7);
  starts_at := local_start_date::timestamp at time zone 'Asia/Ho_Chi_Minh';
  ends_at := starts_at + interval '7 days';
  week_key := 'VN-' || to_char(local_start_date, 'YYYY-MM-DD');
  return next;
end;
$$;

create or replace function public.record_precision_competition_success(p_user_id uuid, p_difficulty text)
returns public.precision_competition_weekly
language plpgsql
security definer
set search_path = public
as $$
declare
  wk record;
  rowv public.precision_competition_weekly;
  d text := upper(trim(coalesce(p_difficulty,'')));
  new_streak integer;
  base_points integer;
  multiplier numeric;
  award_points integer;
begin
  if d not in ('EASY','NORMAL','HARD','INSANE') then
    raise exception 'invalid precision difficulty';
  end if;
  select * into wk from public.cc_precision_week_window(now());
  insert into public.precision_competition_weekly(week_key,user_id)
    values(wk.week_key,p_user_id)
    on conflict (week_key,user_id) do nothing;
  select * into rowv from public.precision_competition_weekly
    where week_key=wk.week_key and user_id=p_user_id for update;

  new_streak := case d
    when 'EASY' then rowv.easy_streak+1
    when 'NORMAL' then rowv.normal_streak+1
    when 'HARD' then rowv.hard_streak+1
    else rowv.insane_streak+1 end;
  base_points := case d when 'EASY' then 10 when 'NORMAL' then 25 when 'HARD' then 60 else 150 end;
  multiplier := case
    when new_streak >= 15 then 1.25
    when new_streak >= 10 then 1.20
    when new_streak >= 7 then 1.15
    when new_streak >= 5 then 1.10
    when new_streak >= 3 then 1.05
    else 1.00 end;
  award_points := round(base_points * multiplier)::integer;

  update public.precision_competition_weekly set
    total_points = total_points + award_points,
    easy_points = easy_points + case when d='EASY' then award_points else 0 end,
    normal_points = normal_points + case when d='NORMAL' then award_points else 0 end,
    hard_points = hard_points + case when d='HARD' then award_points else 0 end,
    insane_points = insane_points + case when d='INSANE' then award_points else 0 end,
    easy_streak = case when d='EASY' then new_streak else easy_streak end,
    normal_streak = case when d='NORMAL' then new_streak else normal_streak end,
    hard_streak = case when d='HARD' then new_streak else hard_streak end,
    insane_streak = case when d='INSANE' then new_streak else insane_streak end,
    easy_completions = easy_completions + case when d='EASY' then 1 else 0 end,
    normal_completions = normal_completions + case when d='NORMAL' then 1 else 0 end,
    hard_completions = hard_completions + case when d='HARD' then 1 else 0 end,
    insane_completions = insane_completions + case when d='INSANE' then 1 else 0 end,
    updated_at = now()
  where week_key=wk.week_key and user_id=p_user_id
  returning * into rowv;
  return rowv;
end;
$$;

create or replace function public.reset_precision_competition_streak(p_user_id uuid, p_difficulty text)
returns void
language plpgsql
security definer
set search_path = public
as $$
declare
  wk record;
  d text := upper(trim(coalesce(p_difficulty,'')));
begin
  if d not in ('EASY','NORMAL','HARD','INSANE') then return; end if;
  select * into wk from public.cc_precision_week_window(now());
  update public.precision_competition_weekly set
    easy_streak = case when d='EASY' then 0 else easy_streak end,
    normal_streak = case when d='NORMAL' then 0 else normal_streak end,
    hard_streak = case when d='HARD' then 0 else hard_streak end,
    insane_streak = case when d='INSANE' then 0 else insane_streak end,
    updated_at = now()
  where week_key=wk.week_key and user_id=p_user_id;
end;
$$;

revoke all on function public.record_precision_competition_success(uuid,text) from public, anon, authenticated;
revoke all on function public.reset_precision_competition_streak(uuid,text) from public, anon, authenticated;
grant execute on function public.record_precision_competition_success(uuid,text) to service_role;
grant execute on function public.reset_precision_competition_streak(uuid,text) to service_role;
