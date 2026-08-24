package main

import (
	"fmt"
	"math"
	"time"
)

// v202 polish foundation ----------------------------------------------------
// Centralized easing, pacing and lightweight VFX. These systems deliberately
// avoid allocations in the hot render path.

func easeSmoothStep(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func easeOutCubic(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	x := 1 - t
	return 1 - x*x*x
}

type encounterPhase int

const (
	encounterRecovery encounterPhase = iota
	encounterBuild
	encounterPressure
	encounterPeak
)

func enduranceEncounterPhase(distance float64) encounterPhase {
	if distance < 120 {
		return encounterRecovery
	}
	// A 420m repeating pacing cycle: recovery -> build -> pressure -> peak.
	p := math.Mod(distance-120, 420)
	switch {
	case p < 80:
		return encounterRecovery
	case p < 210:
		return encounterBuild
	case p < 340:
		return encounterPressure
	default:
		return encounterPeak
	}
}

func enduranceEncounterIntensity(distance float64) float64 {
	switch enduranceEncounterPhase(distance) {
	case encounterRecovery:
		return 0.72
	case encounterBuild:
		return 0.92
	case encounterPressure:
		return 1.08
	case encounterPeak:
		return 1.22
	default:
		return 1
	}
}

func enduranceEncounterLabel(distance float64) string {
	switch enduranceEncounterPhase(distance) {
	case encounterRecovery:
		return "RECOVERY"
	case encounterBuild:
		return "BUILD"
	case encounterPressure:
		return "PRESSURE"
	case encounterPeak:
		return "PEAK"
	default:
		return "NORMAL"
	}
}

func enduranceWorldDepth(distance float64) float64 {
	// Visual depth only. It never changes collision or scroll speed.
	x := distance / 5000.0
	if x < 0 {
		x = 0
	}
	if x > 1 {
		x = 1
	}
	return easeSmoothStep(x)
}

type polishVFX struct {
	X, Y  float32
	Start time.Time
	Life  float64
	Kind  int
}

const (
	polishVFXHit = iota
	polishVFXPickup
	polishVFXBoss
)

var polishEffects []polishVFX

func addPolishVFX(kind int, x, y float32, life float64) {
	if life <= 0 {
		return
	}
	polishEffects = append(polishEffects, polishVFX{X: x, Y: y, Start: time.Now(), Life: life, Kind: kind})
	if len(polishEffects) > 24 {
		copy(polishEffects, polishEffects[len(polishEffects)-24:])
		polishEffects = polishEffects[:24]
	}
}

func resetPolishVFX() {
	if cap(polishEffects) < 24 {
		polishEffects = make([]polishVFX, 0, 24)
		return
	}
	polishEffects = polishEffects[:0]
}

func d2dDrawPolishVFX(ar RECT) {
	if len(polishEffects) == 0 {
		return
	}
	now := time.Now()
	kept := polishEffects[:0]
	for _, fx := range polishEffects {
		age := now.Sub(fx.Start).Seconds()
		if age < 0 || age >= fx.Life {
			continue
		}
		kept = append(kept, fx)
		p := age / fx.Life
		r := float32(8 + 28*easeOutCubic(p))
		switch fx.Kind {
		case polishVFXPickup:
			d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{fx.X, fx.Y}, RadiusX: r, RadiusY: r}, d2dPowerupLightBrush, 2.0)
			for i := 0; i < 4; i++ {
				a := float64(i)*math.Pi/2 + p*1.5
				sx := fx.X + float32(math.Cos(a))*r
				sy := fx.Y + float32(math.Sin(a))*r
				d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{sx, sy}, RadiusX: 1.8, RadiusY: 1.8}, d2dPowerupLightBrush)
			}
		case polishVFXBoss:
			d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{fx.X, fx.Y}, RadiusX: r * 1.5, RadiusY: r * 1.5}, d2dAlienLaserCoreBrush, 2.2)
		default:
			d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{fx.X, fx.Y}, RadiusX: r, RadiusY: r}, d2dExplosionHotBrush, 1.8)
		}
	}
	polishEffects = kept
}

var polishLastMusicDuck = -1.0

func desiredMusicDuck() float64 {
	// Spoken Operator dialogue gets priority over the Starbase music. This is
	// intentionally stronger than the normal critical-SFX duck so every quip is
	// intelligible, then applyAudioVolumes restores the user's exact music level.
	if operatorVoiceDuckActive() {
		return 0.26
	}
	if criticalAudioDuckActive() {
		return 0.70
	}
	if survivalActive() && state == StatePlaying {
		if survivalBoss1Active() || survivalBoss2Active() || survivalBoss3Active() {
			return 0.82
		}
		return 1.0
	}
	if !enduranceActive() || state != StatePlaying {
		return 1.0
	}
	if enduranceAlienBossState == alienBossWarning {
		return 0.72
	}
	if enduranceAlienBossState >= alienBossAim1 && enduranceAlienBossState <= alienBossLaser4Retract {
		return 0.62
	}
	if enduranceInWarpTransition() {
		return 0.80
	}
	return 1.0
}

func syncPolishAudioMix() {
	d := desiredMusicDuck()
	if polishLastMusicDuck >= 0 && math.Abs(d-polishLastMusicDuck) < 0.01 {
		return
	}
	polishLastMusicDuck = d
	applyAudioVolumes()
}

func resetPolishSystems() {
	resetPolishVFX()
	polishLastMusicDuck = -1
}

var polishResultStarted time.Time
var polishEnduranceNewPB bool

func beginPolishResult(newPB bool) {
	polishResultStarted = time.Now()
	polishEnduranceNewPB = newPB
}

func polishResultProgress() float64 {
	if polishResultStarted.IsZero() {
		return 1
	}
	return easeOutCubic(time.Since(polishResultStarted).Seconds() / 0.28)
}

func clearPolishResult() {
	polishResultStarted = time.Time{}
	polishEnduranceNewPB = false
}

// v203 presentation + recovery foundation.
var polishFailureStarted time.Time
var endurancePostEncounterRecoveryUntil float64

func beginPolishFailure() {
	// Immediate, deterministic presentation. The animated fail reveal required
	// repeated full-window software repaints exactly while D2D/gameplay was
	// shutting down, which caused the v212 hitch/jump.
	polishFailureStarted = time.Now().Add(-250 * time.Millisecond)
}

func polishFailureProgress() float64 {
	if polishFailureStarted.IsZero() {
		return 1
	}
	return easeOutCubic(time.Since(polishFailureStarted).Seconds() / 0.18)
}

func polishFailureReasonVisible() bool {
	return polishFailureStarted.IsZero() || time.Since(polishFailureStarted) >= 70*time.Millisecond
}

func modalTransitionProgress() float64 {
	if gameMeta.ReducedMotion || uiTransitionStart.IsZero() {
		return 1
	}
	return easeOutCubic(time.Since(uiTransitionStart).Seconds() / 0.22)
}

func beginEndurancePostEncounterRecovery(meters float64) {
	if meters < 0 {
		meters = 0
	}
	endurancePostEncounterRecoveryUntil = enduranceProgressDistance() + meters
}

func endurancePostEncounterRecoveryActive() bool {
	return endurancePostEncounterRecoveryUntil > 0 && enduranceProgressDistance() < endurancePostEncounterRecoveryUntil
}

func encounterDirectorMultiplier(distance float64) float64 {
	if endurancePostEncounterRecoveryUntil > 0 && distance < endurancePostEncounterRecoveryUntil {
		return 0.62
	}
	return enduranceEncounterIntensity(distance)
}

// v203 input feedback + sector presentation.
var polishClickPoint FPoint
var polishClickUntil time.Time
var polishLastSector int

func recordPolishClick(p FPoint) {
	polishClickPoint = p
	polishClickUntil = time.Now().Add(110 * time.Millisecond)
}

func polishButtonPressed(r RECT) bool {
	return !polishClickUntil.IsZero() && time.Now().Before(polishClickUntil) && pointInRect(polishClickPoint, r)
}

func enduranceSectorIndex(distance float64) int {
	if distance < 0 {
		distance = 0
	}
	return int(distance/2500.0) + 1
}

func enduranceSectorName(index int) string {
	switch index {
	case 1:
		return "INNER SPACE"
	case 2:
		return "DEEP SPACE"
	case 3:
		return "NEBULA FRONTIER"
	case 4:
		return "OUTER REACH"
	default:
		return "UNKNOWN SECTOR"
	}
}

func updateEnduranceSectorPresentation() {
	if !enduranceActive() || state != StatePlaying {
		return
	}
	sector := enduranceSectorIndex(enduranceProgressDistance())
	if polishLastSector == 0 {
		polishLastSector = sector
		return
	}
	if sector != polishLastSector {
		polishLastSector = sector
		showPriorityLiveAnnouncement("SECTOR " + fmt.Sprint(sector) + " // " + enduranceSectorName(sector))
	}
}

func d2dDrawDeepSpaceAtmosphere(ar RECT) {
	if !enduranceActive() {
		return
	}
	depth := enduranceWorldDepth(enduranceProgressDistance())
	if depth < 0.18 {
		return
	}
	w := float32(ar.Right - ar.Left)
	h := float32(ar.Bottom - ar.Top)
	pulse := float32(0.5 + 0.5*math.Sin(enduranceAmbientClockNow()*0.75))
	edge := float32(1.0+3.0*depth) + 1.5*pulse
	d2dFillRect(D2D1RectF{Left: 0, Top: 0, Right: w, Bottom: edge}, d2dParticleBrushes[1])
	d2dFillRect(D2D1RectF{Left: 0, Top: h - edge, Right: w, Bottom: h}, d2dParticleBrushes[0])
	if depth > 0.62 {
		y := h * float32(0.18+0.10*pulse)
		d2dFillRect(D2D1RectF{Left: 0, Top: y, Right: w, Bottom: y + 1.5}, d2dParticleBrushes[2])
	}
}

func drawOverlayTransitionAccent(hdc uintptr, w, hgt int32) {
	if overlayMode == OverlayNone || overlayMode == OverlayDeveloperConsole || overlayMode == OverlaySpaceCache || gameMeta.ReducedMotion {
		return
	}
	p := modalTransitionProgress()
	if p >= 1 {
		return
	}
	y := sy(120, hgt) + int32(float64(sy(720, hgt))*p)
	margin := sx(90, w)
	c := blendColor(rgb(42, 211, 255), rgb(255, 188, 58), p*0.30)
	drawLineSimple(hdc, margin, y, w-margin, y, 1, c)
}

// gameplayCriticalFocusActive is presentation-only. It lets non-essential UI
// back off during mechanics that require rapid spatial attention without hiding
// gameplay-critical HUD state.
func gameplayCriticalFocusActive() bool {
	if state != StatePlaying {
		return false
	}
	if survivalActive() {
		return survivalBoss1Active() || survivalBoss2Active() || survivalBoss3Active()
	}
	if enduranceActive() {
		return enduranceInWarpTransition() || enduranceAlienBossState >= alienBossWarning
	}
	return false
}
