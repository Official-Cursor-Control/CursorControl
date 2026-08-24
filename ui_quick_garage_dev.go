//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func quickAccessRects(w, hgt int32) [3]RECT {
	if !hudLayoutLoaded {
		hudLayoutConfig = defaultHUDLayoutConfig()
		hudLayoutLoaded = true
	}
	// v357: Profile / Local / Global were already enlarged to 130% in the HUD.
	// Add a further true 5% size increase while keeping each button centred on
	// the player's saved layout position: 1.30 * 1.05 = 1.365.
	scale1365 := func(r RECT) RECT {
		cx := (r.Left + r.Right) / 2
		cy := (r.Top + r.Bottom) / 2
		hw := int32(float64(r.Right-r.Left) * 0.6825)
		hh := int32(float64(r.Bottom-r.Top) * 0.6825)
		return RECT{cx - hw, cy - hh, cx + hw, cy + hh}
	}
	return [3]RECT{
		scale1365(designToScreenRect(hudLayoutConfig.Profile, w, hgt)),
		scale1365(designToScreenRect(hudLayoutConfig.Local, w, hgt)),
		scale1365(designToScreenRect(hudLayoutConfig.Global, w, hgt)),
	}
}

func quickBugReportRect(w, hgt int32) RECT {
	if !hudLayoutLoaded {
		hudLayoutConfig = defaultHUDLayoutConfig()
		hudLayoutLoaded = true
	}
	return designToScreenRect(hudLayoutConfig.Bug, w, hgt)
}

func quickSupportDevRect(w, hgt int32) RECT {
	if !hudLayoutLoaded {
		hudLayoutConfig = defaultHUDLayoutConfig()
		hudLayoutLoaded = true
	}
	return designToScreenRect(hudLayoutConfig.SupportDev, w, hgt)
}

func drawQuickBugReport(hdc uintptr, w, hgt int32) {
	if state != StateWaiting || overlayMode != OverlayNone || len(bugReportButtonBGRA) == 0 {
		return
	}
	r := quickBugReportRect(w, hgt)
	drawn := drawRawBGRATrimmedFit(hdc, bugReportButtonBGRA, 495, 169, r)
	if pointInRect(cursorPos, drawn) {
		drawOutlineRect(hdc, drawn, rgb(67, 225, 255), 2)
	}
}

func drawQuickSupportDev(hdc uintptr, w, hgt int32) {
	if state != StateWaiting || overlayMode != OverlayNone || len(supportDevButtonBGRA) == 0 {
		return
	}
	r := quickSupportDevRect(w, hgt)
	drawn := drawRawBGRATrimmedFit(hdc, supportDevButtonBGRA, 477, 167, r)
	if pointInRect(cursorPos, drawn) {
		drawOutlineRect(hdc, drawn, rgb(255, 100, 118), 2)
	}
}

func quickDiscordLoginRect(w, hgt int32) RECT {
	if !hudLayoutLoaded {
		hudLayoutConfig = defaultHUDLayoutConfig()
		hudLayoutLoaded = true
	}
	return designToScreenRect(hudLayoutConfig.Discord, w, hgt)
}

func drawQuickDiscordLogin(hdc uintptr, w, hgt int32) {
	if state != StateWaiting || overlayMode != OverlayNone {
		return
	}
	asset := discordLoginButtonBGRA
	if discordConnected {
		asset = discordLoggedInButtonBGRA
	}
	if len(asset) == 0 {
		return
	}
	r := quickDiscordLoginRect(w, hgt)
	var drawn RECT
	if discordConnected {
		drawn = drawRawBGRATrimmedFit(hdc, asset, 481, 174, r)
	} else {
		drawn = drawRawBGRATrimmedFit(hdc, asset, 475, 185, r)
	}
	if pointInRect(cursorPos, drawn) {
		drawOutlineRect(hdc, drawn, rgb(90, 242, 145), 2)
	}
}

func drawHUDLayoutEditor(hdc uintptr, w, hgt int32) {
	if !hudLayoutEditorActive || state != StateWaiting || overlayMode != OverlayNone {
		return
	}

	panel := RECT{sx(18, w), sy(150, hgt), sx(560, w), sy(310, hgt)}
	overlaySolidAlphaRect(hdc, panel, rgb(3, 10, 30), 225)
	drawOutlineRect(hdc, panel, rgb(38, 220, 255), 2)

	old, _, _ := selectObject.Call(hdc, hudSmallFont)
	setBkMode.Call(hdc, TRANSPARENT)
	setTextColor.Call(hdc, rgb(255, 222, 68))
	textOut(hdc, panel.Left+sx(14, w), panel.Top+sy(10, hgt), "HUD POSITION EDITOR  [F2]")
	setTextColor.Call(hdc, rgb(220, 235, 255))
	textOut(hdc, panel.Left+sx(14, w), panel.Top+sy(38, hgt), "Drag any image button to move it live.")
	textOut(hdc, panel.Left+sx(14, w), panel.Top+sy(60, hgt), "Arrow keys: 1px   Shift+Arrow: 10px")
	textOut(hdc, panel.Left+sx(14, w), panel.Top+sy(82, hgt), "Auto-saves and persists across game versions.")
	textOut(hdc, panel.Left+sx(14, w), panel.Top+sy(104, hgt), "Editable: Mode / Cache / Garage / Profile / Local / Global / Discord / Bug")

	if hudLayoutSelected >= 0 {
		r := hudLayoutRectByIndex(hudLayoutSelected)
		setTextColor.Call(hdc, rgb(86, 255, 158))
		textOut(
			hdc,
			panel.Left+sx(14, w),
			panel.Top+sy(132, hgt),
			fmt.Sprintf("%s  X:%d  Y:%d  W:%d  H:%d",
				hudLayoutName(hudLayoutSelected),
				r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top,
			),
		)
	}
	selectObject.Call(hdc, old)

	type editorOutline struct {
		Index int
		Rect  RECT
		Color uintptr
	}
	items := []editorOutline{
		{0, enduranceModeButtonRect(w, hgt), rgb(214, 77, 255)},
	}
	if enduranceActive() {
		_, cacheR, garageR := enduranceSpaceUIRects(w, hgt)
		items = append(items,
			editorOutline{1, cacheR, rgb(255, 220, 40)},
			editorOutline{2, garageR, rgb(78, 210, 255)},
		)
	}

	q := quickAccessRects(w, hgt)
	items = append(items,
		editorOutline{3, q[0], rgb(52, 180, 255)},
		editorOutline{4, q[1], rgb(255, 178, 46)},
		editorOutline{5, q[2], rgb(255, 67, 92)},
		editorOutline{6, quickDiscordLoginRect(w, hgt), rgb(88, 255, 140)},
		editorOutline{7, quickBugReportRect(w, hgt), rgb(120, 255, 90)},
	)

	for _, item := range items {
		c := item.Color
		width := int32(2)
		if hudLayoutSelected == item.Index {
			width = 4
			c = rgb(255, 245, 80)
		}
		drawOutlineRect(hdc, item.Rect, c, width)
	}
}

func drawQuickAccessDock(hdc uintptr, w, hgt int32) {
	if state != StateWaiting || overlayMode != OverlayNone {
		return
	}
	rects := quickAccessRects(w, hgt)
	assets := [][]byte{profileButtonBGRA, localButtonBGRA, globalButtonBGRA}
	widths := []int32{576, 560, 553}
	heights := []int32{172, 181, 181}
	// Match each hover outline to the actual alpha-trimmed sprite footprint,
	// rather than the larger layout/click box. The outline also follows the
	// button's own accent colour, like LOGGED IN and SUPPORT THE DEV.
	hoverColours := []uintptr{
		rgb(196, 70, 255), // PROFILE purple
		rgb(67, 225, 255), // LOCAL cyan
		rgb(255, 183, 36), // GLOBAL gold/orange
	}
	for i, r := range rects {
		if i < len(assets) && len(assets[i]) >= int(widths[i]*heights[i]*4) {
			drawn := drawRawBGRATrimmedFit(hdc, assets[i], widths[i], heights[i], r)
			if pointInRect(cursorPos, drawn) {
				drawOutlineRect(hdc, drawn, hoverColours[i], 2)
			}
		}
	}
}

func textPixelSize(hdc uintptr, font uintptr, text string) SIZE {
	var out SIZE
	if font == 0 {
		return out
	}
	old, _, _ := selectObject.Call(hdc, font)
	defer selectObject.Call(hdc, old)

	u := syscall.StringToUTF16(text)
	if len(u) > 1 {
		getTextExtentPoint32W.Call(
			hdc,
			uintptr(unsafe.Pointer(&u[0])),
			uintptr(len(u)-1),
			uintptr(unsafe.Pointer(&out)),
		)
	}
	return out
}

func verticallyCenteredTextY(hdc uintptr, font uintptr, r RECT, text string) int32 {
	sz := textPixelSize(hdc, font, text)
	h := sz.Cy
	if h <= 0 {
		h = 13
	}
	return r.Top + ((r.Bottom-r.Top)-h)/2
}

func centeredTextInRect(hdc uintptr, font uintptr, r RECT, text string) {
	if font == 0 || r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	old, _, _ := selectObject.Call(hdc, font)
	defer selectObject.Call(hdc, old)
	centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, font, r, text), text)
}

func centeredTextGroupTop(hdc uintptr, fonts []uintptr, texts []string, gaps []int32, r RECT) int32 {
	if len(fonts) == 0 || len(fonts) != len(texts) {
		return r.Top
	}
	total := int32(0)
	for i := range texts {
		h := textPixelSize(hdc, fonts[i], texts[i]).Cy
		if h <= 0 {
			h = 13
		}
		total += h
		if i < len(texts)-1 && i < len(gaps) {
			total += gaps[i]
		}
	}
	return r.Top + ((r.Bottom-r.Top)-total)/2
}

func wrapTextToWidth(hdc uintptr, font uintptr, text string, maxWidth int32) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxWidth <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, 4)
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if textPixelSize(hdc, font, candidate).Cx <= maxWidth {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func drawWrappedCenteredText(hdc uintptr, font uintptr, r RECT, text string, lineGap int32) int32 {
	if font == 0 || r.Right <= r.Left || r.Bottom <= r.Top {
		return r.Top
	}
	lines := wrapTextToWidth(hdc, font, text, r.Right-r.Left)
	if len(lines) == 0 {
		return r.Top
	}
	lineH := textPixelSize(hdc, font, "Ag").Cy
	if lineH <= 0 {
		lineH = 13
	}
	totalH := int32(len(lines))*lineH + int32(len(lines)-1)*lineGap
	y := r.Top + ((r.Bottom-r.Top)-totalH)/2
	for _, line := range lines {
		centeredTextOut(hdc, r.Left, r.Right, y, line)
		y += lineH + lineGap
	}
	return y
}

func drawEndurancePowerupHUDDock(hdc uintptr, w, hgt int32) {
	if !enduranceActive() || state == StateResult || state == StateFailed {
		return
	}

	// Occupy the same lower-left HUD zone used by PLAYER PROGRESS in Precision,
	// directly above EXP RANK. This keeps gameplay space completely clear.
	left := sx(28, w)
	right := sx(444, w)
	top := sy(806, hgt)
	bottom := sy(895, hgt)

	panel := RECT{left, top, right, bottom}
	drawBevelPanel(hdc, panel, rgb(5, 43, 101), rgb(52, 207, 255), rgb(2, 22, 58), 4)

	gap := sx(10, w)
	inset := sx(12, w)
	slotW := (right - left - inset*2 - gap) / 2
	slotTop := top + sy(9, hgt)
	slotBottom := bottom - sy(9, hgt)

	drawSlot := func(r RECT, icon []byte, sprite *CachedBGRASprite, count int, key string, accent uintptr) {
		drawBevelPanel(hdc, r, rgb(4, 31, 72), accent, rgb(1, 16, 40), 2)

		// Icon sits left; capacity and hotkey sit right for a compact HUD-card layout.
		iconSize := sy(45, hgt)
		if iconSize < 32 {
			iconSize = 32
		}
		iconR := RECT{
			r.Left + sx(9, w),
			r.Top + ((r.Bottom-r.Top)-iconSize)/2,
			r.Left + sx(9, w) + iconSize,
			r.Top + ((r.Bottom-r.Top)-iconSize)/2 + iconSize,
		}
		drawCachedBGRASprite(hdc, sprite, icon, 64, 64, iconR)

		contentLeft := iconR.Right + sx(8, w)
		contentRight := r.Right - sx(8, w)

		// Two thick capacity bars.
		barGap := sx(5, w)
		barW := (contentRight - contentLeft - barGap) / 2
		barTop := r.Top + sy(19, hgt)
		barH := sy(8, hgt)
		for i := 0; i < 2; i++ {
			bx := contentLeft + int32(i)*(barW+barGap)
			br := RECT{bx, barTop, bx + barW, barTop + barH}
			fillSolidRect(hdc, br, rgb(1, 14, 37))
			inner := RECT{br.Left + 2, br.Top + 2, br.Right - 2, br.Bottom - 2}
			if i < count {
				fillSolidRect(hdc, inner, accent)
			} else {
				fillSolidRect(hdc, inner, rgb(45, 80, 111))
			}
		}

		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			keyText := key
			setTextColor.Call(hdc, rgb(255, 255, 255))
			keyBand := RECT{contentLeft, r.Top + sy(35, hgt), contentRight, r.Bottom - sy(5, hgt)}
			centeredTextOut(hdc, contentLeft, contentRight,
				verticallyCenteredTextY(hdc, hudSmallFont, keyBand, keyText), keyText)
			selectObject.Call(hdc, old)
		}

		// Subtle full-capacity pulse using the card's own accent.
		if count >= 2 && (time.Now().UnixMilli()/260)%2 == 0 {
			drawLineSimple(hdc, r.Left+2, r.Top+2, r.Right-2, r.Top+2, 2, accent)
			drawLineSimple(hdc, r.Left+2, r.Bottom-3, r.Right-2, r.Bottom-3, 2, accent)
		}
	}

	shieldR := RECT{left + inset, slotTop, left + inset + slotW, slotBottom}
	timeR := RECT{shieldR.Right + gap, slotTop, right - inset, slotBottom}
	drawSlot(shieldR, powerupShieldBGRA, &powerupShieldSprite, enduranceStoredShields, "Q", rgb(72, 221, 255))
	drawSlot(timeR, powerupTimeBGRA, &powerupTimeSprite, enduranceStoredTime, "W", rgb(255, 73, 84))
}

var expRankUseGlobal bool
var globalAccountEXP int
var globalAccountEXPLoaded bool
var globalAccountEXPLoading bool

func expRankToggleRect(w, hgt int32) RECT {
	// Keep the LOCAL/GLOBAL switch in its own top-right slot inside the EXP
	// progression panel.  The old rectangle sat across the second text row and
	// was also painted before the panel background, which made it disappear.
	return RECT{sx(305, w), sy(922, hgt), sx(429, w), sy(944, hgt)}
}

func drawAdvancedMissionPanel(hdc uintptr, w, hgt int32) {
	if survivalActive() {
		drawSurvivalMissionDock(hdc, w, hgt)
		return
	}
	left := sx(28, w)
	right := sx(444, w)

	if enduranceActive() {
		drawEndurancePowerupHUDDock(hdc, w, hgt)
	}

	// Endurance keeps the EXP Rank progression panel because Space Cache rewards
	// award normal account EXP. Only the Standard difficulty-clear grid is hidden.
	if !enduranceActive() {
		gridR := RECT{left, sy(806, hgt), right, sy(895, hgt)}
		drawBevelPanel(hdc, gridR, rgb(5, 47, 112), rgb(49, 209, 255), rgb(2, 24, 63), 4)
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 255, 255))
			titleBand := RECT{gridR.Left + sx(16, w), gridR.Top + sy(4, hgt), gridR.Right - sx(16, w), gridR.Top + sy(32, hgt)}
			textOut(hdc, titleBand.Left, verticallyCenteredTextY(hdc, hudSmallFont, titleBand, "PLAYER PROGRESS"), "PLAYER PROGRESS")
			selectObject.Call(hdc, old)
		}

		il := gridR.Left + sx(14, w)
		ir := gridR.Right - sx(14, w)
		it := gridR.Top + sy(36, hgt)
		ib := gridR.Bottom - sy(10, hgt)
		gx := sx(8, w)
		gy := sy(5, hgt)
		cw := (ir - il - gx) / 2
		ch := (ib - it - gy) / 2

		type pc struct {
			label  string
			value  int
			accent uintptr
		}
		cells := []pc{
			{"EASY", playerProgress.EasyCompleted, rgb(72, 228, 126)},
			{"NORMAL", playerProgress.NormalCompleted, rgb(255, 178, 40)},
			{"HARD", playerProgress.HardCompleted, rgb(255, 79, 85)},
			{"INSANE", playerProgress.InsaneCompleted, rgb(218, 88, 255)},
		}
		for i, c := range cells {
			row := i / 2
			col := i % 2
			x := il + int32(col)*(cw+gx)
			y := it + int32(row)*(ch+gy)
			cr := RECT{x, y, x + cw, y + ch}
			label := c.label
			if i > 0 && !difficultyUnlocked(i) {
				label = c.label + "  🔒"
			}
			fillSolidRect(hdc, cr, rgb(4, 31, 72))
			drawLineSimple(hdc, cr.Left, cr.Top, cr.Right, cr.Top, 2, c.accent)
			drawLineSimple(hdc, cr.Left+4, cr.Top+4, cr.Left+4, cr.Bottom-4, 2, c.accent)

			if hudTinyFont != 0 {
				old, _, _ := selectObject.Call(hdc, hudTinyFont)
				setBkMode.Call(hdc, TRANSPARENT)
				setTextColor.Call(hdc, c.accent)
				textOut(hdc, cr.Left+sx(12, w), verticallyCenteredTextY(hdc, hudTinyFont, cr, label)+2, label)
				selectObject.Call(hdc, old)
			}
			if hudSmallFont != 0 {
				v := fmt.Sprintf("%d", c.value)
				old, _, _ := selectObject.Call(hdc, hudSmallFont)
				setBkMode.Call(hdc, TRANSPARENT)
				setTextColor.Call(hdc, rgb(255, 255, 255))
				sz := textPixelSize(hdc, hudSmallFont, v)
				textOut(hdc, cr.Right-sx(12, w)-sz.Cx, verticallyCenteredTextY(hdc, hudSmallFont, cr, v)+2, v)
				selectObject.Call(hdc, old)
			}
		}

	}

	expR := RECT{left, sy(916, hgt), right, sy(995, hgt)}
	// Paint the EXP panel first, then draw the LOCAL/GLOBAL switch above it.
	// Previously the order was reversed, so drawBevelPanel(expR) erased the
	// toggle even though its click rectangle and logic were still active.
	drawBevelPanel(hdc, expR, rgb(5, 43, 101), rgb(52, 207, 255), rgb(2, 22, 58), 4)
	toggleR := expRankToggleRect(w, hgt)
	toggleAccent := rgb(80, 214, 255)
	drawBevelPanel(hdc, toggleR, rgb(4, 31, 72), toggleAccent, rgb(1, 15, 38), 2)
	if hudTinyFont != 0 {
		label := "GLOBAL"
		if expRankUseGlobal {
			label = "LOCAL"
		}
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		centeredTextOut(hdc, toggleR.Left, toggleR.Right, verticallyCenteredTextY(hdc, hudTinyFont, toggleR, label), label)
		selectObject.Call(hdc, old)
	}
	displayEXP := playerProgress.EXP
	if expRankUseGlobal && globalAccountEXPLoaded {
		displayEXP = globalAccountEXP
	}
	current, next, hasNext, into, needed := nextRankProgress(displayEXP)
	remaining := 0
	if hasNext {
		remaining = needed - into
		if remaining < 0 {
			remaining = 0
		}
	}

	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(83, 225, 255))
		rankTitle := "LOCAL EXP RANK"
		if expRankUseGlobal {
			rankTitle = "GLOBAL EXP RANK"
		}
		textOut(hdc, expR.Left+sx(15, w), expR.Top+sy(8, hgt), rankTitle)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		textOut(hdc, expR.Left+sx(15, w), expR.Top+sy(27, hgt), current.Name)

		rightLabel := "MAX RANK"
		if hasNext {
			rightLabel = fmt.Sprintf("%d EXP LEFT", remaining)
			setTextColor.Call(hdc, rgb(255, 224, 75))
		} else {
			setTextColor.Call(hdc, rgb(90, 235, 132))
		}
		sz := textPixelSize(hdc, hudTinyFont, rightLabel)
		textOut(hdc, expR.Right-sx(15, w)-sz.Cx, expR.Top+sy(27, hgt), rightLabel)
		selectObject.Call(hdc, old)
	}

	bar := RECT{expR.Left + sx(15, w), expR.Top + sy(50, hgt), expR.Right - sx(15, w), expR.Top + sy(75, hgt)}
	fillSolidRect(hdc, bar, rgb(2, 22, 55))
	drawLineSimple(hdc, bar.Left, bar.Top, bar.Right, bar.Top, 1, rgb(64, 138, 191))
	drawLineSimple(hdc, bar.Left, bar.Bottom, bar.Right, bar.Bottom, 1, rgb(1, 12, 31))
	fr := bar.Right
	if hasNext && needed > 0 {
		q := float64(into) / float64(needed)
		if q < 0 {
			q = 0
		}
		if q > 1 {
			q = 1
		}
		fr = bar.Left + int32(q*float64(bar.Right-bar.Left))
	}
	if fr > bar.Left {
		// v357: EXP progress now reads at a glance from danger -> completion.
		// The filled section uses a continuous four-stage gradient across the
		// *whole* rank bar: red <25%, orange <50%, yellow <75%, green through 99%+.
		// Rendering in compact slices keeps the colour transitions smooth without
		// adding a bitmap asset or changing progression math.
		fillEXPProgressGradient(hdc, bar, fr)
		// v372 customization: animation is visual-only and never changes EXP math.
		switch gameMeta.EXPBarAnimation {
		case 1: // subtle pulse
			pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*4.0)
			alphaSolidRect(hdc, RECT{bar.Left, bar.Top, fr, bar.Bottom}, rgb(255, 255, 255), byte(10+int(26*pulse)))
		case 2: // flowing highlight
			w := fr - bar.Left
			if w > 8 {
				phase := math.Mod(float64(time.Now().UnixMilli())/700.0, 1.0)
				x := bar.Left + int32(phase*float64(w+sx(28, 1536))) - sx(28, 1536)
				for i := int32(0); i < sx(24, 1536); i += 3 {
					if x+i < bar.Left || x+i >= fr {
						continue
					}
					a := byte(30 + int(80*(1.0-math.Abs(float64(i-sx(12, 1536))/float64(maxInt32(1, sx(12, 1536)))))))
					alphaSolidRect(hdc, RECT{x + i, bar.Top + 1, minInt32(fr, x+i+3), bar.Bottom - 1}, rgb(255, 255, 255), a)
				}
			}
		}
		if hasNext && needed > 0 && float64(into)/float64(needed) >= 0.85 {
			pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*5.0)
			glint := blendColor(rgb(92, 235, 112), rgb(255, 255, 255), 0.18+0.20*pulse)
			drawLineSimple(hdc, fr-2, bar.Top+2, fr-2, bar.Bottom-2, 2, glint)
		}
	}

	if hudTinyFont != 0 {
		pt := fmt.Sprintf("%d EXP", displayEXP)
		if expRankUseGlobal && !globalAccountEXPLoaded {
			pt = "SYNCING GLOBAL EXP..."
		}
		if hasNext {
			pt = fmt.Sprintf("%d / %d EXP", into, needed)
		}
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		centeredTextOut(hdc, bar.Left, bar.Right, verticallyCenteredTextY(hdc, hudTinyFont, bar, pt), pt)
		if hasNext {
			setTextColor.Call(hdc, rgb(181, 219, 245))
			centeredTextOut(hdc, expR.Left+sx(15, w), expR.Right-sx(15, w), expR.Top+sy(78, hgt), "NEXT: "+next.Name)
		}
		selectObject.Call(hdc, old)
	}
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
func expProgressGradientColor(q float64) uintptr {
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	red := rgb(235, 52, 52)
	orange := rgb(255, 145, 28)
	yellow := rgb(255, 221, 45)
	green := rgb(68, 220, 104)
	switch {
	case q < 0.25:
		// Stay predominantly red through the first quarter, but begin the blend
		// gently so there is never a hard colour seam.
		return blendColor(red, orange, (q/0.25)*0.18)
	case q < 0.50:
		return blendColor(red, orange, 0.18+((q-0.25)/0.25)*0.82)
	case q < 0.75:
		return blendColor(orange, yellow, (q-0.50)/0.25)
	case q < 0.99:
		return blendColor(yellow, green, (q-0.75)/0.24)
	default:
		return green
	}
}

func fillEXPProgressGradient(hdc uintptr, bar RECT, fillRight int32) {
	if fillRight <= bar.Left || bar.Right <= bar.Left {
		return
	}
	if fillRight > bar.Right {
		fillRight = bar.Right
	}
	width := bar.Right - bar.Left
	// 2px slices are visually smooth at the fixed HUD scale and avoid hundreds
	// of GDI calls per frame on very wide/high-DPI windows.
	step := int32(2)
	for x := bar.Left; x < fillRight; x += step {
		nextX := x + step
		if nextX > fillRight {
			nextX = fillRight
		}
		q := float64(x-bar.Left) / float64(width)
		c := expProgressGradientColor(q)
		fillSolidRect(hdc, RECT{x, bar.Top, nextX, bar.Bottom}, c)
		// Soft highlight follows the same hue instead of reverting to cyan.
		hl := blendColor(c, rgb(255, 255, 255), 0.28)
		fillSolidRect(hdc, RECT{x, bar.Top, nextX, bar.Top + 3}, hl)
	}
}

func enduranceSpaceUIRects(w, hgt int32) (RECT, RECT, RECT) {
	if !hudLayoutLoaded {
		hudLayoutConfig = defaultHUDLayoutConfig()
		hudLayoutLoaded = true
	}

	cache := designToScreenRect(hudLayoutConfig.SpaceCache, w, hgt)
	garage := designToScreenRect(hudLayoutConfig.Garage, w, hgt)

	// v324: the economy row belongs entirely below the playable arena. Older
	// saved layouts may still place it at y=736, so enforce a minimum top below
	// the arena while preserving the user's horizontal layout and button size.
	arenaBottom := arenaRect(w, hgt).Bottom
	minTop := arenaBottom + sy(18, hgt)
	currentTop := cache.Top
	if garage.Top < currentTop {
		currentTop = garage.Top
	}
	if currentTop < minTop {
		dy := minTop - currentTop
		cache.Top += dy
		cache.Bottom += dy
		garage.Top += dy
		garage.Bottom += dy
	}
	// Keep the row inside the client even on short/windowed resolutions. There is
	// enough lower-HUD space for this 60-design-pixel row; do not clamp it upward
	// through the arena boundary just to reserve unused footer space.
	maxBottom := hgt - sy(18, hgt)
	lowest := cache.Bottom
	if garage.Bottom > lowest {
		lowest = garage.Bottom
	}
	if lowest > maxBottom {
		dy := lowest - maxBottom
		cache.Top -= dy
		cache.Bottom -= dy
		garage.Top -= dy
		garage.Bottom -= dy
	}

	// Keep the Space Coin display tied to the cache by default, but it is not
	// editable because it is a status bar rather than a clickable PNG button.
	coinW := sx(185, w)
	gap := sx(10, w)
	coin := RECT{cache.Left - gap - coinW, cache.Top, cache.Left - gap, cache.Bottom}
	return coin, cache, garage
}

func drawEnduranceSpaceHUD(hdc uintptr, w, hgt int32) {
	if !enduranceActive() || state != StateWaiting || (overlayMode != OverlayNone && overlayMode != OverlaySpaceCache) {
		return
	}
	coinR, cacheR, garageR := enduranceSpaceUIRects(w, hgt)
	drawRawBGRAFit(hdc, spaceCoinBarBGRA, 468, 177, coinR)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		coinText := fmt.Sprintf("%d", gameMeta.SpaceCoins)
		centeredTextOut(hdc, coinR.Left+sx(68, w), coinR.Right-sx(10, w), verticallyCenteredTextY(hdc, hudSmallFont, coinR, coinText), coinText)
		selectObject.Call(hdc, old)
	}
	drawRawBGRAFit(hdc, spaceCacheButtonBGRA, 515, 205, cacheR)
	drawRawBGRAFit(hdc, garageButtonBGRA, 470, 176, garageR)
	if pointInRect(cursorPos, cacheR) {
		drawOutlineRect(hdc, cacheR, rgb(255, 218, 55), 2)
	}
	if pointInRect(cursorPos, garageR) {
		drawOutlineRect(hdc, garageR, rgb(68, 224, 255), 2)
	}
	if hudTinyFont != 0 && overlayMode != OverlaySpaceCache {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		if !spaceCacheWarningUntil.IsZero() && time.Now().Before(spaceCacheWarningUntil) {
			setTextColor.Call(hdc, rgb(255, 220, 24))
			centeredTextOut(hdc, cacheR.Left-sx(120, w), cacheR.Right+sx(120, w), cacheR.Bottom+sy(9, hgt), spaceCacheWarningText)
		} else {
			setTextColor.Call(hdc, rgb(255, 220, 24))
			centeredTextOut(hdc, cacheR.Left-sx(90, w), cacheR.Right+sx(90, w), cacheR.Bottom+sy(4, hgt), "CLAIM FREE SPACE CACHE")
			setTextColor.Call(hdc, rgb(128, 220, 255))
			centeredTextOut(hdc, cacheR.Left-sx(90, w), cacheR.Right+sx(90, w), cacheR.Bottom+sy(24, hgt), freeCacheCountdownText())
		}
		selectObject.Call(hdc, old)
	}
}

func garagePanelRect(w, hgt int32) RECT {
	return centeredPanel(w, hgt, 1120, 720)
}

func garageShipCardRects(w, hgt int32) [12]RECT {
	panel := garagePanelRect(w, hgt)
	var out [12]RECT
	gapX := sx(14, w)
	gapY := sy(18, hgt)
	gridLeft := panel.Left + sx(42, w)
	gridRight := panel.Right - sx(42, w)
	gridTop := panel.Top + sy(165, hgt)
	cardW := (gridRight - gridLeft - gapX*3) / 4
	cardH := sy(150, hgt)
	for i := 0; i < 12; i++ {
		row := i / 4
		col := i % 4
		x := gridLeft + int32(col)*(cardW+gapX)
		y := gridTop + int32(row)*(cardH+gapY)
		out[i] = RECT{x, y, x + cardW, y + cardH}
	}
	return out
}

func garageTabRects(w, hgt int32) [3]RECT {
	panel := garagePanelRect(w, hgt)
	gap := sx(12, w)
	top := panel.Top + sy(88, hgt)
	wid := sx(205, w)
	total := wid*3 + gap*2
	left := (panel.Left + panel.Right - total) / 2
	return [3]RECT{
		{left, top, left + wid, top + sy(34, hgt)},
		{left + wid + gap, top, left + wid*2 + gap, top + sy(34, hgt)},
		{left + wid*2 + gap*2, top, left + wid*3 + gap*2, top + sy(34, hgt)},
	}
}

func garageFireCardRects(w, hgt int32) [8]RECT {
	panel := garagePanelRect(w, hgt)
	var out [8]RECT
	gapX := sx(18, w)
	gapY := sy(22, hgt)
	left := panel.Left + sx(72, w)
	right := panel.Right - sx(72, w)
	top := panel.Top + sy(190, hgt)
	cardW := (right - left - gapX*3) / 4
	cardH := sy(155, hgt)
	for i := 0; i < 8; i++ {
		row := i / 4
		col := i % 4
		x := left + int32(col)*(cardW+gapX)
		y := top + int32(row)*(cardH+gapY)
		out[i] = RECT{x, y, x + cardW, y + cardH}
	}
	return out
}

func garageFireSizeCardRects(w, hgt int32) [3]RECT {
	panel := garagePanelRect(w, hgt)
	var out [3]RECT
	gap := sx(28, w)
	cardW := sx(260, w)
	cardH := sy(250, hgt)
	total := cardW*3 + gap*2
	left := (panel.Left + panel.Right - total) / 2
	top := panel.Top + sy(230, hgt)
	for i := 0; i < 3; i++ {
		x := left + int32(i)*(cardW+gap)
		out[i] = RECT{x, top, x + cardW, top + cardH}
	}
	return out
}

const developerOwnerUserID = "6efd951a-81cd-4a74-be44-152bc554cc74"

func isDeveloperOwner() bool {
	authMu.Lock()
	uid := strings.TrimSpace(discordUserID)
	connected := discordConnected
	authMu.Unlock()
	return connected && strings.EqualFold(uid, developerOwnerUserID)
}

func developerHelpText(section string) string {
	switch strings.ToLower(strings.TrimSpace(section)) {
	case "local":
		return "/GodMode On|Off  /GiveMe Coins|EXP N  /SetMe Coins|EXP N  /UnlockMe AllShips  /SetMode Standard|Endurance  /SetDifficulty Easy|Normal|Hard|Insane  /CacheOpen  /CacheFree  /Status  /Diag  /SelfTest  /PathAudit  /DeepAudit  /DPIAudit  /FrameAudit  /StateAudit  /LeakAudit Start|Check  /EconomyAudit  /TechAudit  /UIAudit  /AssetAudit  /Perf On|Off|Reset|Save  /Analytics Status"
	case "endurance", "end":
		return "/SetDistance N  /Sector 1-5  /WarpNow  /WarpNext  /AlienPhase  /BossNow  /Powerups Shield|Time|Both 0-2  /Director  /AchievementStatus  /ClearAll  /Diag  /SelfTest  /PathAudit  /DeepAudit  /DPIAudit  /FrameAudit  /StateAudit  /LeakAudit Start|Check  /EconomyAudit  /TechAudit  /UIAudit  /AssetAudit  /Perf On|Off|Reset|Save  /Analytics Status  /Spawn Target|Shield|Slow|Distance|Alien  /Clear Meteors|Aliens|Targets|Powerups  /Audio EnduranceFail|Meteor|ReadyGo|Rocket|AlienCharge|AlienImpact"
	case "survival", "surv":
		return "/SSection 1|2|3  /SBoss 1|2|3  /SBoss2Energy2  /SetMode Survival  /GodMode On|Off  /Perf On|Off|Reset|Save  /Diag  /SelfTest  /DeepAudit  /FrameAudit  /StateAudit  /LeakAudit Start|Check  /Analytics Status"
	case "admin":
		return "/Announce MESSAGE  /AnnounceClear  /GlobalReset Easy|Normal|Hard|Insane|Endurance|All  /ResetPlayer USER_ID  /Give USER_ID Coins|EXP N  /Unlock USER_ID AllShips  (destructive commands require /Confirm)"
	default:
		return "/Help Local   /Help Endurance   /Help Survival   /Help Admin"
	}
}

func developerAdminRequest(action string, payload map[string]any) string {
	authMu.Lock()
	token := strings.TrimSpace(authSession.AccessToken)
	uid := strings.TrimSpace(discordUserID)
	authMu.Unlock()
	if token == "" || uid == "" || !strings.EqualFold(uid, developerOwnerUserID) {
		return "ADMIN AUTH REQUIRED"
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["action"] = action
	body, err := json.Marshal(payload)
	if err != nil {
		return "ADMIN REQUEST BUILD FAILED"
	}
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/functions/v1/admin-control", bytes.NewReader(body))
	if err != nil {
		return "ADMIN REQUEST FAILED"
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := sharedAdminHTTPClient.Do(req)
	if err != nil {
		return "ADMIN ENDPOINT UNAVAILABLE"
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var out struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(data, &out)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && out.OK {
		if out.Message != "" {
			return out.Message
		}
		return "ADMIN COMMAND COMPLETE"
	}
	if out.Error != "" {
		return "ADMIN ERROR — " + strings.ToUpper(out.Error)
	}
	if resp.StatusCode == 404 {
		return "ADMIN EDGE FUNCTION NOT DEPLOYED"
	}
	return fmt.Sprintf("ADMIN ERROR HTTP %d", resp.StatusCode)
}

func developerJumpToWarp(skipCurrent bool) string {
	if !enduranceActive() || state != StatePlaying || mainHwnd == 0 {
		return "START AN ENDURANCE RUN FIRST"
	}

	track := enduranceTrackDistance()
	cp := nextEnduranceWarpCheckpointAfter(track + 0.001)
	if skipCurrent {
		cp = nextEnduranceWarpCheckpointAfter(cp + 1.0)
	}
	ensureEnduranceWarpScheduleThrough(cp + enduranceWarpStraightMeters + 400.0)

	// Put the player roughly 100m before the physical portal. This keeps the
	// portal visible and gives the pre-generated rail time to approach centre,
	// while still reaching the real portal trigger naturally.
	jumpTrack := cp - 100.0
	if jumpTrack < 0 {
		jumpTrack = 0
	}

	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)
	viewW := float64(ar.Right - ar.Left)
	neededRight := jumpTrack*10.0 + float64(ar.Right) + viewW*2.2
	for len(path) > 0 && path[len(path)-1].X < neededRight {
		appendBufferedEnduranceChunk(w, hgt)
	}

	enduranceCameraX = jumpTrack * 10.0
	enduranceDistance = enduranceCameraX/10.0 + enduranceDistanceBonus
	score = int(math.Round(enduranceDistance*10)) + enduranceBonusScore
	enduranceNextWarpAt = cp
	enduranceWarpCueActive = false
	enduranceWarpActive = false
	enduranceWarpCueStarted = time.Time{}
	enduranceWarpStartDistance = 0
	enduranceWarpCheckpoint = 0
	enduranceWarpRecoveryUntil = 0
	enduranceWarpTargetsSpawned = false
	enduranceBlocks = enduranceBlocks[:0]
	enduranceBlockSpawnTime = time.Time{}
	enduranceTargetExplosions = nil
	targets = targets[:0]
	enduranceNextTargetAt = cp + enduranceWarpStraightMeters + enduranceWarpRecoveryMeters
	stopWarpSounds()
	developerBoundaryGraceUntil = time.Now().Add(5 * time.Second)

	// Snap the logical and physical mouse to the pre-generated rail at the new
	// camera position so /WarpNow cannot immediately kill the player because
	// the old cursor Y belonged to a completely different piece of track.
	if len(path) > 0 {
		worldX := enduranceCameraX + cursorPos.X
		idx := int(math.Round((worldX - path[0].X) / 9.0))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(path) {
			idx = len(path) - 1
		}
		cursorPos.Y = path[idx].Y
		lastMouse = cursorPos
		hasLastMouse = true
		cursorInArena = true
		if mainHwnd != 0 {
			pt := POINT{X: int32(math.Round(cursorPos.X)), Y: int32(math.Round(cursorPos.Y))}
			clientToScreen.Call(mainHwnd, uintptr(unsafe.Pointer(&pt)))
			setCursorPos.Call(uintptr(pt.X), uintptr(pt.Y))
		}
	}

	enduranceLoopAccumulator = 0
	enduranceLastLoopTime = time.Now()
	enduranceLastRenderTime = time.Time{}
	enduranceRailDirty = true
	markMergedEnduranceRailDirty()
	invalidateRect.Call(mainHwnd, 0, 0)

	label := "NEXT"
	if skipCurrent {
		label = "FOLLOWING"
	}
	return fmt.Sprintf("JUMPED TO %s WARP APPROACH — PORTAL %.0fm — 5s BOUNDARY GRACE", label, cp)
}

func developerJumpToAlienPhase() string {
	if !enduranceActive() || state != StatePlaying || mainHwnd == 0 {
		return "START AN ENDURANCE RUN FIRST"
	}
	jumpTrack := enduranceAlienPhaseStartMeters - 12
	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)
	viewW := float64(ar.Right - ar.Left)
	neededRight := jumpTrack*10 + float64(ar.Right) + viewW*2.2
	for len(path) > 0 && path[len(path)-1].X < neededRight {
		appendBufferedEnduranceChunk(w, hgt)
	}
	enduranceCameraX = jumpTrack * 10
	enduranceDistance = enduranceCameraX/10 + enduranceDistanceBonus
	score = int(math.Round(enduranceDistance*10)) + enduranceBonusScore
	enduranceNextWarpAt = nextEnduranceWarpCheckpointAfter(jumpTrack + 1)
	enduranceWarpCueActive = false
	enduranceWarpActive = false
	enduranceWarpCueStarted = time.Time{}
	enduranceWarpRecoveryUntil = 0
	enduranceBlocks = enduranceBlocks[:0]
	enduranceBlockSpawnTime = time.Time{}
	enduranceAlienMinions = enduranceAlienMinions[:0]
	enduranceAlienMinionSpawnTime = time.Time{}
	enduranceAlienPhaseTriggered = false
	enduranceAlienBossState = alienBossIdle
	enduranceAlienBossStateStarted = time.Time{}
	targets = targets[:0]
	enduranceNextTargetAt = enduranceDistance + 45
	stopWarpSounds()
	developerBoundaryGraceUntil = time.Now().Add(5 * time.Second)

	if len(path) > 0 {
		worldX := enduranceCameraX + cursorPos.X
		idx := int(math.Round((worldX - path[0].X) / 9.0))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(path) {
			idx = len(path) - 1
		}
		cursorPos.Y = path[idx].Y
		lastMouse = cursorPos
		hasLastMouse = true
		cursorInArena = true
		pt := POINT{X: int32(math.Round(cursorPos.X)), Y: int32(math.Round(cursorPos.Y))}
		clientToScreen.Call(mainHwnd, uintptr(unsafe.Pointer(&pt)))
		setCursorPos.Call(uintptr(pt.X), uintptr(pt.Y))
	}
	enduranceLoopAccumulator = 0
	enduranceLastLoopTime = time.Now()
	enduranceLastRenderTime = time.Time{}
	enduranceRailDirty = true
	markMergedEnduranceRailDirty()
	invalidateRect.Call(mainHwnd, 0, 0)
	return fmt.Sprintf("JUMPED TO ALIEN PHASE — WARNING AT %.0fm — 5s BOUNDARY GRACE", enduranceAlienPhaseStartMeters)
}

func developerJumpToEnduranceDistance(meters float64) string {
	if !enduranceActive() || state != StatePlaying || mainHwnd == 0 {
		return "START AN ENDURANCE RUN FIRST"
	}
	if meters < 0 {
		meters = 0
	}
	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)
	viewW := float64(ar.Right - ar.Left)
	neededRight := meters*10.0 + float64(ar.Right) + viewW*2.2
	for len(path) > 0 && path[len(path)-1].X < neededRight {
		appendBufferedEnduranceChunk(w, hgt)
	}
	enduranceCameraX = meters * 10.0
	enduranceDistanceBonus = 0
	enduranceDistance = meters
	score = int(math.Round(enduranceDistance*10)) + enduranceBonusScore
	enduranceBlocks = enduranceBlocks[:0]
	enduranceAlienMinions = enduranceAlienMinions[:0]
	endurancePowerups = endurancePowerups[:0]
	targets = targets[:0]
	enduranceBlockSpawnTime = time.Time{}
	enduranceAlienMinionSpawnTime = time.Time{}
	enduranceNextTargetAt = enduranceDistance + 35
	enduranceNextWarpAt = nextEnduranceWarpCheckpointAfter(meters + 1)
	enduranceWarpCueActive = false
	enduranceWarpActive = false
	enduranceWarpCueStarted = time.Time{}
	enduranceWarpRecoveryUntil = 0
	stopTransientGameplayAudio()
	developerBoundaryGraceUntil = time.Now().Add(5 * time.Second)

	if len(path) > 0 {
		worldX := enduranceCameraX + cursorPos.X
		idx := int(math.Round((worldX - path[0].X) / 9.0))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(path) {
			idx = len(path) - 1
		}
		cursorPos.Y = path[idx].Y
		lastMouse = cursorPos
		hasLastMouse = true
		cursorInArena = true
		pt := POINT{X: int32(math.Round(cursorPos.X)), Y: int32(math.Round(cursorPos.Y))}
		clientToScreen.Call(mainHwnd, uintptr(unsafe.Pointer(&pt)))
		setCursorPos.Call(uintptr(pt.X), uintptr(pt.Y))
	}
	enduranceLoopAccumulator = 0
	enduranceLastLoopTime = time.Now()
	enduranceLastRenderTime = time.Time{}
	enduranceRailDirty = true
	markMergedEnduranceRailDirty()
	invalidateRect.Call(mainHwnd, 0, 0)
	return fmt.Sprintf("JUMPED TO %.0fm — 5s BOUNDARY GRACE", meters)
}

func developerGrantStoredPowerups(kind string, count int) string {
	if !enduranceActive() || state != StatePlaying {
		return "START AN ENDURANCE RUN FIRST"
	}
	if count < 0 {
		count = 0
	}
	if count > 2 {
		count = 2
	}
	switch strings.ToLower(kind) {
	case "shield":
		enduranceStoredShields = count
		return fmt.Sprintf("SHIELD STORAGE SET TO %d/2", count)
	case "time", "slow":
		enduranceStoredTime = count
		return fmt.Sprintf("TIME STORAGE SET TO %d/2", count)
	case "both", "all":
		enduranceStoredShields = count
		enduranceStoredTime = count
		return fmt.Sprintf("SHIELD + TIME STORAGE SET TO %d/2", count)
	default:
		return "USE /Powerups Shield|Time|Both 0-2"
	}
}

func executeDeveloperCommand(raw string) string {
	cmd := strings.TrimSpace(raw)
	if cmd == "" {
		return ""
	}
	if !isDeveloperOwner() {
		return "ACCESS DENIED"
	}
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))

	if name == "help" || name == "commands" {
		section := ""
		if len(parts) >= 2 {
			section = parts[1]
		}
		return developerHelpText(section)
	}
	if name == "pathaudit" || name == "path-audit" {
		return runEndurancePathBoundsAudit()
	}
	if name == "deepaudit" || name == "deep-audit" {
		return runDeepAudit()
	}
	if name == "dpiaudit" || name == "dpi-audit" {
		return runDPICursorAudit()
	}
	if name == "frameaudit" || name == "frame-audit" {
		return runFrameIndependenceAudit()
	}
	if name == "stateaudit" || name == "state-audit" {
		return runStateTransitionAudit()
	}
	if name == "leakaudit" || name == "leak-audit" {
		action := "check"
		if len(parts) >= 2 {
			action = parts[1]
		}
		return runLeakAudit(action)
	}
	if name == "economyaudit" || name == "achievementaudit" || name == "achievement-economy-audit" {
		return runAchievementEconomyAudit()
	}
	if name == "techaudit" || name == "tech-audit" {
		return runTechnicalConsistencyAudit()
	}
	if name == "uiaudit" || name == "ui-audit" {
		return runUIConsistencyAudit()
	}
	if name == "assetaudit" || name == "asset-audit" {
		return runAssetConsistencyAudit()
	}
	if name == "selftest" || name == "self-test" {
		return runRuntimeSelfTest()
	}
	if name == "diag" {
		logRuntimeDiagnostic("developer")
		return runtimeDiagnosticText()
	}
	if (name == "ssection" || name == "survivalsection" || name == "survival-section") && len(parts) == 2 {
		var section int
		if _, err := fmt.Sscanf(parts[1], "%d", &section); err != nil || section < 1 || section > 3 {
			return "USE /SSection 1|2|3"
		}
		return developerStartSurvivalSection(section)
	}
	if (name == "sboss" || name == "survivalboss" || name == "survival-boss") && len(parts) == 2 {
		var boss int
		if _, err := fmt.Sscanf(parts[1], "%d", &boss); err != nil || boss < 1 || boss > 3 {
			return "USE /SBoss 1|2|3"
		}
		return developerStartSurvivalBoss(boss)
	}
	if name == "sboss2energy2" || name == "energyball2" || name == "sboss2energyphase2" {
		return developerStartSurvivalBoss2EnergyPhase2()
	}
	if name == "powerups" && len(parts) >= 2 {
		count := 2
		if len(parts) >= 3 {
			if _, err := fmt.Sscanf(parts[2], "%d", &count); err != nil {
				return "USE /Powerups Shield|Time|Both 0-2"
			}
		}
		return developerGrantStoredPowerups(parts[1], count)
	}
	if name == "sector" && len(parts) == 2 {
		if !enduranceActive() || state != StatePlaying {
			return "START AN ENDURANCE RUN FIRST"
		}
		var sector int
		if _, err := fmt.Sscanf(parts[1], "%d", &sector); err != nil || sector < 1 {
			return "USE /Sector 1-5"
		}
		if sector > 5 {
			sector = 5
		}
		return developerJumpToEnduranceDistance(float64((sector-1)*2500 + 25))
	}
	if name == "bossnow" || name == "ufonow" {
		if !enduranceActive() || state != StatePlaying {
			return "START AN ENDURANCE RUN FIRST"
		}
		if enduranceDistance < enduranceAlienPhaseStartMeters {
			_ = developerJumpToEnduranceDistance(enduranceAlienPhaseStartMeters + 2)
		}
		enduranceAlienPhaseTriggered = false
		enduranceAlienBossState = alienBossIdle
		enduranceAlienBossStateStarted = time.Time{}
		enduranceAlienMinions = enduranceAlienMinions[:0]
		enduranceBlocks = enduranceBlocks[:0]
		beginEnduranceAlienBossEvent()
		developerBoundaryGraceUntil = time.Now().Add(5 * time.Second)
		return "UFO BOSS WARNING STARTED — 5s BOUNDARY GRACE"
	}
	if name == "clearall" {
		enduranceBlocks = enduranceBlocks[:0]
		enduranceAlienMinions = enduranceAlienMinions[:0]
		endurancePowerups = endurancePowerups[:0]
		targets = targets[:0]
		enduranceTargetExplosions = nil
		resetPolishVFX()
		return "ALL TRANSIENT HAZARDS / TARGETS / POWER-UPS CLEARED"
	}
	if name == "achievementstatus" || name == "achievements" {
		return fmt.Sprintf("ACHIEVEMENTS %d/%d | ENDURANCE PB %.0fm | WARP %d | UFO %d | POWERUPS %d | CACHES %d",
			len(gameMeta.Achievements), len(achievementDefinitions()), gameMeta.BestEnduranceDistance,
			gameMeta.EnduranceWarpsCompleted, gameMeta.AlienBossesSurvived, gameMeta.EndurancePowerups, gameMeta.SpaceCachesOpened)
	}
	if name == "analytics" {
		d1, d7, d30 := analyticsRetentionFlags()
		consent := "OFF"
		if gameMeta.ShareAnonymousAnalytics {
			consent = "ON"
		}
		return fmt.Sprintf("ANALYTICS SHARE %s | ACTIVE DAYS %d | D1 %t | D7 %t | D30 %t | LOCAL LOG analytics_local.jsonl", consent, len(gameMeta.ActivePlayDates), d1, d7, d30)
	}
	if name == "perf" {
		if len(parts) == 1 {
			return perfStatusText()
		}
		switch strings.ToLower(parts[1]) {
		case "on", "1", "true":
			perfStats.Enabled = true
			return "PERFORMANCE OVERLAY ON"
		case "off", "0", "false":
			perfStats.Enabled = false
			return "PERFORMANCE OVERLAY OFF"
		case "reset":
			enabled := perfStats.Enabled
			perfStats = perfTelemetry{Enabled: enabled}
			return "PERFORMANCE COUNTERS RESET"
		case "save", "snapshot":
			writeRunPerfSnapshot("developer_snapshot")
			return "PERFORMANCE SNAPSHOT SAVED TO run_performance.log"
		}
		return "USE /PERF ON|OFF|RESET|SAVE"
	}
	if name == "director" {
		recovery := "NO"
		if endurancePostEncounterRecoveryActive() {
			recovery = fmt.Sprintf("YES %.0fm", endurancePostEncounterRecoveryUntil-enduranceDistance)
		}
		return fmt.Sprintf("ENCOUNTER %s | INTENSITY %.2f | DEPTH %.2f | RECOVERY %s", enduranceEncounterLabel(enduranceDistance), encounterDirectorMultiplier(enduranceDistance), enduranceWorldDepth(enduranceProgressDistance()), recovery)
	}
	if name == "status" {
		mode := "STANDARD"
		if enduranceActive() {
			mode = "ENDURANCE"
		}
		return fmt.Sprintf("MODE %s | STATE %d | GOD %t | EXP %d | COINS %d | DIST %.1fm | TARGETS %d | METEORS %d | ALIENS %d | BOSS %d | SHIELD %d/2 | TIME %d/2", mode, state, developerGodMode, playerProgress.EXP, gameMeta.SpaceCoins, enduranceDistance, len(targets), len(enduranceBlocks), len(enduranceAlienMinions), enduranceAlienBossState, enduranceStoredShields, enduranceStoredTime)
	}
	if name == "godmode" && len(parts) == 2 {
		switch strings.ToLower(parts[1]) {
		case "on", "1", "true":
			developerGodMode = true
			return "GOD MODE ON — FAIL CONDITIONS DISABLED"
		case "off", "0", "false":
			developerGodMode = false
			return "GOD MODE OFF"
		}
		return "USE /GodMode On OR /GodMode Off"
	}
	if name == "giveme" && len(parts) == 3 {
		var amount int
		if _, err := fmt.Sscanf(parts[2], "%d", &amount); err != nil || amount < 0 {
			return "INVALID AMOUNT"
		}
		switch strings.ToLower(parts[1]) {
		case "spacecoins", "coins":
			gameMeta.SpaceCoins += amount
			saveGameMeta()
			return fmt.Sprintf("GAVE YOUR ACCOUNT %d SPACE COINS", amount)
		case "exp":
			playerProgress.EXP += amount
			savePlayerProgress()
			return fmt.Sprintf("GAVE YOUR ACCOUNT %d EXP", amount)
		}
	}
	if name == "setme" && len(parts) == 3 {
		var amount int
		if _, err := fmt.Sscanf(parts[2], "%d", &amount); err != nil || amount < 0 {
			return "INVALID AMOUNT"
		}
		switch strings.ToLower(parts[1]) {
		case "spacecoins", "coins":
			gameMeta.SpaceCoins = amount
			saveGameMeta()
			return fmt.Sprintf("SPACE COINS SET TO %d", amount)
		case "exp":
			playerProgress.EXP = amount
			savePlayerProgress()
			return fmt.Sprintf("EXP SET TO %d (%s)", amount, rankForEXP(amount))
		}
	}
	if name == "unlockme" && len(parts) == 2 && strings.EqualFold(parts[1], "AllShips") {
		for id := 1; id <= 12; id++ {
			if id == 11 {
				continue
			}
			unlockShip(id)
		}
		saveGameMeta()
		return "ALL SPACESHIPS UNLOCKED"
	}
	if name == "cacheopen" {
		if mainHwnd == 0 {
			return "WINDOW NOT READY"
		}
		if gameMeta.SpaceCoins < 100 {
			return "NEED 100 SPACE COINS — USE /CacheFree FOR A FREE TEST"
		}
		beginSpaceCacheOpen(mainHwnd)
		return "OPENING SPACE CACHE"
	}
	if name == "cachefree" {
		if mainHwnd == 0 {
			return "WINDOW NOT READY"
		}
		gameMeta.SpaceCoins += 100
		saveGameMeta()
		beginSpaceCacheOpen(mainHwnd)
		return "OPENING FREE TEST CACHE"
	}
	if name == "setmode" && len(parts) == 2 {
		if state == StatePlaying {
			return "END RUN BEFORE CHANGING MODE"
		}
		switch strings.ToLower(parts[1]) {
		case "standard", "normal", "precision":
			clearEnduranceTransientObjects()
			gameMode = 0
			if mainHwnd != 0 {
				resetToWaiting(mainHwnd)
			}
			return "MODE SET TO STANDARD"
		case "endurance":
			clearEnduranceTransientObjects()
			gameMode = 1
			if mainHwnd != 0 {
				resetToWaiting(mainHwnd)
			}
			return "MODE SET TO ENDURANCE"
		case "survival":
			clearEnduranceTransientObjects()
			gameMode = 2
			resetSurvivalRun()
			if mainHwnd != 0 {
				resetToWaiting(mainHwnd)
			}
			return "MODE SET TO SURVIVAL"
		}
		return "USE /SetMode Precision, Endurance OR Survival"
	}
	if name == "setdifficulty" && len(parts) == 2 {
		if state == StatePlaying {
			return "END RUN BEFORE CHANGING DIFFICULTY"
		}
		switch strings.ToLower(parts[1]) {
		case "easy":
			difficulty = 0
		case "normal":
			difficulty = 1
		case "hard":
			difficulty = 2
		case "insane":
			difficulty = 3
		default:
			return "USE EASY, NORMAL, HARD OR INSANE"
		}
		gameMode = 0
		return "STANDARD DIFFICULTY SET TO " + strings.ToUpper(parts[1])
	}
	if name == "setdistance" && len(parts) == 2 {
		if !enduranceActive() || state != StatePlaying {
			return "START AN ENDURANCE RUN FIRST"
		}
		var meters float64
		if _, err := fmt.Sscanf(parts[1], "%f", &meters); err != nil || meters < 0 {
			return "INVALID DISTANCE"
		}
		enduranceDistanceBonus = meters - enduranceCameraX/10.0
		enduranceDistance = meters
		score = int(math.Round(enduranceDistance*10)) + enduranceBonusScore
		return fmt.Sprintf("ENDURANCE DISTANCE SET TO %.1fm", meters)
	}
	if name == "warpnow" {
		return developerJumpToWarp(false)
	}
	if name == "warpnext" {
		return developerJumpToWarp(true)
	}
	if name == "alienphase" || name == "aliens" {
		return developerJumpToAlienPhase()
	}
	if name == "spawn" && len(parts) == 2 {
		if !enduranceActive() || state != StatePlaying {
			return "START AN ENDURANCE RUN FIRST"
		}
		switch strings.ToLower(parts[1]) {
		case "target":
			targets = targets[:0]
			spawnEnduranceTarget()
			return "ENDURANCE TARGET SPAWNED"
		case "shield":
			applyEndurancePowerup(endurancePowerupShield)
			return "SHIELD POWER-UP APPLIED"
		case "slow":
			applyEndurancePowerup(endurancePowerupSlow)
			return "SLOW POWER-UP APPLIED"
		case "distance":
			applyEndurancePowerup(endurancePowerupDistance)
			return "+100m POWER-UP APPLIED"
		case "alien", "minion":
			w, hgt := getClient(mainHwnd)
			spawnEnduranceAlienMinion(w, hgt)
			return "ALIEN MINION SPAWNED"
		}
		return "USE /Spawn Target|Shield|Slow|Distance|Alien"
	}
	if name == "clear" && len(parts) == 2 {
		switch strings.ToLower(parts[1]) {
		case "meteors", "meteorites":
			enduranceBlocks = enduranceBlocks[:0]
			return "METEORITES CLEARED"
		case "aliens", "minions":
			enduranceAlienMinions = enduranceAlienMinions[:0]
			return "ALIEN MINIONS CLEARED"
		case "targets":
			targets = targets[:0]
			return "TARGETS CLEARED"
		case "powerups":
			endurancePowerups = endurancePowerups[:0]
			return "POWER-UPS CLEARED"
		}
		return "USE /Clear Meteors|Aliens|Targets|Powerups"
	}
	if name == "audio" && len(parts) == 2 {
		switch strings.ToLower(parts[1]) {
		case "endurancefail", "fail":
			playEnduranceFailSound()
			return "PLAYING ENDURANCE FAIL"
		case "meteor", "explosion":
			playEnduranceExplodeSound()
			return "PLAYING METEOR EXPLOSION"
		case "readygo", "ready":
			playWarpReadySound()
			return "PLAYING READY/GO"
		case "rocket", "warp":
			playWarpRocketSound()
			return "PLAYING ROCKET BOOST"
		case "aliencharge", "charge":
			playAlienChargeSound()
			return "PLAYING ALIEN CHARGE"
		case "alienimpact", "laser":
			playAlienImpactSound()
			return "PLAYING ALIEN LASER IMPACT"
		}
		return "USE /Audio EnduranceFail|Meteor|ReadyGo|Rocket|AlienCharge|AlienImpact"
	}
	if name == "announce" {
		message := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), parts[0]))
		if message == "" {
			return "USE /Announce MESSAGE"
		}
		return developerAdminRequest("announce", map[string]any{"message": message})
	}
	if name == "announceclear" && len(parts) == 1 {
		return developerAdminRequest("announce_clear", nil)
	}
	if name == "globalreset" && len(parts) == 2 {
		scope := strings.ToUpper(parts[1])
		allowed := map[string]bool{"EASY": true, "NORMAL": true, "HARD": true, "INSANE": true, "ENDURANCE": true, "ALL": true}
		if !allowed[scope] {
			return "USE EASY, NORMAL, HARD, INSANE, ENDURANCE OR ALL"
		}
		developerConfirmAction = "globalreset:" + scope
		developerConfirmUntil = time.Now().Add(20 * time.Second)
		return "DANGER: TYPE /Confirm GlobalReset " + scope + " WITHIN 20 SECONDS"
	}
	if name == "confirm" && len(parts) == 3 && strings.EqualFold(parts[1], "GlobalReset") {
		scope := strings.ToUpper(parts[2])
		expected := "globalreset:" + scope
		if developerConfirmAction != expected || time.Now().After(developerConfirmUntil) {
			developerConfirmAction = ""
			return "CONFIRMATION EXPIRED OR DOES NOT MATCH"
		}
		developerConfirmAction = ""
		return developerAdminRequest("global_reset", map[string]any{"scope": scope})
	}
	if name == "resetplayer" && len(parts) == 2 {
		uid := strings.TrimSpace(parts[1])
		developerConfirmAction = "resetplayer:" + uid
		developerConfirmUntil = time.Now().Add(20 * time.Second)
		return "DANGER: TYPE /Confirm ResetPlayer " + uid + " WITHIN 20 SECONDS"
	}
	if name == "confirm" && len(parts) == 3 && strings.EqualFold(parts[1], "ResetPlayer") {
		uid := strings.TrimSpace(parts[2])
		if developerConfirmAction != "resetplayer:"+uid || time.Now().After(developerConfirmUntil) {
			developerConfirmAction = ""
			return "CONFIRMATION EXPIRED OR DOES NOT MATCH"
		}
		developerConfirmAction = ""
		return developerAdminRequest("reset_player", map[string]any{"discord_user_id": uid})
	}
	if name == "give" && len(parts) == 4 {
		uid := strings.TrimSpace(parts[1])
		var amount int
		if _, err := fmt.Sscanf(parts[3], "%d", &amount); err != nil || amount < 0 {
			return "INVALID AMOUNT"
		}
		switch strings.ToLower(parts[2]) {
		case "coins", "spacecoins":
			return developerAdminRequest("give_coins", map[string]any{"discord_user_id": uid, "amount": amount})
		case "exp":
			return developerAdminRequest("give_exp", map[string]any{"discord_user_id": uid, "amount": amount})
		}
	}
	if name == "unlock" && len(parts) == 3 && strings.EqualFold(parts[2], "AllShips") {
		return developerAdminRequest("unlock_all_ships", map[string]any{"discord_user_id": strings.TrimSpace(parts[1])})
	}
	return "UNKNOWN COMMAND - TYPE /Help"
}

func drawDeveloperConsoleOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayDeveloperConsole {
		return
	}
	left := w/2 - sx(390, w)
	right := w/2 + sx(390, w)
	top := sy(150, hgt)
	bottom := top + sy(260, hgt)
	fillSolidRect(hdc, RECT{left, top, right, bottom}, rgb(3, 13, 29))
	border := rgb(0, 225, 255)
	drawLineSimple(hdc, left, top, right, top, 2, border)
	drawLineSimple(hdc, right, top, right, bottom, 2, border)
	drawLineSimple(hdc, right, bottom, left, bottom, 2, border)
	drawLineSimple(hdc, left, bottom, left, top, 2, border)
	setBkMode.Call(hdc, TRANSPARENT)
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, rgb(0, 235, 255))
		textOut(hdc, left+sx(22, w), top+sy(18, hgt), "DEVELOPER CONSOLE")
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, rgb(255, 214, 0))
		textOut(hdc, left+sx(22, w), top+sy(58, hgt), "OWNER ACCESS  •  TAB = CLOSE  •  ENTER = RUN")
		setTextColor.Call(hdc, rgb(210, 230, 245))
		textOut(hdc, left+sx(22, w), top+sy(94, hgt), "> "+developerConsoleInput+"_")
		if strings.TrimSpace(developerConsoleStatus) != "" {
			setTextColor.Call(hdc, rgb(90, 255, 150))
			textOut(hdc, left+sx(22, w), top+sy(134, hgt), developerConsoleStatus)
		}
		setTextColor.Call(hdc, rgb(145, 175, 200))
		textOut(hdc, left+sx(22, w), bottom-sy(48, hgt), "TYPE /Help FOR COMMANDS")
		selectObject.Call(hdc, old)
	}
}

func drawGarageOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayGarage {
		return
	}
	overlaySolidAlpha(hdc, w, hgt, rgb(0, 5, 18), 195)
	panel := garagePanelRect(w, hgt)
	panelFace, panelAccent, panelDark := themedHUDPanelPalette(true)
	drawBevelPanel(hdc, panel, panelFace, panelAccent, panelDark, 5)
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, rgb(57, 225, 255))
		centeredTextOut(hdc, panel.Left, panel.Right, panel.Top+sy(26, hgt), "SPACE GARAGE")
		selectObject.Call(hdc, old)
	}
	tabs := garageTabRects(w, hgt)
	labels := [3]string{"SPACESHIPS", "FIRE COLOUR", "FIRE SIZE"}
	for i, r := range tabs {
		face, edge, tabDark := themedHUDCardPalette(0)
		if garageTab == i {
			face = rgb(12, 72, 128)
			edge = rgb(255, 202, 50)
		}
		if pointInRect(cursorPos, r) {
			edge = blendColor(edge, rgb(255, 255, 255), 0.25)
		}
		drawBevelPanel(hdc, r, face, edge, tabDark, 2)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, rgb(245, 250, 255))
			centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, hudTinyFont, r, labels[i]), labels[i])
			selectObject.Call(hdc, old)
		}
	}
	if garageTab == 0 {
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, rgb(155, 205, 235))
			centeredTextOut(hdc, panel.Left, panel.Right, panel.Top+sy(132, hgt), "COSMETIC ONLY  •  EVERY SHIP USES THE SAME BODY HITBOX")
			selectObject.Call(hdc, old)
		}
		cards := garageShipCardRects(w, hgt)
		for i, r := range cards {
			id := garageShipOrder[i]
			owned := shipUnlocked(id)
			selected := gameMeta.SelectedShip == id
			accent := garageSlotBackdropColour(i)
			face := garageSlotFaceColour(i)
			light := blendColor(accent, rgb(255, 255, 255), 0.14)
			if selected {
				light = rgb(255, 210, 60)
				face = blendColor(face, rgb(255, 210, 60), 0.12)
			}
			if pointInRect(cursorPos, r) {
				light = blendColor(light, rgb(255, 255, 255), 0.28)
				face = blendColor(face, accent, 0.16)
			}
			drawBevelPanel(hdc, r, face, light, rgb(1, 13, 37), 2)
			imgBox := RECT{r.Left + sx(18, w), r.Top + sy(14, hgt), r.Right - sx(18, w), r.Top + sy(78, hgt)}
			backdrop := RECT{imgBox.Left - sx(4, w), imgBox.Top - sy(2, hgt), imgBox.Right + sx(4, w), imgBox.Bottom + sy(2, hgt)}
			overlaySolidAlphaRect(hdc, backdrop, accent, 54)
			drawOutlineRect(hdc, backdrop, blendColor(accent, rgb(255, 255, 255), 0.22), 1)
			drawShipTextureFit(hdc, id, imgBox)
			if !owned {
				overlaySolidAlphaRect(hdc, r, rgb(20, 24, 34), 165)
			}
			if i == len(cards)-1 {
				drawGarageLegendaryTwinkle(hdc, r, w, hgt)
			}
			if hudTinyFont != 0 {
				old, _, _ := selectObject.Call(hdc, hudTinyFont)
				setBkMode.Call(hdc, TRANSPARENT)
				if owned {
					setTextColor.Call(hdc, rgb(247, 251, 255))
				} else {
					setTextColor.Call(hdc, rgb(136, 145, 158))
				}
				centeredTextOut(hdc, r.Left+sx(6, w), r.Right-sx(6, w), r.Top+sy(91, hgt), spaceShipDefs[id].Name)
				if owned {
					setTextColor.Call(hdc, rgb(255, 202, 64))
				} else {
					setTextColor.Call(hdc, rgb(115, 124, 139))
				}
				if id == 0 {
					centeredTextOut(hdc, r.Left+sx(6, w), r.Right-sx(6, w), r.Top+sy(119, hgt), "DEFAULT  •  ALWAYS OWNED")
				} else {
					centeredTextOut(hdc, r.Left+sx(6, w), r.Right-sx(6, w), r.Top+sy(119, hgt), fmt.Sprintf("%s  •  %.0f%%", spaceShipDefs[id].Rarity, spaceShipDefs[id].Chance))
				}
				selectObject.Call(hdc, old)
			}
		}
	} else if garageTab == 1 {
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, rgb(175, 218, 244))
			centeredTextOut(hdc, panel.Left, panel.Right, panel.Top+sy(132, hgt), fmt.Sprintf("SPACE COINS: %d   •   CLICK TO BUY OR EQUIP", gameMeta.SpaceCoins))
			selectObject.Call(hdc, old)
		}
		cards := garageFireCardRects(w, hgt)
		for slot, r := range cards {
			id := garageFireColorOrder[slot]
			owned := fireColorUnlocked(id)
			selected := gameMeta.SelectedFireColor == id
			face, edge, cardDark := themedHUDCardPalette(0)
			if owned {
				edge = rgb(48, 214, 255)
			}
			if selected {
				edge = rgb(255, 210, 60)
				face = rgb(18, 45, 76)
			}
			if pointInRect(cursorPos, r) {
				edge = blendColor(edge, rgb(255, 255, 255), 0.28)
			}
			drawBevelPanel(hdc, r, face, edge, cardDark, 2)
			// Three nested pixel bars preview the coded exhaust without any external particles.
			cx := (r.Left + r.Right) / 2
			cy := r.Top + sy(54, hgt)
			c := fireColorDefs[id].Color
			if id == 7 { // rainbow preview bars
				cols := []uintptr{rgb(255, 55, 30), rgb(255, 205, 45), rgb(55, 245, 85), rgb(55, 145, 255), rgb(170, 70, 255), rgb(255, 80, 190)}
				bw := sx(18, w)
				x0 := cx - bw*3
				for j, col := range cols {
					fillSolidRect(hdc, RECT{x0 + int32(j)*bw, cy - sy(10, hgt), x0 + int32(j+1)*bw, cy + sy(10, hgt)}, col)
				}
			} else {
				fillSolidRect(hdc, RECT{cx - sx(60, w), cy - sy(11, hgt), cx + sx(60, w), cy + sy(11, hgt)}, blendColor(c, rgb(0, 0, 0), 0.30))
				fillSolidRect(hdc, RECT{cx - sx(44, w), cy - sy(7, hgt), cx + sx(48, w), cy + sy(7, hgt)}, c)
				fillSolidRect(hdc, RECT{cx - sx(24, w), cy - sy(3, hgt), cx + sx(52, w), cy + sy(3, hgt)}, blendColor(c, rgb(255, 255, 255), 0.72))
			}
			if hudTinyFont != 0 {
				old, _, _ := selectObject.Call(hdc, hudTinyFont)
				setTextColor.Call(hdc, rgb(248, 252, 255))
				centeredTextOut(hdc, r.Left+sx(4, w), r.Right-sx(4, w), r.Top+sy(91, hgt), fireColorDefs[id].Name)
				if selected {
					setTextColor.Call(hdc, rgb(255, 214, 60))
					centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(120, hgt), "EQUIPPED")
				} else if owned {
					setTextColor.Call(hdc, rgb(85, 240, 150))
					centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(120, hgt), "OWNED")
				} else {
					setTextColor.Call(hdc, rgb(255, 202, 64))
					centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(120, hgt), fmt.Sprintf("%d COINS", fireColorDefs[id].Cost))
				}
				selectObject.Call(hdc, old)
			}
		}
	} else {
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, rgb(175, 218, 244))
			centeredTextOut(hdc, panel.Left, panel.Right, panel.Top+sy(145, hgt), fmt.Sprintf("SPACE COINS: %d   •   FIRE SIZE ALSO SCALES WARP AFTERBURNER", gameMeta.SpaceCoins))
			selectObject.Call(hdc, old)
		}
		cards := garageFireSizeCardRects(w, hgt)
		for id, r := range cards {
			owned := fireSizeUnlocked(id)
			selected := gameMeta.SelectedFireSize == id
			face, edge, cardDark := themedHUDCardPalette(0)
			if owned {
				edge = rgb(48, 214, 255)
			}
			if selected {
				edge = rgb(255, 210, 60)
				face = rgb(18, 45, 76)
			}
			if pointInRect(cursorPos, r) {
				edge = blendColor(edge, rgb(255, 255, 255), 0.28)
			}
			drawBevelPanel(hdc, r, face, edge, cardDark, 2)

			// Center the preview by its true rendered flame bounds, not by the engine
			// anchor.  This keeps SMALL/MEDIUM/LARGE visually centred in each card.
			cy := r.Top + sy(95, hgt)
			m := fireSizeDefs[id].Multiplier
			length := int32(float32(sx(56, w)) * m)
			front := int32(float32(sx(20, w)) * m)
			half := int32(float32(sy(9, hgt)) * m)
			if half < 2 {
				half = 2
			}
			totalW := length + front
			left := (r.Left + r.Right - totalW) / 2
			right := left + totalW
			engineX := right - front
			fillSolidRect(hdc, RECT{left, cy - half, right - sx(2, w), cy + half}, rgb(185, 36, 18))
			innerLeft := engineX - int32(float32(length)*0.72)
			fillSolidRect(hdc, RECT{innerLeft, cy - half*2/3, right, cy + half*2/3}, rgb(255, 88, 22))
			coreHalfPx := half / 3
			if coreHalfPx < 2 {
				coreHalfPx = 2
			}
			coreLeft := engineX - int32(float32(length)*0.42)
			fillSolidRect(hdc, RECT{coreLeft, cy - coreHalfPx, right, cy + coreHalfPx}, rgb(255, 238, 180))

			if hudTinyFont != 0 {
				old, _, _ := selectObject.Call(hdc, hudTinyFont)
				setTextColor.Call(hdc, rgb(248, 252, 255))
				centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(145, hgt), fireSizeDefs[id].Name)
				setTextColor.Call(hdc, rgb(150, 220, 255))
				centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(171, hgt), fmt.Sprintf("%.0f%% FIRE SCALE", fireSizeDefs[id].Multiplier*100))
				if selected {
					setTextColor.Call(hdc, rgb(255, 214, 60))
					centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(206, hgt), "EQUIPPED")
				} else if owned {
					setTextColor.Call(hdc, rgb(85, 240, 150))
					centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(206, hgt), "OWNED")
				} else {
					setTextColor.Call(hdc, rgb(255, 202, 64))
					centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(206, hgt), fmt.Sprintf("%d COINS", fireSizeDefs[id].Cost))
				}
				selectObject.Call(hdc, old)
			}
		}
	}
	if garageNoticeText != "" && time.Now().Before(garageNoticeUntil) && hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 225, 55))
		centeredTextOut(hdc, panel.Left+sx(20, w), panel.Right-sx(20, w), panel.Bottom-sy(58, hgt), garageNoticeText)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, rgb(165, 207, 232))
		centeredTextOut(hdc, panel.Left, panel.Right, panel.Bottom-sy(25, hgt), "ESC / CLICK OUTSIDE = BACK")
		selectObject.Call(hdc, old)
	}
}

func overlaySolidAlphaRect(hdc uintptr, r RECT, color uintptr, alpha byte) {
	alphaSolidRect(hdc, r, color, alpha)
}

func drawSpaceCacheOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlaySpaceCache {
		return
	}
	_, cacheR, _ := enduranceSpaceUIRects(w, hgt)
	// Keep the normal Endurance HUD visible and open the cache directly beneath
	// the OPEN SPACE CACHE button instead of taking over the arena.
	boxW := sx(420, w)
	boxH := sy(132, hgt)
	if spaceCacheOpened && spaceCacheRewardShip > 0 {
		boxH = sy(176, hgt)
	}
	cx := (cacheR.Left + cacheR.Right) / 2
	boxBottom := cacheR.Top - sy(18, hgt)
	box := RECT{cx - boxW/2, boxBottom - boxH, cx + boxW/2, boxBottom}
	// v314: final safety clamp for short/windowed resolutions. The whole reward
	// card, including CLICK TO CONTINUE, must remain on-screen.
	bottomMargin := sy(12, hgt)
	if box.Bottom > hgt-bottomMargin {
		dy := box.Bottom - (hgt - bottomMargin)
		box.Top -= dy
		box.Bottom -= dy
	}
	drawBevelPanel(hdc, box, rgb(2, 19, 55), rgb(50, 215, 255), rgb(1, 9, 30), 3)

	// IMPORTANT: every label uses this stable anchor. Only the cache sprite shakes.
	baseImgBox := RECT{box.Left + sx(14, w), box.Top + sy(8, hgt), box.Left + sx(145, w), box.Top + sy(124, hgt)}
	imgBox := baseImgBox
	if !spaceCacheOpened {
		elapsed := time.Since(spaceCacheOpenStarted).Seconds()
		shake := int32(0)
		if elapsed > 0 {
			shake = int32(math.Sin(elapsed*95.0) * 3.0)
		}
		imgBox.Left += shake
		imgBox.Right += shake
		drawRawBGRAFit(hdc, spaceCacheClosedBGRA, 405, 306, imgBox)
	} else {
		drawRawBGRAFit(hdc, spaceCacheOpenBGRA, 397, 397, imgBox)
	}

	textLeft := baseImgBox.Right + sx(8, w)
	textRight := box.Right - sx(10, w)

	if spaceCacheOpened && spaceCacheRewardShip > 0 {
		ship := spaceCacheRewardShip
		def := spaceShipDefs[ship]
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 208, 60))
			title := "SPACESHIP UNLOCKED"
			if spaceCacheRewardCoins > 0 {
				title = "DUPLICATE SPACESHIP"
			}
			centeredTextOut(hdc, textLeft, textRight, box.Top+sy(16, hgt), title)
			selectObject.Call(hdc, old)
		}
		revealAge := time.Since(spaceCacheOpenStarted).Seconds() - 2.0
		reveal := easeOutCubic(revealAge / 0.38)
		if reveal < 0.72 {
			reveal = 0.72
		}
		shipW := int32(float64(sx(92, w)) * reveal)
		shipH := int32(float64(sy(63, hgt)) * reveal)
		shipMid := (textLeft + textRight) / 2
		shipTop := box.Top + sy(38, hgt) + (sy(63, hgt)-shipH)/2
		shipR := RECT{shipMid - shipW/2, shipTop, shipMid + shipW/2, shipTop + shipH}
		if spaceCacheRewardCoins == 0 {
			pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*5.5)
			glow := blendColor(rgb(255, 207, 57), rgb(73, 224, 255), 0.25+0.20*pulse)
			drawOutlineRect(hdc, RECT{shipR.Left - sx(7, w), shipR.Top - sy(5, hgt), shipR.Right + sx(7, w), shipR.Bottom + sy(5, hgt)}, glow, 2)
		}
		if ship >= 0 && ship < len(spaceShipBGRA) {
			drawShipTextureFit(hdc, ship, shipR)
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 255, 255))
			centeredTextOut(hdc, textLeft, textRight, box.Top+sy(105, hgt), def.Name)
			selectObject.Call(hdc, old)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(86, 225, 255))
			if spaceCacheRewardCoins > 0 {
				centeredTextOut(hdc, textLeft, textRight, box.Top+sy(132, hgt), fmt.Sprintf("%s  •  DUPLICATE +%d", def.Rarity, spaceCacheRewardCoins))
			} else {
				centeredTextOut(hdc, textLeft, textRight, box.Top+sy(132, hgt), def.Rarity)
			}
			setTextColor.Call(hdc, rgb(176, 214, 238))
			centeredTextOut(hdc, textLeft, textRight, box.Bottom-sy(22, hgt), "CLICK TO CONTINUE")
			selectObject.Call(hdc, old)
		}
		return
	}

	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		if spaceCacheOpened {
			setTextColor.Call(hdc, rgb(255, 208, 60))
		} else {
			setTextColor.Call(hdc, rgb(76, 225, 255))
		}
		if spaceCacheOpened && spaceCacheRewardCoins > 0 && len(spaceCoinBGRA) > 0 {
			label := strings.ToUpper(spaceCacheRewardText)
			centeredTextOut(hdc, textLeft, textRight, box.Top+sy(25, hgt), label)
			coinSize := sy(30, hgt)
			if coinSize < 18 {
				coinSize = 18
			}
			amount := fmt.Sprintf("+%d", spaceCacheRewardCoins)
			amountSz := textPixelSize(hdc, hudSmallFont, amount)
			gap := sx(6, w)
			totalW := coinSize + gap + amountSz.Cx
			mid := (textLeft + textRight) / 2
			rowLeft := mid - totalW/2
			coinR := RECT{rowLeft, box.Top + sy(53, hgt), rowLeft + coinSize, box.Top + sy(53, hgt) + coinSize}
			drawRawBGRAFit(hdc, spaceCoinBGRA, 200, 203, coinR)
			textOut(hdc, coinR.Right+gap, verticallyCenteredTextY(hdc, hudSmallFont, coinR, amount), amount)
		} else {
			centeredTextOut(hdc, textLeft, textRight, box.Top+sy(34, hgt), strings.ToUpper(spaceCacheRewardText))
		}
		selectObject.Call(hdc, old)
	}
	if spaceCacheOpened && hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, rgb(176, 214, 238))
		centeredTextOut(hdc, textLeft, textRight, box.Bottom-sy(34, hgt), "CLICK TO CONTINUE")
		selectObject.Call(hdc, old)
	}
}
