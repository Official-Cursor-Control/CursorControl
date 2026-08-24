//go:build windows

package main

import (
	"fmt"
	"time"
)

type afkStarbaseControlSprite struct {
	data []byte
	w, h int32
}

var afkStarbaseControls = map[string]*afkStarbaseControlSprite{
	"facilities": {w: 1052, h: 355},
	"guide":      {w: 1049, h: 356},
	"systems":    {w: 1077, h: 350},
}

var afkFacilitiesPanelOpen bool
var afkGuidePanelOpen bool
var afkGuidePage int

// The facility browser owns a nested bottom drawer. It never replaces the
// Facilities panel: selecting a card slides this detail view up from the panel
// bottom, and switching facilities closes/reopens the drawer quickly.
var afkFacilityDrawerIndex = -1
var afkFacilityDrawerPending = -1
var afkFacilityDrawerFrom = -1
var afkFacilityDrawerPhase int // 0=stable, 1=open, 2=switch-close, 3=close
var afkFacilityDrawerStarted time.Time

var afkFacilityHubOrder = [...]int{
	afkWorldCore,
	afkWorldScout,
	afkWorldTech,
	afkWorldOps,
	afkWorldModules,
	afkWorldDrone,
	afkWorldOrbital,
}

func loadAFKStarbaseControlAssets() {
	for name, s := range afkStarbaseControls {
		if s == nil {
			continue
		}
		s.data = readExternalBytes("ui", "starbase_controls", name+".bgra")
	}
}

func drawAFKStarbaseControl(hdc uintptr, name string, r RECT, selected, hovered bool) {
	s := afkStarbaseControls[name]
	if s != nil && len(s.data) >= int(s.w*s.h*4) {
		if hovered || selected {
			pad := int32(3)
			c := rgb(255, 200, 60)
			if !selected {
				c = rgb(78, 220, 255)
			}
			drawOutlineRect(hdc, RECT{r.Left - pad, r.Top - pad, r.Right + pad, r.Bottom + pad}, c, 2)
		}
		drawRawBGRATrimmedFit(hdc, s.data, s.w, s.h, r)
		return
	}
}

func afkHubButtonRect(w, hgt int32, index int) RECT {
	// Three large image buttons centred in the command deck.
	bw := sx(210, w)
	bh := sy(60, hgt)
	gap := sx(18, w)
	total := bw*3 + gap*2
	left := (w - total) / 2
	x := left + int32(index)*(bw+gap)
	return RECT{x, sy(157, hgt), x + bw, sy(157, hgt) + bh}
}

func afkFacilitiesPanelRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }
func afkFacilityHubCardRect(w, hgt int32, index int) RECT {
	p := afkFacilitiesPanelRect(w, hgt)
	cols := 4
	gapX := sx(14, w)
	gapY := sy(14, hgt)
	mx := sx(26, w)
	top := sy(84, hgt)
	cw := (p.Right - p.Left - mx*2 - gapX*3) / 4
	ch := sy(175, hgt)
	row, col := index/cols, index%cols
	if row == 1 && index >= 4 { // centre second row of three cards
		start := p.Left + (p.Right-p.Left-(cw*3+gapX*2))/2
		x := start + int32(col)*(cw+gapX)
		return RECT{x, p.Top + top + int32(row)*(ch+gapY), x + cw, p.Top + top + int32(row)*(ch+gapY) + ch}
	}
	x := p.Left + mx + int32(col)*(cw+gapX)
	y := p.Top + top + int32(row)*(ch+gapY)
	return RECT{x, y, x + cw, y + ch}
}

func afkFacilityHubName(i int) string {
	return [...]string{"CURSOR CORE", "SCOUT PAD", "TECH LAB", "OPERATIONS", "FABRICATOR", "DRONE BAY", "ORBITAL EXTRACTOR"}[i]
}
func afkFacilityHubSprite(i int) string {
	switch i {
	case afkWorldCore:
		t := afkCoreTier()
		if t < 1 {
			t = 1
		}
		if t > 5 {
			t = 5
		}
		return fmt.Sprintf("cursor_core_t%d", t)
	case afkWorldScout:
		t := afkScoutTier()
		if t < 1 {
			t = 1
		}
		if t > 3 {
			t = 3
		}
		return fmt.Sprintf("scout_pad_t%d", t)
	case afkWorldTech:
		return "tech_lab"
	case afkWorldOps:
		return "operations"
	case afkWorldModules:
		return "module_fabricator"
	case afkWorldDrone:
		t := afkDroneBayTier()
		if t < 1 {
			t = 1
		}
		if t > 3 {
			t = 3
		}
		return fmt.Sprintf("drone_bay_t%d", t)
	case afkWorldOrbital:
		t := afkOrbitalTier()
		if t < 1 {
			t = 1
		}
		if t > 3 {
			t = 3
		}
		return fmt.Sprintf("orbital_extractor_t%d", t)
	}
	return ""
}
func afkFacilityHubDesc(i int) string {
	return [...]string{
		"AUTO CURSORS + CORE TIERS",
		"EXPEDITIONS + SCOUT TIERS",
		"RESEARCH + GLOBAL UPGRADES",
		"OPERATORS + SERVICE BONUSES",
		"MODULE SETS + CRAFTING",
		"MINING DRONES + DRONE OUTPUT",
		"ORBITAL BURSTS + ENDGAME",
	}[i]
}
func afkFacilityHubStatus(i int) string {
	if afkWorldBuilt(i) {
		switch i {
		case afkWorldCore:
			return fmt.Sprintf("TIER %d / %d // ACTIVE", afkCoreTier(), afkCoreMaxTier)
		case afkWorldScout:
			return fmt.Sprintf("TIER %d / %d // ACTIVE", afkScoutTier(), afkScoutMaxTier)
		case afkWorldDrone:
			return fmt.Sprintf("TIER %d / %d // ACTIVE", afkDroneBayTier(), afkDroneBayMaxTier)
		case afkWorldOrbital:
			return fmt.Sprintf("TIER %d / %d // ACTIVE", afkOrbitalTier(), afkOrbitalMaxTier)
		case afkWorldTech:
			return "BUILT // RESEARCH LEVELS"
		case afkWorldOps:
			return "ACTIVE // OPERATOR PROGRESSION"
		case afkWorldModules:
			return "ACTIVE // MODULE PROGRESSION"
		}
	}
	return afkFacilityRequirement(i)
}

func afkFacilityDrawerTitle(i int) string {
	name := afkFacilityHubName(i)
	switch i {
	case afkWorldCore:
		if afkCursorCoreBuilt() {
			return fmt.Sprintf("%s (TIER %d/%d)", name, afkCoreTier(), afkCoreMaxTier)
		}
	case afkWorldScout:
		if afkScoutUnlocked() {
			return fmt.Sprintf("%s (TIER %d/%d)", name, afkScoutTier(), afkScoutMaxTier)
		}
	case afkWorldDrone:
		if afkDroneBayBuilt() {
			return fmt.Sprintf("%s (TIER %d/%d)", name, afkDroneBayTier(), afkDroneBayMaxTier)
		}
	case afkWorldOrbital:
		if afkOrbitalBuilt() {
			return fmt.Sprintf("%s (TIER %d/%d)", name, afkOrbitalTier(), afkOrbitalMaxTier)
		}
	}
	return name
}

type afkRequirementProgress struct {
	Label           string
	Current, Target int64
}

func afkFacilityRequirementProgress(i int) []afkRequirementProgress {
	rows := []afkRequirementProgress{}
	switch i {
	case afkWorldScout:
		if afkScoutUnlocked() {
			nav, runs := afkNextScoutTierCost()
			if nav > 0 {
				rows = append(rows, afkRequirementProgress{fmt.Sprintf("%d NAV DATA", nav), int64(gameMeta.AFKNavigationData), int64(nav)})
				rows = append(rows, afkRequirementProgress{fmt.Sprintf("%d COMPLETED EXPEDITIONS", runs), int64(gameMeta.AFKExpeditionsCompleted), int64(runs)})
			}
		}
	case afkWorldCore:
		if afkCursorCoreBuilt() && afkCoreTier() == 2 {
			rows = append(rows,
				afkRequirementProgress{"SCOUT PAD TIER 2", int64(afkScoutTier()), 2},
				afkRequirementProgress{"6 COMPLETED EXPEDITIONS", int64(gameMeta.AFKExpeditionsCompleted), 6},
				afkRequirementProgress{"EXPEDITION SPEED LV.3", int64(afkScoutSpeedLevel()), 3},
				afkRequirementProgress{"SCOUT SYSTEMS LV.3", int64(afkScoutCapacityLevel()), 3},
				afkRequirementProgress{"CACHE CHANCE LV.3", int64(afkScoutCacheLevel()), 3},
			)
		}
	case afkWorldDrone:
		if afkDroneBayBuilt() && afkDroneBayTier() < afkDroneBayMaxTier {
			min := int64(10)
			if afkDroneBayTier() == 2 {
				min = 20
			}
			rows = append(rows, afkRequirementProgress{"10 DRONES DEPLOYED", int64(gameMeta.AFKDronesDeployed), 10}, afkRequirementProgress{fmt.Sprintf("MINING SPEED LV.%d", min), int64(afkDroneUpgradeLevel(1)), min}, afkRequirementProgress{fmt.Sprintf("DRONE POWER LV.%d", min), int64(afkDroneUpgradeLevel(2)), min}, afkRequirementProgress{fmt.Sprintf("DRONE STORAGE LV.%d", min), int64(afkDroneUpgradeLevel(3)), min})
		}
	case afkWorldOrbital:
		if afkOrbitalBuilt() && afkOrbitalTier() < afkOrbitalMaxTier {
			min := int64(8)
			if afkOrbitalTier() == 2 {
				min = 14
			}
			rows = append(rows, afkRequirementProgress{fmt.Sprintf("ORBITAL OUTPUT LV.%d", min), int64(afkOrbitalUpgradeLevel(0)), min}, afkRequirementProgress{fmt.Sprintf("CYCLE SPEED LV.%d", min), int64(afkOrbitalUpgradeLevel(1)), min}, afkRequirementProgress{fmt.Sprintf("ORBITAL CONTROL LV.%d", min), int64(afkOrbitalUpgradeLevel(2)), min})
		}
	}
	return rows
}

func afkRequirementBarColor(cur, target int64) uintptr {
	if target <= 0 || cur >= target {
		return rgb(65, 210, 105)
	}
	p := float64(cur) / float64(target)
	if p < .33 {
		return rgb(220, 68, 62)
	}
	if p < .66 {
		return rgb(238, 137, 45)
	}
	return rgb(245, 211, 62)
}

func drawAFKRequirementBar(hdc uintptr, r RECT, cur, target int64) {
	fillSolidRect(hdc, r, rgb(20, 28, 42))
	drawOutlineRect(hdc, r, rgb(78, 102, 126), 1)
	if target <= 0 {
		return
	}
	p := float64(cur) / float64(target)
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	inner := RECT{r.Left + 1, r.Top + 1, r.Right - 1, r.Bottom - 1}
	inner.Right = inner.Left + int32(float64(inner.Right-inner.Left)*p)
	if inner.Right > inner.Left {
		fillSolidRect(hdc, inner, afkRequirementBarColor(cur, target))
	}
}

func afkFacilityDrawerBaseRect(w, hgt int32) RECT {
	p := afkFacilitiesPanelRect(w, hgt)
	return RECT{p.Left + sx(18, w), p.Bottom - sy(244, hgt), p.Right - sx(18, w), p.Bottom - sy(10, hgt)}
}
func afkFacilityDrawerManageRect(w, hgt int32) RECT {
	r := afkFacilityDrawerBaseRect(w, hgt)
	return RECT{r.Right - sx(174, w), r.Bottom - sy(44, hgt), r.Right - sx(16, w), r.Bottom - sy(12, hgt)}
}
func afkFacilityDrawerUpgradeRect(w, hgt int32) RECT {
	r := afkFacilityDrawerBaseRect(w, hgt)
	return RECT{r.Right - sx(350, w), r.Bottom - sy(44, hgt), r.Right - sx(184, w), r.Bottom - sy(12, hgt)}
}
func afkFacilityTierLabel(i int) string {
	switch i {
	case afkWorldCore:
		if !afkCursorCoreBuilt() {
			return "NOT BUILT"
		}
		return fmt.Sprintf("TIER %d / %d", afkCoreTier(), afkCoreMaxTier)
	case afkWorldScout:
		if !afkScoutUnlocked() {
			return "NOT BUILT"
		}
		return fmt.Sprintf("TIER %d / %d", afkScoutTier(), afkScoutMaxTier)
	case afkWorldDrone:
		if !afkDroneBayBuilt() {
			return "NOT BUILT"
		}
		return fmt.Sprintf("TIER %d / %d", afkDroneBayTier(), afkDroneBayMaxTier)
	case afkWorldOrbital:
		if !afkOrbitalBuilt() {
			return "NOT BUILT"
		}
		return fmt.Sprintf("TIER %d / %d", afkOrbitalTier(), afkOrbitalMaxTier)
	case afkWorldTech:
		if afkWorldBuilt(i) {
			return "RESEARCH FACILITY"
		}
	case afkWorldOps:
		if afkWorldBuilt(i) {
			return "OPERATOR FACILITY"
		}
	case afkWorldModules:
		if afkWorldBuilt(i) {
			return "CRAFTING FACILITY"
		}
	}
	return "LOCKED"
}
func afkFacilityPurpose(i int) string {
	return [...]string{
		"Runs Starbase production. Higher Core tiers multiply output and unlock the wider facility network.",
		"Launches Space Expeditions, earns NAV Data and Star Caches, and improves scout travel systems.",
		"Researches permanent Starbase upgrades that improve production, capacity and exploration.",
		"Recruits specialist operators whose passive bonuses strengthen different Starbase systems.",
		"Crafts module sets from expedition components to unlock permanent production bonuses.",
		"Deploys mining drones for continuous Starbit production and unlocks stronger Drone Bay tiers.",
		"Fires automated orbital extraction bursts and provides late-game Starbit production.",
	}[i]
}
func afkFacilityCurrentEffect(i int) string {
	switch i {
	case afkWorldCore:
		if !afkCursorCoreBuilt() {
			return "CURRENT // NOT BUILT"
		}
		return fmt.Sprintf("CURRENT // %s CORE MULTIPLIER", afkCoreTierMultiplierText())
	case afkWorldScout:
		if !afkScoutUnlocked() {
			return "CURRENT // NO SCOUT"
		}
		return fmt.Sprintf("CURRENT // TIER %d // %d EXPEDITIONS COMPLETE", afkScoutTier(), gameMeta.AFKExpeditionsCompleted)
	case afkWorldTech:
		return "CURRENT // PERMANENT RESEARCH TREE"
	case afkWorldOps:
		return fmt.Sprintf("CURRENT // %d OPERATORS RECRUITED", gameMeta.AFKOperatorsRecruitedCount)
	case afkWorldModules:
		return fmt.Sprintf("CURRENT // %d MODULE SETS COMPLETE", gameMeta.AFKEquipmentSetsComplete)
	case afkWorldDrone:
		if !afkDroneBayBuilt() {
			return "CURRENT // NO DRONES"
		}
		return fmt.Sprintf("CURRENT // %d/10 DRONES DEPLOYED", gameMeta.AFKDronesDeployed)
	case afkWorldOrbital:
		if !afkOrbitalBuilt() {
			return "CURRENT // NOT BUILT"
		}
		return fmt.Sprintf("CURRENT // %d BURSTS FIRED", gameMeta.AFKOrbitalBurstsFired)
	}
	return ""
}
func afkFacilityNextUpgrade(i int) (string, bool) {
	switch i {
	case afkWorldCore:
		if !afkCursorCoreBuilt() {
			return fmt.Sprintf("BUILD COST // %s STARBITS", formatAFKNumber(afkCursorCoreBuildCost)), false
		}
		c := afkNextCoreTierCost()
		if c <= 0 {
			return "NEXT TIER // MAXIMUM", false
		}
		return fmt.Sprintf("NEXT // TIER %d // %s STARBITS", afkCoreTier()+1, formatAFKNumber(c)), afkCoreTierAdvanceAllowed() && gameMeta.AFKStarbits >= c
	case afkWorldScout:
		if !afkScoutUnlocked() {
			return fmt.Sprintf("BUILD COST // %s STARBITS", formatAFKNumber(afkScoutShipUnlockCost)), false
		}
		c, r := afkNextScoutTierCost()
		if c <= 0 {
			return "NEXT TIER // MAXIMUM", false
		}
		return fmt.Sprintf("NEXT // TIER %d // %d NAV + %d RUNS", afkScoutTier()+1, c, r), afkCanUpgradeScoutTier()
	case afkWorldDrone:
		if !afkDroneBayBuilt() {
			return fmt.Sprintf("BUILD COST // %s STARBITS", formatAFKNumber(afkDroneBayBuildCost)), false
		}
		c := afkDroneBayAscendCost()
		if c <= 0 {
			return "NEXT TIER // MAXIMUM", false
		}
		return fmt.Sprintf("NEXT // TIER %d // %s STARBITS", afkDroneBayTier()+1, formatAFKNumber(c)), afkDroneBayCanAscend() && gameMeta.AFKStarbits >= c
	case afkWorldOrbital:
		if !afkOrbitalBuilt() {
			return fmt.Sprintf("BUILD COST // %s STARBITS", formatAFKNumber(afkOrbitalBuildCost)), false
		}
		c := afkOrbitalAscendCost()
		if c <= 0 {
			return "NEXT TIER // MAXIMUM", false
		}
		return fmt.Sprintf("NEXT // TIER %d // %s STARBITS", afkOrbitalTier()+1, formatAFKNumber(c)), afkOrbitalCanAscend() && gameMeta.AFKStarbits >= c
	case afkWorldTech:
		return "PROGRESSION // RESEARCH LEVELS", false
	case afkWorldOps:
		return "PROGRESSION // RECRUIT + LEVEL OPERATORS", false
	case afkWorldModules:
		return "PROGRESSION // CRAFT MODULE SETS", false
	}
	return "", false
}
func afkFacilityDrawerShownRect(w, hgt int32) (RECT, int) {
	base := afkFacilityDrawerBaseRect(w, hgt)
	idx := afkFacilityDrawerIndex
	if idx < 0 && afkFacilityDrawerFrom < 0 {
		return base, -1
	}
	now := time.Now()
	dur := 160 * time.Millisecond
	p := 1.0
	if !afkFacilityDrawerStarted.IsZero() {
		p = float64(now.Sub(afkFacilityDrawerStarted)) / float64(dur)
		if p > 1 {
			p = 1
		}
		if p < 0 {
			p = 0
		}
	}
	off := int32(0)
	switch afkFacilityDrawerPhase {
	case 1:
		off = int32(float64(base.Bottom-base.Top) * (1 - p))
		if p >= 1 {
			afkFacilityDrawerPhase = 0
		}
	case 2:
		idx = afkFacilityDrawerFrom
		off = int32(float64(base.Bottom-base.Top) * p)
		if p >= 1 {
			afkFacilityDrawerIndex = afkFacilityDrawerPending
			afkFacilityDrawerPending = -1
			afkFacilityDrawerFrom = -1
			afkFacilityDrawerPhase = 1
			afkFacilityDrawerStarted = now
			idx = afkFacilityDrawerIndex
			off = base.Bottom - base.Top
		}
	case 3:
		idx = afkFacilityDrawerFrom
		off = int32(float64(base.Bottom-base.Top) * p)
		if p >= 1 {
			afkFacilityDrawerIndex = -1
			afkFacilityDrawerFrom = -1
			afkFacilityDrawerPhase = 0
			return base, -1
		}
	}
	base.Top += off
	base.Bottom += off
	return base, idx
}
func afkSelectFacilityDrawer(i int) {
	now := time.Now()
	if afkFacilityDrawerIndex < 0 {
		afkFacilityDrawerIndex = i
		afkFacilityDrawerPhase = 1
		afkFacilityDrawerStarted = now
		return
	}
	if afkFacilityDrawerIndex == i {
		afkFacilityDrawerFrom = i
		afkFacilityDrawerPhase = 3
		afkFacilityDrawerStarted = now
		return
	}
	afkFacilityDrawerFrom = afkFacilityDrawerIndex
	afkFacilityDrawerPending = i
	afkFacilityDrawerPhase = 2
	afkFacilityDrawerStarted = now
}
func drawAFKFacilityDrawer(hdc uintptr, w, hgt int32) {
	r, idx := afkFacilityDrawerShownRect(w, hgt)
	if idx < 0 {
		return
	}
	face, light, dark := themedHUDPanelPalette(false)
	drawBevelPanel(hdc, r, blendColor(face, dark, 0.08), blendColor(light, rgb(255, 255, 255), 0.16), dark, 3)
	prev := RECT{r.Left + sx(12, w), r.Top + sy(12, hgt), r.Left + sx(154, w), r.Bottom - sy(12, hgt)}
	drawAFKFacilitySpriteGrounded(hdc, afkFacilityHubSprite(idx), prev)
	x := r.Left + sx(166, w)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(245, 250, 255))
		textOut(hdc, x, r.Top+sy(14, hgt), afkFacilityDrawerTitle(idx))
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(205, 225, 236))
		purpose := fitTextEllipsis(hdc, hudTinyFont, afkFacilityPurpose(idx), r.Right-r.Left-sx(540, w))
		textOut(hdc, x, r.Top+sy(42, hgt), purpose)
		if idx == afkWorldScout {
			setTextColor.Call(hdc, blendColor(light, rgb(180, 236, 255), .38))
			textOut(hdc, x, r.Top+sy(65, hgt), fmt.Sprintf("TOTAL EXPEDITIONS COMPLETED // %d", gameMeta.AFKExpeditionsCompleted))
		} else {
			setTextColor.Call(hdc, blendColor(light, rgb(180, 236, 255), .38))
			textOut(hdc, x, r.Top+sy(65, hgt), afkFacilityCurrentEffect(idx))
		}
		next, _ := afkFacilityNextUpgrade(idx)
		setTextColor.Call(hdc, rgb(255, 211, 74))
		textOut(hdc, x, r.Top+sy(87, hgt), next)
		rows := afkFacilityRequirementProgress(idx)
		if len(rows) > 0 {
			setTextColor.Call(hdc, rgb(225, 238, 248))
			textOut(hdc, x, r.Top+sy(111, hgt), "REQUIREMENTS:")
			cols := 1
			if len(rows) > 3 {
				cols = 2
			}
			colGap := sx(32, w)
			avail := r.Right - x - sx(70, w)
			barW := sx(330, w)
			if cols == 2 {
				barW = sx(285, w)
			}
			if barW > avail {
				barW = avail
			}
			for j, row := range rows {
				col := 0
				rr := j
				if cols == 2 {
					col = j % 2
					rr = j / 2
				}
				xx := x + int32(col)*(barW+colGap)
				yy := r.Top + sy(132, hgt) + int32(rr)*sy(34, hgt)
				label := row.Label
				if row.Target > 0 {
					label = fmt.Sprintf("%s  [%s / %s]", row.Label, formatAFKNumber(row.Current), formatAFKNumber(row.Target))
				}
				setTextColor.Call(hdc, rgb(190, 216, 232))
				label = fitTextEllipsis(hdc, hudTinyFont, label, barW)
				textOut(hdc, xx, yy, label)
				drawAFKRequirementBar(hdc, RECT{xx, yy + sy(21, hgt), xx + barW, yy + sy(28, hgt)}, row.Current, row.Target)
			}
		} else {
			setTextColor.Call(hdc, rgb(151, 198, 220))
			req := fitTextEllipsis(hdc, hudTinyFont, afkFacilityRequirement(idx), r.Right-r.Left-sx(540, w))
			textOut(hdc, x, r.Top+sy(113, hgt), req)
		}
		selectObject.Call(hdc, old)
	}
	upgradeLabel, upgradeCost, can := afkFacilityContextAction(idx)
	drawAFKActionButton(hdc, afkFacilityDrawerUpgradeRect(w, hgt), upgradeLabel, upgradeCost, can, pointInRect(cursorPos, afkFacilityDrawerUpgradeRect(w, hgt)))
	drawAFKActionButton(hdc, afkFacilityDrawerManageRect(w, hgt), "OPEN FACILITY", 0, true, pointInRect(cursorPos, afkFacilityDrawerManageRect(w, hgt)))
}

func afkPerformFacilityContextAction(i int) bool {
	switch i {
	case afkWorldCore:
		if !afkCursorCoreBuilt() {
			if afkSection1IsComplete() && afkActionAllowed("build_cursor_core", time.Now(), afkEconomyActionDebounce) && afkSpendStarbits(afkCursorCoreBuildCost) {
				gameMeta.AFKCursorCoreBuilt = true
				gameMeta.AFKCursorCoreTier = 1
				gameMeta.AFKCursorProcessingLevel = 1
				afkCoreBuildBurstStarted = time.Now()
				playAFKConstructionSound()
				saveGameMeta()
				return true
			}
			return false
		}
		c := afkNextCoreTierCost()
		if c > 0 && afkCoreTierAdvanceAllowed() && afkActionAllowed("upgrade_cursor_core", time.Now(), afkClaimActionDebounce) && afkSpendStarbits(c) {
			gameMeta.AFKCursorCoreTier++
			afkRefreshTalentPointAwards()
			afkRefreshSection3Completion()
			playAFKTierUpSound()
			saveGameMeta()
			return true
		}
	case afkWorldScout:
		if !afkScoutUnlocked() {
			if afkCoreTier() >= 2 && gameMeta.AFKStarbits >= afkScoutShipUnlockCost && afkSpendStarbits(afkScoutShipUnlockCost) {
				gameMeta.AFKScoutShipUnlocked = true
				gameMeta.AFKScoutShipTier = 1
				gameMeta.AFKExpeditionSpeedLevel = 1
				gameMeta.AFKCapacityLevel = 1
				gameMeta.AFKCacheChanceLevel = 1
				playAFKConstructionSound()
				saveGameMeta()
				return true
			}
			return false
		}
		return afkUpgradeScoutTier()
	case afkWorldTech:
		if !afkTechLabUnlocked() {
			if afkTechLabAvailable() && afkActionAllowed("build_tech_lab", time.Now(), afkEconomyActionDebounce) && afkSpendStarbits(afkTechLabUnlockCost) {
				gameMeta.AFKTechLabUnlocked = true
				playAFKConstructionSound()
				afkEnsureSection6SaveShape()
				afkRefreshTalentPointAwards()
				saveGameMeta()
				return true
			}
			return false
		}
		afkOpenFacilityManagement(i)
		return true
	case afkWorldOps:
		if afkOperatorsAvailable() {
			afkOpenFacilityManagement(i)
			return true
		}
	case afkWorldModules:
		if afkModulesAvailable() {
			afkOpenFacilityManagement(i)
			return true
		}
	case afkWorldDrone:
		if !afkDroneBayBuilt() {
			return afkBuildDroneBay()
		}
		return afkAscendDroneBay()
	case afkWorldOrbital:
		if !afkOrbitalBuilt() {
			return afkBuildOrbitalExtractor(time.Now())
		}
		return afkAscendOrbital()
	}
	return false
}

func afkUpgradeFacilityFromDrawer(i int) bool { return afkPerformFacilityContextAction(i) }

func drawAFKFacilitiesHub(hdc uintptr, w, hgt int32) {
	if !afkFacilitiesPanelOpen {
		return
	}
	drawAFKManagementBackdrop(hdc, w, hgt)
	p := afkFacilitiesPanelRect(w, hgt)
	level := afkStarbaseLevel()
	drawAFKManagementChrome(hdc, p, "STARBASE FACILITIES", fmt.Sprintf("STARBASE LEVEL %d / %d // %s // NEXT UNLOCK: %s", level, afkStarbaseMaxLevel, afkStarbaseLevelName(level), afkStarbaseLevelNextUnlock(level)), rgb(83, 209, 255), w, hgt)
	// Cards are deliberately ordered by progression requirement: Core -> Scout ->
	// Tech -> Operations -> Fabricator -> Drone Bay -> Orbital Extractor.
	for displayIndex, worldIndex := range afkFacilityHubOrder {
		r := afkFacilityHubCardRect(w, hgt, displayIndex)
		hov := pointInRect(cursorPos, r)
		fill := rgb(5, 25, 50)
		border := rgb(50, 104, 145)
		if afkWorldBuilt(worldIndex) {
			border = rgb(76, 220, 147)
		}
		if afkFacilityDrawerIndex == worldIndex || afkFacilityDrawerPending == worldIndex {
			fill = blendColor(fill, rgb(85, 160, 210), 0.18)
			border = rgb(255, 211, 74)
		}
		if hov {
			fill = rgb(13, 39, 70)
			border = rgb(255, 199, 60)
		}
		drawBevelPanel(hdc, r, fill, border, rgb(0, 5, 14), 3)
		prev := RECT{r.Left + sx(10, w), r.Top + sy(8, hgt), r.Right - sx(10, w), r.Top + sy(94, hgt)}
		drawAFKFacilitySpriteGrounded(hdc, afkFacilityHubSprite(worldIndex), prev)
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(245, 250, 255))
			name := fitTextEllipsis(hdc, hudSmallFont, afkFacilityHubName(worldIndex), r.Right-r.Left-sx(18, w))
			centeredTextOut(hdc, r.Left+8, r.Right-8, r.Top+sy(102, hgt), name)
			selectObject.Call(hdc, old)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(122, 180, 207))
			d := fitTextEllipsis(hdc, hudTinyFont, afkFacilityHubDesc(worldIndex), r.Right-r.Left-sx(16, w))
			centeredTextOut(hdc, r.Left+8, r.Right-8, r.Top+sy(128, hgt), d)
			setTextColor.Call(hdc, rgb(255, 211, 74))
			st := fitTextEllipsis(hdc, hudTinyFont, afkFacilityHubStatus(worldIndex), r.Right-r.Left-sx(16, w))
			centeredTextOut(hdc, r.Left+8, r.Right-8, r.Bottom-sy(27, hgt), st)
			selectObject.Call(hdc, old)
		}
	}
	drawAFKFacilityDrawer(hdc, w, hgt)
}

func afkGuidePanelRect(w, hgt int32) RECT { return afkFacilitiesPanelRect(w, hgt) }

const afkGuidePageCount = 6

func afkGuideCardRect(w, hgt int32, index int) RECT {
	p := afkGuidePanelRect(w, hgt)
	mx := sx(42, w)
	top := sy(92, hgt)
	bottomReserve := sy(72, hgt)
	gap := sy(14, hgt)
	usable := p.Bottom - bottomReserve - (p.Top + top)
	ch := (usable - gap*2) / 3
	if ch < sy(105, hgt) {
		ch = sy(105, hgt)
	}
	y := p.Top + top + int32(index)*(ch+gap)
	return RECT{p.Left + mx, y, p.Right - mx, y + ch}
}
func afkGuidePrevRect(w, hgt int32) RECT {
	p := afkGuidePanelRect(w, hgt)
	return RECT{p.Left + sx(30, w), p.Bottom - sy(48, hgt), p.Left + sx(180, w), p.Bottom - sy(16, hgt)}
}
func afkGuideNextRect(w, hgt int32) RECT {
	p := afkGuidePanelRect(w, hgt)
	return RECT{p.Right - sx(180, w), p.Bottom - sy(48, hgt), p.Right - sx(30, w), p.Bottom - sy(16, hgt)}
}

func drawGuideCard(hdc uintptr, r RECT, title, line1, line2, line3 string, w, hgt int32) {
	drawBevelPanel(hdc, r, rgb(5, 25, 49), rgb(63, 137, 178), rgb(0, 5, 14), 2)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 214, 70))
		t := fitTextEllipsis(hdc, hudSmallFont, title, r.Right-r.Left-sx(24, w))
		textOut(hdc, r.Left+sx(14, w), r.Top+sy(12, hgt), t)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		lines := []struct {
			text string
			c    uintptr
			y    float64
		}{
			{line1, rgb(226, 240, 249), 48},
			{line2, rgb(151, 205, 231), 75},
			{line3, rgb(111, 160, 188), 102},
		}
		for _, line := range lines {
			if line.text == "" {
				continue
			}
			setTextColor.Call(hdc, line.c)
			t := fitTextEllipsis(hdc, hudTinyFont, line.text, r.Right-r.Left-sx(28, w))
			textOut(hdc, r.Left+sx(14, w), r.Top+sy(line.y, hgt), t)
		}
		selectObject.Call(hdc, old)
	}
}

func drawAFKGuide(hdc uintptr, w, hgt int32) {
	if !afkGuidePanelOpen {
		return
	}
	drawAFKManagementBackdrop(hdc, w, hgt)
	p := afkGuidePanelRect(w, hgt)
	drawAFKManagementChrome(hdc, p, "STARBASE GUIDE", "THREE THINGS AT A TIME // CLEAR RULES // EXACT REQUIREMENTS", rgb(176, 89, 255), w, hgt)
	rankName := afkGlobalRankName()
	rankBonus := afkGlobalRankMultiplierText()
	level := afkStarbaseLevel()
	pages := [][][4]string{
		{
			{"1. FOLLOW THE NEXT OBJECTIVE", fmt.Sprintf("YOU ARE STARBASE LEVEL %d / %d // %s", level, afkStarbaseMaxLevel, afkStarbaseLevelName(level)), afkStarbaseNextObjectiveText(), "THE OBJECTIVE BAR AT THE TOP IS THE FASTEST WAY TO KNOW WHAT TO BUILD OR UPGRADE NEXT."},
			{"2. STARBASE LEVEL IS NOT A FACILITY TIER", "STARBASE LEVEL = YOUR OVERALL CAMPAIGN STAGE.", "FACILITY TIER = THE UPGRADE LEVEL OF ONE BUILDING, SUCH AS CURSOR CORE OR DRONE BAY.", "A NEW STARBASE LEVEL OPENS SYSTEMS; FACILITY TIERS MAKE THOSE SYSTEMS STRONGER."},
			{"3. IF SOMETHING IS LOCKED", "READ THE REQUIREMENT SHOWN ON THE LOCKED CARD OR NODE.", "BUILD OR UPGRADE THE NAMED FACILITY FIRST; A BONUS CANNOT RUN BEFORE ITS PHYSICAL SYSTEM EXISTS.", "FACILITIES -> SELECT A BUILDING TO SEE ITS CURRENT TIER, NEXT COST AND EXACT REQUIREMENT."},
		},
		{
			{"CURSOR CORE // YOUR ENGINE", "THE CURSOR CORE IS THE MAIN STARBIT-PRODUCTION FACILITY AND A MAJOR PROGRESSION GATE.", "UPGRADE ITS TIER WHEN THE FACILITIES DRAWER SAYS THE REQUIREMENT IS COMPLETE.", "NOVA DOES NOT LEVEL PASSIVELY: RECRUIT HER, THEN CLICK HER CARD TO START CORE CALIBRATION ASSIGNMENTS."},
			{"SCOUT PAD // EXPEDITIONS", "LAUNCH SHIPS TO EARN NAV DATA, STARBITS, STAR CACHES AND MODULE COMPONENTS.", "THE SCOUT PAD HAS ITS OWN TIERS; DEEPER TIERS REQUIRE NAV DATA AND COMPLETED EXPEDITIONS.", "VEGA REQUIRES THE SCOUT PAD, BUT EXPEDITIONS DO NOT LEVEL HIM: START ROUTE SURVEY ASSIGNMENTS FROM HIS CARD."},
			{"TECH LAB // RESEARCH", "THE TECH LAB BUYS PERMANENT RESEARCH LEVELS AND UNLOCKS THE PILOT SKILL NETWORK.", "RESEARCH LEVELS ARE SEPARATE FROM STARBASE LEVELS AND FACILITY TIERS.", "BYTE REQUIRES THE TECH LAB; CLICK HER CARD TO PAY FOR A SYSTEM ANALYSIS ASSIGNMENT AND EARN SERVICE XP."},
		},
		{
			{"OPERATORS // START WORK YOURSELF", "OPERATORS NEVER FARM SERVICE XP AUTOMATICALLY. CLICK AN IDLE RECRUITED OPERATOR TO START THEIR PAID ASSIGNMENT.", "EACH SPECIALIST HAS A DIFFERENT WORK TIMER AND COOLDOWN. WHEN THE JOB FINISHES, SERVICE XP IS AWARDED ONCE; NOTHING AUTO-RESTARTS.", "WHEN SERVICE XP REACHES THE TARGET, THE CARD SAYS READY. CLICK THE OPERATOR AGAIN TO LEVEL UP, PLAY THEIR QUIP AND TRIGGER THE COLOUR FILL."},
			{"FABRICATOR // SHIP MODULES", "EXPEDITIONS RECOVER THE COMPONENTS USED TO CRAFT PERMANENT MODULE SETS.", "PIXEL WIRE / STAR ALLOY / POWER CELL: ANY EXPEDITION. QUANTUM GLASS / VOID CRYSTAL: ANY, BETTER ON LONG ROUTES.", "NOVA CORE: DERELICT SIGNAL OR FARTHER. ASTRA REQUIRES THE FABRICATOR; START SUPPLY AUDITS MANUALLY FROM HER CARD."},
			{"DRONE BAY // AUTOMATED MINING", "BUILD THE DRONE BAY BEFORE ANY DRONE-BASED BONUS OR PILOT SKILL CAN BECOME AVAILABLE.", "DEPLOY AND UPGRADE DRONES TO ADD AUTOMATED STARBIT OUTPUT; LATER DRONE BAY TIERS REQUIRE THEIR OWN GOALS.", "FLUX REQUIRES THE DRONE BAY. DRONES DO NOT PASSIVELY LEVEL HER; CLICK FLUX TO START PAID DRONE SERVICE ASSIGNMENTS."},
		},
		{
			{"LAUNCHING AN EXPEDITION", "CHOOSE A ROUTE AND SHIP IN THE SCOUT PAD. PAID MISSIONS SPEND STARBITS WHEN LAUNCHED.", "ONLY THE PASSIVE OF THE SHIP YOU ACTUALLY LAUNCH APPLIES TO THAT EXPEDITION.", "LONGER / DEEPER ROUTES TAKE MORE TIME BUT IMPROVE ACCESS TO RARER COMPONENTS AND REWARDS."},
			{"WHEN YOUR SHIP RETURNS", "A NOTIFICATION SLIDES IN FROM THE RIGHT: YOUR SHIP HAS RETURNED FROM AN EXPEDITION.", "CLICK THE NOTIFICATION TO DISMISS IT AND OPEN THE SCOUT PAD DIRECTLY.", "CLAIM THE RETURNED EXPEDITION THERE; THE REWARD IS NOT LOST IF YOU IGNORE THE NOTIFICATION FOR A WHILE."},
			{"WHAT EXPEDITIONS CAN RECOVER", "COMMON: PIXEL WIRE, STAR ALLOY, POWER CELL // AVAILABLE ON ANY ROUTE.", "RARE: QUANTUM GLASS, VOID CRYSTAL // AVAILABLE ON ANY ROUTE, WITH BETTER ODDS ON LONGER ROUTES.", "NOVA CORE // ONLY DERELICT SIGNAL OR FARTHER. STAR CACHES ARE SEPARATE BONUS FINDS."},
		},
		{
			{"PILOT SKILL POINTS // MILESTONES ONLY", "SKILL POINTS ARE RARE: YOU EARN EXACTLY ONE FOR EACH MAJOR STARBASE MILESTONE, NOT FOR ROUTINE PURCHASES.", "SOURCES INCLUDE RESEARCH TOTALS, CORE TIERS, OPERATOR LEVELS, SCOUT EXPEDITIONS, DRONE / ORBITAL TIERS AND PRESTIGE.", "THE TOP OF THE PILOT NETWORK ALWAYS SHOWS YOUR NEXT SKILL-POINT MILESTONE. THE FULL TREE NOW TAKES THE WHOLE STARBASE CAMPAIGN TO COMPLETE."},
			{"WHY A PILOT NODE CAN STILL BE LOCKED", "A NODE NEEDS ITS CONNECTED PREVIOUS NODE AND ANY PHYSICAL FACILITY THAT THE BONUS USES.", "SCOUT / NAVIGATION SKILLS REQUIRE THE SCOUT PAD. ORBITAL DRONES AND ITS BRANCH REQUIRE THE DRONE BAY.", "THIS PREVENTS YOU FROM BUYING A BONUS FOR A SYSTEM YOU HAVE NOT CONSTRUCTED YET."},
			{"READING THE PILOT NETWORK", "BACKGROUND-COLOURED BLUE LINKS ARE LOCKED. GOLD LINKS TAKE PRIORITY ON UNLOCKED / OWNED ROUTES.", "PURCHASED SKILLS STAY ACTIVE PERMANENTLY AND SPEND ONE PILOT SKILL POINT EACH.", "PAN AROUND THE NETWORK TO INSPECT LATER BRANCHES; OPENING IT AGAIN RECENTRES ON SHARPER CURSORS."},
		},
		{
			{"STARBIT BANK + RUNNING STARBASE", "STARBITS ARE THE SEPARATE CURRENCY USED INSIDE STARBASE.", "THE BANK SAVES YOUR PROGRESS, BUT STARBASE PRODUCTION ONLY RUNS WHILE STARBASE ITSELF IS OPEN.", "YOUR GLOBAL CURSOR CONTROL RANK ALSO MODIFIES PRODUCTION: " + rankName + " // " + rankBonus + "."},
			{"OVERDRIVE", "OVERDRIVE IS A TEMPORARY X2 STARBIT-OUTPUT BURST WHEN A CHARGE IS READY.", "ACTIVATE IT FROM THE STARBASE TELEMETRY CARD; THE BOOST ENDS WHEN ITS TIMER EXPIRES.", "OVERDRIVE DOES NOT REPLACE FACILITY, OPERATOR, MODULE OR PILOT BONUSES; THEY CONTINUE TO STACK NORMALLY."},
			{"STAR CACHES + LATE GAME", "STAR CACHES ARE BONUS EXPLORATION REWARDS FOUND THROUGH EXPEDITIONS.", "CACHE-CHANCE RESEARCH, PILOT SKILLS AND THE ACTIVE EXPEDITION SHIP PASSIVE CAN IMPROVE YOUR ODDS.", "LATER PROGRESSION OPENS ORBITAL EXTRACTION AND OTHER END-GAME SYSTEMS; FOLLOW THE NEXT OBJECTIVE WHEN UNSURE."},
		},
	}
	if afkGuidePage < 0 || afkGuidePage >= len(pages) {
		afkGuidePage = 0
	}
	for i, c := range pages[afkGuidePage] {
		drawGuideCard(hdc, afkGuideCardRect(w, hgt, i), c[0], c[1], c[2], c[3], w, hgt)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(112, 165, 194))
		centeredTextOut(hdc, p.Left+sx(220, w), p.Right-sx(220, w), p.Bottom-sy(38, hgt), fmt.Sprintf("PAGE %d / %d", afkGuidePage+1, len(pages)))
		selectObject.Call(hdc, old)
	}
	prev, next := afkGuidePrevRect(w, hgt), afkGuideNextRect(w, hgt)
	drawAFKActionButton(hdc, prev, "< PREVIOUS", 0, afkGuidePage > 0, pointInRect(cursorPos, prev))
	drawAFKActionButton(hdc, next, "NEXT >", 0, afkGuidePage < len(pages)-1, pointInRect(cursorPos, next))
}

func afkOpenFacilityManagement(index int) {
	afkFacilitiesPanelOpen = false
	afkGuidePanelOpen = false
	afkSystemsMenuOpen = false
	switch index {
	case afkWorldCore:
		// Cursor Core upgrades are already exposed contextually on the BASE view.
		// There is no separate management panel, so the Facilities card is status-only.
		return
	case afkWorldScout:
		afkCloseManagementPanels()
		afkExpeditionPanelOpen = true
	case afkWorldTech:
		afkCloseManagementPanels()
		afkTechPanelOpen = true
	case afkWorldOps:
		if afkOperatorsAvailable() {
			afkCloseManagementPanels()
			afkOperatorPanelOpen = true
		}
	case afkWorldModules:
		if afkModulesAvailable() {
			afkCloseManagementPanels()
			afkModulePanelOpen = true
			afkModuleSelectedItem = -1
		}
	case afkWorldDrone:
		if afkDronesAvailable() || afkDroneBayBuilt() {
			afkCloseManagementPanels()
			afkDronePanelOpen = true
			afkDroneSelectedUpgrade = 0
		}
	case afkWorldOrbital:
		if afkOrbitalAvailable() || afkOrbitalBuilt() {
			afkCloseManagementPanels()
			afkOrbitalPanelOpen = true
			afkOrbitalSelectedUpgrade = 0
		}
	}
}

func handleAFKFacilitiesHubClick(h uintptr, p FPoint, w, hgt int32) bool {
	if !afkFacilitiesPanelOpen || !pointInRect(p, afkFacilitiesPanelRect(w, hgt)) {
		return false
	}
	if afkFacilityDrawerIndex >= 0 && pointInRect(p, afkFacilityDrawerUpgradeRect(w, hgt)) {
		if afkUpgradeFacilityFromDrawer(afkFacilityDrawerIndex) {
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
		}
		return true
	}
	if afkFacilityDrawerIndex >= 0 && pointInRect(p, afkFacilityDrawerManageRect(w, hgt)) {
		idx := afkFacilityDrawerIndex
		afkFacilityDrawerIndex = -1
		afkFacilityDrawerPhase = 0
		afkOpenFacilityManagement(idx)
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	for displayIndex, worldIndex := range afkFacilityHubOrder {
		if pointInRect(p, afkFacilityHubCardRect(w, hgt, displayIndex)) {
			afkSelectFacilityDrawer(worldIndex)
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	// Clicking unused space inside FACILITIES dismisses only the slide-up drawer.
	// The main Facilities interface stays open so the player can immediately pick
	// another facility card.
	if afkFacilityDrawerIndex >= 0 {
		afkSelectFacilityDrawer(afkFacilityDrawerIndex)
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
	}
	return true
}

func handleAFKGuideClick(h uintptr, p FPoint, w, hgt int32) bool {
	if !afkGuidePanelOpen || !pointInRect(p, afkGuidePanelRect(w, hgt)) {
		return false
	}
	if pointInRect(p, afkGuidePrevRect(w, hgt)) && afkGuidePage > 0 {
		afkGuidePage--
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	if pointInRect(p, afkGuideNextRect(w, hgt)) && afkGuidePage < afkGuidePageCount-1 {
		afkGuidePage++
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	return true
}
