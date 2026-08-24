//go:build windows

package main

import (
	"fmt"
	"sync"
	"time"
)

var (
	afkOperatorLevelFillMu       sync.RWMutex
	afkOperatorLevelFillIndex    = -1
	afkOperatorLevelFillStarted  time.Time
	afkOperatorLevelFillDuration = 3400 * time.Millisecond

	afkOperatorCelebrationOnce sync.Once
	afkOperatorCelebrationQ    = make(chan afkOperatorCelebration, 32)
)

type afkOperatorCelebration struct {
	OperatorIndex int
	SoundName     string
	Hold          time.Duration
}

// A single worker owns every Operator celebration. This is stronger than
// spawning one goroutine per quip: Nova and Vega can become ready on the same
// update, but their voice lines/card fills can never start together.
func afkEnsureOperatorCelebrationWorker() {
	afkOperatorCelebrationOnce.Do(func() {
		go func() {
			for ev := range afkOperatorCelebrationQ {
				if ev.OperatorIndex < 0 || ev.OperatorIndex >= len(afkOperatorDefs) {
					continue
				}
				hold := ev.Hold
				if hold < afkOperatorLevelFillDuration {
					hold = afkOperatorLevelFillDuration
				}
				// Duck music before starting the quip so there is no loud first syllable.
				if ev.SoundName != "" && audioReady && gameMeta.EffectsVolume > 0 {
					requestOperatorVoiceDuck(hold)
				}
				afkStartOperatorLevelFill(ev.OperatorIndex)
				if ev.SoundName != "" && audioReady && gameMeta.EffectsVolume > 0 {
					playSFX(ev.SoundName)
				}
				time.Sleep(hold)
			}
		}()
	})
}

func afkQueueOperatorCelebration(ev afkOperatorCelebration) {
	afkEnsureOperatorCelebrationWorker()
	select {
	case afkOperatorCelebrationQ <- ev:
	default:
		// Extremely rapid developer/test promotions should still preserve ordering
		// rather than dropping a quip when the small UI queue is momentarily full.
		go func() { afkOperatorCelebrationQ <- ev }()
	}
}

func afkStartOperatorLevelFill(operatorIndex int) {
	if operatorIndex < 0 || operatorIndex >= len(afkOperatorDefs) {
		return
	}
	afkOperatorLevelFillMu.Lock()
	afkOperatorLevelFillIndex = operatorIndex
	afkOperatorLevelFillStarted = time.Now()
	afkOperatorLevelFillMu.Unlock()
}

func afkOperatorLevelFillState(operatorIndex int, now time.Time) (rise float64, alpha byte, active bool) {
	afkOperatorLevelFillMu.RLock()
	idx := afkOperatorLevelFillIndex
	started := afkOperatorLevelFillStarted
	duration := afkOperatorLevelFillDuration
	afkOperatorLevelFillMu.RUnlock()
	if operatorIndex != idx || started.IsZero() || duration <= 0 {
		return 0, 0, false
	}
	age := now.Sub(started)
	if age < 0 || age >= duration {
		return 0, 0, false
	}
	riseDuration := 2700 * time.Millisecond
	if age < riseDuration {
		rise = float64(age) / float64(riseDuration)
	} else {
		rise = 1
	}
	if rise < 0 {
		rise = 0
	}
	if rise > 1 {
		rise = 1
	}
	alpha = 62
	fadeStart := 2950 * time.Millisecond
	if age > fadeStart {
		left := duration - age
		fadeWindow := duration - fadeStart
		if left <= 0 {
			return 0, 0, false
		}
		alpha = byte(float64(alpha) * float64(left) / float64(fadeWindow))
	}
	return rise, alpha, true
}

func drawAFKOperatorLevelFill(hdc uintptr, card RECT, operatorIndex int, accent uintptr, w, hgt int32) {
	rise, alpha, active := afkOperatorLevelFillState(operatorIndex, time.Now())
	if !active || rise <= 0 || alpha == 0 {
		return
	}
	inner := RECT{card.Left + sx(3, w), card.Top + sy(3, hgt), card.Right - sx(3, w), card.Bottom - sy(3, hgt)}
	if inner.Right <= inner.Left || inner.Bottom <= inner.Top {
		return
	}
	h := inner.Bottom - inner.Top
	fillH := int32(float64(h) * rise)
	if fillH < 1 {
		fillH = 1
	}
	fillTop := inner.Bottom - fillH
	alphaSolidRect(hdc, RECT{inner.Left, fillTop, inner.Right, inner.Bottom}, accent, alpha)

	crestH := sy(4, hgt)
	if crestH < 2 {
		crestH = 2
	}
	crestTop := fillTop - crestH/2
	if crestTop < inner.Top {
		crestTop = inner.Top
	}
	crestBottom := crestTop + crestH
	if crestBottom > inner.Bottom {
		crestBottom = inner.Bottom
	}
	if crestBottom > crestTop {
		alphaSolidRect(hdc, RECT{inner.Left, crestTop, inner.Right, crestBottom}, accent, 150)
		alphaSolidRect(hdc, RECT{inner.Left + sx(8, w), crestTop, inner.Right - sx(8, w), min32(crestBottom, crestTop+1)}, rgb(255, 255, 255), 115)
	}
}

func afkQueueOperatorRecruitCelebration(operatorIndex int) {
	if operatorIndex < 0 || operatorIndex >= len(afkOperatorDefs) {
		return
	}
	afkQueueOperatorCelebration(afkOperatorCelebration{
		OperatorIndex: operatorIndex,
		SoundName:     "afk_operator_recruited",
		Hold:          3800 * time.Millisecond,
	})
}

// Genuine level-up quips only enter this queue after the player physically
// promotes an Operator. XP reaching a threshold by itself never plays dialogue.
func afkQueueOperatorLevelQuip(operatorIndex, level int) {
	if operatorIndex < 0 || operatorIndex >= len(afkOperatorDefs) || level <= 1 {
		return
	}
	variant := 1 + int((time.Now().UnixNano()/int64(time.Millisecond)+int64(operatorIndex*7+level*3))%3)
	name := fmt.Sprintf("afk_operator_%s_quip_%d", afkOperatorSpriteNames[operatorIndex], variant)
	afkQueueOperatorCelebration(afkOperatorCelebration{
		OperatorIndex: operatorIndex,
		SoundName:     name,
		// The fill itself remains ~3.4s; the slightly longer queue slot guarantees
		// a previous spoken line is completely clear before the next one starts.
		Hold: 4200 * time.Millisecond,
	})
}
