//go:build windows

package main

import (
	"fmt"
	"math"
	"time"
)

// v443 Operator assignments: Operators no longer gain Service XP just because
// Starbase systems are running. A recruited specialist must be manually assigned
// work by the player, each assignment costs Starbits, takes a role-specific amount
// of time, then enters a role-specific cooldown. Completed assignments award
// Service XP, but levels are still NEVER awarded automatically: the player must
// physically click a READY Operator to promote them.
const afkOperatorMaxLevel = 10

// Persisted thresholds intentionally keep the old scale so existing v441 saves
// migrate cleanly. The values are Service XP points now, not elapsed seconds.
var afkOperatorLevelXPSeconds = [...]int64{
	0,         // Lv.1
	20 * 60,   // Lv.2
	50 * 60,   // Lv.3
	100 * 60,  // Lv.4
	180 * 60,  // Lv.5
	300 * 60,  // Lv.6
	480 * 60,  // Lv.7
	720 * 60,  // Lv.8
	1020 * 60, // Lv.9
	1440 * 60, // Lv.10
}

type afkOperatorDef struct {
	Name            string
	Role            string
	Description     string
	RecruitCost     int64
	Accent          uintptr
	AssignmentName  string
	AssignmentCost  int64
	WorkSeconds     int64
	CooldownSeconds int64
}

var afkOperatorDefs = [...]afkOperatorDef{
	// v446: longer, deliberately staggered assignments prevent a cluster of
	// Operators finishing together even when the player starts them in one visit.
	{"NOVA", "CORE ENGINEER", "Runs the Cursor Core and improves total Starbit output.", 25_000_000, rgb(77, 218, 255), "CORE CALIBRATION", 750_000, 15 * 60, 7 * 60},
	{"VEGA", "SCOUT NAVIGATOR", "Plots safer routes and shortens future expeditions.", 60_000_000, rgb(114, 234, 176), "ROUTE SURVEY", 1_500_000, 23 * 60, 10 * 60},
	{"BYTE", "TECH ANALYST", "Analyses Starbase systems and reduces future research costs.", 150_000_000, rgb(190, 104, 255), "SYSTEM ANALYSIS", 3_000_000, 34 * 60, 14 * 60},
	{"ASTRA", "QUARTERMASTER", "Audits supplies and improves recovered Starbit rewards.", 400_000_000, rgb(242, 192, 55), "SUPPLY AUDIT", 6_000_000, 48 * 60, 19 * 60},
	{"FLUX", "DRONE TECHNICIAN", "Services mining drones and raises Drone output.", 1_200_000_000, rgb(255, 116, 122), "DRONE SERVICE", 12_000_000, 65 * 60, 25 * 60},
}

var (
	afkOperatorPanelOpen    bool
	afkOperatorSelected     int
	afkOperatorLastXPGain   [5]int64
	afkOperatorLastXPReason [5]string
	afkOperatorLastXPAt     [5]time.Time
)

func afkLegacyOperatorLevelFromXP(xp int64) int {
	lvl := 1
	for j := 1; j < len(afkOperatorLevelXPSeconds); j++ {
		if xp >= afkOperatorLevelXPSeconds[j] {
			lvl = j + 1
		} else {
			break
		}
	}
	if lvl > afkOperatorMaxLevel {
		lvl = afkOperatorMaxLevel
	}
	return lvl
}

func afkEnsureSection7SaveShape() {
	n := len(afkOperatorDefs)
	if len(gameMeta.AFKOperatorsRecruited) != n {
		old := append([]bool(nil), gameMeta.AFKOperatorsRecruited...)
		gameMeta.AFKOperatorsRecruited = make([]bool, n)
		copy(gameMeta.AFKOperatorsRecruited, old)
	}
	if len(gameMeta.AFKOperatorXPSeconds) != n {
		old := append([]int64(nil), gameMeta.AFKOperatorXPSeconds...)
		gameMeta.AFKOperatorXPSeconds = make([]int64, n)
		copy(gameMeta.AFKOperatorXPSeconds, old)
	}
	oldLevelLen := len(gameMeta.AFKOperatorLevels)
	if oldLevelLen != n {
		old := append([]int(nil), gameMeta.AFKOperatorLevels...)
		gameMeta.AFKOperatorLevels = make([]int, n)
		copy(gameMeta.AFKOperatorLevels, old)
	}
	if len(gameMeta.AFKOperatorWorkStartedUnix) != n {
		old := append([]int64(nil), gameMeta.AFKOperatorWorkStartedUnix...)
		gameMeta.AFKOperatorWorkStartedUnix = make([]int64, n)
		copy(gameMeta.AFKOperatorWorkStartedUnix, old)
	}
	if len(gameMeta.AFKOperatorWorkEndsUnix) != n {
		old := append([]int64(nil), gameMeta.AFKOperatorWorkEndsUnix...)
		gameMeta.AFKOperatorWorkEndsUnix = make([]int64, n)
		copy(gameMeta.AFKOperatorWorkEndsUnix, old)
	}
	if len(gameMeta.AFKOperatorCooldownEndsUnix) != n {
		old := append([]int64(nil), gameMeta.AFKOperatorCooldownEndsUnix...)
		gameMeta.AFKOperatorCooldownEndsUnix = make([]int64, n)
		copy(gameMeta.AFKOperatorCooldownEndsUnix, old)
	}
	maxXP := afkOperatorLevelXPSeconds[len(afkOperatorLevelXPSeconds)-1]
	recruitedCount := 0
	for i := 0; i < n; i++ {
		if gameMeta.AFKOperatorXPSeconds[i] < 0 {
			gameMeta.AFKOperatorXPSeconds[i] = 0
		}
		if gameMeta.AFKOperatorXPSeconds[i] > maxXP {
			gameMeta.AFKOperatorXPSeconds[i] = maxXP
		}
		if gameMeta.AFKOperatorWorkStartedUnix[i] < 0 {
			gameMeta.AFKOperatorWorkStartedUnix[i] = 0
		}
		if gameMeta.AFKOperatorWorkEndsUnix[i] < 0 {
			gameMeta.AFKOperatorWorkEndsUnix[i] = 0
		}
		if gameMeta.AFKOperatorCooldownEndsUnix[i] < 0 {
			gameMeta.AFKOperatorCooldownEndsUnix[i] = 0
		}
		if gameMeta.AFKOperatorsRecruited[i] {
			recruitedCount++
			// Migration from v441 and older: preserve the level the player's
			// already-earned Service XP had reached before levels became manual.
			if gameMeta.AFKOperatorLevels[i] <= 0 {
				gameMeta.AFKOperatorLevels[i] = afkLegacyOperatorLevelFromXP(gameMeta.AFKOperatorXPSeconds[i])
			}
			if gameMeta.AFKOperatorLevels[i] < 1 {
				gameMeta.AFKOperatorLevels[i] = 1
			}
			if gameMeta.AFKOperatorLevels[i] > afkOperatorMaxLevel {
				gameMeta.AFKOperatorLevels[i] = afkOperatorMaxLevel
			}
			// v443 deliberately allows at most one pending promotion. Old v442 saves
			// could bank XP beyond the visible target, which overflowed Byte's bar and
			// enabled several rapid promotions. Clamp that legacy surplus to the next
			// threshold so every future level requires fresh manual assignments.
			lvl := gameMeta.AFKOperatorLevels[i]
			if lvl > 0 && lvl < afkOperatorMaxLevel {
				capXP := afkOperatorLevelXPSeconds[lvl]
				if gameMeta.AFKOperatorXPSeconds[i] > capXP {
					gameMeta.AFKOperatorXPSeconds[i] = capXP
				}
			}
		} else if i >= oldLevelLen || gameMeta.AFKOperatorLevels[i] < 0 {
			gameMeta.AFKOperatorLevels[i] = 0
		}
	}
	// Keep the convenience counter coherent with the authoritative bool slice.
	gameMeta.AFKOperatorsRecruitedCount = recruitedCount
}

func afkOperatorsAvailable() bool {
	return afkCoreTier() >= 4 && gameMeta.AFKSection6Complete
}

func afkOperatorRecruited(i int) bool {
	afkEnsureSection7SaveShape()
	return i >= 0 && i < len(gameMeta.AFKOperatorsRecruited) && gameMeta.AFKOperatorsRecruited[i]
}

func afkOperatorLevel(i int) int {
	afkEnsureSection7SaveShape()
	if i < 0 || i >= len(afkOperatorDefs) || !gameMeta.AFKOperatorsRecruited[i] {
		return 0
	}
	lvl := gameMeta.AFKOperatorLevels[i]
	if lvl < 1 {
		lvl = 1
	}
	if lvl > afkOperatorMaxLevel {
		lvl = afkOperatorMaxLevel
	}
	return lvl
}

func afkOperatorXPToNext(i int) (current, target int64) {
	if !afkOperatorRecruited(i) {
		return 0, 0
	}
	lvl := afkOperatorLevel(i)
	if lvl >= afkOperatorMaxLevel {
		return 0, 0
	}
	base := afkOperatorLevelXPSeconds[lvl-1]
	next := afkOperatorLevelXPSeconds[lvl]
	xp := gameMeta.AFKOperatorXPSeconds[i]
	if xp < base {
		xp = base
	}
	current = xp - base
	target = next - base
	if current > target {
		current = target
	}
	return current, target
}

func afkOperatorCanLevel(i int) bool {
	if !afkOperatorRecruited(i) {
		return false
	}
	lvl := afkOperatorLevel(i)
	if lvl <= 0 || lvl >= afkOperatorMaxLevel {
		return false
	}
	return gameMeta.AFKOperatorXPSeconds[i] >= afkOperatorLevelXPSeconds[lvl]
}

func afkOperatorRecruitRequirementMet(i int) bool {
	switch i {
	case 0: // Nova
		return afkCursorCoreBuilt()
	case 1: // Vega
		return afkScoutUnlocked()
	case 2: // Byte
		return afkTechLabUnlocked()
	case 3: // Astra
		return afkModulesAvailable()
	case 4: // Flux
		return afkDroneBayBuilt()
	}
	return false
}

func afkOperatorRecruitRequirementText(i int) string {
	switch i {
	case 0:
		return "REQUIRES CURSOR CORE"
	case 1:
		return "REQUIRES SCOUT PAD"
	case 2:
		return "REQUIRES TECH LAB"
	case 3:
		return "REQUIRES FABRICATOR"
	case 4:
		return "REQUIRES DRONE BAY"
	}
	return "REQUIREMENT UNKNOWN"
}

func afkOperatorWorkText(i int) string {
	if i < 0 || i >= len(afkOperatorDefs) {
		return "ASSIGNMENT"
	}
	return afkOperatorDefs[i].AssignmentName
}

func afkRecruitOperator(i int) bool {
	afkEnsureSection7SaveShape()
	if !afkOperatorsAvailable() || i < 0 || i >= len(afkOperatorDefs) || afkOperatorRecruited(i) || !afkOperatorRecruitRequirementMet(i) {
		return false
	}
	cost := afkOperatorDefs[i].RecruitCost
	if gameMeta.AFKStarbits < cost {
		return false
	}
	if !afkActionAllowed(fmt.Sprintf("recruit_operator_%d", i), time.Now(), afkEconomyActionDebounce) || !afkSpendStarbits(cost) {
		return false
	}
	gameMeta.AFKOperatorsRecruited[i] = true
	gameMeta.AFKOperatorXPSeconds[i] = 0
	gameMeta.AFKOperatorLevels[i] = 1
	gameMeta.AFKOperatorWorkStartedUnix[i] = 0
	gameMeta.AFKOperatorWorkEndsUnix[i] = 0
	gameMeta.AFKOperatorCooldownEndsUnix[i] = 0
	gameMeta.AFKOperatorsRecruitedCount++
	afkRefreshSection7Completion()
	afkRefreshTalentPointAwards()
	afkQueueOperatorRecruitCelebration(i)
	return true
}

func afkGrantOperatorServiceXP(i int, amount int64, reason string) {
	if amount <= 0 || i < 0 || i >= len(afkOperatorDefs) || !afkOperatorRecruited(i) {
		return
	}
	lvl := afkOperatorLevel(i)
	if lvl <= 0 || lvl >= afkOperatorMaxLevel || afkOperatorCanLevel(i) {
		return
	}
	amount = amount * afkModuleOperatorXPMultiplierMilli() / 1000
	if amount < 1 {
		amount = 1
	}
	// Service XP stops exactly at the next promotion threshold. The player must
	// promote before another assignment can be started; XP can never silently
	// leapfrog multiple levels while the player is away.
	capXP := afkOperatorLevelXPSeconds[lvl]
	before := gameMeta.AFKOperatorXPSeconds[i]
	gameMeta.AFKOperatorXPSeconds[i] = saturatingAdd(before, amount)
	if gameMeta.AFKOperatorXPSeconds[i] > capXP {
		gameMeta.AFKOperatorXPSeconds[i] = capXP
	}
	gained := gameMeta.AFKOperatorXPSeconds[i] - before
	if gained > 0 {
		afkOperatorLastXPGain[i] = gained
		afkOperatorLastXPReason[i] = reason
		afkOperatorLastXPAt[i] = time.Now()
	}
}

func afkOperatorAssignmentCost(i int) int64 {
	if i < 0 || i >= len(afkOperatorDefs) || !afkOperatorRecruited(i) {
		return 0
	}
	lvl := afkOperatorLevel(i)
	if lvl < 1 {
		lvl = 1
	}
	// Assignment cost rises gently with experience so late-game specialists still
	// consume meaningful Starbits without becoming an economy sink.
	return afkOperatorDefs[i].AssignmentCost * int64(100+(lvl-1)*22) / 100
}

func afkOperatorAssignmentXP(i int) int64 {
	if i < 0 || i >= len(afkOperatorDefs) || !afkOperatorRecruited(i) {
		return 0
	}
	lvl := afkOperatorLevel(i)
	if lvl < 1 || lvl >= afkOperatorMaxLevel {
		return 0
	}
	base := afkOperatorLevelXPSeconds[lvl-1]
	next := afkOperatorLevelXPSeconds[lvl]
	need := next - base
	// Four completed assignments is the baseline for one level. The Operator
	// Locker module's Service-XP bonus can reduce this to roughly three.
	return (need + 3) / 4
}

func afkOperatorWorkDurationSeconds(i int) int64 {
	if i < 0 || i >= len(afkOperatorDefs) {
		return 0
	}
	lvl := afkOperatorLevel(i)
	if lvl < 1 {
		lvl = 1
	}
	return afkOperatorDefs[i].WorkSeconds + int64(lvl-1)*30
}

func afkOperatorCooldownSeconds(i int) int64 {
	if i < 0 || i >= len(afkOperatorDefs) {
		return 0
	}
	lvl := afkOperatorLevel(i)
	if lvl < 1 {
		lvl = 1
	}
	return afkOperatorDefs[i].CooldownSeconds + int64(lvl-1)*20
}

func afkOperatorAssignmentActive(i int, now time.Time) bool {
	afkEnsureSection7SaveShape()
	return i >= 0 && i < len(afkOperatorDefs) && gameMeta.AFKOperatorWorkEndsUnix[i] > now.Unix()
}

func afkOperatorCooldownActive(i int, now time.Time) bool {
	afkEnsureSection7SaveShape()
	if i < 0 || i >= len(afkOperatorDefs) || afkOperatorAssignmentActive(i, now) {
		return false
	}
	return gameMeta.AFKOperatorCooldownEndsUnix[i] > now.Unix()
}

func afkOperatorCanStartAssignment(i int, now time.Time) bool {
	if !afkOperatorRecruited(i) || afkOperatorLevel(i) >= afkOperatorMaxLevel || afkOperatorCanLevel(i) {
		return false
	}
	if afkOperatorAssignmentActive(i, now) || afkOperatorCooldownActive(i, now) {
		return false
	}
	return gameMeta.AFKStarbits >= afkOperatorAssignmentCost(i)
}

func afkStartOperatorAssignment(i int, now time.Time) bool {
	if i < 0 || i >= len(afkOperatorDefs) || !afkOperatorCanStartAssignment(i, now) {
		return false
	}
	cost := afkOperatorAssignmentCost(i)
	if !afkActionAllowed(fmt.Sprintf("operator_assignment_%d", i), now, afkEconomyActionDebounce) || !afkSpendStarbits(cost) {
		return false
	}
	work := afkOperatorWorkDurationSeconds(i)
	cooldown := afkOperatorCooldownSeconds(i)
	gameMeta.AFKOperatorWorkStartedUnix[i] = now.Unix()
	gameMeta.AFKOperatorWorkEndsUnix[i] = now.Unix() + work
	gameMeta.AFKOperatorCooldownEndsUnix[i] = now.Unix() + work + cooldown
	afkOperatorLastXPReason[i] = "ASSIGNMENT STARTED // " + afkOperatorDefs[i].AssignmentName
	afkOperatorLastXPAt[i] = now
	return true
}

// Returns true when persisted state changed. Assignments are single-shot: a
// completed job awards XP exactly once, then the Operator waits out the cooldown.
// Nothing automatically restarts after the cooldown.
func afkRefreshOperatorAssignments(now time.Time) bool {
	afkEnsureSection7SaveShape()
	changed := false
	for i := range afkOperatorDefs {
		end := gameMeta.AFKOperatorWorkEndsUnix[i]
		if end <= 0 || now.Unix() < end {
			continue
		}
		gameMeta.AFKOperatorWorkStartedUnix[i] = 0
		gameMeta.AFKOperatorWorkEndsUnix[i] = 0
		xp := afkOperatorAssignmentXP(i)
		if xp > 0 {
			afkGrantOperatorServiceXP(i, xp, afkOperatorDefs[i].AssignmentName+" COMPLETE")
		}
		changed = true
	}
	return changed
}

func afkOperatorAssignmentStatus(i int, now time.Time) string {
	if !afkOperatorRecruited(i) {
		return "UNRECRUITED"
	}
	if afkOperatorCanLevel(i) {
		return "LEVEL UP"
	}
	if afkOperatorLevel(i) >= afkOperatorMaxLevel {
		return "MAX LEVEL"
	}
	if afkOperatorAssignmentActive(i, now) {
		return fmt.Sprintf("%s // %s", afkOperatorDefs[i].AssignmentName, afkFormatDuration(gameMeta.AFKOperatorWorkEndsUnix[i]-now.Unix()))
	}
	if afkOperatorCooldownActive(i, now) {
		return fmt.Sprintf("COOLDOWN // %s", afkFormatDuration(gameMeta.AFKOperatorCooldownEndsUnix[i]-now.Unix()))
	}
	cost := afkOperatorAssignmentCost(i)
	if gameMeta.AFKStarbits < cost {
		return fmt.Sprintf("NEED %s STARBITS", formatAFKNumber(cost))
	}
	return fmt.Sprintf("START // %s", formatAFKNumber(cost))
}

// Compatibility helpers retained for legacy call sites/tests. v443 deliberately
// gives no passive Service XP from live production or unrelated facility events.
func afkAccumulateOperatorLiveDuration(d time.Duration) { _ = d }
func afkAccumulateOperatorXP(points int64)              { _ = points }

func afkLevelUpOperator(i int) bool {
	if !afkOperatorCanLevel(i) || !afkActionAllowed(fmt.Sprintf("level_operator_%d", i), time.Now(), 180*time.Millisecond) {
		return false
	}
	gameMeta.AFKOperatorLevels[i]++
	if gameMeta.AFKOperatorLevels[i] > afkOperatorMaxLevel {
		gameMeta.AFKOperatorLevels[i] = afkOperatorMaxLevel
	}
	lvl := gameMeta.AFKOperatorLevels[i]
	afkRefreshSection7Completion()
	afkRefreshTalentPointAwards()
	afkQueueOperatorLevelQuip(i, lvl)
	return true
}

func afkOperatorStarbitProductionMultiplierMilli() int64 {
	lvl := afkOperatorLevel(0)
	if lvl <= 0 {
		return 1000
	}
	return 1000 + int64(lvl)*20
}
func afkOperatorExpeditionMultiplierMilli() int64 {
	lvl := afkOperatorLevel(1)
	if lvl <= 0 {
		return 1000
	}
	v := int64(1000 - lvl*10)
	if v < 900 {
		v = 900
	}
	return v
}
func afkOperatorResearchCostMultiplierMilli() int64 {
	lvl := afkOperatorLevel(2)
	if lvl <= 0 {
		return 1000
	}
	v := int64(1000 - lvl*10)
	if v < 900 {
		v = 900
	}
	return v
}
func afkOperatorStarbitRewardMultiplierMilli() int64 {
	lvl := afkOperatorLevel(3)
	if lvl <= 0 {
		return 1000
	}
	return 1000 + int64(lvl)*30
}
func afkOperatorDroneMultiplierMilli() int64 {
	lvl := afkOperatorLevel(4)
	if lvl <= 0 {
		return 1000
	}
	return 1000 + int64(lvl)*20
}

func afkOperatorBonusText(i int) string {
	lvl := afkOperatorLevel(i)
	if lvl <= 0 {
		return "NOT RECRUITED"
	}
	switch i {
	case 0:
		return fmt.Sprintf("ALL STARBITS +%d%%", lvl*2)
	case 1:
		return fmt.Sprintf("EXPEDITION TIME -%d%%", lvl)
	case 2:
		return fmt.Sprintf("TECH LAB COST -%d%%", lvl)
	case 3:
		return fmt.Sprintf("RECOVERY REWARDS +%d%%", lvl*3)
	case 4:
		return fmt.Sprintf("DRONE OUTPUT +%d%%", lvl*2)
	}
	return "ACTIVE"
}

func afkOperatorNextBonusText(i int) string {
	lvl := afkOperatorLevel(i)
	if lvl <= 0 || lvl >= afkOperatorMaxLevel {
		return "MAX BONUS"
	}
	next := lvl + 1
	switch i {
	case 0:
		return fmt.Sprintf("NEXT // ALL STARBITS +%d%%", next*2)
	case 1:
		return fmt.Sprintf("NEXT // EXPEDITION TIME -%d%%", next)
	case 2:
		return fmt.Sprintf("NEXT // TECH LAB COST -%d%%", next)
	case 3:
		return fmt.Sprintf("NEXT // RECOVERY REWARDS +%d%%", next*3)
	case 4:
		return fmt.Sprintf("NEXT // DRONE OUTPUT +%d%%", next*2)
	}
	return "NEXT BONUS"
}

func afkRefreshSection7Completion() {
	recruited := 0
	for i := range afkOperatorDefs {
		if afkOperatorRecruited(i) {
			recruited++
		}
	}
	if recruited >= 3 {
		for i := range afkOperatorDefs {
			if afkOperatorLevel(i) >= 2 {
				gameMeta.AFKSection7Complete = true
				return
			}
		}
	}
}

func afkSection7Objective() (string, string, string, int64, int64, bool) {
	if !afkOperatorsAvailable() {
		return "OPERATORS LOCKED", "REACH CURSOR CORE TIER 4", "AND COMPLETE TECH LAB DEVELOPMENT", int64(afkCoreTier()), 4, false
	}
	recruited := 0
	for i := range afkOperatorDefs {
		if afkOperatorRecruited(i) {
			recruited++
		}
	}
	if recruited < 1 {
		return "RECRUIT YOUR FIRST OPERATOR", "OPEN OPERATIONS / OPERATORS", "NOVA IS YOUR FIRST CORE SPECIALIST", int64(recruited), 1, false
	}
	if recruited < 3 {
		return "BUILD AN OPERATOR TEAM", "RECRUIT NOVA, VEGA AND BYTE", "EACH REQUIRES ITS OWN FACILITY", int64(recruited), 3, false
	}
	best := 0
	ready := false
	for i := range afkOperatorDefs {
		if lvl := afkOperatorLevel(i); lvl > best {
			best = lvl
		}
		if afkOperatorCanLevel(i) {
			ready = true
		}
	}
	if best < 2 {
		if ready {
			return "PROMOTE AN OPERATOR", "SERVICE XP IS READY", "CLICK THE READY OPERATOR TO LEVEL UP", int64(best), 2, false
		}
		return "ASSIGN OPERATOR WORK", "CLICK A READY OPERATOR TO START A PAID JOB", "JOB FINISHES -> COOLDOWN -> SERVICE XP // LEVEL UP MANUALLY", int64(best), 2, false
	}
	return "SECTION 7 COMPLETE", "OPERATOR TEAM ONLINE", "START WORK -> WAIT -> EARN XP -> CLICK READY OPERATOR -> LEVEL UP", 1, 1, true
}

func afkOperatorPanelRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }

func afkOperatorCardRect(w, hgt int32, i int) RECT {
	p := afkOperatorPanelRect(w, hgt)
	gap := sx(12, w)
	cw := (p.Right - p.Left - gap*4) / 5
	x := p.Left + int32(i)*(cw+gap)
	return RECT{x, p.Top + sy(90, hgt), x + cw, p.Bottom - sy(30, hgt)}
}

func drawAFKPixelOperator(hdc uintptr, r RECT, i int, recruited bool, w, hgt int32) {
	cx := (r.Left + r.Right) / 2
	top := r.Top + sy(66, hgt)
	scaleX, scaleY := sx(5, w), sy(5, hgt)
	if scaleX < 2 {
		scaleX = 2
	}
	if scaleY < 2 {
		scaleY = 2
	}
	body := rgb(39, 68, 105)
	skin := rgb(225, 195, 161)
	visor := afkOperatorDefs[i].Accent
	if !recruited {
		body, skin, visor = rgb(12, 22, 36), rgb(19, 29, 43), rgb(30, 42, 58)
	}
	// 8-bit helmet/head/body; all geometry is hard-coded rectangles.
	fillSolidRect(hdc, RECT{cx - 4*scaleX, top, cx + 4*scaleX, top + 2*scaleY}, body)
	fillSolidRect(hdc, RECT{cx - 5*scaleX, top + 2*scaleY, cx + 5*scaleX, top + 7*scaleY}, body)
	fillSolidRect(hdc, RECT{cx - 3*scaleX, top + 3*scaleY, cx + 3*scaleX, top + 6*scaleY}, skin)
	fillSolidRect(hdc, RECT{cx - 3*scaleX, top + 3*scaleY, cx + 3*scaleX, top + 4*scaleY}, visor)
	fillSolidRect(hdc, RECT{cx - 4*scaleX, top + 8*scaleY, cx + 4*scaleX, top + 15*scaleY}, body)
	fillSolidRect(hdc, RECT{cx - 7*scaleX, top + 9*scaleY, cx - 4*scaleX, top + 13*scaleY}, body)
	fillSolidRect(hdc, RECT{cx + 4*scaleX, top + 9*scaleY, cx + 7*scaleX, top + 13*scaleY}, body)
	fillSolidRect(hdc, RECT{cx - 4*scaleX, top + 15*scaleY, cx - 1*scaleX, top + 20*scaleY}, body)
	fillSolidRect(hdc, RECT{cx + 1*scaleX, top + 15*scaleY, cx + 4*scaleX, top + 20*scaleY}, body)
	if recruited && !gameMeta.ReducedMotion {
		pulse := int32((time.Now().UnixMilli()/280 + int64(i)) % 3)
		fillSolidRect(hdc, RECT{cx - 1*scaleX, top + 10*scaleY, cx + scaleX, top + (11+pulse)*scaleY}, visor)
	}
}

func afkOperatorAnimatedRect(r RECT, dx, dy int32) RECT {
	return RECT{r.Left + dx, r.Top + dy, r.Right + dx, r.Bottom + dy}
}

// v445: Operator ambient motion uses one monotonic UI clock shared by every
// portrait layer. The previous implementation sampled wall-clock time directly
// inside several helpers, which made the code harder to verify and left room for
// a frozen/zeroed path when UI state changed. This epoch is never recreated by
// card redraws, so offsets necessarily change while the Operators panel is open.
var afkOperatorMotionEpoch = time.Now()

func afkOperatorMotionSeconds() float64 {
	if afkOperatorMotionEpoch.IsZero() {
		afkOperatorMotionEpoch = time.Now()
	}
	return time.Since(afkOperatorMotionEpoch).Seconds()
}

func afkOperatorAmbientScale() float64 {
	// Reduced Motion still reduces the travel substantially, but it no longer
	// makes recruited companions appear broken/frozen. Locked Operators remain
	// completely static because drawAFKOperatorPortrait returns before this path.
	if gameMeta.ReducedMotion {
		return 0.28
	}
	return 1.0
}

func afkOperatorPetFloatY(seconds float64, operatorIndex, petSlot int, hgt int32) int32 {
	period := 2.15 + float64(operatorIndex)*0.23 + float64(petSlot)*0.31
	phase := float64(operatorIndex)*0.91 + float64(petSlot)*1.43
	baseAmp := 20.0 + float64((operatorIndex+petSlot)%3)*3.0
	amp := float64(sy(baseAmp, hgt)) * afkOperatorAmbientScale()
	return int32(math.Round(math.Sin(seconds*2*math.Pi/period+phase) * amp))
}

func afkOperatorByteFloatY(seconds float64, hgt int32) int32 {
	amp := float64(sy(16, hgt)) * afkOperatorAmbientScale()
	return int32(math.Round(math.Sin(seconds*2*math.Pi/2.65+0.45) * amp))
}

func afkOperatorFluxSpiderX(seconds float64, w int32) int32 {
	// Smooth horizontal patrol. Sine motion naturally eases at both turnarounds.
	amp := float64(sx(46, w)) * afkOperatorAmbientScale()
	return int32(math.Round(math.Sin(seconds*2*math.Pi/4.0+1.1) * amp))
}

func drawAFKOperatorPortrait(hdc uintptr, artBox RECT, i int, recruited bool, w, hgt int32) bool {
	if !recruited {
		// Locked Operators intentionally remain completely static: no companion
		// sprites and no chair movement.
		return drawAFKOperatorSprite(hdc, artBox, i, false)
	}
	seconds := afkOperatorMotionSeconds()
	bodyBox := artBox
	if i == 2 { // Byte's flying chair and Byte move as one body sprite.
		bodyBox = afkOperatorAnimatedRect(bodyBox, 0, afkOperatorByteFloatY(seconds, hgt))
	}

	// Draw the freshly re-applied body artwork first, then companions on top.
	// This guarantees the separated pet sprites are visible and makes their
	// independent motion obvious rather than letting the body cover the movement.
	drawn := drawAFKOperatorSprite(hdc, bodyBox, i, true)

	switch i {
	case 0: // Nova
		pet := RECT{artBox.Left + sx(4, w), artBox.Top + sy(6, hgt), artBox.Left + sx(76, w), artBox.Top + sy(96, hgt)}
		pet = afkOperatorAnimatedRect(pet, 0, afkOperatorPetFloatY(seconds, i, 0, hgt))
		drawAFKOperatorPetSprite(hdc, pet, afkOperatorPetNova)
	case 1: // Vega
		pet := RECT{artBox.Left + sx(2, w), artBox.Top + sy(6, hgt), artBox.Left + sx(92, w), artBox.Top + sy(88, hgt)}
		pet = afkOperatorAnimatedRect(pet, 0, afkOperatorPetFloatY(seconds, i, 0, hgt))
		drawAFKOperatorPetSprite(hdc, pet, afkOperatorPetVega)
	case 2: // Byte's companion moves independently from her flying chair.
		petW := sx(160, w)
		cx := (artBox.Left + artBox.Right) / 2
		pet := RECT{cx - petW/2, artBox.Top + sy(3, hgt), cx + petW/2, artBox.Top + sy(82, hgt)}
		pet = afkOperatorAnimatedRect(pet, 0, afkOperatorPetFloatY(seconds, i, 1, hgt))
		drawAFKOperatorPetSprite(hdc, pet, afkOperatorPetByte)
	case 3: // Astra
		pet := RECT{artBox.Right - sx(82, w), artBox.Top + sy(2, hgt), artBox.Right - sx(4, w), artBox.Top + sy(98, hgt)}
		pet = afkOperatorAnimatedRect(pet, 0, afkOperatorPetFloatY(seconds, i, 0, hgt))
		drawAFKOperatorPetSprite(hdc, pet, afkOperatorPetAstra)
	case 4: // Flux: hovering drone bobs, floor spider patrols horizontally.
		hover := RECT{artBox.Right - sx(105, w), artBox.Top + sy(4, hgt), artBox.Right - sx(5, w), artBox.Top + sy(63, hgt)}
		hover = afkOperatorAnimatedRect(hover, 0, afkOperatorPetFloatY(seconds, i, 0, hgt))
		drawAFKOperatorPetSprite(hdc, hover, afkOperatorPetFluxHover)
		spider := RECT{artBox.Left + sx(10, w), artBox.Bottom - sy(63, hgt), artBox.Left + sx(88, w), artBox.Bottom - sy(7, hgt)}
		spider = afkOperatorAnimatedRect(spider, afkOperatorFluxSpiderX(seconds, w), 0)
		drawAFKOperatorPetSprite(hdc, spider, afkOperatorPetFluxSpider)
	}
	return drawn
}

var afkOperatorWaitNoticeStarted time.Time

// afkOperatorWaitNoticeEvent identifies the newest Operator event that needs a
// physical player action. READY promotions notify immediately after work ends;
// otherwise an Operator notifies when their cooldown has finished. Freshly
// recruited idle Operators do not create a reminder.
func afkOperatorWaitNoticeEvent(now time.Time) int64 {
	afkEnsureSection7SaveShape()
	newest := int64(0)
	for i := range afkOperatorDefs {
		if !afkOperatorRecruited(i) || afkOperatorLevel(i) >= afkOperatorMaxLevel || afkOperatorAssignmentActive(i, now) {
			continue
		}
		cooldownEnd := gameMeta.AFKOperatorCooldownEndsUnix[i]
		if cooldownEnd <= 0 {
			continue
		}
		if afkOperatorCanLevel(i) || cooldownEnd <= now.Unix() {
			if cooldownEnd > newest {
				newest = cooldownEnd
			}
		}
	}
	return newest
}

func afkOperatorWaitNoticeVisible(now time.Time) bool {
	event := afkOperatorWaitNoticeEvent(now)
	if event <= 0 || event <= gameMeta.AFKOperatorWaitNoticeSeenUnix {
		afkOperatorWaitNoticeStarted = time.Time{}
		return false
	}
	if afkOperatorPanelOpen {
		gameMeta.AFKOperatorWaitNoticeSeenUnix = event
		afkOperatorWaitNoticeStarted = time.Time{}
		saveGameMeta()
		return false
	}
	return !afkAnyManagementPanelOpen()
}

func afkOperatorWaitNoticeRect(w, hgt int32, now time.Time) RECT {
	width := sx(400, w)
	height := sy(82, hgt)
	if width < 320 {
		width = 320
	}
	if height < 70 {
		height = 70
	}
	targetLeft := w - width - sx(18, w)
	top := sy(214, hgt)
	if afkExpeditionReturnNoticeVisible() {
		top += sy(104, hgt)
	}
	if afkOperatorWaitNoticeStarted.IsZero() {
		afkOperatorWaitNoticeStarted = now
	}
	progress := float64(now.Sub(afkOperatorWaitNoticeStarted)) / float64(520*time.Millisecond)
	progress = afkEase01(progress)
	startLeft := w + sx(12, w)
	left := startLeft + int32(float64(targetLeft-startLeft)*progress)
	return RECT{left, top, left + width, top + height}
}

func drawAFKOperatorWaitNotification(hdc uintptr, w, hgt int32) {
	now := time.Now()
	if !afkOperatorWaitNoticeVisible(now) {
		return
	}
	r := afkOperatorWaitNoticeRect(w, hgt, now)
	pulse := afkSoftPulse(now, 1.1, 0.35)
	border := afkRGBLerp(190, 104, 255, 237, 218, 255, pulse)
	drawBevelPanel(hdc, r, rgb(7, 15, 35), border, rgb(0, 5, 14), 2)
	fillSolidRect(hdc, RECT{r.Left, r.Top, r.Left + sx(5, w), r.Bottom}, rgb(190, 104, 255))
	icon := RECT{r.Left + sx(12, w), r.Top + sy(9, hgt), r.Left + sx(78, w), r.Bottom - sy(9, hgt)}
	drawAFKFacilitySprite(hdc, "operations", icon)
	textLeft := r.Left + sx(88, w)
	textRight := r.Right - sx(12, w)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(239, 248, 255))
		line := fitTextEllipsis(hdc, hudTinyFont, "YOUR OPERATORS ARE WAITING FOR YOU", textRight-textLeft)
		textOut(hdc, textLeft, r.Top+sy(18, hgt), line)
		setTextColor.Call(hdc, rgb(209, 154, 255))
		action := fitTextEllipsis(hdc, hudTinyFont, "OPEN OPERATIONS", textRight-textLeft)
		textOut(hdc, textLeft, r.Top+sy(46, hgt), action)
		selectObject.Call(hdc, old)
	}
}

func handleAFKOperatorWaitNotificationClick(h uintptr, p FPoint, w, hgt int32) bool {
	now := time.Now()
	if !afkOperatorWaitNoticeVisible(now) {
		return false
	}
	r := afkOperatorWaitNoticeRect(w, hgt, now)
	if !pointInRect(p, r) {
		return false
	}
	if event := afkOperatorWaitNoticeEvent(now); event > gameMeta.AFKOperatorWaitNoticeSeenUnix {
		gameMeta.AFKOperatorWaitNoticeSeenUnix = event
	}
	afkOperatorWaitNoticeStarted = time.Time{}
	afkCloseManagementPanels()
	afkOperatorPanelOpen = true
	playUIButtonClickSound()
	saveGameMeta()
	invalidateRect.Call(h, 0, 0)
	return true
}

func drawAFKOperators(hdc uintptr, w, hgt int32) {
	if !afkOperatorPanelOpen || !afkOperatorsAvailable() {
		return
	}
	afkEnsureSection7SaveShape()
	if afkRefreshOperatorAssignments(time.Now()) {
		saveGameMeta()
	}
	p := afkOperatorPanelRect(w, hgt)
	drawAFKManagementChrome(hdc, p, "OPERATORS", "START JOBS // EARN SERVICE XP // CLICK LEVEL UP", rgb(96, 78, 161), w, hgt)
	for i, dfn := range afkOperatorDefs {
		r := afkOperatorCardRect(w, hgt, i)
		recruited := afkOperatorRecruited(i)
		selected := i == afkOperatorSelected
		border := rgb(52, 84, 118)
		readyToLevel := recruited && afkOperatorCanLevel(i)
		if recruited {
			border = dfn.Accent
		}
		if readyToLevel {
			pulse := 1.0
			if !gameMeta.ReducedMotion {
				pulse = 0.45 + 0.55*afkSoftPulse(time.Now(), 1.15, float64(i)*0.17)
			}
			border = blendColor(dfn.Accent, rgb(255, 255, 255), pulse*0.58)
		}
		if selected {
			border = rgb(246, 235, 255)
		}
		face := rgb(5, 25, 49)
		if !recruited {
			face = rgb(3, 13, 26)
		}
		drawBevelPanel(hdc, r, face, border, rgb(0, 5, 14), 2)
		if readyToLevel {
			drawOutlineRect(hdc, RECT{r.Left + sx(3, w), r.Top + sy(3, hgt), r.Right - sx(3, w), r.Bottom - sy(3, hgt)}, dfn.Accent, 2)
		}
		// Keep the NPCs comfortably inside the upper half of each card. v440 moves
		// the portrait block further down so the characters sit comfortably inside
		// their cards instead of crowding the upper frame. Identity text follows the
		// artwork down while the Service XP bar remains anchored at the card bottom.
		artBox := RECT{r.Left + sx(14, w), r.Top + sy(48, hgt), r.Right - sx(14, w), r.Top + sy(338, hgt)}
		if recruited {
			drawAFKOperatorLevelFill(hdc, r, i, dfn.Accent, w, hgt)
		}
		if !drawAFKOperatorPortrait(hdc, artBox, i, recruited, w, hgt) {
			drawAFKPixelOperator(hdc, r, i, recruited, w, hgt)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, func() uintptr {
				if recruited {
					return rgb(245, 248, 252)
				}
				return rgb(85, 104, 124)
			}())
			centeredTextOut(hdc, r.Left+sx(4, w), r.Right-sx(4, w), r.Top+sy(348, hgt), dfn.Name)
			setTextColor.Call(hdc, dfn.Accent)
			centeredTextOut(hdc, r.Left+sx(4, w), r.Right-sx(4, w), r.Top+sy(370, hgt), dfn.Role)
			if recruited {
				lvl := afkOperatorLevel(i)
				setTextColor.Call(hdc, rgb(203, 218, 229))
				centeredTextOut(hdc, r.Left+sx(4, w), r.Right-sx(4, w), r.Top+sy(394, hgt), fmt.Sprintf("LV.%d / %d", lvl, afkOperatorMaxLevel))
				setTextColor.Call(hdc, rgb(117, 218, 246))
				centeredTextOut(hdc, r.Left+sx(4, w), r.Right-sx(4, w), r.Top+sy(418, hgt), afkOperatorBonusText(i))
				cur, target := afkOperatorXPToNext(i)
				if target > 0 && lvl < afkOperatorMaxLevel {
					bar := RECT{r.Left + sx(14, w), r.Bottom - sy(38, hgt), r.Right - sx(14, w), r.Bottom - sy(24, hgt)}
					fillSolidRect(hdc, bar, rgb(6, 13, 25))
					fillW := int32(0)
					if target > 0 {
						fillW = (bar.Right - bar.Left) * int32(cur) / int32(target)
					}
					fillSolidRect(hdc, RECT{bar.Left, bar.Top, bar.Left + fillW, bar.Bottom}, dfn.Accent)
					setTextColor.Call(hdc, rgb(203, 218, 229))
					centeredTextOut(hdc, r.Left+sx(4, w), r.Right-sx(4, w), r.Bottom-sy(59, hgt), fmt.Sprintf("SERVICE XP %s / %s", formatAFKNumber(cur), formatAFKNumber(target)))
					status := afkOperatorAssignmentStatus(i, time.Now())
					setTextColor.Call(hdc, dfn.Accent)
					status = fitTextEllipsis(hdc, hudTinyFont, status, r.Right-r.Left-sx(12, w))
					centeredTextOut(hdc, r.Left+sx(6, w), r.Right-sx(6, w), r.Bottom-sy(82, hgt), status)
				} else {
					setTextColor.Call(hdc, rgb(72, 224, 132))
					centeredTextOut(hdc, r.Left+sx(4, w), r.Right-sx(4, w), r.Bottom-sy(46, hgt), "MAX LEVEL")
				}
			} else {
				setTextColor.Call(hdc, rgb(84, 105, 126))
				centeredTextOut(hdc, r.Left+sx(4, w), r.Right-sx(4, w), r.Top+sy(396, hgt), "UNRECRUITED")
				setTextColor.Call(hdc, func() uintptr {
					if afkOperatorRecruitRequirementMet(i) {
						return dfn.Accent
					}
					return rgb(112, 92, 112)
				}())
				req := fitTextEllipsis(hdc, hudTinyFont, afkOperatorRecruitRequirementText(i), r.Right-r.Left-sx(12, w))
				centeredTextOut(hdc, r.Left+sx(6, w), r.Right-sx(6, w), r.Top+sy(420, hgt), req)
			}
			selectObject.Call(hdc, old)
		}
	}
}
