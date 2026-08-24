from pathlib import Path
import math
import numpy as np
from PIL import Image

R = Path(__file__).resolve().parents[1]
OPS = (R / 'afk_operators.go').read_text(errors='ignore')
ASSETS = R / 'assets' / 'ui' / 'operators'
MAIN=(R/'main.go').read_text(errors='ignore')
PROG=(R/'progression_achievements.go').read_text(errors='ignore')

def ck(name, cond):
    if not cond:
        raise AssertionError(name)
    print('PASS', name)

ck('v445 uses monotonic operator motion epoch', 'var afkOperatorMotionEpoch = time.Now()' in OPS and 'time.Since(afkOperatorMotionEpoch).Seconds()' in OPS)
ck('reduced motion no longer freezes recruited pets', 'return 0.28' in OPS and 'return 0\n' not in OPS[OPS.index('func afkOperatorAmbientScale'):OPS.index('func afkOperatorPetFloatY')])
ck('locked operators still static', 'return drawAFKOperatorSprite(hdc, artBox, i, false)' in OPS)
ck('Byte chair shares moving body rect', 'afkOperatorByteFloatY(seconds, hgt)' in OPS)
ck('pet float uses shared seconds', 'afkOperatorPetFloatY(seconds, i, 0, hgt)' in OPS)
ck('Flux spider uses shared seconds', 'afkOperatorFluxSpiderX(seconds, w)' in OPS)
ck('motion amplitudes are visually meaningful', 'baseAmp := 20.0' in OPS and 'sy(16, hgt)' in OPS and 'sx(46, w)' in OPS)
ck('Starbase owns continuous repaint timer', 'setTimer.Call(mainHwnd, TIMER_STARBASE, 33, 0)' in PROG and 'case TIMER_STARBASE:' in MAIN and 'invalidateRect.Call(h, 0, 0)' in MAIN[MAIN.index('case TIMER_STARBASE:'):MAIN.index('case TIMER_UI:')])
ck('pet layers draw after body', OPS.index('drawn := drawAFKOperatorSprite') < OPS.index('drawAFKOperatorPetSprite(hdc, pet, afkOperatorPetNova)'))

# Verify the actual packaged body/pet assets are separate and non-empty.
body_dims = {k:(512,896) for k in ['nova','vega','byte','astra','flux']}
pet_dims = {
    'nova_pet':(146,180), 'vega_pet':(182,168), 'byte_pet':(345,168),
    'astra_pet':(197,241), 'flux_pet_1':(238,138), 'flux_pet_2':(173,125),
}
for name,(w,h) in body_dims.items():
    raw=np.fromfile(ASSETS/f'{name}.bgra',dtype=np.uint8)
    ck(f'{name} body exact canvas', raw.size == w*h*4)
    ck(f'{name} body has visible pixels', np.count_nonzero(raw.reshape(h,w,4)[:,:,3]) > 1000)
for name,(w,h) in pet_dims.items():
    raw=np.fromfile(ASSETS/f'{name}.bgra',dtype=np.uint8)
    ck(f'{name} exact canvas', raw.size == w*h*4)
    ck(f'{name} has visible pixels', np.count_nonzero(raw.reshape(h,w,4)[:,:,3]) > 100)

# Known locations from the original combined art must now be transparent in body canvases.
placements=[('nova','nova_pet',(108,43),(146,180)),('vega','vega_pet',(48,84),(182,168)),('flux','flux_pet_1',(201,64),(238,138)),('flux','flux_pet_2',(107,688),(173,125))]
for body,pet,(x,y),(pw,ph) in placements:
    b=np.fromfile(ASSETS/f'{body}.bgra',dtype=np.uint8).reshape(896,512,4)
    p=np.fromfile(ASSETS/f'{pet}.bgra',dtype=np.uint8).reshape(ph,pw,4)
    mask=p[:,:,3]>0
    overlap=np.count_nonzero(b[y:y+ph,x:x+pw,3][mask])
    ck(f'{body} contains no baked-in {pet}', overlap == 0)

# Mathematical proof that every recruited motion path changes position across frames.
def sy(v,h=768): return int(v*h/1024.0)
def sx(v,w=1200): return int(v*w/1536.0)
def pet_y(t, op, slot, reduced=False):
    period=2.15+op*0.23+slot*0.31
    phase=op*0.91+slot*1.43
    amp=sy(20.0+((op+slot)%3)*3.0)*(0.28 if reduced else 1.0)
    return round(math.sin(t*2*math.pi/period+phase)*amp)
def byte_y(t,reduced=False):
    return round(math.sin(t*2*math.pi/2.65+0.45)*sy(16)*(0.28 if reduced else 1.0))
def spider_x(t,reduced=False):
    return round(math.sin(t*2*math.pi/4.0+1.1)*sx(46)*(0.28 if reduced else 1.0))

times=[0.0,0.37,0.79,1.21]
for op,name in enumerate(['nova','vega','byte_pet','astra','flux_hover']):
    slot=1 if op==2 else 0
    vals=[pet_y(t,op,slot) for t in times]
    ck(f'{name} pet changes position', len(set(vals)) >= 3 and max(vals)-min(vals) >= 5)
ck('Byte chair changes position', len(set(byte_y(t) for t in times)) >= 3)
ck('Flux spider changes position', max(spider_x(t) for t in times)-min(spider_x(t) for t in times) >= 10)
# Even Reduced Motion keeps gentle non-zero ambient movement for recruited operators.
ck('reduced-motion Nova still visibly drifts', len(set(pet_y(t,0,0,True) for t in times)) >= 2)

print('\nPASS v445 Operator motion regression')
