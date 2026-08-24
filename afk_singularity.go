//go:build windows

package main

import (
	"fmt"
	"math"
	"time"
	"unsafe"
)

// Sections 1-5 of the Cursor Control AFK system. Everything on this screen is
// drawn directly with GDI primitives so the feature has no runtime sprite/UI
// dependency. Later AFK sections build on this state instead of replacing it.
const (
	afkSection1StarbitTarget    int64 = 100
	afkManualClickValue         int64 = 5
	afkCursorCoreBuildCost      int64 = 100
	afkAutoCursorMax                  = 10
	afkProcessingMaxLevel             = 10
	afkCoreMaxTier                    = 5
	afkOfflineMinimumSeconds    int64 = 60
	afkBaseCapacitySeconds      int64 = 2 * 60 * 60
	afkOverdriveDurationSeconds int64 = 15 * 60
	afkOverdriveCycleSeconds    int64 = 60 * 60
	afkActiveCacheFirstSeconds  int64 = 30 * 60
	afkActiveCacheCycleSeconds  int64 = 45 * 60
)

// Section 2 progression is intentionally table-driven rather than formula-
// generated. Every cost and output breakpoint is explicit so pacing can be
// tuned without hidden curves. Index N is the cost to deploy cursor N+1.
var afkAutoCursorCosts = [...]int64{
	0, 50, 100, 200, 400, 800, 1_600, 3_200, 6_400, 12_800,
}

// Cost to advance from the current Cursor Processing level to the next one.
// Level 1 is the baseline unlocked with the Cursor Core, so index 1 is 1->2.
var afkProcessingUpgradeCosts = [...]int64{
	0, 100, 250, 600, 1_400, 3_200, 7_000, 15_000, 32_000, 68_000,
}

// Milli-Starbits produced by each deployed Auto Cursor every second. Integer
// milli-units preserve fractional rates without floating-point save drift.
var afkProcessingRateMilli = [...]int64{
	0, 1_000, 1_250, 1_600, 2_100, 2_800, 3_800, 5_200, 7_000, 9_500, 13_000,
}

// Section 3: every Core tier requirement and production multiplier is explicit.
// Index N describes the upgrade from tier N to N+1. Tier 5 is the initial
// AFK release cap; later systems plug into these tier milestones.
var afkCoreTierUpgradeCosts = [...]int64{
	0, 100_000, 5_000_000, 75_000_000, 1_500_000_000, 0,
}

// Milli-multipliers applied to the complete Auto Cursor output. These create
// meaningful power jumps without changing the saved meaning of Section 2
// Cursor Processing levels.
var afkCoreTierProductionMultiplierMilli = [...]int64{
	0, 1_000, 4_000, 16_000, 70_000, 300_000,
}

// Section 4 keeps the AFK economy completely isolated from the main game.
// Starbits are the only AFK currency; main-game SpaceCoins are never generated,
// spent, converted, or modified by Singularity progression.

var afkCoreTierSystemNames = [...]string{
	"",
	"AUTO CURSOR NETWORK",
	"SPACE EXPEDITIONS",
	"TECH LAB",
	"OPERATORS + MODULES",
	"DRONE BAY",
}

type afkClickBurst struct {
	Started time.Time
	X       float64
	Y       float64
	Amount  int64
}

type afkStarbitFlowParticle struct {
	Started time.Time
	X       float64
	Y       float64
	Amount  int64
	Index   int64
}

// Cosmetic Starbits living inside the Singularity use normalized coordinates
// so they survive window resizing. They have independent velocities, wander
// slightly, and reflect off the circular event-horizon wall. They never touch
// economy state.
type afkSingularityFreeParticle struct {
	X, Y   float64
	VX, VY float64
	Seed   float64
	Size   int32
}

var (
	afkClickBursts                 []afkClickBurst
	afkStarbitFlowParticles        []afkStarbitFlowParticle
	afkSingularityFreeParticles    []afkSingularityFreeParticle
	afkSingularityParticleLastTick time.Time
	afkSingularityParticleSerial   int64
	afkSingularityRotatedBGRA      []byte
	afkSingularityRotationDegree   int = -1
	afkSingularityRotationDC       uintptr
	afkSingularityRotationBitmap   uintptr
	afkSingularityRotationOld      uintptr
	afkSingularityRotationBits     uintptr
	afkSingularitySurfaceDegree    int = -1
	afkStarbaseAnimationEpoch      time.Time
	afkNextStarbitFlow             time.Time
	afkStarbitFlowSerial           int64
	afkLiveLastTick                time.Time
	afkLastProductionSave          time.Time
	afkWelcomeJustClaimed          time.Time
	afkCoreBuildBurstStarted       time.Time
)

func afkOverdriveActive(now time.Time) bool {
	return gameMeta.AFKOverdriveUntilUnix > now.Unix()
}

func afkOverdriveReady(now time.Time) bool {
	return !afkOverdriveActive(now) && now.Unix() >= gameMeta.AFKOverdriveReadyUnix
}

func afkStartOverdrive(now time.Time) bool {
	if !afkCursorCoreBuilt() || !afkOverdriveReady(now) {
		return false
	}
	duration := afkOverdriveDurationSeconds * afkTalentOverdriveDurationMultiplierMilli() / 1000
	gameMeta.AFKOverdriveUntilUnix = now.Unix() + duration
	gameMeta.AFKOverdriveReadyUnix = now.Unix() + afkOverdriveCycleSeconds
	playAFKOverdriveSound()
	return true
}

func afkOverdriveStatus(now time.Time) string {
	if afkOverdriveActive(now) {
		return "x2 " + afkFormatDuration(gameMeta.AFKOverdriveUntilUnix-now.Unix())
	}
	if now.Unix() < gameMeta.AFKOverdriveReadyUnix {
		return "READY " + afkFormatDuration(gameMeta.AFKOverdriveReadyUnix-now.Unix())
	}
	return "x2 READY"
}

func afkRefreshActiveCache(now time.Time) {
	if afkCoreTier() < 2 || gameMeta.AFKActiveCacheReward > 0 {
		return
	}
	if gameMeta.AFKNextActiveCacheUnix <= 0 {
		gameMeta.AFKNextActiveCacheUnix = now.Unix() + afkActiveCacheFirstSeconds
		return
	}
	if now.Unix() < gameMeta.AFKNextActiveCacheUnix {
		return
	}
	// Reference Lucky Finds scale with the current economy. Award 15 minutes of
	// current continuous production, with a useful minimum for early Tier 2.
	reward := afkContinuousStarbitsPerSecondMilli() * 900 / 1000
	reward = reward * afkOperatorStarbitRewardMultiplierMilli() / 1000
	reward = reward * afkModuleCacheRewardMultiplierMilli() / 1000
	reward = reward * afkPrestigeCacheMultiplierMilli() / 1000
	if reward < 1_000 {
		reward = 1_000
	}
	gameMeta.AFKActiveCacheReward = reward
}

func afkClaimActiveCache(now time.Time) bool {
	if gameMeta.AFKActiveCacheReward <= 0 || !afkActionAllowed("claim_active_cache", now, afkClaimActionDebounce) {
		return false
	}
	// Consume the persisted entitlement before crediting it. A duplicate click,
	// re-entrant message, or crash/restart cannot replay the same reward.
	r := gameMeta.AFKActiveCacheReward
	gameMeta.AFKActiveCacheReward = 0
	gameMeta.AFKNextActiveCacheUnix = now.Unix() + afkActiveCacheCycleSeconds
	gameMeta.AFKActiveCachesClaimed++
	afkGrantStarbits(r)
	playAFKCollectItemSound()
	return true
}

func afkCapacitySeconds() int64 {
	if afkScoutUnlocked() {
		return afkSection5CapacitySeconds()
	}
	return afkBaseCapacitySeconds
}

func afkFormatDuration(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func afkStampSaveTime(now time.Time) {
	unix := now.Unix()
	if unix <= 0 {
		return
	}
	// Never move the trusted local watermark backwards. A clock rollback freezes
	// AFK time until the machine clock catches up instead of creating free loops.
	if gameMeta.AFKMaxObservedUnix > unix {
		gameMeta.AFKLastSavedUnix = gameMeta.AFKMaxObservedUnix
		return
	}
	gameMeta.AFKMaxObservedUnix = unix
	gameMeta.AFKLastSavedUnix = unix
}

func afkAccumulateOfflineWindow(awaySeconds int64) {
	// v423 Starbit Bank rewrite: Starbase production is active-session only.
	// Closing Cursor Control never creates new Starbits. Existing pending rewards
	// from pre-v423 saves may still be claimed once for migration compatibility.
	_ = awaySeconds
}

func afkPrepareOfflineCatchup(now time.Time) {
	unix := now.Unix()
	if unix <= 0 {
		return
	}
	last := gameMeta.AFKLastSavedUnix
	maxSeen := gameMeta.AFKMaxObservedUnix
	if last <= 0 {
		// Migration from Sections 1-3: establish the watermark with no retroactive
		// reward because there is no trustworthy departure timestamp yet.
		gameMeta.AFKLastSavedUnix = unix
		if maxSeen < unix {
			gameMeta.AFKMaxObservedUnix = unix
		}
		return
	}
	if maxSeen > 0 && unix < maxSeen {
		gameMeta.AFKClockRollbackCount++
		gameMeta.AFKLastSavedUnix = maxSeen
		return
	}
	away := unix - last
	if away > 0 {
		afkAccumulateOfflineWindow(away)
	}
	if unix > gameMeta.AFKMaxObservedUnix {
		gameMeta.AFKMaxObservedUnix = unix
	}
	gameMeta.AFKLastSavedUnix = unix
	afkRefreshExpeditionCompletion(now)
	afkRefreshActiveCache(now)
}

func afkHasOfflineReward() bool {
	return gameMeta.AFKOfflinePendingStarbits > 0 || gameMeta.AFKOfflinePendingPaidSeconds > 0
}

func afkClaimOfflineReward() {
	now := time.Now()
	if !afkHasOfflineReward() || !afkActionAllowed("claim_offline", now, afkClaimActionDebounce) {
		return
	}
	// Consume pending state first, then credit the captured amount exactly once.
	reward := gameMeta.AFKOfflinePendingStarbits
	gameMeta.AFKOfflinePendingAwaySeconds = 0
	gameMeta.AFKOfflinePendingPaidSeconds = 0
	gameMeta.AFKOfflinePendingStarbits = 0
	gameMeta.AFKOfflineClaims++
	gameMeta.AFKSection4Complete = true
	if reward > 0 {
		afkGrantStarbits(reward)
	}
	afkWelcomeJustClaimed = now
}

func afkPanelRect(w, hgt int32) RECT {
	return RECT{0, 0, w, hgt}
}

func afkBackRect(w, hgt int32) RECT {
	// Keep Back completely outside the Starbase telemetry/navigation deck.
	return RECT{sx(8, w), sy(8, hgt), sx(118, w), sy(40, hgt)}
}

func afkMainFieldRect(w, hgt int32) RECT {
	// Starbase owns its own presentation geometry: the arena extends much farther
	// down than the normal modes because permanent navigation has moved to the
	// top command deck and the bottom is reserved for one contextual action row.
	return RECT{sx(14, w), sy(198, hgt), w - sx(14, w), hgt - sy(112, hgt)}
}

func afkSidePanelRect(w, hgt int32) RECT {
	return afkBottomDockRect(w, hgt)
}

var afkStarbaseSnareBeatsMS = [...]int64{
	708, 1904, 3111, 4319, 5515, 6711, 7918, 9125, 10298, 11494, 12701, 13897, 15093, 16289, 17496, 18704, 19900, 21095, 22291, 23499, 24694, 25890, 27098, 28305, 29501, 30697, 31916, 33112, 34296, 35492, 36699, 37907, 39102, 40298, 41517, 42713, 43897, 45093, 46289, 47496, 48704, 49923, 51130, 52326, 53534, 54741, 55937, 57133, 58305, 59513, 60720, 61939, 63112, 64319, 65550, 66723, 67930, 69137, 70333, 71541, 72737, 73932, 75140, 76336, 77531, 78739, 79935, 81131, 82338, 83534, 84741, 85937, 87110, 88305, 89513, 90709, 91916, 93100, 94308, 95504, 96711, 97895, 99103, 100310, 101518, 102702, 103921, 105105, 106313, 107508, 108704, 109912, 111107, 112303, 113511, 114707, 115914, 117098, 118306, 119513, 120721, 121893, 123112, 124471,
}

var (
	afkStarbaseMusicPosCached int64
	afkStarbaseMusicPosAt     time.Time
)

func afkStarbaseMusicPositionMS() int64 {
	now := time.Now()
	if !afkStarbaseMusicPosAt.IsZero() && now.Sub(afkStarbaseMusicPosAt) < 28*time.Millisecond {
		return afkStarbaseMusicPosCached
	}
	var pos int64
	if _, err := fmt.Sscan(mciQuery("status starbase_music position"), &pos); err == nil && pos >= 0 {
		afkStarbaseMusicPosCached = pos
		afkStarbaseMusicPosAt = now
	}
	return afkStarbaseMusicPosCached
}

func afkSingularitySnarePulse() float64 {
	if overlayMode != OverlayAFKSingularity || !starbaseMusicReady {
		return 0
	}
	pos := afkStarbaseMusicPositionMS()
	if pos < 0 {
		return 0
	}
	last := int64(-1)
	for _, beat := range afkStarbaseSnareBeatsMS {
		if beat > pos {
			break
		}
		last = beat
	}
	if last < 0 {
		return 0
	}
	age := float64(pos-last) / 1000.0
	if age < 0 || age > 0.65 {
		return 0
	}
	// The snare, not the kick, is the impact cue. A short exponential release
	// makes the black event-horizon core hit its former full size on the snare and then
	// smoothly breathe back down before the next backbeat.
	return math.Exp(-age / 0.18)
}

func afkSingularityBaseGeometry(w, hgt int32) (float64, float64, float64) {
	r := afkMainFieldRect(w, hgt)
	cx := float64(r.Left+r.Right) * 0.5
	cy := float64(r.Top+r.Bottom) * 0.5
	fieldW := float64(r.Right - r.Left)
	fieldH := float64(r.Bottom - r.Top)
	radius := fieldH * 0.455
	if maxFromWidth := fieldW * 0.255; radius > maxFromWidth {
		radius = maxFromWidth
	}
	if radius < 92 {
		radius = 92
	}
	return cx, cy, radius
}

func afkSingularityGeometry(w, hgt int32) (float64, float64, float64) {
	// World geometry remains stable. The authored spiral/backdrop, facilities,
	// Auto Cursors, drones and orbital systems must not breathe with the music.
	return afkSingularityBaseGeometry(w, hgt)
}

func afkSingularityCoreGeometry(w, hgt int32) (float64, float64, float64) {
	cx, cy, radius := afkSingularityBaseGeometry(w, hgt)
	// Only the black event-horizon core and its local particles react to the
	// selected snare backbeats. Rest smaller, then hit the old full size.
	scale := 0.90 + 0.10*afkSingularitySnarePulse()
	return cx, cy, radius * scale
}

func afkSingularityHitRect(w, hgt int32) RECT {
	// Keep interaction stable even while the artwork pulses. The hit target uses
	// the maximum/original Singularity size rather than shrinking between snares.
	cx, cy, radius := afkSingularityBaseGeometry(w, hgt)
	hit := int32(radius * 1.28)
	return RECT{int32(cx) - hit, int32(cy) - hit, int32(cx) + hit, int32(cy) + hit}
}

func afkSection1Progress() int64 {
	v := gameMeta.AFKStarbits
	if v < 0 {
		return 0
	}
	if v > afkSection1StarbitTarget {
		return afkSection1StarbitTarget
	}
	return v
}

func afkSection1IsComplete() bool {
	return gameMeta.AFKSection1Complete || gameMeta.AFKStarbits >= afkSection1StarbitTarget
}

func afkCursorCoreBuilt() bool {
	return gameMeta.AFKCursorCoreBuilt
}

func afkManualExtractionAvailable() bool {
	return !afkSection1IsComplete() || afkCursorCoreBuilt()
}

func afkProcessingLevel() int {
	lvl := gameMeta.AFKCursorProcessingLevel
	if !afkCursorCoreBuilt() {
		return 0
	}
	if lvl < 1 {
		return 1
	}
	if lvl > afkProcessingMaxLevel {
		return afkProcessingMaxLevel
	}
	return lvl
}

func afkCoreTier() int {
	if !afkCursorCoreBuilt() {
		return 0
	}
	tier := gameMeta.AFKCursorCoreTier
	if tier < 1 {
		return 1
	}
	if tier > afkCoreMaxTier {
		return afkCoreMaxTier
	}
	return tier
}

func afkCoreTierMultiplierMilli() int64 {
	tier := afkCoreTier()
	if tier <= 0 {
		return 0
	}
	return afkCoreTierProductionMultiplierMilli[tier]
}

func afkCoreTierMultiplierText() string {
	m := afkCoreTierMultiplierMilli()
	if m <= 0 {
		return "x0"
	}
	if m%1000 == 0 {
		return fmt.Sprintf("x%d", m/1000)
	}
	return fmt.Sprintf("x%.2f", float64(m)/1000.0)
}

func afkNextCoreTierCost() int64 {
	tier := afkCoreTier()
	if tier < 1 || tier >= afkCoreMaxTier {
		return 0
	}
	return afkCoreTierUpgradeCosts[tier]
}

func afkCoreTierSystemName(tier int) string {
	if tier < 1 || tier >= len(afkCoreTierSystemNames) {
		return "UNKNOWN SYSTEM"
	}
	return afkCoreTierSystemNames[tier]
}

func afkSection3IsComplete() bool {
	return gameMeta.AFKSection3Complete || afkCoreTier() >= afkCoreMaxTier
}

func afkRefreshSection3Completion() {
	if afkCoreTier() >= afkCoreMaxTier {
		gameMeta.AFKSection3Complete = true
	}
}

func afkAutoCursorStarbitsPerSecondMilli() int64 {
	if !afkCursorCoreBuilt() || gameMeta.AFKAutoCursors <= 0 {
		return 0
	}
	lvl := afkProcessingLevel()
	base := int64(gameMeta.AFKAutoCursors) * afkProcessingRateMilli[lvl]
	base = base * afkCoreTierMultiplierMilli() / 1000
	// The Extraction Depot is Auto-Cursor-specific. Broad permanent bonuses are
	// combined additively later so they cannot compound into runaway output.
	base = base * afkModuleAutoCursorMultiplierMilli() / 1000
	return base
}

func afkPermanentStarbitSupportMultiplierMilli() int64 {
	// v434: broad permanent bonuses add together before being applied once.
	// This preserves the value of every system without Research x Talents x
	// Operators x Modules x Prestige producing exponential late-game growth.
	m := int64(1000)
	m += afkResearchStarbitMultiplierMilli() - 1000
	m += afkTalentStarbitMultiplierMilli() - 1000
	m += afkOperatorStarbitProductionMultiplierMilli() - 1000
	m += afkModuleProductionMultiplierMilli() - 1000
	m += afkPrestigeMultiplierMilli() - 1000
	if m < 1000 {
		m = 1000
	}
	return m
}

func afkContinuousStarbitsPerSecondBaseMilli() int64 {
	// Base production before the authenticated Global-rank bonus. Keeping this
	// separate lets the cloud authority apply the same rank multiplier from the
	// server-side Global EXP record instead of trusting a client-supplied rank.
	v := saturatingAdd(afkAutoCursorStarbitsPerSecondMilli(), afkDroneStarbitsPerSecondMilli())
	return saturatingMulDiv(v, afkPermanentStarbitSupportMultiplierMilli(), 1000)
}

func afkContinuousStarbitsPerSecondMilli() int64 {
	return saturatingMulDiv(afkContinuousStarbitsPerSecondBaseMilli(), afkGlobalRankMultiplierMilli(), 1000)
}

func afkStarbitsPerSecondMilli() int64 {
	// HUD-facing effective rate includes the average value of the Orbital burst
	// cycle while live production still pays that layer discretely. Overdrive is
	// deliberately a live 15-minute x2 state, mirroring the reference boost.
	v := saturatingAdd(afkContinuousStarbitsPerSecondMilli(), afkOrbitalEffectivePerSecondMilli())
	if afkOverdriveActive(time.Now()) {
		v = saturatingMul(v, 2)
	}
	return v
}

func afkStarbitsPerSecondText() string {
	rate := afkStarbitsPerSecondMilli()
	if rate <= 0 {
		return "0"
	}
	if rate%1000 == 0 {
		return formatAFKNumber(rate / 1000)
	}
	return fmt.Sprintf("%.2f", float64(rate)/1000.0)
}

func afkManualExtractionBaseValue() int64 {
	amount := afkManualClickValue
	amount = amount * afkResearchManualMultiplierMilli() / 1000
	amount = amount * afkTalentManualMultiplierMilli() / 1000
	return amount
}

func afkManualExtractionValue() int64 {
	amount := afkManualExtractionBaseValue()
	amount = amount * afkGlobalRankMultiplierMilli() / 1000
	amount = amount * afkModuleManualMultiplierMilli() / 1000
	if afkOverdriveActive(time.Now()) {
		amount *= 2
	}
	if amount < 1 {
		amount = 1
	}
	return amount
}

func afkNextAutoCursorCost() int64 {
	count := gameMeta.AFKAutoCursors
	if count < 0 || count >= afkAutoCursorMax {
		return 0
	}
	return afkAutoCursorCosts[count]
}

func afkNextProcessingCost() int64 {
	lvl := afkProcessingLevel()
	if lvl < 1 || lvl >= afkProcessingMaxLevel {
		return 0
	}
	return afkProcessingUpgradeCosts[lvl]
}

func afkSection2IsComplete() bool {
	return gameMeta.AFKSection2Complete || (afkCursorCoreBuilt() && gameMeta.AFKAutoCursors >= afkAutoCursorMax && afkProcessingLevel() >= afkProcessingMaxLevel)
}

func afkRefreshSection2Completion() {
	if afkCursorCoreBuilt() && gameMeta.AFKAutoCursors >= afkAutoCursorMax && afkProcessingLevel() >= afkProcessingMaxLevel {
		gameMeta.AFKSection2Complete = true
	}
}

func afkObjective() (title, detail1, detail2 string, current, target int64, complete bool) {
	if !afkSection1IsComplete() {
		return "COLLECT 100 STARBITS", "MANUALLY STABILISE THE FIRST", "STARBIT LINK TO THE SINGULARITY.", afkSection1Progress(), afkSection1StarbitTarget, false
	}
	if !afkCursorCoreBuilt() {
		return "BUILD THE CURSOR CORE", "SPEND THE CALIBRATION STARBITS", "TO BRING THE CORE ONLINE.", gameMeta.AFKStarbits, afkCursorCoreBuildCost, false
	}
	if gameMeta.AFKAutoCursors < 1 {
		return "DEPLOY 1 AUTO CURSOR", "START AUTOMATIC STARBIT", "EXTRACTION AROUND THE SINGULARITY.", int64(gameMeta.AFKAutoCursors), 1, false
	}
	if gameMeta.AFKAutoCursors < 5 {
		return "DEPLOY 5 AUTO CURSORS", "EXPAND THE FIRST ORBITAL", "EXTRACTION FORMATION.", int64(gameMeta.AFKAutoCursors), 5, false
	}
	if gameMeta.AFKAutoCursors < afkAutoCursorMax {
		return "DEPLOY 10 AUTO CURSORS", "COMPLETE THE TIER 1 AUTO", "CURSOR EXTRACTION RING.", int64(gameMeta.AFKAutoCursors), afkAutoCursorMax, false
	}
	if afkProcessingLevel() < 5 {
		return "CURSOR PROCESSING LV. 5", "INCREASE EACH AUTO CURSOR'S", "STARBIT PROCESSING RATE.", int64(afkProcessingLevel()), 5, false
	}
	if afkProcessingLevel() < afkProcessingMaxLevel {
		return "CURSOR PROCESSING LV. 10", "MAXIMISE TIER 1 PROCESSING", "BEFORE THE NEXT CORE PHASE.", int64(afkProcessingLevel()), afkProcessingMaxLevel, false
	}
	if afkCoreTier() < 2 {
		cost := afkNextCoreTierCost()
		return "UPGRADE CURSOR CORE // TIER 2", fmt.Sprintf("STORE %s STARBITS", formatAFKNumber(cost)), "UNLOCK LINK: SPACE EXPEDITIONS", gameMeta.AFKStarbits, cost, false
	}
	if !afkSection5IsComplete() {
		if !afkScoutUnlocked() {
			return "BUILD THE SCOUT SHIP", "OPEN EXPEDITIONS AND ASSEMBLE", "THE FIRST SPACE SCOUT.", gameMeta.AFKStarbits, afkScoutShipUnlockCost, false
		}
		if afkExpeditionRewardPending() {
			return "CLAIM EXPEDITION", "THE SCOUT HAS RETURNED.", "BANK STARBITS + NAVIGATION DATA.", 0, 1, false
		}
		if afkSpaceCachePending() {
			return "OPEN STAR CACHE", "A CACHE WAS FOUND IN DEEP SPACE.", "OPEN IT BEFORE THE NEXT FLIGHT.", 0, 1, false
		}
		if gameMeta.AFKExpeditionsCompleted < 1 {
			return "COMPLETE 1 EXPEDITION", "LAUNCH THE SCOUT SHIP", "FROM THE EXPEDITIONS PANEL.", int64(gameMeta.AFKExpeditionsCompleted), 1, false
		}
		if afkScoutSpeedLevel() < 3 {
			return "EXPEDITION SPEED LV. 3", "SPEND NAVIGATION DATA TO", "SHORTEN ALL FUTURE FLIGHTS.", int64(afkScoutSpeedLevel()), 3, false
		}
		if afkScoutCapacityLevel() < 3 {
			return "SCOUT SYSTEMS LV. 3", "RAISE THE SCOUT CAPACITY SYSTEM", "TO LEVEL 3 FOR NETWORK PROGRESSION.", int64(afkScoutCapacityLevel()), 3, false
		}
		if afkScoutCacheLevel() < 3 {
			return "CACHE CHANCE LV. 3", "IMPROVE THE SCOUT'S", "STAR CACHE DETECTION ARRAY.", int64(afkScoutCacheLevel()), 3, false
		}
		if afkScoutTier() < 2 {
			cost, req := afkNextScoutTierCost()
			return "SCOUT SHIP // TIER 2", fmt.Sprintf("REQUIRES %d NAV DATA", cost), fmt.Sprintf("AND %d COMPLETED EXPEDITIONS", req), gameMeta.AFKNavigationData, cost, false
		}
		if gameMeta.AFKExpeditionsCompleted < 6 {
			return "COMPLETE 6 EXPEDITIONS", "PROVE THE TIER 2 SCOUT", "NETWORK IS FULLY OPERATIONAL.", int64(gameMeta.AFKExpeditionsCompleted), 6, false
		}
		afkRefreshSection5Completion()
	}
	if !gameMeta.AFKSection4Complete {
		available := int64(0)
		if gameMeta.AFKOfflinePendingStarbits > 0 {
			available = 1
		}
		return "CLAIM AFK STARBITS", "AFK STARBIT STORAGE IS ONLINE.", "RETURN AFTER 1+ MINUTE AND CLAIM EARNINGS.", available, 1, false
	}
	if afkCoreTier() >= 3 && !gameMeta.AFKSection6Complete {
		return afkSection6Objective()
	}
	if afkCoreTier() >= 4 && !gameMeta.AFKSection7Complete {
		return afkSection7Objective()
	}
	if afkCoreTier() >= 4 && gameMeta.AFKSection7Complete && !gameMeta.AFKSection8Complete {
		return afkSection8Objective()
	}
	if afkCoreTier() >= 5 && gameMeta.AFKSection8Complete && !gameMeta.AFKSection9Complete {
		return afkSection9Objective()
	}
	if afkCoreTier() < afkCoreMaxTier {
		nextTier := afkCoreTier() + 1
		cost := afkNextCoreTierCost()
		return fmt.Sprintf("UPGRADE CURSOR CORE // TIER %d", nextTier), fmt.Sprintf("STORE %s STARBITS", formatAFKNumber(cost)), fmt.Sprintf("NEXT LINK: %s", afkCoreTierSystemName(nextTier)), gameMeta.AFKStarbits, cost, false
	}
	if gameMeta.AFKSection9Complete && !gameMeta.AFKSection10Complete {
		return afkSection10Objective()
	}
	if gameMeta.AFKSection10Complete && !gameMeta.AFKSection11Complete {
		return afkSection11Objective()
	}
	afkEnsureSection12State()
	return "AFK NETWORK COMPLETE", fmt.Sprintf("STELLAR PRESTIGE RANK %d", afkPrestigeRank()), "ENDGAME STARBIT NETWORK ACTIVE.", 1, 1, true
}

func afkBuildCoreRect(w, hgt int32) RECT {
	side := afkSidePanelRect(w, hgt)
	return RECT{side.Left + sx(34, w), side.Bottom - sy(116, hgt), side.Right - sx(34, w), side.Bottom - sy(66, hgt)}
}

func afkDeployCursorRect(w, hgt int32) RECT {
	side := afkSidePanelRect(w, hgt)
	return RECT{side.Left + sx(34, w), side.Bottom - sy(166, hgt), side.Right - sx(34, w), side.Bottom - sy(122, hgt)}
}

func afkProcessingUpgradeRect(w, hgt int32) RECT {
	side := afkSidePanelRect(w, hgt)
	return RECT{side.Left + sx(34, w), side.Bottom - sy(82, hgt), side.Right - sx(34, w), side.Bottom - sy(38, hgt)}
}

func afkCoreTierUpgradeRect(w, hgt int32) RECT {
	side := afkSidePanelRect(w, hgt)
	return RECT{side.Left + sx(34, w), side.Bottom - sy(108, hgt), side.Right - sx(34, w), side.Bottom - sy(50, hgt)}
}

func afkClaimCoinsRect(w, hgt int32) RECT {
	field := afkMainFieldRect(w, hgt)
	return RECT{field.Right - sx(250, w), field.Top + sy(100, hgt), field.Right - sx(18, w), field.Top + sy(142, hgt)}
}

func afkWelcomeRect(w, hgt int32) RECT {
	p := afkPanelRect(w, hgt)
	ww := sx(560, w)
	hh := sy(390, hgt)
	cx := (p.Left + p.Right) / 2
	cy := (p.Top + p.Bottom) / 2
	return RECT{cx - ww/2, cy - hh/2, cx + ww/2, cy + hh/2}
}

func afkWelcomeClaimRect(w, hgt int32) RECT {
	r := afkWelcomeRect(w, hgt)
	return RECT{r.Left + sx(82, w), r.Bottom - sy(76, hgt), r.Right - sx(82, w), r.Bottom - sy(28, hgt)}
}

func updateAFKLiveProduction(h uintptr, now time.Time) {
	if afkLiveLastTick.IsZero() {
		afkLiveLastTick = now
		return
	}
	delta := now.Sub(afkLiveLastTick)
	afkLiveLastTick = now
	if delta <= 0 {
		return
	}
	// When Discord cloud sync is active, a fresh server lease is required for
	// economy progression. This prevents a disconnected old device from farming
	// in parallel after the same account signs in elsewhere.
	if !afkCloudEconomyAllowed(now) {
		return
	}
	afkUpdateExpeditionPresentation(now)
	afkRefreshExpeditionCompletion(now)
	// Operator progression is assignment-driven in v443. A completed manual job
	// awards Service XP exactly once; nothing auto-starts or passively farms XP.
	if afkRefreshOperatorAssignments(now) {
		saveGameMeta()
	}
	// A long heartbeat gap means Windows suspended/slept the process. Treat it as
	// the same capped AFK window as a full game exit, so suspend cannot bypass
	// AFK Capacity and normal frame stalls do not lose production.
	if delta >= time.Duration(afkOfflineMinimumSeconds)*time.Second {
		afkAccumulateOfflineWindow(int64(delta / time.Second))
		saveGameMeta()
		afkLastProductionSave = now
		if h != 0 && overlayMode == OverlayAFKSingularity {
			invalidateRect.Call(h, 0, 0)
		}
		return
	}
	afkMaybePlayDroneMiningSound(now)
	// The Orbital Extractor pays discrete bursts while Cursors/Drones remain
	// continuous. This keeps the visible charge/fire cycle economically real.
	afkorbital := afkProcessOrbitalLive(now)
	rate := afkContinuousStarbitsPerSecondMilli()
	if afkOverdriveActive(now) {
		rate = saturatingMul(rate, 2)
	}
	if rate <= 0 && afkorbital <= 0 {
		return
	}
	producedMilli := rate * delta.Milliseconds() / 1000
	if producedMilli > 0 {
		gameMeta.AFKStarbitRemainderMilli += producedMilli
	}
	gained := gameMeta.AFKStarbitRemainderMilli / 1000
	gameMeta.AFKStarbitRemainderMilli %= 1000
	if gained > 0 {
		afkGrantStarbits(gained)
	}
	if gained <= 0 && afkorbital <= 0 {
		return
	}
	if afkLastProductionSave.IsZero() || now.Sub(afkLastProductionSave) >= 5*time.Second {
		saveGameMeta()
		afkLastProductionSave = now
	}
	if h != 0 && overlayMode == OverlayAFKSingularity {
		invalidateRect.Call(h, 0, 0)
	}
}

func formatAFKNumber(v int64) string {
	if v < 1000 {
		return fmt.Sprintf("%d", v)
	}
	if v < 1_000_000 {
		return fmt.Sprintf("%.2fK", float64(v)/1_000)
	}
	if v < 1_000_000_000 {
		return fmt.Sprintf("%.2fM", float64(v)/1_000_000)
	}
	if v < 1_000_000_000_000 {
		return fmt.Sprintf("%.2fB", float64(v)/1_000_000_000)
	}
	return fmt.Sprintf("%.2fT", float64(v)/1_000_000_000_000)
}

func releaseAFKSingularityRotationSurface() {
	if afkSingularityRotationDC != 0 {
		if afkSingularityRotationOld != 0 {
			selectObject.Call(afkSingularityRotationDC, afkSingularityRotationOld)
		}
		if afkSingularityRotationBitmap != 0 {
			deleteObject.Call(afkSingularityRotationBitmap)
		}
		deleteDC.Call(afkSingularityRotationDC)
	}
	afkSingularityRotationDC = 0
	afkSingularityRotationBitmap = 0
	afkSingularityRotationOld = 0
	afkSingularityRotationBits = 0
	afkSingularitySurfaceDegree = -1
	afkSingularityRotationDegree = -1
	afkSingularityRotatedBGRA = nil
}

func afkStarbaseVisualElapsedSeconds() float64 {
	// Core Starbase world motion must never depend on mouse input, a resettable
	// epoch, or the global Reduced Motion toggle. Using the current monotonic-backed
	// process clock directly prevents the shared failure where the background,
	// Singularity artwork and Starbit simulation all freeze at phase zero.
	return float64(time.Now().UnixMilli()) / 1000.0
}

// drawAFKScrollingArtBackground intentionally follows the same two-tile,
// elapsed-time scrolling structure used by the established Precision/Endurance/
// Survival backgrounds. It is driven only by the Starbase animation heartbeat;
// mouse input never changes the phase.
func drawAFKScrollingArtBackground(hdc uintptr, r RECT, w, hgt int32) {
	const srcW int32 = 1672
	const srcH int32 = 941
	if len(starbaseBackgroundBGRA) < int(srcW*srcH*4) {
		drawAFKSpaceBackdrop(hdc, r, w, hgt)
		return
	}
	dstW := r.Right - r.Left
	dstH := r.Bottom - r.Top
	if dstW <= 0 || dstH <= 0 {
		return
	}
	spr := ensureRuntimeSprite(hdc, starbaseBackgroundBGRA, srcW, srcH)
	if spr == nil || spr.dc == 0 {
		drawAFKSpaceBackdrop(hdc, r, w, hgt)
		return
	}
	// This is intentionally the same proven pattern used by Endurance: a
	// persistent DIB, elapsed-time phase, and two adjacent full-width tiles.
	phase := 0.0
	if backgroundMotionEnabled() {
		phase = math.Mod(afkStarbaseVisualElapsedSeconds()*34.0, float64(dstW))
	}
	x0 := r.Left - int32(math.Round(phase))
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	saved, _, _ := saveDC.Call(hdc)
	if saved != 0 {
		defer restoreDC.Call(hdc, saved)
		intersectClipRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom))
	}
	for k := 0; k < 2; k++ {
		x := x0 + int32(k)*dstW
		alphaBlend.Call(hdc, uintptr(x), uintptr(r.Top), uintptr(dstW+1), uintptr(dstH), spr.dc, 0, 0, uintptr(srcW), uintptr(srcH), blend)
	}
}

// afkRotateSingularitySource builds an actual rotated BGRA frame in software.
// The previous implementation relied on a GDI world transform around AlphaBlend;
// on some Windows/GDI paths that transform is ignored for AlphaBlend, leaving the
// image visually static. This replacement rotates pixels explicitly, so clockwise
// motion is guaranteed regardless of graphics-driver behaviour.
func afkRotateSingularitySource(degree int) ([]byte, int32, int32) {
	const srcW int32 = 1254
	const srcH int32 = 1254
	const outW int32 = 768
	const outH int32 = 768
	if len(starbaseSingularityBGRA) < int(srcW*srcH*4) {
		return nil, 0, 0
	}
	degree %= 360
	if degree < 0 {
		degree += 360
	}
	if afkSingularityRotationDegree == degree && len(afkSingularityRotatedBGRA) == int(outW*outH*4) {
		return afkSingularityRotatedBGRA, outW, outH
	}
	need := int(outW * outH * 4)
	if len(afkSingularityRotatedBGRA) != need {
		afkSingularityRotatedBGRA = make([]byte, need)
	} else {
		clear(afkSingularityRotatedBGRA)
	}
	// Positive visual angle is clockwise in screen coordinates (Y grows down).
	a := float64(degree) * math.Pi / 180.0
	cs, sn := math.Cos(a), math.Sin(a)
	cxOut := float64(outW-1) * 0.5
	cyOut := float64(outH-1) * 0.5
	cxSrc := float64(srcW-1) * 0.5
	cySrc := float64(srcH-1) * 0.5
	scaleX := float64(srcW) / float64(outW)
	scaleY := float64(srcH) / float64(outH)
	for y := int32(0); y < outH; y++ {
		dy := (float64(y) - cyOut) * scaleY
		for x := int32(0); x < outW; x++ {
			dx := (float64(x) - cxOut) * scaleX
			// Inverse map output -> source for clockwise output rotation.
			sx0 := cs*dx + sn*dy + cxSrc
			sy0 := -sn*dx + cs*dy + cySrc
			sxI, syI := int32(math.Round(sx0)), int32(math.Round(sy0))
			if sxI < 0 || sxI >= srcW || syI < 0 || syI >= srcH {
				continue
			}
			si := int((syI*srcW + sxI) * 4)
			di := int((y*outW + x) * 4)
			copy(afkSingularityRotatedBGRA[di:di+4], starbaseSingularityBGRA[si:si+4])
		}
	}
	afkSingularityRotationDegree = degree
	return afkSingularityRotatedBGRA, outW, outH
}

func ensureAFKSingularityRotationSurface(hdc uintptr, data []byte, w, h int32, degree int) bool {
	if hdc == 0 || w <= 0 || h <= 0 || len(data) < int(w*h*4) {
		return false
	}
	if afkSingularityRotationDC == 0 {
		dc, _, _ := createCompatibleDC.Call(hdc)
		if dc == 0 {
			return false
		}
		var bits uintptr
		bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
			BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			BiWidth:       w,
			BiHeight:      -h,
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: BI_RGB,
		}}
		bmp, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
		if bmp == 0 || bits == 0 {
			deleteDC.Call(dc)
			return false
		}
		old, _, _ := selectObject.Call(dc, bmp)
		afkSingularityRotationDC = dc
		afkSingularityRotationBitmap = bmp
		afkSingularityRotationOld = old
		afkSingularityRotationBits = bits
		afkSingularitySurfaceDegree = -1
	}
	if afkSingularitySurfaceDegree != degree {
		copy(unsafe.Slice((*byte)(unsafe.Pointer(afkSingularityRotationBits)), int(w*h*4)), data[:int(w*h*4)])
		afkSingularitySurfaceDegree = degree
	}
	return true
}

func drawAFKRotatedSurfaceAlpha(hdc uintptr, data []byte, srcW, srcH int32, box RECT, alpha byte, degree int) RECT {
	if alpha == 0 || box.Right <= box.Left || box.Bottom <= box.Top || !ensureAFKSingularityRotationSurface(hdc, data, srcW, srcH, degree) {
		return RECT{}
	}
	boxW, boxH := box.Right-box.Left, box.Bottom-box.Top
	dstW := boxW
	dstH := int32(float64(dstW) * float64(srcH) / float64(srcW))
	if dstH > boxH {
		dstH = boxH
		dstW = int32(float64(dstH) * float64(srcW) / float64(srcH))
	}
	dstX := box.Left + (boxW-dstW)/2
	dstY := box.Top + (boxH-dstH)/2
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH), afkSingularityRotationDC, 0, 0, uintptr(srcW), uintptr(srcH), blend)
	return RECT{dstX, dstY, dstX + dstW, dstY + dstH}
}

func drawAFKRotatingSingularityBackdrop(hdc uintptr, w, hgt int32) {
	if !afkPrimaryWorldVisible() || len(starbaseSingularityBGRA) < 1254*1254*4 {
		return
	}
	cx, cy, radius := afkSingularityGeometry(w, hgt)
	field := afkMainFieldRect(w, hgt)
	dst := int32(radius * 5.15)
	if dst < 1 {
		return
	}
	// Clockwise world motion: one revolution every 24s. Reduced Motion freezes
	// this rotating Singularity layer while leaving the artwork itself visible.
	degree := 0
	if !gameMeta.ReducedMotion {
		degree = int(math.Mod(afkStarbaseVisualElapsedSeconds()*15.0, 360.0))
	}
	rotated, rw, rh := afkRotateSingularitySource(degree)
	if len(rotated) == 0 {
		return
	}
	saved, _, _ := saveDC.Call(hdc)
	if saved != 0 {
		defer restoreDC.Call(hdc, saved)
		intersectClipRect.Call(hdc, uintptr(field.Left+3), uintptr(field.Top+3), uintptr(field.Right-3), uintptr(field.Bottom-3))
	}
	const starbaseSingularityOpacity byte = 204 // exact 80% SourceConstantAlpha
	box := RECT{int32(cx) - dst/2, int32(cy) - dst/2, int32(cx) + dst/2, int32(cy) + dst/2}
	drawAFKRotatedSurfaceAlpha(hdc, rotated, rw, rh, box, starbaseSingularityOpacity, degree)
}

func drawAFKMoonRockForeground(hdc uintptr, w, hgt int32) {
	const srcW int32 = 2048
	const srcH int32 = 176
	if len(starbaseMoonRockBGRA) < int(srcW*srcH*4) || !afkPrimaryWorldVisible() {
		return
	}
	field := afkMainFieldRect(w, hgt)
	fieldW := field.Right - field.Left
	if fieldW <= 0 {
		return
	}
	// The moon-rock strip is the Starbase construction floor: a foreground layer
	// that spans the full arena width and covers the lower edge of both the
	// scrolling space background and rotating Singularity artwork. Major Starbase
	// facilities are rendered after this layer so they appear physically built on it.
	dstH := int32(float64(fieldW) * float64(srcH) / float64(srcW))
	minH := sy(84, hgt)
	maxH := sy(132, hgt)
	if dstH < minH {
		dstH = minH
	}
	if maxH > 0 && dstH > maxH {
		dstH = maxH
	}
	overscan := sx(22, w)
	box := RECT{field.Left - overscan, field.Bottom - dstH + sy(20, hgt), field.Right + overscan, field.Bottom + sy(20, hgt)}
	saved, _, _ := saveDC.Call(hdc)
	if saved != 0 {
		defer restoreDC.Call(hdc, saved)
		intersectClipRect.Call(hdc, uintptr(field.Left+3), uintptr(field.Top+3), uintptr(field.Right-3), uintptr(field.Bottom-3))
	}
	drawRawBGRAFit(hdc, starbaseMoonRockBGRA, srcW, srcH, box)
}

func drawAFKSpaceBackdrop(hdc uintptr, r RECT, w, hgt int32) {
	// Layered hard-coded space backdrop: deep navy base, subtle nebula bands,
	// horizon glow and sparse tech-grid details. This remains deterministic and
	// asset-free so the Starbase mode keeps its crisp 8-bit identity.
	fillSolidRect(hdc, r, rgb(0, 5, 18))
	bandH := sy(34, hgt)
	if bandH < 12 {
		bandH = 12
	}
	for i := int32(0); i < 7; i++ {
		y := r.Top + sy(22, hgt) + i*(bandH+sy(14, hgt))
		if y >= r.Bottom {
			break
		}
		c := rgb(3, 12, 31)
		if i%3 == 1 {
			c = rgb(8, 10, 39)
		}
		if i%3 == 2 {
			c = rgb(3, 19, 39)
		}
		fillSolidRect(hdc, RECT{r.Left + sx(6, w), y, r.Right - sx(6, w), func() int32 {
			v := y + bandH
			if v > r.Bottom {
				return r.Bottom
			}
			return v
		}()}, c)
	}
	// Faint perspective lanes keep the arena visually grounded without looking
	// like a debugging grid.
	grid := rgb(9, 31, 56)
	for i := int32(1); i < 8; i++ {
		x := r.Left + (r.Right-r.Left)*i/8
		drawLineSimple(hdc, x, r.Top+sy(10, hgt), x, r.Bottom-sy(10, hgt), 1, grid)
	}
	for i := int32(1); i < 6; i++ {
		y := r.Top + (r.Bottom-r.Top)*i/6
		drawLineSimple(hdc, r.Left+sx(10, w), y, r.Right-sx(10, w), y, 1, grid)
	}
	// Low horizon strips behind the Singularity give the arena a base/station
	// feel instead of an empty star field.
	horizonY := r.Bottom - sy(56, hgt)
	fillSolidRect(hdc, RECT{r.Left + sx(8, w), horizonY, r.Right - sx(8, w), horizonY + sy(4, hgt)}, rgb(11, 44, 72))
	fillSolidRect(hdc, RECT{r.Left + sx(8, w), horizonY + sy(7, hgt), r.Right - sx(8, w), horizonY + sy(9, hgt)}, rgb(25, 17, 58))
}

func drawAFKArenaChrome(hdc uintptr, r RECT, w, hgt int32) {
	// Angular corner hardware and inner rails make the arena feel like a proper
	// Starbase viewport instead of a generic bordered rectangle.
	cyan := rgb(57, 213, 255)
	violet := rgb(142, 78, 232)
	dim := rgb(24, 61, 92)
	cutX, cutY := sx(22, w), sy(14, hgt)
	// top-left / top-right rails
	drawLineSimple(hdc, r.Left+cutX, r.Top+sy(4, hgt), r.Left+sx(160, w), r.Top+sy(4, hgt), 2, cyan)
	drawLineSimple(hdc, r.Right-sx(160, w), r.Top+sy(4, hgt), r.Right-cutX, r.Top+sy(4, hgt), 2, cyan)
	drawLineSimple(hdc, r.Left+sx(8, w), r.Top+cutY, r.Left+sx(8, w), r.Top+sy(76, hgt), 2, dim)
	drawLineSimple(hdc, r.Right-sx(8, w), r.Top+cutY, r.Right-sx(8, w), r.Top+sy(76, hgt), 2, dim)
	// bottom accent rails
	drawLineSimple(hdc, r.Left+sx(24, w), r.Bottom-sy(8, hgt), r.Left+sx(210, w), r.Bottom-sy(8, hgt), 2, violet)
	drawLineSimple(hdc, r.Right-sx(210, w), r.Bottom-sy(8, hgt), r.Right-sx(24, w), r.Bottom-sy(8, hgt), 2, violet)
	// Corner blocks
	for _, q := range []RECT{
		{r.Left + sx(5, w), r.Top + sy(5, hgt), r.Left + sx(18, w), r.Top + sy(18, hgt)},
		{r.Right - sx(18, w), r.Top + sy(5, hgt), r.Right - sx(5, w), r.Top + sy(18, hgt)},
		{r.Left + sx(5, w), r.Bottom - sy(18, hgt), r.Left + sx(18, w), r.Bottom - sy(5, hgt)},
		{r.Right - sx(18, w), r.Bottom - sy(18, hgt), r.Right - sx(5, w), r.Bottom - sy(5, hgt)},
	} {
		fillSolidRect(hdc, q, cyan)
	}
}

func drawAFKWorldBadge(hdc uintptr, w, hgt int32) {
	if !afkPrimaryWorldVisible() || hudTinyFont == 0 {
		return
	}
	field := afkMainFieldRect(w, hgt)
	badge := RECT{field.Left + sx(24, w), field.Top + sy(18, hgt), field.Left + sx(280, w), field.Top + sy(54, hgt)}
	drawBevelPanel(hdc, badge, rgb(5, 24, 49), rgb(242, 192, 55), rgb(0, 6, 16), 2)
	old, _, _ := selectObject.Call(hdc, hudTinyFont)
	setBkMode.Call(hdc, TRANSPARENT)
	setTextColor.Call(hdc, rgb(255, 225, 116))
	textOut(hdc, badge.Left+sx(10, w), badge.Top+sy(7, hgt), fmt.Sprintf("STARBITS/sec  %s", afkStarbitsPerSecondText()))
	selectObject.Call(hdc, old)
}

func drawAFKPixelStarfield(hdc uintptr, r RECT, w, hgt int32) {
	// Deterministic integer sequence: the star map is identical on every frame
	// and on every machine, but still feels irregular.
	var seed uint32 = 0xC0A5E11
	width := r.Right - r.Left
	height := r.Bottom - r.Top
	if width <= 0 || height <= 0 {
		return
	}
	for i := 0; i < 78; i++ {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		x := r.Left + int32(seed%uint32(width))
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		y := r.Top + int32(seed%uint32(height))
		sz := int32(1)
		if i%11 == 0 {
			sz = 3
		} else if i%4 == 0 {
			sz = 2
		}
		c := rgb(111, 163, 207)
		switch i % 9 {
		case 0:
			c = rgb(238, 247, 255)
		case 1:
			c = rgb(105, 232, 255)
		case 2:
			c = rgb(185, 128, 255)
		}
		fillSolidRect(hdc, RECT{x, y, x + sz, y + sz}, c)
	}
}

func drawAFKPixelRing(hdc uintptr, cx, cy float64, radius, thickness int32, color uintptr, phase float64) {
	if radius <= 0 || thickness <= 0 {
		return
	}
	// Block-sampled circumference intentionally keeps a chunky 8-bit silhouette.
	segments := 72
	pixel := thickness
	if pixel < 2 {
		pixel = 2
	}
	for i := 0; i < segments; i++ {
		a := (float64(i)/float64(segments))*math.Pi*2 + phase
		x := int32(cx + math.Cos(a)*float64(radius))
		y := int32(cy + math.Sin(a)*float64(radius))
		if (i+int(phase*20))%7 == 0 {
			continue
		}
		fillSolidRect(hdc, RECT{x - pixel/2, y - pixel/2, x + pixel/2 + 1, y + pixel/2 + 1}, color)
	}
}

func drawAFKSingularity(hdc uintptr, w, hgt int32) {
	cx, cy, worldRadiusF := afkSingularityGeometry(w, hgt)
	_, _, coreRadiusF := afkSingularityCoreGeometry(w, hgt)
	worldRadius := int32(worldRadiusF)
	coreRadius := int32(coreRadiusF)
	elapsed := float64(time.Now().UnixNano()%int64(10*time.Second)) / float64(time.Second)
	reduced := gameMeta.ReducedMotion
	if reduced {
		elapsed = 0
	}

	// The surrounding accretion spiral/glow keeps a fixed size. It may rotate, but
	// it must never scale to the music; only the event-horizon core below pulses.
	drawAFKPixelRing(hdc, cx, cy, worldRadius+sx(60, w), 5, rgb(53, 46, 109), elapsed*0.08)
	drawAFKPixelRing(hdc, cx, cy, worldRadius+sx(43, w), 6, rgb(93, 65, 170), -elapsed*0.13)
	drawAFKPixelRing(hdc, cx, cy, worldRadius+sx(28, w), 7, rgb(116, 83, 224), elapsed*0.17)
	drawAFKPixelRing(hdc, cx, cy, worldRadius+sx(14, w), 6, rgb(76, 219, 255), -elapsed*0.22)

	// Cover the authored backdrop's original fixed black centre with a stable dark
	// cavity first. That prevents the source image's old full-size hole from showing
	// through behind the smaller pulsing core. The surrounding spiral stays fixed.
	for yy := -worldRadius; yy <= worldRadius; yy += 4 {
		fy := float64(yy) / float64(worldRadius)
		half := int32(math.Sqrt(math.Max(0, 1-fy*fy)) * float64(worldRadius))
		shade := rgb(6, 4, 20)
		if absInt32(yy) > worldRadius*4/5 {
			shade = rgb(11, 7, 31)
		}
		fillSolidRect(hdc, RECT{int32(cx) - half, int32(cy) + yy, int32(cx) + half + 1, int32(cy) + yy + 4}, shade)
	}

	// Pixelated black core. This is the only geometric layer that expands to the
	// snare beat. Resting radius is 90%; major snares bring it to the old full size.
	for yy := -coreRadius; yy <= coreRadius; yy += 4 {
		fy := float64(yy) / float64(coreRadius)
		half := int32(math.Sqrt(math.Max(0, 1-fy*fy)) * float64(coreRadius))
		shade := rgb(0, 0, 5)
		if absInt32(yy) > coreRadius*3/4 {
			shade = rgb(5, 4, 15)
		}
		fillSolidRect(hdc, RECT{int32(cx) - half, int32(cy) + yy, int32(cx) + half + 1, int32(cy) + yy + 4}, shade)
	}

	// Local event-horizon pixels move with the core so the pulse has a crisp edge.
	drawAFKPixelRing(hdc, cx, cy, coreRadius-sx(8, w), 5, rgb(134, 95, 255), elapsed*0.27)
	drawAFKPixelRing(hdc, cx, cy, coreRadius-sx(18, w), 3, rgb(53, 214, 255), -elapsed*0.31)

	// Hover cue is interaction UI, not music-reactive geometry.
	if afkManualExtractionAvailable() && pointInRect(cursorPos, afkSingularityHitRect(w, hgt)) {
		drawAFKPixelRing(hdc, cx, cy, worldRadius+sx(76, w), 5, rgb(255, 214, 66), elapsed*0.18)
	}
}

// drawAFKSingularityStarbits fills the event horizon with purely visual Starbit
// particles. Their density is derived from the authoritative Starbits/sec rate,
// but the particles never award currency; changing FPS or animation state cannot
// affect the economy. The logarithmic scale keeps late-game rates visually richer
// without allowing thousands of particles to hurt frame time.
func afkDesiredSingularityParticleCount() int {
	rateMilli := afkStarbitsPerSecondMilli()
	if rateMilli <= 0 {
		return 0
	}
	rate := float64(rateMilli) / 1000.0
	count := 8 + int(math.Log10(1.0+rate)*21.0)
	if count < 8 {
		count = 8
	}
	if count > 180 {
		count = 180
	}
	return count
}

func afkSpawnFreeStarbit(index int64) afkSingularityFreeParticle {
	// Deterministic pseudo-random initialization avoids importing math/rand and
	// keeps the visual cloud reproducible while every particle still behaves
	// independently after spawning.
	a := float64((index*137+29)%1009) / 1009.0 * math.Pi * 2
	r := 0.10 + 0.70*math.Sqrt(float64((index*271+61)%997)/997.0)
	speed := 0.22 + 0.34*float64((index*193+47)%991)/991.0
	dir := float64((index*83+11)%360) * math.Pi / 180.0
	return afkSingularityFreeParticle{
		X:    math.Cos(a) * r,
		Y:    math.Sin(a) * r,
		VX:   math.Cos(dir) * speed,
		VY:   math.Sin(dir) * speed,
		Seed: float64((index*317+17)%983) / 983.0 * math.Pi * 2,
		Size: int32(2 + index%3),
	}
}

func drawAFKSingularityStarbits(hdc uintptr, w, hgt int32) {
	if !afkPrimaryWorldVisible() {
		afkSingularityParticleLastTick = time.Time{}
		return
	}
	desired := afkDesiredSingularityParticleCount()
	for len(afkSingularityFreeParticles) < desired {
		afkSingularityFreeParticles = append(afkSingularityFreeParticles, afkSpawnFreeStarbit(afkSingularityParticleSerial))
		afkSingularityParticleSerial++
	}
	if len(afkSingularityFreeParticles) > desired {
		afkSingularityFreeParticles = afkSingularityFreeParticles[:desired]
	}
	if desired == 0 {
		return
	}

	now := time.Now()
	dt := 0.0
	if !afkSingularityParticleLastTick.IsZero() {
		dt = now.Sub(afkSingularityParticleLastTick).Seconds()
		if dt < 0 {
			dt = 0
		}
		if dt > 0.04 {
			dt = 0.04
		}
	}
	afkSingularityParticleLastTick = now
	// Starbits are part of the core Starbase world simulation and continue moving
	// independently even if the general UI Reduced Motion option is enabled.
	t := float64(now.UnixNano()) / float64(time.Second)
	if dt > 0 {
		// 1) Independent free motion + subtle individual steering.
		for i := range afkSingularityFreeParticles {
			p := &afkSingularityFreeParticles[i]
			p.VX += math.Cos(t*0.91+p.Seed*1.17) * 0.09 * dt
			p.VY += math.Sin(t*0.77+p.Seed*1.41) * 0.09 * dt
			speed := math.Hypot(p.VX, p.VY)
			if speed < 0.19 {
				s := 0.19 / math.Max(speed, 0.001)
				p.VX *= s
				p.VY *= s
			} else if speed > 0.70 {
				s := 0.70 / speed
				p.VX *= s
				p.VY *= s
			}
			p.X += p.VX * dt
			p.Y += p.VY * dt
		}

		// 2) Equal-mass particle/particle collisions. Pairwise work is capped at
		// 180 particles (~16k pairs), cheap enough for the dedicated Starbase tick.
		for i := 0; i < len(afkSingularityFreeParticles); i++ {
			a := &afkSingularityFreeParticles[i]
			ra := 0.010 + float64(a.Size)*0.0035
			for j := i + 1; j < len(afkSingularityFreeParticles); j++ {
				b := &afkSingularityFreeParticles[j]
				rb := 0.010 + float64(b.Size)*0.0035
				dx, dy := b.X-a.X, b.Y-a.Y
				minD := ra + rb
				d2 := dx*dx + dy*dy
				if d2 <= 0 || d2 >= minD*minD {
					continue
				}
				d := math.Sqrt(d2)
				nx, ny := dx/d, dy/d
				// Separate overlap equally so particles cannot remain glued together.
				overlap := minD - d
				a.X -= nx * overlap * 0.5
				a.Y -= ny * overlap * 0.5
				b.X += nx * overlap * 0.5
				b.Y += ny * overlap * 0.5
				// Resolve only when approaching; equal masses exchange normal velocity.
				rel := (b.VX-a.VX)*nx + (b.VY-a.VY)*ny
				if rel < 0 {
					impulse := -rel
					a.VX -= impulse * nx
					a.VY -= impulse * ny
					b.VX += impulse * nx
					b.VY += impulse * ny
				}
			}
		}

		// 3) Circular wall collision, accounting for each particle's own radius.
		for i := range afkSingularityFreeParticles {
			p := &afkSingularityFreeParticles[i]
			pr := 0.010 + float64(p.Size)*0.0035
			wall := 0.84 - pr
			d := math.Hypot(p.X, p.Y)
			if d > wall && d > 0 {
				nx, ny := p.X/d, p.Y/d
				p.X, p.Y = nx*wall, ny*wall
				dot := p.VX*nx + p.VY*ny
				if dot > 0 {
					p.VX -= 2 * dot * nx
					p.VY -= 2 * dot * ny
				}
				// Tiny unique tangent impulse avoids repeated diametric ping-pong.
				kick := 0.025 * math.Sin(p.Seed+t*0.23)
				p.VX += -ny * kick
				p.VY += nx * kick
			}
		}
	}

	cx, cy, radius := afkSingularityCoreGeometry(w, hgt)
	for i, p := range afkSingularityFreeParticles {
		x := int32(cx + p.X*radius)
		y := int32(cy + p.Y*radius)
		sz := sx(float64(p.Size), w)
		if sz < 2 {
			sz = 2
		}
		col := rgb(255, 194, 34)
		if i%5 == 0 {
			col = rgb(255, 224, 72)
		}
		fillSolidRect(hdc, RECT{x - sz, y - sz, x + sz + 1, y + sz + 1}, col)
		inner := sz / 2
		if inner < 1 {
			inner = 1
		}
		fillSolidRect(hdc, RECT{x - inner, y - inner, x + inner + 1, y + inner + 1}, rgb(255, 249, 180))
	}
}

func drawAFKAutoCursors(hdc uintptr, w, hgt int32) {
	if !afkCursorCoreBuilt() || gameMeta.AFKAutoCursors <= 0 {
		return
	}
	cx, cy, radius := afkSingularityGeometry(w, hgt)
	count := gameMeta.AFKAutoCursors
	if count > afkAutoCursorMax {
		count = afkAutoCursorMax
	}
	phase := 0.0
	if !gameMeta.ReducedMotion {
		phase = float64(time.Now().UnixMilli()%12000) / 12000.0 * math.Pi * 2
	}
	orbit := radius + float64(sx(94, w))
	pixel := sx(4, w)
	if pixel < 3 {
		pixel = 3
	}
	for i := 0; i < count; i++ {
		a := phase + float64(i)/float64(count)*math.Pi*2
		x := int32(cx + math.Cos(a)*orbit)
		y := int32(cy + math.Sin(a)*orbit*0.55)
		// Tiny hard-coded crosshair drone/cursor: cyan body, white core, violet tail.
		fillSolidRect(hdc, RECT{x - pixel*2, y - pixel/2, x + pixel*2 + 1, y + pixel/2 + 1}, rgb(80, 221, 255))
		fillSolidRect(hdc, RECT{x - pixel/2, y - pixel*2, x + pixel/2 + 1, y + pixel*2 + 1}, rgb(80, 221, 255))
		fillSolidRect(hdc, RECT{x - pixel/2, y - pixel/2, x + pixel/2 + 1, y + pixel/2 + 1}, rgb(245, 252, 255))
		tx := int32(float64(x) - math.Cos(a)*float64(pixel*4))
		ty := int32(float64(y) - math.Sin(a)*float64(pixel*4))
		fillSolidRect(hdc, RECT{tx - pixel/2, ty - pixel/2, tx + pixel/2 + 1, ty + pixel/2 + 1}, rgb(159, 94, 255))

		// Subtle extraction pulse: each cursor emits independently so the core feels
		// active without competing visually with Mining Drones or the Orbital beam.
		pulsePhase := math.Mod(float64(time.Now().UnixNano())/1e9+float64(i)*0.29, 1.65)
		if pulsePhase < 0.22 {
			t := pulsePhase / 0.22
			px := int32(float64(x) + (cx-float64(x))*t*0.72)
			py := int32(float64(y) + (cy-float64(y))*t*0.72)
			drawLineSimple(hdc, x, y, px, py, 1, rgb(87, 169, 204))
			fillSolidRect(hdc, RECT{px - 1, py - 1, px + 2, py + 2}, rgb(213, 249, 255))
		}
	}
}

func drawAFKCoreBuildBurst(hdc uintptr, w, hgt int32) {
	if afkCoreBuildBurstStarted.IsZero() {
		return
	}
	age := time.Since(afkCoreBuildBurstStarted)
	if age < 0 || age > 1100*time.Millisecond {
		afkCoreBuildBurstStarted = time.Time{}
		return
	}
	field := afkMainFieldRect(w, hgt)
	cx := (field.Left + field.Right) / 2
	baseY := field.Bottom - sy(46, hgt)
	t := float64(age) / float64(1100*time.Millisecond)
	// Large hard-edged pixel construction plume, matching the reference build reveal.
	for i := 0; i < 24; i++ {
		seed := float64((i*37)%97) / 97.0
		spread := (seed - 0.5) * float64(sx(180, w)) * (0.25 + t)
		rise := float64(sy(float64(24+(i%7)*15), hgt)) * (0.4 + 1.8*t)
		x := cx + int32(spread)
		y := baseY - int32(rise)
		sz := sx(float64(3+(i%3)*2), w)
		if sz < 2 {
			sz = 2
		}
		col := rgb(241, 250, 255)
		if i%4 == 0 {
			col = rgb(76, 214, 255)
		}
		if i%7 == 0 {
			col = rgb(178, 104, 255)
		}
		fillSolidRect(hdc, RECT{x - sz, y - sz, x + sz, y + sz}, col)
	}
}

func drawAFKCursorCore(hdc uintptr, w, hgt int32) {
	if !afkCursorCoreBuilt() {
		return
	}
	field := afkMainFieldRect(w, hgt)
	cx, cy, radius := afkSingularityGeometry(w, hgt)
	baseY := field.Bottom - sy(46, hgt)
	baseX := int32(cx)
	// Hard-coded 8-bit control plinth linked to the Singularity.
	drawLineSimple(hdc, baseX, baseY-sy(38, hgt), baseX, int32(cy)+int32(radius)+sy(18, hgt), 3, rgb(76, 111, 157))
	fillSolidRect(hdc, RECT{baseX - sx(72, w), baseY - sy(24, hgt), baseX + sx(72, w), baseY + sy(24, hgt)}, rgb(8, 38, 70))
	drawOutlineRect(hdc, RECT{baseX - sx(72, w), baseY - sy(24, hgt), baseX + sx(72, w), baseY + sy(24, hgt)}, rgb(85, 217, 255), 3)
	fillSolidRect(hdc, RECT{baseX - sx(48, w), baseY - sy(12, hgt), baseX + sx(48, w), baseY + sy(10, hgt)}, rgb(18, 20, 58))
	fillSolidRect(hdc, RECT{baseX - sx(7, w), baseY - sy(15, hgt), baseX + sx(7, w), baseY + sy(13, hgt)}, rgb(164, 101, 255))
	fillSolidRect(hdc, RECT{baseX - sx(3, w), baseY - sy(11, hgt), baseX + sx(3, w), baseY + sy(9, hgt)}, rgb(231, 246, 255))
	if hudTinyFont != 0 && baseY+sy(34, hgt) < field.Bottom-sy(8, hgt) {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(111, 215, 243))
		centeredTextOut(hdc, baseX-sx(130, w), baseX+sx(130, w), baseY+sy(32, hgt), fmt.Sprintf("CURSOR CORE // TIER %d", afkCoreTier()))
		selectObject.Call(hdc, old)
	}
}

func absInt32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func drawAFKStatCard(hdc uintptr, r RECT, label, value string, accent uintptr) {
	// Compact command-deck telemetry card: dark inset, coloured top rail and a
	// dedicated label strip so the value is never fighting the label for space.
	drawBevelPanel(hdc, r, rgb(4, 22, 47), rgb(35, 68, 101), rgb(0, 7, 18), 2)
	fillSolidRect(hdc, RECT{r.Left + 2, r.Top + 2, r.Right - 2, r.Top + 4}, accent)
	labelBottom := r.Top + (r.Bottom-r.Top)*2/5
	fillSolidRect(hdc, RECT{r.Left + 3, r.Top + 5, r.Right - 3, labelBottom}, rgb(3, 16, 36))
	// small accent notch
	fillSolidRect(hdc, RECT{r.Left + 4, labelBottom - 3, r.Left + sx(24, r.Right-r.Left), labelBottom}, accent)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(142, 180, 207))
		displayLabel := fitTextEllipsis(hdc, hudTinyFont, label, r.Right-r.Left-sx(12, r.Right-r.Left))
		centeredTextOut(hdc, r.Left+6, r.Right-6, r.Top+7, displayLabel)
		selectObject.Call(hdc, old)
	}
	// Telemetry values deliberately use the tiny HUD face. The previous stat
	// font was tall enough to overlap the label strip on compact resolutions.
	valueFont := hudTinyFont
	if valueFont == 0 {
		valueFont = hudSmallFont
	}
	if valueFont != 0 {
		old, _, _ := selectObject.Call(hdc, valueFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(250, 252, 255))
		valueArea := RECT{r.Left + 4, labelBottom, r.Right - 4, r.Bottom - 3}
		displayValue := fitTextEllipsis(hdc, valueFont, value, valueArea.Right-valueArea.Left)
		centeredTextOut(hdc, valueArea.Left, valueArea.Right, verticallyCenteredTextY(hdc, valueFont, valueArea, displayValue)+3, displayValue)
		selectObject.Call(hdc, old)
	}
}

func drawAFKProgressBar(hdc uintptr, r RECT, current, target int64) {
	fillSolidRect(hdc, r, rgb(2, 10, 24))
	drawOutlineRect(hdc, r, rgb(52, 123, 179), 2)
	if target <= 0 {
		return
	}
	frac := float64(current) / float64(target)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	inner := RECT{r.Left + 3, r.Top + 3, r.Right - 3, r.Bottom - 3}
	fillW := int32(float64(inner.Right-inner.Left) * frac)
	if fillW > 0 {
		fillSolidRect(hdc, RECT{inner.Left, inner.Top, inner.Left + fillW, inner.Bottom}, rgb(68, 216, 255))
		if fillW > sx(10, r.Right-r.Left) {
			fillSolidRect(hdc, RECT{inner.Left, inner.Top, inner.Left + fillW, inner.Top + 3}, rgb(170, 245, 255))
		}
	}
}

func drawAFKClickBursts(hdc uintptr, w, hgt int32) {
	now := time.Now()
	kept := afkClickBursts[:0]
	for _, b := range afkClickBursts {
		age := now.Sub(b.Started)
		if age > 850*time.Millisecond {
			continue
		}
		kept = append(kept, b)
		t := float64(age) / float64(850*time.Millisecond)
		x := int32(b.X)
		y := int32(b.Y - t*float64(sy(54, hgt)))
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 224, 73))
			text := fmt.Sprintf("+%d", b.Amount)
			sz := textPixelSize(hdc, hudSmallFont, text)
			textOut(hdc, x-sz.Cx/2, y, text)
			selectObject.Call(hdc, old)
		}
		// Four hard square sparks.
		spread := int32(t * float64(sx(32, w)))
		s := int32(3)
		fillSolidRect(hdc, RECT{x - spread, y + 18, x - spread + s, y + 18 + s}, rgb(101, 227, 255))
		fillSolidRect(hdc, RECT{x + spread, y + 8, x + spread + s, y + 8 + s}, rgb(166, 112, 255))
	}
	afkClickBursts = kept
}

func afkPrimaryWorldVisible() bool {
	return !afkFacilitiesPanelOpen && !afkGuidePanelOpen && !afkExpeditionPanelOpen && !afkTechPanelOpen && !afkTalentPanelOpen && !afkOperatorPanelOpen && !afkModulePanelOpen && !afkDronePanelOpen && !afkOrbitalPanelOpen && !afkPrestigePanelOpen && !afkEarningsPanelOpen && !afkSystemsMenuOpen
}

// Starbit flow is deliberately cosmetic. Production remains entirely in the
// economy tick/server-sync path, so animation cadence or dropped frames can
// never create extra currency. The particles simply make passive production
// visibly travel from the extraction network into the Starbits counter.
func drawAFKStarbitFlow(hdc uintptr, w, hgt int32) {
	if !afkCursorCoreBuilt() || !afkPrimaryWorldVisible() {
		afkStarbitFlowParticles = afkStarbitFlowParticles[:0]
		return
	}
	now := time.Now()
	rateMilli := afkStarbitsPerSecondMilli()
	if rateMilli <= 0 {
		return
	}
	if afkNextStarbitFlow.IsZero() || !now.Before(afkNextStarbitFlow) {
		cx, cy, radius := afkSingularityGeometry(w, hgt)
		serial := afkStarbitFlowSerial
		afkStarbitFlowSerial++
		angle := float64((serial*137)%360) * math.Pi / 180.0
		ring := radius * (0.82 + 0.18*math.Sin(float64(serial)*1.71))
		x := cx + math.Cos(angle)*ring
		y := cy + math.Sin(angle)*ring*0.56
		// About three visual collection packets per second. The shown number is
		// an informational slice of the current passive rate, never an award.
		amount := rateMilli * 340 / 1_000_000
		if amount < 1 {
			amount = 1
		}
		afkStarbitFlowParticles = append(afkStarbitFlowParticles, afkStarbitFlowParticle{Started: now, X: x, Y: y, Amount: amount, Index: serial})
		if len(afkStarbitFlowParticles) > 24 {
			afkStarbitFlowParticles = afkStarbitFlowParticles[len(afkStarbitFlowParticles)-24:]
		}
		afkNextStarbitFlow = now.Add(340 * time.Millisecond)
	}

	target := afkLiveStatCardRect(w, hgt, 0)
	tx := float64(target.Left+target.Right) * 0.5
	ty := float64(target.Top+target.Bottom) * 0.5
	kept := afkStarbitFlowParticles[:0]
	for _, p := range afkStarbitFlowParticles {
		age := now.Sub(p.Started)
		if age >= 1150*time.Millisecond {
			continue
		}
		kept = append(kept, p)
		t := float64(age) / float64(1150*time.Millisecond)
		// Smoothstep keeps the motion deliberate rather than looking like a
		// straight debug line toward the HUD.
		ease := t * t * (3 - 2*t)
		arc := math.Sin(t*math.Pi) * float64(sy(56, hgt))
		x := p.X + (tx-p.X)*ease
		y := p.Y + (ty-p.Y)*ease - arc
		sz := sx(5, w)
		if sz < 3 {
			sz = 3
		}
		ix, iy := int32(x), int32(y)
		fillSolidRect(hdc, RECT{ix - sz, iy - sz, ix + sz, iy + sz}, rgb(255, 195, 35))
		fillSolidRect(hdc, RECT{ix - sz/2, iy - sz/2, ix + sz/2 + 1, iy + sz/2 + 1}, rgb(255, 247, 164))
		// Pixel trail.
		for j := int32(1); j <= 3; j++ {
			ps := sz - j
			if ps < 2 {
				ps = 2
			}
			px := int32(p.X + (tx-p.X)*math.Max(0, ease-float64(j)*0.045))
			py := int32(p.Y + (ty-p.Y)*math.Max(0, ease-float64(j)*0.045) - arc)
			fillSolidRect(hdc, RECT{px - ps/2, py - ps/2, px + ps/2, py + ps/2}, rgb(242, 145, 28))
		}
		// Only every third packet carries text so the playfield stays readable.
		if p.Index%3 == 0 && t < 0.72 && hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 224, 73))
			text := "+" + formatAFKNumber(p.Amount)
			textOut(hdc, ix+sx(9, w), iy-sy(7, hgt), text)
			selectObject.Call(hdc, old)
		}
	}
	afkStarbitFlowParticles = kept
}

func drawAFKActionButton(hdc uintptr, r RECT, label string, cost int64, enabled, hovered bool) {
	face, light, dark := themedHUDPanelPalette(enabled)
	fill := blendColor(face, dark, 0.14)
	outline := light
	textc := rgb(239, 250, 255)
	if !enabled {
		fill = blendColor(face, dark, 0.34)
		outline = blendColor(light, dark, 0.48)
		textc = blendColor(light, rgb(128, 128, 128), 0.54)
	}
	drawBevelPanel(hdc, r, fill, outline, dark, 2)
	if enabled && hovered {
		drawOutlineRect(hdc, r, rgb(255, 221, 78), 2)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, textc)
		text := label
		if cost > 0 {
			text = fmt.Sprintf("%s // %s", label, formatAFKNumber(cost))
		}
		display := fitTextEllipsis(hdc, hudTinyFont, text, r.Right-r.Left-sx(18, r.Right-r.Left))
		centeredTextOut(hdc, r.Left+sx(9, r.Right-r.Left), r.Right-sx(9, r.Right-r.Left), verticallyCenteredTextY(hdc, hudTinyFont, r, display), display)
		selectObject.Call(hdc, old)
	}
}

func afkActiveCacheRect(w, hgt int32) RECT { return afkPhysicalCacheRect(w, hgt) }

func drawAFKActiveCache(hdc uintptr, w, hgt int32) {
	drawAFKPhysicalCache(hdc, w, hgt)
}

func afkActiveCacheClaimRect(w, hgt int32) RECT { return afkPhysicalCacheRect(w, hgt) }

func drawAFKWelcomeBack(hdc uintptr, w, hgt int32) {
	if !afkHasOfflineReward() {
		return
	}
	// Darken only the AFK panel behind the hard-coded reward card.
	panel := afkPanelRect(w, hgt)
	overlaySolidAlphaRect(hdc, panel, rgb(0, 0, 8), 190)
	r := afkWelcomeRect(w, hgt)
	drawBevelPanel(hdc, r, rgb(4, 20, 43), rgb(91, 218, 255), rgb(0, 5, 16), 4)
	drawOutlineRect(hdc, RECT{r.Left + 7, r.Top + 7, r.Right - 7, r.Bottom - 7}, rgb(117, 75, 219), 2)

	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(245, 250, 255))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(28, hgt), "WELCOME BACK")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(120, 190, 221))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(82, hgt), fmt.Sprintf("AWAY FOR %s", afkFormatDuration(gameMeta.AFKOfflinePendingAwaySeconds)))
		if gameMeta.AFKOfflinePendingAwaySeconds > gameMeta.AFKOfflinePendingPaidSeconds {
			setTextColor.Call(hdc, rgb(255, 204, 78))
			centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(106, hgt), fmt.Sprintf("AFK CAPACITY CREDITED %s / %s", afkFormatDuration(gameMeta.AFKOfflinePendingPaidSeconds), afkFormatDuration(afkCapacitySeconds())))
		} else {
			setTextColor.Call(hdc, rgb(132, 158, 181))
			centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(106, hgt), fmt.Sprintf("AFK CAPACITY %s", afkFormatDuration(afkCapacitySeconds())))
		}
		selectObject.Call(hdc, old)
	}

	energyCard := RECT{r.Left + sx(46, w), r.Top + sy(148, hgt), r.Right - sx(46, w), r.Top + sy(212, hgt)}
	drawAFKStatCard(hdc, energyCard, "STARBITS EARNED", formatAFKCountUpReward(gameMeta.AFKOfflinePendingStarbits), rgb(76, 214, 255))
	capacityCard := RECT{r.Left + sx(46, w), r.Top + sy(224, hgt), r.Right - sx(46, w), r.Top + sy(288, hgt)}
	drawAFKStatCard(hdc, capacityCard, "AFK CAPACITY USED", afkFormatDuration(gameMeta.AFKOfflinePendingPaidSeconds), rgb(242, 192, 55))

	claim := afkWelcomeClaimRect(w, hgt)
	drawAFKActionButton(hdc, claim, "CLAIM AFK EARNINGS", 0, true, pointInRect(cursorPos, claim))
}

func drawAFKSingularityOverlay(hdc uintptr, w, hgt int32) {
	resetAFKWelcomeCountIfNeeded()
	if overlayMode != OverlayAFKSingularity && !(overlayMode == OverlaySettings && starbaseSettingsReturn) {
		return
	}

	// AFK is presented as a real game mode: the arena is the animated world and
	// the entire normal lower HUD is replaced by a dedicated AFK operations dock.
	overlayBlack(hdc, w, hgt, 236)
	fillSolidRect(hdc, RECT{0, 0, w, sy(198, hgt)}, rgb(1, 12, 34))
	// Layered command-deck header with gold Starbase identity and cyan tech rails.
	fillSolidRect(hdc, RECT{0, sy(148, hgt), w, sy(198, hgt)}, rgb(2, 20, 47))
	drawLineSimple(hdc, 0, sy(194, hgt), w, sy(194, hgt), 3, rgb(57, 213, 255))
	drawLineSimple(hdc, sx(300, w), sy(114, hgt), w-sx(300, w), sy(114, hgt), 2, rgb(86, 52, 145))
	// v441: use the new authored Starbase wordmark instead of the legacy text
	// label so the mode has a proper visual identity across every Starbase view.
	// Keep the identity physically inside the clear centre gap between the
	// STARBITS / SEC and NAV DATA cards at every supported resolution.
	prodCard := afkLiveStatCardRect(w, hgt, 1)
	navCard := afkLiveStatCardRect(w, hgt, 2)
	logoLeft := prodCard.Right + sx(12, w)
	logoRight := navCard.Left - sx(12, w)
	if logoRight < logoLeft+sx(180, w) {
		logoLeft = w/2 - sx(100, w)
		logoRight = w/2 + sx(100, w)
	}
	logoW := logoRight - logoLeft
	logoH := int32(float64(logoW) * 129.0 / 948.0)
	maxLogoH := sy(62, hgt)
	if logoH > maxLogoH {
		logoH = maxLogoH
	}
	logoY := sy(29, hgt)
	if len(starbaseLogoWordmarkBGRA) >= 948*129*4 {
		drawRawBGRATrimmedFit(hdc, starbaseLogoWordmarkBGRA, 948, 129, RECT{logoLeft, logoY, logoRight, logoY + logoH})
	} else if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 215, 70))
		centeredTextOut(hdc, logoLeft, logoRight, logoY, "STARBASE")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(219, 240, 251))
		centeredTextOut(hdc, logoLeft, logoRight, logoY+logoH+sy(3, hgt), "SINGULARITY NETWORK")
		selectObject.Call(hdc, old)
	}
	back := afkBackRect(w, hgt)
	drawAFKActionButton(hdc, back, "< BACK", 0, true, pointInRect(cursorPos, back))

	field := afkMainFieldRect(w, hgt)
	// Frame first, then artwork inside it. The previous order painted an opaque
	// bevel panel over the scrolling background, which made the animation appear
	// frozen/absent and also prevented the 80%-opacity Singularity from revealing
	// the space layer beneath it.
	drawBevelPanel(hdc, field, rgb(1, 8, 22), rgb(45, 83, 139), rgb(0, 2, 8), 3)
	drawAFKScrollingArtBackground(hdc, field, w, hgt)
	drawAFKArenaChrome(hdc, field, w, hgt)
	drawAFKRotatingSingularityBackdrop(hdc, w, hgt)
	drawAFKSingularity(hdc, w, hgt)
	drawAFKSingularityStarbits(hdc, w, hgt)
	drawAFKAutoCursors(hdc, w, hgt)
	drawAFKMoonRockForeground(hdc, w, hgt)
	drawAFKPhysicalStarbaseWorld(hdc, w, hgt)
	drawAFKCoreBuildBurst(hdc, w, hgt)
	drawAFKDroneWorld(hdc, w, hgt)
	drawAFKOrbitalWorld(hdc, w, hgt)
	drawAFKClickBursts(hdc, w, hgt)

	// Facility sub-screens replace the arena content but never steal the bottom HUD.
	drawAFKExpeditionLayer(hdc, w, hgt)
	drawAFKTechLab(hdc, w, hgt)
	drawAFKTalentTree(hdc, w, hgt)
	drawAFKOperators(hdc, w, hgt)
	drawAFKModules(hdc, w, hgt)
	drawAFKDrones(hdc, w, hgt)
	drawAFKOrbital(hdc, w, hgt)
	drawAFKPrestige(hdc, w, hgt)
	drawAFKFacilitiesHub(hdc, w, hgt)
	drawAFKGuide(hdc, w, hgt)
	drawAFKSystemsMenu(hdc, w, hgt)
	drawAFKExchange(hdc, w, hgt)
	drawAFKExpeditionReturnNotification(hdc, w, hgt)
	drawAFKOperatorWaitNotification(hdc, w, hgt)

	// Keep the one-time manual extraction hint only before the Cursor Core is built.
	// Once automation is online, the world stays visually clean with no persistent
	// "AUTO CURSORS..." footer competing with the moon-base scene.
	if afkPrimaryWorldVisible() && !afkCursorCoreBuilt() && hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(109, 229, 255))
		msg := "CLICK THE SINGULARITY TO EXTRACT +5 STARBITS"
		promptR := RECT{field.Left + sx(180, w), field.Bottom - sy(60, hgt), field.Right - sx(180, w), field.Bottom - sy(32, hgt)}
		msg = fitTextEllipsis(hdc, hudSmallFont, msg, promptR.Right-promptR.Left)
		centeredTextOut(hdc, promptR.Left, promptR.Right, verticallyCenteredTextY(hdc, hudSmallFont, promptR, msg), msg)
		selectObject.Call(hdc, old)
	}

	drawAFKActiveCache(hdc, w, hgt)
	drawAFKBottomHUD(hdc, w, hgt)
	drawAFKStarbitFlow(hdc, w, hgt)
	drawAFKWelcomeBack(hdc, w, hgt)
	drawAFKMilestoneBanner(hdc, w, hgt)
}

func handleAFKSingularityClick(h uintptr, p FPoint, w, hgt int32) bool {
	if overlayMode != OverlayAFKSingularity {
		return false
	}
	if afkHasOfflineReward() {
		if pointInRect(p, afkWelcomeClaimRect(w, hgt)) {
			afkClaimOfflineReward()
			saveGameMeta()
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
		}
		return true
	}
	if gameMeta.AFKActiveCacheReward > 0 && pointInRect(p, afkActiveCacheClaimRect(w, hgt)) {
		if afkClaimActiveCache(time.Now()) {
			saveGameMeta()
			playUIButtonClickSound()
		}
		invalidateRect.Call(h, 0, 0)
		return true
	}
	if pointInRect(p, afkBackRect(w, hgt)) {
		playUIButtonClickSound()
		saveGameMeta()
		// Starbase BACK is a direct return to Precision. Do not detour through
		// the ESC/main menu or restore whichever mode happened to open Starbase.
		selectGameMode(h, 0)
		return true
	}
	if handleAFKExpeditionReturnNotificationClick(h, p, w, hgt) {
		return true
	}
	if handleAFKOperatorWaitNotificationClick(h, p, w, hgt) {
		return true
	}
	if handleAFKUnlockedShipFlybyClick(h, p, w, hgt) {
		return true
	}
	if handleAFKBottomHUDClick(h, p, w, hgt) {
		return true
	}
	if handleAFKPhysicalBuildingClick(h, p, w, hgt) {
		return true
	}
	if afkExpeditionPanelOpen {
		if handleAFKExpeditionClick(h, p, w, hgt) {
			return true
		}
		return true
	}
	if afkTechPanelOpen || afkTalentPanelOpen {
		if handleAFKSection6Click(h, p, w, hgt) {
			return true
		}
		return true
	}
	if afkOperatorPanelOpen {
		for i := range afkOperatorDefs {
			if pointInRect(p, afkOperatorCardRect(w, hgt, i)) {
				afkOperatorSelected = i
				// Card interaction is intentionally physical: READY means promote;
				// otherwise an idle recruited Operator starts their paid assignment.
				changed := false
				if afkOperatorRecruited(i) {
					if afkOperatorCanLevel(i) {
						changed = afkLevelUpOperator(i)
					} else {
						changed = afkStartOperatorAssignment(i, time.Now())
					}
				}
				if changed {
					saveGameMeta()
				}
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return true
			}
		}
		return true
	}
	if afkDronePanelOpen {
		for i := 0; i < afkDroneUpgradeCount; i++ {
			if pointInRect(p, afkDroneUpgradeRect(w, hgt, i)) {
				afkDroneSelectedUpgrade = i
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return true
			}
		}
	}
	if afkOrbitalPanelOpen && afkOrbitalBuilt() {
		for i := 0; i < afkOrbitalUpgradeCount; i++ {
			if pointInRect(p, afkOrbitalUpgradeRect(w, hgt, i)) {
				afkOrbitalSelectedUpgrade = i
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return true
			}
		}
	}
	if afkModulePanelOpen {
		if afkModuleSelectedItem < 0 {
			for i := 0; i < afkModuleSetCount; i++ {
				if pointInRect(p, afkModuleSetRect(w, hgt, i)) {
					afkModuleSelectedSet = i
					afkModuleSelectedItem = 0
					playUIButtonClickSound()
					invalidateRect.Call(h, 0, 0)
					return true
				}
			}
		} else {
			for i := 0; i < afkModuleItemsPerSet; i++ {
				if pointInRect(p, afkModuleItemRect(w, hgt, i)) {
					afkModuleSelectedItem = i
					playUIButtonClickSound()
					invalidateRect.Call(h, 0, 0)
					return true
				}
			}
		}
		return true
	}
	if afkManualExtractionAvailable() && pointInRect(p, afkSingularityHitRect(w, hgt)) {
		now := time.Now()
		if !afkManualClickAllowed(now) {
			return true
		}
		amount := afkManualExtractionValue()
		before := gameMeta.AFKStarbits
		next := saturatingAdd(before, amount)
		if !afkSection1IsComplete() && next >= afkSection1StarbitTarget {
			next = afkSection1StarbitTarget
			gameMeta.AFKSection1Complete = true
		}
		gain := next - before
		if gain > 0 {
			afkGrantStarbits(gain)
		}
		afkCloudRecordManualGain(gain)
		saveGameMeta()
		playAFKStarbitCollectSound()
		afkClickBursts = append(afkClickBursts, afkClickBurst{Started: now, X: p.X, Y: p.Y, Amount: amount})
		if len(afkClickBursts) > 14 {
			afkClickBursts = afkClickBursts[len(afkClickBursts)-14:]
		}
		invalidateRect.Call(h, 0, 0)
		return true
	}
	return true
}
