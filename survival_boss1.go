//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// Sentinel / Boss 1 is deliberately isolated from the normal Survival wave director.
// This makes boss mechanics easy to test and prevents normal station/enemy/power-up
// logic from accidentally running during the player-vs-boss encounter.
const (
	survivalBoss1None = iota
	survivalBoss1FadeOut
	survivalBoss1FadeIn
	survivalBoss1Intro
	survivalBoss1Phase1
	survivalBoss1Reconfig1
	survivalBoss1Phase2
	survivalBoss1Reconfig2
	survivalBoss1Phase3
	survivalBoss1CriticalTelegraph
	survivalBoss1CriticalLocked
	survivalBoss1FinalCore
	survivalBoss1CycleReset
	survivalBoss1Dying
	survivalBoss1FadeToReport
	survivalBoss1ReturnFadeIn
)

const (
	survivalBoss1AppearanceNone = iota
	survivalBoss1AppearanceNormalToDamaged
	survivalBoss1AppearanceDamagedToCore
	survivalBoss1AppearanceCoreToDamaged
)

const (
	// Each rotation has 8 armour-node hits and 10 returning core-orb hits.
	// The exposed core is a rapid-destruction sequence: 10 hits after the
	// first ring phase and 20 hits for the final kill sequence.
	survivalBoss1CoreOrbCount      = 10
	survivalBoss1FirstCoreSpamHits = 10
	survivalBoss1FinalCoreSpamHits = 20
	survivalBoss1TotalHits         = (8 + survivalBoss1CoreOrbCount + survivalBoss1FirstCoreSpamHits) +
		(8 + survivalBoss1CoreOrbCount + survivalBoss1FinalCoreSpamHits)
)

type SurvivalBossCoreOrb struct {
	Angle     float64
	Red       bool
	Destroyed bool
}

type SurvivalBossNode struct {
	DX, DY float64
	Red    bool
	Active bool
}

var (
	survivalBoss1Stage     = survivalBoss1None
	survivalBoss1StageAt   time.Time
	survivalBoss1StartedAt time.Time
	survivalBoss1FightAt   time.Time
	survivalBoss1Hits      int
	survivalBoss1Cycle     int
	survivalBoss1CycleHits int
	survivalBoss1HurtAt    time.Time
	survivalBoss1NodeIndex int
	survivalBoss1NodeRed   bool
	survivalBoss1NodeAt    time.Time
	survivalBoss1LastSwap  time.Time
	// Short white evolution/damage flash around every actual Sentinel sprite change.
	// The old sprite is held for the first beat, then the new sprite appears under a
	// second flash so the swap reads as damage/evolution rather than a hard texture cut.
	survivalBoss1AppearanceFlashAt   time.Time
	survivalBoss1AppearanceFlashKind int
	survivalBoss1SectionReport       bool
	survivalSectionReportSector      int
	survivalBoss1ClearTime           float64
	survivalBoss1SectionTime         float64

	// Grand-finale core-collapse state.
	survivalBoss1CoreOrbs     []SurvivalBossCoreOrb
	survivalBoss1CoreOrbIndex int
	survivalBoss1CoreOrbAt    time.Time
	survivalBoss1CoreBurstAt  time.Time

	// Rapid-click exposed-core state. The colour/button is selected once when
	// the core opens and remains fixed for the entire sequence (never alternates).
	survivalBoss1CoreSpamTotal     int
	survivalBoss1CoreSpamRemaining int
	survivalBoss1CoreSpamAt        time.Time
	survivalBoss1CoreSpamLastClick time.Time

	survivalBoss1NormalBGRA  []byte
	survivalBoss1DamagedBGRA []byte
	survivalBoss1CoreBGRA    []byte
	survivalBoss1Background  []byte

	survivalBoss1MusicReady   bool
	survivalBoss2MusicReady   bool
	survivalBoss3MusicReady   bool
	survivalBoss1FadeSerial   int64
	survivalBoss1MusicCheckAt time.Time
)

func initSurvivalBoss1Assets() {
	survivalBoss1NormalBGRA = readExternalBytes("survival", "boss1", "sentinel_normal.bgra")
	survivalBoss1DamagedBGRA = readExternalBytes("survival", "boss1", "sentinel_damaged.bgra")
	survivalBoss1CoreBGRA = readExternalBytes("survival", "boss1", "sentinel_core_exposed.bgra")
	survivalBoss1Background = readExternalBytes("survival", "boss1", "background.bgra")
}

func survivalBossMusicAlias(boss int) string {
	switch boss {
	case 2:
		return "survival_boss2_music"
	case 3:
		return "survival_boss3_music"
	default:
		return "survival_boss1_music"
	}
}

func survivalBossMusicPath(boss int) string {
	name := "survival_boss1_music.mp3"
	if boss == 2 {
		name = "survival_boss2_music.mp3"
	} else if boss == 3 {
		name = "survival_boss3_music.mp3"
	}
	return filepath.Join(externalAsset("audio"), name)
}

func survivalActiveBossNumber() int {
	if survivalBoss3Active() {
		return 3
	}
	if survivalBoss2Active() {
		return 2
	}
	return 1
}

func survivalActiveBossMusicAlias() string {
	return survivalBossMusicAlias(survivalActiveBossNumber())
}

func initSurvivalBossMusicAlias(boss int) bool {
	p := survivalBossMusicPath(boss)
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return false
	}
	alias := survivalBossMusicAlias(boss)
	mci("stop " + alias)
	mci("close " + alias)
	return mci(`open "` + p + `" type mpegvideo alias ` + alias)
}

func initSurvivalBoss1Audio() {
	if !audioReady {
		return
	}
	survivalBoss1MusicReady = initSurvivalBossMusicAlias(1)
	survivalBoss2MusicReady = initSurvivalBossMusicAlias(2)
	survivalBoss3MusicReady = initSurvivalBossMusicAlias(3)
}

func survivalBossMusicReady(boss int) bool {
	switch boss {
	case 2:
		return survivalBoss2MusicReady
	case 3:
		return survivalBoss3MusicReady
	default:
		return survivalBoss1MusicReady
	}
}

func setSurvivalBossMusicReady(boss int, ready bool) {
	switch boss {
	case 2:
		survivalBoss2MusicReady = ready
	case 3:
		survivalBoss3MusicReady = ready
	default:
		survivalBoss1MusicReady = ready
	}
}

func survivalBoss1Active() bool { return survivalBoss1Stage != survivalBoss1None }

func survivalBoss1PhaseLabel() string {
	switch survivalBoss1Stage {
	case survivalBoss1Phase1:
		if survivalBoss1Cycle >= 2 {
			return "ROUND 2 // PHASE 1"
		}
		return "PHASE 1 // CALIBRATION"
	case survivalBoss1Phase2:
		return "PHASE 2 // NODE MATRIX"
	case survivalBoss1Phase3, survivalBoss1CriticalTelegraph, survivalBoss1CriticalLocked:
		return "PHASE 3 // CORE COLLAPSE"
	case survivalBoss1FinalCore:
		return "FINAL CORE"
	case survivalBoss1Dying, survivalBoss1FadeToReport:
		return "DESTROYED"
	default:
		return "ENCOUNTER"
	}
}

func survivalBoss1CombatActive() bool {
	switch survivalBoss1Stage {
	case survivalBoss1Phase1, survivalBoss1Phase2, survivalBoss1CriticalLocked, survivalBoss1FinalCore:
		return state == StatePlaying
	}
	return false
}

func survivalBoss1OwnsArena() bool {
	switch survivalBoss1Stage {
	case survivalBoss1FadeIn, survivalBoss1Intro, survivalBoss1Phase1, survivalBoss1Reconfig1,
		survivalBoss1Phase2, survivalBoss1Reconfig2, survivalBoss1Phase3,
		survivalBoss1CriticalTelegraph, survivalBoss1CriticalLocked, survivalBoss1FinalCore,
		survivalBoss1CycleReset, survivalBoss1Dying, survivalBoss1FadeToReport:
		return true
	}
	return false
}

func survivalBoss1SuppressNormalLogic() bool {
	return survivalBoss1Active()
}

func survivalBoundaryFailureActive() bool {
	if !survivalActive() || state != StatePlaying || survivalBossIntroActive() {
		return false
	}
	if survivalBoss1Active() {
		return survivalBoss1CombatActive()
	}
	if survivalBoss2Active() {
		return survivalBoss2CombatActive()
	}
	return true
}

func resetSurvivalBoss1State() {
	survivalBoss1Stage = survivalBoss1None
	survivalBoss1StageAt = time.Time{}
	survivalBoss1StartedAt = time.Time{}
	survivalBoss1FightAt = time.Time{}
	survivalBoss1Hits = 0
	survivalBoss1Cycle = 1
	survivalBoss1CycleHits = 0
	survivalBoss1HurtAt = time.Time{}
	survivalBoss1NodeIndex = 0
	survivalBoss1NodeRed = true
	survivalBoss1NodeAt = time.Time{}
	survivalBoss1LastSwap = time.Time{}
	survivalBoss1AppearanceFlashAt = time.Time{}
	survivalBoss1AppearanceFlashKind = survivalBoss1AppearanceNone
	survivalBoss1SectionReport = false
	survivalSectionReportSector = 0
	survivalBoss1ClearTime = 0
	survivalBoss1SectionTime = 0
	survivalBoss1MusicCheckAt = time.Time{}
	survivalBoss1CoreOrbs = nil
	survivalBoss1CoreOrbIndex = 0
	survivalBoss1CoreOrbAt = time.Time{}
	survivalBoss1CoreBurstAt = time.Time{}
	survivalBoss1CoreSpamTotal = 0
	survivalBoss1CoreSpamRemaining = 0
	survivalBoss1CoreSpamAt = time.Time{}
	survivalBoss1CoreSpamLastClick = time.Time{}
}

func survivalBoss1TargetMusicVolume() int {
	v := gameMeta.MusicVolume * 10
	if v < 0 {
		v = 0
	}
	if v > 1000 {
		v = 1000
	}
	return v
}

func survivalBoss1FadeAlias(alias string, from, to int, duration time.Duration, stopAtEnd bool) {
	serial := atomic.AddInt64(&survivalBoss1FadeSerial, 1)
	if duration <= 0 {
		duration = 1
	}
	go func() {
		const steps = 20
		tick := duration / steps
		if tick < 20*time.Millisecond {
			tick = 20 * time.Millisecond
		}
		for i := 0; i <= steps; i++ {
			if atomic.LoadInt64(&survivalBoss1FadeSerial) != serial {
				return
			}
			t := float64(i) / float64(steps)
			v := int(math.Round(float64(from) + (float64(to-from) * t)))
			mci(fmt.Sprintf("setaudio %s volume to %d", alias, v))
			if i < steps {
				time.Sleep(tick)
			}
		}
		if stopAtEnd && atomic.LoadInt64(&survivalBoss1FadeSerial) == serial {
			mci("stop " + alias)
			mci("seek " + alias + " to start")
		}
	}()
}

func survivalBoss1FadeOutNormalMusic() {
	if !audioReady || !survivalMusicReady {
		return
	}
	// Fade Section 1 while the arena itself fades to full black.
	// Do not stop it early and do not touch Boss 1 audio yet.
	survivalBoss1FadeAlias("survival_music", survivalBoss1TargetMusicVolume(), 0, 950*time.Millisecond, false)
}

func survivalBoss1StartMusic() {
	if !audioReady {
		return
	}

	// FULL-BLACK HANDOFF. Kill every normal music owner first.
	mci("stop survival_music")
	mci("stop survival_section2")
	mci("stop survival_section3")
	mci("stop endurance_music")
	mci("stop boss")
	for _, a := range []string{"survival_boss1_music", "survival_boss2_music", "survival_boss3_music"} {
		mci("stop " + a)
	}
	bossStarted = false
	bossPaused = false

	boss := survivalActiveBossNumber()
	alias := survivalBossMusicAlias(boss)
	if !survivalBossMusicReady(boss) {
		setSurvivalBossMusicReady(boss, initSurvivalBossMusicAlias(boss))
	}
	if !survivalBossMusicReady(boss) {
		status = "BOSS MUSIC ERROR // COULD NOT OPEN MP3"
		return
	}

	mci("stop " + alias)
	mci("seek " + alias + " to start")
	mci(fmt.Sprintf("setaudio %s volume to %d", alias, survivalBoss1TargetMusicVolume()))
	if !mci("play " + alias + " repeat") {
		status = "BOSS MUSIC ERROR // PLAY COMMAND FAILED"
		return
	}

	bossStarted = true
	bossPaused = false
	names := []string{"", "SENTINEL", "VOID SERPENT", "TERMINUS 1337"}
	status = "BOSS MUSIC ACTIVE // " + names[boss]
}
func survivalBoss1EnsureMusic(now time.Time) {
	_ = now
	// Deliberately non-destructive. Never query/close/reopen/reseek Boss 1 during
	// active playback. Earlier MCI watchdog logic caused the ~1 second cut-out.
}

func survivalBoss1FadeOutMusic() {
	boss := survivalActiveBossNumber()
	if !audioReady || !survivalBossMusicReady(boss) {
		return
	}
	survivalBoss1FadeAlias(survivalBossMusicAlias(boss), survivalBoss1TargetMusicVolume(), 0, 900*time.Millisecond, true)
}

func survivalBoss1PauseMusic() {
	boss := survivalActiveBossNumber()
	if survivalBossMusicReady(boss) {
		mci("pause " + survivalBossMusicAlias(boss))
	}
}

func survivalBoss1StopMusic() {
	atomic.AddInt64(&survivalBoss1FadeSerial, 1)
	for boss := 1; boss <= 3; boss++ {
		if survivalBossMusicReady(boss) {
			alias := survivalBossMusicAlias(boss)
			mci("stop " + alias)
			mci("seek " + alias + " to start")
		}
	}
}

func survivalBoss1StartNormalMusicReturn() {
	if !audioReady {
		return
	}
	atomic.AddInt64(&survivalBoss1FadeSerial, 1)

	// Boss 1 clear always enters Sector 2. Do not restart Section 1 here.
	alias := survivalMusicAliasForWave(11)
	if alias == "" {
		return
	}
	mci("stop survival_music")
	mci("stop survival_section2")
	mci("stop survival_section3")
	mci("seek " + alias + " to start")
	mci("setaudio " + alias + " volume to 0")
	mci("play " + alias + " repeat")
	survivalBoss1FadeAlias(alias, 0, survivalBoss1TargetMusicVolume(), 1000*time.Millisecond, false)
	bossStarted = true
	bossPaused = false
}
func beginSurvivalBoss1Transition(h uintptr, now time.Time) {
	if survivalBoss1Active() || survivalWave != 10 {
		return
	}
	if survivalBossIntroHandoff {
		resetSurvivalBoss1State()
	}
	analyticsEvent("boss_attempted", map[string]any{"boss": "SENTINEL", "wave": survivalWave})
	survivalEnemies = nil
	survivalPickup = nil
	survivalPickup2 = nil
	survivalEliminations = nil
	survivalArcFX = nil
	survivalWaveBreakUntil = time.Time{}
	if survivalBossIntroHandoff {
		survivalBoss1Stage = survivalBoss1Intro
	} else {
		survivalBoss1Stage = survivalBoss1FadeOut
	}
	survivalBoss1StageAt = now
	survivalBoss1StartedAt = now
	if survivalBossIntroHandoff {
		survivalWaveBannerText = ""
		survivalWaveBannerUntil = time.Time{}
	} else {
		survivalWaveBannerText = "SECTOR 1 COMPLETE // BOSS SIGNAL DETECTED"
		survivalWaveBannerUntil = now.Add(1100 * time.Millisecond)
		survivalBoss1FadeOutNormalMusic()
	}
	unlockAchievement("SURV_SENTINEL_REACHED")
	enqueueLiveAnnouncement("SURVIVAL ALERT: The Sentinel has been detected beyond Wave 10.")
	status = "BOSS SIGNAL DETECTED"
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
}

func survivalBoss1SetStage(stage int, now time.Time) {
	survivalBoss1Stage = stage
	survivalBoss1StageAt = now
}

func survivalBoss1TriggerAppearanceFlash(kind int, now time.Time) {
	survivalBoss1AppearanceFlashKind = kind
	survivalBoss1AppearanceFlashAt = now
}

func survivalBoss1BeginPhase(stage int, now time.Time) {
	survivalBoss1SetStage(stage, now)
	survivalBoss1NodeAt = now
	survivalBoss1LastSwap = now
	survivalBoss1ChooseNode(now, true)
}

func survivalBoss1ChooseNode(now time.Time, forceDifferent bool) {
	count := 8
	if survivalBoss1Stage == survivalBoss1Phase2 {
		count = 3
	}
	old := survivalBoss1NodeIndex
	idx := rand.Intn(count)
	if forceDifferent && count > 1 && idx == old {
		idx = (idx + 1 + rand.Intn(count-1)) % count
	}
	survivalBoss1NodeIndex = idx
	survivalBoss1NodeRed = rand.Intn(2) == 0
	survivalBoss1NodeAt = now
	survivalBoss1LastSwap = now
}

func survivalBoss1NodeWindow() time.Duration {
	// Every exposed node must now be hit inside a clear reaction window.
	// Round 2 tightens the timing without becoming unreadable.
	switch survivalBoss1Stage {
	case survivalBoss1Phase1:
		if survivalBoss1Cycle >= 2 {
			return 1750 * time.Millisecond
		}
		return 2300 * time.Millisecond
	case survivalBoss1Phase2:
		if survivalBoss1Cycle >= 2 {
			return 1350 * time.Millisecond
		}
		return 1750 * time.Millisecond
	}
	return 2 * time.Second
}

func survivalBoss1NodeTimeRemaining(now time.Time) float64 {
	window := survivalBoss1NodeWindow()
	if window <= 0 || survivalBoss1NodeAt.IsZero() {
		return 0
	}
	left := 1 - float64(now.Sub(survivalBoss1NodeAt))/float64(window)
	if left < 0 {
		left = 0
	}
	if left > 1 {
		left = 1
	}
	return left
}

func survivalBoss1CoreReturnSeconds() float64 {
	if survivalBoss1Cycle >= 2 {
		return 1.10
	}
	return 1.30
}

func survivalBoss1CoreCadence() time.Duration {
	if survivalBoss1Cycle >= 2 {
		return 55 * time.Millisecond
	}
	return 170 * time.Millisecond
}

func survivalBoss1BeginSecondRotation(now time.Time) {
	go playBoss1RoarSound()
	survivalBoss1TriggerAppearanceFlash(survivalBoss1AppearanceCoreToDamaged, now)
	survivalBoss1Cycle = 2
	survivalBoss1CycleHits = 0
	survivalBoss1HurtAt = now
	survivalBoss1CoreOrbs = nil
	survivalBoss1CoreOrbIndex = 0
	survivalBoss1CoreOrbAt = time.Time{}
	survivalBoss1CoreBurstAt = time.Time{}
	survivalBoss1NodeIndex = 0
	survivalBoss1NodeRed = rand.Intn(2) == 0
	survivalBoss1NodeAt = now
	survivalBoss1LastSwap = now
	survivalBoss1BeginPhase(survivalBoss1Phase1, now)
	survivalWaveBannerText = "CRITICAL DAMAGE // SENTINEL ENRAGED"
	survivalWaveBannerUntil = now.Add(1800 * time.Millisecond)
	status = "SENTINEL WOUNDED // ROUND 2"
}

func survivalBoss1BeginCoreBurst(now time.Time) {
	survivalBoss1CoreOrbs = make([]SurvivalBossCoreOrb, survivalBoss1CoreOrbCount)
	for i := range survivalBoss1CoreOrbs {
		// Even angular spacing with a slight rotation gives a deliberate radial burst.
		survivalBoss1CoreOrbs[i] = SurvivalBossCoreOrb{
			Angle: float64(i)*2*math.Pi/float64(survivalBoss1CoreOrbCount) - math.Pi/2,
			Red:   i%2 == 0,
		}
	}
	// Shuffle colours independently enough to prevent memorising L/R alternation.
	for i := range survivalBoss1CoreOrbs {
		if rand.Intn(2) == 0 {
			survivalBoss1CoreOrbs[i].Red = !survivalBoss1CoreOrbs[i].Red
		}
	}

	// Randomise the return order every attempt. The balls still spread evenly
	// around the core, but their inward sequence can no longer be memorised.
	rand.Shuffle(len(survivalBoss1CoreOrbs), func(i, j int) {
		survivalBoss1CoreOrbs[i], survivalBoss1CoreOrbs[j] = survivalBoss1CoreOrbs[j], survivalBoss1CoreOrbs[i]
	})

	survivalBoss1CoreOrbIndex = 0
	survivalBoss1CoreBurstAt = now
	survivalBoss1CoreOrbAt = now.Add(900 * time.Millisecond)
	survivalBoss1SetStage(survivalBoss1CriticalLocked, now)
	survivalWaveBannerText = "CORE COLLAPSE // INTERCEPT THE ENERGY"
	survivalWaveBannerUntil = now.Add(1800 * time.Millisecond)
}

func survivalBoss1CoreOrbPosition(w, h int32, now time.Time, index int) FPoint {
	pos := survivalBoss1Position(w, h, now)
	if index < 0 || index >= len(survivalBoss1CoreOrbs) {
		return pos
	}
	orb := survivalBoss1CoreOrbs[index]

	// All balls explode outward from the core during the first 0.9s.
	burstAge := now.Sub(survivalBoss1CoreBurstAt).Seconds()
	if burstAge < 0 {
		burstAge = 0
	}
	outP := burstAge / .9
	if outP > 1 {
		outP = 1
	}
	outP = 1 - math.Pow(1-outP, 3) // ease-out

	// Push the burst slightly farther than before, but clamp each ball against
	// the arena bounds based on its angle so the full clickable ball stays visible.
	arena := arenaRect(w, h)
	margin := float64(sx(34, w))
	cosA, sinA := math.Cos(orb.Angle), math.Sin(orb.Angle)
	outerR := float64(sx(265, w))

	if math.Abs(cosA) > .0001 {
		if cosA > 0 {
			outerR = math.Min(outerR, (float64(arena.Right)-margin-pos.X)/cosA)
		} else {
			outerR = math.Min(outerR, (pos.X-(float64(arena.Left)+margin))/-cosA)
		}
	}
	if math.Abs(sinA) > .0001 {
		if sinA > 0 {
			outerR = math.Min(outerR, (float64(arena.Bottom)-margin-pos.Y)/sinA)
		} else {
			outerR = math.Min(outerR, (pos.Y-(float64(arena.Top)+margin))/-sinA)
		}
	}
	if outerR < 0 {
		outerR = 0
	}

	radius := outerR * outP

	// Once the burst is complete, only the current ball returns toward the core.
	if index == survivalBoss1CoreOrbIndex && !orb.Destroyed && now.After(survivalBoss1CoreOrbAt) {
		returnAge := now.Sub(survivalBoss1CoreOrbAt).Seconds()
		p := returnAge / survivalBoss1CoreReturnSeconds()
		if p > 1 {
			p = 1
		}
		// Smooth acceleration inward keeps the motion readable while increasing urgency.
		p = p * p
		radius = outerR * (1 - p)
	}

	return FPoint{
		X: pos.X + math.Cos(orb.Angle)*radius,
		Y: pos.Y + math.Sin(orb.Angle)*radius,
	}
}

func survivalBoss1AdvanceCoreOrb(now time.Time) {
	for survivalBoss1CoreOrbIndex < len(survivalBoss1CoreOrbs) &&
		survivalBoss1CoreOrbs[survivalBoss1CoreOrbIndex].Destroyed {
		survivalBoss1CoreOrbIndex++
	}
	if survivalBoss1CoreOrbIndex >= len(survivalBoss1CoreOrbs) {
		unlockAchievement("SURV_CORE_SWEEP")
		if survivalBoss1Cycle >= 2 {
			enqueueLiveAnnouncement("Sentinel final core-collapse intercepted — kill shot exposed.")
		}
		// Enter through the dedicated initializer so the rapid-click hit count,
		// fixed colour and countdown timestamp are all created together. Entering
		// FinalCore directly leaves CoreSpamAt at its zero value, which makes the
		// timeout check fail immediately on the first update tick.
		survivalBoss1BeginCoreSpam(now)
		return
	}
	// Small cadence between successful interceptions.
	survivalBoss1CoreOrbAt = now.Add(survivalBoss1CoreCadence())
}

func survivalBoss1CoreSpamWindow() time.Duration {
	if survivalBoss1CoreSpamTotal >= survivalBoss1FinalCoreSpamHits {
		return 6000 * time.Millisecond
	}
	return 3500 * time.Millisecond
}

func survivalBoss1CoreSpamRemainingRatio(now time.Time) float64 {
	window := survivalBoss1CoreSpamWindow()
	if window <= 0 || survivalBoss1CoreSpamAt.IsZero() {
		return 0
	}
	left := 1 - float64(now.Sub(survivalBoss1CoreSpamAt))/float64(window)
	if left < 0 {
		return 0
	}
	if left > 1 {
		return 1
	}
	return left
}

func survivalBoss1BeginCoreSpam(now time.Time) {
	total := survivalBoss1FirstCoreSpamHits
	if survivalBoss1Cycle >= 2 {
		total = survivalBoss1FinalCoreSpamHits
	}
	survivalBoss1CoreSpamTotal = total
	survivalBoss1CoreSpamRemaining = total
	survivalBoss1CoreSpamAt = now
	survivalBoss1CoreSpamLastClick = time.Time{}
	// Select one required button colour for the whole spam sequence.
	survivalBoss1NodeRed = rand.Intn(2) == 0
	survivalBoss1NodeAt = now
	survivalBoss1SetStage(survivalBoss1FinalCore, now)
	if total >= survivalBoss1FinalCoreSpamHits {
		survivalWaveBannerText = "DESTROY THE CORE // 20 RAPID HITS"
	} else {
		survivalWaveBannerText = "BREAK THE CORE // 10 RAPID HITS"
	}
	survivalWaveBannerUntil = now.Add(1450 * time.Millisecond)
}

func updateSurvivalBoss1(h uintptr, now time.Time, w, hgt int32) bool {
	if !survivalBoss1Active() {
		return false
	}
	now = survivalBossFightNow(now, survivalBoss1StartedAt)
	elapsed := now.Sub(survivalBoss1StageAt)
	switch survivalBoss1Stage {
	case survivalBoss1FadeOut:
		if elapsed >= 1000*time.Millisecond {
			// Full black: Section 1 is now completely stopped.
			mci("stop survival_music")
			mci("seek survival_music to start")
			survivalBoss1StartMusic()
			survivalBoss1SetStage(survivalBoss1FadeIn, now)
		}
	case survivalBoss1FadeIn:
		if elapsed >= 900*time.Millisecond {
			survivalBoss1SetStage(survivalBoss1Intro, now)
			go playBoss1RoarSound()
			survivalWaveBannerText = "WARNING // SENTINEL DETECTED"
			survivalWaveBannerUntil = now.Add(1700 * time.Millisecond)
		}
	case survivalBoss1Intro:
		if elapsed >= 2350*time.Millisecond {
			survivalBoss1FightAt = now
			survivalBoss1BeginPhase(survivalBoss1Phase1, now)
			survivalWaveBannerText = "TARGET WEAK POINTS"
			survivalWaveBannerUntil = now.Add(1400 * time.Millisecond)
		}
	case survivalBoss1Phase1:
		// The active node is a timed reaction check. Waiting it out is now a fail.
		if now.Sub(survivalBoss1NodeAt) >= survivalBoss1NodeWindow() {
			survivalFail(h, "SENTINEL NODE WINDOW MISSED")
			return true
		}
	case survivalBoss1Reconfig1:
		// v352: a short, explicit reconfiguration beat bridges the two armour patterns.
		// This removes the abrupt target teleport while keeping the fight fast.
		if elapsed >= 550*time.Millisecond {
			survivalBoss1BeginPhase(survivalBoss1Phase2, now)
			survivalWaveBannerText = "PHASE 2 // NODE MATRIX"
			survivalWaveBannerUntil = now.Add(900 * time.Millisecond)
		}

	case survivalBoss1Phase2:
		// Phase 2 has a tighter node deadline. The player must react immediately;
		// the node no longer waits forever or simply swaps to another target.
		if now.Sub(survivalBoss1NodeAt) >= survivalBoss1NodeWindow() {
			survivalFail(h, "SENTINEL NODE WINDOW MISSED")
			return true
		}
	case survivalBoss1Reconfig2:
		if elapsed >= 1050*time.Millisecond {
			// Phase 3 is no longer another weak-point phase. The armour opens and the
			// exposed core launches a radial energy-burst finale. Flash across the
			// texture swap so the Sentinel visibly evolves/breaks open.
			survivalBoss1TriggerAppearanceFlash(survivalBoss1AppearanceDamagedToCore, now)
			go playBoss1RoarSound()
			survivalBoss1SetStage(survivalBoss1CriticalTelegraph, now)
			survivalWaveBannerText = "PHASE 3 // CORE COLLAPSE"
			survivalWaveBannerUntil = now.Add(1500 * time.Millisecond)
		}
	case survivalBoss1Phase3:
		// Legacy stage retained for compatibility; immediately enter the core finale.
		survivalBoss1TriggerAppearanceFlash(survivalBoss1AppearanceDamagedToCore, now)
		survivalBoss1SetStage(survivalBoss1CriticalTelegraph, now)
	case survivalBoss1CriticalTelegraph:
		if elapsed >= 1300*time.Millisecond {
			survivalBoss1BeginCoreBurst(now)
		}
	case survivalBoss1CriticalLocked:
		// After the outward burst, each ball returns one-by-one. If the currently
		// active ball reaches the core before being clicked, the fight is lost.
		if survivalBoss1CoreOrbIndex < len(survivalBoss1CoreOrbs) && now.After(survivalBoss1CoreOrbAt) {
			p := now.Sub(survivalBoss1CoreOrbAt).Seconds() / survivalBoss1CoreReturnSeconds()
			if p >= 1 {
				survivalFail(h, "CORE ENERGY REACHED THE SENTINEL")
				return true
			}
		}
	case survivalBoss1FinalCore:
		if now.Sub(survivalBoss1CoreSpamAt) >= survivalBoss1CoreSpamWindow() {
			survivalFail(h, "CORE DESTRUCTION TOO SLOW")
			return true
		}
	case survivalBoss1CycleReset:
		if elapsed >= 1450*time.Millisecond {
			survivalBoss1BeginSecondRotation(now)
		}
	case survivalBoss1Dying:
		if elapsed >= 1650*time.Millisecond {
			survivalBoss1SetStage(survivalBoss1FadeToReport, now)
			survivalBoss1FadeOutMusic()
		}
	case survivalBoss1FadeToReport:
		if elapsed >= 1000*time.Millisecond {
			finishSurvivalBoss1Clear(h, now)
		}
	case survivalBoss1ReturnFadeIn:
		if elapsed >= 1000*time.Millisecond {
			survivalBoss1Stage = survivalBoss1None
			survivalBoss1StageAt = time.Time{}
			beginSurvivalWave()
			survivalLastTick = now
			setCapture.Call(h)
			status = "SECTOR 2 // WAVE 11"
			survivalWaveBannerText = "SECTOR 2 // WAVE 11"
			survivalWaveBannerUntil = now.Add(1800 * time.Millisecond)
		}
	}
	invalidateSurvivalArena(h)
	return true
}

func survivalBoss1Position(w, h int32, now time.Time) FPoint {
	c := survivalArenaCenter(w, h)
	// v312: Boss 1 moved a further 4 design pixels right from v311
	// (9 pixels right in total from the original v309 position).
	c.X -= float64(sx(41, w))

	// Dramatic reveal: the Sentinel begins above the arena, then slams down.
	if survivalBoss1Stage == survivalBoss1FadeIn {
		return FPoint{c.X, float64(arenaRect(w, h).Top) - float64(sy(220, h))}
	}
	if survivalBoss1Stage == survivalBoss1Intro {
		age := now.Sub(survivalBoss1StageAt).Seconds()
		p := age / 1.35
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		ease := 1 - math.Pow(1-p, 3)
		startY := float64(arenaRect(w, h).Top) - float64(sy(210, h))
		y := startY + (c.Y-startY)*ease
		if age > 1.15 && age < 1.75 {
			y -= math.Sin((age-1.15)/.60*math.Pi) * float64(sy(18, h))
		}
		return FPoint{c.X, y}
	}
	if survivalBoss1FightAt.IsZero() || survivalBoss1Stage < survivalBoss1Phase1 {
		return c
	}

	// v265: one continuous movement curve is used for ALL combat/reconfiguration
	// states. Previously Reconfig/Critical stages fell through to `return c`,
	// which visually teleported the boss back to the arena centre between phases.
	//
	// Movement intensity ramps up smoothly over the fight rather than changing
	// equations when the phase changes, so phase transitions cannot introduce
	// positional discontinuities.
	t := now.Sub(survivalBoss1FightAt).Seconds()
	if t < 0 {
		t = 0
	}
	ramp := t / 14.0
	if ramp > 1 {
		ramp = 1
	}
	// Smoothstep avoids a visible acceleration step at the beginning.
	ramp = ramp * ramp * (3 - 2*ramp)

	ampX := float64(sx(52, w)) + float64(sx(108, w))*ramp
	ampY := float64(sy(22, h)) + float64(sy(46, h))*ramp

	// A gently accelerating Lissajous-like drift keeps the boss alive without
	// being erratic. The phase is derived only from global fight time, never from
	// stage start time, so it remains continuous through every transition.
	phaseX := t*1.08 + t*t*.014
	phaseY := t*.82 + 0.72 + t*t*.009

	return FPoint{
		c.X + ampX*math.Sin(phaseX),
		c.Y + ampY*math.Sin(phaseY),
	}
}

func survivalBoss1NodeOffsets(stage int) []FPoint {
	if stage == survivalBoss1Phase2 {
		return []FPoint{{-0.42, -0.15}, {0.40, -0.20}, {0.03, 0.42}}
	}
	return []FPoint{{0, -0.46}, {0.38, -0.30}, {0.47, 0.05}, {0.34, 0.36}, {0, 0.46}, {-0.35, 0.34}, {-0.47, 0.04}, {-0.38, -0.30}}
}

func survivalBoss1Nodes(w, h int32, now time.Time) []SurvivalBossNode {
	pos := survivalBoss1Position(w, h, now)
	offsets := survivalBoss1NodeOffsets(survivalBoss1Stage)
	scaleX := float64(sx(142, w))
	scaleY := float64(sy(142, h))
	out := make([]SurvivalBossNode, len(offsets))
	for i, o := range offsets {
		out[i] = SurvivalBossNode{DX: pos.X + o.X*scaleX, DY: pos.Y + o.Y*scaleY, Red: survivalBoss1NodeRed, Active: i == survivalBoss1NodeIndex}
		if survivalBoss1Stage == survivalBoss1Phase2 && !out[i].Active {
			// Inactive nodes intentionally retain their own colour so colour alone cannot identify the target.
			out[i].Red = ((i + survivalBoss1Hits) % 2) == 0
		}
	}
	return out
}

func survivalBoss1HandleClick(h uintptr, p FPoint, right bool) bool {
	if !survivalBoss1Active() || state != StatePlaying {
		return false
	}
	if !survivalBoss1CombatActive() {
		// Transitions, reconfiguration and critical telegraph are invulnerable / input-suspended.
		return true
	}
	w, hgt := getClient(h)
	now := survivalBossFightNow(time.Now(), survivalBoss1StartedAt)
	if survivalBoss1Stage == survivalBoss1CriticalLocked {
		// Only the currently returning orb is clickable. The remaining balls wait
		// at the perimeter until their turn.
		idx := survivalBoss1CoreOrbIndex
		if idx < 0 || idx >= len(survivalBoss1CoreOrbs) || now.Before(survivalBoss1CoreOrbAt) {
			survivalFail(h, "CORE PHASE MISCLICK")
			return true
		}
		op := survivalBoss1CoreOrbPosition(w, hgt, now, idx)
		radius := float64(sx(27, w))
		d := math.Hypot(p.X-op.X, p.Y-op.Y)
		if d > radius {
			survivalFail(h, "CORE PHASE MISCLICK — ENERGY BALL MISSED")
			return true
		}
		orb := &survivalBoss1CoreOrbs[idx]
		if orb.Red == right {
			survivalFail(h, "CORE PHASE MISCLICK — WRONG BUTTON")
			return true
		}
		playBossClickEffect()
		orb.Destroyed = true
		survivalBoss1Hits++
		survivalBoss1CycleHits++
		survivalTotalHits++
		survivalReactionSum += now.Sub(survivalBoss1CoreOrbAt).Seconds() * 1000
		survivalReactionSamples++
		score += 350
		// Circle phase uses one clean feedback sound only. Do not layer the generic
		// explosion/hit SFX over boss_click_effect.
		status = fmt.Sprintf("CORE ENERGY INTERCEPTED %d/%d", idx+1, len(survivalBoss1CoreOrbs))
		survivalBoss1AdvanceCoreOrb(now)
		invalidateSurvivalHUD(h)
		invalidateSurvivalArena(h)
		return true
	}

	if survivalBoss1Stage == survivalBoss1FinalCore {
		pos := survivalBoss1Position(w, hgt, now)
		radius := float64(sx(48, w))
		if math.Hypot(p.X-pos.X, p.Y-pos.Y) > radius {
			survivalFail(h, "SENTINEL MISCLICK — CORE MISSED")
			return true
		}
		if survivalBoss1NodeRed == right {
			survivalFail(h, "SENTINEL MISCLICK — WRONG BUTTON")
			return true
		}
		// Ignore impossible duplicate/down-bounce events without punishing the player.
		if !survivalBoss1CoreSpamLastClick.IsZero() && now.Sub(survivalBoss1CoreSpamLastClick) < 45*time.Millisecond {
			return true
		}
		survivalBoss1CoreSpamLastClick = now
		if survivalBoss1CoreSpamRemaining <= 0 {
			return true
		}

		survivalBoss1CoreSpamRemaining--
		survivalBoss1Hits++
		survivalBoss1CycleHits++
		survivalTotalHits++
		score += 180
		if survivalBoss1CoreSpamRemaining > 0 {
			// Every setup hit is the dedicated boss-hit sound; the explosion is
			// reserved exclusively for the final successful click.
			playBossClickEffect()
			status = fmt.Sprintf("DESTROY CORE // %d HITS LEFT", survivalBoss1CoreSpamRemaining)
			invalidateSurvivalHUD(h)
			invalidateSurvivalArena(h)
			return true
		}

		playSurvivalExplodeSound()
		if survivalBoss1Cycle == 1 {
			score += 900
			survivalBoss1SetStage(survivalBoss1CycleReset, now)
			survivalWaveBannerText = "CORE SHATTERED // SENTINEL REBOOTING"
			survivalWaveBannerUntil = now.Add(1650 * time.Millisecond)
			status = "SENTINEL REBOOTING // ROUND 2"
			return true
		}

		survivalBoss1Hits = survivalBoss1TotalHits
		survivalBoss1ClearTime = now.Sub(survivalBoss1FightAt).Seconds()
		gameMeta.SentinelDefeats++
		unlockAchievement("SURV_SENTINEL_DOWN")
		if survivalTotalDamageTaken == 0 {
			unlockAchievement("SURV_SENTINEL_FLAWLESS")
		}
		enqueueLiveAnnouncement("SECTOR 1 CLEAR: The Sentinel has been destroyed.")
		score += 1800
		survivalAwardPendingEXP(h, 250)
		survivalBoss1SetStage(survivalBoss1Dying, now)
		survivalWaveBannerText = "SENTINEL DESTROYED"
		survivalWaveBannerUntil = now.Add(1600 * time.Millisecond)
		status = "SENTINEL DESTROYED"
		return true
	}

	nodes := survivalBoss1Nodes(w, hgt, now)
	active := -1
	hit := -1
	best := math.MaxFloat64
	activeRadius := float64(sx(25, w))
	for i, n := range nodes {
		if n.Active {
			active = i
		}
		d := math.Hypot(p.X-n.DX, p.Y-n.DY)
		if d <= activeRadius && d < best {
			hit, best = i, d
		}
	}
	if hit < 0 {
		survivalFail(h, "SENTINEL MISCLICK — ARMOUR / EMPTY SPACE")
		return true
	}
	if hit != active {
		survivalFail(h, "SENTINEL MISCLICK — INACTIVE NODE")
		return true
	}
	node := nodes[active]
	if node.Red == right {
		survivalFail(h, "SENTINEL MISCLICK — WRONG BUTTON")
		return true
	}

	playBossClickEffect()

	// Valid boss hit contributes to the same aim/reaction telemetry as Survival enemies.
	survivalTotalHits++
	precision := 1.0 - best/activeRadius
	if precision >= .72 {
		survivalPerfectHits++
	}
	survivalReactionSum += now.Sub(survivalBoss1NodeAt).Seconds() * 1000
	survivalReactionSamples++
	survivalBoss1Hits++
	survivalBoss1CycleHits++
	score += 250 + int(precision*120)
	playHitSound()
	status = fmt.Sprintf("SENTINEL HIT %d/%d", survivalBoss1Hits, survivalBoss1TotalHits)

	switch survivalBoss1CycleHits {
	case 4:
		// Give the Sentinel a visible mechanical transition before the next node pattern.
		survivalBoss1SetStage(survivalBoss1Reconfig1, now)
		survivalWaveBannerText = "SENTINEL RECONFIGURING"
		survivalWaveBannerUntil = now.Add(650 * time.Millisecond)
	case 8:
		if survivalBoss1Cycle == 1 {
			survivalBoss1TriggerAppearanceFlash(survivalBoss1AppearanceNormalToDamaged, now)
		}
		survivalBoss1SetStage(survivalBoss1Reconfig2, now)
		survivalWaveBannerText = "CORE ARMOUR FAILING"
		survivalWaveBannerUntil = now.Add(1200 * time.Millisecond)
	default:
		survivalBoss1ChooseNode(now, true)
	}
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
	return true
}

func finishSurvivalBoss1Clear(h uintptr, now time.Time) {
	analyticsEvent("boss_cleared", map[string]any{"boss": "SENTINEL", "wave": survivalWave, "kills": survivalKills})
	survivalBoss1SectionTime = now.Sub(survivalStartedAt).Seconds()
	survivalSectionCheckpointAfterBoss(11)
	unlockAchievement("SURV_SECTOR2")
	survivalBoss1SectionReport = true
	survivalSectionReportSector = 1
	survivalBoss1Stage = survivalBoss1None
	state = StateResult
	releaseCapture.Call()
	status = "SECTOR 1 CLEAR // CHECKPOINT SAVED"
	lastResult = ResultData{
		Time:        survivalBoss1SectionTime,
		CombinedAcc: survivalPerfectPercent(),
		TrackingAcc: survivalAccuracyPercent(),
		TargetAcc:   survivalPerfectPercent(),
		TargetsHit:  survivalKills,
		TargetCount: survivalTotalHits,
		TotalScore:  score,
		RoundPoints: score,
		Streak:      10,
		Combo:       float64(survivalBestCombo),
		Rating:      survivalGrade(),
		Rank:        rankForEXP(playerProgress.EXP),
		EXPEarned:   survivalLastEXPAward,
		TotalEXP:    playerProgress.EXP,
		Course:      "SURVIVAL_SECTION_1",
		Difficulty:  "SURVIVAL",
	}
	invalidateRect.Call(h, 0, 0)
	updateWindow.Call(h)
}

func survivalSectionCheckpointAfterBoss(nextWave int) {
	if nextWave <= gameMeta.SurvivalCheckpoint {
		return
	}
	gameMeta.SurvivalCheckpoint = nextWave
	survivalCheckpoint = nextWave
	updatePersistentSurvivalProfileStats(nextWave, survivalKills, survivalBestCombo)
	saveGameMeta()
	requestPlayerProfileSync()
	playLevelUpSound()
}

func continueSurvivalAfterSectionReport(h uintptr) {
	if !survivalBoss1SectionReport || state != StateResult {
		return
	}
	sector := survivalSectionReportSector
	survivalBoss1SectionReport = false
	survivalSectionReportSector = 0
	survivalHP = survivalMaxHP()
	survivalWaveKills = 0
	survivalWaveSpawned = 0
	survivalEnemies = nil
	survivalPickup = nil
	survivalPickup2 = nil
	survivalShieldCharges = 0
	survivalChainCharges = 0
	survivalSlowUntil = time.Time{}
	state = StatePlaying
	now := time.Now()

	if sector == 2 {
		survivalWave = 21
		survivalBoss2SetStage(survivalBoss2ReturnFadeIn, now)
		// Boss 2 completion owns the boss track until the player deliberately continues.
		// Stop it here, then bring Sector 3 up under the same one-second return fade used
		// after Sentinel so there is never boss music underneath the section report.
		survivalBoss1StopMusic()
		if audioReady {
			alias := ensureSurvivalSectionAlias(3)
			if alias != "" {
				mci("stop " + alias)
				mci("seek " + alias + " to start")
				mci("setaudio " + alias + " volume to 0")
				mci("play " + alias + " repeat")
				survivalBoss1FadeAlias(alias, 0, survivalBoss1TargetMusicVolume(), 1000*time.Millisecond, false)
				bossStarted = true
				bossPaused = false
			}
		}
		status = "ENTERING SECTOR 3"
	} else {
		survivalWave = 11
		survivalBoss1SetStage(survivalBoss1ReturnFadeIn, now)
		survivalBoss1StopMusic()
		survivalBoss1StartNormalMusicReturn()
		status = "ENTERING SECTOR 2"
	}
	invalidateRect.Call(h, 0, 0)
}

func survivalBoss1OnFailureReport() {
	survivalBoss1StopMusic()
	survivalBoss1Stage = survivalBoss1None
	survivalBoss1SectionReport = false
	survivalSectionReportSector = 0
}

func drawSurvivalBoss1Arena(hdc uintptr, w, h int32) {
	r := arenaRect(w, h)
	now := survivalBossFightNow(time.Now(), survivalBoss1StartedAt)
	shakeX, shakeY := int32(0), int32(0)
	if survivalBoss1Stage == survivalBoss1Intro {
		age := now.Sub(survivalBoss1StageAt).Seconds()
		if age < 1.65 {
			strength := math.Max(0, 1-age/1.65)
			shakeX = int32(math.Sin(age*71) * float64(sx(8, w)) * strength)
			shakeY = int32(math.Sin(age*93+1.3) * float64(sy(6, h)) * strength)
		}
	} else if survivalBoss1Stage == survivalBoss1CriticalTelegraph {
		age := now.Sub(survivalBoss1StageAt).Seconds()
		if age < 1.15 {
			strength := math.Max(0, 1-age/1.15)
			shakeX = int32(math.Sin(age*109) * float64(sx(10, w)) * strength)
			shakeY = int32(math.Sin(age*137+.8) * float64(sy(8, h)) * strength)
		}
	} else if survivalBoss1Stage == survivalBoss1Phase1 && survivalBoss1Cycle >= 2 && !survivalBoss1HurtAt.IsZero() {
		age := now.Sub(survivalBoss1HurtAt).Seconds()
		if age >= 0 && age < 1.25 {
			strength := math.Max(0, 1-age/1.25)
			// Deliberately harsher than the normal entrance shake: the Sentinel
			// has just had its core shattered and is violently rebooting.
			shakeX = int32(math.Sin(age*131) * float64(sx(13, w)) * strength)
			shakeY = int32(math.Sin(age*167+1.1) * float64(sy(10, h)) * strength)
		}
	}
	shakeScale := screenShakeScale()
	shakeX = int32(float64(shakeX) * shakeScale)
	shakeY = int32(float64(shakeY) * shakeScale)
	bgRect := RECT{r.Left + shakeX, r.Top + shakeY, r.Right + shakeX, r.Bottom + shakeY}
	if len(survivalBoss1Background) >= 1508*592*4 {
		drawRawBGRACover(hdc, survivalBoss1Background, 1508, 592, bgRect)
	} else {
		// Placeholder boss arena until the final Boss 1 background is supplied.
		fillSolidRect(hdc, r, rgb(2, 5, 16))
		c := survivalArenaCenter(w, h)
		for i := 0; i < 48; i++ {
			x := r.Left + int32((i*173)%int(r.Right-r.Left))
			y := r.Top + int32((i*97)%int(r.Bottom-r.Top))
			rr := int32(1 + i%2)
			col := rgb(65, 110, 180)
			if i%7 == 0 {
				col = rgb(185, 65, 75)
			}
			drawSurvivalCircleWithPen(hdc, survivalPen(1, col), x, y, rr)
		}
		drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(80, 20, 30)), int32(c.X), int32(c.Y), sx(235, w))
	}

	pos := survivalBoss1Position(w, h, now)
	boss := survivalBoss1NormalBGRA
	if survivalBoss1Hits >= 8 {
		boss = survivalBoss1DamagedBGRA
	}
	if survivalBoss1Stage == survivalBoss1CriticalTelegraph || survivalBoss1Stage == survivalBoss1CriticalLocked || survivalBoss1Stage == survivalBoss1FinalCore || survivalBoss1Stage == survivalBoss1CycleReset || survivalBoss1Stage == survivalBoss1Dying || survivalBoss1Stage == survivalBoss1FadeToReport {
		boss = survivalBoss1CoreBGRA
	}
	// Hold the OLD appearance for ~110 ms under the first flash, then reveal the
	// new appearance under a second flash. This makes every texture swap feel like
	// visible damage/evolution rather than an instantaneous asset replacement.
	if !survivalBoss1AppearanceFlashAt.IsZero() {
		age := now.Sub(survivalBoss1AppearanceFlashAt)
		if age >= 0 && age < 110*time.Millisecond {
			switch survivalBoss1AppearanceFlashKind {
			case survivalBoss1AppearanceNormalToDamaged:
				boss = survivalBoss1NormalBGRA
			case survivalBoss1AppearanceDamagedToCore:
				boss = survivalBoss1DamagedBGRA
			case survivalBoss1AppearanceCoreToDamaged:
				boss = survivalBoss1CoreBGRA
			}
		}
	}
	half := sx(175, w)
	if len(boss) >= 512*512*4 {
		drawRawBGRAFit(hdc, boss, 512, 512, RECT{int32(pos.X) - half + shakeX, int32(pos.Y) - half + shakeY, int32(pos.X) + half + shakeX, int32(pos.Y) + half + shakeY})
	} else {
		// Code-only placeholder: clear silhouette, core and armour pods.
		coreCol := rgb(230, 48, 42)
		drawSurvivalCircleWithPen(hdc, survivalPen(8, rgb(80, 95, 120)), int32(pos.X), int32(pos.Y), sx(115, w))
		drawSurvivalCircleWithPen(hdc, survivalPen(5, rgb(30, 160, 240)), int32(pos.X), int32(pos.Y), sx(83, w))
		drawSurvivalCircleWithPen(hdc, survivalPen(7, coreCol), int32(pos.X), int32(pos.Y), sx(39, w))
		for i := 0; i < 4; i++ {
			a := float64(i) * math.Pi / 2
			x1 := int32(pos.X + math.Cos(a)*float64(sx(115, w)))
			y1 := int32(pos.Y + math.Sin(a)*float64(sx(115, w)))
			x2 := int32(pos.X + math.Cos(a)*float64(sx(155, w)))
			y2 := int32(pos.Y + math.Sin(a)*float64(sx(155, w)))
			drawSurvivalLineWithPen(hdc, survivalPen(18, rgb(70, 80, 105)), x1, y1, x2, y2)
		}
	}

	// Explicit boss reaction timer: reinforces that node phases are time-limited.
	if survivalBoss1Stage == survivalBoss1Phase1 || survivalBoss1Stage == survivalBoss1Phase2 {
		remaining := survivalBoss1NodeTimeRemaining(now)
		bar := RECT{r.Left + sx(330, w), r.Top + sy(24, h), r.Right - sx(330, w), r.Top + sy(34, h)}
		fillSolidRect(hdc, RECT{bar.Left - 2, bar.Top - 2, bar.Right + 2, bar.Bottom + 2}, rgb(3, 8, 20))
		fillSolidRect(hdc, bar, rgb(55, 62, 78))
		fillW := int32(float64(bar.Right-bar.Left) * remaining)
		timerCol := rgb(70, 220, 255)
		if remaining < .35 {
			timerCol = rgb(255, 175, 30)
		}
		if fillW > 0 {
			fillSolidRect(hdc, RECT{bar.Left, bar.Top, bar.Left + fillW, bar.Bottom}, timerCol)
		}
	}

	switch survivalBoss1Stage {
	case survivalBoss1Phase1, survivalBoss1Phase2, survivalBoss1Phase3:
		nodes := survivalBoss1Nodes(w, h, now)
		for _, n := range nodes {
			col := rgb(58, 78, 92)
			if n.Red {
				col = rgb(92, 42, 42)
			}
			radius := sx(11, w)
			if n.Active {
				remaining := survivalBoss1NodeTimeRemaining(now)
				pulse := .5 + .5*math.Sin(now.Sub(survivalBoss1NodeAt).Seconds()*12)
				radius = sx(23, w) + int32(pulse*float64(sx(7, w)))
				if n.Red {
					col = rgb(255, 35, 35)
				} else {
					col = rgb(30, 205, 255)
				}
				cx, cy := int32(n.DX), int32(n.DY)

				// White target halo plus a warning halo that accelerates visually as
				// the reaction window runs out.
				drawSurvivalCircleWithPen(hdc, survivalPen(7, rgb(255, 255, 255)), cx, cy, radius+sx(5, w))
				if remaining < .40 {
					warnPulse := .5 + .5*math.Sin(now.Sub(survivalBoss1NodeAt).Seconds()*25)
					if warnPulse > .35 {
						drawSurvivalCircleWithPen(hdc, survivalPen(5, rgb(255, 190, 35)), cx, cy, radius+sx(12, w))
					}
				}
				drawSurvivalCircleWithPen(hdc, survivalPen(5, col), cx, cy, radius)

				// Local countdown bar directly above the active node.
				barW := sx(76, w)
				barH := sy(7, h)
				barLeft := cx - barW/2
				barTop := cy - radius - sy(26, h)
				fillSolidRect(hdc, RECT{barLeft - 2, barTop - 2, barLeft + barW + 2, barTop + barH + 2}, rgb(4, 8, 18))
				fillSolidRect(hdc, RECT{barLeft, barTop, barLeft + barW, barTop + barH}, rgb(55, 62, 76))
				fillW := int32(float64(barW) * remaining)
				if fillW > 0 {
					barCol := col
					if remaining < .35 {
						barCol = rgb(255, 185, 30)
					}
					fillSolidRect(hdc, RECT{barLeft, barTop, barLeft + fillW, barTop + barH}, barCol)
				}

				t0, t1 := radius+sx(8, w), radius+sx(17, w)
				drawSurvivalLineWithPen(hdc, survivalPen(4, col), cx-t1, cy, cx-t0, cy)
				drawSurvivalLineWithPen(hdc, survivalPen(4, col), cx+t0, cy, cx+t1, cy)
				drawSurvivalLineWithPen(hdc, survivalPen(4, col), cx, cy-t1, cx, cy-t0)
				drawSurvivalLineWithPen(hdc, survivalPen(4, col), cx, cy+t0, cx, cy+t1)
			} else {
				drawSurvivalCircleWithPen(hdc, survivalPen(2, col), int32(n.DX), int32(n.DY), radius)
			}
			drawSurvivalCircleWithPen(hdc, survivalPen(2, col), int32(n.DX), int32(n.DY), sx(5, w))
		}
	case survivalBoss1CriticalTelegraph:
		// Core charges violently before launching the radial burst.
		pulse := .5 + .5*math.Sin(now.Sub(survivalBoss1StageAt).Seconds()*16)
		rr := sx(36, w) + int32(pulse*float64(sx(15, w)))
		drawSurvivalCircleWithPen(hdc, survivalPen(7, rgb(255, 90, 25)), int32(pos.X), int32(pos.Y), rr)
		drawSurvivalCircleWithPen(hdc, survivalPen(4, rgb(255, 245, 185)), int32(pos.X), int32(pos.Y), sx(23, w))

	case survivalBoss1CriticalLocked:
		burstAge := now.Sub(survivalBoss1CoreBurstAt).Seconds()
		if burstAge >= 0 && burstAge < .95 {
			bp := burstAge / .95
			core := survivalBoss1Position(w, h, now)
			for ring := 0; ring < 3; ring++ {
				rp := bp - float64(ring)*.16
				if rp > 0 && rp < 1 {
					rr := int32(float64(sx(45, w)) + rp*float64(sx(250, w)))
					alphaCol := rgb(150, 165, 185)
					drawSurvivalCircleWithPen(hdc, survivalPen(4, alphaCol), int32(core.X), int32(core.Y), rr)
				}
			}
		}
		// Finale: all dormant balls are neutral grey. Their red/blue identity is
		// revealed ONLY when that ball begins its inward run, forcing reaction.
		for i, orb := range survivalBoss1CoreOrbs {
			if orb.Destroyed {
				continue
			}
			op := survivalBoss1CoreOrbPosition(w, h, now, i)
			active := i == survivalBoss1CoreOrbIndex && !now.Before(survivalBoss1CoreOrbAt)
			col := rgb(125, 135, 150)
			r := sx(15, w)

			if active {
				if orb.Red {
					col = rgb(255, 48, 42)
				} else {
					col = rgb(40, 190, 255)
				}
				pulse := .5 + .5*math.Sin(now.Sub(survivalBoss1CoreOrbAt).Seconds()*17)
				r = sx(18, w) + int32(pulse*float64(sx(5, w)))

				// Bright shock halo + locator ring.
				drawSurvivalCircleWithPen(hdc, survivalPen(7, rgb(255, 255, 255)), int32(op.X), int32(op.Y), r+sx(8, w))
				drawSurvivalCircleWithPen(hdc, survivalPen(5, col), int32(op.X), int32(op.Y), r+sx(2, w))

				// Short energy trail points back toward the core.
				core := survivalBoss1Position(w, h, now)
				dx, dy := core.X-op.X, core.Y-op.Y
				d := math.Hypot(dx, dy)
				if d > 1 {
					dx, dy = dx/d, dy/d
					for t := 1; t <= 3; t++ {
						trail := float64(sx(float64(14+t*10), w))
						tx := int32(op.X - dx*trail)
						ty := int32(op.Y - dy*trail)
						drawSurvivalCircleWithPen(hdc, survivalPen(2, col), tx, ty, sx(float64(4-t/2), w))
					}
				}
			} else {
				// Dormant grey shells have a metallic double-ring look and a faint
				// breathing pulse, but deliberately reveal no click colour.
				breath := .5 + .5*math.Sin(now.Sub(survivalBoss1CoreBurstAt).Seconds()*6+float64(i))
				r += int32(breath * float64(sx(2, w)))
				drawSurvivalCircleWithPen(hdc, survivalPen(5, rgb(70, 78, 92)), int32(op.X), int32(op.Y), r+sx(5, w))
				drawSurvivalCircleWithPen(hdc, survivalPen(4, col), int32(op.X), int32(op.Y), r)
			}
			drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(235, 240, 248)), int32(op.X), int32(op.Y), sx(4, w))
		}

	case survivalBoss1FinalCore:
		col := rgb(255, 65, 55)
		buttonLabel := "LEFT CLICK"
		if !survivalBoss1NodeRed {
			col = rgb(65, 190, 255)
			buttonLabel = "RIGHT CLICK"
		}
		pulse := .5 + .5*math.Sin(now.Sub(survivalBoss1StageAt).Seconds()*19)
		rr := sx(47, w) + int32(pulse*float64(sx(11, w)))
		// Layered shock rings make the rapid-destruction target unmistakable.
		drawSurvivalCircleWithPen(hdc, survivalPen(7, col), int32(pos.X), int32(pos.Y), rr+sx(22, w))
		drawSurvivalCircleWithPen(hdc, survivalPen(10, rgb(255, 255, 255)), int32(pos.X), int32(pos.Y), rr+sx(8, w))
		drawSurvivalCircleWithPen(hdc, survivalPen(8, col), int32(pos.X), int32(pos.Y), rr)

		// Large remaining-hit number inside the core.
		if hudTitleFont != 0 {
			oldFont, _, _ := selectObject.Call(hdc, hudTitleFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 255, 255))
			centeredTextOut(hdc, int32(pos.X)-rr, int32(pos.X)+rr, int32(pos.Y)-sy(15, h), fmt.Sprint(survivalBoss1CoreSpamRemaining))
			selectObject.Call(hdc, oldFont)
		}

		remaining := survivalBoss1CoreSpamRemainingRatio(now)
		barW := sx(170, w)
		barH := sy(12, h)
		barLeft := int32(pos.X) - barW/2
		barTop := int32(pos.Y) - rr - sy(44, h)
		fillSolidRect(hdc, RECT{barLeft - 3, barTop - 3, barLeft + barW + 3, barTop + barH + 3}, rgb(2, 5, 14))
		fillSolidRect(hdc, RECT{barLeft, barTop, barLeft + barW, barTop + barH}, rgb(58, 62, 76))
		fillW := int32(float64(barW) * remaining)
		barCol := col
		if remaining < .30 {
			barCol = rgb(255, 180, 25)
		}
		if fillW > 0 {
			fillSolidRect(hdc, RECT{barLeft, barTop, barLeft + fillW, barTop + barH}, barCol)
		}
		if hudTinyFont != 0 {
			oldFont, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 240, 160))
			centeredTextOut(hdc, int32(pos.X)-sx(130, w), int32(pos.X)+sx(130, w), int32(pos.Y)+rr+sy(18, h), buttonLabel+" RAPIDLY")
			selectObject.Call(hdc, oldFont)
		}
	case survivalBoss1Dying, survivalBoss1FadeToReport:
		age := now.Sub(survivalBoss1StageAt).Seconds()
		for i := 0; i < 8; i++ {
			a := float64(i)*.91 + age*1.6
			rr := float64(sx(float64(25+i*11), w)) * math.Min(1, age+.15)
			x := int32(pos.X + math.Cos(a)*rr)
			y := int32(pos.Y + math.Sin(a)*rr)
			drawSurvivalCircleWithPen(hdc, survivalPen(3, rgb(255, 100, 35)), x, y, sx(float64(6+i%3), w))
		}
	}

	// Violent warning strobe while the Sentinel slams into the arena.
	if survivalBoss1Stage == survivalBoss1Intro {
		age := now.Sub(survivalBoss1StageAt).Seconds()
		if age < 1.70 {
			flash := .5 + .5*math.Sin(age*32)
			if flash > .42 {
				strength := math.Max(0, 1-age/1.70)
				fa := byte(math.Min(190, 45+flash*155*strength))
				alphaSolidRect(hdc, r, rgb(225, 0, 18), fa)
			}
		}
	}

	// Hurt/reboot alarm: when the Sentinel returns to Phase 1 for Round 2,
	// the whole arena violently strobes red as if the boss has been critically wounded.
	if survivalBoss1Stage == survivalBoss1Phase1 && survivalBoss1Cycle >= 2 && !survivalBoss1HurtAt.IsZero() {
		age := now.Sub(survivalBoss1HurtAt).Seconds()
		if age >= 0 && age < 1.25 {
			pulse := .5 + .5*math.Sin(age*55)
			strength := math.Max(0, 1-age/1.25)
			if pulse > .18 {
				fa := byte(math.Min(235, 85+pulse*190*strength))
				alphaSolidRect(hdc, r, rgb(250, 0, 18), fa)
			}
		}
	}

	// Violent red alarm entering the Sentinel's final core-collapse phase.
	if survivalBoss1Stage == survivalBoss1CriticalTelegraph {
		age := now.Sub(survivalBoss1StageAt).Seconds()
		if age < 1.15 {
			pulse := .5 + .5*math.Sin(age*46)
			strength := math.Max(0, 1-age/1.15)
			if pulse > .25 {
				fa := byte(math.Min(225, 75+pulse*180*strength))
				alphaSolidRect(hdc, r, rgb(245, 0, 18), fa)
			}
		}
	}

	// Sentinel appearance-change flash: one bright beat before the held-old sprite
	// swaps, then a second decaying beat immediately after the new sprite appears.
	if !survivalBoss1AppearanceFlashAt.IsZero() {
		age := now.Sub(survivalBoss1AppearanceFlashAt)
		if age >= 0 && age < 300*time.Millisecond {
			ms := float64(age) / float64(time.Millisecond)
			alpha := 0.0
			if ms < 110 {
				// Pre-change flash ramps rapidly to white.
				alpha = 70 + 185*(ms/110.0)
			} else {
				// Post-change flash starts bright then clears quickly.
				alpha = 230 * (1 - (ms-110.0)/190.0)
			}
			if alpha < 0 {
				alpha = 0
			}
			if alpha > 245 {
				alpha = 245
			}
			if alpha > 0 {
				alphaSolidRect(hdc, r, rgb(255, 255, 255), byte(alpha))
			}
		}
	}

	// Symmetrical fades in/out of the boss arena.
	var alpha byte
	switch survivalBoss1Stage {
	case survivalBoss1FadeIn:
		p := math.Min(1, now.Sub(survivalBoss1StageAt).Seconds()/.9)
		alpha = byte((1 - p) * 255)
	case survivalBoss1FadeToReport:
		p := math.Min(1, now.Sub(survivalBoss1StageAt).Seconds()/1.0)
		alpha = byte(p * 255)
	}
	if alpha > 0 {
		alphaSolidRect(hdc, r, rgb(0, 0, 0), alpha)
	}
}

func drawSurvivalBoss1TransitionOverlay(hdc uintptr, w, h int32) {
	if survivalBoss1Stage != survivalBoss1FadeOut && survivalBoss1Stage != survivalBoss1ReturnFadeIn {
		return
	}
	r := arenaRect(w, h)
	now := survivalBossFightNow(time.Now(), survivalBoss1StartedAt)
	p := math.Min(1, now.Sub(survivalBoss1StageAt).Seconds()/1.0)
	alpha := byte(p * 255)
	if survivalBoss1Stage == survivalBoss1ReturnFadeIn {
		alpha = byte((1 - p) * 255)
	}
	if alpha > 0 {
		alphaSolidRect(hdc, r, rgb(0, 0, 0), alpha)
	}
}

func drawSurvivalSectionClearReport(hdc uintptr, w, h int32) {
	if !survivalBoss1SectionReport || state != StateResult {
		return
	}
	ar := arenaRect(w, h)
	alphaSolidRect(hdc, ar, rgb(0, 0, 0), 185)
	width := sx(900, w)
	height := sy(500, h)
	if width < 700 {
		width = 700
	}
	if height < 390 {
		height = 390
	}
	left := ar.Left + (ar.Right-ar.Left-width)/2
	top := ar.Top + (ar.Bottom-ar.Top-height)/2
	panel := RECT{left, top, left + width, top + height}
	drawBevelPanel(hdc, panel, rgb(4, 35, 82), rgb(255, 175, 40), rgb(2, 15, 42), 5)
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 210, 65))
		sectionTitle := "SECTOR 1 CLEAR"
		if survivalSectionReportSector == 2 {
			sectionTitle = "SECTOR 2 CLEAR"
		}
		centeredTextOut(hdc, panel.Left, panel.Right, panel.Top+sy(28, h), sectionTitle)
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(80, 225, 255))
		checkpointText := "CHECKPOINT SECURED // WAVE 11"
		if survivalSectionReportSector == 2 {
			checkpointText = "CHECKPOINT SECURED // WAVE 21"
		}
		centeredTextOut(hdc, panel.Left, panel.Right, panel.Top+sy(78, h), checkpointText)
		selectObject.Call(hdc, old)
	}
	type stat struct{ label, value string }
	damage := survivalTotalDamageTaken
	bossLabel := "SENTINEL DEFEATED"
	if survivalSectionReportSector == 2 {
		bossLabel = "VOID SERPENT DEFEATED"
	}
	stats := []stat{
		{"GRADE", survivalGrade()}, {"SECTION SCORE", fmt.Sprint(score)}, {"ACCURACY", fmt.Sprintf("%.1f%%", survivalPerfectPercent())},
		{"AVG REACTION", fmt.Sprintf("%.0f ms", survivalAverageReactionMS())}, {"PRECISION KILLS", fmt.Sprint(survivalPrecisionKills)}, {"PERFECT WAVES", fmt.Sprint(survivalPerfectWaves)},
		{"HIGHEST COMBO", fmt.Sprintf("x%d", survivalBestCombo)}, {"STATION DAMAGE", fmt.Sprint(damage)}, {"ENEMIES DESTROYED", fmt.Sprint(survivalKills)},
		{"SECTION TIME", fmt.Sprintf("%.1fs", survivalBoss1SectionTime)}, {"BOSS CLEAR TIME", fmt.Sprintf("%.1fs", survivalBoss1ClearTime)}, {"BOSS", bossLabel},
	}
	gridL := panel.Left + sx(40, w)
	gridR := panel.Right - sx(40, w)
	gridT := panel.Top + sy(125, h)
	gapX := sx(10, w)
	gapY := sy(10, h)
	cardW := (gridR - gridL - gapX*3) / 4
	cardH := sy(82, h)
	for i, st := range stats {
		row, col := i/4, i%4
		cr := RECT{gridL + int32(col)*(cardW+gapX), gridT + int32(row)*(cardH+gapY), gridL + int32(col)*(cardW+gapX) + cardW, gridT + int32(row)*(cardH+gapY) + cardH}
		drawBevelPanel(hdc, cr, rgb(3, 25, 62), rgb(45, 145, 205), rgb(1, 12, 32), 2)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, rgb(155, 195, 225))
			centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(14, h), st.label)
			selectObject.Call(hdc, old)
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, rgb(250, 250, 250))
			centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(43, h), st.value)
			selectObject.Call(hdc, old)
		}
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, rgb(95, 255, 135))
		centeredTextOut(hdc, panel.Left, panel.Right, panel.Bottom-sy(62, h), "CHECKPOINT SAVED")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, rgb(255, 215, 80))
		centeredTextOut(hdc, panel.Left, panel.Right, panel.Bottom-sy(34, h), "CLICK TO PROCEED TO SECTOR 2")
		selectObject.Call(hdc, old)
	}
}

// Developer Survival navigation helpers. These intentionally do not alter the
// player's persistent checkpoint; they are QA shortcuts only.
func developerStartSurvivalSection(section int) string {
	if mainHwnd == 0 {
		return "WINDOW NOT READY"
	}
	wave := 1
	switch section {
	case 1:
		wave = 1
	case 2:
		wave = 11
	case 3:
		wave = 21
	default:
		return "USE /SSection 1|2|3"
	}

	stopTransientGameplayAudio()
	survivalBoss1StopMusic()
	clearEnduranceTransientObjects()
	gameMode = 2
	overlayMode = OverlayNone
	menuOpen = false
	resetSurvivalRun()

	now := time.Now()
	survivalWave = wave
	survivalHP = survivalMaxHP()
	survivalStartedAt = now
	survivalLastTick = now
	survivalNextPickupAt = now.Add(10 * time.Second)
	score = 0
	streak = 0
	lastTime = 0
	state = StatePlaying
	startTime = now
	beginSurvivalWave()

	killTimer.Call(mainHwnd, TIMER_GAME)
	setTimer.Call(mainHwnd, TIMER_GAME, 16, 0)
	setCapture.Call(mainHwnd)
	switchModeMusic()
	survivalWaveBannerText = fmt.Sprintf("DEVELOPER // SECTOR %d // WAVE %d", section, wave)
	survivalWaveBannerUntil = now.Add(1800 * time.Millisecond)
	status = fmt.Sprintf("DEVELOPER JUMP // SECTOR %d", section)
	invalidateRect.Call(mainHwnd, 0, 0)
	return fmt.Sprintf("SURVIVAL SECTOR %d STARTED AT WAVE %d — CHECKPOINT NOT MODIFIED", section, wave)
}

func developerStartSurvivalBoss(boss int) string {
	if mainHwnd == 0 {
		return "WINDOW NOT READY"
	}
	if boss < 1 || boss > 3 {
		return "USE /SBoss 1|2|3"
	}

	stopTransientGameplayAudio()
	survivalBoss1StopMusic()
	clearEnduranceTransientObjects()
	gameMode = 2
	overlayMode = OverlayNone
	menuOpen = false
	resetSurvivalRun()

	now := time.Now()
	survivalWave = 10
	survivalHP = survivalMaxHP()
	survivalStartedAt = now
	survivalLastTick = now
	survivalNextPickupAt = time.Time{}
	score = 0
	streak = 0
	lastTime = 0
	state = StatePlaying
	startTime = now
	killTimer.Call(mainHwnd, TIMER_GAME)
	setTimer.Call(mainHwnd, TIMER_GAME, 16, 0)
	setCapture.Call(mainHwnd)

	// Start from normal Survival audio and use the exact production dossier transition.
	switchModeMusic()
	if boss == 3 {
		survivalWave = survivalBoss3Wave
	} else if boss == 2 {
		survivalWave = 20
	}
	beginSurvivalBossDossier(mainHwnd, boss, now)
	invalidateRect.Call(mainHwnd, 0, 0)
	return fmt.Sprintf("SURVIVAL BOSS %d DOSSIER TRANSITION STARTED — CHECKPOINT NOT MODIFIED", boss)
}
