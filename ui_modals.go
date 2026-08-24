//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func modalFadeAlpha() byte {
	if uiTransitionStart.IsZero() {
		return 220
	}
	elapsed := time.Since(uiTransitionStart)
	if elapsed >= 220*time.Millisecond {
		return 220
	}
	f := float64(elapsed) / float64(220*time.Millisecond)
	if gameMeta.ReducedMotion {
		f = 1
	}
	return byte(70 + 150*f)
}

func drawModalBackdrop(hdc uintptr, w, hgt int32) {
	overlayBlack(hdc, w, hgt, modalFadeAlpha())
}

func drawPanelFrame(hdc uintptr, r RECT, title, subtitle string) {
	_, cyan, dark := themedHUDPanelPalette(true)
	white := rgb(244, 249, 253)
	muted := blendColor(cyan, rgb(255, 255, 255), 0.28)
	drawStudioPanel(hdc, r, true)
	headerFill := blendColor(dark, rgb(255, 255, 255), 0.08)
	fillSolidRect(hdc, RECT{r.Left + 2, r.Top + 2, r.Right - 2, r.Top + 72}, headerFill)
	fillSolidRect(hdc, RECT{r.Left + 2, r.Top + 69, r.Right - 2, r.Top + 72}, cyan)
	titleBand := RECT{r.Left, r.Top + 7, r.Right, r.Top + 43}
	subtitleBand := RECT{r.Left, r.Top + 43, r.Right, r.Top + 68}
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, white)
		centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, hudTitleFont, titleBand, title), title)
		selectObject.Call(hdc, old)
	}
	if subtitle != "" && hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, hudTinyFont, subtitleBand, subtitle), subtitle)
		selectObject.Call(hdc, old)
	}
}

func uiButton(hdc uintptr, r RECT, label, value string, active bool) {
	drawStudioButtonBase(hdc, r, active)
	if hudSmallFont == 0 {
		return
	}

	o, _, _ := selectObject.Call(hdc, hudSmallFont)
	defer selectObject.Call(hdc, o)
	setBkMode.Call(hdc, TRANSPARENT)

	labelY := verticallyCenteredTextY(hdc, hudSmallFont, r, label)
	setTextColor.Call(hdc, rgb(255, 255, 255))
	textOut(hdc, r.Left+20, labelY, label)

	if value != "" {
		setTextColor.Call(hdc, rgb(255, 244, 78))
		sz := textPixelSize(hdc, hudSmallFont, value)
		valueY := verticallyCenteredTextY(hdc, hudSmallFont, r, value)
		textOut(hdc, r.Right-18-sz.Cx, valueY, value)
	}
}

func afkMainMenuRect(w, hgt int32) RECT {
	panel := centeredPanel(w, hgt, 980, 800)
	left := panel.Left + sx(72, w)
	right := panel.Right - sx(72, w)
	top := panel.Top + sy(350, hgt)
	bh := sy(55, hgt)
	if bh < 42 {
		bh = 42
	}
	return RECT{left, top, right, top + bh}
}

func mainMenuRects(w, hgt int32) [6]RECT {
	panel := centeredPanel(w, hgt, 980, 800)
	left := panel.Left + sx(72, w)
	right := panel.Right - sx(72, w)
	gapX := sx(22, w)
	gapY := sy(13, hgt)
	if gapY < 8 {
		gapY = 8
	}
	bh := sy(55, hgt)
	if bh < 42 {
		bh = 42
	}

	// The AFK Singularity launch button occupies the first featured row. Resume
	// and the existing menu actions retain their original shapes below it.
	top := panel.Top + sy(418, hgt)
	mid := (left + right) / 2
	var out [6]RECT
	out[0] = RECT{left, top, right, top + bh}
	y := top + bh + gapY
	out[1] = RECT{left, y, mid - gapX/2, y + bh}
	out[2] = RECT{mid + gapX/2, y, right, y + bh}
	y += bh + gapY
	out[3] = RECT{left, y, mid - gapX/2, y + bh}
	out[4] = RECT{mid + gapX/2, y, right, y + bh}
	y += bh + gapY
	out[5] = RECT{left, y, right, y + bh}
	return out
}

func releaseNotesRect(w, hgt int32) RECT {
	panel := centeredPanel(w, hgt, 980, 800)
	return RECT{panel.Left + sx(310, w), panel.Bottom - sy(132, hgt), panel.Right - sx(310, w), panel.Bottom - sy(90, hgt)}
}

func drawMainMenuStatusChip(hdc uintptr, r RECT, title, value string, accent uintptr) {
	face, _, dark := themedHUDCardPalette(0)
	drawBevelPanel(hdc, r, face, accent, dark, 2)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(132, 174, 207))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+6, title)
		setTextColor.Call(hdc, rgb(244, 251, 255))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+24, value)
		selectObject.Call(hdc, old)
	}
}

var musicCreditsPopupOpen bool

func musicCreditsButtonRect(w, hgt int32) RECT {
	panel := centeredPanel(w, hgt, 980, 800)
	return RECT{panel.Left + sx(24, w), panel.Bottom - sy(61, hgt), panel.Left + sx(116, w), panel.Bottom - sy(33, hgt)}
}

var musicCreditRows = []string{
	"PRECISION THEME  //  ASHUTOSH",
	"STARBASE THEME  //  JIGLR",
	"ENDURANCE THEME  //  PUNCH DECK",
	"SBOSS 2 THEME  //  @Makai Symphony",
	"SBOSS 3 THEME  //  @ScottBuckley",
}

var musicCreditNotices = []string{
	"CURSOR CONTROL DOES NOT OWN THESE TRACKS. RIGHTS REMAIN WITH THEIR CREATORS.",
	"USED UNDER THE ARTISTS' STATED FREE-USE TERMS.",
}

func musicCreditsPopupRect(w, hgt int32) RECT {
	panel := centeredPanel(w, hgt, 980, 800)
	longest := len("MUSIC CREDITS")
	for _, line := range musicCreditRows {
		if len(line) > longest {
			longest = len(line)
		}
	}
	for _, line := range musicCreditNotices {
		if len(line) > longest {
			longest = len(line)
		}
	}
	// Size the modal from its actual sentence lengths instead of a fixed box. The
	// tiny HUD font averages ~7 reference pixels per glyph; generous side padding
	// keeps even the longest rights notice clear of the bevel at every UI scale.
	ww := sx(float64(longest*7+96), w)
	minW := sx(570, w)
	maxW := (panel.Right - panel.Left) - sx(56, w)
	if ww < minW {
		ww = minW
	}
	if ww > maxW {
		ww = maxW
	}
	referenceH := 92 + len(musicCreditRows)*29 + 28 + len(musicCreditNotices)*26 + 30
	hh := sy(float64(referenceH), hgt)
	maxH := (panel.Bottom - panel.Top) - sy(56, hgt)
	if hh > maxH {
		hh = maxH
	}
	cx := (panel.Left + panel.Right) / 2
	cy := (panel.Top + panel.Bottom) / 2
	return RECT{cx - ww/2, cy - hh/2, cx + ww/2, cy + hh/2}
}

func drawMusicCreditsPopup(hdc uintptr, w, hgt int32) {
	if !musicCreditsPopupOpen {
		return
	}
	panel := centeredPanel(w, hgt, 980, 800)
	overlaySolidAlphaRect(hdc, panel, rgb(0, 0, 8), 175)
	r := musicCreditsPopupRect(w, hgt)
	drawBevelPanel(hdc, r, rgb(4, 19, 42), rgb(92, 218, 255), rgb(0, 5, 14), 4)
	drawOutlineRect(hdc, RECT{r.Left + 5, r.Top + 5, r.Right - 5, r.Bottom - 5}, rgb(174, 89, 255), 1)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 218, 79))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(22, hgt), "MUSIC CREDITS")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		y := r.Top + sy(68, hgt)
		setTextColor.Call(hdc, rgb(235, 247, 255))
		for _, line := range musicCreditRows {
			centeredTextOut(hdc, r.Left+sx(24, w), r.Right-sx(24, w), y, line)
			y += sy(29, hgt)
		}
		y += sy(12, hgt)
		setTextColor.Call(hdc, rgb(132, 174, 207))
		for _, line := range musicCreditNotices {
			centeredTextOut(hdc, r.Left+sx(24, w), r.Right-sx(24, w), y, line)
			y += sy(26, hgt)
		}
		selectObject.Call(hdc, old)
	}
}

func drawMainMenuOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayMainMenu {
		return
	}
	drawModalBackdrop(hdc, w, hgt)
	panel := centeredPanel(w, hgt, 980, 800)
	// Deep layered frame instead of the old flat blue rectangle.
	drawBevelPanel(hdc, panel, rgb(3, 30, 78), rgb(35, 207, 255), rgb(1, 10, 29), 5)
	inner := RECT{panel.Left + sx(10, w), panel.Top + sy(10, hgt), panel.Right - sx(10, w), panel.Bottom - sy(10, hgt)}
	drawOutlineRect(hdc, inner, rgb(18, 85, 168), 2)

	// Logo/header zone.
	headerBottom := panel.Top + sy(238, hgt)
	fillSolidRect(hdc, RECT{inner.Left + 2, inner.Top + 2, inner.Right - 2, headerBottom}, rgb(2, 18, 49))
	drawCursorControlImage(hdc, RECT{panel.Left + sx(250, w), panel.Top + sy(12, hgt), panel.Right - sx(250, w), panel.Top + sy(192, hgt)})
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(91, 225, 255))
		centeredTextOut(hdc, panel.Left, panel.Right, panel.Top+sy(197, hgt), "GAME MENU")
		selectObject.Call(hdc, old)
	}
	drawLineSimple(hdc, panel.Left+sx(28, w), headerBottom, panel.Right-sx(28, w), headerBottom, 3, rgb(255, 143, 15))

	// Compact live status strip makes the pause screen useful without duplicating
	// Profile/Local/Global Leaderboard buttons already present in the HUD.
	chipTop := panel.Top + sy(258, hgt)
	chipH := sy(64, hgt)
	chipGap := sx(14, w)
	chipLeft := panel.Left + sx(72, w)
	chipRight := panel.Right - sx(72, w)
	chipW := (chipRight - chipLeft - chipGap*2) / 3
	modeName := []string{"PRECISION", "ENDURANCE", "SURVIVAL"}[gameMode]
	diffName := "--"
	if gameMode == 0 {
		diffName = strings.ToUpper(diffs[activeDifficultyIndex()].name)
	} else if gameMode == 1 {
		diffName = "ENDURANCE"
	} else {
		diffName = fmt.Sprintf("WAVE %d", survivalWave)
	}
	online := "NOT LOGGED IN"
	if discordConnected {
		online = "LOGGED IN"
		if strings.TrimSpace(discordDisplayName) != "" {
			online = strings.TrimSpace(discordDisplayName)
		}
	}
	drawMainMenuStatusChip(hdc, RECT{chipLeft, chipTop, chipLeft + chipW, chipTop + chipH}, "MODE", modeName, rgb(34, 157, 242))
	drawMainMenuStatusChip(hdc, RECT{chipLeft + chipW + chipGap, chipTop, chipLeft + chipW*2 + chipGap, chipTop + chipH}, "CURRENT", diffName, rgb(255, 151, 17))
	drawMainMenuStatusChip(hdc, RECT{chipLeft + (chipW+chipGap)*2, chipTop, chipRight, chipTop + chipH}, "DISCORD", online, rgb(90, 218, 126))

	afkr := afkMainMenuRect(w, hgt)
	afkv := fmt.Sprintf("%d / %d STARBITS", afkSection1Progress(), afkSection1StarbitTarget)
	if afkSection1IsComplete() && !afkCursorCoreBuilt() {
		afkv = "BUILD CURSOR CORE"
	} else if afkCursorCoreBuilt() {
		afkv = fmt.Sprintf("TIER %d  •  %s STARBITS/S  •  %d/10 CURSORS", afkCoreTier(), afkStarbitsPerSecondText(), gameMeta.AFKAutoCursors)
	}
	drawBevelPanel(hdc, afkr, rgb(22, 28, 78), rgb(155, 96, 255), rgb(4, 7, 27), 3)
	if pointInRect(cursorPos, afkr) {
		drawOutlineRect(hdc, afkr, rgb(92, 226, 255), 2)
	}
	uiButton(hdc, afkr, "AFK // SINGULARITY", afkv, false)

	rects := mainMenuRects(w, hgt)
	labels := []string{"RESUME", "DISCORD ACCOUNT", "ACHIEVEMENTS", "SETTINGS", "PLAY TUTORIAL", "QUIT GAME"}
	values := []string{"BACK TO GAME", "LOGIN", fmt.Sprintf("%d / %d", len(gameMeta.Achievements), len(achievementDefinitions())), "AUDIO + DISPLAY", "CHOOSE MODE", "EXIT"}
	if discordConnected {
		values[1] = "LOGGED IN"
	}
	for i, r := range rects {
		if i == 5 {
			drawBevelPanel(hdc, r, rgb(107, 20, 31), rgb(255, 91, 103), rgb(43, 7, 15), 3)
			drawArcadeLabel(hdc, r, labels[i], rgb(255, 240, 242))
			continue
		}
		uiButton(hdc, r, labels[i], values[i], i == 0)
	}
	uiButton(hdc, releaseNotesRect(w, hgt), "VERSION 1.0", "NOTES", false)
	cr := musicCreditsButtonRect(w, hgt)
	drawBevelPanel(hdc, cr, rgb(4, 18, 39), rgb(62, 112, 151), rgb(0, 5, 13), 2)
	if pointInRect(cursorPos, cr) {
		drawOutlineRect(hdc, cr, rgb(92, 218, 255), 1)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(145, 189, 217))
		centeredTextOut(hdc, cr.Left, cr.Right, verticallyCenteredTextY(hdc, hudTinyFont, cr, "CREDITS"), "CREDITS")
		setTextColor.Call(hdc, rgb(132, 174, 207))
		centeredTextOut(hdc, panel.Left, panel.Right, panel.Bottom-sy(34, hgt), "ESC = RESUME   •   KONG GAME©   •   BUILD "+clientBuildVersion)
		selectObject.Call(hdc, old)
	}
	drawMusicCreditsPopup(hdc, w, hgt)
}

func centeredPanel(w, hgt int32, width, height float64) RECT {
	pw := sx(width, w)
	ph := sy(height, hgt)
	if pw > int32(float64(w)*0.92) {
		pw = int32(float64(w) * 0.92)
	}
	if ph > int32(float64(hgt)*0.88) {
		ph = int32(float64(hgt) * 0.88)
	}
	top := (hgt - ph) / 2
	if overlayMode != OverlayNone && overlayMode != OverlaySpaceCache {
		reveal := modalTransitionProgress()
		top += int32(float64(sy(18, hgt)) * (1.0 - reveal))
	}
	return RECT{(w - pw) / 2, top, (w + pw) / 2, top + ph}
}

func rankBadgeFileName(rank string) string {
	rank = strings.TrimSpace(strings.ToUpper(rank))
	repl := strings.NewReplacer(" ", "_")
	return strings.ToLower(repl.Replace(rank)) + ".bgra"
}

func getRankBadgeBytes(rank string) []byte {
	key := rankBadgeFileName(rank)
	if b, ok := rankBadgeCache[key]; ok {
		return b
	}
	b := readExternalBytes("ranks", "user_hq", key)
	if len(b) == 0 {
		return nil
	}
	rankBadgeCache[key] = b
	return b
}

func rankIndexForName(name string) int {
	name = strings.TrimSpace(strings.ToUpper(name))
	for i, r := range expRanks {
		if strings.ToUpper(r.Name) == name {
			return i
		}
	}
	return 0
}

func drawRankBadge(hdc uintptr, r RECT, rank string) {
	const badgeW = 1024
	const badgeH = 1024
	raw := getRankBadgeBytes(rank)
	if len(raw) < badgeW*badgeH*4 {
		drawRankEmblem(hdc, (r.Left+r.Right)/2, (r.Top+r.Bottom)/2, (r.Bottom-r.Top)/3, rank)
		return
	}
	drawRawBGRAFit(hdc, raw, badgeW, badgeH, r)
}

func drawRankEmblem(hdc uintptr, cx, cy, r int32, rank string) {
	color := rgb(177, 118, 68)
	u := strings.ToUpper(rank)
	switch {
	case strings.HasPrefix(u, "SILVER"):
		color = rgb(184, 198, 214)
	case strings.HasPrefix(u, "GOLD"):
		color = rgb(242, 192, 55)
	case strings.HasPrefix(u, "PLATINUM"):
		color = rgb(86, 213, 201)
	case strings.HasPrefix(u, "DIAMOND"):
		color = rgb(89, 155, 255)
	case strings.HasPrefix(u, "MASTER"):
		color = rgb(204, 83, 245)
	}
	outer, _, _ := createPen.Call(PS_SOLID, 4, color)
	if outer != 0 {
		old, _, _ := selectObject.Call(hdc, outer)
		ellipse.Call(hdc, uintptr(cx-r), uintptr(cy-r), uintptr(cx+r), uintptr(cy+r))
		ellipse.Call(hdc, uintptr(cx-r+10), uintptr(cy-r+10), uintptr(cx+r-10), uintptr(cy+r-10))
		selectObject.Call(hdc, old)
		deleteObject.Call(outer)
	}
	letter := "B"
	if len(u) > 0 {
		letter = string(u[0])
	}
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, color)
		centeredTextOut(hdc, cx-r, cx+r, cy-14, letter)
		selectObject.Call(hdc, old)
	}
}

func drawProgressBar(hdc uintptr, r RECT, current, total int, color uintptr) {
	if total <= 0 {
		total = 1
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	fillSolidRect(hdc, r, rgb(11, 36, 57))
	w := int32(float64(r.Right-r.Left) * float64(current) / float64(total))
	if w > 0 {
		fillSolidRect(hdc, RECT{r.Left, r.Top, r.Left + w, r.Bottom}, color)
	}
}

func localToGlobalProfileButtonRect(w, hgt int32) RECT {
	r := centeredPanel(w, hgt, 1180, 840)
	bw, bh := sx(196, w), sy(32, hgt)
	return RECT{r.Right - sx(28, w) - bw, r.Top + sy(34, hgt), r.Right - sx(28, w), r.Top + sy(34, hgt) + bh}
}

func drawProfileOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayProfile {
		return
	}
	drawModalBackdrop(hdc, w, hgt)

	r := centeredPanel(w, hgt, 1180, 840)
	// Outer deep-blue arcade frame.
	drawStudioPanel(hdc, r, true)

	white := rgb(255, 255, 255)
	cyan := rgb(80, 231, 255)
	muted := rgb(174, 208, 232)
	green := rgb(105, 236, 97)
	darkCard, _, _ := themedHUDCardPalette(2)

	// Header strip.
	header := RECT{r.Left + sx(18, w), r.Top + sy(18, hgt), r.Right - sx(18, w), r.Top + sy(84, hgt)}
	fillSolidRect(hdc, header, themedHUDHeaderColor())
	drawLineSimple(hdc, header.Left, header.Bottom, header.Right, header.Bottom, 3, cyan)
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, white)
		textOut(hdc, header.Left+sx(20, w), header.Top+sy(14, hgt), "PLAYER PROFILE")
		selectObject.Call(hdc, old)
	}
	// Local Profile is view-only. Profile appearance is edited from Global Profile.
	globalBtn := localToGlobalProfileButtonRect(w, hgt)
	drawStudioButtonBase(hdc, globalBtn, !discordConnected)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		centeredTextOut(hdc, globalBtn.Left, globalBtn.Right, verticallyCenteredTextY(hdc, hudSmallFont, globalBtn, "GLOBAL PROFILE"), "GLOBAL PROFILE")
		selectObject.Call(hdc, old)
	}

	// Global-profile-inspired local identity card: clean, view-only, no banner/customisation.
	hero := RECT{r.Left + sx(28, w), r.Top + sy(104, hgt), r.Right - sx(28, w), r.Top + sy(264, hgt)}
	heroFace, _, _ := themedHUDCardPalette(0)
	fillSolidRect(hdc, hero, heroFace)
	drawLineSimple(hdc, hero.Left, hero.Top, hero.Right, hero.Top, 2, cyan)
	drawLineSimple(hdc, hero.Left, hero.Bottom, hero.Right, hero.Bottom, 2, cyan)

	rank := rankForEXP(playerProgress.EXP)
	authMu.Lock()
	selfAvatar := append([]byte(nil), avatarAnimationFrame(discordAvatarAnim, discordAvatarBGRA)...)
	selfAvatarW, selfAvatarH := discordAvatarW, discordAvatarH
	authMu.Unlock()
	avatarRect := RECT{hero.Left + sx(18, w), hero.Top + sy(16, hgt), hero.Left + sx(142, w), hero.Bottom - sy(16, hgt)}
	drawProfileAvatar(hdc, avatarRect, selfAvatar, selfAvatarW, selfAvatarH, localPlayerName())
	// v329: local-profile rank emblem +25% for stronger progression identity.
	badgeSize := sy(138, hgt)
	if sx(138, w) < badgeSize {
		badgeSize = sx(138, w)
	}
	badgeTop := hero.Top + (hero.Bottom-hero.Top-badgeSize)/2
	badgeRect := RECT{avatarRect.Right + sx(14, w), badgeTop, avatarRect.Right + sx(14, w) + badgeSize, badgeTop + badgeSize}
	drawRankBadge(hdc, badgeRect, rank)

	nameX := badgeRect.Right + sx(18, w)
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, profileNameColour(gameMeta.SelectedNameColour))
		textOut(hdc, nameX, hero.Top+sy(18, hgt), strings.ToUpper(cleanDiscordDisplayName(localPlayerName())))
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, rgb(255, 220, 70))
		maxTitleW := hero.Right - sx(318, w) - nameX
		if maxTitleW < sx(120, w) {
			maxTitleW = hero.Right - sx(24, w) - nameX
		}
		profileTitle := fitTextEllipsis(hdc, hudTinyFont, selectedProfileTitle(), maxTitleW)
		textOut(hdc, nameX, hero.Top+sy(50, hgt), profileTitle)
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)

		// Local progression is device-specific. Label it explicitly so it cannot
		// be confused with the server-side/global progression rank.
		setTextColor.Call(hdc, white)
		textOut(hdc, nameX, hero.Top+sy(76, hgt), fmt.Sprintf("LOCAL RANK  %s", rank))
		textOut(hdc, nameX, hero.Top+sy(100, hgt), fmt.Sprintf("LOCAL EXP   %d", playerProgress.EXP))
		selectObject.Call(hdc, old)
	}
	globalPositionText := "GLOBAL POSITION  UNRANKED"
	globalRankText := "GLOBAL RANK  SYNCING..."
	if e, ok := localPlayerOverallEntry(); ok && e.Position > 0 {
		globalPositionText = fmt.Sprintf("GLOBAL POSITION  #%d", e.Position)
	}
	if serverRank, ok := localPlayerServerRank(); ok {
		globalRankText = fmt.Sprintf("GLOBAL RANK  %s", serverRank)
	} else if !discordConnected {
		globalRankText = "GLOBAL RANK  SIGN IN REQUIRED"
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, cyan)
		textOut(hdc, hero.Right-sx(300, w), hero.Top+sy(29, hgt), globalRankText)
		textOut(hdc, hero.Right-sx(300, w), hero.Top+sy(55, hgt), globalPositionText)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 && (strings.TrimSpace(gameMeta.CompetitiveBadge) != "" || strings.TrimSpace(gameMeta.SeasonBest) != "") {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 211, 69))
		label := strings.TrimSpace(gameMeta.CompetitiveBadge)
		if strings.TrimSpace(gameMeta.SeasonBest) != "" {
			if label != "" {
				label += "  •  "
			}
			label += strings.TrimSpace(gameMeta.SeasonBest)
		}
		label = fitTextEllipsis(hdc, hudTinyFont, label, sx(290, w))
		textOut(hdc, hero.Right-sx(300, w), hero.Top+sy(83, hgt), label)
		selectObject.Call(hdc, old)
	}

	// EXP bar + next rank.
	cur, next, hasNext, into, needed := nextRankProgress(playerProgress.EXP)
	bar := RECT{nameX, hero.Top + sy(137, hgt), hero.Right - sx(28, w), hero.Top + sy(153, hgt)}
	fillSolidRect(hdc, bar, rgb(5, 33, 75))
	fillEnd := bar.Right
	if hasNext && needed > 0 {
		q := float64(into) / float64(needed)
		if q < 0 {
			q = 0
		}
		if q > 1 {
			q = 1
		}
		fillEnd = bar.Left + int32(float64(bar.Right-bar.Left)*q)
	}
	if fillEnd > bar.Left {
		fillSolidRect(hdc, RECT{bar.Left, bar.Top, fillEnd, bar.Bottom}, arcadeGreen())
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		if hasNext {
			remaining := needed - into
			if remaining < 0 {
				remaining = 0
			}
			setTextColor.Call(hdc, rgb(255, 231, 79))
			textOut(hdc, nameX, hero.Top+sy(116, hgt), fmt.Sprintf("%d EXP UNTIL %s", remaining, next.Name))
		} else {
			setTextColor.Call(hdc, green)
			textOut(hdc, nameX, hero.Top+sy(116, hgt), "MAX RANK")
		}
		selectObject.Call(hdc, old)
	}
	_ = cur

	// Player-first profile summary. Keep the first row focused on the three game
	// modes plus long-term achievement progress; secondary lifetime counters no
	// longer compete with the information players actually compare.
	type stat struct{ label, value string }
	stats := []stat{
		{"PRECISION CLEARS", fmt.Sprint(gameMeta.TotalClears)},
		{"ENDURANCE BEST", fmt.Sprintf("%.0fm", gameMeta.BestEnduranceDistance)},
		{"SURVIVAL BEST", fmt.Sprintf("WAVE %d", gameMeta.BestSurvivalWave)},
		{"ACHIEVEMENTS", fmt.Sprintf("%d / %d", len(gameMeta.Achievements), len(achievementDefinitions()))},
	}
	gridLeft := r.Left + sx(28, w)
	gridTop := r.Top + sy(284, hgt)
	gap := sx(12, w)
	cardW := (r.Right - r.Left - sx(56, w) - gap*3) / 4
	cardH := sy(76, hgt)
	for i, st := range stats {
		x := gridLeft + int32(i)*(cardW+gap)
		cr := RECT{x, gridTop, x + cardW, gridTop + cardH}
		drawBevelPanel(hdc, cr, darkCard, rgb(43, 165, 220), rgb(2, 17, 39), 2)
		drawLineSimple(hdc, cr.Left+8, cr.Top+7, cr.Right-8, cr.Top+7, 1, rgb(47, 209, 255))
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, muted)
			centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(12, hgt), st.label)
			selectObject.Call(hdc, old)
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, white)
			centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(42, hgt), st.value)
			selectObject.Call(hdc, old)
		}
	}

	// Achievement showcase: difficulty colour remains visible even after completion.
	showcase := normalizedAchievementShowcase()
	showRects := profileShowcaseRects(r, w, hgt)
	for i, cr := range showRects {
		id := ""
		if i < len(showcase) {
			id = strings.TrimSpace(showcase[i])
		}
		accent := rgb(117, 78, 220)
		if id != "" {
			if a, ok := achievementByID(id); ok {
				switch achievementDifficultyLevel(id, a.Tier) {
				case 0:
					accent = rgb(59, 130, 246) // Easy blue
				case 1:
					accent = rgb(245, 158, 11) // Medium orange
				case 2:
					accent = rgb(239, 68, 68) // Expert red
				case 3:
					accent = rgb(192, 38, 211) // Legendary purple
				}
			}
		}
		showBG := blendColor(rgb(4, 16, 30), accent, 0.22)
		showDeep := blendColor(rgb(1, 8, 18), accent, 0.10)
		drawBevelPanel(hdc, cr, showBG, accent, showDeep, 2)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			if id != "" {
				if a, ok := achievementByID(id); ok {
					setTextColor.Call(hdc, accent)
					title := fitTextEllipsis(hdc, hudTinyFont, a.Title, cr.Right-cr.Left-sx(18, w))
					centeredTextOut(hdc, cr.Left+sx(8, w), cr.Right-sx(8, w), cr.Top+sy(7, hgt), title)
					setTextColor.Call(hdc, muted)
					desc := fitTextEllipsis(hdc, hudTinyFont, a.Description, cr.Right-cr.Left-sx(18, w))
					centeredTextOut(hdc, cr.Left+sx(8, w), cr.Right-sx(8, w), cr.Top+sy(27, hgt), desc)
					setTextColor.Call(hdc, accent)
					centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(49, hgt), achievementDifficultyName(id, a.Tier))
				}
			} else {
				setTextColor.Call(hdc, muted)
				centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(37, hgt), "NO SHOWCASE SET")
			}
			selectObject.Call(hdc, old)
		}
	}

	// Global rankings + public ship collection. These use fixed-height panels so
	// long names and statistics never spill outside their assigned boxes.
	positions := profilePositions(localPlayerName())
	rankR := RECT{r.Left + sx(28, w), r.Top + sy(486, hgt), r.Right - sx(28, w), r.Top + sy(552, hgt)}
	drawRankingCards(hdc, rankR, positions, w, hgt)

	pbs := profilePBs(localPlayerName())
	pbTop := r.Top + sy(568, hgt)
	pbLeft := r.Left + sx(28, w)
	pbRight := r.Right - sx(28, w)
	pbGap := sx(8, w)
	pbW := (pbRight - pbLeft - pbGap*4) / 5
	pbH := sy(105, hgt)
	for i, d := range []string{"EASY", "NORMAL", "HARD", "INSANE", "ENDURANCE"} {
		x := pbLeft + int32(i)*(pbW+pbGap)
		cr := RECT{x, pbTop, x + pbW, pbTop + pbH}
		drawBevelPanel(hdc, cr, rgb(7, 33, 67), rgb(34, 133, 185), rgb(2, 15, 37), 2)
		pe, has := pbs[d]
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, cyan)
			centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(8, hgt), d)
			setTextColor.Call(hdc, muted)
			if has {
				if d == "ENDURANCE" {
					dist := pe.Distance
					if dist <= 0 {
						dist = float64(pe.Score) / 10.0
					}
					centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(37, hgt), fmt.Sprintf("%.0fm • %d TARGETS", dist, pe.TargetsHit))
				} else {
					centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(37, hgt), fmt.Sprintf("%d • STREAK %d", pe.Score, pe.Streak))
				}
				centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(64, hgt), fmt.Sprintf("%.1f%% ACCURACY", pe.Accuracy))
			} else {
				centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(48, hgt), "NO GLOBAL PB")
			}
			selectObject.Call(hdc, old)
		}
	}

	ships := RECT{r.Left + sx(28, w), r.Top + sy(692, hgt), r.Right - sx(28, w), r.Bottom - sy(42, hgt)}
	drawShipCollectionStrip(hdc, ships, gameMeta.UnlockedShips, gameMeta.SelectedShip, w, hgt)
	drawBackHint(hdc, r, w, hgt)
}

func profileShowcaseRects(r RECT, w, hgt int32) [3]RECT {
	left := r.Left + sx(28, w)
	right := r.Right - sx(28, w)
	gap := sx(12, w)
	top := r.Top + sy(378, hgt)
	h := sy(82, hgt)
	wid := (right - left - gap*2) / 3
	return [3]RECT{
		{left, top, left + wid, top + h},
		{left + wid + gap, top, left + wid*2 + gap, top + h},
		{left + (wid+gap)*2, top, right, top + h},
	}
}

func profileCustomisationRects(r RECT, w, hgt int32) (nameR, titleR, avatarR RECT) {
	hero := RECT{r.Left + sx(28, w), r.Top + sy(104, hgt), r.Right - sx(28, w), r.Top + sy(264, hgt)}
	avatarR = RECT{hero.Left + sx(18, w), hero.Top + sy(16, hgt), hero.Left + sx(142, w), hero.Bottom - sy(16, hgt)}
	badgeSize := sy(138, hgt)
	if sx(138, w) < badgeSize {
		badgeSize = sx(138, w)
	}
	badgeRight := avatarR.Right + sx(14, w) + badgeSize
	nameX := badgeRight + sx(18, w)
	contentRight := hero.Right - sx(315, w)
	if contentRight <= nameX+sx(100, w) {
		contentRight = hero.Right - sx(20, w)
	}
	nameR = RECT{nameX, hero.Top + sy(10, hgt), contentRight, hero.Top + sy(45, hgt)}
	titleR = RECT{nameX, hero.Top + sy(45, hgt), contentRight, hero.Top + sy(72, hgt)}
	return
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func drawBackHint(hdc uintptr, r RECT, w, hgt int32) {
	if hudTinyFont == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, hudTinyFont)
	setTextColor.Call(hdc, rgb(145, 181, 208))
	centeredTextOut(hdc, r.Left, r.Right, r.Bottom-sy(30, hgt), "ESC / CLICK OUTSIDE PANEL = BACK")
	selectObject.Call(hdc, old)
}

const (
	settingsActionNone = iota
	settingsActionParticle
	settingsActionBackground
	settingsActionFPS
	settingsActionReducedMotion
	settingsActionShake
	settingsActionMusic
	settingsActionEffects
	settingsActionFailSound
	settingsActionFont
	settingsActionHUDCorners
	settingsActionHUDBackgroundTheme
	settingsActionEXPAnim
	settingsActionBossHPTheme
	settingsActionHover
	settingsActionAnnouncementTheme
	settingsActionCrosshairStyle
	settingsActionCrosshairSize
	settingsActionCrosshairColour
	settingsActionHitFeedback
	settingsActionShipHitbox
)

type settingsEntry struct {
	Category string
	Label    string
	Value    string
	Action   int
}

func settingsEntries() []settingsEntry {
	particle := []string{"OFF", "LOW", "HIGH"}[gameMeta.ParticleQuality]
	bgState := "OFF"
	if gameMeta.MovingBackground {
		bgState = "ON"
	}
	fps := []string{"60", "120", "UNLIMITED"}[gameMeta.FPSMode]
	motion := "OFF"
	if gameMeta.ReducedMotion {
		motion = "ON"
	}
	shakeNames := []string{"NORMAL", "OFF", "LOW", "HIGH"}
	cornerNames := []string{"SHARP / ANGULAR", "COMPACT", "ROUNDED SCI-FI", "HEAVY INDUSTRIAL"}
	hudBgNames := []string{"DEFAULT SCI-FI", "DARK GLASS", "MILITARY", "TERMINAL", "MINIMAL", "INDUSTRIAL"}
	expAnimNames := []string{"STATIC", "SUBTLE PULSE", "FLOWING HIGHLIGHT"}
	bossThemeNames := []string{"BOSS-SPECIFIC", "RED PLASMA", "SEGMENTED ARCADE", "CLEAN MINIMAL"}
	hoverNames := []string{"BRIGHTEN", "OUTLINE", "GLOW", "PULSE"}
	announcementNames := []string{"STANDARD SCI-FI", "WARNING", "HOLOGRAM", "MINIMAL", "INDUSTRIAL", "NEON"}
	styleNames := []string{"PLUS", "DOT", "PRECISION", "CIRCLE-DOT", "SNIPER"}
	sizeNames := []string{"SMALL", "MEDIUM", "LARGE"}
	colorNames := []string{"BLACK", "WHITE", "CYAN", "GREEN", "RED", "YELLOW", "PURPLE"}
	hit := "OFF"
	if hitFXEnabled {
		hit = "ON"
	}
	shipHitbox := "OFF"
	if gameMeta.ShowShipHitbox {
		shipHitbox = "ON"
	}
	return []settingsEntry{
		{Category: "PERFORMANCE"},
		{Label: "PARTICLE QUALITY", Value: particle, Action: settingsActionParticle},
		{Label: "MOVING BACKGROUND", Value: bgState, Action: settingsActionBackground},
		{Label: "FPS LIMIT", Value: fps, Action: settingsActionFPS},
		{Label: "REDUCED MOTION", Value: motion, Action: settingsActionReducedMotion},
		{Label: "SCREEN-SHAKE STRENGTH", Value: shakeNames[gameMeta.ScreenShakeStrength], Action: settingsActionShake},
		{Category: "AUDIO"},
		{Label: "MUSIC VOLUME", Value: fmt.Sprintf("%d%%", gameMeta.MusicVolume), Action: settingsActionMusic},
		{Label: "EFFECTS VOLUME", Value: fmt.Sprintf("%d%%", gameMeta.EffectsVolume), Action: settingsActionEffects},
		{Label: "PRECISION FAIL SOUND", Value: failureSoundName(), Action: settingsActionFailSound},
		{Category: "CUSTOMISATION"},
		{Label: "FONT OVERRIDE", Value: selectedUIFontLabel(), Action: settingsActionFont},
		{Label: "HUD CORNER STYLE", Value: cornerNames[gameMeta.HUDCornerStyle], Action: settingsActionHUDCorners},
		{Label: "HUD BACKGROUND THEME", Value: hudBgNames[gameMeta.HUDBackgroundTheme], Action: settingsActionHUDBackgroundTheme},
		{Label: "EXP BAR ANIMATION", Value: expAnimNames[gameMeta.EXPBarAnimation], Action: settingsActionEXPAnim},
		{Label: "BOSS HP BAR THEME", Value: bossThemeNames[gameMeta.BossHPBarTheme], Action: settingsActionBossHPTheme},
		{Label: "BUTTON HOVER EFFECT", Value: hoverNames[gameMeta.ButtonHoverEffect], Action: settingsActionHover},
		{Label: "ANNOUNCEMENT THEME", Value: announcementNames[gameMeta.AnnouncementTheme], Action: settingsActionAnnouncementTheme},
		{Category: "GAMEPLAY & ACCESSIBILITY"},
		{Label: "CROSSHAIR STYLE", Value: styleNames[gameMeta.CrosshairStyle], Action: settingsActionCrosshairStyle},
		{Label: "CROSSHAIR SIZE", Value: sizeNames[gameMeta.CrosshairSize], Action: settingsActionCrosshairSize},
		{Label: "CROSSHAIR COLOUR", Value: colorNames[gameMeta.CrosshairColor], Action: settingsActionCrosshairColour},
		{Label: "HIT FEEDBACK", Value: hit, Action: settingsActionHitFeedback},
		{Label: "ENDURANCE HITBOX", Value: shipHitbox, Action: settingsActionShipHitbox},
	}
}

func settingsPanelRect(w, hgt int32) RECT {
	r := centeredPanel(w, hgt, 900, 840)
	// v373: visually center the populated Settings menu lower on the Y axis.
	// The generic modal frame is mathematically centered, but the settings title/header
	// and scroll viewport made the menu feel top-heavy. Shift the whole Settings panel
	// down as one unit while keeping a safe bottom margin on smaller windows.
	offset := sy(42, hgt)
	maxBottom := hgt - sy(14, hgt)
	if r.Bottom+offset > maxBottom {
		offset = maxBottom - r.Bottom
	}
	if offset < 0 {
		offset = 0
	}
	r.Top += offset
	r.Bottom += offset
	return r
}

func settingsViewportRect(r RECT, w, hgt int32) RECT {
	return RECT{r.Left + sx(54, w), r.Top + sy(92, hgt), r.Right - sx(50, w), r.Bottom - sy(62, hgt)}
}

func settingsRowMetrics(w, hgt int32) (int32, int32) {
	bh := sy(38, hgt)
	if bh < 24 {
		bh = 24
	}
	gap := sy(5, hgt)
	if gap < 3 {
		gap = 3
	}
	return bh, gap
}

func settingsVisibleCount(w, hgt int32) int32 {
	r := settingsPanelRect(w, hgt)
	v := settingsViewportRect(r, w, hgt)
	bh, gap := settingsRowMetrics(w, hgt)
	n := (v.Bottom - v.Top + gap) / (bh + gap)
	if n < 1 {
		n = 1
	}
	return n
}

func settingsMaxScroll(w, hgt int32) int32 {
	m := int32(len(settingsEntries())) - settingsVisibleCount(w, hgt)
	if m < 0 {
		m = 0
	}
	return m
}

func clampSettingsScroll(w, hgt int32) {
	if settingsScroll < 0 {
		settingsScroll = 0
	}
	m := settingsMaxScroll(w, hgt)
	if settingsScroll > m {
		settingsScroll = m
	}
}

func settingsVisibleRows(w, hgt int32) []struct {
	Entry settingsEntry
	Rect  RECT
} {
	clampSettingsScroll(w, hgt)
	r := settingsPanelRect(w, hgt)
	v := settingsViewportRect(r, w, hgt)
	bh, gap := settingsRowMetrics(w, hgt)
	entries := settingsEntries()
	count := settingsVisibleCount(w, hgt)
	out := make([]struct {
		Entry settingsEntry
		Rect  RECT
	}, 0, count)
	left := v.Left
	right := v.Right - sx(24, w)
	for j := int32(0); j < count; j++ {
		idx := settingsScroll + j
		if idx >= int32(len(entries)) {
			break
		}
		top := v.Top + j*(bh+gap)
		br := RECT{left, top, right, top + bh}
		if br.Bottom > v.Bottom {
			break
		}
		out = append(out, struct {
			Entry settingsEntry
			Rect  RECT
		}{entries[idx], br})
	}
	return out
}

func settingsScrollbarRects(w, hgt int32) (RECT, RECT) {
	r := settingsPanelRect(w, hgt)
	v := settingsViewportRect(r, w, hgt)
	track := RECT{v.Right - sx(15, w), v.Top, v.Right, v.Bottom}
	max := settingsMaxScroll(w, hgt)
	if max <= 0 {
		return track, track
	}
	vis := settingsVisibleCount(w, hgt)
	total := int32(len(settingsEntries()))
	th := int32(float64(track.Bottom-track.Top) * float64(vis) / float64(total))
	if th < sy(54, hgt) {
		th = sy(54, hgt)
	}
	if th > track.Bottom-track.Top {
		th = track.Bottom - track.Top
	}
	rangeH := (track.Bottom - track.Top) - th
	y := track.Top + int32(float64(rangeH)*float64(settingsScroll)/float64(max))
	return track, RECT{track.Left + 2, y, track.Right - 2, y + th}
}

func settingsScrollbarDragTo(top int32, w, hgt int32) {
	track, thumb := settingsScrollbarRects(w, hgt)
	max := settingsMaxScroll(w, hgt)
	if max <= 0 {
		return
	}
	th := thumb.Bottom - thumb.Top
	minY := track.Top
	maxY := track.Bottom - th
	if top < minY {
		top = minY
	}
	if top > maxY {
		top = maxY
	}
	if maxY <= minY {
		return
	}
	settingsScroll = int32(math.Round(float64(top-minY) * float64(max) / float64(maxY-minY)))
	clampSettingsScroll(w, hgt)
}

func settingsSliderTrack(r RECT, w int32) RECT {
	left := r.Left + sx(285, w)
	right := r.Right - sx(92, w)
	if right <= left+20 {
		right = left + 20
	}
	cy := (r.Top + r.Bottom) / 2
	return RECT{left, cy - 4, right, cy + 4}
}

func drawSettingsVolumeSlider(hdc uintptr, r RECT, label string, value int, w int32) {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	drawStudioButtonBase(hdc, r, false)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		textOut(hdc, r.Left+sx(20, w), verticallyCenteredTextY(hdc, hudSmallFont, r, label), label)
		val := fmt.Sprintf("%d%%", value)
		sz := textPixelSize(hdc, hudSmallFont, val)
		setTextColor.Call(hdc, rgb(255, 244, 78))
		textOut(hdc, r.Right-sx(18, w)-sz.Cx, verticallyCenteredTextY(hdc, hudSmallFont, r, val), val)
		selectObject.Call(hdc, old)
	}
	track := settingsSliderTrack(r, w)
	fillSolidRect(hdc, track, rgb(2, 18, 45))
	drawOutlineRect(hdc, track, rgb(49, 209, 255), 1)
	fr := track.Left + int32(float64(track.Right-track.Left)*float64(value)/100.0)
	if fr > track.Left {
		fillSolidRect(hdc, RECT{track.Left, track.Top, fr, track.Bottom}, rgb(45, 214, 255))
	}
	knob := RECT{fr - 4, track.Top - 5, fr + 4, track.Bottom + 5}
	fillSolidRect(hdc, knob, rgb(238, 248, 255))
	drawOutlineRect(hdc, knob, rgb(5, 49, 105), 1)
}

func settingsVolumeFromPoint(row RECT, x int32, w int32) int {
	track := settingsSliderTrack(row, w)
	if x <= track.Left {
		return 0
	}
	if x >= track.Right {
		return 100
	}
	v := int(math.Round(float64(x-track.Left) * 100 / float64(track.Right-track.Left)))
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}

func settingsRowForAction(action int, w, hgt int32) (RECT, bool) {
	for _, row := range settingsVisibleRows(w, hgt) {
		if row.Entry.Action == action {
			return row.Rect, true
		}
	}
	return RECT{}, false
}

func updateSettingsVolumeFromPoint(which int, p FPoint, w, hgt int32) {
	action := settingsActionMusic
	if which == 2 {
		action = settingsActionEffects
	}
	row, ok := settingsRowForAction(action, w, hgt)
	if !ok {
		return
	}
	v := settingsVolumeFromPoint(row, int32(p.X), w)
	if which == 1 {
		gameMeta.MusicVolume = v
	} else {
		gameMeta.EffectsVolume = v
	}
	applyAudioVolumes()
	saveGameMeta()
}

func drawSettingsCategory(hdc uintptr, r RECT, label string, w int32) {
	fillSolidRect(hdc, r, rgb(2, 20, 48))
	drawLineSimple(hdc, r.Left, r.Bottom-1, r.Right, r.Bottom-1, 1, rgb(45, 205, 255))
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(94, 225, 255))
		textOut(hdc, r.Left+sx(12, w), verticallyCenteredTextY(hdc, hudSmallFont, r, label), label)
		selectObject.Call(hdc, old)
	}
}

func drawSettingsOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlaySettings {
		return
	}
	drawModalBackdrop(hdc, w, hgt)
	r := settingsPanelRect(w, hgt)
	drawPanelFrame(hdc, r, "SETTINGS", "PERFORMANCE • AUDIO • CUSTOMISATION • GAMEPLAY & ACCESSIBILITY")
	for _, row := range settingsVisibleRows(w, hgt) {
		e := row.Entry
		if e.Category != "" {
			drawSettingsCategory(hdc, row.Rect, e.Category, w)
			continue
		}
		switch e.Action {
		case settingsActionMusic:
			drawSettingsVolumeSlider(hdc, row.Rect, e.Label, gameMeta.MusicVolume, w)
		case settingsActionEffects:
			drawSettingsVolumeSlider(hdc, row.Rect, e.Label, gameMeta.EffectsVolume, w)
		default:
			uiButton(hdc, row.Rect, e.Label, e.Value, false)
		}
	}
	track, thumb := settingsScrollbarRects(w, hgt)
	fillSolidRect(hdc, track, rgb(1, 14, 34))
	drawOutlineRect(hdc, track, rgb(31, 101, 145), 1)
	fillSolidRect(hdc, thumb, rgb(39, 174, 226))
	drawOutlineRect(hdc, thumb, rgb(115, 230, 255), 1)
	drawBackHint(hdc, r, w, hgt)
}

func settingsActionAtPoint(p FPoint, w, hgt int32) (int, RECT) {
	for _, row := range settingsVisibleRows(w, hgt) {
		if row.Entry.Action != settingsActionNone && pointInRect(p, row.Rect) {
			return row.Entry.Action, row.Rect
		}
	}
	return settingsActionNone, RECT{}
}

func fitTextEllipsis(hdc, font uintptr, text string, maxWidth int32) string {
	text = strings.TrimSpace(text)
	if text == "" || font == 0 || maxWidth <= 0 {
		return text
	}
	if textPixelSize(hdc, font, text).Cx <= maxWidth {
		return text
	}
	ellipsis := "..."
	if textPixelSize(hdc, font, ellipsis).Cx > maxWidth {
		return ""
	}
	r := []rune(text)
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		candidate := strings.TrimSpace(string(r[:mid])) + ellipsis
		if textPixelSize(hdc, font, candidate).Cx <= maxWidth {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo <= 0 {
		return ellipsis
	}
	return strings.TrimSpace(string(r[:lo])) + ellipsis
}

func achievementPanelRect(w, hgt int32) RECT {
	return centeredPanel(w, hgt, 1080, 820)
}

func achievementScrollMetrics(r RECT, w, hgt int32) (viewTop, viewBottom, cardH, gapY, maxScroll int32) {
	viewTop = r.Top + sy(150, hgt)
	viewBottom = r.Bottom - sy(58, hgt)
	cardH = sy(96, hgt)
	if cardH < 72 {
		cardH = 72
	}
	gapY = sy(12, hgt)
	if gapY < 8 {
		gapY = 8
	}
	rows := (len(achievementDisplayDefinitions()) + 1) / 2
	contentH := int32(rows)*(cardH+gapY) - gapY
	viewH := viewBottom - viewTop
	maxScroll = contentH - viewH
	if maxScroll < 0 {
		maxScroll = 0
	}
	return
}

func clampAchievementScroll(w, hgt int32) {
	r := achievementPanelRect(w, hgt)
	_, _, _, _, maxScroll := achievementScrollMetrics(r, w, hgt)
	if achievementScroll < 0 {
		achievementScroll = 0
	}
	if achievementScroll > maxScroll {
		achievementScroll = maxScroll
	}
}

func scrollAchievements(delta int32, w, hgt int32) {
	step := sy(150, hgt)
	if step < 90 {
		step = 90
	}
	if delta > 0 {
		achievementScroll -= step
	} else if delta < 0 {
		achievementScroll += step
	}
	clampAchievementScroll(w, hgt)
}

func achievementScrollbarRects(w, hgt int32) (track, thumb RECT) {
	r := achievementPanelRect(w, hgt)
	viewTop, viewBottom, _, _, maxScroll := achievementScrollMetrics(r, w, hgt)
	track = RECT{r.Right - sx(27, w), viewTop, r.Right - sx(15, w), viewBottom}
	if maxScroll <= 0 {
		thumb = track
		return
	}
	viewH := viewBottom - viewTop
	rows := (len(achievementDisplayDefinitions()) + 1) / 2
	_, _, cardH, gapY, _ := achievementScrollMetrics(r, w, hgt)
	contentH := int32(rows)*(cardH+gapY) - gapY
	thumbH := int32(float64(viewH) * float64(viewH) / float64(contentH))
	minThumb := sy(48, hgt)
	if thumbH < minThumb {
		thumbH = minThumb
	}
	travel := viewH - thumbH
	top := viewTop
	if maxScroll > 0 {
		top += int32(float64(travel) * float64(achievementScroll) / float64(maxScroll))
	}
	thumb = RECT{track.Left, top, track.Right, top + thumbH}
	return
}

func updateAchievementDrag(mouseY int32, w, hgt int32) {
	track, thumb := achievementScrollbarRects(w, hgt)
	r := achievementPanelRect(w, hgt)
	_, _, _, _, maxScroll := achievementScrollMetrics(r, w, hgt)
	if maxScroll <= 0 {
		achievementScroll = 0
		return
	}

	thumbH := thumb.Bottom - thumb.Top
	travel := (track.Bottom - track.Top) - thumbH
	if travel <= 0 {
		achievementScroll = 0
		return
	}

	newTop := mouseY - achievementDragOffset
	if newTop < track.Top {
		newTop = track.Top
	}
	if newTop > track.Top+travel {
		newTop = track.Top + travel
	}
	f := float64(newTop-track.Top) / float64(travel)
	achievementScroll = int32(math.Round(f * float64(maxScroll)))
	clampAchievementScroll(w, hgt)
}

func achievementScrollbarClick(y int32, w, hgt int32) {
	track, thumb := achievementScrollbarRects(w, hgt)
	if y < thumb.Top {
		achievementScroll -= track.Bottom - track.Top
	} else if y > thumb.Bottom {
		achievementScroll += track.Bottom - track.Top
	}
	clampAchievementScroll(w, hgt)
}

func achievementDisplayDefinitions() []AchievementDef {
	defs := append([]AchievementDef(nil), achievementDefinitions()...)
	sort.SliceStable(defs, func(i, j int) bool {
		li := achievementDifficultyLevel(defs[i].ID, defs[i].Tier)
		lj := achievementDifficultyLevel(defs[j].ID, defs[j].Tier)
		if li != lj {
			return li < lj // Easy first, Legendary last
		}
		a := strings.ToLower(strings.TrimSpace(defs[i].Title))
		b := strings.ToLower(strings.TrimSpace(defs[j].Title))
		if a != b {
			return a < b
		}
		return defs[i].ID < defs[j].ID
	})
	if achievementFilter == 0 {
		return defs
	}
	filtered := make([]AchievementDef, 0, len(defs))
	for _, a := range defs {
		unlocked := achievementUnlocked(a.ID)
		if (achievementFilter == 1 && unlocked) || (achievementFilter == 2 && !unlocked) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func achievementFilterRects(r RECT, w, hgt int32) [3]RECT {
	gap := sx(14, w)
	boxW := sx(180, w)
	boxH := sy(30, hgt)
	if boxH < 24 {
		boxH = 24
	}
	total := boxW*3 + gap*2
	left := r.Left + (r.Right-r.Left-total)/2
	top := r.Top + sy(104, hgt)
	return [3]RECT{
		{left, top, left + boxW, top + boxH},
		{left + boxW + gap, top, left + boxW*2 + gap, top + boxH},
		{left + boxW*2 + gap*2, top, left + boxW*3 + gap*2, top + boxH},
	}
}

func drawAchievementFilters(hdc uintptr, r RECT, w, hgt int32) {
	labels := []string{"ALL", "UNLOCKED", "LOCKED"}
	rects := achievementFilterRects(r, w, hgt)
	for i, fr := range rects {
		selected := achievementFilter == i
		border := rgb(60, 120, 155)
		bg := rgb(7, 26, 42)
		check := rgb(70, 100, 120)
		if selected {
			border = rgb(47, 211, 255)
			bg = rgb(8, 45, 66)
			check = rgb(34, 197, 94)
		}
		fillSolidRect(hdc, fr, bg)
		drawOutlineRect(hdc, fr, border, 2)
		cs := sy(14, hgt)
		if cs < 12 {
			cs = 12
		}
		cy := fr.Top + (fr.Bottom-fr.Top-cs)/2
		cb := RECT{fr.Left + sx(10, w), cy, fr.Left + sx(10, w) + cs, cy + cs}
		fillSolidRect(hdc, cb, rgb(3, 15, 27))
		drawOutlineRect(hdc, cb, border, 1)
		if selected {
			fillSolidRect(hdc, RECT{cb.Left + 3, cb.Top + 3, cb.Right - 3, cb.Bottom - 3}, check)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, rgb(235, 245, 250))
			textOut(hdc, cb.Right+sx(8, w), fr.Top+(fr.Bottom-fr.Top-textPixelSize(hdc, hudTinyFont, labels[i]).Cy)/2, labels[i])
			selectObject.Call(hdc, old)
		}
	}
}

func drawWrappedLeftText(hdc uintptr, font uintptr, r RECT, text string, lineGap int32, maxLines int) int32 {
	if font == 0 || r.Right <= r.Left || r.Bottom <= r.Top || maxLines <= 0 {
		return 0
	}
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return 0
	}
	lineH := textPixelSize(hdc, font, "Ag").Cy
	if lineH <= 0 {
		lineH = 13
	}
	lines := make([]string, 0, maxLines)
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if textPixelSize(hdc, font, candidate).Cx <= r.Right-r.Left {
			line = candidate
			continue
		}
		if line != "" {
			lines = append(lines, line)
			if len(lines) == maxLines {
				break
			}
		}
		line = word
	}
	if len(lines) < maxLines && line != "" {
		lines = append(lines, line)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	y := r.Top
	for i, lineText := range lines {
		if i == maxLines-1 {
			lineText = fitTextEllipsis(hdc, font, lineText, r.Right-r.Left)
		}
		textOut(hdc, r.Left, y, lineText)
		y += lineH + lineGap
	}
	return y - r.Top
}

func achievementAtPoint(p FPoint, w, hgt int32) (AchievementDef, bool) {
	r := achievementPanelRect(w, hgt)
	viewTop, viewBottom, cardH, gapY, _ := achievementScrollMetrics(r, w, hgt)
	startX := r.Left + sx(42, w)
	contentRight := r.Right - sx(48, w)
	gapX := sx(18, w)
	cardW := (contentRight - startX - gapX) / 2
	if p.X < float64(startX) || p.X > float64(contentRight) || p.Y < float64(viewTop) || p.Y > float64(viewBottom) {
		return AchievementDef{}, false
	}
	for i, a := range achievementDisplayDefinitions() {
		col := i % 2
		row := i / 2
		y := viewTop + int32(row)*(cardH+gapY) - achievementScroll
		x := startX + int32(col)*(cardW+gapX)
		cr := RECT{x, y, x + cardW, y + cardH}
		if pointInRect(p, cr) {
			return a, true
		}
	}
	return AchievementDef{}, false
}

func drawAchievementsOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayAchievements {
		return
	}
	drawModalBackdrop(hdc, w, hgt)

	r := achievementPanelRect(w, hgt)
	defs := achievementDisplayDefinitions()
	drawPanelFrame(hdc, r, "ACHIEVEMENTS", fmt.Sprintf("%d / %d UNLOCKED  •  EASY → MEDIUM → EXPERT → LEGENDARY", len(gameMeta.Achievements), len(achievementDefinitions())))
	clampAchievementScroll(w, hgt)
	drawAchievementFilters(hdc, r, w, hgt)

	viewTop, viewBottom, cardH, gapY, _ := achievementScrollMetrics(r, w, hgt)
	startX := r.Left + sx(42, w)
	scrollSpace := sx(48, w)
	contentRight := r.Right - scrollSpace
	gapX := sx(18, w)
	cardW := (contentRight - startX - gapX) / 2

	// Hard clip: every card, border and text pixel is restricted to the scroll viewport.
	saved, _, _ := saveDC.Call(hdc)
	intersectClipRect.Call(hdc, uintptr(startX), uintptr(viewTop), uintptr(contentRight), uintptr(viewBottom))

	for i, a := range defs {
		col := i % 2
		row := i / 2
		y := viewTop + int32(row)*(cardH+gapY) - achievementScroll
		if y+cardH <= viewTop || y >= viewBottom {
			continue
		}

		x := startX + int32(col)*(cardW+gapX)
		cr := RECT{x, y, x + cardW, y + cardH}

		unlocked := achievementUnlocked(a.ID)
		displayTitle := a.Title
		displayDescription := a.Description
		if achievementHidden(a.ID) && !unlocked {
			displayTitle = "SECRET ACHIEVEMENT"
			displayDescription = "Discover this challenge through play."
		}
		var bg, border, titleColor, tierColor uintptr
		accent := rgb(59, 130, 246) // Easy blue
		switch achievementDifficultyLevel(a.ID, a.Tier) {
		case 1:
			accent = rgb(245, 158, 11) // Medium orange
		case 2:
			accent = rgb(239, 68, 68) // Expert red
		case 3:
			accent = rgb(192, 38, 211) // Legendary purple
		}
		baseCard, _, _ := themedHUDCardPalette(2)
		bg = blendColor(baseCard, accent, 0.18)
		border = accent
		tierColor = accent
		titleColor = rgb(218, 225, 230)
		if unlocked {
			// Completed achievements use a clear success-green state in the Achievements
			// tab. Public showcase cards retain difficulty colours separately.
			accent = rgb(34, 197, 94) // #22C55E
			bg = blendColor(baseCard, accent, 0.24)
			border = accent
			tierColor = accent
			titleColor = rgb(250, 252, 253)
		} else {
			bg = blendColor(bg, themedHUDHeaderColor(), 0.34)
		}

		fillSolidRect(hdc, cr, bg)
		drawLineSimple(hdc, cr.Left, cr.Top, cr.Right, cr.Top, 2, border)
		drawLineSimple(hdc, cr.Left, cr.Bottom, cr.Right, cr.Bottom, 2, border)
		drawLineSimple(hdc, cr.Left, cr.Top, cr.Left, cr.Bottom, 2, border)
		drawLineSimple(hdc, cr.Right, cr.Top, cr.Right, cr.Bottom, 2, border)

		statusW := sx(120, w)
		if statusW < 102 {
			statusW = 102
		}
		statusLeft := cr.Right - statusW
		drawLineSimple(hdc, statusLeft, cr.Top+8, statusLeft, cr.Bottom-8, 1, blendColor(border, bg, 0.55))

		contentR := RECT{cr.Left + 14, cr.Top + 8, statusLeft - 10, cr.Bottom - 8}
		titleH := textPixelSize(hdc, hudSmallFont, displayTitle).Cy
		if titleH <= 0 {
			titleH = 15
		}
		contentGap := sy(5, hgt)
		if contentGap < 4 {
			contentGap = 4
		}
		titleTop := contentR.Top
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, titleColor)
			cardTitle := fitTextEllipsis(hdc, hudSmallFont, displayTitle, contentR.Right-contentR.Left)
			textOut(hdc, contentR.Left, titleTop, cardTitle)
			selectObject.Call(hdc, old)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, rgb(180, 198, 211))
			descTop := titleTop + titleH + contentGap
			descBottom := cr.Bottom - sy(27, hgt)
			progressVisible := false
			progressCurrent, progressTarget, progressOK := achievementProgress(a.ID)
			if !unlocked && !achievementHidden(a.ID) && progressOK && progressCurrent > 0 && progressTarget > 0 {
				progressVisible = true
				descBottom -= sy(18, hgt)
			}
			if descBottom > descTop {
				drawWrappedLeftText(hdc, hudTinyFont, RECT{contentR.Left, descTop, contentR.Right, descBottom}, displayDescription, sy(2, hgt), 2)
			}
			if progressVisible {
				barH := sy(7, hgt)
				if barH < 5 {
					barH = 5
				}
				barBottom := cr.Bottom - sy(8, hgt)
				bar := RECT{contentR.Left, barBottom - barH, contentR.Right, barBottom}
				fillSolidRect(hdc, bar, rgb(3, 18, 30))
				q := progressCurrent / progressTarget
				if q < 0 {
					q = 0
				}
				if q > 0.999 {
					q = 0.999
				}
				fillRight := bar.Left + int32(float64(bar.Right-bar.Left)*q)
				progressColor := rgb(225, 50, 55)
				if q >= 0.60 {
					progressColor = rgb(255, 218, 58)
				} else if q >= 0.30 {
					progressColor = rgb(245, 145, 28)
				}
				if fillRight > bar.Left {
					fillSolidRect(hdc, RECT{bar.Left, bar.Top, fillRight, bar.Bottom}, progressColor)
				}
				progressText := fmt.Sprintf("%.0f / %.0f", math.Min(progressCurrent, progressTarget), progressTarget)
				setTextColor.Call(hdc, rgb(205, 224, 236))
				textOut(hdc, contentR.Left, bar.Top-sy(15, hgt), progressText)
			}

			stateText := "LOCKED"
			if unlocked {
				stateText = "COMPLETED"
			}
			tierText := achievementDifficultyName(a.ID, a.Tier)
			rewardText := fmt.Sprintf("+%d EXP", achievementRewardEXP(a.Tier))
			if achievementHidden(a.ID) && !unlocked {
				rewardText = "SECRET"
			}
			if t := achievementTitleReward(a.ID); t != "" {
				rewardText = "TITLE: " + t
			}
			if unlocked && a.ID == achievementShowcaseConfirmID && time.Now().Before(achievementShowcaseConfirmUntil) {
				action := fmt.Sprintf("SET AS SHOWCASE %d", achievementShowcaseConfirmSlot+1)
				setTextColor.Call(hdc, rgb(105, 229, 255))
				action = fitTextEllipsis(hdc, hudTinyFont, action, contentR.Right-contentR.Left)
				textOut(hdc, contentR.Left, cr.Bottom-sy(22, hgt), action)
			}
			statusR := RECT{statusLeft, cr.Top + 8, cr.Right - 4, cr.Bottom - 8}
			rewardText = fitTextEllipsis(hdc, hudTinyFont, rewardText, statusR.Right-statusR.Left-8)
			statusH1 := textPixelSize(hdc, hudTinyFont, tierText).Cy
			statusH2 := textPixelSize(hdc, hudTinyFont, rewardText).Cy
			statusH3 := textPixelSize(hdc, hudTinyFont, stateText).Cy
			if statusH1 <= 0 {
				statusH1 = 13
			}
			if statusH2 <= 0 {
				statusH2 = 13
			}
			if statusH3 <= 0 {
				statusH3 = 13
			}
			statusGap := sy(5, hgt)
			statusTop := statusR.Top + ((statusR.Bottom-statusR.Top)-(statusH1+statusH2+statusH3+statusGap*2))/2
			setTextColor.Call(hdc, tierColor)
			centeredTextOut(hdc, statusR.Left, statusR.Right, statusTop, tierText)
			centeredTextOut(hdc, statusR.Left, statusR.Right, statusTop+statusH1+statusGap, rewardText)
			centeredTextOut(hdc, statusR.Left, statusR.Right, statusTop+statusH1+statusGap+statusH2+statusGap, stateText)
			selectObject.Call(hdc, old)
		}
	}
	if saved != 0 {
		restoreDC.Call(hdc, saved)
	}

	// Scrollbar and footer deliberately render OUTSIDE the clipped card viewport.
	track, thumb := achievementScrollbarRects(w, hgt)
	fillSolidRect(hdc, track, rgb(8, 30, 47))
	fillSolidRect(hdc, thumb, rgb(26, 205, 239))
}

func tutorialPanelRect(w, hgt int32) RECT {
	return centeredPanel(w, hgt, 920, 650)
}

func tutorialModeCardRects(w, hgt int32) [3]RECT {
	r := tutorialPanelRect(w, hgt)
	cardW := sx(690, w)
	cardH := sy(104, hgt)
	gap := sy(14, hgt)
	left := (r.Left + r.Right - cardW) / 2
	top := r.Top + sy(176, hgt)
	return [3]RECT{
		{left, top, left + cardW, top + cardH},
		{left, top + cardH + gap, left + cardW, top + cardH*2 + gap},
		{left, top + (cardH+gap)*2, left + cardW, top + cardH*3 + gap*2},
	}
}

func tutorialBackRect(w, hgt int32) RECT {
	r := tutorialPanelRect(w, hgt)
	return RECT{r.Left + sx(38, w), r.Bottom - sy(78, hgt), r.Left + sx(250, w), r.Bottom - sy(34, hgt)}
}

func tutorialNextRect(w, hgt int32) RECT {
	r := tutorialPanelRect(w, hgt)
	return RECT{r.Right - sx(250, w), r.Bottom - sy(78, hgt), r.Right - sx(38, w), r.Bottom - sy(34, hgt)}
}

func tutorialPageCount(mode int) int {
	if mode < 0 || mode > 2 {
		return 0
	}
	return 4
}

func tutorialCopy(mode, page int) (string, string, string) {
	precision := [4][3]string{
		{"1. START THE RUN", "Move the cursor onto START to begin. Precision Mode is about tracing the generated rail cleanly from START to END.", "The timer starts as soon as the run engages, so settle your cursor before entering START."},
		{"2. STAY INSIDE THE RAIL", "Guide the cursor continuously along the path. The cursor point must remain inside the permitted track at all times.", "The centre of the rail gives the most room for turns. Touching or crossing the boundary ends the attempt immediately."},
		{"3. HIT TARGETS IN ORDER", "Click every target that appears on the path in the correct order while continuing to trace the rail.", "Missing a required target, clicking the wrong target, or making an unnecessary shot can fail the run."},
		{"4. REACH END", "After all required targets are cleared, continue to END to complete the run.", "Your result is driven by difficulty, speed, accuracy and streak. Harder tracks are narrower and demand cleaner control."},
	}
	endurance := [4][3]string{
		{"1. SURVIVE THE RAIL", "Endurance Mode is an endless run. Keep your spaceship cursor inside the rail and travel as far to the right as possible.", "Distance is your main score. The ship skin is cosmetic only: every ship uses the same gameplay cursor-point collision."},
		{"2. TARGETS & HAZARDS", "Hit targets as they appear while avoiding meteorites and other hazards. Leaving the rail or touching a lethal hazard ends the run.", "Hazards become more intense as distance increases, but target hits, accuracy and survival all contribute to progression."},
		{"3. POWER-UPS", "Shield and Time Slow can be collected into storage. Press Q to activate Shield and W to activate Time Slow.", "+100m Distance activates immediately. Shield protects against meteorites and minions, but it does NOT protect you from the UFO laser."},
		{"4. SPECIAL PHASES", "Long runs include warp corridors, meteor events and alien encounters. Read the warnings and react before the danger arrives.", "Warp phases accelerate the run; alien laser attacks must be dodged. Keep control through the recovery section before normal Endurance resumes."},
	}
	survival := [4][3]string{
		{"1. DEFEND THE STATION", "Survival Mode is a wave defence mode. Enemies enter the arena and must be destroyed before they damage the station.", "Clear waves to progress through each sector. Station health carries through the current run, so avoid unnecessary damage."},
		{"2. MATCH THE CLICK", "RED enemies require LEFT CLICK. BLUE enemies require RIGHT CLICK. Large enemies take 2 hits and deal 2 station damage if they get through.", "Reaction speed matters: identify the colour first, then use the correct mouse button."},
		{"3. USE SURVIVAL POWER-UPS", "Collect defensive power-ups such as Shield, Slow, Repair, Bomb and Chain when they appear.", "Use them to control crowded waves and protect station health. Boss checkpoints are earned by defeating the sector boss."},
		{"4. BOSS FIGHTS", "Bosses introduce their own mechanics. Follow telegraphs instead of treating every boss object as a normal enemy.", "VOID SERPENT: normal body sections are lethal. When a section FLASHES PURPLE its collision is disabled, so you may move onto that section to hit the active node safely."},
	}
	if page < 0 {
		page = 0
	}
	if page > 3 {
		page = 3
	}
	switch mode {
	case 1:
		return endurance[page][0], endurance[page][1], endurance[page][2]
	case 2:
		return survival[page][0], survival[page][1], survival[page][2]
	default:
		return precision[page][0], precision[page][1], precision[page][2]
	}
}

func drawTutorialOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayTutorial {
		return
	}
	drawModalBackdrop(hdc, w, hgt)
	r := tutorialPanelRect(w, hgt)
	cyan := rgb(26, 205, 239)
	white := rgb(245, 248, 252)
	muted := rgb(145, 181, 208)
	gold := rgb(255, 205, 64)

	if tutorialMode < 0 {
		drawPanelFrame(hdc, r, "PLAY TUTORIAL", "CHOOSE A GAME MODE")
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, white)
			centeredTextOut(hdc, r.Left+sx(45, w), r.Right-sx(45, w), r.Top+sy(126, hgt), "SELECT A MODE TO VIEW ITS TUTORIAL")
			selectObject.Call(hdc, old)
		}
		cards := tutorialModeCardRects(w, hgt)
		assets := [][]byte{modePrecisionCardBGRA, modeEnduranceCardBGRA, modeSurvivalCardBGRA}
		widths := []int32{1368, 1368, 1371}
		heights := []int32{267, 259, 260}
		fallback := []string{"PRECISION MODE", "ENDURANCE MODE", "SURVIVAL MODE"}
		for i, cr := range cards {
			overlaySolidAlphaRect(hdc, RECT{cr.Left - 3, cr.Top - 3, cr.Right + 3, cr.Bottom + 3}, rgb(1, 8, 24), 235)
			if len(assets[i]) >= int(widths[i]*heights[i]*4) {
				drawRawBGRATrimmedFit(hdc, assets[i], widths[i], heights[i], cr)
			} else {
				face, _, dark := themedHUDCardPalette(0)
				drawBevelPanel(hdc, cr, face, cyan, dark, 3)
				drawArcadeLabel(hdc, cr, fallback[i], white)
			}
			if pointInRect(cursorPos, cr) {
				drawOutlineRect(hdc, cr, gold, 3)
			}
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, muted)
			centeredTextOut(hdc, r.Left, r.Right, r.Bottom-sy(34, hgt), "ESC = BACK")
			selectObject.Call(hdc, old)
		}
		return
	}

	count := tutorialPageCount(tutorialMode)
	if tutorialPage < 0 {
		tutorialPage = 0
	}
	if tutorialPage >= count {
		tutorialPage = count - 1
	}
	modeName := []string{"PRECISION", "ENDURANCE", "SURVIVAL"}[tutorialMode]
	drawPanelFrame(hdc, r, modeName+" TUTORIAL", fmt.Sprintf("PAGE %d / %d", tutorialPage+1, count))
	title, line1, line2 := tutorialCopy(tutorialMode, tutorialPage)

	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, cyan)
		centeredTextOut(hdc, r.Left+sx(50, w), r.Right-sx(50, w), r.Top+sy(144, hgt), title)
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, white)
		textLeft := r.Left + sx(72, w)
		textRight := r.Right - sx(72, w)
		firstR := RECT{textLeft, r.Top + sy(198, hgt), textRight, r.Top + sy(310, hgt)}
		secondR := RECT{textLeft, r.Top + sy(330, hgt), textRight, r.Top + sy(452, hgt)}
		drawWrappedCenteredText(hdc, hudSmallFont, firstR, line1, sy(6, hgt))
		drawWrappedCenteredText(hdc, hudSmallFont, secondR, line2, sy(6, hgt))
		selectObject.Call(hdc, old)
	}

	segLeft := r.Left + sx(180, w)
	segRight := r.Right - sx(180, w)
	gap := sx(12, w)
	segW := (segRight - segLeft - gap*int32(count-1)) / int32(count)
	for i := 0; i < count; i++ {
		c := rgb(33, 56, 75)
		if i <= tutorialPage {
			c = cyan
		}
		x := segLeft + int32(i)*(segW+gap)
		fillSolidRect(hdc, RECT{x, r.Top + sy(482, hgt), x + segW, r.Top + sy(491, hgt)}, c)
	}

	back := tutorialBackRect(w, hgt)
	next := tutorialNextRect(w, hgt)
	uiButton(hdc, back, "BACK", "", false)
	if tutorialPage < count-1 {
		uiButton(hdc, next, "NEXT", fmt.Sprintf("%d / %d", tutorialPage+2, count), true)
	} else {
		uiButton(hdc, next, "DONE", "MODE SELECT", true)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, r.Left+sx(270, w), r.Right-sx(270, w), r.Bottom-sy(47, hgt), "ESC = MODE SELECT")
		selectObject.Call(hdc, old)
	}
}

func drawReleaseNotesOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayReleaseNotes {
		return
	}
	drawModalBackdrop(hdc, w, hgt)
	r := centeredPanel(w, hgt, 940, 690)
	drawPanelFrame(hdc, r, "CURSOR CONTROL TRAINER 1.0", "v338 PROFILE NAV & REFRESH")

	notes := []string{
		"• New commercial front-end with Profile, Settings, Achievements and Tutorial",
		"• Lifetime statistics, playtime tracking and persistent player records",
		"• 95 achievements with six prestige levels across Precision, Endurance and Survival",
		"• ESC now opens menus/goes back; quitting requires QUIT or the window X",
		"• Daily Ops (+50 EXP) and Weekly Ops (+200 EXP)",
		"• Crosshair rank badges integrated into player profiles",
		"• Graphics controls: particles, frame rate target and reduced motion",
		"• Audio controls for music and effects",
		"• Five crosshair styles with three sizes",
		"• Automatic local personal-best saving and difficulty filters",
		"• Global Top 20 player names open public player profiles",
		"• Improved rendering path and lower-cost particle system",
		"• Offline-first: training, EXP, records, stats and achievements work without login",
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		y := r.Top + sy(105, hgt)
		for _, line := range notes {
			setTextColor.Call(hdc, rgb(222, 236, 247))
			textOut(hdc, r.Left+sx(70, w), y, line)
			y += sy(39, hgt)
		}
		selectObject.Call(hdc, old)
	}
	drawBackHint(hdc, r, w, hgt)
}
