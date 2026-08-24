from pathlib import Path
R=Path(__file__).resolve().parents[1]
comp=(R/'competition_v320.go').read_text()
hub=(R/'backend/competitive-hub/index.ts').read_text()
submit=(R/'backend/submit-score/index.ts').read_text()
mig=(R/'migrations/SUPABASE_v453_WEEKLY_RESET_MONDAY.sql').read_text()
checks={
 'client fallback Monday': 'daysSinceMonday := (int(local.Weekday()) + 6) % 7' in comp,
 'hub Monday': 'daysSinceMonday=(dow+6)%7' in hub and 'Sunday is the final day' in hub,
 'submit Monday': 'daysSinceMonday=(dow+6)%7' in submit and 'Sunday remains active' in submit,
 'database Monday': '(extract(dow from local_now)::int + 6) % 7' in mig,
 'compensation hidden from all-time': '.like("week_key","VN-%")' in hub,
}
for name,ok in checks.items():
 print(('PASS' if ok else 'FAIL'), name)
if not all(checks.values()): raise SystemExit(1)
