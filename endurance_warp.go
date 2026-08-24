//go:build windows

package main

import (
	"fmt"
	"math"
	"time"
)

func enduranceActive() bool { return gameMode == 1 }

// enduranceTrackDistance is the physical distance travelled along the pre-generated rail.
// Warp geometry is generated against this physical value so bonus-distance rewards cannot desynchronise
// the pre-generated warp corridors from the path itself.
func enduranceTrackDistance() float64 { return enduranceCameraX / 10.0 }

func enduranceProgressDistance() float64 {
	// Gameplay pacing, world difficulty, encounters and visual depth must follow
	// the player's physical position on the rail only. +100m pickups affect the
	// displayed/rewarded distance, never the camera or current world state.
	return enduranceTrackDistance()
}

const (
	enduranceWarpFirstMeters       = 1000.0
	enduranceWarpMinGapMeters      = 900.0
	enduranceWarpMaxGapMeters      = 1100.0
	enduranceWarpLengthMeters      = 120.0
	enduranceWarpCueSeconds        = 2.95 // new Ready GO! cue: launch begins as GO starts
	enduranceWarpSpeedMultiplier   = 3.25 // short, punchy warp burst rather than a long empty corridor
	enduranceWarpApproachMeters    = 75.0
	enduranceWarpRecoveryMeters    = 28.0
	enduranceWarpStraightMeters    = 155.0 // cue reaches portal, 120m boost, short physical recovery
	enduranceWarpMarkerShowMeters  = 145.0
	enduranceWarpTargetGraceMeters = 38.0 // empty boosted rail after portal before target 1

	enduranceAlienPhaseStartMeters = 1180.0
	enduranceAlienAimSeconds       = 1.55
	enduranceAlienLaserExtendSecs  = 0.28
	enduranceAlienLaserHoldSecs    = 1.0
	enduranceAlienLaserRetractSecs = 0.30
	enduranceAlienBossEntrySecs    = 1.35
	enduranceAlienBossWidth        = 220.0
	enduranceAlienBossHeight       = 127.0
)

// Warp corridors are pre-generated, but their spacing is intentionally organic.
// The first event is at 1000m and later events are 900-1100m apart.  A deterministic
// sequence keeps the runtime trigger and pre-generated rail in perfect agreement.
func enduranceWarpGapForIndex(index int) float64 {
	if index <= 0 {
		return enduranceWarpFirstMeters
	}
	x := uint32(index)*1664525 + 1013904223
	span := uint32(enduranceWarpMaxGapMeters-enduranceWarpMinGapMeters) + 1
	return enduranceWarpMinGapMeters + float64((x>>8)%span)
}

func ensureEnduranceWarpScheduleThrough(distance float64) {
	if len(enduranceWarpCheckpoints) == 0 {
		enduranceWarpCheckpoints = []float64{enduranceWarpFirstMeters}
	}
	for enduranceWarpCheckpoints[len(enduranceWarpCheckpoints)-1] < distance {
		idx := len(enduranceWarpCheckpoints)
		next := enduranceWarpCheckpoints[len(enduranceWarpCheckpoints)-1] + enduranceWarpGapForIndex(idx)
		enduranceWarpCheckpoints = append(enduranceWarpCheckpoints, next)
	}
}

func nextEnduranceWarpCheckpointAfter(distance float64) float64 {
	ensureEnduranceWarpScheduleThrough(distance + enduranceWarpMaxGapMeters + enduranceWarpStraightMeters)
	for _, cp := range enduranceWarpCheckpoints {
		if cp > distance {
			return cp
		}
	}
	return enduranceWarpCheckpoints[len(enduranceWarpCheckpoints)-1]
}

func enduranceWarpGeometryAt(distance float64) (float64, bool, bool) {
	ensureEnduranceWarpScheduleThrough(distance + enduranceWarpStraightMeters)
	for _, cp := range enduranceWarpCheckpoints {
		if distance < cp-enduranceWarpApproachMeters {
			break
		}
		if distance >= cp-enduranceWarpApproachMeters && distance < cp {
			return cp, true, false
		}
		if distance >= cp && distance < cp+enduranceWarpStraightMeters {
			return cp, false, true
		}
	}
	return 0, false, false
}

func enduranceWarpSegmentAt(distance float64) bool {
	_, _, straight := enduranceWarpGeometryAt(distance)
	return straight
}

func enduranceInWarpTransition() bool {
	return enduranceWarpCueActive || enduranceWarpActive
}

func endurancePlayerEnteringWarpPortal() bool {
	if enduranceNextWarpAt <= 0 || mainHwnd == 0 {
		return false
	}
	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)
	track := enduranceTrackDistance()
	ahead := enduranceNextWarpAt - track
	portalCenterLocalX := 42.0 + ahead*10.0

	// Start READY early enough that the portal physically travels to the rocket
	// while the new cue plays. GO therefore lands on the instant the rocket starts
	// entering the portal instead of after the portal has already flown past.
	const portalHalfWidth = 64.0
	rocketLocalX := cursorPos.X - float64(ar.Left)
	predictedCueTravelPixels := enduranceScrollSpeed() * enduranceWarpCueSeconds
	return portalCenterLocalX-portalHalfWidth <= rocketLocalX+predictedCueTravelPixels
}

func beginEnduranceWarpCue() {
	if enduranceWarpCueActive || enduranceWarpActive {
		return
	}
	enduranceWarpCueActive = true
	enduranceWarpCueStarted = time.Now()
	enduranceWarpCheckpoint = enduranceNextWarpAt
	if enduranceWarpCheckpoint <= 0 {
		enduranceWarpCheckpoint = enduranceWarpFirstMeters
	}
	enduranceWarpRecoveryUntil = 0
	enduranceWarpTargetsSpawned = false
	enduranceWarpAmbientReturnAt = time.Time{}
	enduranceWarpAmbientReturnFrom = 1.0
	enduranceBlocks = enduranceBlocks[:0]
	enduranceBlockSpawnTime = time.Time{}
	enduranceAlienMinions = enduranceAlienMinions[:0]
	enduranceAlienMinionSpawnTime = time.Time{}
	enduranceTargetExplosions = nil
	targets = targets[:0]
	enduranceNextTargetAt = math.MaxFloat64
	status = "WARP ZONE — READY..."
	playWarpReadySound()
}

func updateEnduranceWarpState() {
	if enduranceWarpCueActive {
		// READY/GO never pauses simulation. The course continues at normal speed.
		if time.Since(enduranceWarpCueStarted).Seconds() >= enduranceWarpCueSeconds {
			enduranceWarpCueActive = false
			enduranceWarpActive = true
			enduranceWarpStartDistance = enduranceTrackDistance()
			enduranceWarpRecoveryUntil = enduranceWarpStartDistance + enduranceWarpLengthMeters + enduranceWarpRecoveryMeters
			enduranceWarpTargetsSpawned = false
			enduranceWarpAmbientReturnAt = time.Time{}
			enduranceWarpAmbientReturnFrom = 1.0
			enduranceBlocks = enduranceBlocks[:0]
			enduranceBlockSpawnTime = time.Time{}
			enduranceNextTargetAt = math.MaxFloat64
			status = "WARP BOOST — 120M"
			playWarpRocketSound()
		}
		return
	}
	if enduranceWarpActive && enduranceTrackDistance()-enduranceWarpStartDistance >= enduranceWarpLengthMeters {
		if enduranceWarpAmbientReturnAt.IsZero() {
			startEnduranceAmbientReturn()
		}
		enduranceWarpActive = false
		gameMeta.EnduranceWarpsCompleted++
		evaluateEnduranceAchievements()
		saveGameMeta()
		enduranceNextWarpAt = nextEnduranceWarpCheckpointAfter(enduranceWarpCheckpoint + 1.0)
		enduranceBlockSpawnTime = time.Time{}
		enduranceNextTargetAt = enduranceWarpRecoveryUntil + enduranceTargetGap()
		status = "WARP RECOVERY — HOLD THE LINE"
		stopWarpSounds()
	}
	if !enduranceWarpCueActive && !enduranceWarpActive && enduranceWarpRecoveryUntil > 0 && enduranceTrackDistance() >= enduranceWarpRecoveryUntil {
		enduranceWarpRecoveryUntil = 0
		enduranceWarpCheckpoint = 0
		status = "ENDURANCE — SURVIVE AS LONG AS POSSIBLE"
	}
}

func enduranceWarpSafetyActive() bool {
	track := enduranceTrackDistance()
	approaching := enduranceNextWarpAt > 0 && track >= enduranceNextWarpAt-enduranceWarpApproachMeters && track < enduranceNextWarpAt
	return approaching || enduranceWarpCueActive || enduranceWarpActive || (enduranceWarpRecoveryUntil > 0 && track < enduranceWarpRecoveryUntil)
}

// Legacy rail-enforcement fallback. The primary warp geometry is pre-generated from the
// deterministic checkpoint schedule; this remains available only as a safety helper.
func enforceEnduranceWarpRail(w, hgt int32) {
	if len(path) == 0 {
		return
	}
	ar := arenaRect(w, hgt)
	centerY := float64(ar.Top+ar.Bottom) / 2.0
	checkpoint := enduranceWarpCheckpoint
	if checkpoint <= 0 {
		checkpoint = enduranceNextWarpAt
	}
	if checkpoint <= 0 {
		return
	}
	approachStart := checkpoint - enduranceWarpApproachMeters
	straightEnd := checkpoint + enduranceWarpStraightMeters
	if enduranceWarpRecoveryUntil > straightEnd {
		straightEnd = enduranceWarpRecoveryUntil
	}
	if enduranceDistance < approachStart-20 || enduranceDistance > straightEnd+20 {
		return
	}

	changed := false
	for i := range path {
		screenX := path[i].X - enduranceCameraX
		if screenX < float64(ar.Left)-80 || screenX > float64(ar.Right)+1800 {
			continue
		}
		// Estimate the displayed distance when this point reaches the player's x position.
		projectedDistance := enduranceDistance + (screenX-cursorPos.X)/10.0
		if projectedDistance >= checkpoint && projectedDistance <= straightEnd {
			if math.Abs(path[i].Y-centerY) > 0.01 {
				path[i].Y = centerY
				changed = true
			}
			continue
		}
		if projectedDistance >= approachStart && projectedDistance < checkpoint {
			t := (projectedDistance - approachStart) / enduranceWarpApproachMeters
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			ease := t * t * (3.0 - 2.0*t)
			ny := path[i].Y + (centerY-path[i].Y)*ease
			if math.Abs(ny-path[i].Y) > 0.01 {
				path[i].Y = ny
				changed = true
			}
		}
	}
	if changed {
		enduranceRailDirty = true
		markMergedEnduranceRailDirty()
	}
}

func spawnEnduranceWarpTargets() {
	if !enduranceWarpActive || enduranceWarpTargetsSpawned || len(path) < 2 {
		return
	}

	// Give the player a genuine acclimatisation window after the portal.
	// Warp speed ramps for the first ~12m, then the player gets another stretch
	// of empty straight rail to judge the new speed before target 1 arrives.
	//
	// Targets are anchored to the physical Warp start rather than "wherever the
	// camera happens to be" on the spawn frame. This makes their placement fully
	// deterministic and prevents a frame hitch from moving target 1 closer.
	// Keep every Warp target on an exact 18m rhythm so players can settle into
	// the boosted speed and anticipate the next click instead of reacting to
	// uneven spacing.
	const warpTargetSpacingMeters = 18.0
	offsets := []float64{
		enduranceWarpTargetGraceMeters,
		enduranceWarpTargetGraceMeters + warpTargetSpacingMeters,
		enduranceWarpTargetGraceMeters + warpTargetSpacingMeters*2,
		enduranceWarpTargetGraceMeters + warpTargetSpacingMeters*3,
		enduranceWarpTargetGraceMeters + warpTargetSpacingMeters*4,
	}
	baseWorldX := enduranceWarpStartDistance*10.0 + cursorPos.X
	for _, metersFromWarpStart := range offsets {
		wantX := baseWorldX + metersFromWarpStart*10.0
		idx := 0
		best := math.MaxFloat64
		for i := range path {
			d := math.Abs(path[i].X - wantX)
			if d < best {
				best = d
				idx = i
			}
		}
		enduranceTargetSerial++
		targets = append(targets, Target{
			Point: path[idx], Index: idx, MinIndex: idx, MaxIndex: idx, Clicked: false,
		})
	}
	enduranceWarpTargetsSpawned = true
	enduranceNextTargetAt = math.MaxFloat64
}

func enduranceDifficultyBlend(distance float64) float64 {
	if distance <= 80 {
		return distance / 80.0
	}
	if distance <= 180 {
		return 1.0 + (distance-80.0)/100.0
	}
	return 2.0
}

func enduranceTurnStrengthAt(distance float64) float64 {
	b := enduranceDifficultyBlend(distance)
	if b <= 1.0 {
		return 10.0 + (19.0-10.0)*b
	}
	return 19.0 + (31.0-19.0)*(b-1.0)
}

func enduranceTrackWidth() float64 {
	// v446: slightly thicker Endurance rail for clearer readability.
	return 30.0
}

func endurancePathCenterBounds(ar RECT) (float64, float64) {
	// The widest visible Endurance rail stroke is now 56px (28px per side).
	// Keep a 55px centerline margin so the COMPLETE rendered rail, glow included,
	// always remains inside the game area and never enters the HUD.
	const minimumCenterMargin = 55.0
	visualHalfWidth := (enduranceTrackWidth() + 26.0) / 2.0
	margin := math.Max(minimumCenterMargin, visualHalfWidth+10.0)
	top := float64(ar.Top) + margin
	bottom := float64(ar.Bottom) - margin
	if bottom < top {
		mid := float64(ar.Top+ar.Bottom) * 0.5
		return mid, mid
	}
	return top, bottom
}

func clampEndurancePathToArena(w, hgt int32) bool {
	if len(path) == 0 {
		return false
	}
	ar := arenaRect(w, hgt)
	top, bottom := endurancePathCenterBounds(ar)
	changed := false
	for i := range path {
		if path[i].Y < top {
			path[i].Y = top
			changed = true
		} else if path[i].Y > bottom {
			path[i].Y = bottom
			changed = true
		}
	}
	if changed {
		for i := range targets {
			idx := targets[i].Index
			if idx >= 0 && idx < len(path) {
				targets[i].Point = path[idx]
			}
		}
		enduranceRailDirty = true
		markMergedEnduranceRailDirty()
		logRuntimeEvent("path_bounds_repair", fmt.Sprintf("clamped path top=%.1f bottom=%.1f", top, bottom))
	}
	return changed
}

func endurancePathWithinArena(w, hgt int32) bool {
	if len(path) == 0 {
		return true
	}
	ar := arenaRect(w, hgt)
	top, bottom := endurancePathCenterBounds(ar)
	for _, p := range path {
		if p.Y < top-0.001 || p.Y > bottom+0.001 {
			return false
		}
	}
	return true
}

func enduranceWarpSpeedMultiplierNow() float64 {
	smooth := func(t float64) float64 {
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		return t * t * (3.0 - 2.0*t)
	}

	track := enduranceTrackDistance()
	if enduranceWarpActive {
		progress := track - enduranceWarpStartDistance
		// A quick ~12m launch ramp sells acceleration without delaying the event.
		if progress < 12.0 {
			return 1.0 + (enduranceWarpSpeedMultiplier-1.0)*smooth(progress/12.0)
		}
		// Stay at full warp through the final target and for a short beat afterwards.
		if progress <= 100.0 {
			return enduranceWarpSpeedMultiplier
		}
		// The last 20m eases down, but deliberately exits the active phase still
		// carrying momentum. Recovery then completes the return to normal speed.
		t := smooth((progress - 100.0) / 20.0)
		return enduranceWarpSpeedMultiplier - (enduranceWarpSpeedMultiplier-1.45)*t
	}

	if enduranceWarpRecoveryUntil > 0 && enduranceWarpStartDistance > 0 {
		recoveryStart := enduranceWarpStartDistance + enduranceWarpLengthMeters
		if track >= recoveryStart && track < enduranceWarpRecoveryUntil {
			t := smooth((track - recoveryStart) / enduranceWarpRecoveryMeters)
			return 1.45 + (1.0-1.45)*t
		}
	}
	return 1.0
}

// enduranceAmbientWarpBaseMultiplier keeps the scenery tied to the rail's
// acceleration while exaggerating it enough to make warp visually convincing.
func enduranceAmbientWarpBaseMultiplier() float64 {
	rail := enduranceWarpSpeedMultiplierNow()
	span := enduranceWarpSpeedMultiplier - 1.0
	if span <= 0 {
		return 1.0
	}
	t := (rail - 1.0) / span
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return 1.0 + 2.00*t // v175: background peaks at 3.0x normal
}

func startEnduranceAmbientReturn() {
	if !enduranceWarpAmbientReturnAt.IsZero() {
		return
	}
	enduranceWarpAmbientReturnFrom = enduranceAmbientWarpBaseMultiplier()
	if enduranceWarpAmbientReturnFrom < 1.0 {
		enduranceWarpAmbientReturnFrom = 1.0
	}
	enduranceWarpAmbientReturnAt = time.Now()
}

func enduranceAmbientSpeedMultiplierNow() float64 {
	return enduranceAmbientWarpBaseMultiplier()
}

func enduranceParticleSpeedMultiplierNow() float64 {
	rail := enduranceWarpSpeedMultiplierNow()
	span := enduranceWarpSpeedMultiplier - 1.0
	t := 0.0
	if span > 0 {
		t = (rail - 1.0) / span
	}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	// Normal Endurance gradually *looks* faster as the run travels deeper,
	// without secretly increasing collision/gameplay velocity.
	depthBase := 1.0 + 0.38*enduranceWorldDepth(enduranceProgressDistance())
	return depthBase * (1.0 + 2.60*t)
}

func enduranceParticleClockNow() float64 {
	now := time.Now()
	if enduranceParticleLastFrame.IsZero() {
		enduranceParticleLastFrame = now
		return enduranceParticleClock
	}
	if enduranceActive() && (state == StateFailed || state == StateResult || overlayMode == OverlayGarage || overlayMode == OverlaySpaceCache) {
		enduranceParticleLastFrame = now
		return enduranceParticleClock
	}
	dt := now.Sub(enduranceParticleLastFrame).Seconds()
	enduranceParticleLastFrame = now
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	enduranceParticleClock += dt * enduranceParticleSpeedMultiplierNow()
	return enduranceParticleClock
}

// enduranceAmbientClockNow is a cumulative visual clock. Unlike elapsedTime*speed,
// changing speed cannot jump the background/particles to a new position; it only
// changes how far they travel from this frame onward.
func enduranceAmbientClockNow() float64 {
	now := time.Now()
	if enduranceAmbientLastFrame.IsZero() {
		enduranceAmbientLastFrame = now
		return enduranceAmbientClock
	}
	if enduranceActive() && (state == StateFailed || state == StateResult || overlayMode == OverlayGarage || overlayMode == OverlaySpaceCache) {
		enduranceAmbientLastFrame = now
		return enduranceAmbientClock
	}
	dt := now.Sub(enduranceAmbientLastFrame).Seconds()
	enduranceAmbientLastFrame = now
	if dt < 0 {
		dt = 0
	}
	// Avoid a giant visual leap after opening the dev console, alt-tabbing, etc.
	if dt > 0.05 {
		dt = 0.05
	}
	enduranceAmbientClock += dt * enduranceAmbientSpeedMultiplierNow()
	return enduranceAmbientClock
}

func enduranceWarpTargetsAllHit() bool {
	if !enduranceWarpActive || !enduranceWarpTargetsSpawned || len(targets) == 0 {
		return false
	}
	for i := range targets {
		if !targets[i].Clicked {
			return false
		}
	}
	return true
}

func enduranceScrollSpeed() float64 {
	b := enduranceDifficultyBlend(enduranceDistance)
	var speed float64
	if b <= 1.0 {
		speed = 150.0 + (185.0-150.0)*b
	} else {
		speed = 185.0 + (225.0-185.0)*(b-1.0)
	}
	speed *= enduranceWarpSpeedMultiplierNow()
	if !enduranceSlowUntil.IsZero() && time.Now().Before(enduranceSlowUntil) {
		speed *= 0.50
	}
	return speed
}

func enduranceTurnStrength() float64 {
	return enduranceTurnStrengthAt(enduranceProgressDistance())
}

func enduranceTargetGap() float64 {
	if enduranceWarpActive {
		return 4.5
	}
	switch {
	case enduranceProgressDistance() < 80:
		return 24
	case enduranceProgressDistance() < 160:
		return 18
	case enduranceProgressDistance() < 260:
		return 14
	default:
		return 11
	}
}
