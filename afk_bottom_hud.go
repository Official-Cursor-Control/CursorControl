//go:build windows

package main

import (
	"fmt"
	"time"
)

var afkEarningsPanelOpen bool
var afkSystemsMenuOpen bool
var afkSystemLockedPopupText string
var afkSystemLockedPopupUntil time.Time

var afkProductionPopupOpen bool

func afkAnyManagementPanelOpen() bool {
	return afkFacilitiesPanelOpen || afkGuidePanelOpen || afkExpeditionPanelOpen || afkTechPanelOpen || afkTalentPanelOpen || afkOperatorPanelOpen || afkModulePanelOpen || afkDronePanelOpen || afkOrbitalPanelOpen || afkPrestigePanelOpen || afkEarningsPanelOpen || afkSystemsMenuOpen || afkExchangePanelOpen
}

func afkCloseManagementPanels() {
	afkFacilitiesPanelOpen = false
	afkGuidePanelOpen = false
	afkExpeditionPanelOpen = false
	afkTechPanelOpen = false
	afkTalentPanelOpen = false
	afkOperatorPanelOpen = false
	afkModulePanelOpen = false
	afkDronePanelOpen = false
	afkOrbitalPanelOpen = false
	afkPrestigePanelOpen = false
	afkEarningsPanelOpen = false
	afkSystemsMenuOpen = false
	afkExchangePanelOpen = false
}

func afkSecondaryManagementPanelOpen() bool {
	return afkExpeditionPanelOpen || afkTechPanelOpen || afkTalentPanelOpen || afkOperatorPanelOpen || afkModulePanelOpen || afkDronePanelOpen || afkOrbitalPanelOpen || afkPrestigePanelOpen || afkEarningsPanelOpen || afkExchangePanelOpen
}

// Starbase keeps its permanent HUD intentionally small. The world owns the
// screen; only navigation and the currently relevant action live at the bottom.
func afkBottomDockRect(w, hgt int32) RECT {
	f := afkMainFieldRect(w, hgt)
	return RECT{sx(20, w), f.Bottom + sy(8, hgt), w - sx(20, w), hgt - sy(12, hgt)}
}

// The objective now belongs to the upper command deck. This keeps progression
// visible without consuming any of the Singularity arena or bottom action dock.
func afkObjectiveStripRect(w, hgt int32) RECT {
	// Dedicated command-deck row below telemetry. It intentionally spans most of
	// the window so objective text can never collide with the Starbase title or
	// the left/right telemetry groups.
	return RECT{sx(300, w), sy(118, hgt), w - sx(300, w), sy(146, hgt)}
}

// Telemetry lives in the top command deck, grouped around the centred Starbase
// identity. The world itself therefore remains almost completely unobstructed.
func afkLiveStatCardRect(w, hgt int32, index int) RECT {
	top := sy(58, hgt)
	bottom := sy(112, hgt)
	switch index {
	case 0: // Starbits
		return RECT{sx(130, w), top, sx(330, w), bottom}
	case 1: // production
		return RECT{sx(340, w), top, sx(540, w), bottom}
	case 2: // nav data
		return RECT{w - sx(530, w), top, w - sx(370, w), bottom}
	case 3: // AFK capacity
		return RECT{w - sx(360, w), top, w - sx(195, w), bottom}
	default: // Overdrive
		return RECT{w - sx(185, w), top, w - sx(20, w), bottom}
	}
}
func afkOverdriveCardRect(w, hgt int32) RECT { return afkLiveStatCardRect(w, hgt, 4) }

// Primary Starbase navigation now occupies the top command rail instead of the
// bottom HUD. Five comfortably sized controls stay visible without competing
// with contextual upgrade actions.
func afkPrimaryNavRect(w, hgt int32, index int) RECT {
	left := sx(270, w)
	right := w - sx(270, w)
	gap := sx(10, w)
	bw := (right - left - gap*4) / 5
	x := left + int32(index)*(bw+gap)
	return RECT{x, sy(151, hgt), x + bw, sy(188, hgt)}
}

// Starbase world navigation is intentionally sparse. Once a physical facility
// exists on the moon, the facility itself is its navigation button. The top
// rail only exposes systems that do not yet have a clickable world object.
func afkUtilityNavRect(w, hgt int32, index, count int) RECT {
	if count < 1 {
		count = 1
	}
	gap := sx(12, w)
	bw := sx(190, w)
	total := int32(count)*bw + int32(count-1)*gap
	left := (w - total) / 2
	x := left + int32(index)*(bw+gap)
	return RECT{x, sy(151, hgt), x + bw, sy(188, hgt)}
}

type afkUtilityNavItem struct {
	label  string
	action int
}

const (
	afkUtilityFacilities = iota
	afkUtilityGuide
	afkUtilitySystems
)

func afkUtilityNavItems() []afkUtilityNavItem {
	return []afkUtilityNavItem{{"FACILITIES", afkUtilityFacilities}, {"GUIDE", afkUtilityGuide}, {"SYSTEMS", afkUtilitySystems}}
}

// Legacy helper retained for panel code/tests; system slots are surfaced
// through the large SYSTEMS modal rather than a permanent facility strip.
func afkFacilityButtonRect(w, hgt int32, index int) RECT {
	if index < 5 {
		return afkPrimaryNavRect(w, hgt, index)
	}
	return RECT{}
}

// The lower dock is now action-only. Context controls consume all available
// space, so labels and prices can stay readable even at smaller resolutions.
func afkContextButtonRect(w, hgt int32, index int, count int) RECT {
	d := afkBottomDockRect(w, hgt)
	gap := sx(12, w)
	left, right := d.Left, d.Right
	bw := (right - left - gap*int32(count-1)) / int32(count)
	x := left + int32(index)*(bw+gap)
	return RECT{x, d.Top, x + bw, d.Bottom}
}

// Module item detail needs a much wider material requirement field than the two
// navigation/action controls. v442 deliberately gives MODULE SETS and CRAFT
// only the width they need, leaving the recipe readable without truncation.
func afkModuleContextRects(w, hgt int32) (back, craft, requirements RECT) {
	d := afkBottomDockRect(w, hgt)
	gap := sx(12, w)
	usable := d.Right - d.Left - gap*2
	backW := usable * 18 / 100
	craftW := usable * 25 / 100
	back = RECT{d.Left, d.Top, d.Left + backW, d.Bottom}
	craft = RECT{back.Right + gap, d.Top, back.Right + gap + craftW, d.Bottom}
	requirements = RECT{craft.Right + gap, d.Top, d.Right, d.Bottom}
	return
}
func afkTechToggleRect(w, hgt int32) RECT   { return afkPrimaryNavRect(w, hgt, 2) }
func afkTalentToggleRect(w, hgt int32) RECT { return afkSystemCardRect(w, hgt, 0) }

func afkSystemMenuRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }
func afkSystemCardRect(w, hgt int32, index int) RECT {
	p := afkSystemMenuRect(w, hgt)
	gapX, gapY := sx(16, w), sy(16, hgt)
	marginX, marginTop := sx(34, w), sy(86, hgt)
	cw := (p.Right - p.Left - marginX*2 - gapX*2) / 3
	ch := sy(125, hgt)
	row, col := index/3, index%3
	x := p.Left + marginX + int32(col)*(cw+gapX)
	y := p.Top + marginTop + int32(row)*(ch+gapY)
	return RECT{x, y, x + cw, y + ch}
}

func drawAFKPrimaryNavButton(hdc uintptr, r RECT, label string, enabled, selected, hovered bool, w, hgt int32) {
	fill := rgb(5, 24, 49)
	outline := rgb(50, 106, 150)
	textc := rgb(174, 204, 224)
	if enabled {
		outline = rgb(69, 207, 255)
		textc = rgb(240, 249, 255)
	}
	if hovered && enabled {
		fill = rgb(12, 43, 72)
	}
	if selected {
		fill = rgb(31, 28, 76)
		outline = rgb(255, 201, 60)
		textc = rgb(255, 238, 174)
	}
	drawBevelPanel(hdc, r, fill, outline, rgb(0, 6, 16), 3)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, textc)
		display := fitTextEllipsis(hdc, hudSmallFont, label, r.Right-r.Left-sx(16, w))
		centeredTextOut(hdc, r.Left+sx(8, w), r.Right-sx(8, w), verticallyCenteredTextY(hdc, hudSmallFont, r, display), display)
		selectObject.Call(hdc, old)
	}
}

func afkSystemUnlockHint(index int) string {
	switch index {
	case 0:
		return "PILOT SKILLS // BUILD THE TECH LAB (CORE TIER 3 + EXPEDITION PROGRESS)"
	case 1:
		return "MODULES // REACH CORE TIER 4 AND PROGRESS OPERATIONS"
	case 2:
		return "DRONE BAY // REACH CORE TIER 5 AND COMPLETE MODULE PROGRESSION"
	case 3:
		return "ORBITAL // BUILD DRONE BAY AND ASCEND IT TO TIER 3"
	case 4:
		return "PRESTIGE // COMPLETE THE LATE-GAME STARBASE REQUIREMENTS"
	}
	return "LOCKED // CHECK STARBASE GUIDE OR FACILITY REQUIREMENTS"
}

func showAFKSystemLockedPopup(index int) {
	afkSystemLockedPopupText = afkSystemUnlockHint(index)
	afkSystemLockedPopupUntil = time.Now().Add(2400 * time.Millisecond)
}

func drawAFKSystemLockedPopup(hdc uintptr, w, hgt int32) {
	if afkSystemLockedPopupText == "" || time.Now().After(afkSystemLockedPopupUntil) {
		return
	}
	p := afkSystemMenuRect(w, hgt)
	ww := sx(720, w)
	if maxW := p.Right - p.Left - sx(40, w); ww > maxW {
		ww = maxW
	}
	r := RECT{(p.Left + p.Right - ww) / 2, p.Bottom - sy(66, hgt), (p.Left + p.Right + ww) / 2, p.Bottom - sy(22, hgt)}
	drawBevelPanel(hdc, r, rgb(22, 13, 39), rgb(255, 194, 64), rgb(5, 3, 12), 3)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 234, 177))
		t := fitTextEllipsis(hdc, hudTinyFont, afkSystemLockedPopupText, r.Right-r.Left-sx(24, w))
		centeredTextOut(hdc, r.Left+sx(12, w), r.Right-sx(12, w), verticallyCenteredTextY(hdc, hudTinyFont, r, t), t)
		selectObject.Call(hdc, old)
	}
}

func drawAFKSystemsMenu(hdc uintptr, w, hgt int32) {
	if !afkSystemsMenuOpen {
		return
	}
	drawAFKManagementBackdrop(hdc, w, hgt)
	p := afkSystemMenuRect(w, hgt)
	drawAFKManagementChrome(hdc, p, "STARBASE SYSTEMS", "NON-FACILITY PROGRESSION // LOCKED CARDS EXPLAIN THEIR REQUIREMENT", rgb(176, 89, 255), w, hgt)
	labels := []string{"PILOT SKILLS", "MODULES", "DRONE BAY", "ORBITAL", "PRESTIGE", "EXCHANGE"}
	for i, label := range labels {
		r := afkSystemCardRect(w, hgt, i)
		enabled := true
		switch i {
		case 0:
			enabled = afkTechLabUnlocked()
		case 1:
			enabled = afkModulesAvailable()
		case 2:
			enabled = afkDronesAvailable()
		case 3:
			enabled = afkOrbitalAvailable()
		case 4:
			enabled = afkPrestigeAvailable()
		case 5:
			enabled = discordConnected
		}
		fill := rgb(7, 29, 57)
		outline := rgb(52, 96, 133)
		txt := rgb(110, 130, 145)
		if enabled {
			outline = rgb(83, 200, 255)
			txt = rgb(239, 248, 255)
		}
		if enabled && pointInRect(cursorPos, r) {
			fill = rgb(24, 39, 79)
			outline = rgb(255, 199, 62)
		}
		drawBevelPanel(hdc, r, fill, outline, rgb(0, 5, 15), 3)
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, txt)
			centeredTextOut(hdc, r.Left+8, r.Right-8, r.Top+sy(24, hgt), label)
			selectObject.Call(hdc, old)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			if enabled {
				setTextColor.Call(hdc, rgb(132, 190, 218))
			} else {
				setTextColor.Call(hdc, rgb(75, 88, 102))
			}
			status := "OPEN SYSTEM"
			if !enabled {
				status = afkSystemUnlockHint(i)
			}
			status = fitTextEllipsis(hdc, hudTinyFont, status, r.Right-r.Left-sx(16, w))
			centeredTextOut(hdc, r.Left+8, r.Right-8, r.Bottom-sy(34, hgt), status)
			selectObject.Call(hdc, old)
		}
	}
	drawAFKSystemLockedPopup(hdc, w, hgt)
}

func drawAFKFacilityTab(hdc uintptr, r RECT, label string, index int, enabled, selected, hovered bool, w, hgt int32) {
	drawAFKPrimaryNavButton(hdc, r, label, enabled, selected, hovered, w, hgt)
}

func afkProductionPopupRect(w, hgt int32) RECT {
	c := afkLiveStatCardRect(w, hgt, 1)
	pw := sx(390, w)
	ph := sy(285, hgt)
	return RECT{c.Left, c.Bottom + sy(8, hgt), c.Left + pw, c.Bottom + sy(8, hgt) + ph}
}
func afkProductionMultiplierRows() []string {
	rows := []string{}
	addMultiplier := func(name string, m int64) {
		if m != 1000 && m > 0 {
			rows = append(rows, fmt.Sprintf("%s  //  x%.2f", name, float64(m)/1000.0))
		}
	}
	addBonus := func(name string, m int64) {
		if m != 1000 {
			rows = append(rows, fmt.Sprintf("%s  //  +%.1f%%", name, float64(m-1000)/10.0))
		}
	}
	addMultiplier("GLOBAL RANK", afkGlobalRankMultiplierMilli())
	addMultiplier("CURSOR CORE", afkCoreTierMultiplierMilli())
	addBonus("RESEARCH", afkResearchStarbitMultiplierMilli())
	addBonus("TALENTS", afkTalentStarbitMultiplierMilli())
	addBonus("OPERATORS", afkOperatorStarbitProductionMultiplierMilli())
	addBonus("MODULE PRODUCTION", afkModuleProductionMultiplierMilli())
	addBonus("STELLAR PRESTIGE", afkPrestigeMultiplierMilli())
	if support := afkPermanentStarbitSupportMultiplierMilli(); support != 1000 {
		rows = append(rows, fmt.Sprintf("PERMANENT SUPPORT TOTAL  //  x%.2f", float64(support)/1000.0))
	}
	addMultiplier("AUTO-CURSOR MODULE", afkModuleAutoCursorMultiplierMilli())
	if afkOverdriveActive(time.Now()) {
		rows = append(rows, "OVERDRIVE  //  x2.00")
	}
	if len(rows) == 0 {
		rows = append(rows, "NO BONUS MULTIPLIERS ACTIVE  //  x1.00")
	}
	return rows
}
func drawAFKProductionPopup(hdc uintptr, w, hgt int32) {
	if !afkProductionPopupOpen {
		return
	}
	r := afkProductionPopupRect(w, hgt)
	face, light, dark := themedHUDPanelPalette(false)
	drawBevelPanel(hdc, r, blendColor(face, dark, .08), light, dark, 3)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(245, 250, 255))
		textOut(hdc, r.Left+sx(16, w), r.Top+sy(14, hgt), "CURRENT STARBIT MULTIPLIERS")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		y := r.Top + sy(48, hgt)
		for _, row := range afkProductionMultiplierRows() {
			setTextColor.Call(hdc, rgb(190, 224, 240))
			textOut(hdc, r.Left+sx(18, w), y, row)
			y += sy(23, hgt)
		}
		setTextColor.Call(hdc, rgb(255, 211, 74))
		textOut(hdc, r.Left+sx(18, w), r.Bottom-sy(34, hgt), "FINAL RATE  //  "+afkStarbitsPerSecondText()+" STARBITS/SEC")
		selectObject.Call(hdc, old)
	}
}

func drawAFKStarbitBankPulse(hdc uintptr, r RECT, w, hgt int32) {
	strength := afkBankPulseStrength(time.Now())
	if strength <= 0 {
		return
	}
	// The Bank is deliberately much louder than ordinary hover feedback: a bank
	// update should be instantly noticeable in peripheral vision.
	alpha := byte(28 + int(120*strength))
	overlaySolidAlphaRect(hdc, RECT{r.Left + 2, r.Top + 2, r.Right - 2, r.Bottom - 2}, rgb(255, 211, 36), alpha)
	drawOutlineRect(hdc, r, rgb(255, 226, 72), 3)
	pad := sx(2+5*(1-strength), w)
	drawOutlineRect(hdc, RECT{r.Left - pad, r.Top - pad, r.Right + pad, r.Bottom + pad}, rgb(255, 196, 24), 2)
}

func drawAFKBottomHUD(hdc uintptr, w, hgt int32) {
	d := afkBottomDockRect(w, hgt)
	fillSolidRect(hdc, d, rgb(1, 11, 29))
	drawLineSimple(hdc, d.Left, d.Top-sy(3, hgt), d.Right, d.Top-sy(3, hgt), 2, rgb(75, 210, 255))

	// Floating objective strip at the top of the world.
	title, detail1, detail2, cur, target, done := afkObjective()
	title = fmt.Sprintf("STARBASE LV.%d // %s", afkStarbaseLevel(), title)
	obj := afkObjectiveStripRect(w, hgt)
	objBorder := rgb(71, 142, 181)
	if done {
		objBorder = rgb(74, 220, 137)
	}
	drawBevelPanel(hdc, obj, rgb(4, 23, 48), objBorder, rgb(0, 5, 14), 2)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		inner := RECT{obj.Left + sx(10, w), obj.Top + sy(4, hgt), obj.Right - sx(10, w), obj.Bottom - sy(4, hgt)}
		progressText := ""
		progressW := int32(0)
		if target > 0 {
			progressText = fmt.Sprintf("%s / %s", formatAFKNumber(cur), formatAFKNumber(target))
			progressW = textPixelSize(hdc, hudTinyFont, progressText).Cx + sx(18, w)
		}
		titleW := (inner.Right - inner.Left) * 29 / 100
		titleR := RECT{inner.Left, inner.Top, inner.Left + titleW, inner.Bottom}
		detailR := RECT{titleR.Right + sx(8, w), inner.Top, inner.Right - progressW - sx(8, w), inner.Bottom}
		progressR := RECT{inner.Right - progressW, inner.Top, inner.Right, inner.Bottom}
		setTextColor.Call(hdc, rgb(245, 250, 255))
		t := fitTextEllipsis(hdc, hudTinyFont, title, titleR.Right-titleR.Left)
		textOut(hdc, titleR.Left, verticallyCenteredTextY(hdc, hudTinyFont, titleR, t), t)
		setTextColor.Call(hdc, rgb(125, 181, 208))
		detail := fitTextEllipsis(hdc, hudTinyFont, detail1+"  //  "+detail2, detailR.Right-detailR.Left)
		centeredTextOut(hdc, detailR.Left, detailR.Right, verticallyCenteredTextY(hdc, hudTinyFont, detailR, detail), detail)
		if progressText != "" {
			setTextColor.Call(hdc, rgb(255, 215, 70))
			p := fitTextEllipsis(hdc, hudTinyFont, progressText, progressR.Right-progressR.Left)
			centeredTextOut(hdc, progressR.Left, progressR.Right, verticallyCenteredTextY(hdc, hudTinyFont, progressR, p), p)
		}
		selectObject.Call(hdc, old)
	}

	// Sparse floating telemetry. These replace the previous five-card bottom row.
	bankLabel, bankValue := afkBankHUDText(time.Now())
	cards := []struct {
		label, value string
		accent       uintptr
	}{
		{bankLabel, bankValue, rgb(255, 199, 55)},
		{"STARBITS / SEC", afkStarbitsPerSecondText(), rgb(78, 217, 255)},
		{"NAV DATA", fmt.Sprintf("%d", gameMeta.AFKNavigationData), rgb(170, 105, 255)},
		{fmt.Sprintf("STARBASE LEVEL %d", afkStarbaseLevel()), afkStarbaseLevelName(afkStarbaseLevel()), rgb(75, 222, 145)},
		{"OVERDRIVE", afkOverdriveStatus(time.Now()), rgb(234, 91, 255)},
	}
	for i, c := range cards {
		r := afkLiveStatCardRect(w, hgt, i)
		drawAFKStatCard(hdc, r, c.label, c.value, c.accent)
		if i == 0 {
			drawAFKStarbitBankPulse(hdc, r, w, hgt)
		}
	}
	drawAFKProductionPopup(hdc, w, hgt)

	// Three permanent Starbase controls. Physical facilities remain the fastest
	// management route; FACILITIES is the organised build overview, GUIDE
	// explains every stat, and SYSTEMS holds non-building progression.
	items := afkUtilityNavItems()
	for i, item := range items {
		r := afkHubButtonRect(w, hgt, i)
		selected := (item.action == afkUtilityFacilities && afkFacilitiesPanelOpen) || (item.action == afkUtilityGuide && afkGuidePanelOpen) || (item.action == afkUtilitySystems && afkSystemsMenuOpen)
		name := []string{"facilities", "guide", "systems"}[i]
		drawAFKStarbaseControl(hdc, name, r, selected, pointInRect(cursorPos, r))
	}

	// Facilities owns the bottom action row while its browser is open. This
	// prevents the legacy Core action from appearing under a different selected
	// facility and makes the action explicitly belong to the current selection.
	if afkFacilitiesPanelOpen {
		r := afkContextButtonRect(w, hgt, 0, 1)
		idx := afkFacilityDrawerIndex
		if idx < 0 {
			drawAFKActionButton(hdc, r, "SELECT A FACILITY // VIEW TIER + REQUIREMENTS + NEXT UPGRADE", 0, false, pointInRect(cursorPos, r))
		} else {
			label, cost, enabled := afkFacilityContextAction(idx)
			drawAFKActionButton(hdc, r, label, cost, enabled, pointInRect(cursorPos, r))
		}
		return
	}

	// Bottom context controls. Core controls remain available whenever no
	// sub-panel is open; this is the principal buy/upgrade area requested.
	if afkEarningsPanelOpen {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if afkHasOfflineReward() {
			label := fmt.Sprintf("CLAIM AFK EARNINGS // %s STARBITS // %s CREDITED", formatAFKNumber(gameMeta.AFKOfflinePendingStarbits), afkFormatDuration(gameMeta.AFKOfflinePendingPaidSeconds))
			drawAFKActionButton(hdc, r, label, 0, true, pointInRect(cursorPos, r))
		} else {
			drawAFKActionButton(hdc, r, "AFK EARNINGS // NOTHING TO CLAIM YET", 0, false, pointInRect(cursorPos, r))
		}
		return
	}
	if afkPrestigePanelOpen {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if afkPrestigeRank() >= afkPrestigeMaxRank {
			drawAFKActionButton(hdc, r, "STELLAR PRESTIGE // RANK 10 MAX", 0, false, pointInRect(cursorPos, r))
		} else {
			drawAFKActionButton(hdc, r, fmt.Sprintf("STELLAR PRESTIGE // RANK %d // +5%% ALL PRODUCTION", afkPrestigeRank()+1), afkPrestigeNextCost(), afkCanPrestige(), pointInRect(cursorPos, r))
		}
		return
	}
	if afkTechPanelOpen && !afkTechLabUnlocked() {
		r := afkContextButtonRect(w, hgt, 0, 1)
		drawAFKActionButton(hdc, r, "BUILD TECH LAB", afkTechLabUnlockCost, gameMeta.AFKStarbits >= afkTechLabUnlockCost, pointInRect(cursorPos, r))
		return
	}
	if afkTechPanelOpen && afkTechLabUnlocked() {
		i := afkResearchSelected
		if i < 0 || i >= afkVisibleResearchCount() {
			i = 0
		}
		d := afkResearchDefs[i]
		cost := afkResearchCost(i)
		lvl := afkResearchLevel(i)
		r := afkContextButtonRect(w, hgt, 0, 1)
		label := fmt.Sprintf("UPGRADE %s // LV.%d", d.Name, lvl)
		if cost <= 0 {
			label = fmt.Sprintf("%s // MAX LEVEL", d.Name)
		}
		drawAFKActionButton(hdc, r, label, cost, cost > 0 && gameMeta.AFKStarbits >= cost, pointInRect(cursorPos, r))
		return
	}
	if afkTalentPanelOpen && afkTalentInfoOpen {
		i := afkTalentSelected
		if i < 0 || i >= len(afkTalentDefs) {
			i = 0
		}
		r := afkContextButtonRect(w, hgt, 0, 1)
		owned := afkTalentUnlocked(i)
		enabled := !owned && afkTalentAvailable(i) && afkTalentPointsAvailable() > 0
		label := fmt.Sprintf("BUY %s // 1 PILOT POINT", afkTalentDefs[i].Name)
		if owned {
			label = afkTalentDefs[i].Name + " // ACTIVE"
		}
		drawAFKActionButton(hdc, r, label, 0, enabled, pointInRect(cursorPos, r))
		return
	}
	if afkDronePanelOpen {
		if !afkDroneBayBuilt() {
			r := afkContextButtonRect(w, hgt, 0, 1)
			drawAFKActionButton(hdc, r, "BUILD DRONE BAY // TIER 1", afkDroneBayBuildCost, gameMeta.AFKStarbits >= afkDroneBayBuildCost, pointInRect(cursorPos, r))
			return
		}
		upgrade := afkContextButtonRect(w, hgt, 0, 2)
		ascend := afkContextButtonRect(w, hgt, 1, 2)
		i := afkDroneSelectedUpgrade
		if i < 0 || i >= afkDroneUpgradeCount {
			i = 0
		}
		cost := afkDroneUpgradeCost(i)
		label := fmt.Sprintf("UPGRADE %s // LV.%d", afkDroneUpgradeDefs[i].Name, afkDroneUpgradeLevel(i))
		if i == 0 {
			label = fmt.Sprintf("DEPLOY DRONE // %d/%d", gameMeta.AFKDronesDeployed, afkDroneMaxCount)
		}
		drawAFKActionButton(hdc, upgrade, label, cost, cost > 0 && gameMeta.AFKStarbits >= cost, pointInRect(cursorPos, upgrade))
		ac := afkDroneBayAscendCost()
		alabel := fmt.Sprintf("DRONE BAY TIER %d // ASCEND LOCKED", afkDroneBayTier())
		if afkDroneBayTier() >= afkDroneBayMaxTier {
			alabel = "DRONE BAY // TIER 3 MAX"
		} else if afkDroneBayCanAscend() {
			alabel = fmt.Sprintf("ASCEND DRONE BAY // TIER %d", afkDroneBayTier()+1)
		}
		drawAFKActionButton(hdc, ascend, alabel, ac, ac > 0 && afkDroneBayCanAscend() && gameMeta.AFKStarbits >= ac, pointInRect(cursorPos, ascend))
		return
	}
	if afkOrbitalPanelOpen {
		if !afkOrbitalBuilt() {
			r := afkContextButtonRect(w, hgt, 0, 1)
			drawAFKActionButton(hdc, r, "BUILD ORBITAL EXTRACTOR // TIER 1", afkOrbitalBuildCost, afkOrbitalAvailable() && gameMeta.AFKStarbits >= afkOrbitalBuildCost, pointInRect(cursorPos, r))
			return
		}
		upgrade := afkContextButtonRect(w, hgt, 0, 2)
		ascend := afkContextButtonRect(w, hgt, 1, 2)
		i := afkOrbitalSelectedUpgrade
		if i < 0 || i >= afkOrbitalUpgradeCount {
			i = 0
		}
		cost := afkOrbitalUpgradeCost(i)
		label := fmt.Sprintf("UPGRADE %s // LV.%d", afkOrbitalUpgradeDefs[i].Name, afkOrbitalUpgradeLevel(i))
		drawAFKActionButton(hdc, upgrade, label, cost, cost > 0 && gameMeta.AFKStarbits >= cost, pointInRect(cursorPos, upgrade))
		ac := afkOrbitalAscendCost()
		alabel := fmt.Sprintf("ORBITAL TIER %d // ASCEND LOCKED", afkOrbitalTier())
		if afkOrbitalTier() >= afkOrbitalMaxTier {
			alabel = "ORBITAL EXTRACTOR // TIER 3 MAX"
		} else if afkOrbitalCanAscend() {
			alabel = fmt.Sprintf("ASCEND ORBITAL EXTRACTOR // TIER %d", afkOrbitalTier()+1)
		}
		drawAFKActionButton(hdc, ascend, alabel, ac, ac > 0 && afkOrbitalCanAscend() && gameMeta.AFKStarbits >= ac, pointInRect(cursorPos, ascend))
		return
	}
	if afkModulePanelOpen {
		if gameMeta.AFKPendingComponentCount > 0 {
			r := afkContextButtonRect(w, hgt, 0, 1)
			d := afkCraftComponentDefs[gameMeta.AFKPendingComponent]
			drawAFKActionButton(hdc, r, fmt.Sprintf("CLAIM COMPONENT // %s x%d // %s", d.Name, gameMeta.AFKPendingComponentCount, d.Rarity), 0, true, pointInRect(cursorPos, r))
			return
		}
		if afkModuleSelectedItem >= 0 {
			back, craft, info := afkModuleContextRects(w, hgt)
			drawAFKActionButton(hdc, back, "< MODULE SETS", 0, true, pointInRect(cursorPos, back))
			s, i := afkModuleSelectedSet, afkModuleSelectedItem
			name := afkModuleDisplayName(s, i)
			label := "CRAFT " + name
			if afkModuleCrafted(s, i) {
				label = name + " // CRAFTED"
			}
			drawAFKActionButton(hdc, craft, label, 0, afkCanCraftModule(s, i), pointInRect(cursorPos, craft))
			drawAFKActionButton(hdc, info, afkModuleRecipeText(s, i), 0, false, pointInRect(cursorPos, info))
		} else {
			r := afkContextButtonRect(w, hgt, 0, 1)
			if afkAllModuleSetsRewardAvailable() {
				drawAFKActionButton(hdc, r, "COMPLETE ALL SETS 9/9 // CLAIM x500 NAV DATA", 0, true, pointInRect(cursorPos, r))
			} else {
				drawAFKActionButton(hdc, r, fmt.Sprintf("MODULE COLLECTION // %d / 9 SETS COMPLETE // ALL SETS REWARD x500 NAV DATA", afkModuleSetsComplete()), 0, false, pointInRect(cursorPos, r))
			}
		}
		return
	}
	if afkOperatorPanelOpen {
		i := afkOperatorSelected
		if i < 0 || i >= len(afkOperatorDefs) {
			i = 0
		}
		r := afkContextButtonRect(w, hgt, 0, 1)
		if afkOperatorRecruited(i) {
			lvl := afkOperatorLevel(i)
			now := time.Now()
			if lvl >= afkOperatorMaxLevel {
				drawAFKActionButton(hdc, r, fmt.Sprintf("%s // LV.%d MAX // %s", afkOperatorDefs[i].Name, lvl, afkOperatorBonusText(i)), 0, false, pointInRect(cursorPos, r))
			} else if afkOperatorCanLevel(i) {
				drawAFKActionButton(hdc, r, fmt.Sprintf("LEVEL UP %s // LV.%d -> LV.%d // %s", afkOperatorDefs[i].Name, lvl, lvl+1, afkOperatorNextBonusText(i)), 0, true, pointInRect(cursorPos, r))
			} else if afkOperatorAssignmentActive(i, now) || afkOperatorCooldownActive(i, now) {
				drawAFKActionButton(hdc, r, afkOperatorAssignmentStatus(i, now), 0, false, pointInRect(cursorPos, r))
			} else {
				cur, targetXP := afkOperatorXPToNext(i)
				label := fmt.Sprintf("START %s // COST %s // SERVICE XP %s / %s", afkOperatorDefs[i].AssignmentName, formatAFKNumber(afkOperatorAssignmentCost(i)), formatAFKNumber(cur), formatAFKNumber(targetXP))
				drawAFKActionButton(hdc, r, label, afkOperatorAssignmentCost(i), afkOperatorCanStartAssignment(i, now), pointInRect(cursorPos, r))
			}
		} else {
			ready := afkOperatorRecruitRequirementMet(i)
			if ready {
				ready = gameMeta.AFKStarbits >= afkOperatorDefs[i].RecruitCost
				drawAFKActionButton(hdc, r, fmt.Sprintf("RECRUIT %s // %s", afkOperatorDefs[i].Name, afkOperatorDefs[i].Role), afkOperatorDefs[i].RecruitCost, ready, pointInRect(cursorPos, r))
			} else {
				drawAFKActionButton(hdc, r, fmt.Sprintf("%s LOCKED // %s", afkOperatorDefs[i].Name, afkOperatorRecruitRequirementText(i)), 0, false, pointInRect(cursorPos, r))
			}
		}
		return
	}
	if afkExpeditionPanelOpen {
		r := afkContextButtonRect(w, hgt, 0, 1)
		face := rgb(6, 35, 64)
		border := rgb(73, 133, 173)
		if done {
			border = rgb(70, 216, 125)
		}
		drawBevelPanel(hdc, r, face, border, rgb(0, 7, 17), 2)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(242, 248, 252))
			textOut(hdc, r.Left+sx(12, w), r.Top+sy(8, hgt), title)
			setTextColor.Call(hdc, rgb(130, 173, 199))
			textOut(hdc, r.Left+sx(12, w), r.Top+sy(29, hgt), detail1+"  //  "+detail2)
			if target > 0 {
				setTextColor.Call(hdc, rgb(94, 221, 255))
				textOut(hdc, r.Right-sx(220, w), r.Top+sy(8, hgt), fmt.Sprintf("%s / %s", formatAFKNumber(cur), formatAFKNumber(target)))
			}
			selectObject.Call(hdc, old)
		}
		return
	}

	// Context-sensitive Core progression row.
	if !afkCursorCoreBuilt() {
		r := afkContextButtonRect(w, hgt, 0, 1)
		drawAFKActionButton(hdc, r, "BUILD CURSOR CORE", afkCursorCoreBuildCost, afkSection1IsComplete() && gameMeta.AFKStarbits >= afkCursorCoreBuildCost, pointInRect(cursorPos, r))
		return
	}
	if !afkSection2IsComplete() {
		deploy := afkContextButtonRect(w, hgt, 0, 2)
		proc := afkContextButtonRect(w, hgt, 1, 2)
		dc := afkNextAutoCursorCost()
		pc := afkNextProcessingCost()
		drawAFKActionButton(hdc, deploy, fmt.Sprintf("DEPLOY AUTO CURSOR  %d/%d", gameMeta.AFKAutoCursors, afkAutoCursorMax), dc, (dc == 0 && gameMeta.AFKAutoCursors == 0) || (dc > 0 && gameMeta.AFKStarbits >= dc), pointInRect(cursorPos, deploy))
		drawAFKActionButton(hdc, proc, fmt.Sprintf("CURSOR PROCESSING  LV.%d", afkProcessingLevel()), pc, pc > 0 && gameMeta.AFKStarbits >= pc, pointInRect(cursorPos, proc))
		return
	}
	if afkCoreTier() < afkCoreMaxTier {
		// Offline earnings are claimed by the Welcome Back popup, so do not waste
		// permanent dock space on a non-interactive duplicate.
		r := afkContextButtonRect(w, hgt, 0, 1)
		cost := afkNextCoreTierCost()
		enabled := afkCoreTierAdvanceAllowed() && gameMeta.AFKStarbits >= cost
		drawAFKActionButton(hdc, r, fmt.Sprintf("UPGRADE CORE // TIER %d", afkCoreTier()+1), cost, enabled, pointInRect(cursorPos, r))
		return
	}
	// At max Core tier the clean world needs no persistent footer action. Offline
	// rewards continue to surface through the Welcome Back claim popup only.
	return
}

func afkHubControlAtPoint(p FPoint, w, hgt int32) int {
	for i := range afkUtilityNavItems() {
		if pointInRect(p, afkHubButtonRect(w, hgt, i)) {
			return i
		}
	}
	return -1
}

func handleAFKBottomHUDClick(h uintptr, p FPoint, w, hgt int32) bool {
	// The three main Starbase tabs behave like true modals: clicking anywhere
	// outside the active interface closes it. Hub buttons are exempt so the
	// player can switch directly between FACILITIES / GUIDE / SYSTEMS.
	hubIndex := afkHubControlAtPoint(p, w, hgt)
	// Every Starbase management interface behaves as a modal surface. The panel
	// itself and its contextual bottom action dock count as one interface; a click
	// anywhere else closes it. Top FACILITIES / GUIDE / SYSTEMS buttons remain an
	// intentional exception so players can switch interfaces in one click.
	if afkSecondaryManagementPanelOpen() && hubIndex < 0 && !pointInRect(p, afkManagementPanelRect(w, hgt)) && !pointInRect(p, afkBottomDockRect(w, hgt)) {
		afkCloseManagementPanels()
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	// Every Starbase stat card explicitly owns its click so input can never fall
	// through to the gameplay HUD underneath Starbase.
	for i := 0; i < 5; i++ {
		if pointInRect(p, afkLiveStatCardRect(w, hgt, i)) {
			if i == 1 {
				afkProductionPopupOpen = !afkProductionPopupOpen
				playUIButtonClickSound()
			} else if i == 4 {
				afkProductionPopupOpen = false
				if afkActionAllowed("overdrive", time.Now(), afkClaimActionDebounce) && afkStartOverdrive(time.Now()) {
					saveGameMeta()
					playUIButtonClickSound()
				}
			} else {
				afkProductionPopupOpen = false
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	if afkProductionPopupOpen {
		if pointInRect(p, afkProductionPopupRect(w, hgt)) {
			return true
		}
		afkProductionPopupOpen = false
		invalidateRect.Call(h, 0, 0)
		return true
	}
	// Starbase navigation buttons are true toggles. Clicking the button for the
	// interface that is already open closes it and returns to the clean Starbase
	// world instead of immediately reopening the same panel.
	if hubIndex >= 0 {
		item := afkUtilityNavItems()[hubIndex]
		alreadyOpen := (item.action == afkUtilityFacilities && afkFacilitiesPanelOpen) ||
			(item.action == afkUtilityGuide && afkGuidePanelOpen) ||
			(item.action == afkUtilitySystems && afkSystemsMenuOpen)
		if alreadyOpen {
			afkCloseManagementPanels()
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	if afkFacilitiesPanelOpen && !pointInRect(p, afkFacilitiesPanelRect(w, hgt)) && hubIndex < 0 {
		afkFacilitiesPanelOpen = false
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	if afkGuidePanelOpen && !pointInRect(p, afkGuidePanelRect(w, hgt)) && hubIndex < 0 {
		afkGuidePanelOpen = false
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	if afkSystemsMenuOpen && !pointInRect(p, afkSystemMenuRect(w, hgt)) && hubIndex < 0 {
		afkSystemsMenuOpen = false
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}

	if handleAFKFacilitiesHubClick(h, p, w, hgt) {
		return true
	}
	if handleAFKGuideClick(h, p, w, hgt) {
		return true
	}
	if handleAFKExchangeClick(h, p, w, hgt) {
		return true
	}

	// When the Systems selector is open its six large cards own input first.
	if afkSystemsMenuOpen {
		for i := 0; i < 6; i++ {
			r := afkSystemCardRect(w, hgt, i)
			if !pointInRect(p, r) {
				continue
			}
			enabled := false
			switch i {
			case 0:
				enabled = afkTechLabUnlocked()
			case 1:
				enabled = afkModulesAvailable()
			case 2:
				enabled = afkDronesAvailable()
			case 3:
				enabled = afkOrbitalAvailable()
			case 4:
				enabled = afkPrestigeAvailable()
			case 5:
				enabled = discordConnected
			}
			if !enabled {
				if i == 5 {
					afkSystemLockedPopupText = "EXCHANGE // DISCORD LOGIN REQUIRED FOR SERVER-VALIDATED PURCHASES"
					afkSystemLockedPopupUntil = time.Now().Add(2400 * time.Millisecond)
				} else {
					showAFKSystemLockedPopup(i)
				}
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return true
			}
			switch i {
			case 0:
				if afkTechLabUnlocked() {
					afkCloseManagementPanels()
					afkTalentPanelOpen = true
					afkTalentInfoOpen = false
					afkTalentInfoStarted = time.Time{}
					afkResetTalentPanToStart(w, hgt)
				}
			case 1:
				if afkModulesAvailable() {
					afkCloseManagementPanels()
					afkModulePanelOpen = true
					afkModuleSelectedItem = -1
				}
			case 2:
				if afkDronesAvailable() {
					afkCloseManagementPanels()
					afkDronePanelOpen = true
					afkDroneSelectedUpgrade = 0
				}
			case 3:
				if afkOrbitalAvailable() {
					afkCloseManagementPanels()
					afkOrbitalPanelOpen = true
					afkOrbitalSelectedUpgrade = 0
				}
			case 4:
				if afkPrestigeAvailable() {
					afkCloseManagementPanels()
					afkPrestigePanelOpen = true
				}
			case 5:
				if discordConnected {
					afkCloseManagementPanels()
					afkExchangePanelOpen = true
				}
			}
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
		// Click outside the menu closes it and returns to the clean world.
		if !pointInRect(p, afkSystemMenuRect(w, hgt)) {
			afkSystemsMenuOpen = false
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return true
	}

	// Three permanent top controls. Opening one closes every other management
	// surface so Starbase never stacks panels or creates visual clutter.
	items := afkUtilityNavItems()
	for i, item := range items {
		r := afkHubButtonRect(w, hgt, i)
		if !pointInRect(p, r) {
			continue
		}
		afkCloseManagementPanels()
		switch item.action {
		case afkUtilityFacilities:
			afkFacilitiesPanelOpen = true
		case afkUtilityGuide:
			afkGuidePanelOpen = true
			afkGuidePage = 0
		case afkUtilitySystems:
			afkSystemsMenuOpen = true
		}
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}

	if afkFacilitiesPanelOpen {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) && afkFacilityDrawerIndex >= 0 {
			if afkPerformFacilityContextAction(afkFacilityDrawerIndex) {
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}

	if afkEarningsPanelOpen {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) {
			if afkHasOfflineReward() {
				afkClaimOfflineReward()
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}
	if afkPrestigePanelOpen {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) {
			if afkBuyPrestige(time.Now()) {
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}
	if afkTechPanelOpen && !afkTechLabUnlocked() {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) {
			if afkActionAllowed("build_tech_lab", time.Now(), afkEconomyActionDebounce) && afkSpendStarbits(afkTechLabUnlockCost) {
				gameMeta.AFKTechLabUnlocked = true
				playAFKConstructionSound()
				afkEnsureSection6SaveShape()
				afkRefreshTalentPointAwards()
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	if afkTechPanelOpen && afkTechLabUnlocked() {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) {
			i := afkResearchSelected
			if i < 0 || i >= afkVisibleResearchCount() {
				i = 0
			}
			if afkBuyResearch(i) {
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}
	if afkTalentPanelOpen && afkTalentInfoOpen {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) {
			i := afkTalentSelected
			if i < 0 || i >= len(afkTalentDefs) {
				i = 0
			}
			if afkBuyTalent(i) {
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}
	if afkDronePanelOpen {
		if !afkDroneBayBuilt() {
			r := afkContextButtonRect(w, hgt, 0, 1)
			if pointInRect(p, r) {
				if afkBuildDroneBay() {
					saveGameMeta()
					playUIButtonClickSound()
				}
				invalidateRect.Call(h, 0, 0)
				return true
			}
			return false
		}
		u := afkContextButtonRect(w, hgt, 0, 2)
		a := afkContextButtonRect(w, hgt, 1, 2)
		if pointInRect(p, u) {
			if afkBuyDroneUpgrade(afkDroneSelectedUpgrade) {
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		if pointInRect(p, a) {
			if afkAscendDroneBay() {
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}

	if afkOrbitalPanelOpen {
		if !afkOrbitalBuilt() {
			r := afkContextButtonRect(w, hgt, 0, 1)
			if pointInRect(p, r) {
				if afkBuildOrbitalExtractor(time.Now()) {
					saveGameMeta()
					playUIButtonClickSound()
				}
				invalidateRect.Call(h, 0, 0)
				return true
			}
			return false
		}
		u := afkContextButtonRect(w, hgt, 0, 2)
		a := afkContextButtonRect(w, hgt, 1, 2)
		if pointInRect(p, u) {
			if afkBuyOrbitalUpgrade(afkOrbitalSelectedUpgrade) {
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		if pointInRect(p, a) {
			if afkAscendOrbital() {
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}

	if afkModulePanelOpen {
		if gameMeta.AFKPendingComponentCount > 0 {
			r := afkContextButtonRect(w, hgt, 0, 1)
			if pointInRect(p, r) {
				if afkClaimCraftComponent() {
					saveGameMeta()
					playUIButtonClickSound()
				}
				invalidateRect.Call(h, 0, 0)
				return true
			}
		}
		if afkModuleSelectedItem >= 0 {
			back, craft, _ := afkModuleContextRects(w, hgt)
			if pointInRect(p, back) {
				afkModuleSelectedItem = -1
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return true
			}
			if pointInRect(p, craft) {
				if afkCraftModule(afkModuleSelectedSet, afkModuleSelectedItem) {
					saveGameMeta()
					playUIButtonClickSound()
				}
				invalidateRect.Call(h, 0, 0)
				return true
			}
		} else {
			r := afkContextButtonRect(w, hgt, 0, 1)
			if pointInRect(p, r) && afkClaimAllModuleSetsReward() {
				saveGameMeta()
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return true
			}
		}
		return false
	}
	if afkOperatorPanelOpen {
		i := afkOperatorSelected
		if i < 0 || i >= len(afkOperatorDefs) {
			i = 0
		}
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) {
			changed := false
			if afkOperatorRecruited(i) {
				if afkOperatorCanLevel(i) {
					changed = afkLevelUpOperator(i)
				} else {
					changed = afkStartOperatorAssignment(i, time.Now())
				}
			} else {
				changed = afkRecruitOperator(i)
			}
			if changed {
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}
	if afkExpeditionPanelOpen {
		return false
	}

	if !afkCursorCoreBuilt() {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) {
			if afkSection1IsComplete() && afkActionAllowed("build_cursor_core", time.Now(), afkEconomyActionDebounce) && afkSpendStarbits(afkCursorCoreBuildCost) {
				gameMeta.AFKCursorCoreBuilt = true
				gameMeta.AFKCursorCoreTier = 1
				gameMeta.AFKCursorProcessingLevel = 1
				afkCoreBuildBurstStarted = time.Now()
				playAFKConstructionSound()
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}
	if !afkSection2IsComplete() {
		d := afkContextButtonRect(w, hgt, 0, 2)
		u := afkContextButtonRect(w, hgt, 1, 2)
		if pointInRect(p, d) {
			c := afkNextAutoCursorCost()
			if afkActionAllowed("deploy_auto_cursor", time.Now(), afkEconomyActionDebounce) && ((c == 0 && gameMeta.AFKAutoCursors == 0) || (c > 0 && afkSpendStarbits(c))) {
				gameMeta.AFKAutoCursors++
				playAFKUpgradeSound()
				afkRefreshSection2Completion()
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		if pointInRect(p, u) {
			c := afkNextProcessingCost()
			if c > 0 && afkActionAllowed("upgrade_cursor_processing", time.Now(), afkEconomyActionDebounce) && afkSpendStarbits(c) {
				gameMeta.AFKCursorProcessingLevel++
				playAFKUpgradeSound()
				afkRefreshSection2Completion()
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return false
	}
	if afkCoreTier() < afkCoreMaxTier {
		r := afkContextButtonRect(w, hgt, 0, 2)
		bank := afkContextButtonRect(w, hgt, 1, 2)
		if pointInRect(p, r) {
			c := afkNextCoreTierCost()
			if c > 0 && afkCoreTierAdvanceAllowed() && afkActionAllowed("upgrade_cursor_core", time.Now(), afkClaimActionDebounce) && afkSpendStarbits(c) {
				gameMeta.AFKCursorCoreTier++
				afkRefreshTalentPointAwards()
				playAFKTierUpSound()
				afkRefreshSection3Completion()
				saveGameMeta()
				playUIButtonClickSound()
			}
			invalidateRect.Call(h, 0, 0)
			return true
		}
		if pointInRect(p, bank) {
			invalidateRect.Call(h, 0, 0)
			return true
		}
	} else {
		r := afkContextButtonRect(w, hgt, 0, 1)
		if pointInRect(p, r) {
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	return false
}

func afkCoreTierAdvanceAllowed() bool {
	switch afkCoreTier() {
	case 2:
		return afkSection5IsComplete()
	case 3:
		return gameMeta.AFKSection6Complete
	case 4:
		return gameMeta.AFKSection8Complete
	default:
		return true
	}
}
