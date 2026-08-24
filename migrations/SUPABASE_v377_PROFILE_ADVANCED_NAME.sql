alter table public.global_player_profiles
  add column if not exists selected_profile_name_font integer not null default 0,
  add column if not exists profile_name_shadow boolean not null default true,
  add column if not exists profile_shadow_colour integer not null default 0,
  add column if not exists profile_gradient_vertical boolean not null default false;

alter table public.global_player_profiles
  drop constraint if exists global_player_profiles_selected_profile_name_font_check,
  add constraint global_player_profiles_selected_profile_name_font_check check (selected_profile_name_font between 0 and 12),
  drop constraint if exists global_player_profiles_selected_profile_primary_colour_check,
  add constraint global_player_profiles_selected_profile_primary_colour_check check (selected_profile_primary_colour between 0 and 11),
  drop constraint if exists global_player_profiles_selected_profile_secondary_colour_check,
  add constraint global_player_profiles_selected_profile_secondary_colour_check check (selected_profile_secondary_colour between 0 and 11),
  drop constraint if exists global_player_profiles_profile_shadow_colour_check,
  add constraint global_player_profiles_profile_shadow_colour_check check (profile_shadow_colour between 0 and 11);
