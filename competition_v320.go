package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Top 20 is permanent prestige. Weekly is the normal rolling PB/clear board.
// Competition is the cumulative Precision Points race described in the in-game guide.
const (
	leaderboardScopeTop20 = iota
	leaderboardScopeWeekly
	leaderboardScopeCompetition
)

const (
	competitionViewCurrent = iota
	competitionViewPrevious
	competitionViewAllTime
)

type PrecisionCompetitionEntry struct {
	UserID            string
	Position          int
	Name              string
	NameColour        int
	Rank              string
	TotalPoints       int
	EasyPoints        int
	NormalPoints      int
	HardPoints        int
	InsanePoints      int
	EasyStreak        int
	NormalStreak      int
	HardStreak        int
	InsaneStreak      int
	EasyCompletions   int
	NormalCompletions int
	HardCompletions   int
	InsaneCompletions int
	EXPReward         int
	SpaceCoinsReward  int
}

type CompetitionAllTimeEntry struct {
	UserID      string
	Position    int
	Name        string
	NameColour  int
	Rank        string
	Wins        int
	Podiums     int
	Firsts      int
	Seconds     int
	Thirds      int
	LastWinWeek string
}

var (
	globalLeaderboardScope        = leaderboardScopeTop20
	globalLeaderboardWeekly       []GlobalLeaderboardEntry
	precisionCompetitionEntries   []PrecisionCompetitionEntry
	precisionCompetitionPrevious  []PrecisionCompetitionEntry
	precisionCompetitionAllTime   []CompetitionAllTimeEntry
	competitiveLeaderboardStatus  string
	competitiveLeaderboardLoading bool
	competitiveWeekKey            string
	competitionHistoryWeekKey     string
	competitiveResetAt            time.Time
	competitionGuideOpen          bool
	competitionView               = competitionViewCurrent
	competitionRewardNoticeAt     time.Time
	competitionRewardNoticeText   string
)

func leaderboardScopeName(scope int) string {
	switch scope {
	case leaderboardScopeWeekly:
		return "WEEKLY"
	case leaderboardScopeCompetition:
		return "COMPETITION"
	default:
		return "TOP 20"
	}
}

func competitionViewName(view int) string {
	switch view {
	case competitionViewPrevious:
		return "PREVIOUS WINNERS"
	case competitionViewAllTime:
		return "ALL-TIME WINNERS"
	default:
		return "THIS WEEK"
	}
}

func globalLeaderboardScopeRects(left, right, top int32, w, hgt int32) [3]RECT {
	gap := sx(8, w)
	wid := sx(116, w)
	hh := sy(20, hgt)
	if hh < 18 {
		hh = 18
	}
	// v446: all three competition controls are intentionally left aligned.
	x := left
	return [3]RECT{
		{x, top, x + wid, top + hh},
		{x + wid + gap, top, x + wid*2 + gap, top + hh},
		{x + (wid+gap)*2, top, x + wid*3 + gap*2, top + hh},
	}
}

func competitionGuideButtonRect(left, right, top int32, w, hgt int32) RECT {
	wid := sx(84, w)
	hh := sy(20, hgt)
	if hh < 18 {
		hh = 18
	}
	return RECT{right - wid, top, right, top + hh}
}

func competitionViewRects(left, right, top int32, w, hgt int32) [3]RECT {
	gap := sx(8, w)
	if gap < 4 {
		gap = 4
	}
	hh := sy(28, hgt)
	if hh < 23 {
		hh = 23
	}
	wid := (right - left - gap*2) / 3
	return [3]RECT{
		{left, top, left + wid, top + hh},
		{left + wid + gap, top, left + wid*2 + gap, top + hh},
		{left + (wid+gap)*2, top, right, top + hh},
	}
}

func drawCompetitionViewTabs(hdc uintptr, left, right, top int32, w, hgt int32) {
	if hudTinyFont == 0 {
		return
	}
	labels := []string{"THIS WEEK", "PREVIOUS WINNERS", "ALL-TIME WINS"}
	for i, r := range competitionViewRects(left, right, top, w, hgt) {
		face := rgb(13, 28, 53)
		edge := rgb(67, 97, 138)
		text := rgb(165, 190, 218)
		if i == competitionView {
			face = rgb(42, 27, 72)
			edge = rgb(192, 111, 255)
			text = rgb(244, 230, 255)
		}
		if pointInRect(cursorPos, r) {
			edge = blendColor(edge, rgb(255, 255, 255), .22)
		}
		drawBevelPanel(hdc, r, face, edge, rgb(3, 8, 20), 1)
		setTextColor.Call(hdc, text)
		centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, hudTinyFont, r, labels[i]), labels[i])
	}
}

func competitionViewAt(p FPoint, left, right, top int32, w, hgt int32) int {
	for i, r := range competitionViewRects(left, right, top, w, hgt) {
		if pointInRect(p, r) {
			return i
		}
	}
	return -1
}

func drawGlobalLeaderboardScopeTabs(hdc uintptr, left, right, top int32, w, hgt int32) {
	if hudTinyFont == 0 {
		return
	}
	rects := globalLeaderboardScopeRects(left, right, top, w, hgt)
	for i, r := range rects {
		face := rgb(5, 29, 56)
		edge := rgb(34, 107, 150)
		text := rgb(150, 188, 215)
		if i == globalLeaderboardScope {
			face = rgb(8, 58, 83)
			edge = rgb(42, 218, 255)
			text = rgb(235, 250, 255)
		}
		if pointInRect(cursorPos, r) {
			edge = blendColor(edge, rgb(255, 255, 255), .22)
		}
		drawBevelPanel(hdc, r, face, edge, rgb(1, 13, 31), 1)
		setTextColor.Call(hdc, text)
		centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, hudTinyFont, r, leaderboardScopeName(i)), leaderboardScopeName(i))
	}
	if globalLeaderboardScope == leaderboardScopeCompetition {
		r := competitionGuideButtonRect(left, right, top, w, hgt)
		face := rgb(31, 24, 61)
		edge := rgb(184, 104, 255)
		text := rgb(239, 225, 255)
		if competitionGuideOpen {
			face = rgb(60, 31, 92)
			edge = rgb(229, 184, 255)
		}
		if pointInRect(cursorPos, r) {
			edge = blendColor(edge, rgb(255, 255, 255), .22)
		}
		drawBevelPanel(hdc, r, face, edge, rgb(8, 5, 18), 1)
		setTextColor.Call(hdc, text)
		centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, hudTinyFont, r, "GUIDE"), "GUIDE")
	}
}

func competitionAsGlobalEntries() []GlobalLeaderboardEntry {
	switch competitionView {
	case competitionViewPrevious:
		out := make([]GlobalLeaderboardEntry, 0, len(precisionCompetitionPrevious))
		for _, e := range precisionCompetitionPrevious {
			out = append(out, GlobalLeaderboardEntry{UserID: e.UserID, Position: e.Position, Name: e.Name, NameColour: e.NameColour, Rank: e.Rank, Score: e.TotalPoints, Difficulty: "COMPETITION_PREVIOUS"})
		}
		return out
	case competitionViewAllTime:
		out := make([]GlobalLeaderboardEntry, 0, len(precisionCompetitionAllTime))
		for _, e := range precisionCompetitionAllTime {
			out = append(out, GlobalLeaderboardEntry{UserID: e.UserID, Position: e.Position, Name: e.Name, NameColour: e.NameColour, Rank: e.Rank, Score: e.Wins, Streak: e.Podiums, Difficulty: "COMPETITION_ALLTIME"})
		}
		return out
	default:
		out := make([]GlobalLeaderboardEntry, 0, len(precisionCompetitionEntries))
		for _, e := range precisionCompetitionEntries {
			out = append(out, GlobalLeaderboardEntry{UserID: e.UserID, Position: e.Position, Name: e.Name, NameColour: e.NameColour, Rank: e.Rank, Score: e.TotalPoints, Difficulty: "COMPETITION"})
		}
		return out
	}
}

func scopedGlobalEntries(defaultEntries []GlobalLeaderboardEntry) []GlobalLeaderboardEntry {
	globalMu.Lock()
	defer globalMu.Unlock()
	var src []GlobalLeaderboardEntry
	switch globalLeaderboardScope {
	case leaderboardScopeWeekly:
		src = globalLeaderboardWeekly
	case leaderboardScopeCompetition:
		src = competitionAsGlobalEntries()
	default:
		src = defaultEntries
	}
	out := make([]GlobalLeaderboardEntry, len(src))
	copy(out, src)
	return out
}

func competitionAnyInt(m map[string]any, key string) int {
	v := m[key]
	switch x := v.(type) {
	case float64:
		return int(math.Round(x))
	case int:
		return x
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	}
	return 0
}
func competitionAnyString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			return s
		}
	}
	return ""
}

func updateCompetitiveResetMetadata(weekKey, resetText string) {
	competitiveWeekKey = weekKey
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(resetText)); err == nil {
		competitiveResetAt = t
	}
}

func fallbackCompetitionResetAt(now time.Time) time.Time {
	loc := time.FixedZone("ICT", 7*60*60)
	local := now.In(loc)
	// Competition resets after Sunday: Monday 00:00 ICT.
	// Go weekday Sunday == 0; normalize Monday to zero days since week start.
	daysSinceMonday := (int(local.Weekday()) + 6) % 7
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysSinceMonday)
	return start.AddDate(0, 0, 7)
}
func competitionResetTime() time.Time {
	if !competitiveResetAt.IsZero() {
		return competitiveResetAt
	}
	return fallbackCompetitionResetAt(time.Now())
}
func competitionResetCountdownText() string {
	d := time.Until(competitionResetTime())
	if d < 0 {
		d = 0
	}
	secs := int(d.Seconds())
	days := secs / 86400
	secs %= 86400
	hours := secs / 3600
	secs %= 3600
	mins := secs / 60
	sec := secs % 60
	if days > 0 {
		return fmt.Sprintf("RESET %dd %02dh %02dm %02ds", days, hours, mins, sec)
	}
	return fmt.Sprintf("RESET %02dh %02dm %02ds", hours, mins, sec)
}

func parsePrecisionCompetitionEntries(entries []map[string]any) []PrecisionCompetitionEntry {
	parsed := make([]PrecisionCompetitionEntry, 0, len(entries))
	for i, m := range entries {
		name := cleanDiscordDisplayName(competitionAnyString(m, "display_name", "name"))
		if strings.TrimSpace(name) == "" {
			continue
		}
		pos := competitionAnyInt(m, "position")
		if pos <= 0 {
			pos = i + 1
		}
		parsed = append(parsed, PrecisionCompetitionEntry{
			UserID: competitionAnyString(m, "user_id"), Position: pos, Name: name,
			NameColour: competitionAnyInt(m, "selected_name_colour"), Rank: competitionAnyString(m, "exp_rank"),
			TotalPoints: competitionAnyInt(m, "total_points"), EasyPoints: competitionAnyInt(m, "easy_points"), NormalPoints: competitionAnyInt(m, "normal_points"), HardPoints: competitionAnyInt(m, "hard_points"), InsanePoints: competitionAnyInt(m, "insane_points"),
			EasyStreak: competitionAnyInt(m, "easy_streak"), NormalStreak: competitionAnyInt(m, "normal_streak"), HardStreak: competitionAnyInt(m, "hard_streak"), InsaneStreak: competitionAnyInt(m, "insane_streak"),
			EasyCompletions: competitionAnyInt(m, "easy_completions"), NormalCompletions: competitionAnyInt(m, "normal_completions"), HardCompletions: competitionAnyInt(m, "hard_completions"), InsaneCompletions: competitionAnyInt(m, "insane_completions"),
			EXPReward: competitionAnyInt(m, "exp_reward"), SpaceCoinsReward: competitionAnyInt(m, "spacecoins_reward"),
		})
	}
	return parsed
}

func fetchCompetitiveHub() {
	scope := globalLeaderboardScope
	view := competitionView
	if scope == leaderboardScopeTop20 {
		return
	}
	diff := leaderboardFilterName(globalLeaderboardFilter)
	token, err := validAuthAccessToken()
	if err != nil || token == "" {
		globalMu.Lock()
		competitiveLeaderboardLoading = false
		competitiveLeaderboardStatus = "LOGIN REQUIRED FOR " + leaderboardScopeName(scope)
		globalMu.Unlock()
		if mainHwnd != 0 {
			invalidateRect.Call(mainHwnd, 0, 0)
		}
		return
	}
	globalMu.Lock()
	competitiveLeaderboardLoading = true
	competitiveLeaderboardStatus = "LOADING " + leaderboardScopeName(scope) + "..."
	globalMu.Unlock()
	q := url.Values{}
	if scope == leaderboardScopeCompetition {
		q.Set("scope", "competition")
		switch view {
		case competitionViewPrevious:
			q.Set("view", "previous")
		case competitionViewAllTime:
			q.Set("view", "alltime")
		default:
			q.Set("view", "current")
		}
	} else {
		q.Set("scope", "weekly")
		q.Set("difficulty", diff)
	}
	endpoint := supabaseProjectURL + "/functions/v1/competitive-hub?" + q.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := authHTTPClient().Do(req)
	if err != nil {
		globalMu.Lock()
		competitiveLeaderboardLoading = false
		competitiveLeaderboardStatus = "COMPETITIVE SERVER UNAVAILABLE"
		globalMu.Unlock()
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		globalMu.Lock()
		competitiveLeaderboardLoading = false
		competitiveLeaderboardStatus = fmt.Sprintf("COMPETITIVE SERVER %d", resp.StatusCode)
		globalMu.Unlock()
		return
	}
	var env struct {
		Entries  []map[string]any `json:"entries"`
		WeekKey  string           `json:"week_key"`
		ResetsAt string           `json:"resets_at"`
	}
	if json.Unmarshal(body, &env) != nil {
		globalMu.Lock()
		competitiveLeaderboardLoading = false
		competitiveLeaderboardStatus = "COMPETITIVE DATA FORMAT ERROR"
		globalMu.Unlock()
		return
	}
	globalMu.Lock()
	updateCompetitiveResetMetadata(env.WeekKey, env.ResetsAt)
	if scope == leaderboardScopeCompetition {
		competitionHistoryWeekKey = env.WeekKey
		if view == competitionViewAllTime {
			parsed := make([]CompetitionAllTimeEntry, 0, len(env.Entries))
			for i, m := range env.Entries {
				name := cleanDiscordDisplayName(competitionAnyString(m, "display_name", "name"))
				if strings.TrimSpace(name) == "" {
					continue
				}
				pos := competitionAnyInt(m, "position")
				if pos <= 0 {
					pos = i + 1
				}
				parsed = append(parsed, CompetitionAllTimeEntry{UserID: competitionAnyString(m, "user_id"), Position: pos, Name: name, NameColour: competitionAnyInt(m, "selected_name_colour"), Rank: competitionAnyString(m, "exp_rank"), Wins: competitionAnyInt(m, "wins"), Podiums: competitionAnyInt(m, "podiums"), Firsts: competitionAnyInt(m, "firsts"), Seconds: competitionAnyInt(m, "seconds"), Thirds: competitionAnyInt(m, "thirds"), LastWinWeek: competitionAnyString(m, "last_win_week")})
			}
			precisionCompetitionAllTime = parsed
			if len(parsed) == 0 {
				competitiveLeaderboardStatus = "NO WEEKLY WINNERS YET"
			} else {
				competitiveLeaderboardStatus = ""
			}
		} else {
			parsed := parsePrecisionCompetitionEntries(env.Entries)
			if view == competitionViewPrevious {
				precisionCompetitionPrevious = parsed
				if len(parsed) == 0 {
					competitiveLeaderboardStatus = "NO PREVIOUS COMPETITION WINNERS YET"
				} else {
					competitiveLeaderboardStatus = ""
				}
			} else {
				precisionCompetitionEntries = parsed
				if len(parsed) == 0 {
					competitiveLeaderboardStatus = "NO COMPETITION SCORES YET"
				} else {
					competitiveLeaderboardStatus = ""
				}
			}
		}
	} else {
		parsed := make([]GlobalLeaderboardEntry, 0, len(env.Entries))
		for i, m := range env.Entries {
			e := globalEntryFromMap(m, i)
			if strings.TrimSpace(e.Name) != "" {
				parsed = append(parsed, e)
			}
		}
		globalLeaderboardWeekly = parsed
		if len(parsed) == 0 {
			competitiveLeaderboardStatus = "NO WEEKLY SCORES YET"
		} else {
			competitiveLeaderboardStatus = ""
		}
	}
	competitiveLeaderboardLoading = false
	globalMu.Unlock()
	go autoSyncPrecisionCompetitionRewards()
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

func selectGlobalLeaderboardScope(scope int) {
	if scope < leaderboardScopeTop20 || scope > leaderboardScopeCompetition {
		return
	}
	if globalLeaderboardScope == scope {
		return
	}
	globalLeaderboardScope = scope
	selectedGlobalPlayer = -1
	globalLeaderboardScroll = 0
	competitionGuideOpen = false
	if scope == leaderboardScopeCompetition {
		competitionView = competitionViewCurrent
	}
	if scope != leaderboardScopeTop20 {
		go fetchCompetitiveHub()
	}
}

func selectCompetitionView(view int) {
	if view < competitionViewCurrent || view > competitionViewAllTime || view == competitionView {
		return
	}
	competitionView = view
	competitionGuideOpen = false
	selectedGlobalPlayer = -1
	globalLeaderboardScroll = 0
	go fetchCompetitiveHub()
}

func reportPrecisionCompetitionFailure(difficulty string) {
	d := canonicalOnlineDifficulty(difficulty)
	if d == "" || d == "ENDURANCE" {
		return
	}
	token, err := validAuthAccessToken()
	if err != nil || token == "" {
		return
	}
	payload, _ := json.Marshal(map[string]any{"action": "precision_failure", "difficulty": d})
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/functions/v1/competitive-hub", bytes.NewReader(payload))
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
}

// Rewards require no player action. The server credits EXP during weekly finalization;
// this sync only pulls any pending Space Coins into the local wallet and reconciles EXP.
func autoSyncPrecisionCompetitionRewards() {
	token, err := validAuthAccessToken()
	if err != nil || token == "" {
		return
	}
	payload, _ := json.Marshal(map[string]any{"action": "sync_precision_rewards"})
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/functions/v1/competitive-hub", bytes.NewReader(payload))
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
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	var out struct {
		Awarded          bool `json:"awarded"`
		AwardCount       int  `json:"award_count"`
		Placement        int  `json:"placement"`
		EXPReward        int  `json:"exp_reward"`
		SpaceCoinsReward int  `json:"spacecoins_reward"`
		GlobalEXP        int  `json:"global_exp"`
	}
	if json.Unmarshal(body, &out) != nil {
		return
	}
	changed := false
	if out.GlobalEXP > playerProgress.EXP {
		playerProgress.EXP = out.GlobalEXP
		changed = true
	}
	if out.SpaceCoinsReward > 0 {
		gameMeta.SpaceCoins += out.SpaceCoinsReward
		changed = true
	}
	if changed {
		savePlayerProgress()
		saveGameMeta()
	}
	if out.Awarded {
		if out.AwardCount <= 1 && out.Placement > 0 {
			competitionRewardNoticeText = fmt.Sprintf("WEEKLY COMPETITION #%d  +%d EXP  +%d SPACECOINS", out.Placement, out.EXPReward, out.SpaceCoinsReward)
		} else {
			competitionRewardNoticeText = fmt.Sprintf("WEEKLY COMPETITION REWARDS  +%d SPACECOINS  // EXP CREDITED", out.SpaceCoinsReward)
		}
		competitionRewardNoticeAt = time.Now()
	}
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

// Legacy hooks retained for older callers; reward delivery is still automatic.
func autoClaimPrecisionCompetitionReward() { autoSyncPrecisionCompetitionRewards() }
func claimCompetitiveReward()              { go autoSyncPrecisionCompetitionRewards() }
