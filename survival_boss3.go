//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
	"unsafe"
)

// v401: Terminus final-boss polish + homing missile interception pass: premium rhythm-game presentation,
// three left-to-right mechanics per loop, safer input timing, larger readable satellite
// hit objects, true-aspect seamless fleet scrolling, slow lethal border meteorites,
// and deliberate bottom-cannon pressure with long readable telegraphs. The ship
// remains collidable until it reaches 25% visible and flashes red.
const (
	survivalBoss3None = iota
	survivalBoss3Intro
	survivalBoss3Combat
	survivalBoss3Vulnerable
	survivalBoss3Dying
	survivalBoss3Return
)

const (
	terminusMechChain = iota
	terminusMechSlider
	terminusMechStream
	terminusMechHoldRail
	terminusMechSpinner
	terminusMechLaserSlider
	terminusMechJump
	terminusMechRelay
	terminusMechCount
)

const (
	terminusBossHP            = 250
	terminusMechanicsToExpose = 9 // three full 3-mechanic loops for the opening push
	terminusDamageCPS         = 12
)

type TerminusAsset struct {
	data   []byte
	w, h   int32
	sprite CachedBGRASprite
}

type TerminusLaser struct {
	A, B                  FPoint
	WarnAt, FireAt, EndAt time.Time
}

type TerminusTurret struct {
	Aim                           float64
	Target                        FPoint
	WarnAt, LockAt, FireAt, EndAt time.Time
	Active                        bool
	Heavy                         bool
}

type TerminusHitBurst struct {
	P    FPoint
	At   time.Time
	Slot int
}

type TerminusMeteorSpec struct {
	Asset  int
	XFrac  float64
	Top    bool
	SizePx float64
	AmpPx  float64
	Phase  float64
	Speed  float64
}

type TerminusWreckSpec struct {
	Asset        int
	XFrac, YFrac float64
	SizePx       float64
	BobPx        float64
	Phase        float64
}

type TerminusWreckState struct {
	DestroyedUntil time.Time
}

type TerminusMissile struct {
	P, V      FPoint
	SpawnedAt time.Time
	LastAt    time.Time
	Trail     []FPoint
}

type TerminusMissileBurst struct {
	P  FPoint
	At time.Time
}

type TerminusChallenge struct {
	Kind          int
	Slot          int
	Points        []FPoint
	Path          []FPoint
	Current       int
	StartedAt     time.Time
	TargetSpawnAt time.Time
	TargetHitAt   time.Time
	Deadline      time.Time
	Duration      time.Duration

	Holding       bool
	Dragging      bool
	PathProgress  float64
	MissSince     time.Time
	SpinnerCentre FPoint
	SpinnerLast   float64
	SpinnerAccum  float64
	SpinnerNeed   float64
	SpinnerDir    float64

	RelayModes []int // 0 click, 1 drag to next, 2 hold-track to next
	RelayMode  int

	ShipStartProgress float64
	PushEarned        float64
	Lasers            []TerminusLaser
}

var (
	survivalBoss3Stage       = survivalBoss3None
	survivalBoss3StageAt     time.Time
	survivalBoss3LastUpdate  time.Time
	survivalBoss3StartedAt   time.Time
	survivalBoss3DamageStart int

	survivalBoss3LoopKinds [3]int
	survivalBoss3Slot      int
	survivalBoss3Challenge TerminusChallenge
	survivalBoss3LoopCount int
	survivalBoss3Clears    int

	survivalBoss3ShipTargetProgress float64
	survivalBoss3ShipVisualProgress float64
	survivalBoss3ShipLastTick       time.Time
	survivalBoss3VulnerableUntil    time.Time
	survivalBoss3Damage             int
	survivalBoss3LastDamageClick    time.Time
	survivalBoss3ShipHitFlashUntil  time.Time
	survivalBoss3MistakeGraceUntil  time.Time

	terminusShip             TerminusAsset
	terminusShipRed          TerminusAsset
	terminusBG               TerminusAsset
	terminusSat              [8]TerminusAsset
	terminusSatWreck         [8]TerminusAsset
	terminusMeteor           [12]TerminusAsset
	terminusRocket           TerminusAsset
	terminusCannonBase       TerminusAsset
	terminusCannonLong       TerminusAsset
	terminusCannonHeavy      TerminusAsset
	terminusTurrets          [3]TerminusTurret
	terminusTurretNextAttack time.Time
	terminusTurretCursor     int
	terminusHitBursts        []TerminusHitBurst
	terminusMissiles         []TerminusMissile
	terminusMissileBursts    []TerminusMissileBurst
	terminusMissileNextAt    time.Time
	terminusWreckState       [6]TerminusWreckState
	terminusBrushCache       = map[uintptr]uintptr{}
	terminusAssetsLoaded     bool
)

var terminusMeteorSpecs = [...]TerminusMeteorSpec{
	{Asset: 1, XFrac: .08, Top: true, SizePx: 34, AmpPx: 5, Phase: .3, Speed: .72},
	{Asset: 2, XFrac: .28, Top: true, SizePx: 27, AmpPx: 7, Phase: 1.4, Speed: .58},
	{Asset: 6, XFrac: .52, Top: true, SizePx: 31, AmpPx: 5, Phase: 2.2, Speed: .66},
	{Asset: 10, XFrac: .76, Top: true, SizePx: 28, AmpPx: 6, Phase: 3.1, Speed: .62},
	{Asset: 3, XFrac: .93, Top: true, SizePx: 36, AmpPx: 4, Phase: 4.2, Speed: .55},
	{Asset: 5, XFrac: .10, Top: false, SizePx: 32, AmpPx: 5, Phase: 1.0, Speed: .61},
	{Asset: 7, XFrac: .34, Top: false, SizePx: 26, AmpPx: 6, Phase: 2.5, Speed: .69},
	{Asset: 8, XFrac: .59, Top: false, SizePx: 33, AmpPx: 4, Phase: 3.6, Speed: .57},
	{Asset: 11, XFrac: .84, Top: false, SizePx: 29, AmpPx: 6, Phase: 4.8, Speed: .64},
}

// Desaturated satellite wrecks are deliberately kept around the upper/lower
// edges of the rhythm corridor. They are missile cover, not rhythm targets.
var terminusWreckSpecs = [...]TerminusWreckSpec{
	{Asset: 0, XFrac: .12, YFrac: .22, SizePx: 48, BobPx: 5, Phase: .2},
	{Asset: 5, XFrac: .30, YFrac: .16, SizePx: 43, BobPx: 7, Phase: 1.1},
	{Asset: 2, XFrac: .49, YFrac: .23, SizePx: 45, BobPx: 5, Phase: 2.2},
	{Asset: 3, XFrac: .19, YFrac: .80, SizePx: 47, BobPx: 6, Phase: 2.9},
	{Asset: 6, XFrac: .37, YFrac: .84, SizePx: 42, BobPx: 5, Phase: 3.8},
	{Asset: 7, XFrac: .53, YFrac: .77, SizePx: 46, BobPx: 7, Phase: 4.6},
}

func survivalBoss3Active() bool              { return survivalBoss3Stage != survivalBoss3None }
func survivalBoss3OwnsArena() bool           { return survivalBoss3Active() }
func survivalBoss3SuppressNormalLogic() bool { return survivalBoss3Active() }
func survivalBoss3CombatActive() bool {
	return state == StatePlaying && (survivalBoss3Stage == survivalBoss3Combat || survivalBoss3Stage == survivalBoss3Vulnerable)
}

func terminusMechanicName(kind int) string {
	switch kind {
	case terminusMechChain:
		return "CHAIN REACTION"
	case terminusMechSlider:
		return "SATELLITE SLIDER"
	case terminusMechStream:
		return "SATELLITE STREAM"
	case terminusMechHoldRail:
		return "HOLD RAIL"
	case terminusMechSpinner:
		return "ORBIT CHARGE"
	case terminusMechLaserSlider:
		return "LASER SLIDER"
	case terminusMechJump:
		return "JUMP PATTERN"
	case terminusMechRelay:
		return "RELAY COMBO"
	}
	return "TERMINUS"
}

func survivalBoss3PhaseLabel() string {
	switch survivalBoss3Stage {
	case survivalBoss3Intro:
		return "FINAL APPROACH"
	case survivalBoss3Vulnerable:
		return fmt.Sprintf("VULNERABLE // %d / %d", survivalBoss3Damage, terminusBossHP)
	case survivalBoss3Combat:
		return terminusMechanicName(survivalBoss3Challenge.Kind)
	case survivalBoss3Dying:
		return "TERMINUS DESTROYED"
	}
	return "FINAL BOSS"
}

func terminusLoadAsset(parts []string, w, h int32) TerminusAsset {
	return TerminusAsset{data: readExternalBytes(parts...), w: w, h: h}
}

func terminusTintRed(src TerminusAsset) TerminusAsset {
	if len(src.data) < int(src.w*src.h*4) {
		return TerminusAsset{w: src.w, h: src.h}
	}
	out := make([]byte, len(src.data))
	copy(out, src.data)
	for i := 0; i+3 < len(out); i += 4 {
		a := out[i+3]
		if a == 0 {
			continue
		}
		b, g, r := int(out[i]), int(out[i+1]), int(out[i+2])
		lum := (r*3 + g*4 + b) / 8
		out[i] = byte(terminusMinInt(255, 20+lum/4))
		out[i+1] = byte(terminusMinInt(255, 24+lum/3))
		out[i+2] = byte(terminusMinInt(255, 145+lum/2))
	}
	return TerminusAsset{data: out, w: src.w, h: src.h}
}

func terminusTintWreck(src TerminusAsset) TerminusAsset {
	if len(src.data) < int(src.w*src.h*4) {
		return TerminusAsset{w: src.w, h: src.h}
	}
	out := make([]byte, len(src.data))
	copy(out, src.data)
	for i := 0; i+3 < len(out); i += 4 {
		if out[i+3] == 0 {
			continue
		}
		b, g, r := int(out[i]), int(out[i+1]), int(out[i+2])
		lum := (r*3 + g*5 + b*2) / 10
		// Preserve a little cold-blue metal while killing the saturated target glow.
		out[i] = byte(terminusMinInt(255, lum*72/100+18))
		out[i+1] = byte(terminusMinInt(255, lum*68/100+14))
		out[i+2] = byte(terminusMinInt(255, lum*62/100+12))
	}
	return TerminusAsset{data: out, w: src.w, h: src.h}
}

func terminusMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func terminusEnsureAssets() {
	if terminusAssetsLoaded {
		return
	}
	terminusAssetsLoaded = true
	terminusShip = terminusLoadAsset([]string{"survival", "boss3", "terminus_main.bgra"}, 1248, 535)
	terminusShipRed = terminusTintRed(terminusShip)
	// v398 uses the accepted final-fleet artwork followed by its mirror. This makes
	// a true seamless panorama with exact edge continuity and no smear/stretch seam.
	terminusBG = terminusLoadAsset([]string{"survival", "boss3", "terminus_fleet_background.bgra"}, 1983, 793)
	terminusCannonBase = terminusLoadAsset([]string{"survival", "boss3", "terminus_cannon_base.bgra"}, 100, 55)
	terminusCannonLong = terminusLoadAsset([]string{"survival", "boss3", "terminus_cannon_barrel_long.bgra"}, 98, 41)
	terminusCannonHeavy = terminusLoadAsset([]string{"survival", "boss3", "terminus_cannon_barrel_heavy.bgra"}, 113, 51)
	dims := [8][2]int32{{170, 150}, {145, 138}, {131, 99}, {176, 151}, {129, 108}, {151, 137}, {179, 142}, {168, 143}}
	for i := 0; i < 8; i++ {
		terminusSat[i] = terminusLoadAsset([]string{"survival", "boss3", fmt.Sprintf("terminus_satellite_%d.bgra", i+1)}, dims[i][0], dims[i][1])
		terminusSatWreck[i] = terminusTintWreck(terminusSat[i])
	}
	terminusRocket = terminusLoadAsset([]string{"survival", "boss3", "terminus_rocket.bgra"}, 223, 116)
	meteorDims := [12][2]int32{{177, 168}, {56, 52}, {85, 89}, {11, 28}, {126, 146}, {50, 50}, {103, 95}, {103, 99}, {107, 106}, {67, 77}, {79, 68}, {58, 51}}
	for i := 0; i < 12; i++ {
		terminusMeteor[i] = terminusLoadAsset([]string{"survival", "boss3", fmt.Sprintf("terminus_meteorite_%02d.bgra", i+1)}, meteorDims[i][0], meteorDims[i][1])
	}
}

func resetSurvivalBoss3State() {
	survivalBoss3Stage = survivalBoss3None
	survivalBoss3StageAt = time.Time{}
	survivalBoss3LastUpdate = time.Time{}
	survivalBoss3StartedAt = time.Time{}
	survivalBoss3LoopKinds = [3]int{}
	survivalBoss3Slot = 0
	survivalBoss3Challenge = TerminusChallenge{}
	survivalBoss3LoopCount = 0
	survivalBoss3Clears = 0
	survivalBoss3ShipTargetProgress = 0
	survivalBoss3ShipVisualProgress = 0
	survivalBoss3ShipLastTick = time.Time{}
	survivalBoss3VulnerableUntil = time.Time{}
	survivalBoss3Damage = 0
	survivalBoss3LastDamageClick = time.Time{}
	survivalBoss3ShipHitFlashUntil = time.Time{}
	survivalBoss3MistakeGraceUntil = time.Time{}
	terminusTurrets = [3]TerminusTurret{}
	for i := range terminusTurrets {
		terminusTurrets[i].Aim = -math.Pi / 2
	}
	terminusTurretNextAttack = time.Time{}
	terminusTurretCursor = 0
	terminusHitBursts = nil
	terminusMissiles = nil
	terminusMissileBursts = nil
	terminusMissileNextAt = time.Time{}
	terminusWreckState = [6]TerminusWreckState{}
}

func beginSurvivalBoss3(h uintptr, now time.Time) {
	// Boss 3 owns a dedicated wave number. This prevents Wave 30 from remaining
	// semantically active underneath the encounter and keeps HUD/checkpoint state honest.
	survivalWave = survivalBoss3Wave
	analyticsEvent("boss_attempted", map[string]any{"boss": "TERMINUS", "wave": survivalWave})
	resetSurvivalBoss3State()
	terminusEnsureAssets()
	survivalBoss3Stage = survivalBoss3Intro
	survivalBoss3StageAt = now
	survivalBoss3LastUpdate = now
	survivalBoss3StartedAt = now
	survivalBoss3ShipLastTick = now
	survivalBoss3DamageStart = survivalTotalDamageTaken
	// Mirror the proven Boss 1/Boss 2 handoff: destroy every ordinary-wave owner before
	// the boss becomes active. W31 is a clean encounter, never an overlay on Wave 30.
	survivalEnemies = nil
	survivalPickup = nil
	survivalPickup2 = nil
	survivalEliminations = nil
	survivalArcFX = nil
	survivalOrbitals = map[int]*SurvivalOrbital{}
	survivalShieldCharges = 0
	survivalChainCharges = 0
	survivalSlowUntil = time.Time{}
	survivalWaveBreakUntil = time.Time{}
	survivalNextSpawn = time.Time{}
	survivalNextPickupAt = time.Time{}
	survivalWaveSpawned = 0
	survivalWaveBudget = 0
	survivalWaveKills = 0
	survivalWaveType = survivalWaveNormal
	survivalWaveDamageTaken = false
	resetSurvivalSector3FieldEvent()
	stopLoopSFX("survival_1hp_siren")
	// Entering the dedicated boss checkpoint always starts from a clean, fully repaired
	// station, including any permanent +1/+2 Station HP bought in Starbase.
	survivalHP = survivalMaxHP()
	if survivalBossIntroHandoff {
		survivalWaveBannerText = ""
		survivalWaveBannerUntil = time.Time{}
		status = "TERMINUS 1337 // DOSSIER COMPLETE"
	} else {
		survivalWaveBannerText = "BOSS 3 // THE TERMINUS"
		survivalWaveBannerUntil = now.Add(1800 * time.Millisecond)
		status = "FINAL CONTACT // THE TERMINUS"
	}
	unlockAchievement("SURV_ARRAY_REACHED")
	if h != 0 {
		invalidateRect.Call(h, 0, 0)
	}
}

func survivalBoss3SetStage(s int, now time.Time) {
	survivalBoss3Stage = s
	survivalBoss3StageAt = now
	survivalBoss3LastUpdate = now
}

func terminusShipVisibleFraction(progress float64) float64 {
	progress = math.Max(0, math.Min(1, progress))
	return .75 - .50*progress
}

func terminusShipRectForProgress(w, h int32, progress float64) RECT {
	r := arenaRect(w, h)
	aw := r.Right - r.Left
	shipW := int32(float64(aw) * .58)
	shipH := int32(float64(shipW) * 535.0 / 1248.0)
	visible := terminusShipVisibleFraction(progress)
	shipX := r.Right - int32(float64(shipW)*visible)
	shipY := r.Top + (r.Bottom-r.Top-shipH)/2
	return RECT{shipX, shipY, shipX + shipW, shipY + shipH}
}

func terminusShipRect(w, h int32) RECT {
	r := terminusShipRectForProgress(w, h, survivalBoss3ShipVisualProgress)
	if survivalBoss3Active() && !survivalBoss3StartedAt.IsZero() {
		bob := int32(math.Round(math.Sin(survivalBossFightNow(time.Now(), survivalBoss3StartedAt).Sub(survivalBoss3StartedAt).Seconds()*1.18) * float64(sy(4, h))))
		r.Top += bob
		r.Bottom += bob
	}
	return r
}

func terminusShipPixelSolid(p FPoint, w, h int32) bool {
	terminusEnsureAssets()
	r := terminusShipRect(w, h)
	if p.X < float64(r.Left) || p.X >= float64(r.Right) || p.Y < float64(r.Top) || p.Y >= float64(r.Bottom) {
		return false
	}
	if len(terminusShip.data) < int(terminusShip.w*terminusShip.h*4) {
		return true
	}
	u := (p.X - float64(r.Left)) / float64(r.Right-r.Left)
	v := (p.Y - float64(r.Top)) / float64(r.Bottom-r.Top)
	sx0 := int32(u * float64(terminusShip.w))
	sy0 := int32(v * float64(terminusShip.h))
	if sx0 < 0 || sx0 >= terminusShip.w || sy0 < 0 || sy0 >= terminusShip.h {
		return false
	}
	idx := int((sy0*terminusShip.w+sx0)*4 + 3)
	return idx >= 0 && idx < len(terminusShip.data) && terminusShip.data[idx] >= 72
}

func terminusUpdateShipMotion(now time.Time) {
	if survivalBoss3ShipLastTick.IsZero() {
		survivalBoss3ShipLastTick = now
		return
	}
	dt := now.Sub(survivalBoss3ShipLastTick).Seconds()
	if dt < 0 {
		dt = 0
	}
	if dt > .1 {
		dt = .1
	}
	survivalBoss3ShipLastTick = now
	// Exponential smoothing gives the ship a heavy but responsive capital-vessel glide.
	alpha := 1 - math.Exp(-dt*1.55)
	survivalBoss3ShipVisualProgress += (survivalBoss3ShipTargetProgress - survivalBoss3ShipVisualProgress) * alpha
}

func terminusOpenLane(w, h int32) (left, right float64) {
	r := arenaRect(w, h)
	left = float64(r.Left + sx(54, w))
	ship := terminusShipRectForProgress(w, h, survivalBoss3ShipTargetProgress)
	right = float64(ship.Left - sx(24, w))
	minRight := left + float64(sx(500, w))
	if right < minRight {
		right = minRight
	}
	maxRight := float64(r.Right - sx(72, w))
	if right > maxRight {
		right = maxRight
	}
	return
}

func terminusMeteorRect(spec TerminusMeteorSpec, w, h int32, now time.Time) RECT {
	r := arenaRect(w, h)
	elapsed := 0.0
	if !survivalBoss3StartedAt.IsZero() {
		elapsed = now.Sub(survivalBoss3StartedAt).Seconds()
	}
	if elapsed < 0 {
		elapsed = 0
	}
	cx := float64(r.Left) + float64(r.Right-r.Left)*spec.XFrac
	edge := float64(sy(28, h))
	cy := float64(r.Top) + edge
	if !spec.Top {
		cy = float64(r.Bottom) - edge
	}
	cy += math.Sin(elapsed*spec.Speed*math.Pi*2+spec.Phase) * float64(sy(spec.AmpPx, h))
	asset := &terminusMeteor[spec.Asset%len(terminusMeteor)]
	ww := sx(spec.SizePx, w)
	if ww < 12 {
		ww = 12
	}
	hh := ww
	if asset.w > 0 {
		hh = int32(float64(ww) * float64(asset.h) / float64(asset.w))
	}
	return RECT{int32(cx) - ww/2, int32(cy) - hh/2, int32(cx) + ww/2, int32(cy) + hh/2}
}

func terminusMeteorCollision(p FPoint, w, h int32, now time.Time) bool {
	for _, spec := range terminusMeteorSpecs {
		r := terminusMeteorRect(spec, w, h, now)
		cx := float64(r.Left+r.Right) * .5
		cy := float64(r.Top+r.Bottom) * .5
		rad := float64(terminusMinInt(int(r.Right-r.Left), int(r.Bottom-r.Top))) * .36
		if rad < float64(sx(7, w)) {
			rad = float64(sx(7, w))
		}
		if math.Hypot(p.X-cx, p.Y-cy) <= rad {
			return true
		}
	}
	return false
}

func terminusKillOnMeteor(h uintptr, now time.Time) {
	if state != StatePlaying {
		return
	}
	if survivalReviveImmune(now) {
		status = "REVIVE IMMUNITY // METEOR ABSORBED"
		return
	}
	survivalHP = 0
	survivalWaveDamageTaken = true
	survivalTotalDamageTaken++
	survivalCombo = 0
	playSurvivalDamageTakenSound()
	survivalFail(h, "Meteorite collision")
}

func terminusDrawMeteors(hdc uintptr, w, h int32, now time.Time) {
	for _, spec := range terminusMeteorSpecs {
		a := &terminusMeteor[spec.Asset%len(terminusMeteor)]
		r := terminusMeteorRect(spec, w, h, now)
		drawTerminusAsset(hdc, a, r)
	}
}

func terminusSlotBounds(slot int, w, h int32) (float64, float64, float64, float64) {
	left, right := terminusOpenLane(w, h)
	span := (right - left) / 3
	x0 := left + float64(slot)*span + float64(sx(7, w))
	x1 := left + float64(slot+1)*span - float64(sx(7, w))
	r := arenaRect(w, h)
	y0 := float64(r.Top + sy(54, h))
	y1 := float64(r.Bottom - sy(54, h))
	return x0, x1, y0, y1
}

func terminusClampY(y, lo, hi float64) float64 {
	if y < lo {
		return lo
	}
	if y > hi {
		return hi
	}
	return y
}

func terminusPatternY(i, n, variant int, lo, hi float64) float64 {
	mid := (lo + hi) * .5
	amp := (hi - lo) * .34
	t := 0.0
	if n > 1 {
		t = float64(i) / float64(n-1)
	}
	switch variant % 5 {
	case 0:
		return mid + math.Sin(t*math.Pi*2)*amp
	case 1:
		if i%2 == 0 {
			return mid - amp*.82
		}
		return mid + amp*.82
	case 2:
		return mid + math.Sin(t*math.Pi)*amp*.9
	case 3:
		return mid - amp + t*amp*2
	default:
		return mid + math.Cos(t*math.Pi*3)*amp*.72
	}
}

func terminusBuildSmoothPath(start, end FPoint, variant int, loY, hiY float64, samples int) []FPoint {
	if samples < 20 {
		samples = 20
	}
	out := make([]FPoint, samples+1)
	amp := math.Min((hiY-loY)*.22, math.Abs(end.X-start.X)*.28)
	phase := 0.0
	if variant%2 == 1 {
		phase = math.Pi
	}
	waves := 1.0 + float64(variant%3)*.5
	for i := 0; i <= samples; i++ {
		t := float64(i) / float64(samples)
		x := start.X + (end.X-start.X)*t
		base := start.Y + (end.Y-start.Y)*t
		y := base + math.Sin(t*math.Pi*waves+phase)*amp*math.Sin(math.Pi*t)
		out[i] = FPoint{X: x, Y: terminusClampY(y, loY, hiY)}
	}
	return out
}

func terminusPathLength(path []FPoint) float64 {
	total := 0.0
	for i := 1; i < len(path); i++ {
		total += math.Hypot(path[i].X-path[i-1].X, path[i].Y-path[i-1].Y)
	}
	return total
}

func terminusPointAtPath(path []FPoint, progress float64) FPoint {
	if len(path) == 0 {
		return FPoint{}
	}
	if len(path) == 1 || progress <= 0 {
		return path[0]
	}
	if progress >= 1 {
		return path[len(path)-1]
	}
	total := terminusPathLength(path)
	if total <= 0 {
		return path[0]
	}
	want := total * progress
	walk := 0.0
	for i := 1; i < len(path); i++ {
		seg := math.Hypot(path[i].X-path[i-1].X, path[i].Y-path[i-1].Y)
		if walk+seg >= want {
			f := (want - walk) / math.Max(seg, .001)
			return FPoint{X: path[i-1].X + (path[i].X-path[i-1].X)*f, Y: path[i-1].Y + (path[i].Y-path[i-1].Y)*f}
		}
		walk += seg
	}
	return path[len(path)-1]
}

func terminusNearestPathProgress(p FPoint, path []FPoint) (float64, float64) {
	if len(path) < 2 {
		return 0, math.Inf(1)
	}
	total := terminusPathLength(path)
	walk := 0.0
	bestD := math.Inf(1)
	bestProg := 0.0
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		dx, dy := b.X-a.X, b.Y-a.Y
		seg2 := dx*dx + dy*dy
		t := 0.0
		if seg2 > 0 {
			t = ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / seg2
		}
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		q := FPoint{X: a.X + dx*t, Y: a.Y + dy*t}
		d := math.Hypot(p.X-q.X, p.Y-q.Y)
		seg := math.Sqrt(seg2)
		if d < bestD {
			bestD = d
			if total > 0 {
				bestProg = (walk + seg*t) / total
			}
		}
		walk += seg
	}
	return bestProg, bestD
}

func terminusGenerateChallenge(kind, slot int, w, h int32) TerminusChallenge {
	x0, x1, y0, y1 := terminusSlotBounds(slot, w, h)
	variant := rand.Intn(5)
	c := TerminusChallenge{Kind: kind, Slot: slot}
	midY := (y0 + y1) * .5

	switch kind {
	case terminusMechChain:
		n := 5 + rand.Intn(2)
		for i := 0; i < n; i++ {
			t := float64(i) / float64(n-1)
			c.Points = append(c.Points, FPoint{X: x0 + (x1-x0)*t, Y: terminusPatternY(i, n, variant, y0, y1)})
		}
	case terminusMechStream:
		n := 9 + rand.Intn(4)
		for i := 0; i < n; i++ {
			t := float64(i) / float64(n-1)
			y := midY + math.Sin(t*math.Pi*2+float64(variant))*((y1-y0)*.22)
			c.Points = append(c.Points, FPoint{X: x0 + (x1-x0)*t, Y: terminusClampY(y, y0, y1)})
		}
	case terminusMechJump:
		n := 5
		for i := 0; i < n; i++ {
			t := float64(i) / float64(n-1)
			y := y0 + (y1-y0)*.16
			if i%2 == 1 {
				y = y1 - (y1-y0)*.16
			}
			if variant%2 == 1 {
				y = y0 + y1 - y
			}
			c.Points = append(c.Points, FPoint{X: x0 + (x1-x0)*t, Y: y})
		}
	case terminusMechSlider, terminusMechLaserSlider, terminusMechHoldRail:
		start := FPoint{X: x0, Y: terminusPatternY(0, 2, variant, y0, y1)}
		end := FPoint{X: x1, Y: terminusPatternY(1, 2, variant+2, y0, y1)}
		c.Points = []FPoint{start, end}
		c.Path = terminusBuildSmoothPath(start, end, variant, y0, y1, 48)
		if kind == terminusMechLaserSlider {
			crossX := x0 + (x1-x0)*(.53+float64(rand.Intn(12))/100)
			c.Lasers = []TerminusLaser{{A: FPoint{X: crossX, Y: y0 - float64(sy(30, h))}, B: FPoint{X: crossX, Y: y1 + float64(sy(30, h))}}}
		}
	case terminusMechSpinner:
		centre := FPoint{X: (x0 + x1) * .5, Y: midY}
		c.SpinnerCentre = centre
		c.Points = []FPoint{{X: x0, Y: centre.Y}, centre, {X: x1, Y: centre.Y}}
		c.SpinnerNeed = math.Pi * 2 * (1.8 + float64(rand.Intn(3))*.15)
		c.SpinnerDir = 1
		if variant%2 == 1 {
			c.SpinnerDir = -1
		}
	case terminusMechRelay:
		n := 6
		for i := 0; i < n; i++ {
			t := float64(i) / float64(n-1)
			y := midY + math.Sin(float64(i)*1.45+float64(variant))*((y1-y0)*.27)
			c.Points = append(c.Points, FPoint{X: x0 + (x1-x0)*t, Y: terminusClampY(y, y0, y1)})
		}
		// click -> drag -> click -> hold-track -> click -> finish click
		c.RelayModes = []int{0, 1, 0, 2, 0, 0}
	}
	return c
}

func terminusPickLoop(w, h int32) {
	perm := rand.Perm(terminusMechCount)
	for i := 0; i < 3; i++ {
		survivalBoss3LoopKinds[i] = perm[i]
	}
	survivalBoss3Slot = 0
	survivalBoss3Challenge = terminusGenerateChallenge(survivalBoss3LoopKinds[0], 0, w, h)
}

func terminusTempoScale() float64 {
	// The map accelerates naturally as the fight deepens and as a push nears
	// vulnerability. The floor keeps every pattern humanly readable.
	s := 1.0 - .12*(float64(survivalBoss3Damage)/float64(terminusBossHP)) - .055*survivalBoss3ShipTargetProgress
	if s < .81 {
		s = .81
	}
	return s
}

func terminusTempoDuration(d time.Duration) time.Duration {
	return time.Duration(float64(d) * terminusTempoScale())
}

func terminusSetTargetTiming(now time.Time, approach, late time.Duration) {
	c := &survivalBoss3Challenge
	approach = terminusTempoDuration(approach)
	late = time.Duration(float64(late) * math.Max(.86, terminusTempoScale()))
	c.TargetSpawnAt = now
	c.TargetHitAt = now.Add(approach)
	c.Deadline = c.TargetHitAt.Add(late)
}

func terminusTargetEarlyWindow(kind int) time.Duration {
	switch kind {
	case terminusMechStream:
		return 155 * time.Millisecond
	case terminusMechChain:
		return 220 * time.Millisecond
	case terminusMechJump:
		return 195 * time.Millisecond
	case terminusMechRelay:
		return 185 * time.Millisecond
	default:
		return 220 * time.Millisecond
	}
}

func terminusTargetTooEarly(now time.Time) bool {
	c := &survivalBoss3Challenge
	if c.TargetHitAt.IsZero() {
		return false
	}
	return now.Before(c.TargetHitAt.Add(-terminusTargetEarlyWindow(c.Kind)))
}

func terminusTargetOnTime(now time.Time) bool {
	c := &survivalBoss3Challenge
	if c.TargetHitAt.IsZero() {
		return true
	}
	return !now.Before(c.TargetHitAt.Add(-terminusTargetEarlyWindow(c.Kind))) && !now.After(c.Deadline)
}

func terminusActivateChallenge(now time.Time) {
	c := &survivalBoss3Challenge
	c.StartedAt = now
	c.Current = 0
	c.Holding = false
	c.Dragging = false
	c.PathProgress = 0
	c.MissSince = time.Time{}
	c.SpinnerAccum = 0
	c.SpinnerLast = 0
	c.RelayMode = 0
	c.ShipStartProgress = survivalBoss3ShipTargetProgress
	c.PushEarned = 0

	switch c.Kind {
	case terminusMechChain:
		c.Duration = 620 * time.Millisecond
		terminusSetTargetTiming(now, 820*time.Millisecond, 190*time.Millisecond)
	case terminusMechStream:
		c.Duration = 285 * time.Millisecond
		terminusSetTargetTiming(now, 610*time.Millisecond, 125*time.Millisecond)
	case terminusMechJump:
		c.Duration = 535 * time.Millisecond
		terminusSetTargetTiming(now, 760*time.Millisecond, 185*time.Millisecond)
	case terminusMechSlider:
		c.Duration = terminusTempoDuration(1650 * time.Millisecond)
		terminusSetTargetTiming(now, 790*time.Millisecond, 205*time.Millisecond)
	case terminusMechLaserSlider:
		c.Duration = terminusTempoDuration(1780 * time.Millisecond)
		terminusSetTargetTiming(now, 790*time.Millisecond, 205*time.Millisecond)
		for i := range c.Lasers {
			c.Lasers[i].WarnAt = now.Add(430 * time.Millisecond)
			c.Lasers[i].FireAt = now.Add(1450 * time.Millisecond)
			c.Lasers[i].EndAt = now.Add(1860 * time.Millisecond)
		}
	case terminusMechHoldRail:
		c.Duration = terminusTempoDuration(1700 * time.Millisecond)
		terminusSetTargetTiming(now, 790*time.Millisecond, 205*time.Millisecond)
	case terminusMechSpinner:
		c.Duration = terminusTempoDuration(2050 * time.Millisecond)
		terminusSetTargetTiming(now, 790*time.Millisecond, 205*time.Millisecond)
	case terminusMechRelay:
		c.Duration = 520 * time.Millisecond
		terminusSetTargetTiming(now, 700*time.Millisecond, 175*time.Millisecond)
	}
	status = fmt.Sprintf("TERMINUS // %s", terminusMechanicName(c.Kind))
}

func terminusBeginCombatLoop(now time.Time, w, h int32) {
	terminusPickLoop(w, h)
	terminusActivateChallenge(now)
	if terminusTurretNextAttack.IsZero() {
		terminusTurretNextAttack = now.Add(2100 * time.Millisecond)
	}
}

func terminusAdvanceTargetTimer(now time.Time) {
	c := &survivalBoss3Challenge
	switch c.Kind {
	case terminusMechChain:
		approach := 545 - c.Current*20
		if approach < 425 {
			approach = 425
		}
		terminusSetTargetTiming(now, time.Duration(approach)*time.Millisecond, 175*time.Millisecond)
	case terminusMechStream:
		approach := 300 - c.Current*5
		if approach < 235 {
			approach = 235
		}
		terminusSetTargetTiming(now, time.Duration(approach)*time.Millisecond, 115*time.Millisecond)
	case terminusMechJump:
		terminusSetTargetTiming(now, 565*time.Millisecond, 165*time.Millisecond)
	case terminusMechRelay:
		approach := 540
		if c.Current >= 3 {
			approach = 460
		}
		terminusSetTargetTiming(now, time.Duration(approach)*time.Millisecond, 165*time.Millisecond)
	}
}

func terminusPushChallengeFraction(fraction float64) {
	if fraction <= 0 || survivalBoss3Stage != survivalBoss3Combat {
		return
	}
	c := &survivalBoss3Challenge
	totalPush := 1.0 / float64(terminusMechanicsToExpose)
	amount := totalPush * fraction
	remaining := totalPush - c.PushEarned
	if amount > remaining {
		amount = remaining
	}
	if amount <= 0 {
		return
	}
	// Do not let a partial sequence expose the ship before the sequence itself is
	// complete. The final input is what earns the 25% vulnerable threshold.
	if c.PushEarned+amount < totalPush-.000001 && survivalBoss3ShipTargetProgress+amount > .985 {
		amount = math.Max(0, .985-survivalBoss3ShipTargetProgress)
	}
	if amount <= 0 {
		return
	}
	c.PushEarned += amount
	survivalBoss3ShipTargetProgress = math.Min(1, survivalBoss3ShipTargetProgress+amount)
}

func terminusCompleteChallenge(h uintptr, now time.Time, w, hgt int32) {
	survivalPerfectHits++
	survivalTotalHits++
	score += 900 + survivalBoss3Clears*55
	playSurvivalHitSound()
	survivalBoss3Clears++

	// Nine successful mechanics move 75% visible -> 25% visible. Sequence mechanics
	// can earn part of this movement hit-by-hit; completion adds only the remainder.
	push := 1.0 / float64(terminusMechanicsToExpose)
	remainingPush := push - survivalBoss3Challenge.PushEarned
	if remainingPush > 0 {
		survivalBoss3ShipTargetProgress = math.Min(1, survivalBoss3ShipTargetProgress+remainingPush)
	}

	if survivalBoss3ShipTargetProgress >= .999 {
		survivalBoss3SetStage(survivalBoss3Vulnerable, now)
		survivalBoss3VulnerableUntil = now.Add(8 * time.Second)
		survivalBoss3LastDamageClick = time.Time{}
		status = "TERMINUS VULNERABLE // ATTACK THE HULL"
		return
	}

	if survivalBoss3Slot < 2 {
		survivalBoss3Slot++
		survivalBoss3Challenge = terminusGenerateChallenge(survivalBoss3LoopKinds[survivalBoss3Slot], survivalBoss3Slot, w, hgt)
		terminusActivateChallenge(now)
		return
	}

	// Three-mechanic map complete. Immediately roll a new map; the first approach
	// circle appears on the left with no rest screen, giving the familiar osu jump.
	survivalBoss3LoopCount++
	terminusBeginCombatLoop(now, w, hgt)
}

func terminusDamagePlayer(h uintptr, now time.Time, reason string) {
	if survivalReviveImmune(now) {
		status = "REVIVE IMMUNITY"
		return
	}
	if now.Before(survivalBoss3MistakeGraceUntil) || state != StatePlaying {
		return
	}
	survivalBoss3MistakeGraceUntil = now.Add(520 * time.Millisecond)
	survivalHP--
	playSurvivalDamageTakenSound()
	survivalWaveDamageTaken = true
	survivalTotalDamageTaken++
	survivalCombo = 0
	if survivalHP <= 0 {
		survivalHP = 0
		survivalFail(h, reason)
		return
	}
	status = reason + " // -1 STATION HP"
	// A broken chain cannot be farmed for forward movement: roll back only the
	// partial push earned inside the current mechanic, then restart it cleanly.
	if survivalBoss3Challenge.PushEarned > 0 {
		survivalBoss3ShipTargetProgress = survivalBoss3Challenge.ShipStartProgress
	}
	// Restart only the current mechanic. The fight stays continuous and learnable.
	terminusActivateChallenge(now.Add(90 * time.Millisecond))
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
}

func finishSurvivalBoss3Clear(h uintptr, now time.Time) {
	analyticsEvent("boss_cleared", map[string]any{"boss": "TERMINUS", "wave": survivalWave, "kills": survivalKills})
	unlockSurvivalCheckpoint(survivalPostBoss3Wave)
	gameMeta.ArrayDefeats++
	unlockAchievement("SURV_ARRAY_DOWN")
	if survivalTotalDamageTaken == survivalBoss3DamageStart {
		unlockAchievement("SURV_ARRAY_FLAWLESS")
	}
	updatePersistentSurvivalProfileStats(survivalPostBoss3Wave, survivalKills, survivalBestCombo)
	survivalWave = survivalPostBoss3Wave
	survivalHP = survivalMaxHP()
	survivalWaveKills = 0
	survivalWaveSpawned = 0
	survivalEnemies = nil
	survivalPickup = nil
	survivalPickup2 = nil
	survivalWaveBreakUntil = time.Time{}
	resetSurvivalBoss3State()
	beginSurvivalWave()
	survivalLastTick = now
	setCapture.Call(h)
	status = "SECTOR 4 // ENDLESS DEFENCE"
	survivalWaveBannerText = fmt.Sprintf("SECTOR 4 // WAVE %d", survivalPostBoss3Wave)
	survivalWaveBannerUntil = now.Add(2 * time.Second)
	playLevelUpSound()
	requestPlayerProfileSync()
	invalidateRect.Call(h, 0, 0)
}

func pointLineDistance(p, a, b FPoint) float64 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return math.Hypot(p.X-(a.X+t*dx), p.Y-(a.Y+t*dy))
}

func terminusAngleDelta(a, b float64) float64 {
	d := b - a
	for d > math.Pi {
		d -= 2 * math.Pi
	}
	for d < -math.Pi {
		d += 2 * math.Pi
	}
	return d
}

func terminusPollHardwareCursor(h uintptr, w, hgt int32) {
	if h == 0 {
		return
	}
	var pt POINT
	if ok, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&pt))); ok == 0 {
		return
	}
	screenToClient.Call(h, uintptr(unsafe.Pointer(&pt)))
	cursorPos = FPoint{X: float64(pt.X), Y: float64(pt.Y)}
	cursorInArena = pointInArena(cursorPos, w, hgt)
}

func terminusWreckPoint(index int, w, h int32, now time.Time) (FPoint, int32) {
	r := arenaRect(w, h)
	spec := terminusWreckSpecs[index%len(terminusWreckSpecs)]
	aw := float64(r.Right - r.Left)
	ah := float64(r.Bottom - r.Top)
	age := 0.0
	if !survivalBoss3StartedAt.IsZero() {
		age = now.Sub(survivalBoss3StartedAt).Seconds()
	}
	yBob := math.Sin(age*.74+spec.Phase) * float64(sy(spec.BobPx, h))
	p := FPoint{
		X: float64(r.Left) + aw*spec.XFrac,
		Y: float64(r.Top) + ah*spec.YFrac + yBob,
	}
	sz := sx(spec.SizePx, w)
	if sz < sx(24, w) {
		sz = sx(24, w)
	}
	return p, sz
}

func terminusWreckAlive(index int, now time.Time) bool {
	if index < 0 || index >= len(terminusWreckState) {
		return false
	}
	return terminusWreckState[index].DestroyedUntil.IsZero() || !now.Before(terminusWreckState[index].DestroyedUntil)
}

func terminusMissileAllowedForChallenge() bool {
	c := &survivalBoss3Challenge
	if c.Dragging || c.Holding {
		return false
	}
	switch c.Kind {
	case terminusMechChain, terminusMechStream, terminusMechJump:
		return true
	case terminusMechRelay:
		return c.RelayMode == 0
	default:
		return false
	}
}

func terminusMissileCadence() time.Duration {
	d := 5600 * time.Millisecond
	if survivalBoss3ShipTargetProgress > .45 {
		d = 5000 * time.Millisecond
	}
	if survivalBoss3Damage >= 120 {
		d = 4450 * time.Millisecond
	}
	if survivalBoss3Damage >= 200 {
		d = 3900 * time.Millisecond
	}
	return d
}

func terminusSpawnMissile(now time.Time, w, h int32) {
	if len(terminusMissiles) >= 2 {
		return
	}
	ship := terminusShipRect(w, h)
	shipH := float64(ship.Bottom - ship.Top)
	lane := .39
	if (survivalBoss3LoopCount+len(terminusMissiles))%2 == 1 {
		lane = .62
	}
	p := FPoint{X: float64(ship.Left) + float64(sx(18, w)), Y: float64(ship.Top) + shipH*lane}
	target := cursorPos
	a := math.Atan2(target.Y-p.Y, target.X-p.X)
	// The missile leaves the left-facing launch bay with momentum. It can home, but
	// cannot instantly snap around cover, which is what makes the wreck interception
	// mechanic readable and skill-based.
	if d := terminusAngleDelta(math.Pi, a); d > .58 {
		a = math.Pi + .58
	} else if d < -.58 {
		a = math.Pi - .58
	}
	speed := float64(sx(205, w))
	m := TerminusMissile{
		P: p, V: FPoint{X: math.Cos(a) * speed, Y: math.Sin(a) * speed},
		SpawnedAt: now, LastAt: now,
	}
	m.Trail = append(m.Trail, p)
	terminusMissiles = append(terminusMissiles, m)
	terminusMissileNextAt = now.Add(terminusMissileCadence())
	playWarpRocketSound()
}

func terminusAddMissileBurst(p FPoint, now time.Time) {
	terminusMissileBursts = append(terminusMissileBursts, TerminusMissileBurst{P: p, At: now})
	if len(terminusMissileBursts) > 12 {
		terminusMissileBursts = terminusMissileBursts[len(terminusMissileBursts)-12:]
	}
}

func terminusUpdateMissiles(h uintptr, now time.Time, w, hgt int32) {
	if survivalBoss3Stage != survivalBoss3Combat {
		return
	}
	if terminusMissileNextAt.IsZero() {
		terminusMissileNextAt = now.Add(3100 * time.Millisecond)
	}
	if !now.Before(terminusMissileNextAt) && terminusMissileAllowedForChallenge() {
		terminusSpawnMissile(now, w, hgt)
	}
	r := arenaRect(w, hgt)
	out := terminusMissiles[:0]
	for i := range terminusMissiles {
		m := terminusMissiles[i]
		dt := now.Sub(m.LastAt).Seconds()
		if dt <= 0 {
			dt = .016
		}
		if dt > .05 {
			dt = .05
		}
		m.LastAt = now

		curA := math.Atan2(m.V.Y, m.V.X)
		desired := math.Atan2(cursorPos.Y-m.P.Y, cursorPos.X-m.P.X)
		turnRate := 1.18
		if survivalBoss3Damage >= 180 {
			turnRate = 1.34
		}
		delta := terminusAngleDelta(curA, desired)
		maxTurn := turnRate * dt
		if delta > maxTurn {
			delta = maxTurn
		} else if delta < -maxTurn {
			delta = -maxTurn
		}
		newA := curA + delta
		speed := math.Hypot(m.V.X, m.V.Y)
		maxSpeed := float64(sx(300, w))
		speed = math.Min(maxSpeed, speed+float64(sx(22, w))*dt)
		m.V = FPoint{X: math.Cos(newA) * speed, Y: math.Sin(newA) * speed}
		m.P.X += m.V.X * dt
		m.P.Y += m.V.Y * dt

		if len(m.Trail) == 0 || math.Hypot(m.P.X-m.Trail[len(m.Trail)-1].X, m.P.Y-m.Trail[len(m.Trail)-1].Y) >= float64(sx(7, w)) {
			m.Trail = append(m.Trail, m.P)
			if len(m.Trail) > 16 {
				m.Trail = m.Trail[len(m.Trail)-16:]
			}
		}

		hitWreck := false
		for wi := range terminusWreckSpecs {
			if !terminusWreckAlive(wi, now) {
				continue
			}
			wp, wsz := terminusWreckPoint(wi, w, hgt, now)
			if math.Hypot(m.P.X-wp.X, m.P.Y-wp.Y) <= float64(wsz)*.38+float64(sx(8, w)) {
				terminusWreckState[wi].DestroyedUntil = now.Add(6200 * time.Millisecond)
				terminusAddMissileBurst(m.P, now)
				go playSurvivalExplodeSound()
				hitWreck = true
				break
			}
		}
		if hitWreck {
			continue
		}

		if math.Hypot(m.P.X-cursorPos.X, m.P.Y-cursorPos.Y) <= float64(sx(12, w)) {
			terminusAddMissileBurst(m.P, now)
			terminusDamagePlayer(h, now, "Tracking missile impact")
			continue
		}

		margin := sx(120, w)
		if now.Sub(m.SpawnedAt) > 11*time.Second || m.P.X < float64(r.Left-margin) || m.P.X > float64(r.Right+margin) || m.P.Y < float64(r.Top-margin) || m.P.Y > float64(r.Bottom+margin) {
			continue
		}
		out = append(out, m)
	}
	terminusMissiles = out
}

func terminusTurretAnchor(index int, w, h int32) FPoint {
	r := arenaRect(w, h)
	fractions := [3]float64{.20, .45, .69}
	f := fractions[index%len(fractions)]
	return FPoint{
		X: float64(r.Left) + float64(r.Right-r.Left)*f,
		Y: float64(r.Bottom - sy(16, h)),
	}
}

func terminusTurretRay(index int, t *TerminusTurret, w, h int32) (FPoint, FPoint, FPoint) {
	pivot := terminusTurretAnchor(index, w, h)
	barrelW := float64(sx(88, w))
	if t.Heavy {
		barrelW = float64(sx(96, w))
	}
	dir := FPoint{X: math.Cos(t.Aim), Y: math.Sin(t.Aim)}
	centre := FPoint{X: pivot.X + dir.X*barrelW*.34, Y: pivot.Y + dir.Y*barrelW*.34}
	muzzle := FPoint{X: pivot.X + dir.X*barrelW*.76, Y: pivot.Y + dir.Y*barrelW*.76}
	end := FPoint{X: muzzle.X + dir.X*float64(sx(1800, w)), Y: muzzle.Y + dir.Y*float64(sx(1800, w))}
	return centre, muzzle, end
}

func terminusTurretCadence() time.Duration {
	// Deliberate final-boss pressure: cannons are dangerous because they overlap
	// precision play, not because they machine-gun the player. Even late in the
	// fight there is enough time to read one telegraph before the next begins.
	cadence := 2750 * time.Millisecond
	if survivalBoss3ShipTargetProgress > .48 {
		cadence = 2450 * time.Millisecond
	}
	if survivalBoss3Damage >= 120 {
		cadence = 2200 * time.Millisecond
	}
	if survivalBoss3Damage >= 200 {
		cadence = 1950 * time.Millisecond
	}
	return cadence
}

func terminusScheduleTurret(now time.Time) {
	idx := terminusTurretCursor % len(terminusTurrets)
	terminusTurretCursor++
	t := &terminusTurrets[idx]
	t.Active = true
	t.Heavy = (terminusTurretCursor+survivalBoss3LoopCount)%4 == 0
	t.WarnAt = now
	if t.Heavy {
		t.LockAt = now.Add(1250 * time.Millisecond)
		t.FireAt = now.Add(1780 * time.Millisecond)
		t.EndAt = now.Add(2240 * time.Millisecond)
	} else {
		t.LockAt = now.Add(1050 * time.Millisecond)
		t.FireAt = now.Add(1500 * time.Millisecond)
		t.EndAt = now.Add(1880 * time.Millisecond)
	}
	t.Target = cursorPos
	terminusTurretNextAttack = now.Add(terminusTurretCadence())
}

func terminusUpdateTurrets(h uintptr, now time.Time, w, hgt int32) {
	if survivalBoss3Stage != survivalBoss3Combat {
		return
	}
	if terminusTurretNextAttack.IsZero() {
		terminusTurretNextAttack = now.Add(1900 * time.Millisecond)
	}
	if !now.Before(terminusTurretNextAttack) {
		terminusScheduleTurret(now)
	}
	for i := range terminusTurrets {
		t := &terminusTurrets[i]
		pivot := terminusTurretAnchor(i, w, hgt)
		if t.Active {
			if now.Before(t.LockAt) {
				// Track the live cursor during the telegraph, then hard-lock before firing.
				t.Target = cursorPos
			}
			desired := math.Atan2(t.Target.Y-pivot.Y, t.Target.X-pivot.X)
			t.Aim += terminusAngleDelta(t.Aim, desired) * .28
			if now.After(t.EndAt) {
				t.Active = false
				continue
			}
			if !now.Before(t.FireAt) && now.Before(t.EndAt) {
				_, muzzle, end := terminusTurretRay(i, t, w, hgt)
				if pointLineDistance(cursorPos, muzzle, end) < float64(sx(7, w)) {
					terminusDamagePlayer(h, now, "Terminus cannon strike")
					return
				}
			}
		} else {
			// Resting barrels remain visibly armed and slowly return to an upward pose.
			t.Aim += terminusAngleDelta(t.Aim, -math.Pi/2) * .06
		}
	}
}

func terminusAddHitBurst(p FPoint, now time.Time, slot int) {
	terminusHitBursts = append(terminusHitBursts, TerminusHitBurst{P: p, At: now, Slot: slot})
	if len(terminusHitBursts) > 24 {
		terminusHitBursts = terminusHitBursts[len(terminusHitBursts)-24:]
	}
}

func terminusUpdateChallenge(h uintptr, now time.Time, w, hgt int32) {
	c := &survivalBoss3Challenge
	if !c.Deadline.IsZero() && now.After(c.Deadline) {
		terminusDamagePlayer(h, now, "Terminus timing missed")
		return
	}

	// Any active Terminus beam can punish the cursor while the mechanic continues.
	for _, l := range c.Lasers {
		if !now.Before(l.FireAt) && now.Before(l.EndAt) && pointLineDistance(cursorPos, l.A, l.B) < float64(sx(8, w)) {
			terminusDamagePlayer(h, now, "Terminus laser strike")
			return
		}
	}

	switch c.Kind {
	case terminusMechSlider, terminusMechLaserSlider:
		if !c.Dragging {
			return
		}
		prog, d := terminusNearestPathProgress(cursorPos, c.Path)
		corridor := float64(sx(15, w))
		if d > corridor {
			if c.MissSince.IsZero() {
				c.MissSince = now
			} else if now.Sub(c.MissSince) > 78*time.Millisecond {
				terminusDamagePlayer(h, now, "Slider left the rail")
				return
			}
		} else {
			c.MissSince = time.Time{}
		}
		if prog+0.020 < c.PathProgress {
			terminusDamagePlayer(h, now, "Slider reversed off route")
			return
		}
		if prog > c.PathProgress {
			c.PathProgress = prog
		}
		if c.PathProgress >= .985 && len(c.Path) > 0 {
			end := c.Path[len(c.Path)-1]
			if math.Hypot(cursorPos.X-end.X, cursorPos.Y-end.Y) <= float64(sx(22, w)) {
				c.Dragging = false
				c.Holding = false
				terminusAddHitBurst(end, now, c.Slot)
				terminusCompleteChallenge(h, now, w, hgt)
				return
			}
		}

	case terminusMechHoldRail:
		if !c.Holding {
			return
		}
		prog := math.Min(1, now.Sub(c.StartedAt).Seconds()/c.Duration.Seconds())
		target := terminusPointAtPath(c.Path, prog)
		if math.Hypot(cursorPos.X-target.X, cursorPos.Y-target.Y) > float64(sx(16, w)) {
			if c.MissSince.IsZero() {
				c.MissSince = now
			} else if now.Sub(c.MissSince) > 78*time.Millisecond {
				terminusDamagePlayer(h, now, "Hold rail lost")
				return
			}
		} else {
			c.MissSince = time.Time{}
		}
		if prog >= 1 {
			terminusAddHitBurst(target, now, c.Slot)
			terminusCompleteChallenge(h, now, w, hgt)
		}

	case terminusMechSpinner:
		if !c.Holding {
			return
		}
		v := FPoint{X: cursorPos.X - c.SpinnerCentre.X, Y: cursorPos.Y - c.SpinnerCentre.Y}
		rad := math.Hypot(v.X, v.Y)
		minR := float64(sx(42, w))
		maxR := float64(sx(72, w))
		if rad < minR || rad > maxR {
			if c.MissSince.IsZero() {
				c.MissSince = now
			} else if now.Sub(c.MissSince) > 90*time.Millisecond {
				terminusDamagePlayer(h, now, "Orbit charge lost")
				return
			}
		} else {
			c.MissSince = time.Time{}
		}
		ang := math.Atan2(v.Y, v.X)
		if c.SpinnerLast != 0 {
			d := terminusAngleDelta(c.SpinnerLast, ang) * c.SpinnerDir
			if d > 0 {
				c.SpinnerAccum += d
			}
		}
		c.SpinnerLast = ang
		if c.SpinnerAccum >= c.SpinnerNeed {
			terminusAddHitBurst(c.SpinnerCentre, now, c.Slot)
			terminusCompleteChallenge(h, now, w, hgt)
		}

	case terminusMechRelay:
		if c.Current >= len(c.Points) || c.Current >= len(c.RelayModes) {
			return
		}
		mode := c.RelayModes[c.Current]
		if mode == 1 && c.Dragging {
			if c.Current+1 >= len(c.Points) {
				return
			}
			a, b := c.Points[c.Current], c.Points[c.Current+1]
			path := terminusBuildSmoothPath(a, b, c.Current+c.Slot, a.Y-float64(sy(50, hgt)), a.Y+float64(sy(50, hgt)), 18)
			prog, d := terminusNearestPathProgress(cursorPos, path)
			if d > float64(sx(15, w)) {
				if c.MissSince.IsZero() {
					c.MissSince = now
				} else if now.Sub(c.MissSince) > 78*time.Millisecond {
					terminusDamagePlayer(h, now, "Relay drag left the rail")
					return
				}
			} else {
				c.MissSince = time.Time{}
			}
			if prog+0.02 < c.PathProgress {
				terminusDamagePlayer(h, now, "Relay drag reversed")
				return
			}
			if prog > c.PathProgress {
				c.PathProgress = prog
			}
			if c.PathProgress >= .985 && math.Hypot(cursorPos.X-b.X, cursorPos.Y-b.Y) <= float64(sx(21, w)) {
				terminusAddHitBurst(b, now, c.Slot)
				c.Dragging = false
				c.Holding = false
				c.Current += 2
				if c.Current >= len(c.Points) {
					terminusCompleteChallenge(h, now, w, hgt)
					return
				}
				c.StartedAt = now
				c.MissSince = time.Time{}
				terminusAdvanceTargetTimer(now)
			}
		} else if mode == 2 && c.Holding {
			if c.Current+1 >= len(c.Points) {
				return
			}
			elapsed := now.Sub(c.StartedAt).Seconds()
			prog := math.Min(1, elapsed/.88)
			a, b := c.Points[c.Current], c.Points[c.Current+1]
			path := terminusBuildSmoothPath(a, b, c.Current+c.Slot, a.Y-float64(sy(50, hgt)), a.Y+float64(sy(50, hgt)), 18)
			target := terminusPointAtPath(path, prog)
			c.PathProgress = prog
			if math.Hypot(cursorPos.X-target.X, cursorPos.Y-target.Y) > float64(sx(16, w)) {
				if c.MissSince.IsZero() {
					c.MissSince = now
				} else if now.Sub(c.MissSince) > 78*time.Millisecond {
					terminusDamagePlayer(h, now, "Relay hold lost")
					return
				}
			} else {
				c.MissSince = time.Time{}
			}
			if prog >= 1 {
				terminusAddHitBurst(b, now, c.Slot)
				c.Holding = false
				c.Current += 2
				if c.Current >= len(c.Points) {
					terminusCompleteChallenge(h, now, w, hgt)
					return
				}
				c.StartedAt = now
				terminusAdvanceTargetTimer(now)
			}
		}
	}
}

func updateSurvivalBoss3(h uintptr, now time.Time, w, hgt int32) bool {
	if !survivalBoss3Active() {
		return false
	}
	now = survivalBossFightNow(now, survivalBoss3StartedAt)
	survivalBoss3LastUpdate = now
	// Poll the OS cursor directly every game tick. Rendering or synchronous GDI work
	// from the cannon layer can no longer leave the custom cursor frozen on an old
	// WM_MOUSEMOVE coordinate.
	terminusPollHardwareCursor(h, w, hgt)
	terminusUpdateShipMotion(now)

	if (survivalBoss3Stage == survivalBoss3Combat || survivalBoss3Stage == survivalBoss3Vulnerable) &&
		terminusMeteorCollision(cursorPos, w, hgt, now) {
		terminusKillOnMeteor(h, now)
		return true
	}

	switch survivalBoss3Stage {
	case survivalBoss3Intro:
		if now.Sub(survivalBoss3StageAt) >= 2300*time.Millisecond {
			survivalBoss3SetStage(survivalBoss3Combat, now)
			terminusBeginCombatLoop(now, w, hgt)
			survivalBoss3MistakeGraceUntil = now.Add(520 * time.Millisecond)
			status = "TERMINUS // THREE-MECHANIC ASSAULT"
		}

	case survivalBoss3Combat:
		// The mothership is physically dangerous whenever it is not flashing red.
		if now.After(survivalBoss3MistakeGraceUntil) && terminusShipPixelSolid(cursorPos, w, hgt) {
			terminusDamagePlayer(h, now, "Cursor touched The Terminus")
			return true
		}
		terminusUpdateTurrets(h, now, w, hgt)
		terminusUpdateMissiles(h, now, w, hgt)
		terminusUpdateChallenge(h, now, w, hgt)

	case survivalBoss3Vulnerable:
		if survivalBoss3Damage >= terminusBossHP {
			survivalBoss3SetStage(survivalBoss3Dying, now)
			status = "TERMINUS // CRITICAL FAILURE"
			playSurvivalExplodeSound()
			return true
		}
		if !survivalBoss3VulnerableUntil.IsZero() && !now.Before(survivalBoss3VulnerableUntil) {
			// Surviving ship surges back in, but not all the way to 75%. This keeps later
			// loops relentless while still requiring another left-to-right push.
			survivalBoss3ShipTargetProgress = .36
			survivalBoss3SetStage(survivalBoss3Combat, now)
			terminusBeginCombatLoop(now, w, hgt)
			survivalBoss3MistakeGraceUntil = now.Add(520 * time.Millisecond)
			status = "TERMINUS RECOVERED // PUSH FORWARD"
		}

	case survivalBoss3Dying:
		survivalBoss3ShipTargetProgress = 1.12
		if now.Sub(survivalBoss3StageAt) > 3200*time.Millisecond {
			survivalBoss3SetStage(survivalBoss3Return, now)
		}
	case survivalBoss3Return:
		if now.Sub(survivalBoss3StageAt) > 1200*time.Millisecond {
			finishSurvivalBoss3Clear(h, now)
			return true
		}
	}
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
	return true
}

func terminusHitRadius(w int32, stream bool) float64 {
	if stream {
		return float64(sx(16, w))
	}
	return float64(sx(21, w))
}

func terminusHandleSequenceClick(h uintptr, now time.Time, p FPoint, w, hgt int32) bool {
	c := &survivalBoss3Challenge
	if c.Current < 0 || c.Current >= len(c.Points) {
		return true
	}
	stream := c.Kind == terminusMechStream
	radius := terminusHitRadius(w, stream)
	d := math.Hypot(p.X-c.Points[c.Current].X, p.Y-c.Points[c.Current].Y)

	// Correct-object clicks are never treated as a mysterious failure. A click that
	// is visibly too early is simply ignored; once the approach ring enters the
	// hit window, the same object responds immediately.
	if d <= radius && terminusTargetTooEarly(now) {
		status = "TERMINUS // WAIT FOR THE APPROACH RING"
		return true
	}
	if d > radius {
		// During pre-empt/fade-in, stray clicks should not punish the player. Once
		// the target is live, clicking elsewhere is a genuine accuracy mistake.
		if terminusTargetTooEarly(now) {
			return true
		}
		terminusDamagePlayer(h, now, "Wrong Terminus target")
		return true
	}
	if !terminusTargetOnTime(now) {
		terminusDamagePlayer(h, now, "Terminus hit mistimed")
		return true
	}

	terminusAddHitBurst(c.Points[c.Current], now, c.Slot)
	if len(c.Points) > 0 {
		terminusPushChallengeFraction(1.0 / float64(len(c.Points)))
	}
	c.Current++
	playSurvivalHitSound()
	score += 90
	if c.Current >= len(c.Points) {
		terminusCompleteChallenge(h, now, w, hgt)
		return true
	}
	terminusAdvanceTargetTimer(now)
	return true
}

func terminusHandleRelayClick(h uintptr, now time.Time, p FPoint, right bool, w, hgt int32) bool {
	c := &survivalBoss3Challenge
	if c.Current >= len(c.Points) || c.Current >= len(c.RelayModes) {
		return true
	}
	mode := c.RelayModes[c.Current]
	d := math.Hypot(p.X-c.Points[c.Current].X, p.Y-c.Points[c.Current].Y)
	if d <= float64(sx(23, w)) && terminusTargetTooEarly(now) {
		status = "TERMINUS // WAIT FOR THE APPROACH RING"
		return true
	}
	if d > float64(sx(23, w)) {
		if terminusTargetTooEarly(now) {
			return true
		}
		terminusDamagePlayer(h, now, "Relay target missed")
		return true
	}
	if !terminusTargetOnTime(now) {
		terminusDamagePlayer(h, now, "Relay hit mistimed")
		return true
	}
	if mode == 0 {
		terminusAddHitBurst(c.Points[c.Current], now, c.Slot)
		terminusPushChallengeFraction(1.0 / float64(len(c.Points)))
		c.Current++
		playSurvivalHitSound()
		if c.Current >= len(c.Points) {
			terminusCompleteChallenge(h, now, w, hgt)
			return true
		}
		terminusAdvanceTargetTimer(now)
		return true
	}
	if mode == 1 {
		c.Dragging = true
		c.Holding = true
		c.PathProgress = 0
		c.TargetHitAt = time.Time{}
		c.Deadline = now.Add(1180 * time.Millisecond)
		return true
	}
	if mode == 2 {
		c.Holding = true
		c.StartedAt = now
		c.TargetHitAt = time.Time{}
		c.Deadline = now.Add(1220 * time.Millisecond)
		return true
	}
	return true
}

func survivalBoss3HandleClick(h uintptr, p FPoint, right bool) bool {
	if !survivalBoss3Active() {
		return false
	}
	realNow := time.Now()
	now := survivalBossFightNow(realNow, survivalBoss3StartedAt)
	w, hgt := getClient(h)

	if survivalBoss3Stage == survivalBoss3Vulnerable {
		if !terminusShipPixelSolid(p, w, hgt) {
			return true
		}
		minGap := time.Second / terminusDamageCPS
		if !survivalBoss3LastDamageClick.IsZero() && realNow.Sub(survivalBoss3LastDamageClick) < minGap {
			return true
		}
		survivalBoss3LastDamageClick = realNow
		survivalBoss3Damage++
		survivalBoss3ShipHitFlashUntil = now.Add(65 * time.Millisecond)
		score += 125
		playSurvivalHitSound()
		if survivalBoss3Damage >= terminusBossHP {
			survivalBoss3SetStage(survivalBoss3Dying, now)
			status = "TERMINUS // CRITICAL FAILURE"
			playSurvivalExplodeSound()
		} else {
			status = fmt.Sprintf("TERMINUS // %d / %d DAMAGE", survivalBoss3Damage, terminusBossHP)
		}
		invalidateSurvivalHUD(h)
		invalidateSurvivalArena(h)
		return true
	}

	if survivalBoss3Stage != survivalBoss3Combat {
		return true
	}
	c := &survivalBoss3Challenge
	switch c.Kind {
	case terminusMechChain, terminusMechStream, terminusMechJump:
		return terminusHandleSequenceClick(h, now, p, w, hgt)

	case terminusMechSlider, terminusMechLaserSlider:
		if c.Dragging {
			return true
		}
		if len(c.Path) == 0 {
			return true
		}
		d := math.Hypot(p.X-c.Path[0].X, p.Y-c.Path[0].Y)
		if d <= float64(sx(25, w)) && terminusTargetTooEarly(now) {
			status = "TERMINUS // WAIT FOR THE APPROACH RING"
			return true
		}
		if d > float64(sx(25, w)) {
			if terminusTargetTooEarly(now) {
				return true
			}
			terminusDamagePlayer(h, now, "Slider start missed")
			return true
		}
		if !terminusTargetOnTime(now) {
			terminusDamagePlayer(h, now, "Slider start mistimed")
			return true
		}
		terminusAddHitBurst(c.Path[0], now, c.Slot)
		c.Dragging = true
		c.Holding = true
		c.PathProgress = 0
		c.StartedAt = now
		c.TargetHitAt = time.Time{}
		c.Deadline = now.Add(c.Duration + 180*time.Millisecond)
		return true

	case terminusMechHoldRail:
		if len(c.Path) == 0 {
			return true
		}
		d := math.Hypot(p.X-c.Path[0].X, p.Y-c.Path[0].Y)
		if d <= float64(sx(25, w)) && terminusTargetTooEarly(now) {
			status = "TERMINUS // WAIT FOR THE APPROACH RING"
			return true
		}
		if d > float64(sx(25, w)) {
			if terminusTargetTooEarly(now) {
				return true
			}
			terminusDamagePlayer(h, now, "Hold rail start missed")
			return true
		}
		if !terminusTargetOnTime(now) {
			terminusDamagePlayer(h, now, "Hold rail start mistimed")
			return true
		}
		terminusAddHitBurst(c.Path[0], now, c.Slot)
		c.Holding = true
		c.StartedAt = now
		c.TargetHitAt = time.Time{}
		c.Deadline = now.Add(c.Duration + 130*time.Millisecond)
		return true

	case terminusMechSpinner:
		d := math.Hypot(p.X-c.SpinnerCentre.X, p.Y-c.SpinnerCentre.Y)
		if d <= float64(sx(25, w)) && terminusTargetTooEarly(now) {
			status = "TERMINUS // WAIT FOR THE APPROACH RING"
			return true
		}
		if d > float64(sx(25, w)) {
			if terminusTargetTooEarly(now) {
				return true
			}
			terminusDamagePlayer(h, now, "Orbit charge start missed")
			return true
		}
		if !terminusTargetOnTime(now) {
			terminusDamagePlayer(h, now, "Orbit charge mistimed")
			return true
		}
		terminusAddHitBurst(c.SpinnerCentre, now, c.Slot)
		c.Holding = true
		c.StartedAt = now
		c.TargetHitAt = time.Time{}
		c.Deadline = now.Add(c.Duration)
		c.SpinnerLast = 0
		c.SpinnerAccum = 0
		return true

	case terminusMechRelay:
		return terminusHandleRelayClick(h, now, p, right, w, hgt)
	}
	return true
}

func survivalBoss3HandleRelease(h uintptr, p FPoint, right bool) bool {
	if !survivalBoss3Active() || survivalBoss3Stage != survivalBoss3Combat {
		return false
	}
	now := survivalBossFightNow(time.Now(), survivalBoss3StartedAt)
	w, hgt := getClient(h)
	c := &survivalBoss3Challenge
	switch c.Kind {
	case terminusMechSlider, terminusMechLaserSlider:
		if !c.Dragging {
			return true
		}
		end := c.Path[len(c.Path)-1]
		if c.PathProgress >= .965 && math.Hypot(p.X-end.X, p.Y-end.Y) <= float64(sx(24, w)) {
			c.Dragging = false
			c.Holding = false
			terminusAddHitBurst(end, now, c.Slot)
			terminusCompleteChallenge(h, now, w, hgt)
			return true
		}
		terminusDamagePlayer(h, now, "Slider released too early")
		return true
	case terminusMechHoldRail:
		if c.Holding {
			terminusDamagePlayer(h, now, "Hold rail released too early")
		}
		return true
	case terminusMechSpinner:
		if c.Holding {
			terminusDamagePlayer(h, now, "Orbit charge released too early")
		}
		return true
	case terminusMechRelay:
		if c.Current >= len(c.RelayModes) {
			return true
		}
		mode := c.RelayModes[c.Current]
		if mode == 1 && c.Dragging {
			if c.Current+1 < len(c.Points) {
				end := c.Points[c.Current+1]
				if c.PathProgress >= .94 && math.Hypot(p.X-end.X, p.Y-end.Y) <= float64(sx(23, w)) {
					terminusAddHitBurst(end, now, c.Slot)
					c.Dragging = false
					c.Holding = false
					c.Current += 2
					if c.Current >= len(c.Points) {
						terminusCompleteChallenge(h, now, w, hgt)
					} else {
						terminusAdvanceTargetTimer(now)
					}
					return true
				}
			}
			terminusDamagePlayer(h, now, "Relay drag released early")
			return true
		}
		if mode == 2 && c.Holding {
			terminusDamagePlayer(h, now, "Relay hold released early")
			return true
		}
	}
	return true
}

func survivalBoss3HealthUnits() (remain, total int) {
	total = terminusBossHP
	remain = terminusBossHP - survivalBoss3Damage
	if remain < 0 {
		remain = 0
	}
	return
}

func drawTerminusAsset(hdc uintptr, a *TerminusAsset, dst RECT) {
	if len(a.data) >= int(a.w*a.h*4) {
		drawCachedBGRASprite(hdc, &a.sprite, a.data, a.w, a.h, dst)
	}
}

func terminusComboColours(slot int) (fill, accent uintptr) {
	switch slot % 3 {
	case 0:
		return rgb(23, 122, 171), rgb(83, 224, 255)
	case 1:
		return rgb(173, 118, 24), rgb(255, 205, 54)
	default:
		return rgb(170, 43, 43), rgb(255, 82, 70)
	}
}

func terminusBrush(col uintptr) uintptr {
	if b := terminusBrushCache[col]; b != 0 {
		return b
	}
	b, _, _ := createSolidBrush.Call(col)
	terminusBrushCache[col] = b
	return b
}

func terminusFillCircle(hdc uintptr, x, y, radius int32, fill, border uintptr, borderW int32) {
	if radius <= 0 {
		return
	}
	b := terminusBrush(fill)
	p := survivalPen(borderW, border)
	if b == 0 || p == 0 {
		return
	}
	oldB, _, _ := selectObject.Call(hdc, b)
	oldP, _, _ := selectObject.Call(hdc, p)
	ellipse.Call(hdc, uintptr(x-radius), uintptr(y-radius), uintptr(x+radius+1), uintptr(y+radius+1))
	selectObject.Call(hdc, oldP)
	selectObject.Call(hdc, oldB)
}

func terminusDrawPolyline(hdc uintptr, path []FPoint, pen uintptr) {
	if len(path) < 2 || pen == 0 {
		return
	}
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		drawSurvivalLineWithPen(hdc, pen, int32(a.X), int32(a.Y), int32(b.X), int32(b.Y))
	}
}

func terminusDrawPartialPath(hdc uintptr, path []FPoint, w int32, progress float64, pen uintptr) {
	if len(path) < 2 || progress <= 0 || pen == 0 {
		return
	}
	if progress >= 1 {
		terminusDrawPolyline(hdc, path, pen)
		return
	}
	total := terminusPathLength(path)
	want := total * progress
	walk := 0.0
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		seg := math.Hypot(b.X-a.X, b.Y-a.Y)
		if walk+seg <= want {
			drawSurvivalLineWithPen(hdc, pen, int32(a.X), int32(a.Y), int32(b.X), int32(b.Y))
			walk += seg
			continue
		}
		if want > walk {
			f := (want - walk) / math.Max(seg, .001)
			q := FPoint{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f}
			drawSurvivalLineWithPen(hdc, pen, int32(a.X), int32(a.Y), int32(q.X), int32(q.Y))
		}
		break
	}
}

func terminusDrawPath(hdc uintptr, path []FPoint, w int32, active bool) {
	if len(path) < 2 {
		return
	}
	fill, accent := terminusComboColours(survivalBoss3Challenge.Slot)
	if !active {
		fill = rgb(35, 48, 58)
		accent = rgb(85, 103, 116)
	}

	// Premium rhythm-game rail: deep shadow -> white rim -> coloured rim -> dark
	// playable channel. The luminous completion trace sits inside that channel.
	terminusDrawPolyline(hdc, path, survivalPen(sx(32, w), rgb(3, 6, 12)))
	terminusDrawPolyline(hdc, path, survivalPen(sx(27, w), rgb(236, 241, 246)))
	terminusDrawPolyline(hdc, path, survivalPen(sx(23, w), accent))
	terminusDrawPolyline(hdc, path, survivalPen(sx(17, w), rgb(10, 20, 30)))
	terminusDrawPolyline(hdc, path, survivalPen(sx(11, w), fill))

	if survivalBoss3Challenge.Dragging || survivalBoss3Challenge.Holding {
		terminusDrawPartialPath(hdc, path, w, survivalBoss3Challenge.PathProgress, survivalPen(sx(12, w), accent))
		terminusDrawPartialPath(hdc, path, w, survivalBoss3Challenge.PathProgress, survivalPen(sx(4, w), rgb(247, 251, 255)))
	}

	// Regular ticks visually establish slider velocity before the player commits.
	for _, f := range []float64{.16, .32, .48, .64, .80} {
		q := terminusPointAtPath(path, f)
		terminusFillCircle(hdc, int32(q.X), int32(q.Y), sx(3, w), rgb(248, 251, 255), accent, 1)
	}
}

func terminusTimerFraction(now, started, deadline time.Time) float64 {
	if deadline.IsZero() || started.IsZero() || !deadline.After(started) {
		return 0
	}
	f := now.Sub(started).Seconds() / deadline.Sub(started).Seconds()
	return math.Max(0, math.Min(1, f))
}

func terminusDrawCountdown(hdc uintptr, w, h int32, p FPoint, now, deadline time.Time) {
	if hudTinyFont == 0 || deadline.IsZero() {
		return
	}
	remain := deadline.Sub(now).Seconds()
	if remain < 0 {
		remain = 0
	}
	col := rgb(255, 211, 62)
	if remain < .45 {
		col = rgb(255, 70, 54)
	}
	old, _, _ := selectObject.Call(hdc, hudTinyFont)
	setBkMode.Call(hdc, TRANSPARENT)
	setTextColor.Call(hdc, col)
	text := fmt.Sprintf("%.1f", remain)
	centeredTextOut(hdc, int32(p.X)-sx(38, w), int32(p.X)+sx(38, w), int32(p.Y)-sy(45, h), text)
	selectObject.Call(hdc, old)
}

func terminusDrawTimerDots(hdc uintptr, w int32, p FPoint, frac float64, radius int32, accent uintptr) {
	frac = math.Max(0, math.Min(1, frac))
	const dots = 24
	remain := int(math.Round((1 - frac) * dots))
	for i := 0; i < dots; i++ {
		a := -math.Pi/2 + float64(i)/dots*math.Pi*2
		x := p.X + math.Cos(a)*float64(radius)
		y := p.Y + math.Sin(a)*float64(radius)
		col := rgb(48, 58, 69)
		if i < remain {
			col = accent
		}
		terminusFillCircle(hdc, int32(x), int32(y), sx(2, w), col, col, 1)
	}
}

func terminusDrawFollowDots(hdc uintptr, w int32, a, b FPoint, accent uintptr) {
	for i := 1; i <= 4; i++ {
		f := float64(i) / 5
		p := FPoint{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f}
		r := sx(2, w)
		if i == 3 {
			r = sx(3, w)
		}
		terminusFillCircle(hdc, int32(p.X), int32(p.Y), r, accent, rgb(245, 249, 252), 1)
	}
}

func terminusDrawTarget(hdc uintptr, w, h int32, p FPoint, index int, active bool, timerFrac float64, satelliteIndex int, small bool) {
	base := sx(35, w)
	if small {
		base = sx(27, w)
	}
	if base < 14 {
		base = 14
	}
	fill, accent := terminusComboColours(survivalBoss3Challenge.Slot)
	if !active {
		fill = rgb(31, 43, 54)
		accent = rgb(92, 111, 126)
	}

	// osu-inspired object stack: deep separation shadow, vivid combo rim, thick white
	// keyline and a clean inner plate. The hierarchy remains legible over the busy fleet.
	terminusFillCircle(hdc, int32(p.X)+sx(2, w), int32(p.Y)+sx(3, w), base+sx(7, w), rgb(1, 4, 9), rgb(1, 4, 9), 1)
	terminusFillCircle(hdc, int32(p.X), int32(p.Y), base+sx(5, w), accent, accent, 1)
	terminusFillCircle(hdc, int32(p.X), int32(p.Y), base+sx(1, w), rgb(246, 249, 252), rgb(255, 255, 255), 1)
	terminusFillCircle(hdc, int32(p.X), int32(p.Y), base-sx(3, w), fill, accent, 2)
	terminusFillCircle(hdc, int32(p.X), int32(p.Y), base-sx(9, w), rgb(8, 15, 24), rgb(235, 242, 247), 1)

	// The old 1.32x sprite was visually lost inside the circle. The satellite now
	// occupies almost the complete hit object while the small centre remains the
	// actual interaction anchor.
	sat := &terminusSat[satelliteIndex%8]
	sw := int32(float64(base) * 2.02)
	sh := int32(float64(sw) * float64(sat.h) / float64(sat.w))
	drawTerminusAsset(hdc, sat, RECT{int32(p.X) - sw/2, int32(p.Y) - sh/2, int32(p.X) + sw/2, int32(p.Y) + sh/2})

	if active {
		approach := base + sx(5, w) + int32(float64(sx(62, w))*(1-timerFrac))
		drawSurvivalCircleWithPen(hdc, survivalPen(4, accent), int32(p.X), int32(p.Y), approach)
		drawSurvivalCircleWithPen(hdc, survivalPen(2, rgb(252, 253, 255)), int32(p.X), int32(p.Y), approach+sx(4, w))
		if timerFrac > .72 {
			pulse := sx(3, w) + int32((math.Sin(float64(survivalBossFightNow(time.Now(), survivalBoss3StartedAt).UnixMilli())*.026)+1)*float64(sx(2, w)))
			drawSurvivalCircleWithPen(hdc, survivalPen(3, rgb(255, 255, 255)), int32(p.X), int32(p.Y), base+pulse)
		}
	}

	if hudSmallFont != 0 && index > 0 {
		// Large centred sequence numbers match the immediate readability of osu hit
		// circles while the satellite remains visible around and behind the glyph.
		font := hudStatFont
		if font == 0 {
			font = hudSmallFont
		}
		if index >= 10 && hudSmallFont != 0 {
			font = hudSmallFont
		}
		old, _, _ := selectObject.Call(hdc, font)
		setBkMode.Call(hdc, TRANSPARENT)
		text := fmt.Sprintf("%d", index)
		textR := base - sx(7, w)
		y := verticallyCenteredTextY(hdc, font, RECT{int32(p.X) - textR, int32(p.Y) - textR, int32(p.X) + textR, int32(p.Y) + textR}, text)
		setTextColor.Call(hdc, rgb(0, 0, 0))
		centeredTextOut(hdc, int32(p.X)-textR+sx(2, w), int32(p.X)+textR+sx(2, w), y+sy(2, h), text)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		centeredTextOut(hdc, int32(p.X)-textR, int32(p.X)+textR, y, text)
		selectObject.Call(hdc, old)
	}
}

func terminusDrawHitBursts(hdc uintptr, w, h int32, now time.Time) {
	out := terminusHitBursts[:0]
	for _, b := range terminusHitBursts {
		age := now.Sub(b.At).Seconds()
		if age < 0 || age > .34 {
			continue
		}
		out = append(out, b)
		_, accent := terminusComboColours(b.Slot)
		r := sx(10, w) + int32(age*float64(sx(72, w)))
		drawSurvivalCircleWithPen(hdc, survivalPen(3, rgb(255, 255, 255)), int32(b.P.X), int32(b.P.Y), r)
		drawSurvivalCircleWithPen(hdc, survivalPen(2, accent), int32(b.P.X), int32(b.P.Y), r+sx(5, w))
		for i := 0; i < 4; i++ {
			a := float64(i)*math.Pi/2 + age*5.5
			l0 := float64(r) + float64(sx(3, w))
			l1 := l0 + float64(sx(12, w))
			x0 := b.P.X + math.Cos(a)*l0
			y0 := b.P.Y + math.Sin(a)*l0
			x1 := b.P.X + math.Cos(a)*l1
			y1 := b.P.Y + math.Sin(a)*l1
			drawSurvivalLineWithPen(hdc, survivalPen(2, accent), int32(x0), int32(y0), int32(x1), int32(y1))
		}
	}
	terminusHitBursts = out
}

func terminusDrawWrecks(hdc uintptr, w, h int32, now time.Time) {
	for i, spec := range terminusWreckSpecs {
		if !terminusWreckAlive(i, now) {
			continue
		}
		p, sz := terminusWreckPoint(i, w, h, now)
		a := &terminusSatWreck[spec.Asset%len(terminusSatWreck)]
		if a.w <= 0 || a.h <= 0 {
			continue
		}
		dstW := sz
		dstH := int32(float64(dstW) * float64(a.h) / float64(a.w))
		drawTerminusAsset(hdc, a, RECT{int32(p.X) - dstW/2, int32(p.Y) - dstH/2, int32(p.X) + dstW/2, int32(p.Y) + dstH/2})
		// Tiny inert shard marks communicate "cover / wreckage", not a clickable target.
		for k := 0; k < 3; k++ {
			ang := spec.Phase + float64(k)*2.094 + now.Sub(survivalBoss3StartedAt).Seconds()*.12
			r0 := float64(sz) * .48
			r1 := r0 + float64(sx(5, w))
			drawSurvivalLineWithPen(hdc, survivalPen(1, rgb(92, 105, 116)),
				int32(p.X+math.Cos(ang)*r0), int32(p.Y+math.Sin(ang)*r0),
				int32(p.X+math.Cos(ang)*r1), int32(p.Y+math.Sin(ang)*r1))
		}
	}
}

func terminusDrawMissileBursts(hdc uintptr, w, h int32, now time.Time) {
	out := terminusMissileBursts[:0]
	for _, b := range terminusMissileBursts {
		age := now.Sub(b.At).Seconds()
		if age < 0 || age > .46 {
			continue
		}
		out = append(out, b)
		p := 1 - age/.46
		r := sx(8, w) + int32(age*float64(sx(70, w)))
		drawSurvivalCircleWithPen(hdc, survivalPen(4, rgb(255, 234, 198)), int32(b.P.X), int32(b.P.Y), r)
		drawSurvivalCircleWithPen(hdc, survivalPen(3, rgb(255, 72, 38)), int32(b.P.X), int32(b.P.Y), r+sx(7, w))
		for i := 0; i < 8; i++ {
			a := float64(i)*math.Pi/4 + age*2.1
			l0 := float64(r) * .55
			l1 := l0 + float64(sx(18, w))*p
			drawSurvivalLineWithPen(hdc, survivalPen(2, rgb(255, 118, 52)),
				int32(b.P.X+math.Cos(a)*l0), int32(b.P.Y+math.Sin(a)*l0),
				int32(b.P.X+math.Cos(a)*l1), int32(b.P.Y+math.Sin(a)*l1))
		}
	}
	terminusMissileBursts = out
}

func terminusDrawMissiles(hdc uintptr, w, h int32, now time.Time) {
	for i := range terminusMissiles {
		m := &terminusMissiles[i]
		if len(m.Trail) > 1 {
			for j := 1; j < len(m.Trail); j++ {
				a, b := m.Trail[j-1], m.Trail[j]
				frac := float64(j) / float64(len(m.Trail))
				col := rgb(byte(100+frac*120), byte(18+frac*48), 12)
				drawSurvivalLineWithPen(hdc, survivalPen(1+int32(frac*2), col), int32(a.X), int32(a.Y), int32(b.X), int32(b.Y))
			}
		}
		ang := math.Atan2(m.V.Y, m.V.X) - math.Pi // source rocket faces left
		rw := sx(48, w)
		rh := int32(float64(rw) * float64(terminusRocket.h) / float64(terminusRocket.w))
		drawBoss2RotatedBGRA(hdc, terminusRocket.data, terminusRocket.w, terminusRocket.h, m.P, rw, rh, ang)
		pulse := sx(9, w) + int32((math.Sin(now.Sub(m.SpawnedAt).Seconds()*10)+1)*float64(sx(2, w)))
		drawSurvivalCircleWithPen(hdc, survivalPen(1, rgb(255, 68, 44)), int32(m.P.X), int32(m.P.Y), pulse)
	}
	terminusDrawMissileBursts(hdc, w, h, now)
}

func terminusDrawTurrets(hdc uintptr, w, h int32, now time.Time) {
	if survivalBoss3Stage != survivalBoss3Combat {
		return
	}
	for i := range terminusTurrets {
		t := &terminusTurrets[i]
		pivot := terminusTurretAnchor(i, w, h)
		baseW := sx(72, w)
		baseH := int32(float64(baseW) * float64(terminusCannonBase.h) / float64(terminusCannonBase.w))
		drawTerminusAsset(hdc, &terminusCannonBase, RECT{
			int32(pivot.X) - baseW/2, int32(pivot.Y) - baseH/2 + sy(10, h),
			int32(pivot.X) + baseW/2, int32(pivot.Y) + baseH/2 + sy(10, h),
		})

		barrel := &terminusCannonLong
		barrelW := sx(88, w)
		if t.Heavy {
			barrel = &terminusCannonHeavy
			barrelW = sx(96, w)
		}
		barrelH := int32(float64(barrelW) * float64(barrel.h) / float64(barrel.w))
		centre, muzzle, end := terminusTurretRay(i, t, w, h)
		drawBoss2RotatedBGRA(hdc, barrel.data, barrel.w, barrel.h, centre, barrelW, barrelH, t.Aim-math.Pi)

		if !t.Active || now.Before(t.WarnAt) || now.After(t.EndAt) {
			continue
		}
		if now.Before(t.FireAt) {
			// Long, premium telegraph: warm tracking beam, visible charge at the muzzle,
			// then a red hard-lock before the shot. The player should always understand
			// why a cannon hit them.
			warnFrac := terminusTimerFraction(now, t.WarnAt, t.FireAt)
			col := rgb(255, 155, 52)
			if !now.Before(t.LockAt) {
				col = rgb(255, 63, 43)
			}
			drawSurvivalLineWithPen(hdc, survivalPen(5, rgb(35, 8, 8)), int32(muzzle.X), int32(muzzle.Y), int32(end.X), int32(end.Y))
			drawSurvivalLineWithPen(hdc, survivalPen(2, col), int32(muzzle.X), int32(muzzle.Y), int32(end.X), int32(end.Y))
			tr := sx(16, w)
			pulse := int32((math.Sin(float64(now.UnixMilli())*.018) + 1) * float64(sx(3, w)))
			drawSurvivalCircleWithPen(hdc, survivalPen(3, col), int32(t.Target.X), int32(t.Target.Y), tr+pulse)
			drawSurvivalCircleWithPen(hdc, survivalPen(1, rgb(255, 245, 225)), int32(t.Target.X), int32(t.Target.Y), tr+sx(6, w)+pulse)
			terminusDrawTimerDots(hdc, w, t.Target, warnFrac, tr+sx(12, w), col)
			charge := sx(4, w) + int32(warnFrac*float64(sx(7, w)))
			terminusFillCircle(hdc, int32(muzzle.X), int32(muzzle.Y), charge, rgb(255, 225, 190), col, 2)
		} else {
			// Three-layer beam gives the cannon a proper final-boss punch.
			drawSurvivalLineWithPen(hdc, survivalPen(sx(16, w), rgb(74, 8, 12)), int32(muzzle.X), int32(muzzle.Y), int32(end.X), int32(end.Y))
			drawSurvivalLineWithPen(hdc, survivalPen(sx(9, w), rgb(255, 35, 26)), int32(muzzle.X), int32(muzzle.Y), int32(end.X), int32(end.Y))
			drawSurvivalLineWithPen(hdc, survivalPen(sx(3, w), rgb(255, 235, 216)), int32(muzzle.X), int32(muzzle.Y), int32(end.X), int32(end.Y))
			terminusFillCircle(hdc, int32(muzzle.X), int32(muzzle.Y), sx(8, w), rgb(255, 225, 190), rgb(255, 55, 35), 2)
		}
	}
}

func terminusRelayEndpoint(c *TerminusChallenge, idx int) bool {
	if idx <= 0 {
		return false
	}
	for i, mode := range c.RelayModes {
		if (mode == 1 || mode == 2) && i+1 == idx {
			return true
		}
	}
	return false
}

func terminusDrawFollowSatellite(hdc uintptr, w, h int32, p FPoint, satIndex int, accent uintptr) {
	sat := &terminusSat[satIndex%8]
	sw := sx(54, w)
	sh := int32(float64(sw) * float64(sat.h) / float64(sat.w))
	terminusFillCircle(hdc, int32(p.X), int32(p.Y), sx(29, w), rgb(5, 12, 20), accent, 4)
	terminusFillCircle(hdc, int32(p.X), int32(p.Y), sx(23, w), rgb(14, 49, 68), rgb(255, 255, 255), 3)
	drawTerminusAsset(hdc, sat, RECT{int32(p.X) - sw/2, int32(p.Y) - sh/2, int32(p.X) + sw/2, int32(p.Y) + sh/2})
	drawSurvivalCircleWithPen(hdc, survivalPen(3, rgb(255, 255, 255)), int32(p.X), int32(p.Y), sx(16, w))
}

func terminusDrawChallenge(hdc uintptr, w, h int32, now time.Time) {
	c := &survivalBoss3Challenge
	timerFrac := terminusTimerFraction(now, c.TargetSpawnAt, c.TargetHitAt)
	_, accent := terminusComboColours(c.Slot)

	switch c.Kind {
	case terminusMechChain, terminusMechStream, terminusMechJump:
		// Draw the route first so targets stay visually dominant, like osu hit circles.
		for i := c.Current; i+1 < len(c.Points); i++ {
			a, b := c.Points[i], c.Points[i+1]
			drawSurvivalLineWithPen(hdc, survivalPen(5, rgb(3, 9, 16)), int32(a.X), int32(a.Y), int32(b.X), int32(b.Y))
			drawSurvivalLineWithPen(hdc, survivalPen(2, rgb(86, 103, 118)), int32(a.X), int32(a.Y), int32(b.X), int32(b.Y))
			terminusDrawFollowDots(hdc, w, a, b, accent)
		}
		for i := c.Current; i < len(c.Points); i++ {
			active := i == c.Current
			terminusDrawTarget(hdc, w, h, c.Points[i], i+1, active, timerFrac, i, c.Kind == terminusMechStream)
			if active {
				terminusDrawTimerDots(hdc, w, c.Points[i], timerFrac, sx(40, w), accent)
			}
			if active && c.Kind == terminusMechChain {
				terminusDrawCountdown(hdc, w, h, c.Points[i], now, c.Deadline)
			}
		}

	case terminusMechSlider, terminusMechLaserSlider:
		terminusDrawPath(hdc, c.Path, w, true)
		start := c.Path[0]
		end := c.Path[len(c.Path)-1]
		if !c.Dragging {
			terminusDrawTarget(hdc, w, h, start, 1, true, timerFrac, c.Slot, false)
		} else {
			// Project the dragged satellite onto the actual rail. The player's small
			// hit-circle may wander slightly inside the corridor without making the
			// artwork jitter or look detached from the slider.
			ball := terminusPointAtPath(c.Path, c.PathProgress)
			terminusDrawFollowSatellite(hdc, w, h, ball, c.Slot+survivalBoss3LoopCount, accent)
		}
		terminusDrawTarget(hdc, w, h, end, 2, false, 0, c.Slot+3, false)
		if c.Dragging {
			elapsed := math.Min(1, now.Sub(c.StartedAt).Seconds()/math.Max(.001, c.Duration.Seconds()))
			terminusDrawTimerDots(hdc, w, end, elapsed, sx(42, w), accent)
			terminusDrawCountdown(hdc, w, h, end, now, c.Deadline)
		}
		for _, l := range c.Lasers {
			if now.Before(l.WarnAt) || now.After(l.EndAt) {
				continue
			}
			if now.Before(l.FireAt) {
				drawSurvivalLineWithPen(hdc, survivalPen(2, rgb(205, 57, 46)), int32(l.A.X), int32(l.A.Y), int32(l.B.X), int32(l.B.Y))
			} else {
				drawSurvivalLineWithPen(hdc, survivalPen(sx(12, w), rgb(89, 8, 12)), int32(l.A.X), int32(l.A.Y), int32(l.B.X), int32(l.B.Y))
				drawSurvivalLineWithPen(hdc, survivalPen(sx(6, w), rgb(255, 48, 34)), int32(l.A.X), int32(l.A.Y), int32(l.B.X), int32(l.B.Y))
				drawSurvivalLineWithPen(hdc, survivalPen(2, rgb(255, 235, 220)), int32(l.A.X), int32(l.A.Y), int32(l.B.X), int32(l.B.Y))
			}
		}

	case terminusMechHoldRail:
		terminusDrawPath(hdc, c.Path, w, true)
		if !c.Holding {
			terminusDrawTarget(hdc, w, h, c.Path[0], 1, true, timerFrac, c.Slot, false)
		} else {
			prog := math.Min(1, now.Sub(c.StartedAt).Seconds()/c.Duration.Seconds())
			c.PathProgress = prog
			target := terminusPointAtPath(c.Path, prog)
			terminusDrawFollowSatellite(hdc, w, h, target, survivalBoss3LoopCount+c.Slot, accent)
		}

	case terminusMechSpinner:
		centre := c.SpinnerCentre
		radius := sx(58, w)
		terminusFillCircle(hdc, int32(centre.X), int32(centre.Y), radius, rgb(10, 28, 39), rgb(238, 244, 248), 3)
		drawSurvivalCircleWithPen(hdc, survivalPen(5, accent), int32(centre.X), int32(centre.Y), radius-sx(7, w))
		for i := 0; i < 12; i++ {
			a := float64(i)/12*2*math.Pi + float64(now.UnixMilli())*.0032*c.SpinnerDir
			p := FPoint{X: centre.X + math.Cos(a)*float64(radius), Y: centre.Y + math.Sin(a)*float64(radius)}
			terminusFillCircle(hdc, int32(p.X), int32(p.Y), sx(4, w), rgb(247, 249, 252), accent, 1)
		}
		if !c.Holding {
			terminusDrawTarget(hdc, w, h, centre, 1, true, timerFrac, c.Slot, false)
		} else {
			fill := math.Min(1, c.SpinnerAccum/math.Max(.001, c.SpinnerNeed))
			arcR := radius + sx(13, w) + int32(fill*float64(sx(8, w)))
			drawSurvivalCircleWithPen(hdc, survivalPen(5, accent), int32(centre.X), int32(centre.Y), arcR)
			terminusDrawTimerDots(hdc, w, centre, fill, radius+sx(22, w), accent)
			terminusDrawCountdown(hdc, w, h, centre, now, c.Deadline)
		}

	case terminusMechRelay:
		// Relay is a readable sequence of actual actions. Drag/hold endpoints are
		// destinations, not fake numbered circles that disappear without a click.
		for i := c.Current; i < len(c.Points)-1; {
			mode := 0
			if i < len(c.RelayModes) {
				mode = c.RelayModes[i]
			}
			if (mode == 1 || mode == 2) && i+1 < len(c.Points) {
				segment := terminusBuildSmoothPath(c.Points[i], c.Points[i+1], i+c.Slot, c.Points[i].Y-float64(sy(50, h)), c.Points[i].Y+float64(sy(50, h)), 18)
				if i == c.Current {
					terminusDrawPath(hdc, segment, w, true)
					end := c.Points[i+1]
					terminusDrawTarget(hdc, w, h, end, 0, false, 0, i+1, false)
					if mode == 1 && c.Dragging {
						ball := terminusPointAtPath(segment, c.PathProgress)
						terminusDrawFollowSatellite(hdc, w, h, ball, i+c.Slot, accent)
						elapsed := terminusTimerFraction(now, c.StartedAt, c.Deadline)
						terminusDrawTimerDots(hdc, w, end, elapsed, sx(42, w), accent)
					}
					if mode == 2 && c.Holding {
						prog := math.Min(1, now.Sub(c.StartedAt).Seconds()/.88)
						ball := terminusPointAtPath(segment, prog)
						terminusDrawFollowSatellite(hdc, w, h, ball, i+c.Slot, accent)
						terminusDrawTimerDots(hdc, w, end, prog, sx(42, w), accent)
					}
				} else {
					terminusDrawPolyline(hdc, segment, survivalPen(5, rgb(46, 60, 72)))
				}
				i += 2
				continue
			}
			j := i + 1
			for j < len(c.Points) && terminusRelayEndpoint(c, j) {
				j++
			}
			if j < len(c.Points) {
				drawSurvivalLineWithPen(hdc, survivalPen(2, rgb(20, 31, 41)), int32(c.Points[i].X), int32(c.Points[i].Y), int32(c.Points[j].X), int32(c.Points[j].Y))
				terminusDrawFollowDots(hdc, w, c.Points[i], c.Points[j], accent)
			}
			i++
		}

		for i := c.Current; i < len(c.Points); i++ {
			if terminusRelayEndpoint(c, i) {
				continue
			}
			active := i == c.Current && !c.Dragging && !c.Holding
			terminusDrawTarget(hdc, w, h, c.Points[i], i+1, active, timerFrac, i, false)
			if active {
				terminusDrawTimerDots(hdc, w, c.Points[i], timerFrac, sx(40, w), accent)
			}
			if active && i < len(c.RelayModes) && c.RelayModes[i] == 0 {
				terminusDrawCountdown(hdc, w, h, c.Points[i], now, c.Deadline)
			}
		}
	}
}

func terminusDrawScrollingBackground(hdc uintptr, w, h int32, now time.Time) {
	r := arenaRect(w, h)
	if len(terminusBG.data) < int(terminusBG.w*terminusBG.h*4) ||
		!ensureCachedBGRASprite(hdc, &terminusBG.sprite, terminusBG.data, terminusBG.w, terminusBG.h) {
		fillSolidRect(hdc, r, rgb(2, 5, 18))
		return
	}
	// Preserve the artwork's aspect ratio exactly. The v397 cover-fit path stretched/cropped
	// individual fleet ships as each scrolling tile was mapped to the arena rectangle.
	tileH := r.Bottom - r.Top
	tileW := int32(math.Round(float64(tileH) * float64(terminusBG.w) / float64(terminusBG.h)))
	if tileW < 1 {
		tileW = 1
	}
	off := int32(0)
	if backgroundMotionEnabled() {
		elapsed := now.Sub(survivalBoss3StartedAt).Seconds()
		if elapsed < 0 {
			elapsed = 0
		}
		off = int32(math.Mod(elapsed*22.0, float64(tileW)))
	}
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	for x := r.Left - off; x < r.Right; x += tileW {
		alphaBlend.Call(hdc, uintptr(x), uintptr(r.Top), uintptr(tileW), uintptr(tileH),
			terminusBG.sprite.DC, 0, 0, uintptr(terminusBG.w), uintptr(terminusBG.h), blend)
	}
}

func drawSurvivalBoss3Arena(hdc uintptr, w, h int32) {
	r := arenaRect(w, h)
	terminusEnsureAssets()
	now := survivalBossFightNow(time.Now(), survivalBoss3StartedAt)
	terminusDrawScrollingBackground(hdc, w, h, now)
	// A restrained foreground star streak layer adds parallax and motion without
	// obscuring hit circles or changing the accepted fleet artwork.
	drawSurvivalParticles(hdc, w, h)
	terminusDrawMeteors(hdc, w, h, now)
	terminusDrawWrecks(hdc, w, h, now)

	shipR := terminusShipRect(w, h)
	shipAsset := &terminusShip
	if survivalBoss3Stage == survivalBoss3Vulnerable {
		pulse := int(now.UnixMilli()/115) % 2
		if pulse == 0 || now.Before(survivalBoss3ShipHitFlashUntil) {
			shipAsset = &terminusShipRed
		}
	}
	drawTerminusAsset(hdc, shipAsset, shipR)

	if survivalBoss3Stage == survivalBoss3Combat {
		terminusDrawChallenge(hdc, w, h, now)
		terminusDrawMissiles(hdc, w, h, now)
		terminusDrawTurrets(hdc, w, h, now)
		terminusDrawHitBursts(hdc, w, h, now)
	}

	if survivalBoss3Stage == survivalBoss3Vulnerable {
		left := shipR.Left
		for i := 0; i < 5; i++ {
			x := left - sx(float64(12+i*12), w)
			drawSurvivalLineWithPen(hdc, survivalPen(2, rgb(255, 68, 45)), x, shipR.Top+sy(18, h), x, shipR.Bottom-sy(18, h))
		}
	}

	if survivalBoss3Stage == survivalBoss3Dying {
		age := now.Sub(survivalBoss3StageAt).Seconds()
		if age < 2.2 {
			overlaySolidAlphaRect(hdc, r, rgb(255, 55, 30), byte(90*(1-age/2.2)))
		}
	}

	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(245, 245, 245))
		label := "BOSS 3 // THE TERMINUS"
		if survivalBoss3Stage == survivalBoss3Combat {
			label = fmt.Sprintf("%s // MAP %d-%d", terminusMechanicName(survivalBoss3Challenge.Kind), survivalBoss3LoopCount+1, survivalBoss3Slot+1)
		} else if survivalBoss3Stage == survivalBoss3Vulnerable {
			remain := math.Max(0, survivalBoss3VulnerableUntil.Sub(now).Seconds())
			label = fmt.Sprintf("VULNERABLE // ATTACK // %.1fs", remain)
		}
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(20, h), label)
		selectObject.Call(hdc, old)
	}
}
