//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
	"unsafe"
)

func enduranceRenderInterval() time.Duration {
	switch gameMeta.FPSMode {
	case 1:
		return time.Second / 120
	case 2:
		return time.Second / 240
	default:
		return time.Second / 60
	}
}

func updateEnduranceFixed(h uintptr, dt float64) {
	runtimeStateSanityCheck()
	if !enduranceActive() || state != StatePlaying || len(path) < 2 {
		return
	}
	if dt <= 0 {
		return
	}

	w, hgt := getClient(h)

	if enduranceNextWarpAt <= 0 {
		ensureEnduranceWarpScheduleThrough(enduranceWarpFirstMeters)
		enduranceNextWarpAt = nextEnduranceWarpCheckpointAfter(enduranceTrackDistance() - 0.001)
	}
	if !enduranceInWarpTransition() && endurancePlayerEnteringWarpPortal() {
		beginEnduranceWarpCue()
	}
	updateEnduranceWarpState()

	speed := enduranceScrollSpeed()
	delta := speed * dt

	enduranceCameraX += delta
	enduranceDistance = enduranceCameraX/10.0 + enduranceDistanceBonus
	updateEnduranceWarpState()
	score = int(math.Round(enduranceDistance*10)) + enduranceBonusScore
	if score > bestScore {
		bestScore = score
	}

	trimEndurancePath(w, hgt)
	appendEndurancePoints(w, hgt)
	spawnEnduranceWarpTargets()
	updateEndurancePowerups(w, hgt)
	updateEnduranceSectorPresentation()
	syncShieldProtectSound()
	syncPolishAudioMix()

	if !updateEnduranceBlocks(w, hgt, dt) {
		loseGame(h, "Hit a falling meteorite")
		return
	}
	if ok, reason := updateEnduranceAlienThreats(w, hgt, dt); !ok {
		loseGame(h, reason)
		return
	}
	if !developerBoundaryGraceActive() && !enduranceShipInsideTrack(cursorPos) {
		loseGame(h, "Cursor crossed the side boundary")
		return
	}
	t := currentRequiredTarget()
	if t >= 0 {
		tp := targetCurrentPoint(t)
		if tp.X < cursorPos.X-26 {
			loseGame(h, "Target missed")
			return
		}
	}
	if currentRequiredTarget() < 0 &&
		!enduranceWarpCueActive &&
		!enduranceWarpActive &&
		time.Since(startTime).Seconds() >= 2.0 &&
		enduranceProgressDistance() >= enduranceNextTargetAt {
		spawnEnduranceTarget()
	}
}

func runMainLoop(h uintptr) {
	var msg MSG
	running := true
	enduranceLastLoopTime = time.Now()
	enduranceLastRenderTime = time.Time{}
	enduranceLastHUDPaint = time.Time{}

	const fixedStep = 1.0 / 125.0

	for running {
		for {
			r, _, _ := peekMessageW.Call(
				uintptr(unsafe.Pointer(&msg)),
				0, 0, 0, PM_REMOVE,
			)
			if r == 0 {
				break
			}
			if msg.Message == WM_QUIT {
				running = false
				break
			}
			translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			dispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
		if !running {
			break
		}

		now := time.Now()
		dt := now.Sub(enduranceLastLoopTime).Seconds()
		enduranceLastLoopTime = now
		if dt < 0 {
			dt = 0
		}
		if dt > 0.050 {
			dt = 0.050
		}

		// If Direct2D was lost, retry cleanly after a short cool-down.
		if !d2dReady && d2dChildHwnd != 0 && enduranceActive() && state == StatePlaying &&
			(d2dRetryAfter.IsZero() || !now.Before(d2dRetryAfter)) {
			killTimer.Call(h, TIMER_GAME)
			if initD2D(d2dChildHwnd) {
				logRuntimeEvent("d2d_recovered", "Direct2D reinitialized successfully")
				d2dRetryAfter = time.Time{}
				enduranceLastRenderTime = time.Time{}
				enduranceLastHUDPaint = time.Time{}
			} else {
				logRuntimeEvent("d2d_retry_failed", "Direct2D reinitialization failed")
				d2dRetryAfter = now.Add(750 * time.Millisecond)
			}
		}

		active := d2dReady && enduranceActive() && state == StatePlaying && overlayMode == OverlayNone
		setD2DPlayfieldVisible(active)

		// Generation already clamps every new chunk. Verify the buffered path only
		// once per second as a corruption/resize safety net, not 125 times/second.
		if active && (endurancePathSafetyCheckAt.IsZero() || now.Sub(endurancePathSafetyCheckAt) >= time.Second) {
			endurancePathSafetyCheckAt = now
			w, hgt := getClient(h)
			if !endurancePathWithinArena(w, hgt) {
				clampEndurancePathToArena(w, hgt)
			}
		}

		if active {
			enduranceLoopAccumulator += dt
			steps := 0
			for enduranceLoopAccumulator >= fixedStep && steps < 6 {
				updateStart := time.Now()
				updateEnduranceFixed(h, fixedStep)
				perfMeasureUpdate(updateStart)
				enduranceLoopAccumulator -= fixedStep
				steps++
				if state != StatePlaying {
					break
				}
			}

			if enduranceLoopAccumulator >= fixedStep {
				perfStats.DroppedSteps++
				enduranceLoopAccumulator = math.Mod(enduranceLoopAccumulator, fixedStep)
			}

			if state == StatePlaying {
				// Explicitly bound presentation work to the selected FPS mode.
				// The old "render every loop" path could saturate a CPU core/GPU
				// when Present returned immediately, starving input and simulation.
				renderInterval := enduranceRenderInterval()
				if enduranceLastRenderTime.IsZero() || now.Sub(enduranceLastRenderTime) >= renderInterval {
					frameStart := perfBeginFrame()
					renderStart := time.Now()
					renderEnduranceD2D()
					perfMeasureRender(renderStart)
					perfEndFrame(frameStart)
					enduranceLastRenderTime = now
				}

				// HUD values do not need a full parent-window repaint every frame.
				if enduranceLastHUDPaint.IsZero() || now.Sub(enduranceLastHUDPaint) >= 250*time.Millisecond {
					invalidateRect.Call(h, 0, 0)
					enduranceLastHUDPaint = now
				}
			}
		} else {
			enduranceLoopAccumulator = 0
			time.Sleep(time.Millisecond)
		}
	}
	setD2DPlayfieldVisible(false)
}

func buildEnduranceChunk(startX, startY float64, pointCount int, top, bottom float64, straightPixels float64) []FPoint {
	if pointCount < 2 {
		pointCount = 2
	}
	const spacing = 9.0
	out := make([]FPoint, 0, pointCount)
	x := startX
	y := startY
	if y < top {
		y = top
	}
	if y > bottom {
		y = bottom
	}

	phase1 := randf(0, math.Pi*2)
	phase2 := randf(0, math.Pi*2)
	phase3 := randf(0, math.Pi*2)
	mid := (top + bottom) / 2.0
	halfRange := (bottom - top) / 2.0

	for i := 0; i < pointCount; i++ {
		if i > 0 {
			x += spacing
			travel := float64(i) * spacing
			projectedDistance := enduranceGeneratedDistance + travel/10.0

			checkpoint, inWarpApproach, inWarpStraight := enduranceWarpGeometryAt(projectedDistance)

			if travel <= straightPixels || inWarpStraight {
				enduranceChunkDY = 0
				if travel <= straightPixels {
					y = startY
				} else {
					// Warp rail is locked exactly to the vertical centre of the arena.
					y = mid
				}
			} else if inWarpApproach {
				// Smoothly pull the pre-generated rail onto the centreline before the
				// wormhole marker.  The checkpoint itself may vary by +/- about 100m.
				startPhase := checkpoint - enduranceWarpApproachMeters
				t := (projectedDistance - startPhase) / enduranceWarpApproachMeters
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				ease := t * t * (3.0 - 2.0*t)
				enduranceChunkDY *= 0.86
				y += (mid - y) * (0.12 + 0.58*ease)
			} else {
				blend := enduranceDifficultyBlend(projectedDistance)

				mainFreq := 0.018 + (0.052-0.018)*(blend/2.0)
				secondFreq := 0.007 + (0.032-0.007)*(blend/2.0)
				sharpFreq := 0.0
				sharpWeight := 0.0
				if blend > 0.35 {
					u := (blend - 0.35) / 1.65
					if u > 1 {
						u = 1
					}
					sharpFreq = 0.045 + (0.115-0.045)*u
					sharpWeight = 0.16 + 0.26*u
				}

				ampRatio := 0.44
				if blend <= 1 {
					ampRatio = 0.44 + (0.72-0.44)*blend
				} else {
					ampRatio = 0.72 + (0.94-0.72)*(blend-1)
				}
				amp := halfRange * ampRatio
				t := float64(i)

				desiredY := mid +
					math.Sin(t*mainFreq+phase1)*amp*0.70 +
					math.Sin(t*secondFreq+phase2)*amp*0.28
				if sharpWeight > 0 {
					// Tanh turns the extra wave into broad plateaus with decisive
					// reversals, giving Hard/Insane-style bends without jitter.
					turnShape := math.Tanh(math.Sin(t*sharpFreq+phase3) * 2.8)
					desiredY += turnShape * amp * sharpWeight
				}

				errorY := desiredY - y
				response := 0.040 + 0.045*(blend/2.0)
				desiredDY := errorY * response
				maxDY := 3.2 + 5.5*(blend/2.0)
				if desiredDY > maxDY {
					desiredDY = maxDY
				}
				if desiredDY < -maxDY {
					desiredDY = -maxDY
				}

				slopeBlend := 0.14 + 0.22*(blend/2.0)
				enduranceChunkDY += (desiredDY - enduranceChunkDY) * slopeBlend
				y += enduranceChunkDY
			}

			steerMargin := math.Min(42.0, math.Max(0, (bottom-top)*0.16))
			if y < top+steerMargin {
				enduranceChunkDY += 0.30
			}
			if y > bottom-steerMargin {
				enduranceChunkDY -= 0.30
			}
			if y < top {
				y = top
				enduranceChunkDY = math.Abs(enduranceChunkDY) * 0.52
			}
			if y > bottom {
				y = bottom
				enduranceChunkDY = -math.Abs(enduranceChunkDY) * 0.52
			}
		}
		out = append(out, FPoint{X: x, Y: y})
		enduranceWorldStep++
	}
	enduranceGeneratedDistance += float64(pointCount-1) * spacing / 10.0
	return out
}

func appendBufferedEnduranceChunk(w, hgt int32) {
	if len(path) == 0 {
		return
	}
	ar := arenaRect(w, hgt)
	top, bottom := endurancePathCenterBounds(ar)

	start := path[len(path)-1]
	chunk := buildEnduranceChunk(
		start.X,
		start.Y,
		enduranceChunkPoints,
		top,
		bottom,
		0,
	)
	if len(chunk) > 1 {
		path = append(path, chunk[1:]...)
		enduranceChunksBuilt++
		enduranceLastX = path[len(path)-1].X
		enduranceRailDirty = true
		markMergedEnduranceRailDirty()
	}
}

func generateEndurancePath(h uintptr) {
	w, hgt := getClient(h)
	ar := arenaRect(w, hgt)

	left := float64(ar.Left) + 42
	top, bottom := endurancePathCenterBounds(ar)
	centerY := float64(ar.Top+ar.Bottom) / 2

	releaseEnduranceD2DGeometries()
	enduranceDistance = 0
	enduranceBonusScore = 0
	enduranceTargetsHit = 0
	resetEndurancePowerups()
	enduranceNextTargetAt = 34
	enduranceWorldStep = 0
	enduranceTargetSerial = 0
	enduranceLastTick = time.Time{}
	enduranceLastX = left
	enduranceChunkDY = 0
	enduranceChunksBuilt = 0
	enduranceGeneratedDistance = 0
	enduranceCameraX = 0
	enduranceBlocks = enduranceBlocks[:0]
	enduranceBlockSpawnTime = time.Time{}
	enduranceAlienMinions = enduranceAlienMinions[:0]
	enduranceAlienMinionSpawnTime = time.Time{}
	enduranceAlienPhaseTriggered = false
	resetV300EncounterDirector()
	enduranceAlienBossState = alienBossIdle
	enduranceAlienBossStateStarted = time.Time{}
	enduranceAlienBossX = 0
	enduranceAlienBossY = 0
	enduranceAlienBossTargetY = 0
	enduranceAlienBossMoveTargetY = 0
	enduranceAlienBossLockedX = 0
	enduranceAlienBossLockedY = 0
	enduranceAlienBossEntryStartX = 0
	enduranceAlienBossEntryStartY = 0
	enduranceAlienBossEntryTargetX = 0
	enduranceAlienBossEntryTargetY = 0
	enduranceWarpCheckpoints = []float64{enduranceWarpFirstMeters}
	enduranceNextWarpAt = enduranceWarpFirstMeters
	enduranceWarpCueActive = false
	enduranceWarpActive = false
	enduranceWarpCueStarted = time.Time{}
	enduranceWarpStartDistance = 0
	enduranceWarpCheckpoint = 0
	enduranceWarpRecoveryUntil = 0
	enduranceWarpTargetsSpawned = false

	// One chunk is a little wider than the visible arena. Two chunks therefore
	// give us a full off-screen buffer before gameplay even starts.
	chunkWidth := float64(ar.Right-ar.Left) + 180.0
	enduranceChunkPoints = int(math.Ceil(chunkWidth/9.0)) + 1
	if enduranceChunkPoints < 80 {
		enduranceChunkPoints = 80
	}

	first := buildEnduranceChunk(
		left,
		centerY,
		enduranceChunkPoints,
		top,
		bottom,
		300.0, // 2 seconds straight at 150 px/s
	)
	path = append(path[:0], first...)
	enduranceChunksBuilt = 1
	markMergedEnduranceRailDirty()

	// Pre-generate and stitch chunk B before the player starts.
	appendBufferedEnduranceChunk(w, hgt)
	clampEndurancePathToArena(w, hgt)

	enduranceLastX = path[len(path)-1].X
	targets = targets[:0]
	progressIndex = 0
	previousEnd = FPoint{}
	hasPreviousEnd = false
	currentCourse = 0
	enduranceRailDirty = true
	status = "Click START to begin Endurance"
}

func appendEndurancePoints(w, hgt int32) {
	if len(path) == 0 || enduranceChunkPoints <= 1 {
		return
	}
	ar := arenaRect(w, hgt)
	viewW := float64(ar.Right - ar.Left)

	// Keep a large pre-generated buffer so a 3x warp cannot outrun the rail and
	// force an expensive geometry rebuild mid-boost. Fill the buffer before it is
	// needed; the physical course remains one continuous world-space path.
	requiredWorldRight := enduranceCameraX + float64(ar.Right) + viewW*4.5
	for path[len(path)-1].X < requiredWorldRight {
		appendBufferedEnduranceChunk(w, hgt)
	}
	clampEndurancePathToArena(w, hgt)
}

func trimEndurancePath(w, hgt int32) {
	// Trimming invalidates/rebuilds the merged Direct2D rail geometry. Never do
	// that during the portal approach, Ready/Go, boost or recovery; keeping the
	// same geometry alive removes the hitch that could feel like a teleport.
	if enduranceWarpSafetyActive() {
		return
	}
	if enduranceChunkPoints <= 1 || len(path) < enduranceChunkPoints*3 {
		return
	}
	ar := arenaRect(w, hgt)
	firstChunkEnd := enduranceChunkPoints - 1
	if firstChunkEnd >= len(path) {
		return
	}

	// World-space trim. Camera moves; geometry never does.
	if path[firstChunkEnd].X-enduranceCameraX >= float64(ar.Left)-300 {
		return
	}

	cut := firstChunkEnd
	if cut <= 0 {
		return
	}
	path = append([]FPoint(nil), path[cut:]...)
	markMergedEnduranceRailDirty()
	progressIndex -= cut
	if progressIndex < 0 {
		progressIndex = 0
	}

	kept := targets[:0]
	for _, t := range targets {
		t.Index -= cut
		t.MinIndex -= cut
		t.MaxIndex -= cut
		if t.Index >= 0 && t.Index < len(path) {
			t.Point = path[t.Index]
			kept = append(kept, t)
		}
	}
	targets = kept
	enduranceRailDirty = true
}

// enduranceTargetOverlapsWarpPortal rejects target candidates that would visually
// sit on top of a wormhole.  It uses the same fixed portal dimensions as the D2D
// renderer and checks the pre-generated checkpoint schedule, so a target cannot
// be queued on the portal even a few moments before the portal becomes visible.
func enduranceTargetOverlapsWarpPortal(candidate FPoint) bool {
	if mainHwnd == 0 {
		return false
	}
	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)
	if ar.Right <= ar.Left || ar.Bottom <= ar.Top {
		return false
	}

	// Portal is 102x182 on screen. Expand by the 22px target glow plus a small
	// breathing margin so targets never appear to touch the portal artwork.
	const halfPortalW = 51.0
	const halfPortalH = 91.0
	const targetGlow = 22.0
	const margin = 10.0
	blockHalfW := halfPortalW + targetGlow + margin
	blockHalfH := halfPortalH + targetGlow + margin

	centerY := float64(ar.Top+ar.Bottom) * 0.5
	// A checkpoint at cp metres is anchored to the same world X used by the
	// portal renderer: localX = 42 + (cp-trackDistance)*10.
	// Converting that back to world coordinates removes camera position entirely.
	ensureEnduranceWarpScheduleThrough(enduranceTrackDistance() + enduranceWarpMaxGapMeters + enduranceWarpStraightMeters + 300)
	for _, cp := range enduranceWarpCheckpoints {
		portalWorldX := float64(ar.Left) + 42.0 + cp*10.0
		if math.Abs(candidate.X-portalWorldX) <= blockHalfW && math.Abs(candidate.Y-centerY) <= blockHalfH {
			return true
		}
		// Checkpoints are ordered; once the portal is well beyond this candidate,
		// later portals cannot overlap it either.
		if portalWorldX > candidate.X+blockHalfW {
			break
		}
	}
	return false
}

func spawnEnduranceTarget() {
	if currentRequiredTarget() >= 0 || len(path) < 90 {
		return
	}
	worldCursor := cursorPos
	worldCursor.X += enduranceCameraX
	center := 0
	best := math.MaxFloat64
	for i, p := range path {
		if d := dist(worldCursor, p); d < best {
			best = d
			center = i
		}
	}

	count := 1
	if enduranceWarpActive {
		count = 4
	} else {
		switch {
		case enduranceProgressDistance() >= 200:
			count = 4
		case enduranceProgressDistance() >= 120:
			count = 3
		case enduranceProgressDistance() >= 60:
			count = 2
		}
	}

	startIdx := center + 36
	stepIdx := 44
	if enduranceWarpActive {
		stepIdx = 28
	}
	for k := 0; k < count; k++ {
		idx := startIdx + k*stepIdx
		if idx > len(path)-24 {
			break
		}
		if idx <= center+20 {
			continue
		}
		candidate := path[idx]
		// Never place a target on the warp marker. This is checked against the
		// actual pre-generated portal footprint, including the target's glow radius.
		if enduranceTargetOverlapsWarpPortal(candidate) {
			continue
		}
		blockedByPowerup := false
		for _, pu := range endurancePowerups {
			if dist(candidate, pu.Point) < 72.0 {
				blockedByPowerup = true
				break
			}
		}
		if blockedByPowerup {
			continue
		}
		enduranceTargetSerial++
		targets = append(targets, Target{
			Point: path[idx], Index: idx, MinIndex: idx, MaxIndex: idx, Clicked: false,
		})
	}
}

func enduranceAlienBossEventActive() bool {
	return enduranceAlienBossState >= alienBossWarning && enduranceAlienBossState < alienBossDone
}

func enduranceAlienMinionInterval() float64 {
	d := enduranceProgressDistance() - enduranceAlienPhaseStartMeters
	if d < 0 {
		d = 0
	}
	v := 5.8 - d*0.00115
	if v < 1.35 {
		v = 1.35
	}
	v /= encounterDirectorMultiplier(enduranceProgressDistance())
	if v < 1.15 {
		v = 1.15
	}
	return v
}

func enduranceAlienMinionSpeed() float64 {
	d := enduranceProgressDistance() - enduranceAlienPhaseStartMeters
	if d < 0 {
		d = 0
	}
	// v193: horizontal minions should feel like fast interception hazards rather
	// than slowly drifting obstacles. They still scale progressively with distance.
	v := 420.0 + d*0.10
	if v > 760.0 {
		v = 760.0
	}
	return v
}

func visibleEndurancePathYRange(ar RECT) (float64, float64) {
	minY := float64(ar.Bottom)
	maxY := float64(ar.Top)
	found := false
	for _, p := range path {
		screenX := p.X - enduranceCameraX
		if screenX < float64(ar.Left)-20 || screenX > float64(ar.Right)+20 {
			continue
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
		found = true
	}
	if !found {
		cy := float64(ar.Top+ar.Bottom) * 0.5
		return cy - 70, cy + 70
	}
	return minY, maxY
}

func spawnEnduranceAlienMinion(w, hgt int32) {
	spawnEnduranceAlienMinionInternal(w, hgt, false)
}

func spawnEnduranceAlienMinionInternal(w, hgt int32, bossSupport bool) {
	if enduranceProgressDistance() < enduranceAlienPhaseStartMeters || enduranceWarpSafetyActive() || (enduranceAlienBossEventActive() && !bossSupport) {
		return
	}
	if len(enduranceAlienMinions) >= 6 {
		return
	}
	ar := arenaRect(w, hgt)
	const mw = 58.0
	const mh = 39.0
	minPathY, maxPathY := visibleEndurancePathYRange(ar)
	minY := math.Max(float64(ar.Top)+8, minPathY-mh*0.5)
	maxY := math.Min(float64(ar.Bottom)-mh-8, maxPathY-mh*0.5)
	if maxY < minY {
		minY = float64(ar.Top) + 20
		maxY = float64(ar.Bottom) - mh - 20
	}
	y := randf(minY, maxY)
	for attempt := 0; attempt < 10; attempt++ {
		clear := true
		for _, t := range targets {
			if t.Clicked {
				continue
			}
			tp := targetCurrentPointByTarget(t)
			tp.X -= enduranceCameraX
			if tp.X >= float64(ar.Left)-50 && tp.X <= float64(ar.Right)+80 &&
				math.Abs(tp.Y-(y+mh*0.5)) < 58 {
				clear = false
				break
			}
		}
		if clear {
			for _, pu := range endurancePowerups {
				pp := pu.Point
				pp.X -= enduranceCameraX
				if pp.X >= float64(ar.Left)-50 && pp.X <= float64(ar.Right)+80 &&
					math.Abs(pp.Y-(y+mh*0.5)) < 52 {
					clear = false
					break
				}
			}
		}
		for _, a := range enduranceAlienMinions {
			if a.X > float64(ar.Right)-180 && math.Abs((a.Y+a.Height*0.5)-(y+mh*0.5)) < mh+18 {
				clear = false
				break
			}
		}
		if clear {
			break
		}
		if attempt == 9 {
			return
		}
		y = randf(minY, maxY)
	}
	enduranceAlienMinions = append(enduranceAlienMinions, EnduranceAlienMinion{
		X: float64(ar.Right) + 12, Y: y, Width: mw, Height: mh,
		Speed: enduranceAlienMinionSpeed() * randf(0.92, 1.10),
	})
}

func beginEnduranceAlienBossEvent() {
	if enduranceAlienBossEventActive() {
		return
	}
	enduranceAlienPhaseTriggered = true
	enduranceAlienBossState = alienBossWarning
	enduranceAlienBossStateStarted = time.Now()
	showPriorityLiveAnnouncement("CAUTION: UFO INCOMING")
	status = "CAUTION: UFO INCOMING"
}

func setAlienBossState(state int) {
	previous := enduranceAlienBossState
	if (previous == alienBossAim1 || previous == alienBossAim2 || previous == alienBossAim3 || previous == alienBossAim4) && state != previous {
		stopAlienChargeSound()
	}

	enduranceAlienBossState = state
	enduranceAlienBossStateStarted = time.Now()

	switch state {
	case alienBossAim1, alienBossAim2, alienBossAim3, alienBossAim4:
		// Freeze the boss completely before any warning beam is shown.
		enduranceAlienBossLockedX = enduranceAlienBossX
		enduranceAlienBossLockedY = enduranceAlienBossY
		playAlienChargeSound()
	case alienBossLaser1Extend, alienBossLaser2Extend, alienBossLaser3Extend, alienBossLaser4Extend:
		// Deadly laser begins only after the boss is already locked in place.
		enduranceAlienBossX = enduranceAlienBossLockedX
		enduranceAlienBossY = enduranceAlienBossLockedY
		stopAlienChargeSound()
		playAlienImpactSound()
	}
}

func enforceAlienBossFiringLock() {
	switch enduranceAlienBossState {
	case alienBossAim1, alienBossLaser1Extend, alienBossLaser1Hold, alienBossLaser1Retract,
		alienBossAim2, alienBossLaser2Extend, alienBossLaser2Hold, alienBossLaser2Retract,
		alienBossAim3, alienBossLaser3Extend, alienBossLaser3Hold, alienBossLaser3Retract,
		alienBossAim4, alienBossLaser4Extend, alienBossLaser4Hold, alienBossLaser4Retract:
		enduranceAlienBossX = enduranceAlienBossLockedX
		enduranceAlienBossY = enduranceAlienBossLockedY
	}
}

func alienBossCannonX() float64 {
	// Actual muzzle centre in the normalized 260x150 Alien Boss sprite.
	const muzzleTextureX = 28.0
	return enduranceAlienBossX + (muzzleTextureX/260.0)*enduranceAlienBossWidth
}

func alienBossCannonY() float64 {
	// Actual muzzle centre in the normalized 260x150 Alien Boss sprite.
	const muzzleTextureY = 93.0
	return enduranceAlienBossY + (muzzleTextureY/150.0)*enduranceAlienBossHeight
}

func alienBossTopForCannonY(targetY float64, ar RECT) float64 {
	const muzzleTextureY = 93.0
	offset := (muzzleTextureY / 150.0) * enduranceAlienBossHeight
	top := targetY - offset
	minTop := float64(ar.Top) + 8
	maxTop := float64(ar.Bottom) - 8 - enduranceAlienBossHeight
	if top < minTop {
		top = minTop
	}
	if top > maxTop {
		top = maxTop
	}
	return top
}

func alienLaserVisibleSegment(ar RECT) (float64, float64, bool) {
	cannonX := alienBossCannonX()
	leftEdge := float64(ar.Left)
	elapsed := time.Since(enduranceAlienBossStateStarted).Seconds()
	switch enduranceAlienBossState {
	case alienBossLaser1Extend, alienBossLaser2Extend, alienBossLaser3Extend, alienBossLaser4Extend:
		p := elapsed / enduranceAlienLaserExtendSecs
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		return cannonX - (cannonX-leftEdge)*p, cannonX, true
	case alienBossLaser1Hold, alienBossLaser2Hold, alienBossLaser3Hold, alienBossLaser4Hold:
		return leftEdge, cannonX, true
	case alienBossLaser1Retract, alienBossLaser2Retract, alienBossLaser3Retract, alienBossLaser4Retract:
		p := elapsed / enduranceAlienLaserRetractSecs
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		// Disappear from right -> left.
		return leftEdge, cannonX - (cannonX-leftEdge)*p, true
	}
	return 0, 0, false
}

func alienBossBodyCollision(p FPoint) bool {
	if enduranceAlienBossState < alienBossEntering || enduranceAlienBossState >= alienBossDone {
		return false
	}
	return p.X >= enduranceAlienBossX+15 &&
		p.X <= enduranceAlienBossX+enduranceAlienBossWidth-12 &&
		p.Y >= enduranceAlienBossY+10 &&
		p.Y <= enduranceAlienBossY+enduranceAlienBossHeight-10
}

func updateEnduranceAlienThreats(w, hgt int32, dt float64) (bool, string) {
	// The alien sprites/laser are rendered by the D2D Endurance playfield.
	// If hardware rendering falls back, skip this mechanic rather than allow
	// invisible hazards to collide with the player.
	if !d2dReady || d2dAlienMinionBitmap == 0 || d2dAlienBossBitmap == 0 {
		return true, ""
	}
	ar := arenaRect(w, hgt)
	if v300AlienBossReady(enduranceTrackDistance()) && !enduranceWarpSafetyActive() {
		beginEnduranceAlienBossEvent()
	}

	// Minions begin after the boss introduction and become more frequent with distance.
	if enduranceAlienBossState == alienBossDone && !enduranceWarpSafetyActive() && !endurancePostEncounterRecoveryActive() {
		if enduranceAlienMinionSpawnTime.IsZero() {
			enduranceAlienMinionSpawnTime = time.Now()
		}
		if time.Since(enduranceAlienMinionSpawnTime).Seconds() >= enduranceAlienMinionInterval() {
			spawnEnduranceAlienMinion(w, hgt)
			enduranceAlienMinionSpawnTime = time.Now()
		}
	}
	if enduranceWarpSafetyActive() {
		enduranceAlienMinions = enduranceAlienMinions[:0]
		enduranceAlienMinionSpawnTime = time.Time{}
	}

	kept := enduranceAlienMinions[:0]
	alienTimeScale := 1.0
	if enduranceTimeSlowActive() {
		alienTimeScale = 0.55
	}
	for _, a := range enduranceAlienMinions {
		a.X -= a.Speed * alienTimeScale * dt
		if cursorPos.X >= a.X+5 && cursorPos.X <= a.X+a.Width-5 &&
			cursorPos.Y >= a.Y+4 && cursorPos.Y <= a.Y+a.Height-4 {
			if enduranceShieldActive() {
				addPolishVFX(polishVFXPickup, float32((a.X+a.Width*0.5)-float64(ar.Left)), float32((a.Y+a.Height*0.5)-float64(ar.Top)), 0.24)
				continue
			}
			return false, "Collided with an alien minion"
		}
		if a.X+a.Width > float64(ar.Left)-8 {
			kept = append(kept, a)
		}
	}
	enduranceAlienMinions = kept

	now := time.Now()
	enforceAlienBossFiringLock()
	switch enduranceAlienBossState {
	case alienBossIdle, alienBossDone:
		return true, ""
	case alienBossWarning:
		if now.Sub(enduranceAlienBossStateStarted) >= 3*time.Second {
			// Capture a complete entry trajectory once. The boss now eases smoothly
			// from off-screen into its firing position instead of moving linearly
			// and then snapping on the final frame.
			enduranceAlienBossEntryStartX = float64(ar.Right) + enduranceAlienBossWidth + 20
			enduranceAlienBossEntryTargetX = float64(ar.Right) - enduranceAlienBossWidth - 26
			enduranceAlienBossEntryTargetY = alienBossTopForCannonY(cursorPos.Y, ar)
			enduranceAlienBossEntryStartY = enduranceAlienBossEntryTargetY
			enduranceAlienBossX = enduranceAlienBossEntryStartX
			enduranceAlienBossY = enduranceAlienBossEntryStartY
			setAlienBossState(alienBossEntering)
		}
	case alienBossEntering:
		p := now.Sub(enduranceAlienBossStateStarted).Seconds() / alienBossEntryDuration()
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		// Smoothstep gives a natural deceleration into the lock point with zero
		// visual jump when the boss changes into the warning-beam state.
		eased := p * p * (3.0 - 2.0*p)
		enduranceAlienBossX = enduranceAlienBossEntryStartX +
			(enduranceAlienBossEntryTargetX-enduranceAlienBossEntryStartX)*eased
		enduranceAlienBossY = enduranceAlienBossEntryStartY +
			(enduranceAlienBossEntryTargetY-enduranceAlienBossEntryStartY)*eased
		if p >= 1 {
			enduranceAlienBossX = enduranceAlienBossEntryTargetX
			enduranceAlienBossY = enduranceAlienBossEntryTargetY
			enduranceAlienBossTargetY = alienBossCannonY()
			enduranceAlienBossLockedX = enduranceAlienBossX
			enduranceAlienBossLockedY = enduranceAlienBossY
			setAlienBossState(alienBossAim1)
		}
	case alienBossAim1:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= alienBossAimDuration() {
			setAlienBossState(alienBossLaser1Extend)
		}
	case alienBossLaser1Extend:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= enduranceAlienLaserExtendSecs {
			setAlienBossState(alienBossLaser1Hold)
		}
	case alienBossLaser1Hold:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= alienBossHoldDuration() {
			setAlienBossState(alienBossLaser1Retract)
		}
	case alienBossLaser1Retract:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= enduranceAlienLaserRetractSecs {
			enduranceAlienBossMoveTargetY = alienBossTopForCannonY(cursorPos.Y, ar)
			setAlienBossState(alienBossReposition)
		}
	case alienBossReposition:
		dy := enduranceAlienBossMoveTargetY - enduranceAlienBossY
		step := 270 * dt
		if math.Abs(dy) <= step {
			enduranceAlienBossY = enduranceAlienBossMoveTargetY
			// Lock only after the cannon reaches its final aligned position.
			enduranceAlienBossTargetY = alienBossCannonY()
			enduranceAlienBossLockedX = enduranceAlienBossX
			enduranceAlienBossLockedY = enduranceAlienBossY
			setAlienBossState(alienBossAim2)
		} else if dy < 0 {
			enduranceAlienBossY -= step
		} else {
			enduranceAlienBossY += step
		}
	case alienBossAim2:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= alienBossAimDuration() {
			setAlienBossState(alienBossLaser2Extend)
		}
	case alienBossLaser2Extend:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= enduranceAlienLaserExtendSecs {
			setAlienBossState(alienBossLaser2Hold)
		}
	case alienBossLaser2Hold:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= alienBossHoldDuration() {
			setAlienBossState(alienBossLaser2Retract)
		}
	case alienBossLaser2Retract:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= enduranceAlienLaserRetractSecs {
			// First support wave: already-learned horizontal hazard, introduced only after two clean boss shots.
			spawnEnduranceAlienMinionInternal(w, hgt, true)
			if rand.Intn(2) == 0 {
				spawnEnduranceAlienMinionInternal(w, hgt, true)
			}
			enduranceAlienBossMoveTargetY = alienBossTopForCannonY(cursorPos.Y, ar)
			setAlienBossState(alienBossReposition2)
		}
	case alienBossReposition2:
		dy := enduranceAlienBossMoveTargetY - enduranceAlienBossY
		step := 300 * dt
		if math.Abs(dy) <= step {
			enduranceAlienBossY = enduranceAlienBossMoveTargetY
			enduranceAlienBossLockedX = enduranceAlienBossX
			enduranceAlienBossLockedY = enduranceAlienBossY
			setAlienBossState(alienBossAim3)
		} else if dy < 0 {
			enduranceAlienBossY -= step
		} else {
			enduranceAlienBossY += step
		}
	case alienBossAim3:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= alienBossAimDuration() {
			setAlienBossState(alienBossLaser3Extend)
		}
	case alienBossLaser3Extend:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= enduranceAlienLaserExtendSecs {
			setAlienBossState(alienBossLaser3Hold)
		}
	case alienBossLaser3Hold:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= alienBossHoldDuration() {
			setAlienBossState(alienBossLaser3Retract)
		}
	case alienBossLaser3Retract:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= enduranceAlienLaserRetractSecs {
			spawnEnduranceAlienMinionInternal(w, hgt, true)
			if rand.Intn(2) == 0 {
				spawnEnduranceAlienMinionInternal(w, hgt, true)
			}
			enduranceAlienBossMoveTargetY = alienBossTopForCannonY(cursorPos.Y, ar)
			setAlienBossState(alienBossReposition3)
		}
	case alienBossReposition3:
		dy := enduranceAlienBossMoveTargetY - enduranceAlienBossY
		step := 330 * dt
		if math.Abs(dy) <= step {
			enduranceAlienBossY = enduranceAlienBossMoveTargetY
			enduranceAlienBossLockedX = enduranceAlienBossX
			enduranceAlienBossLockedY = enduranceAlienBossY
			setAlienBossState(alienBossAim4)
		} else if dy < 0 {
			enduranceAlienBossY -= step
		} else {
			enduranceAlienBossY += step
		}
	case alienBossAim4:
		// Keep the fourth warning fair as well: full reaction time before the shot.
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= alienBossAimDuration() {
			setAlienBossState(alienBossLaser4Extend)
		}
	case alienBossLaser4Extend:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= enduranceAlienLaserExtendSecs {
			setAlienBossState(alienBossLaser4Hold)
		}
	case alienBossLaser4Hold:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= alienBossHoldDuration() {
			setAlienBossState(alienBossLaser4Retract)
		}
	case alienBossLaser4Retract:
		if now.Sub(enduranceAlienBossStateStarted).Seconds() >= enduranceAlienLaserRetractSecs+0.5 {
			setAlienBossState(alienBossExiting)
		}
	case alienBossExiting:
		elapsed := now.Sub(enduranceAlienBossStateStarted).Seconds()
		launch := easeSmoothStep(elapsed / 0.55)
		exitSpeed := 120.0 + 690.0*launch

		// Exit back through the right side where the boss already lives. Sending
		// this large sprite left across the whole arena forced the player to dodge
		// the boss itself after surviving the four-beam encounter.
		enduranceAlienBossX += exitSpeed * dt
		if enduranceAlienBossX > float64(ar.Right)+20 {
			setAlienBossState(alienBossDone)
			gameMeta.AlienBossesSurvived++
			evaluateEnduranceAchievements()
			saveGameMeta()
			beginEndurancePostEncounterRecovery(120)
			scheduleNextV300AlienBoss(enduranceProgressDistance())
			status = "ALIEN CONTACT CLEARED — RECOVERY WINDOW"
			enduranceAlienMinionSpawnTime = time.Now()
			return true, ""
		}
	}

	enforceAlienBossFiringLock()

	if enduranceShipHitsAlienBoss() && !enduranceShieldActive() {
		return false, "Collided with the alien UFO"
	}
	if x1, x2, active := alienLaserVisibleSegment(ar); active && x2 > x1 {
		laserY := alienBossCannonY()
		if enduranceShipRectIntersects(cursorPos, x1, laserY-8, x2, laserY+8) {
			// Boss laser pierces Shield. Shield still protects against alien
			// minions (and the existing non-laser hazards) as intended.
			return false, "Hit by the alien laser"
		}
	}
	return true, ""
}

func enduranceShipHitsAlienBoss() bool {
	for _, q := range enduranceShipHitboxPointsAt(cursorPos) {
		if alienBossBodyCollision(q) {
			return true
		}
	}
	return false
}

func enduranceBlockInterval() float64 {
	if enduranceProgressDistance() < 50 {
		return 999
	}

	// v217 pacing: meteors should build pressure gradually BEFORE the first
	// 1000m Warp rather than reaching near-max frequency only a few hundred
	// metres into a run.
	//
	// 50m    -> 3.40s base interval
	// 1000m  -> 1.75s base interval
	// 3000m+ -> 0.72s base interval
	//
	// The Encounter Director still adds Recovery/Build/Pressure/Peak variation,
	// so the run breathes without the early game becoming a meteor wall.
	var v float64
	if enduranceProgressDistance() <= enduranceWarpFirstMeters {
		t := (enduranceProgressDistance() - 50.0) / (enduranceWarpFirstMeters - 50.0)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		v = 3.40 + (1.75-3.40)*t
	} else {
		t := (enduranceProgressDistance() - enduranceWarpFirstMeters) / 2000.0
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		v = 1.75 + (0.72-1.75)*t
	}

	v /= encounterDirectorMultiplier(enduranceProgressDistance())
	if v < 0.60 {
		v = 0.60
	}
	return v
}

func enduranceBlockFallSpeed() float64 {
	v := 185.0 + (enduranceProgressDistance()-50.0)*0.28
	if v < 185 {
		v = 185
	}
	if v > 360 {
		v = 360
	}
	return v
}

func spawnEnduranceBlock(w, hgt int32) {
	if enduranceProgressDistance() < 50 || len(enduranceBlocks) >= 8 {
		return
	}
	ar := arenaRect(w, hgt)
	arenaW := float64(ar.Right - ar.Left)
	if arenaW < 160 {
		return
	}

	size := randf(28.0, 48.0)
	minX := float64(ar.Left) + 8
	maxX := float64(ar.Right) - 8 - size
	if maxX < minX {
		return
	}

	// Aim some cubes directly at the current cursor lane and some just around it.
	// This creates deliberate dodging choices without making every drop unavoidable.
	var desired float64
	if randf(0, 1) < 0.58 {
		desired = cursorPos.X - size/2 + randf(-24, 24)
	} else {
		side := -1.0
		if randf(0, 1) > 0.5 {
			side = 1
		}
		desired = cursorPos.X - size/2 + side*randf(62, 165)
	}
	if desired < minX {
		desired = minX
	}
	if desired > maxX {
		desired = maxX
	}

	// Never spawn intersecting/stacked cubes. Try nearby alternatives before skipping.
	x := desired
	ok := false
	for attempt := 0; attempt < 14; attempt++ {
		if attempt > 0 {
			x = desired + randf(-150, 150)
			if x < minX {
				x = minX
			}
			if x > maxX {
				x = maxX
			}
		}
		candidate := RECT{
			Left:   int32(math.Round(x - 10)),
			Top:    ar.Top - 80,
			Right:  int32(math.Round(x + size + 10)),
			Bottom: ar.Top + 20,
		}
		overlap := false
		// Do not visually cover an active target or pickup with a falling meteor.
		screenCandidate := FPoint{X: x + size*0.5, Y: float64(ar.Top) + 20}
		for _, t := range targets {
			if t.Clicked {
				continue
			}
			tp := targetCurrentPointByTarget(t)
			tp.X -= enduranceCameraX
			if math.Abs(tp.X-screenCandidate.X) < size*0.75+34 {
				overlap = true
				break
			}
		}
		if !overlap {
			for _, pu := range endurancePowerups {
				pp := pu.Point
				pp.X -= enduranceCameraX
				if math.Abs(pp.X-screenCandidate.X) < size*0.75+34 {
					overlap = true
					break
				}
			}
		}
		for _, b := range enduranceBlocks {
			br := RECT{
				Left:   int32(math.Round(b.X - 10)),
				Top:    int32(math.Round(b.Y - 10)),
				Right:  int32(math.Round(b.X + b.Width + 10)),
				Bottom: int32(math.Round(b.Y + b.Height + 10)),
			}
			if candidate.Left < br.Right && candidate.Right > br.Left &&
				candidate.Top < br.Bottom && candidate.Bottom > br.Top {
				overlap = true
				break
			}
		}
		if !overlap {
			ok = true
			break
		}
	}
	if !ok {
		return
	}

	enduranceBlocks = append(enduranceBlocks, EnduranceBlock{
		X:      x,
		Y:      float64(ar.Top) - size - 4,
		Width:  size,
		Height: size,
		Speed:  enduranceBlockFallSpeed() * randf(0.92, 1.10),
		Orange: randf(0, 1) < 0.42,
	})
}

func resetEndurancePowerups() {
	stopShieldProtectSound()
	endurancePowerups = endurancePowerups[:0]
	enduranceNextPowerupAt = randf(120.0, 190.0)
	enduranceShieldUntil = time.Time{}
	enduranceSlowUntil = time.Time{}
	enduranceStoredShields = 0
	enduranceStoredTime = 0
	enduranceDistanceBonus = 0
}

func enduranceTimeSlowActive() bool {
	return !enduranceSlowUntil.IsZero() && time.Now().Before(enduranceSlowUntil)
}

func rollEndurancePowerupKind() int {
	// Distance is intentionally rarer. Shield/Time are weighted toward whichever
	// storage slot still has room, so a pickup is more likely to be useful.
	shieldRoom := 2 - enduranceStoredShields
	timeRoom := 2 - enduranceStoredTime
	if shieldRoom < 0 {
		shieldRoom = 0
	}
	if timeRoom < 0 {
		timeRoom = 0
	}

	r := randf(0, 1)
	if r < 0.18 {
		return endurancePowerupDistance
	}
	if shieldRoom == 0 && timeRoom == 0 {
		return endurancePowerupDistance
	}
	if shieldRoom == 0 {
		return endurancePowerupSlow
	}
	if timeRoom == 0 {
		return endurancePowerupShield
	}
	// Slight Shield bias because it is the emergency defensive resource.
	if r < 0.62 {
		return endurancePowerupShield
	}
	return endurancePowerupSlow
}

func endurancePickupConflict(candidate FPoint, w, hgt int32, radius float64) bool {
	ar := arenaRect(w, hgt)
	for _, t := range targets {
		if t.Clicked {
			continue
		}
		p := targetCurrentPointByTarget(t)
		p.X -= enduranceCameraX
		if p.X >= float64(ar.Left)-60 && p.X <= float64(ar.Right)+60 && dist(candidate, p) < radius {
			return true
		}
	}
	for _, pu := range endurancePowerups {
		p := pu.Point
		p.X -= enduranceCameraX
		if p.X >= float64(ar.Left)-60 && p.X <= float64(ar.Right)+60 && dist(candidate, p) < radius {
			return true
		}
	}
	return false
}

func spawnEndurancePowerup() {
	if len(path) < 90 || enduranceInWarpTransition() || enduranceAlienBossEventActive() || endurancePostEncounterRecoveryActive() || len(endurancePowerups) >= 1 {
		return
	}
	worldCursor := cursorPos
	worldCursor.X += enduranceCameraX
	center := 0
	best := math.MaxFloat64
	for i, p := range path {
		if d := dist(worldCursor, p); d < best {
			best = d
			center = i
		}
	}
	// Try several future path positions. A pickup must have clear visual/click
	// space around every target and every other pickup.
	chosen := -1
	for attempt := 0; attempt < 14; attempt++ {
		idx := center + int(randf(55, 96)) + attempt*5
		if idx > len(path)-25 {
			idx = len(path) - 25
		}
		if idx <= center+28 || idx < 0 || idx >= len(path) {
			continue
		}
		candidate := path[idx]
		clear := true
		for _, t := range targets {
			if !t.Clicked && dist(candidate, targetCurrentPointByTarget(t)) < 78.0 {
				clear = false
				break
			}
		}
		if clear {
			for _, pu := range endurancePowerups {
				if dist(candidate, pu.Point) < 78.0 {
					clear = false
					break
				}
			}
		}
		if clear {
			chosen = idx
			break
		}
	}
	if chosen < 0 {
		// Skip this opportunity rather than force an overlap.
		enduranceNextPowerupAt = enduranceProgressDistance() + randf(45.0, 80.0)
		return
	}
	endurancePowerups = append(endurancePowerups, EndurancePowerup{Point: path[chosen], Kind: rollEndurancePowerupKind()})
	enduranceNextPowerupAt = enduranceProgressDistance() + randf(145.0, 235.0)
}

func targetCurrentPointByTarget(t Target) FPoint {
	if t.Index >= 0 && t.Index < len(path) {
		return path[t.Index]
	}
	return t.Point
}

func storeEndurancePowerup(kind int) bool {
	switch kind {
	case endurancePowerupShield:
		if enduranceStoredShields >= 2 {
			status = "SHIELD STORAGE FULL — PRESS Q TO USE ONE"
			return false
		}
		enduranceStoredShields++
		status = fmt.Sprintf("SHIELD STORED — %d/2  •  Q TO ACTIVATE", enduranceStoredShields)
		return true
	case endurancePowerupSlow:
		if enduranceStoredTime >= 2 {
			status = "TIME STORAGE FULL — PRESS W TO USE ONE"
			return false
		}
		enduranceStoredTime++
		status = fmt.Sprintf("TIME STORED — %d/2  •  W TO ACTIVATE", enduranceStoredTime)
		return true
	}
	return false
}

func activateStoredEnduranceShield() bool {
	if !enduranceActive() || state != StatePlaying || enduranceStoredShields <= 0 {
		return false
	}
	enduranceStoredShields--
	gameMeta.EnduranceShieldUses++
	applyEndurancePowerup(endurancePowerupShield)
	evaluateEnduranceAchievements()
	saveGameMeta()
	return true
}

func activateStoredEnduranceTime() bool {
	if !enduranceActive() || state != StatePlaying || enduranceStoredTime <= 0 {
		return false
	}
	enduranceStoredTime--
	gameMeta.EnduranceTimeUses++
	applyEndurancePowerup(endurancePowerupSlow)
	evaluateEnduranceAchievements()
	saveGameMeta()
	return true
}

func applyEndurancePowerup(kind int) {
	now := time.Now()
	switch kind {
	case endurancePowerupDistance:
		enduranceDistanceBonus += 100.0
		enduranceDistance = enduranceTrackDistance() + enduranceDistanceBonus
		// Reward only: no camera movement, no spawn/progression threshold jump,
		// no pause/freeze, and no alteration to future pickup cadence.
		status = "POWER-UP — +100 METRES"
	case endurancePowerupShield:
		enduranceShieldUntil = now.Add(5 * time.Second)
		status = "SHIELD ACTIVATED — 5s"
	case endurancePowerupSlow:
		enduranceSlowUntil = now.Add(5 * time.Second)
		status = "TIME SLOW ACTIVATED — 5s"
	}
}

func updateEndurancePowerups(w, hgt int32) {
	if enduranceInWarpTransition() || enduranceAlienBossEventActive() {
		endurancePowerups = endurancePowerups[:0]
		return
	}
	if enduranceNextPowerupAt <= 0 {
		enduranceNextPowerupAt = randf(120.0, 190.0)
	}
	if enduranceProgressDistance() >= enduranceNextPowerupAt && len(endurancePowerups) == 0 {
		spawnEndurancePowerup()
	}
	ar := arenaRect(w, hgt)
	kept := endurancePowerups[:0]
	for _, pu := range endurancePowerups {
		screen := pu.Point
		screen.X -= enduranceCameraX
		if screen.X >= float64(ar.Left)-32 && screen.X <= float64(ar.Right)+80 {
			kept = append(kept, pu)
		} else if screen.X > float64(ar.Right)+80 {
			kept = append(kept, pu)
		}
	}
	endurancePowerups = kept
}

func enduranceShieldActive() bool {
	return !enduranceShieldUntil.IsZero() && time.Now().Before(enduranceShieldUntil)
}

func cursorTouchesEnduranceBlock(p FPoint) bool {
	// The shield is authoritative at the collision layer.  Keeping this guard
	// here prevents mouse-move collision checks from bypassing the protection
	// handled by updateEnduranceBlocks.
	if enduranceShieldActive() {
		return false
	}
	for _, b := range enduranceBlocks {
		if p.X >= b.X && p.X <= b.X+b.Width && p.Y >= b.Y && p.Y <= b.Y+b.Height {
			return true
		}
	}
	return false
}

func updateEnduranceBlocks(w, hgt int32, dt float64) bool {
	if enduranceWarpSafetyActive() || enduranceAlienBossEventActive() {
		// Keep the first Alien Boss encounter focused on lasers and minion support.
		// Remove any meteor already on-screen and suppress all meteor spawning until
		// the boss has completely left the encounter.
		enduranceBlocks = enduranceBlocks[:0]
		enduranceBlockSpawnTime = time.Time{}
		return true
	}
	if enduranceProgressDistance() < 50 {
		enduranceBlocks = enduranceBlocks[:0]
		return true
	}
	ar := arenaRect(w, hgt)
	if enduranceBlockSpawnTime.IsZero() {
		enduranceBlockSpawnTime = time.Now()
	}
	if time.Since(enduranceBlockSpawnTime).Seconds() >= enduranceBlockInterval() {
		spawnEnduranceBlock(w, hgt)
		enduranceBlockSpawnTime = time.Now()
	}

	kept := enduranceBlocks[:0]
	hazardTimeScale := 1.0
	if enduranceTimeSlowActive() {
		hazardTimeScale = 0.55
	}
	for _, b := range enduranceBlocks {
		speed := b.Speed
		if speed <= 0 {
			speed = enduranceBlockFallSpeed()
		}
		b.Y += speed * hazardTimeScale * dt
		if enduranceShipRectIntersects(cursorPos, b.X, b.Y, b.X+b.Width, b.Y+b.Height) {
			if enduranceShieldActive() {
				// Shield destroys the meteor and gives immediate impact feedback.
				addPolishVFX(polishVFXPickup, float32((b.X+b.Width*0.5)-float64(ar.Left)), float32((b.Y+b.Height*0.5)-float64(ar.Top)), 0.24)
				continue
			}
			return false
		}
		if b.Y < float64(ar.Bottom)+4 {
			kept = append(kept, b)
		}
	}
	enduranceBlocks = kept
	return true
}

func drawEnduranceBlocks(hdc uintptr, w, hgt int32) {
	if !enduranceActive() || len(enduranceBlocks) == 0 {
		return
	}
	ar := arenaRect(w, hgt)
	saved, _, _ := saveDC.Call(hdc)
	if saved != 0 {
		intersectClipRect.Call(hdc, uintptr(ar.Left+1), uintptr(ar.Top+1), uintptr(ar.Right-1), uintptr(ar.Bottom-1))
		defer restoreDC.Call(hdc, saved)
	}

	// Software fallback now uses the exact same meteor art as the Direct2D path.
	// This prevents device-loss fallback from ever reverting hazards to square blocks.
	const spriteAspect = 160.0 / 64.0
	for _, b := range enduranceBlocks {
		size := int32(math.Round(b.Width))
		if size < 1 {
			continue
		}
		spriteH := int32(math.Round(float64(size) * spriteAspect))
		x := int32(math.Round(b.X))
		y := int32(math.Round(b.Y)) + size - spriteH
		dst := RECT{x, y, x + size, y + spriteH}
		if b.Orange {
			drawCachedBGRASprite(hdc, &hazardOrangeSprite, hazardOrangeBGRA, 64, 160, dst)
		} else {
			drawCachedBGRASprite(hdc, &hazardBlueSprite, hazardBlueBGRA, 64, 160, dst)
		}
	}
}

func updateEndurance(h uintptr) {
	if !enduranceActive() || state != StatePlaying || len(path) < 2 {
		return
	}
	now := time.Now()
	if enduranceLastTick.IsZero() {
		enduranceLastTick = now
		return
	}
	dt := now.Sub(enduranceLastTick).Seconds()
	enduranceLastTick = now
	if dt <= 0 {
		return
	}
	if dt > 0.025 {
		dt = 0.025
	}

	w, hgt := getClient(h)

	if enduranceNextWarpAt <= 0 {
		ensureEnduranceWarpScheduleThrough(enduranceWarpFirstMeters)
		enduranceNextWarpAt = nextEnduranceWarpCheckpointAfter(enduranceTrackDistance() - 0.001)
	}
	if !enduranceInWarpTransition() && endurancePlayerEnteringWarpPortal() {
		beginEnduranceWarpCue()
	}
	updateEnduranceWarpState()

	speed := enduranceScrollSpeed()
	delta := speed * dt

	// Static-world renderer: course geometry never moves; only cameraX changes.
	enduranceCameraX += delta
	enduranceDistance = enduranceCameraX/10.0 + enduranceDistanceBonus
	updateEnduranceWarpState()
	score = int(math.Round(enduranceDistance*10)) + enduranceBonusScore
	if score > bestScore {
		bestScore = score
	}

	trimEndurancePath(w, hgt)
	appendEndurancePoints(w, hgt)
	spawnEnduranceWarpTargets()
	updateEndurancePowerups(w, hgt)

	if !updateEnduranceBlocks(w, hgt, dt) {
		loseGame(h, "Hit a falling meteorite")
		return
	}

	if !developerBoundaryGraceActive() && !enduranceShipInsideTrack(cursorPos) {
		loseGame(h, "Cursor crossed the side boundary")
		return
	}

	t := currentRequiredTarget()
	if t >= 0 {
		tp := targetCurrentPoint(t)
		if tp.X < cursorPos.X-26 {
			loseGame(h, "Target missed")
			return
		}
	}

	if currentRequiredTarget() < 0 &&
		!enduranceWarpCueActive &&
		!enduranceWarpActive &&
		time.Since(startTime).Seconds() >= 2.0 &&
		enduranceProgressDistance() >= enduranceNextTargetAt {
		spawnEnduranceTarget()
	}
}

func finishEnduranceFailure(h uintptr) {
	if !enduranceActive() {
		return
	}
	stopTransientGameplayAudio()
	coins := enduranceTargetsHit
	if coins < 0 {
		coins = 0
	}
	lastResult.CoinsEarned = coins
	gameMeta.SpaceCoins += coins
	saveGameMeta()
	state = StateResult
	beginPolishResult(polishEnduranceNewPB)
	writeRunPerfSnapshot("endurance_result")
	analyticsEvent("run_completed", map[string]any{"mode": "ENDURANCE", "distance": lastResult.Distance, "targets": lastResult.TargetsHit, "accuracy": lastResult.CombinedAcc, "coins": coins})
	status = "Endurance run over"
	invalidateRect.Call(h, 0, 0)
}
