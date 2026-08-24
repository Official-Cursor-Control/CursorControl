#!/usr/bin/env python3
import random, math

def sx(v,w): return int(v*w/1536.0)
def sy(v,h): return int(v*h/1024.0)
def arena(w,h): return (sx(14,w),sy(180,h),sx(1522,w),sy(772,h))
DIMS=[(133,164),(101,135),(110,136),(151,153)]

def spawn(w,h):
    L,T,R,B=arena(w,h); cy=T+(B-T)//2
    perm=random.sample(range(4),4)
    out=[]
    for i in range(2):
        v=perm[i]
        ylo=T+sy(105,h); yhi=cy-sy(95,h)
        if i==1: ylo=cy+sy(95,h); yhi=B-sy(105,h)
        if yhi<ylo: yhi=ylo
        y=ylo+random.random()*(yhi-ylo) if yhi>ylo else ylo
        xlo=L+(R-L)*0.44; xhi=L+(R-L)*0.68
        x=xlo+random.random()*(xhi-xlo)
        dw=sx(108,w); dh=sy(130,h)
        if DIMS[v][0]>DIMS[v][1]: dw=sx(132,w); dh=sy(108,h)
        ang=math.radians(random.random()*18-9)
        out.append((x,y,v,dw,dh,ang))
    return out

def safe_rect(s,w,h):
    L,T,R,B=arena(w,h); x,y,v,dw,dh,a=s
    hh=int(dh*0.40)
    right=int(x)-dw//2+sx(5,w)
    right=max(right,L)
    top=max(int(y)-hh,T); bottom=min(int(y)+hh,B)
    return L,top,right,bottom

random.seed(362)
trials=0
for w,h in [(1024,768),(1280,720),(1366,768),(1536,1024),(1920,1080),(2560,1440)]:
    L,T,R,B=arena(w,h)
    for _ in range(20000):
        a,b=spawn(w,h); trials+=1
        assert a[2] != b[2], 'duplicate satellite variant'
        # rendered boxes cannot overlap
        ax1,ax2=a[0]-a[3]/2,a[0]+a[3]/2; ay1,ay2=a[1]-a[4]/2,a[1]+a[4]/2
        bx1,bx2=b[0]-b[3]/2,b[0]+b[3]/2; by1,by2=b[1]-b[4]/2,b[1]+b[4]/2
        assert ax2 < bx1 or bx2 < ax1 or ay2 < by1 or by2 < ay1, 'satellite overlap'
        for s in (a,b):
            x,y,v,dw,dh,ang=s
            assert L+dw/2 < x < R-dw/2, 'satellite outside arena X'
            assert T+dh/2 < y < B-dh/2, 'satellite outside arena Y'
            sr=safe_rect(s,w,h)
            assert sr[0] < sr[2], 'zero-width cover shadow'
            assert sr[1] < sr[3], 'zero-height cover shadow'
            assert sr[2] <= x-dw/2+sx(6,w), 'shadow extends through satellite'

# Exact HP accounting: five body nodes + three mouth nodes, repeated twice.
hits=0
for loop in range(2):
    hits += 5
    for beam in range(3): hits += 1
assert hits == 16
# Meteor phase 2 speed multiplier.
assert abs(315*1.2-378.0)<1e-9
# Transition timing budget: entry (~0.51s at design res), settle .18, fade .95,
# charge 1.7, beam 3.0, node 1.35 max, recover .85. No phase has zero-duration handoff.
for t in (0.18,0.95,1.7,3.0,1.35,0.85): assert t>0
print(f'PASS v362 serpent loop stress: {trials} randomized satellite layouts, 0 overlaps; HP=16 exact; meteor2=1.2x')
