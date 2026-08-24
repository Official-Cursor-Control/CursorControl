//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"time"
	"unsafe"
)

// Boss 2 — The Void Serpent.
// The encounter is isolated from the normal Survival wave director just like Boss 1.
// It is intentionally geometry-led: the body itself is lethal moving level geometry,
// while short-lived red/blue nodes force fast but readable routing decisions.
const (
	survivalBoss2None = iota
	survivalBoss2FadeOut
	survivalBoss2FadeIn
	survivalBoss2Intro
	survivalBoss2Hunt
	survivalBoss2Sweep
	survivalBoss2Coil
	survivalBoss2Frenzy
	survivalBoss2Final
	survivalBoss2Dying
	survivalBoss2FadeToNext
	survivalBoss2ReturnFadeIn
	survivalBoss2MeteorWarning
	survivalBoss2MeteorRun
	survivalBoss2Dodge
	survivalBoss2BeamSetup
	survivalBoss2BeamCharge
	survivalBoss2BeamFire
	survivalBoss2BeamNode
	survivalBoss2BeamRecover
	survivalBoss2BeamExit
	survivalBoss2FinalePreBlack
	survivalBoss2FinaleHead1
	survivalBoss2FinaleHead2
	survivalBoss2FinaleHead3
	survivalBoss2FinaleNode
	survivalBoss2FinaleFlash
)

const (
	survivalBoss2SegmentCount = 35
	survivalBoss2TotalHits    = 16
	survivalBoss2HistoryMax   = 3600
)

const (
	survivalBoss2ReentryNone = iota
	survivalBoss2ReentryLeft
	survivalBoss2ReentryRight
	survivalBoss2ReentryTop
	survivalBoss2ReentryBottom
)

type SurvivalBoss2HistoryPoint struct{ X, Y float64 }
type SurvivalBoss2Segment struct {
	P     FPoint
	Angle float64
	Valid bool
}

type SurvivalBoss2RushEcho struct {
	Segments []SurvivalBoss2Segment
	Vel      FPoint
}

type SurvivalBoss2Meteor struct {
	P         FPoint
	Breakable bool
	Broken    bool
	Variant   int
}

type SurvivalBoss2Satellite struct {
	P       FPoint
	Variant int
	W, H    int32
	Angle   float64
	Row     int
}

// v376: The old full-screen beam was replaced by six discrete Void Energy orbs.
// Each orb is lethal, carries a persistent electric trail, and travels from the
// serpent mouth into one of six horizontal lanes before racing off-screen left.
type SurvivalBoss2EnergyTrailPoint struct {
	P  FPoint
	At time.Time
}

type SurvivalBoss2EnergyBall struct {
	Row      int
	P        FPoint
	LaneY    float64
	OnLane   bool
	Active   bool
	Absorbed bool
	Trail    []SurvivalBoss2EnergyTrailPoint
}

// v407: after the Void Energy volley, the player must clear a ten-hit ordered
// chain of airborne nodes before the mouth core can be damaged. A missed/out-of-order
// relay click or an expired 2s relay timer resets the relay to node 1 without damaging
// the player; only the continuously-running 10s mouth-core timer can fail the phase.
// The nodes are intentionally neutral (purple/white) so this sequence is about
// speed/order/timing, not the red/blue mouse-button rule used by normal serpent cores.
type SurvivalBoss2BeamChainNode struct {
	BaseP FPoint
	Index int
}

// v412 Hunt safety: all five body cores remain permanently anchored to their selected
// articulated body segment. A live core makes a three-piece pocket safe: its host segment
// plus the immediately adjacent body piece on either side. Each core takes five correct hits;
// only the fifth destroys it, after which that same three-piece pocket stays safe for two seconds.
type SurvivalBoss2HuntNode struct {
	Segment        int
	Red            bool
	Alive          bool
	Hits           int
	FlashRemaining int
	FlashUntil     time.Time
	RestoreUntil   time.Time
}

type xformBoss2 struct{ M11, M12, M21, M22, Dx, Dy float32 }

var (
	setGraphicsModeBoss2   = gdi32.NewProc("SetGraphicsMode")
	setWorldTransformBoss2 = gdi32.NewProc("SetWorldTransform")

	survivalBoss2Stage      = survivalBoss2None
	survivalBoss2StageAt    time.Time
	survivalBoss2StartedAt  time.Time
	survivalBoss2LastUpdate time.Time
	survivalBoss2Hits       int
	survivalBoss2Head       FPoint
	survivalBoss2Vel        FPoint
	// v370: randomized node-phase steering. A fresh route is generated for every
	// node-combo stage so repeated playthroughs cannot collapse into the same two
	// recognizable Lissajous paths. Values are bounded so the serpent still presents
	// enough body inside the legal core-spawn rectangle.
	survivalBoss2PathPhaseX float64
	survivalBoss2PathPhaseY float64
	survivalBoss2PathFreqX  float64
	survivalBoss2PathFreqY  float64
	survivalBoss2PathAmpX   float64
	survivalBoss2PathAmpY   float64
	survivalBoss2PathBiasX  float64
	survivalBoss2PathBiasY  float64
	survivalBoss2PathMirror float64
	survivalBoss2History    []SurvivalBoss2HistoryPoint
	survivalBoss2Segments   [survivalBoss2SegmentCount]SurvivalBoss2Segment

	survivalBoss2NodeSegment     int
	survivalBoss2NodeRed         bool
	survivalBoss2NodeTelegraphAt time.Time
	survivalBoss2NodeActiveAt    time.Time
	survivalBoss2NodeExpiresAt   time.Time
	survivalBoss2NextNodeAt      time.Time
	survivalBoss2LastNodeSegment int

	// Nodes are delivered as deliberate attack-run combos instead of unrelated random
	// openings. Each pass places 5-8 core opportunities across front, middle and rear
	// body sections. Nodes may leave the screen without penalty; survival is the primary
	// objective, while successful clicks provide score and speed up visual progress.
	survivalBoss2ComboNumber      int
	survivalBoss2ComboActive      bool
	survivalBoss2ComboAwaitExit   bool
	survivalBoss2ComboAwaitEntry  bool
	survivalBoss2ComboTotal       int
	survivalBoss2ComboIndex       int // number of Hunt cores destroyed in the current pass
	survivalBoss2HuntNodes        [5]SurvivalBoss2HuntNode
	survivalBoss2ComboEntryAt     time.Time
	survivalBoss2ComboCompletedAt time.Time
	// Collision cutout retained briefly after a node is cleared so the body cannot
	// become lethal underneath the cursor on the same click/frame.
	survivalBoss2SafeExitPoint   FPoint
	survivalBoss2SafeExitSegment int
	survivalBoss2SafeExitUntil   time.Time

	survivalBoss2LungeTelegraphAt time.Time
	survivalBoss2LungeStartAt     time.Time
	survivalBoss2LungeEndAt       time.Time
	survivalBoss2LungeTarget      FPoint
	survivalBoss2NextLungeAt      time.Time

	// Re-entry attack state. Once the head has genuinely been on-screen, every later
	// exit arms a predatory return: the current crosshair is sampled, the relevant
	// arena edge telegraphs the attack, and the serpent commits to a broad arc through
	// that sampled point rather than wandering back in on a generic phase path.
	survivalBoss2WasInside bool
	// v414: remember which edge the current roaming pass entered from. Once the head
	// has crossed into the arena, that same edge is forbidden as an exit for the rest
	// of the pass. This prevents the serpent from doubling back through its own body
	// and getting stranded on the same boundary.
	survivalBoss2PassEntrySide      int
	survivalBoss2PassInside         bool
	survivalBoss2ReentryActive      bool
	survivalBoss2ReentrySide        int // committed ENTRY edge; never equals the edge just exited
	survivalBoss2ReentryExitSide    int
	survivalBoss2ReentryRouteStage  int // 0: outside exit edge -> corner, 1: outside entry edge -> staging, 2: attack inward
	survivalBoss2ReentryCorner      FPoint
	survivalBoss2ReentryStaging     FPoint
	survivalBoss2ReentryTarget      FPoint
	survivalBoss2ReentryTelegraphAt time.Time
	survivalBoss2ReentryCommitAt    time.Time
	survivalBoss2ReentryEndAt       time.Time

	// Intermission unlocks only after the player has ACTUALLY hit at least five serpent cores: three falling meteor walls.
	survivalBoss2MeteorDone        bool
	survivalBoss2MeteorRound       int
	survivalBoss2MeteorWaveAt      time.Time
	survivalBoss2MeteorLastUpdate  time.Time
	survivalBoss2MeteorRumbleUntil time.Time
	survivalBoss2Meteors           []SurvivalBoss2Meteor

	// v361 macro-loop: nodes -> meteors -> 5 fast dodges -> 3 satellite beam checks.
	survivalBoss2Loop                 int
	survivalBoss2LoopStartHits        int
	survivalBoss2DodgeIndex           int
	survivalBoss2DodgeFromLeft        bool // retained for compatibility with older debug/status helpers
	survivalBoss2DodgeSide            int
	survivalBoss2DodgePrevSide        int
	survivalBoss2DodgeY               float64
	survivalBoss2DodgeTrailLen        float64
	survivalBoss2DodgeRunAt           time.Time
	survivalBoss2DodgeGapUntil        time.Time
	survivalBoss2DodgeTelegraphAt     time.Time
	survivalBoss2DodgeReturnFlashAt   time.Time // second rush pass: flash the opposite entry edge again
	survivalBoss2DodgeTarget          FPoint
	survivalBoss2DodgeEntered         bool
	survivalBoss2DodgeSubPass         int // final loop: 0 = first strike, 1 = immediate opposite return
	survivalBoss2RushEchoes           []SurvivalBoss2RushEcho
	survivalBoss2BeamCycle            int
	survivalBoss2BeamNodeDeadline     time.Time
	survivalBoss2BeamParkedAt         time.Time
	survivalBoss2BeamSatelliteAt      time.Time
	survivalBoss2BeamRecoverUntil     time.Time
	survivalBoss2BeamRecoverFinal     bool
	survivalBoss2BeamExitStartedAt    time.Time
	survivalBoss2Satellites           []SurvivalBoss2Satellite
	survivalBoss2EnergyBalls          []SurvivalBoss2EnergyBall
	survivalBoss2EnergyLastUpdate     time.Time
	survivalBoss2EnergyDoneAt         time.Time
	survivalBoss2BeamChainNodes       []SurvivalBoss2BeamChainNode
	survivalBoss2BeamChainIndex       int
	survivalBoss2BeamChainDeadline    time.Time
	survivalBoss2BeamChainStartedAt   time.Time
	survivalBoss2BeamHeadFlashUntil   time.Time
	survivalBoss2FinaleHits           int
	survivalBoss2FinaleDeadline       time.Time
	survivalBoss2FinaleHeadFlashUntil time.Time
	survivalBoss2HeartbeatReady       bool
	survivalBoss2HeartbeatActive      bool
	survivalBoss2FinalRoarPlayed      bool

	// Optional audio hooks. Missing files are silently ignored so the fight works now
	// and the user can drop final rumble/smash/serpent vocal assets in later.
	survivalBoss2SerpentStartAudio  bool
	survivalBoss2SerpentAttackAudio [3]bool
	survivalBoss2MeteorRumbleAudio  bool
	survivalBoss2MeteorSmashAudio   bool
	survivalBoss2EnergyBallAudio    bool
	survivalBoss2NextVocalAt        time.Time
	survivalBoss2MouthOpenUntil     time.Time
	survivalBoss2MeteorPending      bool
	survivalBoss2MeteorSerpentTick  time.Time

	survivalBoss2HeadClosed         []byte
	survivalBoss2HeadOpen           []byte
	survivalBoss2Body1              []byte
	survivalBoss2Body2              []byte
	survivalBoss2Body3              []byte
	survivalBoss2Body1Purple        []byte
	survivalBoss2Body2Purple        []byte
	survivalBoss2Body3Purple        []byte
	survivalBoss2HeadPurple         []byte
	survivalBoss2HeadClosedRed      []byte
	survivalBoss2Tail               []byte
	survivalBoss2RedNode            []byte
	survivalBoss2BlueNode           []byte
	survivalBoss2Background         []byte
	survivalBoss2Boulder1           []byte
	survivalBoss2Boulder2           []byte
	survivalBoss2Boulder3           []byte
	survivalBoss2Boulder4           []byte
	survivalBoss2BreakBoulder       []byte
	survivalBoss2BreakBoulderPurple []byte
	survivalBoss2BreakGlow          []byte
	survivalBoss2Satellite1         []byte
	survivalBoss2Satellite2         []byte
	survivalBoss2Satellite3         []byte
	survivalBoss2Satellite4         []byte
	survivalBoss2FinalHead1         []byte
	survivalBoss2FinalHead2         []byte
	survivalBoss2FinalHead3         []byte
	survivalBoss2FinalHead3Red      []byte
	survivalBoss2BeamChainNodeArt   []byte
)

func initSurvivalBoss2Assets() {
	survivalBoss2HeadClosed = readExternalBytes("survival", "boss2", "head_closed.bgra")
	survivalBoss2HeadOpen = readExternalBytes("survival", "boss2", "head_open.bgra")
	survivalBoss2Body1 = readExternalBytes("survival", "boss2", "body_01.bgra")
	survivalBoss2Body2 = readExternalBytes("survival", "boss2", "body_02.bgra")
	survivalBoss2Body3 = readExternalBytes("survival", "boss2", "body_03.bgra")
	// Safe node sections are rendered with an unmistakable purple energy state.
	// The tint is generated once at load time, not per frame.
	survivalBoss2Body1Purple = survivalBoss2PurpleTintBGRA(survivalBoss2Body1)
	survivalBoss2Body2Purple = survivalBoss2PurpleTintBGRA(survivalBoss2Body2)
	survivalBoss2Body3Purple = survivalBoss2PurpleTintBGRA(survivalBoss2Body3)
	survivalBoss2HeadPurple = survivalBoss2PurpleTintBGRA(survivalBoss2HeadClosed)
	survivalBoss2HeadClosedRed = survivalBoss2RedTintBGRA(survivalBoss2HeadClosed)
	survivalBoss2Tail = readExternalBytes("survival", "boss2", "tail.bgra")
	survivalBoss2RedNode = readExternalBytes("survival", "boss2", "core_red.bgra")
	survivalBoss2BlueNode = readExternalBytes("survival", "boss2", "core_blue.bgra")
	survivalBoss2Background = readExternalBytes("survival", "boss2", "background.bgra")
	survivalBoss2Boulder1 = readExternalBytes("survival", "boss2", "boulder_01.bgra")
	survivalBoss2Boulder2 = readExternalBytes("survival", "boss2", "boulder_02.bgra")
	survivalBoss2Boulder3 = readExternalBytes("survival", "boss2", "boulder_03.bgra")
	survivalBoss2Boulder4 = readExternalBytes("survival", "boss2", "boulder_04.bgra")
	survivalBoss2BreakBoulder = readExternalBytes("survival", "boss2", "break_boulder.bgra")
	// v369: the breakable meteor itself flashes purple. Keep the original texture
	// visible underneath and use this precomputed tint only as a low-opacity overlay.
	survivalBoss2BreakBoulderPurple = survivalBoss2PurpleTintBGRA(survivalBoss2BreakBoulder)
	// Legacy asset remains loadable for backwards-compatible asset packs, but the
	// old glowing ring is intentionally no longer rendered.
	survivalBoss2BreakGlow = readExternalBytes("survival", "boss2", "break_boulder_glow.bgra")
	survivalBoss2Satellite1 = readExternalBytes("survival", "boss2", "satellite_01.bgra")
	survivalBoss2Satellite2 = readExternalBytes("survival", "boss2", "satellite_02.bgra")
	survivalBoss2Satellite3 = readExternalBytes("survival", "boss2", "satellite_03.bgra")
	survivalBoss2Satellite4 = readExternalBytes("survival", "boss2", "satellite_04.bgra")
	survivalBoss2FinalHead1 = readExternalBytes("survival", "boss2", "final_head_1.bgra")
	survivalBoss2FinalHead2 = readExternalBytes("survival", "boss2", "final_head_2.bgra")
	survivalBoss2FinalHead3 = readExternalBytes("survival", "boss2", "final_head_3.bgra")
	survivalBoss2FinalHead3Red = survivalBoss2RedTintBGRA(survivalBoss2FinalHead3)
	// Neutral chain-node art: keep the existing node silhouette while removing any
	// red/blue input implication from this ordered timing challenge.
	survivalBoss2BeamChainNodeArt = survivalBoss2PurpleTintBGRA(survivalBoss2RedNode)
}

func survivalBoss2PurpleTintBGRA(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	out := make([]byte, len(src))
	copy(out, src)
	// BGRA: preserve transparency and detail while strongly biasing lit pixels purple.
	for i := 0; i+3 < len(out); i += 4 {
		a := out[i+3]
		if a == 0 {
			continue
		}
		b, g, r := int(out[i]), int(out[i+1]), int(out[i+2])
		lum := (r*3 + g*4 + b*2) / 9
		nr := minIntBoss2(255, 120+lum)
		ng := minIntBoss2(255, 25+lum/3)
		nb := minIntBoss2(255, 150+lum)
		out[i] = byte(nb)
		out[i+1] = byte(ng)
		out[i+2] = byte(nr)
	}
	return out
}

func survivalBoss2RedTintBGRA(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	out := append([]byte(nil), src...)
	for i := 0; i+3 < len(out); i += 4 {
		if out[i+3] == 0 {
			continue
		}
		b, g, r := int(out[i]), int(out[i+1]), int(out[i+2])
		lum := (r*3 + g*4 + b*2) / 9
		out[i] = byte(minIntBoss2(255, 15+lum/5))
		out[i+1] = byte(minIntBoss2(255, 20+lum/4))
		out[i+2] = byte(minIntBoss2(255, 155+lum))
	}
	return out
}

func minIntBoss2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxIntBoss2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func survivalBoss2Active() bool { return survivalBoss2Stage != survivalBoss2None }
func survivalBoss2OwnsArena() bool {
	// Match Boss 1: the outgoing wave remains visible beneath fade-out, and the
	// incoming Sector 3 arena remains visible beneath the return fade-in.
	return survivalBoss2Active() && survivalBoss2Stage != survivalBoss2FadeOut && survivalBoss2Stage != survivalBoss2ReturnFadeIn
}
func survivalBoss2SuppressNormalLogic() bool { return survivalBoss2Active() }
func survivalBoss2CombatActive() bool {
	if state != StatePlaying {
		return false
	}
	return (survivalBoss2Stage >= survivalBoss2Hunt && survivalBoss2Stage <= survivalBoss2Final) ||
		survivalBoss2Stage == survivalBoss2MeteorWarning || survivalBoss2Stage == survivalBoss2MeteorRun ||
		survivalBoss2Stage == survivalBoss2Dodge || survivalBoss2Stage == survivalBoss2BeamSetup ||
		survivalBoss2Stage == survivalBoss2BeamCharge || survivalBoss2Stage == survivalBoss2BeamFire ||
		survivalBoss2Stage == survivalBoss2BeamNode || survivalBoss2Stage == survivalBoss2BeamRecover
}

func survivalBoss2PhaseLabel() string {
	switch survivalBoss2Stage {
	case survivalBoss2FadeOut:
		return "BOSS SIGNAL DETECTED"
	case survivalBoss2FadeIn:
		return "BOSS 2 // VOID SERPENT"
	case survivalBoss2Intro:
		return "BOSS 2 // VOID SERPENT"
	case survivalBoss2Hunt:
		return "PHASE 1 // THE HUNT"
	case survivalBoss2Sweep:
		return "PHASE 2 // THE SWEEP"
	case survivalBoss2Coil:
		return "PHASE 3 // THE COIL"
	case survivalBoss2Frenzy:
		return "PHASE 4 // VOID FRENZY"
	case survivalBoss2Final:
		return "FINAL // HEAD ASSAULT"
	case survivalBoss2Dying, survivalBoss2FadeToNext:
		return "VOID SERPENT DESTROYED"
	case survivalBoss2ReturnFadeIn:
		return "SECTOR 3 // INBOUND"
	case survivalBoss2MeteorWarning:
		return "VOID DISTURBANCE // BRACE"
	case survivalBoss2MeteorRun:
		return fmt.Sprintf("METEOR BREACH // %d / 5", survivalBoss2MeteorRound+1)
	case survivalBoss2Dodge:
		return fmt.Sprintf("VOID RUSH // %d / 5", survivalBoss2DodgeIndex+1)
	case survivalBoss2BeamSetup:
		return "SATELLITE COVER // INBOUND"
	case survivalBoss2BeamCharge:
		return "VOID ENERGY // GET BEHIND COVER"
	case survivalBoss2BeamFire:
		return "VOID ENERGY VOLLEY"
	case survivalBoss2BeamNode:
		if !survivalBoss2BeamChainComplete() && len(survivalBoss2BeamChainNodes) > 0 {
			return fmt.Sprintf("VOID RELAY // NODE %d / %d", survivalBoss2BeamChainIndex+1, len(survivalBoss2BeamChainNodes))
		}
		return "MOUTH CORE // STRIKE NOW"
	case survivalBoss2BeamRecover:
		return "VOID SERPENT // ROAR"
	case survivalBoss2BeamExit:
		return "VOID SERPENT // RETREAT"
	case survivalBoss2FinalePreBlack, survivalBoss2FinaleHead1, survivalBoss2FinaleHead2, survivalBoss2FinaleHead3:
		return "FINAL // CONSCIOUSNESS FADING"
	case survivalBoss2FinaleNode:
		return "FINAL // DESTROY THE MOUTH CORE"
	case survivalBoss2FinaleFlash:
		return "VOID SERPENT DESTROYED"
	}
	return "BOSS 2"
}

func resetSurvivalBoss2State() {
	survivalBoss2Stage = survivalBoss2None
	survivalBoss2StageAt = time.Time{}
	survivalBoss2StartedAt = time.Time{}
	survivalBoss2LastUpdate = time.Time{}
	survivalBoss2Hits = 0
	survivalBoss2Head = FPoint{}
	survivalBoss2Vel = FPoint{}
	survivalBoss2PathPhaseX = 0
	survivalBoss2PathPhaseY = 0
	survivalBoss2PathFreqX = 0
	survivalBoss2PathFreqY = 0
	survivalBoss2PathAmpX = 0
	survivalBoss2PathAmpY = 0
	survivalBoss2PathBiasX = 0
	survivalBoss2PathBiasY = 0
	survivalBoss2PathMirror = 1
	survivalBoss2History = nil
	survivalBoss2Segments = [survivalBoss2SegmentCount]SurvivalBoss2Segment{}
	survivalBoss2NodeSegment = -1
	survivalBoss2LastNodeSegment = -1
	survivalBoss2ComboNumber = 0
	survivalBoss2ComboActive = false
	survivalBoss2ComboAwaitExit = false
	survivalBoss2ComboAwaitEntry = true
	survivalBoss2ComboTotal = 0
	survivalBoss2ComboIndex = 0
	survivalBoss2HuntNodes = [5]SurvivalBoss2HuntNode{}
	survivalBoss2ComboEntryAt = time.Time{}
	survivalBoss2ComboCompletedAt = time.Time{}
	survivalBoss2SafeExitPoint = FPoint{}
	survivalBoss2SafeExitSegment = -1
	survivalBoss2SafeExitUntil = time.Time{}
	survivalBoss2NodeTelegraphAt = time.Time{}
	survivalBoss2NodeActiveAt = time.Time{}
	survivalBoss2NodeExpiresAt = time.Time{}
	survivalBoss2NextNodeAt = time.Time{}
	survivalBoss2NextLungeAt = time.Time{}
	survivalBoss2LungeTelegraphAt = time.Time{}
	survivalBoss2LungeStartAt = time.Time{}
	survivalBoss2LungeEndAt = time.Time{}
	survivalBoss2WasInside = false
	survivalBoss2PassEntrySide = survivalBoss2ReentryNone
	survivalBoss2PassInside = false
	survivalBoss2ReentryActive = false
	survivalBoss2ReentrySide = survivalBoss2ReentryNone
	survivalBoss2ReentryExitSide = survivalBoss2ReentryNone
	survivalBoss2ReentryRouteStage = 0
	survivalBoss2ReentryCorner = FPoint{}
	survivalBoss2ReentryStaging = FPoint{}
	survivalBoss2ReentryTarget = FPoint{}
	survivalBoss2ReentryTelegraphAt = time.Time{}
	survivalBoss2ReentryCommitAt = time.Time{}
	survivalBoss2ReentryEndAt = time.Time{}
	survivalBoss2MeteorDone = false
	survivalBoss2MeteorRound = 0
	survivalBoss2MeteorWaveAt = time.Time{}
	survivalBoss2MeteorLastUpdate = time.Time{}
	survivalBoss2Meteors = nil
	survivalBoss2Loop = 0
	survivalBoss2LoopStartHits = 0
	survivalBoss2DodgeIndex = 0
	survivalBoss2DodgeSide = survivalBoss2ReentryNone
	survivalBoss2DodgePrevSide = survivalBoss2ReentryNone
	survivalBoss2DodgeTrailLen = 0
	survivalBoss2DodgeRunAt = time.Time{}
	survivalBoss2DodgeGapUntil = time.Time{}
	survivalBoss2DodgeTelegraphAt = time.Time{}
	survivalBoss2DodgeReturnFlashAt = time.Time{}
	survivalBoss2DodgeTarget = FPoint{}
	survivalBoss2DodgeEntered = false
	survivalBoss2DodgeSubPass = 0
	survivalBoss2RushEchoes = nil
	survivalBoss2BeamCycle = 0
	survivalBoss2BeamNodeDeadline = time.Time{}
	survivalBoss2BeamParkedAt = time.Time{}
	survivalBoss2BeamSatelliteAt = time.Time{}
	survivalBoss2BeamRecoverUntil = time.Time{}
	survivalBoss2BeamRecoverFinal = false
	survivalBoss2BeamExitStartedAt = time.Time{}
	survivalBoss2Satellites = nil
	survivalBoss2EnergyBalls = nil
	survivalBoss2EnergyLastUpdate = time.Time{}
	survivalBoss2EnergyDoneAt = time.Time{}
	survivalBoss2BeamChainNodes = nil
	survivalBoss2BeamChainIndex = 0
	survivalBoss2BeamChainDeadline = time.Time{}
	survivalBoss2BeamChainStartedAt = time.Time{}
	survivalBoss2BeamHeadFlashUntil = time.Time{}
	survivalBoss2FinaleHits = 0
	survivalBoss2FinaleDeadline = time.Time{}
	survivalBoss2FinaleHeadFlashUntil = time.Time{}
	survivalBoss2HeartbeatActive = false
	survivalBoss2FinalRoarPlayed = false
	mci("stop survival_boss2_heartbeat")
	mci("seek survival_boss2_heartbeat to start")
	survivalBoss2NextVocalAt = time.Time{}
	survivalBoss2MouthOpenUntil = time.Time{}
	survivalBoss2MeteorPending = false
	survivalBoss2MeteorSerpentTick = time.Time{}
}

func beginSurvivalBoss2(h uintptr, now time.Time) {
	analyticsEvent("boss_attempted", map[string]any{"boss": "VOID_SERPENT", "wave": survivalWave})
	resetSurvivalBoss2State()
	if survivalBossIntroHandoff {
		survivalBoss2Stage = survivalBoss2Intro
	} else {
		survivalBoss2Stage = survivalBoss2FadeOut
	}
	survivalBoss2StageAt = now
	survivalBoss2StartedAt = now
	survivalBoss2LastUpdate = now
	w, hgt := getClient(h)
	r := arenaRect(w, hgt)
	headX := r.Right + sx(110, w)
	if survivalBossIntroHandoff {
		// Let the player see the first section of the Serpent as the dossier fades
		// into the live arena; combat itself still cannot begin under black.
		headX = r.Right + sx(24, w)
	}
	survivalBoss2Head = FPoint{X: float64(headX), Y: float64(r.Top + (r.Bottom-r.Top)/2)}
	survivalBoss2Vel = FPoint{X: -290, Y: 0}
	// The opening pass enters from the right. Once it has crossed into the arena it
	// must leave by left, top or bottom -- never by the right edge again.
	survivalBoss2PassEntrySide = survivalBoss2ReentryRight
	survivalBoss2PassInside = false
	// Pre-seed the full serpent behind the entering head instead of growing the chain
	// from nothing. The head can enter the arena while metres of body and the tail are
	// still trailing far offscreen, giving Boss 2 the scale of one continuous creature.
	spacing := survivalBoss2SegmentSpacing(w)
	trailLen := float64(survivalBoss2SegmentCount+3) * spacing
	for d := trailLen; d >= 0; d -= 1.5 {
		survivalBoss2History = append(survivalBoss2History, SurvivalBoss2HistoryPoint{survivalBoss2Head.X + d, survivalBoss2Head.Y})
	}
	survivalBoss2NextNodeAt = now.Add(3300 * time.Millisecond)
	survivalBoss2NextLungeAt = now.Add(6500 * time.Millisecond)
	if survivalBossIntroHandoff {
		survivalWaveBannerText = ""
		survivalWaveBannerUntil = time.Time{}
		status = "VOID SERPENT // DOSSIER COMPLETE"
	} else {
		survivalWaveBannerText = "SECTOR 2 COMPLETE // BOSS SIGNAL DETECTED"
		survivalWaveBannerUntil = now.Add(1100 * time.Millisecond)
		status = "BOSS SIGNAL DETECTED // VOID SERPENT"
		// Reuse Boss 1's proven MCI fade/handoff system. Sector 2 fades while
		// the arena fades to full black; boss music starts only at full black.
		if audioReady && survivalSection2MusicReady {
			survivalBoss1FadeAlias("survival_section2", survivalBoss1TargetMusicVolume(), 0, 950*time.Millisecond, false)
		}
	}
	survivalEnemies = nil
	survivalPickup = nil
	survivalPickup2 = nil
	invalidateSurvivalArena(h)
	invalidateSurvivalHUD(h)
}

func survivalBoss2RandomizeNodePath(stage int) {
	// Hundreds of combinations are possible before even accounting for starting
	// position/re-entry angle. Frequencies deliberately avoid simple integer ratios.
	survivalBoss2PathPhaseX = rand.Float64() * 2 * math.Pi
	survivalBoss2PathPhaseY = rand.Float64() * 2 * math.Pi
	survivalBoss2PathFreqX = 0.32 + rand.Float64()*0.42
	survivalBoss2PathFreqY = 0.39 + rand.Float64()*0.56
	if math.Abs(survivalBoss2PathFreqX-survivalBoss2PathFreqY) < 0.08 {
		survivalBoss2PathFreqY += 0.13
	}
	survivalBoss2PathAmpX = 360 + rand.Float64()*215
	survivalBoss2PathAmpY = 135 + rand.Float64()*105
	survivalBoss2PathBiasX = (rand.Float64()*2 - 1) * 105
	survivalBoss2PathBiasY = (rand.Float64()*2 - 1) * 60
	survivalBoss2PathMirror = 1
	if rand.Intn(2) == 0 {
		survivalBoss2PathMirror = -1
	}
	// Later node phases can roam a little wider while still respecting the spawn filter.
	if stage >= survivalBoss2Frenzy {
		survivalBoss2PathAmpX *= 1.06
		survivalBoss2PathAmpY *= 1.08
	}
}

func survivalBoss2SetStage(stage int, now time.Time) {
	if survivalBoss2Stage == stage {
		return
	}
	survivalBoss2Stage = stage
	survivalBoss2StageAt = now
	if stage >= survivalBoss2Hunt && stage <= survivalBoss2Final {
		survivalBoss2RandomizeNodePath(stage)
		survivalBoss2HuntNodes = [5]SurvivalBoss2HuntNode{}
		survivalBoss2ComboTotal = 0
		survivalBoss2ComboIndex = 0
	}
	survivalBoss2NextNodeAt = time.Time{}
	survivalBoss2NodeSegment = -1
	survivalBoss2NodeActiveAt = time.Time{}
	survivalBoss2NodeExpiresAt = time.Time{}
	survivalBoss2NodeTelegraphAt = time.Time{}
	survivalWaveBannerText = survivalBoss2PhaseLabel()
	survivalWaveBannerUntil = now.Add(1500 * time.Millisecond)
}

func survivalBoss2NodeWindow() time.Duration {
	// v309: cores remain available much longer. Boss 2 is an endurance fight,
	// not a twitch-only reaction check.
	switch survivalBoss2Stage {
	case survivalBoss2Hunt:
		return 3200 * time.Millisecond
	case survivalBoss2Sweep:
		return 3050 * time.Millisecond
	case survivalBoss2Coil:
		return 2900 * time.Millisecond
	case survivalBoss2Frenzy:
		return 2750 * time.Millisecond
	case survivalBoss2Final:
		return 2600 * time.Millisecond
	}
	return 3200 * time.Millisecond
}

func survivalBoss2SegmentSpacing(w int32) float64     { return float64(sx(63, w)) }
func survivalBoss2BodyRadius(w int32) float64         { return float64(sx(32, w)) }
func survivalBoss2HeadRadius(w int32) float64         { return float64(sx(48, w)) }
func survivalBoss2NodeRadius(w int32) float64         { return float64(sx(50, w)) }
func survivalBoss2NodeSafeRadius(w int32) float64     { return float64(sx(92, w)) }
func survivalBoss2NodeExitSafeRadius(w int32) float64 { return float64(sx(108, w)) }

func boss2Norm(v FPoint) FPoint {
	d := math.Hypot(v.X, v.Y)
	if d < 1e-6 {
		return FPoint{1, 0}
	}
	return FPoint{v.X / d, v.Y / d}
}
func boss2Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// survivalBoss2SelfClearanceAt measures the head/path clearance from the older,
// visible body. The first several neck segments are intentionally ignored because
// they are supposed to sit directly behind the head. Any later segment represents
// an older section of the travelled path and must never be crossed by the head.
func survivalBoss2SelfClearanceAt(p FPoint, w int32) (float64, FPoint, bool) {
	minD := math.MaxFloat64
	nearest := FPoint{}
	found := false
	const neckIgnore = 8
	for i := neckIgnore; i < len(survivalBoss2Segments); i++ {
		seg := survivalBoss2Segments[i]
		if !seg.Valid {
			continue
		}
		d := math.Hypot(p.X-seg.P.X, p.Y-seg.P.Y)
		if d < minD {
			minD = d
			nearest = seg.P
			found = true
		}
	}
	return minD, nearest, found
}

// survivalBoss2ApplySelfAvoidance starts bending the head away well before a
// self-cross can occur. This keeps the avoidance looking like deliberate serpent
// steering rather than an emergency teleport or snap.
func survivalBoss2ApplySelfAvoidance(desired FPoint, w int32) (FPoint, float64) {
	minD, nearest, ok := survivalBoss2SelfClearanceAt(survivalBoss2Head, w)
	if !ok {
		return desired, 0
	}
	spacing := survivalBoss2SegmentSpacing(w)
	safe := math.Max(spacing*1.62, survivalBoss2HeadRadius(w)+survivalBoss2BodyRadius(w)+float64(sx(12, w)))
	influence := safe + spacing*0.92
	if minD >= influence {
		return desired, 0
	}
	risk := boss2Clamp((influence-minD)/(influence-safe), 0, 1)
	away := boss2Norm(FPoint{X: survivalBoss2Head.X - nearest.X, Y: survivalBoss2Head.Y - nearest.Y})
	forward := boss2Norm(desired)
	// Increase repulsion non-linearly near the no-cross boundary while retaining a
	// meaningful forward component, so the serpent flows around itself instead of stalling.
	weight := 0.75 + 2.35*risk*risk
	steered := boss2Norm(FPoint{X: forward.X + away.X*weight, Y: forward.Y + away.Y*weight})
	speed := math.Hypot(desired.X, desired.Y)
	return FPoint{X: steered.X * speed, Y: steered.Y * speed}, risk
}

// survivalBoss2GuardSelfIntersection is the final geometric guarantee. Normal
// steering should resolve almost every case before this runs; if a single high-speed
// frame would still place the head inside an older body section, choose the smallest
// nearby heading that increases clearance. This prevents visual/body path crossings
// at low frame rates without changing position discontinuously.
func survivalBoss2GuardSelfIntersection(dt float64, w int32) {
	if dt <= 0 {
		return
	}
	speed := math.Hypot(survivalBoss2Vel.X, survivalBoss2Vel.Y)
	if speed < 1 {
		return
	}
	spacing := survivalBoss2SegmentSpacing(w)
	safe := math.Max(spacing*1.52, survivalBoss2HeadRadius(w)+survivalBoss2BodyRadius(w)+float64(sx(8, w)))
	candidate := FPoint{X: survivalBoss2Head.X + survivalBoss2Vel.X*dt, Y: survivalBoss2Head.Y + survivalBoss2Vel.Y*dt}
	baseClear, _, ok := survivalBoss2SelfClearanceAt(candidate, w)
	if !ok || baseClear >= safe {
		return
	}

	baseA := math.Atan2(survivalBoss2Vel.Y, survivalBoss2Vel.X)
	bestA := baseA
	bestClear := baseClear
	// Search symmetrically so avoidance does not have a permanent clockwise bias.
	for step := 1; step <= 9; step++ {
		delta := float64(step) * (math.Pi / 18.0) // 10 degree increments, max 90 degrees.
		for _, sign := range []float64{-1, 1} {
			a := baseA + sign*delta
			q := FPoint{X: survivalBoss2Head.X + math.Cos(a)*speed*dt, Y: survivalBoss2Head.Y + math.Sin(a)*speed*dt}
			clearance, _, found := survivalBoss2SelfClearanceAt(q, w)
			if !found {
				continue
			}
			if clearance > bestClear {
				bestClear = clearance
				bestA = a
			}
			if clearance >= safe {
				survivalBoss2Vel = FPoint{X: math.Cos(a) * speed, Y: math.Sin(a) * speed}
				return
			}
		}
	}
	if bestClear > baseClear {
		// If angular steering alone cannot restore the full clearance this frame, preserve
		// the safest heading and smoothly shed speed until the next point remains outside
		// the no-cross envelope. The zero-speed fallback is intentionally last-resort and
		// should be practically unreachable because the soft avoidance begins much earlier.
		for _, scale := range []float64{1.0, 0.78, 0.56, 0.34, 0.16, 0.0} {
			q := FPoint{X: survivalBoss2Head.X + math.Cos(bestA)*speed*scale*dt, Y: survivalBoss2Head.Y + math.Sin(bestA)*speed*scale*dt}
			clearance, _, found := survivalBoss2SelfClearanceAt(q, w)
			if !found || clearance >= safe {
				survivalBoss2Vel = FPoint{X: math.Cos(bestA) * speed * scale, Y: math.Sin(bestA) * speed * scale}
				return
			}
		}
	}
	// Absolute geometric guarantee: if the older body temporarily surrounds every
	// candidate heading, do not advance the head into it. Early steering will open a
	// route on the following frame; position continuity is preserved throughout.
	survivalBoss2Vel = FPoint{}
}

func survivalBoss2HeadInsideArena(w, h int32) bool {
	r := arenaRect(w, h)
	return survivalBoss2Head.X >= float64(r.Left) && survivalBoss2Head.X <= float64(r.Right) &&
		survivalBoss2Head.Y >= float64(r.Top) && survivalBoss2Head.Y <= float64(r.Bottom)
}

func survivalBoss2ClampTargetToArena(p FPoint, w, h int32) FPoint {
	r := arenaRect(w, h)
	mx := float64(sx(105, w))
	my := float64(sy(90, h))
	p.X = boss2Clamp(p.X, float64(r.Left)+mx, float64(r.Right)-mx)
	p.Y = boss2Clamp(p.Y, float64(r.Top)+my, float64(r.Bottom)-my)
	return p
}

func survivalBoss2DetectExitSide(w, h int32) int {
	r := arenaRect(w, h)
	p := survivalBoss2Head
	left := float64(r.Left) - p.X
	right := p.X - float64(r.Right)
	top := float64(r.Top) - p.Y
	bottom := p.Y - float64(r.Bottom)
	best, side := left, survivalBoss2ReentryLeft
	if right > best {
		best, side = right, survivalBoss2ReentryRight
	}
	if top > best {
		best, side = top, survivalBoss2ReentryTop
	}
	if bottom > best {
		side = survivalBoss2ReentryBottom
	}
	return side
}

// Returns the edge the head is physically outside of right now. Unlike the older
// nearest-edge helper, this returns NONE while the head is inside, so the Hunt entry
// telegraph can never announce a stale/opposite side.
func survivalBoss2PhysicalOutsideSide(w, h int32) int {
	r := arenaRect(w, h)
	p := survivalBoss2Head
	best := 0.0
	side := survivalBoss2ReentryNone
	if d := float64(r.Left) - p.X; d > best {
		best, side = d, survivalBoss2ReentryLeft
	}
	if d := p.X - float64(r.Right); d > best {
		best, side = d, survivalBoss2ReentryRight
	}
	if d := float64(r.Top) - p.Y; d > best {
		best, side = d, survivalBoss2ReentryTop
	}
	if d := p.Y - float64(r.Bottom); d > best {
		side = survivalBoss2ReentryBottom
	}
	return side
}

// v414: once a normal serpent pass has entered the arena, it may not leave through
// that same edge. If steering tries to double back across the entry boundary, keep the
// head just inside and redirect it inward with a tangential component. The resulting
// broad turn preserves the snake-like motion while guaranteeing a different exit edge.
func survivalBoss2PreventSameSideExit(prev FPoint, w, h int32) {
	if !survivalBoss2PassInside || survivalBoss2PassEntrySide == survivalBoss2ReentryNone {
		return
	}
	outside := survivalBoss2PhysicalOutsideSide(w, h)
	if outside == survivalBoss2ReentryNone || outside != survivalBoss2PassEntrySide {
		return
	}
	r := arenaRect(w, h)
	padX := float64(sx(5, w))
	padY := float64(sy(5, h))
	spd := math.Hypot(survivalBoss2Vel.X, survivalBoss2Vel.Y)
	if spd < 1 {
		spd = float64(sx(315, w)) * survivalBoss2ExtraMovementScale
	}
	// Preserve the existing tangential direction where possible so the correction
	// reads as one continuous arc instead of a bounce. If it is nearly zero, choose
	// the tangent that points towards the arena centre.
	var vx, vy float64
	cx := float64(r.Left+r.Right) * .5
	cy := float64(r.Top+r.Bottom) * .5
	switch outside {
	case survivalBoss2ReentryLeft:
		survivalBoss2Head.X = float64(r.Left) + padX
		survivalBoss2Head.Y = boss2Clamp(prev.Y, float64(r.Top)+padY, float64(r.Bottom)-padY)
		vx = 1.0
		vy = survivalBoss2Vel.Y / spd
		if math.Abs(vy) < .18 {
			vy = math.Copysign(.55, cy-survivalBoss2Head.Y)
		}
	case survivalBoss2ReentryRight:
		survivalBoss2Head.X = float64(r.Right) - padX
		survivalBoss2Head.Y = boss2Clamp(prev.Y, float64(r.Top)+padY, float64(r.Bottom)-padY)
		vx = -1.0
		vy = survivalBoss2Vel.Y / spd
		if math.Abs(vy) < .18 {
			vy = math.Copysign(.55, cy-survivalBoss2Head.Y)
		}
	case survivalBoss2ReentryTop:
		survivalBoss2Head.Y = float64(r.Top) + padY
		survivalBoss2Head.X = boss2Clamp(prev.X, float64(r.Left)+padX, float64(r.Right)-padX)
		vy = 1.0
		vx = survivalBoss2Vel.X / spd
		if math.Abs(vx) < .18 {
			vx = math.Copysign(.55, cx-survivalBoss2Head.X)
		}
	case survivalBoss2ReentryBottom:
		survivalBoss2Head.Y = float64(r.Bottom) - padY
		survivalBoss2Head.X = boss2Clamp(prev.X, float64(r.Left)+padX, float64(r.Right)-padX)
		vy = -1.0
		vx = survivalBoss2Vel.X / spd
		if math.Abs(vx) < .18 {
			vx = math.Copysign(.55, cx-survivalBoss2Head.X)
		}
	}
	n := boss2Norm(FPoint{X: vx, Y: vy})
	survivalBoss2Vel = FPoint{X: n.X * spd, Y: n.Y * spd}
}

// survivalBoss2ChooseReentrySide deliberately chooses an ADJACENT edge, preserving
// the serpent's current travel direction around an outside corner. This guarantees the
// return edge can never be the edge it just left while avoiding an unnatural 180-degree
// reversal outside the arena.
func survivalBoss2ChooseReentrySide(exitSide int) int {
	switch exitSide {
	case survivalBoss2ReentryLeft, survivalBoss2ReentryRight:
		if survivalBoss2Vel.Y >= 0 {
			return survivalBoss2ReentryBottom
		}
		return survivalBoss2ReentryTop
	case survivalBoss2ReentryTop, survivalBoss2ReentryBottom:
		if survivalBoss2Vel.X >= 0 {
			return survivalBoss2ReentryRight
		}
		return survivalBoss2ReentryLeft
	}
	return survivalBoss2ReentryLeft
}

func survivalBoss2BuildReentryRoute(exitSide, entrySide int, target FPoint, w, h int32) (FPoint, FPoint) {
	r := arenaRect(w, h)
	mx := float64(sx(170, w))
	my := float64(sy(145, h))
	left := float64(r.Left) - mx
	right := float64(r.Right) + mx
	top := float64(r.Top) - my
	bottom := float64(r.Bottom) + my
	target.X = boss2Clamp(target.X, float64(r.Left)+float64(sx(110, w)), float64(r.Right)-float64(sx(110, w)))
	target.Y = boss2Clamp(target.Y, float64(r.Top)+float64(sy(90, h)), float64(r.Bottom)-float64(sy(90, h)))

	corner := FPoint{}
	staging := FPoint{}
	switch exitSide {
	case survivalBoss2ReentryBottom:
		if entrySide == survivalBoss2ReentryRight {
			corner = FPoint{X: right, Y: bottom}
			staging = FPoint{X: right, Y: target.Y}
		} else {
			corner = FPoint{X: left, Y: bottom}
			staging = FPoint{X: left, Y: target.Y}
		}
	case survivalBoss2ReentryTop:
		if entrySide == survivalBoss2ReentryRight {
			corner = FPoint{X: right, Y: top}
			staging = FPoint{X: right, Y: target.Y}
		} else {
			corner = FPoint{X: left, Y: top}
			staging = FPoint{X: left, Y: target.Y}
		}
	case survivalBoss2ReentryLeft:
		if entrySide == survivalBoss2ReentryBottom {
			corner = FPoint{X: left, Y: bottom}
			staging = FPoint{X: target.X, Y: bottom}
		} else {
			corner = FPoint{X: left, Y: top}
			staging = FPoint{X: target.X, Y: top}
		}
	case survivalBoss2ReentryRight:
		if entrySide == survivalBoss2ReentryBottom {
			corner = FPoint{X: right, Y: bottom}
			staging = FPoint{X: target.X, Y: bottom}
		} else {
			corner = FPoint{X: right, Y: top}
			staging = FPoint{X: target.X, Y: top}
		}
	}
	return corner, staging
}

// While the serpent is routing around the outside corner, curvature smoothing is not
// allowed to accidentally nose it back through the edge it just exited. Stage 0 stays
// outside the exit edge; stage 1 stays outside the committed new entry edge.
func survivalBoss2KeepReentryRouteOutside(w, h int32) {
	if !survivalBoss2ReentryActive || survivalBoss2ReentryRouteStage >= 2 {
		return
	}
	r := arenaRect(w, h)
	padX := float64(sx(8, w))
	padY := float64(sy(8, h))
	side := survivalBoss2ReentryExitSide
	if survivalBoss2ReentryRouteStage == 1 {
		side = survivalBoss2ReentrySide
	}
	switch side {
	case survivalBoss2ReentryLeft:
		limit := float64(r.Left) - padX
		if survivalBoss2Head.X > limit {
			survivalBoss2Head.X = limit
			if survivalBoss2Vel.X > 0 {
				survivalBoss2Vel.X = 0
			}
		}
	case survivalBoss2ReentryRight:
		limit := float64(r.Right) + padX
		if survivalBoss2Head.X < limit {
			survivalBoss2Head.X = limit
			if survivalBoss2Vel.X < 0 {
				survivalBoss2Vel.X = 0
			}
		}
	case survivalBoss2ReentryTop:
		limit := float64(r.Top) - padY
		if survivalBoss2Head.Y > limit {
			survivalBoss2Head.Y = limit
			if survivalBoss2Vel.Y > 0 {
				survivalBoss2Vel.Y = 0
			}
		}
	case survivalBoss2ReentryBottom:
		limit := float64(r.Bottom) + padY
		if survivalBoss2Head.Y < limit {
			survivalBoss2Head.Y = limit
			if survivalBoss2Vel.Y < 0 {
				survivalBoss2Vel.Y = 0
			}
		}
	}
}

func survivalBoss2NearlyOffscreen(w, h int32) bool {
	r := arenaRect(w, h)
	visible := 0
	marginX := float64(sx(70, w))
	marginY := float64(sy(55, h))
	for _, seg := range survivalBoss2Segments {
		if !seg.Valid {
			continue
		}
		if seg.P.X >= float64(r.Left)-marginX && seg.P.X <= float64(r.Right)+marginX &&
			seg.P.Y >= float64(r.Top)-marginY && seg.P.Y <= float64(r.Bottom)+marginY {
			visible++
		}
	}
	return !survivalBoss2HeadInsideArena(w, h) && visible <= 4
}

func survivalBoss2StartMeteorWhenClear(now time.Time, w, h int32) bool {
	if !survivalBoss2MeteorPending || !survivalBoss2NearlyOffscreen(w, h) {
		return false
	}
	survivalBoss2MeteorPending = false
	survivalBoss2ComboAwaitExit = false
	survivalBoss2SetStage(survivalBoss2MeteorWarning, now)
	survivalBoss2MeteorRound = 0
	survivalBoss2MeteorWaveAt = time.Time{}
	survivalBoss2MeteorLastUpdate = now
	survivalBoss2MeteorSerpentTick = now
	survivalBoss2Meteors = nil
	survivalBoss2ComboAwaitEntry = false
	survivalBoss2ReentryActive = false
	survivalWaveBannerText = "VOID DISTURBANCE // BRACE"
	survivalWaveBannerUntil = now.Add(1200 * time.Millisecond)
	playSurvivalBoss2MeteorRumble()
	return true
}

func survivalBoss2MaybeArmReentry(now time.Time, w, h int32) {
	if survivalBoss2MeteorPending {
		return
	}
	if !survivalBoss2CombatActive() {
		return
	}
	inside := survivalBoss2HeadInsideArena(w, h)
	if inside {
		if !survivalBoss2PassInside {
			survivalBoss2PassInside = true
			// ReentrySide is the committed physical incoming edge for routed returns.
			// The opening pass pre-seeds PassEntrySide directly.
			if survivalBoss2ReentryActive && survivalBoss2ReentrySide != survivalBoss2ReentryNone {
				survivalBoss2PassEntrySide = survivalBoss2ReentrySide
			}
		}
		survivalBoss2WasInside = true
		// Do not cancel the attack merely because the nose crossed the boundary. The
		// return stays committed until it has actually swept close to the sampled
		// crosshair point, preserving the feeling that the serpent came back for you.
		if survivalBoss2ReentryActive && math.Hypot(
			survivalBoss2Head.X-survivalBoss2ReentryTarget.X,
			survivalBoss2Head.Y-survivalBoss2ReentryTarget.Y) <= float64(sx(165, w)) {
			survivalBoss2ReentryActive = false
			survivalBoss2ReentrySide = survivalBoss2ReentryNone
		}
		return
	}
	if survivalBoss2WasInside && !survivalBoss2ReentryActive {
		// Do not arm a new pass if the player let an active combo leave unfinished.
		// updateSurvivalBoss2 will convert this state into a failure on the same tick.
		if survivalBoss2ComboActive {
			return
		}
		survivalBoss2WasInside = false
		survivalBoss2PassInside = false
		if survivalBoss2ComboAwaitExit {
			survivalBoss2ComboAwaitExit = false
			survivalBoss2ComboNumber++
			// Meteor breach is earned by REAL successful core hits, not by surviving a
			// fixed number of passes. It cannot start until at least five nodes have
			// actually been clicked. Missed/off-screen nodes never count toward this gate.
			if survivalBoss2Hits-survivalBoss2LoopStartHits >= 5 && !survivalBoss2MeteorDone {
				// Do not pop the serpent out of existence. Keep it travelling along its
				// retreat vector until almost the entire body has naturally left view,
				// then begin the meteor breach.
				survivalBoss2MeteorPending = true
				survivalBoss2ComboAwaitExit = true
				survivalBoss2ReentryActive = false
				return
			}
			// If the player reaches the nominal final pass without five actual hits,
			// keep cycling the final attack instead of skipping the meteor mechanic.
			// This preserves the endurance-first design while guaranteeing more chances.
			if survivalBoss2ComboNumber >= 5 && !survivalBoss2MeteorDone {
				survivalBoss2ComboNumber = 4
			}
			if survivalBoss2ComboNumber < 5 {
				survivalBoss2SetStage(survivalBoss2ComboStage(survivalBoss2ComboNumber), now)
				survivalBoss2ComboAwaitEntry = true
				survivalBoss2ComboEntryAt = time.Time{}
			}
		}
		survivalBoss2ReentryActive = true
		survivalBoss2ReentryExitSide = survivalBoss2DetectExitSide(w, h)
		survivalBoss2ReentrySide = survivalBoss2ChooseReentrySide(survivalBoss2ReentryExitSide)
		// Hard invariant: the committed re-entry edge can never match the exit edge.
		if survivalBoss2ReentrySide == survivalBoss2ReentryExitSide {
			if survivalBoss2ReentryExitSide == survivalBoss2ReentryLeft || survivalBoss2ReentryExitSide == survivalBoss2ReentryRight {
				survivalBoss2ReentrySide = survivalBoss2ReentryTop
			} else {
				survivalBoss2ReentrySide = survivalBoss2ReentryLeft
			}
		}
		survivalBoss2PassEntrySide = survivalBoss2ReentrySide
		survivalBoss2ReentryTarget = survivalBoss2ClampTargetToArena(cursorPos, w, h)
		survivalBoss2ReentryCorner, survivalBoss2ReentryStaging = survivalBoss2BuildReentryRoute(
			survivalBoss2ReentryExitSide, survivalBoss2ReentrySide, survivalBoss2ReentryTarget, w, h)
		survivalBoss2ReentryRouteStage = 0
		survivalBoss2ReentryTelegraphAt = time.Time{}
		survivalBoss2ReentryCommitAt = now
		survivalBoss2ReentryEndAt = now.Add(9000 * time.Millisecond)
		// Do not let a scheduled head lunge compete with the return attack.
		survivalBoss2LungeTelegraphAt = time.Time{}
		survivalBoss2LungeStartAt = time.Time{}
		survivalBoss2LungeEndAt = time.Time{}
		survivalBoss2NextLungeAt = now.Add(2600 * time.Millisecond)
	}
	if survivalBoss2ReentryActive {
		// The entry edge is selected when the serpent exits and is never re-evaluated to
		// the old edge. A timed-out OFF-SCREEN route is recovered by committing straight
		// back into the arena instead of dropping ReentryActive and risking a stranded boss.
		if !survivalBoss2ReentryEndAt.IsZero() && now.After(survivalBoss2ReentryEndAt) {
			if !survivalBoss2HeadInsideArena(w, h) {
				r := arenaRect(w, h)
				if actual := survivalBoss2PhysicalOutsideSide(w, h); actual != survivalBoss2ReentryNone {
					survivalBoss2ReentrySide = actual
				}
				survivalBoss2ReentryRouteStage = 2
				survivalBoss2ReentryTarget = FPoint{X: float64(r.Left+r.Right) * .5, Y: float64(r.Top+r.Bottom) * .5}
				survivalBoss2ReentryTelegraphAt = now
				survivalBoss2ReentryEndAt = now.Add(4500 * time.Millisecond)
				survivalWaveBannerText = "VOID SERPENT // FORCED RE-ENTRY"
				survivalWaveBannerUntil = now.Add(650 * time.Millisecond)
			} else {
				survivalBoss2ReentryActive = false
				survivalBoss2ReentrySide = survivalBoss2ReentryNone
				survivalBoss2ReentryExitSide = survivalBoss2ReentryNone
				survivalBoss2ReentryRouteStage = 0
			}
		}
	}
}

const survivalBoss2ExtraMovementScale = 1.60 / survivalBossFightSpeed

func survivalBoss2ReentryVelocity(now time.Time, w, h int32, baseSpeed float64) (FPoint, bool) {
	if !survivalBoss2ReentryActive {
		return FPoint{}, false
	}
	p := survivalBoss2Head
	target := survivalBoss2ReentryTarget
	threshold := float64(sx(78, w))

	switch survivalBoss2ReentryRouteStage {
	case 0:
		target = survivalBoss2ReentryCorner
		if math.Hypot(target.X-p.X, target.Y-p.Y) <= threshold {
			survivalBoss2ReentryRouteStage = 1
			target = survivalBoss2ReentryStaging
		}
	case 1:
		target = survivalBoss2ReentryStaging
		if math.Hypot(target.X-p.X, target.Y-p.Y) <= threshold {
			// Resolve the warning from the head's REAL outside edge at the instant the
			// inward attack begins. This removes the stale-side mismatch seen after turns.
			if actual := survivalBoss2PhysicalOutsideSide(w, h); actual != survivalBoss2ReentryNone {
				survivalBoss2ReentrySide = actual
			}
			survivalBoss2ReentryRouteStage = 2
			target = survivalBoss2ReentryTarget
			if survivalBoss2ReentryTelegraphAt.IsZero() {
				survivalBoss2ReentryTelegraphAt = now
				survivalWaveBannerText = "VOID SERPENT // INCOMING"
				survivalWaveBannerUntil = now.Add(700 * time.Millisecond)
			}
		}
	}

	d := boss2Norm(FPoint{target.X - p.X, target.Y - p.Y})
	mult := 1.48
	if survivalBoss2ReentryRouteStage < 2 {
		// Slightly faster outside routing keeps the extra corner path from adding dead time.
		mult = 1.62
	}
	return FPoint{d.X * baseSpeed * mult, d.Y * baseSpeed * mult}, true
}

func survivalBoss2DesiredVelocity(now time.Time, w, h int32) FPoint {
	r := arenaRect(w, h)
	c := FPoint{X: float64(r.Left+r.Right) / 2, Y: float64(r.Top+r.Bottom) / 2}
	p := survivalBoss2Head
	speed := float64(sx(315, w)) * survivalBoss2ExtraMovementScale
	turn := FPoint{}

	// A crosshair-targeted re-entry takes priority over ordinary phase steering.
	// The normal curvature limiter still applies later in the update loop, so this
	// cannot make the enlarged serpent snap or fold sharply back over itself.
	if v, ok := survivalBoss2ReentryVelocity(now, w, h, speed); ok {
		return v
	}
	if survivalBoss2ComboAwaitExit {
		// Once a combo is cleared, continue along the current heading until the head is
		// decisively off-screen. This creates a clean pass/retreat/re-entry rhythm without
		// introducing a sharp scripted turn.
		n := boss2Norm(survivalBoss2Vel)
		return FPoint{n.X * speed * 1.34, n.Y * speed * 1.34}
	}

	// Lunges are deliberately telegraphed: the target is captured before the attack,
	// then the serpent commits rather than unfairly homing through the dodge.
	if !survivalBoss2LungeStartAt.IsZero() && now.After(survivalBoss2LungeStartAt) && now.Before(survivalBoss2LungeEndAt) {
		d := boss2Norm(FPoint{survivalBoss2LungeTarget.X - p.X, survivalBoss2LungeTarget.Y - p.Y})
		return FPoint{d.X * speed * 1.90, d.Y * speed * 1.90}
	}

	switch survivalBoss2Stage {
	case survivalBoss2FadeOut:
		return survivalBoss2Vel
	case survivalBoss2FadeIn, survivalBoss2Intro:
		target := FPoint{c.X + float64(sx(250, w)), c.Y - float64(sy(115, h))}
		d := boss2Norm(FPoint{target.X - p.X, target.Y - p.Y})
		return FPoint{d.X * speed * .85, d.Y * speed * .85}
	case survivalBoss2Hunt, survivalBoss2Sweep, survivalBoss2Coil, survivalBoss2Frenzy, survivalBoss2Final:
		// v370: route is generated anew for every node phase. Combine two non-harmonic
		// waves and a slow secondary wobble; unlike the old fixed formulas this does not
		// settle into one of two memorisable paths across repeated attempts.
		t := now.Sub(survivalBoss2StageAt).Seconds()
		if t < 0 {
			t = 0
		}
		ax := float64(sx(survivalBoss2PathAmpX, w))
		ay := float64(sy(survivalBoss2PathAmpY, h))
		bx := float64(sx(survivalBoss2PathBiasX, w))
		by := float64(sy(survivalBoss2PathBiasY, h))
		xWave := math.Sin(t*survivalBoss2PathFreqX + survivalBoss2PathPhaseX)
		xWave += 0.24 * math.Sin(t*(survivalBoss2PathFreqY*1.71)+survivalBoss2PathPhaseY*0.63)
		yWave := math.Sin(t*survivalBoss2PathFreqY*survivalBoss2PathMirror + survivalBoss2PathPhaseY)
		yWave += 0.20 * math.Cos(t*(survivalBoss2PathFreqX*1.43)+survivalBoss2PathPhaseX*0.77)
		target := FPoint{X: c.X + bx + ax*xWave, Y: c.Y + by + ay*yWave}
		// Clamp steering targets to a generous inner arena so random routes remain smooth
		// and never repeatedly hammer a boundary. Re-entry remains separately targeted.
		marginX := float64(sx(90, w))
		marginY := float64(sy(72, h))
		target.X = boss2Clamp(target.X, float64(r.Left)+marginX, float64(r.Right)-marginX)
		target.Y = boss2Clamp(target.Y, float64(r.Top)+marginY, float64(r.Bottom)-marginY)
		if survivalBoss2Loop >= 1 && survivalBoss2Stage == survivalBoss2Hunt {
			// The second Hunt is a visibly more aggressive chase before the next core chain.
			speed *= 1.28
		}
		if survivalBoss2Stage == survivalBoss2Sweep {
			speed *= 1.12
		}
		if survivalBoss2Stage == survivalBoss2Coil {
			speed *= 1.06
		}
		if survivalBoss2Stage >= survivalBoss2Frenzy {
			speed *= 1.30
		}
		d := boss2Norm(FPoint{target.X - p.X, target.Y - p.Y})
		turn = FPoint{d.X * speed, d.Y * speed}
	default:
		turn = survivalBoss2Vel
	}
	// Soft boundary steering keeps the head readable and prevents long periods fully offscreen.
	padX := float64(sx(125, w))
	padY := float64(sy(85, h))
	if p.X < float64(r.Left)+padX {
		turn.X += speed * 1.4
	}
	if p.X > float64(r.Right)-padX {
		turn.X -= speed * 1.4
	}
	if p.Y < float64(r.Top)+padY {
		turn.Y += speed * 1.4
	}
	if p.Y > float64(r.Bottom)-padY {
		turn.Y -= speed * 1.4
	}
	// Once a pass has entered, apply extra inward steering near its entry edge. This
	// makes the hard same-side guard above a rare safety net rather than visible bounce
	// behaviour and naturally carries the serpent toward one of the other three exits.
	if survivalBoss2PassInside {
		switch survivalBoss2PassEntrySide {
		case survivalBoss2ReentryLeft:
			if p.X < float64(r.Left)+padX*1.45 {
				turn.X += speed * 1.25
			}
		case survivalBoss2ReentryRight:
			if p.X > float64(r.Right)-padX*1.45 {
				turn.X -= speed * 1.25
			}
		case survivalBoss2ReentryTop:
			if p.Y < float64(r.Top)+padY*1.45 {
				turn.Y += speed * 1.25
			}
		case survivalBoss2ReentryBottom:
			if p.Y > float64(r.Bottom)-padY*1.45 {
				turn.Y -= speed * 1.25
			}
		}
	}
	n := boss2Norm(turn)
	return FPoint{n.X * speed, n.Y * speed}
}

func survivalBoss2UpdateHistory(w int32) {
	p := survivalBoss2Head
	if len(survivalBoss2History) == 0 || math.Hypot(p.X-survivalBoss2History[len(survivalBoss2History)-1].X, p.Y-survivalBoss2History[len(survivalBoss2History)-1].Y) >= 1.5 {
		survivalBoss2History = append(survivalBoss2History, SurvivalBoss2HistoryPoint{p.X, p.Y})
		if len(survivalBoss2History) > survivalBoss2HistoryMax {
			survivalBoss2History = append([]SurvivalBoss2HistoryPoint(nil), survivalBoss2History[len(survivalBoss2History)-survivalBoss2HistoryMax:]...)
		}
	}
	spacing := survivalBoss2SegmentSpacing(w)
	idx := len(survivalBoss2History) - 1
	prev := FPoint{p.X, p.Y}
	distAcc := 0.0
	target := spacing
	seg := 0
	for idx >= 0 && seg < survivalBoss2SegmentCount {
		hp := survivalBoss2History[idx]
		cur := FPoint{hp.X, hp.Y}
		step := math.Hypot(prev.X-cur.X, prev.Y-cur.Y)
		distAcc += step
		for distAcc >= target && seg < survivalBoss2SegmentCount {
			// Interpolating within the history edge keeps spacing stable at high speed.
			over := distAcc - target
			f := 0.0
			if step > 1e-6 {
				f = over / step
			}
			q := FPoint{cur.X + (prev.X-cur.X)*f, cur.Y + (prev.Y-cur.Y)*f}
			ahead := prev
			survivalBoss2Segments[seg] = SurvivalBoss2Segment{P: q, Angle: math.Atan2(ahead.Y-q.Y, ahead.X-q.X), Valid: true}
			seg++
			target += spacing
		}
		prev = cur
		idx--
	}
	for ; seg < survivalBoss2SegmentCount; seg++ {
		survivalBoss2Segments[seg].Valid = false
	}
}

func survivalBoss2MaybeLunge(now time.Time) {
	if survivalBoss2ReentryActive || survivalBoss2ComboAwaitExit {
		return
	}
	// Early combo passes teach the new sequence cleanly. Frenzy/final may layer lunges
	// over the combo to preserve Boss 2 difficulty once the player understands it.
	if survivalBoss2ComboActive && survivalBoss2Stage < survivalBoss2Frenzy {
		return
	}
	if survivalBoss2Stage < survivalBoss2Hunt || survivalBoss2Stage > survivalBoss2Final {
		return
	}
	if !survivalBoss2NextLungeAt.IsZero() && now.Before(survivalBoss2NextLungeAt) {
		return
	}
	survivalBoss2LungeTelegraphAt = now
	survivalBoss2LungeStartAt = now.Add(600 * time.Millisecond)
	survivalBoss2LungeEndAt = survivalBoss2LungeStartAt.Add(720 * time.Millisecond)
	survivalBoss2LungeTarget = cursorPos
	cadence := 5400 * time.Millisecond
	if survivalBoss2Stage >= survivalBoss2Sweep {
		cadence = 4600 * time.Millisecond
	}
	if survivalBoss2Stage >= survivalBoss2Frenzy {
		cadence = 3500 * time.Millisecond
	}
	survivalBoss2NextLungeAt = now.Add(cadence)
	survivalWaveBannerText = "HEAD LUNGE // MOVE"
	survivalWaveBannerUntil = now.Add(700 * time.Millisecond)
}

func survivalBoss2NodeVisualPoint() (FPoint, bool) {
	if survivalBoss2NodeSegment == -2 {
		return survivalBoss2Head, true
	}
	if survivalBoss2NodeSegment < 0 || survivalBoss2NodeSegment >= len(survivalBoss2Segments) {
		return FPoint{}, false
	}
	s := survivalBoss2Segments[survivalBoss2NodeSegment]
	return s.P, s.Valid
}

func survivalBoss2ComboSize(combo int) int {
	_ = combo
	return 5
}

func survivalBoss2ComboStage(combo int) int {
	switch combo {
	case 0:
		return survivalBoss2Hunt
	case 1:
		return survivalBoss2Sweep
	case 2:
		return survivalBoss2Coil
	case 3:
		return survivalBoss2Frenzy
	default:
		return survivalBoss2Final
	}
}

func survivalBoss2HuntNodePoint(index int) (FPoint, bool) {
	if index < 0 || index >= len(survivalBoss2HuntNodes) {
		return FPoint{}, false
	}
	n := survivalBoss2HuntNodes[index]
	if n.Segment < 0 || n.Segment >= len(survivalBoss2Segments) {
		return FPoint{}, false
	}
	s := survivalBoss2Segments[n.Segment]
	return s.P, s.Valid
}

func survivalBoss2HuntNodeSafe(n SurvivalBoss2HuntNode, now time.Time) bool {
	if n.Segment < 0 {
		return false
	}
	return n.Alive || (!n.RestoreUntil.IsZero() && now.Before(n.RestoreUntil))
}

func survivalBoss2HuntLiveCount() int {
	left := 0
	for i := 0; i < survivalBoss2ComboTotal && i < len(survivalBoss2HuntNodes); i++ {
		if survivalBoss2HuntNodes[i].Alive {
			left++
		}
	}
	return left
}

// v415: Hunt nodes are selected across the shortened 35-piece articulated body.
// The first two neck pieces are deliberately excluded as node hosts: they sit directly
// beneath/behind the head sprite and can be visually occluded, which made a legitimate
// Hunt node impossible to click. The final two pieces are also excluded as hosts so every
// node can always open the full three-piece safe pocket (host-1, host, host+1).
//
// The remaining host range is divided into five broad longitudinal bands and one random
// host is chosen from each band. This preserves the full-body Hunt feel while preventing
// clustering. Hosts stay at least four indices apart so their three-piece safe pockets
// never overlap.
func survivalBoss2SelectHuntSegments(w, h int32, total int) ([]int, bool) {
	_ = w
	_ = h
	if total <= 0 || total > survivalBoss2SegmentCount {
		return nil, false
	}

	const firstHost = 2
	lastHostExclusive := survivalBoss2SegmentCount - 2
	usable := lastHostExclusive - firstHost
	if usable < total {
		return nil, false
	}

	for attempt := 0; attempt < 256; attempt++ {
		chosen := make([]int, 0, total)
		phase := rand.Intn(maxIntBoss2(1, usable/total))
		for band := 0; band < total; band++ {
			lo := firstHost + band*usable/total
			hi := firstHost + (band+1)*usable/total
			if band > 0 {
				lo = maxIntBoss2(firstHost, lo-1)
			}
			if band < total-1 {
				hi = minIntBoss2(lastHostExclusive, hi+1)
			}
			if hi <= lo {
				continue
			}

			span := hi - lo
			perm := rand.Perm(span)
			pick := -1
			for _, off := range perm {
				candidate := lo + ((off + phase) % span)
				legal := true
				for _, prev := range chosen {
					if absIntBoss2(candidate-prev) < 4 {
						legal = false
						break
					}
				}
				if legal {
					pick = candidate
					break
				}
			}
			if pick >= 0 {
				chosen = append(chosen, pick)
			}
		}

		if len(chosen) != total {
			continue
		}
		sort.Ints(chosen)

		// Keep the first and last Hunt nodes far apart so the five targets still read as
		// attacks on the whole serpent despite the body being shorter than before.
		if chosen[len(chosen)-1]-chosen[0] < 21 {
			continue
		}
		return chosen, true
	}

	return nil, false
}

func survivalBoss2StartCombo(now time.Time, w, h int32) bool {
	if survivalBoss2ComboNumber >= 5 || survivalBoss2ComboActive || survivalBoss2ComboAwaitExit {
		return false
	}
	total := survivalBoss2ComboSize(survivalBoss2ComboNumber)
	chosen, ok := survivalBoss2SelectHuntSegments(w, h, total)
	if !ok {
		return false
	}

	// All five cores spawn together. There is no sequential retargeting, expiry, hidden
	// rectangle or moving anchor. Each core stays on its original segment until clicked.
	survivalBoss2HuntNodes = [5]SurvivalBoss2HuntNode{}
	lastRed := false
	for j := 0; j < total; j++ {
		red := rand.Intn(2) == 0
		if j > 0 && red == lastRed && rand.Intn(3) != 0 {
			red = !red
		}
		lastRed = red
		survivalBoss2HuntNodes[j] = SurvivalBoss2HuntNode{
			Segment: chosen[j],
			Red:     red,
			Alive:   true,
		}
	}

	survivalBoss2ComboTotal = total
	survivalBoss2ComboIndex = 0
	survivalBoss2ComboActive = true
	survivalBoss2ComboAwaitEntry = false
	survivalBoss2ComboEntryAt = now
	// The old single-core state is explicitly disabled for Hunt. It must never select,
	// slide or replace any of the five permanent body anchors above.
	survivalBoss2NodeSegment = -1
	survivalBoss2NodeTelegraphAt = time.Time{}
	survivalBoss2NodeActiveAt = time.Time{}
	survivalBoss2NodeExpiresAt = time.Time{}
	status = "VOID HUNT // 5 BODY CORES ACTIVE"
	survivalWaveBannerText = "HUNT // DESTROY ALL 5 BODY CORES"
	survivalWaveBannerUntil = now.Add(1050 * time.Millisecond)
	return true
}

func survivalBoss2ChooseNode(now time.Time, w, h int32) {
	_ = survivalBoss2StartCombo(now, w, h)
}

func survivalBoss2UpdateNode(h uintptr, now time.Time, w, hgt int32) bool {
	inside := survivalBoss2HeadInsideArena(w, hgt)
	if survivalBoss2ComboActive {
		// v411: Hunt cores never expire, advance, or despawn on their own. The only state
		// transition available to an active core is a correct player click.
		return true
	}
	if survivalBoss2ComboAwaitExit {
		return true
	}
	if survivalBoss2ComboAwaitEntry {
		if inside {
			if survivalBoss2ComboEntryAt.IsZero() {
				survivalBoss2ComboEntryAt = now
			}
			// Spawn on the first frame where five legal body anchors are genuinely visible.
			// There is no artificial Hunt delay once the serpent presents enough body.
			if survivalBoss2StartCombo(now, w, hgt) {
				return true
			}
		} else {
			survivalBoss2ComboEntryAt = time.Time{}
		}
	}
	return true
}

func initSurvivalBoss2Audio() {
	// v389: the finale heartbeat is a long looping music-bed, so keep it on the MCI
	// music bus rather than the one-shot PCM SFX bus. It obeys Music Volume.
	heartbeatPath := filepath.Join(externalAsset("audio"), "heartbeat.mp3")
	if heartbeatPath != "" {
		survivalBoss2HeartbeatReady = reopenMusicAlias("survival_boss2_heartbeat", "", heartbeatPath)
	}
	// v306: Boss 2 effects are loaded by the central PCM SFX registry.
	survivalBoss2SerpentStartAudio = sfxLoaded("boss2_serpent_start")
	for i := 0; i < 3; i++ {
		survivalBoss2SerpentAttackAudio[i] = sfxLoaded(fmt.Sprintf("boss2_serpent_attack_%d", i+1))
	}
	survivalBoss2MeteorRumbleAudio = sfxLoaded("boss2_meteor_rumble")
	survivalBoss2MeteorSmashAudio = sfxLoaded("boss2_meteor_smash")
	survivalBoss2EnergyBallAudio = sfxLoaded("boss2_energy_ball")
}

func survivalBoss2StartHeartbeat() {
	if !survivalBoss2HeartbeatReady {
		heartbeatPath := filepath.Join(externalAsset("audio"), "heartbeat.mp3")
		survivalBoss2HeartbeatReady = reopenMusicAlias("survival_boss2_heartbeat", "", heartbeatPath)
	}
	if !survivalBoss2HeartbeatReady {
		return
	}
	mci("stop survival_boss2_heartbeat")
	mci("seek survival_boss2_heartbeat to start")
	mci("setaudio survival_boss2_heartbeat volume to 0")
	if mci("play survival_boss2_heartbeat repeat") {
		survivalBoss2HeartbeatActive = true
		// Fade up with the first return from blackout, creating the unconscious/panic bed
		// without an abrupt audio cut-in.
		survivalBoss1FadeAlias("survival_boss2_heartbeat", 0, survivalBoss1TargetMusicVolume(), 900*time.Millisecond, false)
	}
}

func survivalBoss2StopHeartbeat() {
	if !survivalBoss2HeartbeatActive {
		return
	}
	// The heartbeat lasts until the FINAL roar. Stop it right on that cue so the roar
	// and the mouth-core threat suddenly own the mix.
	mci("stop survival_boss2_heartbeat")
	mci("seek survival_boss2_heartbeat to start")
	survivalBoss2HeartbeatActive = false
}

func playSurvivalBoss2FinaleRoar(head int) {
	if !audioReady {
		return
	}
	alias := "boss2_final_roar_3"
	if head == 1 {
		alias = "boss2_final_roar_1"
	} else if head == 2 {
		alias = "boss2_final_roar_2"
	}
	if sfxLoaded(alias) {
		playOneShotAsync(alias)
		survivalBoss2MouthOpenUntil = survivalBossFightNow(time.Now(), survivalBoss2StartedAt).Add(1500 * time.Millisecond)
	}
}

func playSurvivalBoss2HuntHitSound(hit int) {
	if hit < 1 || hit > 5 {
		playHitSound()
		return
	}
	alias := fmt.Sprintf("boss2_hunt_hit_%d", hit)
	if audioReady && sfxLoaded(alias) {
		playOneShotAsync(alias)
		return
	}
	// Safe fallback for incomplete asset installs: never make a valid Hunt click silent.
	playHitSound()
}

func playSurvivalBoss2SerpentStart() {
	if audioReady && survivalBoss2SerpentStartAudio {
		playOneShotAsync("boss2_serpent_start")
		survivalBoss2MouthOpenUntil = survivalBossFightNow(time.Now(), survivalBoss2StartedAt).Add(1500 * time.Millisecond)
	}
}
func playSurvivalBoss2MeteorRumble() {
	if audioReady && survivalBoss2MeteorRumbleAudio {
		playOneShotAsync("boss2_meteor_rumble")
	}
}
func playSurvivalBoss2MeteorSmash() {
	if audioReady && survivalBoss2MeteorSmashAudio {
		playOneShotAsync("boss2_meteor_smash")
	} else {
		playSurvivalExplodeSound()
	}
}
func survivalBoss2MaybeVocalise(now time.Time) {
	// Random serpent vocals are reserved for the free-moving node phases. Special
	// choreography (meteor, rush and beam phases) owns its own timing/audio so no
	// unscheduled roar can overlap a telegraph, beam charge or mouth-core window.
	if state != StatePlaying || survivalBoss2Stage < survivalBoss2Hunt || survivalBoss2Stage > survivalBoss2Final {
		return
	}
	if survivalBoss2NextVocalAt.IsZero() {
		survivalBoss2NextVocalAt = now.Add(time.Duration(5500+rand.Intn(4500)) * time.Millisecond)
		return
	}
	if now.Before(survivalBoss2NextVocalAt) {
		return
	}
	available := make([]int, 0, 3)
	for i, ok := range survivalBoss2SerpentAttackAudio {
		if ok {
			available = append(available, i)
		}
	}
	if len(available) > 0 && audioReady {
		i := available[rand.Intn(len(available))]
		playOneShotAsync(fmt.Sprintf("boss2_serpent_attack_%d", i+1))
		// Keep the mouth animation tied to the audible attack cue, but close it promptly
		// so the serpent does not remain visually frozen open after the vocal hit.
		survivalBoss2MouthOpenUntil = now.Add(1500 * time.Millisecond)
	}
	survivalBoss2NextVocalAt = now.Add(time.Duration(6500+rand.Intn(5500)) * time.Millisecond)
}

func survivalBoss2SpawnMeteorWave(now time.Time, w, h int32) {
	r := arenaRect(w, h)
	left := float64(r.Left) + float64(sx(42, w))
	right := float64(r.Right) - float64(sx(42, w))
	// Build a true falling wall: neighbouring lethal rock hitboxes overlap slightly,
	// so the ONLY route through the row is the cracked meteor after it is destroyed.
	targetStep := float64(sx(96, w))
	count := int(math.Ceil((right-left)/targetStep)) + 1
	if count < 9 {
		count = 9
	}
	step := (right - left) / float64(count-1)
	// The break boulder can occupy any non-edge lane. Normal boulder art is shuffled
	// independently every wave so the falling wall never looks tiled or repeated.
	breakable := 1 + rand.Intn(count-2)
	variants := rand.Perm(4)
	y := float64(r.Top) - float64(sy(75, h))
	survivalBoss2Meteors = make([]SurvivalBoss2Meteor, count)
	normalN := 0
	for i := 0; i < count; i++ {
		isBreak := i == breakable
		variant := 0
		if !isBreak {
			if normalN > 0 && normalN%4 == 0 {
				variants = rand.Perm(4)
			}
			variant = variants[normalN%4]
			normalN++
		}
		survivalBoss2Meteors[i] = SurvivalBoss2Meteor{P: FPoint{X: left + float64(i)*step, Y: y}, Breakable: isBreak, Variant: variant}
	}
	survivalBoss2MeteorWaveAt = now
	survivalBoss2MeteorLastUpdate = now
	survivalBoss2MeteorRumbleUntil = now.Add(900 * time.Millisecond)
	status = fmt.Sprintf("METEOR BREACH %d/5 // BREAK THE PURPLE METEOR", survivalBoss2MeteorRound+1)
	survivalWaveBannerText = fmt.Sprintf("METEOR BREACH %d / 5", survivalBoss2MeteorRound+1)
	survivalWaveBannerUntil = now.Add(800 * time.Millisecond)
	playSurvivalBoss2MeteorRumble()
}

func survivalBoss2MeteorRockRadius(w int32) float64  { return float64(sx(52, w)) }
func survivalBoss2MeteorClickRadius(w int32) float64 { return float64(sx(50, w)) }

func survivalBoss2MeteorShake(now time.Time, w, h int32) (float64, float64) {
	if survivalBoss2MeteorRumbleUntil.IsZero() || !now.Before(survivalBoss2MeteorRumbleUntil) {
		return 0, 0
	}
	left := survivalBoss2MeteorRumbleUntil.Sub(now).Seconds() / 0.9
	if left < 0 {
		left = 0
	}
	if left > 1 {
		left = 1
	}
	t := float64(now.UnixNano()) / 1e9
	return math.Sin(t*73.0) * float64(sx(5, w)) * left, math.Cos(t*61.0) * float64(sy(4, h)) * left
}

func survivalBoss2DriftOutDuringMeteor(now time.Time, w int32) {
	if survivalBoss2MeteorSerpentTick.IsZero() {
		survivalBoss2MeteorSerpentTick = now
		return
	}
	dt := now.Sub(survivalBoss2MeteorSerpentTick).Seconds()
	if dt < 0 {
		dt = 0
	}
	if dt > .05 {
		dt = .05
	}
	survivalBoss2MeteorSerpentTick = now
	n := boss2Norm(survivalBoss2Vel)
	spd := float64(sx(425, w)) * survivalBoss2ExtraMovementScale
	survivalBoss2Vel = FPoint{n.X * spd, n.Y * spd}
	survivalBoss2Head.X += survivalBoss2Vel.X * dt
	survivalBoss2Head.Y += survivalBoss2Vel.Y * dt
	survivalBoss2UpdateHistory(w)
}

func survivalBoss2RespawnAfterBeamExit(now time.Time, w, h int32, exitSide int) {
	// v409: the energy finale leaves through the right edge. Re-enter as a fresh,
	// fully-initialised attack from a DIFFERENT edge. The previous implementation set
	// ReentryActive without initialising its route stage/corner/staging state, which
	// could leave the Hunt steering against stale re-entry data and strand the serpent
	// off-screen after its next exit.
	r := arenaRect(w, h)
	candidates := []int{survivalBoss2ReentryLeft, survivalBoss2ReentryRight, survivalBoss2ReentryTop, survivalBoss2ReentryBottom}
	filtered := make([]int, 0, 3)
	for _, side := range candidates {
		if side != exitSide {
			filtered = append(filtered, side)
		}
	}
	entrySide := filtered[rand.Intn(len(filtered))]
	padX := float64(sx(150, w))
	padY := float64(sy(125, h))
	insetX := float64(sx(115, w))
	insetY := float64(sy(100, h))
	spdX := float64(sx(355, w)) * survivalBoss2ExtraMovementScale
	spdY := float64(sy(355, h)) * survivalBoss2ExtraMovementScale

	var inward FPoint
	switch entrySide {
	case survivalBoss2ReentryLeft:
		y := float64(r.Top) + insetY + rand.Float64()*(float64(r.Bottom-r.Top)-2*insetY)
		survivalBoss2Head = FPoint{X: float64(r.Left) - padX, Y: y}
		inward = FPoint{X: spdX, Y: 0}
	case survivalBoss2ReentryRight:
		y := float64(r.Top) + insetY + rand.Float64()*(float64(r.Bottom-r.Top)-2*insetY)
		survivalBoss2Head = FPoint{X: float64(r.Right) + padX, Y: y}
		inward = FPoint{X: -spdX, Y: 0}
	case survivalBoss2ReentryTop:
		x := float64(r.Left) + insetX + rand.Float64()*(float64(r.Right-r.Left)-2*insetX)
		survivalBoss2Head = FPoint{X: x, Y: float64(r.Top) - padY}
		inward = FPoint{X: 0, Y: spdY}
	default: // bottom
		x := float64(r.Left) + insetX + rand.Float64()*(float64(r.Right-r.Left)-2*insetX)
		survivalBoss2Head = FPoint{X: x, Y: float64(r.Bottom) + padY}
		inward = FPoint{X: 0, Y: -spdY}
	}
	survivalBoss2Vel = inward

	// Seed the body directly behind the incoming head so the visual chain is valid on
	// frame one regardless of which edge was selected.
	survivalBoss2History = nil
	spacing := survivalBoss2SegmentSpacing(w)
	trailLen := float64(survivalBoss2SegmentCount+3) * spacing
	n := boss2Norm(inward)
	for d := trailLen; d >= 0; d -= 1.5 {
		survivalBoss2History = append(survivalBoss2History, SurvivalBoss2HistoryPoint{
			X: survivalBoss2Head.X - n.X*d,
			Y: survivalBoss2Head.Y - n.Y*d,
		})
	}
	survivalBoss2UpdateHistory(w)

	// This is already the entry leg, so initialise it at route stage 2 instead of
	// asking the generic outside-corner router to infer a route from stale state.
	survivalBoss2WasInside = false
	survivalBoss2ReentryActive = true
	survivalBoss2ReentryExitSide = exitSide
	survivalBoss2ReentrySide = entrySide
	if actual := survivalBoss2PhysicalOutsideSide(w, h); actual != survivalBoss2ReentryNone {
		survivalBoss2ReentrySide = actual
	}
	survivalBoss2PassEntrySide = survivalBoss2ReentrySide
	survivalBoss2PassInside = false
	survivalBoss2ReentryRouteStage = 2
	survivalBoss2ReentryCorner = FPoint{}
	survivalBoss2ReentryStaging = FPoint{}
	survivalBoss2ReentryTarget = survivalBoss2ClampTargetToArena(cursorPos, w, h)
	survivalBoss2ReentryTelegraphAt = now
	survivalBoss2ReentryCommitAt = now
	survivalBoss2ReentryEndAt = now.Add(9000 * time.Millisecond)
}

func updateSurvivalBoss2Meteor(h uintptr, now time.Time, w, hgt int32) bool {
	// The serpent remains a real moving creature during the intermission and simply
	// continues its retreat off-screen; it is never despawned for the meteor phase.
	survivalBoss2DriftOutDuringMeteor(now, w)
	if survivalBoss2Stage == survivalBoss2MeteorWarning {
		if now.Sub(survivalBoss2StageAt) >= 900*time.Millisecond {
			survivalBoss2SetStage(survivalBoss2MeteorRun, now)
			survivalBoss2MeteorRound = 0
			survivalBoss2SpawnMeteorWave(now, w, hgt)
		}
		invalidateSurvivalArena(h)
		return true
	}
	if survivalBoss2Stage != survivalBoss2MeteorRun {
		return false
	}
	dt := now.Sub(survivalBoss2MeteorLastUpdate).Seconds()
	if dt <= 0 {
		dt = .016
	}
	if dt > .05 {
		dt = .05
	}
	survivalBoss2MeteorLastUpdate = now
	speed := float64(sy(385, hgt))
	if survivalBoss2Loop >= 1 {
		// Final-loop meteor walls are deliberately faster again than the first breach.
		speed *= 1.20
	}
	for i := range survivalBoss2Meteors {
		survivalBoss2Meteors[i].P.Y += speed * dt
	}
	// Normal falling rocks are lethal. The cracked/damaged purple meteor is NEVER
	// a collision hazard: it has a click hitbox only, so the player can safely place
	// the cursor on it and destroy it without dying from contact first.
	radius := survivalBoss2MeteorRockRadius(w)
	shakeX, shakeY := survivalBoss2MeteorShake(now, w, hgt)
	for _, m := range survivalBoss2Meteors {
		if m.Broken || m.Breakable {
			continue
		}
		if math.Hypot(cursorPos.X-(m.P.X+shakeX), cursorPos.Y-(m.P.Y+shakeY)) <= radius {
			survivalFail(h, "Hit a falling meteorite")
			return true
		}
	}
	r := arenaRect(w, hgt)
	allPast := true
	for _, m := range survivalBoss2Meteors {
		if m.P.Y <= float64(r.Bottom)+float64(sy(105, hgt)) {
			allPast = false
			break
		}
	}
	if allPast {
		survivalBoss2MeteorRound++
		if survivalBoss2MeteorRound >= 5 {
			survivalBoss2MeteorDone = true
			survivalBoss2Meteors = nil
			survivalBoss2StartDodgePhase(now, w, hgt)
			return true
		} else {
			survivalBoss2SpawnMeteorWave(now.Add(60*time.Millisecond), w, hgt)
			// Keep the next row briefly above the arena to create a readable beat.
			for i := range survivalBoss2Meteors {
				survivalBoss2Meteors[i].P.Y -= float64(sy(25, hgt))
			}
		}
	}
	invalidateSurvivalArena(h)
	invalidateSurvivalHUD(h)
	return true
}

func survivalBoss2HandleMeteorClick(h uintptr, p FPoint) bool {
	if survivalBoss2Stage != survivalBoss2MeteorRun {
		return false
	}
	w, hgt := getClient(h)
	now := survivalBossFightNow(time.Now(), survivalBoss2StartedAt)
	shakeX, shakeY := survivalBoss2MeteorShake(now, w, hgt)
	for i := range survivalBoss2Meteors {
		m := &survivalBoss2Meteors[i]
		if m.Broken || !m.Breakable {
			continue
		}
		if math.Hypot(p.X-(m.P.X+shakeX), p.Y-(m.P.Y+shakeY)) <= survivalBoss2MeteorClickRadius(w) {
			m.Broken = true
			score += 500
			playSurvivalBoss2MeteorSmash()
			status = "PURPLE METEOR SHATTERED // MOVE THROUGH"
			survivalWaveBannerText = "BREACH OPEN"
			survivalWaveBannerUntil = now.Add(550 * time.Millisecond)
			invalidateSurvivalArena(h)
			return true
		}
	}
	survivalFail(h, "Wrong meteorite")
	return true
}

func updateSurvivalBoss2(h uintptr, now time.Time, w, hgt int32) bool {
	if !survivalBoss2Active() {
		return false
	}
	now = survivalBossFightNow(now, survivalBoss2StartedAt)
	// Boss 1-style full-black audio/arena handoff on both sides of the encounter.
	elapsed := now.Sub(survivalBoss2StageAt)
	switch survivalBoss2Stage {
	case survivalBoss2FadeOut:
		if elapsed >= 1000*time.Millisecond {
			mci("stop survival_section2")
			mci("seek survival_section2 to start")
			survivalBoss1StartMusic()
			survivalBoss2SetStage(survivalBoss2FadeIn, now)
		}
		invalidateSurvivalArena(h)
		return true
	case survivalBoss2FadeIn:
		if elapsed >= 900*time.Millisecond {
			survivalBoss2SetStage(survivalBoss2Intro, now)
			playSurvivalBoss2SerpentStart()
			survivalWaveBannerText = "WARNING // VOID SERPENT DETECTED"
			survivalWaveBannerUntil = now.Add(1700 * time.Millisecond)
			status = "AVOID THE BODY // HIT THE CORES"
		}
	case survivalBoss2Dying:
		if elapsed >= 1650*time.Millisecond {
			survivalBoss2SetStage(survivalBoss2FadeToNext, now)
			survivalBoss1FadeOutMusic()
		}
	case survivalBoss2FadeToNext:
		if elapsed >= 1000*time.Millisecond {
			finishSurvivalBoss2Clear(h, now)
		}
		invalidateSurvivalArena(h)
		return true
	case survivalBoss2ReturnFadeIn:
		if elapsed >= 1000*time.Millisecond {
			survivalBoss2Stage = survivalBoss2None
			survivalBoss2StageAt = time.Time{}
			beginSurvivalWave()
			survivalLastTick = now
			setCapture.Call(h)
			status = "SECTOR 3 // WAVE 21"
			survivalWaveBannerText = "SECTOR 3 // WAVE 21"
			survivalWaveBannerUntil = now.Add(1800 * time.Millisecond)
		}
		invalidateSurvivalArena(h)
		return true
	}
	if survivalBoss2Stage == survivalBoss2MeteorWarning || survivalBoss2Stage == survivalBoss2MeteorRun {
		return updateSurvivalBoss2Meteor(h, now, w, hgt)
	}
	if survivalBoss2Stage == survivalBoss2Dodge {
		return updateSurvivalBoss2Dodge(h, now, w, hgt)
	}
	if survivalBoss2Stage == survivalBoss2BeamSetup || survivalBoss2Stage == survivalBoss2BeamCharge || survivalBoss2Stage == survivalBoss2BeamFire || survivalBoss2Stage == survivalBoss2BeamNode || survivalBoss2Stage == survivalBoss2BeamRecover {
		return updateSurvivalBoss2Beam(h, now, w, hgt)
	}
	if survivalBoss2Stage == survivalBoss2BeamExit || survivalBoss2Stage == survivalBoss2FinalePreBlack ||
		survivalBoss2Stage == survivalBoss2FinaleHead1 || survivalBoss2Stage == survivalBoss2FinaleHead2 ||
		survivalBoss2Stage == survivalBoss2FinaleHead3 || survivalBoss2Stage == survivalBoss2FinaleNode ||
		survivalBoss2Stage == survivalBoss2FinaleFlash {
		return updateSurvivalBoss2Finale(h, now, w, hgt)
	}
	dt := now.Sub(survivalBoss2LastUpdate).Seconds()
	if dt <= 0 {
		dt = .016
	}
	if dt > .05 {
		dt = .05
	}
	survivalBoss2LastUpdate = now
	// Detect exits before steering, so the next return trip becomes a deliberate
	// crosshair-targeted attack instead of a passive boundary correction.
	survivalBoss2MaybeArmReentry(now, w, hgt)
	desired := survivalBoss2DesiredVelocity(now, w, hgt)
	desired, selfRisk := survivalBoss2ApplySelfAvoidance(desired, w)
	// Curvature-limited steering: the head cannot snap back across its own body.
	// Instead it arcs through every change of direction with a large minimum turn radius.
	curSpeed := math.Hypot(survivalBoss2Vel.X, survivalBoss2Vel.Y)
	desiredSpeed := math.Hypot(desired.X, desired.Y)
	if curSpeed < 1 {
		curSpeed = desiredSpeed
	}
	curA := math.Atan2(survivalBoss2Vel.Y, survivalBoss2Vel.X)
	desA := math.Atan2(desired.Y, desired.X)
	deltaA := math.Atan2(math.Sin(desA-curA), math.Cos(desA-curA))
	turnRate := 0.78 // rad/s: broad, readable arcs in the normal phases
	if survivalBoss2Stage == survivalBoss2Coil {
		turnRate = 0.88
	}
	if survivalBoss2Stage >= survivalBoss2Frenzy {
		turnRate = 0.96
	}
	if survivalBoss2ReentryActive {
		// Assertive enough to deliberately return towards the player, but still a broad
		// arc: at re-entry speed this remains hundreds of pixels of turning radius.
		turnRate = 1.10
	}
	if !survivalBoss2LungeStartAt.IsZero() && now.After(survivalBoss2LungeStartAt) && now.Before(survivalBoss2LungeEndAt) {
		turnRate = 1.10
	}
	if selfRisk > 0 {
		// Start reacting early, then permit a stronger but still continuous turn only as
		// the old body approaches the hard no-cross clearance.
		avoidTurnRate := 1.15 + 1.35*selfRisk
		if avoidTurnRate > turnRate {
			turnRate = avoidTurnRate
		}
	}
	maxTurn := turnRate * dt
	if deltaA > maxTurn {
		deltaA = maxTurn
	}
	if deltaA < -maxTurn {
		deltaA = -maxTurn
	}
	newA := curA + deltaA
	// Speed changes are also eased so a turn never appears as a sudden whip.
	spdBlend := boss2Clamp(dt*2.1, 0, 1)
	newSpeed := curSpeed + (desiredSpeed-curSpeed)*spdBlend
	survivalBoss2Vel = FPoint{X: math.Cos(newA) * newSpeed, Y: math.Sin(newA) * newSpeed}
	survivalBoss2GuardSelfIntersection(dt, w)
	prevHead := survivalBoss2Head
	survivalBoss2Head.X += survivalBoss2Vel.X * dt
	survivalBoss2Head.Y += survivalBoss2Vel.Y * dt
	survivalBoss2KeepReentryRouteOutside(w, hgt)
	survivalBoss2PreventSameSideExit(prevHead, w, hgt)
	survivalBoss2UpdateHistory(w)
	// Catch the exact frame the head crosses out of the arena as well.
	survivalBoss2MaybeArmReentry(now, w, hgt)
	if survivalBoss2StartMeteorWhenClear(now, w, hgt) {
		invalidateSurvivalArena(h)
		return true
	}

	if survivalBoss2Stage == survivalBoss2Intro && now.Sub(survivalBoss2StageAt) > 2400*time.Millisecond {
		survivalBoss2SetStage(survivalBoss2Hunt, now)
		survivalBoss2ComboAwaitEntry = true
		survivalBoss2ComboEntryAt = time.Time{}
	}
	if survivalBoss2CombatActive() {
		survivalBoss2MaybeVocalise(now)
		survivalBoss2MaybeLunge(now)
		if !survivalBoss2UpdateNode(h, now, w, hgt) {
			return true
		}
		if survivalBoss2CursorCollides(cursorPos, w, hgt) {
			survivalFail(h, "Cursor touched the Void Serpent")
			return true
		}
	}
	invalidateSurvivalArena(h)
	invalidateSurvivalHUD(h)
	return true
}

func absIntBoss2(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func survivalBoss2NodeSafetyActive(now time.Time) bool {
	return survivalBoss2NodeSegment != -1 && !survivalBoss2NodeTelegraphAt.IsZero() &&
		!now.Before(survivalBoss2NodeTelegraphAt) && now.Before(survivalBoss2NodeExpiresAt)
}

func survivalBoss2SegmentCollisionDisabled(i int, now time.Time) bool {
	// v412 Hunt contract: a node opens a three-piece interaction pocket. If a node is on
	// segment 16, segments 15, 16 and 17 all flash purple and have collision disabled.
	// The complete three-piece pocket stays safe for two seconds after that node is hit,
	// giving the player enough room to enter, click and escape before collision returns.
	for j := 0; j < survivalBoss2ComboTotal && j < len(survivalBoss2HuntNodes); j++ {
		n := survivalBoss2HuntNodes[j]
		if survivalBoss2HuntNodeSafe(n, now) && absIntBoss2(i-n.Segment) <= 1 {
			return true
		}
	}
	// Preserve any non-Hunt legacy safety pocket used by special mouth interactions.
	if survivalBoss2NodeSafetyActive(now) && survivalBoss2NodeSegment >= 0 && i == survivalBoss2NodeSegment {
		return true
	}
	if !survivalBoss2SafeExitUntil.IsZero() && now.Before(survivalBoss2SafeExitUntil) &&
		survivalBoss2SafeExitSegment >= 0 && i == survivalBoss2SafeExitSegment {
		return true
	}
	return false
}

func survivalBoss2HeadCollisionDisabled(now time.Time) bool {
	if survivalBoss2Stage == survivalBoss2BeamNode {
		return true
	}
	if survivalBoss2NodeSafetyActive(now) && survivalBoss2NodeSegment == -2 {
		return true
	}
	return false
}

func survivalBoss2PurplePulseBright(now time.Time) bool {
	// Two purple intensities alternate, so the disabled host section is always visibly
	// purple until its collision actually returns.
	return (now.UnixMilli()/125)%2 == 0
}

func survivalBoss2CursorCollides(p FPoint, w, h int32) bool {
	if !survivalBoss2CombatActive() {
		return false
	}
	now := survivalBossFightNow(time.Now(), survivalBoss2StartedAt)
	// Beam choreography uses a purpose-built fixed firing pose. Never test stale
	// free-moving segment history here, otherwise old dodge/body points can create
	// invisible phantom collisions during the beam or mouth-core interaction.
	if survivalBoss2Stage == survivalBoss2BeamSetup || survivalBoss2Stage == survivalBoss2BeamCharge ||
		survivalBoss2Stage == survivalBoss2BeamFire || survivalBoss2Stage == survivalBoss2BeamNode ||
		survivalBoss2Stage == survivalBoss2BeamRecover {
		return survivalBoss2BeamSpecialCollision(p, w, h, now)
	}
	if math.Hypot(p.X-survivalBoss2Head.X, p.Y-survivalBoss2Head.Y) <= survivalBoss2HeadRadius(w) && !survivalBoss2HeadCollisionDisabled(now) {
		return true
	}
	// Collision is evaluated piece-by-piece. Hunt safety is intentionally limited to the
	// three-piece pocket around each anchored node; all other overlapping body pieces stay lethal.
	br := survivalBoss2BodyRadius(w)
	for i, seg := range survivalBoss2Segments {
		if !seg.Valid || survivalBoss2SegmentCollisionDisabled(i, now) {
			continue
		}
		if math.Hypot(p.X-seg.P.X, p.Y-seg.P.Y) <= br {
			return true
		}
	}
	return false
}

func survivalBoss2BeginBeamExit(now time.Time, w, h int32) {
	// Preserve the existing vertical body stream exactly where it is. The body and tail
	// keep descending, while the head retreats back through the RIGHT edge it entered from.
	// This avoids dragging the head unnaturally down with the body.
	survivalBoss2Vel = FPoint{X: float64(sx(300, w)) * survivalBoss2ExtraMovementScale, Y: 0}
	survivalBoss2BeamExitStartedAt = now
	survivalBoss2LastUpdate = now
	survivalBoss2SetStage(survivalBoss2BeamExit, now)
	status = "VOID SERPENT // DESCENDING"
}

func survivalBoss2BeamExitBodyPoint(index int, w, h int32, now time.Time) FPoint {
	// Snapshot each looping beam-body segment at the instant the exit begins, then move
	// that snapshot downward without wrapping. This makes the existing top-to-bottom
	// scroll visibly continue instead of spawning a different retreat animation.
	baseAt := survivalBoss2BeamExitStartedAt
	if baseAt.IsZero() {
		baseAt = survivalBoss2StageAt
	}
	q := survivalBoss2BeamBodyPoint(index, w, h, baseAt)
	d := now.Sub(baseAt).Seconds() * float64(sy(230, h))
	q.Y += d
	return q
}

func survivalBoss2BeamExitTailPoint(w, h int32, now time.Time) FPoint {
	// The tail trails the highest body piece so it is naturally the final visible part
	// of the serpent to leave the bottom edge.
	q := survivalBoss2BeamExitBodyPoint(0, w, h, now)
	for i := 1; i < 12; i++ {
		p := survivalBoss2BeamExitBodyPoint(i, w, h, now)
		if p.Y < q.Y {
			q = p
		}
	}
	q.Y -= float64(sy(48, h))
	return q
}

func updateSurvivalBoss2Finale(h uintptr, now time.Time, w, hgt int32) bool {
	r := arenaRect(w, hgt)
	elapsed := now.Sub(survivalBoss2StageAt)
	switch survivalBoss2Stage {
	case survivalBoss2BeamExit:
		dt := now.Sub(survivalBoss2LastUpdate).Seconds()
		if dt <= 0 {
			dt = .016
		}
		if dt > .04 {
			dt = .04
		}
		survivalBoss2LastUpdate = now
		survivalBoss2Head.X += float64(sx(300, w)) * survivalBoss2ExtraMovementScale * dt
		survivalBoss2Vel = FPoint{X: float64(sx(300, w)) * survivalBoss2ExtraMovementScale, Y: 0}
		tail := survivalBoss2BeamExitTailPoint(w, hgt, now)
		tailOff := tail.Y > float64(r.Bottom+sy(110, hgt))
		if tailOff {
			if survivalBoss2Loop == 0 {
				survivalBoss2Loop = 1
				survivalBoss2LoopStartHits = survivalBoss2Hits
				survivalBoss2MeteorDone = false
				survivalBoss2MeteorPending = false
				survivalBoss2MeteorRound = 0
				survivalBoss2ComboNumber = 0
				survivalBoss2ComboActive = false
				survivalBoss2ComboAwaitExit = false
				survivalBoss2ComboAwaitEntry = true
				survivalBoss2ComboEntryAt = time.Time{}
				survivalBoss2BeamCycle = 0
				survivalBoss2SetStage(survivalBoss2Hunt, now)
				survivalBoss2RespawnAfterBeamExit(now, w, hgt, survivalBoss2ReentryRight)
				status = "VOID SERPENT // FINAL LOOP"
			} else {
				// The first blackout owns the boss-music exit. Fade it with the picture, then
				// cut/rewind it completely so no boss theme leaks into the consciousness finale.
				survivalBoss1FadeAlias(survivalBossMusicAlias(2), survivalBoss1TargetMusicVolume(), 0, 900*time.Millisecond, true)
				survivalBoss2SetStage(survivalBoss2FinalePreBlack, now)
				status = "SIGNAL LOST // CONSCIOUSNESS FADING"
			}
		}
	case survivalBoss2FinalePreBlack:
		if elapsed >= 900*time.Millisecond {
			// Full black is the clean handoff point: boss theme is gone, heartbeat rises as
			// consciousness returns with Head 1.
			mci("stop " + survivalBossMusicAlias(2))
			mci("seek " + survivalBossMusicAlias(2) + " to start")
			survivalBoss2StartHeartbeat()
			survivalBoss2SetStage(survivalBoss2FinaleHead1, now)
			playSurvivalBoss2FinaleRoar(1)
		}
	case survivalBoss2FinaleHead1:
		if elapsed >= 2300*time.Millisecond {
			survivalBoss2SetStage(survivalBoss2FinaleHead2, now)
			playSurvivalBoss2FinaleRoar(2)
		}
	case survivalBoss2FinaleHead2:
		if elapsed >= 2300*time.Millisecond {
			survivalBoss2SetStage(survivalBoss2FinaleHead3, now)
		}
	case survivalBoss2FinaleHead3:
		// Final vocal lands just before the mouth-core window. The heartbeat cuts on the
		// roar itself, leaving a short exposed beat before the node appears.
		if elapsed >= 800*time.Millisecond && !survivalBoss2FinalRoarPlayed {
			survivalBoss2FinalRoarPlayed = true
			survivalBoss2StopHeartbeat()
			playSurvivalBoss2FinaleRoar(3)
		}
		if elapsed >= 1100*time.Millisecond {
			survivalBoss2StopHeartbeat()
			survivalBoss2FinaleHits = 0
			survivalBoss2FinaleDeadline = now.Add(5500 * time.Millisecond)
			survivalBoss2SetStage(survivalBoss2FinaleNode, now)
			status = "FINAL MOUTH CORE // 20 HITS"
		}
	case survivalBoss2FinaleNode:
		if !survivalBoss2FinaleDeadline.IsZero() && now.After(survivalBoss2FinaleDeadline) {
			survivalFail(h, "Final mouth core timed out")
			return true
		}
	case survivalBoss2FinaleFlash:
		if elapsed >= 900*time.Millisecond {
			finishSurvivalBoss2Clear(h, now)
			return true
		}
	}
	invalidateSurvivalArena(h)
	invalidateSurvivalHUD(h)
	return true
}

func survivalBoss2FinaleNodePoint(w, h int32) FPoint {
	r := arenaRect(w, h)
	return FPoint{X: float64(r.Left+r.Right) / 2, Y: float64(r.Top) + float64(r.Bottom-r.Top)*0.57}
}

func survivalBoss2HandleFinaleNodeClick(h uintptr, p FPoint) bool {
	now := survivalBossFightNow(time.Now(), survivalBoss2StartedAt)
	w, hgt := getClient(h)
	np := survivalBoss2FinaleNodePoint(w, hgt)
	if math.Hypot(p.X-np.X, p.Y-np.Y) > float64(sx(58, w)) {
		survivalFail(h, "Missed the final mouth core")
		return true
	}
	if !survivalBoss2FinaleDeadline.IsZero() && now.After(survivalBoss2FinaleDeadline) {
		survivalFail(h, "Final mouth core timed out")
		return true
	}
	survivalBoss2FinaleHits++
	survivalBoss2FinaleHeadFlashUntil = now.Add(120 * time.Millisecond)
	survivalBoss2Hits++
	survivalTotalHits++
	score += 220
	playBossClickEffect()
	if survivalBoss2FinaleHits >= 20 {
		survivalBoss2FinaleDeadline = time.Time{}
		survivalBoss2StopHeartbeat()
		survivalBoss1StopMusic()
		survivalBoss2SetStage(survivalBoss2FinaleFlash, now)
		status = "VOID SERPENT DESTROYED"
	} else {
		status = fmt.Sprintf("FINAL MOUTH CORE // %d HITS LEFT", 20-survivalBoss2FinaleHits)
	}
	invalidateSurvivalArena(h)
	invalidateSurvivalHUD(h)
	return true
}

func survivalBoss2HandleClick(h uintptr, p FPoint, right bool) bool {
	if survivalBoss2Stage == survivalBoss2FinaleNode {
		return survivalBoss2HandleFinaleNodeClick(h, p)
	}
	if survivalBoss2Stage == survivalBoss2BeamNode {
		return survivalBoss2HandleBeamNodeClick(h, p, right)
	}
	if survivalBoss2Stage == survivalBoss2MeteorRun {
		return survivalBoss2HandleMeteorClick(h, p)
	}
	if survivalBoss2Stage == survivalBoss2Dodge || survivalBoss2Stage == survivalBoss2BeamSetup ||
		survivalBoss2Stage == survivalBoss2BeamCharge || survivalBoss2Stage == survivalBoss2BeamFire ||
		survivalBoss2Stage == survivalBoss2BeamRecover {
		// These are movement/cover phases, not click tests. Ignore stray/double clicks
		// so a successful mouth-core click cannot be followed by an accidental fail.
		return true
	}
	if !survivalBoss2CombatActive() {
		return true
	}
	now := survivalBossFightNow(time.Now(), survivalBoss2StartedAt)
	w, _ := getClient(h)

	// v411 Hunt rewrite: all five body cores are live at once. Resolve the click against
	// the exact permanent host segment for every still-alive node; there is no single
	// active-node pointer and therefore no mechanism capable of sliding a core along the body.
	if !survivalBoss2ComboActive || survivalBoss2ComboTotal <= 0 {
		survivalFail(h, "Void core misclick")
		return true
	}
	hit := -1
	best := math.MaxFloat64
	for i := 0; i < survivalBoss2ComboTotal && i < len(survivalBoss2HuntNodes); i++ {
		n := survivalBoss2HuntNodes[i]
		if !n.Alive {
			continue
		}
		np, ok := survivalBoss2HuntNodePoint(i)
		if !ok {
			continue
		}
		d := math.Hypot(p.X-np.X, p.Y-np.Y)
		if d <= survivalBoss2NodeRadius(w) && d < best {
			hit, best = i, d
		}
	}
	if hit < 0 {
		survivalFail(h, "Void core misclick")
		return true
	}
	n := &survivalBoss2HuntNodes[hit]
	if right == n.Red { // red = left(false), blue = right(true)
		survivalFail(h, "Wrong core button")
		return true
	}

	// v458: every Hunt core takes five correct hits. Hits 1-4 leave the node permanently
	// anchored to the same host segment and keep its three-piece purple safety pocket active.
	// Only hit 5 destroys the core and starts the existing two-second safe-exit window.
	n.Hits++
	survivalBoss2Hits++
	score += 350 + survivalBoss2Hits*20
	survivalPerfectHits++
	survivalTotalHits++
	playSurvivalBoss2HuntHitSound(n.Hits)

	if n.Hits < 5 {
		// v460: briefly flash the number of hits still required beside the exact moving node.
		// Repeated fast hits replace the previous number immediately, producing 4 -> 3 -> 2 -> 1.
		n.FlashRemaining = 5 - n.Hits
		n.FlashUntil = now.Add(460 * time.Millisecond)
		status = fmt.Sprintf("VOID HUNT // CORE %d HIT %d/5 // %d/5 DESTROYED", hit+1, n.Hits, survivalBoss2ComboIndex)
		invalidateSurvivalHUD(h)
		invalidateSurvivalArena(h)
		return true
	}

	n.Alive = false
	n.FlashRemaining = 0
	n.FlashUntil = time.Time{}
	n.RestoreUntil = now.Add(2 * time.Second)
	survivalBoss2ComboIndex++

	remaining := survivalBoss2HuntLiveCount()
	if remaining == 0 {
		survivalBoss2ComboActive = false
		survivalBoss2ComboAwaitExit = true
		survivalBoss2ComboCompletedAt = now
		status = "HUNT COMPLETE // SERPENT RETREATING"
		survivalWaveBannerText = "ALL 5 BODY CORES DESTROYED"
		survivalWaveBannerUntil = now.Add(900 * time.Millisecond)
	} else {
		status = fmt.Sprintf("VOID HUNT // %d/5 DESTROYED // %d REMAIN", survivalBoss2ComboIndex, remaining)
	}
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
	return true
}

func finishSurvivalBoss2Clear(h uintptr, now time.Time) {
	analyticsEvent("boss_cleared", map[string]any{"boss": "VOID_SERPENT", "wave": survivalWave, "kills": survivalKills})
	survivalBoss1StopMusic()
	survivalSectionCheckpointAfterBoss(21)
	gameMeta.SerpentDefeats++
	unlockAchievement("SURV_SECTOR3")
	survivalBoss1SectionTime = now.Sub(survivalStartedAt).Seconds()
	survivalBoss1ClearTime = now.Sub(survivalBoss2StartedAt).Seconds()
	survivalBoss1SectionReport = true
	survivalSectionReportSector = 2
	survivalBoss2Stage = survivalBoss2None
	survivalBoss2StageAt = time.Time{}
	state = StateResult
	releaseCapture.Call()
	status = "SECTOR 2 CLEAR // CHECKPOINT SAVED"
	lastResult = ResultData{
		Time: survivalBoss1SectionTime, CombinedAcc: survivalPerfectPercent(), TrackingAcc: survivalAccuracyPercent(),
		TargetAcc: survivalPerfectPercent(), TargetsHit: survivalKills, TargetCount: survivalTotalHits,
		TotalScore: score, RoundPoints: score, Streak: 20, Combo: float64(survivalBestCombo),
		Rating: survivalGrade(), Rank: rankForEXP(playerProgress.EXP), EXPEarned: survivalLastEXPAward,
		TotalEXP: playerProgress.EXP, Course: "SURVIVAL_SECTION_2", Difficulty: "SURVIVAL",
	}
	invalidateRect.Call(h, 0, 0)
	updateWindow.Call(h)
}

func survivalBoss2HeadOpenNow(now time.Time) bool {
	if survivalBoss2Stage == survivalBoss2BeamNode || survivalBoss2Stage == survivalBoss2BeamFire {
		return true
	}
	if survivalBoss2Stage == survivalBoss2BeamSetup || survivalBoss2Stage == survivalBoss2BeamCharge {
		return false
	}
	if !survivalBoss2MouthOpenUntil.IsZero() && now.Before(survivalBoss2MouthOpenUntil) {
		return true
	}
	if survivalBoss2ReentryActive {
		return true
	}
	if survivalBoss2Stage == survivalBoss2Final {
		return true
	}
	// Blue nodes are announced by the serpent itself: mouth opens as soon as the
	// blue-node telegraph spawns and closes on hit/expiry when the node is cleared.
	if survivalBoss2NodeSegment >= 0 && !survivalBoss2NodeRed && !survivalBoss2NodeTelegraphAt.IsZero() && now.Before(survivalBoss2NodeExpiresAt) {
		return true
	}
	if !survivalBoss2LungeTelegraphAt.IsZero() && now.Sub(survivalBoss2LungeTelegraphAt) < 1400*time.Millisecond {
		return true
	}
	return false
}

func drawBoss2RotatedBGRA(hdc uintptr, data []byte, srcW, srcH int32, c FPoint, dstW, dstH int32, angle float64) {
	if len(data) < int(srcW*srcH*4) || dstW <= 0 || dstH <= 0 {
		return
	}
	spr := ensureRuntimeSprite(hdc, data, srcW, srcH)
	if spr == nil || spr.dc == 0 {
		return
	}
	saved, _, _ := saveDC.Call(hdc)
	if saved == 0 {
		return
	}
	defer restoreDC.Call(hdc, saved)
	setGraphicsModeBoss2.Call(hdc, 2) // GM_ADVANCED
	cs, sn := float32(math.Cos(angle)), float32(math.Sin(angle))
	xf := xformBoss2{M11: cs, M12: sn, M21: -sn, M22: cs, Dx: float32(c.X), Dy: float32(c.Y)}
	setWorldTransformBoss2.Call(hdc, uintptr(unsafe.Pointer(&xf)))
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(int32(-dstW/2)), uintptr(int32(-dstH/2)), uintptr(dstW), uintptr(dstH), spr.dc, 0, 0, uintptr(srcW), uintptr(srcH), blend)
}

func drawBoss2ScrollingBackground(hdc uintptr, r RECT, now time.Time) {
	const srcW int32 = 2048
	const srcH int32 = 682
	if len(survivalBoss2Background) < int(srcW*srcH*4) {
		if len(survivalBackgrounds[1]) >= 1508*592*4 {
			drawRawBGRACover(hdc, survivalBackgrounds[1], 1508, 592, r)
		} else {
			fillSolidRect(hdc, r, rgb(3, 3, 18))
		}
		return
	}
	sprite := ensureRuntimeSprite(hdc, survivalBoss2Background, srcW, srcH)
	if sprite == nil || sprite.dc == 0 {
		fillSolidRect(hdc, r, rgb(3, 3, 18))
		return
	}
	dstW, dstH := r.Right-r.Left, r.Bottom-r.Top
	if dstW <= 0 || dstH <= 0 {
		return
	}
	// Preserve the supplied boss-arena art at cover scale and slide the camera
	// horizontally through the extra source width. A long eased traversal avoids a
	// visible wrap seam while still making the encounter feel like forward flight.
	dstAspect := float64(dstW) / float64(dstH)
	cropW := int32(float64(srcH) * dstAspect)
	if cropW > srcW {
		cropW = srcW
	}
	cropXMax := srcW - cropW
	cropX := int32(0)
	if cropXMax > 0 && backgroundMotionEnabled() {
		elapsed := now.Sub(survivalBoss2StartedAt).Seconds()
		if elapsed < 0 {
			elapsed = 0
		}
		// 13.5 seconds edge-to-edge, then smoothly returns. The serpent itself remains
		// independent of this camera motion, so hitboxes never drift with the background.
		phase := math.Mod(elapsed/27.0, 1.0)
		travel := 0.5 - 0.5*math.Cos(phase*2*math.Pi)
		cropX = int32(math.Round(float64(cropXMax) * travel))
	}
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(dstW), uintptr(dstH),
		sprite.dc, uintptr(cropX), 0, uintptr(cropW), uintptr(srcH), blend)
}

func drawBoss2ReentryTelegraph(hdc uintptr, r RECT, w, h int32, now time.Time) {
	if !survivalBoss2ReentryActive || survivalBoss2ReentrySide == survivalBoss2ReentryNone ||
		survivalBoss2ReentryTelegraphAt.IsZero() || now.After(survivalBoss2ReentryTelegraphAt.Add(700*time.Millisecond)) {
		return
	}
	// The flash follows the physical incoming edge, not a route-planning guess. This is
	// intentionally resolved again at draw time so even an emergency/forced return can
	// never show the opposite side from the serpent that is actually entering.
	side := survivalBoss2ReentrySide
	if actual := survivalBoss2PhysicalOutsideSide(w, h); actual != survivalBoss2ReentryNone {
		side = actual
	}
	pulse := .5 + .5*math.Sin(float64(now.UnixMilli())*.020)
	alpha := byte(20 + int(48*pulse))
	col := rgb(175, 55, 255)
	thickX := sx(58, w)
	thickY := sy(58, h)
	var glow RECT
	switch side {
	case survivalBoss2ReentryLeft:
		glow = RECT{r.Left, r.Top, r.Left + thickX, r.Bottom}
	case survivalBoss2ReentryRight:
		glow = RECT{r.Right - thickX, r.Top, r.Right, r.Bottom}
	case survivalBoss2ReentryTop:
		glow = RECT{r.Left, r.Top, r.Right, r.Top + thickY}
	case survivalBoss2ReentryBottom:
		glow = RECT{r.Left, r.Bottom - thickY, r.Right, r.Bottom}
	default:
		return
	}
	alphaSolidRect(hdc, glow, col, alpha)
	// A crisp inner line makes the warning readable even on bright parts of the
	// scrolling boss background without covering the playable arena.
	switch side {
	case survivalBoss2ReentryLeft:
		drawLineSimple(hdc, glow.Right, glow.Top, glow.Right, glow.Bottom, 2, col)
	case survivalBoss2ReentryRight:
		drawLineSimple(hdc, glow.Left, glow.Top, glow.Left, glow.Bottom, 2, col)
	case survivalBoss2ReentryTop:
		drawLineSimple(hdc, glow.Left, glow.Bottom, glow.Right, glow.Bottom, 2, col)
	case survivalBoss2ReentryBottom:
		drawLineSimple(hdc, glow.Left, glow.Top, glow.Right, glow.Top, 2, col)
	}
}

func drawSurvivalBoss2BodyAndTail(hdc uintptr, w, h int32, now time.Time) {
	r := arenaRect(w, h)
	nodePresent := survivalBoss2NodeSegment >= 0 && survivalBoss2NodeSafetyActive(now)

	// Draw the COMPLETE serpent at all times, including underneath an active node.
	// Node safety is handled exclusively by collision suppression, never by hiding art.
	// This keeps the boss visually continuous while still creating a genuinely clickable
	// interaction pocket in the serpent contact geometry.
	_ = nodePresent
	for i := survivalBoss2SegmentCount - 1; i >= 0; i-- {
		s := survivalBoss2Segments[i]
		if !s.Valid {
			continue
		}
		// The serpent is deliberately much longer than the arena. Cull only pieces that
		// are well outside the viewport so the extra offscreen length costs virtually no
		// GDI work while still remaining present in the articulated chain.
		marginX := float64(sx(180, w))
		marginY := float64(sy(150, h))
		if s.P.X < float64(r.Left)-marginX || s.P.X > float64(r.Right)+marginX ||
			s.P.Y < float64(r.Top)-marginY || s.P.Y > float64(r.Bottom)+marginY {
			continue
		}
		data := survivalBoss2Body1
		sw, sh := int32(162), int32(73)
		switch i % 3 {
		case 1:
			data = survivalBoss2Body2
			sw, sh = 155, 75
		case 2:
			data = survivalBoss2Body3
			sw, sh = 145, 81
		}
		if survivalBoss2SegmentCollisionDisabled(i, now) {
			// Every collision-disabled Hunt body sprite flashes purple. This intentionally includes
			// the host segment and its immediate neighbours, matching the three-piece safe pocket.
			purple := survivalBoss2Body1Purple
			switch i % 3 {
			case 1:
				purple = survivalBoss2Body2Purple
			case 2:
				purple = survivalBoss2Body3Purple
			}
			drawBoss2RotatedBGRA(hdc, data, sw, sh, s.P, sx(116, w), sy(67, h), s.Angle)
			alpha := byte(150)
			if survivalBoss2PurplePulseBright(now) {
				alpha = 245
			}
			drawBoss2RotatedBGRAAlpha(hdc, purple, sw, sh, s.P, sx(116, w), sy(67, h), s.Angle, alpha)
			continue
		}
		// Source body art is horizontal; angle is the tangent toward the head.
		drawBoss2RotatedBGRA(hdc, data, sw, sh, s.P, sx(116, w), sy(67, h), s.Angle)
	}
	// Tail is rendered on the last valid chain point, flipped to point away from the head.
	for i := survivalBoss2SegmentCount - 1; i >= 0; i-- {
		if s := survivalBoss2Segments[i]; s.Valid {
			marginX := float64(sx(220, w))
			marginY := float64(sy(170, h))
			if s.P.X >= float64(r.Left)-marginX && s.P.X <= float64(r.Right)+marginX &&
				s.P.Y >= float64(r.Top)-marginY && s.P.Y <= float64(r.Bottom)+marginY {
				drawBoss2RotatedBGRA(hdc, survivalBoss2Tail, 210, 53, s.P, sx(172, w), sy(50, h), s.Angle+math.Pi)
			}
			break
		}
	}

}

func drawSurvivalBoss2MeteorPhase(hdc uintptr, w, h int32, now time.Time) {
	r := arenaRect(w, h)
	// Warning beat: shake the travelling-space background and pulse purple before
	// any lethal geometry appears.
	if survivalBoss2Stage == survivalBoss2MeteorWarning {
		age := now.Sub(survivalBoss2StageAt).Seconds()
		shake := 1.0 - math.Min(1, age/1.2)
		shakeScale := screenShakeScale()
		dx := int32(math.Sin(age*78.0) * float64(sx(9, w)) * shake * shakeScale)
		dy := int32(math.Cos(age*63.0) * float64(sy(7, h)) * shake * shakeScale)
		shift := RECT{r.Left + dx, r.Top + dy, r.Right + dx, r.Bottom + dy}
		drawBoss2ScrollingBackground(hdc, shift, now)
		drawSurvivalParticles(hdc, w, h)
		drawSurvivalBoss2BodyAndTail(hdc, w, h, now)
		headAngle := math.Atan2(survivalBoss2Vel.Y, survivalBoss2Vel.X) + math.Pi
		headData := survivalBoss2HeadClosed
		hw, hh := int32(197), int32(125)
		if survivalBoss2HeadOpenNow(now) {
			headData, hw, hh = survivalBoss2HeadOpen, 185, 152
		}
		drawBoss2RotatedBGRA(hdc, headData, hw, hh, survivalBoss2Head, sx(188, w), sy(130, h), headAngle)
		pulse := .5 + .5*math.Sin(age*math.Pi*8)
		alphaSolidRect(hdc, r, rgb(120, 20, 180), byte(35+int(105*pulse)))
		remain := survivalBoss2TotalHits - survivalBoss2Hits
		if remain < 0 {
			remain = 0
		}
		drawSurvivalBossHealthBar(hdc, w, h, "THE VOID SERPENT", remain, survivalBoss2TotalHits, rgb(142, 62, 235), rgb(220, 100, 255))
		return
	}
	drawBoss2ScrollingBackground(hdc, r, now)
	drawSurvivalParticles(hdc, w, h)
	drawSurvivalBoss2BodyAndTail(hdc, w, h, now)
	headAngle := math.Atan2(survivalBoss2Vel.Y, survivalBoss2Vel.X) + math.Pi
	headData := survivalBoss2HeadClosed
	hw, hh := int32(197), int32(125)
	if survivalBoss2HeadOpenNow(now) {
		headData, hw, hh = survivalBoss2HeadOpen, 185, 152
	}
	drawBoss2RotatedBGRA(hdc, headData, hw, hh, survivalBoss2Head, sx(188, w), sy(130, h), headAngle)
	for mi, m := range survivalBoss2Meteors {
		if m.Broken {
			continue
		}
		// v370: every falling boulder continuously rattles side-to-side at high frequency.
		// Each rock gets a different phase/frequency and only ~2-4 px of travel, adding
		// violent falling weight without changing lanes or creating accidental overlaps.
		t := float64(now.UnixNano()) / 1e9
		phase := float64(mi)*1.731 + float64(m.Variant)*0.827
		freq := 92.0 + float64((mi+m.Variant*3)%5)*7.0
		amp := float64(sx(2.6+float64((mi+m.Variant)%3)*0.55, w))
		shakeX := math.Sin(t*freq+phase) * amp
		p := FPoint{X: m.P.X + shakeX, Y: m.P.Y}
		if m.Breakable {
			// v369: no halo/ring behind the mandatory meteor. The actual meteor texture
			// flashes purple instead, using a deliberately translucent overlay so cracks,
			// shading and rock detail remain clearly visible throughout the pulse.
			pulse := .5 + .5*math.Sin(float64(now.UnixMilli())*.0140)
			dw, dh := sx(108, w), sy(108, h)
			drawBoss2RotatedBGRA(hdc, survivalBoss2BreakBoulder, 128, 128, p, dw, dh, 0)
			if len(survivalBoss2BreakBoulderPurple) >= 128*128*4 {
				alpha := byte(42 + int(78*pulse)) // 16-47% overlay: obvious purple flash, rock remains readable.
				drawBoss2RotatedBGRAAlpha(hdc, survivalBoss2BreakBoulderPurple, 128, 128, p, dw, dh, 0, alpha)
			}
		} else {
			data := survivalBoss2Boulder1
			switch m.Variant & 3 {
			case 1:
				data = survivalBoss2Boulder2
			case 2:
				data = survivalBoss2Boulder3
			case 3:
				data = survivalBoss2Boulder4
			}
			drawBoss2RotatedBGRA(hdc, data, 128, 128, p, sx(108, w), sy(108, h), 0)
		}
	}

}

var excludeClipRectBoss2 = gdi32.NewProc("ExcludeClipRect")

func survivalBoss2StartDodgePhase(now time.Time, w, h int32) {
	survivalBoss2SetStage(survivalBoss2Dodge, now)
	survivalBoss2DodgeIndex = 0
	survivalBoss2DodgeRunAt = time.Time{}
	survivalBoss2DodgeSide = survivalBoss2ReentryNone
	survivalBoss2DodgePrevSide = survivalBoss2ReentryNone
	survivalBoss2DodgeTrailLen = 0
	survivalBoss2DodgeTelegraphAt = time.Time{}
	survivalBoss2DodgeReturnFlashAt = time.Time{}
	survivalBoss2DodgeTarget = FPoint{}
	survivalBoss2DodgeEntered = false
	survivalBoss2DodgeSubPass = 0
	survivalBoss2RushEchoes = nil
	survivalBoss2DodgeGapUntil = now.Add(650 * time.Millisecond)
	survivalBoss2Satellites = nil
	status = "VOID RUSH // DODGE THE SERPENT"
	survivalWaveBannerText = "VOID RUSH // 5 PASSES"
	survivalWaveBannerUntil = now.Add(1200 * time.Millisecond)
}

func survivalBoss2ChooseDodgeSide() int {
	// Every pass may come from any edge, but never from the same edge twice in a row.
	choices := []int{survivalBoss2ReentryLeft, survivalBoss2ReentryRight, survivalBoss2ReentryTop, survivalBoss2ReentryBottom}
	if survivalBoss2DodgePrevSide == survivalBoss2ReentryNone {
		return choices[rand.Intn(len(choices))]
	}
	filtered := choices[:0]
	for _, side := range choices {
		if side != survivalBoss2DodgePrevSide {
			filtered = append(filtered, side)
		}
	}
	return filtered[rand.Intn(len(filtered))]
}

func survivalBoss2OppositeDodgeSide(side int) int {
	switch side {
	case survivalBoss2ReentryLeft:
		return survivalBoss2ReentryRight
	case survivalBoss2ReentryRight:
		return survivalBoss2ReentryLeft
	case survivalBoss2ReentryTop:
		return survivalBoss2ReentryBottom
	case survivalBoss2ReentryBottom:
		return survivalBoss2ReentryTop
	}
	return survivalBoss2ReentryNone
}

func survivalBoss2DodgeHeadExitedArena(w, h int32) bool {
	if !survivalBoss2DodgeEntered {
		return false
	}
	r := arenaRect(w, h)
	return survivalBoss2Head.X < float64(r.Left) || survivalBoss2Head.X > float64(r.Right) ||
		survivalBoss2Head.Y < float64(r.Top) || survivalBoss2Head.Y > float64(r.Bottom)
}

func survivalBoss2SnapshotRushEcho() {
	parts := make([]SurvivalBoss2Segment, 0, survivalBoss2SegmentCount)
	for _, seg := range survivalBoss2Segments {
		if seg.Valid {
			parts = append(parts, seg)
		}
	}
	if len(parts) == 0 {
		return
	}
	survivalBoss2RushEchoes = append(survivalBoss2RushEchoes, SurvivalBoss2RushEcho{
		Segments: parts,
		Vel:      survivalBoss2Vel,
	})
}

func survivalBoss2UpdateRushEchoes(dt float64, w, h int32) {
	if len(survivalBoss2RushEchoes) == 0 {
		return
	}
	r := arenaRect(w, h)
	padX := float64(sx(220, w))
	padY := float64(sy(180, h))
	out := survivalBoss2RushEchoes[:0]
	for ei := range survivalBoss2RushEchoes {
		e := survivalBoss2RushEchoes[ei]
		visible := false
		for i := range e.Segments {
			e.Segments[i].P.X += e.Vel.X * dt
			e.Segments[i].P.Y += e.Vel.Y * dt
			p := e.Segments[i].P
			if p.X >= float64(r.Left)-padX && p.X <= float64(r.Right)+padX && p.Y >= float64(r.Top)-padY && p.Y <= float64(r.Bottom)+padY {
				visible = true
			}
		}
		if visible {
			out = append(out, e)
		}
	}
	survivalBoss2RushEchoes = out
}

func survivalBoss2RushEchoCollides(p FPoint, w int32) bool {
	rad := survivalBoss2BodyRadius(w)
	for _, e := range survivalBoss2RushEchoes {
		for _, seg := range e.Segments {
			if math.Hypot(p.X-seg.P.X, p.Y-seg.P.Y) <= rad {
				return true
			}
		}
	}
	return false
}

func survivalBoss2PrepareDodgeRun(now time.Time, w, h int32) {
	// Choose the entry edge BEFORE the serpent appears, snapshot the cursor position,
	// then flash that exact edge purple. The rush is committed to this captured point:
	// moving after the telegraph is how the player dodges it.
	side := survivalBoss2ChooseDodgeSide()
	survivalBoss2DodgeSide = side
	survivalBoss2DodgePrevSide = side
	survivalBoss2DodgeFromLeft = side == survivalBoss2ReentryLeft
	survivalBoss2DodgeTarget = cursorPos
	r := arenaRect(w, h)
	// Clamp the captured aim point inside the playable arena so an edge pixel cannot
	// produce a degenerate vector or an unfair skim along the boundary.
	padX := float64(sx(18, w))
	padY := float64(sy(18, h))
	survivalBoss2DodgeTarget.X = boss2Clamp(survivalBoss2DodgeTarget.X, float64(r.Left)+padX, float64(r.Right)-padX)
	survivalBoss2DodgeTarget.Y = boss2Clamp(survivalBoss2DodgeTarget.Y, float64(r.Top)+padY, float64(r.Bottom)-padY)
	survivalBoss2DodgeTelegraphAt = now
	status = fmt.Sprintf("VOID RUSH // PASS %d/5 INCOMING", survivalBoss2DodgeIndex+1)
	survivalWaveBannerText = fmt.Sprintf("VOID RUSH // %d / 5", survivalBoss2DodgeIndex+1)
	survivalWaveBannerUntil = now.Add(520 * time.Millisecond)
}

func survivalBoss2BeginDodgeRun(now time.Time, w, h int32) {
	r := arenaRect(w, h)
	side := survivalBoss2DodgeSide
	if side == survivalBoss2ReentryNone {
		survivalBoss2PrepareDodgeRun(now, w, h)
		side = survivalBoss2DodgeSide
	}
	target := survivalBoss2DodgeTarget
	entryPadX := float64(sx(210, w))
	entryPadY := float64(sy(210, h))
	// Randomize the point along the warned edge, then calculate a normalized velocity
	// directly through the cursor snapshot. This preserves variation while guaranteeing
	// the serpent actually aims at where the player was when the warning appeared.
	jitterX := float64(r.Right-r.Left) * 0.15
	jitterY := float64(r.Bottom-r.Top) * 0.15
	start := FPoint{}
	switch side {
	case survivalBoss2ReentryLeft:
		start = FPoint{X: float64(r.Left) - entryPadX, Y: boss2Clamp(target.Y+(rand.Float64()*2-1)*jitterY, float64(r.Top)+20, float64(r.Bottom)-20)}
	case survivalBoss2ReentryRight:
		start = FPoint{X: float64(r.Right) + entryPadX, Y: boss2Clamp(target.Y+(rand.Float64()*2-1)*jitterY, float64(r.Top)+20, float64(r.Bottom)-20)}
	case survivalBoss2ReentryTop:
		start = FPoint{X: boss2Clamp(target.X+(rand.Float64()*2-1)*jitterX, float64(r.Left)+20, float64(r.Right)-20), Y: float64(r.Top) - entryPadY}
	default: // bottom
		start = FPoint{X: boss2Clamp(target.X+(rand.Float64()*2-1)*jitterX, float64(r.Left)+20, float64(r.Right)-20), Y: float64(r.Bottom) + entryPadY}
	}
	survivalBoss2Head = start
	dx, dy := target.X-start.X, target.Y-start.Y
	mag := math.Hypot(dx, dy)
	if mag < 1 {
		mag = 1
	}
	// v369: begin each rush at a readable but still dangerous entry speed, then
	// accelerate hard once the head begins committing to the arena. This preserves
	// the cursor-targeted attack while giving the player a short reaction window.
	speed := float64(sx(1050, w))
	if side == survivalBoss2ReentryTop || side == survivalBoss2ReentryBottom {
		speed = float64(sy(1050, h))
	}
	survivalBoss2Vel = FPoint{X: dx / mag * speed, Y: dy / mag * speed}
	survivalBoss2DodgeY = survivalBoss2Head.Y

	// Pre-seed a full body behind the head along the reverse travel vector. The next
	// pass cannot start until the final tail has physically cleared the arena.
	survivalBoss2History = nil
	spacing := survivalBoss2SegmentSpacing(w)
	trailLen := float64(survivalBoss2SegmentCount+4) * spacing
	survivalBoss2DodgeTrailLen = trailLen
	ux, uy := survivalBoss2Vel.X/speed, survivalBoss2Vel.Y/speed
	for d := trailLen; d >= 0; d -= 1.5 {
		survivalBoss2History = append(survivalBoss2History, SurvivalBoss2HistoryPoint{X: survivalBoss2Head.X - ux*d, Y: survivalBoss2Head.Y - uy*d})
	}
	survivalBoss2UpdateHistory(w)
	survivalBoss2DodgeRunAt = now
	survivalBoss2DodgeTelegraphAt = time.Time{}
	survivalBoss2DodgeEntered = false
	status = fmt.Sprintf("VOID RUSH // PASS %d/5", survivalBoss2DodgeIndex+1)
}

func survivalBoss2DodgeTailFullyExited(w, h int32) bool {
	if !survivalBoss2DodgeEntered {
		return false
	}
	r := arenaRect(w, h)
	padX := float64(sx(125, w))
	padY := float64(sy(100, h))
	outside := func(p FPoint) bool {
		return p.X < float64(r.Left)-padX || p.X > float64(r.Right)+padX || p.Y < float64(r.Top)-padY || p.Y > float64(r.Bottom)+padY
	}
	if !outside(survivalBoss2Head) {
		return false
	}
	for _, seg := range survivalBoss2Segments {
		if seg.Valid && !outside(seg.P) {
			return false
		}
	}
	return true
}

func updateSurvivalBoss2Dodge(h uintptr, now time.Time, w, hgt int32) bool {
	if survivalBoss2DodgeRunAt.IsZero() {
		if now.Before(survivalBoss2DodgeGapUntil) {
			invalidateSurvivalArena(h)
			return true
		}
		if survivalBoss2DodgeTelegraphAt.IsZero() {
			survivalBoss2PrepareDodgeRun(now, w, hgt)
			invalidateSurvivalArena(h)
			return true
		}
		if now.Sub(survivalBoss2DodgeTelegraphAt) < 500*time.Millisecond {
			invalidateSurvivalArena(h)
			return true
		}
		survivalBoss2BeginDodgeRun(now, w, hgt)
	}
	dt := now.Sub(survivalBoss2LastUpdate).Seconds()
	if dt <= 0 {
		dt = .016
	}
	if dt > .04 {
		dt = .04
	}
	survivalBoss2LastUpdate = now
	survivalBoss2UpdateRushEchoes(dt, w, hgt)

	// Accelerate from ~55% to full rush speed over the first 420 ms. Smoothstep
	// avoids a visible speed snap while still ramping aggressively enough to feel
	// like the serpent launches after its initial readable entry.
	elapsed := now.Sub(survivalBoss2DodgeRunAt).Seconds()
	t := boss2Clamp(elapsed/0.420, 0, 1)
	ease := t * t * (3 - 2*t)
	maxSpeed := float64(sx(1900, w)) * survivalBoss2ExtraMovementScale
	if survivalBoss2DodgeSide == survivalBoss2ReentryTop || survivalBoss2DodgeSide == survivalBoss2ReentryBottom {
		maxSpeed = float64(sy(1900, hgt)) * survivalBoss2ExtraMovementScale
	}
	startSpeed := maxSpeed * (1050.0 / 1900.0)
	currentSpeed := startSpeed + (maxSpeed-startSpeed)*ease
	vmag := math.Hypot(survivalBoss2Vel.X, survivalBoss2Vel.Y)
	if vmag > 0.001 {
		survivalBoss2Vel.X = survivalBoss2Vel.X / vmag * currentSpeed
		survivalBoss2Vel.Y = survivalBoss2Vel.Y / vmag * currentSpeed
	}
	survivalBoss2Head.X += survivalBoss2Vel.X * dt
	survivalBoss2Head.Y += survivalBoss2Vel.Y * dt
	survivalBoss2UpdateHistory(w)
	r := arenaRect(w, hgt)
	if survivalBoss2Head.X >= float64(r.Left) && survivalBoss2Head.X <= float64(r.Right) && survivalBoss2Head.Y >= float64(r.Top) && survivalBoss2Head.Y <= float64(r.Bottom) {
		survivalBoss2DodgeEntered = true
	}
	if survivalBoss2CursorCollides(cursorPos, w, hgt) || survivalBoss2RushEchoCollides(cursorPos, w) {
		survivalFail(h, "Hit by the Void Serpent rush")
		return true
	}

	// Final-loop rushes are paired. The instant the first head clears the arena, keep
	// its still-moving body trail alive and launch the serpent back from the exact
	// opposite edge. This creates two simultaneous trails without cloning rewards or
	// advancing the pass counter twice.
	if survivalBoss2Loop >= 1 && survivalBoss2DodgeSubPass == 0 && survivalBoss2DodgeHeadExitedArena(w, hgt) {
		firstSide := survivalBoss2DodgeSide
		survivalBoss2SnapshotRushEcho()
		survivalBoss2DodgeSubPass = 1
		survivalBoss2DodgeSide = survivalBoss2OppositeDodgeSide(firstSide)
		survivalBoss2DodgePrevSide = survivalBoss2DodgeSide
		survivalBoss2DodgeTarget = survivalBoss2ClampTargetToArena(cursorPos, w, hgt)
		// The paired return is intentionally immediate, but the opposite entry edge must
		// still flash again so the player receives the same directional warning cue on
		// pass two. Keep this independent from DodgeTelegraphAt because BeginDodgeRun
		// clears the normal pre-run telegraph timestamp.
		survivalBoss2DodgeReturnFlashAt = now
		survivalBoss2BeginDodgeRun(now, w, hgt)
		status = fmt.Sprintf("VOID RUSH // PASS %d/5 // RETURN STRIKE", survivalBoss2DodgeIndex+1)
		survivalWaveBannerText = "VOID RUSH // OPPOSITE RETURN"
		survivalWaveBannerUntil = now.Add(420 * time.Millisecond)
		invalidateSurvivalArena(h)
		return true
	}

	if survivalBoss2DodgeTailFullyExited(w, hgt) {
		survivalBoss2DodgeIndex++
		survivalBoss2DodgeSubPass = 0
		survivalBoss2DodgeRunAt = time.Time{}
		survivalBoss2DodgeTelegraphAt = time.Time{}
		survivalBoss2DodgeSide = survivalBoss2ReentryNone
		survivalBoss2DodgeEntered = false
		if survivalBoss2DodgeIndex >= 5 {
			survivalBoss2RushEchoes = nil
			survivalBoss2BeamCycle = 0
			survivalBoss2StartBeamCycle(now, w, hgt)
			return true
		}
		survivalBoss2DodgeGapUntil = now.Add(430 * time.Millisecond)
	}
	invalidateSurvivalArena(h)
	invalidateSurvivalHUD(h)
	return true
}

func survivalBoss2BeamRowRect(row int, w, h int32) RECT {
	r := arenaRect(w, h)
	if row < 0 {
		row = 0
	}
	if row > 5 {
		row = 5
	}
	height := r.Bottom - r.Top
	top := r.Top + int32(int64(height)*int64(row)/6)
	bottom := r.Top + int32(int64(height)*int64(row+1)/6)
	if row == 5 {
		bottom = r.Bottom
	}
	return RECT{Left: r.Left, Top: top, Right: r.Right, Bottom: bottom}
}

func survivalBoss2SpawnBeamSatellites(now time.Time, w, h int32) {
	r := arenaRect(w, h)
	dims := [4][2]int32{{133, 164}, {101, 135}, {110, 136}, {151, 153}}
	survivalBoss2Satellites = make([]SurvivalBoss2Satellite, 0, 1)
	// v407: exactly ONE cover satellite. It always lives on the left side of the
	// arena so the player has a reliable emergency lane instead of scanning for two
	// randomly placed wrecks. The vertical lane still changes between volleys.
	row := 1 + rand.Intn(4) // avoid the cramped top/bottom HUD-adjacent lanes
	v := rand.Intn(4)
	rr := survivalBoss2BeamRowRect(row, w, h)
	y := float64(rr.Top+rr.Bottom) * 0.5
	width := float64(r.Right - r.Left)
	x := float64(r.Left) + width*(0.225+(rand.Float64()-.5)*0.035)
	dw := sx(100, w)
	dh := sy(118, h)
	if dims[v][0] > dims[v][1] {
		dw = sx(124, w)
		dh = sy(96, h)
	}
	maxH := (rr.Bottom - rr.Top) - sy(8, h)
	if dh > maxH {
		dh = maxH
	}
	angle := (rand.Float64()*10.0 - 5.0) * math.Pi / 180.0
	survivalBoss2Satellites = append(survivalBoss2Satellites, SurvivalBoss2Satellite{
		P: FPoint{X: x, Y: y}, Variant: v, W: dw, H: dh, Angle: angle, Row: row,
	})
	survivalBoss2BeamSatelliteAt = now
	status = fmt.Sprintf("SATELLITE INTERCEPT // VOLLEY %d/3", survivalBoss2BeamCycle+1)
	survivalWaveBannerText = "ONE COVER SATELLITE // LEFT SIDE"
	survivalWaveBannerUntil = now.Add(1150 * time.Millisecond)
}

func survivalBoss2StartBeamCycle(now time.Time, w, h int32) {
	r := arenaRect(w, h)
	survivalBoss2SetStage(survivalBoss2BeamSetup, now)
	survivalBoss2BeamNodeDeadline = time.Time{}
	survivalBoss2BeamSatelliteAt = time.Time{}
	survivalBoss2BeamRecoverUntil = time.Time{}
	survivalBoss2BeamRecoverFinal = false
	survivalBoss2BeamExitStartedAt = time.Time{}
	survivalBoss2Satellites = nil
	survivalBoss2EnergyBalls = nil
	survivalBoss2EnergyLastUpdate = time.Time{}
	survivalBoss2EnergyDoneAt = time.Time{}
	survivalBoss2BeamChainNodes = nil
	survivalBoss2BeamChainIndex = 0
	survivalBoss2BeamChainDeadline = time.Time{}
	survivalBoss2BeamChainStartedAt = time.Time{}
	survivalBoss2BeamHeadFlashUntil = time.Time{}
	survivalBoss2FinaleHits = 0
	survivalBoss2FinaleDeadline = time.Time{}
	survivalBoss2LastUpdate = now
	parkX := float64(r.Right - sx(92, w))
	centreY := float64(r.Top + (r.Bottom-r.Top)/2)
	if survivalBoss2BeamCycle == 0 {
		survivalBoss2Head = FPoint{X: float64(r.Right + sx(175, w)), Y: centreY}
		survivalBoss2Vel = FPoint{X: -float64(sx(520, w)) * survivalBoss2ExtraMovementScale, Y: 0}
		survivalBoss2BeamParkedAt = time.Time{}
		status = "VOID SERPENT // INCOMING"
		survivalWaveBannerText = "VOID SERPENT // INCOMING"
	} else {
		survivalBoss2Head = FPoint{X: parkX, Y: centreY}
		survivalBoss2Vel = FPoint{X: -1, Y: 0}
		survivalBoss2BeamParkedAt = now
		status = fmt.Sprintf("SATELLITE INTERCEPT // VOLLEY %d/3", survivalBoss2BeamCycle+1)
		survivalWaveBannerText = "NEW SATELLITES INCOMING"
	}
	survivalWaveBannerUntil = now.Add(1000 * time.Millisecond)
}

func developerStartSurvivalBoss2EnergyPhase2() string {
	if mainHwnd == 0 {
		return "WINDOW NOT READY"
	}
	// Use the production Boss 2 setup first so audio, assets, HP, timers and reporting
	// are identical to a real encounter, then advance only the encounter state.
	_ = developerStartSurvivalBoss(2)
	now := time.Now()
	w, h := getClient(mainHwnd)
	survivalBoss2Loop = 1
	survivalBoss2LoopStartHits = survivalBoss2Hits
	survivalBoss2MeteorDone = true
	survivalBoss2MeteorPending = false
	survivalBoss2MeteorRound = 5
	survivalBoss2DodgeIndex = 5
	survivalBoss2ComboNumber = 5
	survivalBoss2ComboActive = false
	survivalBoss2ComboAwaitExit = false
	survivalBoss2ComboAwaitEntry = false
	survivalBoss2BeamCycle = 0
	survivalBoss2Satellites = nil
	survivalBoss2EnergyBalls = nil
	survivalBoss2FinalRoarPlayed = false
	survivalBoss2StopHeartbeat()
	survivalBoss2StartBeamCycle(now, w, h)
	developerSurvivalGraceUntil = now.Add(5 * time.Second)
	status = "DEVELOPER // VOID SERPENT ENERGY PHASE 2"
	survivalWaveBannerText = "DEV WARP // ENERGY BALL PHASE 2 // 5s INVINCIBLE"
	survivalWaveBannerUntil = now.Add(1800 * time.Millisecond)
	invalidateRect.Call(mainHwnd, 0, 0)
	return "VOID SERPENT // ENERGY BALL PHASE 2 STARTED — 5s INVINCIBILITY"
}

func survivalBoss2SpawnEnergyBalls(now time.Time, w, h int32) {
	mouth := survivalBoss2BeamNodePoint(w, h)
	survivalBoss2EnergyBalls = make([]SurvivalBoss2EnergyBall, 0, 6)
	for row := 0; row < 6; row++ {
		rr := survivalBoss2BeamRowRect(row, w, h)
		y := float64(rr.Top+rr.Bottom) * 0.5
		survivalBoss2EnergyBalls = append(survivalBoss2EnergyBalls, SurvivalBoss2EnergyBall{
			Row: row, P: mouth, LaneY: y, Active: true,
			Trail: []SurvivalBoss2EnergyTrailPoint{{P: mouth, At: now}},
		})
	}
	survivalBoss2EnergyLastUpdate = now
	survivalBoss2EnergyDoneAt = time.Time{}
	survivalBoss2FinaleHits = 0
	survivalBoss2FinaleDeadline = time.Time{}
}

func survivalBoss2RemoveSatelliteRow(row int) {
	if len(survivalBoss2Satellites) == 0 {
		return
	}
	out := survivalBoss2Satellites[:0]
	for _, sat := range survivalBoss2Satellites {
		if sat.Row != row {
			out = append(out, sat)
		}
	}
	survivalBoss2Satellites = out
}

func survivalBoss2EnergySatelliteHit(ball *SurvivalBoss2EnergyBall, w, h int32) bool {
	if ball == nil || !ball.Active || !ball.OnLane {
		return false
	}
	for _, sat := range survivalBoss2Satellites {
		if sat.Row != ball.Row {
			continue
		}
		halfW := float64(sat.W)/2 + float64(sx(22, w))
		halfH := float64(sat.H)/2 + float64(sy(16, h))
		if math.Abs(ball.P.X-sat.P.X) <= halfW && math.Abs(ball.P.Y-sat.P.Y) <= halfH {
			ball.Active = false
			ball.Absorbed = true
			survivalBoss2RemoveSatelliteRow(ball.Row)
			playSurvivalExplodeSound()
			return true
		}
	}
	return false
}

func survivalBoss2SatelliteCollides(p FPoint, w, h int32) bool {
	// During the Void Energy volley the satellite wreckage is physical cover, not a
	// safe object to sit inside. Use the same generous footprint the energy balls use
	// for interception so the visible sprite and cursor hazard agree.
	for _, sat := range survivalBoss2Satellites {
		halfW := float64(sat.W)/2 + float64(sx(10, w))
		halfH := float64(sat.H)/2 + float64(sy(8, h))
		if math.Abs(p.X-sat.P.X) <= halfW && math.Abs(p.Y-sat.P.Y) <= halfH {
			return true
		}
	}
	return false
}

func boss2PointSegmentDistance(p, a, b FPoint) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	den := dx*dx + dy*dy
	if den <= 0.0001 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / den
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	q := FPoint{X: a.X + t*dx, Y: a.Y + t*dy}
	return math.Hypot(p.X-q.X, p.Y-q.Y)
}

func survivalBoss2EnergyCollides(p FPoint, w, h int32, now time.Time) bool {
	coreR := float64(sx(27, w))
	trailR := float64(sx(18, w))
	for i := range survivalBoss2EnergyBalls {
		b := &survivalBoss2EnergyBalls[i]
		if b.Active && math.Hypot(p.X-b.P.X, p.Y-b.P.Y) <= coreR {
			return true
		}
		for j := 1; j < len(b.Trail); j++ {
			a, c := b.Trail[j-1], b.Trail[j]
			if now.Sub(c.At) > 720*time.Millisecond {
				continue
			}
			if boss2PointSegmentDistance(p, a.P, c.P) <= trailR {
				return true
			}
		}
	}
	return false
}

func updateSurvivalBoss2EnergyBalls(now time.Time, w, h int32) bool {
	r := arenaRect(w, h)
	dt := now.Sub(survivalBoss2EnergyLastUpdate).Seconds()
	if dt <= 0 {
		dt = .016
	}
	if dt > .04 {
		dt = .04
	}
	survivalBoss2EnergyLastUpdate = now
	allInactive := true
	for i := range survivalBoss2EnergyBalls {
		b := &survivalBoss2EnergyBalls[i]
		// Trail remains dangerous for a short decay window after the orb is absorbed/exits.
		cut := now.Add(-720 * time.Millisecond)
		k := 0
		for _, tp := range b.Trail {
			if tp.At.After(cut) {
				b.Trail[k] = tp
				k++
			}
		}
		b.Trail = b.Trail[:k]
		if !b.Active {
			continue
		}
		allInactive = false
		if !b.OnLane {
			startX := float64(r.Right - sx(235, w))
			target := FPoint{X: startX, Y: b.LaneY}
			dx, dy := target.X-b.P.X, target.Y-b.P.Y
			d := math.Hypot(dx, dy)
			speed := float64(sx(980, w))
			step := speed * dt
			if d <= step || d < 4 {
				b.P = target
				b.OnLane = true
			} else {
				b.P.X += dx / d * step
				b.P.Y += dy / d * step
			}
		} else {
			b.P.X -= float64(sx(1080, w)) * dt
		}
		if len(b.Trail) == 0 || math.Hypot(b.P.X-b.Trail[len(b.Trail)-1].P.X, b.P.Y-b.Trail[len(b.Trail)-1].P.Y) >= float64(sx(10, w)) {
			b.Trail = append(b.Trail, SurvivalBoss2EnergyTrailPoint{P: b.P, At: now})
		}
		if survivalBoss2EnergySatelliteHit(b, w, h) {
			continue
		}
		if b.P.X < float64(r.Left-sx(55, w)) {
			b.Active = false
		}
	}
	if allInactive {
		if survivalBoss2EnergyDoneAt.IsZero() {
			survivalBoss2EnergyDoneAt = now
		}
		trails := false
		for i := range survivalBoss2EnergyBalls {
			if len(survivalBoss2EnergyBalls[i].Trail) > 1 {
				trails = true
				break
			}
		}
		return !trails && now.Sub(survivalBoss2EnergyDoneAt) >= 120*time.Millisecond
	}
	return false
}

func survivalBoss2BeamBodyPoint(index int, w, h int32, now time.Time) FPoint {
	r := arenaRect(w, h)
	// A continuous body column loops from top to bottom behind the beam/head. It plugs
	// any right-edge/corner sliver while visibly moving instead of looking like a wall.
	step := float64(sy(62, h))
	span := float64(r.Bottom-r.Top) + step*3
	speed := float64(sy(155, h))
	phase := math.Mod(float64(now.UnixNano())/1e9*speed+float64(index)*step, span)
	y := float64(r.Top) - step + phase
	return FPoint{X: float64(r.Right - sx(54, w)), Y: y}
}

func survivalBoss2BeamBodyCollides(p FPoint, w, h int32, now time.Time) bool {
	r := arenaRect(w, h)
	rad := float64(sx(32, w))
	for i := 0; i < 12; i++ {
		q := survivalBoss2BeamBodyPoint(i, w, h, now)
		if q.Y < float64(r.Top)-rad || q.Y > float64(r.Bottom)+rad {
			continue
		}
		if math.Hypot(p.X-q.X, p.Y-q.Y) <= rad {
			return true
		}
	}
	return false
}

func survivalBoss2BeamNodePoint(w, h int32) FPoint {
	return FPoint{X: survivalBoss2Head.X - float64(sx(72, w)), Y: survivalBoss2Head.Y}
}

func survivalBoss2BeamChainNodeWindow() time.Duration {
	// v409: the ten-hit relay is meant to feel like a fast rhythm stream, not ten
	// separate two-second prompts. Boss clocks run at 1.15x speed, so scale the
	// virtual deadline to preserve a real-world 1.25 second window per active node.
	return time.Duration(float64(950*time.Millisecond) * survivalBossFightSpeed)
}

func survivalBoss2BeamMouthWindow() time.Duration {
	// Same treatment as the chain timers: ten real seconds from the instant the
	// mouth core and ordered chain appear together.
	return time.Duration(float64(7*time.Second) * survivalBossFightSpeed)
}

func survivalBoss2ResetBeamChain(now time.Time, reason string) {
	// Missing the relay is NOT a Survival failure. The player simply loses the chain
	// and must immediately restart at node 1 while the mouth's original 10s clock keeps
	// running. That makes the sequence fast and punishing without cheap HP loss.
	survivalBoss2BeamChainIndex = 0
	survivalBoss2BeamChainDeadline = now.Add(survivalBoss2BeamChainNodeWindow())
	status = "VOID RELAY RESET // NODE 1/10"
	if reason != "" {
		survivalWaveBannerText = "RELAY RESET // START AT NODE 1"
		survivalWaveBannerUntil = now.Add(520 * time.Millisecond)
	}
}

func survivalBoss2SpawnBeamChain(now time.Time, w, h int32) {
	r := arenaRect(w, h)
	mouth := survivalBoss2BeamNodePoint(w, h)
	const count = 10
	survivalBoss2BeamChainNodes = make([]SurvivalBoss2BeamChainNode, 0, count)
	survivalBoss2BeamChainIndex = 0
	survivalBoss2BeamChainStartedAt = now
	survivalBoss2BeamChainDeadline = now.Add(survivalBoss2BeamChainNodeWindow())
	survivalBoss2BeamNodeDeadline = now.Add(survivalBoss2BeamMouthWindow())

	// v409: spread the relay across substantially more of the arena. The route still
	// reads cleanly left-to-right into the mouth, but horizontal spacing is wider and
	// the vertical swing is deliberately larger so the player has to make decisive,
	// fast mouse movements instead of clicking a compressed row of targets.
	width := float64(r.Right - r.Left)
	height := float64(r.Bottom - r.Top)
	startX := float64(r.Left) + width*0.17
	endX := mouth.X - float64(sx(72, w))
	minSpan := float64(sx(610, w))
	if endX < startX+minSpan {
		startX = math.Max(float64(r.Left)+float64(sx(58, w)), endX-minSpan)
	}
	centreY := float64(r.Top+r.Bottom) * 0.5
	mirror := 1.0
	if rand.Intn(2) == 0 {
		mirror = -1
	}
	pattern := rand.Intn(6)
	phase := rand.Float64() * math.Pi * 2
	for i := 0; i < count; i++ {
		t := float64(i) / float64(count-1)
		x := startX + (endX-startX)*t
		offset := 0.0
		switch pattern {
		case 0: // broad wave
			offset = .235 * math.Sin(t*math.Pi*2.10+phase)
		case 1: // fast double-wave
			offset = .195*math.Sin(t*math.Pi*3.15+phase) + .055*math.Sin(t*math.Pi*6.30+phase*.35)
		case 2: // one huge sweep from one side of the arena to the other
			offset = .255 * math.Sin((t-.08)*math.Pi*1.55+phase*.28)
		case 3: // strong high/low rhythm
			zig := 1.0
			if i%2 == 0 {
				zig = -1
			}
			offset = zig*(.175+.030*math.Sin(t*math.Pi*2)) + .035*math.Sin(t*math.Pi*2+phase)
		case 4: // alternating groups rather than a uniform zig-zag
			group := 1.0
			if (i/2)%2 == 0 {
				group = -1
			}
			offset = group*.185 + .060*math.Sin(t*math.Pi*3+phase)
		default: // tightening wave into the mouth
			offset = .215 * math.Sin(t*math.Pi*4+phase) * (1 - .24*t)
		}
		y := centreY + offset*mirror*height + (rand.Float64()-.5)*float64(sy(10, h))
		margin := float64(sy(72, h))
		y = boss2Clamp(y, float64(r.Top)+margin, float64(r.Bottom)-margin)
		survivalBoss2BeamChainNodes = append(survivalBoss2BeamChainNodes, SurvivalBoss2BeamChainNode{
			BaseP: FPoint{X: x, Y: y}, Index: i,
		})
	}

	// The cover satellite has finished its job once the energy volley clears. Remove
	// it before the precision chain so it cannot visually compete with the targets.
	survivalBoss2Satellites = nil
	status = fmt.Sprintf("VOID RELAY // NODE 1/%d", count)
	survivalWaveBannerText = fmt.Sprintf("CLICK 1 > %d > MOUTH CORE // 1.25s EACH", count)
	survivalWaveBannerUntil = now.Add(1200 * time.Millisecond)
}

func survivalBoss2BeamChainNodePoint(index int, w, h int32, now time.Time) FPoint {
	if index < 0 || index >= len(survivalBoss2BeamChainNodes) {
		return FPoint{}
	}
	b := survivalBoss2BeamChainNodes[index]
	age := now.Sub(survivalBoss2BeamChainStartedAt).Seconds()
	// Small independent hover motion makes the nodes feel airborne without making the
	// two-second precision window visually slippery or unfair.
	bobX := math.Sin(age*1.85+float64(index)*1.7) * float64(sx(3, w))
	bobY := math.Sin(age*2.35+float64(index)*2.1) * float64(sy(5, h))
	return FPoint{X: b.BaseP.X + bobX, Y: b.BaseP.Y + bobY}
}

func survivalBoss2BeamChainComplete() bool {
	return len(survivalBoss2BeamChainNodes) > 0 && survivalBoss2BeamChainIndex >= len(survivalBoss2BeamChainNodes)
}

func survivalBoss2BeamSpecialCollision(p FPoint, w, h int32, now time.Time) bool {
	if survivalBoss2Stage == survivalBoss2BeamNode {
		return false
	}
	if survivalBoss2Stage == survivalBoss2BeamSetup && survivalBoss2BeamParkedAt.IsZero() {
		return math.Hypot(p.X-survivalBoss2Head.X, p.Y-survivalBoss2Head.Y) <= survivalBoss2HeadRadius(w)
	}
	if !survivalBoss2SafeExitUntil.IsZero() && now.Before(survivalBoss2SafeExitUntil) &&
		math.Hypot(p.X-survivalBoss2SafeExitPoint.X, p.Y-survivalBoss2SafeExitPoint.Y) <= survivalBoss2NodeExitSafeRadius(w) {
		return false
	}
	if math.Hypot(p.X-survivalBoss2Head.X, p.Y-survivalBoss2Head.Y) <= survivalBoss2HeadRadius(w) {
		return true
	}
	return survivalBoss2BeamBodyCollides(p, w, h, now)
}

func survivalBoss2PlayBeamRoar() {
	// The shipped boss2_serpent_start sample is the dedicated serpent roar. Play it
	// directly here instead of playSurvivalBoss2SerpentStart(), because that helper
	// intentionally opens the mouth for 1.5s and would contradict this mechanic: the
	// mouth must CLOSE immediately after the successful mouth-core hit.
	if audioReady && survivalBoss2SerpentStartAudio {
		playOneShotAsync("boss2_serpent_start")
	}
	survivalBoss2MouthOpenUntil = time.Time{}
}

func updateSurvivalBoss2Beam(h uintptr, now time.Time, w, hgt int32) bool {
	r := arenaRect(w, hgt)
	parkX := float64(r.Right - sx(92, w))
	centreY := float64(r.Top + (r.Bottom-r.Top)/2)
	elapsed := now.Sub(survivalBoss2StageAt)

	switch survivalBoss2Stage {
	case survivalBoss2BeamSetup:
		if survivalBoss2BeamParkedAt.IsZero() {
			dt := now.Sub(survivalBoss2LastUpdate).Seconds()
			if dt <= 0 {
				dt = .016
			}
			if dt > .04 {
				dt = .04
			}
			survivalBoss2LastUpdate = now
			survivalBoss2Head.Y = centreY
			survivalBoss2Head.X += survivalBoss2Vel.X * dt
			if survivalBoss2Head.X <= parkX {
				survivalBoss2Head.X = parkX
				survivalBoss2Vel = FPoint{X: -1, Y: 0}
				survivalBoss2BeamParkedAt = now
				status = "VOID SERPENT // HOLDING POSITION"
			}
		} else {
			survivalBoss2Head = FPoint{X: parkX, Y: centreY}
			if len(survivalBoss2Satellites) == 0 && now.Sub(survivalBoss2BeamParkedAt) >= 180*time.Millisecond {
				survivalBoss2SpawnBeamSatellites(now, w, hgt)
			}
			if !survivalBoss2BeamSatelliteAt.IsZero() && now.Sub(survivalBoss2BeamSatelliteAt) >= 700*time.Millisecond {
				survivalBoss2SetStage(survivalBoss2BeamCharge, now)
				status = "VOID ENERGY CHARGING // WATCH THE SIX LANES"
				survivalWaveBannerText = "VOID ENERGY CHARGING"
				survivalWaveBannerUntil = now.Add(1200 * time.Millisecond)
			}
		}
		if survivalBoss2BeamSpecialCollision(cursorPos, w, hgt, now) {
			survivalFail(h, "Touched the Void Serpent")
			return true
		}

	case survivalBoss2BeamCharge:
		survivalBoss2Head = FPoint{X: parkX, Y: centreY}
		if survivalBoss2BeamSpecialCollision(cursorPos, w, hgt, now) {
			survivalFail(h, "Touched the Void Serpent")
			return true
		}
		if elapsed >= 1050*time.Millisecond {
			survivalBoss2SetStage(survivalBoss2BeamFire, now)
			survivalBoss2SpawnEnergyBalls(now, w, hgt)
			if survivalBoss2EnergyBallAudio {
				playOneShotAsync("boss2_energy_ball")
			}
			status = "VOID ENERGY VOLLEY // DODGE THE TRAILS"
			survivalWaveBannerText = "SIX-LANE VOID ENERGY // DODGE"
			survivalWaveBannerUntil = now.Add(850 * time.Millisecond)
		}

	case survivalBoss2BeamFire:
		survivalBoss2Head = FPoint{X: parkX, Y: centreY}
		if survivalBoss2SatelliteCollides(cursorPos, w, hgt) {
			survivalFail(h, "Hit satellite wreckage")
			return true
		}
		if survivalBoss2BeamBodyCollides(cursorPos, w, hgt, now) || survivalBoss2EnergyCollides(cursorPos, w, hgt, now) {
			survivalFail(h, "Hit by Void Energy")
			return true
		}
		if updateSurvivalBoss2EnergyBalls(now, w, hgt) {
			survivalBoss2SetStage(survivalBoss2BeamNode, now)
			survivalBoss2SpawnBeamChain(now, w, hgt)
		}

	case survivalBoss2BeamNode:
		survivalBoss2Head = FPoint{X: parkX, Y: centreY}
		// Head/body collision is fully disabled in this interaction window. This is
		// deliberate: the player must be able to cross the serpent geometry, complete
		// the ordered relay, then strike the mouth node before its ten-second timer ends.
		if !survivalBoss2BeamNodeDeadline.IsZero() && now.After(survivalBoss2BeamNodeDeadline) {
			survivalFail(h, "Mouth core timed out")
			return true
		}
		if !survivalBoss2BeamChainComplete() && !survivalBoss2BeamChainDeadline.IsZero() && now.After(survivalBoss2BeamChainDeadline) {
			survivalBoss2ResetBeamChain(now, "timer")
			invalidateSurvivalArena(h)
			invalidateSurvivalHUD(h)
			return true
		}

	case survivalBoss2BeamRecover:
		survivalBoss2Head = FPoint{X: parkX, Y: centreY}
		// After a mouth hit, keep the encounter stable for one clear roar beat. The
		// old satellite pair fades away before any new cover can appear.
		if !survivalBoss2BeamRecoverUntil.IsZero() && now.After(survivalBoss2BeamRecoverUntil) {
			if survivalBoss2BeamRecoverFinal {
				survivalBoss2Satellites = nil
				// Never teleport away from the energy-ball finale. Rebuild a horizontal
				// history chain behind the parked head, then let the entire serpent leave
				// through the right edge until the tail has naturally followed off-screen.
				survivalBoss2BeginBeamExit(now, w, hgt)
				return true
			}
			survivalBoss2StartBeamCycle(now, w, hgt)
			return true
		}
	}
	invalidateSurvivalArena(h)
	invalidateSurvivalHUD(h)
	return true
}

func survivalBoss2HandleBeamNodeClick(h uintptr, p FPoint, right bool) bool {
	now := survivalBossFightNow(time.Now(), survivalBoss2StartedAt)
	w, hgt := getClient(h)
	mouth := survivalBoss2BeamNodePoint(w, hgt)
	chainComplete := survivalBoss2BeamChainComplete()

	if !chainComplete {
		active := survivalBoss2BeamChainIndex
		if active >= 0 && active < len(survivalBoss2BeamChainNodes) {
			ap := survivalBoss2BeamChainNodePoint(active, w, hgt, now)
			if math.Hypot(p.X-ap.X, p.Y-ap.Y) <= float64(sx(43, w)) {
				// Relay nodes are deliberately button-neutral: either mouse button is valid.
				survivalBoss2BeamChainIndex++
				survivalPerfectHits++
				survivalTotalHits++
				score += 180
				playHitSound()
				if survivalBoss2BeamChainComplete() {
					survivalBoss2BeamChainDeadline = time.Time{}
					status = "VOID RELAY COMPLETE // MOUTH CORE UNLOCKED"
					survivalWaveBannerText = "MOUTH CORE UNLOCKED // STRIKE BEFORE TIMER EXPIRES"
					survivalWaveBannerUntil = now.Add(900 * time.Millisecond)
				} else {
					survivalBoss2BeamChainDeadline = now.Add(survivalBoss2BeamChainNodeWindow())
					status = fmt.Sprintf("VOID RELAY // NODE %d/%d", survivalBoss2BeamChainIndex+1, len(survivalBoss2BeamChainNodes))
				}
				invalidateSurvivalHUD(h)
				invalidateSurvivalArena(h)
				return true
			}
		}

		// The mouth core is visible from the start and its 10-second timer is already
		// running, but clicking it early cannot bypass the relay. Ignore the early core
		// click rather than unfairly killing the player for aiming near the final node.
		if math.Hypot(p.X-mouth.X, p.Y-mouth.Y) <= float64(sx(54, w)) {
			remaining := len(survivalBoss2BeamChainNodes) - survivalBoss2BeamChainIndex
			status = fmt.Sprintf("MOUTH CORE LOCKED // %d RELAY NODES REMAIN", remaining)
			invalidateSurvivalHUD(h)
			return true
		}

		// Any relay miss/out-of-order click breaks the chain but never damages the player.
		// The mouth timer is deliberately NOT restarted, so repeated mistakes still create
		// real pressure and can eventually fail the phase through the mouth timeout.
		for i := range survivalBoss2BeamChainNodes {
			if i == survivalBoss2BeamChainIndex {
				continue
			}
			np := survivalBoss2BeamChainNodePoint(i, w, hgt, now)
			if math.Hypot(p.X-np.X, p.Y-np.Y) <= float64(sx(43, w)) {
				survivalBoss2ResetBeamChain(now, "order")
				invalidateSurvivalHUD(h)
				invalidateSurvivalArena(h)
				return true
			}
		}
		survivalBoss2ResetBeamChain(now, "miss")
		invalidateSurvivalHUD(h)
		invalidateSurvivalArena(h)
		return true
	}

	// The energy-relay mouth core is button-neutral like its relay nodes. Missing it is
	// not an instant failure either: only the continuously-running mouth timer can fail
	// this precision sequence.
	if math.Hypot(p.X-mouth.X, p.Y-mouth.Y) > float64(sx(54, w)) {
		return true
	}
	if !survivalBoss2BeamNodeDeadline.IsZero() && now.After(survivalBoss2BeamNodeDeadline) {
		survivalFail(h, "Mouth core timed out")
		return true
	}

	survivalBoss2Hits++
	survivalPerfectHits++
	survivalTotalHits++
	score += 900
	playHitSound()

	// Prevent the freshly-closed head from becoming lethal under the cursor before
	// the player has physically had time to move away from the mouth.
	survivalBoss2SafeExitPoint = mouth
	survivalBoss2SafeExitSegment = -2
	survivalBoss2SafeExitUntil = now.Add(760 * time.Millisecond)
	survivalBoss2BeamNodeDeadline = time.Time{}
	survivalBoss2BeamChainDeadline = time.Time{}
	survivalBoss2MouthOpenUntil = time.Time{}
	survivalBoss2PlayBeamRoar()

	// v407: a direct mouth hit violently strobes the entire serpent head red. The
	// recovery stage renders this as rapid red/normal alternation plus a small local
	// jitter so the impact is unmistakable without flashing the entire screen.
	survivalBoss2BeamHeadFlashUntil = now.Add(650 * time.Millisecond)

	survivalBoss2BeamCycle++
	survivalBoss2SetStage(survivalBoss2BeamRecover, now)
	survivalBoss2BeamRecoverUntil = now.Add(900 * time.Millisecond)
	survivalBoss2BeamRecoverFinal = survivalBoss2BeamCycle >= 3
	status = "DIRECT HIT // VOID SERPENT STAGGERED"
	survivalWaveBannerText = "DIRECT HIT // SERPENT ROAR"
	survivalWaveBannerUntil = now.Add(800 * time.Millisecond)
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
	return true
}

func drawBoss2RotatedBGRAAlpha(hdc uintptr, data []byte, srcW, srcH int32, c FPoint, dstW, dstH int32, angle float64, alpha byte) {
	if len(data) < int(srcW*srcH*4) || dstW <= 0 || dstH <= 0 || alpha == 0 {
		return
	}
	spr := ensureRuntimeSprite(hdc, data, srcW, srcH)
	if spr == nil || spr.dc == 0 {
		return
	}
	saved, _, _ := saveDC.Call(hdc)
	if saved == 0 {
		return
	}
	defer restoreDC.Call(hdc, saved)
	setGraphicsModeBoss2.Call(hdc, 2)
	cs, sn := float32(math.Cos(angle)), float32(math.Sin(angle))
	xf := xformBoss2{M11: cs, M12: sn, M21: -sn, M22: cs, Dx: float32(c.X), Dy: float32(c.Y)}
	setWorldTransformBoss2.Call(hdc, uintptr(unsafe.Pointer(&xf)))
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(int32(-dstW/2)), uintptr(int32(-dstH/2)), uintptr(dstW), uintptr(dstH), spr.dc, 0, 0, uintptr(srcW), uintptr(srcH), blend)
}

func survivalBoss2SatelliteData(v int) ([]byte, int32, int32) {
	switch v {
	case 0:
		return survivalBoss2Satellite1, 133, 164
	case 1:
		return survivalBoss2Satellite2, 101, 135
	case 2:
		return survivalBoss2Satellite3, 110, 136
	default:
		return survivalBoss2Satellite4, 151, 153
	}
}

func drawSurvivalBoss2BeamBody(hdc uintptr, w, h int32, now time.Time) {
	r := arenaRect(w, h)
	for i := 0; i < 12; i++ {
		q := survivalBoss2BeamBodyPoint(i, w, h, now)
		if q.Y < float64(r.Top)-float64(sy(70, h)) || q.Y > float64(r.Bottom)+float64(sy(70, h)) {
			continue
		}
		data, sw, sh := survivalBoss2Body1, int32(162), int32(73)
		if i%3 == 1 {
			data, sw, sh = survivalBoss2Body2, 155, 75
		} else if i%3 == 2 {
			data, sw, sh = survivalBoss2Body3, 145, 81
		}
		drawBoss2RotatedBGRA(hdc, data, sw, sh, q, sx(112, w), sy(64, h), math.Pi/2)
	}
}

func drawSurvivalBoss2DodgeTelegraph(hdc uintptr, w, h int32, now time.Time) {
	normalFlash := survivalBoss2DodgeRunAt.IsZero() && !survivalBoss2DodgeTelegraphAt.IsZero()
	returnFlash := !survivalBoss2DodgeReturnFlashAt.IsZero() && now.Sub(survivalBoss2DodgeReturnFlashAt) >= 0 && now.Sub(survivalBoss2DodgeReturnFlashAt) < 360*time.Millisecond
	if (normalFlash || returnFlash) && survivalBoss2DodgeSide != survivalBoss2ReentryNone {
		r := arenaRect(w, h)
		pulse := .5 + .5*math.Sin(float64(now.UnixMilli())*.026)
		alpha := byte(55 + int(125*pulse))
		// Make the immediate opposite return flash slightly punchier because there is no
		// separate 500 ms wait before the second head arrives.
		if returnFlash {
			alpha = byte(115 + int(95*pulse))
		}
		col := rgb(190, 45, 255)
		thickX, thickY := sx(72, w), sy(72, h)
		var glow RECT
		switch survivalBoss2DodgeSide {
		case survivalBoss2ReentryLeft:
			glow = RECT{Left: r.Left, Top: r.Top, Right: r.Left + thickX, Bottom: r.Bottom}
		case survivalBoss2ReentryRight:
			glow = RECT{Left: r.Right - thickX, Top: r.Top, Right: r.Right, Bottom: r.Bottom}
		case survivalBoss2ReentryTop:
			glow = RECT{Left: r.Left, Top: r.Top, Right: r.Right, Bottom: r.Top + thickY}
		case survivalBoss2ReentryBottom:
			glow = RECT{Left: r.Left, Top: r.Bottom - thickY, Right: r.Right, Bottom: r.Bottom}
		}
		alphaSolidRect(hdc, glow, col, alpha)
	}
}

func drawSurvivalBoss2RushEchoes(hdc uintptr, w, h int32) {
	for _, e := range survivalBoss2RushEchoes {
		for i := len(e.Segments) - 1; i >= 0; i-- {
			seg := e.Segments[i]
			data, sw, sh := survivalBoss2Body1, int32(162), int32(73)
			switch i % 3 {
			case 1:
				data, sw, sh = survivalBoss2Body2, 155, 75
			case 2:
				data, sw, sh = survivalBoss2Body3, 145, 81
			}
			drawBoss2RotatedBGRA(hdc, data, sw, sh, seg.P, sx(116, w), sy(67, h), seg.Angle)
		}
	}
}

func drawSurvivalBoss2DodgeArena(hdc uintptr, w, h int32, now time.Time) {
	r := arenaRect(w, h)
	drawBoss2ScrollingBackground(hdc, r, now)
	drawSurvivalParticles(hdc, w, h)
	drawSurvivalBoss2DodgeTelegraph(hdc, w, h, now)
	drawSurvivalBoss2RushEchoes(hdc, w, h)
	drawSurvivalBoss2BodyAndTail(hdc, w, h, now)
	headAngle := math.Atan2(survivalBoss2Vel.Y, survivalBoss2Vel.X) + math.Pi
	drawBoss2RotatedBGRA(hdc, survivalBoss2HeadClosed, 197, 125, survivalBoss2Head, sx(188, w), sy(130, h), headAngle)
}

func drawSurvivalBoss2BeamEntryBody(hdc uintptr, w, h int32) {
	for i := 1; i <= 5; i++ {
		p := FPoint{X: survivalBoss2Head.X + float64(sx(float64(i*76), w)), Y: survivalBoss2Head.Y}
		data, sw, sh := survivalBoss2Body1, int32(162), int32(73)
		if i%3 == 1 {
			data, sw, sh = survivalBoss2Body2, 155, 75
		} else if i%3 == 2 {
			data, sw, sh = survivalBoss2Body3, 145, 81
		}
		drawBoss2RotatedBGRA(hdc, data, sw, sh, p, sx(116, w), sy(67, h), 0)
	}
}

func drawBoss2EnergyOrb(hdc uintptr, b SurvivalBoss2EnergyBall, w, h int32, now time.Time) {
	// v390: deliberately oversized, unstable plasma wake. Multiple independently
	// displaced filaments prevent the trail reading as one clean neon rope.
	for j := 1; j < len(b.Trail); j++ {
		a, c := b.Trail[j-1], b.Trail[j]
		age := now.Sub(c.At).Seconds()
		if age < 0 || age > .72 {
			continue
		}
		fade := 1.0 - age/.72
		if fade < .06 {
			continue
		}
		seed := float64(b.Row*37 + j*11)
		phase := float64(now.UnixMilli())*.031 + seed
		amp := float64(sy(12, h)) * (0.55 + 0.45*fade)
		j1 := math.Sin(phase*1.17) * amp
		j2 := math.Cos(phase*1.73+1.4) * amp * .82
		ax, ay := int32(a.P.X), int32(a.P.Y+j1)
		cx, cy := int32(c.P.X), int32(c.P.Y+j2)

		// Wide bruised-purple aura plus a violently bright central plasma channel.
		drawLineSimple(hdc, ax, ay, cx, cy, int(max32(3, sx(42, w))), rgb(41, 7, 76))
		drawLineSimple(hdc, ax, ay, cx, cy, int(max32(3, sx(28, w))), rgb(92, 20, 180))
		drawLineSimple(hdc, ax, ay, cx, cy, int(max32(2, sx(17, w))), rgb(190, 46, 255))
		drawLineSimple(hdc, ax, ay, cx, cy, int(max32(1, sx(7, w))), rgb(255, 202, 255))
		drawLineSimple(hdc, ax, ay, cx, cy, int(max32(1, sx(3, w))), rgb(255, 255, 255))

		// Forks shoot both above and below the wake with uneven lengths and directions.
		if fade > .16 {
			mx, my := (ax+cx)/2, (ay+cy)/2
			for k := 0; k < 2; k++ {
				sign := float64(1)
				if k == 1 {
					sign = -1
				}
				lenX := sx(15+float64((j+k)%4)*7, w)
				lenY := sy(18+float64((j*3+k)%5)*8, h)
				kick := math.Sin(phase*2.11 + float64(k)*2.7)
				ex := mx - lenX
				ey := my + int32(sign*float64(lenY)*(0.58+0.42*math.Abs(kick)))
				bendX := mx - lenX/2 + int32(math.Cos(phase*1.9+float64(k))*float64(sx(8, w)))
				bendY := (my+ey)/2 + int32(math.Sin(phase*2.6+float64(k))*float64(sy(10, h)))
				drawLineSimple(hdc, mx, my, bendX, bendY, int(max32(1, sx(5, w))), rgb(126, 28, 229))
				drawLineSimple(hdc, bendX, bendY, ex, ey, int(max32(1, sx(2, w))), rgb(247, 168, 255))
			}
		}
	}
	if !b.Active {
		return
	}
	cx, cy := int32(b.P.X), int32(b.P.Y)
	pulse := 0.5 + 0.5*math.Sin(float64(now.UnixMilli())*.053+float64(b.Row)*1.7)
	for i := 5; i >= 1; i-- {
		rad := sx(float64(20+i*8)*(0.91+.14*pulse), w)
		cols := []uintptr{rgb(48, 9, 92), rgb(89, 18, 176), rgb(144, 30, 238), rgb(211, 71, 255), rgb(251, 191, 255)}
		drawCircleOutline(hdc, cx, cy, rad, max32(1, sx(float64(2+i), w)), cols[i-1])
	}
	brush, _, _ := createSolidBrush.Call(rgb(255, 240, 255))
	pen, _, _ := createPen.Call(PS_SOLID, 1, rgb(230, 115, 255))
	oldBrush, _, _ := selectObject.Call(hdc, brush)
	oldPen, _, _ := selectObject.Call(hdc, pen)
	rr := sx(15, w)
	ellipse.Call(hdc, uintptr(cx-rr), uintptr(cy-rr), uintptr(cx+rr), uintptr(cy+rr))
	selectObject.Call(hdc, oldPen)
	selectObject.Call(hdc, oldBrush)
	deleteObject.Call(pen)
	deleteObject.Call(brush)
	for k := 0; k < 11; k++ {
		ang := float64(k)*math.Pi*2/11 + float64(now.UnixMilli())*.009
		inner := float64(sx(13, w))
		outer := float64(sx(43+float64((k%4)*9), w))
		bend := math.Sin(float64(now.UnixMilli())*.039+float64(k*17)) * float64(sy(16, h))
		x1 := cx + int32(math.Cos(ang)*inner)
		y1 := cy + int32(math.Sin(ang)*inner)
		x2 := cx + int32(math.Cos(ang+.28)*outer)
		y2 := cy + int32(math.Sin(ang+.28)*outer+bend)
		midx := (x1+x2)/2 + int32(math.Sin(ang*4+float64(now.UnixMilli())*.031)*float64(sx(11, w)))
		midy := (y1+y2)/2 - int32(math.Cos(ang*3+float64(now.UnixMilli())*.027)*float64(sy(13, h)))
		drawLineSimple(hdc, x1, y1, midx, midy, int(max32(1, sx(3, w))), rgb(249, 188, 255))
		drawLineSimple(hdc, midx, midy, x2, y2, int(max32(1, sx(2, w))), rgb(177, 48, 255))
	}
}

func drawSurvivalBoss2EnergyVolley(hdc uintptr, w, h int32, now time.Time) {
	for _, b := range survivalBoss2EnergyBalls {
		drawBoss2EnergyOrb(hdc, b, w, h, now)
	}
}

func drawSurvivalBoss2EnergyLaneGuides(hdc uintptr, w, h int32, now time.Time) {
	if survivalBoss2Stage != survivalBoss2BeamCharge && survivalBoss2Stage != survivalBoss2BeamSetup {
		return
	}
	r := arenaRect(w, h)
	pulse := 0.5 + 0.5*math.Sin(float64(now.UnixMilli())*.014)
	for row := 0; row < 6; row++ {
		rr := survivalBoss2BeamRowRect(row, w, h)
		y := (rr.Top + rr.Bottom) / 2
		// restrained lane telegraph: dotted purple/cyan energy marks, not a solid wall.
		for x := r.Left + sx(16, w); x < r.Right-sx(115, w); x += sx(34, w) {
			lenx := sx(13, w)
			c := rgb(126, 63, 255)
			if (row+int(x))%2 == 0 {
				c = rgb(52, 202, 255)
			}
			alpha := byte(28 + int(34*pulse))
			alphaSolidRect(hdc, RECT{Left: x, Top: y - sy(1, h), Right: x + lenx, Bottom: y + sy(1, h) + 1}, c, alpha)
		}
	}
}

func drawSurvivalBoss2NodeTimerBar(hdc uintptr, p FPoint, remain float64, active bool, completed bool, w, h int32) {
	remain = boss2Clamp(remain, 0, 1)
	bw, bh := sx(78, w), max32(4, sy(7, h))
	left := int32(p.X) - bw/2
	top := int32(p.Y) - sy(48, h)
	bar := RECT{Left: left, Top: top, Right: left + bw, Bottom: top + bh}
	// Dark backing + white keyline keeps every timer readable over the moving space
	// background. Future nodes have a dormant full bar; completed nodes stay visibly
	// cleared instead of disappearing and making the sequence hard to read.
	fillSolidRect(hdc, bar, rgb(13, 12, 24))
	fill := bar
	if completed {
		fill.Right = fill.Left + bw
		fillSolidRect(hdc, fill, rgb(75, 226, 244))
	} else if active {
		fill.Right = fill.Left + int32(float64(bw)*remain)
		c := rgb(224, 64, 255)
		if remain < .34 {
			c = rgb(255, 74, 74)
		}
		fillSolidRect(hdc, fill, c)
	} else {
		fill.Right = fill.Left + bw
		fillSolidRect(hdc, fill, rgb(62, 54, 83))
	}
	drawOutlineRect(hdc, bar, rgb(238, 232, 255), 1)
}

func drawSurvivalBoss2BeamChain(hdc uintptr, w, h int32, now time.Time) {
	if survivalBoss2Stage != survivalBoss2BeamNode || len(survivalBoss2BeamChainNodes) == 0 {
		return
	}
	mouth := survivalBoss2BeamNodePoint(w, h)
	points := make([]FPoint, 0, len(survivalBoss2BeamChainNodes)+1)
	for i := range survivalBoss2BeamChainNodes {
		points = append(points, survivalBoss2BeamChainNodePoint(i, w, h, now))
	}
	points = append(points, mouth)

	// Ordered flight path: a restrained underglow plus bright completed sections makes
	// the rapid 1 > 2 > ... > 10 > mouth route instantly legible.
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		completedSegment := i <= survivalBoss2BeamChainIndex
		c := rgb(94, 57, 126)
		if completedSegment {
			c = rgb(77, 223, 245)
		}
		drawLineSimple(hdc, int32(a.X), int32(a.Y), int32(b.X), int32(b.Y), int(max32(1, sx(5, w))), rgb(20, 11, 35))
		drawLineSimple(hdc, int32(a.X), int32(a.Y), int32(b.X), int32(b.Y), int(max32(1, sx(2, w))), c)
	}

	for i := range survivalBoss2BeamChainNodes {
		p := points[i]
		completed := i < survivalBoss2BeamChainIndex
		active := i == survivalBoss2BeamChainIndex && !survivalBoss2BeamChainComplete()
		remain := 1.0
		if active && !survivalBoss2BeamChainDeadline.IsZero() {
			remain = survivalBoss2BeamChainDeadline.Sub(now).Seconds() / survivalBoss2BeamChainNodeWindow().Seconds()
		}
		drawSurvivalBoss2NodeTimerBar(hdc, p, remain, active, completed, w, h)

		pulse := .5 + .5*math.Sin(float64(now.UnixMilli())*.025+float64(i)*.8)
		outer := sx(34, w)
		if active {
			outer = sx(38+5*pulse, w)
			drawCircleOutline(hdc, int32(p.X), int32(p.Y), outer+sx(7, w), max32(1, sx(3, w)), rgb(214, 74, 255))
		}
		key := rgb(142, 105, 168)
		if active {
			key = rgb(255, 245, 255)
		} else if completed {
			key = rgb(107, 237, 247)
		}
		drawCircleOutline(hdc, int32(p.X), int32(p.Y), outer, max32(1, sx(3, w)), key)
		alpha := byte(110)
		if active {
			alpha = 255
		} else if completed {
			alpha = 145
		}
		if len(survivalBoss2BeamChainNodeArt) >= 97*100*4 {
			drawBoss2RotatedBGRAAlpha(hdc, survivalBoss2BeamChainNodeArt, 97, 100, p, sx(48, w), sy(48, h), 0, alpha)
		}

		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 255, 255))
			label := fmt.Sprintf("%d", i+1)
			centeredTextOut(hdc, int32(p.X)-sx(18, w), int32(p.X)+sx(18, w), int32(p.Y)-sy(7, h), label)
			selectObject.Call(hdc, old)
		}
	}

	// The boss core appears at the SAME moment as node 1, so this timer has already
	// been draining while the relay is cleared. It only becomes clickable after the
	// final flying node is destroyed.
	mouthRemain := 1.0
	if !survivalBoss2BeamNodeDeadline.IsZero() {
		mouthRemain = survivalBoss2BeamNodeDeadline.Sub(now).Seconds() / survivalBoss2BeamMouthWindow().Seconds()
	}
	unlocked := survivalBoss2BeamChainComplete()
	// The mouth timer is active even while the core is locked by the relay.
	drawSurvivalBoss2NodeTimerBar(hdc, mouth, mouthRemain, true, false, w, h)
	pulse := .5 + .5*math.Sin(float64(now.UnixMilli())*.031)
	mouthRad := sx(42+6*pulse, w)
	ringCol := rgb(116, 57, 65)
	alpha := byte(125)
	if unlocked {
		ringCol = rgb(255, 72, 72)
		alpha = 255
	}
	drawCircleOutline(hdc, int32(mouth.X), int32(mouth.Y), mouthRad, max32(1, sx(4, w)), ringCol)
	drawBoss2RotatedBGRAAlpha(hdc, survivalBoss2RedNode, 97, 100, mouth, sx(68, w), sy(68, h), 0, alpha)
	if !unlocked && hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(235, 165, 175))
		centeredTextOut(hdc, int32(mouth.X)-sx(60, w), int32(mouth.X)+sx(60, w), int32(mouth.Y)+sy(39, h), "LOCKED")
		selectObject.Call(hdc, old)
	}
}

func drawSurvivalBoss2BeamArena(hdc uintptr, w, h int32, now time.Time) {
	r := arenaRect(w, h)
	drawBoss2ScrollingBackground(hdc, r, now)
	drawSurvivalParticles(hdc, w, h)
	drawSurvivalBoss2EnergyLaneGuides(hdc, w, h, now)

	parked := !survivalBoss2BeamParkedAt.IsZero() || survivalBoss2Stage != survivalBoss2BeamSetup
	if survivalBoss2Stage == survivalBoss2BeamSetup && !parked {
		drawSurvivalBoss2BeamEntryBody(hdc, w, h)
	} else {
		// The looping body is always behind the energy volley and behind the head.
		drawSurvivalBoss2BeamBody(hdc, w, h, now)
	}

	fade := byte(255)
	if survivalBoss2Stage == survivalBoss2BeamSetup && !survivalBoss2BeamSatelliteAt.IsZero() {
		p := boss2Clamp(now.Sub(survivalBoss2BeamSatelliteAt).Seconds()/.75, 0, 1)
		fade = byte(p * 255)
	} else if survivalBoss2Stage == survivalBoss2BeamRecover {
		p := boss2Clamp(now.Sub(survivalBoss2StageAt).Seconds()/.45, 0, 1)
		fade = byte((1 - p) * 255)
	}
	for _, sat := range survivalBoss2Satellites {
		data, sw, sh := survivalBoss2SatelliteData(sat.Variant)
		drawBoss2RotatedBGRAAlpha(hdc, data, sw, sh, sat.P, sat.W, sat.H, sat.Angle, fade)
	}

	if survivalBoss2Stage == survivalBoss2BeamFire {
		// Six independent electric orbs replace the old full-screen laser. Their
		// historical trails remain visible/lethal briefly after each orb passes.
		drawSurvivalBoss2EnergyVolley(hdc, w, h, now)
	}

	headData := survivalBoss2HeadClosed
	hw, hh := int32(197), int32(125)
	headP := survivalBoss2Head
	// The mouth stays open during the energy volley. The moving body and all orb
	// trails render first; the head is always the final boss layer on top.
	if survivalBoss2Stage == survivalBoss2BeamFire || survivalBoss2Stage == survivalBoss2BeamNode {
		headData = survivalBoss2HeadOpen
		hw, hh = 185, 152
	} else if survivalBoss2Stage == survivalBoss2BeamCharge && (now.UnixMilli()/120)%2 == 0 {
		headData = survivalBoss2HeadPurple
	}
	flashActive := !survivalBoss2BeamHeadFlashUntil.IsZero() && now.Before(survivalBoss2BeamHeadFlashUntil)
	if flashActive {
		// Rapid red/normal strobe plus a small local impact jitter. Only the boss head
		// flashes; the whole screen remains stable and readable.
		phase := (now.UnixMilli() / 55) % 2
		if phase == 0 && len(survivalBoss2HeadClosedRed) > 0 {
			headData = survivalBoss2HeadClosedRed
			hw, hh = 197, 125
		} else {
			headData = survivalBoss2HeadClosed
			hw, hh = 197, 125
		}
		shake := screenShakeScale()
		headP.X += math.Sin(float64(now.UnixMilli())*.19) * float64(sx(5, w)) * shake
		headP.Y += math.Cos(float64(now.UnixMilli())*.23) * float64(sy(4, h)) * shake
		if len(survivalBoss2HeadClosedRed) > 0 {
			drawBoss2RotatedBGRAAlpha(hdc, survivalBoss2HeadClosedRed, 197, 125, FPoint{X: headP.X - float64(sx(4, w)), Y: headP.Y}, sx(194, w), sy(134, h), 0, 80)
			drawBoss2RotatedBGRAAlpha(hdc, survivalBoss2HeadClosedRed, 197, 125, FPoint{X: headP.X + float64(sx(4, w)), Y: headP.Y}, sx(194, w), sy(134, h), 0, 80)
		}
	}
	drawBoss2RotatedBGRA(hdc, headData, hw, hh, headP, sx(188, w), sy(130, h), 0)
	drawSurvivalBoss2BeamChain(hdc, w, h, now)
}

func drawSurvivalBoss2BeamExitArena(hdc uintptr, w, h int32, now time.Time) {
	r := arenaRect(w, h)
	drawBoss2ScrollingBackground(hdc, r, now)
	drawSurvivalParticles(hdc, w, h)
	// Continue the exact vertical body stream from the energy phase, but without
	// modulo wrapping. Each existing piece simply descends and eventually leaves.
	for i := 0; i < 12; i++ {
		q := survivalBoss2BeamExitBodyPoint(i, w, h, now)
		if q.Y < float64(r.Top)-float64(sy(90, h)) || q.Y > float64(r.Bottom)+float64(sy(90, h)) {
			continue
		}
		data, sw, sh := survivalBoss2Body1, int32(162), int32(73)
		if i%3 == 1 {
			data, sw, sh = survivalBoss2Body2, 155, 75
		} else if i%3 == 2 {
			data, sw, sh = survivalBoss2Body3, 145, 81
		}
		drawBoss2RotatedBGRA(hdc, data, sw, sh, q, sx(112, w), sy(64, h), math.Pi/2)
	}
	tail := survivalBoss2BeamExitTailPoint(w, h, now)
	if tail.Y > float64(r.Top)-float64(sy(100, h)) && tail.Y < float64(r.Bottom)+float64(sy(120, h)) {
		drawBoss2RotatedBGRA(hdc, survivalBoss2Tail, 210, 53, tail, sx(172, w), sy(50, h), math.Pi/2)
	}
	if survivalBoss2Head.X < float64(r.Right)+float64(sx(150, w)) {
		drawBoss2RotatedBGRA(hdc, survivalBoss2HeadClosed, 197, 125, survivalBoss2Head, sx(188, w), sy(130, h), 0)
	}
}

func drawSurvivalBoss2FinaleArena(hdc uintptr, w, h int32, now time.Time) {
	r := arenaRect(w, h)
	elapsed := now.Sub(survivalBoss2StageAt)
	if survivalBoss2Stage == survivalBoss2FinalePreBlack {
		drawBoss2ScrollingBackground(hdc, r, now)
		p := math.Min(1, elapsed.Seconds()/.9)
		alphaSolidRect(hdc, r, rgb(0, 0, 0), byte(p*255))
		return
	}
	drawBoss2ScrollingBackground(hdc, r, now)
	drawSurvivalParticles(hdc, w, h)
	data := survivalBoss2FinalHead3
	if survivalBoss2Stage == survivalBoss2FinaleNode && now.Before(survivalBoss2FinaleHeadFlashUntil) && len(survivalBoss2FinalHead3Red) > 0 {
		data = survivalBoss2FinalHead3Red
	}
	sw, sh := int32(521), int32(969)
	dstW := sx(390, w)
	if survivalBoss2Stage == survivalBoss2FinaleHead1 {
		data, sw, sh = survivalBoss2FinalHead1, 516, 797
		dstW = sx(150, w)
	}
	if survivalBoss2Stage == survivalBoss2FinaleHead2 {
		data, sw, sh = survivalBoss2FinalHead2, 494, 833
		dstW = sx(274, w)
	}
	if survivalBoss2Stage == survivalBoss2FinaleHead3 || survivalBoss2Stage == survivalBoss2FinaleNode || survivalBoss2Stage == survivalBoss2FinaleFlash {
		dstW = sx(445, w)
	}
	if len(data) >= int(sw*sh*4) {
		aspect := float64(sh) / float64(sw)
		dstH := int32(float64(dstW) * aspect)
		maxH := (r.Bottom - r.Top) - sy(12, h)
		if dstH > maxH {
			dstH = maxH
			dstW = int32(float64(dstH) / aspect)
		}
		cx, cy := (r.Left+r.Right)/2, (r.Top+r.Bottom)/2
		// Later heads sit progressively lower/larger, creating the feeling that the
		// player is slipping in and out while the serpent closes the distance.
		if survivalBoss2Stage == survivalBoss2FinaleHead1 {
			cy -= sy(18, h)
		}
		if survivalBoss2Stage == survivalBoss2FinaleHead2 {
			cy += sy(4, h)
		}
		if survivalBoss2Stage == survivalBoss2FinaleHead3 || survivalBoss2Stage == survivalBoss2FinaleNode || survivalBoss2Stage == survivalBoss2FinaleFlash {
			cy += sy(28, h)
		}
		drawRawBGRAFit(hdc, data, sw, sh, RECT{cx - dstW/2, cy - dstH/2, cx + dstW/2, cy + dstH/2})
	}
	if survivalBoss2Stage == survivalBoss2FinaleHead1 || survivalBoss2Stage == survivalBoss2FinaleHead2 {
		t := elapsed.Seconds()
		alpha := 0.0
		if t < .7 {
			alpha = 1 - t/.7
		} else if t > 1.6 {
			alpha = (t - 1.6) / .7
		}
		alpha = boss2Clamp(alpha, 0, 1)
		if alpha > 0 {
			alphaSolidRect(hdc, r, rgb(0, 0, 0), byte(alpha*255))
		}
	} else if survivalBoss2Stage == survivalBoss2FinaleHead3 {
		t := elapsed.Seconds()
		if t < .7 {
			alphaSolidRect(hdc, r, rgb(0, 0, 0), byte((1-t/.7)*255))
		}
	}
	if survivalBoss2Stage == survivalBoss2FinaleNode {
		np := survivalBoss2FinaleNodePoint(w, h)
		pulse := .5 + .5*math.Sin(float64(now.UnixMilli())*.035)
		rad := sx(42+8*pulse, w)
		drawSurvivalCircleWithPen(hdc, survivalPen(5, rgb(226, 65, 255)), int32(np.X), int32(np.Y), rad)
		drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(255, 230, 255)), int32(np.X), int32(np.Y), sx(28, w))
		if !survivalBoss2FinaleDeadline.IsZero() {
			remain := boss2Clamp(survivalBoss2FinaleDeadline.Sub(now).Seconds()/5.5, 0, 1)
			bw, bh := sx(260, w), sy(11, h)
			br := RECT{int32(np.X) - bw/2, int32(np.Y) - sy(76, h), int32(np.X) + bw/2, int32(np.Y) - sy(76, h) + bh}
			fillSolidRect(hdc, br, rgb(30, 20, 40))
			fr := br
			fr.Right = fr.Left + int32(float64(bw)*remain)
			fillSolidRect(hdc, fr, rgb(205, 45, 255))
			drawOutlineRect(hdc, br, rgb(255, 255, 255), 1)
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 240, 255))
			centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(22, h), fmt.Sprintf("FINAL CORE // %d / 20", survivalBoss2FinaleHits))
			selectObject.Call(hdc, old)
		}
	}
	if survivalBoss2Stage == survivalBoss2FinaleFlash {
		pulse := .5 + .5*math.Sin(float64(now.UnixMilli())*.095)
		a := byte(90 + int(165*pulse))
		alphaSolidRect(hdc, r, rgb(190, 20, 255), a)
	}
}

func drawSurvivalBoss2Arena(hdc uintptr, w, h int32) {
	r := arenaRect(w, h)
	now := survivalBossFightNow(time.Now(), survivalBoss2StartedAt)
	if survivalBoss2Stage == survivalBoss2BeamExit {
		drawSurvivalBoss2BeamExitArena(hdc, w, h, now)
		return
	}
	if survivalBoss2Stage == survivalBoss2FinalePreBlack || survivalBoss2Stage == survivalBoss2FinaleHead1 || survivalBoss2Stage == survivalBoss2FinaleHead2 || survivalBoss2Stage == survivalBoss2FinaleHead3 || survivalBoss2Stage == survivalBoss2FinaleNode || survivalBoss2Stage == survivalBoss2FinaleFlash {
		drawSurvivalBoss2FinaleArena(hdc, w, h, now)
		return
	}
	if survivalBoss2Stage == survivalBoss2MeteorWarning || survivalBoss2Stage == survivalBoss2MeteorRun {
		drawSurvivalBoss2MeteorPhase(hdc, w, h, now)
		return
	}
	if survivalBoss2Stage == survivalBoss2Dodge {
		drawSurvivalBoss2DodgeArena(hdc, w, h, now)
		return
	}
	if survivalBoss2Stage == survivalBoss2BeamSetup || survivalBoss2Stage == survivalBoss2BeamCharge || survivalBoss2Stage == survivalBoss2BeamFire || survivalBoss2Stage == survivalBoss2BeamNode || survivalBoss2Stage == survivalBoss2BeamRecover {
		drawSurvivalBoss2BeamArena(hdc, w, h, now)
		return
	}
	drawBoss2ScrollingBackground(hdc, r, now)
	drawSurvivalParticles(hdc, w, h)
	drawBoss2ReentryTelegraph(hdc, r, w, h, now)
	nodePresent := survivalBoss2NodeSegment >= 0 && survivalBoss2NodeSafetyActive(now)

	// Draw the COMPLETE serpent at all times, including underneath an active node.
	// Node safety is handled exclusively by collision suppression, never by hiding art.
	// This keeps the boss visually continuous while still creating a genuinely clickable
	// interaction pocket in the serpent contact geometry.
	_ = nodePresent
	for i := survivalBoss2SegmentCount - 1; i >= 0; i-- {
		s := survivalBoss2Segments[i]
		if !s.Valid {
			continue
		}
		// The serpent is deliberately much longer than the arena. Cull only pieces that
		// are well outside the viewport so the extra offscreen length costs virtually no
		// GDI work while still remaining present in the articulated chain.
		marginX := float64(sx(180, w))
		marginY := float64(sy(150, h))
		if s.P.X < float64(r.Left)-marginX || s.P.X > float64(r.Right)+marginX ||
			s.P.Y < float64(r.Top)-marginY || s.P.Y > float64(r.Bottom)+marginY {
			continue
		}
		data := survivalBoss2Body1
		sw, sh := int32(162), int32(73)
		switch i % 3 {
		case 1:
			data = survivalBoss2Body2
			sw, sh = 155, 75
		case 2:
			data = survivalBoss2Body3
			sw, sh = 145, 81
		}
		if survivalBoss2SegmentCollisionDisabled(i, now) {
			// Every collision-disabled Hunt body sprite flashes purple. This intentionally includes
			// the host segment and its immediate neighbours, matching the three-piece safe pocket.
			purple := survivalBoss2Body1Purple
			switch i % 3 {
			case 1:
				purple = survivalBoss2Body2Purple
			case 2:
				purple = survivalBoss2Body3Purple
			}
			drawBoss2RotatedBGRA(hdc, data, sw, sh, s.P, sx(116, w), sy(67, h), s.Angle)
			alpha := byte(150)
			if survivalBoss2PurplePulseBright(now) {
				alpha = 245
			}
			drawBoss2RotatedBGRAAlpha(hdc, purple, sw, sh, s.P, sx(116, w), sy(67, h), s.Angle, alpha)
			continue
		}
		// Source body art is horizontal; angle is the tangent toward the head.
		drawBoss2RotatedBGRA(hdc, data, sw, sh, s.P, sx(116, w), sy(67, h), s.Angle)
	}
	// Tail is rendered on the last valid chain point, flipped to point away from the head.
	for i := survivalBoss2SegmentCount - 1; i >= 0; i-- {
		if s := survivalBoss2Segments[i]; s.Valid {
			marginX := float64(sx(220, w))
			marginY := float64(sy(170, h))
			if s.P.X >= float64(r.Left)-marginX && s.P.X <= float64(r.Right)+marginX &&
				s.P.Y >= float64(r.Top)-marginY && s.P.Y <= float64(r.Bottom)+marginY {
				drawBoss2RotatedBGRA(hdc, survivalBoss2Tail, 210, 53, s.P, sx(172, w), sy(50, h), s.Angle+math.Pi)
			}
			break
		}
	}

	// v411 Hunt presentation: all five cores are visible immediately and remain attached
	// to their exact host body pieces until clicked. No invisible rectangle, sequence
	// pointer, retargeting or expiry can move/despawn them.
	if survivalBoss2ComboTotal > 0 {
		pulse := .5 + .5*math.Sin(float64(now.UnixMilli())*.010)
		for i := 0; i < survivalBoss2ComboTotal && i < len(survivalBoss2HuntNodes); i++ {
			n := survivalBoss2HuntNodes[i]
			if !n.Alive {
				continue
			}
			np, ok := survivalBoss2HuntNodePoint(i)
			if !ok {
				continue
			}
			col := rgb(255, 72, 72)
			if !n.Red {
				col = rgb(70, 185, 255)
			}
			// A restrained halo keeps five simultaneous nodes readable against the animated
			// purple host sections without looking like five unrelated floating objects.
			drawSurvivalCircleWithPen(hdc, survivalPen(2, col), int32(np.X), int32(np.Y), sx(42+4*pulse, w))
			if n.Red {
				drawBoss2RotatedBGRA(hdc, survivalBoss2RedNode, 97, 100, np, sx(68, w), sy(68, h), 0)
			} else {
				drawBoss2RotatedBGRA(hdc, survivalBoss2BlueNode, 92, 94, np, sx(68, w), sy(68, h), 0)
			}

			// v460 Hunt combo feedback: after hits 1-4, a compact red remaining-hit
			// number flashes beside the node and then disappears. The fifth hit destroys
			// the node, so there is intentionally never a visible zero.
			if n.FlashRemaining > 0 && !n.FlashUntil.IsZero() && now.Before(n.FlashUntil) && hudSmallFont != 0 {
				oldFont, _, _ := selectObject.Call(hdc, hudSmallFont)
				setBkMode.Call(hdc, TRANSPARENT)
				remainingText := fmt.Sprintf("%d", n.FlashRemaining)
				left := int32(np.X) + sx(39, w)
				right := left + sx(28, w)
				top := int32(np.Y) - sy(12, h)
				// Dark one-pixel shadow prevents the red digit disappearing against bright body art.
				setTextColor.Call(hdc, rgb(30, 5, 8))
				centeredTextOut(hdc, left+sx(1, w), right+sx(1, w), top+sy(1, h), remainingText)
				flash := (now.UnixMilli()/70)%2 == 0
				if flash {
					setTextColor.Call(hdc, rgb(255, 78, 78))
				} else {
					setTextColor.Call(hdc, rgb(215, 35, 45))
				}
				centeredTextOut(hdc, left, right, top, remainingText)
				selectObject.Call(hdc, oldFont)
			}
		}
	}

	headAngle := math.Atan2(survivalBoss2Vel.Y, survivalBoss2Vel.X) + math.Pi // source head faces left
	headData := survivalBoss2HeadClosed
	hw, hh := int32(197), int32(125)
	if survivalBoss2HeadOpenNow(now) {
		headData = survivalBoss2HeadOpen
		hw, hh = 185, 152
	}
	drawBoss2RotatedBGRA(hdc, headData, hw, hh, survivalBoss2Head, sx(188, w), sy(130, h), headAngle)

	// Boss 1-style fade-in and fade-to-next overlays.
	var fadeAlpha byte
	switch survivalBoss2Stage {
	case survivalBoss2FadeIn:
		p := math.Min(1, now.Sub(survivalBoss2StageAt).Seconds()/.9)
		fadeAlpha = byte((1 - p) * 255)
	case survivalBoss2FadeToNext:
		p := math.Min(1, now.Sub(survivalBoss2StageAt).Seconds()/1.0)
		fadeAlpha = byte(p * 255)
	}
	if fadeAlpha > 0 {
		alphaSolidRect(hdc, r, rgb(0, 0, 0), fadeAlpha)
	}
}

func drawSurvivalBoss2TransitionOverlay(hdc uintptr, w, h int32) {
	if survivalBoss2Stage != survivalBoss2FadeOut && survivalBoss2Stage != survivalBoss2ReturnFadeIn {
		return
	}
	r := arenaRect(w, h)
	now := survivalBossFightNow(time.Now(), survivalBoss2StartedAt)
	p := math.Min(1, now.Sub(survivalBoss2StageAt).Seconds()/1.0)
	alpha := byte(p * 255)
	if survivalBoss2Stage == survivalBoss2ReturnFadeIn {
		alpha = byte((1 - p) * 255)
	}
	if alpha > 0 {
		alphaSolidRect(hdc, r, rgb(0, 0, 0), alpha)
	}
}
