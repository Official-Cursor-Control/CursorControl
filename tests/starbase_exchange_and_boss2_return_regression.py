from pathlib import Path
r=Path(__file__).resolve().parents[1]
ex=(r/'afk_exchange.go').read_text(errors='ignore')
b2=(r/'survival_boss2.go').read_text(errors='ignore')
edge=(r/'backend/starbase-exchange/index.ts').read_text(errors='ignore')
checks=[]
def ck(n,c): checks.append((n,bool(c)))
ck('SpaceCoin bundles exact', all(x in ex for x in ['{100_000_000, 50}','{500_000_000, 260}','{1_000_000_000, 550}','{5_000_000_000, 3000}']))
ck('EXP bundles exact', all(x in ex for x in ['{100_000_000, 10}','{500_000_000, 55}','{1_000_000_000, 120}','{5_000_000_000, 650}']))
ck('Exchange server checks active device lease', 'active_game_sessions' in edge and 'session_nonce' in edge)
ck('Exchange uses revision compare-and-swap', '.eq("revision",currentRevision)' in edge)
ck('EXP grant is server-side', 'global_player_progress' in edge and 'exp_rank' in edge)
ck('Second rush has independent return flash', 'survivalBoss2DodgeReturnFlashAt = now' in b2)
ck('Return flash renders while second run active', 'returnFlash :=' in b2 and 'normalFlash || returnFlash' in b2)
ck('Second rush is opposite side', 'survivalBoss2OppositeDodgeSide(firstSide)' in b2)
for n,ok in checks: print(('PASS' if ok else 'FAIL')+' - '+n)
print(f'\n{sum(ok for _,ok in checks)}/{len(checks)} exchange/return-flash assertions passed')
raise SystemExit(0 if all(ok for _,ok in checks) else 1)
