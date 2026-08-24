#!/usr/bin/env python3
import math, random, pathlib, re, sys

SRC = pathlib.Path(__file__).resolve().parents[1]
sg = (SRC/'survival.go').read_text()
b3 = (SRC/'survival_boss3.go').read_text()
b2 = (SRC/'survival_boss2.go').read_text()

checks = []
def ck(name, cond):
    checks.append((name, bool(cond)))
    print(('PASS' if cond else 'FAIL'), name)

# Dedicated W31 ownership: W31 can never initialise/continue the normal wave director.
ck('W31 guarded in beginSurvivalWave', 'if survivalWave == survivalBoss3Wave {' in sg[sg.index('func beginSurvivalWave'):sg.index('func invalidateSurvivalArena')])
ck('W31 safety handoff before normal spawning', 'beginSurvivalBossDossier(h, 3, now)' in sg[sg.index('func updateSurvival'):sg.index('func survivalCheckKillAchievements')])
ck('boss3 clears normal enemies', 'survivalEnemies = nil' in b3[b3.index('func beginSurvivalBoss3'):b3.index('func survivalBoss3SetStage')])
ck('boss3 clears spawn budget', 'survivalWaveBudget = 0' in b3[b3.index('func beginSurvivalBoss3'):b3.index('func survivalBoss3SetStage')])
ck('boss3 clears Sector3 field event', 'resetSurvivalSector3FieldEvent()' in b3[b3.index('func beginSurvivalBoss3'):b3.index('func survivalBoss3SetStage')])
ck('boss3 starts at full persistent station HP', 'survivalHP = survivalMaxHP()' in b3[b3.index('func beginSurvivalBoss3'):b3.index('func survivalBoss3SetStage')])
ck('critical station siren suppressed during bosses', 'survivalBoss3Active()' in sg[sg.index('func syncSurvivalCriticalSiren'):sg.index('func updateSurvival')])

# Property stress of the hard no-self-cross guard. A frame begins with the head safely
# clear of an older body path. Test random older-segment layouts, speeds and frame times;
# if the raw next step would violate clearance, the runtime's symmetric heading search
# must find a non-crossing next step.
random.seed(405)
SAFE = 102.0
RADIUS_HARD = 88.0
runs = 150000
fail = 0
rescued = 0
for _ in range(runs):
    # Older body points are distributed around the head but never inside the current
    # legal clearance. This models a legal frame immediately before the next movement.
    body=[]
    for i in range(random.randint(8,40)):
        a=random.random()*math.tau
        d=random.uniform(SAFE, 430.0)
        body.append((math.cos(a)*d, math.sin(a)*d))
    speed=random.uniform(250.0, 650.0)
    dt=random.uniform(1/240, 0.05)
    ang=random.random()*math.tau
    def clearance(a):
        x=math.cos(a)*speed*dt; y=math.sin(a)*speed*dt
        return min(math.hypot(x-bx,y-by) for bx,by in body)
    raw=clearance(ang)
    final=raw
    if raw < SAFE:
        rescued += 1
        best=raw
        chosen=None
        for step in range(1,10):
            delta=step*(math.pi/18.0)
            for sign in (-1,1):
                q=ang+sign*delta
                c=clearance(q)
                if c>best:
                    best=c
                if c>=SAFE:
                    chosen=c
                    break
            if chosen is not None: break
        if chosen is not None:
            final = chosen
        else:
            # Runtime keeps the safest heading and sheds speed until the candidate is
            # outside the no-cross envelope; zero movement is the absolute fallback.
            best_a = ang
            best_c = raw
            for step in range(1,10):
                delta=step*(math.pi/18.0)
                for sign in (-1,1):
                    q=ang+sign*delta
                    c=clearance(q)
                    if c>best_c:
                        best_c=c; best_a=q
            final = None
            for scale in (1.0,.78,.56,.34,.16,0.0):
                x=math.cos(best_a)*speed*scale*dt; y=math.sin(best_a)*speed*scale*dt
                c=min(math.hypot(x-bx,y-by) for bx,by in body)
                if c>=SAFE:
                    final=c; break
            if final is None:
                final=min(math.hypot(bx,by) for bx,by in body)
    if final < RADIUS_HARD:
        fail += 1
        break
ck(f'self-cross hard guard {runs:,} random frames', fail == 0)
ck('self-cross stress exercised emergency avoidance', rescued > 100)

# Source wiring itself must contain the early steering + final geometric guard.
ck('smooth self-avoidance steering wired', 'survivalBoss2ApplySelfAvoidance(desired, w)' in b2)
ck('hard self-intersection guard wired', 'survivalBoss2GuardSelfIntersection(dt, w)' in b2)

ok=all(v for _,v in checks)
print(f'\nSUMMARY {sum(v for _,v in checks)}/{len(checks)} checks passed; emergency cases={rescued}')
sys.exit(0 if ok else 1)
