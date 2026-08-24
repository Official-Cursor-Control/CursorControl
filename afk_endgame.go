//go:build windows

package main

import (
	"fmt"
	"time"
)

// Section 11 — Stellar Prestige. This is the repeatable endgame sink after the
// Orbital Extractor loop. It uses Starbits only: no second currency is created.
const afkPrestigeMaxRank = 10

var afkPrestigeCosts = [...]int64{
	0,
	25_000_000_000_000_000,
	60_000_000_000_000_000,
	140_000_000_000_000_000,
	300_000_000_000_000_000,
	600_000_000_000_000_000,
	1_100_000_000_000_000_000,
	1_900_000_000_000_000_000,
	3_000_000_000_000_000_000,
	4_500_000_000_000_000_000,
	6_500_000_000_000_000_000,
}

var afkPrestigePanelOpen bool
var afkPrestigePulseStarted time.Time

func afkPrestigeAvailable() bool { return gameMeta.AFKSection10Complete }
func afkPrestigeRank() int {
	if gameMeta.AFKPrestigeRank < 0 {
		gameMeta.AFKPrestigeRank = 0
	}
	if gameMeta.AFKPrestigeRank > afkPrestigeMaxRank {
		gameMeta.AFKPrestigeRank = afkPrestigeMaxRank
	}
	return gameMeta.AFKPrestigeRank
}
func afkPrestigeNextCost() int64 {
	r := afkPrestigeRank()
	if r >= afkPrestigeMaxRank {
		return 0
	}
	return afkPrestigeCosts[r+1]
}
func afkPrestigeMultiplierMilli() int64 {
	// Each rank permanently adds +5% to the entire Starbit network.
	// Prestige remains valuable without becoming another runaway global multiplier.
	return 1000 + int64(afkPrestigeRank())*50
}
func afkPrestigeCacheMultiplierMilli() int64 {
	// Cache rewards gain +5% per rank as a secondary endgame benefit.
	return 1000 + int64(afkPrestigeRank())*50
}
func afkCanPrestige() bool {
	c := afkPrestigeNextCost()
	return afkPrestigeAvailable() && c > 0 && gameMeta.AFKStarbits >= c
}
func afkBuyPrestige(now time.Time) bool {
	if !afkCanPrestige() || !afkActionAllowed("prestige", now, afkClaimActionDebounce) {
		return false
	}
	c := afkPrestigeNextCost()
	if !afkSpendStarbits(c) {
		return false
	}
	gameMeta.AFKPrestigeRank++
	gameMeta.AFKPrestigePurchases++
	afkRefreshTalentPointAwards()
	if gameMeta.AFKPrestigeRank >= 3 {
		gameMeta.AFKSection11Complete = true
	}
	afkPrestigePulseStarted = now
	afkQueueMilestone(fmt.Sprintf("STELLAR PRESTIGE %d", gameMeta.AFKPrestigeRank), "+5% ALL STARBIT PRODUCTION")
	playAFKPrestigeSound()
	return true
}
func afkSection11Objective() (string, string, string, int64, int64, bool) {
	if !afkPrestigeAvailable() {
		return "STELLAR PRESTIGE LOCKED", "COMPLETE THE ORBITAL NETWORK", "THEN BEGIN ENDGAME CALIBRATION.", 0, 1, false
	}
	if afkPrestigeRank() < 3 {
		return fmt.Sprintf("STELLAR PRESTIGE // RANK %d", afkPrestigeRank()+1),
			fmt.Sprintf("STORE %s STARBITS", formatAFKNumber(afkPrestigeNextCost())),
			"PERMANENT +5% ALL STARBIT PRODUCTION.", gameMeta.AFKStarbits, afkPrestigeNextCost(), false
	}
	gameMeta.AFKSection11Complete = true
	return "ENDGAME NETWORK ONLINE", fmt.Sprintf("STELLAR PRESTIGE RANK %d", afkPrestigeRank()), "RANKS 4-10 REMAIN AS LONG-TERM GOALS.", 1, 1, true
}

func drawAFKPrestige(hdc uintptr, w, hgt int32) {
	if !afkPrestigePanelOpen {
		return
	}
	field := afkMainFieldRect(w, hgt)
	fillSolidRect(hdc, field, rgb(2, 7, 20))
	drawAFKPixelStarfield(hdc, field, w, hgt)
	cx := (field.Left + field.Right) / 2
	cy := field.Top + (field.Bottom-field.Top)/2
	// hard-coded 8-bit star core / prestige emblem
	s := max32(2, sx(4, w))
	col := rgb(242, 192, 55)
	fillSolidRect(hdc, RECT{cx - s*2, cy - s*8, cx + s*2, cy + s*8}, col)
	fillSolidRect(hdc, RECT{cx - s*8, cy - s*2, cx + s*8, cy + s*2}, col)
	fillSolidRect(hdc, RECT{cx - s*5, cy - s*5, cx + s*5, cy + s*5}, rgb(129, 86, 214))
	fillSolidRect(hdc, RECT{cx - s*2, cy - s*2, cx + s*2, cy + s*2}, rgb(235, 248, 255))
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(242, 192, 55))
		centeredTextOut(hdc, field.Left, field.Right, field.Top+sy(42, hgt), "STELLAR PRESTIGE")
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(200, 221, 239))
		centeredTextOut(hdc, field.Left, field.Right, field.Top+sy(95, hgt), fmt.Sprintf("RANK %d / %d  //  ALL PRODUCTION x%.2f", afkPrestigeRank(), afkPrestigeMaxRank, float64(afkPrestigeMultiplierMilli())/1000.0))
		centeredTextOut(hdc, field.Left, field.Right, field.Bottom-sy(74, hgt), "PRESTIGE SPENDS STARBITS ONLY // FACILITIES AND UPGRADES ARE NEVER RESET")
		selectObject.Call(hdc, old)
	}
}
