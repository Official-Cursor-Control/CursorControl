//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// Section 5: Space Expeditions + Scout Ship. Every duration, reward, upgrade
// requirement and chance breakpoint is explicit so the AFK pacing is completely
// deterministic and can be rebalanced without changing save semantics.
const (
	afkScoutShipUnlockCost int64 = 250_000
	afkScoutMaxTier              = 3
	afkScoutStatMaxLevel         = 8
)

var (
	starCacheTraceOnce sync.Once
	starCacheTraceDim  []byte
	starCacheTraceHot  []byte
)

type afkExpeditionDef struct {
	Name             string
	Subtitle         string
	BaseSeconds      int64
	LaunchCost       int64
	StarbitReward    int64
	DataReward       int64
	RequiredShipTier int
	BaseCacheBonus   int
}

// Free routes introduce exploration. Advanced routes are Starbit sinks that
// trade a launch fee for substantially stronger NAV and cache opportunities.
// NAV Data is never consumed by missions; it remains exploration progression.
var afkExpeditions = [...]afkExpeditionDef{
	{"LUNAR RELAY", "LOCAL SIGNAL SWEEP", 16 * 60, 0, 220_000, 20, 1, 0},
	{"VIRELLA BELT", "ASTEROID SURVEY", 30 * 60, 0, 650_000, 40, 1, 0},
	{"CYAN NEBULA", "DEEP-SKY MAPPING", 55 * 60, 0, 2_000_000, 75, 1, 1},
	{"ORBITAL SURVEY", "SATELLITE CARTOGRAPHY", 70 * 60, 50_000, 3_500_000, 95, 1, 2},
	{"DERELICT SIGNAL", "SALVAGE RECON", 95 * 60, 150_000, 6_500_000, 130, 1, 4},
	{"ASTEROID FRONTIER", "MINERAL MAPPING", 2 * 60 * 60, 500_000, 14_000_000, 190, 2, 5},
	{"SHATTERED MOON", "FRACTURE SCAN", 3 * 60 * 60, 1_000_000, 28_000_000, 260, 2, 7},
	{"OUTER SECTOR", "LONG-RANGE RECON", 4 * 60 * 60, 2_500_000, 65_000_000, 380, 2, 9},
	{"DEEP ORBIT", "DARK-SPACE SURVEY", 6 * 60 * 60, 5_000_000, 160_000_000, 520, 2, 11},
	{"LOST EXPEDITION", "RECOVERY MISSION", 8 * 60 * 60, 10_000_000, 320_000_000, 700, 3, 14},
	{"VOID FRONTIER", "UNCHARTED SPACE", 10 * 60 * 60, 25_000_000, 700_000_000, 950, 3, 17},
	{"EVENT HORIZON", "BLACK-SECTOR EXPEDITION", 12 * 60 * 60, 50_000_000, 1_500_000_000, 1_300, 3, 22},
}

// Endurance ships now double as Starbase exploration hulls. The passive is
// snapshotted by ship ID when an expedition launches, so changing the equipped
// Endurance ship mid-flight cannot alter a mission that is already underway.
type afkShipExplorationPassive struct {
	Name                    string
	Description             string
	StarbitMultiplierMilli  int64
	NavMultiplierMilli      int64
	DurationMultiplierMilli int64
	CacheChanceBonus        int
}

func afkExplorationPassive(shipID int) afkShipExplorationPassive {
	base := afkShipExplorationPassive{
		Name:                    "CADET ARRAY",
		Description:             "+5% STARBITS // +5% NAV DATA",
		StarbitMultiplierMilli:  1050,
		NavMultiplierMilli:      1050,
		DurationMultiplierMilli: 1000,
	}
	switch shipID {
	case 1: // SOLAR WARDEN
		return afkShipExplorationPassive{"SOLAR HARVEST", "+12% EXPEDITION STARBITS", 1120, 1000, 1000, 0}
	case 3: // COBALT COMET
		return afkShipExplorationPassive{"DEEP SCAN", "+12% NAV DATA", 1000, 1120, 1000, 0}
	case 4: // ION VIPER
		return afkShipExplorationPassive{"ION DRIVE", "-10% EXPEDITION TIME", 1000, 1000, 900, 0}
	case 5: // CRIMSON NOVA
		return afkShipExplorationPassive{"NOVA RESERVE", "+15% EXPEDITION STARBITS", 1150, 1000, 1000, 0}
	case 6: // GOLD RUSH
		return afkShipExplorationPassive{"PROSPECTOR", "+10% STARBITS // +8% NAV DATA", 1100, 1080, 1000, 0}
	case 7: // EMBER HAWK
		return afkShipExplorationPassive{"AFTERBURNER", "-12% EXPEDITION TIME", 1000, 1000, 880, 0}
	case 10: // TOXIC PHANTOM
		return afkShipExplorationPassive{"CACHE SNIFFER", "+10% STAR CACHE CHANCE", 1000, 1000, 1000, 10}
	case 2: // NEON RIFT
		return afkShipExplorationPassive{"RIFT MAPPER", "+18% NAV DATA", 1000, 1180, 1000, 0}
	case 8: // ORCHID PRISM
		return afkShipExplorationPassive{"PRISMATIC SCANNER", "+8% NAV DATA // +8% STAR CACHE CHANCE", 1000, 1080, 1000, 8}
	case 12: // CELESTIAL PEARL
		return afkShipExplorationPassive{"CELESTIAL SURVEY", "+12% STARBITS // +12% NAV DATA", 1120, 1120, 1000, 0}
	case 9: // AURORA BLOOM
		return afkShipExplorationPassive{"AURORA SYNERGY", "+15% STARBITS // +15% NAV // -8% TIME", 1150, 1150, 920, 0}
	default:
		return base
	}
}

func afkValidExplorationShipID(shipID int) int {
	if shipID < 0 || shipID >= len(spaceShipDefs) || strings.TrimSpace(spaceShipDefs[shipID].Name) == "" {
		return 0
	}
	if shipID != 0 && !shipUnlocked(shipID) {
		return 0
	}
	return shipID
}

func afkCurrentExplorationShipID() int {
	// Keep the launched hull visually locked through flight, return, and reward
	// collection. Garage changes made during the mission only affect the next run.
	if afkExpeditionActive() || afkExpeditionRewardPending() || afkSpaceCachePending() || gameMeta.AFKPendingComponentCount > 0 {
		return afkValidExplorationShipID(gameMeta.AFKExpeditionShipID)
	}
	return afkValidExplorationShipID(gameMeta.SelectedShip)
}

// Level 1 is the baseline. Values are percentage of base expedition duration.
var afkExpeditionSpeedPercent = [...]int64{0, 100, 90, 80, 70, 60, 50, 42, 35}
var afkCapacitySecondsByLevel = [...]int64{
	0,
	2 * 60 * 60,
	3 * 60 * 60,
	4 * 60 * 60,
	6 * 60 * 60,
	8 * 60 * 60,
	12 * 60 * 60,
	16 * 60 * 60,
	24 * 60 * 60,
}
var afkCacheChancePercent = [...]int64{0, 5, 8, 12, 17, 23, 30, 38, 47}

// Navigation Data costs for stat upgrades. Index L is the cost from L -> L+1.
var afkScoutSpeedUpgradeCosts = [...]int64{0, 25, 60, 120, 220, 380, 620, 950, 0}
var afkScoutCapacityUpgradeCosts = [...]int64{0, 30, 75, 150, 280, 480, 780, 1_200, 0}
var afkScoutCacheUpgradeCosts = [...]int64{0, 35, 90, 180, 330, 560, 900, 1_400, 0}

// Scout tier 1->2 and 2->3. Tier upgrades also require completed expeditions.
var afkScoutTierDataCosts = [...]int64{0, 250, 1_500, 0}
var afkScoutTierExpeditionRequirements = [...]int{0, 4, 14, 0}

// Runtime-only page state. Progression remains entirely in gameMeta.
var afkExpeditionPanelOpen bool
var afkExpeditionSelected int
var afkShipGuideOpen bool

// v427 mission browser + launch presentation state. Scroll position is UI-only;
// expedition progression remains in gameMeta. Launch/return animation timestamps
// are deliberately runtime-only, so reopening the game during a mission jumps
// straight to the stable in-flight view instead of replaying launch audio.
var afkExpeditionScroll int32
var afkExpeditionScrollDragging bool
var afkExpeditionScrollDragOffset int32
var afkExpeditionLaunchVisualAt time.Time
var afkExpeditionLaunchSoundPending bool
var afkExpeditionReturnVisualAt time.Time

// v441: a returned Scout Ship announces itself in the Starbase world instead
// of relying on the player to notice the Scout Pad state. The notice is tied to
// the expedition serial so each completed mission can announce exactly once.
var afkExpeditionNoticeSeenSerial int = -1
var afkExpeditionNoticeStarted time.Time

const afkExpeditionLaunchTransition = 2400 * time.Millisecond
const afkExpeditionReturnTransition = 1900 * time.Millisecond

func afkScoutUnlocked() bool { return gameMeta.AFKScoutShipUnlocked }

func afkScoutTier() int {
	if !afkScoutUnlocked() {
		return 0
	}
	t := gameMeta.AFKScoutShipTier
	if t < 1 {
		return 1
	}
	if t > afkScoutMaxTier {
		return afkScoutMaxTier
	}
	return t
}

func afkScoutSpeedLevel() int {
	if !afkScoutUnlocked() {
		return 0
	}
	l := gameMeta.AFKExpeditionSpeedLevel
	if l < 1 {
		return 1
	}
	if l > afkScoutStatMaxLevel {
		return afkScoutStatMaxLevel
	}
	return l
}
func afkScoutCapacityLevel() int {
	if !afkScoutUnlocked() {
		return 1
	}
	l := gameMeta.AFKCapacityLevel
	if l < 1 {
		return 1
	}
	if l > afkScoutStatMaxLevel {
		return afkScoutStatMaxLevel
	}
	return l
}
func afkScoutCacheLevel() int {
	if !afkScoutUnlocked() {
		return 0
	}
	l := gameMeta.AFKCacheChanceLevel
	if l < 1 {
		return 1
	}
	if l > afkScoutStatMaxLevel {
		return afkScoutStatMaxLevel
	}
	return l
}

func afkSection5CapacitySeconds() int64 {
	l := afkScoutCapacityLevel()
	if l < 1 || l >= len(afkCapacitySecondsByLevel) {
		return afkBaseCapacitySeconds + afkResearchCapacityBonusSeconds()
	}
	return afkCapacitySecondsByLevel[l] + afkResearchCapacityBonusSeconds() + afkModuleCapacityBonusSeconds()
}

func afkExpeditionDurationSecondsForShip(index, shipID int) int64 {
	if index < 0 || index >= len(afkExpeditions) {
		return 0
	}
	l := afkScoutSpeedLevel()
	if l < 1 {
		l = 1
	}
	pct := afkExpeditionSpeedPercent[l]
	sec := afkExpeditions[index].BaseSeconds * pct / 100
	sec = sec * afkResearchExpeditionMultiplierMilli() / 1000
	sec = sec * afkTalentExpeditionMultiplierMilli() / 1000
	sec = sec * afkOperatorExpeditionMultiplierMilli() / 1000
	sec = sec * afkModuleExpeditionMultiplierMilli() / 1000
	passive := afkExplorationPassive(afkValidExplorationShipID(shipID))
	sec = sec * passive.DurationMultiplierMilli / 1000
	if sec < 60 {
		sec = 60
	}
	return sec
}

func afkExpeditionDurationSeconds(index int) int64 {
	return afkExpeditionDurationSecondsForShip(index, gameMeta.SelectedShip)
}

func afkExpeditionPreviewRewards(index, shipID int) (int64, int64) {
	if index < 0 || index >= len(afkExpeditions) {
		return 0, 0
	}
	d := afkExpeditions[index]
	passive := afkExplorationPassive(afkValidExplorationShipID(shipID))
	starbits := d.StarbitReward * afkOperatorStarbitRewardMultiplierMilli() / 1000
	starbits = starbits * passive.StarbitMultiplierMilli / 1000
	nav := d.DataReward * passive.NavMultiplierMilli / 1000
	nav = nav * afkTalentNavRewardMultiplierMilli() / 1000
	return starbits, nav
}

func afkExpeditionActive() bool { return gameMeta.AFKExpeditionEndsUnix > 0 }
func afkExpeditionRewardPending() bool {
	return gameMeta.AFKExpeditionPendingStarbits > 0 || gameMeta.AFKExpeditionPendingData > 0
}

func afkExpeditionReturnNoticeVisible() bool {
	return afkExpeditionRewardPending() && gameMeta.AFKExpeditionSerial != afkExpeditionNoticeSeenSerial && !afkExpeditionPanelOpen
}

func afkExpeditionReturnNoticeRect(w, hgt int32, now time.Time) RECT {
	width := sx(400, w)
	height := sy(94, hgt)
	if width < 320 {
		width = 320
	}
	if height < 76 {
		height = 76
	}
	targetLeft := w - width - sx(18, w)
	top := sy(214, hgt)
	if afkExpeditionNoticeStarted.IsZero() {
		afkExpeditionNoticeStarted = now
	}
	progress := float64(now.Sub(afkExpeditionNoticeStarted)) / float64(520*time.Millisecond)
	progress = afkEase01(progress)
	startLeft := w + sx(12, w)
	left := startLeft + int32(float64(targetLeft-startLeft)*progress)
	return RECT{left, top, left + width, top + height}
}

func drawAFKExpeditionReturnNotification(hdc uintptr, w, hgt int32) {
	if !afkExpeditionReturnNoticeVisible() || afkAnyManagementPanelOpen() {
		return
	}
	now := time.Now()
	r := afkExpeditionReturnNoticeRect(w, hgt, now)
	pulse := afkSoftPulse(now, 1.1, 0)
	border := afkRGBLerp(56, 181, 232, 112, 232, 255, pulse)
	drawBevelPanel(hdc, r, rgb(3, 19, 40), border, rgb(0, 5, 14), 2)
	fillSolidRect(hdc, RECT{r.Left, r.Top, r.Left + sx(5, w), r.Bottom}, rgb(255, 205, 68))
	icon := RECT{r.Left + sx(12, w), r.Top + sy(14, hgt), r.Left + sx(78, w), r.Bottom - sy(14, hgt)}
	drawAFKFacilitySprite(hdc, "scout_ship", icon)
	textLeft := r.Left + sx(88, w)
	textRight := r.Right - sx(12, w)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(239, 248, 255))
		line1 := fitTextEllipsis(hdc, hudTinyFont, "YOUR SHIP HAS RETURNED", textRight-textLeft)
		line2 := fitTextEllipsis(hdc, hudTinyFont, "FROM AN EXPEDITION.", textRight-textLeft)
		textOut(hdc, textLeft, r.Top+sy(14, hgt), line1)
		textOut(hdc, textLeft, r.Top+sy(35, hgt), line2)
		setTextColor.Call(hdc, rgb(255, 215, 70))
		reward := fitTextEllipsis(hdc, hudTinyFont, "CLAIM YOUR REWARD", textRight-textLeft)
		textOut(hdc, textLeft, r.Top+sy(60, hgt), reward)
		selectObject.Call(hdc, old)
	}
}

func handleAFKExpeditionReturnNotificationClick(h uintptr, p FPoint, w, hgt int32) bool {
	if !afkExpeditionReturnNoticeVisible() || afkAnyManagementPanelOpen() {
		return false
	}
	r := afkExpeditionReturnNoticeRect(w, hgt, time.Now())
	if !pointInRect(p, r) {
		return false
	}
	afkExpeditionNoticeSeenSerial = gameMeta.AFKExpeditionSerial
	afkExpeditionNoticeStarted = time.Time{}
	afkCloseManagementPanels()
	afkExpeditionPanelOpen = true
	afkShipGuideOpen = false
	if gameMeta.AFKExpeditionDestination >= 0 && gameMeta.AFKExpeditionDestination < len(afkExpeditions) {
		afkExpeditionSelected = gameMeta.AFKExpeditionDestination
	}
	playUIButtonClickSound()
	invalidateRect.Call(h, 0, 0)
	return true
}
func afkSpaceCachePending() bool { return gameMeta.AFKExpeditionPendingCache }

func afkExpeditionAvailable(index int) bool {
	return afkScoutUnlocked() && index >= 0 && index < len(afkExpeditions) && afkScoutTier() >= afkExpeditions[index].RequiredShipTier
}

func afkSelectedExpeditionIndex() int {
	idx := afkExpeditionSelected
	if idx < 0 || idx >= len(afkExpeditions) {
		idx = 0
	}
	if !afkExpeditionAvailable(idx) {
		for i := range afkExpeditions {
			if afkExpeditionAvailable(i) {
				return i
			}
		}
		return 0
	}
	return idx
}

func afkLaunchExpedition(index int, now time.Time) bool {
	if !afkExpeditionAvailable(index) || afkExpeditionActive() || afkExpeditionRewardPending() || afkSpaceCachePending() || (afkModulesAvailable() && gameMeta.AFKPendingComponentCount > 0) {
		return false
	}
	shipID := afkValidExplorationShipID(gameMeta.SelectedShip)
	duration := afkExpeditionDurationSecondsForShip(index, shipID)
	if duration <= 0 {
		return false
	}
	cost := afkExpeditions[index].LaunchCost
	if cost > 0 && !afkSpendStarbits(cost) {
		return false
	}
	gameMeta.AFKExpeditionSerial++
	gameMeta.AFKExpeditionDestination = index
	gameMeta.AFKExpeditionShipID = shipID
	gameMeta.AFKExpeditionFireColorID = gameMeta.SelectedFireColor
	gameMeta.AFKExpeditionFireSizeID = gameMeta.SelectedFireSize
	gameMeta.AFKExpeditionStartedUnix = now.Unix()
	gameMeta.AFKExpeditionEndsUnix = now.Unix() + duration
	afkExpeditionNoticeStarted = time.Time{}
	afkExpeditionLaunchVisualAt = now
	afkExpeditionLaunchSoundPending = true
	afkExpeditionReturnVisualAt = time.Time{}
	return true
}

func afkDeterministicCacheRoll(serial, destination, chanceLevel, shipID int) (bool, int) {
	chance := int(afkCacheChancePercent[afkScoutCacheLevel()]) + afkResearchCacheBonusPercent() + afkTalentCacheChanceBonusPercent() + afkExplorationPassive(afkValidExplorationShipID(shipID)).CacheChanceBonus
	if destination >= 0 && destination < len(afkExpeditions) {
		chance += afkExpeditions[destination].BaseCacheBonus
	}
	if chance > 85 {
		chance = 85
	}
	if chance <= 0 {
		return false, 0
	}
	// Fixed integer hash: no RNG state, no rerolling by restarting the game.
	x := uint32(serial*1103515245 + destination*12345 + chanceLevel*7919 + 0x5A17)
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	if int(x%100) >= chance {
		return false, 0
	}
	rarityRoll := int((x / 101) % 1000)
	switch {
	case rarityRoll < 15:
		return true, 4 // legendary 1.5%
	case rarityRoll < 90:
		return true, 3 // epic 7.5%
	case rarityRoll < 300:
		return true, 2 // rare 21%
	default:
		return true, 1
	}
}

func afkRefreshExpeditionCompletion(now time.Time) {
	if !afkExpeditionActive() || now.Unix() < gameMeta.AFKExpeditionEndsUnix {
		return
	}
	idx := gameMeta.AFKExpeditionDestination
	if idx < 0 || idx >= len(afkExpeditions) {
		gameMeta.AFKExpeditionStartedUnix = 0
		gameMeta.AFKExpeditionEndsUnix = 0
		return
	}
	d := afkExpeditions[idx]
	shipID := afkValidExplorationShipID(gameMeta.AFKExpeditionShipID)
	passive := afkExplorationPassive(shipID)
	gameMeta.AFKExpeditionPendingStarbits = d.StarbitReward * afkOperatorStarbitRewardMultiplierMilli() / 1000
	gameMeta.AFKExpeditionPendingStarbits = gameMeta.AFKExpeditionPendingStarbits * passive.StarbitMultiplierMilli / 1000
	gameMeta.AFKExpeditionPendingData = d.DataReward * passive.NavMultiplierMilli / 1000
	found, rarity := afkDeterministicCacheRoll(gameMeta.AFKExpeditionSerial, idx, afkScoutCacheLevel(), shipID)
	gameMeta.AFKExpeditionPendingCache = found
	gameMeta.AFKSpaceCacheRarity = rarity
	gameMeta.AFKExpeditionStartedUnix = 0
	gameMeta.AFKExpeditionEndsUnix = 0
	afkExpeditionNoticeStarted = time.Time{}
	afkExpeditionReturnVisualAt = now
}

func afkClaimExpeditionReward() bool {
	if !afkActionAllowed("claim_expedition", time.Now(), afkClaimActionDebounce) || !afkExpeditionRewardPending() {
		return false
	}
	if gameMeta.AFKExpeditionPendingStarbits > 0 {
		afkGrantStarbits(gameMeta.AFKExpeditionPendingStarbits)
	}
	gameMeta.AFKNavigationData += gameMeta.AFKExpeditionPendingData
	gameMeta.AFKExpeditionPendingStarbits = 0
	gameMeta.AFKExpeditionPendingData = 0
	afkExpeditionNoticeSeenSerial = gameMeta.AFKExpeditionSerial
	afkExpeditionNoticeStarted = time.Time{}
	gameMeta.AFKExpeditionsCompleted++
	afkRefreshTalentPointAwards()
	// v461: the material shown in the Expedition Complete popup is collected
	// together with the Starbits/NAV reward, removing the old second thin claim.
	afkAwardPendingCraftComponent()
	afkCollectPendingCraftComponent()
	afkRefreshSection5Completion()
	return true
}

func afkCacheReward(rarity int) (starbits int64, label string) {
	idx := gameMeta.AFKExpeditionDestination
	base := int64(250_000)
	if idx >= 0 && idx < len(afkExpeditions) {
		base = afkExpeditions[idx].StarbitReward
	}
	var mult int64
	switch rarity {
	case 4:
		mult, label = 20, "LEGENDARY"
	case 3:
		mult, label = 8, "EPIC"
	case 2:
		mult, label = 4, "RARE"
	default:
		mult, label = 2, "COMMON"
	}
	bonus := afkTalentCacheRewardMultiplierMilli()
	bonus = bonus * afkOperatorStarbitRewardMultiplierMilli() / 1000
	bonus = bonus * afkModuleCacheRewardMultiplierMilli() / 1000
	starbits = base * mult * bonus / 1000
	return
}

func afkClaimSpaceCache() bool {
	if !afkActionAllowed("claim_space_cache", time.Now(), afkClaimActionDebounce) || !afkSpaceCachePending() {
		return false
	}
	starbits, _ := afkCacheReward(gameMeta.AFKSpaceCacheRarity)
	afkGrantStarbits(starbits)
	gameMeta.AFKSpaceCachesClaimed++
	gameMeta.AFKExpeditionPendingCache = false
	gameMeta.AFKSpaceCacheRarity = 0
	return true
}

func afkScoutStatUpgradeCost(kind int) int64 {
	var lvl int
	var table []int64
	switch kind {
	case 0:
		lvl, table = afkScoutSpeedLevel(), afkScoutSpeedUpgradeCosts[:]
	case 1:
		lvl, table = afkScoutCapacityLevel(), afkScoutCapacityUpgradeCosts[:]
	default:
		lvl, table = afkScoutCacheLevel(), afkScoutCacheUpgradeCosts[:]
	}
	if lvl < 1 || lvl >= afkScoutStatMaxLevel {
		return 0
	}
	return table[lvl]
}

func afkUpgradeScoutStat(kind int) bool {
	if !afkActionAllowed(fmt.Sprintf("upgrade_scout_%d", kind), time.Now(), afkEconomyActionDebounce) {
		return false
	}
	cost := afkScoutStatUpgradeCost(kind)
	if cost <= 0 || gameMeta.AFKNavigationData < cost {
		return false
	}
	gameMeta.AFKNavigationData -= cost
	switch kind {
	case 0:
		gameMeta.AFKExpeditionSpeedLevel++
	case 1:
		gameMeta.AFKCapacityLevel++
	default:
		gameMeta.AFKCacheChanceLevel++
	}
	afkRefreshSection5Completion()
	afkCloudMarkProgressDirty()
	return true
}

func afkNextScoutTierCost() (int64, int) {
	t := afkScoutTier()
	if t < 1 || t >= afkScoutMaxTier {
		return 0, 0
	}
	return afkScoutTierDataCosts[t], afkScoutTierExpeditionRequirements[t]
}
func afkCanUpgradeScoutTier() bool {
	cost, req := afkNextScoutTierCost()
	return cost > 0 && gameMeta.AFKNavigationData >= cost && gameMeta.AFKExpeditionsCompleted >= req
}
func afkUpgradeScoutTier() bool {
	if !afkActionAllowed("upgrade_scout_tier", time.Now(), afkClaimActionDebounce) || !afkCanUpgradeScoutTier() {
		return false
	}
	cost, _ := afkNextScoutTierCost()
	gameMeta.AFKNavigationData -= cost
	gameMeta.AFKScoutShipTier++
	if gameMeta.AFKScoutShipTier > afkScoutMaxTier {
		gameMeta.AFKScoutShipTier = afkScoutMaxTier
	}
	afkRefreshSection5Completion()
	afkRefreshTalentPointAwards()
	afkCloudMarkProgressDirty()
	playAFKTierUpSound()
	return true
}

func afkSection5IsComplete() bool {
	return gameMeta.AFKSection5Complete || (afkScoutTier() >= 2 && gameMeta.AFKExpeditionsCompleted >= 6 && afkScoutSpeedLevel() >= 3 && afkScoutCapacityLevel() >= 3 && afkScoutCacheLevel() >= 3)
}
func afkRefreshSection5Completion() {
	if afkScoutTier() >= 2 && gameMeta.AFKExpeditionsCompleted >= 6 && afkScoutSpeedLevel() >= 3 && afkScoutCapacityLevel() >= 3 && afkScoutCacheLevel() >= 3 {
		gameMeta.AFKSection5Complete = true
	}
}

func afkExpeditionToggleRect(w, hgt int32) RECT {
	return afkFacilityButtonRect(w, hgt, 1)
}
func afkExpeditionPanelRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }
func afkScoutUnlockRect(w, hgt int32) RECT {
	p := afkExpeditionPanelRect(w, hgt)
	return RECT{p.Left + sx(210, w), p.Bottom - sy(78, hgt), p.Right - sx(210, w), p.Bottom - sy(30, hgt)}
}
func afkExpeditionLaunchRect(w, hgt int32) RECT {
	p := afkExpeditionPanelRect(w, hgt)
	return RECT{p.Left + sx(265, w), p.Bottom - sy(76, hgt), p.Right - sx(265, w), p.Bottom - sy(30, hgt)}
}
func afkExpeditionClaimRect(w, hgt int32) RECT { return afkExpeditionLaunchRect(w, hgt) }
func afkCacheClaimRect(w, hgt int32) RECT      { return afkExpeditionLaunchRect(w, hgt) }

func afkExpeditionRewardPopupRect(w, hgt int32) RECT {
	p := afkExpeditionPanelRect(w, hgt)
	rw := sx(820, w)
	rh := sy(410, hgt)
	maxW := p.Right - p.Left - sx(54, w)
	maxH := p.Bottom - p.Top - sy(54, hgt)
	if rw > maxW {
		rw = maxW
	}
	if rh > maxH {
		rh = maxH
	}
	cx := (p.Left + p.Right) / 2
	cy := (p.Top + p.Bottom) / 2
	return RECT{cx - rw/2, cy - rh/2, cx + rw/2, cy + rh/2}
}

func afkExpeditionRewardPopupClaimRect(w, hgt int32) RECT {
	r := afkExpeditionRewardPopupRect(w, hgt)
	return RECT{r.Left + sx(220, w), r.Bottom - sy(64, hgt), r.Right - sx(220, w), r.Bottom - sy(20, hgt)}
}
func afkExpeditionColumns(w, hgt int32) (RECT, RECT) {
	p := afkExpeditionPanelRect(w, hgt)
	gap := sx(18, w)
	innerL := p.Left + sx(22, w)
	innerR := p.Right - sx(22, w)
	contentTop := p.Top + sy(88, hgt)
	contentBottom := p.Bottom - sy(92, hgt)
	available := innerR - innerL - gap
	leftW := available * 61 / 100
	left := RECT{innerL, contentTop, innerL + leftW, contentBottom}
	right := RECT{left.Right + gap, contentTop, innerR, contentBottom}
	return left, right
}
func afkExpeditionMissionViewportRect(w, hgt int32) RECT {
	l, _ := afkExpeditionColumns(w, hgt)
	return RECT{l.Left + sx(4, w), l.Top + sy(30, hgt), l.Right - sx(22, w), l.Bottom - sy(7, hgt)}
}

func afkExpeditionMissionCardHeight(w, hgt int32) int32 {
	ch := sy(94, hgt)
	if ch < sy(84, hgt) {
		ch = sy(84, hgt)
	}
	return ch
}

func afkExpeditionMissionContentHeight(w, hgt int32) int32 {
	rows := (len(afkExpeditions) + 1) / 2
	if rows <= 0 {
		return 0
	}
	return int32(rows)*afkExpeditionMissionCardHeight(w, hgt) + int32(rows-1)*sy(10, hgt)
}

func afkExpeditionMaxScroll(w, hgt int32) int32 {
	vp := afkExpeditionMissionViewportRect(w, hgt)
	m := afkExpeditionMissionContentHeight(w, hgt) - (vp.Bottom - vp.Top)
	if m < 0 {
		return 0
	}
	return m
}

func clampAFKExpeditionScroll(w, hgt int32) {
	m := afkExpeditionMaxScroll(w, hgt)
	if afkExpeditionScroll < 0 {
		afkExpeditionScroll = 0
	}
	if afkExpeditionScroll > m {
		afkExpeditionScroll = m
	}
}

func afkExpeditionScrollbarRects(w, hgt int32) (RECT, RECT) {
	l, _ := afkExpeditionColumns(w, hgt)
	vp := afkExpeditionMissionViewportRect(w, hgt)
	track := RECT{l.Right - sx(15, w), vp.Top, l.Right - sx(5, w), vp.Bottom}
	maxScroll := afkExpeditionMaxScroll(w, hgt)
	if maxScroll <= 0 {
		return track, track
	}
	visible := vp.Bottom - vp.Top
	content := afkExpeditionMissionContentHeight(w, hgt)
	thumbH := visible * visible / content
	if thumbH < sy(42, hgt) {
		thumbH = sy(42, hgt)
	}
	travel := (track.Bottom - track.Top) - thumbH
	top := track.Top
	if travel > 0 {
		top += int32(int64(travel) * int64(afkExpeditionScroll) / int64(maxScroll))
	}
	return track, RECT{track.Left, top, track.Right, top + thumbH}
}

func afkExpeditionScrollbarDragTo(y, w, hgt int32) {
	track, thumb := afkExpeditionScrollbarRects(w, hgt)
	maxScroll := afkExpeditionMaxScroll(w, hgt)
	if maxScroll <= 0 {
		afkExpeditionScroll = 0
		return
	}
	thumbH := thumb.Bottom - thumb.Top
	travel := (track.Bottom - track.Top) - thumbH
	if travel <= 0 {
		afkExpeditionScroll = 0
		return
	}
	pos := y - track.Top
	if pos < 0 {
		pos = 0
	}
	if pos > travel {
		pos = travel
	}
	afkExpeditionScroll = int32(int64(pos) * int64(maxScroll) / int64(travel))
	clampAFKExpeditionScroll(w, hgt)
}

func afkExpeditionDestinationRect(w, hgt int32, index int) RECT {
	vp := afkExpeditionMissionViewportRect(w, hgt)
	col := index % 2
	row := index / 2
	gapX := sx(12, w)
	gapY := sy(10, hgt)
	cardW := (vp.Right - vp.Left - gapX) / 2
	cardH := afkExpeditionMissionCardHeight(w, hgt)
	x := vp.Left + int32(col)*(cardW+gapX)
	y := vp.Top + int32(row)*(cardH+gapY) - afkExpeditionScroll
	return RECT{x, y, x + cardW, y + cardH}
}

func afkExpeditionFlightViewportRect(w, hgt int32) RECT {
	_, r := afkExpeditionColumns(w, hgt)
	return RECT{r.Left + sx(6, w), r.Top + sy(6, hgt), r.Right - sx(6, w), r.Top + sy(168, hgt)}
}

func afkShipGuideRect(w, hgt int32) RECT {
	_, r := afkExpeditionColumns(w, hgt)
	return RECT{r.Left + sx(18, w), r.Top + sy(178, hgt), r.Right - sx(18, w), r.Top + sy(206, hgt)}
}

func afkScoutTierRect(w, hgt int32) RECT {
	_, r := afkExpeditionColumns(w, hgt)
	return RECT{r.Left + sx(18, w), r.Top + sy(214, hgt), r.Right - sx(18, w), r.Top + sy(246, hgt)}
}
func afkScoutStatRect(w, hgt int32, kind int) RECT {
	_, r := afkExpeditionColumns(w, hgt)
	top := r.Top + sy(float64(254+kind*39), hgt)
	return RECT{r.Left + sx(18, w), top, r.Right - sx(18, w), top + sy(31, hgt)}
}

func afkExpeditionFireColorID() int {
	id := gameMeta.SelectedFireColor
	if afkExpeditionActive() || afkExpeditionRewardPending() || afkSpaceCachePending() {
		id = gameMeta.AFKExpeditionFireColorID
	}
	if id < 0 || id >= len(fireColorDefs) || !fireColorUnlocked(id) {
		id = 0
	}
	return id
}

func afkExpeditionFireSizeID() int {
	id := gameMeta.SelectedFireSize
	if afkExpeditionActive() || afkExpeditionRewardPending() || afkSpaceCachePending() {
		id = gameMeta.AFKExpeditionFireSizeID
	}
	if id < 0 || id >= len(fireSizeDefs) || !fireSizeUnlocked(id) {
		id = 0
	}
	return id
}

func afkExpeditionFireSizeMultiplier() float32 {
	id := afkExpeditionFireSizeID()
	if id < 0 || id >= len(fireSizeDefs) {
		return 1
	}
	return fireSizeDefs[id].Multiplier
}

func afkExpeditionThrusterColour() uintptr {
	id := afkExpeditionFireColorID()
	if id != 7 {
		return fireColorDefs[id].Color
	}
	palette := []uintptr{rgb(255, 55, 30), rgb(255, 205, 45), rgb(55, 245, 85), rgb(55, 145, 255), rgb(170, 70, 255), rgb(255, 80, 190)}
	return palette[int(time.Now().UnixMilli()/120)%len(palette)]
}

func drawAFKExpeditionBackgroundSlice(hdc uintptr, src RECT, dst RECT) {
	const srcW int32 = 930
	const srcH int32 = 1692
	if len(expeditionHangarBGRA) < int(srcW*srcH*4) || dst.Right <= dst.Left || dst.Bottom <= dst.Top {
		return
	}
	if src.Left < 0 {
		src.Left = 0
	}
	if src.Top < 0 {
		src.Top = 0
	}
	if src.Right > srcW {
		src.Right = srcW
	}
	if src.Bottom > srcH {
		src.Bottom = srcH
	}
	if src.Right <= src.Left || src.Bottom <= src.Top {
		return
	}
	sprite := ensureRuntimeSprite(hdc, expeditionHangarBGRA, srcW, srcH)
	if sprite == nil || sprite.dc == 0 {
		return
	}
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(dst.Left), uintptr(dst.Top), uintptr(dst.Right-dst.Left), uintptr(dst.Bottom-dst.Top), sprite.dc, uintptr(src.Left), uintptr(src.Top), uintptr(src.Right-src.Left), uintptr(src.Bottom-src.Top), blend)
}

func afkExpeditionBackgroundCropForViewport(view RECT) (int32, int32) {
	const srcW int32 = 930
	const srcH int32 = 1692
	dw := view.Right - view.Left
	dh := view.Bottom - view.Top
	if dw <= 0 || dh <= 0 {
		return srcW, 420
	}
	cropH := int32(float64(srcW) * float64(dh) / float64(dw))
	if cropH < 260 {
		cropH = 260
	}
	if cropH > srcH {
		cropH = srcH
	}
	return srcW, cropH
}

func afkExpeditionLaunchProgress(now time.Time) (float64, bool) {
	if !afkExpeditionActive() {
		return 0, false
	}
	if afkExpeditionLaunchVisualAt.IsZero() {
		// Active mission restored from cloud/save: do not replay launch animation.
		return 1, true
	}
	p := float64(now.Sub(afkExpeditionLaunchVisualAt)) / float64(afkExpeditionLaunchTransition)
	if p < 0 {
		p = 0
	}
	if p >= 1 {
		p = 1
		return p, true
	}
	// Smoothstep keeps the hangar departure quick but not abrupt.
	p = p * p * (3 - 2*p)
	return p, false
}

func afkUpdateExpeditionPresentation(now time.Time) {
	if afkExpeditionLaunchSoundPending && afkExpeditionActive() && !afkExpeditionLaunchVisualAt.IsZero() && now.Sub(afkExpeditionLaunchVisualAt) >= afkExpeditionLaunchTransition {
		afkExpeditionLaunchSoundPending = false
		playAFKExpeditionLaunchSound()
	}
}

func drawAFKExpeditionFlightBackground(hdc uintptr, w, hgt int32) {
	view := afkExpeditionFlightViewportRect(w, hgt)
	if len(expeditionHangarBGRA) < 930*1692*4 {
		return
	}
	now := time.Now()
	_, cropH := afkExpeditionBackgroundCropForViewport(view)
	bottomY := int32(1692) - cropH
	spaceY := int32(150)
	if spaceY+cropH > 1180 {
		spaceY = 0
	}

	if afkExpeditionActive() {
		// Moving Background OFF and Reduced Motion preserve the scene but stop
		// the travelling camera scroll.
		if !backgroundMotionEnabled() {
			drawAFKExpeditionBackgroundSlice(hdc, RECT{0, spaceY, 930, spaceY + cropH}, view)
			return
		}
		p, flying := afkExpeditionLaunchProgress(now)
		if !flying {
			y := bottomY - int32(float64(bottomY-spaceY)*p)
			drawAFKExpeditionBackgroundSlice(hdc, RECT{0, y, 930, y + cropH}, view)
			return
		}
		// Once clear of the hangar, keep the viewport fixed in open space and move
		// two copies leftward. The background—not the ship—creates the travel motion.
		phase := math.Mod(float64(now.UnixMilli())/1000.0/9.0, 1.0)
		vw := view.Right - view.Left
		off := int32(math.Round(phase * float64(vw)))
		saved, _, _ := saveDC.Call(hdc)
		if saved != 0 {
			intersectClipRect.Call(hdc, uintptr(view.Left), uintptr(view.Top), uintptr(view.Right), uintptr(view.Bottom))
			defer restoreDC.Call(hdc, saved)
		}
		drawAFKExpeditionBackgroundSlice(hdc, RECT{0, spaceY, 930, spaceY + cropH}, RECT{view.Left - off, view.Top, view.Right - off, view.Bottom})
		drawAFKExpeditionBackgroundSlice(hdc, RECT{0, spaceY, 930, spaceY + cropH}, RECT{view.Left - off + vw, view.Top, view.Right - off + vw, view.Bottom})
		return
	}

	// A mission that completed while this process was running glides back to the
	// hangar. Reloaded claimable missions simply show the stable hangar view.
	if !afkExpeditionReturnVisualAt.IsZero() {
		age := now.Sub(afkExpeditionReturnVisualAt)
		if age < afkExpeditionReturnTransition {
			p := float64(age) / float64(afkExpeditionReturnTransition)
			p = p * p * (3 - 2*p)
			y := spaceY + int32(float64(bottomY-spaceY)*p)
			drawAFKExpeditionBackgroundSlice(hdc, RECT{0, y, 930, y + cropH}, view)
			return
		}
		afkExpeditionReturnVisualAt = time.Time{}
	}
	drawAFKExpeditionBackgroundSlice(hdc, RECT{0, bottomY, 930, bottomY + cropH}, view)
}

func afkExpeditionFireVisible(now time.Time) bool {
	if !afkExpeditionActive() {
		return false
	}
	_, flying := afkExpeditionLaunchProgress(now)
	return flying
}

func drawAFKScoutShip(hdc uintptr, r RECT, w, hgt int32) {
	// The ship is always a separate sprite over the hangar/space viewport. This
	// preserves the exact snapshotted Endurance ship and prevents a later Garage
	// change from changing either visuals or passive during an active expedition.
	t := float64(time.Now().UnixMilli()) / 1000.0
	shipID := afkCurrentExplorationShipID()
	if shipID < 0 || shipID >= len(spaceShipBGRA) || !shipUnlocked(shipID) {
		shipID = 0
	}
	data, sw, sh := shipTextureDataAndSize(shipID)
	if sw <= 0 || sh <= 0 || len(data) < int(sw*sh*4) {
		return
	}
	crop := alphaBoundsBGRA(data, sw, sh, 5)
	cropW, cropH := crop.Right-crop.Left, crop.Bottom-crop.Top
	if cropW <= 0 || cropH <= 0 {
		return
	}

	flightBox := afkExpeditionFlightViewportRect(w, hgt)
	boxW, boxH := sx(112, w), sy(66, hgt)
	dstW := boxW
	dstH := int32(float64(dstW) * float64(cropH) / float64(cropW))
	if dstH > boxH {
		dstH = boxH
		dstW = int32(float64(dstH) * float64(cropW) / float64(cropH))
	}
	cx := (flightBox.Left + flightBox.Right) / 2
	baseY := flightBox.Top + (flightBox.Bottom-flightBox.Top)*47/100
	bob := int32(math.Round(math.Sin(t*2.0) * float64(max32(1, sy(3, hgt)))))
	shipR := RECT{cx - dstW/2, baseY + bob - dstH/2, cx + dstW/2, baseY + bob + dstH/2}

	saved, _, _ := saveDC.Call(hdc)
	if saved != 0 {
		intersectClipRect.Call(hdc, uintptr(flightBox.Left), uintptr(flightBox.Top), uintptr(flightBox.Right), uintptr(flightBox.Bottom))
		defer restoreDC.Call(hdc, saved)
	}

	if afkExpeditionFireVisible(time.Now()) {
		fire := afkExpeditionThrusterColour()
		sizeMul := float64(afkExpeditionFireSizeMultiplier())
		pulse := 0.84 + 0.16*(0.5+0.5*math.Sin(t*15.0))
		flameLen := int32(float64(sx(30, w)) * sizeMul * pulse)
		flameHalf := int32(float64(sy(6, hgt)) * (0.80 + 0.20*sizeMul))
		if flameHalf < 2 {
			flameHalf = 2
		}
		nozzleX := shipR.Left + max32(1, (shipR.Right-shipR.Left)*2/32)
		nozzleY := (shipR.Top + shipR.Bottom) / 2
		fillSolidRect(hdc, RECT{nozzleX - flameLen, nozzleY - flameHalf, nozzleX, nozzleY + flameHalf}, blendColor(fire, rgb(0, 0, 0), 0.20))
		fillSolidRect(hdc, RECT{nozzleX - flameLen*3/4, nozzleY - flameHalf*2/3, nozzleX + sx(1, w), nozzleY + flameHalf*2/3}, fire)
		fillSolidRect(hdc, RECT{nozzleX - flameLen/2, nozzleY - max32(1, flameHalf/3), nozzleX + sx(2, w), nozzleY + max32(1, flameHalf/3)}, blendColor(fire, rgb(255, 255, 255), 0.58))
	}
	drawRawBGRATrimmedFit(hdc, data, sw, sh, shipR)
}

func afkExpeditionPreviewCacheChance(index, shipID int) int {
	chance := int(afkCacheChancePercent[afkScoutCacheLevel()]) + afkResearchCacheBonusPercent() + afkTalentCacheChanceBonusPercent()
	chance += afkExplorationPassive(afkValidExplorationShipID(shipID)).CacheChanceBonus
	if index >= 0 && index < len(afkExpeditions) {
		chance += afkExpeditions[index].BaseCacheBonus
	}
	if chance > 85 {
		chance = 85
	}
	if chance < 0 {
		chance = 0
	}
	return chance
}

func afkShipGuideEntryRect(w, hgt int32, slot int) RECT {
	p := afkExpeditionPanelRect(w, hgt)
	inner := RECT{p.Left + sx(34, w), p.Top + sy(102, hgt), p.Right - sx(34, w), p.Bottom - sy(44, hgt)}
	col := slot % 2
	row := slot / 2
	gapX := sx(16, w)
	gapY := sy(7, hgt)
	cw := (inner.Right - inner.Left - gapX) / 2
	ch := (inner.Bottom - inner.Top - gapY*5) / 6
	x := inner.Left + int32(col)*(cw+gapX)
	y := inner.Top + int32(row)*(ch+gapY)
	return RECT{x, y, x + cw, y + ch}
}

func afkRarityGuideStrength(rarity string) string {
	switch strings.ToUpper(strings.TrimSpace(rarity)) {
	case "CELESTIAL":
		return "MAXIMUM"
	case "NOVA":
		return "VERY HIGH"
	case "NEBULA":
		return "HIGH"
	case "ORBITAL":
		return "STANDARD"
	default:
		return "BASIC"
	}
}

func drawAFKShipGuide(hdc uintptr, w, hgt int32) {
	p := afkExpeditionPanelRect(w, hgt)
	face, light, dark := themedHUDPanelPalette(false)
	drawBevelPanel(hdc, p, blendColor(face, dark, 0.10), light, dark, 3)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 220, 72))
		centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(18, hgt), "EXPEDITION SHIP GUIDE")
		setTextColor.Call(hdc, blendColor(light, rgb(235, 245, 252), 0.40))
		centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(42, hgt), "EVERY ENDURANCE SHIP HAS AN EXPLORATION PASSIVE // HIGHER RARITY = STRONGER ABILITIES")
		setTextColor.Call(hdc, rgb(142, 190, 218))
		centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(64, hgt), "THE EQUIPPED SHIP IS LOCKED TO THE MISSION WHEN THE SCOUT LAUNCHES")
		selectObject.Call(hdc, old)
	}
	for slot, shipID := range garageShipOrder {
		r := afkShipGuideEntryRect(w, hgt, slot)
		cf, cl, cd := themedHUDCardPalette(1)
		equipped := shipID == afkValidExplorationShipID(gameMeta.SelectedShip)
		border := blendColor(cl, cd, 0.15)
		if equipped {
			border = blendColor(cl, rgb(255, 255, 255), 0.28)
		}
		drawBevelPanel(hdc, r, blendColor(cf, cd, 0.08), border, cd, 2)
		icon := RECT{r.Left + sx(7, w), r.Top + sy(5, hgt), r.Left + sx(57, w), r.Bottom - sy(5, hgt)}
		drawShipTextureFit(hdc, shipID, icon)
		passive := afkExplorationPassive(shipID)
		def := spaceShipDefs[shipID]
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(240, 247, 252))
			textOut(hdc, r.Left+sx(65, w), r.Top+sy(8, hgt), def.Name)
			setTextColor.Call(hdc, blendColor(cl, rgb(255, 225, 115), 0.38))
			textOut(hdc, r.Left+sx(65, w), r.Top+sy(20, hgt), fmt.Sprintf("%s // %s", def.Rarity, afkRarityGuideStrength(def.Rarity)))
			setTextColor.Call(hdc, rgb(132, 214, 244))
			textOut(hdc, r.Left+sx(65, w), r.Top+sy(35, hgt), passive.Name)
			setTextColor.Call(hdc, rgb(185, 205, 218))
			textOut(hdc, r.Left+sx(65, w), r.Top+sy(49, hgt), passive.Description)
			if equipped {
				setTextColor.Call(hdc, rgb(126, 238, 164))
				textOut(hdc, r.Right-sx(86, w), r.Top+sy(8, hgt), "EQUIPPED")
			}
			selectObject.Call(hdc, old)
		}
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(160, 192, 210))
		centeredTextOut(hdc, p.Left, p.Right, p.Bottom-sy(24, hgt), "CLICK ANYWHERE TO CLOSE SHIP GUIDE")
		selectObject.Call(hdc, old)
	}
}

func makeTintedAlphaBGRA(src []byte, sw, sh int32, r, g, b, alpha uint8) []byte {
	n := int(sw * sh * 4)
	if len(src) < n || n <= 0 {
		return nil
	}
	out := make([]byte, n)
	for i := 0; i < n; i += 4 {
		a := src[i+3]
		if a == 0 {
			continue
		}
		fa := uint16(a) * uint16(alpha) / 255
		out[i+0] = uint8(uint16(b) * fa / 255)
		out[i+1] = uint8(uint16(g) * fa / 255)
		out[i+2] = uint8(uint16(r) * fa / 255)
		out[i+3] = uint8(fa)
	}
	return out
}

func ensureStarCacheTraceBuffers() {
	starCacheTraceOnce.Do(func() {
		starCacheTraceDim = makeTintedAlphaBGRA(starCacheBGRA, 1536, 1024, 255, 190, 24, 95)
		starCacheTraceHot = makeTintedAlphaBGRA(starCacheBGRA, 1536, 1024, 255, 235, 90, 155)
	})
}

func drawAFKStarCachePreview(hdc uintptr, r RECT, w, hgt int32) {
	if len(starCacheBGRA) < 1536*1024*4 {
		return
	}
	// Star Cache is a small physical prop in the hangar, not a full-width preview.
	// Keep it beside the parked expedition ship and anchored close to the floor.
	flight := afkExpeditionFlightViewportRect(w, hgt)
	cacheW := sx(58, w)
	cacheH := int32(float64(cacheW) * 1024.0 / 1536.0)
	if cacheH < sy(24, hgt) {
		cacheH = sy(24, hgt)
	}
	cx := (flight.Left + flight.Right) / 2
	left := cx + sx(180, w)
	if left+cacheW > flight.Right-sx(10, w) {
		left = flight.Right - sx(10, w) - cacheW
	}
	bottom := flight.Bottom - sy(12, hgt)
	img := RECT{left, bottom - cacheH, left + cacheW, bottom}
	saved, _, _ := saveDC.Call(hdc)
	if saved != 0 {
		intersectClipRect.Call(hdc, uintptr(flight.Left), uintptr(flight.Top), uintptr(flight.Right), uintptr(flight.Bottom))
		defer restoreDC.Call(hdc, saved)
	}
	// Reward trace: no rectangular frame. Draw the cache itself several times at
	// small offsets using a yellow-tinted silhouette buffer, matching the facility
	// glow language while preserving the original sprite on top.
	pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixNano())/1e9*3.2)
	ensureStarCacheTraceBuffers()
	trace := starCacheTraceDim
	if pulse > 0.52 {
		trace = starCacheTraceHot
	}
	for _, off := range []struct{ x, y int32 }{
		{-sx(3, w), 0}, {sx(3, w), 0}, {0, -sy(3, hgt)}, {0, sy(3, hgt)},
		{-sx(2, w), -sy(2, hgt)}, {sx(2, w), -sy(2, hgt)}, {-sx(2, w), sy(2, hgt)}, {sx(2, w), sy(2, hgt)},
	} {
		g := RECT{img.Left + off.x, img.Top + off.y, img.Right + off.x, img.Bottom + off.y}
		drawRawBGRAFit(hdc, trace, 1536, 1024, g)
	}
	drawRawBGRAFit(hdc, starCacheBGRA, 1536, 1024, img)
}

func drawAFKExpeditionRewardPopup(hdc uintptr, w, hgt int32) {
	if !afkExpeditionRewardPending() {
		return
	}
	p := afkExpeditionPanelRect(w, hgt)
	r := afkExpeditionRewardPopupRect(w, hgt)
	overlaySolidAlphaRect(hdc, p, rgb(0, 4, 12), 190)
	drawBevelPanel(hdc, r, rgb(4, 18, 35), rgb(88, 210, 244), rgb(0, 4, 12), 4)
	drawOutlineRect(hdc, RECT{r.Left + sx(6, w), r.Top + sy(6, hgt), r.Right - sx(6, w), r.Bottom - sy(6, hgt)}, rgb(255, 206, 70), 1)

	idx := gameMeta.AFKExpeditionDestination
	mission := "SPACE EXPEDITION"
	if idx >= 0 && idx < len(afkExpeditions) {
		mission = afkExpeditions[idx].Name
	}
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 218, 79))
		centeredTextOut(hdc, r.Left+sx(20, w), r.Right-sx(20, w), r.Top+sy(22, hgt), "EXPEDITION COMPLETE")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(132, 203, 231))
		centeredTextOut(hdc, r.Left+sx(20, w), r.Right-sx(20, w), r.Top+sy(60, hgt), mission+" // RECOVERED CARGO")
		selectObject.Call(hdc, old)
	}

	gap := sx(12, w)
	innerLeft := r.Left + sx(24, w)
	innerRight := r.Right - sx(24, w)
	top := r.Top + sy(92, hgt)
	bottom := r.Bottom - sy(96, hgt)
	cardW := (innerRight - innerLeft - gap*2) / 3
	cards := [3]RECT{}
	for i := 0; i < 3; i++ {
		x := innerLeft + int32(i)*(cardW+gap)
		cards[i] = RECT{x, top, x + cardW, bottom}
	}

	// Starbits reward.
	drawBevelPanel(hdc, cards[0], rgb(7, 29, 52), rgb(255, 206, 70), rgb(0, 6, 15), 2)
	fillSolidRect(hdc, RECT{cards[0].Left + sx(18, w), cards[0].Top + sy(42, hgt), cards[0].Right - sx(18, w), cards[0].Top + sy(47, hgt)}, rgb(255, 196, 50))
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 213, 83))
		centeredTextOut(hdc, cards[0].Left, cards[0].Right, cards[0].Top+sy(15, hgt), "STARBITS")
		setTextColor.Call(hdc, rgb(246, 250, 252))
		centeredTextOut(hdc, cards[0].Left+sx(5, w), cards[0].Right-sx(5, w), cards[0].Top+sy(72, hgt), "+"+formatAFKNumber(gameMeta.AFKExpeditionPendingStarbits))
		selectObject.Call(hdc, old)
	}

	// Navigation Data reward.
	drawBevelPanel(hdc, cards[1], rgb(7, 29, 52), rgb(86, 197, 233), rgb(0, 6, 15), 2)
	fillSolidRect(hdc, RECT{cards[1].Left + sx(18, w), cards[1].Top + sy(42, hgt), cards[1].Right - sx(18, w), cards[1].Top + sy(47, hgt)}, rgb(86, 197, 233))
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(119, 220, 249))
		centeredTextOut(hdc, cards[1].Left, cards[1].Right, cards[1].Top+sy(15, hgt), "NAV DATA")
		setTextColor.Call(hdc, rgb(246, 250, 252))
		centeredTextOut(hdc, cards[1].Left+sx(5, w), cards[1].Right-sx(5, w), cards[1].Top+sy(72, hgt), fmt.Sprintf("+%d", gameMeta.AFKExpeditionPendingData))
		selectObject.Call(hdc, old)
	}

	// Ship Module crafting material reward, with the real supplied icon.
	matIndex, matCount, hasMaterial := afkPendingExpeditionCraftComponent()
	matBorder := rgb(74, 94, 118)
	matName := "SHIP MATERIALS LOCKED"
	matRarity := "COMPLETE OPERATOR DEVELOPMENT"
	if hasMaterial && matIndex >= 0 && matIndex < len(afkCraftComponentDefs) {
		matBorder = afkCraftComponentDefs[matIndex].Accent
		matName = fmt.Sprintf("%s x%d", afkCraftComponentDefs[matIndex].Name, matCount)
		matRarity = afkCraftComponentDefs[matIndex].Rarity + " MATERIAL"
	}
	drawBevelPanel(hdc, cards[2], rgb(7, 29, 52), matBorder, rgb(0, 6, 15), 2)
	if hasMaterial {
		icon := RECT{cards[2].Left + sx(46, w), cards[2].Top + sy(35, hgt), cards[2].Right - sx(46, w), cards[2].Bottom - sy(50, hgt)}
		drawAFKCraftMaterialSprite(hdc, icon, matIndex, 255)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, matBorder)
		centeredTextOut(hdc, cards[2].Left+sx(4, w), cards[2].Right-sx(4, w), cards[2].Top+sy(15, hgt), "SHIP MATERIAL")
		setTextColor.Call(hdc, rgb(246, 250, 252))
		centeredTextOut(hdc, cards[2].Left+sx(5, w), cards[2].Right-sx(5, w), cards[2].Bottom-sy(38, hgt), fitTextEllipsis(hdc, hudTinyFont, matName, cards[2].Right-cards[2].Left-sx(10, w)))
		setTextColor.Call(hdc, rgb(138, 164, 186))
		centeredTextOut(hdc, cards[2].Left+sx(5, w), cards[2].Right-sx(5, w), cards[2].Bottom-sy(18, hgt), fitTextEllipsis(hdc, hudTinyFont, matRarity, cards[2].Right-cards[2].Left-sx(10, w)))
		selectObject.Call(hdc, old)
	}

	if afkSpaceCachePending() && hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(210, 128, 255))
		centeredTextOut(hdc, r.Left+sx(30, w), r.Right-sx(30, w), r.Bottom-sy(86, hgt), "BONUS // STAR CACHE RECOVERED // OPEN AFTER CLAIMING CARGO")
		selectObject.Call(hdc, old)
	}
	claim := afkExpeditionRewardPopupClaimRect(w, hgt)
	drawAFKActionButton(hdc, claim, "CLAIM ALL REWARDS", 0, true, pointInRect(cursorPos, claim))
}

func drawAFKExpeditionLayer(hdc uintptr, w, hgt int32) {
	if afkCoreTier() < 2 {
		return
	}
	if !afkExpeditionPanelOpen {
		return
	}

	afkRefreshExpeditionCompletion(time.Now())
	p := afkExpeditionPanelRect(w, hgt)
	drawAFKManagementChrome(hdc, p, "SPACE EXPEDITIONS", "CHOOSE A ROUTE // UPGRADE THE SCOUT // CLAIM REWARDS FROM ONE SCREEN", rgb(86, 197, 233), w, hgt)
	if !afkScoutUnlocked() {
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(195, 218, 232))
			centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(92, hgt), "CURSOR CORE TIER 2 HAS OPENED THE EXPEDITION LINK")
			centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(118, hgt), "ASSEMBLE THE FIRST HARD-CODED SCOUT SHIP")
			setTextColor.Call(hdc, rgb(255, 214, 73))
			centeredTextOut(hdc, p.Left, p.Right, p.Top+sy(153, hgt), fmt.Sprintf("REQUIRES %s STARBITS", formatAFKNumber(afkScoutShipUnlockCost)))
			selectObject.Call(hdc, old)
		}
		b := afkScoutUnlockRect(w, hgt)
		drawAFKActionButton(hdc, b, "BUILD SCOUT SHIP", afkScoutShipUnlockCost, gameMeta.AFKStarbits >= afkScoutShipUnlockCost, pointInRect(cursorPos, b))
		return
	}

	left, right := afkExpeditionColumns(w, hgt)
	baseFace, baseLight, baseDark := themedHUDPanelPalette(false)
	drawBevelPanel(hdc, left, blendColor(baseFace, baseDark, 0.18), blendColor(baseLight, rgb(49, 91, 132), 0.28), baseDark, 2)
	drawBevelPanel(hdc, right, blendColor(baseFace, baseDark, 0.18), blendColor(baseLight, rgb(115, 91, 205), 0.28), baseDark, 2)

	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, blendColor(baseLight, rgb(235, 247, 252), 0.35))
		centeredTextOut(hdc, left.Left, left.Right, left.Top+sy(10, hgt), "SELECT DESTINATION // 12 ROUTES // PAID ROUTES COST STARBITS")
		selectObject.Call(hdc, old)
	}
	clampAFKExpeditionScroll(w, hgt)
	missionViewport := afkExpeditionMissionViewportRect(w, hgt)
	missionSaved, _, _ := saveDC.Call(hdc)
	if missionSaved != 0 {
		intersectClipRect.Call(hdc, uintptr(missionViewport.Left), uintptr(missionViewport.Top), uintptr(missionViewport.Right), uintptr(missionViewport.Bottom))
	}
	for i, d := range afkExpeditions {
		r := afkExpeditionDestinationRect(w, hgt, i)
		unlocked := afkExpeditionAvailable(i)
		selected := i == afkSelectedExpeditionIndex()
		affordable := d.LaunchCost <= 0 || gameMeta.AFKStarbits >= d.LaunchCost
		cardFace, cardLight, cardDark := themedHUDCardPalette(1)
		fill := blendColor(cardFace, cardDark, 0.08)
		border := blendColor(cardLight, cardDark, 0.18)
		if selected {
			fill = blendColor(cardFace, cardLight, 0.10)
			border = blendColor(cardLight, rgb(255, 255, 255), 0.25)
		}
		if !unlocked || !affordable {
			fill = blendColor(cardFace, cardDark, 0.42)
			border = blendColor(cardLight, cardDark, 0.58)
		}
		drawBevelPanel(hdc, r, fill, border, cardDark, 2)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			if unlocked {
				setTextColor.Call(hdc, rgb(238, 247, 252))
			} else {
				setTextColor.Call(hdc, rgb(100, 110, 120))
			}
			innerW := r.Right - r.Left - sx(16, w)
			textOut(hdc, r.Left+sx(8, w), r.Top+sy(8, hgt), fitTextEllipsis(hdc, hudTinyFont, d.Name, innerW))
			setTextColor.Call(hdc, blendColor(cardLight, rgb(214, 230, 240), 0.46))
			textOut(hdc, r.Left+sx(8, w), r.Top+sy(29, hgt), fitTextEllipsis(hdc, hudTinyFont, d.Subtitle, innerW))
			if !unlocked {
				setTextColor.Call(hdc, rgb(115, 125, 135))
				lockText := fmt.Sprintf("LOCKED // REQUIRES SCOUT TIER %d", d.RequiredShipTier)
				textOut(hdc, r.Left+sx(8, w), r.Top+sy(59, hgt), fitTextEllipsis(hdc, hudTinyFont, lockText, innerW))
			} else {
				star, nav := afkExpeditionPreviewRewards(i, gameMeta.SelectedShip)
				setTextColor.Call(hdc, rgb(152, 202, 226))
				stats := fmt.Sprintf("%s // NAV +%d // STAR CACHE %d%%", afkFormatDuration(afkExpeditionDurationSeconds(i)), nav, afkExpeditionPreviewCacheChance(i, gameMeta.SelectedShip))
				textOut(hdc, r.Left+sx(8, w), r.Top+sy(52, hgt), fitTextEllipsis(hdc, hudTinyFont, stats, innerW))
				if d.LaunchCost > 0 {
					if affordable {
						setTextColor.Call(hdc, rgb(255, 211, 76))
					} else {
						setTextColor.Call(hdc, rgb(230, 108, 108))
					}
					costReward := fmt.Sprintf("COST %s STARBITS // REWARD %s", formatAFKNumber(d.LaunchCost), formatAFKNumber(star))
					textOut(hdc, r.Left+sx(8, w), r.Top+sy(73, hgt), fitTextEllipsis(hdc, hudTinyFont, costReward, innerW))
				} else {
					setTextColor.Call(hdc, rgb(126, 230, 164))
					reward := fmt.Sprintf("FREE LAUNCH // REWARD %s STARBITS", formatAFKNumber(star))
					textOut(hdc, r.Left+sx(8, w), r.Top+sy(73, hgt), fitTextEllipsis(hdc, hudTinyFont, reward, innerW))
				}
			}
			selectObject.Call(hdc, old)
		}
	}

	if missionSaved != 0 {
		restoreDC.Call(hdc, missionSaved)
	}
	track, thumb := afkExpeditionScrollbarRects(w, hgt)
	fillSolidRect(hdc, track, blendColor(baseDark, rgb(0, 0, 0), 0.18))
	if afkExpeditionMaxScroll(w, hgt) > 0 {
		drawBevelPanel(hdc, thumb, blendColor(baseFace, baseLight, 0.18), baseLight, baseDark, 1)
	}

	drawAFKExpeditionFlightBackground(hdc, w, hgt)
	drawAFKScoutShip(hdc, right, w, hgt)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(244, 249, 252))
		shipID := afkCurrentExplorationShipID()
		passive := afkExplorationPassive(shipID)
		shipName := spaceShipDefs[shipID].Name
		centeredTextOut(hdc, right.Left, right.Right, right.Top+sy(116, hgt), fmt.Sprintf("%s // SCOUT TIER %d", shipName, afkScoutTier()))
		setTextColor.Call(hdc, rgb(128, 220, 255))
		centeredTextOut(hdc, right.Left, right.Right, right.Top+sy(136, hgt), fmt.Sprintf("%s // %s", passive.Name, passive.Description))
		setTextColor.Call(hdc, rgb(242, 192, 55))
		centeredTextOut(hdc, right.Left, right.Right, right.Top+sy(154, hgt), fmt.Sprintf("NAV DATA: %d", gameMeta.AFKNavigationData))
		selectObject.Call(hdc, old)
	}
	guideBtn := afkShipGuideRect(w, hgt)
	drawAFKActionButton(hdc, guideBtn, "SHIP GUIDE", 0, true, pointInRect(cursorPos, guideBtn))
	if afkScoutTier() < afkScoutMaxTier {
		cost, req := afkNextScoutTierCost()
		b := afkScoutTierRect(w, hgt)
		label := fmt.Sprintf("TIER UP // %d DATA + %d RUNS", cost, req)
		drawAFKActionButton(hdc, b, label, 0, afkCanUpgradeScoutTier(), pointInRect(cursorPos, b))
	}
	labels := []string{"EXPEDITION SPEED", "SCOUT SYSTEMS", "CACHE CHANCE"}
	vals := []string{fmt.Sprintf("LV.%d // %d%% TIME", afkScoutSpeedLevel(), afkExpeditionSpeedPercent[afkScoutSpeedLevel()]), fmt.Sprintf("LV.%d // %s", afkScoutCapacityLevel(), afkFormatDuration(afkSection5CapacitySeconds())), fmt.Sprintf("LV.%d // %d%%", afkScoutCacheLevel(), afkCacheChancePercent[afkScoutCacheLevel()])}
	for k := 0; k < 3; k++ {
		r := afkScoutStatRect(w, hgt, k)
		c := afkScoutStatUpgradeCost(k)
		text := fmt.Sprintf("%s  %s", labels[k], vals[k])
		if c > 0 {
			text += fmt.Sprintf("  //  %d DATA", c)
		} else {
			text += "  //  MAX"
		}
		drawAFKActionButton(hdc, r, text, 0, c > 0 && gameMeta.AFKNavigationData >= c, pointInRect(cursorPos, r))
	}

	if afkSpaceCachePending() {
		drawAFKStarCachePreview(hdc, right, w, hgt)
	}
	action := afkExpeditionLaunchRect(w, hgt)
	switch {
	case afkSpaceCachePending() && !afkExpeditionRewardPending():
		starbits, rar := afkCacheReward(gameMeta.AFKSpaceCacheRarity)
		drawAFKActionButton(hdc, action, fmt.Sprintf("OPEN %s STAR CACHE // +%s STARBITS", rar, formatAFKNumber(starbits)), 0, true, pointInRect(cursorPos, action))
	case !afkExpeditionRewardPending() && afkModulesAvailable() && gameMeta.AFKPendingComponentCount > 0:
		d := afkCraftComponentDefs[gameMeta.AFKPendingComponent]
		drawAFKActionButton(hdc, action, fmt.Sprintf("CONGRATS! %s x%d // %s CRAFTING COMPONENT", d.Name, gameMeta.AFKPendingComponentCount, d.Rarity), 0, true, pointInRect(cursorPos, action))
	case afkExpeditionActive():
		remain := gameMeta.AFKExpeditionEndsUnix - time.Now().Unix()
		if remain < 0 {
			remain = 0
		}
		drawAFKActionButton(hdc, action, fmt.Sprintf("SCOUT IN FLIGHT // ETA %s", afkFormatDuration(remain)), 0, false, false)
	default:
		if !afkExpeditionRewardPending() {
			idx := afkSelectedExpeditionIndex()
			d := afkExpeditions[idx]
			starbits, nav := afkExpeditionPreviewRewards(idx, gameMeta.SelectedShip)
			canLaunch := d.LaunchCost <= 0 || gameMeta.AFKStarbits >= d.LaunchCost
			costText := "FREE"
			if d.LaunchCost > 0 {
				costText = formatAFKNumber(d.LaunchCost) + " STARBITS"
			}
			drawAFKActionButton(hdc, action, fmt.Sprintf("LAUNCH %s // COST %s // +%s STARBITS // +%d NAV", d.Name, costText, formatAFKNumber(starbits), nav), 0, canLaunch, pointInRect(cursorPos, action))
		}
	}
	if afkExpeditionRewardPending() {
		drawAFKExpeditionRewardPopup(hdc, w, hgt)
	}
	if afkShipGuideOpen && !afkExpeditionRewardPending() {
		drawAFKShipGuide(hdc, w, hgt)
	}
}

func handleAFKExpeditionWheel(delta int32, w, hgt int32) bool {
	if !afkExpeditionPanelOpen || !afkScoutUnlocked() || afkShipGuideOpen || afkExpeditionRewardPending() {
		return false
	}
	step := sy(104, hgt)
	if delta < 0 {
		afkExpeditionScroll += step
	} else if delta > 0 {
		afkExpeditionScroll -= step
	}
	clampAFKExpeditionScroll(w, hgt)
	return true
}

func handleAFKExpeditionClick(h uintptr, p FPoint, w, hgt int32) bool {
	if afkCoreTier() < 2 {
		return false
	}
	if !afkExpeditionPanelOpen {
		return false
	}
	if !afkScoutUnlocked() {
		if pointInRect(p, afkScoutUnlockRect(w, hgt)) && gameMeta.AFKStarbits >= afkScoutShipUnlockCost && afkSpendStarbits(afkScoutShipUnlockCost) {
			gameMeta.AFKScoutShipUnlocked = true
			gameMeta.AFKScoutShipTier = 1
			gameMeta.AFKExpeditionSpeedLevel = 1
			gameMeta.AFKCapacityLevel = 1
			gameMeta.AFKCacheChanceLevel = 1
			saveGameMeta()
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
		}
		return true
	}
	if afkExpeditionRewardPending() {
		if pointInRect(p, afkExpeditionRewardPopupClaimRect(w, hgt)) {
			if afkClaimExpeditionReward() {
				saveGameMeta()
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
			}
			return true
		}
		// Reward popup is modal: do not let clicks leak into mission/upgrade controls.
		return pointInRect(p, afkExpeditionPanelRect(w, hgt))
	}
	track, thumb := afkExpeditionScrollbarRects(w, hgt)
	if afkExpeditionMaxScroll(w, hgt) > 0 && pointInRect(p, thumb) {
		afkExpeditionScrollDragging = true
		afkExpeditionScrollDragOffset = int32(p.Y) - thumb.Top
		setCapture.Call(h)
		return true
	}
	if afkExpeditionMaxScroll(w, hgt) > 0 && pointInRect(p, track) {
		afkExpeditionScrollbarDragTo(int32(p.Y)-(thumb.Bottom-thumb.Top)/2, w, hgt)
		invalidateRect.Call(h, 0, 0)
		return true
	}
	if afkShipGuideOpen {
		if pointInRect(p, afkShipGuideRect(w, hgt)) || pointInRect(p, afkExpeditionPanelRect(w, hgt)) {
			afkShipGuideOpen = false
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	if pointInRect(p, afkShipGuideRect(w, hgt)) {
		afkShipGuideOpen = true
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	missionViewport := afkExpeditionMissionViewportRect(w, hgt)
	for i := range afkExpeditions {
		if pointInRect(p, missionViewport) && pointInRect(p, afkExpeditionDestinationRect(w, hgt, i)) && afkExpeditionAvailable(i) && !afkExpeditionActive() && !afkExpeditionRewardPending() && !afkSpaceCachePending() && gameMeta.AFKPendingComponentCount == 0 {
			afkExpeditionSelected = i
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	if pointInRect(p, afkScoutTierRect(w, hgt)) && afkUpgradeScoutTier() {
		saveGameMeta()
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	for k := 0; k < 3; k++ {
		if pointInRect(p, afkScoutStatRect(w, hgt, k)) {
			if afkUpgradeScoutStat(k) {
				saveGameMeta()
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
			}
			return true
		}
	}
	if pointInRect(p, afkExpeditionLaunchRect(w, hgt)) {
		changed := false
		switch {
		case afkExpeditionRewardPending():
			changed = afkClaimExpeditionReward()
		case afkSpaceCachePending():
			changed = afkClaimSpaceCache()
		case afkModulesAvailable() && gameMeta.AFKPendingComponentCount > 0:
			changed = afkClaimCraftComponent()
		case !afkExpeditionActive():
			changed = afkLaunchExpedition(afkSelectedExpeditionIndex(), time.Now())
		}
		if changed {
			saveGameMeta()
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
		}
		return true
	}
	return pointInRect(p, afkExpeditionPanelRect(w, hgt))
}
