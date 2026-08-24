#!/usr/bin/env python3
import pathlib,re,sys,statistics
p=pathlib.Path(sys.argv[1] if len(sys.argv)>1 else 'logs/run_performance.log')
if not p.exists(): print('No performance log yet:',p); raise SystemExit(0)
rows=[]
for line in p.read_text(errors='ignore').splitlines():
    d={k:v.strip('"') for k,v in re.findall(r'(\w+)=((?:"[^"]*")|\S+)',line)}
    try:
        rows.append({k:float(d[k]) for k in ('fps','frame_ms','frame_max_ms','heap_mb','peak_heap_mb','alloc_mb_s') if k in d}|{'event':d.get('event','')})
    except ValueError: pass
if not rows: print('No parsable rows'); raise SystemExit(0)
for k in ('fps','frame_ms','frame_max_ms','heap_mb','peak_heap_mb','alloc_mb_s'):
    vals=[r[k] for r in rows if k in r]
    if vals: print(k,'median',round(statistics.median(vals),3),'p95-ish',round(sorted(vals)[int(.95*(len(vals)-1))],3),'max',round(max(vals),3))
print('NOTE: performance values are diagnostic; compare the same hardware/build/section before making optimizations.')
