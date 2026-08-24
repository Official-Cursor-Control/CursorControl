from pathlib import Path

root = Path(__file__).resolve().parents[1]
ops = (root / "afk_operators.go").read_text()
quips = (root / "afk_operator_quips.go").read_text()
build = (root / "build_identity.go").read_text()

def ck(name, cond):
    if not cond:
        raise AssertionError(name)
    print("PASS", name)

ck("v440 identity", 'clientBuildVersion = "v440"' in build)
ck("operator portraits moved down", 'r.Top + sy(28, hgt)' in ops and 'r.Top + sy(318, hgt)' in ops)
ck("operator identity text moved down", 'r.Top+sy(328, hgt)' in ops and 'r.Top+sy(398, hgt)' in ops)
ck("level fill renderer wired into recruited cards", 'drawAFKOperatorLevelFill(hdc, r, i, dfn.Accent, w, hgt)' in ops)
ck("fill starts with serialized quip", 'afkStartOperatorLevelFill(operatorIndex)' in quips and quips.index('afkStartOperatorLevelFill(operatorIndex)') < quips.index('playSFX(name)'))
ck("fill rises from card bottom", 'fillTop := inner.Bottom - fillH' in quips and 'RECT{inner.Left, fillTop, inner.Right, inner.Bottom}' in quips)
ck("fill uses operator accent", 'alphaSolidRect(hdc, RECT{inner.Left, fillTop, inner.Right, inner.Bottom}, accent, alpha)' in quips)
ck("fill lasts across quip", '3400 * time.Millisecond' in quips and '2700 * time.Millisecond' in quips)
ck("fill has bright moving crest", 'crestTop := fillTop - crestH/2' in quips and 'rgb(255, 255, 255)' in quips)
