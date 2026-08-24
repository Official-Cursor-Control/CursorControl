-- Cursor Control v376: rank-gated Global Profile customization.
alter table public.global_player_profiles
  add column if not exists selected_profile_font integer not null default 0,
  add column if not exists selected_profile_primary_colour integer not null default 0,
  add column if not exists selected_profile_secondary_colour integer not null default 1,
  add column if not exists selected_profile_animation integer not null default 0,
  add column if not exists sentinel_defeats integer not null default 0,
  add column if not exists serpent_defeats integer not null default 0,
  add column if not exists array_defeats integer not null default 0;

alter table public.global_player_profiles
  drop constraint if exists global_player_profiles_selected_profile_font_check,
  add constraint global_player_profiles_selected_profile_font_check check (selected_profile_font between 0 and 7),
  drop constraint if exists global_player_profiles_selected_profile_primary_colour_check,
  add constraint global_player_profiles_selected_profile_primary_colour_check check (selected_profile_primary_colour between 0 and 7),
  drop constraint if exists global_player_profiles_selected_profile_secondary_colour_check,
  add constraint global_player_profiles_selected_profile_secondary_colour_check check (selected_profile_secondary_colour between 0 and 7),
  drop constraint if exists global_player_profiles_selected_profile_animation_check,
  add constraint global_player_profiles_selected_profile_animation_check check (selected_profile_animation between 0 and 3),
  drop constraint if exists global_player_profiles_sentinel_defeats_check,
  add constraint global_player_profiles_sentinel_defeats_check check (sentinel_defeats >= 0),
  drop constraint if exists global_player_profiles_serpent_defeats_check,
  add constraint global_player_profiles_serpent_defeats_check check (serpent_defeats >= 0),
  drop constraint if exists global_player_profiles_array_defeats_check,
  add constraint global_player_profiles_array_defeats_check check (array_defeats >= 0);
