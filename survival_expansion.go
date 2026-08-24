//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	survivalEnemyScout = iota
	survivalEnemyHeavy
	survivalEnemySplitter
	survivalEnemyShieldDrone
	survivalEnemyPulseMine
	survivalEnemyInterceptor
	survivalEnemyPhantom
	survivalEnemySwarmDrone
	survivalEnemyGravityNode
	survivalEnemyArmouredSatellite
)

var (
	survivalExpansionSprites [10][]byte
	survivalExpansionW       = [10]int32{0, 0, 392, 329, 349, 369, 353, 158, 357, 421}
	survivalExpansionH       = [10]int32{0, 0, 412, 416, 338, 348, 510, 149, 434, 528}
	survivalSwarmSprites     [4][]byte
	survivalSwarmW           = [4]int32{158, 121, 144, 118}
	survivalSwarmH           = [4]int32{149, 112, 119, 119}
	survivalSwarmGuide       []byte
	survivalGuideScroll      int32
)

func initSurvivalExpansionAssets() {
	survivalExpansionSprites[survivalEnemySplitter] = readExternalBytes("survival", "enemies", "expansion", "splitter.bgra")
	survivalExpansionSprites[survivalEnemyShieldDrone] = readExternalBytes("survival", "enemies", "expansion", "shield_drone.bgra")
	survivalExpansionSprites[survivalEnemyPulseMine] = readExternalBytes("survival", "enemies", "expansion", "pulse_mine.bgra")
	survivalExpansionSprites[survivalEnemyInterceptor] = readExternalBytes("survival", "enemies", "expansion", "interceptor.bgra")
	survivalExpansionSprites[survivalEnemyPhantom] = readExternalBytes("survival", "enemies", "expansion", "phantom.bgra")
	survivalExpansionSprites[survivalEnemyGravityNode] = readExternalBytes("survival", "enemies", "expansion", "gravity_node.bgra")
	survivalExpansionSprites[survivalEnemyArmouredSatellite] = readExternalBytes("survival", "enemies", "expansion", "armoured_satellite.bgra")
	for i := 0; i < 4; i++ {
		survivalSwarmSprites[i] = readExternalBytes("survival", "enemies", "expansion", fmt.Sprintf("swarm_%d.bgra", i+1))
	}
	survivalSwarmGuide = readExternalBytes("survival", "enemies", "expansion", "swarm_guide.bgra")
}

func survivalEnemyFirstWave(kind int) int {
	switch kind {
	case survivalEnemyScout:
		return 1
	case survivalEnemyHeavy:
		return 3
	case survivalEnemySplitter:
		return 11
	case survivalEnemyShieldDrone:
		return 13
	case survivalEnemyPulseMine:
		return 15
	case survivalEnemyInterceptor:
		return 18
	case survivalEnemyPhantom:
		return 21
	case survivalEnemySwarmDrone:
		return 23
	case survivalEnemyGravityNode:
		return 26
	case survivalEnemyArmouredSatellite:
		return 28
	}
	return 99
}

func survivalEnemyName(kind int) string {
	return []string{"Scout", "Heavy", "Splitter", "Shield Drone", "Pulse Mine", "Interceptor", "Phantom", "Swarm Drone", "Gravity Node", "Armoured Satellite"}[kind]
}

func survivalEnemyRarity(kind int) string {
	if kind <= survivalEnemyHeavy {
		return "COMMON"
	}
	if kind == survivalEnemyArmouredSatellite || kind == survivalEnemyGravityNode {
		return "VERY RARE"
	}
	return "RARE"
}

func survivalEnemyDescription(kind int) string {
	switch kind {
	case survivalEnemyScout:
		return "Standard frontline attacker."
	case survivalEnemyHeavy:
		return "Durable station attacker with increased impact damage."
	case survivalEnemySplitter:
		return "Neutral hybrid enemy that breaks into two coloured Scouts."
	case survivalEnemyShieldDrone:
		return "Protected drone carrying a disposable energy shield."
	case survivalEnemyPulseMine:
		return "Area-denial mine that alternates between movement and pulse phases."
	case survivalEnemyInterceptor:
		return "Fast assault craft built around sudden directional bursts."
	case survivalEnemyPhantom:
		return "Phase-shifting attacker that periodically becomes untargetable."
	case survivalEnemySwarmDrone:
		return "Four-unit drone formation that behaves as a single encounter."
	case survivalEnemyGravityNode:
		return "Control unit that distorts nearby enemy movement."
	case survivalEnemyArmouredSatellite:
		return "Slow siege target with heavy armour and high core damage."
	}
	return ""
}

func survivalEnemyBehaviour(kind int) string {
	switch kind {
	case survivalEnemyScout:
		return "Fast standard attacker that flies directly towards the station."
	case survivalEnemyHeavy:
		return "Slower, larger attacker with extra health and higher station damage."
	case survivalEnemySplitter:
		return "Neutral enemy. Either mouse button destroys it, then it breaks into two red/blue Scouts; the Scouts themselves still require the matching mouse button."
	case survivalEnemyShieldDrone:
		return "Starts protected by an energy shield. Either mouse button breaks the shield; either button can then damage the exposed drone."
	case survivalEnemyPulseMine:
		return "Drifts into the arena, anchors, then emits timed danger pulses before advancing again."
	case survivalEnemyInterceptor:
		return "Makes short high-speed bursts and changes its approach angle while closing on the station. Either mouse button can hit it."
	case survivalEnemyPhantom:
		return "Cycles between visible and phased states. It can only be hit while fully visible, but either mouse button works when exposed."
	case survivalEnemySwarmDrone:
		return "Always appears as a formation of four. Each hit removes one drone from the group, and either mouse button works."
	case survivalEnemyGravityNode:
		return "Anchors temporarily and creates a gravity field that bends nearby enemy movement towards it."
	case survivalEnemyArmouredSatellite:
		return "Very slow, heavily armoured target that takes several hits to destroy. Either mouse button works."
	}
	return ""
}

func survivalEnemyCounter(kind int) string {
	switch kind {
	case survivalEnemyScout:
		return "Read the colour and remove it before it reaches the inner defence ring."
	case survivalEnemyHeavy:
		return "Commit to both required hits early; leaving it late can cost multiple HP."
	case survivalEnemySplitter:
		return "Click it with either button. After it splits, read each Scout colour and react to the pair that appears."
	case survivalEnemyShieldDrone:
		return "Use one hit to strip the shield, then immediately hit the exposed drone again; either mouse button works."
	case survivalEnemyPulseMine:
		return "Destroy it before its pulse cycle completes; do not hover inside the warning ring."
	case survivalEnemyInterceptor:
		return "Track the burst movement and click after each dash instead of predicting too far ahead."
	case survivalEnemyPhantom:
		return "Wait for the sprite to become solid. Clicking it while phased counts as a misclick."
	case survivalEnemySwarmDrone:
		return "Treat the four-drone formation as one target and clear all four health segments; either mouse button works."
	case survivalEnemyGravityNode:
		return "Prioritise it when other enemies are nearby so it cannot distort several paths at once."
	case survivalEnemyArmouredSatellite:
		return "Start attacking it early; it is intentionally too durable to leave until the last moment."
	}
	return ""
}

func survivalEnemyHazard(kind int) string {
	switch kind {
	case survivalEnemyPulseMine:
		return "Cursor inside an active pulse ring damages the station once per pulse."
	case survivalEnemyPhantom:
		return "Phased body is non-clickable; attempting to hit it is treated as empty space."
	case survivalEnemySwarmDrone:
		return "Formation is capped at one active group at a time."
	case survivalEnemyGravityNode:
		return "Only one Gravity Node can be active at once. Its field affects enemies, not the cursor."
	case survivalEnemyArmouredSatellite:
		return "Deals 3 station damage if it reaches the core."
	default:
		return "Either mouse button can damage this enemy."
	}
}

func survivalKindCount(kind int) int {
	n := 0
	for _, e := range survivalEnemies {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func survivalSpecialCount() int {
	n := 0
	for _, e := range survivalEnemies {
		if e.Kind >= survivalEnemySplitter {
			n++
		}
	}
	return n
}

func survivalChooseEnemyKind() int {
	// Standard enemies remain the backbone of every wave. Specials are deliberately sparse.
	if survivalWave < 11 {
		if rand.Float64() < .22 {
			return survivalEnemyHeavy
		}
		return survivalEnemyScout
	}
	specialCap := 2
	if survivalWave >= 21 {
		specialCap = 3
	}
	if survivalSpecialCount() >= specialCap || rand.Float64() > .31 {
		if rand.Float64() < .24 {
			return survivalEnemyHeavy
		}
		return survivalEnemyScout
	}
	candidates := make([]int, 0, 8)
	weights := make([]int, 0, 8)
	for k := survivalEnemySplitter; k <= survivalEnemyArmouredSatellite; k++ {
		if survivalWave < survivalEnemyFirstWave(k) {
			continue
		}
		if (k == survivalEnemyPulseMine || k == survivalEnemyGravityNode || k == survivalEnemySwarmDrone) && survivalKindCount(k) > 0 {
			continue
		}
		if k == survivalEnemyArmouredSatellite && survivalKindCount(k) > 0 {
			continue
		}
		w := 10
		if k == survivalEnemyGravityNode || k == survivalEnemyArmouredSatellite {
			w = 5
		}
		if survivalWave-survivalEnemyFirstWave(k) <= 1 {
			w += 5
		}
		candidates = append(candidates, k)
		weights = append(weights, w)
	}
	if len(candidates) == 0 {
		return survivalEnemyScout
	}
	total := 0
	for _, w := range weights {
		total += w
	}
	roll := rand.Intn(total)
	for i, w := range weights {
		if roll < w {
			return candidates[i]
		}
		roll -= w
	}
	return survivalEnemyScout
}

func survivalApplyKindDefaults(e *SurvivalEnemy) {
	if e.Kind == survivalEnemyHeavy {
		e.Large = true
		if e.HP < 2 {
			e.HP = 2
		}
		return
	}
	switch e.Kind {
	case survivalEnemySplitter:
		e.HP = 1
		e.SpecialTimer = time.Now().Add(700 * time.Millisecond)
	case survivalEnemyShieldDrone:
		e.HP = 1
		e.Special = 1 // shield intact
	case survivalEnemyPulseMine:
		e.HP = 1
		e.Special = 0
		e.SpecialTimer = time.Now().Add(1800 * time.Millisecond)
	case survivalEnemyInterceptor:
		e.HP = 1
		e.SpecialTimer = time.Now().Add(time.Duration(700+rand.Intn(450)) * time.Millisecond)
	case survivalEnemyPhantom:
		e.HP = 1
		e.Special = 1
		e.SpecialTimer = time.Now().Add(1100 * time.Millisecond)
	case survivalEnemySwarmDrone:
		e.HP = 4
		e.Special = 4
	case survivalEnemyGravityNode:
		e.HP = 2
		e.SpecialTimer = time.Now().Add(4200 * time.Millisecond)
	case survivalEnemyArmouredSatellite:
		e.HP = 4
		e.Large = true
	}
}

func survivalExpansionSpeedScale(kind int) float64 {
	switch kind {
	case survivalEnemySplitter:
		return .92
	case survivalEnemyShieldDrone:
		return .82
	case survivalEnemyPulseMine:
		return .58
	case survivalEnemyInterceptor:
		return 1.25
	case survivalEnemyPhantom:
		return 1.02
	case survivalEnemySwarmDrone:
		return .88
	case survivalEnemyGravityNode:
		return .48
	case survivalEnemyArmouredSatellite:
		return .55
	}
	return 1
}

func survivalUpdateSpecialEnemies(now time.Time, w, h int32, dt float64) {
	c := survivalArenaCenter(w, h)
	// Gravity nodes bend nearby enemy velocity toward the field centre. This is bounded and never affects the cursor.
	for i := range survivalEnemies {
		if survivalEnemies[i].Kind != survivalEnemyGravityNode {
			continue
		}
		g := &survivalEnemies[i]
		if now.After(g.SpecialTimer) { // resumes station approach after anchoring window
			dx, dy := c.X-g.X, c.Y-g.Y
			d := math.Hypot(dx, dy)
			if d > 1 {
				s := math.Hypot(g.VX, g.VY)
				g.VX = dx / d * s
				g.VY = dy / d * s
			}
		} else {
			g.VX = 0
			g.VY = 0
		}
		for j := range survivalEnemies {
			if i == j || survivalEnemies[j].Kind == survivalEnemyGravityNode {
				continue
			}
			o := &survivalEnemies[j]
			dx, dy := g.X-o.X, g.Y-o.Y
			d := math.Hypot(dx, dy)
			if d > 1 && d < float64(sx(190, w)) {
				pull := 18.0 * (1 - d/float64(sx(190, w)))
				o.VX += dx / d * pull * dt * 12
				o.VY += dy / d * pull * dt * 12
			}
		}
	}
	for i := range survivalEnemies {
		e := &survivalEnemies[i]
		switch e.Kind {
		case survivalEnemyPulseMine:
			// Alternates between a stationary 0.9s warning/active pulse and a short advance.
			if now.Before(e.SpecialTimer) {
				e.VX = 0
				e.VY = 0
			} else {
				if e.Special == 0 {
					e.Special = 1
					e.SpecialTimer = now.Add(900 * time.Millisecond)
					e.VX = 0
					e.VY = 0
				} else {
					e.Special = 0
					e.SpecialTimer = now.Add(1500 * time.Millisecond)
					dx, dy := c.X-e.X, c.Y-e.Y
					d := math.Hypot(dx, dy)
					if d > 1 {
						e.VX = dx / d * 50
						e.VY = dy / d * 50
					}
				}
			}
		case survivalEnemyInterceptor:
			if !now.Before(e.SpecialTimer) { // burst with a mild tangent component, then re-aim next cycle
				dx, dy := c.X-e.X, c.Y-e.Y
				d := math.Hypot(dx, dy)
				if d > 1 {
					side := 1.0
					if rand.Intn(2) == 0 {
						side = -1
					}
					speed := 150.0 + float64(survivalWave)
					e.VX = (dx / d * speed) + (-dy / d * 52 * side)
					e.VY = (dy / d * speed) + (dx / d * 52 * side)
				}
				e.SpecialTimer = now.Add(time.Duration(850+rand.Intn(500)) * time.Millisecond)
			}
		case survivalEnemyPhantom:
			if !now.Before(e.SpecialTimer) {
				if e.Special == 1 {
					e.Special = 0
					e.SpecialTimer = now.Add(650 * time.Millisecond)
				} else {
					e.Special = 1
					e.SpecialTimer = now.Add(1150 * time.Millisecond)
				}
			}
		}
	}
}

func survivalPulseMineHazard(h uintptr, now time.Time, w, hgt int32) bool {
	if !cursorInArena {
		return true
	}
	for i := range survivalEnemies {
		e := &survivalEnemies[i]
		if e.Kind != survivalEnemyPulseMine || e.Special != 1 {
			continue
		}
		// last 300ms of the active phase is the damaging pulse; one hit per activation.
		if time.Until(e.SpecialTimer) > 300*time.Millisecond || e.HazardLatched {
			continue
		}
		if math.Hypot(cursorPos.X-e.X, cursorPos.Y-e.Y) <= float64(sx(92, w)) {
			e.HazardLatched = true
			if survivalReviveImmune(now) {
				status = "REVIVE IMMUNITY // PULSE ABSORBED"
				continue
			}
			survivalHP--
			survivalWaveDamageTaken = true
			survivalTotalDamageTaken++
			playSurvivalDamageTakenSound()
			status = "PULSE MINE — -1 HP"
			invalidateSurvivalHUD(h)
			if survivalHP <= 0 {
				survivalHP = 0
				survivalFail(h, "Pulse mine overload")
				return false
			}
		}
	}
	return true
}

func survivalSpecialClickable(e SurvivalEnemy) bool {
	return !(e.Kind == survivalEnemyPhantom && e.Special == 0)
}

func survivalSpecialClickPreDamage(e *SurvivalEnemy) (handled bool, destroyed bool) {
	if e.Kind == survivalEnemyShieldDrone && e.Special == 1 {
		e.Special = 0
		status = "SHIELD BROKEN — DRONE EXPOSED"
		return true, false
	}
	return false, false
}

func survivalSpecialImpactDamage(e SurvivalEnemy) int {
	switch e.Kind {
	case survivalEnemyArmouredSatellite:
		return 3
	case survivalEnemyHeavy:
		return 2
	case survivalEnemySwarmDrone:
		return 2
	default:
		return 1
	}
}

func survivalSpawnSplitterChildren(parent SurvivalEnemy, w, h int32) {
	c := survivalArenaCenter(w, h)
	// Three equally valid outcomes: red/red, blue/blue, or one of each.
	// The mixed outcome randomly swaps which side gets which colour so position
	// never becomes a reliable shortcut for identifying the required click.
	var childRed [2]bool
	switch rand.Intn(3) {
	case 0:
		childRed = [2]bool{true, true}
	case 1:
		childRed = [2]bool{false, false}
	default:
		if rand.Intn(2) == 0 {
			childRed = [2]bool{true, false}
		} else {
			childRed = [2]bool{false, true}
		}
	}
	for n := 0; n < 2; n++ {
		ang := math.Atan2(c.Y-parent.Y, c.X-parent.X) + (-.30 + float64(n)*.60)
		speed := 88.0 + float64(survivalWave)*1.5
		child := SurvivalEnemy{X: parent.X + math.Cos(ang)*14, Y: parent.Y + math.Sin(ang)*14, VX: math.Cos(ang) * speed, VY: math.Sin(ang) * speed, Red: childRed[n], HP: 1, SpawnedAt: time.Now(), Kind: survivalEnemyScout}
		if survivalNormalWaveOwnsArena() && survivalActiveEnemySlots()+survivalEnemySlotWeight(child) <= survivalMaxAlive() {
			survivalEnemies = append(survivalEnemies, child)
		}
	}
}

func survivalExpansionVisualRadius(e SurvivalEnemy, w int32) (float64, bool) {
	switch e.Kind {
	case survivalEnemySplitter, survivalEnemyShieldDrone:
		return float64(sx(43, w)), true
	case survivalEnemyPulseMine:
		return float64(sx(40, w)), true
	case survivalEnemyInterceptor:
		return float64(sx(44, w)), true
	case survivalEnemyPhantom:
		return float64(sx(42, w)), true
	case survivalEnemySwarmDrone:
		return float64(sx(52, w)), true
	case survivalEnemyGravityNode:
		return float64(sx(46, w)), true
	case survivalEnemyArmouredSatellite:
		return float64(sx(61, w)), true
	}
	return 0, false
}

func survivalExpansionFacesStation(kind int) bool {
	switch kind {
	case survivalEnemySplitter,
		survivalEnemyShieldDrone,
		survivalEnemyPulseMine,
		survivalEnemyInterceptor,
		survivalEnemyPhantom,
		survivalEnemySwarmDrone,
		survivalEnemyGravityNode,
		survivalEnemyArmouredSatellite:
		return true
	}
	return false
}

// Expansion artwork is authored facing to the right. Station-targeting enemies
// therefore rotate from that neutral right-facing pose to aim directly at the
// station centre, matching the directional language used by the original Scout
// and Heavy frames. This deliberately follows the target vector rather than the
// instantaneous velocity so Interceptor side-bursts still visually threaten the
// station instead of appearing to attack sideways.
func survivalExpansionAuthoredFacing(kind int) float64 {
	// Source-art forward direction in screen-space radians (+X = right, +Y = down).
	// Symmetric hazards use zero because their facing is visually neutral.
	switch kind {
	case survivalEnemySplitter:
		return -math.Pi / 2 // authored pointing up
	case survivalEnemyInterceptor:
		return 3 * math.Pi / 4 // authored pointing down-left
	case survivalEnemyPhantom:
		return -math.Pi / 2 // authored pointing up
	case survivalEnemySwarmDrone:
		return -math.Pi / 4 // authored pointing up-right
	case survivalEnemyArmouredSatellite:
		return math.Pi / 2 // authored pointing down
	default:
		return 0
	}
}

func survivalExpansionStationAngle(e SurvivalEnemy, w, h int32) float64 {
	c := survivalArenaCenter(w, h)
	target := math.Atan2(c.Y-e.Y, c.X-e.X)
	return target - survivalExpansionAuthoredFacing(e.Kind)
}

func drawSurvivalExpansionEnemy(hdc uintptr, e SurvivalEnemy, w, h int32, now time.Time) bool {
	if e.Kind < survivalEnemySplitter {
		return false
	}
	rad, _ := survivalExpansionVisualRadius(e, w)
	if e.Kind == survivalEnemySwarmDrone {
		offsets := [4][2]int32{{-24, -20}, {24, -20}, {-24, 21}, {24, 21}}
		alive := e.HP
		if alive > 4 {
			alive = 4
		}
		angle := survivalExpansionStationAngle(e, w, h)
		for i := 0; i < alive; i++ {
			rr := int32(rad * .48)
			q := FPoint{X: e.X + float64(offsets[i][0]), Y: e.Y + float64(offsets[i][1])}
			drawBoss2RotatedBGRA(hdc, survivalSwarmSprites[i], survivalSwarmW[i], survivalSwarmH[i], q, rr*2, rr*2, angle)
		}
		return true
	}
	data := survivalExpansionSprites[e.Kind]
	iw, ih := survivalExpansionW[e.Kind], survivalExpansionH[e.Kind]
	if len(data) == 0 {
		return false
	}
	rr := int32(rad)
	if e.Kind == survivalEnemyPhantom && e.Special == 0 {
		// Ghost state: draw only intermittent outline/glitch bars so the non-clickable state is obvious.
		if (now.UnixNano()/100000000)%2 == 0 {
			drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(104, 102, 255)), int32(e.X), int32(e.Y), rr)
		}
		return true
	}
	if survivalExpansionFacesStation(e.Kind) {
		drawBoss2RotatedBGRA(hdc, data, iw, ih, FPoint{X: e.X, Y: e.Y}, rr*2, rr*2, survivalExpansionStationAngle(e, w, h))
	} else {
		drawRawBGRAFit(hdc, data, iw, ih, RECT{int32(e.X) - rr, int32(e.Y) - rr, int32(e.X) + rr, int32(e.Y) + rr})
	}
	switch e.Kind {
	case survivalEnemyShieldDrone:
		if e.Special == 1 {
			drawSurvivalCircleWithPen(hdc, survivalPen(3, rgb(86, 205, 255)), int32(e.X), int32(e.Y), rr+sx(7, w))
		}
	case survivalEnemyPulseMine:
		if e.Special == 1 {
			p := .5 + .5*math.Sin(float64(now.UnixNano())/1e9*10)
			ring := rr + int32(float64(sx(50, w))*p)
			drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(255, 70, 95)), int32(e.X), int32(e.Y), ring)
		}
	case survivalEnemyGravityNode:
		drawSurvivalCircleWithPen(hdc, survivalPen(1, rgb(159, 83, 255)), int32(e.X), int32(e.Y), sx(190, w))
	case survivalEnemyArmouredSatellite:
		for n := 0; n < e.HP-1 && n < 3; n++ {
			drawSurvivalCircleWithPen(hdc, survivalPen(1, rgb(255, 205, 72)), int32(e.X), int32(e.Y), rr+sx(float64(5+n*4), w))
		}
	}
	return true
}

func survivalGuideButtonRect(w, h int32) RECT {
	return RECT{sx(650, w), sy(986, h), sx(850, w), sy(1018, h)}
}

func drawSurvivalGuideButton(hdc uintptr, w, h int32) {
	if !survivalActive() || state != StateWaiting {
		return
	}
	r := survivalGuideButtonRect(w, h)
	face, light, dark := themedHUDPanelPalette(true)
	drawBevelPanel(hdc, r, blendColor(face, dark, 0.14), blendColor(light, rgb(135, 92, 255), 0.34), dark, 2)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(240, 235, 255))
		centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, hudTinyFont, r, "MONSTER GUIDE"), "MONSTER GUIDE")
		selectObject.Call(hdc, old)
	}
}

func survivalGuidePanelRect(w, h int32) RECT {
	return centeredPanel(w, h, 1240, 820)
}

func survivalGuideViewportRect(w, h int32) RECT {
	panel := survivalGuidePanelRect(w, h)
	return RECT{panel.Left + sx(36, w), panel.Top + sy(104, h), panel.Right - sx(46, w), panel.Bottom - sy(58, h)}
}

func survivalGuideMaxScroll(w, h int32) int32 {
	view := survivalGuideViewportRect(w, h)
	cardH := sy(210, h)
	gap := sy(14, h)
	total := int32(10)*cardH + int32(9)*gap
	maxScroll := total - (view.Bottom - view.Top)
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func clampSurvivalGuideScroll(w, h int32) {
	if survivalGuideScroll < 0 {
		survivalGuideScroll = 0
	}
	if m := survivalGuideMaxScroll(w, h); survivalGuideScroll > m {
		survivalGuideScroll = m
	}
}

func drawSurvivalGuideEnemyImage(hdc uintptr, kind int, r RECT) {
	if kind == survivalEnemySwarmDrone && len(survivalSwarmGuide) > 0 {
		drawRawBGRAFit(hdc, survivalSwarmGuide, 327, 312, r)
		return
	}
	if kind >= survivalEnemySplitter {
		d := survivalExpansionSprites[kind]
		drawRawBGRAFit(hdc, d, survivalExpansionW[kind], survivalExpansionH[kind], r)
		return
	}
	e := SurvivalEnemy{Kind: kind, Large: kind == survivalEnemyHeavy, Red: true, VX: 1}
	d, iw, ih := survivalEnemySprite(e)
	drawRawBGRAFit(hdc, d, iw, ih, r)
}

func drawSurvivalMonsterGuide(hdc uintptr, w, h int32) {
	if overlayMode != OverlaySurvivalMonsterGuide {
		return
	}
	drawModalBackdrop(hdc, w, h)
	panel := survivalGuidePanelRect(w, h)
	drawPanelFrame(hdc, panel, "MONSTER GUIDE", "SURVIVAL // REGULAR ENEMIES")

	view := survivalGuideViewportRect(w, h)
	clampSurvivalGuideScroll(w, h)
	cardH := sy(210, h)
	gap := sy(14, h)

	saved, _, _ := saveDC.Call(hdc)
	if saved != 0 {
		intersectClipRect.Call(hdc, uintptr(view.Left), uintptr(view.Top), uintptr(view.Right), uintptr(view.Bottom))
	}

	for kind := 0; kind < 10; kind++ {
		y := view.Top + int32(kind)*(cardH+gap) - survivalGuideScroll
		card := RECT{view.Left, y, view.Right - sx(18, w), y + cardH}
		if card.Bottom < view.Top || card.Top > view.Bottom {
			continue
		}

		unlocked := survivalEnemyFirstWave(kind) <= gameMeta.BestSurvivalWave || kind <= 1 || (gameMeta.SurvivalGuideMask&(1<<uint(kind))) != 0
		border := rgb(55, 178, 255)
		if kind >= survivalEnemySplitter {
			border = rgb(133, 91, 255)
		}
		guideFace, guideLight, guideDark := themedHUDCardPalette(1)
		drawBevelPanel(hdc, card, blendColor(guideFace, guideDark, 0.12), blendColor(guideLight, border, 0.34), guideDark, 2)

		img := RECT{card.Left + sx(16, w), card.Top + sy(16, h), card.Left + sx(172, w), card.Bottom - sy(16, h)}
		drawBevelPanel(hdc, img, blendColor(guideDark, guideFace, 0.10), blendColor(guideLight, guideDark, 0.36), guideDark, 1)
		if unlocked {
			padX := sx(18, w)
			padY := sy(14, h)
			drawSurvivalGuideEnemyImage(hdc, kind, RECT{img.Left + padX, img.Top + padY, img.Right - padX, img.Bottom - padY})
		}

		tx := img.Right + sx(22, w)
		right := card.Right - sx(18, w)
		name := "???"
		if unlocked {
			name = survivalEnemyName(kind)
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 255, 255))
			textOut(hdc, tx, card.Top+sy(16, h), name)
			if unlocked {
				setTextColor.Call(hdc, rgb(255, 211, 72))
				meta := fmt.Sprintf("WAVE %d   •   %s", survivalEnemyFirstWave(kind), survivalEnemyRarity(kind))
				textOut(hdc, tx, card.Top+sy(48, h), meta)
			} else {
				setTextColor.Call(hdc, rgb(140, 151, 174))
				textOut(hdc, tx, card.Top+sy(48, h), fmt.Sprintf("ENCOUNTER FROM WAVE %d TO UNLOCK", survivalEnemyFirstWave(kind)))
			}
			selectObject.Call(hdc, old)
		}

		if unlocked && hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			labelX := tx
			bodyX := tx + sx(108, w)
			rowY := card.Top + sy(78, h)

			setTextColor.Call(hdc, rgb(100, 208, 255))
			textOut(hdc, labelX, rowY, "DESCRIPTION")
			setTextColor.Call(hdc, rgb(231, 239, 249))
			drawWrappedLeftText(hdc, hudTinyFont, RECT{bodyX, rowY, right, rowY + sy(44, h)}, survivalEnemyDescription(kind), sy(1, h), 3)

			rowY += sy(46, h)
			setTextColor.Call(hdc, rgb(100, 208, 255))
			textOut(hdc, labelX, rowY, "BEHAVIOUR")
			setTextColor.Call(hdc, rgb(231, 239, 249))
			behaviour := fmt.Sprintf("%s  %s  %s", survivalEnemyBehaviour(kind), survivalEnemyCounter(kind), survivalEnemyHazard(kind))
			drawWrappedLeftText(hdc, hudTinyFont, RECT{bodyX, rowY, right, card.Bottom - sy(14, h)}, behaviour, sy(1, h), 5)
			selectObject.Call(hdc, old)
		}
	}

	if saved != 0 {
		restoreDC.Call(hdc, saved)
	}

	// Persistent scroll rail makes it obvious that this is one continuous guide.
	track := RECT{view.Right - sx(8, w), view.Top, view.Right - sx(2, w), view.Bottom}
	fillSolidRect(hdc, track, rgb(17, 31, 57))
	maxScroll := survivalGuideMaxScroll(w, h)
	if maxScroll > 0 {
		viewH := view.Bottom - view.Top
		cardH := sy(210, h)
		gap := sy(14, h)
		total := int32(10)*cardH + int32(9)*gap
		thumbH := int32(float64(viewH) * float64(viewH) / float64(total))
		if thumbH < sy(54, h) {
			thumbH = sy(54, h)
		}
		travel := viewH - thumbH
		thumbTop := view.Top + int32(float64(travel)*float64(survivalGuideScroll)/float64(maxScroll))
		fillSolidRect(hdc, RECT{track.Left, thumbTop, track.Right, thumbTop + thumbH}, rgb(92, 191, 255))
	}

}
