import cv2, numpy as np, math, os, subprocess
from pathlib import Path

SRC = Path('/mnt/data/Intro 1.mp4')
RAW = Path('/mnt/data/konggames_intro_v452_video.mp4')
OUT = Path('/mnt/data/konggames_intro_v452.mp4')

cap = cv2.VideoCapture(str(SRC))
fps = cap.get(cv2.CAP_PROP_FPS) or 60.0
w = int(cap.get(cv2.CAP_PROP_FRAME_WIDTH)); h = int(cap.get(cv2.CAP_PROP_FRAME_HEIGHT))
frames=[]
while True:
    ok, fr = cap.read()
    if not ok: break
    frames.append(fr)
cap.release()
print('frames',len(frames),'fps',fps,'size',w,h)

# Geometric K/O/N/G line paths, normalized around each letter center.
def letter_paths(letter, cx, cy, sx=1.0, sy=1.0):
    def P(points): return [(int(cx+x*sx), int(cy+y*sy)) for x,y in points]
    if letter=='K':
        return [P([(-64,-46),(-64,46)]), P([(-64,5),(54,-46)]), P([(-12,-16),(56,46)])]
    if letter=='O':
        return [P([(-48,-46),(42,-46),(62,-26),(62,26),(42,46),(-48,46),(-64,28),(-64,-27),(-48,-46)])]
    if letter=='N':
        return [P([(-62,46),(-62,-46),(-42,-46),(59,30),(59,-46)]), P([(59,-46),(59,46),(39,46),(-42,-30),(-42,46)])]
    if letter=='G':
        return [P([(55,-26),(39,-46),(-42,-46),(-62,-26),(-62,27),(-42,46),(37,46),(58,25),(58,4),(5,4)]), P([(5,4),(58,4),(58,33)])]
    return []

letters=['K','O','N','G']
centers=[(768,230),(768,365),(768,500),(768,635)]
paths=[]
for L,(cx,cy) in zip(letters,centers):
    paths.append(letter_paths(L,cx,cy,1.05,1.0))

def path_length(path):
    return sum(math.hypot(path[i+1][0]-path[i][0], path[i+1][1]-path[i][1]) for i in range(len(path)-1))

def draw_partial_poly(img, path, frac, color, thickness, lineType=cv2.LINE_AA):
    if len(path)<2 or frac<=0: return
    frac=min(1.0,frac)
    total=path_length(path)
    remain=total*frac
    for i in range(len(path)-1):
        a=np.array(path[i],float); b=np.array(path[i+1],float)
        seg=float(np.linalg.norm(b-a))
        if seg<=0: continue
        if remain>=seg:
            cv2.line(img, tuple(a.astype(int)), tuple(b.astype(int)), color, thickness, lineType)
            remain-=seg
        else:
            p=a+(b-a)*(remain/seg)
            cv2.line(img, tuple(a.astype(int)), tuple(p.astype(int)), color, thickness, lineType)
            break

def smoothstep(a,b,x):
    if x<=a: return 0.0
    if x>=b: return 1.0
    t=(x-a)/(b-a)
    return t*t*(3-2*t)

def remove_old_center(frame):
    out=frame.copy()
    x0,x1=575,950; y0,y1=60,770
    roi=out[y0:y1,x0:x1]
    gray=cv2.cvtColor(roi,cv2.COLOR_BGR2GRAY)
    # Remove only the bright geometric strokes/glow from the old opening mark.
    mask=np.zeros_like(gray)
    mask[gray>58]=255
    # retain tiny speckles by requiring connected-ish strokes through dilation/opening
    mask=cv2.dilate(mask,np.ones((3,3),np.uint8),iterations=1)
    # Slightly expand to include the bloom around the line work.
    mask=cv2.GaussianBlur(mask,(7,7),0)
    mask=(mask>20).astype(np.uint8)*255
    clean=cv2.inpaint(roi,mask,5,cv2.INPAINT_TELEA)
    out[y0:y1,x0:x1]=clean
    return out

processed=[]
for i,orig in enumerate(frames):
    t=i/fps
    if t>=1.0:
        processed.append(orig)
        continue
    clean=remove_old_center(orig)
    layer=np.zeros_like(clean)
    glow=np.zeros_like(clean)
    # Progressive top-to-bottom construction like Intro 1's original opening pattern.
    global_p=smoothstep(0.08,0.76,t)
    for li,letter_set in enumerate(paths):
        # each letter gets a staggered quarter of the total build
        lp=np.clip((global_p*4.0-li),0.0,1.0)
        # draw all constituent paths, each with slight sequential staggering
        for pi,p in enumerate(letter_set):
            pp=np.clip(lp*1.35-pi*0.16,0.0,1.0)
            draw_partial_poly(glow,p,pp,(185,185,185),9)
            draw_partial_poly(layer,p,pp,(218,218,218),2)
        # faint offset echo reinforces the abstract/pattern feel without becoming normal typography
        if lp>0.32:
            echo_alpha=min(1.0,(lp-0.32)/0.4)
            for p in letter_set:
                pe=[(x+4,y-2) for x,y in p]
                draw_partial_poly(layer,pe,min(1,lp), (65,65,65),1)
    glow=cv2.GaussianBlur(glow,(0,0),5.5)
    # flicker exactly in the restrained, near-monochrome style of the source
    flick=0.92 + 0.08*math.sin(t*2*math.pi*24.0)
    if 0.43<t<0.47 or 0.78<t<0.805:
        flick*=0.58
    comp=cv2.addWeighted(clean,1.0,glow,0.34*flick,0)
    comp=cv2.addWeighted(comp,1.0,layer,0.95*flick,0)

    # A few horizontal scan echoes clipped to the KONG mark, matching source pattern language.
    if t>0.30:
        echo=comp.copy()
        for yy in range(185,690,18):
            alpha=0.06*(0.5+0.5*math.sin(yy*0.17+t*18))
            cv2.line(echo,(650,yy),(885,yy),(120,120,120),1,cv2.LINE_AA)
        comp=cv2.addWeighted(comp,0.97,echo,0.03,0)

    # Blend all the way back to the untouched Intro 1 at the 1.000s boundary.
    keep=1.0-smoothstep(0.86,1.0,t)
    comp=cv2.addWeighted(comp,keep,orig,1.0-keep,0)
    processed.append(comp)

# Write high-quality intermediate using ffmpeg pipe -> H.264 so dimensions/fps match source exactly.
proc=subprocess.Popen([
    'ffmpeg','-y','-loglevel','error','-f','rawvideo','-pix_fmt','bgr24','-s',f'{w}x{h}','-r',str(fps),'-i','-',
    '-an','-c:v','libx264','-preset','slow','-crf','10','-pix_fmt','yuv420p','-movflags','+faststart',str(RAW)
],stdin=subprocess.PIPE)
for fr in processed:
    proc.stdin.write(fr.tobytes())
proc.stdin.close(); rc=proc.wait()
if rc: raise SystemExit(rc)
# Mux the original Intro 1 AAC bitstream without re-encoding audio.
subprocess.check_call([
    'ffmpeg','-y','-loglevel','error','-i',str(RAW),'-i',str(SRC),'-map','0:v:0','-map','1:a:0?',
    '-c:v','copy','-c:a','copy','-shortest','-movflags','+faststart',str(OUT)
])
print(OUT)
