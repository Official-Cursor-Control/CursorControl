-- v351 live migration: server-authoritative weekly leaderboard.
-- Active week rolls at Monday 00:00 Asia/Ho_Chi_Minh (UTC+7); historical rows remain keyed by week_key.
create table if not exists public.weekly_scores (
  week_key text not null,
  user_id uuid not null references auth.users(id) on delete cascade,
  difficulty text not null,
  score bigint not null default 0,
  streak integer not null default 0,
  accuracy double precision not null default 0,
  distance double precision not null default 0,
  targets_hit integer not null default 0,
  easy_clears integer not null default 0,
  normal_clears integer not null default 0,
  hard_clears integer not null default 0,
  insane_clears integer not null default 0,
  total_clears integer generated always as (easy_clears + normal_clears + hard_clears + insane_clears) stored,
  achieved_at timestamptz not null default now(),
  primary key (week_key, user_id, difficulty),
  constraint weekly_scores_difficulty check (difficulty in ('OVERALL','EASY','NORMAL','HARD','INSANE','ENDURANCE','SURVIVAL'))
);
create index if not exists weekly_scores_rank_idx on public.weekly_scores (week_key, difficulty, score desc, streak desc, accuracy desc);
alter table public.weekly_scores enable row level security;
