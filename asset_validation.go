package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type requiredAsset struct {
	path    string
	minSize int64
}

func validateRuntimeAssets() error {
	required := []requiredAsset{
		{filepath.Join(assetRoot, "ui", "ui_base.bgra"), 6291456},
		{filepath.Join(assetRoot, "ui", "cursor_control_logo_hud.bgra"), 800000},
		{filepath.Join(assetRoot, "backgrounds", "arena_scrolling_bg.bgra"), 1900000},
		{filepath.Join(textureRoot, "endurance_background.bgra"), 6200000},
		{filepath.Join(textureRoot, "hazard_blue.bgra"), 40960},
		{filepath.Join(textureRoot, "hazard_orange.bgra"), 40960},
		{filepath.Join(textureRoot, "rocket_cursor.bgra"), 2816},
		{filepath.Join(textureRoot, "alien_minion.bgra"), 24576},
		{filepath.Join(textureRoot, "alien_boss.bgra"), 156000},
		{filepath.Join(textureRoot, "powerup_distance.bgra"), 16384},
		{filepath.Join(textureRoot, "powerup_shield.bgra"), 16384},
		{filepath.Join(textureRoot, "powerup_slow.bgra"), 16384},
		{filepath.Join(assetRoot, "audio", "endurance_theme.mp3"), 1000},
		{filepath.Join(assetRoot, "audio", "precision_theme.mp3"), 1000},
		{filepath.Join(assetRoot, "audio", "starbase_theme.mp3"), 1000},
		{filepath.Join(assetRoot, "audio", "survival_boss_intro.mp3"), 50000},
		{filepath.Join(assetRoot, "survival", "boss_intro", "sentinel_sharp.bgra"), 3600000},
		{filepath.Join(assetRoot, "survival", "boss_intro", "sentinel_blur.bgra"), 3600000},
		{filepath.Join(assetRoot, "survival", "boss_intro", "void_serpent_sharp.bgra"), 3600000},
		{filepath.Join(assetRoot, "survival", "boss_intro", "void_serpent_blur.bgra"), 3600000},
		{filepath.Join(assetRoot, "survival", "boss_intro", "terminus_sharp.bgra"), 3600000},
		{filepath.Join(assetRoot, "survival", "boss_intro", "terminus_blur.bgra"), 3600000},
	}
	for _, req := range required {
		st, err := os.Stat(req.path)
		if err != nil {
			return fmt.Errorf("required asset missing: %s: %w", req.path, err)
		}
		if st.IsDir() || st.Size() < req.minSize {
			return fmt.Errorf("required asset invalid: %s (size=%d, expected >= %d)", req.path, st.Size(), req.minSize)
		}
	}
	return nil
}

func writeStartupAssetError(err error) {
	if err == nil || logRoot == "" {
		return
	}
	_ = os.MkdirAll(logRoot, 0755)
	_ = os.WriteFile(
		filepath.Join(logRoot, "startup_asset_error.log"),
		[]byte(err.Error()+"\n"),
		0644,
	)
}
