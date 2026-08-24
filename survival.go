//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"time"
	"unsafe"
)

type SurvivalEnemy struct {
	X, Y          float64
	VX, VY        float64
	Red           bool
	Large         bool
	HP            int
	SpawnedAt     time.Time
	Reacted       bool // reaction time is sampled once, on the first valid click
	Critical      bool // large ships expose a small precision core for bonus score
	Elite         int  // 0 normal, 1 armoured elite, 2 striker elite
	Kind          int  // 0 Scout, 1 Heavy, 2-9 expansion roster
	Special       int
	SpecialTimer  time.Time
	HazardLatched bool
}

type SurvivalPickup struct {
	X, Y      float64
	Kind      int
	SpawnedAt time.Time
}

type SurvivalOrbital struct {
	Kind       int
	Level      int
	Angle      float64
	CooldownTo time.Time
}

type SurvivalElimination struct {
	X, Y    float64
	Red     bool
	Large   bool
	Started time.Time
}

type SurvivalArcFX struct {
	X1, Y1, X2, Y2 float64
	Started        time.Time
}

type survivalPenKey struct {
	Width int32
	Color uintptr
}

var survivalPenCache = map[survivalPenKey]uintptr{}

func survivalPen(width int32, color uintptr) uintptr {
	if width < 1 {
		width = 1
	}
	key := survivalPenKey{Width: width, Color: color}
	if p := survivalPenCache[key]; p != 0 {
		return p
	}
	p, _, _ := createPen.Call(PS_SOLID, uintptr(width), color)
	if p != 0 {
		survivalPenCache[key] = p
	}
	return p
}

func releaseSurvivalGDIResources() {
	for key, p := range survivalPenCache {
		if p != 0 {
			deleteObject.Call(p)
		}
		delete(survivalPenCache, key)
	}
}

func drawSurvivalLineWithPen(hdc, pen uintptr, x1, y1, x2, y2 int32) {
	if hdc == 0 || pen == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, pen)
	moveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
	lineTo.Call(hdc, uintptr(x2), uintptr(y2))
	selectObject.Call(hdc, old)
}

func drawSurvivalCircleWithPen(hdc, pen uintptr, cx, cy, radius int32) {
	if hdc == 0 || pen == 0 || radius <= 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, pen)
	const seg = 56
	for i := 0; i <= seg; i++ {
		a := float64(i) / seg * 2 * math.Pi
		x := cx + int32(math.Cos(a)*float64(radius))
		y := cy + int32(math.Sin(a)*float64(radius))
		if i == 0 {
			moveToEx.Call(hdc, uintptr(x), uintptr(y), 0)
		} else {
			lineTo.Call(hdc, uintptr(x), uintptr(y))
		}
	}
	selectObject.Call(hdc, old)
}

const (
	survivalPickupShield = iota
	survivalPickupSlow
	survivalPickupRepair
	survivalPickupBomb
	survivalPickupChain
)

const (
	survivalWaveNormal = iota
	survivalWaveSwarm
	survivalWaveHeavy
	survivalWaveReaction
	survivalWaveCrossfire
	survivalWavePowerSurge
	survivalWaveNoSupport
	survivalWaveOverclocked
	survivalWaveMeteorDrift
)

func survivalWaveTypeName(t int) string {
	switch t {
	case survivalWaveSwarm:
		return "SWARM DETECTED"
	case survivalWaveHeavy:
		return "HEAVY ASSAULT"
	case survivalWaveReaction:
		return "REACTION WAVE"
	case survivalWaveCrossfire:
		return "CROSSFIRE"
	case survivalWavePowerSurge:
		return "POWER SURGE"
	case survivalWaveNoSupport:
		return "NO SUPPORT // BONUS SCORE"
	case survivalWaveOverclocked:
		return "OVERCLOCKED"
	case survivalWaveMeteorDrift:
		return "METEOR DRIFT // KEEP MOVING"
	default:
		return ""
	}
}

func survivalChooseWaveType(wave int) int {
	if wave < 3 || wave%10 == 0 {
		return survivalWaveNormal
	}
	// Sector 3 introduces movement pressure without replacing the existing click
	// language: intermittent meteor-drift waves require cursor relocation while
	// standard red/blue energy enemies keep their colour input rules; special enemies are neutral.
	if wave >= 23 && wave%5 == 3 {
		return survivalWaveMeteorDrift
	}
	switch wave % 7 {
	case 0:
		return survivalWaveSwarm
	case 1:
		return survivalWaveHeavy
	case 2:
		return survivalWaveReaction
	case 3:
		return survivalWaveCrossfire
	case 4:
		return survivalWavePowerSurge
	case 5:
		return survivalWaveNoSupport
	default:
		return survivalWaveOverclocked
	}
}

var (
	survivalEnemies         []SurvivalEnemy
	survivalPickup          *SurvivalPickup
	survivalPickup2         *SurvivalPickup
	survivalOrbitals        = map[int]*SurvivalOrbital{}
	survivalShieldCharges   int
	survivalChainCharges    int
	survivalSlowUntil       time.Time
	survivalEliminations    []SurvivalElimination
	survivalArcFX           []SurvivalArcFX
	survivalWaveBannerUntil time.Time
	survivalWaveBannerText  string

	survivalHP               = 6
	survivalWave             = 1
	survivalCheckpoint       = 1
	survivalKills            int
	survivalWaveKills        int
	survivalWaveBudget       int
	survivalWaveSpawned      int
	survivalNextSpawn        time.Time
	survivalWaveBreakUntil   time.Time
	survivalNextPickupAt     time.Time
	survivalStartedAt        time.Time
	survivalLastTick         time.Time
	survivalLastValidClick   time.Time
	survivalReactionSum      float64
	survivalReactionSamples  int
	survivalPerfectHits      int
	survivalTotalHits        int
	survivalCombo            int
	survivalBestCombo        int
	survivalLastKillAt       time.Time
	survivalPerfectWaves     int
	survivalPrecisionKills   int
	survivalEliteKills       int
	survivalScoreEXPClaimed  int
	survivalBossEXPClaimed   int
	survivalLastEXPAward     int
	survivalWaveDamageStart  int
	survivalWaveDamageTaken  bool
	survivalTotalDamageTaken int
	survivalWaveShieldStart  int
	survivalWaveShieldUsed   int
	survivalWaveType         int
	survivalFormationIndex   int

	survivalModeButtonBGRA      []byte
	survivalStationPerfectBGRA  []byte
	survivalStationDamagedBGRA  []byte
	survivalStationHeavyBGRA    []byte
	survivalStationCriticalBGRA []byte
	survivalHPActiveBGRA        []byte
	survivalHPEmptyBGRA         []byte
	survivalSmallRed            [16][]byte
	survivalSmallBlue           [16][]byte
	survivalBigRed              [16][]byte
	survivalBigBlue             [16][]byte
	survivalShieldBGRA          []byte
	survivalSlowBGRA            []byte
	survivalRepairBGRA          []byte
	survivalBombBGRA            []byte
	survivalChainBGRA           []byte
	survivalBackgrounds         [3][]byte
	survivalFailedBGRA          []byte

	survivalMusicReady                 bool
	survivalSelectedStartWave          int
	survivalSelectedStartBoss          int
	survivalLockedSectionPopup         int
	survivalCheckpointScroll           int32
	survivalCheckpointScrollDragging   bool
	survivalCheckpointScrollDragOffset int32
	survivalSection2MusicReady         bool
	survivalSection3MusicReady         bool
	survivalRenderPrewarmed            bool
	survivalNextBackgroundPrewarmed    int
)

func survivalActive() bool { return gameMode == 2 }

const (
	survivalBossFightSpeed = 1.40
	survivalBoss3Wave      = 31
	survivalPostBoss3Wave  = 32
)

func survivalBossFightNow(realNow, startedAt time.Time) time.Time {
	if startedAt.IsZero() || !realNow.After(startedAt) {
		return realNow
	}
	elapsed := realNow.Sub(startedAt)
	return startedAt.Add(time.Duration(float64(elapsed) * survivalBossFightSpeed))
}

// survivalDisplayWave keeps the waiting-screen preview aligned with the player's
// highest boss-earned checkpoint. Cloud/profile sync can update gameMeta after the
// mode was first drawn, so presentation must not rely on the package default W1.
func survivalDisplayWave() int {
	wave := survivalWave
	if state == StateWaiting {
		if survivalSelectedStartWave > 0 {
			wave = survivalSelectedStartWave
		}
		checkpoint := gameMeta.SurvivalCheckpoint
		if survivalCheckpoint > checkpoint {
			checkpoint = survivalCheckpoint
		}
		if checkpoint == 30 {
			checkpoint = survivalBoss3Wave
		}
		if checkpoint < 1 {
			checkpoint = 1
		}
		if survivalSelectedStartWave <= 0 && checkpoint > wave {
			wave = checkpoint
		}
	}
	if wave < 1 {
		wave = 1
	}
	return wave
}

func survivalSectionStartWave(section int) int {
	switch section {
	case 2:
		return 11
	case 3:
		return 21
	default:
		return 1
	}
}

func survivalSectionUnlocked(section int) bool {
	if section <= 1 {
		return true
	}
	checkpoint := gameMeta.SurvivalCheckpoint
	if survivalCheckpoint > checkpoint {
		checkpoint = survivalCheckpoint
	}
	if section == 2 {
		return checkpoint >= 11
	}
	if section == 3 {
		return checkpoint >= 21
	}
	return false
}

type survivalReplayCheckpoint struct {
	Label    string
	Wave     int
	Boss     int
	UnlockAt int
}

var survivalReplayCheckpoints = [...]survivalReplayCheckpoint{
	{Label: "SECTOR 1", Wave: 1, UnlockAt: 1},
	{Label: "SBOSS 1", Wave: 10, Boss: 1, UnlockAt: 10},
	{Label: "SECTOR 2", Wave: 11, UnlockAt: 11},
	{Label: "SBOSS 2", Wave: 20, Boss: 2, UnlockAt: 20},
	{Label: "SECTOR 3", Wave: 21, UnlockAt: 21},
	{Label: "SBOSS 3", Wave: survivalBoss3Wave, Boss: 3, UnlockAt: survivalBoss3Wave},
}

func survivalReplayCheckpointValue() int {
	cp := gameMeta.SurvivalCheckpoint
	if survivalCheckpoint > cp {
		cp = survivalCheckpoint
	}
	// v404 migration: v403 stored Boss 3 entry as CP30. Boss 3 is now its own
	// dedicated Wave 31, so treat that legacy value as the new boss checkpoint.
	if cp == 30 {
		cp = survivalBoss3Wave
	}
	if cp < 1 {
		cp = 1
	}
	return cp
}

func survivalReplayCheckpointUnlocked(index int) bool {
	if index < 0 || index >= len(survivalReplayCheckpoints) {
		return false
	}
	return survivalReplayCheckpointValue() >= survivalReplayCheckpoints[index].UnlockAt
}

func survivalCheckpointViewportRect(w, h int32) RECT {
	return RECT{sx(28, w), sy(966, h), sx(444, w), sy(1006, h)}
}

func survivalCheckpointContentWidth(w int32) int32 {
	bw := sx(118, w)
	gap := sx(8, w)
	return int32(len(survivalReplayCheckpoints))*bw + int32(len(survivalReplayCheckpoints)-1)*gap
}

func survivalCheckpointMaxScroll(w, h int32) int32 {
	v := survivalCheckpointViewportRect(w, h)
	m := survivalCheckpointContentWidth(w) - (v.Right - v.Left)
	if m < 0 {
		return 0
	}
	return m
}

func clampSurvivalCheckpointScroll(w, h int32) {
	max := survivalCheckpointMaxScroll(w, h)
	if survivalCheckpointScroll < 0 {
		survivalCheckpointScroll = 0
	}
	if survivalCheckpointScroll > max {
		survivalCheckpointScroll = max
	}
}

func survivalCheckpointButtonRects(w, h int32) [6]RECT {
	v := survivalCheckpointViewportRect(w, h)
	bw := sx(118, w)
	gap := sx(8, w)
	var out [6]RECT
	for i := range out {
		x := v.Left + int32(i)*(bw+gap) - survivalCheckpointScroll
		out[i] = RECT{x, v.Top, x + bw, v.Bottom}
	}
	return out
}

func survivalCheckpointScrollbarRects(w, h int32) (track, thumb RECT) {
	v := survivalCheckpointViewportRect(w, h)
	track = RECT{v.Left, sy(1010, h), v.Right, sy(1019, h)}
	max := survivalCheckpointMaxScroll(w, h)
	if max <= 0 {
		return track, track
	}
	viewW := v.Right - v.Left
	contentW := survivalCheckpointContentWidth(w)
	thumbW := int32(float64(viewW) * float64(viewW) / float64(contentW))
	if thumbW < sx(70, w) {
		thumbW = sx(70, w)
	}
	travel := viewW - thumbW
	x := track.Left + int32(float64(travel)*float64(survivalCheckpointScroll)/float64(max))
	thumb = RECT{x, track.Top, x + thumbW, track.Bottom}
	return track, thumb
}

func survivalCheckpointScrollbarDragTo(left int32, w, h int32) {
	track, thumb := survivalCheckpointScrollbarRects(w, h)
	travel := (track.Right - track.Left) - (thumb.Right - thumb.Left)
	max := survivalCheckpointMaxScroll(w, h)
	if travel <= 0 || max <= 0 {
		survivalCheckpointScroll = 0
		return
	}
	pos := left - track.Left
	if pos < 0 {
		pos = 0
	}
	if pos > travel {
		pos = travel
	}
	survivalCheckpointScroll = int32(float64(max) * float64(pos) / float64(travel))
	clampSurvivalCheckpointScroll(w, h)
}

func selectSurvivalReplayCheckpoint(h uintptr, index int) {
	if index < 0 || index >= len(survivalReplayCheckpoints) {
		return
	}
	cp := survivalReplayCheckpoints[index]
	playUIButtonClickSound()
	if !survivalReplayCheckpointUnlocked(index) {
		// Keep the existing lock modal, but identify the requested sector/boss slot.
		survivalLockedSectionPopup = index + 1
		setOverlay(OverlaySurvivalSectionLocked)
		status = cp.Label + " LOCKED"
		invalidateRect.Call(h, 0, 0)
		return
	}

	survivalSelectedStartWave = cp.Wave
	survivalSelectedStartBoss = cp.Boss
	survivalWave = cp.Wave
	status = "SURVIVAL // " + cp.Label + " SELECTED"

	// Sector previews use their native music. Boss checkpoints remain silent until
	// START, where the shared dossier cinematic owns the full audio handoff.
	if audioReady && survivalActive() && state == StateWaiting && cp.Boss == 0 {
		switchModeMusic()
	}
	invalidateRect.Call(h, 0, 0)
}

// Legacy helpers remain available to older call sites.
func survivalSectionSelectorRects(w, h int32) [3]RECT {
	buttons := survivalCheckpointButtonRects(w, h)
	return [3]RECT{buttons[0], buttons[2], buttons[4]}
}

func selectSurvivalSection(h uintptr, section int) {
	index := (section - 1) * 2
	selectSurvivalReplayCheckpoint(h, index)
}

func initSurvivalAssets() {
	survivalModeButtonBGRA = readExternalBytes("ui", "survival_mode_button.bgra")
	survivalFailedBGRA = readExternalBytes("ui", "failed_survival.bgra")
	survivalStationPerfectBGRA = readExternalBytes("survival", "station_perfect.bgra")
	survivalStationDamagedBGRA = readExternalBytes("survival", "station_damaged.bgra")
	survivalStationHeavyBGRA = readExternalBytes("survival", "station_heavy.bgra")
	survivalStationCriticalBGRA = readExternalBytes("survival", "station_heavy_critical.bgra")
	survivalHPActiveBGRA = readExternalBytes("survival", "hp_active.bgra")
	survivalHPEmptyBGRA = readExternalBytes("survival", "hp_empty.bgra")
	survivalShieldBGRA = readExternalBytes("survival", "powerup_shield.bgra")
	survivalSlowBGRA = readExternalBytes("survival", "powerup_slow.bgra")
	survivalRepairBGRA = readExternalBytes("survival", "powerup_repair.bgra")
	survivalBombBGRA = readExternalBytes("survival", "powerup_bomb.bgra")
	survivalChainBGRA = readExternalBytes("survival", "powerup_chain.bgra")
	for i := 0; i < 16; i++ {
		survivalSmallRed[i] = readExternalBytes("survival", "enemies", fmt.Sprintf("small_red_%02d.bgra", i))
		survivalSmallBlue[i] = readExternalBytes("survival", "enemies", fmt.Sprintf("small_blue_%02d.bgra", i))
		survivalBigRed[i] = readExternalBytes("survival", "enemies", fmt.Sprintf("big_red_%02d.bgra", i))
		survivalBigBlue[i] = readExternalBytes("survival", "enemies", fmt.Sprintf("big_blue_%02d.bgra", i))
	}
	for i := 0; i < 3; i++ {
		survivalBackgrounds[i] = readExternalBytes("survival", fmt.Sprintf("background_%d.bgra", i))
	}
	initSurvivalExpansionAssets()
	initSurvivalBoss1Assets()
	initSurvivalBoss2Assets()
}

func survivalArenaCenter(w, h int32) FPoint {
	r := arenaRect(w, h)
	return FPoint{float64(r.Left+r.Right) / 2, float64(r.Top+r.Bottom) / 2}
}

func survivalImpactRadius(w, h int32) float64  { return float64(sx(92, w)) }
func survivalOrbitalRadius(w, h int32) float64 { return float64(sx(176, w)) }

// Visual and collision radii are kept separate. The collision radius is deliberately
// a little more forgiving than the visible body so fast flicks at an antialiased edge
// still count as a clean hit.
func survivalEnemyVisualRadius(e SurvivalEnemy, w int32) float64 {
	if r, ok := survivalExpansionVisualRadius(e, w); ok {
		return r
	}
	if e.Large {
		return float64(sx(58, w))
	}
	return float64(sx(35, w))
}

func survivalEnemyHitRadius(e SurvivalEnemy, w int32) float64 {
	if r, ok := survivalExpansionVisualRadius(e, w); ok {
		return r + float64(sx(8, w))
	}
	// Slightly larger than the visible ship so fast, accurate flicks at the edge remain fair.
	if e.Large {
		return float64(sx(68, w))
	}
	return float64(sx(42, w))
}

func survivalEnemySeparationRadius(e SurvivalEnemy, w int32) float64 {
	if r, ok := survivalExpansionVisualRadius(e, w); ok {
		return r + float64(sx(4, w))
	}
	// Equal to / slightly larger than the visible footprint so sprites never overlap.
	if e.Large {
		return float64(sx(61, w))
	}
	return float64(sx(38, w))
}

func survivalEnemyShieldCollisionRadius(e SurvivalEnemy, w int32) float64 {
	if r, ok := survivalExpansionVisualRadius(e, w); ok {
		return r * .68
	}
	// Tight body collision for orbiting shield balls. This intentionally does NOT
	// use the broad separation radius, otherwise the orbital ring feels solid.
	if e.Large {
		return float64(sx(40, w))
	}
	return float64(sx(24, w))
}

func survivalShieldBallCollisionRadius(w int32) float64 {
	// Shield artwork is drawn at ~29 px radius; keep collision slightly inside the
	// visible edge so the enemy must visibly touch the ball to consume it.
	return float64(sx(21, w))
}

func resetSurvivalRun() {
	if gameMeta.SurvivalCheckpoint < 1 {
		gameMeta.SurvivalCheckpoint = 1
	}
	if gameMeta.SurvivalCheckpoint == 30 {
		gameMeta.SurvivalCheckpoint = survivalBoss3Wave
		saveGameMeta()
	}
	survivalCheckpoint = gameMeta.SurvivalCheckpoint
	survivalEnemies = nil
	survivalPickup = nil
	survivalOrbitals = map[int]*SurvivalOrbital{}
	survivalShieldCharges = 0
	survivalChainCharges = 0
	survivalSlowUntil = time.Time{}
	survivalEliminations = nil
	survivalWaveBannerUntil = time.Time{}
	survivalWaveBannerText = ""
	survivalHP = survivalMaxHP()
	// Restart from the selected replay checkpoint. Boss checkpoints (10/20/31)
	// start the dossier immediately; sector checkpoints (1/11/21) start the wave.
	if survivalSelectedStartWave > 0 {
		valid := false
		for i, cp := range survivalReplayCheckpoints {
			if cp.Wave == survivalSelectedStartWave && cp.Boss == survivalSelectedStartBoss && survivalReplayCheckpointUnlocked(i) {
				valid = true
				break
			}
		}
		if valid {
			survivalWave = survivalSelectedStartWave
		} else {
			survivalSelectedStartWave = 0
			survivalSelectedStartBoss = 0
			survivalWave = survivalCheckpoint
		}
	} else {
		// A raw persistent boss checkpoint should replay that boss on Continue.
		survivalWave = survivalCheckpoint
		switch survivalCheckpoint {
		case 10:
			survivalSelectedStartBoss = 1
		case 20:
			survivalSelectedStartBoss = 2
		case survivalBoss3Wave:
			survivalSelectedStartBoss = 3
		default:
			survivalSelectedStartBoss = 0
		}
	}
	if survivalWave < 1 {
		survivalWave = 1
	}
	survivalKills = 0
	survivalWaveKills = 0
	survivalWaveBudget = 0
	survivalWaveSpawned = 0
	survivalNextSpawn = time.Time{}
	survivalWaveBreakUntil = time.Time{}
	survivalNextPickupAt = time.Time{}
	survivalStartedAt = time.Time{}
	survivalLastTick = time.Time{}
	survivalLastValidClick = time.Time{}
	survivalReactionSum = 0
	survivalReactionSamples = 0
	survivalPerfectHits = 0
	survivalTotalHits = 0
	survivalCombo = 0
	survivalBestCombo = 0
	survivalLastKillAt = time.Time{}
	survivalPerfectWaves = 0
	survivalPrecisionKills = 0
	survivalEliteKills = 0
	survivalScoreEXPClaimed = 0
	survivalBossEXPClaimed = 0
	survivalLastEXPAward = 0
	survivalWaveDamageStart = 0
	survivalWaveDamageTaken = false
	survivalTotalDamageTaken = 0
	survivalWaveShieldStart = 0
	survivalWaveShieldUsed = 0
	survivalWaveType = survivalWaveNormal
	survivalFormationIndex = 0
	survivalPickup2 = nil
	survivalArcFX = nil
	resetSurvivalBoss1State()
	resetSurvivalBoss2State()
	resetSurvivalBoss3State()
	survivalBossIntroStopAudio()
	survivalBossIntroStage = survivalBossIntroNone
	survivalBossIntroBoss = 0
	survivalBossIntroAt = time.Time{}
	survivalBossIntroRed = nil
	survivalBossIntroCyan = nil
}

func beginSurvivalWave() {
	// Wave 31 is a dedicated boss slot, never a normal Survival wave. Keep this
	// hard guard here as well as in updateSurvival so no call path can accidentally
	// initialise enemy budgets, pickups or Sector 3 field events under The Terminus.
	if survivalWave == survivalBoss3Wave {
		survivalEnemies = nil
		survivalPickup = nil
		survivalPickup2 = nil
		survivalEliminations = nil
		survivalArcFX = nil
		survivalOrbitals = map[int]*SurvivalOrbital{}
		survivalWaveKills = 0
		survivalWaveSpawned = 0
		survivalWaveBudget = 0
		survivalNextSpawn = time.Time{}
		survivalNextPickupAt = time.Time{}
		survivalWaveBreakUntil = time.Time{}
		survivalWaveType = survivalWaveNormal
		resetSurvivalSector3FieldEvent()
		stopLoopSFX("survival_1hp_siren")
		status = "SURVIVAL // BOSS WAVE 31"
		return
	}
	// Section music changes only at 1/11/21. Boss-specific transitions handle
	// their own fades; this guard covers checkpoint starts and future bosses.
	if audioReady && survivalActive() && !survivalBoss1Active() &&
		(survivalWave == 11 || survivalWave == 21) && state == StatePlaying {
		// Section transitions are explicit music boundaries. Stop the other section
		// tracks before starting the new one, so no two Survival themes overlap.
		alias := survivalMusicAliasForWave(survivalWave)
		if alias != "" {
			for _, other := range []string{"survival_music", "survival_section2", "survival_section3"} {
				if other != alias {
					mci("stop " + other)
				}
			}
			mci("seek " + alias + " to start")
			mci(fmt.Sprintf("setaudio %s volume to %d", alias, gameMeta.MusicVolume*10))
			mci("play " + alias + " repeat")
			bossStarted = true
			bossPaused = false
		}
	}
	survivalWaveKills = 0
	survivalWaveSpawned = 0
	survivalFormationIndex = 0
	survivalWaveDamageStart = survivalHP
	survivalWaveDamageTaken = false
	survivalWaveShieldStart = survivalShieldCharges
	survivalWaveShieldUsed = 0
	survivalWaveType = survivalChooseWaveType(survivalWave)
	resetSurvivalSector3FieldEvent()
	if survivalWaveType == survivalWaveNoSupport {
		survivalPickup = nil
		survivalPickup2 = nil
	}
	survivalWaveBudget = 7 + survivalWave*2
	if survivalWaveType == survivalWaveSwarm {
		survivalWaveBudget += 8
	}
	if survivalWaveType == survivalWaveOverclocked && survivalWaveBudget > 10 {
		survivalWaveBudget -= 5
	}
	if survivalWaveBudget > 44 {
		survivalWaveBudget = 44
	}
	survivalNextSpawn = time.Now().Add(500 * time.Millisecond)
	survivalWaveBreakUntil = time.Time{}
	status = fmt.Sprintf("SURVIVAL // WAVE %d", survivalWave)
	if name := survivalWaveTypeName(survivalWaveType); name != "" {
		survivalWaveBannerText = "⚠ " + name + " ⚠"
		survivalWaveBannerUntil = time.Now().Add(1900 * time.Millisecond)
	} else if survivalWave >= 2 {
		survivalWaveBannerText = fmt.Sprintf("WAVE %d INCOMING", survivalWave)
		survivalWaveBannerUntil = time.Now().Add(1500 * time.Millisecond)
	}
}

func invalidateSurvivalArena(h uintptr) {
	if h == 0 {
		return
	}
	r := arenaRectForInvalidate(h)
	invalidateRect.Call(h, uintptr(unsafe.Pointer(&r)), 0)
}

func invalidateSurvivalHUD(h uintptr) {
	if h == 0 {
		return
	}
	w, hgt := getClient(h)
	top := RECT{sx(580, w), sy(2, hgt), w, sy(132, hgt)}
	dock := RECT{sx(20, w), sy(798, hgt), sx(452, w), hgt}
	invalidateRect.Call(h, uintptr(unsafe.Pointer(&top)), 0)
	invalidateRect.Call(h, uintptr(unsafe.Pointer(&dock)), 0)
}

func prewarmSurvivalRuntimeSprites(hdc uintptr) {
	if hdc == 0 {
		return
	}
	if !survivalRenderPrewarmed {
		ensureRuntimeSprite(hdc, survivalBackgrounds[0], 1508, 592)
		ensureRuntimeSprite(hdc, survivalStationPerfectBGRA, 256, 256)
		ensureRuntimeSprite(hdc, survivalStationDamagedBGRA, 256, 256)
		ensureRuntimeSprite(hdc, survivalStationHeavyBGRA, 256, 256)
		ensureRuntimeSprite(hdc, survivalShieldBGRA, 96, 96)
		ensureRuntimeSprite(hdc, survivalSlowBGRA, 96, 96)
		ensureRuntimeSprite(hdc, survivalRepairBGRA, 96, 96)
		ensureRuntimeSprite(hdc, survivalBombBGRA, 96, 96)
		ensureRuntimeSprite(hdc, survivalChainBGRA, 96, 96)
		for i := 0; i < 16; i++ {
			ensureRuntimeSprite(hdc, survivalSmallRed[i], 96, 96)
			ensureRuntimeSprite(hdc, survivalSmallBlue[i], 96, 96)
			ensureRuntimeSprite(hdc, survivalBigRed[i], 144, 144)
			ensureRuntimeSprite(hdc, survivalBigBlue[i], 144, 144)
		}
		survivalRenderPrewarmed = true
		survivalNextBackgroundPrewarmed = 0
	}
	// Warm the correct current/next backdrop. While waiting, use the boss-earned
	// checkpoint preview so returning players do not briefly show/prewarm Sector 1.
	previewWave := survivalDisplayWave()
	want := -1
	if previewWave >= 21 {
		want = 2
	} else if previewWave >= 11 {
		want = 1
	} else if previewWave >= 7 && previewWave < 10 {
		want = 1
	}
	if want > survivalNextBackgroundPrewarmed && want < len(survivalBackgrounds) {
		ensureRuntimeSprite(hdc, survivalBackgrounds[want], 1508, 592)
		survivalNextBackgroundPrewarmed = want
	}
}

func startSurvivalGame(h uintptr) {
	if state != StateWaiting || !survivalActive() {
		return
	}
	resetSurvivalRun()
	if survivalSelectedStartWave <= 0 && survivalCheckpoint > 1 {
		survivalWave = survivalCheckpoint
	}
	survivalStartedAt = time.Now()
	survivalLastTick = survivalStartedAt
	survivalNextPickupAt = survivalStartedAt.Add(time.Duration(10+rand.Intn(7)) * time.Second)
	state = StatePlaying
	startTime = survivalStartedAt
	lastTime = 0
	score = 0
	streak = 0
	setCapture.Call(h)
	killTimer.Call(h, TIMER_GAME)
	setTimer.Call(h, TIMER_GAME, 16, 0)
	if survivalSelectedStartBoss > 0 {
		// Boss replay checkpoints bypass the preceding wave entirely. Reset the
		// station and transient wave state, then play the same dossier cinematic.
		survivalHP = survivalMaxHP()
		survivalEnemies = nil
		survivalWaveSpawned = 0
		survivalWaveBudget = 0
		survivalWaveBreakUntil = time.Time{}
		beginSurvivalBossDossier(h, survivalSelectedStartBoss, survivalStartedAt)
	} else {
		beginSurvivalWave()
		switchModeMusic()
	}
	invalidateRect.Call(h, 0, 0)
}

func survivalSpawnEnemy(w, h int32) {
	r := arenaRect(w, h)
	c := survivalArenaCenter(w, h)
	kind := survivalChooseEnemyKind()
	largeChance := math.Min(.12+float64(survivalWave-1)*.012, .34)
	if survivalWaveType == survivalWaveHeavy {
		largeChance = .78
	}
	if survivalWaveType == survivalWaveSwarm {
		largeChance = .03
	}
	large := rand.Float64() < largeChance
	if kind == survivalEnemyScout || kind == survivalEnemyHeavy {
		if survivalWaveType == survivalWaveHeavy {
			kind = survivalEnemyHeavy
		} else if kind == survivalEnemyScout && large {
			kind = survivalEnemyHeavy
		}
		large = kind == survivalEnemyHeavy
	} else {
		large = kind == survivalEnemyArmouredSatellite
	}
	red := rand.Intn(2) == 0
	if survivalWaveType == survivalWaveReaction {
		red = survivalWaveSpawned%2 == 0
	}
	hp := 1
	if large {
		hp = 2
	}
	elite := survivalRollEliteType(survivalWave, survivalWaveType)
	if elite == survivalEliteArmoured {
		hp++
	}

	// Reserve a clear footprint before accepting a spawn. This prevents two enemies
	// from ever being born on top of each other or on top of a live pickup.
	probe := SurvivalEnemy{Large: large, Elite: elite, Kind: kind}
	survivalApplyKindDefaults(&probe)
	need := survivalEnemySeparationRadius(probe, w)
	// Keep the whole sprite inside the arena on its first visible frame.
	margin := survivalEnemyVisualRadius(probe, w) + float64(sx(5, w))
	for tries := 0; tries < 48; tries++ {
		side := rand.Intn(4)
		if survivalWaveType == survivalWaveCrossfire {
			side = survivalFormationIndex % 2
			if side == 1 {
				side = 3
			}
		}
		if survivalWaveType == survivalWaveReaction {
			side = survivalFormationIndex % 4
		}
		var x, y float64
		switch side {
		case 0:
			x = float64(r.Left) + margin + rand.Float64()*(float64(r.Right-r.Left)-2*margin)
			y = float64(r.Top) + margin
		case 1:
			x = float64(r.Right) - margin
			y = float64(r.Top) + margin + rand.Float64()*(float64(r.Bottom-r.Top)-2*margin)
		case 2:
			x = float64(r.Left) + margin + rand.Float64()*(float64(r.Right-r.Left)-2*margin)
			y = float64(r.Bottom) - margin
		default:
			x = float64(r.Left) + margin
			y = float64(r.Top) + margin + rand.Float64()*(float64(r.Bottom-r.Top)-2*margin)
		}
		// Splitters immediately create two new reaction targets when destroyed, so never
		// introduce one close to the station where the player has almost no time to
		// identify the child colours. Top/bottom spawns near the station centre are
		// rejected while side/corner approaches remain available.
		if kind == survivalEnemySplitter && math.Hypot(x-c.X, y-c.Y) < float64(sx(360, w)) {
			continue
		}

		blocked := false
		for _, other := range survivalEnemies {
			minD := need + survivalEnemySeparationRadius(other, w) + float64(sx(8, w))
			if math.Hypot(x-other.X, y-other.Y) < minD {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		dx, dy := c.X-x, c.Y-y
		d := math.Hypot(dx, dy)
		if d < 1 {
			continue
		}
		speed := 54.0 + float64(survivalWave)*2.7
		if speed > 130 {
			speed = 130
		}
		if large {
			speed *= .78
		}
		if survivalWaveType == survivalWaveOverclocked {
			speed *= 1.28
		}
		if survivalWaveType == survivalWaveSwarm {
			speed *= .86
		}
		if elite == survivalEliteArmoured {
			speed *= .76
		} else if elite == survivalEliteStriker {
			speed *= 1.22
		}
		speed *= survivalExpansionSpeedScale(kind)
		critical := large && kind <= survivalEnemyHeavy && rand.Float64() < .55
		e := SurvivalEnemy{X: x, Y: y, VX: dx / d * speed, VY: dy / d * speed, Red: red, Large: large, HP: hp, SpawnedAt: time.Now(), Critical: critical, Elite: elite, Kind: kind}
		survivalApplyKindDefaults(&e)
		survivalEnemies = append(survivalEnemies, e)
		oldGuideMask := gameMeta.SurvivalGuideMask
		gameMeta.SurvivalGuideMask |= 1 << uint(kind)
		if gameMeta.SurvivalGuideMask != oldGuideMask {
			saveGameMeta()
		}
		survivalWaveSpawned++
		survivalFormationIndex++
		return
	}
}

func survivalSpawnInterval() time.Duration {
	ms := 1150 - survivalWave*28
	if survivalWaveType == survivalWaveSwarm {
		ms = int(float64(ms) * .72)
	}
	if survivalWaveType == survivalWaveReaction {
		ms = int(float64(ms) * .82)
	}
	if survivalWaveType == survivalWaveOverclocked {
		ms = int(float64(ms) * .78)
	}
	if ms < 330 {
		ms = 330
	}
	return time.Duration(ms) * time.Millisecond
}

func survivalMaxHP() int {
	bonus := gameMeta.AFKStationHPBonus
	if bonus < 0 {
		bonus = 0
	}
	if bonus > 2 {
		bonus = 2
	}
	return 6 + bonus
}

func survivalNormalWaveOwnsArena() bool {
	return state == StatePlaying && !survivalBossIntroActive() && !survivalBoss1Active() && !survivalBoss2Active() && !survivalBoss3Active() && survivalWave != survivalBoss3Wave
}

func survivalEnemySlotWeight(e SurvivalEnemy) int {
	switch e.Kind {
	case survivalEnemyGravityNode, survivalEnemyArmouredSatellite:
		return 3
	case survivalEnemySplitter, survivalEnemyShieldDrone, survivalEnemyPulseMine, survivalEnemyInterceptor, survivalEnemyPhantom, survivalEnemySwarmDrone:
		return 2
	default:
		return 1
	}
}

func survivalActiveEnemySlots() int {
	total := 0
	for _, e := range survivalEnemies {
		total += survivalEnemySlotWeight(e)
	}
	return total
}

func survivalMaxAlive() int {
	if survivalWave >= 28 && survivalWave <= 30 {
		return 9
	}
	if survivalWave >= 25 && survivalWave <= 27 {
		return 8
	}
	if survivalWave >= 20 && survivalWave <= 24 {
		return 7
	}
	n := 4 + survivalWave/3
	if n > 7 {
		n = 7
	}
	return n
}

// survivalResolveSeparation keeps ENEMIES visually distinct from each other.
// Pickups are intentionally excluded: ships may fly over them without changing course.
func survivalResolveSeparation(w, h int32) {
	if len(survivalEnemies) == 0 {
		return
	}
	r := arenaRect(w, h)

	clampEnemy := func(e *SurvivalEnemy) {
		pad := survivalEnemySeparationRadius(*e, w)
		e.X = math.Max(float64(r.Left)+pad, math.Min(float64(r.Right)-pad, e.X))
		e.Y = math.Max(float64(r.Top)+pad, math.Min(float64(r.Bottom)-pad, e.Y))
	}

	// Pickups are visual/gameplay overlays, not physical obstacles.
	// Enemies intentionally fly straight through them on their route to the station.

	if len(survivalEnemies) < 2 {
		return
	}

	// Three tiny O(n²) relaxation passes are still negligible at the 11-enemy cap,
	// but eliminate the one-frame overlaps that two passes could leave after clamping.
	for pass := 0; pass < 3; pass++ {
		for i := 0; i < len(survivalEnemies); i++ {
			for j := i + 1; j < len(survivalEnemies); j++ {
				a := &survivalEnemies[i]
				b := &survivalEnemies[j]
				minD := survivalEnemySeparationRadius(*a, w) + survivalEnemySeparationRadius(*b, w) + float64(sx(6, w))
				dx, dy := b.X-a.X, b.Y-a.Y
				d := math.Hypot(dx, dy)
				if d >= minD {
					continue
				}
				if d < .001 {
					// Stable deterministic split direction avoids random jitter when centres match.
					a := float64((i+1)*(j+3)) * 1.61803398875
					dx, dy = math.Cos(a), math.Sin(a)
					d = 1
				}
				push := (minD - d) * .52
				nx, ny := dx/d, dy/d
				a.X -= nx * push
				a.Y -= ny * push
				b.X += nx * push
				b.Y += ny * push
				clampEnemy(a)
				clampEnemy(b)
			}
		}
	}
}

func unlockSurvivalCheckpoint(wave int) {
	// Replay progression records boss-entry and post-boss checkpoints. Boss 3 is
	// its own Wave 31; Wave 32 is the post-Terminus endless continuation.
	valid := wave == 10 || wave == 11 || wave == 20 || wave == 21 || wave == survivalBoss3Wave || wave == survivalPostBoss3Wave
	if !valid || wave <= gameMeta.SurvivalCheckpoint {
		return
	}
	gameMeta.SurvivalCheckpoint = wave
	survivalCheckpoint = wave
	saveGameMeta()
	playLevelUpSound()
	requestPlayerProfileSync()
}

func syncSurvivalCriticalSiren() {
	// Boss encounters own the arena and audio mix completely. The station siren belongs
	// only to ordinary Survival waves; carrying it into a boss makes the boss feel like
	// an overlay on the preceding wave and can mask the dedicated boss soundscape.
	if survivalBossIntroActive() || survivalBoss1Active() || survivalBoss2Active() || survivalBoss3Active() {
		stopLoopSFX("survival_1hp_siren")
		return
	}
	// Quiet emergency ambience only while the station is genuinely one hit from destruction.
	// The loop is owned by the centralized SFX bus, so it cannot steal the Survival music alias.
	if survivalActive() && state == StatePlaying && survivalHP == 1 {
		startLoopSFX("survival_1hp_siren")
	} else {
		stopLoopSFX("survival_1hp_siren")
	}
}

func updateSurvival(h uintptr) {
	if !survivalActive() || state != StatePlaying {
		stopLoopSFX("survival_1hp_siren")
		return
	}
	syncSurvivalCriticalSiren()
	now := time.Now()
	dt := now.Sub(survivalLastTick).Seconds()
	if dt <= 0 {
		dt = .016
	}
	// Never turn a temporary scheduling hitch into an artificial 16 ms pause.
	// Preserve elapsed movement, but cap catch-up so a long OS stall cannot teleport ships.
	if dt > .050 {
		dt = .050
	}
	survivalLastTick = now
	w, hgt := getClient(h)
	c := survivalArenaCenter(w, hgt)

	if survivalBossIntroActive() {
		updateSurvivalBossIntro(h, now, w, hgt)
		lastTime = now.Sub(survivalStartedAt).Seconds()
		return
	}

	if survivalBoss1SuppressNormalLogic() {
		updateSurvivalBoss1(h, now, w, hgt)
		lastTime = now.Sub(survivalStartedAt).Seconds()
		return
	}
	if survivalBoss2SuppressNormalLogic() {
		updateSurvivalBoss2(h, now, w, hgt)
		lastTime = now.Sub(survivalStartedAt).Seconds()
		return
	}
	if survivalBoss3SuppressNormalLogic() {
		updateSurvivalBoss3(h, now, w, hgt)
		lastTime = now.Sub(survivalStartedAt).Seconds()
		return
	}

	// Safety-net ownership rule copied from the proven Boss 1/Boss 2 sequencing:
	// once the run reaches the dedicated boss wave there is no normal-wave frame at all.
	// If a checkpoint, transition or future code path leaves us on W31 without an active
	// dossier/boss owner, hand control straight to the dossier before spawning anything.
	if survivalWave == survivalBoss3Wave {
		survivalEnemies = nil
		survivalPickup = nil
		survivalPickup2 = nil
		survivalEliminations = nil
		survivalArcFX = nil
		survivalWaveSpawned = 0
		survivalWaveBudget = 0
		survivalNextSpawn = time.Time{}
		survivalWaveBreakUntil = time.Time{}
		resetSurvivalSector3FieldEvent()
		stopLoopSFX("survival_1hp_siren")
		beginSurvivalBossDossier(h, 3, now)
		lastTime = now.Sub(survivalStartedAt).Seconds()
		return
	}

	if !survivalWaveBreakUntil.IsZero() {
		if now.After(survivalWaveBreakUntil) {
			survivalWave++
			beginSurvivalWave()
			invalidateSurvivalHUD(h)
		}
		invalidateSurvivalArena(h)
		return
	}

	if survivalNormalWaveOwnsArena() && survivalWaveSpawned < survivalWaveBudget && survivalActiveEnemySlots() < survivalMaxAlive() && (survivalNextSpawn.IsZero() || !now.Before(survivalNextSpawn)) {
		beforeSlots := survivalActiveEnemySlots()
		survivalSpawnEnemy(w, hgt)
		if survivalActiveEnemySlots() > survivalMaxAlive() {
			// A newly rolled special can exceed the weighted cap; defer it instead.
			if len(survivalEnemies) > 0 {
				survivalEnemies = survivalEnemies[:len(survivalEnemies)-1]
			}
			if survivalWaveSpawned > 0 {
				survivalWaveSpawned--
			}
			_ = beforeSlots
			survivalNextSpawn = now.Add(180 * time.Millisecond)
			return
		}
		survivalNextSpawn = now.Add(survivalSpawnInterval())
	}

	impactR := survivalImpactRadius(w, hgt)
	speedScale := 1.0
	if !survivalSlowUntil.IsZero() && now.Before(survivalSlowUntil) {
		speedScale = .22
	}
	for i := range survivalEnemies {
		e := &survivalEnemies[i]
		e.X += e.VX * dt * speedScale
		e.Y += e.VY * dt * speedScale
	}
	survivalUpdateSpecialEnemies(now, w, hgt, dt)
	if !survivalPulseMineHazard(h, now, w, hgt) {
		return
	}
	survivalResolveSeparation(w, hgt)
	// Separation can nudge a ship sideways. Re-aim its velocity at the station while
	// preserving speed so every enemy sprite continues to fly nose-first toward the core.
	for i := range survivalEnemies {
		e := &survivalEnemies[i]
		if (e.Kind == survivalEnemyPulseMine && now.Before(e.SpecialTimer)) || (e.Kind == survivalEnemyGravityNode && now.Before(e.SpecialTimer)) {
			continue
		}
		speed := math.Hypot(e.VX, e.VY)
		dx, dy := c.X-e.X, c.Y-e.Y
		d := math.Hypot(dx, dy)
		if d > .001 && speed > 0 {
			e.VX = dx / d * speed
			e.VY = dy / d * speed
		}
	}
	shieldPts, shieldCount := survivalShieldPositions(w, hgt, now)
	var shieldUsed [8]bool
	out := survivalEnemies[:0]
	for _, e := range survivalEnemies {
		intercepted := false
		if survivalShieldCharges > 0 {
			er := survivalEnemyShieldCollisionRadius(e, w)
			shieldR := survivalShieldBallCollisionRadius(w)
			for si := 0; si < shieldCount; si++ {
				sp := shieldPts[si]
				if shieldUsed[si] {
					continue
				}
				if math.Hypot(e.X-sp.X, e.Y-sp.Y) <= er+shieldR {
					shieldUsed[si] = true
					survivalShieldCharges--
					survivalWaveShieldUsed++
					survivalAddElimination(e)
					playSurvivalExplodeSound()
					survivalKills++
					survivalCheckKillAchievements()
					survivalWaveKills++
					score += 35
					status = "SHIELD INTERCEPT"
					intercepted = true
					break
				}
			}
		}
		if intercepted {
			continue
		}
		if math.Hypot(e.X-c.X, e.Y-c.Y) <= impactR {
			if survivalReviveImmune(now) {
				survivalAddElimination(e)
				status = "REVIVE IMMUNITY // IMPACT ABSORBED"
				continue
			}
			dmg := survivalSpecialImpactDamage(e)
			if e.Elite == survivalEliteArmoured {
				dmg++
			}
			survivalHP -= dmg
			playSurvivalDamageTakenSound()
			survivalWaveDamageTaken = true
			survivalTotalDamageTaken += dmg
			status = fmt.Sprintf("STATION HIT -%d HP", dmg)
			if survivalHP <= 0 {
				survivalHP = 0
				invalidateSurvivalHUD(h)
				survivalFail(h, "Station destroyed")
				return
			}
			invalidateSurvivalHUD(h)
			continue
		}
		out = append(out, e)
	}
	survivalEnemies = out

	survivalUpdateOrbitals(now, w, hgt)
	if !updateSurvivalSector3FieldEvent(h, now, w, hgt, dt) {
		return
	}

	if survivalPickup != nil && now.Sub(survivalPickup.SpawnedAt) >= 5*time.Second {
		survivalPickup = nil
	}
	if survivalPickup2 != nil && now.Sub(survivalPickup2.SpawnedAt) >= 5*time.Second {
		survivalPickup2 = nil
	}
	if survivalPickup == nil && now.After(survivalNextPickupAt) {
		survivalCreatePickup(w, hgt)
		if survivalWaveType == survivalWavePowerSurge {
			survivalCreatePickup(w, hgt)
			survivalNextPickupAt = now.Add(time.Duration(6+rand.Intn(5)) * time.Second)
		} else {
			survivalNextPickupAt = now.Add(time.Duration(12+rand.Intn(10)) * time.Second)
		}
	}

	if survivalWaveSpawned >= survivalWaveBudget && len(survivalEnemies) == 0 {
		updatePersistentSurvivalProfileStats(survivalWave, survivalKills, survivalBestCombo)
		perfect := !survivalWaveDamageTaken && survivalWaveShieldUsed == 0
		unlockAchievement("SURV_FIRST_WAVE")
		if perfect {
			survivalPerfectWaves++
			score += 500 + survivalWave*25
			status = "PERFECT DEFENCE // BONUS"
			unlockAchievement("SURV_PERFECT_WAVE")
			if survivalWave == 5 || survivalWave == 10 || survivalWave == 20 || survivalWave == 30 {
				enqueueLiveAnnouncement(fmt.Sprintf("Perfect Defence on Survival Wave %d — station untouched.", survivalWave))
			}
		} else {
			status = fmt.Sprintf("WAVE %d CLEARED", survivalWave)
		}
		if survivalWaveType == survivalWaveNoSupport {
			score += 350 + survivalWave*15
		}
		if survivalWave == 10 {
			unlockSurvivalCheckpoint(10)
			beginSurvivalBossDossier(h, 1, now)
			lastTime = now.Sub(survivalStartedAt).Seconds()
			return
		}
		if survivalWave == 20 {
			unlockSurvivalCheckpoint(20)
			beginSurvivalBossDossier(h, 2, now)
			lastTime = now.Sub(survivalStartedAt).Seconds()
			return
		}
		if survivalWave == 30 {
			// Boss 3 is a dedicated boss wave, not an overlay on Sector 3 Wave 30.
			survivalWave = survivalBoss3Wave
			unlockSurvivalCheckpoint(survivalBoss3Wave)
			beginSurvivalBossDossier(h, 3, now)
			lastTime = now.Sub(survivalStartedAt).Seconds()
			return
		}
		survivalWaveBreakUntil = now.Add(2400 * time.Millisecond)
		invalidateSurvivalHUD(h)
	}
	lastTime = now.Sub(survivalStartedAt).Seconds()
	invalidateSurvivalArena(h)
}

func survivalCheckKillAchievements() {
	if survivalKills >= 25 {
		unlockAchievement("SURV_25_KILLS")
	}
	if survivalKills >= 50 {
		unlockAchievement("SURV_50_KILLS")
	}
}

func survivalAddElimination(e SurvivalEnemy) {
	if len(survivalEliminations) >= 24 {
		survivalEliminations = survivalEliminations[len(survivalEliminations)-23:]
	}
	survivalEliminations = append(survivalEliminations, SurvivalElimination{X: e.X, Y: e.Y, Red: e.Red, Large: e.Large, Started: time.Now()})
}

func survivalTrimEliminations(now time.Time) {
	out := survivalEliminations[:0]
	for _, fx := range survivalEliminations {
		if now.Sub(fx.Started) < 360*time.Millisecond {
			out = append(out, fx)
		}
	}
	survivalEliminations = out
}

func survivalShieldPositions(w, h int32, now time.Time) ([8]FPoint, int) {
	var out [8]FPoint
	if survivalShieldCharges <= 0 {
		return out, 0
	}
	c := survivalArenaCenter(w, h)
	r := survivalOrbitalRadius(w, h)
	count := survivalShieldCharges
	if count > len(out) {
		count = len(out)
	}
	base := now.Sub(survivalStartedAt).Seconds() * 2.85
	for i := 0; i < count; i++ {
		a := base + float64(i)*2*math.Pi/float64(count)
		out[i] = FPoint{X: c.X + r*math.Cos(a), Y: c.Y + r*math.Sin(a)}
	}
	return out, count
}

func survivalUpdateOrbitals(now time.Time, w, h int32) {
	// Only Shield remains an orbital. Other pickups are instant-use or armed effects.
	survivalTrimEliminations(now)
}

func survivalCreatePickup(w, h int32) {
	if survivalWaveType == survivalWaveNoSupport {
		return
	}
	r := arenaRect(w, h)
	c := survivalArenaCenter(w, h)
	for tries := 0; tries < 24; tries++ {
		x := float64(r.Left+sx(90, w)) + rand.Float64()*float64((r.Right-r.Left)-2*sx(90, w))
		y := float64(r.Top+sy(70, h)) + rand.Float64()*float64((r.Bottom-r.Top)-2*sy(70, h))
		if math.Hypot(x-c.X, y-c.Y) < survivalOrbitalRadius(w, h)+50 {
			continue
		}
		// Weighted Survival power-ups.
		// Bombs are intentionally rare, and Chain Lightning is a Sector 3 reward.
		roll := rand.Intn(100)
		kind := survivalPickupShield
		if survivalWave >= 21 {
			switch {
			case roll < 29:
				kind = survivalPickupShield
			case roll < 55:
				kind = survivalPickupSlow
			case roll < 81:
				kind = survivalPickupRepair
			case roll < 90:
				kind = survivalPickupBomb
			default:
				kind = survivalPickupChain
			}
		} else {
			switch {
			case roll < 33:
				kind = survivalPickupShield
			case roll < 63:
				kind = survivalPickupSlow
			case roll < 92:
				kind = survivalPickupRepair
			default:
				kind = survivalPickupBomb
			}
		}
		pr := survivalPickupRadius(kind, w)
		safe := true
		for _, e := range survivalEnemies {
			minD := pr + survivalEnemySeparationRadius(e, w) + float64(sx(16, w))
			if math.Hypot(x-e.X, y-e.Y) < minD {
				safe = false
				break
			}
		}
		if !safe {
			continue
		}
		pu := &SurvivalPickup{X: x, Y: y, Kind: kind, SpawnedAt: time.Now()}
		if survivalPickup == nil {
			survivalPickup = pu
		} else if survivalPickup2 == nil {
			if math.Hypot(x-survivalPickup.X, y-survivalPickup.Y) < pr+survivalPickupRadius(survivalPickup.Kind, w)+float64(sx(40, w)) {
				continue
			}
			survivalPickup2 = pu
		}
		return
	}
}

func survivalPickupRadius(kind int, w int32) float64 {
	if kind == survivalPickupChain {
		return float64(sx(42, w))
	}
	return float64(sx(34, w))
}

func survivalApplyWaveClickPenalty(h uintptr, message string) {
	if survivalReviveImmune(time.Now()) {
		status = "REVIVE IMMUNITY"
		return
	}
	// Normal Survival waves are recoverable: a bad click damages the station instead
	// of instantly ending the run. Boss click handlers return before reaching this
	// helper, so Boss 1/2 mistakes remain one-strike failures; Boss 3 owns its recoverable station-damage penalties.
	if state != StatePlaying || survivalBoss1Active() || survivalBoss2Active() || survivalBoss3Active() {
		return
	}
	survivalHP--
	playSurvivalDamageTakenSound()
	survivalWaveDamageTaken = true
	survivalTotalDamageTaken++
	survivalCombo = 0
	survivalLastKillAt = time.Time{}
	if survivalHP <= 0 {
		survivalHP = 0
		invalidateSurvivalHUD(h)
		invalidateSurvivalArena(h)
		survivalFail(h, "Station destroyed")
		return
	}
	status = message
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
}

func survivalHandleClick(h uintptr, p FPoint, right bool) bool {
	if !survivalActive() {
		return false
	}
	w, hgt := getClient(h)
	if state == StateWaiting {
		c := survivalArenaCenter(w, hgt)
		if math.Hypot(p.X-c.X, p.Y-c.Y) <= float64(sx(125, w)) {
			startSurvivalGame(h)
		}
		return true
	}
	if state != StatePlaying {
		return true
	}
	if survivalBossIntroActive() {
		return true
	}
	if survivalBoss1Active() {
		return survivalBoss1HandleClick(h, p, right)
	}
	if survivalBoss2Active() {
		return survivalBoss2HandleClick(h, p, right)
	}
	if survivalBoss3Active() {
		return survivalBoss3HandleClick(h, p, right)
	}
	if !pointInArena(p, w, hgt) {
		return true
	}

	for slot, pu := range []*SurvivalPickup{survivalPickup, survivalPickup2} {
		if pu != nil && math.Hypot(p.X-pu.X, p.Y-pu.Y) <= survivalPickupRadius(pu.Kind, w) {
			survivalCollectPickup(pu.Kind)
			if slot == 0 {
				survivalPickup = nil
			} else {
				survivalPickup2 = nil
			}
			playPowerupPickupSound()
			invalidateSurvivalHUD(h)
			invalidateSurvivalArena(h)
			return true
		}
	}

	hit := -1
	best := math.MaxFloat64
	// Prefer the visible body before considering the forgiving outer hitbox. This
	// prevents an expanded neighbouring hitbox from stealing a click aimed at a
	// clearly visible enemy sprite.
	for visiblePass := 0; visiblePass < 2 && hit < 0; visiblePass++ {
		for i, e := range survivalEnemies {
			if !survivalSpecialClickable(e) {
				continue
			}
			radius := survivalEnemyHitRadius(e, w)
			if visiblePass == 0 {
				radius = survivalEnemyVisualRadius(e, w)
			}
			d := math.Hypot(p.X-e.X, p.Y-e.Y)
			if d <= radius && d < best {
				hit = i
				best = d
			}
		}
	}
	if hit < 0 {
		survivalApplyWaveClickPenalty(h, "MISCLICK — -1 STATION HP")
		return true
	}
	e := survivalEnemies[hit]
	// Only the standard red/blue energy enemies use colour-specific mouse buttons.
	// Scouts (including Scouts created by a Splitter) and Heavies keep the original
	// red = left / blue = right rule. Every expansion enemy is neutral and accepts
	// either mouse button while preserving its own shield/HP/phase behaviour.
	colourSensitive := e.Kind == survivalEnemyScout || e.Kind == survivalEnemyHeavy
	if colourSensitive && e.Red == right { // red requires left(false), blue requires right(true)
		survivalApplyWaveClickPenalty(h, "WRONG COLOUR — -1 STATION HP")
		return true
	}
	radius := survivalEnemyHitRadius(e, w)
	precision := 1.0 - best/radius
	if precision < 0 {
		precision = 0
	}
	survivalTotalHits++
	if precision >= .72 {
		survivalPerfectHits++
	}
	if !e.Reacted {
		survivalReactionSum += time.Since(e.SpawnedAt).Seconds() * 1000
		survivalReactionSamples++
		e.Reacted = true
	}
	if handled, _ := survivalSpecialClickPreDamage(&e); handled {
		survivalEnemies[hit] = e
		score += 20
		playSurvivalHitSound()
		invalidateSurvivalHUD(h)
		invalidateSurvivalArena(h)
		return true
	}
	e.HP--
	if e.HP > 0 {
		survivalEnemies[hit] = e
		score += 25
		if e.Elite == survivalEliteArmoured {
			status = fmt.Sprintf("ELITE ARMOUR // %d HITS REMAIN", e.HP)
		} else {
			status = "ARMOUR HIT — ONE MORE"
		}
		playSurvivalHitSound()
		invalidateSurvivalHUD(h)
		invalidateSurvivalArena(h)
		return true
	}
	killed := e
	survivalEnemies = append(survivalEnemies[:hit], survivalEnemies[hit+1:]...)
	if killed.Kind == survivalEnemySplitter {
		survivalSpawnSplitterChildren(killed, w, hgt)
	}
	survivalAddElimination(killed)
	survivalKills++
	survivalCheckKillAchievements()
	survivalWaveKills++
	streak++
	// Combo rewards consecutive clean eliminations; normal-wave misclicks break the combo and cost 1 station HP.
	if !survivalLastKillAt.IsZero() && time.Since(survivalLastKillAt) <= 2400*time.Millisecond {
		survivalCombo++
	} else {
		survivalCombo = 1
	}
	survivalLastKillAt = time.Now()
	if survivalCombo > 5 {
		survivalCombo = 5
	}
	if survivalCombo > survivalBestCombo {
		survivalBestCombo = survivalCombo
	}
	// Letting a target approach the station is worth more, creating optional risk/reward.
	c := survivalArenaCenter(w, hgt)
	dist := math.Hypot(killed.X-c.X, killed.Y-c.Y)
	orbitR := survivalOrbitalRadius(w, hgt)
	dangerBonus := 0
	if dist < orbitR*.62 {
		dangerBonus = 75
	} else if dist < orbitR {
		dangerBonus = 25
	}
	criticalBonus := 0
	if killed.Critical && precision >= .78 {
		criticalBonus = 50
		survivalPrecisionKills++
		status = "CRITICAL +50"
	}
	eliteBonus := 0
	if killed.Elite != survivalEliteNone {
		eliteBonus = 175
		survivalEliteKills++
		if survivalEliteKills >= 10 {
			unlockAchievement("SURV_ELITE_10")
		}
	}
	base := 100 + survivalWave*8 + int(precision*70) + dangerBonus + criticalBonus + eliteBonus
	score += base * survivalCombo
	// Normal Survival enemy hits use only the dedicated Survival Hit SFX.
	// The explosion remains visual-only here so a kill never stacks a second sound.
	playSurvivalHitSound()
	survivalTryChain(killed, w, hgt)
	status = "TARGET DESTROYED"
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
	return true
}

func survivalCollectPickup(kind int) {
	now := time.Now()
	switch kind {
	case survivalPickupShield:
		survivalShieldCharges++
		status = fmt.Sprintf("SHIELD STACK x%d", survivalShieldCharges)
	case survivalPickupSlow:
		survivalSlowUntil = now.Add(2 * time.Second)
		status = "TIME SLOW — 2 SECONDS"
	case survivalPickupRepair:
		if survivalHP < survivalMaxHP() {
			survivalHP++
		}
		status = "STATION REPAIRED +1 HP"
	case survivalPickupBomb:
		for _, e := range survivalEnemies {
			survivalAddElimination(e)
		}
		k := len(survivalEnemies)
		if k > 0 {
			playSurvivalExplodeSound()
		}
		survivalKills += k
		survivalWaveKills += k
		score += k * 35
		survivalEnemies = nil
		status = fmt.Sprintf("NOVA BOMB — %d TARGETS CLEARED", k)
	case survivalPickupChain:
		survivalChainCharges++
		status = "ARC RELAY ARMED"
	}
}

func survivalTryChain(killed SurvivalEnemy, w, h int32) {
	if survivalChainCharges <= 0 {
		return
	}
	survivalChainCharges--
	maxKills := 3
	radius := float64(sx(170, w))
	cx, cy := killed.X, killed.Y
	chained := 0
	for chained < maxKills {
		idx := -1
		best := radius
		for i, e := range survivalEnemies {
			if e.Red != killed.Red {
				continue
			}
			d := math.Hypot(e.X-cx, e.Y-cy)
			if d < best {
				best = d
				idx = i
			}
		}
		if idx < 0 {
			break
		}
		e := survivalEnemies[idx]
		survivalArcFX = append(survivalArcFX, SurvivalArcFX{X1: cx, Y1: cy, X2: e.X, Y2: e.Y, Started: time.Now().Add(time.Duration(chained) * 70 * time.Millisecond)})
		cx, cy = e.X, e.Y
		survivalEnemies = append(survivalEnemies[:idx], survivalEnemies[idx+1:]...)
		survivalAddElimination(e)
		survivalKills++
		survivalCheckKillAchievements()
		survivalWaveKills++
		score += 60
		chained++
	}
	if chained > 0 {
		playSurvivalExplodeSound()
		status = fmt.Sprintf("ARC RELAY x%d", chained)
	}
}

func survivalAwardPendingEXP(h uintptr, bossBonus int) int {
	// v360: Survival EXP economy reduced by 50%. Every 200 Survival points = 1 account EXP.
	// Boss bonuses are also halved below so the complete mode awards exactly half the old EXP.
	// Only the unclaimed score delta is paid so checkpoint reports cannot duplicate rewards.
	targetScoreEXP := score / 200
	scoreEXP := targetScoreEXP - survivalScoreEXPClaimed
	if scoreEXP < 0 {
		scoreEXP = 0
	}

	// Boss bonus parameters retain their historical values at call sites; convert them to
	// the new 50% Survival economy before comparing against the claimed total.
	scaledBossBonus := bossBonus / 2
	bonusEXP := 0
	if scaledBossBonus > survivalBossEXPClaimed {
		bonusEXP = scaledBossBonus - survivalBossEXPClaimed
	}

	earned := scoreEXP + bonusEXP
	if earned <= 0 {
		survivalLastEXPAward = 0
		return 0
	}

	oldRankIndex := rankIndexForEXP(playerProgress.EXP)
	oldRankName := rankForEXP(playerProgress.EXP)

	playerProgress.EXP += earned
	survivalScoreEXPClaimed = targetScoreEXP
	if scaledBossBonus > survivalBossEXPClaimed {
		survivalBossEXPClaimed = scaledBossBonus
	}
	survivalLastEXPAward = earned

	newRankIndex := rankIndexForEXP(playerProgress.EXP)
	newRankName := rankForEXP(playerProgress.EXP)
	if newRankIndex > oldRankIndex {
		levelUpAt = time.Now()
		levelUpFrom = oldRankName
		levelUpTo = newRankName
		playLevelUpSound()
		if h != 0 {
			setTimer.Call(h, TIMER_LEVELUP, 33, 0)
		}
	}

	savePlayerProgress()
	requestPlayerProfileSync()
	return earned
}

func survivalFail(h uintptr, reason string) {
	now := time.Now()
	if isDeveloperOwner() && !developerSurvivalGraceUntil.IsZero() && now.Before(developerSurvivalGraceUntil) {
		return
	}
	if survivalReviveImmune(now) {
		if survivalHP <= 0 {
			survivalHP = survivalMaxHP()
		}
		status = "REVIVE IMMUNITY"
		invalidateSurvivalHUD(h)
		invalidateSurvivalArena(h)
		return
	}
	if state != StatePlaying {
		return
	}
	killTimer.Call(h, TIMER_GAME)
	releaseCapture.Call()
	stopTransientGameplayAudio()
	if survivalBoss1Active() {
		survivalBoss1PauseMusic()
		playEnduranceFailSound()
	} else {
		pauseBossForFailure()
	}
	updatePersistentSurvivalProfileStats(survivalWave, survivalKills, survivalBestCombo)
	survivalRespawnRememberFailure()
	state = StateFailed
	analyticsEvent("run_failed", map[string]any{"mode": "SURVIVAL", "reason": analyticsFailureCategory(reason), "wave": survivalWave, "kills": survivalKills, "hp": survivalHP})
	status = reason
	hasLastMouse = false
	// The revive choice is a true frozen decision state. Do not start the generic
	// failure animation while Survival is waiting for the revive/report choice.
	polishFailureStarted = time.Time{}
	expEarned := survivalAwardPendingEXP(h, 0)
	lastResult = ResultData{
		Time:        time.Since(survivalStartedAt).Seconds(),
		CombinedAcc: survivalPerfectPercent(),
		TrackingAcc: survivalAccuracyPercent(),
		TargetAcc:   survivalPerfectPercent(),
		TargetsHit:  survivalKills,
		TargetCount: survivalTotalHits,
		TotalScore:  score,
		RoundPoints: score,
		Streak:      survivalWave,
		Combo:       float64(survivalBestCombo),
		Rating:      survivalGrade(),
		Rank:        rankForEXP(playerProgress.EXP),
		EXPEarned:   expEarned,
		TotalEXP:    playerProgress.EXP,
		Course:      "SURVIVAL",
		Difficulty:  "SURVIVAL",
	}
	killTimer.Call(h, TIMER_FAIL_RESET)
	survivalRespawnRefreshAsync()
	invalidateRect.Call(h, 0, 0)
	updateWindow.Call(h)
}

func finishSurvivalFailure(h uintptr) {
	if state != StateFailed || !survivalActive() {
		return
	}
	killTimer.Call(h, TIMER_FAIL_RESET)
	survivalBoss1OnFailureReport()
	resetSurvivalBoss2State()
	resetSurvivalBoss3State()
	state = StateResult
	status = "SURVIVAL MISSION REPORT"
	invalidateRect.Call(h, 0, 0)
	updateWindow.Call(h)
}

func survivalGrade() string {
	acc := survivalAccuracyPercent()
	precision := survivalPerfectPercent()
	react := survivalAverageReactionMS()
	reactionScore := 100.0
	if react > 250 {
		reactionScore = math.Max(0, 100-(react-250)/6)
	}
	waveScore := math.Min(100, float64(survivalWave)*3.5)
	perfectScore := math.Min(100, float64(survivalPerfectWaves)*14)
	composite := acc*.32 + precision*.25 + reactionScore*.18 + waveScore*.15 + perfectScore*.10
	switch {
	case composite >= 96:
		return "S+"
	case composite >= 90:
		return "S"
	case composite >= 82:
		return "A"
	case composite >= 72:
		return "B"
	case composite >= 60:
		return "C"
	default:
		return "D"
	}
}

func survivalAccuracyPercent() float64 {
	// Target accuracy here measures completed valid hits; normal-wave misclicks are
	// represented as station damage rather than an instant run failure. Centre
	// placement is tracked separately below.
	if survivalTotalHits == 0 {
		return 100
	}
	return 100
}
func survivalPerfectPercent() float64 {
	if survivalTotalHits == 0 {
		return 0
	}
	return float64(survivalPerfectHits) / float64(survivalTotalHits) * 100
}
func survivalAverageReactionMS() float64 {
	if survivalReactionSamples == 0 {
		return 0
	}
	return survivalReactionSum / float64(survivalReactionSamples)
}

func survivalDirectionIndex(e SurvivalEnemy) int {
	// Pre-rendered frames: 0 points UP, increasing clockwise.
	a := math.Atan2(e.VX, -e.VY)
	if a < 0 {
		a += 2 * math.Pi
	}
	return int(math.Round(a/(2*math.Pi)*16)) % 16
}

func survivalEnemySprite(e SurvivalEnemy) ([]byte, int32, int32) {
	i := survivalDirectionIndex(e)
	if e.Large {
		if e.Red {
			return survivalBigRed[i], 144, 144
		}
		return survivalBigBlue[i], 144, 144
	}
	if e.Red {
		return survivalSmallRed[i], 96, 96
	}
	return survivalSmallBlue[i], 96, 96
}

func survivalPickupSprite(kind int) ([]byte, int32, int32) {
	switch kind {
	case survivalPickupShield:
		return survivalShieldBGRA, 96, 96
	case survivalPickupSlow:
		return survivalSlowBGRA, 96, 96
	case survivalPickupRepair:
		return survivalRepairBGRA, 96, 96
	case survivalPickupBomb:
		return survivalBombBGRA, 96, 96
	default:
		return survivalChainBGRA, 96, 96
	}
}

func drawRawBGRACover(hdc uintptr, data []byte, srcW, srcH int32, box RECT) RECT {
	if srcW <= 0 || srcH <= 0 || box.Right <= box.Left || box.Bottom <= box.Top || len(data) < int(srcW*srcH*4) {
		return RECT{}
	}
	sprite := ensureRuntimeSprite(hdc, data, srcW, srcH)
	if sprite == nil || sprite.dc == 0 {
		return RECT{}
	}
	boxW := box.Right - box.Left
	boxH := box.Bottom - box.Top
	srcAspect := float64(srcW) / float64(srcH)
	dstAspect := float64(boxW) / float64(boxH)
	cropX, cropY := int32(0), int32(0)
	cropW, cropH := srcW, srcH
	if srcAspect > dstAspect {
		cropW = int32(float64(srcH) * dstAspect)
		cropX = (srcW - cropW) / 2
	} else if srcAspect < dstAspect {
		cropH = int32(float64(srcW) / dstAspect)
		cropY = (srcH - cropH) / 2
	}
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(box.Left), uintptr(box.Top), uintptr(boxW), uintptr(boxH),
		sprite.dc, uintptr(cropX), uintptr(cropY), uintptr(cropW), uintptr(cropH), blend)
	return box
}

func survivalVisualElapsedSeconds() float64 {
	if survivalStartedAt.IsZero() {
		return 0
	}
	if state == StatePlaying {
		return time.Since(survivalStartedAt).Seconds()
	}
	if lastTime > 0 {
		return lastTime
	}
	return 0
}

func drawSurvivalParticles(hdc uintptr, w, h int32) {
	if gameMeta.ParticleQuality == 0 {
		return
	}
	r := arenaRect(w, h)
	count := 70
	if gameMeta.ParticleQuality == 2 {
		count = 120
	}
	t := survivalVisualElapsedSeconds()
	aw, ah := float64(r.Right-r.Left), float64(r.Bottom-r.Top)
	bluePen := survivalPen(1, rgb(110, 180, 255))
	whitePen := survivalPen(1, rgb(205, 225, 255))
	if bluePen == 0 || whitePen == 0 {
		return
	}
	// Batch all particles through two persistent pens. The previous path allocated
	// and destroyed 70–120 GDI pens every frame, which could become a major hitch source.
	for pass := 0; pass < 2; pass++ {
		pen := bluePen
		if pass == 1 {
			pen = whitePen
		}
		old, _, _ := selectObject.Call(hdc, pen)
		for i := 0; i < count; i++ {
			isWhite := i%4 == 0
			if (pass == 1) != isWhite {
				continue
			}
			sx0 := math.Mod(math.Abs(math.Sin(float64(i)*17.731+1.7)*43758.5453), 1)
			sy0 := math.Mod(math.Abs(math.Sin(float64(i)*37.113+4.9)*24634.6345), 1)
			speed := 22.0 + float64((i*13)%42)
			travel := math.Mod(sx0*aw+t*speed, aw)
			x := float64(r.Right) - travel
			y := float64(r.Top) + sy0*ah
			length := int32(2 + (i%3)*2)
			moveToEx.Call(hdc, uintptr(int32(x)-length), uintptr(int32(y)), 0)
			lineTo.Call(hdc, uintptr(int32(x)+1), uintptr(int32(y)))
		}
		selectObject.Call(hdc, old)
	}
}

func drawSurvivalArena(hdc uintptr, w, h int32) {
	r := arenaRect(w, h)
	if survivalBoss1SectionReport && state == StateResult {
		fillSolidRect(hdc, r, rgb(0, 0, 0))
		return
	}
	savedClip, _, _ := saveDC.Call(hdc)
	intersectClipRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom))
	if savedClip != 0 {
		defer restoreDC.Call(hdc, savedClip)
	}
	if survivalBoss1OwnsArena() {
		drawSurvivalBoss1Arena(hdc, w, h)
		return
	}
	if survivalBoss2OwnsArena() {
		drawSurvivalBoss2Arena(hdc, w, h)
		return
	}
	if survivalBoss3OwnsArena() {
		drawSurvivalBoss3Arena(hdc, w, h)
		return
	}
	bgIndex := 0
	visibleWave := survivalDisplayWave()
	if visibleWave >= 21 {
		bgIndex = 2
	} else if visibleWave >= 11 {
		bgIndex = 1
	}
	prewarmSurvivalRuntimeSprites(hdc)
	if b := survivalBackgrounds[bgIndex]; len(b) >= 1508*592*4 {
		wid := r.Right - r.Left
		off := int32(0)
		if backgroundMotionEnabled() {
			elapsed := survivalVisualElapsedSeconds()
			off = int32(math.Mod(elapsed*22.0, float64(wid)))
		}
		// v265: adjacent scrolling tiles overlap by 2 px to prevent a raster seam.
		// Sector 2's texture is also authored as a horizontally wrapping texture.
		drawRawBGRACover(hdc, b, 1508, 592, RECT{r.Left - off, r.Top, r.Right - off + 2, r.Bottom})
		drawRawBGRACover(hdc, b, 1508, 592, RECT{r.Left - off + wid - 2, r.Top, r.Right - off + wid, r.Bottom})
	} else {
		fillSolidRect(hdc, r, rgb(2, 7, 20))
	}
	drawSurvivalParticles(hdc, w, h)
	c := survivalArenaCenter(w, h)
	// One exact integer anchor drives both rings and the station. This avoids the
	// one-pixel drift that can appear when separate float-to-int conversions differ.
	cx, cy := int32(math.Round(c.X)), int32(math.Round(c.Y))
	impact := int32(survivalImpactRadius(w, h))
	orbit := int32(survivalOrbitalRadius(w, h))
	drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(255, 80, 80)), cx, cy, impact)
	// Orbital ring pulses like a station energy field charging and discharging.
	now := time.Now()
	pulse := 0.5 + 0.5*math.Sin(float64(now.UnixNano())/1e9*3.4)
	drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(72, 198, 255)), cx, cy, orbit)
	if pulse > 0.58 {
		extra := int32(2 + 4*pulse)
		drawSurvivalCircleWithPen(hdc, survivalPen(1, rgb(35, 135, 225)), cx, cy, orbit+extra)
	}
	// Threat intensity: as enemies enter the inner half of the orbital field the defence
	// ring gains a restrained warning pulse. Music volume remains controlled only by Settings.
	closest := float64(1e9)
	for _, e := range survivalEnemies {
		if d := math.Hypot(e.X-c.X, e.Y-c.Y); d < closest {
			closest = d
		}
	}
	if closest < float64(orbit)*.72 {
		threatPulse := 0.5 + 0.5*math.Sin(float64(now.UnixNano())/1e9*6.4)
		if threatPulse > .42 {
			drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(255, 110, 45)), cx, cy, orbit-sx(5, w))
		}
	}

	// Station health state.
	station := survivalStationPerfectBGRA
	if survivalHP <= 2 {
		station = survivalStationHeavyBGRA
	} else if survivalHP <= 4 {
		station = survivalStationDamagedBGRA
	}
	sz := sx(112, w)
	stationX := cx + sx(9, w)
	// The damaged source sprites have progressively more transparent padding on
	// their left side than station_perfect.bgra. Counter that source-art offset so
	// taking damage never makes the station appear to jump sideways.
	if survivalHP <= 2 {
		stationX -= sx(15, w)
	} else if survivalHP <= 4 {
		stationX -= sx(7, w)
	}
	if survivalHP == 1 {
		alarm := 0.5 + 0.5*math.Sin(float64(now.UnixNano())/1e9*9.5)
		if alarm > .48 && len(survivalStationCriticalBGRA) >= 256*256*4 {
			station = survivalStationCriticalBGRA
		}
		drawRawBGRAFit(hdc, station, 256, 256, RECT{stationX - sz, cy - sz, stationX + sz, cy + sz})
		if alarm > .40 {
			drawSurvivalCircleWithPen(hdc, survivalPen(4, rgb(255, 38, 32)), cx, cy, impact+sx(9, w))
			drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(255, 105, 65)), cx, cy, orbit+sx(6, w))
		}
	} else {
		drawRawBGRAFit(hdc, station, 256, 256, RECT{stationX - sz, cy - sz, stationX + sz, cy + sz})
	}

	for _, pu := range []*SurvivalPickup{survivalPickup, survivalPickup2} {
		if pu == nil {
			continue
		}
		age := now.Sub(pu.SpawnedAt).Seconds()
		visible := age < 3.5 || (age < 4.4 && int(age*6)%2 == 0) || (age >= 4.4 && int(age*14)%2 == 0)
		if visible {
			d, iw, ih := survivalPickupSprite(pu.Kind)
			rad := int32(survivalPickupRadius(pu.Kind, w))
			drawRawBGRAFit(hdc, d, iw, ih, RECT{int32(pu.X) - rad, int32(pu.Y) - rad, int32(pu.X) + rad, int32(pu.Y) + rad})
		}
	}

	for _, e := range survivalEnemies {
		rad := int32(survivalEnemyVisualRadius(e, w))
		if !drawSurvivalExpansionEnemy(hdc, e, w, h, now) {
			d, iw, ih := survivalEnemySprite(e)
			drawRawBGRAFit(hdc, d, iw, ih, RECT{int32(e.X) - rad, int32(e.Y) - rad, int32(e.X) + rad, int32(e.Y) + rad})
		}
		if e.Large && e.HP == 1 {
			drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(255, 220, 70)), int32(e.X), int32(e.Y), rad-3)
		}
		if e.Elite == survivalEliteArmoured {
			drawSurvivalCircleWithPen(hdc, survivalPen(3, rgb(255, 202, 62)), int32(e.X), int32(e.Y), rad+sx(6, w))
			drawSurvivalCircleWithPen(hdc, survivalPen(1, rgb(255, 241, 170)), int32(e.X), int32(e.Y), rad+sx(11, w))
		} else if e.Elite == survivalEliteStriker {
			p := 0.5 + 0.5*math.Sin(float64(now.UnixNano())/1e9*7.0+float64(int(e.X+e.Y)%9))
			col := rgb(194, 86, 255)
			if p > .5 {
				col = rgb(88, 220, 255)
			}
			drawSurvivalCircleWithPen(hdc, survivalPen(2, col), int32(e.X), int32(e.Y), rad+sx(8, w))
		}
		if e.Critical {
			core := sx(10, w)
			col := rgb(90, 210, 255)
			if e.Red {
				col = rgb(255, 110, 75)
			}
			drawSurvivalCircleWithPen(hdc, survivalPen(2, col), int32(e.X), int32(e.Y), core)
		}
	}
	drawSurvivalSector3FieldEvent(hdc, w, h, now)

	// Only stacked shields orbit the station.
	shieldPts, shieldCount := survivalShieldPositions(w, h, now)
	for i := 0; i < shieldCount; i++ {
		sp := shieldPts[i]
		rr := sx(29, w)
		drawRawBGRAFit(hdc, survivalShieldBGRA, 96, 96, RECT{int32(sp.X) - rr, int32(sp.Y) - rr, int32(sp.X) + rr, int32(sp.Y) + rr})
	}

	// Short, lightweight elimination burst.
	for _, fx := range survivalEliminations {
		age := now.Sub(fx.Started).Seconds()
		if age < 0 || age > .36 {
			continue
		}
		t := age / .36
		rr := int32(float64(sx(12, w)) + float64(sx(28, w))*t)
		cfx := rgb(70, 190, 255)
		if fx.Red {
			cfx = rgb(255, 82, 72)
		}
		fxPen := survivalPen(2, cfx)
		drawSurvivalCircleWithPen(hdc, fxPen, int32(fx.X), int32(fx.Y), rr)
		for k := 0; k < 6; k++ {
			a := float64(k)*math.Pi/3 + t*.7
			r1 := float64(rr) * .45
			r2 := float64(rr) * .95
			drawSurvivalLineWithPen(hdc, fxPen, int32(fx.X+math.Cos(a)*r1), int32(fx.Y+math.Sin(a)*r1), int32(fx.X+math.Cos(a)*r2), int32(fx.Y+math.Sin(a)*r2))
		}
	}

	// Chain lightning is resolved instantly for fairness, but each link flashes in sequence.
	arcOut := survivalArcFX[:0]
	for _, a := range survivalArcFX {
		age := now.Sub(a.Started)
		if age < 0 {
			arcOut = append(arcOut, a)
			continue
		}
		if age < 260*time.Millisecond {
			pen := survivalPen(3, rgb(120, 225, 255))
			drawSurvivalLineWithPen(hdc, pen, int32(a.X1), int32(a.Y1), int32(a.X2), int32(a.Y2))
			arcOut = append(arcOut, a)
		}
	}
	survivalArcFX = arcOut

	if !survivalWaveBannerUntil.IsZero() && now.Before(survivalWaveBannerUntil) && survivalWaveBannerText != "" {
		// v360: warning/banner width is derived from the rendered text. The side rails
		// therefore expand and contract with the message instead of clipping long warnings.
		bh := sy(46, h)
		bw := sx(250, w)
		if hudSmallFont != 0 {
			tw := textPixelSize(hdc, hudSmallFont, survivalWaveBannerText).Cx + sx(72, w)
			if tw > bw {
				bw = tw
			}
		}
		maxW := (r.Right - r.Left) - sx(44, w)
		if bw > maxW {
			bw = maxW
		}
		br := RECT{cx - bw/2, r.Top + sy(25, h), cx + bw/2, r.Top + sy(25, h) + bh}
		fillSolidRect(hdc, br, rgb(3, 17, 44))
		drawOutlineRect(hdc, br, rgb(255, 184, 38), 2)
		// Warning side caps track the computed box edges.
		cap := sx(13, w)
		drawLineSimple(hdc, br.Left-cap, br.Top+bh/2, br.Left-2, br.Top+bh/2, 3, rgb(255, 184, 38))
		drawLineSimple(hdc, br.Right+2, br.Top+bh/2, br.Right+cap, br.Top+bh/2, 3, rgb(255, 184, 38))
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 212, 65))
			centeredTextOut(hdc, br.Left+sx(18, w), br.Right-sx(18, w), verticallyCenteredTextY(hdc, hudSmallFont, br, survivalWaveBannerText), survivalWaveBannerText)
			selectObject.Call(hdc, old)
		}
	}

	drawSurvivalBoss1TransitionOverlay(hdc, w, h)
	drawSurvivalBoss2TransitionOverlay(hdc, w, h)

	// Arena intentionally contains no duplicate stat text; HUD cards own this information.

}

func drawSurvivalGameFrame(hdc uintptr, w, h int32) {
	setBkMode.Call(hdc, TRANSPARENT)
	drawUIBase(hdc, w, h)
	drawSurvivalArena(hdc, w, h)
	drawPlayfieldBorder(hdc, w, h)
	drawGlobalAnnouncementBar(hdc, w, h)
	drawSurvivalActiveBossHealthBar(hdc, w, h)

	// Reuse the existing premium HUD layout, but give each card Survival semantics.
	dark := rgb(4, 27, 72)
	cyan := rgb(58, 224, 255)
	white := rgb(250, 252, 255)
	hudCards := []RECT{{sx(585, w), sy(7, h), sx(755, w), sy(124, h)}, {sx(770, w), sy(7, h), sx(940, w), sy(124, h)}, {sx(955, w), sy(7, h), sx(1125, w), sy(124, h)}, {sx(1140, w), sy(7, h), sx(1320, w), sy(124, h)}, {sx(1335, w), sy(7, h), sx(1513, w), sy(124, h)}}
	for _, card := range hudCards {
		fillSolidRect(hdc, RECT{card.Left + 14, sy(53, h), card.Right - 14, sy(119, h)}, dark)
	}
	if hudStatFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudStatFont)
		setTextColor.Call(hdc, cyan)
		if survivalBoss1Active() {
			textOut(hdc, sx(621, w), sy(70, h), "BOSS")
		} else {
			textOut(hdc, sx(621, w), sy(70, h), fmt.Sprintf("W%d", survivalDisplayWave()))
		}
		textOut(hdc, sx(811, w), sy(70, h), fmt.Sprint(score))
		setTextColor.Call(hdc, white)
		// v358: STATION HP remains station HP during bosses. Boss health is represented
		// only by the dedicated bar below the arena; no numeric boss HP is shown.
		drawSurvivalHPHearts(hdc, w, h, RECT{sx(978, w), sy(57, h), sx(1105, w), sy(116, h)}, 3)
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, cyan)
		textOut(hdc, sx(1167, w), sy(75, h), fmt.Sprintf("CP %d", survivalReplayCheckpointValue()))
		setTextColor.Call(hdc, rgb(255, 78, 78))
		centeredTextOut(hdc, sx(1349, w), sx(1499, w), sy(69, h), "SURVIVAL")
		selectObject.Call(hdc, old)
	}
	drawAdvancedMissionPanel(hdc, w, h)
	drawAdvancedObjectivePanel(hdc, w, h)
	drawQuickAccessDock(hdc, w, h)
	drawQuickBugReport(hdc, w, h)
	drawQuickSupportDev(hdc, w, h)
	drawQuickDiscordLogin(hdc, w, h)
	// Draw the Monster Guide after the lower HUD panels so the dedicated button
	// cannot be painted over by the centre objective card.
	drawSurvivalGuideButton(hdc, w, h)
	drawModeSelectorDropdown(hdc, w, h)
	// v370: keep the Kong Game watermark consistent across Precision, Endurance, and Survival.
	drawKongGameWatermark(hdc, w, h)
	// v275: legacy orange training menu is fully retired in Survival.
	// ESC now owns menu/settings/account navigation.
	drawResultOverlay(hdc, w, h)
	drawSurvivalSectionClearReport(hdc, w, h)
	drawLeaderboardOverlay(hdc, w, h)
	drawGlobalLeaderboardOverlay(hdc, w, h)
	drawRemoteProfileOverlay(hdc, w, h)
	drawProfileSkinGallery(hdc, w, h)
	drawDifficultyLockedOverlay(hdc, w, h)
	drawSurvivalSectionLockedOverlay(hdc, w, h)
	// Survival has its own top-level renderer. Keep shared overlays here as well;
	// otherwise their state changes succeed but nothing is drawn because
	// drawGameFrame() returns before reaching the shared overlay calls.
	drawSurvivalMonsterGuide(hdc, w, h)
	drawAFKSingularityOverlay(hdc, w, h)
	drawDeveloperConsoleOverlay(hdc, w, h)
	drawNameEntryOverlay(hdc, w, h)
	drawMainMenuOverlay(hdc, w, h)
	drawProfileOverlay(hdc, w, h)
	drawSettingsOverlay(hdc, w, h)
	drawTutorialOverlay(hdc, w, h)
	drawAchievementsOverlay(hdc, w, h)
	drawReleaseNotesOverlay(hdc, w, h)
	drawAchievementToast(hdc, w, h)
	drawRewardToast(hdc, w, h)
	if (state == StateWaiting || state == StatePlaying) && overlayMode == OverlayNone && !survivalBossIntroActive() && !pointInMenuUI(cursorPos, w, h) && (!survivalBoss1Active() || survivalBoss1CombatActive()) {
		drawSurvivalCrosshair(hdc)
	}
	drawFailedOverlay(hdc, w, h)
	drawSurvivalRespawnBox(hdc, w, h)
	// The dossier cinematic is intentionally last: it replaces the full HUD/game
	// frame, matching the supplied full-screen military-intro reference.
	drawSurvivalBossIntroOverlay(hdc, w, h)
}

func drawSurvivalCrosshair(hdc uintptr) {
	if !cursorInArena {
		return
	}
	x, y := int32(math.Round(cursorPos.X)), int32(math.Round(cursorPos.Y))
	white := rgb(255, 255, 255)
	ringR, arm, gap := int32(11), int32(15), int32(3)
	penWidth := int32(2)
	// The Terminus fight is deliberately zoomed out: shrink the cursor with the
	// rest of the gameplay layer so the capital ship retains its enormous scale.
	if survivalBoss3Active() {
		ringR, arm, gap = 7, 10, 2
		penWidth = 1
	}
	pen := survivalPen(penWidth, white)
	drawSurvivalLineWithPen(hdc, pen, x-arm, y, x-gap, y)
	drawSurvivalLineWithPen(hdc, pen, x+gap, y, x+arm, y)
	drawSurvivalLineWithPen(hdc, pen, x, y-arm, x, y-gap)
	drawSurvivalLineWithPen(hdc, pen, x, y+gap, x, y+arm)
	drawSurvivalCircleWithPen(hdc, pen, x, y, ringR)
}

// drawCircleOutline uses the game's existing GDI line primitive to avoid allocating brushes/pens per frame.
func drawCircleOutline(hdc uintptr, cx, cy, radius, width int32, color uintptr) {
	if hdc == 0 || radius <= 0 {
		return
	}
	if width < 1 {
		width = 1
	}
	p, _, _ := createPen.Call(PS_SOLID, uintptr(width), color)
	if p == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, p)
	const seg = 64
	for i := 0; i <= seg; i++ {
		a := float64(i) / seg * 2 * math.Pi
		x := cx + int32(math.Cos(a)*float64(radius))
		y := cy + int32(math.Sin(a)*float64(radius))
		if i == 0 {
			moveToEx.Call(hdc, uintptr(x), uintptr(y), 0)
		} else {
			lineTo.Call(hdc, uintptr(x), uintptr(y))
		}
	}
	selectObject.Call(hdc, old)
	deleteObject.Call(p)
}

func survivalMusicPath() string { return survivalSectionMusicPath(1) }

func survivalSectionMusicPath(section int) string {
	if section < 1 {
		section = 1
	}
	if section > 3 {
		section = 3
	}
	return filepath.Join(externalAsset("audio"), fmt.Sprintf("survival_section_%d.mp3", section))
}

func survivalSectionMusicFallbackPath(section int) string {
	if section < 1 {
		section = 1
	}
	if section > 3 {
		section = 3
	}
	return filepath.Join(externalAsset("audio"), fmt.Sprintf("survival_section_%d.mp3", section))
}

func survivalMusicSectionForWave(wave int) int {
	if wave >= 21 {
		return 3
	}
	if wave >= 11 {
		return 2
	}
	return 1
}

func survivalMusicAliasForWave(wave int) string {
	switch survivalMusicSectionForWave(wave) {
	case 3:
		if survivalSection3MusicReady {
			return "survival_section3"
		}
	case 2:
		if survivalSection2MusicReady {
			return "survival_section2"
		}
	}
	if survivalMusicReady {
		return "survival_music"
	}
	return ""
}

func survivalCurrentMusicAlias() string {
	wave := survivalWave
	if state == StateWaiting {
		wave = survivalDisplayWave()
	}
	return survivalMusicAliasForWave(wave)
}

var survivalBossHPFlashUntil time.Time
var survivalBossHPLastHits = -1

func survivalBossHPRegisterHits(hits int) {
	if survivalBossHPLastHits < 0 {
		survivalBossHPLastHits = hits
		return
	}
	if hits > survivalBossHPLastHits {
		survivalBossHPFlashUntil = time.Now().Add(220 * time.Millisecond)
	}
	survivalBossHPLastHits = hits
}

func drawSurvivalBossHealthBar(hdc uintptr, w, h int32, name string, remain, total int, fillCol, accent uintptr) {
	if total <= 0 {
		return
	}
	if remain < 0 {
		remain = 0
	}
	if remain > total {
		remain = total
	}
	barW := sx(544, w)
	barH := sy(28, h)
	left := sx(478, w)
	top := sy(784, h)
	bar := RECT{left, top, left + barW, top + barH}
	flash := time.Now().Before(survivalBossHPFlashUntil)
	theme := gameMeta.BossHPBarTheme
	frameAccent := accent
	activeFill := fillCol
	if theme == 1 {
		frameAccent = rgb(255, 72, 72)
		activeFill = rgb(225, 28, 45)
	}
	if flash {
		frameAccent = rgb(255, 45, 45)
		activeFill = rgb(255, 35, 35)
	}

	if theme == 3 { // clean minimalist
		fillSolidRect(hdc, bar, rgb(3, 13, 28))
		drawOutlineRect(hdc, bar, frameAccent, 1)
	} else {
		drawBevelPanel(hdc, bar, rgb(4, 24, 58), frameAccent, rgb(1, 10, 28), 3)
	}
	inner := RECT{bar.Left + sx(7, w), bar.Top + sy(7, h), bar.Right - sx(7, w), bar.Bottom - sy(7, h)}
	if theme == 3 {
		inner = RECT{bar.Left + sx(5, w), bar.Top + sy(9, h), bar.Right - sx(5, w), bar.Bottom - sy(9, h)}
	}
	fillSolidRect(hdc, inner, rgb(20, 9, 22))
	fillRight := inner.Left + int32(float64(inner.Right-inner.Left)*float64(remain)/float64(total))
	if fillRight > inner.Left {
		switch theme {
		case 1: // red plasma
			fillSolidRect(hdc, RECT{inner.Left, inner.Top, fillRight, inner.Bottom}, activeFill)
			pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*7.0)
			alphaSolidRect(hdc, RECT{inner.Left, inner.Top, fillRight, inner.Bottom}, rgb(255, 120, 80), byte(20+int(34*pulse)))
			alphaSolidRect(hdc, RECT{inner.Left, inner.Top + 1, fillRight, inner.Top + sy(4, h)}, rgb(255, 255, 255), 100)
		case 2: // segmented arcade
			segments := 10
			gap := sx(3, w)
			sw := (inner.Right - inner.Left - gap*int32(segments-1)) / int32(segments)
			for i := 0; i < segments; i++ {
				segL := inner.Left + int32(i)*(sw+gap)
				segR := segL + sw
				if segL >= fillRight {
					break
				}
				if segR > fillRight {
					segR = fillRight
				}
				fillSolidRect(hdc, RECT{segL, inner.Top, segR, inner.Bottom}, activeFill)
				alphaSolidRect(hdc, RECT{segL, inner.Top + 1, segR, inner.Top + sy(3, h)}, rgb(255, 255, 255), 75)
			}
		case 3: // clean minimalist
			fillSolidRect(hdc, RECT{inner.Left, inner.Top, fillRight, inner.Bottom}, activeFill)
		default: // boss-specific
			fillSolidRect(hdc, RECT{inner.Left, inner.Top, fillRight, inner.Bottom}, activeFill)
			alphaSolidRect(hdc, RECT{inner.Left, inner.Top + 1, fillRight, inner.Top + sy(4, h)}, rgb(255, 255, 255), 80)
		}
	}
	if theme != 3 {
		drawOutlineRect(hdc, inner, rgb(250, 225, 190), 1)
	}
	_ = name
}
func drawSurvivalActiveBossHealthBar(hdc uintptr, w, h int32) {
	// Convert the encounter's required successful interactions into one 100% bar.
	// No numeric HP is displayed: each valid boss hit/objective removes its exact
	// fraction of the full bar.
	if survivalBoss1Active() || survivalBoss1OwnsArena() {
		survivalBossHPRegisterHits(survivalBoss1Hits)
		remain := survivalBoss1TotalHits - survivalBoss1Hits
		drawSurvivalBossHealthBar(hdc, w, h, "THE SENTINEL", remain, survivalBoss1TotalHits, rgb(232, 58, 62), rgb(255, 190, 70))
		return
	}
	if survivalBoss2Active() || survivalBoss2OwnsArena() {
		survivalBossHPRegisterHits(survivalBoss2Hits)
		remain := survivalBoss2TotalHits - survivalBoss2Hits
		drawSurvivalBossHealthBar(hdc, w, h, "THE VOID SERPENT", remain, survivalBoss2TotalHits, rgb(142, 62, 235), rgb(220, 100, 255))
		return
	}
	if survivalBoss3Active() || survivalBoss3OwnsArena() {
		remain, total := survivalBoss3HealthUnits()
		survivalBossHPRegisterHits(total - remain)
		drawSurvivalBossHealthBar(hdc, w, h, "THE TERMINUS", remain, total, rgb(230, 55, 48), rgb(255, 172, 42))
		return
	}
	survivalBossHPLastHits = -1
}

// drawSurvivalHPHearts renders all six station HP slots at all times. Filled
// hearts represent current HP; supplied grey hearts represent lost HP. Because the
// same six slots are redrawn from survivalHP every frame, repair pickups immediately
// light the appropriate hearts back up without any extra state bookkeeping.
func drawSurvivalHPHearts(hdc uintptr, w, h int32, box RECT, columns int) {
	totalHP := survivalMaxHP()
	if columns <= 0 {
		columns = totalHP
	}
	if columns > totalHP {
		columns = totalHP
	}
	rows := (totalHP + columns - 1) / columns
	gapX := sx(4, w)
	gapY := sy(3, h)
	cellW := (box.Right - box.Left - int32(columns-1)*gapX) / int32(columns)
	cellH := (box.Bottom - box.Top - int32(rows-1)*gapY) / int32(rows)
	if cellW <= 0 || cellH <= 0 {
		return
	}
	for i := 0; i < totalHP; i++ {
		col := i % columns
		row := i / columns
		left := box.Left + int32(col)*(cellW+gapX)
		top := box.Top + int32(row)*(cellH+gapY)
		iconBox := RECT{left, top, left + cellW, top + cellH}
		asset := survivalHPEmptyBGRA
		if i < survivalHP {
			asset = survivalHPActiveBGRA
		}
		if len(asset) >= 128*128*4 {
			drawRawBGRAFit(hdc, asset, 128, 128, iconBox)
		}
	}
}

func drawSurvivalMissionDock(hdc uintptr, w, h int32) {
	left := sx(28, w)
	right := sx(444, w)
	r := RECT{left, sy(806, h), right, sy(970, h)}

	// During a boss phase this card becomes a centred boss briefing rather than
	// continuing to show generic Survival defence statistics.
	bossName, bossDesc := "", ""
	accent, face, deep := rgb(255, 72, 72), rgb(5, 43, 101), rgb(2, 22, 58)
	phase := ""
	switch {
	case survivalBoss1Active() || survivalBoss1OwnsArena():
		bossName = "THE SENTINEL"
		bossDesc = "Break exposed nodes and survive each weapon cycle to collapse the Sentinel core."
		phase = survivalBoss1PhaseLabel()
		accent, face, deep = rgb(255, 111, 52), rgb(77, 29, 18), rgb(31, 10, 8)
	case survivalBoss2Active() || survivalBoss2OwnsArena():
		bossName = "THE VOID SERPENT"
		bossDesc = "Strike the coloured cores fixed to its body, dodge the serpent, and breach the meteor wall."
		phase = survivalBoss2PhaseLabel()
		accent, face, deep = rgb(205, 92, 255), rgb(54, 20, 79), rgb(19, 7, 34)
	case survivalBoss3Active() || survivalBoss3OwnsArena():
		bossName = "THE TERMINUS"
		bossDesc = "Clear three osu-style mechanics from left to right, drive the warship back, then attack while it flashes red."
		phase = survivalBoss3PhaseLabel()
		accent, face, deep = rgb(245, 72, 54), rgb(55, 25, 28), rgb(20, 8, 13)
	}

	if bossName != "" {
		drawBevelPanel(hdc, r, face, accent, deep, 4)
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, accent)
			titleR := RECT{r.Left + sx(14, w), r.Top + sy(12, h), r.Right - sx(14, w), r.Top + sy(42, h)}
			centeredTextOut(hdc, titleR.Left, titleR.Right, verticallyCenteredTextY(hdc, hudSmallFont, titleR, bossName), bossName)
			selectObject.Call(hdc, old)
		}
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 255, 255))
			descR := RECT{r.Left + sx(22, w), r.Top + sy(47, h), r.Right - sx(22, w), r.Top + sy(119, h)}
			drawWrappedCenteredText(hdc, hudTinyFont, descR, bossDesc, sy(3, h))
			setTextColor.Call(hdc, accent)
			phaseR := RECT{r.Left + sx(18, w), r.Top + sy(125, h), r.Right - sx(18, w), r.Bottom - sy(10, h)}
			centeredTextOut(hdc, phaseR.Left, phaseR.Right, verticallyCenteredTextY(hdc, hudTinyFont, phaseR, phase), phase)
			selectObject.Call(hdc, old)
		}
		return
	}

	drawBevelPanel(hdc, r, rgb(5, 43, 101), rgb(255, 72, 72), rgb(2, 22, 58), 4)
	if hudSmallFont == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, hudSmallFont)
	defer selectObject.Call(hdc, old)
	setBkMode.Call(hdc, TRANSPARENT)
	setTextColor.Call(hdc, rgb(255, 92, 92))
	textOut(hdc, r.Left+sx(16, w), r.Top+sy(13, h), "SURVIVAL DEFENCE")
	setTextColor.Call(hdc, rgb(255, 255, 255))
	drawSurvivalHPHearts(hdc, w, h, RECT{r.Left + sx(15, w), r.Top + sy(39, h), r.Left + sx(245, w), r.Top + sy(73, h)}, 6)
	textOut(hdc, r.Left+sx(16, w), r.Top+sy(78, h), fmt.Sprintf("WAVE         %d", survivalDisplayWave()))
	textOut(hdc, r.Left+sx(16, w), r.Top+sy(107, h), fmt.Sprintf("KILLS        %d", survivalKills))
	textOut(hdc, r.Left+sx(16, w), r.Top+sy(136, h), fmt.Sprintf("REACTION     %.0f ms   COMBO x%d", survivalAverageReactionMS(), survivalCombo))

	// Replay selector is shown only before a Survival run starts. Boss-entry
	// checkpoints and sector starts share one clipped horizontal carousel.
	if state == StateWaiting {
		clampSurvivalCheckpointScroll(w, h)
		view := survivalCheckpointViewportRect(w, h)
		fillSolidRect(hdc, RECT{view.Left - sx(3, w), view.Top - sy(3, h), view.Right + sx(3, w), sy(1021, h)}, rgb(2, 12, 30))
		drawOutlineRect(hdc, RECT{view.Left - sx(3, w), view.Top - sy(3, h), view.Right + sx(3, w), sy(1021, h)}, rgb(24, 112, 179), 1)

		saved, _, _ := saveDC.Call(hdc)
		if saved != 0 {
			intersectClipRect.Call(hdc, uintptr(view.Left), uintptr(view.Top), uintptr(view.Right), uintptr(view.Bottom))
		}
		buttons := survivalCheckpointButtonRects(w, h)
		for i, sr := range buttons {
			if sr.Right <= view.Left || sr.Left >= view.Right {
				continue
			}
			cp := survivalReplayCheckpoints[i]
			unlocked := survivalReplayCheckpointUnlocked(i)
			selected := cp.Wave == survivalSelectedStartWave && cp.Boss == survivalSelectedStartBoss
			if survivalSelectedStartWave <= 0 {
				raw := survivalReplayCheckpointValue()
				selected = cp.Wave == raw
			}
			face := rgb(5, 34, 77)
			light := rgb(36, 130, 215)
			dark := rgb(2, 15, 38)
			textCol := rgb(190, 220, 245)
			if cp.Boss > 0 {
				face = rgb(45, 20, 22)
				light = rgb(205, 66, 66)
				textCol = rgb(255, 188, 188)
			}
			if !unlocked {
				face = rgb(17, 24, 36)
				light = rgb(66, 75, 90)
				dark = rgb(5, 8, 14)
				textCol = rgb(105, 115, 128)
			}
			if selected && unlocked {
				if cp.Boss > 0 {
					face = rgb(94, 28, 30)
					light = rgb(255, 92, 72)
				} else {
					face = rgb(9, 66, 100)
					light = rgb(75, 231, 255)
				}
				textCol = rgb(255, 255, 255)
			}
			drawBevelPanel(hdc, sr, face, light, dark, 2)
			setTextColor.Call(hdc, textCol)
			centeredTextOut(hdc, sr.Left, sr.Right, verticallyCenteredTextY(hdc, hudSmallFont, sr, cp.Label), cp.Label)
		}
		if saved != 0 {
			restoreDC.Call(hdc, saved)
		}

		track, thumb := survivalCheckpointScrollbarRects(w, h)
		fillSolidRect(hdc, track, rgb(8, 24, 48))
		drawOutlineRect(hdc, track, rgb(25, 75, 118), 1)
		fillSolidRect(hdc, thumb, rgb(34, 169, 226))
		drawOutlineRect(hdc, thumb, rgb(132, 238, 255), 1)
	}
}
func drawSurvivalSectionLockedOverlay(hdc uintptr, w, h int32) {
	if overlayMode != OverlaySurvivalSectionLocked {
		return
	}
	drawModalBackdrop(hdc, w, h)
	index := survivalLockedSectionPopup - 1
	if index < 0 || index >= len(survivalReplayCheckpoints) {
		index = 1
	}
	cp := survivalReplayCheckpoints[index]

	r := centeredPanel(w, h, 700, 320)
	drawPanelFrame(hdc, r, "CHECKPOINT LOCKED", "SURVIVAL PROGRESSION")

	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 80, 80))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(94, h), cp.Label+" IS LOCKED")
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(245, 249, 252))
		requirement := "Reach this checkpoint in Survival to unlock replay access."
		switch index {
		case 1:
			requirement = "Clear Wave 10 to unlock direct replay of THE SENTINEL."
		case 2:
			requirement = "Defeat THE SENTINEL to unlock Sector 2."
		case 3:
			requirement = "Clear Wave 20 to unlock direct replay of THE VOID SERPENT."
		case 4:
			requirement = "Defeat THE VOID SERPENT to unlock Sector 3."
		case 5:
			requirement = "Clear Wave 30 to unlock direct replay of THE TERMINUS 1337."
		}
		centeredTextOut(hdc, r.Left+sx(30, w), r.Right-sx(30, w), r.Top+sy(154, h), requirement)
		setTextColor.Call(hdc, rgb(150, 195, 225))
		centeredTextOut(hdc, r.Left+sx(30, w), r.Right-sx(30, w), r.Top+sy(206, h), "Boss-entry and post-boss checkpoints are saved permanently.")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 210, 65))
		centeredTextOut(hdc, r.Left, r.Right, r.Bottom-sy(38, h), "CLICK ANYWHERE TO CLOSE")
		selectObject.Call(hdc, old)
	}
}
