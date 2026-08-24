//go:build windows

package main

import "time"

// AFK input hardening. Rewards are state/timer driven; UI input may only request
// a transition. Rapid duplicate requests are ignored before they reach economy
// code. This prevents mouse macros/autoclickers from multiplying claims or buys.
const (
	afkManualClickMinInterval = 350 * time.Millisecond // <= 2.86 accepted clicks/s
	afkEconomyActionDebounce  = 180 * time.Millisecond
	afkClaimActionDebounce    = 500 * time.Millisecond
)

var afkLastActionAt = map[string]time.Time{}
var afkManualLastAccepted time.Time
var afkRejectedRapidInputs int64

func afkActionAllowed(key string, now time.Time, minGap time.Duration) bool {
	if now.IsZero() {
		now = time.Now()
	}
	if prev, ok := afkLastActionAt[key]; ok && now.Sub(prev) < minGap {
		afkRejectedRapidInputs++
		return false
	}
	afkLastActionAt[key] = now
	return true
}

func afkManualClickAllowed(now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	if !afkManualLastAccepted.IsZero() && now.Sub(afkManualLastAccepted) < afkManualClickMinInterval {
		afkRejectedRapidInputs++
		return false
	}
	afkManualLastAccepted = now
	return true
}

// Economy mutations use this helper after validating state/cost. The current
// balance is always re-read at execution time so queued rapid clicks cannot buy
// multiple items using a stale displayed balance.
func afkSpendStarbits(cost int64) bool {
	if cost < 0 || gameMeta.AFKStarbits < cost {
		return false
	}
	gameMeta.AFKStarbits -= cost
	afkBankRecordSpend(cost)
	return true
}
