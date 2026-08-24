//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Starbit Bank v2.
//
// The client never submits a Starbit balance. It reports only monotonic earned
// and spent counters for the currently leased Discord session. Supabase owns the
// real balance. A small unbanked amount may exist internally between heartbeats,
// but only the protected Bank Total is shown to the player. The server sweeps
// validated session activity into the Bank every 15 seconds, so a fresh local
// zero can never overwrite the account.
const (
	afkBankEndpoint       = "/functions/v1/starbit-bank"
	afkBankRequestTimeout = 10 * time.Second
	afkBankSyncInterval   = 15 * time.Second
)

var (
	afkBankMu           sync.Mutex
	afkBankEarnedTotal  int64
	afkBankSpentTotal   int64
	afkBankProtected    int64
	afkBankUnbanked     int64
	afkBankLastSync     time.Time
	afkBankNextDeposit  time.Time
	afkBankInFlight     bool
	afkBankReady        bool
	afkBankStatus       string
	afkBankPulseStarted time.Time
)

type afkBankResponse struct {
	OK              bool   `json:"ok"`
	SessionLost     bool   `json:"session_lost"`
	Message         string `json:"message"`
	BankBalance     string `json:"bank_balance"`
	UnbankedBalance string `json:"unbanked_balance"`
	TotalBalance    string `json:"total_balance"`
	Revision        int64  `json:"revision"`
	LastBankAt      string `json:"last_bank_at"`
	NextBankAt      string `json:"next_bank_at"`
	EarnedReported  string `json:"earned_reported"`
	SpentReported   string `json:"spent_reported"`

	RequestEarned int64  `json:"-"`
	RequestSpent  int64  `json:"-"`
	RequestAction string `json:"-"`
}

func afkBankParse(v string) int64 {
	s := strings.TrimSpace(v)
	if s == "" {
		return 0
	}
	// Supabase numeric may be returned as an integer-looking decimal string.
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func afkBankRecordEarn(v int64) {
	if v <= 0 {
		return
	}
	afkBankMu.Lock()
	afkBankEarnedTotal = saturatingAdd(afkBankEarnedTotal, v)
	afkBankMu.Unlock()
}

func afkBankRecordSpend(v int64) {
	if v <= 0 {
		return
	}
	afkBankMu.Lock()
	afkBankSpentTotal = saturatingAdd(afkBankSpentTotal, v)
	afkBankMu.Unlock()
}

func afkGrantStarbits(v int64) {
	if v <= 0 {
		return
	}
	gameMeta.AFKStarbits = saturatingAdd(gameMeta.AFKStarbits, v)
	afkBankRecordEarn(v)
}

func afkBankSnapshot() (bank, unbanked int64, next time.Time, ready bool) {
	afkBankMu.Lock()
	defer afkBankMu.Unlock()
	return afkBankProtected, afkBankUnbanked, afkBankNextDeposit, afkBankReady
}

func afkBankPost(action string) (afkBankResponse, error) {
	tok, err := validAuthAccessToken()
	if err != nil {
		return afkBankResponse{}, err
	}

	afkCloudMu.Lock()
	sessionToken := afkCloudSessionToken
	afkCloudMu.Unlock()
	if sessionToken == "" {
		return afkBankResponse{}, fmt.Errorf("cloud session unavailable")
	}

	afkBankMu.Lock()
	earned := afkBankEarnedTotal
	spent := afkBankSpentTotal
	afkBankMu.Unlock()
	if action == "claim" {
		earned, spent = 0, 0
	}

	payload := map[string]any{
		"action":        action,
		"session_token": sessionToken,
		"earned_total":  earned,
		"spent_total":   spent,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+afkBankEndpoint, bytes.NewReader(body))
	if err != nil {
		return afkBankResponse{}, err
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: afkBankRequestTimeout}
	res, err := client.Do(req)
	if err != nil {
		return afkBankResponse{}, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var out afkBankResponse
	_ = json.Unmarshal(raw, &out)
	out.RequestEarned = earned
	out.RequestSpent = spent
	out.RequestAction = action
	if res.StatusCode == http.StatusConflict || out.SessionLost {
		out.SessionLost = true
		if out.Message == "" {
			out.Message = "This Discord account was opened on another device."
		}
		return out, fmt.Errorf("session lost")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || !out.OK {
		if out.Message == "" {
			out.Message = fmt.Sprintf("Starbit Bank failed (%d)", res.StatusCode)
		}
		return out, fmt.Errorf("%s", out.Message)
	}
	return out, nil
}

func afkBankApply(out afkBankResponse) {
	serverTotal := afkBankParse(out.TotalBalance)
	serverBank := afkBankParse(out.BankBalance)
	serverUnbanked := afkBankParse(out.UnbankedBalance)
	next := time.Time{}
	if out.NextBankAt != "" {
		next, _ = time.Parse(time.RFC3339Nano, out.NextBankAt)
	}

	afkBankMu.Lock()
	oldProtected := afkBankProtected
	wasReady := afkBankReady
	// Preserve only activity that happened after this exact request snapshot.
	postEarn := afkBankEarnedTotal - out.RequestEarned
	postSpend := afkBankSpentTotal - out.RequestSpent
	if postEarn < 0 {
		postEarn = 0
	}
	if postSpend < 0 {
		postSpend = 0
	}
	estimated := saturatingAdd(serverTotal, postEarn)
	if postSpend >= estimated {
		estimated = 0
	} else {
		estimated -= postSpend
	}

	afkBankProtected = serverBank
	afkBankUnbanked = serverUnbanked
	if estimated >= serverBank {
		afkBankUnbanked = estimated - serverBank
	} else {
		// A post-request purchase may have reached into the bank. Keep the UI
		// total correct without inventing negative unbanked currency.
		afkBankProtected = estimated
		afkBankUnbanked = 0
	}
	afkBankNextDeposit = next
	afkBankReady = true
	afkBankLastSync = time.Now()
	afkBankStatus = "STARBITS BANKED"
	if wasReady && afkBankProtected != oldProtected {
		afkBankPulseStarted = time.Now()
	}
	afkBankMu.Unlock()

	gameMeta.AFKStarbits = estimated
	saveGameMeta()
}

func afkBankClaimAsync() {
	afkBankMu.Lock()
	if afkBankInFlight {
		afkBankMu.Unlock()
		return
	}
	afkBankInFlight = true
	afkBankReady = false
	afkBankStatus = "OPENING STARBIT BANK..."
	// Every device lease creates a fresh monotonic counter namespace.
	afkBankEarnedTotal = 0
	afkBankSpentTotal = 0
	afkBankMu.Unlock()
	go func() {
		out, err := afkBankPost("claim")
		afkBankMu.Lock()
		afkBankInFlight = false
		afkBankMu.Unlock()
		if err != nil {
			afkBankMu.Lock()
			afkBankStatus = "STARBIT BANK UNAVAILABLE"
			afkBankMu.Unlock()
			return
		}
		postMainThreadTask(func() {
			afkBankApply(out)
			if mainHwnd != 0 {
				invalidateRect.Call(mainHwnd, 0, 0)
			}
		})
	}()
}

func afkBankSyncAsync(now time.Time) {
	afkBankMu.Lock()
	if !afkBankReady || afkBankInFlight || (!afkBankLastSync.IsZero() && now.Sub(afkBankLastSync) < afkBankSyncInterval) {
		afkBankMu.Unlock()
		return
	}
	afkBankInFlight = true
	afkBankMu.Unlock()
	go func() {
		out, err := afkBankPost("sync")
		afkBankMu.Lock()
		afkBankInFlight = false
		afkBankMu.Unlock()
		if err != nil {
			if out.SessionLost {
				afkBankMu.Lock()
				afkBankReady = false
				afkBankStatus = "SIGNED OUT // ACCOUNT ACTIVE ON ANOTHER DEVICE"
				afkBankMu.Unlock()
				postMainThreadTask(clearAuthSession)
			} else {
				afkBankMu.Lock()
				afkBankStatus = "STARBIT BANK RECONNECTING..."
				afkBankMu.Unlock()
			}
			return
		}
		postMainThreadTask(func() {
			afkBankApply(out)
			if mainHwnd != 0 {
				invalidateRect.Call(mainHwnd, 0, 0)
			}
		})
	}()
}

func afkBankTick(now time.Time) { afkBankSyncAsync(now) }

func afkBankReset() {
	afkBankMu.Lock()
	afkBankEarnedTotal = 0
	afkBankSpentTotal = 0
	afkBankProtected = 0
	afkBankUnbanked = 0
	afkBankNextDeposit = time.Time{}
	afkBankInFlight = false
	afkBankReady = false
	afkBankStatus = ""
	afkBankPulseStarted = time.Time{}
	afkBankMu.Unlock()
}

func afkBankPulseStrength(now time.Time) float64 {
	afkBankMu.Lock()
	started := afkBankPulseStarted
	afkBankMu.Unlock()
	if started.IsZero() {
		return 0
	}
	const duration = 1350 * time.Millisecond
	age := now.Sub(started)
	if age < 0 || age >= duration {
		return 0
	}
	// Strong immediate flash with a smooth decay. The Starbase repaint heartbeat
	// keeps the yellow pulse visibly animated without another timer.
	t := float64(age) / float64(duration)
	return (1 - t) * (1 - t)
}

func afkBankHUDText(now time.Time) (label, value string) {
	bank, _, _, ready := afkBankSnapshot()
	if !ready {
		// Keep the last confirmed protected total visible during transient cloud
		// reconnects. A temporary progression-sync failure is not a Bank reset.
		if bank > 0 {
			return "STARBIT BANK", formatAFKNumber(bank)
		}
		return "STARBIT BANK", "SYNCING..."
	}
	// v424: only the protected Bank Total is visible. The session's current/live
	// amount remains internal and is swept into the Bank on the 15-second server
	// heartbeat.
	return "STARBIT BANK", formatAFKNumber(bank)
}

// Best-effort clean-exit flush. Normal 10-second ledger syncs already protect
// unbanked Starbits server-side; this simply asks the server to sweep them into
// the protected Bank before a normal application shutdown.
func afkBankFlushOnExit() {
	afkBankMu.Lock()
	ready := afkBankReady
	afkBankMu.Unlock()
	if !ready {
		return
	}
	done := make(chan struct{}, 1)
	go func() {
		_, _ = afkBankPost("bank_now")
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(1200 * time.Millisecond):
	}
}
