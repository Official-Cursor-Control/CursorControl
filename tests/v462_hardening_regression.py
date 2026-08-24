from pathlib import Path
import re, sys
root=Path(__file__).resolve().parents[1]
checks=[]
def ck(name, cond):
    checks.append((name,bool(cond))); print(('PASS' if cond else 'FAIL')+' - '+name)
main=(root/'main.go').read_text()
dispatch=(root/'main_thread_dispatch.go').read_text()
rel=(root/'reliability.go').read_text()
relw=(root/'reliability_windows.go').read_text()
cloud=(root/'afk_cloud_sync.go').read_text()
bank=(root/'afk_starbit_bank.go').read_text()
exchange=(root/'afk_exchange.go').read_text()
online=(root/'online_sync.go').read_text()
build=(root/'build_identity.go').read_text()
ck('v462 identity', 'clientBuildVersion = "v462"' in build and 'clientBuildNumber  = 402' in build)
ck('main-thread task Win32 message exists', 'WM_MAIN_THREAD_TASK' in main and 'processMainThreadTasks()' in main)
ck('main-thread dispatcher exists', 'func postMainThreadTask(' in dispatch and 'func postMainThreadTaskAndWait(' in dispatch)
ck('AFK cloud applies state on main thread', 'postMainThreadTask(func() {' in cloud and 'afkApplyCloudState(out.State, out)' in cloud)
ck('Starbit bank applies response on main thread', 'postMainThreadTask(func() {' in bank and 'afkBankApply(out)' in bank)
ck('Starbase exchange applies rewards on main thread', 'postMainThreadTask(func() {' in exchange and 'gameMeta.AFKStarbits = serverTotal' in exchange)
ck('profile sync captures coherent main-thread snapshot', 'postMainThreadTaskAndWait(func() {' in online and 'payload = playerProfileSyncPayload(true)' in online)
ck('profile sync response applies on main thread', re.search(r'json\.Unmarshal\(body, &env\).*?postMainThreadTask\(func\(\) \{.*?gameMeta\.UnlockedShips', online, re.S) is not None)
ck('atomic writes are serialized', 'var atomicWriteMu sync.Mutex' in rel and 'atomicWriteMu.Lock()' in rel)
ck('Windows atomic replace uses MoveFileExW', 'MoveFileExW' in relw and 'moveFileReplaceExisting|moveFileWriteThrough' in relw)
ck('last-known-good backup retained', 'backup := path + ".bak"' in rel and 'atomicReplaceFile(backupTmp, backup)' in rel)
ck('metadata recovery helper retained', 'func readJSONWithRecovery' in rel)
ck('leaderboard uses recovery loader', 'readJSONWithRecovery(leaderboardFile, &leaderboard)' in online)
ck('auth session uses recovery loader', 'readJSONWithRecovery(authSessionFile, &sess)' in online)
ck('logout deletes auth backup', 'os.Remove(path + ".bak")' in online)
ck('HUD config uses recovery loader', 'readJSONWithRecovery(path, &cfg)' in main)
for name,res in checks:
    pass
print(f'\n{sum(v for _,v in checks)}/{len(checks)} v462 hardening assertions passed')
sys.exit(0 if all(v for _,v in checks) else 1)
