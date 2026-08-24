//go:build windows

package main

import (
	"fmt"
	"time"
)

// Section 6: Tech Lab + Pilot Skills. All costs, levels and multipliers are
// explicit tables to preserve the hard-coded progression design.
const (
	afkTechLabUnlockCost int64 = 12_000_000
	afkResearchMaxLevel        = 10
)

type afkResearchDef struct {
	Name        string
	Description string
	Effect      string
	Costs       [10]int64 // index current level, cost to next
}

var afkResearchDefs = [...]afkResearchDef{
	{"VOID CURSOR", "Sharper orbital routines increase Auto Cursor extraction.", "AUTO CURSOR OUTPUT", [10]int64{75_000, 160_000, 340_000, 720_000, 1_500_000, 3_100_000, 6_500_000, 13_500_000, 28_000_000, 60_000_000}},
	{"BLACKSTONE ROUTING", "Compresses the route between the Cursor Core and Singularity.", "CORE OUTPUT", [10]int64{90_000, 190_000, 400_000, 850_000, 1_800_000, 3_800_000, 8_000_000, 17_000_000, 35_000_000, 75_000_000}},
	{"SNARE ALGORITHM", "Makes manual Singularity extraction hit harder.", "MANUAL EXTRACTION", [10]int64{105_000, 220_000, 470_000, 1_000_000, 2_100_000, 4_400_000, 9_200_000, 19_000_000, 40_000_000, 85_000_000}},
	{"SINGULARITY POINTER", "Improves Scout Ship path prediction and travel time.", "EXPEDITION SPEED", [10]int64{125_000, 270_000, 580_000, 1_220_000, 2_600_000, 5_500_000, 11_500_000, 24_000_000, 50_000_000, 105_000_000}},
	{"STEALTH PATHING", "Reduces interference and improves Star Cache detection.", "CACHE CHANCE", [10]int64{145_000, 310_000, 660_000, 1_400_000, 2_950_000, 6_200_000, 13_000_000, 27_000_000, 56_000_000, 118_000_000}},
	{"PIXEL COMPRESSION FIELD", "Stores more AFK extraction before the network saturates.", "AFK CAPACITY", [10]int64{175_000, 370_000, 790_000, 1_670_000, 3_500_000, 7_400_000, 15_500_000, 32_000_000, 67_000_000, 140_000_000}},
	{"CURSOR PRISM", "Splits extraction cycles into additional productive passes.", "ALL STARBITS", [10]int64{210_000, 450_000, 950_000, 2_000_000, 4_200_000, 8_900_000, 18_500_000, 38_500_000, 80_000_000, 165_000_000}},
	{"OVERDRIVE REGULATORS", "Keeps Auto Cursors stable while operating above nominal output.", "AUTO CURSOR OUTPUT", [10]int64{250_000, 530_000, 1_120_000, 2_360_000, 5_000_000, 10_500_000, 22_000_000, 46_000_000, 96_000_000, 200_000_000}},
	{"QUANTUM CONDUIT", "Routes excess Starbits back through the Core.", "CORE OUTPUT", [10]int64{300_000, 640_000, 1_350_000, 2_850_000, 6_000_000, 12_600_000, 26_500_000, 55_000_000, 115_000_000, 240_000_000}},
	{"CONVERGENCE", "Synchronises every extraction subsystem around one timing pulse.", "ALL STARBITS", [10]int64{360_000, 770_000, 1_620_000, 3_420_000, 7_200_000, 15_200_000, 32_000_000, 67_000_000, 140_000_000, 290_000_000}},
	// Drone-era research appears only once the Drone Bay is built, mirroring the
	// late Research Lab rows visible after the reference game's Drone unlock.
	{"OVERCHARGED CORES", "Feeds unstable Starbits directly into Drone power cores.", "DRONE OUTPUT", [10]int64{8_000_000_000, 16_000_000_000, 32_000_000_000, 64_000_000_000, 128_000_000_000, 256_000_000_000, 512_000_000_000, 1_000_000_000_000, 2_000_000_000_000, 4_000_000_000_000}},
	{"NEURAL SYNC", "Lets every Drone share target solutions in real time.", "DRONE OUTPUT", [10]int64{10_000_000_000, 20_000_000_000, 40_000_000_000, 80_000_000_000, 160_000_000_000, 320_000_000_000, 640_000_000_000, 1_280_000_000_000, 2_560_000_000_000, 5_120_000_000_000}},
	{"ENGINEER TRAINING", "Prepares the crew for the future Orbital Extractor firing cycle.", "ORBITAL SPEED", [10]int64{12_000_000_000, 24_000_000_000, 48_000_000_000, 96_000_000_000, 192_000_000_000, 384_000_000_000, 768_000_000_000, 1_536_000_000_000, 3_072_000_000_000, 6_144_000_000_000}},
	{"OMEGA PROTOCOL", "Coordinates high-density Drone swarms around one extraction pulse.", "DRONE OUTPUT", [10]int64{15_000_000_000, 30_000_000_000, 60_000_000_000, 120_000_000_000, 240_000_000_000, 480_000_000_000, 960_000_000_000, 1_920_000_000_000, 3_840_000_000_000, 7_680_000_000_000}},
	{"VOID PIERCERS", "Focuses future orbital beams through the Singularity boundary.", "ORBITAL OUTPUT", [10]int64{18_000_000_000, 36_000_000_000, 72_000_000_000, 144_000_000_000, 288_000_000_000, 576_000_000_000, 1_152_000_000_000, 2_304_000_000_000, 4_608_000_000_000, 9_216_000_000_000}},
}

type afkTalentDef struct {
	Name     string
	Flavour  string
	Effect   string
	Requires int // talent index, -1 root
}

var afkTalentDefs = [...]afkTalentDef{
	{"SHARPER CURSORS", "Your Auto Cursors hit the gym and got pixel-swole.", "AUTO CURSOR OUTPUT +10%", -1},
	{"ORBITAL CURSOR MATH 2.0", "The Core finally learned to count past one orbit.", "CURSOR PROCESSING +10%", 0},
	{"TAP PRECISION I", "Every manual extraction lands exactly where it matters.", "MANUAL EXTRACTION +25%", 0},
	{"DEEP SPACE NAVIGATOR", "The Scout Ship stops taking the scenic route.", "EXPEDITION TIME -10%", 1},
	{"CACHE HUNTER", "Signal noise starts looking suspiciously like treasure.", "STAR CACHE REWARD +20%", 3},
	{"CURSOR SUPREMACY", "Every extraction subsystem shares one perfected cycle.", "ALL STARBITS +10%", 1},
	{"OVERDRIVE CHARGE", "Stabilised field coils keep Overdrive burning longer.", "OVERDRIVE DURATION +15%", 5},
	{"OVERDRIVE EXTENSION", "A second regulator stage stretches every Overdrive window.", "OVERDRIVE DURATION +15%", 6},
	{"NAV DATA ANALYSIS", "Scout telemetry is converted into more useful navigation data.", "EXPEDITION NAV DATA +10%", 3},
	{"SIGNAL INTELLIGENCE", "Better signal filtering exposes hidden Star Cache signatures.", "STAR CACHE CHANCE +5%", 8},
	{"RAPID DISPATCH", "Launch routines are precomputed before the Scout Ship departs.", "EXPEDITION TIME -5%", 3},
	{"MAGNETIC ATTRACTOR", "Recovered cache fragments are pulled in before they drift away.", "STAR CACHE REWARD +10%", 4},
	{"ORBITAL DRONES", "Pilot telemetry lets Drone swarms extract more efficiently.", "DRONE OUTPUT +10%", 5},
	{"STEALTH PROTOCOL", "Low-signature routing makes rare cache encounters more likely.", "STAR CACHE CHANCE +5%", 12},
}

var (
	afkTechPanelOpen        bool
	afkTalentPanelOpen      bool
	afkResearchSelected     int
	afkTalentSelected       int
	afkTechScroll           int32
	afkTechScrollDragging   bool
	afkTechScrollDragOffset int32
	afkTalentInfoOpen       bool
	afkTalentInfoStarted    time.Time
	afkTalentPanX           int32
	afkTalentPanY           int32
	afkTalentPanDragging    bool
	afkTalentPanLastX       int32
	afkTalentPanLastY       int32
)

func afkEnsureSection6SaveShape() {
	if len(gameMeta.AFKResearchLevels) != len(afkResearchDefs) {
		old := append([]int(nil), gameMeta.AFKResearchLevels...)
		gameMeta.AFKResearchLevels = make([]int, len(afkResearchDefs))
		copy(gameMeta.AFKResearchLevels, old)
	}
	for i := range gameMeta.AFKResearchLevels {
		if gameMeta.AFKResearchLevels[i] < 0 {
			gameMeta.AFKResearchLevels[i] = 0
		}
		if gameMeta.AFKResearchLevels[i] > afkResearchMaxLevel {
			gameMeta.AFKResearchLevels[i] = afkResearchMaxLevel
		}
	}
}

func afkTechLabAvailable() bool { return afkCoreTier() >= 3 && gameMeta.AFKSection5Complete }
func afkTechLabUnlocked() bool  { return gameMeta.AFKTechLabUnlocked }

func afkResearchLevel(i int) int {
	afkEnsureSection6SaveShape()
	if i < 0 || i >= len(gameMeta.AFKResearchLevels) {
		return 0
	}
	return gameMeta.AFKResearchLevels[i]
}
func afkResearchCost(i int) int64 {
	lvl := afkResearchLevel(i)
	if i < 0 || i >= len(afkResearchDefs) || lvl >= afkResearchMaxLevel {
		return 0
	}
	base := afkResearchDefs[i].Costs[lvl]
	base = base * afkOperatorResearchCostMultiplierMilli() / 1000
	return base * afkModuleResearchCostMultiplierMilli() / 1000
}
func afkBuyResearch(i int) bool {
	if !afkTechLabUnlocked() || i < 0 || i >= len(afkResearchDefs) {
		return false
	}
	if i >= 10 && !afkDroneBayBuilt() {
		return false
	}
	cost := afkResearchCost(i)
	if cost <= 0 || gameMeta.AFKStarbits < cost {
		return false
	}
	if !afkActionAllowed(fmt.Sprintf("buy_research_%d", i), time.Now(), afkEconomyActionDebounce) || !afkSpendStarbits(cost) {
		return false
	}
	gameMeta.AFKResearchLevels[i]++
	gameMeta.AFKResearchPurchases++
	afkRefreshTalentPointAwards()
	afkRefreshSection6Completion()
	playAFKResearchSound()
	return true
}

func afkResearchTotalLevels() int {
	afkEnsureSection6SaveShape()
	total := 0
	for _, v := range gameMeta.AFKResearchLevels {
		total += v
	}
	return total
}

// v434 economy balance: Tech Lab is a support layer, not the main source of
// Starbit power. All six production researches together cap at +30% at Lv.10.
// Existing research levels are preserved; only their effect curve is rebalanced.
func afkResearchStarbitMultiplierMilli() int64 {
	if !afkTechLabUnlocked() {
		return 1000
	}
	total := int64(1000)
	total += int64(afkResearchLevel(0)) * 5 // +0.5%/level, +5% max
	total += int64(afkResearchLevel(1)) * 4 // +0.4%/level, +4% max
	total += int64(afkResearchLevel(6)) * 5 // +0.5%/level, +5% max
	total += int64(afkResearchLevel(7)) * 4 // +0.4%/level, +4% max
	total += int64(afkResearchLevel(8)) * 4 // +0.4%/level, +4% max
	total += int64(afkResearchLevel(9)) * 8 // +0.8%/level, +8% max
	return total
}
func afkResearchManualMultiplierMilli() int64 {
	return 1000 + int64(afkResearchLevel(2))*40
}
func afkResearchExpeditionMultiplierMilli() int64 {
	// 1.5% faster per level, capped at -15%.
	v := int64(1000 - afkResearchLevel(3)*15)
	if v < 850 {
		v = 850
	}
	return v
}
func afkResearchCacheBonusPercent() int      { return afkResearchLevel(4) }
func afkResearchCapacityBonusSeconds() int64 { return int64(afkResearchLevel(5)) * 30 * 60 }
func afkVisibleResearchCount() int {
	// v436: the Tech Lab presents the ten core research programmes only.
	// Legacy late-game research values remain in saves for compatibility.
	return 10
}
func afkAdvancedDroneResearchMultiplierMilli() int64 {
	if !afkDroneBayBuilt() {
		return 1000
	}
	return 1000 + int64(afkResearchLevel(10))*10 + int64(afkResearchLevel(11))*10 + int64(afkResearchLevel(13))*10
}
func afkOrbitalResearchSpeedMultiplierMilli() int64 {
	v := int64(1000 - afkResearchLevel(12)*15)
	if v < 850 {
		v = 850
	}
	return v
}
func afkOrbitalResearchOutputMultiplierMilli() int64 {
	return 1000 + int64(afkResearchLevel(14))*30
}

func afkTalentUnlocked(i int) bool {
	for _, v := range gameMeta.AFKTalentsUnlocked {
		if v == i {
			return true
		}
	}
	return false
}
func afkTalentAvailable(i int) bool {
	if !afkTechLabUnlocked() || i < 0 || i >= len(afkTalentDefs) {
		return false
	}
	req := afkTalentDefs[i].Requires
	parentReady := req < 0 || afkTalentUnlocked(req)
	return parentReady && afkTalentFacilityRequirementMet(i)
}

// Pilot skills can depend on the physical Starbase system they improve. This
// prevents the tree from granting bonuses to facilities the player has not
// actually constructed yet (for example ORBITAL DRONES before the Drone Bay).
// Existing purchased skills remain grandfathered by afkTalentUnlocked.
func afkTalentFacilityRequirementMet(i int) bool {
	switch i {
	case 3, 4, 8, 9, 10, 11: // navigation / cache branch
		return afkScoutUnlocked()
	case 12, 13: // ORBITAL DRONES branch
		return afkDroneBayBuilt()
	default:
		return true
	}
}

func afkTalentFacilityRequirementText(i int) string {
	switch i {
	case 3, 4, 8, 9, 10, 11:
		return "BUILD SCOUT PAD"
	case 12, 13:
		return "BUILD DRONE BAY"
	default:
		return ""
	}
}

type afkTalentPointMilestone struct {
	Name string
	Done func() bool
}

func afkHighestOperatorLevel() int {
	best := 0
	for i := range afkOperatorDefs {
		if lvl := afkOperatorLevel(i); lvl > best {
			best = lvl
		}
	}
	return best
}

// v443: Pilot Skill Points are deliberately scarce, milestone rewards. The tree
// has fourteen nodes and no longer fills simply because the player bought lots
// of inexpensive Tech Lab levels. Completing the entire network now spans the
// whole Starbase campaign and reaches into prestige progression.
func afkTalentPointMilestones() []afkTalentPointMilestone {
	return []afkTalentPointMilestone{
		{"BUILD THE TECH LAB", func() bool { return afkTechLabUnlocked() }},
		{"REACH 20 TOTAL RESEARCH LEVELS", func() bool { return afkResearchTotalLevels() >= 20 }},
		{"UPGRADE CURSOR CORE TO TIER 4", func() bool { return afkCoreTier() >= 4 }},
		{"RECRUIT 3 OPERATORS", func() bool { afkEnsureSection7SaveShape(); return gameMeta.AFKOperatorsRecruitedCount >= 3 }},
		{"REACH 45 TOTAL RESEARCH LEVELS", func() bool { return afkResearchTotalLevels() >= 45 }},
		{"LEVEL ANY OPERATOR TO 4", func() bool { return afkHighestOperatorLevel() >= 4 }},
		{"SCOUT TIER 2 + 8 EXPEDITIONS", func() bool { return afkScoutTier() >= 2 && gameMeta.AFKExpeditionsCompleted >= 8 }},
		{"UPGRADE CURSOR CORE TO TIER 5", func() bool { return afkCoreTier() >= 5 }},
		{"REACH 70 TOTAL RESEARCH LEVELS", func() bool { return afkResearchTotalLevels() >= 70 }},
		{"BUILD DRONE BAY TIER 2", func() bool { return afkDroneBayTier() >= 2 }},
		{"LEVEL ANY OPERATOR TO 7", func() bool { return afkHighestOperatorLevel() >= 7 }},
		{"SCOUT TIER 3 + 20 EXPEDITIONS", func() bool { return afkScoutTier() >= 3 && gameMeta.AFKExpeditionsCompleted >= 20 }},
		{"BUILD ORBITAL EXTRACTOR TIER 2", func() bool { return afkOrbitalTier() >= 2 }},
		{"REACH STELLAR PRESTIGE 3", func() bool { return afkPrestigeRank() >= 3 }},
	}
}

func afkTalentMilestonesCompleted() int {
	if !afkTechLabUnlocked() {
		return 0
	}
	n := 0
	for _, m := range afkTalentPointMilestones() {
		if m.Done != nil && m.Done() {
			n++
		}
	}
	return n
}

func afkTalentNextPointText() string {
	if !afkTechLabUnlocked() {
		return "NEXT SKILL POINT // BUILD THE TECH LAB"
	}
	for _, m := range afkTalentPointMilestones() {
		if m.Done != nil && !m.Done() {
			return "NEXT SKILL POINT // " + m.Name
		}
	}
	return "ALL SKILL-POINT MILESTONES COMPLETE"
}

func afkTalentPointSourceSummary() string {
	return "EARN 1 POINT PER MAJOR MILESTONE // RESEARCH // OPERATORS // EXPEDITIONS // FACILITIES // PRESTIGE"
}

func afkTalentPointsAvailable() int {
	// Refresh here as well as on purchase events so imported/cloud saves immediately
	// show the correct milestone total when the Pilot Network is opened.
	afkRefreshTalentPointAwards()
	n := gameMeta.AFKTalentPointsEarned - gameMeta.AFKTalentPointsSpent
	if n < 0 {
		return 0
	}
	return n
}

func afkRefreshTalentPointAwards() {
	earned := afkTalentMilestonesCompleted()
	// Skills purchased on earlier versions are never revoked. Any oversized pool of
	// unspent legacy points is intentionally removed by the v443 rebalance, which is
	// what makes future Pilot progression scarce again without damaging owned nodes.
	if earned < gameMeta.AFKTalentPointsSpent {
		earned = gameMeta.AFKTalentPointsSpent
	}
	if earned != gameMeta.AFKTalentPointsEarned {
		gameMeta.AFKTalentPointsEarned = earned
	}
}
func afkBuyTalent(i int) bool {
	if !afkActionAllowed(fmt.Sprintf("buy_talent_%d", i), time.Now(), afkEconomyActionDebounce) || !afkTalentAvailable(i) || afkTalentUnlocked(i) || afkTalentPointsAvailable() <= 0 {
		return false
	}
	gameMeta.AFKTalentsUnlocked = append(gameMeta.AFKTalentsUnlocked, i)
	gameMeta.AFKTalentPointsSpent++
	afkRefreshSection6Completion()
	return true
}

func afkTalentStarbitMultiplierMilli() int64 {
	m := int64(1000)
	if afkTalentUnlocked(0) {
		m += 100
	}
	if afkTalentUnlocked(1) {
		m += 100
	}
	if afkTalentUnlocked(5) {
		m += 100
	}
	return m
}
func afkTalentManualMultiplierMilli() int64 {
	if afkTalentUnlocked(2) {
		return 1250
	}
	return 1000
}
func afkTalentExpeditionMultiplierMilli() int64 {
	m := int64(1000)
	if afkTalentUnlocked(3) {
		m -= 100
	}
	if afkTalentUnlocked(10) {
		m -= 50
	}
	if m < 800 {
		m = 800
	}
	return m
}
func afkTalentCacheRewardMultiplierMilli() int64 {
	m := int64(1000)
	if afkTalentUnlocked(4) {
		m += 200
	}
	if afkTalentUnlocked(11) {
		m += 100
	}
	return m
}
func afkTalentNavRewardMultiplierMilli() int64 {
	if afkTalentUnlocked(8) {
		return 1100
	}
	return 1000
}
func afkTalentCacheChanceBonusPercent() int {
	bonus := 0
	if afkTalentUnlocked(9) {
		bonus += 5
	}
	if afkTalentUnlocked(13) {
		bonus += 5
	}
	return bonus
}
func afkTalentOverdriveDurationMultiplierMilli() int64 {
	m := int64(1000)
	if afkTalentUnlocked(6) {
		m += 150
	}
	if afkTalentUnlocked(7) {
		m += 150
	}
	return m
}
func afkTalentDroneMultiplierMilli() int64 {
	if afkTalentUnlocked(12) {
		return 1100
	}
	return 1000
}

func afkRefreshSection6Completion() {
	if afkTechLabUnlocked() && afkResearchTotalLevels() >= 12 && afkTalentUnlocked(0) && afkTalentUnlocked(1) {
		gameMeta.AFKSection6Complete = true
	}
}

func afkSection6Objective() (string, string, string, int64, int64, bool) {
	if !afkTechLabAvailable() {
		return "TECH LAB LOCKED", "REACH CURSOR CORE TIER 3", "AND COMPLETE SCOUT DEVELOPMENT", int64(afkCoreTier()), 3, false
	}
	if !afkTechLabUnlocked() {
		return "UNLOCK TECH LAB", fmt.Sprintf("BUILD COST %s STARBITS", formatAFKNumber(afkTechLabUnlockCost)), "RESEARCH STACKS ON TOP OF AUTO CURSORS", gameMeta.AFKStarbits, afkTechLabUnlockCost, false
	}
	if afkResearchTotalLevels() < 5 {
		return "RESEARCH THE SINGULARITY", "PURCHASE 5 TECH LAB LEVELS", "PILOT SKILLS BEGIN TO OPEN", int64(afkResearchTotalLevels()), 5, false
	}
	if !afkTalentUnlocked(0) {
		return "SPEND A PILOT SKILL POINT", "BUY SHARPER CURSORS", "AUTO CURSOR OUTPUT +10%", int64(boolToInt(afkTalentUnlocked(0))), 1, false
	}
	if afkResearchTotalLevels() < 12 {
		return "DEEPEN TECH LAB", "REACH 12 TOTAL RESEARCH LEVELS", "KEEP EARLIER SYSTEMS RUNNING", int64(afkResearchTotalLevels()), 12, false
	}
	if !afkTalentUnlocked(1) {
		return "ORBITAL CURSOR MATH 2.0", "SPEND 1 PILOT SKILL POINT", "CURSOR PROCESSING +10%", int64(boolToInt(afkTalentUnlocked(1))), 1, false
	}
	return "SECTION 6 COMPLETE", "TECH LAB + PILOT SKILLS ONLINE", "ALL EARLIER AFK SYSTEMS REMAIN ACTIVE", 1, 1, true
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func afkTechLabRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }
func afkTechRowRect(w, hgt int32, i int) RECT {
	p := afkTechLabRect(w, hgt)
	col := i % 2
	row := i / 2
	cw := (p.Right - p.Left - sx(44, w)) / 2
	left := p.Left + sx(18, w) + int32(col)*(cw+sx(8, w))
	top := p.Top + sy(float64(56+row*64), hgt)
	return RECT{left, top, left + cw, top + sy(54, hgt)}
}
func afkTalentTreeRect(w, hgt int32) RECT {
	ar := afkMainFieldRect(w, hgt)
	return RECT{ar.Left + sx(40, w), ar.Top + sy(22, hgt), ar.Right - sx(40, w), ar.Bottom - sy(24, hgt)}
}

func afkTalentTreeCanvasRect(w, hgt int32) RECT {
	p := afkTalentTreeRect(w, hgt)
	return RECT{p.Left + sx(28, w), p.Top + sy(94, hgt), p.Right - sx(28, w), p.Bottom - sy(18, hgt)}
}

func afkTalentVirtualSize(w, hgt int32) (int32, int32) {
	// v438: the Pilot Network is a genuinely larger 2D world. Both dimensions
	// exceed the viewport so empty-space dragging can explore the tree vertically
	// as well as horizontally.
	return sx(2400, w), sy(1600, hgt)
}

func clampAFKTalentPan(w, hgt int32) {
	vp := afkTalentTreeCanvasRect(w, hgt)
	worldW, worldH := afkTalentVirtualSize(w, hgt)
	viewW, viewH := vp.Right-vp.Left, vp.Bottom-vp.Top
	minX := viewW - worldW
	minY := viewH - worldH
	if minX > 0 {
		minX = 0
	}
	if minY > 0 {
		minY = 0
	}
	if afkTalentPanX > 0 {
		afkTalentPanX = 0
	}
	if afkTalentPanX < minX {
		afkTalentPanX = minX
	}
	if afkTalentPanY > 0 {
		afkTalentPanY = 0
	}
	if afkTalentPanY < minY {
		afkTalentPanY = minY
	}
}

func afkTalentNodeRect(w, hgt int32, i int) RECT {
	vp := afkTalentTreeCanvasRect(w, hgt)
	// v438: deliberately spacious left-to-right branches. The canvas is larger
	// than the viewport in both axes, so nodes no longer need to be compressed
	// merely to fit on one screen.
	centers := [][2]float64{
		{170, 790},   // 0 SHARPER CURSORS
		{600, 420},   // 1 ORBITAL CURSOR MATH
		{600, 1160},  // 2 TAP PRECISION
		{1050, 250},  // 3 DEEP SPACE NAVIGATOR
		{1510, 120},  // 4 CACHE HUNTER
		{1050, 760},  // 5 CURSOR SUPREMACY
		{1510, 690},  // 6 OVERDRIVE CHARGE
		{2010, 690},  // 7 OVERDRIVE EXTENSION
		{1510, 350},  // 8 NAV DATA ANALYSIS
		{2010, 300},  // 9 SIGNAL INTELLIGENCE
		{1510, 1040}, // 10 RAPID DISPATCH
		{2010, 100},  // 11 MAGNETIC ATTRACTOR
		{1510, 1280}, // 12 ORBITAL DRONES
		{2010, 1320}, // 13 STEALTH PROTOCOL
	}
	if i < 0 || i >= len(centers) {
		return RECT{}
	}
	cx := vp.Left + sx(centers[i][0], w) + afkTalentPanX
	cy := vp.Top + sy(centers[i][1], hgt) + afkTalentPanY
	d := sx(76, w)
	if dh := sy(76, hgt); dh < d {
		d = dh
	}
	return RECT{cx - d/2, cy - d/2, cx + d/2, cy + d/2}
}

func afkTalentInfoBaseRect(w, hgt int32) RECT {
	p := afkTalentTreeRect(w, hgt)
	infoH := sy(176, hgt)
	return RECT{p.Left + sx(14, w), p.Bottom - infoH, p.Right - sx(14, w), p.Bottom - sy(10, hgt)}
}

func afkTalentInfoShownRect(w, hgt int32) (RECT, float64) {
	base := afkTalentInfoBaseRect(w, hgt)
	dur := 170 * time.Millisecond
	progress := 0.0
	if afkTalentInfoStarted.IsZero() {
		if afkTalentInfoOpen {
			progress = 1
		}
	} else {
		progress = float64(time.Since(afkTalentInfoStarted)) / float64(dur)
		if progress > 1 {
			progress = 1
		}
		if !afkTalentInfoOpen {
			progress = 1 - progress
		}
	}
	off := int32(float64(base.Bottom-base.Top) * (1 - progress))
	return RECT{base.Left, base.Top + off, base.Right, base.Bottom + off}, progress
}

func afkTalentNodeState(i int) string {
	if afkTalentUnlocked(i) {
		return "active"
	}
	if i < 0 || i >= len(afkTalentDefs) || !afkTechLabUnlocked() {
		return "hidden"
	}
	req := afkTalentDefs[i].Requires
	if req >= 0 && !afkTalentUnlocked(req) {
		return "hidden"
	}
	if afkTalentAvailable(i) {
		return "available"
	}
	// The branch prerequisite is owned, but a physical facility requirement is
	// still missing. Reveal the real locked icon/name so the detail drawer can
	// explain exactly what the player must build next.
	return "locked"
}

func afkResetTalentPanToStart(w, hgt int32) {
	// Every time the Pilot Network opens, place SHARPER CURSORS in the centre of
	// the visible canvas rather than remembering the previous exploration pan.
	afkTalentPanX = 0
	afkTalentPanY = 0
	vp := afkTalentTreeCanvasRect(w, hgt)
	r := afkTalentNodeRect(w, hgt, 0)
	wantX := (vp.Left + vp.Right) / 2
	wantY := (vp.Top + vp.Bottom) / 2
	haveX := (r.Left + r.Right) / 2
	haveY := (r.Top + r.Bottom) / 2
	afkTalentPanX += wantX - haveX
	afkTalentPanY += wantY - haveY
	clampAFKTalentPan(w, hgt)
}

func afkTechResearchViewportRect(w, hgt int32) RECT {
	p := afkTechLabRect(w, hgt)
	return RECT{p.Left + sx(34, w), p.Top + sy(112, hgt), p.Right - sx(34, w), p.Bottom - sy(26, hgt)}
}

func afkTechResearchRowHeight(w, hgt int32) int32 { return sy(92, hgt) }
func afkTechResearchGap(w, hgt int32) int32       { return sy(9, hgt) }

func afkTechResearchContentHeight(w, hgt int32) int32 {
	visible := afkVisibleResearchCount()
	if visible <= 0 {
		return 0
	}
	return int32(visible)*afkTechResearchRowHeight(w, hgt) + int32(visible-1)*afkTechResearchGap(w, hgt)
}

func afkTechResearchMaxScroll(w, hgt int32) int32 {
	vp := afkTechResearchViewportRect(w, hgt)
	m := afkTechResearchContentHeight(w, hgt) - (vp.Bottom - vp.Top)
	if m < 0 {
		return 0
	}
	return m
}

func clampAFKTechScroll(w, hgt int32) {
	m := afkTechResearchMaxScroll(w, hgt)
	if afkTechScroll < 0 {
		afkTechScroll = 0
	}
	if afkTechScroll > m {
		afkTechScroll = m
	}
}

func afkTechScrollbarRects(w, hgt int32) (RECT, RECT) {
	vp := afkTechResearchViewportRect(w, hgt)
	track := RECT{vp.Right - sx(10, w), vp.Top, vp.Right, vp.Bottom}
	maxScroll := afkTechResearchMaxScroll(w, hgt)
	if maxScroll <= 0 {
		return track, track
	}
	visible := vp.Bottom - vp.Top
	content := afkTechResearchContentHeight(w, hgt)
	thumbH := visible * visible / content
	if thumbH < sy(42, hgt) {
		thumbH = sy(42, hgt)
	}
	travel := (track.Bottom - track.Top) - thumbH
	top := track.Top
	if travel > 0 {
		top += int32(int64(travel) * int64(afkTechScroll) / int64(maxScroll))
	}
	return track, RECT{track.Left, top, track.Right, top + thumbH}
}

func afkTechScrollbarDragTo(y, w, hgt int32) {
	track, thumb := afkTechScrollbarRects(w, hgt)
	maxScroll := afkTechResearchMaxScroll(w, hgt)
	if maxScroll <= 0 {
		afkTechScroll = 0
		return
	}
	thumbH := thumb.Bottom - thumb.Top
	travel := (track.Bottom - track.Top) - thumbH
	if travel <= 0 {
		afkTechScroll = 0
		return
	}
	pos := y - track.Top
	if pos < 0 {
		pos = 0
	}
	if pos > travel {
		pos = travel
	}
	afkTechScroll = int32(int64(pos) * int64(maxScroll) / int64(travel))
	clampAFKTechScroll(w, hgt)
}

func afkResearchCardRect(w, hgt int32, index int) RECT {
	vp := afkTechResearchViewportRect(w, hgt)
	rowH := afkTechResearchRowHeight(w, hgt)
	gap := afkTechResearchGap(w, hgt)
	y := vp.Top + int32(index)*(rowH+gap) - afkTechScroll
	return RECT{vp.Left, y, vp.Right - sx(18, w), y + rowH}
}

func afkResearchEffectText(i, lvl int) string {
	switch i {
	case 0:
		return fmt.Sprintf("+%.1f%% STARBIT PRODUCTION", float64(lvl)*0.5)
	case 1:
		return fmt.Sprintf("+%.1f%% STARBIT PRODUCTION", float64(lvl)*0.4)
	case 2:
		return fmt.Sprintf("+%d%% MANUAL EXTRACTION", lvl*4)
	case 3:
		return fmt.Sprintf("-%.1f%% EXPEDITION TIME", float64(lvl)*1.5)
	case 4:
		return fmt.Sprintf("+%d%% STAR CACHE CHANCE", lvl)
	case 5:
		return fmt.Sprintf("+%d MIN SCOUT CAPACITY", lvl*30)
	case 6:
		return fmt.Sprintf("+%.1f%% STARBIT PRODUCTION", float64(lvl)*0.5)
	case 7:
		return fmt.Sprintf("+%.1f%% STARBIT PRODUCTION", float64(lvl)*0.4)
	case 8:
		return fmt.Sprintf("+%.1f%% STARBIT PRODUCTION", float64(lvl)*0.4)
	case 9:
		return fmt.Sprintf("+%.1f%% STARBIT PRODUCTION", float64(lvl)*0.8)
	case 10:
		return fmt.Sprintf("+%d%% DRONE OUTPUT", lvl)
	case 11:
		return fmt.Sprintf("+%d%% DRONE OUTPUT", lvl)
	case 12:
		return fmt.Sprintf("-%.1f%% ORBITAL CYCLE TIME", float64(lvl)*1.5)
	case 13:
		return fmt.Sprintf("+%d%% DRONE OUTPUT", lvl)
	case 14:
		return fmt.Sprintf("+%d%% ORBITAL OUTPUT", lvl*3)
	}
	return "NO EFFECT"
}

func afkResearchNextEffectText(i, lvl int) string {
	if lvl >= afkResearchMaxLevel {
		return "MAXIMUM EFFECT REACHED"
	}
	return afkResearchEffectText(i, lvl+1)
}

func drawAFKTechLab(hdc uintptr, w, hgt int32) {
	if !afkTechPanelOpen {
		return
	}
	p := afkTechLabRect(w, hgt)
	drawAFKManagementChrome(hdc, p, "TECH LAB", "PERMANENT RESEARCH // SELECT A CARD TO SEE ITS CURRENT EFFECT AND NEXT COST", rgb(80, 214, 255), w, hgt)
	if !afkTechLabUnlocked() {
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(225, 238, 246))
			centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(135, hgt), "CURSOR CORE TIER 3 HAS OPENED THE TECH LINK")
			setTextColor.Call(hdc, rgb(255, 211, 62))
			centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(170, hgt), "BUILD THE TECH LAB USING THE LARGE ACTION BUTTON BELOW")
			selectObject.Call(hdc, old)
		}
		return
	}
	if hudTinyFont != 0 {
		i := afkResearchSelected
		if i < 0 || i >= afkVisibleResearchCount() {
			i = 0
		}
		d := afkResearchDefs[i]
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 220, 78))
		centeredTextOut(hdc, p.Left+sx(40, w), p.Right-sx(40, w), p.Top+sy(52, hgt), fmt.Sprintf("SELECTED // %s // LV.%d", d.Name, afkResearchLevel(i)))
		setTextColor.Call(hdc, rgb(151, 190, 211))
		centeredTextOut(hdc, p.Left+sx(80, w), p.Right-sx(80, w), p.Top+sy(79, hgt), d.Description)
		selectObject.Call(hdc, old)
	}
	visible := afkVisibleResearchCount()
	clampAFKTechScroll(w, hgt)
	vp := afkTechResearchViewportRect(w, hgt)
	saved, _, _ := saveDC.Call(hdc)
	intersectClipRect.Call(hdc, uintptr(vp.Left), uintptr(vp.Top), uintptr(vp.Right), uintptr(vp.Bottom))
	for i := 0; i < visible; i++ {
		r := afkResearchCardRect(w, hgt, i)
		if r.Bottom < vp.Top || r.Top > vp.Bottom {
			continue
		}
		d := afkResearchDefs[i]
		lvl := afkResearchLevel(i)
		cost := afkResearchCost(i)
		selected := i == afkResearchSelected
		fill := rgb(6, 31, 55)
		outline := rgb(46, 100, 135)
		if selected {
			fill = rgb(31, 24, 71)
			outline = rgb(255, 198, 59)
		}
		if pointInRect(cursorPos, r) {
			outline = rgb(94, 222, 255)
		}
		drawBevelPanel(hdc, r, fill, outline, rgb(0, 5, 14), 2)
		textLeft := r.Left + sx(14, w)
		if icon := afkTechIconForResearch(i); icon != nil {
			iconBox := RECT{r.Left + sx(8, w), r.Top + sy(8, hgt), r.Left + sx(78, w), r.Bottom - sy(8, hgt)}
			drawAFKIconSprite(hdc, icon, iconBox)
			textLeft = r.Left + sx(88, w)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(241, 248, 252))
			textOut(hdc, textLeft, r.Top+sy(10, hgt), fmt.Sprintf("%s   //   LEVEL %d/%d", d.Name, lvl, afkResearchMaxLevel))
			setTextColor.Call(hdc, rgb(151, 190, 211))
			textOut(hdc, textLeft, r.Top+sy(30, hgt), d.Description)
			setTextColor.Call(hdc, rgb(116, 222, 255))
			textOut(hdc, textLeft, r.Top+sy(51, hgt), "CURRENT // "+afkResearchEffectText(i, lvl))
			setTextColor.Call(hdc, rgb(184, 205, 216))
			nextX := r.Left + sx(440, w)
			if textLeft > r.Left+sx(20, w) {
				nextX = r.Left + sx(490, w)
			}
			textOut(hdc, nextX, r.Top+sy(51, hgt), "NEXT // "+afkResearchNextEffectText(i, lvl))
			if cost > 0 {
				setTextColor.Call(hdc, rgb(255, 213, 72))
				textOut(hdc, textLeft, r.Top+sy(70, hgt), "UPGRADE COST // "+formatAFKNumber(cost)+" STARBITS")
			} else {
				setTextColor.Call(hdc, rgb(84, 220, 142))
				textOut(hdc, textLeft, r.Top+sy(70, hgt), "MAX LEVEL")
			}
			selectObject.Call(hdc, old)
		}
	}
	restoreDC.Call(hdc, saved)
	track, thumb := afkTechScrollbarRects(w, hgt)
	fillSolidRect(hdc, track, rgb(3, 15, 28))
	if afkTechResearchMaxScroll(w, hgt) > 0 {
		drawBevelPanel(hdc, thumb, rgb(19, 69, 96), rgb(80, 214, 255), rgb(0, 5, 14), 1)
	}
}

func drawAFKTalentPlaceholderIcon(hdc uintptr, r RECT, i int, c uintptr, w, hgt int32) {
	cx := (r.Left + r.Right) / 2
	cy := (r.Top + r.Bottom) / 2
	rad := (r.Right-r.Left)/2 - sx(12, w)
	if rad < 5 {
		rad = 5
	}
	lw := max32(1, sx(2, w))
	// Temporary symbolic icons. These are deliberately simple and will be
	// replaced by the six circular PNG assets supplied after the art pass.
	switch i {
	case 0: // sharper cursors - crosshair
		drawCircleOutline(hdc, cx, cy, rad, lw, c)
		drawLineSimple(hdc, cx-rad, cy, cx+rad, cy, int(lw), c)
		drawLineSimple(hdc, cx, cy-rad, cx, cy+rad, int(lw), c)
	case 1: // orbital maths - orbit
		drawCircleOutline(hdc, cx, cy, rad, lw, c)
		drawCircleOutline(hdc, cx, cy, max32(3, rad/3), lw, c)
		drawLineSimple(hdc, cx-rad, cy+rad/2, cx+rad, cy-rad/2, int(lw), c)
	case 2: // tap precision - target
		drawCircleOutline(hdc, cx, cy, rad, lw, c)
		drawCircleOutline(hdc, cx, cy, max32(3, rad/2), lw, c)
		fillSolidRect(hdc, RECT{cx - lw, cy - lw, cx + lw + 1, cy + lw + 1}, c)
	case 3: // deep space navigator - arrow
		drawLineSimple(hdc, cx-rad/2, cy+rad/2, cx+rad/2, cy-rad/2, int(lw+1), c)
		drawLineSimple(hdc, cx+rad/2, cy-rad/2, cx+rad/5, cy-rad/2, int(lw+1), c)
		drawLineSimple(hdc, cx+rad/2, cy-rad/2, cx+rad/2, cy-rad/5, int(lw+1), c)
	case 4: // cache hunter - diamond
		drawLineSimple(hdc, cx, cy-rad, cx+rad, cy, int(lw), c)
		drawLineSimple(hdc, cx+rad, cy, cx, cy+rad, int(lw), c)
		drawLineSimple(hdc, cx, cy+rad, cx-rad, cy, int(lw), c)
		drawLineSimple(hdc, cx-rad, cy, cx, cy-rad, int(lw), c)
	case 5: // supremacy - star / convergence
		drawCircleOutline(hdc, cx, cy, rad, lw, c)
		drawLineSimple(hdc, cx-rad, cy, cx+rad, cy, int(lw), c)
		drawLineSimple(hdc, cx, cy-rad, cx, cy+rad, int(lw), c)
		drawLineSimple(hdc, cx-rad*2/3, cy-rad*2/3, cx+rad*2/3, cy+rad*2/3, int(lw), c)
		drawLineSimple(hdc, cx+rad*2/3, cy-rad*2/3, cx-rad*2/3, cy+rad*2/3, int(lw), c)
	}
}

func drawAFKTalentTree(hdc uintptr, w, hgt int32) {
	if !afkTalentPanelOpen {
		return
	}
	clampAFKTalentPan(w, hgt)
	p := afkTalentTreeRect(w, hgt)
	drawBevelPanel(hdc, p, rgb(8, 13, 24), rgb(103, 117, 132), rgb(1, 4, 10), 2)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(227, 236, 241))
		textOut(hdc, p.Left+sx(24, w), p.Top+sy(14, hgt), "ACQUIRE PILOT SKILLS")
		setTextColor.Call(hdc, rgb(151, 171, 184))
		textOut(hdc, p.Left+sx(24, w), p.Top+sy(35, hgt), fmt.Sprintf("AVAILABLE SKILL POINTS // %d // MILESTONES %d / %d", afkTalentPointsAvailable(), afkTalentMilestonesCompleted(), len(afkTalentPointMilestones())))
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(112, 165, 194))
		source := fitTextEllipsis(hdc, hudTinyFont, afkTalentPointSourceSummary(), p.Right-p.Left-sx(48, w))
		textOut(hdc, p.Left+sx(24, w), p.Top+sy(56, hgt), source)
		setTextColor.Call(hdc, rgb(246, 205, 82))
		next := fitTextEllipsis(hdc, hudTinyFont, afkTalentNextPointText(), p.Right-p.Left-sx(48, w))
		textOut(hdc, p.Left+sx(24, w), p.Top+sy(75, hgt), next)
		selectObject.Call(hdc, old)
	}

	vp := afkTalentTreeCanvasRect(w, hgt)
	saved, _, _ := saveDC.Call(hdc)
	if saved != 0 {
		intersectClipRect.Call(hdc, uintptr(vp.Left), uintptr(vp.Top), uintptr(vp.Right), uintptr(vp.Bottom))
	}
	// v438: the supplied Pilot Network artwork is a world-space background. It
	// moves with the same pan offset as the nodes, so dragging feels like moving
	// around one large map rather than sliding icons over a static wallpaper.
	if afkPilotNetworkBackground != nil && len(afkPilotNetworkBackground.data) >= int(afkPilotNetworkBackground.w*afkPilotNetworkBackground.h*4) {
		worldW, worldH := afkTalentVirtualSize(w, hgt)
		bgRect := RECT{vp.Left + afkTalentPanX, vp.Top + afkTalentPanY, vp.Left + afkTalentPanX + worldW, vp.Top + afkTalentPanY + worldH}
		drawRawBGRAFit(hdc, afkPilotNetworkBackground.data, afkPilotNetworkBackground.w, afkPilotNetworkBackground.h, bgRect)
	}
	pulse := afkSoftPulse(time.Now(), 1.35, 0)
	brightPulse := afkSoftPulse(time.Now(), 0.95, .17)

	// Branch visibility follows prerequisites independently. Locked/future links
	// use the same electric-blue family as the supplied Pilot Network background;
	// owned/available paths stay gold and therefore always take visual priority.
	for i, d := range afkTalentDefs {
		if d.Requires < 0 {
			continue
		}
		a := afkTalentNodeRect(w, hgt, d.Requires)
		b := afkTalentNodeRect(w, hgt, i)
		childState := afkTalentNodeState(i)
		parentOwned := afkTalentUnlocked(d.Requires)
		c := afkRGBLerp(8, 54, 103, 18, 101, 173, pulse)
		lw := max32(1, sx(2, w))
		if parentOwned {
			if childState == "active" {
				c = afkRGBLerp(128, 88, 18, 250, 202, 68, pulse)
				lw = max32(2, sx(3, w))
			} else if childState == "available" {
				c = afkRGBLerp(186, 112, 16, 255, 226, 92, brightPulse)
				lw = max32(3, sx(4, w))
			}
		}
		drawLineSimple(hdc, (a.Left+a.Right)/2, (a.Top+a.Bottom)/2, (b.Left+b.Right)/2, (b.Top+b.Bottom)/2, int(lw), uintptr(c))
	}

	for i := range afkTalentDefs {
		r := afkTalentNodeRect(w, hgt, i)
		state := afkTalentNodeState(i)
		selected := afkTalentInfoOpen && i == afkTalentSelected && state != "hidden"
		cx, cy := (r.Left+r.Right)/2, (r.Top+r.Bottom)/2
		rad := (r.Right - r.Left) / 2
		if state == "active" {
			// Owned skills stay full-colour with a calm, steady gold halo. The
			// stronger animation is reserved for the next purchasable skill.
			drawCircleOutline(hdc, cx, cy, rad+sx(7, w), max32(2, sx(3, w)), rgb(188, 132, 34))
			drawCircleOutline(hdc, cx, cy, rad+sx(3, w), max32(1, sx(2, w)), rgb(246, 205, 82))
		} else if state == "available" {
			c1 := afkRGBLerp(166, 98, 8, 255, 223, 72, brightPulse)
			c2 := afkRGBLerp(220, 153, 20, 255, 244, 164, brightPulse)
			drawCircleOutline(hdc, cx, cy, rad+sx(9, w), max32(3, sx(5, w)), c1)
			drawCircleOutline(hdc, cx, cy, rad+sx(4, w), max32(2, sx(3, w)), c2)
		}
		if selected {
			drawCircleOutline(hdc, cx, cy, rad+sx(11, w), max32(2, sx(2, w)), rgb(244, 237, 195))
		}
		iconState := state
		if iconState == "available" {
			iconState = "locked"
		}
		iconBox := RECT{r.Left - sx(7, w), r.Top - sy(7, hgt), r.Right + sx(7, w), r.Bottom + sy(7, hgt)}
		if sprite := afkSkillTreeIcons[afkSkillIconName(i, iconState)]; !drawAFKIconSprite(hdc, sprite, iconBox) {
			drawAFKTalentPlaceholderIcon(hdc, r, i, rgb(160, 160, 160), w, hgt)
		}
	}
	if saved != 0 {
		restoreDC.Call(hdc, saved)
	}

	// Slide-up detail drawer: hidden by default to maximise tree space.
	info, prog := afkTalentInfoShownRect(w, hgt)
	if prog <= 0.01 || afkTalentSelected < 0 || afkTalentSelected >= len(afkTalentDefs) || afkTalentNodeState(afkTalentSelected) == "hidden" {
		return
	}
	drawBevelPanel(hdc, info, rgb(20, 22, 22), rgb(113, 106, 73), rgb(3, 5, 6), 2)
	d := afkTalentDefs[afkTalentSelected]
	active := afkTalentUnlocked(afkTalentSelected)
	available := afkTalentAvailable(afkTalentSelected)
	status := "LOCKED"
	statusCol := rgb(183, 73, 73)
	if available {
		status, statusCol = "AVAILABLE", rgb(234, 190, 70)
	}
	if active {
		status, statusCol = "ACQUIRED", rgb(116, 218, 139)
	}
	req := "ROOT SKILL"
	if d.Requires >= 0 {
		req = "ACQUIRE " + afkTalentDefs[d.Requires].Name
	}
	if facilityReq := afkTalentFacilityRequirementText(afkTalentSelected); facilityReq != "" {
		if d.Requires >= 0 {
			req += " + " + facilityReq
		} else {
			req = facilityReq
		}
	}
	if active {
		req = "REQUIREMENT COMPLETE"
	}
	costText := "1 PILOT SKILL POINT"
	if active {
		costText = "OWNED"
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(238, 231, 211))
		textOut(hdc, info.Left+sx(20, w), info.Top+sy(14, hgt), d.Name)
		setTextColor.Call(hdc, statusCol)
		textOut(hdc, info.Right-sx(150, w), info.Top+sy(14, hgt), status)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		left := info.Left + sx(20, w)
		mid := info.Left + (info.Right-info.Left)/2
		rightCol := mid + sx(18, w)
		setTextColor.Call(hdc, rgb(202, 205, 198))
		textOut(hdc, left, info.Top+sy(48, hgt), "EFFECT")
		setTextColor.Call(hdc, rgb(246, 223, 151))
		textOut(hdc, left, info.Top+sy(69, hgt), d.Effect)
		setTextColor.Call(hdc, rgb(202, 205, 198))
		textOut(hdc, left, info.Top+sy(103, hgt), "DESCRIPTION")
		setTextColor.Call(hdc, rgb(163, 169, 166))
		textOut(hdc, left, info.Top+sy(124, hgt), d.Flavour)
		setTextColor.Call(hdc, rgb(202, 205, 198))
		textOut(hdc, rightCol, info.Top+sy(48, hgt), "REQUIREMENT")
		setTextColor.Call(hdc, rgb(163, 169, 166))
		textOut(hdc, rightCol, info.Top+sy(69, hgt), req)
		setTextColor.Call(hdc, rgb(202, 205, 198))
		textOut(hdc, rightCol, info.Top+sy(103, hgt), "COST")
		setTextColor.Call(hdc, rgb(246, 223, 151))
		textOut(hdc, rightCol, info.Top+sy(124, hgt), costText)
		selectObject.Call(hdc, old)
	}
}

func handleAFKTechLabWheel(delta int32, w, hgt int32) bool {
	if !afkTechPanelOpen || !afkTechLabUnlocked() {
		return false
	}
	step := afkTechResearchRowHeight(w, hgt)
	if delta < 0 {
		afkTechScroll += step
	} else if delta > 0 {
		afkTechScroll -= step
	}
	clampAFKTechScroll(w, hgt)
	return true
}

func handleAFKSection6Click(h uintptr, p FPoint, w, hgt int32) bool {
	if afkTechPanelOpen {
		if afkTechLabUnlocked() {
			track, thumb := afkTechScrollbarRects(w, hgt)
			if afkTechResearchMaxScroll(w, hgt) > 0 && pointInRect(p, thumb) {
				afkTechScrollDragging = true
				afkTechScrollDragOffset = int32(p.Y) - thumb.Top
				setCapture.Call(h)
				return true
			}
			if afkTechResearchMaxScroll(w, hgt) > 0 && pointInRect(p, track) {
				afkTechScrollbarDragTo(int32(p.Y)-(thumb.Bottom-thumb.Top)/2, w, hgt)
				invalidateRect.Call(h, 0, 0)
				return true
			}
			vp := afkTechResearchViewportRect(w, hgt)
			if pointInRect(p, vp) {
				for i := 0; i < afkVisibleResearchCount(); i++ {
					if pointInRect(p, afkResearchCardRect(w, hgt, i)) {
						afkResearchSelected = i
						playUIButtonClickSound()
						invalidateRect.Call(h, 0, 0)
						return true
					}
				}
			}
		}
		return pointInRect(p, afkTechLabRect(w, hgt))
	}
	if afkTalentPanelOpen {
		for i := range afkTalentDefs {
			if pointInRect(p, afkTalentNodeRect(w, hgt, i)) {
				// Future skills remain truly hidden: their shared question-mark node
				// never leaks the name/effect until the prerequisite is owned.
				if afkTalentNodeState(i) == "hidden" {
					playUIButtonClickSound()
					return true
				}
				if afkTalentInfoOpen && afkTalentSelected == i {
					afkTalentInfoOpen = false
					afkTalentInfoStarted = time.Now()
				} else {
					afkTalentSelected = i
					afkTalentInfoOpen = true
					afkTalentInfoStarted = time.Now()
				}
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return true
			}
		}
		if pointInRect(p, afkTalentTreeCanvasRect(w, hgt)) {
			// Empty-space click closes the drawer and also begins drag-to-pan.
			if afkTalentInfoOpen {
				afkTalentInfoOpen = false
				afkTalentInfoStarted = time.Now()
			}
			afkTalentPanDragging = true
			afkTalentPanLastX = int32(p.X)
			afkTalentPanLastY = int32(p.Y)
			setCapture.Call(h)
			invalidateRect.Call(h, 0, 0)
			return true
		}
		return pointInRect(p, afkTalentTreeRect(w, hgt))
	}
	return false
}
