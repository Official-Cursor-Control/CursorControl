package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

// v205 production hardening -------------------------------------------------

var allMCIAliases = []string{
	"boss",
	"endurance_music",
	"survival_music",
	"survival_section2",
	"survival_section3",
	"survival_boss1_music",
	"survival_boss2_music",
	"survival_boss3_music",
	"starbase_music",
}

var transientGameplayAliases = []string{}

func stopTransientGameplayAudio() {
	stopAllSFXLoops()
	shieldProtectSoundPlaying = false
}

func closeAllMCIAliases() {
	for _, alias := range allMCIAliases {
		mci("stop " + alias)
		mci("close " + alias)
	}
}

func logRuntimeEvent(kind, detail string) {
	if logRoot == "" {
		return
	}
	_ = os.MkdirAll(logRoot, 0755)
	detail = strings.ReplaceAll(detail, "\r", " ")
	detail = strings.ReplaceAll(detail, "\n", " ")
	line := fmt.Sprintf("%s kind=%s detail=%s\n", time.Now().Format(time.RFC3339Nano), kind, detail)
	f, err := os.OpenFile(filepath.Join(logRoot, "runtime_events.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

func clearEnduranceTransientObjects() {
	enduranceBlocks = nil
	enduranceAlienMinions = nil
	endurancePowerups = nil
	enduranceTargetExplosions = nil
	targets = nil
	polishEffects = nil

	enduranceBlockSpawnTime = time.Time{}
	enduranceAlienMinionSpawnTime = time.Time{}
	enduranceShieldUntil = time.Time{}
	enduranceSlowUntil = time.Time{}
	enduranceStoredShields = 0
	enduranceStoredTime = 0

	enduranceWarpCueActive = false
	enduranceWarpActive = false
	enduranceWarpCueStarted = time.Time{}
	enduranceWarpStartDistance = 0
	enduranceWarpCheckpoint = 0
	enduranceWarpRecoveryUntil = 0
	enduranceWarpTargetsSpawned = false
	enduranceWarpAmbientReturnAt = time.Time{}
	enduranceWarpAmbientReturnFrom = 1

	enduranceAlienBossState = alienBossIdle
	enduranceAlienBossStateStarted = time.Time{}
	enduranceAlienBossX = 0
	enduranceAlienBossY = 0
	enduranceAlienBossLockedX = 0
	enduranceAlienBossLockedY = 0
	enduranceAlienPhaseTriggered = false
	endurancePostEncounterRecoveryUntil = 0

	stopTransientGameplayAudio()
}

func normalizeShipList(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || id >= len(spaceShipDefs) || strings.TrimSpace(spaceShipDefs[id].Name) == "" {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

func mergeShipCollections(local, remote []int) []int {
	merged := append([]int(nil), local...)
	merged = append(merged, remote...)
	return normalizeShipList(merged)
}

func mergeCosmeticIDCollections(local, remote []int, maxID int) []int {
	seen := make(map[int]bool, maxID+1)
	out := make([]int, 0, len(local)+len(remote))
	for _, src := range [][]int{local, remote} {
		for _, id := range src {
			if id <= 0 || id > maxID || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Ints(out)
	return out
}

func runtimeStateSanityCheck() {
	// Never let corrupted/transient numeric state propagate into geometry,
	// collision or renderer calculations.
	if math.IsNaN(enduranceDistance) || math.IsInf(enduranceDistance, 0) || enduranceDistance < 0 {
		logRuntimeEvent("sanity_repair", "invalid enduranceDistance")
		enduranceDistance = 0
	}
	if math.IsNaN(enduranceCameraX) || math.IsInf(enduranceCameraX, 0) || enduranceCameraX < 0 {
		logRuntimeEvent("sanity_repair", "invalid enduranceCameraX")
		enduranceCameraX = 0
	}
	if enduranceStoredShields < 0 || enduranceStoredShields > 2 {
		logRuntimeEvent("sanity_repair", "shield storage out of range")
		if enduranceStoredShields < 0 {
			enduranceStoredShields = 0
		} else {
			enduranceStoredShields = 2
		}
	}
	if enduranceStoredTime < 0 || enduranceStoredTime > 2 {
		logRuntimeEvent("sanity_repair", "time storage out of range")
		if enduranceStoredTime < 0 {
			enduranceStoredTime = 0
		} else {
			enduranceStoredTime = 2
		}
	}
	if enduranceAlienBossState < alienBossIdle || enduranceAlienBossState > alienBossDone {
		logRuntimeEvent("sanity_repair", "alien boss state out of range")
		enduranceAlienBossState = alienBossDone
		enduranceAlienBossStateStarted = time.Now()
	}
	if len(enduranceBlocks) > 16 {
		logRuntimeEvent("sanity_repair", "meteor list exceeded emergency ceiling")
		enduranceBlocks = enduranceBlocks[len(enduranceBlocks)-8:]
	}
	if len(enduranceAlienMinions) > 12 {
		logRuntimeEvent("sanity_repair", "alien list exceeded emergency ceiling")
		enduranceAlienMinions = enduranceAlienMinions[len(enduranceAlienMinions)-6:]
	}
	if len(targets) > 24 {
		logRuntimeEvent("sanity_repair", "target list exceeded emergency ceiling")
		targets = targets[len(targets)-12:]
	}
	if len(endurancePowerups) > 4 {
		logRuntimeEvent("sanity_repair", "powerup list exceeded emergency ceiling")
		endurancePowerups = endurancePowerups[len(endurancePowerups)-1:]
	}

	// Warp cue and active boost are mutually exclusive.
	if enduranceWarpCueActive && enduranceWarpActive {
		logRuntimeEvent("sanity_repair", "warp cue and active simultaneously")
		enduranceWarpCueActive = false
	}
	// Path containment is enforced at chunk generation and by a low-frequency
	// safety check outside the 125 Hz simulation hot path.
}

func runtimeLifecycleSnapshot(label string) {
	logRuntimeEvent("lifecycle",
		fmt.Sprintf("%s state=%d overlay=%d endurance=%t blocks=%d aliens=%d targets=%d powerups=%d fx=%d d2d=%t",
			label, state, overlayMode, enduranceActive(), len(enduranceBlocks), len(enduranceAlienMinions),
			len(targets), len(endurancePowerups), len(polishEffects), d2dReady))
}

func runtimeDiagnosticText() string {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return fmt.Sprintf(
		"DIAG state=%d overlay=%d mode=%d d2d=%t goroutines=%d alloc_mb=%.1f sys_mb=%.1f blocks=%d aliens=%d targets=%d powerups=%d fx=%d boss=%d warpCue=%t warp=%t",
		state, overlayMode, gameMode, d2dReady, runtime.NumGoroutine(),
		float64(ms.Alloc)/(1024*1024), float64(ms.Sys)/(1024*1024),
		len(enduranceBlocks), len(enduranceAlienMinions), len(targets), len(endurancePowerups), len(polishEffects),
		enduranceAlienBossState, enduranceWarpCueActive, enduranceWarpActive,
	)
}

func logRuntimeDiagnostic(label string) {
	logRuntimeEvent("diagnostic", label+" "+runtimeDiagnosticText())
}

func recordRecoveredPanic(scope string, recovered any) {
	if recovered == nil {
		return
	}
	logRuntimeEvent("panic_recovered", fmt.Sprintf("scope=%s panic=%v stack=%s", scope, recovered, string(debug.Stack())))
}
