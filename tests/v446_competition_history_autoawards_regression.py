from pathlib import Path
R=Path(__file__).resolve().parents[1]
comp=(R/'competition_v320.go').read_text()
ui=(R/'ui_leaderboards_profiles.go').read_text()
hub=(R/'backend/competitive-hub/index.ts').read_text()
mig=(R/'migrations/SUPABASE_v446_COMPETITION_HISTORY_AUTOAWARDS.sql').read_text()
main=(R/'main.go').read_text()
online=(R/'online_sync.go').read_text()
checks=[]
def ck(name, cond):
    checks.append((name,bool(cond)))
    print(('PASS' if cond else 'FAIL'),name)
ck('competition has current/previous/alltime views', all(x in comp for x in ['competitionViewCurrent','competitionViewPrevious','competitionViewAllTime']))
ck('history buttons rendered', all(x in comp for x in ['THIS WEEK','PREVIOUS WINNERS','ALL-TIME WINS']))
ck('history view clickable', 'competitionViewAt' in main and 'selectCompetitionView(v)' in main)
ck('previous winners endpoint', 'view==="previous"' in hub and 'previous_ranking_failed' in hub)
ck('all-time weekly winners endpoint', 'view==="alltime"' in hub and 'wins:r.wins' in hub)
ck('reward values updated', all(x in hub for x in ['{exp:5000,coins:1500}','{exp:3000,coins:750}','{exp:1500,coins:400}']))
ck('server automatic finalization', 'finalize_precision_competition_week' in hub and 'exp_credited_at' in mig)
ck('automatic pending coin delivery', 'collect_precision_competition_rewards' in hub and 'spacecoins_delivered_at' in mig)
ck('client uses automatic reward sync action', 'sync_precision_rewards' in comp and 'claim_precision_reward' not in comp)
ck('reward sync runs on stored login', 'go autoSyncPrecisionCompetitionRewards()' in online)
ck('previous winners reward column', 'AUTOMATIC REWARD' in ui and 'XP + %d SC' in ui)
ck('all-time winners columns', all(x in ui for x in ['"WINS"','"PODIUMS"','"1ST"','"2ND"','"3RD"']))
ck('guide explains automatic credit', 'EXP IS AUTO-CREDITED' in ui)
failed=[n for n,v in checks if not v]
print(f'\n{len(checks)-len(failed)}/{len(checks)} passed')
if failed:
    raise SystemExit('FAILED: '+', '.join(failed))
