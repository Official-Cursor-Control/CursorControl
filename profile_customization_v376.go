//go:build windows

package main

import (
	"fmt"
	"math"
	"time"
)

// Global-profile customisation ladder:
// Gold I = readable profile-wide font
// Platinum I = banners
// Diamond I = Advanced Names
// Master I = profile animations
var profileCustomizeTab int // 0 font, 1 banners, 2 advanced name, 3 animation
var profileColourTarget int // 0 primary, 1 secondary

var profileStyleColours = []uintptr{
	rgb(8, 10, 14),     // black
	rgb(246, 249, 252), // white
	rgb(126, 67, 255),  // purple
	rgb(255, 64, 48),   // coral
	rgb(43, 220, 230),  // cyan
	rgb(55, 220, 115),  // emerald
	rgb(58, 153, 255),  // blue
	rgb(210, 62, 255),  // magenta
	rgb(255, 55, 125),  // pink
	rgb(235, 191, 47),  // gold
	rgb(255, 139, 38),  // orange
	rgb(150, 238, 64),  // lime
}
var profileStyleColourNames = []string{"BLACK", "WHITE", "PURPLE", "CORAL", "CYAN", "EMERALD", "BLUE", "MAGENTA", "PINK", "GOLD", "ORANGE", "LIME"}
var profileAnimationNames = []string{"STATIC", "PULSE GLOW", "COLOUR WAVE", "HOLO SCAN"}

var profilePublicFontCache [8][4]uintptr

func publicProfileFont(style, kind int) uintptr {
	if style < 0 || style >= len(uiFontFaces) {
		style = 0
	}
	if kind < 0 || kind > 3 {
		kind = 0
	}
	if profilePublicFontCache[style][kind] != 0 {
		return profilePublicFontCache[style][kind]
	}
	heights := []uint32{23, 30, 15, 12}
	weights := []uintptr{700, 800, 600, 600}
	h := makeFontForFace(uiFontFaces[style], heights[kind], weights[kind])
	profilePublicFontCache[style][kind] = h
	return h
}

func profileCustomizerTabRects(w, h int32) [4]RECT {
	r := profileSkinPanelRect(w, h)
	left := r.Left + sx(28, w)
	right := r.Right - sx(28, w)
	gap := sx(8, w)
	cw := (right - left - gap*3) / 4
	y := r.Top + sy(68, h)
	bh := sy(44, h)
	return [4]RECT{{left, y, left + cw, y + bh}, {left + cw + gap, y, left + 2*cw + gap, y + bh}, {left + 2*cw + 2*gap, y, left + 3*cw + 2*gap, y + bh}, {left + 3*cw + 3*gap, y, right, y + bh}}
}

func profileCustomizerFontRects(w, h int32) []RECT {
	r := profileSkinPanelRect(w, h)
	left := r.Left + sx(64, w)
	top := r.Top + sy(205, h)
	gapX := sx(16, w)
	gapY := sy(14, h)
	cw := (r.Right - r.Left - sx(128, w) - 3*gapX) / 4
	ch := sy(98, h)
	out := make([]RECT, 8)
	for i := 0; i < 8; i++ {
		col := int32(i % 4)
		row := int32(i / 4)
		x := left + col*(cw+gapX)
		y := top + row*(ch+gapY)
		out[i] = RECT{x, y, x + cw, y + ch}
	}
	return out
}

func drawRankFeatureLock(hdc uintptr, r RECT, required, desc string) {
	drawBevelPanel(hdc, r, rgb(22, 28, 39), rgb(93, 110, 130), rgb(6, 10, 18), 3)
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, rgb(240, 244, 249))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+42, "LOCKED // "+required)
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, rgb(160, 176, 192))
		centeredTextOut(hdc, r.Left+20, r.Right-20, r.Top+96, desc)
		selectObject.Call(hdc, old)
	}
}

func drawProfileFontPreview(hdc uintptr, r RECT, idx int, selected bool) {
	border := rgb(65, 94, 124)
	if selected {
		border = rgb(119, 88, 255)
	}
	drawBevelPanel(hdc, r, rgb(14, 29, 43), border, rgb(3, 12, 23), 2)
	pf := publicProfileFont(idx, 1)
	if pf != 0 {
		old, _, _ := selectObject.Call(hdc, pf)
		setTextColor.Call(hdc, rgb(245, 248, 252))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+18, "Aa")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, rgb(150, 180, 205))
		centeredTextOut(hdc, r.Left+6, r.Right-6, r.Bottom-25, uiFontLabels[idx])
		selectObject.Call(hdc, old)
	}
}

func profileBannerViewportRect(w, h int32) RECT {
	r := profileSkinPanelRect(w, h)
	return RECT{r.Left + sx(42, w), r.Top + sy(142, h), r.Right - sx(42, w), r.Bottom - sy(28, h)}
}

func profileBannerMaxScroll(w, h int32) int {
	v := profileBannerViewportRect(w, h)
	rowH := sy(146, h)
	gap := sy(18, h)
	visibleRows := int((v.Bottom - v.Top + gap) / (rowH + gap))
	if visibleRows < 1 {
		visibleRows = 1
	}
	rows := (len(profileSkinDefs) + 1) / 2
	m := rows - visibleRows
	if m < 0 {
		m = 0
	}
	return m
}

func profileBannerCardRects(w, h int32) []RECT {
	v := profileBannerViewportRect(w, h)
	scrollbarW := sx(18, w)
	gapX := sx(18, w)
	gapY := sy(18, h)
	usableRight := v.Right - scrollbarW - sx(8, w)
	cw := (usableRight - v.Left - gapX) / 2
	ch := sy(146, h)
	rowStep := ch + gapY
	startY := v.Top - int32(profileSkinScroll)*rowStep
	out := make([]RECT, len(profileSkinDefs))
	for i := range out {
		col := int32(i % 2)
		row := int32(i / 2)
		x := v.Left + col*(cw+gapX)
		y := startY + row*rowStep
		out[i] = RECT{x, y, x + cw, y + ch}
	}
	return out
}

func drawBannerScrollbar(hdc uintptr, w, h int32) {
	v := profileBannerViewportRect(w, h)
	max := profileBannerMaxScroll(w, h)
	track := RECT{v.Right - sx(10, w), v.Top + 4, v.Right - sx(4, w), v.Bottom - 4}
	fillSolidRect(hdc, track, rgb(21, 37, 53))
	if max <= 0 {
		return
	}
	rows := (len(profileSkinDefs) + 1) / 2
	visible := rows - max
	th := int32(float64(track.Bottom-track.Top) * float64(visible) / float64(rows))
	if th < sy(44, h) {
		th = sy(44, h)
	}
	travel := (track.Bottom - track.Top) - th
	y := track.Top + int32(float64(travel)*float64(profileSkinScroll)/float64(max))
	fillSolidRect(hdc, RECT{track.Left - 2, y, track.Right + 2, y + th}, rgb(95, 181, 235))
}

func drawBannerTab(hdc uintptr, w, h int32, theme profileTheme) {
	v := profileBannerViewportRect(w, h)
	drawBevelPanel(hdc, v, rgb(4, 15, 27), theme.Accent, theme.Deep, 2)
	saved, _, _ := saveDC.Call(hdc)
	intersectClipRect.Call(hdc, uintptr(v.Left+3), uintptr(v.Top+3), uintptr(v.Right-sx(16, w)), uintptr(v.Bottom-3))
	for i, cr := range profileBannerCardRects(w, h) {
		if i >= len(profileSkinDefs) || cr.Bottom <= v.Top || cr.Top >= v.Bottom {
			continue
		}
		d := profileSkinDefs[i]
		unlocked := profileSkinUnlocked(d.ID)
		equipped := gameMeta.SelectedProfileFrame == d.ID
		border := theme.Accent
		if equipped {
			border = rgb(72, 240, 120)
		} else if !unlocked {
			border = rgb(78, 88, 104)
		}
		drawBevelPanel(hdc, cr, theme.Panel2, border, theme.Deep, 2)
		preview := RECT{cr.Left + 7, cr.Top + 7, cr.Right - 7, cr.Top + 70}
		fillSolidRect(hdc, preview, rgb(2, 12, 28))
		drawProfileSkinPreview(hdc, preview, d.ID)
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, border)
			textOut(hdc, cr.Left+15, cr.Top+69, d.Name)
			selectObject.Call(hdc, old)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, theme.Muted)
			req := fitTextEllipsis(hdc, hudTinyFont, d.Requirement, cr.Right-cr.Left-20)
			textOut(hdc, cr.Left+15, cr.Top+91, req)
			status := "LOCKED"
			if unlocked {
				status = "CLICK TO EQUIP"
			}
			if equipped {
				status = "EQUIPPED"
			}
			setTextColor.Call(hdc, border)
			textOut(hdc, cr.Left+15, cr.Top+109, status)
			selectObject.Call(hdc, old)
		}
		if !unlocked {
			// Grey the complete locked card, including its banner artwork, so locked
			// cosmetics read as unavailable rather than merely having grey text.
			alphaSolidRect(hdc, RECT{cr.Left + 2, cr.Top + 2, cr.Right - 2, cr.Bottom - 2}, rgb(58, 62, 68), 155)
		}
	}
	if saved != 0 {
		restoreDC.Call(hdc, saved)
	}
	drawBannerScrollbar(hdc, w, h)
}

func profileColourCards(w, h int32) (RECT, RECT) {
	r := profileSkinPanelRect(w, h)
	top := r.Top + sy(295, h)
	gap := sx(18, w)
	cw := sx(340, w)
	cx := (r.Left + r.Right) / 2
	return RECT{cx - gap/2 - cw, top, cx - gap/2, top + sy(82, h)}, RECT{cx + gap/2, top, cx + gap/2 + cw, top + sy(82, h)}
}
func profileShadowRects(w, h int32) (RECT, RECT, RECT) {
	r := profileSkinPanelRect(w, h)
	y := r.Top + sy(520, h)
	gap := sx(14, w)
	total := r.Right - r.Left - sx(150, w)
	cw := (total - gap*2) / 3
	left := r.Left + sx(75, w)
	return RECT{left, y, left + cw, y + sy(48, h)}, RECT{left + cw + gap, y, left + 2*cw + gap, y + sy(48, h)}, RECT{left + 2*cw + 2*gap, y, left + 3*cw + 2*gap, y + sy(48, h)}
}
func advancedFontSelectorRect(w, h int32) RECT {
	r := profileSkinPanelRect(w, h)
	return RECT{r.Left + sx(120, w), r.Top + sy(164, h), r.Right - sx(120, w), r.Top + sy(264, h)}
}
func advancedPreviewRect(w, h int32) RECT {
	r := profileSkinPanelRect(w, h)
	return RECT{r.Left + sx(185, w), r.Top + sy(600, h), r.Right - sx(185, w), r.Top + sy(696, h)}
}

func drawColourCard(hdc uintptr, r RECT, label string, idx int, active bool) {
	if idx < 0 || idx >= len(profileStyleColours) {
		idx = 0
	}
	border := blendColor(profileStyleColours[idx], rgb(255, 255, 255), 0.25)
	if active {
		border = rgb(255, 215, 70)
	}
	drawBevelPanel(hdc, r, rgb(9, 18, 30), border, rgb(2, 7, 15), 2)
	sw := RECT{r.Left + 12, r.Top + 12, r.Left + 74, r.Bottom - 12}
	fillSolidRect(hdc, sw, profileStyleColours[idx])
	drawOutlineRect(hdc, sw, border, 2)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, rgb(245, 248, 252))
		textOut(hdc, sw.Right+14, r.Top+13, label)
		setTextColor.Call(hdc, profileStyleColours[idx])
		textOut(hdc, sw.Right+14, r.Top+43, profileStyleColourNames[idx])
		selectObject.Call(hdc, old)
	}
}

func profileColourSwatches(w, h int32) []RECT {
	r := profileSkinPanelRect(w, h)
	sz := sx(42, w)
	gap := sx(9, w)
	cols := int32(12)
	total := cols*sz + (cols-1)*gap
	left := (r.Left + r.Right - total) / 2
	y := r.Top + sy(396, h)
	out := make([]RECT, len(profileStyleColours))
	for i := range out {
		x := left + int32(i)*(sz+gap)
		out[i] = RECT{x, y, x + sz, y + sz}
	}
	return out
}
func drawColourSwatches(hdc uintptr, w, h int32) {
	for i, sr := range profileColourSwatches(w, h) {
		fillSolidRect(hdc, sr, profileStyleColours[i])
		outline := rgb(53, 72, 92)
		chosen := (profileColourTarget == 0 && gameMeta.ProfilePrimaryColour == i) || (profileColourTarget == 1 && gameMeta.ProfileSecondaryColour == i)
		if chosen {
			outline = rgb(255, 224, 70)
		}
		drawOutlineRect(hdc, sr, outline, 2)
	}
}

func selectedAdvancedNameLabel() string {
	if gameMeta.SelectedProfileNameFont <= 0 {
		return "USE PROFILE FONT"
	}
	i := gameMeta.SelectedProfileNameFont - 1
	if i >= 0 && i < len(advancedProfileFontNames) {
		return advancedProfileFontNames[i]
	}
	return "USE PROFILE FONT"
}

func drawAdvancedNameTab(hdc uintptr, w, h int32, theme profileTheme) {
	r := profileSkinPanelRect(w, h)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, theme.Accent)
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(133, h), "ADVANCED NAME // DISPLAY-NAME ONLY")
		selectObject.Call(hdc, old)
	}
	fs := advancedFontSelectorRect(w, h)
	drawBevelPanel(hdc, fs, rgb(8, 20, 34), theme.Accent, rgb(2, 8, 16), 2)
	advancedSelectorDrawn := false
	if gameMeta.SelectedProfileNameFont > 0 {
		drawn := drawAdvancedProfileTextInBox(hdc, RECT{fs.Left + 20, fs.Top + 8, fs.Right - 20, fs.Top + 62}, "Aa  "+selectedAdvancedNameLabel(), gameMeta.SelectedProfileNameFont, 1, 1, false, false, 0)
		advancedSelectorDrawn = drawn.Right > drawn.Left && drawn.Bottom > drawn.Top
	}
	if !advancedSelectorDrawn && hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, rgb(245, 248, 252))
		label := "USE PROFILE FONT"
		if gameMeta.SelectedProfileNameFont > 0 {
			label = selectedAdvancedNameLabel()
		}
		centeredTextOut(hdc, fs.Left, fs.Right, fs.Top+23, label)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, theme.Muted)
		centeredTextOut(hdc, fs.Left, fs.Right, fs.Bottom-24, "CLICK TO CYCLE // 12 ADVANCED FONTS")
		selectObject.Call(hdc, old)
	}
	pc, sc := profileColourCards(w, h)
	drawColourCard(hdc, pc, "PRIMARY COLOUR", gameMeta.ProfilePrimaryColour, profileColourTarget == 0)
	drawColourCard(hdc, sc, "SECONDARY COLOUR", gameMeta.ProfileSecondaryColour, profileColourTarget == 1)
	drawColourSwatches(hdc, w, h)
	shadowToggle, shadowColour, gradient := profileShadowRects(w, h)
	shadowValue := "OFF"
	if gameMeta.ProfileNameShadow {
		shadowValue = "ON"
	}
	uiButton(hdc, shadowToggle, "DROP SHADOW", shadowValue, gameMeta.ProfileNameShadow)
	uiButton(hdc, shadowColour, "SHADOW COLOUR", profileStyleColourNames[gameMeta.ProfileShadowColour], false)
	direction := "HORIZONTAL"
	if gameMeta.ProfileGradientVertical {
		direction = "VERTICAL"
	}
	uiButton(hdc, gradient, "GRADIENT", direction, false)
	pr := advancedPreviewRect(w, h)
	drawBevelPanel(hdc, pr, rgb(8, 20, 34), theme.Accent, rgb(2, 8, 16), 2)
	advancedPreviewDrawn := false
	if gameMeta.SelectedProfileNameFont > 0 {
		drawn := drawAdvancedProfileTextInBox(hdc, RECT{pr.Left + 16, pr.Top + 12, pr.Right - 16, pr.Bottom - 12}, localPlayerName(), gameMeta.SelectedProfileNameFont, gameMeta.ProfilePrimaryColour, gameMeta.ProfileSecondaryColour, gameMeta.ProfileGradientVertical, gameMeta.ProfileNameShadow, gameMeta.ProfileShadowColour)
		advancedPreviewDrawn = drawn.Right > drawn.Left && drawn.Bottom > drawn.Top
	}
	if !advancedPreviewDrawn {
		pf := publicProfileFont(gameMeta.SelectedProfileFont, 1)
		drawGradientProfileTextStyled(hdc, pr.Left+22, pr.Top+31, localPlayerName(), gameMeta.ProfilePrimaryColour, gameMeta.ProfileSecondaryColour, pf, gameMeta.ProfileGradientVertical, gameMeta.ProfileNameShadow, gameMeta.ProfileShadowColour)
	}
}

func profileAnimationCardRects(w, h int32) []RECT {
	r := profileSkinPanelRect(w, h)
	left := r.Left + sx(85, w)
	right := r.Right - sx(85, w)
	gap := sx(20, w)
	cw := (right - left - gap) / 2
	ch := sy(150, h)
	top := r.Top + sy(210, h)
	return []RECT{{left, top, left + cw, top + ch}, {left + cw + gap, top, right, top + ch}, {left, top + ch + sy(22, h), left + cw, top + 2*ch + sy(22, h)}, {left + cw + gap, top + ch + sy(22, h), right, top + 2*ch + sy(22, h)}}
}

func drawGlobalProfileCustomizer(hdc uintptr, w, h int32) {
	if overlayMode != OverlayProfileSkins {
		return
	}
	drawModalBackdrop(hdc, w, h)
	r := profileSkinPanelRect(w, h)
	theme := profileThemeForSkin(gameMeta.SelectedProfileFrame)
	drawBevelPanel(hdc, r, rgb(5, 22, 39), theme.Accent, rgb(1, 8, 18), 5)
	uiButton(hdc, profileSkinBackRect(w, h), "BACK", "", false)
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, rgb(248, 250, 253))
		textOut(hdc, r.Left+sx(165, w), r.Top+sy(20, h), "GLOBAL PROFILE CUSTOMISATION")
		selectObject.Call(hdc, old)
	}
	labels := []string{"FONT", "BANNERS", "ADVANCED NAME", "ANIMATIONS"}
	for i, tr := range profileCustomizerTabRects(w, h) {
		uiButton(hdc, tr, labels[i], "", profileCustomizeTab == i)
	}
	switch profileCustomizeTab {
	case 0:
		if !profileFontFeatureUnlocked() {
			drawRankFeatureLock(hdc, RECT{r.Left + 65, r.Top + 150, r.Right - 65, r.Bottom - 55}, "GOLD I", "Reach Global Gold I to choose the font used across your entire public Global Profile.")
			return
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, theme.Accent)
			centeredTextOut(hdc, r.Left, r.Right, r.Top+151, "GLOBAL PROFILE FONT // NO COLOUR CUSTOMISATION AT THIS TIER")
			selectObject.Call(hdc, old)
		}
		for i, fr := range profileCustomizerFontRects(w, h) {
			drawProfileFontPreview(hdc, fr, i, gameMeta.SelectedProfileFont == i)
		}
		pr := RECT{r.Left + 220, r.Top + 510, r.Right - 220, r.Top + 600}
		drawBevelPanel(hdc, pr, rgb(8, 20, 34), theme.Accent, rgb(2, 8, 16), 2)
		pf := publicProfileFont(gameMeta.SelectedProfileFont, 1)
		if pf != 0 {
			old, _, _ := selectObject.Call(hdc, pf)
			setTextColor.Call(hdc, rgb(245, 248, 252))
			centeredTextOut(hdc, pr.Left, pr.Right, pr.Top+30, localPlayerName())
			selectObject.Call(hdc, old)
		}
	case 1:
		if !profileBannersFeatureUnlocked() {
			drawRankFeatureLock(hdc, RECT{r.Left + 65, r.Top + 150, r.Right - 65, r.Bottom - 55}, "PLATINUM I", "Reach Global Platinum I to unlock Profile Banners. Each banner then has its own moderate challenge.")
			return
		}
		drawBannerTab(hdc, w, h, theme)
	case 2:
		if !profileNameStylingFeatureUnlocked() {
			drawRankFeatureLock(hdc, RECT{r.Left + 65, r.Top + 150, r.Right - 65, r.Bottom - 55}, "DIAMOND I", "Reach Global Diamond I to unlock Advanced Names: premium name fonts, two-colour gradients and shadow styling.")
			return
		}
		drawAdvancedNameTab(hdc, w, h, theme)
	case 3:
		if !profileAnimationFeatureUnlocked() {
			drawRankFeatureLock(hdc, RECT{r.Left + 65, r.Top + 150, r.Right - 65, r.Bottom - 55}, "MASTER I", "Reach Global Master I to unlock animated Global Profile effects.")
			return
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, theme.Muted)
			centeredTextOut(hdc, r.Left, r.Right, r.Top+155, "Animations are cosmetic and shown to everyone who opens your Global Profile.")
			selectObject.Call(hdc, old)
		}
		for i, cr := range profileAnimationCardRects(w, h) {
			active := gameMeta.ProfileAnimation == i
			value := "SELECT"
			if active {
				value = "EQUIPPED"
			}
			uiButton(hdc, cr, profileAnimationNames[i], value, active)
		}
	}
}

func drawGradientProfileTextStyled(hdc uintptr, x, y int32, text string, p1, p2 int, font uintptr, vertical, shadow bool, shadowC int) {
	if font == 0 || text == "" {
		return
	}
	if p1 < 0 || p1 >= len(profileStyleColours) {
		p1 = 1
	}
	if p2 < 0 || p2 >= len(profileStyleColours) {
		p2 = 2
	}
	if shadowC < 0 || shadowC >= len(profileStyleColours) {
		shadowC = 0
	}
	old, _, _ := selectObject.Call(hdc, font)
	defer selectObject.Call(hdc, old)
	setBkMode.Call(hdc, TRANSPARENT)
	if shadow {
		setTextColor.Call(hdc, profileStyleColours[shadowC])
		textOut(hdc, x+2, y+2, text)
	}
	if vertical {
		height := int32(38)
		step := int32(2)
		for yy := int32(0); yy < height; yy += step {
			t := float64(yy) / float64(max32(1, height-1))
			saved, _, _ := saveDC.Call(hdc)
			intersectClipRect.Call(hdc, uintptr(x), uintptr(y+yy), uintptr(x+2048), uintptr(y+minInt32(height, yy+step)))
			setTextColor.Call(hdc, blendColor(profileStyleColours[p1], profileStyleColours[p2], t))
			textOut(hdc, x, y, text)
			if saved != 0 {
				restoreDC.Call(hdc, saved)
			}
		}
		return
	}
	rr := []rune(text)
	n := len(rr)
	if n < 2 {
		n = 2
	}
	cur := x
	for i, ch := range rr {
		t := float64(i) / float64(n-1)
		setTextColor.Call(hdc, blendColor(profileStyleColours[p1], profileStyleColours[p2], t))
		st := string(ch)
		textOut(hdc, cur, y, st)
		cur += textPixelSize(hdc, font, st).Cx
	}
}
func drawGradientProfileText(hdc uintptr, x, y int32, text string, p1, p2 int, font uintptr) {
	drawGradientProfileTextStyled(hdc, x, y, text, p1, p2, font, false, false, 0)
}

func drawGlobalProfileAnimation(hdc uintptr, r RECT, anim int, primary, secondary int) {
	if anim <= 0 || anim >= 4 {
		return
	}
	now := float64(time.Now().UnixMilli()) / 1000.0
	c1 := profileStyleColours[primary%len(profileStyleColours)]
	c2 := profileStyleColours[secondary%len(profileStyleColours)]
	switch anim {
	case 1:
		p := 0.5 + 0.5*math.Sin(now*5)
		c := blendColor(c1, c2, p)
		drawOutlineRect(hdc, RECT{r.Left + 3, r.Top + 3, r.Right - 3, r.Bottom - 3}, c, 2)
	case 2:
		p := 0.5 + 0.5*math.Sin(now*3.5)
		c := blendColor(c1, c2, p)
		drawLineSimple(hdc, r.Left+4, r.Top+4, r.Right-4, r.Top+4, 3, c)
		drawLineSimple(hdc, r.Left+4, r.Bottom-5, r.Right-4, r.Bottom-5, 3, blendColor(c2, c1, p))
	case 3:
		yy := r.Top + int32(math.Mod(now*130, float64(max32(1, r.Bottom-r.Top))))
		fillSolidRect(hdc, RECT{r.Left + 6, yy, r.Right - 6, yy + 2}, blendColor(c1, rgb(255, 255, 255), 0.45))
	}
}

func handleProfileCustomizerClick(p FPoint, w, h int32) bool {
	if overlayMode != OverlayProfileSkins {
		return false
	}
	if pointInRect(p, profileSkinBackRect(w, h)) {
		playUIButtonClickSound()
		setOverlay(OverlayRemoteProfile)
		return true
	}
	for i, tr := range profileCustomizerTabRects(w, h) {
		if pointInRect(p, tr) {
			playUIButtonClickSound()
			profileCustomizeTab = i
			profileSkinScroll = 0
			invalidateRect.Call(mainHwnd, 0, 0)
			return true
		}
	}
	switch profileCustomizeTab {
	case 0:
		if !profileFontFeatureUnlocked() {
			return true
		}
		for i, r := range profileCustomizerFontRects(w, h) {
			if pointInRect(p, r) {
				gameMeta.SelectedProfileFont = i
				syncProfileStyleImmediate()
				return true
			}
		}
	case 1:
		if !profileBannersFeatureUnlocked() {
			return true
		}
		v := profileBannerViewportRect(w, h)
		if !pointInRect(p, v) {
			return true
		}
		for i, cr := range profileBannerCardRects(w, h) {
			if i < len(profileSkinDefs) && pointInRect(p, cr) && cr.Top >= v.Top && cr.Bottom <= v.Bottom {
				if profileSkinUnlocked(profileSkinDefs[i].ID) {
					equipProfileSkin(profileSkinDefs[i].ID)
				}
				return true
			}
		}
	case 2:
		if !profileNameStylingFeatureUnlocked() {
			return true
		}
		if pointInRect(p, advancedFontSelectorRect(w, h)) {
			gameMeta.SelectedProfileNameFont = (gameMeta.SelectedProfileNameFont + 1) % 13
			syncProfileStyleImmediate()
			return true
		}
		pc, sc := profileColourCards(w, h)
		if pointInRect(p, pc) {
			profileColourTarget = 0
			invalidateRect.Call(mainHwnd, 0, 0)
			return true
		}
		if pointInRect(p, sc) {
			profileColourTarget = 1
			invalidateRect.Call(mainHwnd, 0, 0)
			return true
		}
		for i, sr := range profileColourSwatches(w, h) {
			if pointInRect(p, sr) {
				if profileColourTarget == 0 {
					gameMeta.ProfilePrimaryColour = i
				} else {
					gameMeta.ProfileSecondaryColour = i
				}
				syncProfileStyleImmediate()
				return true
			}
		}
		st, scR, gr := profileShadowRects(w, h)
		if pointInRect(p, st) {
			gameMeta.ProfileNameShadow = !gameMeta.ProfileNameShadow
			syncProfileStyleImmediate()
			return true
		}
		if pointInRect(p, scR) {
			gameMeta.ProfileShadowColour = (gameMeta.ProfileShadowColour + 1) % len(profileStyleColours)
			syncProfileStyleImmediate()
			return true
		}
		if pointInRect(p, gr) {
			gameMeta.ProfileGradientVertical = !gameMeta.ProfileGradientVertical
			syncProfileStyleImmediate()
			return true
		}
	case 3:
		if !profileAnimationFeatureUnlocked() {
			return true
		}
		for i, r := range profileAnimationCardRects(w, h) {
			if pointInRect(p, r) {
				gameMeta.ProfileAnimation = i
				syncProfileStyleImmediate()
				return true
			}
		}
	}
	return true
}

func syncProfileStyleImmediate() {
	playUIButtonClickSound()
	saveGameMeta()
	requestPlayerProfileSync()
	if remoteProfileIsSelf() {
		remoteProfileMu.Lock()
		remoteProfile.SelectedProfileFont = gameMeta.SelectedProfileFont
		remoteProfile.SelectedProfileNameFont = gameMeta.SelectedProfileNameFont
		remoteProfile.ProfilePrimaryColour = gameMeta.ProfilePrimaryColour
		remoteProfile.ProfileSecondaryColour = gameMeta.ProfileSecondaryColour
		remoteProfile.ProfileNameShadow = gameMeta.ProfileNameShadow
		remoteProfile.ProfileShadowColour = gameMeta.ProfileShadowColour
		remoteProfile.ProfileGradientVertical = gameMeta.ProfileGradientVertical
		remoteProfile.ProfileAnimation = gameMeta.ProfileAnimation
		remoteProfileMu.Unlock()
	}
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}
func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
func profileFeatureSummary() string {
	return fmt.Sprintf("Font %v // Banners %v // Advanced %v // Animation %v", profileFontFeatureUnlocked(), profileBannersFeatureUnlocked(), profileNameStylingFeatureUnlocked(), profileAnimationFeatureUnlocked())
}
