from pathlib import Path
import hashlib
import re

ROOT = Path(__file__).resolve().parents[1]
MAIN = (ROOT / 'main.go').read_text(encoding='utf-8')
MP4 = ROOT / 'assets' / 'video' / 'konggames_intro.mp4'
EXPECTED_SHA256 = '47037397343c8888a4b8446525e20905c40b1312ab72d472d8a79a0e3f619a8d'


def require(cond, msg):
    if not cond:
        raise AssertionError(msg)

require('WINDOW_STYLE&^WS_VISIBLE' in MAIN, 'real game window must be created hidden')
require('func introSplashWndProc' in MAIN, 'dedicated splash wndproc missing')
require('started = startKongIntroVideo(introSplashHwnd)' in MAIN, 'video must be parented to splash, not game')
require('showWindow.Call(splash, SW_SHOW)' in MAIN, 'splash must be explicitly shown')
require('updateWindow.Call(splash)' in MAIN, 'splash must synchronously paint black before video init')
require('showWindow.Call(h, SW_SHOW)' in MAIN, 'game must be shown at intro handoff')
require('destroyWindow.Call(introSplashHwnd)' in MAIN, 'splash must be destroyed at handoff')
require('elapsed >= 3600*time.Millisecond && introSplashHwnd != 0' in MAIN, 'handoff must wait for full 3.600s intro')
require('elapsed >= 3740*time.Millisecond' in MAIN, '140ms black-to-game fade must remain')
require('WM_LBUTTONDOWN, WM_LBUTTONUP, WM_RBUTTONDOWN, WM_RBUTTONUP, WM_MOUSEWHEEL, WM_MOUSEMOVE, WM_KEYDOWN, WM_CHAR' in MAIN,
        'splash must consume mouse/keyboard input')
require('MFCreatePMPMediaSession' not in MAIN and 'MFPCreateMediaPlayer' not in MAIN and 'mfplay.dll' not in MAIN.lower(), 'MFPlay runtime code must not return')
require(MP4.exists(), 'exact intro MP4 missing')
sha = hashlib.sha256(MP4.read_bytes()).hexdigest()
require(sha == EXPECTED_SHA256, f'intro MP4 changed: {sha}')

# Ordering invariant inside startup: splash show must precede beginIntro on the normal path.
start = MAIN.index('// The splash is the FIRST Cursor Control window Windows is allowed to display.')
normal = MAIN[start:MAIN.index('runMainLoop(h)', MAIN.index('introSplashHwnd = splash', start))]
require(normal.index('showWindow.Call(splash, SW_SHOW)') < normal.index('beginIntro(h)'), 'splash must show before intro begins')
normal_tail = normal[normal.index('introSplashHwnd = splash'):]
require('showWindow.Call(h, SW_SHOW)' not in normal_tail, 'game must remain hidden on normal intro path')

print('v457 intro splash regression: PASS')
