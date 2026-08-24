package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Local analytics are deliberately privacy-minimal: no display name, Discord
// identifier, email, IP, free-form chat, or raw cursor path is written. The
// optional network batch is OFF by default and only runs after the player opts
// in from Settings.
type gameplayAnalyticsEvent struct {
	At     string         `json:"at"`
	Event  string         `json:"event"`
	Mode   string         `json:"mode,omitempty"`
	Fields map[string]any `json:"fields,omitempty"`
}

var analyticsRuntime = struct {
	sync.Mutex
	started bool
	ch      chan gameplayAnalyticsEvent
	batch   []gameplayAnalyticsEvent
}{ch: make(chan gameplayAnalyticsEvent, 512)}

func analyticsModeName() string {
	if survivalActive() {
		return "SURVIVAL"
	}
	if enduranceActive() {
		return "ENDURANCE"
	}
	return "PRECISION"
}

func analyticsDifficultyName() string {
	if survivalActive() {
		return "SURVIVAL"
	}
	if enduranceActive() {
		return "ENDURANCE"
	}
	name, _ := difficultyDisplay()
	return strings.ToUpper(strings.TrimSpace(name))
}

func analyticsFailureCategory(reason string) string {
	r := strings.ToLower(reason)
	switch {
	case strings.Contains(r, "boundary") || strings.Contains(r, "rail"):
		return "BOUNDARY"
	case strings.Contains(r, "meteor") || strings.Contains(r, "rock"):
		return "METEOR"
	case strings.Contains(r, "laser") || strings.Contains(r, "ufo") || strings.Contains(r, "alien"):
		return "ALIEN"
	case strings.Contains(r, "time") || strings.Contains(r, "slow"):
		return "TIME"
	case strings.Contains(r, "target") || strings.Contains(r, "core") || strings.Contains(r, "signal"):
		return "OBJECTIVE"
	case strings.Contains(r, "serpent") || strings.Contains(r, "boss") || strings.Contains(r, "array"):
		return "BOSS"
	default:
		return "OTHER"
	}
}

func initGameplayAnalytics() {
	analyticsRuntime.Lock()
	if analyticsRuntime.started {
		analyticsRuntime.Unlock()
		return
	}
	analyticsRuntime.started = true
	analyticsRuntime.Unlock()

	today := time.Now().Format("2006-01-02")
	if gameMeta.FirstPlayedDate == "" {
		gameMeta.FirstPlayedDate = today
	}
	gameMeta.LastPlayedDate = today
	seen := false
	for _, d := range gameMeta.ActivePlayDates {
		if d == today {
			seen = true
			break
		}
	}
	if !seen {
		gameMeta.ActivePlayDates = append(gameMeta.ActivePlayDates, today)
		if len(gameMeta.ActivePlayDates) > 120 {
			gameMeta.ActivePlayDates = append([]string(nil), gameMeta.ActivePlayDates[len(gameMeta.ActivePlayDates)-120:]...)
		}
	}
	saveGameMeta()
	go analyticsWriterLoop()
	analyticsEvent("session_started", map[string]any{"session": gameMeta.Sessions, "active_days": len(gameMeta.ActivePlayDates)})
}

func analyticsEvent(name string, fields map[string]any) {
	if strings.TrimSpace(name) == "" {
		return
	}
	e := gameplayAnalyticsEvent{At: time.Now().UTC().Format(time.RFC3339Nano), Event: name, Mode: analyticsModeName(), Fields: fields}
	select {
	case analyticsRuntime.ch <- e:
	default:
	}
}

func analyticsLogPath() string {
	root := logRoot
	if root == "" {
		root = filepath.Join(gameRoot, "logs")
	}
	_ = os.MkdirAll(root, 0755)
	return filepath.Join(root, "analytics_local.jsonl")
}

func rotateAnalyticsLog(path string) {
	st, err := os.Stat(path)
	if err != nil || st.Size() < 3*1024*1024 {
		return
	}
	_ = os.Remove(path + ".1")
	_ = os.Rename(path, path+".1")
}

func analyticsWriterLoop() {
	path := analyticsLogPath()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case e := <-analyticsRuntime.ch:
			rotateAnalyticsLog(path)
			if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
				w := bufio.NewWriterSize(f, 4096)
				if b, err := json.Marshal(e); err == nil {
					_, _ = w.Write(append(b, '\n'))
				}
				_ = w.Flush()
				_ = f.Close()
			}
			analyticsRuntime.Lock()
			analyticsRuntime.batch = append(analyticsRuntime.batch, e)
			if len(analyticsRuntime.batch) > 100 {
				analyticsRuntime.batch = append([]gameplayAnalyticsEvent(nil), analyticsRuntime.batch[len(analyticsRuntime.batch)-100:]...)
			}
			analyticsRuntime.Unlock()
		case <-ticker.C:
			flushAnonymousAnalyticsBatch()
		}
	}
}

func flushAnonymousAnalyticsBatch() {
	if !gameMeta.ShareAnonymousAnalytics {
		return
	}
	token, err := validAuthAccessToken()
	if err != nil || token == "" {
		return
	}
	analyticsRuntime.Lock()
	if len(analyticsRuntime.batch) == 0 {
		analyticsRuntime.Unlock()
		return
	}
	batch := append([]gameplayAnalyticsEvent(nil), analyticsRuntime.batch...)
	analyticsRuntime.Unlock()
	payload, _ := json.Marshal(map[string]any{"events": batch})
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/functions/v1/analytics-batch", bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := authHTTPClient().Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		analyticsRuntime.Lock()
		if len(analyticsRuntime.batch) >= len(batch) {
			analyticsRuntime.batch = append([]gameplayAnalyticsEvent(nil), analyticsRuntime.batch[len(batch):]...)
		}
		analyticsRuntime.Unlock()
	}
}

func analyticsRetentionFlags() (d1, d7, d30 bool) {
	if gameMeta.FirstPlayedDate == "" {
		return
	}
	first, err := time.Parse("2006-01-02", gameMeta.FirstPlayedDate)
	if err != nil {
		return
	}
	for _, s := range gameMeta.ActivePlayDates {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			continue
		}
		delta := int(d.Sub(first).Hours() / 24)
		if delta >= 1 {
			d1 = true
		}
		if delta >= 7 {
			d7 = true
		}
		if delta >= 30 {
			d30 = true
		}
	}
	return
}
