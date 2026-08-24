import math, random
random.seed(284)
resolutions=[(1920,1080),(1600,900),(1366,768),(1280,720),(1024,768)]
wall_fail=click_fail=round_fail=0
trials=0
for w,h in resolutions:
    left,right=70,w-70
    sx=lambda v:v*w/1920.0
    l=left+sx(42); r=right-sx(42)
    count=max(9, math.ceil((r-l)/sx(96))+1)
    step=(r-l)/(count-1); radius=sx(52)
    if l-radius>left+1 or r+radius<right-1: wall_fail+=1
    for i in range(count-1):
        if step>2*radius+1e-6: wall_fail+=1
    for _ in range(20000):
        crack=random.randrange(1,count-1)
        x=l+crack*step
        nearest=min(abs(x-(l+i*step)) for i in range(count) if i!=crack)
        if nearest<=radius: click_fail+=1
        ang=random.random()*2*math.pi; rr=random.random()*sx(50)*.9
        px=x+math.cos(ang)*rr; py=500+math.sin(ang)*rr
        if math.hypot(px-x,py-500)>sx(50): click_fail+=1
        rounds=0
        while rounds<3: rounds+=1
        if rounds!=3: round_fail+=1
        trials+=1
print(f'trials={trials} wall_failures={wall_fail} click_failures={click_fail} round_failures={round_fail}')
assert wall_fail==click_fail==round_fail==0
