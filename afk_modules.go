//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Section 8 mirrors the reference game's Equipment screen: nine themed sets,
// eight crafted pieces in every set, and exploration-sourced crafting parts.
// All recipes, names and bonuses are explicitly authored.
const (
	afkModuleSetCount      = 9
	afkModuleItemsPerSet   = 8
	afkModuleItemCount     = afkModuleSetCount * afkModuleItemsPerSet
	afkCraftComponentCount = 6
)

type afkCraftComponentDef struct {
	Name, Rarity string
	Accent       uintptr
}

var afkCraftComponentDefs = [...]afkCraftComponentDef{
	{"PIXEL WIRE", "ESSENTIAL", rgb(120, 211, 255)},
	{"STAR ALLOY", "ESSENTIAL", rgb(175, 190, 210)},
	{"POWER CELL", "ESSENTIAL", rgb(255, 211, 78)},
	{"QUANTUM GLASS", "RARE", rgb(192, 110, 255)},
	{"VOID CRYSTAL", "RARE", rgb(255, 102, 188)},
	{"NOVA CORE", "EPIC", rgb(255, 116, 92)},
}

type afkModuleSetDef struct {
	Name, Bonus string
	Accent      uintptr
	Items       [8]string
}

var afkModuleSetDefs = [...]afkModuleSetDef{
	{"OPERATOR LOCKER", "OPERATOR SERVICE XP +25%", rgb(78, 218, 255), [8]string{"HOLO PLATFORM", "OPERATOR ID", "STASIS POD", "CREW CRATE", "COMMS HEADSET", "OPERATOR SEAT", "SUPPORT DRONE", "HOLO CARD"}},
	{"STARBIT GENERATOR", "ALL STARBITS +15%", rgb(255, 207, 70), [8]string{"COLLECTOR RING", "GENERATOR COIL", "STAR CRYSTAL", "ENERGY CANISTER", "REACTOR CORE", "STARBIT BARS", "STARBIT ORB", "COLLECTOR UNIT"}},
	{"TECH LAB", "RESEARCH COST -10%", rgb(193, 107, 255), [8]string{"LAB PLATFORM", "RESEARCH CHIP", "SAMPLE VIAL", "ROBOTIC ARM", "RESEARCH TERMINAL", "SCAN DISH", "QUANTUM CANISTER", "TECH CRATE"}},
	{"HYDRO BAY", "EXPEDITION TIME -10%", rgb(91, 224, 143), [8]string{"HYDRO HULL", "MINI SUB", "TURBINE FAN", "OXYGEN CANISTER", "HYDRO DRILL", "SOLAR ARRAY", "SCANNER MODULE", "SONAR DOME"}},
	{"STAR LOUNGE", "AFK CAPACITY +2 HOURS", rgb(255, 132, 181), [8]string{"LOUNGE HABITAT", "LOUNGE CHAIR", "SLEEP POD", "PLANT TERRARIUM", "REFRESHMENT UNIT", "FITNESS MODULE", "GAME CONSOLE", "JUKEBOX"}},
	{"QUANTUM ARCADE", "MANUAL EXTRACTION +40%", rgb(104, 151, 255), [8]string{"ARCADE CABINET", "RACING SEAT", "VR VISOR", "GAME CARTRIDGE", "JOYSTICK", "CLAW MACHINE", "TOKEN CHEST", "DANCE PAD"}},
	{"EXTRACTION DEPOT", "AUTO CURSOR OUTPUT +15%", rgb(255, 151, 77), [8]string{"EXTRACTOR BASE", "DRILL BIT", "CARGO CRATE", "ORE CART", "MINING MECH", "ROBOTIC CLAW", "ORE CONVEYOR", "PROCESSING CANISTER"}},
	{"VOID VAULT", "STAR CACHE REWARD +25%", rgb(156, 89, 255), [8]string{"VOID PORTAL", "VOID CRYSTAL", "VAULT CUBE", "VOID SPIRAL", "RIFT DRIVE", "CONTAINMENT TUBE", "PHASE RING", "VOID GEM"}},
	{"ORBITAL ARSENAL", "DRONE + ORBITAL OUTPUT +20%", rgb(255, 93, 100), [8]string{"DEFENCE PLATFORM", "PULSE CANNON", "ORBITAL MISSILE", "SATELLITE TURRET", "TARGETING TERMINAL", "MISSILE RACK", "ORBITAL MINE", "MISSILE BATTERY"}},
}

// Each item consumes [Wire, Alloy, Cell, Glass, Crystal, Nova Core]. Every
// recipe intentionally uses no more than three distinct material types so the
// requirement line stays readable; rarer materials are introduced as sets progress.
var afkModuleRecipes = [afkModuleItemCount][afkCraftComponentCount]int{
	{2, 1, 0, 0, 0, 0}, {1, 2, 0, 0, 0, 0}, {2, 1, 1, 0, 0, 0}, {1, 1, 2, 0, 0, 0}, {2, 2, 1, 0, 0, 0}, {2, 2, 2, 0, 0, 0}, {3, 2, 2, 0, 0, 0}, {0, 3, 3, 1, 0, 0},
	{2, 2, 2, 0, 0, 0}, {3, 2, 2, 0, 0, 0}, {2, 3, 3, 0, 0, 0}, {3, 0, 3, 1, 0, 0}, {4, 3, 0, 1, 0, 0}, {0, 4, 3, 1, 0, 0}, {4, 0, 4, 2, 0, 0}, {5, 0, 0, 2, 1, 0},
	{0, 2, 2, 1, 0, 0}, {3, 0, 2, 1, 0, 0}, {4, 3, 0, 1, 0, 0}, {0, 3, 3, 1, 0, 0}, {4, 0, 3, 2, 0, 0}, {5, 4, 0, 2, 0, 0}, {0, 5, 0, 2, 1, 0}, {0, 0, 5, 3, 1, 0},
	{3, 3, 2, 0, 0, 0}, {0, 3, 3, 1, 0, 0}, {4, 0, 3, 1, 0, 0}, {5, 4, 0, 1, 0, 0}, {0, 4, 4, 2, 0, 0}, {0, 0, 4, 2, 1, 0}, {6, 0, 0, 2, 1, 0}, {0, 6, 0, 3, 1, 0},
	{4, 0, 3, 1, 0, 0}, {4, 4, 0, 1, 0, 0}, {0, 4, 4, 1, 0, 0}, {5, 0, 4, 2, 0, 0}, {6, 0, 0, 2, 1, 0}, {0, 5, 0, 2, 1, 0}, {0, 0, 5, 3, 1, 0}, {7, 0, 0, 3, 2, 0},
	{0, 4, 3, 1, 0, 0}, {5, 0, 4, 1, 0, 0}, {5, 5, 0, 2, 0, 0}, {0, 5, 4, 2, 0, 0}, {0, 0, 5, 2, 1, 0}, {7, 0, 0, 3, 1, 0}, {0, 6, 0, 3, 1, 0}, {0, 0, 6, 4, 2, 0},
	{5, 4, 0, 1, 0, 0}, {0, 5, 4, 2, 0, 0}, {0, 0, 5, 2, 1, 0}, {6, 0, 0, 2, 1, 0}, {0, 6, 0, 3, 1, 0}, {0, 0, 6, 3, 2, 0}, {8, 0, 0, 4, 2, 0}, {0, 0, 0, 4, 2, 1},
	{5, 0, 4, 2, 0, 0}, {6, 0, 0, 2, 1, 0}, {0, 6, 0, 3, 1, 0}, {0, 0, 6, 3, 1, 0}, {8, 0, 0, 3, 2, 0}, {0, 7, 0, 4, 2, 0}, {0, 0, 0, 4, 2, 1}, {0, 0, 0, 5, 3, 1},
	{0, 5, 0, 2, 1, 0}, {0, 0, 5, 3, 1, 0}, {7, 0, 0, 3, 1, 0}, {0, 7, 0, 4, 2, 0}, {0, 0, 7, 4, 2, 0}, {0, 0, 0, 4, 2, 1}, {0, 0, 0, 5, 3, 1}, {0, 0, 0, 6, 4, 2},
}

var afkModulePanelOpen bool
var afkModuleSelectedSet int
var afkModuleSelectedItem int

func afkEnsureSection8SaveShape() {
	if len(gameMeta.AFKCraftComponents) != afkCraftComponentCount {
		old := append([]int(nil), gameMeta.AFKCraftComponents...)
		gameMeta.AFKCraftComponents = make([]int, afkCraftComponentCount)
		copy(gameMeta.AFKCraftComponents, old)
	}
	if len(gameMeta.AFKEquipmentCrafted) != afkModuleItemCount {
		old := append([]bool(nil), gameMeta.AFKEquipmentCrafted...)
		gameMeta.AFKEquipmentCrafted = make([]bool, afkModuleItemCount)
		copy(gameMeta.AFKEquipmentCrafted, old)
	}
	for i := range gameMeta.AFKCraftComponents {
		if gameMeta.AFKCraftComponents[i] < 0 {
			gameMeta.AFKCraftComponents[i] = 0
		}
	}
	if gameMeta.AFKPendingComponent < 0 || gameMeta.AFKPendingComponent >= afkCraftComponentCount {
		gameMeta.AFKPendingComponent = 0
		gameMeta.AFKPendingComponentCount = 0
	}
	if gameMeta.AFKPendingComponentCount < 0 {
		gameMeta.AFKPendingComponentCount = 0
	}
}
func afkModulesAvailable() bool        { return afkCoreTier() >= 4 && gameMeta.AFKSection7Complete }
func afkModuleIndex(set, item int) int { return set*afkModuleItemsPerSet + item }

// afkModuleDisplayName is deliberately sprite-bound. The visible item name is
// derived from the exact authored sprite filename at the same save/index slot,
// preventing labels and artwork from drifting apart again.
func afkModuleDisplayName(set, item int) string {
	idx := afkModuleIndex(set, item)
	if idx < 0 || idx >= len(afkModuleSprites) {
		if set >= 0 && set < len(afkModuleSetDefs) && item >= 0 && item < len(afkModuleSetDefs[set].Items) {
			return afkModuleSetDefs[set].Items[item]
		}
		return "MODULE"
	}
	base := strings.TrimSuffix(filepath.Base(afkModuleSprites[idx].file), filepath.Ext(afkModuleSprites[idx].file))
	if cut := strings.IndexByte(base, '_'); cut >= 0 && cut+1 < len(base) {
		base = base[cut+1:]
	}
	base = strings.ReplaceAll(base, "_", " ")
	return strings.ToUpper(base)
}
func afkModuleCrafted(set, item int) bool {
	afkEnsureSection8SaveShape()
	i := afkModuleIndex(set, item)
	return i >= 0 && i < len(gameMeta.AFKEquipmentCrafted) && gameMeta.AFKEquipmentCrafted[i]
}
func afkModuleSetProgress(set int) int {
	n := 0
	for i := 0; i < 8; i++ {
		if afkModuleCrafted(set, i) {
			n++
		}
	}
	return n
}
func afkModuleSetComplete(set int) bool { return afkModuleSetProgress(set) >= 8 }
func afkAllModuleSetsRewardAvailable() bool {
	return afkModuleSetsComplete() >= 9 && !gameMeta.AFKEquipmentAllSetsReward
}
func afkClaimAllModuleSetsReward() bool {
	if !afkActionAllowed("claim_all_modules", time.Now(), afkClaimActionDebounce) || !afkAllModuleSetsRewardAvailable() {
		return false
	}
	gameMeta.AFKNavigationData += 500
	gameMeta.AFKEquipmentAllSetsReward = true
	return true
}
func afkModuleSetsComplete() int {
	n := 0
	for s := 0; s < 9; s++ {
		if afkModuleSetComplete(s) {
			n++
		}
	}
	return n
}
func afkCanCraftModule(set, item int) bool {
	if !afkModulesAvailable() || afkModuleCrafted(set, item) {
		return false
	}
	r := afkModuleRecipes[afkModuleIndex(set, item)]
	for i, v := range r {
		if gameMeta.AFKCraftComponents[i] < v {
			return false
		}
	}
	return true
}
func afkCraftModule(set, item int) bool {
	if !afkActionAllowed(fmt.Sprintf("craft_module_%d_%d", set, item), time.Now(), afkEconomyActionDebounce) || !afkCanCraftModule(set, item) {
		return false
	}
	r := afkModuleRecipes[afkModuleIndex(set, item)]
	for i, v := range r {
		gameMeta.AFKCraftComponents[i] -= v
	}
	gameMeta.AFKEquipmentCrafted[afkModuleIndex(set, item)] = true
	afkRefreshSection8Completion()
	playAFKModuleCraftedSound()
	return true
}
func afkRefreshSection8Completion() {
	if len(gameMeta.AFKEquipmentCrafted) != afkModuleItemCount {
		return
	}
	n := 0
	for s := 0; s < 9; s++ {
		ok := true
		for i := 0; i < 8; i++ {
			if !gameMeta.AFKEquipmentCrafted[afkModuleIndex(s, i)] {
				ok = false
				break
			}
		}
		if ok {
			n++
		}
	}
	gameMeta.AFKEquipmentSetsComplete = n
	if n >= 3 {
		gameMeta.AFKSection8Complete = true
	}
}

func afkModuleProductionMultiplierMilli() int64 {
	if afkModuleSetComplete(1) {
		return 1150
	}
	return 1000
}
func afkModuleAutoCursorMultiplierMilli() int64 {
	if afkModuleSetComplete(6) {
		return 1150
	}
	return 1000
}
func afkModuleResearchCostMultiplierMilli() int64 {
	if afkModuleSetComplete(2) {
		return 900
	}
	return 1000
}
func afkModuleExpeditionMultiplierMilli() int64 {
	if afkModuleSetComplete(3) {
		return 900
	}
	return 1000
}
func afkModuleCacheRewardMultiplierMilli() int64 {
	if afkModuleSetComplete(7) {
		return 1250
	}
	return 1000
}
func afkModuleOperatorXPMultiplierMilli() int64 {
	if afkModuleSetComplete(0) {
		return 1250
	}
	return 1000
}
func afkModuleManualMultiplierMilli() int64 {
	if afkModuleSetComplete(5) {
		return 1400
	}
	return 1000
}
func afkModuleCapacityBonusSeconds() int64 {
	if afkModuleSetComplete(4) {
		return 2 * 60 * 60
	}
	return 0
}
func afkModuleDroneOrbitalMultiplierMilli() int64 {
	if afkModuleSetComplete(8) {
		return 1200
	}
	return 1000
}

func afkDeterministicCraftDrop(serial, destination int) (int, int) {
	// Every completed expedition returns a component, like the Yarn/Flask/Duct Tape
	// reward cadence visible in the reference. Longer routes bias toward rare parts.
	x := uint32(serial*1664525 + destination*1013904223 + 0xC8A7)
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	roll := int(x % 1000)
	idx := 0
	if destination >= 4 && roll < 70 {
		idx = 5
	} else if roll < 240+destination*25 {
		idx = 3 + int((x/97)%2)
	} else {
		idx = int((x / 41) % 3)
	}
	count := 1
	if destination >= 2 && int((x/313)%100) < 35 {
		count = 2
	}
	if destination >= 5 && int((x/719)%100) < 20 {
		count = 3
	}
	return idx, count
}
func afkAwardPendingCraftComponent() {
	afkEnsureSection8SaveShape()
	if !afkModulesAvailable() || gameMeta.AFKPendingComponentCount > 0 {
		return
	}
	i, n := afkDeterministicCraftDrop(gameMeta.AFKExpeditionSerial, gameMeta.AFKExpeditionDestination)
	gameMeta.AFKPendingComponent = i
	gameMeta.AFKPendingComponentCount = n
}
func afkCollectPendingCraftComponent() bool {
	afkEnsureSection8SaveShape()
	if gameMeta.AFKPendingComponentCount <= 0 {
		return false
	}
	i := gameMeta.AFKPendingComponent
	n := gameMeta.AFKPendingComponentCount
	gameMeta.AFKCraftComponents[i] += n
	gameMeta.AFKCraftComponentsFound += n
	gameMeta.AFKPendingComponentCount = 0
	return true
}

func afkClaimCraftComponent() bool {
	afkEnsureSection8SaveShape()
	if !afkActionAllowed("claim_craft_component", time.Now(), afkClaimActionDebounce) || gameMeta.AFKPendingComponentCount <= 0 {
		return false
	}
	return afkCollectPendingCraftComponent()
}

// v461 reward preview: show the exact deterministic crafting material before the
// player claims the expedition. Older saves with an already-staged component
// keep their persisted reward instead of rolling anything again.
func afkPendingExpeditionCraftComponent() (int, int, bool) {
	afkEnsureSection8SaveShape()
	if !afkModulesAvailable() {
		return 0, 0, false
	}
	if gameMeta.AFKPendingComponentCount > 0 {
		return gameMeta.AFKPendingComponent, gameMeta.AFKPendingComponentCount, true
	}
	if !afkExpeditionRewardPending() {
		return 0, 0, false
	}
	i, n := afkDeterministicCraftDrop(gameMeta.AFKExpeditionSerial, gameMeta.AFKExpeditionDestination)
	return i, n, n > 0
}

func afkSection8Objective() (string, string, string, int64, int64, bool) {
	if !afkModulesAvailable() {
		return "SHIP MODULES LOCKED", "COMPLETE OPERATOR DEVELOPMENT", "CURSOR CORE TIER 4 REQUIRED", int64(afkModuleSetsComplete()), 3, false
	}
	sets := afkModuleSetsComplete()
	if gameMeta.AFKCraftComponentsFound < 1 {
		return "RECOVER A CRAFTING COMPONENT", "COMPLETE A SPACE EXPEDITION", "COMPONENTS BUILD SHIP MODULES", int64(gameMeta.AFKCraftComponentsFound), 1, false
	}
	if sets < 1 {
		crafted := 0
		for _, v := range gameMeta.AFKEquipmentCrafted {
			if v {
				crafted++
			}
		}
		return "COMPLETE YOUR FIRST MODULE SET", "CRAFT ALL 8 PIECES IN ONE SET", fmt.Sprintf("CRAFTED %d / 72 TOTAL PIECES", crafted), int64(sets), 1, false
	}
	if sets < 3 {
		return "COMPLETE 3 MODULE SETS", "EACH COMPLETED SET GRANTS A PERMANENT BONUS", "FULL COLLECTION: 9 SETS x 8 PIECES", int64(sets), 3, false
	}
	return "SECTION 8 COMPLETE", "SHIP MODULE NETWORK ONLINE", fmt.Sprintf("COLLECTION %d / 9 SETS", sets), int64(sets), 3, true
}

func afkModulePanelRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }
func afkModuleSetRect(w, hgt int32, i int) RECT {
	p := afkModulePanelRect(w, hgt)
	gap := sx(10, w)
	cw := (p.Right - p.Left - sx(40, w) - gap*2) / 3
	ch := (p.Bottom - p.Top - sy(156, hgt) - sy(12, hgt)*2) / 3
	col, row := i%3, i/3
	x := p.Left + sx(20, w) + int32(col)*(cw+gap)
	y := p.Top + sy(142, hgt) + int32(row)*(ch+sy(12, hgt))
	return RECT{x, y, x + cw, y + ch}
}
func afkModuleItemRect(w, hgt int32, i int) RECT {
	p := afkModulePanelRect(w, hgt)
	gap := sx(12, w)
	cw := (p.Right - p.Left - sx(80, w) - gap*3) / 4
	ch := (p.Bottom - p.Top - sy(190, hgt) - sy(14, hgt)) / 2
	col, row := i%4, i/4
	x := p.Left + sx(40, w) + int32(col)*(cw+gap)
	y := p.Top + sy(152, hgt) + int32(row)*(ch+sy(14, hgt))
	return RECT{x, y, x + cw, y + ch}
}

func drawAFKModuleIcon(hdc uintptr, r RECT, set, item int, crafted bool, w, hgt int32) {
	// v463: each authored sprite is bound to the matching row/column item from the 9x8 source grid.
	// Reserve the lower strip for the item name / crafted state and fit the art
	// proportionally into the remaining card area.
	art := RECT{
		r.Left + sx(12, w),
		r.Top + sy(18, hgt),
		r.Right - sx(12, w),
		r.Bottom - sy(48, hgt),
	}
	if drawAFKModuleSprite(hdc, art, set, item, crafted) {
		return
	}

	// Safe fallback if a distribution is missing an art asset.
	cx := (r.Left + r.Right) / 2
	cy := r.Top + sy(44, hgt)
	c := afkModuleSetDefs[set].Accent
	if !crafted {
		c = rgb(31, 43, 58)
	}
	fillSolidRect(hdc, RECT{cx - sx(18, w), cy - sy(14, hgt), cx + sx(18, w), cy + sy(14, hgt)}, c)
	fillSolidRect(hdc, RECT{cx - sx(10, w), cy - sy(22, hgt), cx + sx(10, w), cy + sy(22, hgt)}, c)
	if crafted {
		fillSolidRect(hdc, RECT{cx - sx(4, w), cy - sy(4, hgt), cx + sx(4, w), cy + sy(4, hgt)}, rgb(240, 249, 255))
	}
}
func drawAFKModuleMaterialStrip(hdc uintptr, p RECT, w, hgt int32) {
	afkEnsureSection8SaveShape()
	left := p.Left + sx(22, w)
	right := p.Right - sx(22, w)
	top := p.Top + sy(72, hgt)
	bottom := p.Top + sy(130, hgt)
	gap := sx(7, w)
	cellW := (right - left - gap*5) / 6
	for i, d := range afkCraftComponentDefs {
		x := left + int32(i)*(cellW+gap)
		r := RECT{x, top, x + cellW, bottom}
		face := rgb(5, 22, 42)
		drawBevelPanel(hdc, r, face, blendColor(d.Accent, rgb(255, 255, 255), 0.08), rgb(0, 5, 14), 1)
		icon := RECT{r.Left + sx(5, w), r.Top + sy(5, hgt), r.Left + sx(43, w), r.Bottom - sy(5, hgt)}
		drawAFKCraftMaterialSprite(hdc, icon, i, 255)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, d.Accent)
			nameLeft := r.Left + sx(47, w)
			textOut(hdc, nameLeft, r.Top+sy(9, hgt), fitTextEllipsis(hdc, hudTinyFont, d.Name, r.Right-nameLeft-sx(4, w)))
			setTextColor.Call(hdc, rgb(238, 246, 251))
			textOut(hdc, nameLeft, r.Top+sy(30, hgt), fmt.Sprintf("x%d", gameMeta.AFKCraftComponents[i]))
			selectObject.Call(hdc, old)
		}
	}
}

func drawAFKModules(hdc uintptr, w, hgt int32) {
	if !afkModulePanelOpen || !afkModulesAvailable() {
		return
	}
	afkEnsureSection8SaveShape()
	p := afkModulePanelRect(w, hgt)
	drawAFKManagementChrome(hdc, p, "SHIP MODULES", "", rgb(86, 115, 168), w, hgt)
	drawAFKModuleMaterialStrip(hdc, p, w, hgt)
	if afkModuleSelectedSet < 0 || afkModuleSelectedSet >= 9 {
		afkModuleSelectedSet = 0
	}
	// Set overview unless an item detail set has been entered via selected item >= 0.
	if afkModuleSelectedItem < 0 {
		for s, dfn := range afkModuleSetDefs {
			r := afkModuleSetRect(w, hgt, s)
			border := dfn.Accent
			if afkModuleSetComplete(s) {
				border = rgb(72, 224, 132)
			}
			drawBevelPanel(hdc, r, rgb(5, 25, 48), border, rgb(0, 5, 14), 2)
			if hudTinyFont != 0 {
				old, _, _ := selectObject.Call(hdc, hudTinyFont)
				setBkMode.Call(hdc, TRANSPARENT)
				setTextColor.Call(hdc, rgb(244, 248, 252))
				centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(15, hgt), dfn.Name)
				setTextColor.Call(hdc, dfn.Accent)
				centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(41, hgt), fmt.Sprintf("%d / 8", afkModuleSetProgress(s)))
				setTextColor.Call(hdc, rgb(132, 168, 195))
				centeredTextOut(hdc, r.Left+sx(5, w), r.Right-sx(5, w), r.Top+sy(68, hgt), dfn.Bonus)
				selectObject.Call(hdc, old)
			}
		}
		return
	}
	s := afkModuleSelectedSet
	dfn := afkModuleSetDefs[s]
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, dfn.Accent)
		centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(134, hgt), fmt.Sprintf("%s // %d/8 // %s", dfn.Name, afkModuleSetProgress(s), dfn.Bonus))
		selectObject.Call(hdc, old)
	}
	for i := range dfn.Items {
		name := afkModuleDisplayName(s, i)
		r := afkModuleItemRect(w, hgt, i)
		crafted := afkModuleCrafted(s, i)
		border := rgb(52, 74, 98)
		if crafted {
			border = dfn.Accent
		}
		if i == afkModuleSelectedItem {
			border = rgb(246, 239, 255)
		}
		drawBevelPanel(hdc, r, rgb(5, 22, 42), border, rgb(0, 5, 14), 2)
		drawAFKModuleIcon(hdc, r, s, i, crafted, w, hgt)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, func() uintptr {
				if crafted {
					return rgb(241, 248, 252)
				}
				return rgb(91, 110, 129)
			}())
			centeredTextOut(hdc, r.Left+sx(3, w), r.Right-sx(3, w), r.Bottom-sy(31, hgt), name)
			if crafted {
				setTextColor.Call(hdc, rgb(72, 224, 132))
				centeredTextOut(hdc, r.Left, r.Right, r.Bottom-sy(13, hgt), "CRAFTED")
			}
			selectObject.Call(hdc, old)
		}
	}
}

func afkModuleRecipeText(set, item int) string {
	r := afkModuleRecipes[afkModuleIndex(set, item)]
	out := ""
	for i, n := range r {
		if n <= 0 {
			continue
		}
		if out != "" {
			out += "  •  "
		}
		out += fmt.Sprintf("%s %d/%d", afkCraftComponentDefs[i].Name, gameMeta.AFKCraftComponents[i], n)
	}
	return out
}
