package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type selfTestResult struct {
	Name   string
	OK     bool
	Detail string
}

func runRuntimeSelfTest() string {
	results := make([]selfTestResult, 0, 16)
	add := func(name string, ok bool, detail string) {
		results = append(results, selfTestResult{Name: name, OK: ok, Detail: detail})
	}

	if err := validateRuntimeAssets(); err != nil {
		add("critical_assets", false, err.Error())
	} else {
		add("critical_assets", true, "required assets valid")
	}

	add("game_root", gameRoot != "", gameRoot)
	add("asset_root", assetRoot != "" && textureRoot != "", textureRoot)
	add("save_paths", localProgressPath() != "" && localMetaPath() != "", "local persistence paths resolved")

	// Verify writable data/log folders without touching progression.
	writeProbe := func(dir, name string) bool {
		if dir == "" {
			return false
		}
		_ = os.MkdirAll(dir, 0755)
		p := filepath.Join(dir, name)
		if err := atomicWriteFile(p, []byte("cursor-control-self-test\n"), 0644); err != nil {
			return false
		}
		_ = os.Remove(p)
		_ = os.Remove(p + ".bak")
		return true
	}
	add("data_writable", writeProbe(dataRoot, ".selftest_data.tmp"), dataRoot)
	add("logs_writable", writeProbe(logRoot, ".selftest_log.tmp"), logRoot)

	normalized := normalizeShipList(gameMeta.UnlockedShips)
	add("ship_collection", len(normalized) == len(gameMeta.UnlockedShips), fmt.Sprintf("local=%d normalized=%d", len(gameMeta.UnlockedShips), len(normalized)))
	add("selected_ship", gameMeta.SelectedShip == 0 || shipUnlocked(gameMeta.SelectedShip), fmt.Sprintf("selected=%d", gameMeta.SelectedShip))
	add("powerup_storage", enduranceStoredShields >= 0 && enduranceStoredShields <= 2 && enduranceStoredTime >= 0 && enduranceStoredTime <= 2,
		fmt.Sprintf("shield=%d time=%d", enduranceStoredShields, enduranceStoredTime))
	add("warp_state", !(enduranceWarpCueActive && enduranceWarpActive), fmt.Sprintf("cue=%t active=%t", enduranceWarpCueActive, enduranceWarpActive))
	add("object_caps", len(enduranceBlocks) <= 16 && len(enduranceAlienMinions) <= 12 && len(targets) <= 24 && len(endurancePowerups) <= 4,
		fmt.Sprintf("meteors=%d aliens=%d targets=%d powerups=%d", len(enduranceBlocks), len(enduranceAlienMinions), len(targets), len(endurancePowerups)))
	add("audio_music_bus", len(allMCIAliases) == 7, fmt.Sprintf("music_aliases=%d", len(allMCIAliases)))
	add("audio_sfx_bus", len(sfxBus.effects) >= 27, fmt.Sprintf("pcm_effects=%d", len(sfxBus.effects)))
	add("renderer", d2dChildHwnd == 0 || d2dReady || !enduranceActive() || state != StatePlaying,
		fmt.Sprintf("child=%t d2d=%t state=%d", d2dChildHwnd != 0, d2dReady, state))
	add("runtime", true, runtimeDiagnosticText())

	passed := 0
	var b strings.Builder
	b.WriteString("Cursor Control " + clientBuildVersion + " Runtime Self-Test\n")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n\n")
	for _, r := range results {
		status := "PASS"
		if !r.OK {
			status = "FAIL"
		} else {
			passed++
		}
		fmt.Fprintf(&b, "[%s] %s — %s\n", status, r.Name, r.Detail)
	}
	fmt.Fprintf(&b, "\nRESULT %d/%d PASS\n", passed, len(results))
	report := b.String()
	if logRoot != "" {
		_ = atomicWriteFile(filepath.Join(logRoot, "selftest_latest.txt"), []byte(report), 0644)
	}
	logRuntimeEvent("self_test", fmt.Sprintf("%d/%d pass", passed, len(results)))
	return fmt.Sprintf("SELF TEST %d/%d PASS — logs/selftest_latest.txt", passed, len(results))
}
