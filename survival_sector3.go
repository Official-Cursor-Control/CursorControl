//go:build windows

package main

import (
	"math"
	"math/rand"
	"time"
)

const (
	survivalEliteNone = iota
	survivalEliteArmoured
	survivalEliteStriker
)

type SurvivalFieldMeteor struct {
	X, Y    float64
	Speed   float64
	Variant int
}

var (
	survivalFieldMeteors    []SurvivalFieldMeteor
	survivalNextFieldMeteor time.Time
)

func survivalRollEliteType(wave, waveType int) int {
	if wave < 21 {
		return survivalEliteNone
	}
	chance := 0.08 + float64(wave-21)*0.012
	if chance > 0.28 {
		chance = 0.28
	}
	if waveType == survivalWaveSwarm {
		chance *= 0.55
	}
	if rand.Float64() >= chance {
		return survivalEliteNone
	}
	if rand.Intn(2) == 0 {
		return survivalEliteArmoured
	}
	return survivalEliteStriker
}

func resetSurvivalSector3FieldEvent() {
	survivalFieldMeteors = nil
	survivalNextFieldMeteor = time.Time{}
	if survivalWaveType == survivalWaveMeteorDrift {
		survivalNextFieldMeteor = time.Now().Add(900 * time.Millisecond)
	}
}

func spawnSurvivalFieldMeteor(now time.Time, w, h int32) {
	r := arenaRect(w, h)
	margin := float64(sx(52, w))
	span := float64(r.Right-r.Left) - margin*2
	if span <= 1 {
		return
	}
	x := float64(r.Left) + margin + rand.Float64()*span
	// Avoid stacking multiple rocks in a single narrow lane.
	for tries := 0; tries < 8; tries++ {
		ok := true
		for _, m := range survivalFieldMeteors {
			if math.Abs(m.X-x) < float64(sx(85, w)) && m.Y < float64(r.Top+sy(180, h)) {
				ok = false
				break
			}
		}
		if ok {
			break
		}
		x = float64(r.Left) + margin + rand.Float64()*span
	}
	survivalFieldMeteors = append(survivalFieldMeteors, SurvivalFieldMeteor{
		X: x, Y: float64(r.Top - sy(60, h)), Speed: float64(sy(float64(165+rand.Intn(55)), h)), Variant: rand.Intn(4),
	})
	survivalNextFieldMeteor = now.Add(time.Duration(1150+rand.Intn(600)) * time.Millisecond)
}

func updateSurvivalSector3FieldEvent(h uintptr, now time.Time, w, hgt int32, dt float64) bool {
	if survivalWaveType != survivalWaveMeteorDrift {
		return true
	}
	if survivalNextFieldMeteor.IsZero() || !now.Before(survivalNextFieldMeteor) {
		spawnSurvivalFieldMeteor(now, w, hgt)
	}
	r := arenaRect(w, hgt)
	out := survivalFieldMeteors[:0]
	radius := float64(sx(34, w))
	for _, m := range survivalFieldMeteors {
		m.Y += m.Speed * dt
		if cursorInArena && math.Hypot(cursorPos.X-m.X, cursorPos.Y-m.Y) <= radius {
			survivalFail(h, "Hit by meteor drift")
			return false
		}
		if m.Y-radius <= float64(r.Bottom) {
			out = append(out, m)
		}
	}
	survivalFieldMeteors = out
	return true
}

func drawSurvivalSector3FieldEvent(hdc uintptr, w, h int32, now time.Time) {
	if survivalWaveType != survivalWaveMeteorDrift || len(survivalFieldMeteors) == 0 {
		return
	}
	for _, m := range survivalFieldMeteors {
		sz := sx(42, w)
		data := survivalBoss2Boulder1
		switch m.Variant {
		case 1:
			data = survivalBoss2Boulder2
		case 2:
			data = survivalBoss2Boulder3
		case 3:
			data = survivalBoss2Boulder4
		}
		if len(data) >= 128*128*4 {
			drawRawBGRAFit(hdc, data, 128, 128, RECT{int32(m.X) - sz, int32(m.Y) - sz, int32(m.X) + sz, int32(m.Y) + sz})
		} else {
			drawSurvivalCircleWithPen(hdc, survivalPen(4, rgb(150, 157, 173)), int32(m.X), int32(m.Y), sz)
		}
		// short directional streak makes vertical motion readable without fire.
		drawSurvivalLineWithPen(hdc, survivalPen(2, rgb(122, 84, 165)), int32(m.X), int32(m.Y)-sz-sy(15, h), int32(m.X), int32(m.Y)-sz-sy(3, h))
	}
	_ = now
}
