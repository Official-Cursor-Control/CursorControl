package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func playerProfileSyncPayload(includeV320 bool) []byte {
	m := map[string]any{
		"action":                            "sync",
		"unlocked_ships":                    gameMeta.UnlockedShips,
		"selected_ship":                     gameMeta.SelectedShip,
		"unlocked_fire_colors":              gameMeta.UnlockedFireColors,
		"selected_fire_color":               gameMeta.SelectedFireColor,
		"unlocked_fire_sizes":               gameMeta.UnlockedFireSizes,
		"selected_fire_size":                gameMeta.SelectedFireSize,
		"unlocked_titles":                   unlockedProfileTitles(),
		"selected_title":                    selectedProfileTitle(),
		"unlocked_name_colours":             gameMeta.UnlockedNameColours,
		"selected_name_colour":              gameMeta.SelectedNameColour,
		"unlocked_profile_frames":           gameMeta.UnlockedProfileFrames,
		"selected_profile_frame":            gameMeta.SelectedProfileFrame,
		"unlocked_profile_skins":            gameMeta.UnlockedProfileFrames,
		"selected_profile_skin":             gameMeta.SelectedProfileFrame,
		"selected_profile_font":             gameMeta.SelectedProfileFont,
		"selected_profile_name_font":        gameMeta.SelectedProfileNameFont,
		"selected_profile_primary_colour":   gameMeta.ProfilePrimaryColour,
		"selected_profile_secondary_colour": gameMeta.ProfileSecondaryColour,
		"profile_name_shadow":               gameMeta.ProfileNameShadow,
		"profile_shadow_colour":             gameMeta.ProfileShadowColour,
		"profile_gradient_vertical":         gameMeta.ProfileGradientVertical,
		"selected_profile_animation":        gameMeta.ProfileAnimation,
		"survival_checkpoint":               gameMeta.SurvivalCheckpoint,
		"sentinel_defeats":                  gameMeta.SentinelDefeats,
		"serpent_defeats":                   gameMeta.SerpentDefeats,
		"array_defeats":                     gameMeta.ArrayDefeats,
	}
	if includeV320 {
		m["achievement_showcase"] = normalizedAchievementShowcase()
		m["best_survival_wave"] = gameMeta.BestSurvivalWave
		m["best_survival_kills"] = gameMeta.BestSurvivalKills
	}
	b, _ := json.Marshal(m)
	return b
}

func postPlayerProfileSync(token string, payload []byte) ([]byte, bool) {
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/functions/v1/player-profile", bytes.NewReader(payload))
	if err != nil {
		return nil, false
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := authHTTPClient().Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return body, resp.StatusCode >= 200 && resp.StatusCode < 300
}
