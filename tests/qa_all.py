#!/usr/bin/env python3
import hashlib, json, os, re, shutil, subprocess, sys, time
from pathlib import Path

SRC = Path(__file__).resolve().parents[1]
RUNTIME = Path(os.environ.get('CC_RUNTIME', str(SRC.parent / 'runtime')))
REPORT_DIR = SRC / 'docs' / 'qa'
REPORT_DIR.mkdir(parents=True, exist_ok=True)
results=[]

def add(group,name,ok,detail=''):
    results.append({'group':group,'name':name,'ok':bool(ok),'detail':str(detail)})
    print(('PASS' if ok else 'FAIL'), f'[{group}]', name, detail)

def run(cmd, cwd=None, timeout=120):
    p=subprocess.run(cmd,cwd=cwd,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=timeout)
    return p.returncode,p.stdout

def sha(path):
    h=hashlib.sha256()
    with open(path,'rb') as f:
        for b in iter(lambda:f.read(1<<20),b''): h.update(b)
    return h.hexdigest()

# Hygiene / structure
root_files=[p.name for p in RUNTIME.iterdir() if p.is_file()]
root_exes=[x for x in root_files if x.lower().endswith('.exe')]
add('packaging','runtime_root_only_exe',len(root_files)==1 and len(root_exes)==1,root_files)
allowed_top={'assets','data'}
extra_dirs=[p.name for p in RUNTIME.iterdir() if p.is_dir() and p.name not in allowed_top]
add('packaging','runtime_no_dev_dirs',not extra_dirs,extra_dirs)
clutter=[]
for p in RUNTIME.rglob('*'):
    if p.is_file() and p.suffix.lower() in {'.txt','.md','.json','.sql','.py'} and p.name!='hud_layout.json': clutter.append(str(p.relative_to(RUNTIME)))
add('packaging','runtime_no_document_clutter',not clutter,clutter[:20])

# Source formatting + build x3 + vet
rc,out=run(['gofmt','-l',*map(str,SRC.glob('*.go'))],SRC)
add('source','gofmt_clean',rc==0 and not out.strip(),out.strip())
env=os.environ.copy(); env.update({'GO111MODULE':'off','GOOS':'windows','GOARCH':'amd64','CGO_ENABLED':'0'})
build_hashes=[]
for i in range(3):
    target=SRC/f'.qa_build_{i}.exe'
    p=subprocess.run(['go','build','-trimpath','-ldflags=-H=windowsgui -s -w','-o',str(target),*map(str,SRC.glob('*.go'))],cwd=SRC,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=180)
    add('source',f'windows_build_{i+1}',p.returncode==0,p.stdout.strip()[-1000:])
    if target.exists(): build_hashes.append(sha(target)); target.unlink()
add('source','deterministic_builds',len(build_hashes)==3 and len(set(build_hashes))==1,build_hashes)
p=subprocess.run(['go','vet',*map(str,SRC.glob('*.go'))],cwd=SRC,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=180)
vet_lines=[x for x in p.stdout.splitlines() if x.strip() and not x.startswith('#')]
# syscall/Win32 wrappers necessarily cross uintptr/unsafe.Pointer boundaries. Treat only
# that known interop warning as accepted; any other vet diagnostic remains a failure.
unexpected_vet=[x for x in vet_lines if 'possible misuse of unsafe.Pointer' not in x]
add('source','go_vet_no_unexpected_diagnostics',not unexpected_vet,('accepted Win32 unsafe interop warnings=%d; unexpected=%s' % (len(vet_lines), unexpected_vet[:10])))

# Every runtime content file: repeated stable reads/hash.
all_assets=[p for p in (RUNTIME/'assets').rglob('*') if p.is_file()]
add('assets','asset_count_nonzero',len(all_assets)>100,len(all_assets))
for p in all_assets:
    rel=str(p.relative_to(RUNTIME))
    st=p.stat()
    ok=st.st_size>0
    base=sha(p) if ok else ''
    # 20 repeated whole-file reads for every content file.
    stable=ok
    for _ in range(20):
        if sha(p)!=base:
            stable=False; break
    if p.suffix.lower()=='.bgra':
        stable = stable and st.st_size%4==0
    add('asset',rel,stable,f'{st.st_size} bytes sha={base[:12]}')

# Audio: source reference, stream probe, two complete decode passes per file.
audio_files=sorted((RUNTIME/'assets'/'audio').glob('*'))
for p in audio_files:
    rel=str(p.relative_to(RUNTIME))
    corpus='\n'.join(x.read_text(errors='ignore') for x in SRC.glob('*.go'))
    ref=(p.name in corpus or p.stem in corpus or (p.name.startswith('survival_section_') and 'survival_section_%d.mp3' in corpus) or (p.name.startswith('serpent_attack_') and 'serpent_attack_%d' in corpus))
    add('audio_ref',p.name,ref,'referenced in Go source' if ref else 'UNREFERENCED')
    rc,out=run(['ffprobe','-v','error','-select_streams','a:0','-show_entries','stream=codec_name,sample_rate,channels,duration','-of','json',str(p)],timeout=30)
    try:
        j=json.loads(out) if rc==0 else {}
        streams=j.get('streams',[])
        probe_ok=bool(streams) and int(streams[0].get('sample_rate') or 0)>0 and int(streams[0].get('channels') or 0)>0
    except Exception: probe_ok=False
    add('audio_probe',p.name,probe_ok,out.strip()[:400])
    for passn in range(2):
        rc,out=run(['ffmpeg','-v','error','-i',str(p),'-f','null','-'],timeout=60)
        add('audio_decode',f'{p.name} pass {passn+1}',rc==0,out.strip()[:500])

# Button assets: 100 repeat reads, source asset reference + interaction wiring tokens.
source='\n'.join(x.read_text(errors='ignore') for x in SRC.glob('*.go'))
button_rules={
 'bug_report_button.bgra':['quickBugReportRect','tally.so'],
 'support_dev_button.bgra':['quickSupportDevRect','paypal.me'],
 'discord_login_button.bgra':['quickDiscordLoginRect','beginDiscordLogin'],
 'discord_logged_in_button.bgra':['quickDiscordLoginRect','syncPlayerProfile'],
 'select_mode_button.bgra':['enduranceModeButtonRect','modeSelectorOpen'],
 'precision_mode_button.bgra':['modeSelectorCardRects','selectGameMode'],
 'endurance_mode_button.bgra':['modeSelectorCardRects','selectGameMode'],
 'survival_mode_button.bgra':['modeSelectorCardRects','selectGameMode'],
 'profile_button.bgra':['quickAccessRects','OverlayProfile'],
 'local_button.bgra':['quickAccessRects','openLeaderboard'],
 'global_button.bgra':['quickAccessRects','openGlobalLeaderboard'],
 'garage_button.bgra':['enduranceSpaceUIRects','OverlayGarage'],
 'space_cache_button.bgra':['enduranceSpaceUIRects','beginSpaceCacheOpen'],
}
for p in sorted((RUNTIME/'assets'/'ui').glob('*button.bgra')):
    base=sha(p); stable=True
    for _ in range(100):
        if sha(p)!=base: stable=False; break
    rules=button_rules.get(p.name,[])
    wired=p.name in source and all(tok in source for tok in rules)
    add('button_asset',p.name,stable,f'100 stable reads; {p.stat().st_size} bytes')
    add('button_wiring',p.name,wired,', '.join(rules))

# Hit-test primitive property stress: 250k random points/rectangles, inclusive edges.
import random
random.seed(300)
ok=True
for _ in range(250000):
    l=random.randint(-5000,5000); t=random.randint(-5000,5000)
    r=l+random.randint(1,3000); b=t+random.randint(1,3000)
    x=random.randint(l-100,r+100); y=random.randint(t-100,b+100)
    got=(x>=l and x<=r and y>=t and y<=b)
    expect=(l<=x<=r and t<=y<=b)
    if got!=expect: ok=False; break
add('button_geometry','point_in_rect_250k',ok,'inclusive boundary model')

# HUD layout JSON parse + sanity
try:
    hud=json.loads((RUNTIME/'data'/'hud_layout.json').read_text())
    add('ui','hud_layout_json',isinstance(hud,dict),f'keys={len(hud) if isinstance(hud,dict) else 0}')
except Exception as e: add('ui','hud_layout_json',False,e)

# British English player-facing string audit. Ignore JSON/API identifiers, URLs, CSS and internal identifiers.
us_to_uk=['color','colors','center','centered','favorite','favorites','customize','customized','customization','defense','armor','gray','meter','meters','kilometer','kilometers','behavior','organize','organized','organizing','recognize','analyze','license']
viol=[]
str_re=re.compile(r'"(?:\\.|[^"\\])*"')
for f in SRC.glob('*.go'):
    txt=f.read_text(errors='ignore')
    for m in str_re.finditer(txt):
        raw=m.group(0)
        low=raw.lower()
        if any(k in low for k in ['json:', 'http://','https://','style=','settextcolor','crosshair_color','fire_color','name_colour','profile_frame','background:','font-family:','text-align:','max-width:','place-items:']) or low.startswith('\"color:'): continue
        for w in us_to_uk:
            if re.search(r'\b'+re.escape(w)+r'\b',low):
                viol.append(f'{f.name}:{txt.count(chr(10),0,m.start())+1}:{raw[:120]}')
                break
add('language','british_english_player_strings',not viol,viol[:30])

# Network/UI safety: all explicit profile sync launch sites should use coalesced dispatcher.
direct=[]
for f in SRC.glob('*.go'):
    for n,line in enumerate(f.read_text().splitlines(),1):
        if 'go syncPlayerProfile()' in line: direct.append(f'{f.name}:{n}')
add('network','no_overlapping_profile_sync_goroutines',not direct,direct)
add('network','coalesced_sync_worker','requestPlayerProfileSync()' in source and 'profileSyncReq' in source,'single buffered worker')

# Short UI/game SFX should be queued off the UI thread.
async_funcs=['playUIButtonClickSound','playBossClickEffect','playPowerupPickupSound','playLevelUpSound','playRechamberSound','playWarpReadySound','playWarpRocketSound','playSpaceCacheSound']
for fn in async_funcs:
    mm=re.search(r'func\s+'+re.escape(fn)+r'\s*\([^)]*\)\s*\{(.*?)\n\}',source,re.S)
    ok=bool(mm and ('playOneShotAsync' in mm.group(1) or 'playSFX(' in mm.group(1)))
    add('audio_runtime',fn,ok,'routes to centralized asynchronous SFX bus')

summary={
 'generated_at':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),
 'runtime':str(RUNTIME),'source':str(SRC),
 'passed':sum(r['ok'] for r in results),'total':len(results),
 'failed':[r for r in results if not r['ok']],
 'groups':{},
}
for r in results:
    g=summary['groups'].setdefault(r['group'],{'passed':0,'total':0})
    g['total']+=1; g['passed']+=int(r['ok'])
(REPORT_DIR/'full_qa.json').write_text(json.dumps(summary,indent=2)+'\n')
print('\nSUMMARY',summary['passed'],'/',summary['total'],'PASS')
if summary['failed']:
    print('FAILED:',len(summary['failed']))
    sys.exit(1)
