from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
bank=(ROOT/'afk_starbit_bank.go').read_text()
main=(ROOT/'main.go').read_text()
cloud=(ROOT/'afk_cloud_sync.go').read_text()

def ck(n,c):
    assert c,n
    print('PASS',n)

ck('15_second_bank_sync','afkBankSyncInterval   = 15 * time.Second' in bank)
ck('bank_hud_hides_live','// LIVE ' not in bank and 'formatAFKNumber(bank)' in bank)
ck('bank_hud_no_countdown','BANK // NEXT' not in bank)
ck('client_response_no_lifetime','LifetimeEarned' not in bank)
ck('grant_no_lifetime_counter','AFKLifetimeStarbits' not in bank)
ck('save_model_no_lifetime_starbits','AFKLifetimeStarbits' not in main)
ck('cloud_response_no_lifetime','LifetimeStarbits' not in cloud and 'RequestLifetimeStarbits' not in cloud)
hud=(ROOT/'afk_bottom_hud.go').read_text()
ck('top_hud_uses_bank_not_current','{bankLabel, bankValue' in hud and '{"STARBITS", formatAFKNumber(gameMeta.AFKStarbits)' not in hud)
print('PASS v424 Starbit Bank simplification 8/8')
