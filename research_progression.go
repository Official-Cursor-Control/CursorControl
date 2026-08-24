package main

import (
	"math"
	"math/rand"
	"strings"
)

// Research-guided cosmetic rarity model.
// The cache first chooses a rarity tier, then a ship within that tier. This is
// easier for players to understand and easier for us to tune from telemetry than
// eleven unrelated per-ship weights.
const (
	shipTierMilSpec = iota
	shipTierRestricted
	shipTierCovert
	shipTierContraband
)

func shipRarityTier(id int) int {
	// garage order: default | blue x3 | purple x4 | red x3 | gold x1
	switch id {
	case 1, 3, 4:
		return shipTierMilSpec
	case 5, 6, 7, 10:
		return shipTierRestricted
	case 2, 8, 12:
		return shipTierCovert
	case 9:
		return shipTierContraband
	default:
		return shipTierMilSpec
	}
}

func shipRarityName(id int) string {
	switch shipRarityTier(id) {
	case shipTierRestricted:
		return "NEBULA"
	case shipTierCovert:
		return "NOVA"
	case shipTierContraband:
		return "CELESTIAL"
	default:
		return "ORBITAL"
	}
}

func shipRarityBaseChance(tier int) float64 {
	// Percent of spaceship-category rolls. Research baseline: common outcomes
	// dominate, aspirational outcomes remain genuinely rare, but never impossible.
	switch tier {
	case shipTierRestricted:
		return 24.0
	case shipTierCovert:
		return 7.0
	case shipTierContraband:
		return 1.0
	default:
		return 68.0
	}
}

func effectiveShipTierChances(redPity, goldPity int) [4]float64 {
	c := [4]float64{68, 24, 7, 1}

	// Soft Red+ protection begins after 15 unsuccessful spaceship rolls.
	// It rises gradually and reaches a hard Red+ guarantee on roll 25.
	if redPity >= 24 {
		c = [4]float64{0, 0, 99, 1}
	} else if redPity >= 15 {
		boost := float64(redPity-14) * 2.1
		c[shipTierCovert] += boost
		c[shipTierMilSpec] -= boost * 0.72
		c[shipTierRestricted] -= boost * 0.28
	}

	// Gold protection remains long-term prestige. Soft protection begins at 60
	// spaceship rolls and the 100th unresolved roll is guaranteed Celestial.
	if goldPity >= 99 {
		return [4]float64{0, 0, 0, 100}
	}
	if goldPity >= 60 {
		goldBoost := math.Min(9.0, float64(goldPity-59)*0.23)
		c[shipTierContraband] += goldBoost
		take := goldBoost
		for _, tier := range []int{shipTierMilSpec, shipTierRestricted, shipTierCovert} {
			share := []float64{0.70, 0.23, 0.07}[tier]
			c[tier] = math.Max(0, c[tier]-take*share)
		}
	}

	// Numerical guard so simulated and live probabilities always sum to 100.
	total := c[0] + c[1] + c[2] + c[3]
	if total <= 0 {
		return [4]float64{68, 24, 7, 1}
	}
	for i := range c {
		c[i] = c[i] * 100.0 / total
	}
	return c
}

func rollShipTierWithPity(redPity, goldPity int, r float64) int {
	c := effectiveShipTierChances(redPity, goldPity)
	acc := 0.0
	for tier := 0; tier < len(c); tier++ {
		acc += c[tier]
		if r < acc {
			return tier
		}
	}
	return shipTierContraband
}

func shipsForTier(tier int) []int {
	switch tier {
	case shipTierRestricted:
		return []int{5, 6, 7, 10}
	case shipTierCovert:
		return []int{2, 8, 12}
	case shipTierContraband:
		return []int{9}
	default:
		return []int{1, 3, 4}
	}
}

func rollSpaceShipProtected() int {
	tier := rollShipTierWithPity(gameMeta.ShipDropsSinceRedPlus, gameMeta.ShipDropsSinceGold, randf(0, 100))
	pool := shipsForTier(tier)
	return pool[rand.Intn(len(pool))]
}

func updateShipPityAfterRoll(ship int) {
	tier := shipRarityTier(ship)
	if tier >= shipTierCovert {
		gameMeta.ShipDropsSinceRedPlus = 0
	} else {
		gameMeta.ShipDropsSinceRedPlus++
	}
	if tier == shipTierContraband {
		gameMeta.ShipDropsSinceGold = 0
	} else {
		gameMeta.ShipDropsSinceGold++
	}
}

func duplicateCompensation(ship int) int {
	switch shipRarityTier(ship) {
	case shipTierRestricted:
		return 175
	case shipTierCovert:
		return 300
	case shipTierContraband:
		return 900
	default:
		return 100
	}
}

func normalizedAchievementShowcase() []string {
	out := []string{"", "", ""}
	seen := map[string]bool{}
	for i := 0; i < len(gameMeta.AchievementShowcase) && i < 3; i++ {
		id := strings.TrimSpace(gameMeta.AchievementShowcase[i])
		if id == "" || seen[id] || !achievementUnlocked(id) {
			continue
		}
		seen[id] = true
		out[i] = id
	}
	return out
}

func normalizeAchievementShowcaseValues(values []string) []string {
	out := []string{"", "", ""}
	seen := map[string]bool{}
	for i := 0; i < len(values) && i < 3; i++ {
		id := strings.TrimSpace(values[i])
		if id == "" || seen[id] || !achievementUnlocked(id) {
			continue
		}
		seen[id] = true
		out[i] = id
	}
	return out
}

func achievementShowcasesEqual(a, b []string) bool {
	aa := normalizeAchievementShowcaseValues(a)
	bb := normalizeAchievementShowcaseValues(b)
	for i := 0; i < 3; i++ {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func firstAvailableShowcaseSlot() int {
	showcase := normalizedAchievementShowcase()
	for i, id := range showcase {
		if strings.TrimSpace(id) == "" {
			return i
		}
	}
	return 0
}

func setAchievementShowcaseSlot(slot int, achievementID string) {
	if slot < 0 || slot >= 3 || !achievementUnlocked(achievementID) {
		return
	}
	showcase := normalizedAchievementShowcase()
	for len(showcase) < 3 {
		showcase = append(showcase, "")
	}
	// One achievement may occupy only one public showcase slot. Moving it to a
	// new slot clears its old position rather than duplicating it.
	for i := range showcase {
		if i != slot && showcase[i] == achievementID {
			showcase[i] = ""
		}
	}
	showcase[slot] = achievementID
	gameMeta.AchievementShowcase = showcase
	markAchievementShowcaseSelectionDirty()
	analyticsEvent("profile_customized", map[string]any{"type": "achievement_showcase", "slot": slot, "achievement": achievementID})
	saveGameMeta()
	requestPlayerProfileSync()
}

func cycleAchievementShowcase(slot int) {
	if slot < 0 || slot >= 3 || len(gameMeta.Achievements) == 0 {
		return
	}
	showcase := normalizedAchievementShowcase()
	for len(showcase) < 3 {
		showcase = append(showcase, "")
	}

	current := showcase[slot]
	defs := achievementDefinitions()
	unlocked := make([]string, 0, len(gameMeta.Achievements))
	for _, a := range defs {
		if achievementUnlocked(a.ID) {
			unlocked = append(unlocked, a.ID)
		}
	}
	if len(unlocked) == 0 {
		return
	}
	idx := -1
	for i, id := range unlocked {
		if id == current {
			idx = i
			break
		}
	}
	for step := 1; step <= len(unlocked); step++ {
		candidate := unlocked[(idx+step+len(unlocked))%len(unlocked)]
		duplicate := false
		for other, id := range showcase {
			if other != slot && id == candidate {
				duplicate = true
				break
			}
		}
		if !duplicate {
			showcase[slot] = candidate
			break
		}
	}
	gameMeta.AchievementShowcase = showcase
	markAchievementShowcaseSelectionDirty()
	analyticsEvent("profile_customized", map[string]any{"type": "achievement_showcase", "slot": slot})
	saveGameMeta()
	requestPlayerProfileSync()
}

func achievementHidden(id string) bool {
	// About 9% of the catalogue: discovery/status achievements only. No core
	// progression requirement is hidden from the player.
	switch id {
	case "PERFECTION", "UNBREAKABLE", "END_10000", "END_TARGETS_100", "END_UFO_10", "SURV_SENTINEL_FLAWLESS", "SURV_ARRAY_FLAWLESS", "TRIPLE_CROWN", "FLEET_10":
		return true
	default:
		return false
	}
}

func applyAchievementCosmeticReward(id string) {
	// Skill route to the Celestial ship: exceptional mastery in all three modes,
	// not luck alone. Cache ownership remains another route.
	if id == "TRIPLE_CROWN" && !shipUnlocked(9) {
		unlockShip(9)
		go publishSpaceCacheShipUnlock(9)
	}
}

func updatePersistentSurvivalProfileStats(wave, kills, combo int) {
	changed := false
	if wave > gameMeta.BestSurvivalWave {
		gameMeta.BestSurvivalWave = wave
		changed = true
	}
	if kills > gameMeta.BestSurvivalKills {
		gameMeta.BestSurvivalKills = kills
		changed = true
	}
	if combo > gameMeta.BestSurvivalCombo {
		gameMeta.BestSurvivalCombo = combo
		changed = true
	}
	if changed {
		saveGameMeta()
	}
}

func achievementPrestigeLevel(id string, legacyTier int) int {
	// Six presentation tiers built on the existing achievement contract, avoiding
	// destructive save migrations while giving players a clearer prestige ladder.
	legendary := map[string]bool{
		"PERFECTION": true, "UNBREAKABLE": true, "END_10000": true,
		"SURV_SENTINEL_FLAWLESS": true, "SURV_ARRAY_FLAWLESS": true, "TRIPLE_CROWN": true,
	}
	elite := map[string]bool{
		"THOUSAND_CLEARS": true, "TARGET_LEGEND": true, "INSANE_VETERAN": true,
		"MASTER": true, "END_7500": true, "END_TARGETS_100": true, "FLEET_10": true,
	}
	expert := map[string]bool{
		"VETERAN": true, "LASER_FOCUS": true, "NEAR_PERFECT": true,
		"IRON_STREAK": true, "MACHINE": true, "END_5000": true, "END_WARP_20": true,
		"END_UFO_10": true, "END_POWER_100": true, "END_SHIELD_50": true, "END_TIME_50": true,
		"SURV_SECTOR3": true, "SURV_ARRAY_DOWN": true,
	}
	if legendary[id] {
		return 5
	}
	if elite[id] {
		return 4
	}
	if expert[id] {
		return 3
	}
	if legacyTier >= 2 {
		return 3
	}
	if legacyTier == 1 {
		return 2
	}
	// Early achievements split between onboarding and common repeat goals.
	intro := map[string]bool{
		"FIRST_BLOOD": true, "TRIGGER_FINGER": true, "THREE_CHAIN": true,
		"HARD_CONTACT": true, "END_FIRST_FLIGHT": true, "END_WARP_1": true,
		"END_UFO_1": true, "CACHE_1": true, "SURV_FIRST_WAVE": true,
	}
	if intro[id] {
		return 0
	}
	return 1
}

func achievementPrestigeName(id string, legacyTier int) string {
	names := []string{"INTRO", "COMMON", "SKILLED", "EXPERT", "ELITE", "LEGENDARY"}
	return names[achievementPrestigeLevel(id, legacyTier)]
}

// achievementDifficultyLevel collapses the older six internal prestige bands into
// four clear player-facing difficulty levels. Internal prestige/reward logic stays
// intact; only presentation and sorting use this simpler ladder.
// 0 = Easy, 1 = Medium, 2 = Expert, 3 = Legendary.
func achievementDifficultyLevel(id string, legacyTier int) int {
	switch achievementPrestigeLevel(id, legacyTier) {
	case 0, 1:
		return 0
	case 2:
		return 1
	case 3, 4:
		return 2
	default:
		return 3
	}
}

func achievementDifficultyName(id string, legacyTier int) string {
	names := []string{"EASY", "MEDIUM", "EXPERT", "LEGENDARY"}
	return names[achievementDifficultyLevel(id, legacyTier)]
}
