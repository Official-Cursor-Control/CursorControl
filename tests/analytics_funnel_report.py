#!/usr/bin/env python3
import json, sys, collections, pathlib
p=pathlib.Path(sys.argv[1] if len(sys.argv)>1 else 'logs/analytics_local.jsonl')
if not p.exists():
    print('No analytics log yet:',p); raise SystemExit(0)
events=[]
for line in p.read_text(errors='ignore').splitlines():
    try: events.append(json.loads(line))
    except Exception: pass
c=collections.Counter(e.get('event') for e in events)
fail=collections.Counter((e.get('fields') or {}).get('reason') for e in events if e.get('event')=='run_failed')
modes=collections.Counter((e.get('fields') or {}).get('mode') or e.get('mode') for e in events if e.get('event') in ('run_started','mode_selected'))
print('events',len(events))
for k in ['session_started','tutorial_completed','run_started','run_completed','run_failed','achievement_unlocked','space_cache_open_started','profile_customized','boss_attempted']:
    print(f'{k:26s}',c[k])
print('failure categories',dict(fail))
print('mode exposure',dict(modes))
if c['run_started']:
    print('run completion ratio',round(c['run_completed']/c['run_started'],3))
