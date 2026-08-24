from pathlib import Path
import re, sys
ROOT=Path(__file__).resolve().parents[1]
checks=[]
def ck(name, cond, detail=''):
    checks.append((name,bool(cond),detail))

bank=(ROOT/'afk_starbit_bank.go').read_text()
cloud=(ROOT/'afk_cloud_sync.go').read_text()
sec=(ROOT/'afk_security.go').read_text()
ex=(ROOT/'afk_exchange.go').read_text()
sing=(ROOT/'afk_singularity.go').read_text()
allgo='\n'.join(p.read_text(errors='ignore') for p in ROOT.glob('*.go'))

ck('dedicated_bank_endpoint', '/functions/v1/starbit-bank' in bank)
ck('client_never_posts_balance_to_bank', '"starbits"' not in re.search(r'func afkBankPost\(.*?\n}', bank, re.S).group(0))
ck('monotonic_earned_counter', 'afkBankEarnedTotal = saturatingAdd' in bank)
ck('monotonic_spent_counter', 'afkBankSpentTotal = saturatingAdd' in bank)
ck('central_grant_helper', 'func afkGrantStarbits' in bank)
ck('central_spend_records_ledger', 'afkBankRecordSpend(cost)' in sec)
ck('generic_cloud_drops_starbits', 'delete(state, "afk_starbits")' in cloud)
ck('generic_cloud_drops_lifetime', 'delete(state, "afk_lifetime_starbits")' in cloud)
ck('generic_cloud_no_balance_payload', 'payload["starbits"]' not in cloud)
ck('generic_cloud_no_lifetime_payload', 'payload["lifetime_starbits"]' not in cloud)
ck('bank_required_for_starbase', 'OPENING STARBIT BANK' in cloud and 'bankReady' in cloud)
ck('bank_tick_runs_with_cloud_tick', 'afkBankTick(now)' in cloud)
ck('lease_reset_resets_bank', 'afkBankReset()' in cloud)
ck('clean_exit_bank_flush', 'afkBankFlushOnExit()' in allgo)
ck('15_second_bank_heartbeat', 'afkBankSyncInterval   = 15 * time.Second' in bank)
ck('bank_hud_hides_live', 'BANK // NEXT' not in bank and '// LIVE ' not in bank)
ck('bank_hud_visible', 'afkBankHUDText' in (ROOT/'afk_bottom_hud.go').read_text())
ck('exchange_reserves_bank_lane', 'afkBankInFlight = true' in ex and 'STARBIT BANK IS SYNCING' in ex)
ck('exchange_updates_bank_snapshot', 'afkBankProtected = serverBank' in ex)
ck('exchange_forces_logout_on_lost_session', 'out.SessionLost || out.ForceLogout' in ex)
ck('offline_generation_disabled', 'Starbase production is active-session only' in sing)

# Every direct positive Starbit mutation outside the single grant helper is a regression.
viol=[]
for p in ROOT.glob('*.go'):
    txt=p.read_text(errors='ignore')
    if p.name=='afk_starbit_bank.go':
        continue
    for i,line in enumerate(txt.splitlines(),1):
        if re.search(r'AFKStarbits\s*(\+=|=\s*saturatingAdd)', line):
            viol.append(f'{p.name}:{i}:{line.strip()}')
ck('all_gains_centralized', not viol, '; '.join(viol))

# All direct subtraction should be centralized in afkSpendStarbits.
viol2=[]
for p in ROOT.glob('*.go'):
    txt=p.read_text(errors='ignore')
    for i,line in enumerate(txt.splitlines(),1):
        if 'AFKStarbits -=' in line and p.name!='afk_security.go':
            viol2.append(f'{p.name}:{i}:{line.strip()}')
ck('all_normal_spends_centralized', not viol2, '; '.join(viol2))

for n,ok,d in checks:
    print(('PASS' if ok else 'FAIL'), n, d)
failed=[x for x in checks if not x[1]]
print(f'SUMMARY {len(checks)-len(failed)} / {len(checks)} PASS')
sys.exit(1 if failed else 0)
