//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Starbase physical-world layer. Major progression systems are represented by
// persistent structures on the moon floor and can be clicked directly. The
// buildings are cosmetic/navigation only; all economy mutations remain in the
// existing guarded action handlers.

const (
	afkWorldCore = iota
	afkWorldScout
	afkWorldTech
	afkWorldOps
	afkWorldModules
	afkWorldDrone
	afkWorldOrbital
	afkWorldBuildingCount
)

func afkClamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// afkEase01 is used by world-only animation so moving props accelerate and
// decelerate instead of snapping between constant-speed segments.
func afkEase01(v float64) float64 {
	v = afkClamp01(v)
	return v * v * v * (v*(v*6-15) + 10)
}

func afkPingPongEase(now time.Time, seconds float64, phase float64) float64 {
	if seconds <= 0 {
		return 0
	}
	u := math.Mod(float64(now.UnixMilli())/1000.0/seconds+phase, 2.0)
	if u > 1 {
		u = 2 - u
	}
	return afkEase01(u)
}

func afkSoftPulse(now time.Time, seconds float64, phase float64) float64 {
	if seconds <= 0 {
		return 0.5
	}
	return 0.5 + 0.5*math.Sin((float64(now.UnixMilli())/1000.0/seconds+phase)*2*math.Pi)
}

func afkRGBLerp(aR, aG, aB, bR, bG, bB int, t float64) uintptr {
	t = afkClamp01(t)
	lerp := func(a, b int) byte { return byte(float64(a) + (float64(b)-float64(a))*t) }
	return rgb(lerp(aR, bR), lerp(aG, bG), lerp(aB, bB))
}

var (
	afkWorldSnapshotInit      bool
	afkWorldBuiltSnapshot     [afkWorldBuildingCount]bool
	afkWorldConstructionIndex = -1
	afkWorldConstructionStart time.Time
	afkWorldHoverIndex        = -1
	afkWelcomeCountStart      time.Time

	afkFlybyNextAt    time.Time
	afkFlybyStartedAt time.Time
	afkFlybyShipID    = -1
	afkFlybyYFraction float64
	afkFlybyLastRect  RECT
)

func afkMoonSurfaceY(w, hgt int32) int32 {
	const srcW int32 = 2048
	const srcH int32 = 176
	field := afkMainFieldRect(w, hgt)
	fieldW := field.Right - field.Left
	dstH := int32(float64(fieldW) * float64(srcH) / float64(srcW))
	minH := sy(84, hgt)
	maxH := sy(132, hgt)
	if dstH < minH {
		dstH = minH
	}
	if maxH > 0 && dstH > maxH {
		dstH = maxH
	}
	return field.Bottom - dstH + sy(27, hgt)
}

func afkWorldBuilt(index int) bool {
	switch index {
	case afkWorldCore:
		return afkCursorCoreBuilt()
	case afkWorldScout:
		return afkScoutUnlocked()
	case afkWorldTech:
		return afkTechLabUnlocked()
	case afkWorldOps:
		return afkOperatorsAvailable()
	case afkWorldModules:
		return afkModulesAvailable()
	case afkWorldDrone:
		return afkDroneBayBuilt()
	case afkWorldOrbital:
		return afkOrbitalBuilt()
	}
	return false
}

func afkWorldBuildingRect(w, hgt int32, index int) RECT {
	field := afkMainFieldRect(w, hgt)
	baseY := afkMoonSurfaceY(w, hgt) + sy(50, hgt)
	// v442: facilities sit directly on the authored moon surface with no artificial
	// placement rings. Pronounced, irregular ground heights break the ruler-straight
	// lineup and make the base read as a settlement built across uneven moon terrain.
	fracs := [...]float64{0.11, 0.24, 0.37, 0.50, 0.63, 0.76, 0.89}
	yOffsets := [...]float64{8, -24, 14, -36, 5, 31, -17}
	widths := [...]float64{112, 104, 112, 110, 106, 122, 124}
	heights := [...]float64{80, 56, 72, 62, 60, 74, 94}
	if index < 0 || index >= len(fracs) {
		return RECT{}
	}
	cx := field.Left + int32(float64(field.Right-field.Left)*fracs[index])
	y := baseY + sy(yOffsets[index], hgt)
	// v444 player-directed placement nudges. These are applied after the broader
	// stagger so the three named facilities land exactly where requested.
	switch index {
	case afkWorldCore:
		cx -= sx(13, w)
		y += sy(8, hgt)
	case afkWorldScout:
		y += sy(18, hgt)
	case afkWorldOps:
		y += sy(18, hgt)
	}
	ww := sx(widths[index]*1.875, w)
	hh := sy(heights[index]*1.875, hgt)
	return RECT{cx - ww/2, y - hh, cx + ww/2, y}
}

func afkWorldFacilityRingCenter(w, hgt int32, index int) (int32, int32) {
	r := afkWorldBuildingRect(w, hgt, index)
	return (r.Left + r.Right) / 2, r.Bottom - sy(2, hgt)
}

func drawAFKFloorEllipse(hdc uintptr, cx, cy, rx, ry, lw int32, c uintptr) {
	if rx <= 0 || ry <= 0 {
		return
	}
	const segments = 36
	var px, py int32
	for i := 0; i <= segments; i++ {
		a := float64(i) / float64(segments) * 2 * math.Pi
		x := cx + int32(math.Cos(a)*float64(rx))
		y := cy + int32(math.Sin(a)*float64(ry))
		if i > 0 {
			drawLineSimple(hdc, px, py, x, y, int(lw), c)
		}
		px, py = x, y
	}
}

func drawAFKFacilityFloorRings(hdc uintptr, w, hgt int32) {
	if !afkPrimaryWorldVisible() {
		return
	}
	now := time.Now()
	pulse := afkSoftPulse(now, 2.4, 0)
	for i := 0; i < afkWorldBuildingCount; i++ {
		cx, cy := afkWorldFacilityRingCenter(w, hgt, i)
		rx := sx(39, w)
		ry := sy(10, hgt)
		c := afkRGBLerp(27, 76, 129, 54, 151, 214, .25+.35*pulse)
		if afkWorldBuilt(i) {
			c = afkRGBLerp(48, 143, 198, 86, 223, 255, .35+.45*pulse)
		}
		drawAFKFloorEllipse(hdc, cx, cy, rx, ry, max32(1, sx(1.2, w)), c)
		drawAFKFloorEllipse(hdc, cx, cy, rx-sx(6, w), max32(2, ry-sy(2, hgt)), max32(1, sx(1, w)), blendColor(c, rgb(8, 20, 46), 0.38))
	}
}

func afkWorldBuildingLabel(index int) string {
	return [...]string{"CURSOR CORE", "SCOUT PAD", "TECH LAB", "OPERATIONS", "FABRICATOR", "DRONE BAY", "ORBITAL"}[index]
}

func afkRefreshPhysicalBuildingTransitions(now time.Time) {
	var cur [afkWorldBuildingCount]bool
	for i := 0; i < afkWorldBuildingCount; i++ {
		cur[i] = afkWorldBuilt(i)
	}
	if !afkWorldSnapshotInit {
		afkWorldBuiltSnapshot = cur
		afkWorldSnapshotInit = true
		return
	}
	for i := 0; i < afkWorldBuildingCount; i++ {
		if cur[i] && !afkWorldBuiltSnapshot[i] {
			afkWorldConstructionIndex = i
			afkWorldConstructionStart = now
			afkQueueMilestone("SYSTEM ONLINE", afkWorldBuildingLabel(i)+" CONSTRUCTED")
			break
		}
	}
	afkWorldBuiltSnapshot = cur
}

func drawAFKPixelBuildingBase(hdc uintptr, r RECT, accent uintptr, tier int, blink bool) {
	w := r.Right - r.Left
	h := r.Bottom - r.Top
	baseY := r.Bottom - h/8
	// shadow + foundation
	fillSolidRect(hdc, RECT{r.Left + w/10, baseY - h/12, r.Right - w/10, baseY + h/14}, rgb(2, 6, 18))
	fillSolidRect(hdc, RECT{r.Left + w/7, baseY - h/9, r.Right - w/7, baseY}, rgb(30, 45, 69))
	fillSolidRect(hdc, RECT{r.Left + w/6, baseY - h/9, r.Right - w/6, baseY - h/13}, accent)
	// tiered central chassis
	bodyL := r.Left + w/4
	bodyR := r.Right - w/4
	bodyTop := r.Top + h/3
	fillSolidRect(hdc, RECT{bodyL, bodyTop, bodyR, baseY - h/10}, rgb(22, 39, 65))
	drawOutlineRect(hdc, RECT{bodyL, bodyTop, bodyR, baseY - h/10}, accent, 2)
	if tier >= 2 {
		fillSolidRect(hdc, RECT{r.Left + w/6, bodyTop + h/7, bodyL, baseY - h/7}, rgb(17, 31, 54))
		fillSolidRect(hdc, RECT{bodyR, bodyTop + h/7, r.Right - w/6, baseY - h/7}, rgb(17, 31, 54))
	}
	if tier >= 3 {
		fillSolidRect(hdc, RECT{r.Left + w/9, baseY - h/4, r.Left + w/5, baseY - h/10}, accent)
		fillSolidRect(hdc, RECT{r.Right - w/5, baseY - h/4, r.Right - w/9, baseY - h/10}, accent)
	}
	// windows/status lights
	light := accent
	if blink && !gameMeta.ReducedMotion && (time.Now().UnixMilli()/420)%2 == 0 {
		light = rgb(255, 235, 122)
	}
	for i := int32(0); i < 3; i++ {
		x := bodyL + (i+1)*(bodyR-bodyL)/4
		fillSolidRect(hdc, RECT{x - 2, bodyTop + h/8, x + 3, bodyTop + h/8 + 4}, light)
	}
}

func drawAFKCoreBuilding(hdc uintptr, r RECT) {
	if drawAFKFacilitySpriteGrounded(hdc, afkFacilityHubSprite(afkWorldCore), r) {
		return
	}
	tier := afkCoreTier()
	if tier < 1 {
		tier = 1
	}
	drawAFKPixelBuildingBase(hdc, r, rgb(75, 214, 255), tier, true)
	cx := (r.Left + r.Right) / 2
	towerTop := r.Top + (r.Bottom-r.Top)/8
	fillSolidRect(hdc, RECT{cx - 5, towerTop, cx + 6, r.Top + (r.Bottom-r.Top)/3}, rgb(166, 101, 255))
	fillSolidRect(hdc, RECT{cx - 2, towerTop - 13, cx + 3, towerTop}, rgb(231, 246, 255))
	if tier >= 4 {
		drawLineSimple(hdc, cx-24, towerTop+8, cx-36, towerTop-8, 2, rgb(75, 214, 255))
		drawLineSimple(hdc, cx+24, towerTop+8, cx+36, towerTop-8, 2, rgb(75, 214, 255))
	}
}

func drawAFKScoutPadWorld(hdc uintptr, r RECT, w, hgt int32) {
	if drawAFKFacilitySpriteGrounded(hdc, afkFacilityHubSprite(afkWorldScout), r) {
		// Separate scout ship asset animates independently from the permanent pad.
		baseY := r.Bottom - sy(8, hgt)
		visible := true
		shipY := baseY - sy(24, hgt)
		if afkExpeditionActive() {
			age := time.Since(time.Unix(gameMeta.AFKExpeditionStartedUnix, 0))
			if age >= 0 && age < 3*time.Second {
				shipY -= int32(float64(sy(125, hgt)) * float64(age) / (3 * float64(time.Second)))
			} else {
				visible = false
			}
		}
		if visible {
			cx := (r.Left + r.Right) / 2
			drawAFKFacilitySprite(hdc, "scout_ship", RECT{cx - sx(36, w), shipY - sy(22, hgt), cx + sx(36, w), shipY + sy(22, hgt)})
		}
		return
	}
	accent := rgb(80, 205, 255)
	baseY := r.Bottom - sy(8, hgt)
	fillSolidRect(hdc, RECT{r.Left + sx(8, w), baseY - sy(10, hgt), r.Right - sx(8, w), baseY}, rgb(25, 40, 61))
	drawOutlineRect(hdc, RECT{r.Left + sx(12, w), baseY - sy(13, hgt), r.Right - sx(12, w), baseY - sy(2, hgt)}, accent, 2)
	cx := (r.Left + r.Right) / 2
	// pad beacons
	phase := (time.Now().UnixMilli()/250)%2 == 0
	lc := rgb(87, 142, 174)
	if phase {
		lc = rgb(255, 211, 70)
	}
	fillSolidRect(hdc, RECT{r.Left + sx(12, w), baseY - sy(25, hgt), r.Left + sx(16, w), baseY - sy(16, hgt)}, lc)
	fillSolidRect(hdc, RECT{r.Right - sx(16, w), baseY - sy(25, hgt), r.Right - sx(12, w), baseY - sy(16, hgt)}, lc)
	// physical scout ship: on pad when idle/pending, takes off during first 3 sec of expedition.
	shipY := baseY - sy(26, hgt)
	visible := true
	if afkExpeditionActive() {
		idx := afkSelectedExpeditionIndex()
		dur := afkExpeditionDurationSeconds(idx)
		start := gameMeta.AFKExpeditionEndsUnix - dur
		elapsed := time.Now().Unix() - start
		if elapsed >= 0 && elapsed < 3 && !gameMeta.ReducedMotion {
			shipY -= sy(float64(elapsed)*38, hgt)
		} else {
			visible = false
		}
	}
	if visible {
		s := max32(2, sx(2.4, w))
		fillSolidRect(hdc, RECT{cx - s*6, shipY - s*2, cx + s*4, shipY + s*2}, rgb(47, 114, 165))
		fillSolidRect(hdc, RECT{cx - s*2, shipY - s*3, cx + s*3, shipY + s*3}, rgb(210, 236, 248))
		fillSolidRect(hdc, RECT{cx - s*7, shipY - s, cx - s*5, shipY + s}, rgb(178, 89, 255))
		if afkExpeditionRewardPending() {
			fillSolidRect(hdc, RECT{cx + s*5, shipY - s, cx + s*7, shipY + s}, rgb(82, 230, 145))
		}
	}
}

func drawAFKTechBuilding(hdc uintptr, r RECT) {
	if drawAFKFacilitySpriteGrounded(hdc, "tech_lab", r) {
		return
	}
	drawAFKPixelBuildingBase(hdc, r, rgb(184, 85, 255), 2, true)
	cx := (r.Left + r.Right) / 2
	y := r.Top + 10
	// scanning dome
	drawOutlineRect(hdc, RECT{cx - 18, y, cx + 18, y + 18}, rgb(83, 215, 255), 2)
	scan := int32(0)
	if !gameMeta.ReducedMotion {
		scan = int32((time.Now().UnixMilli()/80)%28) - 14
	}
	fillSolidRect(hdc, RECT{cx + scan - 2, y + 2, cx + scan + 2, y + 16}, rgb(232, 93, 255))
}

func drawAFKOpsBuilding(hdc uintptr, r RECT) {
	if drawAFKFacilitySpriteGrounded(hdc, "operations", r) {
		return
	}
	drawAFKPixelBuildingBase(hdc, r, rgb(88, 218, 145), 2, true)
	// antenna
	cx := (r.Left + r.Right) / 2
	drawLineSimple(hdc, cx, r.Top+8, cx, r.Top+24, 2, rgb(211, 238, 247))
	drawLineSimple(hdc, cx, r.Top+12, cx-10, r.Top+4, 1, rgb(88, 218, 145))
}

func drawAFKFabricatorBuilding(hdc uintptr, r RECT) {
	if drawAFKFacilitySpriteGrounded(hdc, "module_fabricator", r) {
		return
	}
	drawAFKPixelBuildingBase(hdc, r, rgb(255, 177, 55), 2, true)
	cx := (r.Left + r.Right) / 2
	// forge aperture
	fillSolidRect(hdc, RECT{cx - 14, r.Top + 20, cx + 14, r.Top + 34}, rgb(9, 14, 28))
	pulse := rgb(255, 112, 42)
	if !gameMeta.ReducedMotion && (time.Now().UnixMilli()/180)%2 == 0 {
		pulse = rgb(255, 222, 83)
	}
	fillSolidRect(hdc, RECT{cx - 7, r.Top + 24, cx + 7, r.Top + 31}, pulse)
}

func drawAFKDroneBayBuilding(hdc uintptr, r RECT, w, hgt int32) {
	if drawAFKFacilitySpriteGrounded(hdc, afkFacilityHubSprite(afkWorldDrone), r) {
		if gameMeta.AFKDronesDeployed > 0 && !gameMeta.ReducedMotion {
			phase := float64(time.Now().UnixMilli()%2600) / 2600.0
			cx := (r.Left+r.Right)/2 + int32((phase-.5)*float64(r.Right-r.Left))
			cy := r.Top - sy(8, hgt) - int32(math.Sin(phase*math.Pi)*float64(sy(18, hgt)))
			drawAFKFacilitySprite(hdc, fmt.Sprintf("mining_drone_%02d", 1+int(time.Now().Unix()/2)%4), RECT{cx - sx(18, w), cy - sy(14, hgt), cx + sx(18, w), cy + sy(14, hgt)})
		}
		return
	}
	tier := afkDroneBayTier()
	if tier < 1 {
		tier = 1
	}
	drawAFKPixelBuildingBase(hdc, r, rgb(79, 155, 255), tier, true)
	// hangar door
	cx := (r.Left + r.Right) / 2
	base := r.Bottom - sy(18, hgt)
	fillSolidRect(hdc, RECT{cx - sx(20, w), base - sy(18, hgt), cx + sx(20, w), base}, rgb(2, 12, 27))
	drawOutlineRect(hdc, RECT{cx - sx(20, w), base - sy(18, hgt), cx + sx(20, w), base}, rgb(86, 208, 255), 2)
	if !gameMeta.ReducedMotion && gameMeta.AFKDronesDeployed > 0 {
		phase := float64(time.Now().UnixMilli()%2400) / 2400
		x := cx + int32((phase-.5)*float64(sx(70, w)))
		y := r.Top + sy(6, hgt) - int32(math.Sin(phase*math.Pi)*float64(sy(18, hgt)))
		drawAFKDroneSprite(hdc, x, y, max32(2, sx(1.5, w)), rgb(86, 208, 255))
	}
}

func drawAFKOrbitalBuilding(hdc uintptr, r RECT, w, hgt int32) {
	if drawAFKFacilitySpriteGrounded(hdc, afkFacilityHubSprite(afkWorldOrbital), r) {
		return
	}
	tier := afkOrbitalTier()
	if tier < 1 {
		tier = 1
	}
	drawAFKPixelBuildingBase(hdc, r, rgb(255, 95, 205), tier, true)
	cx := (r.Left + r.Right) / 2
	// angled cannon facing Singularity
	y := r.Top + sy(22, hgt)
	fillSolidRect(hdc, RECT{cx - sx(9, w), y, cx + sx(9, w), y + sy(12, hgt)}, rgb(119, 94, 219))
	drawLineSimple(hdc, cx, y, cx-sx(24, w), y-sy(20, hgt), 4, rgb(184, 207, 225))
	if afkOrbitalNextFireIn(time.Now()) <= 0 {
		fillSolidRect(hdc, RECT{cx - sx(2, w), y - sy(4, hgt), cx + sx(3, w), y + sy(3, hgt)}, rgb(255, 235, 125))
	}
}

func drawAFKConstructionHologram(hdc uintptr, w, hgt int32) {
	if afkWorldConstructionIndex < 0 || afkWorldConstructionStart.IsZero() {
		return
	}
	age := time.Since(afkWorldConstructionStart)
	const total = 2400 * time.Millisecond
	if age < 0 || age > total {
		afkWorldConstructionIndex = -1
		return
	}
	r := afkWorldBuildingRect(w, hgt, afkWorldConstructionIndex)
	t := afkEase01(float64(age) / float64(total))
	name := afkWorldBuildingSpriteName(afkWorldConstructionIndex)

	// Build the actual silhouette up with layered holographic copies instead of
	// putting a temporary rectangle around the facility.
	pulse := afkSoftPulse(time.Now(), 0.85, 0)
	pad := sx(3+4*pulse, w)
	q := RECT{r.Left - pad, r.Top - pad, r.Right + pad, r.Bottom + pad}
	drawAFKFacilityGlowSpriteGrounded(hdc, name, q, byte(42+110*t))
	drawAFKFacilityGlowSpriteGrounded(hdc, name, r, byte(35+80*t))

	// A scan line rises through the building during assembly and eases at both
	// ends so construction feels deliberate rather than like a flashing box.
	scanT := afkEase01(math.Min(1, float64(age)/float64(1850*time.Millisecond)))
	scanY := r.Bottom - int32(float64(r.Bottom-r.Top)*scanT)
	drawLineSimple(hdc, r.Left+sx(6, w), scanY, r.Right-sx(6, w), scanY, 1, rgb(110, 231, 255))

	// Sparse sparks rise from the foundation with staggered deterministic phases.
	for i := 0; i < 9; i++ {
		u := math.Mod(float64(age.Milliseconds())/1150.0+float64(i)*0.173, 1)
		u = afkEase01(u)
		xBase := r.Left + int32(float64(r.Right-r.Left)*(0.12+0.76*float64((i*37)%100)/100.0))
		y := r.Bottom - sy(5, hgt) - int32(u*float64(sy(58, hgt)))
		drift := int32(math.Sin(float64(i)*1.7+u*math.Pi*2) * float64(sx(5, w)))
		c := rgb(83, 218, 255)
		if i%3 == 0 {
			c = rgb(202, 112, 255)
		}
		fillSolidRect(hdc, RECT{xBase + drift - 1, y - 1, xBase + drift + 2, y + 2}, c)
	}
}

func afkWorldBuildingSpriteName(index int) string {
	return afkFacilityHubSprite(index)
}

func drawAFKFacilityAmbientGlow(hdc uintptr, index int, r RECT, w, hgt int32) {
	// Sprite-shaped breathing halo: expanded silhouette copies hug the facility
	// artwork itself instead of drawing a rectangle around the hit box.
	phase := (math.Sin(float64(time.Now().UnixMilli())/520.0) + 1.0) * 0.5
	pad1 := int32(2 + phase*3)
	pad2 := pad1 + sx(3, w)
	name := afkWorldBuildingSpriteName(index)
	q2 := RECT{r.Left - pad2, r.Top - pad2, r.Right + pad2, r.Bottom + pad2}
	q1 := RECT{r.Left - pad1, r.Top - pad1, r.Right + pad1, r.Bottom + pad1}
	drawAFKFacilityGlowSpriteGrounded(hdc, name, q2, byte(70+phase*45))
	drawAFKFacilityGlowSpriteGrounded(hdc, name, q1, byte(95+phase*65))
}

func drawAFKFacilityHoverFlash(hdc uintptr, index int, r RECT, w, hgt int32) {
	// Hover uses the same silhouette, brighter and tighter. This preserves the
	// irregular outline of each facility and removes the old rectangular box.
	phase := (math.Sin(float64(time.Now().UnixMilli())/115.0) + 1.0) * 0.5
	pad := int32(3 + phase*3)
	q := RECT{r.Left - pad, r.Top - pad, r.Right + pad, r.Bottom + pad}
	drawAFKFacilityGlowSpriteGrounded(hdc, afkWorldBuildingSpriteName(index), q, byte(150+phase*80))
}

const afkFlybyDuration = 8500 * time.Millisecond

func afkScheduleNextFlyby(now time.Time) {
	// One unlocked ship crosses the Starbase every 2-5 minutes. A fresh random
	// delay is chosen after every pass/claim so the event never becomes clockwork.
	delaySeconds := 120 + rand.Intn(181)
	afkFlybyNextAt = now.Add(time.Duration(delaySeconds) * time.Second)
}

func afkChooseUnlockedFlybyShip() int {
	choices := make([]int, 0, len(garageShipOrder))
	for _, id := range garageShipOrder {
		if id >= 0 && id < len(spaceShipDefs) && shipUnlocked(id) {
			choices = append(choices, id)
		}
	}
	if len(choices) == 0 {
		return 0
	}
	return choices[rand.Intn(len(choices))]
}

func afkUpdateUnlockedShipFlyby(now time.Time) {
	if !afkScoutUnlocked() {
		return
	}
	if afkFlybyShipID >= 0 {
		if !afkFlybyStartedAt.IsZero() && now.Sub(afkFlybyStartedAt) >= afkFlybyDuration {
			afkFlybyShipID = -1
			afkFlybyStartedAt = time.Time{}
			afkFlybyLastRect = RECT{}
			afkScheduleNextFlyby(now)
		}
		return
	}
	if afkFlybyNextAt.IsZero() {
		afkScheduleNextFlyby(now)
		return
	}
	if now.Before(afkFlybyNextAt) || afkAnyManagementPanelOpen() {
		return
	}
	afkFlybyShipID = afkChooseUnlockedFlybyShip()
	afkFlybyStartedAt = now
	// Keep the ship in the upper/middle sky so it never collides visually with
	// the facility silhouettes or command rail.
	afkFlybyYFraction = 0.16 + rand.Float64()*0.48
	afkFlybyNextAt = time.Time{}
}

func afkUnlockedShipFlybyRect(w, hgt int32, now time.Time) (RECT, bool) {
	afkUpdateUnlockedShipFlyby(now)
	if afkFlybyShipID < 0 || afkFlybyStartedAt.IsZero() {
		return RECT{}, false
	}
	age := now.Sub(afkFlybyStartedAt)
	if age < 0 || age >= afkFlybyDuration {
		return RECT{}, false
	}
	field := afkMainFieldRect(w, hgt)
	surface := afkMoonSurfaceY(w, hgt)
	shipW := sx(104, w)
	data, sw, sh := shipTextureDataAndSize(afkFlybyShipID)
	_ = data
	shipH := sy(54, hgt)
	if sw > 0 && sh > 0 {
		shipH = int32(float64(shipW) * float64(sh) / float64(sw))
		maxH := sy(70, hgt)
		if shipH > maxH {
			shipH = maxH
		}
	}
	u := afkEase01(float64(age) / float64(afkFlybyDuration))
	startX := field.Left - shipW
	endX := field.Right + shipW
	cx := startX + int32(u*float64(endX-startX))
	topSky := field.Top + sy(52, hgt)
	bottomSky := surface - sy(118, hgt)
	if bottomSky <= topSky {
		bottomSky = topSky + sy(60, hgt)
	}
	cy := topSky + int32(afkFlybyYFraction*float64(bottomSky-topSky))
	// A slight arc keeps each pass from reading as a UI marquee.
	cy -= int32(math.Sin(u*math.Pi) * float64(sy(18, hgt)))
	r := RECT{cx - shipW/2, cy - shipH/2, cx + shipW/2, cy + shipH/2}
	afkFlybyLastRect = r
	return r, true
}

func drawAFKUnlockedShipFlyby(hdc uintptr, w, hgt int32, now time.Time) {
	if gameMeta.ReducedMotion || !afkPrimaryWorldVisible() || afkAnyManagementPanelOpen() {
		return
	}
	r, ok := afkUnlockedShipFlybyRect(w, hgt, now)
	if !ok {
		return
	}
	// Soft gold/cyan cue: enough to teach that the ship is collectible without
	// turning the flyby into a giant button.
	if pointInRect(cursorPos, r) {
		pad := sx(6, w)
		drawOutlineRect(hdc, RECT{r.Left - pad, r.Top - pad, r.Right + pad, r.Bottom + pad}, rgb(255, 218, 63), 2)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(255, 226, 91))
			centeredTextOut(hdc, r.Left-sx(35, w), r.Right+sx(35, w), r.Bottom+sy(5, hgt), "CLICK // NAV DATA")
			selectObject.Call(hdc, old)
		}
	}
	drawShipTextureFit(hdc, afkFlybyShipID, r)
}

func handleAFKUnlockedShipFlybyClick(h uintptr, p FPoint, w, hgt int32) bool {
	if afkFlybyShipID < 0 || afkFlybyStartedAt.IsZero() || afkAnyManagementPanelOpen() {
		return false
	}
	r, ok := afkUnlockedShipFlybyRect(w, hgt, time.Now())
	if !ok || !pointInRect(p, r) {
		return false
	}
	reward := int64(25 + rand.Intn(126))
	shipName := "SHIP"
	if afkFlybyShipID >= 0 && afkFlybyShipID < len(spaceShipDefs) && spaceShipDefs[afkFlybyShipID].Name != "" {
		shipName = spaceShipDefs[afkFlybyShipID].Name
	}
	gameMeta.AFKNavigationData = saturatingAdd(gameMeta.AFKNavigationData, reward)
	afkQueueMilestone("NAV DATA RECOVERED", fmt.Sprintf("%s // +%d NAV DATA", shipName, reward))
	afkFlybyShipID = -1
	afkFlybyStartedAt = time.Time{}
	afkFlybyLastRect = RECT{}
	afkScheduleNextFlyby(time.Now())
	saveGameMeta()
	playUIButtonClickSound()
	invalidateRect.Call(h, 0, 0)
	return true
}

func drawAFKPhysicalStarbaseWorld(hdc uintptr, w, hgt int32) {
	if !afkPrimaryWorldVisible() {
		return
	}
	now := time.Now()
	afkRefreshPhysicalBuildingTransitions(now)
	// Show faint blueprint silhouettes for future facilities before drawing the
	// structures that are already online. This makes progression physically
	// legible without adding another HUD element.
	drawAFKFutureFacilityHolograms(hdc, w, hgt, now)
	drawAFKUnlockedShipFlyby(hdc, w, hgt, now)
	afkWorldHoverIndex = -1
	for i := 0; i < afkWorldBuildingCount; i++ {
		if !afkWorldBuilt(i) {
			continue
		}
		r := afkWorldBuildingRect(w, hgt, i)
		drawAFKFacilityAmbientGlow(hdc, i, r, w, hgt)
		if pointInRect(cursorPos, r) {
			afkWorldHoverIndex = i
		}
		switch i {
		case afkWorldCore:
			drawAFKCoreBuilding(hdc, r)
		case afkWorldScout:
			drawAFKScoutPadWorld(hdc, r, w, hgt)
		case afkWorldTech:
			drawAFKTechBuilding(hdc, r)
		case afkWorldOps:
			drawAFKOpsBuilding(hdc, r)
		case afkWorldModules:
			drawAFKFabricatorBuilding(hdc, r)
		case afkWorldDrone:
			drawAFKDroneBayBuilding(hdc, r, w, hgt)
		case afkWorldOrbital:
			drawAFKOrbitalBuilding(hdc, r, w, hgt)
		}
		drawAFKFacilityStateOverlay(hdc, i, r, w, hgt, now)
		if afkWorldHoverIndex == i {
			drawAFKFacilityHoverFlash(hdc, i, r, w, hgt)
			if hudTinyFont != 0 {
				old, _, _ := selectObject.Call(hdc, hudTinyFont)
				setBkMode.Call(hdc, TRANSPARENT)
				setTextColor.Call(hdc, rgb(255, 240, 190))
				label := afkWorldBuildingLabel(i)
				if i != afkWorldCore {
					label += " // CLICK TO MANAGE"
				}
				tw := textPixelSize(hdc, hudTinyFont, label).Cx + sx(18, w)
				lr := RECT{(r.Left+r.Right)/2 - tw/2, r.Top - sy(27, hgt), (r.Left+r.Right)/2 + tw/2, r.Top - sy(7, hgt)}
				fillSolidRect(hdc, lr, rgb(2, 12, 28))
				drawOutlineRect(hdc, lr, rgb(255, 199, 55), 1)
				centeredTextOut(hdc, lr.Left+4, lr.Right-4, verticallyCenteredTextY(hdc, hudTinyFont, lr, label), label)
				selectObject.Call(hdc, old)
			}
		}
	}
	drawAFKFacilityDecor(hdc, w, hgt)
	drawAFKConstructionHologram(hdc, w, hgt)
	drawAFKWorldAmbientActivity(hdc, w, hgt)
	drawAFKEnvironmentalTraffic(hdc, w, hgt, now)
	drawAFKWorldReactionLighting(hdc, w, hgt, now)
}

func drawAFKWorldAmbientActivity(hdc uintptr, w, hgt int32) {
	if gameMeta.ReducedMotion {
		return
	}
	now := time.Now()
	field := afkMainFieldRect(w, hgt)
	surface := afkMoonSurfaceY(w, hgt)
	y := surface - sy(9, hgt)
	built := afkBuiltFacilityCount()

	// Maintenance traffic uses an eased ping-pong route. The drone visibly slows
	// near each end rather than teleporting from one side of the base to the other.
	if afkTechLabUnlocked() {
		t := afkPingPongEase(now, 9.5, 0)
		x := field.Left + sx(42, w) + int32(t*float64(field.Right-field.Left-sx(84, w)))
		bob := int32(math.Sin((float64(now.UnixMilli())/1000.0)*1.7) * float64(sy(3.5, hgt)))
		pulse := afkSoftPulse(now, 2.6, .1)
		c := afkRGBLerp(112, 164, 190, 255, 205, 94, pulse*.55)
		drawAFKDroneSprite(hdc, x, y+bob, max32(1, sx(1.2, w)), c)
	}

	// The floor power network is intentionally sparse. Packets are staggered and
	// slightly offset vertically so they read as energy flow, not marching dots.
	if afkCursorCoreBuilt() {
		count := 2
		if built >= 5 {
			count = 3
		}
		for i := 0; i < count; i++ {
			u := math.Mod(float64(now.UnixMilli())/6200.0+float64(i)/float64(count), 1)
			x := field.Left + sx(18, w) + int32(u*float64(field.Right-field.Left-sx(36, w)))
			wave := int32(math.Sin((u*2*math.Pi)+float64(i)) * float64(sy(1.5, hgt)))
			c := rgb(80, 205, 255)
			if i%2 == 0 {
				c = rgb(181, 104, 255)
			}
			fillSolidRect(hdc, RECT{x - 2, surface + sy(6, hgt) + wave, x + 3, surface + sy(8, hgt) + wave}, c)
		}
	}
}

func afkMin32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func afkBuiltFacilityCount() int {
	n := 0
	for i := 0; i < afkWorldBuildingCount; i++ {
		if afkWorldBuilt(i) {
			n++
		}
	}
	return n
}

func drawAFKFutureFacilityHolograms(hdc uintptr, w, hgt int32, now time.Time) {
	if gameMeta.ReducedMotion {
		return
	}
	phase := (math.Sin(float64(now.UnixMilli())/900.0) + 1.0) * 0.5
	for i := 0; i < afkWorldBuildingCount; i++ {
		if afkWorldBuilt(i) {
			continue
		}
		// Only preview the next two future slots so the early base does not look
		// like seven ghost buildings at once.
		previousBuilt := i == 0 || afkWorldBuilt(i-1)
		nearFuture := previousBuilt || (i > 1 && afkWorldBuilt(i-2))
		if !nearFuture {
			continue
		}
		r := afkWorldBuildingRect(w, hgt, i)
		name := afkWorldBuildingSpriteName(i)
		alpha := byte(26 + phase*24)
		drawAFKFacilityGlowSpriteGrounded(hdc, name, r, alpha)
		// Tiny floor markers make it clear this is a planned construction site.
		y := r.Bottom + sy(2, hgt)
		for x := r.Left; x < r.Right; x += sx(12, w) {
			fillSolidRect(hdc, RECT{x, y, afkMin32(x+sx(5, w), r.Right), y + max32(1, sy(2, hgt))}, rgb(104, 92, 188))
		}
	}
}

func drawAFKFacilityStateOverlay(hdc uintptr, index int, r RECT, w, hgt int32, now time.Time) {
	if gameMeta.ReducedMotion {
		return
	}
	centerX := (r.Left + r.Right) / 2
	pulse := afkSoftPulse(now, 1.8, float64(index)*.13)
	lamp := afkRGBLerp(55, 95, 128, 164, 235, 255, .28+.58*pulse)
	lampSize := max32(2, sx(2.5, w))
	fillSolidRect(hdc, RECT{r.Left + sx(6, w), r.Top + sy(8, hgt), r.Left + sx(6, w) + lampSize, r.Top + sy(8, hgt) + lampSize}, lamp)

	switch index {
	case afkWorldCore:
		// The Core breathes slowly and sends alternating left/right power pulses.
		for k := 0; k < 2; k++ {
			u := math.Mod(float64(now.UnixMilli())/2600.0+float64(k)*.5, 1)
			dx := int32(afkEase01(u) * float64(sx(44, w)))
			c := afkRGBLerp(103, 82, 170, 200, 122, 255, 1-u*.55)
			fillSolidRect(hdc, RECT{centerX - dx - 2, r.Bottom - sy(4, hgt), centerX - dx + 2, r.Bottom - sy(2, hgt)}, c)
			fillSolidRect(hdc, RECT{centerX + dx - 2, r.Bottom - sy(4, hgt), centerX + dx + 2, r.Bottom - sy(2, hgt)}, c)
		}
	case afkWorldScout:
		if afkExpeditionActive() {
			beacon := afkSoftPulse(now, 1.15, 0)
			c := afkRGBLerp(53, 132, 171, 116, 238, 255, .35+.65*beacon)
			rad := sx(3, w) + int32(beacon*float64(sx(3, w)))
			for _, x := range []int32{r.Left + sx(12, w), r.Right - sx(12, w)} {
				drawCircleOutline(hdc, x, r.Bottom-sy(8, hgt), rad, 1, c)
			}
		}
	case afkWorldTech:
		count := 1 + afkMin32(int32(afkResearchTotalLevels()/12), 2)
		for j := int32(0); j < count; j++ {
			a := float64(now.UnixMilli())/1250.0 + float64(j)*2.35
			x2 := centerX + int32(math.Cos(a)*float64(sx(15+float64(j)*5, w)))
			y2 := r.Top + sy(13, hgt) + int32(math.Sin(a)*float64(sy(4, hgt)))
			fillSolidRect(hdc, RECT{x2 - 1, y2 - 1, x2 + 2, y2 + 2}, rgb(116, 218, 255))
		}
	case afkWorldOps:
		a := float64(now.UnixMilli()) / 1450.0
		y := r.Top + sy(10, hgt)
		drawLineSimple(hdc, centerX, y, centerX+int32(math.Cos(a)*float64(sx(16, w))), y+int32(math.Sin(a)*float64(sy(7, hgt))), 1, rgb(104, 225, 174))
	case afkWorldModules:
		// Welding happens in a short burst with a long rest, never as a constant flicker.
		period := int64(max32(3200, int32(6200-afkModuleSetsComplete()*180)))
		burst := now.UnixMilli() % period
		if burst > 350 && burst < 820 {
			u := float64(burst-350) / 470.0
			for j := 0; j < 3; j++ {
				dx := sx(float64(5+j*5), w)
				dy := sy(float64(2+j*2)+u*3, hgt)
				fillSolidRect(hdc, RECT{centerX + dx, r.Top + sy(25, hgt) - dy, centerX + dx + 2, r.Top + sy(25, hgt) - dy + 2}, rgb(255, 210, 92))
			}
		}
	case afkWorldDrone:
		if gameMeta.AFKDronesDeployed > 0 {
			u := afkSoftPulse(now, 2.1, .35)
			rad := sx(8, w) + int32(u*float64(sx(5, w)))
			c := afkRGBLerp(63, 122, 180, 116, 223, 255, u)
			drawCircleOutline(hdc, centerX, r.Top+sy(19, hgt), rad, 1, c)
		}
	case afkWorldOrbital:
		remain := afkOrbitalNextFireIn(now)
		cd := afkOrbitalCooldownSeconds()
		if cd > 0 && remain >= 0 {
			charge := afkClamp01(1 - float64(remain)/float64(cd))
			if charge > .62 {
				u := afkEase01((charge - .62) / .38)
				rad := sx(4, w) + int32(u*float64(sx(13, w)))
				c := afkRGBLerp(112, 66, 156, 230, 128, 255, u)
				drawCircleOutline(hdc, centerX, r.Top+sy(16, hgt), rad, 1, c)
			}
		}
	}
}

func drawAFKEnvironmentalTraffic(hdc uintptr, w, hgt int32, now time.Time) {
	if gameMeta.ReducedMotion {
		return
	}
	field := afkMainFieldRect(w, hgt)
	built := afkBuiltFacilityCount()
	if built < 2 {
		return
	}
	surface := afkMoonSurfaceY(w, hgt)

	// Cargo now shuttles between two service zones with eased turnaround instead
	// of disappearing at the right edge and popping back to the left.
	if built >= 3 {
		t := afkPingPongEase(now, 12.5, .21)
		x := field.Left + sx(74, w) + int32(t*float64(field.Right-field.Left-sx(148, w)))
		y := surface + sy(31, hgt)
		bob := int32(math.Sin(float64(now.UnixMilli())/700.0) * float64(sy(1.2, hgt)))
		if built >= 5 {
			drawAFKFacilitySprite(hdc, "energy_canister", RECT{x - sx(8, w), y - sy(15, hgt) + bob, x + sx(8, w), y + sy(3, hgt) + bob})
		} else {
			drawAFKFacilitySprite(hdc, "cargo_crate_01", RECT{x - sx(10, w), y - sy(10, hgt) + bob, x + sx(10, w), y + sy(4, hgt) + bob})
		}
	}

	if built >= 5 {
		t2 := afkPingPongEase(now, 10.8, .63)
		x2 := field.Right - sx(48, w) - int32(t2*float64(field.Right-field.Left-sx(96, w)))
		y2 := surface - sy(20, hgt) + int32(math.Sin(float64(now.UnixMilli())/920.0)*float64(sy(4, hgt)))
		c := afkRGBLerp(105, 78, 156, 191, 126, 255, afkSoftPulse(now, 2.8, .4))
		drawAFKDroneSprite(hdc, x2, y2, max32(1, sx(1.1, w)), c)
	}

	// Distant traffic has a long quiet gap. It begins and ends off-screen, so the
	// cycle reset is invisible and the pass feels like a real ship crossing space.
	if afkTechLabUnlocked() {
		cycle := now.UnixMilli() % 47000
		if cycle < 9000 {
			u := afkEase01(float64(cycle) / 9000.0)
			x := field.Left - sx(28, w) + int32(u*float64(field.Right-field.Left+sx(56, w)))
			y := field.Top + sy(48, hgt) + int32(math.Sin(u*math.Pi)*float64(sy(14, hgt)))
			sz := max32(1, sx(1.15, w))
			body := afkRGBLerp(90, 112, 147, 197, 225, 241, math.Sin(u*math.Pi)*.65)
			fillSolidRect(hdc, RECT{x - sz*5, y - sz, x + sz*4, y + sz}, body)
			fillSolidRect(hdc, RECT{x - sz, y - sz*2, x + sz*3, y + sz*2}, rgb(185, 213, 232))
			fillSolidRect(hdc, RECT{x - sz*7, y, x - sz*5, y + sz}, rgb(163, 93, 239))
		}
	}

	// Meteors are rare and use eased travel with a soft tail. They never overlap
	// gameplay collision and are deliberately absent from early Starbase.
	if built >= 4 {
		cycle := now.UnixMilli() % 59000
		if cycle < 1750 {
			u := afkEase01(float64(cycle) / 1750.0)
			x := field.Right + sx(34, w) - int32(u*float64((field.Right-field.Left)/2+sx(70, w)))
			y := field.Top + sy(23, hgt) + int32(u*float64(sy(108, hgt)))
			tail := sx(25+10*math.Sin(u*math.Pi), w)
			drawLineSimple(hdc, x, y, x+tail, y-sy(16, hgt), 2, rgb(185, 159, 244))
			drawLineSimple(hdc, x+sx(4, w), y-sy(2, hgt), x+tail+sx(9, w), y-sy(17, hgt), 1, rgb(105, 111, 176))
		}
	}
}

func drawAFKWorldReactionLighting(hdc uintptr, w, hgt int32, now time.Time) {
	if !afkPrimaryWorldVisible() {
		return
	}
	cx, cy, rad := afkSingularityGeometry(w, hgt)
	if !afkOrbitalLastVisualFire.IsZero() {
		age := now.Sub(afkOrbitalLastVisualFire)
		const duration = 980 * time.Millisecond
		if age >= 0 && age < duration {
			u := afkClamp01(float64(age) / float64(duration))
			strength := math.Sin(u * math.Pi) // zero at both ends: no lighting pop.
			ring := int32(float64(rad) * (1.0 + .12*afkEase01(u)))
			c := afkRGBLerp(90, 63, 132, 220, 137, 255, strength)
			drawCircleOutline(hdc, int32(cx), int32(cy), ring, max32(1, sx(1+2*strength, w)), c)
			field := afkMainFieldRect(w, hgt)
			y := afkMoonSurfaceY(w, hgt) + sy(44, hgt)
			if strength > .08 {
				drawLineSimple(hdc, field.Left, y, field.Right, y, int(max32(1, int32(1+2*strength))), afkRGBLerp(70, 51, 105, 167, 111, 226, strength))
			}
		}
	}
}

func handleAFKPhysicalBuildingClick(h uintptr, p FPoint, w, hgt int32) bool {
	if !afkPrimaryWorldVisible() {
		return false
	}
	for i := 0; i < afkWorldBuildingCount; i++ {
		if !afkWorldBuilt(i) {
			continue
		}
		if !pointInRect(p, afkWorldBuildingRect(w, hgt, i)) {
			continue
		}
		if i == afkWorldCore {
			return false
		}
		afkCloseManagementPanels()
		switch i {
		case afkWorldScout:
			afkExpeditionPanelOpen = true
		case afkWorldTech:
			afkTechPanelOpen = true
		case afkWorldOps:
			afkOperatorPanelOpen = true
		case afkWorldModules:
			afkModulePanelOpen = true
		case afkWorldDrone:
			afkDronePanelOpen = true
		case afkWorldOrbital:
			afkOrbitalPanelOpen = true
		}
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	return false
}

// Physical cache: a glowing crate/capsule on the moon surface instead of a
// permanent floating UI card. Claiming still uses the existing consume-first
// guarded reward handler.
func afkPhysicalCacheRect(w, hgt int32) RECT {
	field := afkMainFieldRect(w, hgt)
	y := afkMoonSurfaceY(w, hgt)
	ww := sx(78, w)
	hh := sy(58, hgt)
	x := field.Right - sx(165, w)
	return RECT{x - ww/2, y - hh, x + ww/2, y + sy(5, hgt)}
}

func drawAFKPhysicalCache(hdc uintptr, w, hgt int32) {
	if gameMeta.AFKActiveCacheReward <= 0 || afkHasOfflineReward() || !afkPrimaryWorldVisible() {
		return
	}
	r := afkPhysicalCacheRect(w, hgt)
	// Starbase uses its own Star Cache artwork. Endurance Space Cache assets remain separate.
	img := RECT{r.Left - sx(24, w), r.Top - sy(18, hgt), r.Right + sx(24, w), r.Bottom + sy(8, hgt)}
	drawRawBGRAFit(hdc, starCacheBGRA, 1536, 1024, img)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 232, 130))
		centeredTextOut(hdc, r.Left-sx(35, w), r.Right+sx(35, w), r.Top-sy(20, hgt), "STAR CACHE DETECTED")
		selectObject.Call(hdc, old)
	}
}

func drawAFKManagementBackdropDim(hdc uintptr, w, hgt int32) {
	if !afkAnyManagementPanelOpen() || afkSystemsMenuOpen {
		return
	}
	f := afkMainFieldRect(w, hgt)
	overlaySolidAlphaRect(hdc, f, rgb(0, 2, 12), 135)
}

func formatAFKCountUpReward(total int64) string {
	if total <= 0 {
		return "0"
	}
	if afkWelcomeCountStart.IsZero() {
		afkWelcomeCountStart = time.Now()
	}
	age := time.Since(afkWelcomeCountStart)
	t := float64(age) / float64(1250*time.Millisecond)
	if t > 1 {
		t = 1
	}
	if t < 0 {
		t = 0
	}
	// ease-out cubic
	e := 1 - math.Pow(1-t, 3)
	return formatAFKNumber(int64(float64(total) * e))
}

func resetAFKWelcomeCountIfNeeded() {
	if !afkHasOfflineReward() {
		afkWelcomeCountStart = time.Time{}
	}
}

func afkWorldDebugSummary() string {
	n := 0
	for i := 0; i < afkWorldBuildingCount; i++ {
		if afkWorldBuilt(i) {
			n++
		}
	}
	return fmt.Sprintf("PHYSICAL STARBASE %d/%d", n, afkWorldBuildingCount)
}
