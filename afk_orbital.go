//go:build windows

package main

import (
	"fmt"
	"math"
	"time"
)

// Section 10: Orbital Extractor. The reference video ends while its Rocket
// Launcher equivalent is still locked, but its Research Lab already exposes
// rocket production-speed and rocket-output upgrades. Cursor Control preserves
// that exact tease-and-unlock structure, then completes the loop as a periodic
// hard-coded Starbit burst system.
const (
	afkOrbitalBuildCost       int64 = 75_000_000_000_000
	afkOrbitalMaxTier               = 3
	afkOrbitalUpgradeCount          = 5
	afkOrbitalUpgradeMaxLevel       = 20
)

const afkMaxInt64 = int64(^uint64(0) >> 1)

func saturatingAdd(a, b int64) int64 {
	if b > 0 && a > afkMaxInt64-b {
		return afkMaxInt64
	}
	if b < 0 && a < -afkMaxInt64-b {
		return -afkMaxInt64
	}
	return a + b
}
func saturatingMul(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > 0 && b > 0 && a > afkMaxInt64/b {
		return afkMaxInt64
	}
	return a * b
}
func saturatingMulDiv(a, b, d int64) int64 {
	if d <= 0 || a <= 0 || b <= 0 {
		return 0
	}
	// Divide one factor first where possible to retain range for late-game values.
	if a%d == 0 {
		return saturatingMul(a/d, b)
	}
	if b%d == 0 {
		return saturatingMul(a, b/d)
	}
	if a > afkMaxInt64/b {
		// Conservative saturated quotient rather than overflowing.
		return afkMaxInt64 / d
	}
	return a * b / d
}

type afkOrbitalUpgradeDef struct {
	Name, Description string
	Accent            uintptr
}

var afkOrbitalUpgradeDefs = [...]afkOrbitalUpgradeDef{
	{"CHARGE CAPACITORS", "Stores a larger Starbit charge before every discharge.", rgb(102, 218, 255)},
	{"CYCLE ACCELERATORS", "Shortens the delay between orbital extraction bursts.", rgb(103, 239, 176)},
	{"BEAM FOCUSING", "Compresses each discharge into a stronger extraction beam.", rgb(249, 205, 72)},
	{"QUANTUM LENS", "Improves beam efficiency at the Singularity boundary.", rgb(255, 126, 93)},
	{"ORBITAL SYNC", "Synchronises the cannon with the Drone and Cursor networks.", rgb(201, 108, 255)},
}

// Every price is deliberately authored. Row = upgrade; column = current level.
var afkOrbitalUpgradeCosts = [5][20]int64{
	{2_000_000_000_000, 3_000_000_000_000, 4_500_000_000_000, 6_500_000_000_000, 9_000_000_000_000, 13_000_000_000_000, 18_000_000_000_000, 25_000_000_000_000, 35_000_000_000_000, 48_000_000_000_000, 66_000_000_000_000, 91_000_000_000_000, 125_000_000_000_000, 172_000_000_000_000, 236_000_000_000_000, 324_000_000_000_000, 445_000_000_000_000, 610_000_000_000_000, 835_000_000_000_000, 1_145_000_000_000_000},
	{2_500_000_000_000, 3_700_000_000_000, 5_400_000_000_000, 7_800_000_000_000, 11_200_000_000_000, 16_000_000_000_000, 22_500_000_000_000, 31_500_000_000_000, 44_000_000_000_000, 61_000_000_000_000, 84_000_000_000_000, 116_000_000_000_000, 160_000_000_000_000, 220_000_000_000_000, 303_000_000_000_000, 417_000_000_000_000, 574_000_000_000_000, 790_000_000_000_000, 1_087_000_000_000_000, 1_495_000_000_000_000},
	{3_000_000_000_000, 4_400_000_000_000, 6_400_000_000_000, 9_300_000_000_000, 13_400_000_000_000, 19_200_000_000_000, 27_200_000_000_000, 38_400_000_000_000, 54_000_000_000_000, 75_500_000_000_000, 105_500_000_000_000, 147_500_000_000_000, 206_000_000_000_000, 287_500_000_000_000, 401_000_000_000_000, 559_000_000_000_000, 780_000_000_000_000, 1_089_000_000_000_000, 1_521_000_000_000_000, 2_125_000_000_000_000},
	{3_600_000_000_000, 5_300_000_000_000, 7_700_000_000_000, 11_200_000_000_000, 16_200_000_000_000, 23_300_000_000_000, 33_300_000_000_000, 47_400_000_000_000, 67_200_000_000_000, 95_000_000_000_000, 134_000_000_000_000, 189_000_000_000_000, 266_000_000_000_000, 374_000_000_000_000, 526_000_000_000_000, 740_000_000_000_000, 1_041_000_000_000_000, 1_464_000_000_000_000, 2_059_000_000_000_000, 2_897_000_000_000_000},
	{4_200_000_000_000, 6_200_000_000_000, 9_000_000_000_000, 13_100_000_000_000, 18_900_000_000_000, 27_100_000_000_000, 38_700_000_000_000, 55_100_000_000_000, 78_200_000_000_000, 110_700_000_000_000, 156_500_000_000_000, 221_000_000_000_000, 311_800_000_000_000, 439_700_000_000_000, 619_800_000_000_000, 873_500_000_000_000, 1_230_800_000_000_000, 1_734_000_000_000_000, 2_442_700_000_000_000, 3_441_000_000_000_000},
}

var afkOrbitalAscendCosts = [...]int64{0, 600_000_000_000_000, 6_000_000_000_000_000}
var afkOrbitalPanelOpen bool
var afkOrbitalSelectedUpgrade int
var afkOrbitalLastVisualFire time.Time
var afkOrbitalChargeCueForUnix int64

func afkEnsureSection10SaveShape() {
	if gameMeta.AFKOrbitalExtractorTier < 0 {
		gameMeta.AFKOrbitalExtractorTier = 0
	}
	if gameMeta.AFKOrbitalExtractorTier > afkOrbitalMaxTier {
		gameMeta.AFKOrbitalExtractorTier = afkOrbitalMaxTier
	}
	if len(gameMeta.AFKOrbitalUpgradeLevels) != afkOrbitalUpgradeCount {
		old := append([]int(nil), gameMeta.AFKOrbitalUpgradeLevels...)
		gameMeta.AFKOrbitalUpgradeLevels = make([]int, afkOrbitalUpgradeCount)
		copy(gameMeta.AFKOrbitalUpgradeLevels, old)
	}
	for i := range gameMeta.AFKOrbitalUpgradeLevels {
		if gameMeta.AFKOrbitalUpgradeLevels[i] < 0 {
			gameMeta.AFKOrbitalUpgradeLevels[i] = 0
		}
		if gameMeta.AFKOrbitalUpgradeLevels[i] > afkOrbitalUpgradeMaxLevel {
			gameMeta.AFKOrbitalUpgradeLevels[i] = afkOrbitalUpgradeMaxLevel
		}
	}
}

func afkOrbitalAvailable() bool { return gameMeta.AFKSection9Complete && afkDroneBayTier() >= 3 }
func afkOrbitalBuilt() bool     { return gameMeta.AFKOrbitalExtractorBuilt && afkOrbitalTier() > 0 }
func afkOrbitalTier() int       { afkEnsureSection10SaveShape(); return gameMeta.AFKOrbitalExtractorTier }
func afkOrbitalUpgradeLevel(i int) int {
	afkEnsureSection10SaveShape()
	if i < 0 || i >= afkOrbitalUpgradeCount {
		return 0
	}
	return gameMeta.AFKOrbitalUpgradeLevels[i]
}
func afkBuildOrbitalExtractor(now time.Time) bool {
	if !afkOrbitalAvailable() || afkOrbitalBuilt() || gameMeta.AFKStarbits < afkOrbitalBuildCost {
		return false
	}
	if !afkActionAllowed("build_orbital", now, afkEconomyActionDebounce) || !afkSpendStarbits(afkOrbitalBuildCost) {
		return false
	}
	gameMeta.AFKOrbitalExtractorBuilt = true
	gameMeta.AFKOrbitalExtractorTier = 1
	gameMeta.AFKOrbitalLastFireUnix = now.Unix()
	afkEnsureSection10SaveShape()
	afkRefreshTalentPointAwards()
	playAFKConstructionSound()
	return true
}
func afkOrbitalUpgradeCost(i int) int64 {
	if !afkOrbitalBuilt() || i < 0 || i >= afkOrbitalUpgradeCount {
		return 0
	}
	lvl := afkOrbitalUpgradeLevel(i)
	if lvl >= afkOrbitalUpgradeMaxLevel {
		return 0
	}
	return afkOrbitalUpgradeCosts[i][lvl]
}
func afkBuyOrbitalUpgrade(i int) bool {
	c := afkOrbitalUpgradeCost(i)
	if c <= 0 || gameMeta.AFKStarbits < c {
		return false
	}
	if !afkActionAllowed(fmt.Sprintf("buy_orbital_%d", i), time.Now(), afkEconomyActionDebounce) || !afkSpendStarbits(c) {
		return false
	}
	gameMeta.AFKOrbitalUpgradeLevels[i]++
	playAFKUpgradeSound()
	afkRefreshSection10Completion()
	return true
}
func afkOrbitalCanAscend() bool {
	t := afkOrbitalTier()
	if t < 1 || t >= afkOrbitalMaxTier {
		return false
	}
	min := 8
	if t == 2 {
		min = 14
	}
	return afkOrbitalUpgradeLevel(0) >= min && afkOrbitalUpgradeLevel(1) >= min && afkOrbitalUpgradeLevel(2) >= min
}
func afkOrbitalAscendCost() int64 {
	t := afkOrbitalTier()
	if t < 1 || t >= afkOrbitalMaxTier {
		return 0
	}
	return afkOrbitalAscendCosts[t]
}
func afkAscendOrbital() bool {
	c := afkOrbitalAscendCost()
	if c <= 0 || !afkOrbitalCanAscend() || gameMeta.AFKStarbits < c {
		return false
	}
	if !afkActionAllowed("ascend_orbital", time.Now(), afkClaimActionDebounce) || !afkSpendStarbits(c) {
		return false
	}
	gameMeta.AFKOrbitalExtractorTier++
	playAFKTierUpSound()
	afkRefreshSection10Completion()
	afkRefreshTalentPointAwards()
	return true
}

func afkOrbitalCooldownSeconds() int64 {
	if !afkOrbitalBuilt() {
		return 0
	}
	// 60s base, -1s/level from Cycle Accelerators, plus Engineer Training.
	v := int64(60 - afkOrbitalUpgradeLevel(1))
	v = v * afkOrbitalResearchSpeedMultiplierMilli() / 1000
	if v < 24 {
		v = 24
	}
	return v
}
func afkOrbitalBurstEquivalentSecondsMilli() int64 {
	if !afkOrbitalBuilt() {
		return 0
	}
	// Tier 1 starts at a 30-second-equivalent burst. Later tiers and authored
	// upgrades make the cannon a major layer without replacing Drones/Cursors.
	tier := []int64{0, 30_000, 55_000, 90_000}[afkOrbitalTier()]
	tier = tier * (1000 + int64(afkOrbitalUpgradeLevel(0))*30) / 1000
	tier = tier * (1000 + int64(afkOrbitalUpgradeLevel(2))*30) / 1000
	tier = tier * (1000 + int64(afkOrbitalUpgradeLevel(3))*20) / 1000
	tier = tier * (1000 + int64(afkOrbitalUpgradeLevel(4))*20) / 1000
	tier = tier * afkOrbitalResearchOutputMultiplierMilli() / 1000
	tier = tier * afkModuleDroneOrbitalMultiplierMilli() / 1000
	return tier
}
func afkOrbitalBurstStarbits() int64 {
	baseMilli := afkContinuousStarbitsPerSecondMilli()
	if baseMilli <= 0 {
		return 0
	}
	eqMilli := afkOrbitalBurstEquivalentSecondsMilli()
	// (Starbits/s * 1000) * (seconds * 1000) / 1e6 = Starbits.
	return saturatingMulDiv(baseMilli, eqMilli, 1_000_000)
}
func afkOrbitalEffectivePerSecondMilli() int64 {
	if !afkOrbitalBuilt() {
		return 0
	}
	cd := afkOrbitalCooldownSeconds()
	if cd <= 0 {
		return 0
	}
	burst := afkOrbitalBurstStarbits()
	effective := saturatingMulDiv(burst, 1000, cd)
	// v434: Orbital is a bonus layer, not a replacement for the whole economy.
	// Its long-run contribution is capped at 50% of continuous production.
	cap := afkContinuousStarbitsPerSecondMilli() / 2
	if cap > 0 && effective > cap {
		return cap
	}
	return effective
}
func afkOrbitalNextFireIn(now time.Time) int64 {
	if !afkOrbitalBuilt() {
		return 0
	}
	cd := afkOrbitalCooldownSeconds()
	last := gameMeta.AFKOrbitalLastFireUnix
	if last <= 0 {
		return cd
	}
	remain := last + cd - now.Unix()
	if remain < 0 {
		remain = 0
	}
	return remain
}
func afkProcessOrbitalLive(now time.Time) int64 {
	if !afkOrbitalBuilt() {
		return 0
	}
	cd := afkOrbitalCooldownSeconds()
	if cd <= 0 {
		return 0
	}
	if gameMeta.AFKOrbitalLastFireUnix <= 0 {
		gameMeta.AFKOrbitalLastFireUnix = now.Unix()
		return 0
	}
	elapsed := now.Unix() - gameMeta.AFKOrbitalLastFireUnix
	nextFire := gameMeta.AFKOrbitalLastFireUnix + cd
	chargeLead := int64(5)
	if cd < 12 {
		chargeLead = 3
	}
	if now.Unix() >= nextFire-chargeLead && now.Unix() < nextFire && afkOrbitalChargeCueForUnix != nextFire {
		afkOrbitalChargeCueForUnix = nextFire
		playAFKOrbitalChargeSound()
	}
	if elapsed < cd {
		return 0
	}
	cycles := elapsed / cd
	if cycles > 4 {
		cycles = 4
	} // long gaps are paid through the AFK path.
	burst := afkOrbitalBurstStarbits()
	if afkOverdriveActive(now) {
		burst = saturatingMul(burst, 2)
	}
	gained := saturatingMul(burst, cycles)
	if gained > 0 {
		afkGrantStarbits(gained)
		gameMeta.AFKOrbitalBurstsFired += cycles
		afkOrbitalLastVisualFire = now
		playAFKOrbitalFireSound()
	}
	gameMeta.AFKOrbitalLastFireUnix += cycles * cd
	afkRefreshSection10Completion()
	return gained
}
func afkOrbitalOfflineStarbits(paidSeconds int64) int64 {
	if !afkOrbitalBuilt() || paidSeconds <= 0 {
		return 0
	}
	cd := afkOrbitalCooldownSeconds()
	if cd <= 0 {
		return 0
	}
	cycles := paidSeconds / cd
	if cycles <= 0 {
		return 0
	}
	// Advance the persisted cycle clock as we pay offline bursts. Without this,
	// returning to active play could pay those same completed cycles a second time.
	if gameMeta.AFKOrbitalLastFireUnix > 0 {
		gameMeta.AFKOrbitalLastFireUnix += cycles * cd
	}
	gameMeta.AFKOrbitalBurstsFired = saturatingAdd(gameMeta.AFKOrbitalBurstsFired, cycles)
	afkRefreshSection10Completion()
	return saturatingMul(afkOrbitalBurstStarbits(), cycles)
}
func afkRefreshSection10Completion() {
	if afkOrbitalTier() >= 3 && afkOrbitalUpgradeLevel(0) >= 15 && afkOrbitalUpgradeLevel(1) >= 15 && afkOrbitalUpgradeLevel(2) >= 15 && gameMeta.AFKOrbitalBurstsFired >= 20 {
		gameMeta.AFKSection10Complete = true
	}
}
func afkSection10Objective() (string, string, string, int64, int64, bool) {
	if !afkOrbitalBuilt() {
		return "UNLOCK ORBITAL EXTRACTOR", "THE TECH LAB HAS ALREADY", "PREPARED ORBITAL SYSTEMS.", gameMeta.AFKStarbits, afkOrbitalBuildCost, false
	}
	if gameMeta.AFKOrbitalBurstsFired < 1 {
		return "FIRE ORBITAL EXTRACTOR", "WAIT FOR THE FIRST FULL", "CHARGE + DISCHARGE CYCLE.", gameMeta.AFKOrbitalBurstsFired, 1, false
	}
	if afkOrbitalUpgradeLevel(0) < 8 {
		return "CHARGE CAPACITORS LV. 8", "INCREASE THE STORED STARBIT", "BURST BEFORE FIRST ASCENSION.", int64(afkOrbitalUpgradeLevel(0)), 8, false
	}
	if afkOrbitalTier() < 2 {
		return "ASCEND ORBITAL EXTRACTOR // TIER 2", fmt.Sprintf("STORE %s STARBITS", formatAFKNumber(afkOrbitalAscendCost())), "CAPACITORS + CYCLE + BEAM LV.8 REQUIRED.", gameMeta.AFKStarbits, afkOrbitalAscendCost(), false
	}
	if afkOrbitalUpgradeLevel(0) < 15 || afkOrbitalUpgradeLevel(1) < 15 || afkOrbitalUpgradeLevel(2) < 15 {
		cur := afkOrbitalUpgradeLevel(0)
		if afkOrbitalUpgradeLevel(1) < cur {
			cur = afkOrbitalUpgradeLevel(1)
		}
		if afkOrbitalUpgradeLevel(2) < cur {
			cur = afkOrbitalUpgradeLevel(2)
		}
		return "ORBITAL CORE UPGRADES LV. 15", "CHARGE + CYCLE + BEAM MUST", "ALL REACH LEVEL 15.", int64(cur), 15, false
	}
	if afkOrbitalTier() < 3 {
		return "ASCEND ORBITAL EXTRACTOR // TIER 3", fmt.Sprintf("STORE %s STARBITS", formatAFKNumber(afkOrbitalAscendCost())), "FINALISE THE ORBITAL NETWORK.", gameMeta.AFKStarbits, afkOrbitalAscendCost(), false
	}
	if gameMeta.AFKOrbitalBurstsFired < 20 {
		return "COMPLETE 20 ORBITAL FIRES", "PROVE THE TIER 3 EXTRACTOR", "CAN SUSTAIN REPEATED DISCHARGES.", gameMeta.AFKOrbitalBurstsFired, 20, false
	}
	afkRefreshSection10Completion()
	return "AFK CORE LOOP COMPLETE", "ORBITAL EXTRACTOR TIER 3 ONLINE.", "ENDGAME STARBIT NETWORK ACTIVE.", 1, 1, true
}

func afkOrbitalPanelRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }
func afkOrbitalUpgradeRect(w, hgt int32, i int) RECT {
	p := afkOrbitalPanelRect(w, hgt)
	gap := sx(12, w)
	left := p.Left + sx(28, w)
	right := p.Right - sx(28, w)
	bw := (right - left - gap*4) / 5
	x := left + int32(i)*(bw+gap)
	return RECT{x, p.Top + sy(132, hgt), x + bw, p.Bottom - sy(42, hgt)}
}
func drawAFKOrbitalWorld(hdc uintptr, w, hgt int32) {
	if !afkOrbitalBuilt() || afkOrbitalPanelOpen {
		return
	}
	field := afkMainFieldRect(w, hgt)
	cx, cy, rad := afkSingularityGeometry(w, hgt)
	x := int32(cx)
	y := field.Top + sy(36, hgt)
	s := max32(2, sx(3, w))
	// Hard-coded 8-bit orbital cannon suspended above the Singularity.
	fillSolidRect(hdc, RECT{x - s*9, y, x + s*9, y + s*4}, rgb(43, 58, 88))
	fillSolidRect(hdc, RECT{x - s*6, y - s*3, x + s*6, y + s}, rgb(116, 93, 218))
	fillSolidRect(hdc, RECT{x - s*2, y + s*4, x + s*2, y + s*12}, rgb(172, 198, 220))
	fillSolidRect(hdc, RECT{x - s, y + s*12, x + s, int32(cy - rad*0.82)}, rgb(92, 214, 255))
	// Charge meter immediately under the cannon.
	nowT := time.Now()
	cd := afkOrbitalCooldownSeconds()
	remain := afkOrbitalNextFireIn(nowT)
	filled := int32(0)
	if cd > 0 {
		filled = int32((cd - remain) * 100 / cd)
	}
	bar := RECT{x - s*14, y + s*15, x + s*14, y + s*18}
	fillSolidRect(hdc, bar, rgb(13, 31, 54))
	if filled > 0 {
		f := bar
		f.Right = bar.Left + (bar.Right-bar.Left)*filled/100
		fillSolidRect(hdc, f, rgb(101, 226, 255))
	}

	// The final part of every cooldown visibly charges the muzzle. Particles spiral
	// inward and the glow intensifies so the large extraction shot is readable even
	// without watching the numerical cooldown.
	charge := float64(filled) / 100.0
	if charge > 0.72 {
		local := (charge - 0.72) / 0.28
		muzzleY := y + s*12
		for i := 0; i < 5; i++ {
			a := float64(nowT.UnixNano())/1e9*(2.4+float64(i)*0.12) + float64(i)*1.256
			rr := float64(s) * (11.0 - 7.0*local)
			px := x + int32(math.Cos(a)*rr)
			py := muzzleY + int32(math.Sin(a)*rr)
			fillSolidRect(hdc, RECT{px - s/2, py - s/2, px + s/2 + 1, py + s/2 + 1}, rgb(177, 100, 255))
		}
		glow := max32(s*3, int32(float64(s*8)*local))
		drawCircleOutline(hdc, x, muzzleY, glow, max32(1, s/2), rgb(201, 112, 255))
		fillSolidRect(hdc, RECT{x - s, muzzleY - s, x + s + 1, muzzleY + s + 1}, rgb(241, 225, 255))
	}

	// A fired beam persists visually for ~700ms but the economic burst is paid once.
	if !afkOrbitalLastVisualFire.IsZero() {
		elapsed := nowT.Sub(afkOrbitalLastVisualFire)
		if elapsed < 700*time.Millisecond {
			t := float64(elapsed) / float64(700*time.Millisecond)
			bx := x
			top := y + s*12
			bottom := int32(cy - rad*0.15)
			widthPulse := 1.0 + 0.20*math.Sin(t*math.Pi*6)
			outer := int32(float64(s*7) * widthPulse)
			mid := int32(float64(s*4) * widthPulse)
			fillSolidRect(hdc, RECT{bx - outer, top, bx + outer, bottom}, rgb(68, 31, 128))
			fillSolidRect(hdc, RECT{bx - mid, top, bx + mid, bottom}, rgb(190, 72, 255))
			fillSolidRect(hdc, RECT{bx - s*2, top, bx + s*2, bottom}, rgb(246, 238, 255))
			// Impact ring and Starbit-like sparks bloom where the beam meets the core.
			impactY := bottom
			ring := max32(s*5, int32(float64(s*14)*(0.45+t)))
			drawCircleOutline(hdc, bx, impactY, ring, max32(1, s), rgb(203, 88, 255))
			for i := 0; i < 8; i++ {
				a := float64(i)*math.Pi/4 + t*2.2
				rr := float64(s) * (8 + 18*t)
				px := bx + int32(math.Cos(a)*rr)
				py := impactY + int32(math.Sin(a)*rr)
				fillSolidRect(hdc, RECT{px - s/2, py - s/2, px + s/2 + 1, py + s/2 + 1}, rgb(112, 231, 255))
			}
		}
	}
}
func drawAFKOrbital(hdc uintptr, w, hgt int32) {
	if !afkOrbitalPanelOpen {
		return
	}
	p := afkOrbitalPanelRect(w, hgt)
	title := "ORBITAL EXTRACTOR // LOCKED"
	subtitle := "ENGINEER TRAINING + VOID PIERCERS ARE ONLINE IN THE TECH LAB"
	if afkOrbitalBuilt() {
		title = fmt.Sprintf("ORBITAL EXTRACTOR // TIER %d", afkOrbitalTier())
		subtitle = fmt.Sprintf("BURST %s STARBITS // NEXT FIRE %s // FIRES %d", formatAFKNumber(afkOrbitalBurstStarbits()), afkFormatDuration(afkOrbitalNextFireIn(time.Now())), gameMeta.AFKOrbitalBurstsFired)
	}
	drawAFKManagementChrome(hdc, p, title, subtitle, rgb(176, 89, 255), w, hgt)
	if !afkOrbitalBuilt() {
		return
	}
	for i, d := range afkOrbitalUpgradeDefs {
		r := afkOrbitalUpgradeRect(w, hgt, i)
		border := d.Accent
		if i == afkOrbitalSelectedUpgrade {
			border = rgb(246, 101, 255)
		}
		drawBevelPanel(hdc, r, rgb(7, 26, 54), border, rgb(0, 5, 14), 2)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, d.Accent)
			centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(16, hgt), d.Name)
			setTextColor.Call(hdc, rgb(232, 241, 248))
			centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(45, hgt), fmt.Sprintf("LV.%d", afkOrbitalUpgradeLevel(i)))
			setTextColor.Call(hdc, rgb(116, 154, 180))
			centeredTextOut(hdc, r.Left+sx(8, w), r.Right-sx(8, w), r.Top+sy(76, hgt), d.Description)
			selectObject.Call(hdc, old)
		}
	}
}
