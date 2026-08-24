package main

// v300 encounter consolidation. Boss encounters repeat as long-run milestones,
// with conservative difficulty scaling and a guaranteed recovery window.
var (
	enduranceAlienEncounterIndex int
	enduranceAlienNextBossAt     float64
)

func resetV300EncounterDirector() {
	enduranceAlienEncounterIndex = 0
	enduranceAlienNextBossAt = enduranceAlienPhaseStartMeters
}

func v300AlienBossReady(distance float64) bool {
	if enduranceAlienNextBossAt <= 0 {
		enduranceAlienNextBossAt = enduranceAlienPhaseStartMeters
	}
	return !enduranceAlienPhaseTriggered && !enduranceAlienBossEventActive() && distance >= enduranceAlienNextBossAt
}

func scheduleNextV300AlienBoss(distance float64) {
	enduranceAlienEncounterIndex++
	// Keep encounters meaningful but never stack them on consecutive warp corridors.
	gap := 2050.0 + float64((enduranceAlienEncounterIndex*173)%451) // 2050-2500m
	enduranceAlienNextBossAt = distance + gap
	enduranceAlienPhaseTriggered = false
}

func alienBossDifficultyTier() int {
	if enduranceAlienEncounterIndex < 0 {
		return 0
	}
	if enduranceAlienEncounterIndex > 5 {
		return 5
	}
	return enduranceAlienEncounterIndex
}

func alienBossAimDuration() float64 {
	// Telegraphs remain readable even deep into a run.
	v := enduranceAlienAimSeconds - float64(alienBossDifficultyTier())*0.08
	if v < 1.15 {
		v = 1.15
	}
	return v
}

func alienBossHoldDuration() float64 {
	v := enduranceAlienLaserHoldSecs + float64(alienBossDifficultyTier())*0.045
	if v > 1.22 {
		v = 1.22
	}
	return v
}

func alienBossEntryDuration() float64 {
	v := enduranceAlienBossEntrySecs - float64(alienBossDifficultyTier())*0.045
	if v < 1.08 {
		v = 1.08
	}
	return v
}
