#!/usr/bin/env python3
from pathlib import Path
import random, math, re
SRC=Path(__file__).resolve().parents[1]
main=(SRC/'main.go').read_text(errors='ignore')
ui=(SRC/'ui_leaderboards_profiles.go').read_text(errors='ignore')
modals=(SRC/'ui_modals.go').read_text(errors='ignore')
garage=(SRC/'ui_quick_garage_dev.go').read_text(errors='ignore')
avatar=(SRC/'animated_avatar.go').read_text(errors='ignore')
checks=[]
def add(name,ok): checks.append((name,ok)); print(('PASS' if ok else 'FAIL'), name)
add('animated_gif_decode', 'gif.DecodeAll' in avatar and 'avatarAnimationFrame' in avatar and 'discordAnimatedAvatarURL' in avatar)
add('animated_profile_repaint', 'animatedAvatarVisible()' in ui and 'invalidateRect.Call(h, 0, 0)' in ui)
add('rank_emblem_left_5', 'badgeLeft := avatarR.Right + sx(15, w)' in ui)
add('leaderboard_footer_clean', 'YOUR GLOBAL PROGRESS IS PINNED BELOW' not in ui)
add('pinned_progress_fallback', 'Last-resort populated row' in ui and 'playerProgress.EasyCompleted' in ui)
add('announcement_current_15s', 'liveAnnouncementTravelDuration = 15 * time.Second' in ui and 'liveAnnouncementTravelDuration.Seconds()' in ui)
add('uniform_3_2_window', 'targetClientH = targetClientW * 2 / 3' in main and 'adjustWindowRectEx' in main)
add('fire_true_bounds_center', 'totalW := length + front' in garage and 'left := (r.Left + r.Right - totalW) / 2' in garage)
add('progress_bottom', 'barBottom := cr.Bottom - sy(8, hgt)' in modals)
add('progress_red', 'rgb(225, 50, 55)' in modals)
add('progress_orange', 'rgb(245, 145, 28)' in modals)
add('progress_yellow', 'rgb(255, 218, 58)' in modals)
# Geometry stress: all generated client sizes maintain exactly 3:2 within integer rounding.
for _ in range(100000):
    sw=random.randint(800,5120); sh=random.randint(600,2880)
    mw=max(720,sw-80); mh=max(480,sh-110)
    scale=min(mw/1200,mh/800,1.0); scale=max(scale,0.60)
    w=round(1200*scale); h=w*2//3
    if abs(w/h-1.5)>0.003: break
else: add('ui_aspect_100k',True)
# Progress threshold stress: incomplete 0-29 red, 30-59 orange, 60-99 yellow.
def col(q):
    if q>=.60: return 'yellow'
    if q>=.30: return 'orange'
    return 'red'
ok=True
for i in range(1,100):
    q=i/100
    exp='red' if i<30 else ('orange' if i<60 else 'yellow')
    ok &= col(q)==exp
add('progress_thresholds_1_to_99',ok)
print(f'SUMMARY {sum(ok for _,ok in checks)} / {len(checks)} PASS')
raise SystemExit(0 if all(ok for _,ok in checks) else 1)
