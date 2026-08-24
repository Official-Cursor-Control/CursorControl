//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const afkExchangeEndpoint = "/functions/v1/starbase-exchange"

var (
	afkExchangePanelOpen    bool
	afkExchangeBusy         bool
	afkExchangeMessage      string
	afkExchangeMessageUntil time.Time
)

type afkExchangeBundle struct {
	Cost   int64
	Reward int
}

var afkExchangeSpaceCoinBundles = [...]afkExchangeBundle{
	{100_000_000, 50},
	{500_000_000, 260},
	{1_000_000_000, 550},
	{5_000_000_000, 3000},
}

var afkExchangeEXPBundles = [...]afkExchangeBundle{
	{100_000_000, 10},
	{500_000_000, 55},
	{1_000_000_000, 120},
	{5_000_000_000, 650},
}

var afkExchangeStationHPBundles = [...]afkExchangeBundle{
	{250_000_000, 1},
	{1_000_000_000, 1},
}

func afkExchangePanelRect(w, hgt int32) RECT { return afkManagementPanelRect(w, hgt) }

func afkExchangeBundleRect(w, hgt int32, section, index int) RECT {
	p := afkExchangePanelRect(w, hgt)
	left := p.Left + sx(32, w)
	right := p.Right - sx(32, w)
	gap := sx(12, w)
	cw := (right - left - gap*3) / 4
	top := p.Top + sy(125, hgt)
	if section == 1 {
		top = p.Top + sy(285, hgt)
	} else if section == 2 {
		top = p.Top + sy(445, hgt)
	}
	x := left + int32(index)*(cw+gap)
	return RECT{x, top, x + cw, top + sy(105, hgt)}
}

func afkExchangeSetMessage(s string) {
	afkExchangeMessage = s
	afkExchangeMessageUntil = time.Now().Add(2600 * time.Millisecond)
}

func drawAFKExchangeBundle(hdc uintptr, r RECT, title string, b afkExchangeBundle, enabled bool, hovered bool, w, hgt int32) {
	fill := rgb(6, 26, 51)
	border := rgb(57, 111, 148)
	if enabled {
		border = rgb(79, 208, 255)
	}
	if hovered && enabled {
		fill = rgb(19, 43, 76)
		border = rgb(255, 205, 66)
	}
	drawBevelPanel(hdc, r, fill, border, rgb(0, 5, 14), 3)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(244, 249, 255))
		centeredTextOut(hdc, r.Left+6, r.Right-6, r.Top+sy(13, hgt), title)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 215, 74))
		centeredTextOut(hdc, r.Left+6, r.Right-6, r.Top+sy(49, hgt), formatAFKNumber(b.Cost)+" STARBITS")
		setTextColor.Call(hdc, rgb(128, 185, 210))
		status := "CLICK TO EXCHANGE"
		if !discordConnected {
			status = "DISCORD LOGIN REQUIRED"
		} else if gameMeta.AFKStarbits < b.Cost {
			status = "NOT ENOUGH STARBITS"
		} else if afkExchangeBusy {
			status = "PROCESSING..."
		}
		centeredTextOut(hdc, r.Left+6, r.Right-6, r.Bottom-sy(25, hgt), fitTextEllipsis(hdc, hudTinyFont, status, r.Right-r.Left-sx(14, w)))
		selectObject.Call(hdc, old)
	}
}

func drawAFKExchange(hdc uintptr, w, hgt int32) {
	if !afkExchangePanelOpen {
		return
	}
	drawAFKManagementBackdrop(hdc, w, hgt)
	p := afkExchangePanelRect(w, hgt)
	drawAFKManagementChrome(hdc, p, "STARBASE EXCHANGE", "CONVERT STARBITS INTO SMALL MAIN-GAME REWARDS // PURCHASES ARE NEVER AUTOMATIC", rgb(176, 89, 255), w, hgt)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(115, 224, 255))
		textOut(hdc, p.Left+sx(32, w), p.Top+sy(101, hgt), "SPACECOINS")
		textOut(hdc, p.Left+sx(32, w), p.Top+sy(261, hgt), "GLOBAL EXP")
		textOut(hdc, p.Left+sx(32, w), p.Top+sy(421, hgt), "PERMANENT STATION HP // MAX +2")
		selectObject.Call(hdc, old)
	}
	for i, b := range afkExchangeSpaceCoinBundles {
		r := afkExchangeBundleRect(w, hgt, 0, i)
		enabled := discordConnected && !afkExchangeBusy && gameMeta.AFKStarbits >= b.Cost
		drawAFKExchangeBundle(hdc, r, fmt.Sprintf("+%s", formatAFKNumber(int64(b.Reward))), b, enabled, pointInRect(cursorPos, r), w, hgt)
	}
	for i, b := range afkExchangeEXPBundles {
		r := afkExchangeBundleRect(w, hgt, 1, i)
		enabled := discordConnected && !afkExchangeBusy && gameMeta.AFKStarbits >= b.Cost
		drawAFKExchangeBundle(hdc, r, fmt.Sprintf("+%s EXP", formatAFKNumber(int64(b.Reward))), b, enabled, pointInRect(cursorPos, r), w, hgt)
	}
	for i, b := range afkExchangeStationHPBundles {
		r := afkExchangeBundleRect(w, hgt, 2, i)
		owned := gameMeta.AFKStationHPBonus > i
		sequential := gameMeta.AFKStationHPBonus == i
		enabled := discordConnected && !afkExchangeBusy && !owned && sequential && gameMeta.AFKStarbits >= b.Cost
		title := fmt.Sprintf("+1 STATION HP // %d/2", i+1)
		if owned {
			title = fmt.Sprintf("STATION HP +%d // OWNED", i+1)
		}
		drawAFKExchangeBundle(hdc, r, title, b, enabled, pointInRect(cursorPos, r), w, hgt)
	}
	if afkExchangeMessage != "" && time.Now().Before(afkExchangeMessageUntil) && hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 232, 172))
		centeredTextOut(hdc, p.Left+sx(30, w), p.Right-sx(30, w), p.Bottom-sy(31, hgt), fitTextEllipsis(hdc, hudTinyFont, afkExchangeMessage, p.Right-p.Left-sx(60, w)))
		selectObject.Call(hdc, old)
	}
}

type afkExchangeResponse struct {
	OK               bool   `json:"ok"`
	Message          string `json:"message"`
	Starbits         int64  `json:"starbits"`
	BankBalance      string `json:"bank_balance"`
	UnbankedBalance  string `json:"unbanked_balance"`
	SpaceCoinsReward int    `json:"spacecoins_reward"`
	EXPReward        int    `json:"exp_reward"`
	GlobalEXP        int    `json:"global_exp"`
	GlobalRank       string `json:"global_rank"`
	StationHPReward  int    `json:"station_hp_reward"`
	StationHPBonus   int    `json:"station_hp_bonus"`
	SessionLost      bool   `json:"session_lost"`
	ForceLogout      bool   `json:"force_logout"`
}

func afkExchangePurchase(kind string, index int) {
	if afkExchangeBusy {
		return
	}
	if !discordConnected {
		afkExchangeSetMessage("DISCORD LOGIN REQUIRED FOR STARBASE EXCHANGE")
		return
	}
	var bundles []afkExchangeBundle
	switch kind {
	case "spacecoins":
		bundles = afkExchangeSpaceCoinBundles[:]
	case "exp":
		bundles = afkExchangeEXPBundles[:]
	case "station_hp":
		bundles = afkExchangeStationHPBundles[:]
	default:
		return
	}
	if index < 0 || index >= len(bundles) {
		return
	}
	b := bundles[index]
	if gameMeta.AFKStarbits < b.Cost {
		afkExchangeSetMessage("NOT ENOUGH STARBITS")
		return
	}

	afkBankMu.Lock()
	if !afkBankReady || afkBankInFlight {
		afkBankMu.Unlock()
		afkExchangeSetMessage("STARBIT BANK IS SYNCING // TRY AGAIN IN A MOMENT")
		return
	}
	afkBankInFlight = true
	afkBankMu.Unlock()

	afkCloudMu.Lock()
	if afkCloudSyncInFlight {
		afkCloudMu.Unlock()
		afkBankMu.Lock()
		afkBankInFlight = false
		afkBankMu.Unlock()
		afkExchangeSetMessage("STARBASE CLOUD IS SYNCING // TRY AGAIN IN A MOMENT")
		return
	}
	deviceID := afkCloudDeviceID
	sessionToken := afkCloudSessionToken
	if strings.TrimSpace(sessionToken) == "" {
		afkCloudMu.Unlock()
		afkBankMu.Lock()
		afkBankInFlight = false
		afkBankMu.Unlock()
		afkExchangeSetMessage("STARBASE CLOUD SESSION IS NOT READY YET")
		return
	}
	// Reserve the AFK cloud lane for the purchase so a 10-second heartbeat cannot
	// race the authoritative Starbit deduction and overwrite it.
	afkCloudSyncInFlight = true
	afkCloudMu.Unlock()
	if strings.TrimSpace(deviceID) == "" {
		deviceID = afkEnsureDeviceID()
	}

	tok, err := validAuthAccessToken()
	if err != nil || strings.TrimSpace(tok) == "" {
		afkCloudMu.Lock()
		afkCloudSyncInFlight = false
		afkCloudMu.Unlock()
		afkBankMu.Lock()
		afkBankInFlight = false
		afkBankMu.Unlock()
		afkExchangeSetMessage("DISCORD SESSION EXPIRED // LOG IN AGAIN")
		return
	}

	afkExchangeBusy = true
	afkExchangeSetMessage("PROCESSING EXCHANGE...")
	go func() {
		defer func() {
			afkBankMu.Lock()
			afkBankInFlight = false
			afkBankLastSync = time.Now()
			afkBankMu.Unlock()
			afkCloudMu.Lock()
			afkCloudSyncInFlight = false
			afkCloudLastSync = time.Now()
			afkCloudMu.Unlock()
			postMainThreadTask(func() {
				afkExchangeBusy = false
				if mainHwnd != 0 {
					invalidateRect.Call(mainHwnd, 0, 0)
				}
			})
		}()
		postMessage := func(message string) {
			postMainThreadTask(func() { afkExchangeSetMessage(message) })
		}
		payload := map[string]any{"kind": kind, "bundle": index, "device_id": deviceID, "session_token": sessionToken}
		raw, _ := json.Marshal(payload)
		req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+afkExchangeEndpoint, bytes.NewReader(raw))
		if err != nil {
			postMessage("EXCHANGE REQUEST FAILED")
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("apikey", supabasePublishableKey)
		req.Header.Set("Authorization", "Bearer "+tok)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			postMessage("EXCHANGE SERVER UNAVAILABLE")
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		var out afkExchangeResponse
		if json.Unmarshal(body, &out) != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || !out.OK {
			if out.SessionLost || out.ForceLogout {
				afkBankMu.Lock()
				afkBankReady = false
				afkBankMu.Unlock()
				postMainThreadTask(clearAuthSession)
			}
			if out.Message != "" {
				postMessage(strings.ToUpper(out.Message))
			} else {
				postMessage("EXCHANGE FAILED")
			}
			return
		}

		serverTotal := out.Starbits
		serverBank := afkBankParse(out.BankBalance)
		serverLive := afkBankParse(out.UnbankedBalance)
		postMainThreadTask(func() {
			gameMeta.AFKStarbits = serverTotal
			afkBankMu.Lock()
			afkBankProtected = serverBank
			afkBankUnbanked = serverLive
			afkBankLastSync = time.Now()
			afkBankReady = true
			afkBankMu.Unlock()
			if out.SpaceCoinsReward > 0 {
				gameMeta.SpaceCoins += out.SpaceCoinsReward
			}
			if out.EXPReward > 0 {
				globalAccountEXP = out.GlobalEXP
				globalAccountEXPLoaded = true
			}
			if out.StationHPBonus > gameMeta.AFKStationHPBonus {
				gameMeta.AFKStationHPBonus = out.StationHPBonus
				if gameMeta.AFKStationHPBonus > 2 {
					gameMeta.AFKStationHPBonus = 2
				}
			}
			saveGameMeta()
			if kind == "spacecoins" {
				afkExchangeSetMessage(fmt.Sprintf("EXCHANGE COMPLETE // +%d SPACECOINS", out.SpaceCoinsReward))
			} else if kind == "station_hp" {
				afkExchangeSetMessage(fmt.Sprintf("PERMANENT STATION HP +1 // BONUS %d/2", gameMeta.AFKStationHPBonus))
			} else {
				afkExchangeSetMessage(fmt.Sprintf("EXCHANGE COMPLETE // +%d GLOBAL EXP // %s", out.EXPReward, out.GlobalRank))
			}
			playAFKResearchSound()
		})
	}()

}

func handleAFKExchangeClick(h uintptr, p FPoint, w, hgt int32) bool {
	if !afkExchangePanelOpen {
		return false
	}
	if !pointInRect(p, afkExchangePanelRect(w, hgt)) {
		afkExchangePanelOpen = false
		playUIButtonClickSound()
		invalidateRect.Call(h, 0, 0)
		return true
	}
	for i := range afkExchangeSpaceCoinBundles {
		if pointInRect(p, afkExchangeBundleRect(w, hgt, 0, i)) {
			afkExchangePurchase("spacecoins", i)
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	for i := range afkExchangeEXPBundles {
		if pointInRect(p, afkExchangeBundleRect(w, hgt, 1, i)) {
			afkExchangePurchase("exp", i)
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	for i := range afkExchangeStationHPBundles {
		if pointInRect(p, afkExchangeBundleRect(w, hgt, 2, i)) {
			afkExchangePurchase("station_hp", i)
			playUIButtonClickSound()
			invalidateRect.Call(h, 0, 0)
			return true
		}
	}
	return true
}
