//go:build windows

package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AFK cloud authority / single-device lease.
// A Discord-authenticated account may own exactly one active Cursor Control
// device. The newest login claims the lease and invalidates the old token.
// Logged-out play remains local-only and is never merged into an existing
// account cloud save.
const (
	afkCloudSyncInterval   = 10 * time.Second
	afkCloudLeaseGrace     = 35 * time.Second
	afkCloudRequestTimeout = 10 * time.Second
	afkCloudEndpoint       = "/functions/v1/afk-sync"
)

var (
	afkCloudMu                  sync.Mutex
	afkCloudDeviceID            string
	afkCloudSessionToken        string
	afkCloudLastVerified        time.Time
	afkCloudLastSync            time.Time
	afkCloudSyncInFlight        bool
	afkCloudStatus              string
	afkCloudManualGainSinceSync int64
)

type afkCloudResponse struct {
	OK                 bool           `json:"ok"`
	SessionToken       string         `json:"session_token"`
	SessionLost        bool           `json:"session_lost"`
	Message            string         `json:"message"`
	ServerTime         string         `json:"server_time"`
	Revision           int64          `json:"revision"`
	State              map[string]any `json:"state"`
	Starbits           int64          `json:"starbits"`
	PendingStarbits    int64          `json:"pending_starbits"`
	PendingAwaySeconds int64          `json:"pending_away_seconds"`
	PendingPaidSeconds int64          `json:"pending_paid_seconds"`

	// Local request snapshots are not serialized. They let the client merge any
	// production/rewards/spending that happened while the HTTP request was in
	// flight instead of overwriting the live balance with an older server reply.
	RequestStarbits int64  `json:"-"`
	RequestAction   string `json:"-"`
}

func afkCloudDevicePath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if cfg, err := os.UserConfigDir(); err == nil {
			base = cfg
		}
	}
	if base == "" {
		base = "."
	}
	dir := filepath.Join(base, "KongGame", "CursorControlTrainer")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "device_id.txt")
}

func afkRandomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	// UUID-shaped random identifier without an external dependency.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b)
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}

func afkEnsureDeviceID() string {
	afkCloudMu.Lock()
	defer afkCloudMu.Unlock()
	if afkCloudDeviceID != "" {
		return afkCloudDeviceID
	}
	path := afkCloudDevicePath()
	if b, err := os.ReadFile(path); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" {
			afkCloudDeviceID = v
			return v
		}
	}
	afkCloudDeviceID = afkRandomToken()
	_ = atomicWriteFile(path, []byte(afkCloudDeviceID), 0600)
	return afkCloudDeviceID
}

func afkSnapshotMap() map[string]any {
	raw, _ := json.Marshal(gameMeta)
	all := map[string]any{}
	_ = json.Unmarshal(raw, &all)
	out := map[string]any{}
	for k, v := range all {
		if strings.HasPrefix(k, "afk_") {
			out[k] = v
		}
	}
	return out
}

func afkCloudParseInt64(v string) int64 {
	if strings.TrimSpace(v) == "" {
		return 0
	}
	var n int64
	_, _ = fmt.Sscan(strings.TrimSpace(v), &n)
	if n < 0 {
		return 0
	}
	return n
}

func afkCloudAnyInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		if x, err := n.Int64(); err == nil {
			return int(x)
		}
	}
	return 0
}

func afkCloudKeepHigherInt(state map[string]any, key string, local int) {
	if afkCloudAnyInt(state[key]) < local {
		state[key] = local
	}
}

func afkCloudKeepTrue(state map[string]any, key string, local bool) {
	cloud, _ := state[key].(bool)
	if local || cloud {
		state[key] = true
	}
}

func afkCloudKeepHigherSlice(state map[string]any, key string, local []int) {
	cloudRaw, _ := state[key].([]any)
	n := len(local)
	if len(cloudRaw) > n {
		n = len(cloudRaw)
	}
	out := make([]int, n)
	for i := 0; i < n; i++ {
		cv := 0
		if i < len(cloudRaw) {
			cv = afkCloudAnyInt(cloudRaw[i])
		}
		lv := 0
		if i < len(local) {
			lv = local[i]
		}
		if lv > cv {
			out[i] = lv
		} else {
			out[i] = cv
		}
	}
	state[key] = out
}

func afkCloudKeepHigherInt64Slice(state map[string]any, key string, local []int64) {
	cloudRaw, _ := state[key].([]any)
	n := len(local)
	if len(cloudRaw) > n {
		n = len(cloudRaw)
	}
	out := make([]int64, n)
	for i := 0; i < n; i++ {
		var cv int64
		if i < len(cloudRaw) {
			switch v := cloudRaw[i].(type) {
			case float64:
				cv = int64(v)
			case int64:
				cv = v
			case int:
				cv = int64(v)
			case json.Number:
				cv, _ = v.Int64()
			}
		}
		var lv int64
		if i < len(local) {
			lv = local[i]
		}
		if lv > cv {
			out[i] = lv
		} else {
			out[i] = cv
		}
	}
	state[key] = out
}

func afkCloudKeepTrueSlice(state map[string]any, key string, local []bool) {
	cloudRaw, _ := state[key].([]any)
	n := len(local)
	if len(cloudRaw) > n {
		n = len(cloudRaw)
	}
	out := make([]bool, n)
	for i := 0; i < n; i++ {
		cv := false
		if i < len(cloudRaw) {
			cv, _ = cloudRaw[i].(bool)
		}
		lv := i < len(local) && local[i]
		out[i] = cv || lv
	}
	state[key] = out
}

func afkCloudSanitizeOperatorTimers(state map[string]any, nowUnix int64) {
	if state == nil || nowUnix <= 0 {
		return
	}
	read := func(key string) []int64 {
		raw, _ := state[key].([]any)
		out := make([]int64, len(raw))
		for i, v := range raw {
			out[i] = int64(afkCloudAnyInt(v))
		}
		return out
	}
	starts := read("afk_operator_work_started_unix")
	ends := read("afk_operator_work_ends_unix")
	cooldowns := read("afk_operator_cooldown_ends_unix")
	n := len(starts)
	if len(ends) > n {
		n = len(ends)
	}
	if len(cooldowns) > n {
		n = len(cooldowns)
	}
	if n == 0 {
		return
	}
	resize := func(v []int64) []int64 {
		if len(v) >= n {
			return v
		}
		out := make([]int64, n)
		copy(out, v)
		return out
	}
	starts, ends, cooldowns = resize(starts), resize(ends), resize(cooldowns)
	for i := 0; i < n; i++ {
		// An expired work timestamp from a cloud response must never resurrect a
		// job the live client already completed and award the same Service XP twice.
		if ends[i] > 0 && ends[i] <= nowUnix {
			starts[i], ends[i] = 0, 0
		}
		if cooldowns[i] > 0 && cooldowns[i] <= nowUnix {
			cooldowns[i] = 0
		}
	}
	state["afk_operator_work_started_unix"] = starts
	state["afk_operator_work_ends_unix"] = ends
	state["afk_operator_cooldown_ends_unix"] = cooldowns
}

// Progression purchases are irreversible. A cloud checkpoint can legitimately be
// older than the local save (for example immediately after upgrading and then
// reclaiming the device lease). Preserve the highest purchased progression here;
// spendable balances such as Starbits/Nav Data remain server-controlled elsewhere.
func afkCloudMergeMonotonicProgression(state map[string]any) {
	afkCloudKeepTrue(state, "afk_cursor_core_built", gameMeta.AFKCursorCoreBuilt)
	afkCloudKeepHigherInt(state, "afk_cursor_core_tier", gameMeta.AFKCursorCoreTier)
	afkCloudKeepHigherInt(state, "afk_cursor_processing_level", gameMeta.AFKCursorProcessingLevel)
	afkCloudKeepHigherInt(state, "afk_auto_cursors", gameMeta.AFKAutoCursors)
	afkCloudKeepTrue(state, "afk_section1_complete", gameMeta.AFKSection1Complete)
	afkCloudKeepTrue(state, "afk_section2_complete", gameMeta.AFKSection2Complete)
	afkCloudKeepTrue(state, "afk_section3_complete", gameMeta.AFKSection3Complete)
	afkCloudKeepTrue(state, "afk_section4_complete", gameMeta.AFKSection4Complete)

	afkCloudKeepTrue(state, "afk_scout_ship_unlocked", gameMeta.AFKScoutShipUnlocked)
	afkCloudKeepHigherInt(state, "afk_scout_ship_tier", gameMeta.AFKScoutShipTier)
	afkCloudKeepHigherInt(state, "afk_expedition_speed_level", gameMeta.AFKExpeditionSpeedLevel)
	afkCloudKeepHigherInt(state, "afk_capacity_level", gameMeta.AFKCapacityLevel)
	afkCloudKeepHigherInt(state, "afk_cache_chance_level", gameMeta.AFKCacheChanceLevel)
	afkCloudKeepHigherInt(state, "afk_expeditions_completed", gameMeta.AFKExpeditionsCompleted)
	afkCloudKeepHigherInt(state, "afk_expedition_serial", gameMeta.AFKExpeditionSerial)
	afkCloudKeepTrue(state, "afk_section5_complete", gameMeta.AFKSection5Complete)

	afkCloudKeepTrue(state, "afk_tech_lab_unlocked", gameMeta.AFKTechLabUnlocked)
	afkCloudKeepHigherSlice(state, "afk_research_levels", gameMeta.AFKResearchLevels)
	afkCloudKeepHigherInt(state, "afk_research_purchases", gameMeta.AFKResearchPurchases)
	// v443: earned Pilot Skill Points are derived from current milestone state.
	// Do not preserve an inflated legacy earned pool from older balance versions.
	afkCloudKeepHigherInt(state, "afk_talent_points_spent", gameMeta.AFKTalentPointsSpent)
	afkCloudKeepHigherSlice(state, "afk_talents_unlocked", gameMeta.AFKTalentsUnlocked)
	afkCloudKeepTrue(state, "afk_section6_complete", gameMeta.AFKSection6Complete)

	afkCloudKeepTrueSlice(state, "afk_operators_recruited", gameMeta.AFKOperatorsRecruited)
	afkCloudKeepHigherInt64Slice(state, "afk_operator_xp_seconds", gameMeta.AFKOperatorXPSeconds)
	afkCloudKeepHigherSlice(state, "afk_operator_levels", gameMeta.AFKOperatorLevels)
	afkCloudKeepHigherInt(state, "afk_operators_recruited_count", gameMeta.AFKOperatorsRecruitedCount)
	afkCloudKeepTrue(state, "afk_section7_complete", gameMeta.AFKSection7Complete)

	afkCloudKeepTrueSlice(state, "afk_equipment_crafted", gameMeta.AFKEquipmentCrafted)
	afkCloudKeepHigherInt(state, "afk_equipment_sets_complete", gameMeta.AFKEquipmentSetsComplete)
	afkCloudKeepHigherInt(state, "afk_craft_components_found", gameMeta.AFKCraftComponentsFound)
	afkCloudKeepTrue(state, "afk_equipment_all_sets_reward", gameMeta.AFKEquipmentAllSetsReward)
	afkCloudKeepTrue(state, "afk_section8_complete", gameMeta.AFKSection8Complete)

	afkCloudKeepTrue(state, "afk_drone_bay_built", gameMeta.AFKDroneBayBuilt)
	afkCloudKeepHigherInt(state, "afk_drone_bay_tier", gameMeta.AFKDroneBayTier)
	afkCloudKeepHigherSlice(state, "afk_drone_upgrade_levels", gameMeta.AFKDroneUpgradeLevels)
	afkCloudKeepHigherInt(state, "afk_drones_deployed", gameMeta.AFKDronesDeployed)
	afkCloudKeepTrue(state, "afk_section9_complete", gameMeta.AFKSection9Complete)

	afkCloudKeepTrue(state, "afk_orbital_extractor_built", gameMeta.AFKOrbitalExtractorBuilt)
	afkCloudKeepHigherInt(state, "afk_orbital_extractor_tier", gameMeta.AFKOrbitalExtractorTier)
	afkCloudKeepHigherSlice(state, "afk_orbital_upgrade_levels", gameMeta.AFKOrbitalUpgradeLevels)
	afkCloudKeepTrue(state, "afk_section10_complete", gameMeta.AFKSection10Complete)
	afkCloudKeepHigherInt(state, "afk_prestige_rank", gameMeta.AFKPrestigeRank)
	afkCloudKeepHigherInt(state, "afk_prestige_purchases", gameMeta.AFKPrestigePurchases)
	afkCloudKeepTrue(state, "afk_section11_complete", gameMeta.AFKSection11Complete)
	afkCloudKeepTrue(state, "afk_section12_complete", gameMeta.AFKSection12Complete)
	afkCloudKeepHigherInt(state, "afk_station_hp_bonus", gameMeta.AFKStationHPBonus)
	// Legacy alias retained for v415-era backends/clients.
	afkCloudKeepHigherInt(state, "afk_survival_extra_lives", gameMeta.AFKStationHPBonus)
}

func afkApplyCloudState(state map[string]any, resp afkCloudResponse) {
	if state == nil {
		state = map[string]any{}
	}
	// v423: Starbits are no longer part of generic AFK state reconciliation.
	// The dedicated Starbit Bank ledger is the sole currency authority. Remove
	// every legacy currency/accounting key before merging progression state so an
	// old afk-sync response can never overwrite the Bank-owned visible balance.
	delete(state, "afk_starbits")
	delete(state, "afk_lifetime_starbits")
	delete(state, "afk_offline_pending_starbits")
	delete(state, "afk_offline_pending_away_seconds")
	delete(state, "afk_offline_pending_paid_seconds")
	afkCloudMergeMonotonicProgression(state)
	afkCloudSanitizeOperatorTimers(state, time.Now().Unix())

	starbits := gameMeta.AFKStarbits
	raw, _ := json.Marshal(gameMeta)
	all := map[string]any{}
	if json.Unmarshal(raw, &all) != nil {
		return
	}
	for k, v := range state {
		if strings.HasPrefix(k, "afk_") {
			all[k] = v
		}
	}
	// Belt-and-braces: even if a future cloud state adds these keys again, the
	// generic progression merge cannot alter Starbits.
	all["afk_starbits"] = starbits
	merged, _ := json.Marshal(all)
	var next GameMeta
	if json.Unmarshal(merged, &next) != nil {
		return
	}
	gameMeta = next
	normalizeGameMeta()
	afkRefreshTalentPointAwards()
	saveGameMeta()
}

func afkCloudPost(action string, includeState bool) (afkCloudResponse, error) {
	tok, err := validAuthAccessToken()
	if err != nil {
		return afkCloudResponse{}, err
	}
	deviceID := afkEnsureDeviceID()

	afkCloudMu.Lock()
	sessionToken := afkCloudSessionToken
	afkCloudMu.Unlock()

	requestStarbits := int64(-1)
	payload := map[string]any{
		"action":        action,
		"device_id":     deviceID,
		"session_token": sessionToken,
	}
	if includeState {
		state := afkSnapshotMap()
		delete(state, "afk_starbits")
		delete(state, "afk_lifetime_starbits")
		delete(state, "afk_offline_pending_starbits")
		delete(state, "afk_offline_pending_away_seconds")
		delete(state, "afk_offline_pending_paid_seconds")
		payload["state"] = state
		// Explicitly zero the deprecated AFK production fields. The v423 Bank is
		// responsible for currency and Starbase no longer accrues offline earnings.
		payload["production_per_sec_milli"] = 0
		payload["afk_capacity_seconds"] = afkCapacitySeconds()
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+afkCloudEndpoint, bytes.NewReader(body))
	if err != nil {
		return afkCloudResponse{}, err
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: afkCloudRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return afkCloudResponse{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var out afkCloudResponse
	_ = json.Unmarshal(raw, &out)
	out.RequestStarbits = requestStarbits
	out.RequestAction = action
	if resp.StatusCode == http.StatusConflict || out.SessionLost {
		out.SessionLost = true
		if out.Message == "" {
			out.Message = "This Discord account was opened on another device."
		}
		return out, fmt.Errorf("session lost")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !out.OK {
		if out.Message == "" {
			out.Message = fmt.Sprintf("AFK cloud sync failed (%d)", resp.StatusCode)
		}
		return out, fmt.Errorf("%s", out.Message)
	}
	return out, nil
}

func afkCloudClaimDeviceAsync() {
	afkCloudMu.Lock()
	if afkCloudSyncInFlight {
		afkCloudMu.Unlock()
		return
	}
	afkCloudSyncInFlight = true
	afkCloudStatus = "CONNECTING AFK CLOUD..."
	afkCloudMu.Unlock()
	go func() {
		defer func() { afkCloudMu.Lock(); afkCloudSyncInFlight = false; afkCloudMu.Unlock() }()
		out, err := afkCloudPost("claim_device", true)
		if err != nil {
			afkCloudMu.Lock()
			afkCloudStatus = "AFK CLOUD UNAVAILABLE"
			afkCloudMu.Unlock()
			return
		}
		afkCloudMu.Lock()
		afkCloudSessionToken = out.SessionToken
		afkCloudLastVerified = time.Now()
		afkCloudLastSync = time.Now()
		afkCloudManualGainSinceSync = 0
		afkCloudStatus = "AFK CLOUD SYNCED"
		afkCloudMu.Unlock()
		postMainThreadTask(func() {
			afkApplyCloudState(out.State, out)
			afkBankClaimAsync()
			if mainHwnd != 0 {
				invalidateRect.Call(mainHwnd, 0, 0)
			}
		})
	}()
}

func afkCloudSyncAsync(now time.Time) {
	authMu.Lock()
	connected := discordConnected
	authMu.Unlock()
	if !connected {
		return
	}
	afkCloudMu.Lock()
	if afkCloudSessionToken == "" || afkCloudSyncInFlight || (!afkCloudLastSync.IsZero() && now.Sub(afkCloudLastSync) < afkCloudSyncInterval) {
		afkCloudMu.Unlock()
		return
	}
	afkCloudSyncInFlight = true
	afkCloudMu.Unlock()
	go func() {
		out, err := afkCloudPost("sync", true)
		if err != nil && out.SessionLost {
			afkCloudMu.Lock()
			afkCloudStatus = "SIGNED OUT // ACCOUNT ACTIVE ON ANOTHER DEVICE"
			afkCloudSessionToken = ""
			afkCloudSyncInFlight = false
			afkCloudMu.Unlock()
			postMainThreadTask(func() {
				clearAuthSession()
				if mainHwnd != 0 {
					invalidateRect.Call(mainHwnd, 0, 0)
				}
			})
			return
		}
		afkCloudMu.Lock()
		afkCloudSyncInFlight = false
		if err != nil {
			afkCloudStatus = "AFK CLOUD RECONNECTING..."
			afkCloudMu.Unlock()
			return
		}
		afkCloudLastVerified = time.Now()
		afkCloudLastSync = time.Now()
		afkCloudManualGainSinceSync = 0
		afkCloudStatus = "AFK CLOUD SYNCED"
		afkCloudMu.Unlock()
		postMainThreadTask(func() {
			afkApplyCloudState(out.State, out)
			if mainHwnd != 0 {
				invalidateRect.Call(mainHwnd, 0, 0)
			}
		})
	}()
}

func afkCloudTick(now time.Time) {
	afkCloudSyncAsync(now)
	afkBankTick(now)
}

// Progression upgrades should be pushed on the next heartbeat instead of waiting
// for the normal ten-second cadence. This does not bypass the single-device lease.
func afkCloudMarkProgressDirty() {
	afkCloudMu.Lock()
	afkCloudLastSync = time.Time{}
	afkCloudMu.Unlock()
}

func afkCloudEconomyAllowed(now time.Time) bool {
	// v410: Starbase is cloud-only. Logged-out/local-only economy progression is no
	// longer permitted, so every Starbit and progression action belongs to a Discord
	// account and the single-device lease can remain authoritative.
	authMu.Lock()
	connected := discordConnected
	authMu.Unlock()
	if !connected {
		return false
	}
	afkCloudMu.Lock()
	token := afkCloudSessionToken
	afkCloudMu.Unlock()
	afkBankMu.Lock()
	bankReady := afkBankReady
	afkBankMu.Unlock()
	return token != "" && bankReady
}

func afkCloudStarbaseAccess(now time.Time) (bool, string) {
	authMu.Lock()
	connected := discordConnected
	authMu.Unlock()
	if !connected {
		return false, "STARBASE LOCKED // LOG IN WITH DISCORD"
	}
	afkCloudMu.Lock()
	token := afkCloudSessionToken
	verified := afkCloudLastVerified
	inFlight := afkCloudSyncInFlight
	afkCloudMu.Unlock()
	if token == "" {
		if !inFlight {
			afkCloudClaimDeviceAsync()
		}
		return false, "STARBASE CLOUD SYNCING // TRY AGAIN IN A MOMENT"
	}
	// v430: transient progression-sync failures must not invalidate a valid
	// Starbit Bank/device lease. Keep Starbase usable and retry progression
	// sync in the background; only an explicit session_lost response signs out.
	_ = verified
	afkBankMu.Lock()
	bankReady := afkBankReady
	bankInFlight := afkBankInFlight
	afkBankMu.Unlock()
	if !bankReady {
		if !bankInFlight {
			afkBankClaimAsync()
		}
		return false, "OPENING STARBIT BANK // TRY AGAIN IN A MOMENT"
	}
	return true, ""
}

func afkCloudRecordManualGain(v int64) {
	if v <= 0 {
		return
	}
	authMu.Lock()
	connected := discordConnected
	authMu.Unlock()
	if !connected {
		return
	}
	afkCloudMu.Lock()
	afkCloudManualGainSinceSync = saturatingAdd(afkCloudManualGainSinceSync, v)
	afkCloudMu.Unlock()
}

func afkCloudResetLease() {
	afkCloudMu.Lock()
	afkCloudSessionToken = ""
	afkCloudLastVerified = time.Time{}
	afkCloudLastSync = time.Time{}
	afkCloudManualGainSinceSync = 0
	afkCloudStatus = ""
	afkCloudMu.Unlock()
	afkBankReset()
}
