//go:build windows

package main

import (
	"fmt"
	"math"
	"time"
)

// Section 9: Drone Bay. The reference game introduces a second autonomous
// layer after the extractor economy: a dedicated Hub, ten drones, target
// reticles, several independent upgrades, then Hub ascensions. Cursor Control
// keeps that structure but produces Starbits rather than combat damage.
const (
	afkDroneMaxCount              = 10
	afkDroneUpgradeCount          = 5
	afkDroneUpgradeMaxLevel       = 40
	afkDroneBayMaxTier            = 3
	afkDroneBayBuildCost    int64 = 80_000_000_000
)

type afkDroneUpgradeDef struct {
	Name, Description string
	Accent            uintptr
}

var afkDroneUpgradeDefs = [...]afkDroneUpgradeDef{
	{"DEPLOY DRONE", "Adds another autonomous miner to the swarm.", rgb(86, 218, 255)},
	{"BIGGER PROPELLERS", "More spin = more win. Drones cross targets faster.", rgb(104, 239, 175)},
	{"TARGETING AI", "Turns close-enough tracking into pixel-perfect bullseyes.", rgb(248, 205, 72)},
	{"OVERCHARGED CORES", "Raises Starbit extraction per successful target cycle.", rgb(255, 122, 91)},
	{"NEURAL SYNC", "Synchronises the swarm for a global Drone output bonus.", rgb(196, 108, 255)},
}

// Fully authored per-level costs. Index is current level and value is the cost
// of the next level. Deploy Drone uses its own ten-entry table below.
var afkDroneDeployCosts = [...]int64{
	1_000_000_000, 2_500_000_000, 6_000_000_000, 14_000_000_000, 32_000_000_000,
	70_000_000_000, 150_000_000_000, 320_000_000_000, 680_000_000_000, 1_400_000_000_000,
}

var afkDroneUpgradeCosts = [4][40]int64{
	{120_000_000, 160_000_000, 220_000_000, 300_000_000, 420_000_000, 580_000_000, 800_000_000, 1_100_000_000, 1_500_000_000, 2_100_000_000, 2_900_000_000, 4_000_000_000, 5_500_000_000, 7_500_000_000, 10_000_000_000, 14_000_000_000, 19_000_000_000, 26_000_000_000, 35_000_000_000, 48_000_000_000, 65_000_000_000, 88_000_000_000, 120_000_000_000, 165_000_000_000, 225_000_000_000, 305_000_000_000, 415_000_000_000, 565_000_000_000, 770_000_000_000, 1_050_000_000_000, 1_430_000_000_000, 1_950_000_000_000, 2_650_000_000_000, 3_600_000_000_000, 4_900_000_000_000, 6_650_000_000_000, 9_050_000_000_000, 12_300_000_000_000, 16_700_000_000_000, 22_700_000_000_000},
	{150_000_000, 205_000_000, 280_000_000, 385_000_000, 525_000_000, 720_000_000, 990_000_000, 1_360_000_000, 1_870_000_000, 2_570_000_000, 3_530_000_000, 4_850_000_000, 6_650_000_000, 9_100_000_000, 12_500_000_000, 17_100_000_000, 23_400_000_000, 32_000_000_000, 43_800_000_000, 60_000_000_000, 82_000_000_000, 112_000_000_000, 153_000_000_000, 209_000_000_000, 286_000_000_000, 391_000_000_000, 535_000_000_000, 731_000_000_000, 999_000_000_000, 1_365_000_000_000, 1_865_000_000_000, 2_548_000_000_000, 3_482_000_000_000, 4_758_000_000_000, 6_502_000_000_000, 8_885_000_000_000, 12_140_000_000_000, 16_590_000_000_000, 22_670_000_000_000, 30_980_000_000_000},
	{190_000_000, 260_000_000, 355_000_000, 485_000_000, 665_000_000, 910_000_000, 1_245_000_000, 1_705_000_000, 2_335_000_000, 3_195_000_000, 4_375_000_000, 5_985_000_000, 8_190_000_000, 11_210_000_000, 15_340_000_000, 20_990_000_000, 28_730_000_000, 39_310_000_000, 53_790_000_000, 73_600_000_000, 100_700_000_000, 137_800_000_000, 188_600_000_000, 258_100_000_000, 353_200_000_000, 483_400_000_000, 661_500_000_000, 905_200_000_000, 1_238_700_000_000, 1_695_100_000_000, 2_319_600_000_000, 3_174_100_000_000, 4_343_300_000_000, 5_943_000_000_000, 8_131_000_000_000, 11_124_000_000_000, 15_219_000_000_000, 20_821_000_000_000, 28_488_000_000_000, 38_977_000_000_000},
	{240_000_000, 330_000_000, 455_000_000, 625_000_000, 860_000_000, 1_180_000_000, 1_620_000_000, 2_225_000_000, 3_055_000_000, 4_195_000_000, 5_760_000_000, 7_905_000_000, 10_850_000_000, 14_890_000_000, 20_430_000_000, 28_030_000_000, 38_460_000_000, 52_770_000_000, 72_400_000_000, 99_340_000_000, 136_300_000_000, 187_000_000_000, 256_600_000_000, 352_100_000_000, 483_200_000_000, 663_100_000_000, 910_000_000_000, 1_248_800_000_000, 1_713_800_000_000, 2_351_900_000_000, 3_227_800_000_000, 4_429_800_000_000, 6_079_000_000_000, 8_342_000_000_000, 11_448_000_000_000, 15_710_000_000_000, 21_560_000_000_000, 29_590_000_000_000, 40_610_000_000_000, 55_730_000_000_000},
}

var afkDroneBayAscendCosts = [...]int64{0, 6_000_000_000_000, 250_000_000_000_000}
var afkDronePanelOpen bool
var afkDroneSelectedUpgrade int

func afkEnsureSection9SaveShape() {
	if gameMeta.AFKDroneBayTier < 0 {
		gameMeta.AFKDroneBayTier = 0
	}
	if gameMeta.AFKDroneBayTier > afkDroneBayMaxTier {
		gameMeta.AFKDroneBayTier = afkDroneBayMaxTier
	}
	if gameMeta.AFKDronesDeployed < 0 {
		gameMeta.AFKDronesDeployed = 0
	}
	if gameMeta.AFKDronesDeployed > afkDroneMaxCount {
		gameMeta.AFKDronesDeployed = afkDroneMaxCount
	}
	if len(gameMeta.AFKDroneUpgradeLevels) != afkDroneUpgradeCount {
		old := append([]int(nil), gameMeta.AFKDroneUpgradeLevels...)
		gameMeta.AFKDroneUpgradeLevels = make([]int, afkDroneUpgradeCount)
		copy(gameMeta.AFKDroneUpgradeLevels, old)
	}
	gameMeta.AFKDroneUpgradeLevels[0] = gameMeta.AFKDronesDeployed
	for i := 1; i < afkDroneUpgradeCount; i++ {
		if gameMeta.AFKDroneUpgradeLevels[i] < 0 {
			gameMeta.AFKDroneUpgradeLevels[i] = 0
		}
		if gameMeta.AFKDroneUpgradeLevels[i] > afkDroneUpgradeMaxLevel {
			gameMeta.AFKDroneUpgradeLevels[i] = afkDroneUpgradeMaxLevel
		}
	}
}
func afkDronesAvailable() bool { return afkCoreTier() >= 5 && gameMeta.AFKSection8Complete }
func afkDroneBayBuilt() bool   { return gameMeta.AFKDroneBayBuilt && gameMeta.AFKDroneBayTier > 0 }
func afkDroneBayTier() int     { afkEnsureSection9SaveShape(); return gameMeta.AFKDroneBayTier }
func afkBuildDroneBay() bool {
	if !afkDronesAvailable() || afkDroneBayBuilt() || gameMeta.AFKStarbits < afkDroneBayBuildCost {
		return false
	}
	if !afkActionAllowed("build_drone_bay", time.Now(), afkEconomyActionDebounce) || !afkSpendStarbits(afkDroneBayBuildCost) {
		return false
	}
	gameMeta.AFKDroneBayBuilt = true
	gameMeta.AFKDroneBayTier = 1
	afkEnsureSection9SaveShape()
	afkRefreshTalentPointAwards()
	playAFKConstructionSound()
	return true
}
func afkDroneUpgradeLevel(i int) int {
	afkEnsureSection9SaveShape()
	if i < 0 || i >= afkDroneUpgradeCount {
		return 0
	}
	return gameMeta.AFKDroneUpgradeLevels[i]
}
func afkDroneUpgradeCost(i int) int64 {
	if !afkDroneBayBuilt() || i < 0 || i >= afkDroneUpgradeCount {
		return 0
	}
	if i == 0 {
		if gameMeta.AFKDronesDeployed >= afkDroneMaxCount {
			return 0
		}
		return afkDroneDeployCosts[gameMeta.AFKDronesDeployed]
	}
	lvl := afkDroneUpgradeLevel(i)
	if lvl >= afkDroneUpgradeMaxLevel {
		return 0
	}
	return afkDroneUpgradeCosts[i-1][lvl]
}
func afkBuyDroneUpgrade(i int) bool {
	cost := afkDroneUpgradeCost(i)
	if cost <= 0 || gameMeta.AFKStarbits < cost {
		return false
	}
	if !afkActionAllowed(fmt.Sprintf("buy_drone_%d", i), time.Now(), afkEconomyActionDebounce) || !afkSpendStarbits(cost) {
		return false
	}
	if i == 0 {
		gameMeta.AFKDronesDeployed++
		gameMeta.AFKDroneUpgradeLevels[0] = gameMeta.AFKDronesDeployed
	} else {
		gameMeta.AFKDroneUpgradeLevels[i]++
	}
	afkRefreshSection9Completion()
	if i == 0 {
		playAFKDroneDeploySound()
	} else {
		playAFKUpgradeSound()
	}
	return true
}
func afkDroneBayCanAscend() bool {
	t := afkDroneBayTier()
	if t < 1 || t >= afkDroneBayMaxTier {
		return false
	}
	if gameMeta.AFKDronesDeployed < 10 {
		return false
	}
	// Like the reference, the Hub's basic deploy path must be mature before ascension.
	min := 10
	if t == 2 {
		min = 20
	}
	return afkDroneUpgradeLevel(1) >= min && afkDroneUpgradeLevel(2) >= min && afkDroneUpgradeLevel(3) >= min
}
func afkDroneBayAscendCost() int64 {
	t := afkDroneBayTier()
	if t < 1 || t >= afkDroneBayMaxTier {
		return 0
	}
	return afkDroneBayAscendCosts[t]
}
func afkAscendDroneBay() bool {
	c := afkDroneBayAscendCost()
	if c <= 0 || !afkDroneBayCanAscend() || gameMeta.AFKStarbits < c {
		return false
	}
	if !afkActionAllowed("ascend_drone_bay", time.Now(), afkClaimActionDebounce) || !afkSpendStarbits(c) {
		return false
	}
	gameMeta.AFKDroneBayTier++
	playAFKTierUpSound()
	afkRefreshSection9Completion()
	afkRefreshTalentPointAwards()
	return true
}
func afkDroneBasePerSecondMilli() int64 {
	if !afkDroneBayBuilt() || gameMeta.AFKDronesDeployed <= 0 {
		return 0
	}
	// 75k Starbits/s per drone at Tier 1 before upgrades; Hub tiers are major jumps.
	v := int64(gameMeta.AFKDronesDeployed) * 75_000 * 1000
	tierMul := []int64{0, 1000, 3000, 8000}[afkDroneBayTier()]
	v = v * tierMul / 1000
	v = v * (1000 + int64(afkDroneUpgradeLevel(1))*10) / 1000
	v = v * (1000 + int64(afkDroneUpgradeLevel(2))*10) / 1000
	v = v * (1000 + int64(afkDroneUpgradeLevel(3))*15) / 1000
	v = v * (1000 + int64(afkDroneUpgradeLevel(4))*10) / 1000
	v = v * afkAdvancedDroneResearchMultiplierMilli() / 1000
	v = v * afkOperatorDroneMultiplierMilli() / 1000
	v = v * afkModuleDroneOrbitalMultiplierMilli() / 1000
	v = v * afkTalentDroneMultiplierMilli() / 1000
	return v
}
func afkDroneStarbitsPerSecondMilli() int64 { return afkDroneBasePerSecondMilli() }
func afkDroneOutputText() string {
	v := afkDroneStarbitsPerSecondMilli()
	if v <= 0 {
		return "0"
	}
	return fmt.Sprintf("%s/s", formatAFKNumber(v/1000))
}
func afkRefreshSection9Completion() {
	if afkDroneBayTier() >= 3 && gameMeta.AFKDronesDeployed >= 10 && afkDroneUpgradeLevel(1) >= 25 && afkDroneUpgradeLevel(2) >= 20 && afkDroneUpgradeLevel(3) >= 20 {
		gameMeta.AFKSection9Complete = true
	}
}
func afkSection9Objective() (string, string, string, int64, int64, bool) {
	if !afkDroneBayBuilt() {
		return "BUILD DRONE BAY", "BRING THE SECOND AUTONOMOUS", "STARBIT LAYER ONLINE.", gameMeta.AFKStarbits, afkDroneBayBuildCost, false
	}
	if gameMeta.AFKDronesDeployed < 10 {
		return "DEPLOY 10 DRONES", "FILL THE DRONE BAY'S FIRST", "AUTONOMOUS MINING ARRAY.", int64(gameMeta.AFKDronesDeployed), 10, false
	}
	if afkDroneUpgradeLevel(1) < 10 {
		return "BIGGER PROPELLERS LV. 10", "INCREASE DRONE TRAVEL SPEED", "BETWEEN TARGET RETICLES.", int64(afkDroneUpgradeLevel(1)), 10, false
	}
	if afkDroneBayTier() < 2 {
		return "ASCEND DRONE BAY // TIER 2", fmt.Sprintf("STORE %s STARBITS", formatAFKNumber(afkDroneBayAscendCost())), "MATURE DEPLOY + TARGETING SYSTEMS.", gameMeta.AFKStarbits, afkDroneBayAscendCost(), false
	}
	if afkDroneUpgradeLevel(1) < 25 {
		return "BIGGER PROPELLERS LV. 25", "PUSH THE TIER 2 SWARM", "INTO HIGH-SPEED EXTRACTION.", int64(afkDroneUpgradeLevel(1)), 25, false
	}
	if afkDroneBayTier() < 3 {
		return "ASCEND DRONE BAY // TIER 3", fmt.Sprintf("STORE %s STARBITS", formatAFKNumber(afkDroneBayAscendCost())), "ADVANCED TARGETING REQUIRED.", gameMeta.AFKStarbits, afkDroneBayAscendCost(), false
	}
	if !gameMeta.AFKSection9Complete {
		return "STABILISE DRONE BAY TIER 3", "RAISE TARGETING + CORE SYSTEMS", "TO PREPARE ORBITAL EXTRACTION.", int64(afkDroneUpgradeLevel(2) + afkDroneUpgradeLevel(3)), 40, false
	}
	return "SECTION 9 COMPLETE", "DRONE BAY TIER 3 ONLINE.", "NEXT LINK: ORBITAL EXTRACTOR", 1, 1, true
}

func afkDronePanelRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }
func afkDroneUpgradeRect(w, hgt int32, i int) RECT {
	p := afkDronePanelRect(w, hgt)
	gap := sx(12, w)
	left := p.Left + sx(28, w)
	right := p.Right - sx(28, w)
	bw := (right - left - gap*4) / 5
	x := left + int32(i)*(bw+gap)
	return RECT{x, p.Top + sy(132, hgt), x + bw, p.Bottom - sy(42, hgt)}
}
func drawAFKDroneSprite(hdc uintptr, x, y int32, scale int32, accent uintptr) {
	if scale < 2 {
		scale = 2
	}
	fillSolidRect(hdc, RECT{x - scale*3, y - scale, x + scale*3, y + scale}, rgb(38, 55, 79))
	fillSolidRect(hdc, RECT{x - scale, y - scale*2, x + scale, y + scale*2}, accent)
	fillSolidRect(hdc, RECT{x - scale*5, y - scale*2, x - scale*2, y - scale}, rgb(171, 193, 207))
	fillSolidRect(hdc, RECT{x + scale*2, y - scale*2, x + scale*5, y - scale}, rgb(171, 193, 207))
	fillSolidRect(hdc, RECT{x - scale/2, y - scale/2, x + scale/2 + 1, y + scale/2 + 1}, rgb(244, 248, 255))
}
func drawAFKDroneReticle(hdc uintptr, x, y, r int32, accent uintptr) {
	drawLineSimple(hdc, x-r, y, x-r/2, y, 1, accent)
	drawLineSimple(hdc, x+r/2, y, x+r, y, 1, accent)
	drawLineSimple(hdc, x, y-r, x, y-r/2, 1, accent)
	drawLineSimple(hdc, x, y+r/2, x, y+r, 1, accent)
	drawLineSimple(hdc, x-r/3, y-r/3, x-r/3+1, y-r/3+1, 2, accent)
}
func drawAFKDroneWorld(hdc uintptr, w, hgt int32) {
	if !afkDroneBayBuilt() || gameMeta.AFKDronesDeployed <= 0 || afkDronePanelOpen {
		return
	}
	cx, cy, rad := afkSingularityGeometry(w, hgt)
	now := float64(time.Now().UnixNano()) / 1e9
	n := gameMeta.AFKDronesDeployed
	for i := 0; i < n; i++ {
		a := now*(0.42+float64(afkDroneUpgradeLevel(1))*0.004) + float64(i)*6.283185/float64(n)
		rr := float64(rad) * (0.34 + 0.18*float64((i%3))/2)
		tx := int32(cx + math.Cos(a*0.73+float64(i))*rr*0.70)
		ty := int32(cy + math.Sin(a*0.91+float64(i))*rr*0.50)
		dx := int32(cx + math.Cos(a)*rr)
		dy := int32(cy + math.Sin(a)*rr*0.65)
		drawAFKDroneReticle(hdc, tx, ty, sx(11, w), rgb(95, 226, 255))
		drawAFKDroneSprite(hdc, dx, dy, max32(2, sx(3, w)), rgb(122, 109, 255))

		// Mining fire is deliberately asynchronous. Every drone owns a phase offset,
		// producing short travelling bolts instead of a synchronized wall of beams.
		period := 1.35 + float64(i%4)*0.11
		firePhase := math.Mod(now+float64(i)*0.37, period)
		if firePhase < 0.34 {
			t := firePhase / 0.34
			bx := int32(float64(dx) + (float64(tx-dx) * t))
			by := int32(float64(dy) + (float64(ty-dy) * t))
			// faint tracer, bright bolt and a tiny recoil flare at the drone
			drawLineSimple(hdc, dx, dy, bx, by, int(max32(1, sx(2, w))), rgb(44, 119, 166))
			drawCircleOutline(hdc, bx, by, max32(2, sx(4, w)), max32(1, sx(2, w)), rgb(118, 238, 255))
			fillSolidRect(hdc, RECT{bx - sx(2, w), by - sy(2, hgt), bx + sx(3, w), by + sy(3, hgt)}, rgb(237, 253, 255))
			fillSolidRect(hdc, RECT{dx - sx(2, w), dy - sy(2, hgt), dx + sx(3, w), dy + sy(3, hgt)}, rgb(179, 115, 255))
			if t > 0.82 {
				pulse := max32(3, sx(7, w))
				drawCircleOutline(hdc, tx, ty, pulse, max32(1, sx(2, w)), rgb(196, 103, 255))
			}
		} else if i%2 == 0 {
			drawLineSimple(hdc, dx, dy, tx, ty, 1, rgb(31, 68, 103))
		}
	}
}
func drawAFKDrones(hdc uintptr, w, hgt int32) {
	if !afkDronePanelOpen {
		return
	}
	p := afkDronePanelRect(w, hgt)
	drawAFKManagementChrome(hdc, p, fmt.Sprintf("DRONE BAY // TIER %d", afkDroneBayTier()), fmt.Sprintf("DRONES %d/10 // OUTPUT %s // SELECT AN UPGRADE PATH", gameMeta.AFKDronesDeployed, afkDroneOutputText()), rgb(101, 93, 207), w, hgt)
	for i, d := range afkDroneUpgradeDefs {
		r := afkDroneUpgradeRect(w, hgt, i)
		sel := i == afkDroneSelectedUpgrade
		border := d.Accent
		if sel {
			border = rgb(246, 101, 255)
		}
		drawBevelPanel(hdc, r, rgb(7, 26, 54), border, rgb(0, 5, 14), 2)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, d.Accent)
			centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(16, hgt), d.Name)
			setTextColor.Call(hdc, rgb(232, 241, 248))
			centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(45, hgt), fmt.Sprintf("LV.%d", afkDroneUpgradeLevel(i)))
			setTextColor.Call(hdc, rgb(116, 154, 180))
			centeredTextOut(hdc, r.Left+sx(8, w), r.Right-sx(8, w), r.Top+sy(76, hgt), d.Description)
			selectObject.Call(hdc, old)
		}
	}
}
