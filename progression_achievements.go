//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func activeDifficultyIndex() int {
	if adaptiveMode {
		if adaptiveTier < 0 {
			adaptiveTier = 0
		}
		if adaptiveTier > 3 {
			adaptiveTier = 3
		}
		return adaptiveTier
	}
	return difficulty
}

func difficultyDisplay() (string, uintptr) {
	idx := activeDifficultyIndex()
	name := diffs[idx].name
	color := diffs[idx].color
	if adaptiveMode {
		name = "ADAPTIVE · " + name
		color = rgb(39, 210, 255)
	}
	return name, color
}

func effectiveCourse() int {
	// v274 player rule: Precision always chooses a fresh random archetype.
	// Legacy pathMode remains only for save/source compatibility.
	return 1 + rand.Intn(5)
}

func movingTargetsActive() bool {
	// Moving targets are now part of difficulty identity rather than a player toggle.
	// Easy = static. Normal, Hard and Insane = always moving.
	return activeDifficultyIndex() >= 1
}

func findSeparatedTargetIndex(desired, minIdx, maxIdx int, existing []int, minPixels float64) int {
	if desired < minIdx {
		desired = minIdx
	}
	if desired > maxIdx {
		desired = maxIdx
	}

	valid := func(idx int) bool {
		if idx < minIdx || idx > maxIdx || idx < 0 || idx >= len(path) {
			return false
		}
		for _, other := range existing {
			if other < 0 || other >= len(path) {
				continue
			}
			if math.Abs(float64(idx-other)) < 34 {
				return false
			}
			if dist(path[idx], path[other]) < minPixels {
				return false
			}
		}
		return true
	}

	if valid(desired) {
		return desired
	}

	span := maxIdx - minIdx
	for delta := 1; delta <= span; delta++ {
		if valid(desired - delta) {
			return desired - delta
		}
		if valid(desired + delta) {
			return desired + delta
		}
	}

	return desired
}

func allTargetCurrentIndices() []int {
	out := make([]int, len(targets))
	if len(path) == 0 {
		return out
	}

	elapsed := time.Since(startTime).Seconds()
	used := make([]int, 0, len(targets))

	for i, t := range targets {
		idx := t.Index

		if !t.Clicked && movingTargetsActive() && state == StatePlaying && t.MoveRange > 0 {
			// Moving the path INDEX means the target follows the actual rail
			// rather than moving in a straight screen-space line.
			offset := int(math.Round(math.Sin(elapsed*t.Speed+t.Phase) * float64(t.MoveRange)))
			idx = t.Index + offset
			if idx < t.MinIndex {
				idx = t.MinIndex
			}
			if idx > t.MaxIndex {
				idx = t.MaxIndex
			}
		}

		// Runtime safety: even if two distant sections of the rail bend close
		// together visually, targets are kept apart on screen.
		if !t.Clicked {
			idx = findSeparatedTargetIndex(idx, t.MinIndex, t.MaxIndex, used, 48.0)
		}

		if idx < 0 {
			idx = 0
		}
		if idx >= len(path) {
			idx = len(path) - 1
		}

		out[i] = idx
		used = append(used, idx)
	}
	return out
}

func targetCurrentIndex(i int) int {
	if i < 0 || i >= len(targets) || len(path) == 0 {
		return 0
	}
	indices := allTargetCurrentIndices()
	return indices[i]
}

func targetCurrentPoint(i int) FPoint {
	if i < 0 || i >= len(targets) || len(path) == 0 {
		return FPoint{}
	}
	var p FPoint
	if targets[i].Clicked {
		p = targets[i].Point
	} else {
		p = path[targetCurrentIndex(i)]
	}
	if enduranceActive() {
		p.X -= enduranceCameraX
	}
	return p
}

func trackingAccuracyPercent() float64 {
	if trackAccuracySamples <= 0 {
		return 100
	}
	v := 100 * trackAccuracySum / float64(trackAccuracySamples)
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}

func targetAccuracyPercent() float64 {
	if targetPrecisionHits <= 0 {
		return 100
	}
	v := 100 * targetPrecisionSum / float64(targetPrecisionHits)
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}

func combinedAccuracyPercent() float64 {
	return trackingAccuracyPercent()*0.75 + targetAccuracyPercent()*0.25
}

func ratingForAccuracy(v float64) string {
	switch {
	case v >= 95:
		return "S"
	case v >= 88:
		return "A"
	case v >= 78:
		return "B"
	case v >= 65:
		return "C"
	default:
		return "D"
	}
}

func comboForStreak(nextStreak int) float64 {
	switch {
	case nextStreak >= 15:
		return 2.5
	case nextStreak >= 10:
		return 2.0
	case nextStreak >= 6:
		return 1.5
	case nextStreak >= 3:
		return 1.25
	default:
		return 1.0
	}
}

type ExpRank struct {
	Name      string
	Threshold int
}

var expRanks = []ExpRank{
	// v51 progression rebalance. Early ranks still move quickly, while
	// Platinum, Diamond and especially Master require sustained play.
	{"BRONZE I", 0},
	{"BRONZE II", 100},
	{"BRONZE III", 300},
	{"SILVER I", 750},
	{"SILVER II", 1500},
	{"SILVER III", 2500},
	{"GOLD I", 4000},
	{"GOLD II", 6000},
	{"GOLD III", 8500},
	{"PLATINUM I", 12000},
	{"PLATINUM II", 16000},
	{"PLATINUM III", 22000},
	{"DIAMOND I", 30000},
	{"DIAMOND II", 40000},
	{"DIAMOND III", 55000},
	{"MASTER I", 75000},
	{"MASTER II", 100000},
	{"MASTER III", 135000},
}

func expForDifficultyName(name string) int {
	switch strings.ToUpper(name) {
	case "EASY":
		return 1
	case "NORMAL":
		return 5
	case "HARD":
		return 20
	case "INSANE":
		return 75
	default:
		// Adaptive awards based on its underlying active tier.
		switch activeDifficultyIndex() {
		case 0:
			return 1
		case 1:
			return 5
		case 2:
			return 20
		default:
			return 75
		}
	}
}

func rankIndexForEXP(exp int) int {
	idx := 0
	for i, r := range expRanks {
		if exp >= r.Threshold {
			idx = i
		} else {
			break
		}
	}
	return idx
}

func rankForEXP(exp int) string {
	return expRanks[rankIndexForEXP(exp)].Name
}

func nextRankProgress(exp int) (current ExpRank, next ExpRank, hasNext bool, currentInto int, needed int) {
	idx := rankIndexForEXP(exp)
	current = expRanks[idx]
	if idx >= len(expRanks)-1 {
		return current, ExpRank{}, false, exp - current.Threshold, 0
	}
	next = expRanks[idx+1]
	return current, next, true, exp - current.Threshold, next.Threshold - current.Threshold
}

func difficultyCompletionCount(name string) int {
	switch strings.ToUpper(name) {
	case "EASY":
		return playerProgress.EasyCompleted
	case "NORMAL":
		return playerProgress.NormalCompleted
	case "HARD":
		return playerProgress.HardCompleted
	case "INSANE":
		return playerProgress.InsaneCompleted
	}
	return 0
}

func addDifficultyCompletion(name string) {
	switch strings.ToUpper(name) {
	case "EASY":
		playerProgress.EasyCompleted++
	case "NORMAL":
		playerProgress.NormalCompleted++
	case "HARD":
		playerProgress.HardCompleted++
	case "INSANE":
		playerProgress.InsaneCompleted++
	default:
		// Adaptive counts toward the actual tier played.
		switch activeDifficultyIndex() {
		case 0:
			playerProgress.EasyCompleted++
		case 1:
			playerProgress.NormalCompleted++
		case 2:
			playerProgress.HardCompleted++
		default:
			playerProgress.InsaneCompleted++
		}
	}
}

func localProgressPath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if cfg, err := os.UserConfigDir(); err == nil {
			base = cfg
		}
	}
	if base == "" {
		base = "."
	}
	dir := filepath.Join(base, "KongGame", "CursorControlTrainer")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "progress.json")
}

func loadPlayerProgress() {
	progressFile = localProgressPath()
	playerProgress = PlayerProgress{}
	if err := readJSONWithRecovery(progressFile, &playerProgress); err != nil {
		logRuntimeEvent("save_load", "player progression unavailable or invalid; using defaults")
	}
}

func savePlayerProgress() {
	if progressFile == "" {
		progressFile = localProgressPath()
	}
	data, err := json.MarshalIndent(playerProgress, "", "  ")
	if err == nil {
		_ = atomicWriteFile(progressFile, data, 0644)
	}
}

func localMetaPath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if cfg, err := os.UserConfigDir(); err == nil {
			base = cfg
		}
	}
	if base == "" {
		base = "."
	}
	dir := filepath.Join(base, "KongGame", "CursorControlTrainer")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "game_meta_v40.json")
}

func defaultGameMeta() GameMeta {
	return GameMeta{
		ParticleQuality:         2,
		FPSMode:                 0,
		MusicVolume:             70,
		EffectsVolume:           85,
		FontOverride:            0,
		HUDCornerStyle:          0,
		HUDBackgroundTheme:      0,
		EXPBarAnimation:         0,
		BossHPBarTheme:          0,
		ButtonHoverEffect:       0,
		AnnouncementTheme:       0,
		ScreenShakeStrength:     0,
		CrosshairStyle:          0,
		CrosshairSize:           1,
		CrosshairColor:          0,
		SelectedProfileFont:     0,
		SelectedProfileNameFont: 0,
		ProfilePrimaryColour:    1,
		ProfileSecondaryColour:  2,
		ProfileNameShadow:       true,
		ProfileShadowColour:     0,
		ProfileGradientVertical: false,
		ProfileAnimation:        0,
	}
}

func normalizeGameMeta() {
	if gameMeta.ParticleQuality < 0 || gameMeta.ParticleQuality > 2 {
		gameMeta.ParticleQuality = 2
	}
	if gameMeta.FPSMode < 0 || gameMeta.FPSMode > 2 {
		gameMeta.FPSMode = 0
	}
	if gameMeta.MusicVolume < 0 || gameMeta.MusicVolume > 100 {
		gameMeta.MusicVolume = 70
	}
	if gameMeta.EffectsVolume < 0 || gameMeta.EffectsVolume > 100 {
		gameMeta.EffectsVolume = 85
	}
	if gameMeta.FontOverride < 0 || gameMeta.FontOverride >= len(uiFontFaces) {
		gameMeta.FontOverride = 0
	}
	if gameMeta.HUDCornerStyle < 0 || gameMeta.HUDCornerStyle > 3 {
		gameMeta.HUDCornerStyle = 0
	}
	if gameMeta.HUDBackgroundTheme < 0 || gameMeta.HUDBackgroundTheme > 5 {
		gameMeta.HUDBackgroundTheme = 0
	}
	if gameMeta.EXPBarAnimation < 0 || gameMeta.EXPBarAnimation > 2 {
		gameMeta.EXPBarAnimation = 0
	}
	if gameMeta.BossHPBarTheme < 0 || gameMeta.BossHPBarTheme > 3 {
		gameMeta.BossHPBarTheme = 0
	}
	if gameMeta.ButtonHoverEffect < 0 || gameMeta.ButtonHoverEffect > 3 {
		gameMeta.ButtonHoverEffect = 0
	}
	if gameMeta.AnnouncementTheme < 0 || gameMeta.AnnouncementTheme > 5 {
		gameMeta.AnnouncementTheme = 0
	}
	if gameMeta.ScreenShakeStrength < 0 || gameMeta.ScreenShakeStrength > 3 {
		gameMeta.ScreenShakeStrength = 0
	}
	if gameMeta.CrosshairStyle < 0 || gameMeta.CrosshairStyle > 4 {
		gameMeta.CrosshairStyle = 0
	}
	if gameMeta.CrosshairSize < 0 || gameMeta.CrosshairSize > 2 {
		gameMeta.CrosshairSize = 1
	}
	if gameMeta.CrosshairColor < 0 || gameMeta.CrosshairColor > 6 {
		gameMeta.CrosshairColor = 0
	}
	if gameMeta.SelectedProfileFont < 0 || gameMeta.SelectedProfileFont > 7 {
		gameMeta.SelectedProfileFont = 0
	}
	if gameMeta.SelectedProfileNameFont < 0 || gameMeta.SelectedProfileNameFont > 12 {
		gameMeta.SelectedProfileNameFont = 0
	}
	if gameMeta.ProfilePrimaryColour < 0 || gameMeta.ProfilePrimaryColour >= len(profileStyleColours) {
		gameMeta.ProfilePrimaryColour = 1
	}
	if gameMeta.ProfileSecondaryColour < 0 || gameMeta.ProfileSecondaryColour >= len(profileStyleColours) {
		gameMeta.ProfileSecondaryColour = 2
	}
	if gameMeta.ProfileShadowColour < 0 || gameMeta.ProfileShadowColour >= len(profileStyleColours) {
		gameMeta.ProfileShadowColour = 0
	}
	if gameMeta.ProfileAnimation < 0 || gameMeta.ProfileAnimation > 3 {
		gameMeta.ProfileAnimation = 0
	}
	if gameMeta.SpaceCoins < 0 {
		gameMeta.SpaceCoins = 0
	}
	// One-time AFK currency migration: preserve all pre-Starbits progression while
	// permanently isolating the AFK economy from main-game SpaceCoins.
	if !gameMeta.AFKStarbitsMigrated {
		if gameMeta.AFKStarbits == 0 && gameMeta.AFKLegacyCursorEnergy > 0 {
			gameMeta.AFKStarbits = gameMeta.AFKLegacyCursorEnergy
		}
		if gameMeta.AFKStarbitRemainderMilli == 0 && gameMeta.AFKLegacyEnergyRemainder > 0 {
			gameMeta.AFKStarbitRemainderMilli = gameMeta.AFKLegacyEnergyRemainder
		}
		if gameMeta.AFKOfflinePendingStarbits == 0 && gameMeta.AFKLegacyOfflineEnergy > 0 {
			gameMeta.AFKOfflinePendingStarbits = gameMeta.AFKLegacyOfflineEnergy
		}
		if gameMeta.AFKExpeditionPendingStarbits == 0 && gameMeta.AFKLegacyExpeditionEnergy > 0 {
			gameMeta.AFKExpeditionPendingStarbits = gameMeta.AFKLegacyExpeditionEnergy
		}
		gameMeta.AFKLegacyCursorEnergy = 0
		gameMeta.AFKLegacyEnergyRemainder = 0
		gameMeta.AFKLegacyLifetimeEnergy = 0
		gameMeta.AFKLegacyOfflineEnergy = 0
		gameMeta.AFKLegacyExpeditionEnergy = 0
		gameMeta.AFKStarbitsMigrated = true
	}
	// Deprecated Section-4 AFK SpaceCoin fields are intentionally discarded.
	// The AFK mode never modifies the main SpaceCoin balance after this migration.
	gameMeta.AFKOfflinePendingSpaceCoins = 0
	gameMeta.AFKClaimableSpaceCoins = 0
	gameMeta.AFKSpaceCoinRemainderUnits = 0
	gameMeta.AFKLifetimeAFKSpaceCoins = 0
	if gameMeta.AFKStarbits < 0 {
		gameMeta.AFKStarbits = 0
	}
	// Before the Cursor Core exists, calibration still stops exactly at 100.
	// Once the Core is built, Starbits become the live Section 2 economy
	// and must never be forced back to the calibration amount.
	if !gameMeta.AFKSection1Complete && gameMeta.AFKStarbits >= afkSection1StarbitTarget {
		if !gameMeta.AFKCursorCoreBuilt {
			gameMeta.AFKStarbits = afkSection1StarbitTarget
		}
		gameMeta.AFKSection1Complete = true
	}
	if gameMeta.AFKSection1Complete && !gameMeta.AFKCursorCoreBuilt && gameMeta.AFKStarbits < afkSection1StarbitTarget {
		// Compatibility for Section 1 saves that recorded completion but lost the
		// exact calibration balance before Section 2 existed.
		gameMeta.AFKStarbits = afkSection1StarbitTarget
	}
	if gameMeta.AFKCursorCoreBuilt {
		gameMeta.AFKSection1Complete = true
		if gameMeta.AFKCursorCoreTier < 1 {
			gameMeta.AFKCursorCoreTier = 1
		}
		if gameMeta.AFKCursorCoreTier > afkCoreMaxTier {
			gameMeta.AFKCursorCoreTier = afkCoreMaxTier
		}
		if gameMeta.AFKCursorProcessingLevel < 1 {
			gameMeta.AFKCursorProcessingLevel = 1
		}
		if gameMeta.AFKCursorProcessingLevel > afkProcessingMaxLevel {
			gameMeta.AFKCursorProcessingLevel = afkProcessingMaxLevel
		}
		if gameMeta.AFKAutoCursors < 0 {
			gameMeta.AFKAutoCursors = 0
		}
		if gameMeta.AFKAutoCursors > afkAutoCursorMax {
			gameMeta.AFKAutoCursors = afkAutoCursorMax
		}
	} else {
		gameMeta.AFKCursorCoreTier = 0
		gameMeta.AFKAutoCursors = 0
		gameMeta.AFKCursorProcessingLevel = 0
		gameMeta.AFKStarbitRemainderMilli = 0
		gameMeta.AFKSection2Complete = false
		gameMeta.AFKSection3Complete = false
	}
	if gameMeta.AFKStarbitRemainderMilli < 0 || gameMeta.AFKStarbitRemainderMilli >= 1000 {
		gameMeta.AFKStarbitRemainderMilli = 0
	}
	afkRefreshSection2Completion()
	afkRefreshSection3Completion()
	if gameMeta.AFKLastSavedUnix < 0 {
		gameMeta.AFKLastSavedUnix = 0
	}
	if gameMeta.AFKMaxObservedUnix < 0 {
		gameMeta.AFKMaxObservedUnix = 0
	}
	if gameMeta.AFKOfflinePendingAwaySeconds < 0 {
		gameMeta.AFKOfflinePendingAwaySeconds = 0
	}
	if gameMeta.AFKOfflinePendingPaidSeconds < 0 {
		gameMeta.AFKOfflinePendingPaidSeconds = 0
	}
	if gameMeta.AFKOfflinePendingStarbits < 0 {
		gameMeta.AFKOfflinePendingStarbits = 0
	}
	if gameMeta.AFKOfflineClaims < 0 {
		gameMeta.AFKOfflineClaims = 0
	}
	if gameMeta.AFKClockRollbackCount < 0 {
		gameMeta.AFKClockRollbackCount = 0
	}
	// Section 5 save migration / clamps. Tier 2 merely exposes the expedition
	// link; the Scout Ship is only initialised after the player builds it.
	if gameMeta.AFKScoutShipUnlocked {
		if gameMeta.AFKScoutShipTier < 1 {
			gameMeta.AFKScoutShipTier = 1
		}
		if gameMeta.AFKScoutShipTier > afkScoutMaxTier {
			gameMeta.AFKScoutShipTier = afkScoutMaxTier
		}
		if gameMeta.AFKExpeditionSpeedLevel < 1 {
			gameMeta.AFKExpeditionSpeedLevel = 1
		}
		if gameMeta.AFKExpeditionSpeedLevel > afkScoutStatMaxLevel {
			gameMeta.AFKExpeditionSpeedLevel = afkScoutStatMaxLevel
		}
		if gameMeta.AFKCapacityLevel < 1 {
			gameMeta.AFKCapacityLevel = 1
		}
		if gameMeta.AFKCapacityLevel > afkScoutStatMaxLevel {
			gameMeta.AFKCapacityLevel = afkScoutStatMaxLevel
		}
		if gameMeta.AFKCacheChanceLevel < 1 {
			gameMeta.AFKCacheChanceLevel = 1
		}
		if gameMeta.AFKCacheChanceLevel > afkScoutStatMaxLevel {
			gameMeta.AFKCacheChanceLevel = afkScoutStatMaxLevel
		}
	} else {
		gameMeta.AFKScoutShipTier = 0
		gameMeta.AFKExpeditionSpeedLevel = 0
		gameMeta.AFKCapacityLevel = 0
		gameMeta.AFKCacheChanceLevel = 0
		gameMeta.AFKNavigationData = 0
		gameMeta.AFKExpeditionsCompleted = 0
		gameMeta.AFKExpeditionStartedUnix = 0
		gameMeta.AFKExpeditionEndsUnix = 0
		gameMeta.AFKExpeditionPendingStarbits = 0
		gameMeta.AFKExpeditionPendingData = 0
		gameMeta.AFKExpeditionPendingCache = false
		gameMeta.AFKSpaceCacheRarity = 0
		gameMeta.AFKSection5Complete = false
	}
	if gameMeta.AFKNavigationData < 0 {
		gameMeta.AFKNavigationData = 0
	}
	if gameMeta.AFKExpeditionsCompleted < 0 {
		gameMeta.AFKExpeditionsCompleted = 0
	}
	if gameMeta.AFKExpeditionSerial < 0 {
		gameMeta.AFKExpeditionSerial = 0
	}
	if gameMeta.AFKExpeditionDestination < 0 || gameMeta.AFKExpeditionDestination >= len(afkExpeditions) {
		gameMeta.AFKExpeditionDestination = 0
	}
	if gameMeta.AFKExpeditionStartedUnix < 0 {
		gameMeta.AFKExpeditionStartedUnix = 0
	}
	if gameMeta.AFKExpeditionEndsUnix < 0 {
		gameMeta.AFKExpeditionEndsUnix = 0
	}
	if gameMeta.AFKExpeditionPendingStarbits < 0 {
		gameMeta.AFKExpeditionPendingStarbits = 0
	}
	if gameMeta.AFKExpeditionPendingData < 0 {
		gameMeta.AFKExpeditionPendingData = 0
	}
	if gameMeta.AFKSpaceCacheRarity < 0 || gameMeta.AFKSpaceCacheRarity > 4 {
		gameMeta.AFKSpaceCacheRarity = 0
		gameMeta.AFKExpeditionPendingCache = false
	}
	if gameMeta.AFKSpaceCachesClaimed < 0 {
		gameMeta.AFKSpaceCachesClaimed = 0
	}
	afkRefreshSection5Completion()
	afkEnsureSection6SaveShape()
	afkEnsureSection7SaveShape()
	afkEnsureSection8SaveShape()
	afkRefreshSection8Completion()
	afkEnsureSection9SaveShape()
	afkRefreshSection9Completion()
	afkEnsureSection10SaveShape()
	afkRefreshSection10Completion()
	if gameMeta.SelectedFireColor < 0 || gameMeta.SelectedFireColor >= len(fireColorDefs) {
		gameMeta.SelectedFireColor = 0
	}
	if gameMeta.SelectedFireColor > 0 && !fireColorUnlocked(gameMeta.SelectedFireColor) {
		gameMeta.SelectedFireColor = 0
	}
	if gameMeta.SelectedFireSize < 0 || gameMeta.SelectedFireSize >= len(fireSizeDefs) {
		gameMeta.SelectedFireSize = 0
	}
	if gameMeta.SelectedFireSize > 0 && !fireSizeUnlocked(gameMeta.SelectedFireSize) {
		gameMeta.SelectedFireSize = 0
	}
	if gameMeta.SelectedTitle == "" {
		gameMeta.SelectedTitle = "ROOKIE PILOT"
	}
	if !nameColourUnlocked(gameMeta.SelectedNameColour) {
		gameMeta.SelectedNameColour = 0
	}
	if !profileFrameUnlocked(gameMeta.SelectedProfileFrame) {
		gameMeta.SelectedProfileFrame = 0
	}
	if gameMeta.SelectedShip < 0 || gameMeta.SelectedShip > 12 {
		gameMeta.SelectedShip = 0
	}
	// HYPERNOVA (old ship 11) duplicated ORCHID PRISM. Migrate old ownership/selection.
	if shipUnlocked(11) && !shipUnlocked(8) {
		gameMeta.UnlockedShips = append(gameMeta.UnlockedShips, 8)
	}
	if gameMeta.SelectedShip == 11 {
		gameMeta.SelectedShip = 8
	}
	if gameMeta.SelectedShip > 0 && !shipUnlocked(gameMeta.SelectedShip) {
		gameMeta.SelectedShip = 0
	}
	if gameMeta.AFKExpeditionShipID < 0 || gameMeta.AFKExpeditionShipID >= len(spaceShipDefs) || strings.TrimSpace(spaceShipDefs[gameMeta.AFKExpeditionShipID].Name) == "" {
		gameMeta.AFKExpeditionShipID = 0
	}
	if gameMeta.AFKExpeditionShipID > 0 && !shipUnlocked(gameMeta.AFKExpeditionShipID) {
		gameMeta.AFKExpeditionShipID = 0
	}
	gameMeta.AchievementShowcase = normalizedAchievementShowcase()
	if gameMeta.ShipDropsSinceRedPlus < 0 {
		gameMeta.ShipDropsSinceRedPlus = 0
	}
	if gameMeta.ShipDropsSinceGold < 0 {
		gameMeta.ShipDropsSinceGold = 0
	}
	if gameMeta.SurvivalCheckpoint < 1 {
		gameMeta.SurvivalCheckpoint = 1
	}
}

func currentWeekKey() string {
	y, w := time.Now().Local().ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

func resetChallengePeriodsIfNeeded() {
	today := time.Now().Local().Format("2006-01-02")
	if gameMeta.DailyDate != today {
		gameMeta.DailyDate = today
		gameMeta.DailyClears = 0
		gameMeta.DailyHardClears = 0
		gameMeta.DailyHighAcc = 0
		gameMeta.DailyClearsRewarded = false
		gameMeta.DailyHardRewarded = false
		gameMeta.DailyAccRewarded = false
		gameMeta.DailyRewarded = false
	}
	wk := currentWeekKey()
	if gameMeta.WeeklyKey != wk {
		gameMeta.WeeklyKey = wk
		gameMeta.WeeklyClears = 0
		gameMeta.WeeklyInsane = 0
		gameMeta.WeeklyBestStreak = 0
		gameMeta.WeeklyRewarded = false
	}
}

func loadGameMeta() {
	metaFile = localMetaPath()
	gameMeta = defaultGameMeta()
	if err := readJSONWithRecovery(metaFile, &gameMeta); err != nil {
		logRuntimeEvent("save_load", "game metadata unavailable or invalid; using defaults")
	}
	// v302: permanent Garage purchases have a second tiny entitlement journal so a
	// damaged/reset main metadata file cannot make the player buy fire cosmetics again.
	loadCosmeticEntitlements()
	normalizeGameMeta()
	// v274: Precision courses are always randomized. The old Adaptive/training-course
	// selectors are retired from player-facing UI. Keep the legacy fields/code for
	// save compatibility and developer internals, but never let them affect play.
	adaptiveMode = false
	pathMode = 0
	resetChallengePeriodsIfNeeded()
	// Section 4: calculate the elapsed AFK window before the first launch save
	// refreshes the persisted timestamp. Existing saves migrate without a reward.
	afkPrepareOfflineCatchup(time.Now())
	gameMeta.Sessions++
	sessionStarted = time.Now()
	saveGameMeta()
	if gameMeta.FailureSound < 0 || gameMeta.FailureSound > 4 {
		gameMeta.FailureSound = 0
	}
}

func applyV59RankResetMigration() {
	if gameMeta.RankResetV59Applied {
		return
	}

	// Preserve completion totals, leaderboard PBs, achievements, lifetime stats,
	// playtime and challenge history. Only the obsolete EXP balance is reset.
	playerProgress.EXP = 0
	savePlayerProgress()

	gameMeta.RankResetV59Applied = true
	saveGameMeta()
}

func saveGameMeta() {
	if metaFile == "" {
		metaFile = localMetaPath()
	}
	resetChallengePeriodsIfNeeded()
	afkStampSaveTime(time.Now())
	data, err := json.MarshalIndent(gameMeta, "", "  ")
	if err == nil {
		_ = atomicWriteFile(metaFile, data, 0644)
	}
	saveCosmeticEntitlements()
}

func saveSessionPlaytime() {
	if !sessionStarted.IsZero() {
		gameMeta.PlaySeconds += int64(time.Since(sessionStarted).Seconds())
		sessionStarted = time.Now()
		saveGameMeta()
	}
}

func totalPlaySecondsNow() int64 {
	v := gameMeta.PlaySeconds
	if !sessionStarted.IsZero() {
		v += int64(time.Since(sessionStarted).Seconds())
	}
	return v
}

func formatPlaytime(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func achievementDefinitions() []AchievementDef {
	return []AchievementDef{
		// EASY — approachable early milestones.
		{"FIRST_BLOOD", "FIRST BLOOD", "Complete your first course", 0},
		{"WARMING_UP", "WARMING UP", "Complete 5 courses", 0},
		{"DOUBLE_DIGITS", "DOUBLE DIGITS", "Complete 10 courses", 0},
		{"GETTING_SERIOUS", "GETTING SERIOUS", "Complete 25 courses", 0},
		{"TRIGGER_FINGER", "TRIGGER FINGER", "Hit 10 targets", 0},
		{"TARGET_PRACTICE", "TARGET PRACTICE", "Hit 25 targets", 0},
		{"LOCKED_IN", "LOCKED IN", "Hit 50 targets", 0},
		{"CLEAN_RUN", "CLEAN RUN", "Reach 80% best accuracy", 0},
		{"STEADIER", "STEADIER", "Reach 85% best accuracy", 0},
		{"STEADY_HAND", "STEADY HAND", "Reach 90% best accuracy", 0},
		{"THREE_CHAIN", "THREE CHAIN", "Reach a streak of 3", 0},
		{"FIVE_CHAIN", "FIVE CHAIN", "Reach a streak of 5", 0},
		{"EASY_GRINDER", "EASY GRINDER", "Complete 10 Easy courses", 0},
		{"NORMAL_START", "NORMAL START", "Complete 5 Normal courses", 0},
		{"HARD_CONTACT", "HARD CONTACT", "Complete your first Hard course", 0},
		{"RETURNING_PLAYER", "RETURNING PLAYER", "Play 2 sessions", 0},
		{"TEN_MINUTES", "TEN MINUTES", "Accumulate 10 minutes of playtime", 0},
		{"SILVER_BOUND", "SILVER BOUND", "Reach Silver I (750 EXP)", 0},
		{"RUNNER", "RUNNER", "Start 25 runs", 0},
		{"DAILY_OPERATOR", "DAILY OPERATOR", "Complete a Daily Ops set", 0},

		// ENDURANCE EASY — first contact with the survival systems.
		{"END_FIRST_FLIGHT", "FIRST FLIGHT", "Start your first Endurance run", 0},
		{"END_250", "ORBITAL", "Reach 250m in Endurance", 0},
		{"END_500", "HALF KILOMETRE", "Reach 500m in Endurance", 0},
		{"END_WARP_1", "WARP INITIATE", "Complete your first Warp", 0},
		{"END_UFO_1", "CONTACT SURVIVED", "Survive your first UFO encounter", 0},
		{"END_POWER_5", "POWER USER", "Collect 5 Endurance power-ups", 0},
		{"END_SHIELD_1", "SHIELDS UP", "Activate Shield for the first time", 0},
		{"END_TIME_1", "TIME BENDER", "Activate Time Slow for the first time", 0},
		{"CACHE_1", "OPEN THE CACHE", "Open your first Space Cache", 0},
		{"FLEET_3", "SMALL FLEET", "Own 3 unlockable spacecraft", 0},

		// SURVIVAL EASY — first defensive milestones.
		{"SURV_FIRST_WAVE", "FIRST DEFENCE", "Clear your first Survival wave", 0},
		{"SURV_PERFECT_WAVE", "FLAWLESS DEFENCE", "Clear a Survival wave without station damage or shield interception", 0},
		{"SURV_25_KILLS", "THREAT CONTROL", "Destroy 25 enemies in one Survival run", 0},
		{"SURV_SENTINEL_REACHED", "BOSS SIGNAL", "Reach The Sentinel after Wave 10", 0},

		// MEDIUM — consistency and meaningful progression.
		{"FIFTY_CLEARS", "FIFTY CLEARS", "Complete 50 courses", 1},
		{"CENTURY", "CENTURY", "Complete 100 courses", 1},
		{"TWO_HUNDRED", "TWO HUNDRED", "Complete 200 courses", 1},
		{"TARGET_HUNTER", "TARGET HUNTER", "Hit 100 targets", 1},
		{"TARGET_SPECIALIST", "TARGET SPECIALIST", "Hit 250 targets", 1},
		{"TARGET_MACHINE", "TARGET MACHINE", "Hit 500 targets", 1},
		{"PRECISION_92", "PRECISION 92", "Reach 92% best accuracy", 1},
		{"DEADEYE", "DEADEYE", "Reach 95% best accuracy", 1},
		{"SURGICAL", "SURGICAL", "Reach 97% best accuracy", 1},
		{"UNTOUCHABLE", "UNTOUCHABLE", "Reach a streak of 10", 1},
		{"RELENTLESS", "RELENTLESS", "Reach a streak of 15", 1},
		{"NORMAL_REGULAR", "NORMAL REGULAR", "Complete 25 Normal courses", 1},
		{"HARDENED", "HARDENED", "Complete 10 Hard courses", 1},
		{"INSANITY", "INSANITY", "Complete your first Insane course", 1},
		{"ONE_HOUR", "ONE HOUR", "Accumulate 1 hour of playtime", 1},
		{"PLATINUM_BOUND", "PLATINUM BOUND", "Reach Platinum I (12,000 EXP)", 1},
		{"WEEKLY_OPERATOR", "WEEKLY OPERATOR", "Complete a Weekly Ops set", 1},

		// ENDURANCE MEDIUM — established survival progression.
		{"END_1000", "ONE KILOMETRE", "Reach 1,000m in Endurance", 1},
		{"END_1500", "ALIEN TERRITORY", "Reach 1,500m in Endurance", 1},
		{"END_2500", "DEEP SPACE", "Reach 2,500m in Endurance", 1},
		{"END_WARP_5", "WARP VETERAN", "Complete 5 Warps", 1},
		{"END_UFO_3", "ALIEN HUNTER", "Survive 3 UFO encounters", 1},
		{"END_TARGETS_25", "ENDURANCE GUNNER", "Hit 25 targets in one Endurance run", 1},
		{"END_POWER_25", "CHARGED UP", "Collect 25 Endurance power-ups", 1},
		{"END_SHIELD_10", "BARRIER SPECIALIST", "Activate Shield 10 times", 1},
		{"END_TIME_10", "CHRONO SPECIALIST", "Activate Time Slow 10 times", 1},
		{"CACHE_10", "CACHE RUNNER", "Open 10 Space Caches", 1},

		// SURVIVAL MEDIUM — sector mastery.
		{"SURV_50_KILLS", "STATION GUARDIAN", "Destroy 50 enemies in one Survival run", 1},
		{"SURV_SENTINEL_DOWN", "SENTINEL BREAKER", "Defeat The Sentinel", 1},
		{"SURV_CORE_SWEEP", "CORE INTERCEPTOR", "Destroy every returning core orb in The Sentinel finale", 1},
		{"SURV_SECTOR2", "SECTOR SECURED", "Earn the Wave 11 Survival checkpoint", 1},
		{"SURV_SECTOR3", "VOID SERPENT DOWN", "Defeat The Void Serpent and unlock Sector 3", 1},

		// HARD — long-term mastery.
		{"THREE_HUNDRED", "THREE HUNDRED", "Complete 300 courses", 2},
		{"VETERAN", "VETERAN", "Complete 500 courses", 2},
		{"THOUSAND_CLEARS", "THOUSAND CLEARS", "Complete 1,000 courses", 2},
		{"TARGET_ELITE", "TARGET ELITE", "Hit 1,000 targets", 2},
		{"TARGET_LEGEND", "TARGET LEGEND", "Hit 2,500 targets", 2},
		{"LASER_FOCUS", "LASER FOCUS", "Reach 98% best accuracy", 2},
		{"NEAR_PERFECT", "NEAR PERFECT", "Reach 99% best accuracy", 2},
		{"PERFECTION", "PERFECTION", "Reach 100% combined accuracy", 2},
		{"IRON_STREAK", "IRON STREAK", "Reach a streak of 20", 2},
		{"MACHINE", "MACHINE", "Reach a streak of 25", 2},
		{"UNBREAKABLE", "UNBREAKABLE", "Reach a streak of 50", 2},
		{"INSANE_VETERAN", "INSANE VETERAN", "Complete 25 Insane courses", 2},
		{"MASTER", "MASTER", "Reach Master I", 2},

		// ENDURANCE HARD — long-run survival mastery.
		{"END_5000", "FIVE KILOMETRES", "Reach 5,000m in Endurance", 2},
		{"END_7500", "OUTER REACH", "Reach 7,500m in Endurance", 2},
		{"END_10000", "TEN KILOMETRES", "Reach 10,000m in Endurance", 2},
		{"END_WARP_20", "WARP MASTER", "Complete 20 Warps", 2},
		{"END_UFO_10", "XENO VETERAN", "Survive 10 UFO encounters", 2},
		{"END_TARGETS_100", "STARFIGHTER", "Hit 100 targets in one Endurance run", 2},
		{"END_POWER_100", "OVERCHARGED", "Collect 100 Endurance power-ups", 2},
		{"END_SHIELD_50", "FORTRESS", "Activate Shield 50 times", 2},
		{"END_TIME_50", "TIME LORD", "Activate Time Slow 50 times", 2},
		{"FLEET_10", "STAR FLEET", "Own 10 unlockable spacecraft", 2},

		// SURVIVAL HARD — clean boss execution.
		{"SURV_SENTINEL_FLAWLESS", "UNTOUCHABLE SECTOR", "Defeat The Sentinel after taking no station damage in Sector 1", 2},
		{"SURV_ELITE_10", "ELITE HUNTER", "Destroy 10 elite Survival enemies", 1},
		{"SURV_ARRAY_REACHED", "TERMINUS SIGNAL", "Reach The Terminus after Wave 30", 1},
		{"SURV_ARRAY_DOWN", "TERMINUS BREAKER", "Defeat The Terminus", 2},
		{"SURV_ARRAY_FLAWLESS", "TERMINUS PERFECT", "Defeat The Terminus without taking station damage during the encounter", 2},
		{"TRIPLE_CROWN", "TRIPLE CROWN", "Master all three modes: 99% Precision, 5,000m Endurance, and defeat The Terminus", 2},
	}
}

// achievementProgress returns current/target for measurable locked achievements.
// The UI only draws a bar once current > 0 and removes it when completed.
func achievementProgress(id string) (current, target float64, ok bool) {
	ships := float64(len(normalizeShipList(gameMeta.UnlockedShips)))
	switch id {
	case "FIRST_BLOOD":
		return float64(gameMeta.TotalClears), 1, true
	case "WARMING_UP":
		return float64(gameMeta.TotalClears), 5, true
	case "DOUBLE_DIGITS":
		return float64(gameMeta.TotalClears), 10, true
	case "GETTING_SERIOUS":
		return float64(gameMeta.TotalClears), 25, true
	case "FIFTY_CLEARS":
		return float64(gameMeta.TotalClears), 50, true
	case "CENTURY":
		return float64(gameMeta.TotalClears), 100, true
	case "TWO_HUNDRED":
		return float64(gameMeta.TotalClears), 200, true
	case "THREE_HUNDRED":
		return float64(gameMeta.TotalClears), 300, true
	case "VETERAN":
		return float64(gameMeta.TotalClears), 500, true
	case "THOUSAND_CLEARS":
		return float64(gameMeta.TotalClears), 1000, true
	case "TRIGGER_FINGER":
		return float64(gameMeta.TargetsHit), 10, true
	case "TARGET_PRACTICE":
		return float64(gameMeta.TargetsHit), 25, true
	case "LOCKED_IN":
		return float64(gameMeta.TargetsHit), 50, true
	case "TARGET_HUNTER":
		return float64(gameMeta.TargetsHit), 100, true
	case "TARGET_SPECIALIST":
		return float64(gameMeta.TargetsHit), 250, true
	case "TARGET_MACHINE":
		return float64(gameMeta.TargetsHit), 500, true
	case "TARGET_ELITE":
		return float64(gameMeta.TargetsHit), 1000, true
	case "TARGET_LEGEND":
		return float64(gameMeta.TargetsHit), 2500, true
	case "CLEAN_RUN":
		return gameMeta.BestAccuracy, 80, true
	case "STEADIER":
		return gameMeta.BestAccuracy, 85, true
	case "STEADY_HAND":
		return gameMeta.BestAccuracy, 90, true
	case "PRECISION_92":
		return gameMeta.BestAccuracy, 92, true
	case "DEADEYE":
		return gameMeta.BestAccuracy, 95, true
	case "SURGICAL":
		return gameMeta.BestAccuracy, 97, true
	case "LASER_FOCUS":
		return gameMeta.BestAccuracy, 98, true
	case "NEAR_PERFECT":
		return gameMeta.BestAccuracy, 99, true
	case "PERFECTION":
		return gameMeta.BestAccuracy, 100, true
	case "THREE_CHAIN":
		return float64(gameMeta.BestStreakEver), 3, true
	case "FIVE_CHAIN":
		return float64(gameMeta.BestStreakEver), 5, true
	case "UNTOUCHABLE":
		return float64(gameMeta.BestStreakEver), 10, true
	case "RELENTLESS":
		return float64(gameMeta.BestStreakEver), 15, true
	case "IRON_STREAK":
		return float64(gameMeta.BestStreakEver), 20, true
	case "MACHINE":
		return float64(gameMeta.BestStreakEver), 25, true
	case "UNBREAKABLE":
		return float64(gameMeta.BestStreakEver), 50, true
	case "EASY_GRINDER":
		return float64(playerProgress.EasyCompleted), 10, true
	case "NORMAL_START":
		return float64(playerProgress.NormalCompleted), 5, true
	case "NORMAL_REGULAR":
		return float64(playerProgress.NormalCompleted), 25, true
	case "HARD_CONTACT":
		return float64(playerProgress.HardCompleted), 1, true
	case "HARDENED":
		return float64(playerProgress.HardCompleted), 10, true
	case "INSANITY":
		return float64(playerProgress.InsaneCompleted), 1, true
	case "INSANE_VETERAN":
		return float64(playerProgress.InsaneCompleted), 25, true
	case "RETURNING_PLAYER":
		return float64(gameMeta.Sessions), 2, true
	case "TEN_MINUTES":
		return float64(totalPlaySecondsNow()), 600, true
	case "ONE_HOUR":
		return float64(totalPlaySecondsNow()), 3600, true
	case "SILVER_BOUND":
		return float64(playerProgress.EXP), 750, true
	case "PLATINUM_BOUND":
		return float64(playerProgress.EXP), 12000, true
	case "MASTER":
		return float64(playerProgress.EXP), 75000, true
	case "RUNNER":
		return float64(gameMeta.TotalRuns), 25, true
	case "END_FIRST_FLIGHT":
		return float64(gameMeta.EnduranceRuns), 1, true
	case "END_250":
		return gameMeta.BestEnduranceDistance, 250, true
	case "END_500":
		return gameMeta.BestEnduranceDistance, 500, true
	case "END_1000":
		return gameMeta.BestEnduranceDistance, 1000, true
	case "END_1500":
		return gameMeta.BestEnduranceDistance, 1500, true
	case "END_2500":
		return gameMeta.BestEnduranceDistance, 2500, true
	case "END_5000":
		return gameMeta.BestEnduranceDistance, 5000, true
	case "END_7500":
		return gameMeta.BestEnduranceDistance, 7500, true
	case "END_10000":
		return gameMeta.BestEnduranceDistance, 10000, true
	case "END_WARP_1":
		return float64(gameMeta.EnduranceWarpsCompleted), 1, true
	case "END_WARP_5":
		return float64(gameMeta.EnduranceWarpsCompleted), 5, true
	case "END_WARP_20":
		return float64(gameMeta.EnduranceWarpsCompleted), 20, true
	case "END_UFO_1":
		return float64(gameMeta.AlienBossesSurvived), 1, true
	case "END_UFO_3":
		return float64(gameMeta.AlienBossesSurvived), 3, true
	case "END_UFO_10":
		return float64(gameMeta.AlienBossesSurvived), 10, true
	case "END_TARGETS_25":
		return float64(gameMeta.EnduranceBestTargets), 25, true
	case "END_TARGETS_100":
		return float64(gameMeta.EnduranceBestTargets), 100, true
	case "END_POWER_5":
		return float64(gameMeta.EndurancePowerups), 5, true
	case "END_POWER_25":
		return float64(gameMeta.EndurancePowerups), 25, true
	case "END_POWER_100":
		return float64(gameMeta.EndurancePowerups), 100, true
	case "END_SHIELD_1":
		return float64(gameMeta.EnduranceShieldUses), 1, true
	case "END_SHIELD_10":
		return float64(gameMeta.EnduranceShieldUses), 10, true
	case "END_SHIELD_50":
		return float64(gameMeta.EnduranceShieldUses), 50, true
	case "END_TIME_1":
		return float64(gameMeta.EnduranceTimeUses), 1, true
	case "END_TIME_10":
		return float64(gameMeta.EnduranceTimeUses), 10, true
	case "END_TIME_50":
		return float64(gameMeta.EnduranceTimeUses), 50, true
	case "CACHE_1":
		return float64(gameMeta.SpaceCachesOpened), 1, true
	case "CACHE_10":
		return float64(gameMeta.SpaceCachesOpened), 10, true
	case "FLEET_3":
		return ships, 3, true
	case "FLEET_10":
		return ships, 10, true
	case "SURV_FIRST_WAVE":
		return float64(gameMeta.BestSurvivalWave), 1, true
	case "SURV_25_KILLS":
		return float64(func() int {
			if gameMeta.BestSurvivalKills > survivalKills {
				return gameMeta.BestSurvivalKills
			}
			return survivalKills
		}()), 25, true
	case "SURV_50_KILLS":
		return float64(func() int {
			if gameMeta.BestSurvivalKills > survivalKills {
				return gameMeta.BestSurvivalKills
			}
			return survivalKills
		}()), 50, true
	case "SURV_SENTINEL_REACHED":
		return float64(minInt(func() int {
			if gameMeta.BestSurvivalWave > survivalWave {
				return gameMeta.BestSurvivalWave
			}
			return survivalWave
		}(), 10)), 10, true
	case "SURV_ELITE_10":
		return float64(survivalEliteKills), 10, true
	case "SURV_ARRAY_REACHED":
		return float64(minInt(func() int {
			if gameMeta.BestSurvivalWave > survivalWave {
				return gameMeta.BestSurvivalWave
			}
			return survivalWave
		}(), 30)), 30, true
	}
	return 0, 0, false
}

func achievementTierName(tier int) string {
	switch tier {
	case 1:
		return "MEDIUM"
	case 2:
		return "HARD"
	default:
		return "EASY"
	}
}

func achievementByID(id string) (AchievementDef, bool) {
	for _, a := range achievementDefinitions() {
		if a.ID == id {
			return a, true
		}
	}
	return AchievementDef{}, false
}

func achievementUnlocked(id string) bool {
	for _, a := range gameMeta.Achievements {
		if a == id {
			return true
		}
	}
	return false
}

func achievementTitle(id string) string {
	if a, ok := achievementByID(id); ok {
		return a.Title
	}
	return id
}

func achievementDescription(id string) string {
	if a, ok := achievementByID(id); ok {
		return a.Description
	}
	return ""
}

func achievementTitleReward(id string) string {
	switch id {
	case "STEADY_HAND":
		return "STEADY HAND"
	case "DEADEYE":
		return "DEADEYE"
	case "SURGICAL":
		return "PERFECTIONIST"
	case "END_WARP_5":
		return "WARP PILOT"
	case "END_2500":
		return "PATHFINDER"
	case "END_UFO_3":
		return "ALIEN HUNTER"
	case "SURV_PERFECT_WAVE":
		return "SURVIVOR"
	case "SURV_SENTINEL_REACHED":
		return "BOSS HUNTER"
	case "SURV_SENTINEL_DOWN":
		return "SENTINEL BREAKER"
	case "SURV_SECTOR3":
		return "VOID SLAYER"
	case "SURV_ARRAY_DOWN":
		return "ARRAY BREAKER"
	case "SURV_ARRAY_FLAWLESS":
		return "QUANTUM MASTER"
	case "PERFECTION":
		return "PERFECTIONIST"
	}
	return ""
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
func addUnlockedTitle(v string) {
	v = strings.TrimSpace(v)
	if v != "" && !containsString(gameMeta.UnlockedTitles, v) {
		gameMeta.UnlockedTitles = append(gameMeta.UnlockedTitles, v)
	}
}
func addUnlockedNameColour(v int) {
	if v > 0 && v < 6 && !containsInt(gameMeta.UnlockedNameColours, v) {
		gameMeta.UnlockedNameColours = append(gameMeta.UnlockedNameColours, v)
	}
}
func addUnlockedProfileFrame(v int) {
	if v >= profileSkinFirstID && v <= profileSkinLastID && !containsInt(gameMeta.UnlockedProfileFrames, v) {
		gameMeta.UnlockedProfileFrames = append(gameMeta.UnlockedProfileFrames, v)
	}
}
func unlockProfileCosmeticsFromProgress() {
	addUnlockedTitle("ROOKIE PILOT")
	for _, id := range gameMeta.Achievements {
		if t := achievementTitleReward(id); t != "" {
			addUnlockedTitle(t)
		}
	}
	// Name colours are permanent rewards once their milestone has been achieved.
	colourReq := []string{"", "SILVER_BOUND", "PLATINUM_BOUND", "SURV_SENTINEL_REACHED", "END_2500", "SURGICAL"}
	for id := 1; id < len(colourReq); id++ {
		if achievementUnlocked(colourReq[id]) {
			addUnlockedNameColour(id)
		}
	}
	// v335: legacy outline/rank frames are retired. New profile skins use
	// dedicated 101..111 IDs and gameplay-specific unlock requirements.
	refreshProfileSkinUnlocks()
}
func unlockedProfileTitles() []string {
	unlockProfileCosmeticsFromProgress()
	out := []string{"ROOKIE PILOT"}
	seen := map[string]bool{"ROOKIE PILOT": true}
	for _, t := range gameMeta.UnlockedTitles {
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
func selectedProfileTitle() string {
	for _, t := range unlockedProfileTitles() {
		if t == gameMeta.SelectedTitle {
			return t
		}
	}
	return "ROOKIE PILOT"
}
func profileNameColour(id int) uintptr {
	switch id {
	case 1:
		return rgb(80, 231, 255)
	case 2:
		return rgb(203, 100, 255)
	case 3:
		return rgb(255, 92, 98)
	case 4:
		return rgb(255, 220, 70)
	case 5:
		return rgb(255, 110, 205)
	}
	return rgb(255, 255, 255)
}
func nameColourUnlocked(id int) bool {
	if id == 0 {
		return true
	}
	unlockProfileCosmeticsFromProgress()
	return id > 0 && id < 6 && containsInt(gameMeta.UnlockedNameColours, id)
}
func profileFrameUnlocked(id int) bool { return profileSkinUnlocked(id) }
func profileFrameName(id int) string {
	if id == 0 {
		return "DEFAULT"
	}
	if d, ok := profileSkinDefByID(id); ok {
		return d.Name
	}
	return "DEFAULT"
}
func drawPixelCornerFrame(hdc uintptr, r RECT, c, c2 uintptr, thickness int32, voidStyle bool) {
	drawOutlineRect(hdc, r, c, thickness)
	in := RECT{r.Left + 6, r.Top + 6, r.Right - 6, r.Bottom - 6}
	drawOutlineRect(hdc, in, c2, 1)
	L := int32(34)
	if voidStyle {
		L = 46
	}
	// Chunky transparent-centre corner ornaments: only edges/corners are painted.
	for _, q := range []RECT{
		{r.Left, r.Top, r.Left + L, r.Top + 8}, {r.Left, r.Top, r.Left + 8, r.Top + L},
		{r.Right - L, r.Top, r.Right, r.Top + 8}, {r.Right - 8, r.Top, r.Right, r.Top + L},
		{r.Left, r.Bottom - 8, r.Left + L, r.Bottom}, {r.Left, r.Bottom - L, r.Left + 8, r.Bottom},
		{r.Right - L, r.Bottom - 8, r.Right, r.Bottom}, {r.Right - 8, r.Bottom - L, r.Right, r.Bottom},
	} {
		fillSolidRect(hdc, q, c)
	}
	if voidStyle {
		for i := int32(0); i < 4; i++ {
			o := int32(12) + i*18
			fillSolidRect(hdc, RECT{r.Left + o, r.Top + 2, r.Left + o + 8, r.Top + 5}, c2)
			fillSolidRect(hdc, RECT{r.Right - o - 8, r.Bottom - 5, r.Right - o, r.Bottom - 2}, c2)
		}
	}
}
func drawEquippedProfileFrame(hdc uintptr, r RECT, id int) {
	// Retired outline-frame renderer is kept only for legacy local-profile fallback.
	if id >= profileSkinFirstID {
		return
	}
	if id < 0 || id > 7 {
		id = 0
	}
	cols := []struct {
		a, b  uintptr
		thick int32
		void  bool
	}{
		{rgb(47, 211, 255), rgb(8, 88, 125), 3, false},
		{rgb(196, 122, 72), rgb(91, 55, 35), 4, false},
		{rgb(225, 235, 245), rgb(105, 125, 150), 4, false},
		{rgb(255, 205, 55), rgb(145, 82, 20), 5, false},
		{rgb(90, 220, 255), rgb(88, 145, 255), 5, false},
		{rgb(210, 85, 255), rgb(93, 35, 145), 5, false},
		{rgb(181, 70, 255), rgb(50, 225, 255), 5, true},
		{rgb(255, 105, 210), rgb(255, 220, 70), 6, true},
	}
	v := cols[id]
	drawPixelCornerFrame(hdc, r, v.a, v.b, v.thick, v.void)
}
func cycleProfileTitle() {
	list := unlockedProfileTitles()
	cur := selectedProfileTitle()
	idx := 0
	for i, t := range list {
		if t == cur {
			idx = i
			break
		}
	}
	gameMeta.SelectedTitle = list[(idx+1)%len(list)]
	markProfileTitleSelectionDirty()
	analyticsEvent("profile_customized", map[string]any{"type": "title"})
	saveGameMeta()
	requestPlayerProfileSync()
}
func cycleProfileNameColour() {
	for n := 1; n <= 6; n++ {
		id := (gameMeta.SelectedNameColour + n) % 6
		if nameColourUnlocked(id) {
			gameMeta.SelectedNameColour = id
			break
		}
	}
	markProfileNameColourSelectionDirty()
	analyticsEvent("profile_customized", map[string]any{"type": "name_colour"})
	saveGameMeta()
	requestPlayerProfileSync()
}
func cycleProfileFrame() {
	refreshProfileSkinUnlocks()
	ids := []int{0}
	for _, d := range profileSkinDefs {
		if profileSkinUnlocked(d.ID) {
			ids = append(ids, d.ID)
		}
	}
	idx := 0
	for i, id := range ids {
		if id == gameMeta.SelectedProfileFrame {
			idx = i
			break
		}
	}
	gameMeta.SelectedProfileFrame = ids[(idx+1)%len(ids)]
	markProfileFrameSelectionDirty()
	analyticsEvent("profile_customized", map[string]any{"type": "profile_skin"})
	saveGameMeta()
	requestPlayerProfileSync()
}

func achievementRewardEXP(tier int) int {
	if tier == 1 {
		return 150
	}
	if tier == 2 {
		return 600
	}
	return 30
}
func achievementEXPAlreadyGranted(id string) bool {
	for _, v := range gameMeta.AchievementEXPGranted {
		if v == id {
			return true
		}
	}
	return false
}
func applyAchievementEXPTripleV105Migration() {
	if gameMeta.AchievementEXPTripleV105Applied {
		return
	}
	extra := 0
	for _, id := range gameMeta.AchievementEXPGranted {
		if a, ok := achievementByID(id); ok {
			base := 10
			if a.Tier == 1 {
				base = 50
			}
			if a.Tier == 2 {
				base = 200
			}
			extra += base * 2
		}
	}
	if extra > 0 {
		playerProgress.EXP += extra
		savePlayerProgress()
	}
	gameMeta.AchievementEXPTripleV105Applied = true
	saveGameMeta()
}

func grantAchievementEXP(id string, showToast bool) int {
	if achievementEXPAlreadyGranted(id) {
		return 0
	}
	a, ok := achievementByID(id)
	if !ok {
		return 0
	}
	reward := achievementRewardEXP(a.Tier)
	if achievementTitleReward(id) != "" {
		reward = 0
	}
	gameMeta.AchievementEXPGranted = append(gameMeta.AchievementEXPGranted, id)
	playerProgress.EXP += reward
	savePlayerProgress()
	saveGameMeta()
	if showToast {
		lastAchievementRewardEXP = reward
	}
	return reward
}
func grantMissingAchievementEXPRewards() {
	total := 0
	for _, id := range gameMeta.Achievements {
		if !achievementEXPAlreadyGranted(id) {
			if a, ok := achievementByID(id); ok {
				reward := achievementRewardEXP(a.Tier)
				if achievementTitleReward(id) != "" {
					reward = 0
				}
				gameMeta.AchievementEXPGranted = append(gameMeta.AchievementEXPGranted, id)
				playerProgress.EXP += reward
				total += reward
			}
		}
	}
	if total > 0 {
		savePlayerProgress()
		saveGameMeta()
	}
}

func unlockAchievement(id string) {
	if achievementUnlocked(id) {
		return
	}
	gameMeta.Achievements = append(gameMeta.Achievements, id)
	if t := achievementTitleReward(id); t != "" {
		addUnlockedTitle(t)
	}
	unlockProfileCosmeticsFromProgress()
	applyAchievementCosmeticReward(id)
	lastAchievement = achievementTitle(id)
	lastAchievementRewardEXP = grantAchievementEXP(id, true)
	achievementAt = time.Now()
	analyticsEvent("achievement_unlocked", map[string]any{"achievement": id, "prestige": achievementPrestigeLevel(id, 0)})
	saveGameMeta()
	requestPlayerProfileSync()
}

func evaluateAchievements(elapsed, tracking, target, combined float64, diffName string, currentStreak int) {
	_ = elapsed
	_ = tracking
	_ = target
	_ = diffName
	_ = currentStreak

	checks := map[string]bool{
		"FIRST_BLOOD":      gameMeta.TotalClears >= 1,
		"WARMING_UP":       gameMeta.TotalClears >= 5,
		"DOUBLE_DIGITS":    gameMeta.TotalClears >= 10,
		"GETTING_SERIOUS":  gameMeta.TotalClears >= 25,
		"TRIGGER_FINGER":   gameMeta.TargetsHit >= 10,
		"TARGET_PRACTICE":  gameMeta.TargetsHit >= 25,
		"LOCKED_IN":        gameMeta.TargetsHit >= 50,
		"CLEAN_RUN":        gameMeta.BestAccuracy >= 80,
		"STEADIER":         gameMeta.BestAccuracy >= 85,
		"STEADY_HAND":      gameMeta.BestAccuracy >= 90,
		"THREE_CHAIN":      gameMeta.BestStreakEver >= 3,
		"FIVE_CHAIN":       gameMeta.BestStreakEver >= 5,
		"EASY_GRINDER":     playerProgress.EasyCompleted >= 10,
		"NORMAL_START":     playerProgress.NormalCompleted >= 5,
		"HARD_CONTACT":     playerProgress.HardCompleted >= 1,
		"RETURNING_PLAYER": gameMeta.Sessions >= 2,
		"TEN_MINUTES":      totalPlaySecondsNow() >= 600,
		"SILVER_BOUND":     playerProgress.EXP >= 750,
		"RUNNER":           gameMeta.TotalRuns >= 25,
		"DAILY_OPERATOR":   gameMeta.DailyRewarded,

		"FIFTY_CLEARS":      gameMeta.TotalClears >= 50,
		"CENTURY":           gameMeta.TotalClears >= 100,
		"TWO_HUNDRED":       gameMeta.TotalClears >= 200,
		"TARGET_HUNTER":     gameMeta.TargetsHit >= 100,
		"TARGET_SPECIALIST": gameMeta.TargetsHit >= 250,
		"TARGET_MACHINE":    gameMeta.TargetsHit >= 500,
		"PRECISION_92":      gameMeta.BestAccuracy >= 92,
		"DEADEYE":           gameMeta.BestAccuracy >= 95,
		"SURGICAL":          gameMeta.BestAccuracy >= 97,
		"UNTOUCHABLE":       gameMeta.BestStreakEver >= 10,
		"RELENTLESS":        gameMeta.BestStreakEver >= 15,
		"NORMAL_REGULAR":    playerProgress.NormalCompleted >= 25,
		"HARDENED":          playerProgress.HardCompleted >= 10,
		"INSANITY":          playerProgress.InsaneCompleted >= 1,
		"ONE_HOUR":          totalPlaySecondsNow() >= 3600,
		"PLATINUM_BOUND":    playerProgress.EXP >= 12000,
		"WEEKLY_OPERATOR":   gameMeta.WeeklyRewarded,

		"THREE_HUNDRED":   gameMeta.TotalClears >= 300,
		"VETERAN":         gameMeta.TotalClears >= 500,
		"THOUSAND_CLEARS": gameMeta.TotalClears >= 1000,
		"TARGET_ELITE":    gameMeta.TargetsHit >= 1000,
		"TARGET_LEGEND":   gameMeta.TargetsHit >= 2500,
		"LASER_FOCUS":     gameMeta.BestAccuracy >= 98,
		"NEAR_PERFECT":    gameMeta.BestAccuracy >= 99,
		"PERFECTION":      gameMeta.BestAccuracy >= 99.95,
		"IRON_STREAK":     gameMeta.BestStreakEver >= 20,
		"MACHINE":         gameMeta.BestStreakEver >= 25,
		"UNBREAKABLE":     gameMeta.BestStreakEver >= 50,
		"INSANE_VETERAN":  playerProgress.InsaneCompleted >= 25,
		"MASTER":          playerProgress.EXP >= 75000,
		"TRIPLE_CROWN":    gameMeta.BestAccuracy >= 99 && gameMeta.BestEnduranceDistance >= 5000 && gameMeta.SurvivalCheckpoint >= 31,
	}

	for _, a := range achievementDefinitions() {
		if checks[a.ID] {
			unlockAchievement(a.ID)
		}
	}
}

func evaluateEnduranceAchievements() {
	unlockedShips := len(normalizeShipList(gameMeta.UnlockedShips))
	checks := map[string]bool{
		"END_FIRST_FLIGHT": gameMeta.EnduranceRuns >= 1,
		"END_250":          gameMeta.BestEnduranceDistance >= 250,
		"END_500":          gameMeta.BestEnduranceDistance >= 500,
		"END_WARP_1":       gameMeta.EnduranceWarpsCompleted >= 1,
		"END_UFO_1":        gameMeta.AlienBossesSurvived >= 1,
		"END_POWER_5":      gameMeta.EndurancePowerups >= 5,
		"END_SHIELD_1":     gameMeta.EnduranceShieldUses >= 1,
		"END_TIME_1":       gameMeta.EnduranceTimeUses >= 1,
		"CACHE_1":          gameMeta.SpaceCachesOpened >= 1,
		"FLEET_3":          unlockedShips >= 3,

		"END_1000":       gameMeta.BestEnduranceDistance >= 1000,
		"END_1500":       gameMeta.BestEnduranceDistance >= 1500,
		"END_2500":       gameMeta.BestEnduranceDistance >= 2500,
		"END_WARP_5":     gameMeta.EnduranceWarpsCompleted >= 5,
		"END_UFO_3":      gameMeta.AlienBossesSurvived >= 3,
		"END_TARGETS_25": gameMeta.EnduranceBestTargets >= 25,
		"END_POWER_25":   gameMeta.EndurancePowerups >= 25,
		"END_SHIELD_10":  gameMeta.EnduranceShieldUses >= 10,
		"END_TIME_10":    gameMeta.EnduranceTimeUses >= 10,
		"CACHE_10":       gameMeta.SpaceCachesOpened >= 10,

		"END_5000":        gameMeta.BestEnduranceDistance >= 5000,
		"END_7500":        gameMeta.BestEnduranceDistance >= 7500,
		"END_10000":       gameMeta.BestEnduranceDistance >= 10000,
		"END_WARP_20":     gameMeta.EnduranceWarpsCompleted >= 20,
		"END_UFO_10":      gameMeta.AlienBossesSurvived >= 10,
		"END_TARGETS_100": gameMeta.EnduranceBestTargets >= 100,
		"END_POWER_100":   gameMeta.EndurancePowerups >= 100,
		"END_SHIELD_50":   gameMeta.EnduranceShieldUses >= 50,
		"END_TIME_50":     gameMeta.EnduranceTimeUses >= 50,
		"FLEET_10":        unlockedShips >= 10,
		"TRIPLE_CROWN":    gameMeta.BestAccuracy >= 99 && gameMeta.BestEnduranceDistance >= 5000 && gameMeta.SurvivalCheckpoint >= 31,
	}
	for _, a := range achievementDefinitions() {
		if checks[a.ID] {
			unlockAchievement(a.ID)
		}
	}
}

func updateChallengesAfterClear(diffName string, combined float64, currentStreak int) {
	resetChallengePeriodsIfNeeded()
	gameMeta.DailyClears++
	if strings.EqualFold(diffName, "HARD") || strings.EqualFold(diffName, "INSANE") {
		gameMeta.DailyHardClears++
	}
	if combined >= 92 {
		gameMeta.DailyHighAcc++
	}

	gameMeta.WeeklyClears++
	if strings.EqualFold(diffName, "INSANE") {
		gameMeta.WeeklyInsane++
	}
	if currentStreak > gameMeta.WeeklyBestStreak {
		gameMeta.WeeklyBestStreak = currentStreak
	}

	rewardEarned := 0
	if !gameMeta.DailyClearsRewarded && gameMeta.DailyClears >= 5 {
		gameMeta.DailyClearsRewarded = true
		playerProgress.EXP += 5
		rewardEarned += 5
	}
	if !gameMeta.DailyHardRewarded && gameMeta.DailyHardClears >= 2 {
		gameMeta.DailyHardRewarded = true
		playerProgress.EXP += 10
		rewardEarned += 10
	}
	if !gameMeta.DailyAccRewarded && gameMeta.DailyHighAcc >= 1 {
		gameMeta.DailyAccRewarded = true
		playerProgress.EXP += 10
		rewardEarned += 10
	}
	if !gameMeta.DailyRewarded && gameMeta.DailyClearsRewarded && gameMeta.DailyHardRewarded && gameMeta.DailyAccRewarded {
		gameMeta.DailyRewarded = true
		playerProgress.EXP += 10
		rewardEarned += 10
	}
	if rewardEarned > 0 {
		dailyRewardAt = time.Now()
		lastDailyRewardEXP = rewardEarned
		savePlayerProgress()
	}

	if !gameMeta.WeeklyRewarded && gameMeta.WeeklyClears >= 25 && gameMeta.WeeklyInsane >= 5 && gameMeta.WeeklyBestStreak >= 15 {
		gameMeta.WeeklyRewarded = true
		playerProgress.EXP += 200
		weeklyRewardAt = time.Now()
		savePlayerProgress()
	}
	saveGameMeta()
}

func applyAudioVolumes() {
	// Apply volume unconditionally. Individual MCI aliases can be reopened lazily
	// (notably Starbase) even when the cached global audioReady flag is stale/false.
	// MCI safely ignores aliases that are not currently open.
	music := gameMeta.MusicVolume * 10
	if music > 1000 {
		music = 1000
	}
	if music < 0 {
		music = 0
	}
	duck := desiredMusicDuck()
	if duck < 0.20 {
		duck = 0.20
	}
	if duck > 1.0 {
		duck = 1.0
	}
	music = int(math.Round(float64(music) * duck))
	for _, alias := range []string{"boss", "endurance_music", "survival_music", "survival_section2", "survival_section3", "survival_boss1_music", "survival_boss2_music", "survival_boss3_music", "survival_boss2_heartbeat", "starbase_music"} {
		mci(fmt.Sprintf("setaudio %s volume to %d", alias, music))
	}
	// SFX volume is applied per independent waveOut handle at playback time.
}

func gameTimerInterval() uintptr {
	switch gameMeta.FPSMode {
	case 1:
		return 8
	case 2:
		return 5
	default:
		return 16
	}
}

func particleTimerInterval() uintptr {
	// ~60 Hz visual updates for smooth particles/background on the stable v68 renderer.
	return 16
}

func beginDeveloperPause() {
	if !developerPauseStarted.IsZero() || state != StatePlaying {
		return
	}
	// Capture the exact gameplay clock before the overlay appears.
	lastTime = updateRealTimeClock()
	developerPauseStarted = time.Now()
}

func endDeveloperPause() {
	if developerPauseStarted.IsZero() {
		return
	}
	now := time.Now()
	pausedFor := now.Sub(developerPauseStarted)
	developerPauseStarted = time.Time{}
	if pausedFor <= 0 || state != StatePlaying {
		return
	}
	// Shift absolute timers forward so opening the console consumes zero game time.
	startTime = startTime.Add(pausedFor)
	if !lastHitAt.IsZero() {
		lastHitAt = lastHitAt.Add(pausedFor)
	}
	if !enduranceWarpCueStarted.IsZero() {
		enduranceWarpCueStarted = enduranceWarpCueStarted.Add(pausedFor)
	}
	if !enduranceShieldUntil.IsZero() {
		enduranceShieldUntil = enduranceShieldUntil.Add(pausedFor)
	}
	if !enduranceSlowUntil.IsZero() {
		enduranceSlowUntil = enduranceSlowUntil.Add(pausedFor)
	}
	if !enduranceBlockSpawnTime.IsZero() {
		enduranceBlockSpawnTime = enduranceBlockSpawnTime.Add(pausedFor)
	}
	if !particleEpoch.IsZero() {
		particleEpoch = particleEpoch.Add(pausedFor)
	}
	for i := range enduranceTargetExplosions {
		enduranceTargetExplosions[i].Started = enduranceTargetExplosions[i].Started.Add(pausedFor)
	}
	// Prevent the fixed-step loop from trying to catch up all paused frames.
	enduranceLastTick = now
	enduranceLastLoopTime = now
	enduranceLastRenderTime = time.Time{}
}

var starbaseSettingsReturn bool

func setOverlay(mode OverlayMode) {
	previousOverlay := overlayMode
	if mode == OverlayDeveloperConsole && overlayMode != OverlayDeveloperConsole {
		beginDeveloperPause()
	}
	overlayMode = mode
	starbaseSettingsTransition := starbaseSettingsReturn && ((previousOverlay == OverlayAFKSingularity && mode == OverlaySettings) || (previousOverlay == OverlaySettings && mode == OverlayAFKSingularity))
	if !starbaseSettingsTransition && (previousOverlay == OverlayAFKSingularity) != (mode == OverlayAFKSingularity) {
		if mode == OverlayAFKSingularity {
			// Always reopen Starbase music on entry. The ready flag can outlive an
			// MCI alias that another mode stopped/closed.
			starbaseMusicReady = false
			// Starbase production uses the authenticated GLOBAL EXP rank only.
			// Refresh it on entry so rank-up bonuses appear without requiring the
			// player to visit the profile/EXP panel first.
			requestGlobalAccountEXP()
		}
		switchModeMusic()
		if mode == OverlayAFKSingularity {
			// Re-assert the saved volume after the Starbase alias has been reopened.
			// This avoids a stale MCI default volume when entering Starbase.
			applyAudioVolumes()
		}
	}

	// Normal UI screens render in one deterministic repaint. The previous
	// transition timer redrew the entire GDI window every 16ms and competed with
	// the Endurance Direct2D renderer, producing visible menu/background jumps.
	uiTransitionStart = time.Time{}

	if mode == OverlayProfile && discordConnected {
		go fetchMyGlobalStats()
		requestPlayerProfileSync()
	}
	if mainHwnd != 0 {
		if mode == OverlayGlobalLeaderboard {
			// Weekly/Competition reset countdown is live while the interface is open.
			setTimer.Call(mainHwnd, TIMER_UI, 250, 0)
		} else if mode == OverlaySpaceCache || mode == OverlayGarage || mode == OverlayRemoteProfile || mode == OverlayProfileSkins || mode == OverlayAFKSingularity {
			setTimer.Call(mainHwnd, TIMER_UI, 16, 0)
		} else {
			killTimer.Call(mainHwnd, TIMER_UI)
		}
		if mode == OverlayAFKSingularity || (mode == OverlaySettings && starbaseSettingsReturn) {
			setTimer.Call(mainHwnd, TIMER_STARBASE, 33, 0)
		} else {
			killTimer.Call(mainHwnd, TIMER_STARBASE)
		}
		invalidateRect.Call(mainHwnd, 0, 0)
		updateWindow.Call(mainHwnd)
	}
}

func closeOverlay() {
	wasDeveloperConsole := overlayMode == OverlayDeveloperConsole
	wasStarbase := overlayMode == OverlayAFKSingularity
	overlayMode = OverlayNone
	if wasStarbase {
		switchModeMusic()
	}
	if wasDeveloperConsole {
		endDeveloperPause()
	}
	if mainHwnd != 0 {
		if wasStarbase {
			killTimer.Call(mainHwnd, TIMER_STARBASE)
		}
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}
