-- Cursor Control v446: automatic Precision Competition awards + historical winner views.
-- EXP is credited server-side during weekly finalization. Space Coins are stored as a
-- server-authoritative pending delivery and are injected into the client automatically
-- on the winner's next authenticated connection. No manual claim action exists.

alter table public.precision_competition_awards
  add column if not exists exp_credited_at timestamptz,
  add column if not exists spacecoins_delivered_at timestamptz;

create index if not exists precision_competition_awards_week_placement_idx
  on public.precision_competition_awards(week_key, placement);
create index if not exists precision_competition_awards_history_idx
  on public.precision_competition_awards(placement, created_at desc);

create or replace function public.cc_rank_for_exp(p_exp bigint)
returns text
language sql
immutable
as $$
  select case
    when greatest(coalesce(p_exp,0),0) >= 135000 then 'MASTER III'
    when greatest(coalesce(p_exp,0),0) >= 100000 then 'MASTER II'
    when greatest(coalesce(p_exp,0),0) >= 75000 then 'MASTER I'
    when greatest(coalesce(p_exp,0),0) >= 55000 then 'DIAMOND III'
    when greatest(coalesce(p_exp,0),0) >= 40000 then 'DIAMOND II'
    when greatest(coalesce(p_exp,0),0) >= 30000 then 'DIAMOND I'
    when greatest(coalesce(p_exp,0),0) >= 22000 then 'PLATINUM III'
    when greatest(coalesce(p_exp,0),0) >= 16000 then 'PLATINUM II'
    when greatest(coalesce(p_exp,0),0) >= 12000 then 'PLATINUM I'
    when greatest(coalesce(p_exp,0),0) >= 8500 then 'GOLD III'
    when greatest(coalesce(p_exp,0),0) >= 6000 then 'GOLD II'
    when greatest(coalesce(p_exp,0),0) >= 4000 then 'GOLD I'
    when greatest(coalesce(p_exp,0),0) >= 2500 then 'SILVER III'
    when greatest(coalesce(p_exp,0),0) >= 1500 then 'SILVER II'
    when greatest(coalesce(p_exp,0),0) >= 750 then 'SILVER I'
    when greatest(coalesce(p_exp,0),0) >= 300 then 'BRONZE III'
    when greatest(coalesce(p_exp,0),0) >= 100 then 'BRONZE II'
    else 'BRONZE I'
  end
$$;

create or replace function public.finalize_precision_competition_week(p_week_key text)
returns integer
language plpgsql
security definer
set search_path = public
as $$
declare
  rec record;
  inserted_uid uuid;
  reward_exp integer;
  reward_coins integer;
  new_exp bigint;
  finalized integer := 0;
begin
  if coalesce(trim(p_week_key),'') = '' then
    return 0;
  end if;

  -- One finalizer per weekly key, even when several clients connect at once.
  perform pg_advisory_xact_lock(hashtext('cc_precision_competition:' || p_week_key)::bigint);

  for rec in
    select user_id, total_points, insane_points, hard_points, normal_points, easy_points
    from public.precision_competition_weekly
    where week_key = p_week_key
    order by total_points desc, insane_points desc, hard_points desc,
             normal_points desc, easy_points desc, updated_at asc, user_id asc
    limit 3
  loop
    finalized := finalized + 1;
    reward_exp := case finalized when 1 then 5000 when 2 then 3000 else 1500 end;
    reward_coins := case finalized when 1 then 1500 when 2 then 750 else 400 end;
    inserted_uid := null;

    insert into public.precision_competition_awards(
      week_key,user_id,placement,exp_reward,spacecoins_reward,created_at
    ) values (
      p_week_key,rec.user_id,finalized,reward_exp,reward_coins,now()
    )
    on conflict (week_key,user_id) do nothing
    returning user_id into inserted_uid;

    -- Only the transaction that created the award credits EXP. This makes weekly
    -- finalization safe to call repeatedly and from multiple Edge Function requests.
    if inserted_uid is not null then
      update public.global_player_progress
      set exp = greatest(coalesce(exp,0),0) + reward_exp,
          exp_rank = public.cc_rank_for_exp(greatest(coalesce(exp,0),0) + reward_exp),
          updated_at = now()
      where user_id = inserted_uid
      returning exp into new_exp;

      update public.precision_competition_awards
      set exp_credited_at = now()
      where week_key = p_week_key and user_id = inserted_uid;
    end if;
  end loop;

  return finalized;
end;
$$;

create or replace function public.collect_precision_competition_rewards(p_user_id uuid)
returns table(
  award_count integer,
  spacecoins_reward integer,
  latest_week_key text,
  latest_placement integer,
  latest_exp_reward integer,
  global_exp bigint,
  global_rank text
)
language plpgsql
security definer
set search_path = public
as $$
declare
  rec record;
  n integer := 0;
  coins integer := 0;
  last_week text := '';
  last_place integer := 0;
  last_exp integer := 0;
  gexp bigint := 0;
  grank text := 'BRONZE I';
begin
  -- Lock all undelivered rows for this account so concurrent game instances cannot
  -- receive the same Space Coin award twice.
  for rec in
    select a.week_key, a.placement, a.exp_reward, a.spacecoins_reward, a.created_at
    from public.precision_competition_awards a
    where a.user_id = p_user_id and a.spacecoins_delivered_at is null
    order by a.created_at desc, a.placement asc
    for update
  loop
    n := n + 1;
    coins := coins + greatest(coalesce(rec.spacecoins_reward,0),0);
    if n = 1 then
      last_week := coalesce(rec.week_key,'');
      last_place := coalesce(rec.placement,0);
      last_exp := coalesce(rec.exp_reward,0);
    end if;
  end loop;

  if n > 0 then
    update public.precision_competition_awards
    set spacecoins_delivered_at = now(),
        claimed_at = coalesce(claimed_at, now())
    where user_id = p_user_id and spacecoins_delivered_at is null;
  end if;

  select greatest(coalesce(p.exp,0),0)::bigint,
         coalesce(nullif(p.exp_rank,''), public.cc_rank_for_exp(greatest(coalesce(p.exp,0),0)))
    into gexp, grank
  from public.global_player_progress p
  where p.user_id = p_user_id;

  award_count := n;
  spacecoins_reward := coins;
  latest_week_key := coalesce(last_week,'');
  latest_placement := coalesce(last_place,0);
  latest_exp_reward := coalesce(last_exp,0);
  global_exp := coalesce(gexp,0);
  global_rank := coalesce(grank,'BRONZE I');
  return next;
end;
$$;

revoke all on function public.cc_rank_for_exp(bigint) from public, anon, authenticated;
revoke all on function public.finalize_precision_competition_week(text) from public, anon, authenticated;
revoke all on function public.collect_precision_competition_rewards(uuid) from public, anon, authenticated;
grant execute on function public.cc_rank_for_exp(bigint) to service_role;
grant execute on function public.finalize_precision_competition_week(text) to service_role;
grant execute on function public.collect_precision_competition_rewards(uuid) to service_role;
