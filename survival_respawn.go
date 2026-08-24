//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const survivalRespawnEndpoint = "/functions/v1/survival-respawn"

var (
	survivalRespawnMu             sync.Mutex
	survivalRespawnBusy           bool
	survivalRespawnRemaining      int
	survivalRespawnAllowance      int
	survivalRespawnRank           string
	survivalRespawnResetAt        string
	survivalRespawnMessage        string
	survivalRespawnCanUse         bool
	survivalRespawnFailedWave     int
	survivalRespawnFailedBoss     int
	survivalRespawnGeneration     uint64
	survivalRespawnRestartPending bool
	survivalRespawnRestartWave    int
	survivalRespawnRestartBoss    int
	survivalReviveImmunityUntil   time.Time
	survivalRespawnStatusChecked  bool
)

func survivalRespawnNonNegative(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

type survivalRespawnResponse struct {
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	Allowance  int    `json:"allowance"`
	Remaining  int    `json:"remaining"`
	ResetAt    string `json:"reset_at"`
	GlobalRank string `json:"global_rank"`
}

func survivalRespawnRememberFailure() {
	survivalRespawnMu.Lock()
	defer survivalRespawnMu.Unlock()
	survivalRespawnGeneration++
	survivalRespawnFailedWave = survivalWave
	survivalRespawnFailedBoss = 0
	if survivalBoss1Active() {
		survivalRespawnFailedBoss = 1
	}
	if survivalBoss2Active() {
		survivalRespawnFailedBoss = 2
	}
	if survivalBoss3Active() {
		survivalRespawnFailedBoss = 3
	}
	survivalRespawnBusy = false
	survivalRespawnCanUse = false
	survivalRespawnRemaining = 0
	survivalRespawnAllowance = 0
	survivalRespawnRank = ""
	survivalRespawnResetAt = ""
	survivalRespawnMessage = "CHECKING DAILY REVIVES..."
	survivalRespawnRestartPending = false
	survivalRespawnRestartWave = 0
	survivalRespawnRestartBoss = 0
	survivalRespawnStatusChecked = false
	survivalReviveImmunityUntil = time.Time{}
}

func survivalRespawnPost(action string) (survivalRespawnResponse, error) {
	authMu.Lock()
	token := strings.TrimSpace(authSession.AccessToken)
	connected := discordConnected
	authMu.Unlock()

	// afkEnsureDeviceID owns afkCloudMu internally. The old implementation called
	// it while afkCloudMu was already held, deadlocking the revive worker on every
	// Survival death and eventually freezing the rest of the cloud/UI paths.
	deviceID := afkEnsureDeviceID()
	afkCloudMu.Lock()
	sessionToken := strings.TrimSpace(afkCloudSessionToken)
	afkCloudMu.Unlock()
	if !connected || token == "" || sessionToken == "" {
		return survivalRespawnResponse{}, errors.New("Discord cloud session required")
	}

	payload, err := json.Marshal(map[string]any{"action": action, "device_id": deviceID, "session_token": sessionToken})
	if err != nil {
		return survivalRespawnResponse{}, err
	}
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+survivalRespawnEndpoint, bytes.NewReader(payload))
	if err != nil {
		return survivalRespawnResponse{}, err
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return survivalRespawnResponse{}, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return survivalRespawnResponse{}, readErr
	}
	var out survivalRespawnResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return survivalRespawnResponse{}, errors.New("invalid revive server response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !out.OK {
		if strings.TrimSpace(out.Message) == "" {
			out.Message = fmt.Sprintf("Respawn unavailable (%d)", resp.StatusCode)
		}
		return out, errors.New(out.Message)
	}
	return out, nil
}

func survivalRespawnPostUIMessage() {
	if mainHwnd != 0 {
		postMessageW.Call(mainHwnd, WM_SURVIVAL_RESPAWN, 0, 0)
	}
}

func survivalRespawnRefreshAsync() {
	survivalRespawnMu.Lock()
	if survivalRespawnBusy {
		survivalRespawnMu.Unlock()
		return
	}
	gen := survivalRespawnGeneration
	survivalRespawnBusy = true
	survivalRespawnCanUse = false
	survivalRespawnMessage = "CHECKING DAILY REVIVES..."
	survivalRespawnMu.Unlock()

	go func(generation uint64) {
		out, err := survivalRespawnPost("status")
		survivalRespawnMu.Lock()
		if generation != survivalRespawnGeneration {
			survivalRespawnMu.Unlock()
			return
		}
		survivalRespawnBusy = false
		survivalRespawnStatusChecked = true
		if err != nil {
			survivalRespawnCanUse = false
			if strings.TrimSpace(out.Message) != "" {
				survivalRespawnMessage = out.Message
			} else {
				survivalRespawnMessage = "REVIVE CHECK UNAVAILABLE // CLICK TO RETRY"
			}
		} else {
			survivalRespawnAllowance = survivalRespawnNonNegative(out.Allowance)
			survivalRespawnRemaining = survivalRespawnNonNegative(out.Remaining)
			survivalRespawnRank = strings.TrimSpace(out.GlobalRank)
			survivalRespawnResetAt = strings.TrimSpace(out.ResetAt)
			survivalRespawnCanUse = survivalRespawnRemaining > 0
			survivalRespawnMessage = fmt.Sprintf("%s // %d/%d REVIVES LEFT", survivalRespawnRank, survivalRespawnRemaining, survivalRespawnAllowance)
		}
		survivalRespawnMu.Unlock()
		survivalRespawnPostUIMessage()
	}(gen)
}

func survivalRespawnPanelRect(w, h int32) RECT {
	ar := arenaRect(w, h)
	pw := sx(370, w)
	ph := sy(190, h)
	margin := sx(26, w)
	cy := (ar.Top + ar.Bottom) / 2
	return RECT{ar.Right - margin - pw, cy - ph/2, ar.Right - margin, cy + ph/2}
}

func survivalRespawnButtonRect(w, h int32) RECT {
	p := survivalRespawnPanelRect(w, h)
	marginX := sx(24, w)
	bh := sy(46, h)
	return RECT{p.Left + marginX, p.Bottom - sy(66, h), p.Right - marginX, p.Bottom - sy(66, h) + bh}
}

func drawSurvivalRespawnBox(hdc uintptr, w, h int32) {
	if !survivalActive() || state != StateFailed {
		return
	}
	p := survivalRespawnPanelRect(w, h)
	b := survivalRespawnButtonRect(w, h)
	survivalRespawnMu.Lock()
	busy, canUse, rem, msg := survivalRespawnBusy, survivalRespawnCanUse, survivalRespawnRemaining, survivalRespawnMessage
	survivalRespawnMu.Unlock()

	// Keep the choice visually separate from the central failure/report overlay.
	fillSolidRect(hdc, p, rgb(18, 22, 38))
	border := rgb(142, 101, 235)
	fillSolidRect(hdc, RECT{p.Left, p.Top, p.Right, p.Top + sy(2, h)}, border)
	fillSolidRect(hdc, RECT{p.Left, p.Bottom - sy(2, h), p.Right, p.Bottom}, border)
	fillSolidRect(hdc, RECT{p.Left, p.Top, p.Left + sx(2, w), p.Bottom}, border)
	fillSolidRect(hdc, RECT{p.Right - sx(2, w), p.Top, p.Right, p.Bottom}, border)

	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		centeredTextOut(hdc, p.Left+sx(16, w), p.Right-sx(16, w), p.Top+sy(20, h), "DO YOU WANT TO REVIVE?")
		selectObject.Call(hdc, old)
	}

	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(205, 215, 235))
		line := "CHECKING YOUR DAILY REVIVES..."
		if !busy {
			line = fmt.Sprintf("YOU HAVE %d REVIVES REMAINING.", rem)
		}
		centeredTextOut(hdc, p.Left+sx(14, w), p.Right-sx(14, w), p.Top+sy(57, h), line)
		setTextColor.Call(hdc, rgb(166, 190, 222))
		centeredTextOut(hdc, p.Left+sx(14, w), p.Right-sx(14, w), p.Top+sy(82, h), "FULL HP  •  NOVA BOMB  •  3 SEC IMMUNITY")
		selectObject.Call(hdc, old)
	}

	buttonCol := rgb(52, 55, 72)
	buttonText := "NO REVIVES AVAILABLE"
	hover := pointInRect(cursorPos, b)
	if busy {
		buttonText = "CHECKING..."
	} else if canUse {
		buttonCol = rgb(87, 57, 156)
		if hover {
			buttonCol = rgb(116, 76, 204)
		}
		buttonText = "REVIVE"
	}
	fillSolidRect(hdc, b, buttonCol)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		centeredTextOut(hdc, b.Left, b.Right, b.Top+sy(12, h), buttonText)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(138, 160, 190))
		foot := "CLICK ANYWHERE TO CONTINUE"
		if !busy && !canUse && strings.TrimSpace(msg) != "" {
			foot = "CLICK ANYWHERE TO CONTINUE"
		}
		centeredTextOut(hdc, p.Left+sx(12, w), p.Right-sx(12, w), p.Bottom+sy(10, h), foot)
		selectObject.Call(hdc, old)
	}
}

func survivalRespawnHandleClick(h uintptr, p FPoint, w, hgt int32) bool {
	if !survivalActive() || state != StateFailed || !pointInArena(p, w, hgt) {
		return false
	}
	button := survivalRespawnButtonRect(w, hgt)
	if !pointInRect(p, button) {
		// Any click in the game area that is not the REVIVE button means the
		// player wants to continue to the mission report immediately.
		survivalRespawnMu.Lock()
		survivalRespawnGeneration++
		survivalRespawnBusy = false
		survivalRespawnCanUse = false
		survivalRespawnMu.Unlock()
		finishSurvivalFailure(h)
		return true
	}
	survivalRespawnMu.Lock()
	if survivalRespawnBusy {
		survivalRespawnMu.Unlock()
		return true
	}
	if !survivalRespawnCanUse {
		survivalRespawnMu.Unlock()
		survivalRespawnRefreshAsync()
		return true
	}
	gen := survivalRespawnGeneration
	survivalRespawnBusy = true
	survivalRespawnCanUse = false
	survivalRespawnMessage = "CLAIMING REVIVE..."
	survivalRespawnMu.Unlock()
	survivalRespawnPostUIMessage()

	go func(generation uint64) {
		out, err := survivalRespawnPost("claim")
		survivalRespawnMu.Lock()
		if generation != survivalRespawnGeneration {
			survivalRespawnMu.Unlock()
			return
		}
		survivalRespawnBusy = false
		survivalRespawnStatusChecked = true
		if err != nil {
			if strings.TrimSpace(out.Message) != "" {
				survivalRespawnMessage = out.Message
			} else {
				survivalRespawnMessage = "REVIVE CLAIM FAILED // CLICK TO RETRY"
			}
			survivalRespawnCanUse = survivalRespawnRemaining > 0
			survivalRespawnMu.Unlock()
			survivalRespawnPostUIMessage()
			return
		}
		survivalRespawnRemaining = survivalRespawnNonNegative(out.Remaining)
		survivalRespawnAllowance = survivalRespawnNonNegative(out.Allowance)
		survivalRespawnRank = strings.TrimSpace(out.GlobalRank)
		survivalRespawnResetAt = strings.TrimSpace(out.ResetAt)
		survivalRespawnCanUse = survivalRespawnRemaining > 0
		survivalRespawnMessage = fmt.Sprintf("REVIVE USED // %d/%d LEFT", survivalRespawnRemaining, survivalRespawnAllowance)
		survivalRespawnRestartWave = survivalRespawnFailedWave
		survivalRespawnRestartBoss = survivalRespawnFailedBoss
		survivalRespawnRestartPending = true
		survivalRespawnMu.Unlock()
		survivalRespawnPostUIMessage()
	}(gen)
	return true
}

func survivalReviveImmune(now time.Time) bool {
	return !survivalReviveImmunityUntil.IsZero() && now.Before(survivalReviveImmunityUntil)
}

func survivalRespawnActivateBenefits(h uintptr) {
	now := time.Now()
	survivalHP = survivalMaxHP()
	// Apply the same immediate clear as collecting the normal Survival Bomb pickup.
	// This deliberately does not affect boss HP; it clears normal threats and gives
	// the player a clean re-entry into the checkpoint.
	hadThreats := len(survivalEnemies) > 0
	survivalCollectPickup(survivalPickupBomb)
	if !hadThreats {
		playSurvivalExplodeSound()
	}
	survivalReviveImmunityUntil = now.Add(3 * time.Second)
	status = "REVIVED // FULL HP // NOVA BOMB // 3 SEC IMMUNITY"
	invalidateSurvivalHUD(h)
	invalidateSurvivalArena(h)
}

// Network goroutines only publish data. All gameplay state mutation happens here
// on wndProc's UI thread, preventing revive completions from racing the Windows
// message loop, failure timer, boss cleanup, or checkpoint restart code.
func survivalRespawnProcessMainThread(h uintptr) {
	survivalRespawnMu.Lock()
	checked := survivalRespawnStatusChecked
	busy := survivalRespawnBusy
	canUse := survivalRespawnCanUse
	remaining := survivalRespawnRemaining
	pending := survivalRespawnRestartPending
	wave := survivalRespawnRestartWave
	boss := survivalRespawnRestartBoss
	if pending {
		survivalRespawnRestartPending = false
	}
	survivalRespawnMu.Unlock()

	// If the authoritative status check completed successfully and there are no
	// revives left, do not leave the player sitting on a dead choice panel.
	// Network errors still keep the prompt available so the player can retry or
	// click through manually.
	if checked && !busy && !canUse && remaining == 0 && state == StateFailed && survivalActive() {
		survivalRespawnMu.Lock()
		msg := strings.TrimSpace(survivalRespawnMessage)
		survivalRespawnMu.Unlock()
		if msg != "" && !strings.Contains(strings.ToUpper(msg), "UNAVAILABLE") && !strings.Contains(strings.ToUpper(msg), "RETRY") && !strings.Contains(strings.ToUpper(msg), "SESSION") {
			finishSurvivalFailure(h)
			return
		}
	}

	if pending && state == StateFailed && survivalActive() {
		killTimer.Call(h, TIMER_FAIL_RESET)
		finishSurvivalFailure(h)
		resetToWaiting(h)
		survivalSelectedStartBoss = boss
		if boss == 0 {
			survivalSelectedStartWave = wave
		} else {
			survivalSelectedStartWave = 0
		}
		startSurvivalGame(h)
		survivalRespawnActivateBenefits(h)
		return
	}
	invalidateRect.Call(h, 0, 0)
}
