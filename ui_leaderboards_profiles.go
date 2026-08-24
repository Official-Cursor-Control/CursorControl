//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"
	"unsafe"
)

var (
	localLeaderboardScroll  int
	globalLeaderboardScroll int
)

func clampLeaderboardScroll(offset, total, visible int) int {
	if visible < 1 {
		visible = 1
	}
	max := total - visible
	if max < 0 {
		max = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > max {
		offset = max
	}
	return offset
}

func leaderboardScrollbarRects(track RECT, total, visible, offset int) (RECT, RECT) {
	if track.Bottom <= track.Top {
		track.Bottom = track.Top + 1
	}
	height := track.Bottom - track.Top
	thumbH := height
	if total > visible && total > 0 {
		thumbH = int32(float64(height) * float64(visible) / float64(total))
		if thumbH < 26 {
			thumbH = 26
		}
		if thumbH > height {
			thumbH = height
		}
	}
	maxOffset := total - visible
	thumbTop := track.Top
	if maxOffset > 0 && height > thumbH {
		offset = clampLeaderboardScroll(offset, total, visible)
		thumbTop += int32(float64(height-thumbH) * float64(offset) / float64(maxOffset))
	}
	return track, RECT{track.Left, thumbTop, track.Right, thumbTop + thumbH}
}

func drawLeaderboardScrollbar(hdc uintptr, track RECT, total, visible, offset int, accent uintptr) {
	track, thumb := leaderboardScrollbarRects(track, total, visible, offset)
	fillSolidRect(hdc, track, rgb(2, 15, 34))
	drawOutlineRect(hdc, track, rgb(28, 79, 111), 1)
	fillSolidRect(hdc, thumb, blendColor(accent, rgb(12, 25, 42), 0.18))
	drawOutlineRect(hdc, thumb, accent, 1)
}

func localLeaderboardVisibleRows(w, hgt int32) int {
	ar := arenaRect(w, hgt)
	top := ar.Top + sy(32, hgt)
	bottom := ar.Bottom - sy(26, hgt)
	filterTop := top + sy(51, hgt)
	tableTop := filterTop + sy(42, hgt)
	rowH := sy(38, hgt)
	if rowH < 27 {
		rowH = 27
	}
	available := bottom - sy(35, hgt) - (tableTop + rowH)
	v := int(available / rowH)
	if v < 1 {
		v = 1
	}
	if v > 10 {
		v = 10
	}
	return v
}

func globalLeaderboardVisibleRows(w, hgt int32) int {
	_, _, _, bottom, _, _, tableTop, rowH := globalLeaderboardGeometry(w, hgt)
	pinnedHeaderTop := bottom - sy(66, hgt) - rowH
	available := pinnedHeaderTop - sy(8, hgt) - (tableTop + rowH)
	v := int(available / rowH)
	if v < 1 {
		v = 1
	}
	if v > 20 {
		v = 20
	}
	return v
}

func scrollLocalLeaderboard(delta int32, w, hgt int32) {
	entries := filteredLocalLeaderboard()
	visible := localLeaderboardVisibleRows(w, hgt)
	if delta < 0 {
		localLeaderboardScroll++
	} else if delta > 0 {
		localLeaderboardScroll--
	}
	localLeaderboardScroll = clampLeaderboardScroll(localLeaderboardScroll, len(entries), visible)
}

func scrollGlobalLeaderboard(delta int32, w, hgt int32) {
	entries := globalFilteredEntries()
	visible := globalLeaderboardVisibleRows(w, hgt)
	if delta < 0 {
		globalLeaderboardScroll++
	} else if delta > 0 {
		globalLeaderboardScroll--
	}
	globalLeaderboardScroll = clampLeaderboardScroll(globalLeaderboardScroll, len(entries), visible)
}

func handleLeaderboardScrollbarClick(p FPoint, w, hgt int32, global bool) bool {
	if global {
		_, _, _, bottom, _, tableRight, tableTop, rowH := globalLeaderboardGeometry(w, hgt)
		visible := globalLeaderboardVisibleRows(w, hgt)
		total := len(globalFilteredEntries())
		track := RECT{tableRight - sx(11, w), tableTop + rowH + 1, tableRight - 1, bottom - sy(66, hgt) - rowH - sy(8, hgt)}
		_, thumb := leaderboardScrollbarRects(track, total, visible, globalLeaderboardScroll)
		if !pointInRect(p, track) {
			return false
		}
		if p.Y < float64(thumb.Top) {
			globalLeaderboardScroll -= visible
		} else if p.Y > float64(thumb.Bottom) {
			globalLeaderboardScroll += visible
		}
		globalLeaderboardScroll = clampLeaderboardScroll(globalLeaderboardScroll, total, visible)
		return true
	}
	ar := arenaRect(w, hgt)
	left := ar.Left + sx(55, w)
	right := ar.Right - sx(55, w)
	top := ar.Top + sy(32, hgt)
	bottom := ar.Bottom - sy(26, hgt)
	_ = left
	filterTop := top + sy(51, hgt)
	tableTop := filterTop + sy(42, hgt)
	rowH := sy(38, hgt)
	if rowH < 27 {
		rowH = 27
	}
	tableRight := right - sx(18, w)
	visible := localLeaderboardVisibleRows(w, hgt)
	total := len(filteredLocalLeaderboard())
	track := RECT{tableRight - sx(11, w), tableTop + rowH + 1, tableRight - 1, bottom - sy(35, hgt)}
	_, thumb := leaderboardScrollbarRects(track, total, visible, localLeaderboardScroll)
	if !pointInRect(p, track) {
		return false
	}
	if p.Y < float64(thumb.Top) {
		localLeaderboardScroll -= visible
	} else if p.Y > float64(thumb.Bottom) {
		localLeaderboardScroll += visible
	}
	localLeaderboardScroll = clampLeaderboardScroll(localLeaderboardScroll, total, visible)
	return true
}

func leaderboardColumns(left, right int32) [10]int32 {
	width := right - left

	// 9 columns:
	// # | NAME | SCORE | STREAK | ACCURACY | RANK | DIFFICULTY | DATE | TIME
	// Widths are percentages of the table width.
	pcts := []float64{0.045, 0.175, 0.105, 0.09, 0.11, 0.12, 0.13, 0.13, 0.095}

	var x [10]int32
	x[0] = left
	acc := float64(left)
	for i, p := range pcts {
		acc += float64(width) * p
		x[i+1] = int32(math.Round(acc))
	}
	x[9] = right
	return x
}

func cleanDiscordDisplayName(name string) string {
	name = strings.TrimSpace(name)
	// Discord's legacy discriminator system is retired. Some older leaderboard
	// records still contain the migration-era #0 suffix; never show it in-game.
	if strings.HasSuffix(name, "#0") {
		name = strings.TrimSpace(strings.TrimSuffix(name, "#0"))
	}
	return strings.ToUpper(name)
}

func drawTableCellText(hdc uintptr, left, right, y, rowH int32, text string, color uintptr, center bool) {
	setTextColor.Call(hdc, color)
	cell := RECT{Left: left, Top: y, Right: right, Bottom: y + rowH}
	// Leaderboard tables are rendered with hudTinyFont. Use the same font for
	// measurement as drawing; measuring hudSmallFont shifted every value off the
	// optical Y centre even though the row geometry itself was correct.
	font := hudTinyFont
	if font == 0 {
		font = hudSmallFont
	}
	textY := verticallyCenteredTextY(hdc, font, cell, text)
	if center {
		centeredTextOut(hdc, left+4, right-4, textY, text)
	} else {
		textOut(hdc, left+8, textY, text)
	}
}

func drawLeaderboardGrid(hdc uintptr, left, top, right, bottom int32, cols [10]int32, rowH int32, rows int) {
	grid := rgb(25, 91, 132)

	// Vertical grid lines.
	for _, x := range cols {
		drawLineSimple(hdc, x, top, x, bottom, 1, grid)
	}

	// Header + row horizontal grid lines.
	for i := 0; i <= rows+1; i++ {
		y := top + int32(i)*rowH
		if y > bottom {
			y = bottom
		}
		drawLineSimple(hdc, left, y, right, y, 1, grid)
	}
}

func leaderboardFilterRects(left, right, top int32, w, hgt int32) [6]RECT {
	gap := sx(6, w)
	if gap < 3 {
		gap = 3
	}
	total := right - left
	buttonW := (total - gap*5) / 6
	height := sy(32, hgt)
	if height < 25 {
		height = 25
	}
	var out [6]RECT
	x := left
	for i := 0; i < 6; i++ {
		out[i] = RECT{x, top, x + buttonW, top + height}
		x += buttonW + gap
	}
	out[5].Right = right
	return out
}

func drawLeaderboardFilterTabs(hdc uintptr, rects [6]RECT, selected int) {
	labels := []string{"OVERALL", "EASY", "NORMAL", "HARD", "INSANE", "ENDURANCE"}
	for i, r := range rects {
		accent := rgb(54, 198, 243)
		switch i {
		case 1, 2, 3, 4:
			accent = diffs[i-1].color
		case 5:
			accent = rgb(63, 225, 255)
		}
		face, _, dark := themedHUDCardPalette(0)
		if i == selected {
			face = blendColor(face, accent, 0.24)
		}
		drawBevelPanel(hdc, r, face, accent, dark, 2)
		if i == selected {
			drawLineSimple(hdc, r.Left+8, r.Bottom-5, r.Right-8, r.Bottom-5, 3, accent)
		}
		c := rgb(235, 246, 252)
		if i > 0 {
			c = accent
		}
		drawArcadeLabel(hdc, r, labels[i], c)
	}
}

func leaderboardFilterAt(p FPoint, w, hgt int32) int {
	ar := arenaRect(w, hgt)
	left := ar.Left + sx(73, w)
	right := ar.Right - sx(73, w)
	// Must match the rendered leaderboard tabs: panel top (arena+32) + 51.
	top := ar.Top + sy(83, hgt)
	rects := leaderboardFilterRects(left, right, top, w, hgt)

	for i, r := range rects {
		if pointInRect(p, r) {
			return i
		}
	}
	return -1
}

func globalEntryFromMap(m map[string]any, index int) GlobalLeaderboardEntry {
	getString := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k]; ok && v != nil {
				switch x := v.(type) {
				case string:
					return x
				}
			}
		}
		return ""
	}
	getInt := func(keys ...string) int {
		for _, k := range keys {
			if v, ok := m[k]; ok && v != nil {
				switch x := v.(type) {
				case float64:
					return int(math.Round(x))
				case int:
					return x
				}
			}
		}
		return 0
	}
	getFloat := func(keys ...string) float64 {
		for _, k := range keys {
			if v, ok := m[k]; ok && v != nil {
				switch x := v.(type) {
				case float64:
					return x
				case int:
					return float64(x)
				}
			}
		}
		return 0
	}

	pos := getInt("position", "rank_position")
	if pos <= 0 {
		pos = index + 1
	}
	easy := getInt("easy_clears", "easy_completed", "easy")
	normal := getInt("normal_clears", "normal_completed", "normal")
	hard := getInt("hard_clears", "hard_completed", "hard")
	insane := getInt("insane_clears", "insane_completed", "insane")
	totalClears := getInt("total_clears", "clears", "courses_completed")
	if totalClears <= 0 {
		totalClears = easy + normal + hard + insane
	}

	difficulty := getString("difficulty", "player_difficulty")
	distance := getFloat("distance", "distance_m")
	if strings.EqualFold(difficulty, "ENDURANCE") && distance <= 0 {
		distance = float64(getInt("score", "player_score")) / 10.0
	}
	targetsHit := getInt("targets_hit", "target_count")
	if strings.EqualFold(difficulty, "ENDURANCE") && targetsHit <= 0 {
		targetsHit = getInt("streak", "player_streak")
	}

	return GlobalLeaderboardEntry{
		UserID:       getString("user_id", "player_id"),
		Position:     pos,
		Name:         cleanDiscordDisplayName(getString("display_name", "player_name", "name")),
		NameColour:   getInt("selected_name_colour", "name_colour", "profile_name_colour"),
		Score:        getInt("score", "player_score"),
		Streak:       getInt("streak", "player_streak"),
		Accuracy:     getFloat("accuracy", "player_accuracy"),
		Difficulty:   difficulty,
		Rank:         getString("exp_rank", "player_rank", "rank"),
		Distance:     distance,
		TargetsHit:   targetsHit,
		EasyClears:   easy,
		NormalClears: normal,
		HardClears:   hard,
		InsaneClears: insane,
		TotalClears:  totalClears,
		AchievedAt:   getString("achieved_at", "score_date", "date"),
	}
}

func announcementBarRect(w, hgt int32) RECT {
	return RECT{0, sy(148, hgt), w, sy(194, hgt)}
}

const (
	liveAnnouncementTravelDuration = 15 * time.Second
	liveAnnouncementLaunchInterval = 10 * time.Second
)

func showPriorityLiveAnnouncement(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	liveMu.Lock()
	now := time.Now()
	if liveAnnouncementText != "" && liveAnnouncementText != message {
		// Keep the interrupted announcement moving left while the priority message
		// enters from the right, rather than making the rail pop or go empty.
		livePreviousAnnouncementText = liveAnnouncementText
		livePreviousAnnouncementStarted = liveAnnouncementStarted
	}
	liveAnnouncementText = message
	liveAnnouncementStarted = now
	liveMu.Unlock()
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

func enqueueLiveAnnouncement(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	liveMu.Lock()
	for _, queued := range liveAnnouncementQueue {
		if queued == message {
			liveMu.Unlock()
			return
		}
	}
	if liveAnnouncementText == message {
		liveMu.Unlock()
		return
	}
	if len(liveAnnouncementQueue) >= 24 {
		liveAnnouncementQueue = liveAnnouncementQueue[len(liveAnnouncementQueue)-23:]
	}
	liveAnnouncementQueue = append(liveAnnouncementQueue, message)
	liveMu.Unlock()
}

func ordinalPlace(position int) string {
	switch position {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	default:
		return fmt.Sprintf("%dth", position)
	}
}

func leaderboardAnnouncement(name, mode string, position int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	switch strings.ToUpper(mode) {
	case "OVERALL":
		return fmt.Sprintf("%s is now %s place Overall.", name, ordinalPlace(position))
	case "ENDURANCE":
		return fmt.Sprintf("%s is now %s place in Endurance.", name, ordinalPlace(position))
	default:
		pretty := strings.Title(strings.ToLower(mode))
		return fmt.Sprintf("%s is now %s place in %s mode.", name, ordinalPlace(position), pretty)
	}
}

func compareTopThree(oldEntries, newEntries []GlobalLeaderboardEntry, mode string) {
	for i := 0; i < 3 && i < len(newEntries); i++ {
		newName := strings.TrimSpace(newEntries[i].Name)
		oldName := ""
		if i < len(oldEntries) {
			oldName = strings.TrimSpace(oldEntries[i].Name)
		}
		if newName != "" && !strings.EqualFold(newName, oldName) {
			enqueueLiveAnnouncement(leaderboardAnnouncement(newName, mode, i+1))
		}
	}
}

func ambientAnnouncement() string {
	messages := []string{
		"Don’t forget to claim your FREE daily Space Cache!",
		"Log in with Discord to sync your ships, Space Coins and progress across devices.",
		"Your FREE Space Cache resets every day at midnight JKT — don’t miss it!",
		"Endurance rewards accuracy, survival and speed. How far can you get?",
		"The first Warp Phase starts at 1,000m in Endurance — get ready!",
		"Shield warning: the protection power-up does NOT protect you from Alien Boss laser beams.",
		"Space Caches cost 100 Space Coins and can contain new ships.",
		"Press ESC at any time to open Settings.",
		"Found a bug? Click the Report a Bug button so it can be exterminated.",
		"Check the Global Leaderboard — someone might have stolen your spot.",
		"Your personal best is waiting to be beaten.",
		"Higher difficulties award more EXP. Risk it for faster rank progression.",
		"Click a player on the Global Leaderboard to view their profile.",
		"New here? Start on Easy, then work your way up to Insane.",
		"Accuracy matters. Clean runs are better runs.",
		"Think you’re fast? Prove it on the Global Leaderboard.",
		"Survival boss checkpoints unlock when you reach a boss; beating it unlocks the next sector checkpoint.",
		"Survival: RED enemies need LEFT CLICK, BLUE enemies need RIGHT CLICK.",
		"Large Survival enemies take 2 hits and deal 2 station damage.",
		"A Perfect Defence means no station damage and no shield interception.",
		"The Sentinel waits beyond Wave 10. Beat it to permanently unlock Sector 2.",
		"Survival gets faster and more dangerous as each sector progresses.",
	}
	return messages[rand.Intn(len(messages))]
}

func tickLiveSystems(h uintptr) {
	now := time.Now()
	afkCloudTick(now)
	updateAFKLiveProduction(h, now)
	pollLeaderboard := false
	pollFeed := false

	liveMu.Lock()
	// v356: launch the next ticker before the current one has fully left the
	// screen. Each message travels for 15s, but a new one launches every 10s.
	// This keeps the GLOBAL LIVE rail active without forcing two strings to share
	// the same start position or abruptly replacing the outgoing message.
	if livePreviousAnnouncementText != "" && !livePreviousAnnouncementStarted.IsZero() && now.Sub(livePreviousAnnouncementStarted) >= liveAnnouncementTravelDuration {
		livePreviousAnnouncementText = ""
		livePreviousAnnouncementStarted = time.Time{}
	}
	shouldLaunch := liveAnnouncementText == "" || liveAnnouncementStarted.IsZero() || now.Sub(liveAnnouncementStarted) >= liveAnnouncementLaunchInterval
	if shouldLaunch {
		if liveAnnouncementText != "" && !liveAnnouncementStarted.IsZero() && now.Sub(liveAnnouncementStarted) < liveAnnouncementTravelDuration {
			livePreviousAnnouncementText = liveAnnouncementText
			livePreviousAnnouncementStarted = liveAnnouncementStarted
		}
		if len(liveAnnouncementQueue) > 0 {
			liveAnnouncementText = liveAnnouncementQueue[0]
			liveAnnouncementQueue = liveAnnouncementQueue[1:]
		} else {
			liveAnnouncementText = ambientAnnouncement()
		}
		liveAnnouncementStarted = now
		liveNextAmbientAt = now.Add(liveAnnouncementLaunchInterval)
	}
	if liveLastLeaderboardPoll.IsZero() || now.Sub(liveLastLeaderboardPoll) >= 20*time.Second {
		liveLastLeaderboardPoll = now
		pollLeaderboard = true
	}
	if liveLastFeedPoll.IsZero() || now.Sub(liveLastFeedPoll) >= 10*time.Second {
		liveLastFeedPoll = now
		pollFeed = true
	}
	if freeCacheStatusKnown && !freeCacheAvailable && !freeCacheNextClaimAt.IsZero() && !serverNowLocked(now).Before(freeCacheNextClaimAt) {
		freeCacheAvailable = true
	}
	liveMu.Unlock()

	if pollLeaderboard {
		go fetchGlobalLeaderboard()
	}
	if pollFeed {
		go fetchLiveFeed()
	}

	if h != 0 && enduranceActive() && state == StateWaiting {
		w, hgt := getClient(h)
		freeR := freeCacheClaimRect(w, hgt)
		invalidateRect.Call(h, uintptr(unsafe.Pointer(&freeR)), 0)
	}
}

// tickLiveAnimation is deliberately kept render-only. Network polling, queue
// management and the free-cache clock run on TIMER_LIVE_SYSTEMS instead, so
// the ticker can repaint smoothly at ~60 FPS without doing unrelated work.
func tickLiveAnimation(h uintptr) {
	if h == 0 {
		return
	}
	liveMu.Lock()
	active := (liveAnnouncementText != "" && !liveAnnouncementStarted.IsZero()) ||
		(livePreviousAnnouncementText != "" && !livePreviousAnnouncementStarted.IsZero())
	liveMu.Unlock()
	w, hgt := getClient(h)
	if animatedAvatarVisible() {
		invalidateRect.Call(h, 0, 0)
	}
	// v375: time-based UI effects repaint from the UI clock, never from mouse movement.
	// This keeps FLOWING HIGHLIGHT, subtle pulse, and animated button hover effects alive
	// even when the player holds the mouse perfectly still.
	if gameMeta.EXPBarAnimation != 0 && !survivalActive() {
		expAnimRect := RECT{sx(20, w), sy(900, hgt), sx(455, w), sy(1005, hgt)}
		invalidateRect.Call(h, uintptr(unsafe.Pointer(&expAnimRect)), 0)
	}
	if overlayMode != OverlayNone && (gameMeta.ButtonHoverEffect == 2 || gameMeta.ButtonHoverEffect == 3) {
		invalidateRect.Call(h, 0, 0)
	}
	if !active {
		return
	}
	bar := announcementBarRect(w, hgt)
	invalidateRect.Call(h, uintptr(unsafe.Pointer(&bar)), 0)
}

func serverNowLocked(local time.Time) time.Time {
	return local.Add(liveServerOffset)
}

type announcementThemePalette struct {
	BG      uintptr
	Top     uintptr
	Bottom  uintptr
	TagBG   uintptr
	TagText uintptr
	Text    uintptr
	Muted   uintptr
}

func currentAnnouncementThemePalette() announcementThemePalette {
	switch gameMeta.AnnouncementTheme {
	case 1: // warning
		return announcementThemePalette{BG: rgb(47, 18, 4), Top: rgb(255, 171, 38), Bottom: rgb(255, 110, 36), TagBG: rgb(99, 40, 10), TagText: rgb(255, 217, 92), Text: rgb(255, 240, 221), Muted: rgb(180, 128, 96)}
	case 2: // hologram
		return announcementThemePalette{BG: rgb(6, 30, 52), Top: rgb(92, 236, 255), Bottom: rgb(127, 109, 255), TagBG: rgb(14, 66, 108), TagText: rgb(166, 255, 255), Text: rgb(232, 251, 255), Muted: rgb(124, 162, 185)}
	case 3: // minimal
		return announcementThemePalette{BG: rgb(28, 33, 40), Top: rgb(198, 208, 218), Bottom: rgb(124, 140, 154), TagBG: rgb(48, 56, 66), TagText: rgb(246, 248, 250), Text: rgb(242, 245, 248), Muted: rgb(134, 148, 160)}
	case 4: // industrial
		return announcementThemePalette{BG: rgb(42, 39, 34), Top: rgb(221, 172, 86), Bottom: rgb(120, 96, 64), TagBG: rgb(78, 66, 49), TagText: rgb(255, 231, 174), Text: rgb(246, 239, 228), Muted: rgb(150, 138, 121)}
	case 5: // neon
		return announcementThemePalette{BG: rgb(20, 6, 38), Top: rgb(255, 58, 178), Bottom: rgb(82, 243, 255), TagBG: rgb(57, 16, 97), TagText: rgb(255, 180, 235), Text: rgb(247, 240, 255), Muted: rgb(161, 133, 195)}
	default: // standard sci-fi
		return announcementThemePalette{BG: rgb(2, 17, 48), Top: rgb(255, 143, 15), Bottom: rgb(48, 216, 255), TagBG: rgb(4, 42, 99), TagText: rgb(255, 190, 40), Text: rgb(238, 248, 255), Muted: rgb(120, 145, 164)}
	}
}

func drawGlobalAnnouncementBar(hdc uintptr, w, hgt int32) {
	bar := announcementBarRect(w, hgt)
	pal := currentAnnouncementThemePalette()
	fillSolidRect(hdc, bar, pal.BG)
	drawLineSimple(hdc, bar.Left, bar.Top, bar.Right, bar.Top, 2, pal.Top)
	drawLineSimple(hdc, bar.Left, bar.Bottom-1, bar.Right, bar.Bottom-1, 2, pal.Bottom)

	tagRight := bar.Left + sx(138, w)
	fillSolidRect(hdc, RECT{bar.Left, bar.Top + 2, tagRight, bar.Bottom - 2}, pal.TagBG)
	if hudTinyFont == 0 {
		return
	}
	oldFont, _, _ := selectObject.Call(hdc, hudTinyFont)
	defer selectObject.Call(hdc, oldFont)
	setBkMode.Call(hdc, TRANSPARENT)
	setTextColor.Call(hdc, pal.TagText)
	centeredTextOut(hdc, bar.Left, tagRight, verticallyCenteredTextY(hdc, hudTinyFont, bar, "GLOBAL LIVE"), "GLOBAL LIVE")

	liveMu.Lock()
	text := liveAnnouncementText
	started := liveAnnouncementStarted
	previousText := livePreviousAnnouncementText
	previousStarted := livePreviousAnnouncementStarted
	liveMu.Unlock()
	if (text == "" || started.IsZero()) && (previousText == "" || previousStarted.IsZero()) {
		return
	}
	content := RECT{tagRight + sx(14, w), bar.Top + 4, bar.Right - sx(14, w), bar.Bottom - 4}
	tickerFont := hudSmallFont
	if tickerFont == 0 {
		tickerFont = hudTinyFont
	}
	selectObject.Call(hdc, tickerFont)

	saved, _, _ := saveDC.Call(hdc)
	intersectClipRect.Call(hdc, uintptr(content.Left), uintptr(content.Top), uintptr(content.Right), uintptr(content.Bottom))
	tickerColor := pal.Text
	if gameplayCriticalFocusActive() {
		// Live/community information remains available but deliberately recedes
		// while a boss/warp demands immediate spatial attention.
		tickerColor = pal.Muted
	}
	setTextColor.Call(hdc, tickerColor)

	drawTicker := func(msg string, began time.Time) {
		if msg == "" || began.IsZero() {
			return
		}
		textSize := textPixelSize(hdc, tickerFont, msg)
		progress := time.Since(began).Seconds() / liveAnnouncementTravelDuration.Seconds()
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		startX := content.Right
		endX := content.Left - textSize.Cx
		x := startX + int32(float64(endX-startX)*progress)
		y := verticallyCenteredTextY(hdc, tickerFont, content, msg)
		critical := gameplayCriticalFocusActive()
		if critical {
			setTextColor.Call(hdc, pal.Muted)
			textOut(hdc, x, y, msg)
			return
		}
		switch gameMeta.AnnouncementTheme {
		case 1: // WARNING: warm pulse + dark drop shadow
			pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*5.5)
			setTextColor.Call(hdc, rgb(65, 18, 5))
			textOut(hdc, x+2, y+2, msg)
			setTextColor.Call(hdc, blendColor(pal.Text, pal.TagText, 0.24+0.46*pulse))
			textOut(hdc, x, y, msg)
		case 2: // HOLOGRAM: cyan ghost split around a white-blue core
			setTextColor.Call(hdc, blendColor(pal.Bottom, pal.BG, 0.32))
			textOut(hdc, x-2, y, msg)
			setTextColor.Call(hdc, blendColor(pal.Top, pal.BG, 0.36))
			textOut(hdc, x+2, y+1, msg)
			setTextColor.Call(hdc, pal.Text)
			textOut(hdc, x, y, msg)
		case 3: // MINIMAL: crisp single-line text with a restrained underline travelling with it
			setTextColor.Call(hdc, pal.Text)
			textOut(hdc, x, y, msg)
			lineRight := minInt32(content.Right, x+textSize.Cx)
			if lineRight > x {
				drawLineSimple(hdc, x, y+textSize.Cy+1, lineRight, y+textSize.Cy+1, 1, pal.Bottom)
			}
		case 4: // INDUSTRIAL: stamped amber text with hard offset shadow
			setTextColor.Call(hdc, rgb(30, 25, 18))
			textOut(hdc, x+2, y+2, msg)
			setTextColor.Call(hdc, pal.TagText)
			textOut(hdc, x, y, msg)
		case 5: // NEON: two-colour halo + bright core
			setTextColor.Call(hdc, blendColor(pal.Top, pal.BG, 0.20))
			textOut(hdc, x-2, y, msg)
			textOut(hdc, x+2, y, msg)
			setTextColor.Call(hdc, blendColor(pal.Bottom, pal.BG, 0.18))
			textOut(hdc, x, y-1, msg)
			textOut(hdc, x, y+2, msg)
			setTextColor.Call(hdc, rgb(255, 248, 255))
			textOut(hdc, x, y, msg)
		default:
			setTextColor.Call(hdc, tickerColor)
			textOut(hdc, x, y, msg)
		}
	}

	// Draw the outgoing message first and the incoming one second. The launch
	// interval is shorter than the travel duration, so both are briefly visible.
	drawTicker(previousText, previousStarted)
	drawTicker(text, started)
	if saved != 0 {
		restoreDC.Call(hdc, saved)
	}
}

func fetchLiveFeed() {
	req, err := http.NewRequest(http.MethodGet, supabaseProjectURL+"/functions/v1/live-feed", nil)
	if err != nil {
		return
	}
	req.Header.Set("apikey", supabasePublishableKey)
	if token, err := validAuthAccessToken(); err == nil && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("Authorization", "Bearer "+supabasePublishableKey)
	}
	resp, err := authHTTPClient().Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	var env struct {
		ServerTime    string             `json:"server_time"`
		Announcements []LiveAnnouncement `json:"announcements"`
		FreeCache     struct {
			Known       bool   `json:"known"`
			Available   bool   `json:"available"`
			NextClaimAt string `json:"next_claim_at"`
		} `json:"free_cache"`
	}
	if json.Unmarshal(body, &env) != nil {
		return
	}

	var serverTime time.Time
	if env.ServerTime != "" {
		serverTime, _ = time.Parse(time.RFC3339Nano, env.ServerTime)
	}
	liveMu.Lock()
	if !serverTime.IsZero() {
		liveServerOffset = serverTime.Sub(time.Now())
	}
	if env.FreeCache.Known {
		freeCacheStatusKnown = true
		freeCacheAvailable = env.FreeCache.Available
		if env.FreeCache.NextClaimAt != "" {
			freeCacheNextClaimAt, _ = time.Parse(time.RFC3339Nano, env.FreeCache.NextClaimAt)
		} else {
			freeCacheNextClaimAt = time.Time{}
		}
	}
	maxID := liveLastFeedID
	if !liveFeedBaselineReady {
		for _, a := range env.Announcements {
			if a.ID > maxID {
				maxID = a.ID
			}
		}
		liveLastFeedID = maxID
		liveFeedBaselineReady = true
		liveMu.Unlock()
		return
	}
	var fresh []string
	for _, a := range env.Announcements {
		if a.ID > liveLastFeedID && strings.TrimSpace(a.Message) != "" {
			fresh = append(fresh, strings.TrimSpace(a.Message))
		}
		if a.ID > maxID {
			maxID = a.ID
		}
	}
	liveLastFeedID = maxID
	liveMu.Unlock()
	for _, message := range fresh {
		enqueueLiveAnnouncement(message)
	}
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

func claimFreeSpaceCache(h uintptr) {
	defer func() {
		liveMu.Lock()
		freeCacheClaimInFlight = false
		liveMu.Unlock()
	}()
	token, err := validAuthAccessToken()
	if err != nil || token == "" {
		showSpaceCacheWarning(h, "Log in with Discord to claim your free Space Cache.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"action": "claim_free_cache"})
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/functions/v1/live-feed", bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := authHTTPClient().Do(req)
	if err != nil {
		showSpaceCacheWarning(h, "Free Cache server is unavailable. Try again shortly.")
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env struct {
		Claimed     bool   `json:"claimed"`
		ServerTime  string `json:"server_time"`
		NextClaimAt string `json:"next_claim_at"`
	}
	if json.Unmarshal(body, &env) != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		showSpaceCacheWarning(h, "Free Cache server is unavailable. Try again shortly.")
		return
	}
	liveMu.Lock()
	freeCacheStatusKnown = true
	freeCacheAvailable = false
	if env.NextClaimAt != "" {
		freeCacheNextClaimAt, _ = time.Parse(time.RFC3339Nano, env.NextClaimAt)
	}
	if st, err := time.Parse(time.RFC3339Nano, env.ServerTime); err == nil {
		liveServerOffset = st.Sub(time.Now())
	}
	liveMu.Unlock()
	if !env.Claimed {
		showSpaceCacheWarning(h, "You need 100 Space Coins to open a Cache!")
		return
	}
	startSpaceCacheOpening(h)
}

func publishSpaceCacheShipUnlock(ship int) {
	if ship <= 0 || ship >= len(spaceShipDefs) || strings.TrimSpace(spaceShipDefs[ship].Name) == "" {
		return
	}
	token, err := validAuthAccessToken()
	if err != nil || token == "" {
		return
	}
	payload, _ := json.Marshal(map[string]any{"action": "publish_ship", "ship_id": ship})
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/functions/v1/live-feed", bytes.NewReader(payload))
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

func fetchGlobalLeaderboard() {
	globalMu.Lock()
	globalLeaderboardLoading = true
	globalLeaderboardStatus = "CONNECTING TO GLOBAL TOP 20..."
	globalMu.Unlock()
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}

	req, err := http.NewRequest(http.MethodGet, supabaseProjectURL+"/functions/v1/global-top20", nil)
	if err != nil {
		return
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+supabasePublishableKey)

	resp, err := authHTTPClient().Do(req)
	if err != nil {
		globalMu.Lock()
		globalLeaderboardLoading = false
		globalLeaderboardStatus = "GLOBAL SERVER IS NOT AVAILABLE"
		globalMu.Unlock()
		if mainHwnd != 0 {
			invalidateRect.Call(mainHwnd, 0, 0)
		}
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		globalMu.Lock()
		globalLeaderboardLoading = false
		globalLeaderboardStatus = fmt.Sprintf("GLOBAL SERVER RETURNED %d", resp.StatusCode)
		globalMu.Unlock()
		if mainHwnd != 0 {
			invalidateRect.Call(mainHwnd, 0, 0)
		}
		return
	}

	var env struct {
		Overall     []map[string]any `json:"overall"`
		Easy        []map[string]any `json:"easy"`
		Normal      []map[string]any `json:"normal"`
		Hard        []map[string]any `json:"hard"`
		Insane      []map[string]any `json:"insane"`
		Endurance   []map[string]any `json:"endurance"`
		Leaderboard []map[string]any `json:"leaderboard"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		globalMu.Lock()
		globalLeaderboardLoading = false
		globalLeaderboardStatus = "GLOBAL DATA FORMAT ERROR"
		globalMu.Unlock()
		if mainHwnd != 0 {
			invalidateRect.Call(mainHwnd, 0, 0)
		}
		return
	}
	if len(env.Overall) == 0 && len(env.Leaderboard) > 0 {
		env.Overall = env.Leaderboard
	}

	parse := func(src []map[string]any, diff string) []GlobalLeaderboardEntry {
		out := make([]GlobalLeaderboardEntry, 0, len(src))
		for i, m := range src {
			e := globalEntryFromMap(m, i)
			if diff != "" {
				e.Difficulty = diff
			}
			if strings.TrimSpace(e.Name) != "" {
				out = append(out, e)
			}
		}
		return out
	}
	overall := parse(env.Overall, "")
	easy := parse(env.Easy, "EASY")
	normal := parse(env.Normal, "NORMAL")
	hard := parse(env.Hard, "HARD")
	insane := parse(env.Insane, "INSANE")
	endurance := parse(env.Endurance, "ENDURANCE")
	for i := range endurance {
		if endurance[i].Distance <= 0 {
			endurance[i].Distance = float64(endurance[i].Score) / 10.0
		}
		if endurance[i].TargetsHit <= 0 {
			endurance[i].TargetsHit = endurance[i].Streak
		}
	}
	sort.SliceStable(endurance, func(i, j int) bool {
		if math.Abs(endurance[i].Distance-endurance[j].Distance) > 0.001 {
			return endurance[i].Distance > endurance[j].Distance
		}
		if endurance[i].TargetsHit != endurance[j].TargetsHit {
			return endurance[i].TargetsHit > endurance[j].TargetsHit
		}
		return endurance[i].Accuracy > endurance[j].Accuracy
	})
	for i := range endurance {
		endurance[i].Position = i + 1
	}

	globalMu.Lock()
	oldOverall := append([]GlobalLeaderboardEntry(nil), globalLeaderboardOverall...)
	oldEasy := append([]GlobalLeaderboardEntry(nil), globalLeaderboardEasy...)
	oldNormal := append([]GlobalLeaderboardEntry(nil), globalLeaderboardNormal...)
	oldHard := append([]GlobalLeaderboardEntry(nil), globalLeaderboardHard...)
	oldInsane := append([]GlobalLeaderboardEntry(nil), globalLeaderboardInsane...)
	oldEndurance := append([]GlobalLeaderboardEntry(nil), globalLeaderboardEndurance...)
	globalLeaderboardOverall = overall
	globalLeaderboardEasy = easy
	globalLeaderboardNormal = normal
	globalLeaderboardHard = hard
	globalLeaderboardInsane = insane
	globalLeaderboardEndurance = endurance
	globalLeaderboardLoading = false
	if len(overall)+len(easy)+len(normal)+len(hard)+len(insane)+len(endurance) == 0 {
		globalLeaderboardStatus = "NO GLOBAL SCORES YET"
	} else {
		globalLeaderboardStatus = ""
	}
	globalMu.Unlock()

	liveMu.Lock()
	baselineReady := liveLeaderboardBaseline
	if !liveLeaderboardBaseline {
		liveLeaderboardBaseline = true
	}
	liveMu.Unlock()
	if baselineReady {
		compareTopThree(oldOverall, overall, "OVERALL")
		compareTopThree(oldEasy, easy, "EASY")
		compareTopThree(oldNormal, normal, "NORMAL")
		compareTopThree(oldHard, hard, "HARD")
		compareTopThree(oldInsane, insane, "INSANE")
		compareTopThree(oldEndurance, endurance, "ENDURANCE")
	}
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

func globalFilteredEntries() []GlobalLeaderboardEntry {
	globalMu.Lock()
	defer globalMu.Unlock()
	var src []GlobalLeaderboardEntry
	switch globalLeaderboardScope {
	case leaderboardScopeWeekly:
		src = globalLeaderboardWeekly
	case leaderboardScopeCompetition:
		src = competitionAsGlobalEntries()
	default:
		switch leaderboardFilterName(globalLeaderboardFilter) {
		case "EASY":
			src = globalLeaderboardEasy
		case "NORMAL":
			src = globalLeaderboardNormal
		case "HARD":
			src = globalLeaderboardHard
		case "INSANE":
			src = globalLeaderboardInsane
		case "ENDURANCE":
			src = globalLeaderboardEndurance
		default:
			src = globalLeaderboardOverall
		}
	}
	out := make([]GlobalLeaderboardEntry, len(src))
	copy(out, src)
	return out
}

func localBestForDifficulty(diff string) (LeaderboardEntry, bool) {
	diff = strings.TrimSpace(strings.ToUpper(diff))
	var best LeaderboardEntry
	found := false
	for _, e := range leaderboard {
		if !strings.EqualFold(strings.TrimSpace(e.Difficulty), diff) {
			continue
		}
		if !found || leaderboardEntryIsBetter(e, best) {
			best = e
			found = true
		}
	}
	return best, found
}

func localProgressGlobalPosition(entries []GlobalLeaderboardEntry) int {
	name := strings.TrimSpace(localPlayerName())
	if name == "" {
		return 0
	}
	for i, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e.Name), name) {
			return i + 1
		}
	}
	return 0
}

func localProgressTotalClears() int {
	return playerProgress.EasyCompleted +
		playerProgress.NormalCompleted +
		playerProgress.HardCompleted +
		playerProgress.InsaneCompleted
}

func fetchMyGlobalStats() {
	token, err := validAuthAccessToken()
	if err != nil || strings.TrimSpace(token) == "" {
		return
	}
	req, err := http.NewRequest(http.MethodGet, supabaseProjectURL+"/functions/v1/my-global-stats", nil)
	if err != nil {
		return
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := authHTTPClient().Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return
	}

	e := GlobalLeaderboardEntry{}
	if v, ok := m["player"].(string); ok {
		e.Name = cleanDiscordDisplayName(v)
	} else if v, ok := m["display_name"].(string); ok {
		e.Name = cleanDiscordDisplayName(v)
	}
	if v, ok := m["rank"].(string); ok {
		e.Rank = strings.ToUpper(strings.TrimSpace(v))
	} else if v, ok := m["exp_rank"].(string); ok {
		e.Rank = strings.ToUpper(strings.TrimSpace(v))
	}
	if c, ok := m["completions"].(map[string]any); ok {
		toInt := func(v any) int {
			switch x := v.(type) {
			case float64:
				return int(math.Round(x))
			case int:
				return x
			}
			return 0
		}
		e.EasyClears = toInt(c["easy"])
		e.NormalClears = toInt(c["normal"])
		e.HardClears = toInt(c["hard"])
		e.InsaneClears = toInt(c["insane"])
		e.TotalClears = e.EasyClears + e.NormalClears + e.HardClears + e.InsaneClears
	}
	if pos, ok := m["positions"].(map[string]any); ok {
		switch x := pos["overall"].(type) {
		case float64:
			e.Position = int(math.Round(x))
		case int:
			e.Position = x
		}
	}
	if e.Position <= 0 {
		for _, k := range []string{"global_position", "rank_position", "position"} {
			if v, ok := m[k].(float64); ok && v > 0 {
				e.Position = int(math.Round(v))
				break
			}
		}
	}
	if e.Position <= 0 && strings.TrimSpace(e.Rank) == "" {
		return
	}
	globalMu.Lock()
	globalMyOverall = e
	globalMyOverallValid = true
	globalMu.Unlock()
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

func localPlayerServerRank() (string, bool) {
	name := strings.TrimSpace(localPlayerName())
	if name == "" {
		return "", false
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	// Prefer the dedicated authenticated stats response.
	if globalMyOverallValid {
		r := strings.ToUpper(strings.TrimSpace(globalMyOverall.Rank))
		if r != "" {
			return r, true
		}
	}
	// The same server EXP rank is also carried on difficulty/PB rows. This
	// fallback prevents a legitimate player outside Overall Top 20 from being
	// shown as UNRANKED when another global board already knows their rank.
	lists := [][]GlobalLeaderboardEntry{
		globalLeaderboardOverall, globalLeaderboardEasy, globalLeaderboardNormal,
		globalLeaderboardHard, globalLeaderboardInsane, globalLeaderboardEndurance,
	}
	for _, list := range lists {
		for _, e := range list {
			if strings.EqualFold(strings.TrimSpace(e.Name), name) {
				r := strings.ToUpper(strings.TrimSpace(e.Rank))
				if r != "" {
					return r, true
				}
			}
		}
	}
	if globalEnduranceSyncedValid && (strings.TrimSpace(globalEnduranceSyncedPB.Name) == "" || strings.EqualFold(strings.TrimSpace(globalEnduranceSyncedPB.Name), name)) {
		r := strings.ToUpper(strings.TrimSpace(globalEnduranceSyncedPB.Rank))
		if r != "" {
			return r, true
		}
	}
	return "", false
}

func localPlayerOverallEntry() (GlobalLeaderboardEntry, bool) {
	name := strings.TrimSpace(localPlayerName())
	if name == "" {
		return GlobalLeaderboardEntry{}, false
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalMyOverallValid {
		e := globalMyOverall
		if strings.TrimSpace(e.Name) == "" || strings.EqualFold(strings.TrimSpace(e.Name), name) {
			return e, true
		}
	}
	for i, e := range globalLeaderboardOverall {
		if strings.EqualFold(strings.TrimSpace(e.Name), name) {
			e.Position = i + 1
			return e, true
		}
	}
	return GlobalLeaderboardEntry{}, false
}

func localPlayerGlobalEntry() (GlobalLeaderboardEntry, bool) {
	name := strings.TrimSpace(localPlayerName())
	if name == "" {
		return GlobalLeaderboardEntry{}, false
	}
	filter := leaderboardFilterName(globalLeaderboardFilter)
	if filter == "OVERALL" {
		if e, ok := localPlayerOverallEntry(); ok {
			return e, true
		}
	}
	// Always search the current server set, even when the player is already visible
	// in Top 20.  The pinned row is a mirror of global data, not an exclusion row.
	entries := globalFilteredEntries()
	for i, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e.Name), name) {
			if e.Position <= 0 {
				e.Position = i + 1
			}
			return e, true
		}
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	if filter == "OVERALL" {
		for i, e := range globalLeaderboardOverall {
			if strings.EqualFold(strings.TrimSpace(e.Name), name) {
				if e.Position <= 0 {
					e.Position = i + 1
				}
				return e, true
			}
		}
		if globalMyOverallValid {
			return globalMyOverall, true
		}
	}
	if filter == "ENDURANCE" && globalEnduranceSyncedValid {
		return globalEnduranceSyncedPB, true
	}
	// Last-resort populated row from synced/local progression instead of blank zeros.
	e := GlobalLeaderboardEntry{Name: name, Rank: rankForEXP(playerProgress.EXP), EasyClears: playerProgress.EasyCompleted, NormalClears: playerProgress.NormalCompleted, HardClears: playerProgress.HardCompleted, InsaneClears: playerProgress.InsaneCompleted}
	e.TotalClears = e.EasyClears + e.NormalClears + e.HardClears + e.InsaneClears
	return e, true
}

func globalLeaderboardGeometry(w, hgt int32) (left, right, top, bottom, tableLeft, tableRight, tableTop, rowH int32) {
	ar := arenaRect(w, hgt)
	left = ar.Left + sx(55, w)
	right = ar.Right - sx(55, w)
	top = ar.Top + sy(32, hgt)
	bottom = ar.Bottom - sy(26, hgt)
	tableLeft = left + sx(18, w)
	tableRight = right - sx(18, w)
	tableTop = top + sy(96, hgt)
	rowH = sy(22, hgt)
	if rowH < 18 {
		rowH = 18
	}
	return
}

func globalLeaderboardRowAt(p FPoint, w, hgt int32) int {
	entries := globalFilteredEntries()
	_, _, _, bottom, tableLeft, tableRight, tableTop, rowH := globalLeaderboardGeometry(w, hgt)
	if p.X < float64(tableLeft) || p.X > float64(tableRight-sx(15, w)) {
		return -1
	}
	firstY := tableTop + rowH
	pinnedHeaderTop := bottom - sy(66, hgt) - rowH
	if p.Y < float64(firstY) || p.Y >= float64(pinnedHeaderTop) {
		return -1
	}
	visible := globalLeaderboardVisibleRows(w, hgt)
	globalLeaderboardScroll = clampLeaderboardScroll(globalLeaderboardScroll, len(entries), visible)
	vis := int((int32(p.Y) - firstY) / rowH)
	if vis < 0 || vis >= visible {
		return -1
	}
	idx := globalLeaderboardScroll + vis
	if idx < 0 || idx >= len(entries) || idx >= 20 {
		return -1
	}
	return idx
}

func openGlobalLeaderboard() {
	menuOpen = false
	globalLeaderboardScroll = 0
	competitionGuideOpen = false
	selectedGlobalPlayer = -1
	selectedGlobalOverrideOn = false
	globalLeaderboardScope = leaderboardScopeTop20
	if enduranceActive() || (state == StateResult && lastResult.Course == "ENDURANCE") {
		globalLeaderboardFilter = 5
	}
	setOverlay(OverlayGlobalLeaderboard)
	go retryPendingEnduranceSubmission()
	go fetchGlobalLeaderboard()
}

func openOwnGlobalProfile() {
	if !discordConnected {
		status = "DISCORD LOGIN REQUIRED FOR GLOBAL PROFILE"
		return
	}
	name := cleanDiscordDisplayName(localPlayerName())
	entry := GlobalLeaderboardEntry{
		UserID:       discordUserID,
		Name:         name,
		Rank:         rankForEXP(playerProgress.EXP),
		EasyClears:   playerProgress.EasyCompleted,
		NormalClears: playerProgress.NormalCompleted,
		HardClears:   playerProgress.HardCompleted,
		InsaneClears: playerProgress.InsaneCompleted,
		TotalClears:  localProgressTotalClears(),
	}
	// Prefer any already-synced global row, but always retain a valid direct target
	// so the local-profile shortcut works even when the player is outside Top 20.
	globalMu.Lock()
	for _, list := range [][]GlobalLeaderboardEntry{globalLeaderboardOverall, globalLeaderboardEasy, globalLeaderboardNormal, globalLeaderboardHard, globalLeaderboardInsane, globalLeaderboardEndurance} {
		for _, e := range list {
			if (strings.TrimSpace(discordUserID) != "" && strings.EqualFold(strings.TrimSpace(e.UserID), strings.TrimSpace(discordUserID))) || strings.EqualFold(cleanDiscordDisplayName(e.Name), name) {
				entry = e
				break
			}
		}
	}
	globalMu.Unlock()
	selectedGlobalOverride = entry
	selectedGlobalOverrideOn = true
	selectedGlobalPlayer = -1
	setOverlay(OverlayRemoteProfile)
	prepareRemoteProfile()
}

func drawEnduranceLocalLeaderboard(hdc uintptr, w, hgt int32) {
	ar := arenaRect(w, hgt)
	left := ar.Left + sx(55, w)
	right := ar.Right - sx(55, w)
	top := ar.Top + sy(32, hgt)
	bottom := ar.Bottom - sy(26, hgt)
	baseFace, themeAccent, themeDark := themedHUDPanelPalette(true)
	headerBG := blendColor(themeDark, baseFace, .30)
	rowA := blendColor(baseFace, themeDark, .52)
	rowB := blendColor(baseFace, themeDark, .40)
	cyan := themeAccent
	white := rgb(239, 246, 252)
	muted := rgb(160, 190, 214)
	gold := rgb(242, 192, 55)
	silver := rgb(205, 216, 228)
	bronze := rgb(177, 118, 68)
	drawBevelPanel(hdc, RECT{left, top, right, bottom}, baseFace, themeAccent, themeDark, 4)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, cyan)
		centeredTextOut(hdc, left, right, top+10, "LOCAL ENDURANCE LEADERBOARD // TOP 10")
		selectObject.Call(hdc, old)
	}
	filterLeft := left + sx(18, w)
	filterRight := right - sx(18, w)
	filterTop := top + sy(51, hgt)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		drawLeaderboardFilterTabs(hdc, leaderboardFilterRects(filterLeft, filterRight, filterTop, w, hgt), localLeaderboardFilter)
		selectObject.Call(hdc, old)
	}
	entries := filteredLocalLeaderboard()
	visible := localLeaderboardVisibleRows(w, hgt)
	localLeaderboardScroll = clampLeaderboardScroll(localLeaderboardScroll, len(entries), visible)
	start := localLeaderboardScroll
	end := start + visible
	if end > len(entries) {
		end = len(entries)
	}
	rows := end - start
	tableLeft := filterLeft
	tableRight := filterRight
	tableTop := filterTop + sy(42, hgt)
	rowH := sy(38, hgt)
	if rowH < 27 {
		rowH = 27
	}
	scrollW := sx(14, w)
	if scrollW < 10 {
		scrollW = 10
	}
	contentRight := tableRight - scrollW - sx(3, w)
	total := contentRight - tableLeft
	cols := []int32{tableLeft, tableLeft + total*5/100, tableLeft + total*23/100, tableLeft + total*37/100, tableLeft + total*50/100, tableLeft + total*61/100, tableLeft + total*73/100, tableLeft + total*85/100, tableLeft + total*93/100, contentRight}
	headers := []string{"#", "NAME", "DISTANCE", "SCORE", "TARGETS", "ACCURACY", "RANK", "DATE", "TIME"}
	fillSolidRect(hdc, RECT{tableLeft, tableTop, contentRight, tableTop + rowH}, headerBG)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		for i, h := range headers {
			drawTableCellText(hdc, cols[i], cols[i+1], tableTop, rowH, h, cyan, true)
		}
		saved, _, _ := saveDC.Call(hdc)
		intersectClipRect.Call(hdc, uintptr(tableLeft), uintptr(tableTop+rowH), uintptr(contentRight), uintptr(bottom-sy(35, hgt)))
		for vis := 0; vis < rows; vis++ {
			idx := start + vis
			y := tableTop + rowH*int32(vis+1)
			e := entries[idx]
			bg := rowA
			if idx%2 == 1 {
				bg = rowB
			}
			if idx == 0 {
				bg = rgb(66, 50, 17)
			} else if idx == 1 {
				bg = rgb(38, 48, 62)
			} else if idx == 2 {
				bg = rgb(54, 36, 28)
			}
			if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(localPlayerName())) {
				bg = rgb(5, 55, 66)
			}
			fillSolidRect(hdc, RECT{tableLeft, y, contentRight, y + rowH}, bg)
			pc := white
			if idx == 0 {
				pc = gold
			} else if idx == 1 {
				pc = silver
			} else if idx == 2 {
				pc = bronze
			}
			drawTableCellText(hdc, cols[0], cols[1], y, rowH, fmt.Sprintf("%d", idx+1), pc, true)
			nameColor := white
			if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(localPlayerName())) {
				nameColor = profileNameColour(gameMeta.SelectedNameColour)
			}
			drawTableCellText(hdc, cols[1], cols[2], y, rowH, e.Name, nameColor, false)
			drawTableCellText(hdc, cols[2], cols[3], y, rowH, fmt.Sprintf("%.1f m", e.Distance), cyan, true)
			drawTableCellText(hdc, cols[3], cols[4], y, rowH, fmt.Sprintf("%d", e.Score), white, true)
			drawTableCellText(hdc, cols[4], cols[5], y, rowH, fmt.Sprintf("%d", e.TargetsHit), white, true)
			drawTableCellText(hdc, cols[5], cols[6], y, rowH, fmt.Sprintf("%.1f%%", e.Accuracy), white, true)
			drawTableCellText(hdc, cols[6], cols[7], y, rowH, e.Rank, white, true)
			drawTableCellText(hdc, cols[7], cols[8], y, rowH, e.Date, white, true)
			drawTableCellText(hdc, cols[8], cols[9], y, rowH, e.Time, white, true)
		}
		restoreDC.Call(hdc, saved)
		selectObject.Call(hdc, old)
	}
	gridBottom := tableTop + rowH*int32(rows+1)
	if gridBottom > bottom-sy(35, hgt) {
		gridBottom = bottom - sy(35, hgt)
	}
	for _, x := range cols {
		drawLineSimple(hdc, x, tableTop, x, gridBottom, 1, rgb(20, 72, 106))
	}
	for i := 0; i <= rows+1; i++ {
		y := tableTop + rowH*int32(i)
		if y > gridBottom {
			y = gridBottom
		}
		drawLineSimple(hdc, tableLeft, y, contentRight, y, 1, rgb(20, 72, 106))
	}
	track := RECT{contentRight + sx(4, w), tableTop + rowH + 1, tableRight - 1, bottom - sy(35, hgt)}
	drawLeaderboardScrollbar(hdc, track, len(entries), visible, localLeaderboardScroll, cyan)
	if len(entries) == 0 && hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, tableLeft, contentRight, tableTop+rowH+sy(44, hgt), "NO ENDURANCE RUNS YET")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, left, right, bottom-sy(23, hgt), "RANKED BY DISTANCE • MOUSE WHEEL / SCROLLBAR • L / ESC = CLOSE")
		selectObject.Call(hdc, old)
	}
}

func drawLeaderboardOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayLeaderboard {
		return
	}
	if leaderboardFilterName(localLeaderboardFilter) == "ENDURANCE" {
		drawEnduranceLocalLeaderboard(hdc, w, hgt)
		return
	}
	ar := arenaRect(w, hgt)
	left := ar.Left + sx(55, w)
	right := ar.Right - sx(55, w)
	top := ar.Top + sy(32, hgt)
	bottom := ar.Bottom - sy(26, hgt)
	baseFace, themeAccent, themeDark := themedHUDPanelPalette(true)
	headerBG := blendColor(themeDark, baseFace, .30)
	rowA := blendColor(baseFace, themeDark, .52)
	rowB := blendColor(baseFace, themeDark, .40)
	cyan := themeAccent
	white := rgb(239, 246, 252)
	muted := rgb(160, 190, 214)
	gold := rgb(242, 192, 55)
	silver := rgb(205, 216, 228)
	bronze := rgb(177, 118, 68)
	drawBevelPanel(hdc, RECT{left, top, right, bottom}, baseFace, themeAccent, themeDark, 4)
	if hudSmallFont == 0 {
		return
	}
	oldSmall, _, _ := selectObject.Call(hdc, hudSmallFont)
	setTextColor.Call(hdc, cyan)
	centeredTextOut(hdc, left, right, top+10, "LOCAL LEADERBOARD // TOP 10")
	selectObject.Call(hdc, oldSmall)
	filterLeft := left + sx(18, w)
	filterRight := right - sx(18, w)
	filterTop := top + sy(51, hgt)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		drawLeaderboardFilterTabs(hdc, leaderboardFilterRects(filterLeft, filterRight, filterTop, w, hgt), localLeaderboardFilter)
		selectObject.Call(hdc, old)
	}
	entries := filteredLocalLeaderboard()
	visible := localLeaderboardVisibleRows(w, hgt)
	localLeaderboardScroll = clampLeaderboardScroll(localLeaderboardScroll, len(entries), visible)
	start := localLeaderboardScroll
	end := start + visible
	if end > len(entries) {
		end = len(entries)
	}
	rows := end - start
	tableLeft := filterLeft
	tableRight := filterRight
	tableTop := filterTop + sy(42, hgt)
	rowH := sy(38, hgt)
	if rowH < 27 {
		rowH = 27
	}
	scrollW := sx(14, w)
	if scrollW < 10 {
		scrollW = 10
	}
	contentRight := tableRight - scrollW - sx(3, w)
	cols := leaderboardColumns(tableLeft, contentRight)
	headers := []string{"#", "NAME", "SCORE", "STREAK", "ACCURACY", "RANK", "DIFFICULTY", "DATE", "TIME"}
	fillSolidRect(hdc, RECT{tableLeft, tableTop, contentRight, tableTop + rowH}, headerBG)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		for i, hdr := range headers {
			drawTableCellText(hdc, cols[i], cols[i+1], tableTop, rowH, hdr, cyan, true)
		}
		saved, _, _ := saveDC.Call(hdc)
		intersectClipRect.Call(hdc, uintptr(tableLeft), uintptr(tableTop+rowH), uintptr(contentRight), uintptr(bottom-sy(35, hgt)))
		for vis := 0; vis < rows; vis++ {
			idx := start + vis
			y := tableTop + rowH*int32(vis+1)
			e := entries[idx]
			bg := rowA
			if idx%2 == 1 {
				bg = rowB
			}
			if idx == 0 {
				bg = rgb(66, 50, 17)
			} else if idx == 1 {
				bg = rgb(38, 48, 62)
			} else if idx == 2 {
				bg = rgb(54, 36, 28)
			}
			if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(localPlayerName())) {
				bg = rgb(5, 55, 66)
			}
			fillSolidRect(hdc, RECT{tableLeft, y, contentRight, y + rowH}, bg)
			pc := white
			if idx == 0 {
				pc = gold
			} else if idx == 1 {
				pc = silver
			} else if idx == 2 {
				pc = bronze
			}
			drawTableCellText(hdc, cols[0], cols[1], y, rowH, fmt.Sprintf("%d", idx+1), pc, true)
			nameColor := white
			if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(localPlayerName())) {
				nameColor = profileNameColour(gameMeta.SelectedNameColour)
			}
			drawTableCellText(hdc, cols[1], cols[2], y, rowH, e.Name, nameColor, false)
			drawTableCellText(hdc, cols[2], cols[3], y, rowH, fmt.Sprintf("%d", e.Score), white, true)
			drawTableCellText(hdc, cols[3], cols[4], y, rowH, fmt.Sprintf("%d", e.Streak), white, true)
			drawTableCellText(hdc, cols[4], cols[5], y, rowH, fmt.Sprintf("%.1f%%", e.Accuracy), white, true)
			drawTableCellText(hdc, cols[5], cols[6], y, rowH, e.Rank, white, true)
			drawTableCellText(hdc, cols[6], cols[7], y, rowH, e.Difficulty, white, true)
			drawTableCellText(hdc, cols[7], cols[8], y, rowH, e.Date, white, true)
			drawTableCellText(hdc, cols[8], cols[9], y, rowH, e.Time, white, true)
		}
		restoreDC.Call(hdc, saved)
		selectObject.Call(hdc, old)
	}
	gridBottom := tableTop + rowH*int32(rows+1)
	maxBottom := bottom - sy(35, hgt)
	if gridBottom > maxBottom {
		gridBottom = maxBottom
	}
	drawLeaderboardGrid(hdc, tableLeft, tableTop, contentRight, gridBottom, cols, rowH, rows)
	track := RECT{contentRight + sx(4, w), tableTop + rowH + 1, tableRight - 1, maxBottom}
	drawLeaderboardScrollbar(hdc, track, len(entries), visible, localLeaderboardScroll, cyan)
	if len(entries) == 0 && hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, tableLeft, contentRight, tableTop+rowH+sy(44, hgt), "NO SCORES IN THIS DIFFICULTY YET")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, left, right, bottom-sy(23, hgt), "SCORES SAVE AUTOMATICALLY • MOUSE WHEEL / SCROLLBAR • L / ESC = CLOSE")
		selectObject.Call(hdc, old)
	}
}

func globalLeaderboardFilterAt(p FPoint, w, hgt int32) int {
	left, right, top, _, _, _, _, _ := globalLeaderboardGeometry(w, hgt)
	filterLeft := left + sx(18, w)
	filterRight := right - sx(18, w)
	filterTop := top + sy(51, hgt)
	for i, r := range leaderboardFilterRects(filterLeft, filterRight, filterTop, w, hgt) {
		if pointInRect(p, r) {
			return i
		}
	}
	return -1
}

func drawEnduranceGlobalLeaderboard(hdc uintptr, w, hgt int32) {
	left, right, top, bottom, tableLeft, tableRight, tableTop, rowH := globalLeaderboardGeometry(w, hgt)
	baseFace, themeAccent, themeDark := themedHUDPanelPalette(true)
	headerBG := blendColor(themeDark, baseFace, .30)
	rowA := blendColor(baseFace, themeDark, .52)
	rowB := blendColor(baseFace, themeDark, .40)
	cyan := themeAccent
	white := rgb(239, 246, 252)
	muted := rgb(150, 185, 211)
	gold := rgb(242, 192, 55)
	silver := rgb(205, 216, 228)
	bronze := rgb(177, 118, 68)
	green := rgb(70, 220, 125)
	drawBevelPanel(hdc, RECT{left, top, right, bottom}, baseFace, themeAccent, themeDark, 4)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, cyan)
		centeredTextOut(hdc, left, right, top+10, "GLOBAL ENDURANCE // "+leaderboardScopeName(globalLeaderboardScope))
		selectObject.Call(hdc, old)
	}
	filterLeft := left + sx(18, w)
	filterRight := right - sx(18, w)
	filterTop := top + sy(51, hgt)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		drawLeaderboardFilterTabs(hdc, leaderboardFilterRects(filterLeft, filterRight, filterTop, w, hgt), globalLeaderboardFilter)
		drawGlobalLeaderboardScopeTabs(hdc, filterLeft, filterRight, top+sy(20, hgt), w, hgt)
		if globalLeaderboardScope == leaderboardScopeWeekly {
			setTextColor.Call(hdc, rgb(242, 192, 55))
			text := competitionResetCountdownText()
			textOut(hdc, filterRight-textPixelSize(hdc, hudTinyFont, text).Cx, top+sy(23, hgt), text)
		}
		selectObject.Call(hdc, old)
	}
	entries := globalFilteredEntries()
	visible := globalLeaderboardVisibleRows(w, hgt)
	globalLeaderboardScroll = clampLeaderboardScroll(globalLeaderboardScroll, len(entries), visible)
	start := globalLeaderboardScroll
	end := start + visible
	if end > len(entries) {
		end = len(entries)
	}
	rows := end - start
	pinnedTop := bottom - sy(66, hgt)
	pinnedHeaderTop := pinnedTop - rowH
	maxBottom := pinnedHeaderTop - sy(8, hgt)
	scrollW := sx(14, w)
	if scrollW < 10 {
		scrollW = 10
	}
	contentRight := tableRight - scrollW - sx(3, w)
	total := contentRight - tableLeft
	cols := []int32{tableLeft, tableLeft + total*6/100, tableLeft + total*36/100, tableLeft + total*56/100, tableLeft + total*70/100, tableLeft + total*84/100, contentRight}
	headers := []string{"#", "PLAYER", "DISTANCE", "TARGETS", "ACC", "RANK"}
	fillSolidRect(hdc, RECT{tableLeft, tableTop, contentRight, tableTop + rowH}, headerBG)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		for i, h := range headers {
			drawTableCellText(hdc, cols[i], cols[i+1], tableTop, rowH, h, cyan, true)
		}
		saved, _, _ := saveDC.Call(hdc)
		intersectClipRect.Call(hdc, uintptr(tableLeft), uintptr(tableTop+rowH), uintptr(contentRight), uintptr(maxBottom))
		for vis := 0; vis < rows; vis++ {
			idx := start + vis
			y := tableTop + rowH*int32(vis+1)
			e := entries[idx]
			bg := rowA
			if idx%2 == 1 {
				bg = rowB
			}
			if idx == 0 {
				bg = rgb(66, 50, 17)
			} else if idx == 1 {
				bg = rgb(38, 48, 62)
			} else if idx == 2 {
				bg = rgb(54, 36, 28)
			}
			if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(localPlayerName())) {
				bg = rgb(5, 55, 66)
			}
			fillSolidRect(hdc, RECT{tableLeft, y, contentRight, y + rowH}, bg)
			pc := white
			if idx == 0 {
				pc = gold
			} else if idx == 1 {
				pc = silver
			} else if idx == 2 {
				pc = bronze
			}
			drawTableCellText(hdc, cols[0], cols[1], y, rowH, fmt.Sprintf("%d", e.Position), pc, true)
			nameColor := profileNameColour(e.NameColour)
			if e.NameColour == 0 {
				nameColor = cyan
			}
			drawTableCellText(hdc, cols[1], cols[2], y, rowH, cleanDiscordDisplayName(e.Name)+"  >", nameColor, false)
			drawTableCellText(hdc, cols[2], cols[3], y, rowH, fmt.Sprintf("%.1f m", e.Distance), cyan, true)
			drawTableCellText(hdc, cols[3], cols[4], y, rowH, fmt.Sprintf("%d", e.TargetsHit), white, true)
			drawTableCellText(hdc, cols[4], cols[5], y, rowH, fmt.Sprintf("%.1f%%", e.Accuracy), white, true)
			drawTableCellText(hdc, cols[5], cols[6], y, rowH, e.Rank, white, true)
		}
		restoreDC.Call(hdc, saved)
		for _, x := range cols {
			drawLineSimple(hdc, x, tableTop, x, tableTop+rowH*int32(rows+1), 1, rgb(20, 72, 106))
		}
		for i := 0; i <= rows+1; i++ {
			y := tableTop + rowH*int32(i)
			if y > maxBottom {
				y = maxBottom
			}
			drawLineSimple(hdc, tableLeft, y, contentRight, y, 1, rgb(20, 72, 106))
		}
		fillSolidRect(hdc, RECT{tableLeft, pinnedHeaderTop, contentRight, pinnedHeaderTop + rowH}, rgb(5, 44, 54))
		setTextColor.Call(hdc, green)
		textOut(hdc, tableLeft+sx(10, w), pinnedHeaderTop+3, "YOUR GLOBAL ENDURANCE PB")
		fillSolidRect(hdc, RECT{tableLeft, pinnedTop, contentRight, pinnedTop + rowH}, rgb(7, 36, 42))
		serverEntry, ok := localPlayerGlobalEntry()
		pos := "--"
		if ok && serverEntry.Position > 0 {
			pos = fmt.Sprintf("%d", serverEntry.Position)
		}
		drawTableCellText(hdc, cols[0], cols[1], pinnedTop, rowH, pos, green, true)
		drawTableCellText(hdc, cols[1], cols[2], pinnedTop, rowH, cleanDiscordDisplayName(localPlayerName())+"  (YOU)", profileNameColour(gameMeta.SelectedNameColour), false)
		if ok {
			drawTableCellText(hdc, cols[2], cols[3], pinnedTop, rowH, fmt.Sprintf("%.1f m", serverEntry.Distance), green, true)
			drawTableCellText(hdc, cols[3], cols[4], pinnedTop, rowH, fmt.Sprintf("%d", serverEntry.TargetsHit), white, true)
			drawTableCellText(hdc, cols[4], cols[5], pinnedTop, rowH, fmt.Sprintf("%.1f%%", serverEntry.Accuracy), white, true)
			drawTableCellText(hdc, cols[5], cols[6], pinnedTop, rowH, serverEntry.Rank, white, true)
		}
		selectObject.Call(hdc, old)
	}
	track := RECT{contentRight + sx(4, w), tableTop + rowH + 1, tableRight - 1, maxBottom}
	drawLeaderboardScrollbar(hdc, track, len(entries), visible, globalLeaderboardScroll, cyan)
	if len(entries) == 0 && hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, left, right, tableTop+sy(120, hgt), "NO GLOBAL ENDURANCE RUNS YET")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, left, right, bottom-sy(20, hgt), "RANKED BY DISTANCE • MOUSE WHEEL / SCROLLBAR • G / ESC = CLOSE")
		selectObject.Call(hdc, old)
	}
}

func competitionGuidePanelRect(left, right, top, bottom int32, w, hgt int32) RECT {
	pad := sx(70, w)
	return RECT{left + pad, top + sy(74, hgt), right - pad, bottom - sy(54, hgt)}
}

func drawCompetitionGuidePanel(hdc uintptr, left, right, top, bottom, w, hgt int32) {
	if !competitionGuideOpen {
		return
	}
	r := competitionGuidePanelRect(left, right, top, bottom, w, hgt)
	drawBevelPanel(hdc, r, rgb(8, 18, 42), rgb(192, 111, 255), rgb(2, 7, 20), 4)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, rgb(247, 220, 87))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(14, hgt), "WEEKLY PRECISION COMPETITION")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, hudTinyFont)
	cards := []struct {
		title string
		lines []string
	}{{"BASE POINTS", []string{"EASY 10   •   NORMAL 25   •   HARD 60   •   INSANE 150", "EVERY SUCCESSFUL PRECISION COMPLETION ADDS POINTS TO YOUR WEEKLY TOTAL."}}, {"STREAK MULTIPLIER", []string{"3: ×1.05   5: ×1.10   7: ×1.15   10: ×1.20   15+: ×1.25 MAX", "EACH DIFFICULTY HAS ITS OWN STREAK. FAIL / TIME OUT / QUIT / RESTART BREAKS ONLY THAT DIFFICULTY."}}, {"WEEKLY REWARDS", []string{"1ST: 5000 EXP + 1500 SPACECOINS   •   2ND: 3000 + 750   •   3RD: 1500 + 400", "RESET: SUNDAY 00:00 SERVER TIME. EXP IS AUTO-CREDITED; SPACECOINS ARRIVE AUTOMATICALLY ON YOUR NEXT CONNECTION."}}}
	gap := sy(10, hgt)
	cardTop := r.Top + sy(48, hgt)
	cardH := (r.Bottom - cardTop - sy(18, hgt) - gap*2) / 3
	for i, c := range cards {
		cr := RECT{r.Left + sx(18, w), cardTop + int32(i)*(cardH+gap), r.Right - sx(18, w), cardTop + int32(i)*(cardH+gap) + cardH}
		drawBevelPanel(hdc, cr, rgb(7, 31, 62), rgb(65, 177, 228), rgb(2, 13, 31), 2)
		setTextColor.Call(hdc, rgb(242, 192, 55))
		centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(10, hgt), c.title)
		setTextColor.Call(hdc, rgb(222, 240, 252))
		for j, line := range c.lines {
			fit := fitTextEllipsis(hdc, hudTinyFont, line, cr.Right-cr.Left-sx(24, w))
			centeredTextOut(hdc, cr.Left+sx(10, w), cr.Right-sx(10, w), cr.Top+sy(float64(35+24*j), hgt), fit)
		}
	}
	selectObject.Call(hdc, old)
}

func drawPrecisionCompetitionLeaderboard(hdc uintptr, w, hgt int32) {
	left, right, top, bottom, tableLeft, tableRight, tableTop, rowH := globalLeaderboardGeometry(w, hgt)
	baseFace, themeAccent, themeDark := themedHUDPanelPalette(true)
	cyan := themeAccent
	white := rgb(239, 246, 252)
	muted := rgb(150, 185, 211)
	gold := rgb(247, 195, 62)
	silver := rgb(205, 216, 228)
	bronze := rgb(177, 118, 68)
	headerBG := blendColor(themeDark, baseFace, .30)
	rowA := blendColor(baseFace, themeDark, .52)
	rowB := blendColor(baseFace, themeDark, .40)
	drawBevelPanel(hdc, RECT{left, top, right, bottom}, baseFace, themeAccent, themeDark, 4)

	globalMu.Lock()
	view := competitionView
	current := append([]PrecisionCompetitionEntry(nil), precisionCompetitionEntries...)
	previous := append([]PrecisionCompetitionEntry(nil), precisionCompetitionPrevious...)
	alltime := append([]CompetitionAllTimeEntry(nil), precisionCompetitionAllTime...)
	statusText := competitiveLeaderboardStatus
	loading := competitiveLeaderboardLoading
	historyWeek := competitionHistoryWeekKey
	globalMu.Unlock()

	title := "WEEKLY PRECISION COMPETITION // " + competitionViewName(view)
	if view == competitionViewPrevious && strings.TrimSpace(historyWeek) != "" {
		title += " // " + historyWeek
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, rgb(226, 158, 255))
		centeredTextOut(hdc, left, right, top+10, title)
		selectObject.Call(hdc, old)
	}
	filterLeft := left + sx(18, w)
	filterRight := right - sx(18, w)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		drawGlobalLeaderboardScopeTabs(hdc, filterLeft, filterRight, top+sy(20, hgt), w, hgt)
		drawCompetitionViewTabs(hdc, filterLeft, filterRight, top+sy(51, hgt), w, hgt)
		setTextColor.Call(hdc, gold)
		text := competitionResetCountdownText()
		sz := textPixelSize(hdc, hudTinyFont, text)
		textOut(hdc, filterRight-sz.Cx-sx(94, w), top+sy(23, hgt), text)
		selectObject.Call(hdc, old)
	}

	entries := globalFilteredEntries()
	visible := globalLeaderboardVisibleRows(w, hgt)
	globalLeaderboardScroll = clampLeaderboardScroll(globalLeaderboardScroll, len(entries), visible)
	start := globalLeaderboardScroll
	end := start + visible
	if end > len(entries) {
		end = len(entries)
	}
	rows := end - start
	pinnedTop := bottom - sy(66, hgt)
	maxBottom := pinnedTop - rowH - sy(8, hgt)
	scrollW := sx(14, w)
	if scrollW < 10 {
		scrollW = 10
	}
	contentRight := tableRight - scrollW - sx(3, w)
	total := contentRight - tableLeft

	var cols []int32
	var headers []string
	switch view {
	case competitionViewPrevious:
		cols = []int32{tableLeft, tableLeft + total*7/100, tableLeft + total*44/100, tableLeft + total*64/100, contentRight}
		headers = []string{"#", "PLAYER", "TOTAL POINTS", "AUTOMATIC REWARD"}
	case competitionViewAllTime:
		cols = []int32{tableLeft, tableLeft + total*6/100, tableLeft + total*38/100, tableLeft + total*52/100, tableLeft + total*66/100, tableLeft + total*76/100, tableLeft + total*86/100, contentRight}
		headers = []string{"#", "PLAYER", "WINS", "PODIUMS", "1ST", "2ND", "3RD"}
	default:
		cols = []int32{tableLeft, tableLeft + total*6/100, tableLeft + total*35/100, tableLeft + total*47/100, tableLeft + total*59/100, tableLeft + total*71/100, tableLeft + total*83/100, contentRight}
		headers = []string{"#", "PLAYER", "EASY", "NORMAL", "HARD", "INSANE", "TOTAL POINTS"}
	}
	fillSolidRect(hdc, RECT{tableLeft, tableTop, contentRight, tableTop + rowH}, headerBG)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		for i, h := range headers {
			drawTableCellText(hdc, cols[i], cols[i+1], tableTop, rowH, h, cyan, true)
		}
		saved, _, _ := saveDC.Call(hdc)
		intersectClipRect.Call(hdc, uintptr(tableLeft), uintptr(tableTop+rowH), uintptr(contentRight), uintptr(maxBottom))
		for vis := 0; vis < rows; vis++ {
			idx := start + vis
			y := tableTop + rowH*int32(vis+1)
			bg := rowA
			if idx%2 == 1 {
				bg = rowB
			}
			if idx == 0 {
				bg = rgb(66, 50, 17)
			} else if idx == 1 {
				bg = rgb(38, 48, 62)
			} else if idx == 2 {
				bg = rgb(54, 36, 28)
			}
			var name string
			var nameColour int
			var pos int
			if view == competitionViewAllTime {
				if idx >= len(alltime) {
					break
				}
				e := alltime[idx]
				name, nameColour, pos = e.Name, e.NameColour, e.Position
			} else if view == competitionViewPrevious {
				if idx >= len(previous) {
					break
				}
				e := previous[idx]
				name, nameColour, pos = e.Name, e.NameColour, e.Position
			} else {
				if idx >= len(current) {
					break
				}
				e := current[idx]
				name, nameColour, pos = e.Name, e.NameColour, e.Position
			}
			if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(localPlayerName())) {
				bg = rgb(5, 55, 66)
			}
			fillSolidRect(hdc, RECT{tableLeft, y, contentRight, y + rowH}, bg)
			pc := white
			if idx == 0 {
				pc = gold
			} else if idx == 1 {
				pc = silver
			} else if idx == 2 {
				pc = bronze
			}
			drawTableCellText(hdc, cols[0], cols[1], y, rowH, fmt.Sprintf("%d", pos), pc, true)
			nc := profileNameColour(nameColour)
			if nameColour == 0 {
				nc = cyan
			}
			drawTableCellText(hdc, cols[1], cols[2], y, rowH, name+"  >", nc, false)
			switch view {
			case competitionViewPrevious:
				e := previous[idx]
				drawTableCellText(hdc, cols[2], cols[3], y, rowH, fmt.Sprintf("%d", e.TotalPoints), gold, true)
				reward := fmt.Sprintf("%d XP + %d SC", e.EXPReward, e.SpaceCoinsReward)
				drawTableCellText(hdc, cols[3], cols[4], y, rowH, reward, gold, true)
			case competitionViewAllTime:
				e := alltime[idx]
				drawTableCellText(hdc, cols[2], cols[3], y, rowH, fmt.Sprintf("%d", e.Wins), gold, true)
				drawTableCellText(hdc, cols[3], cols[4], y, rowH, fmt.Sprintf("%d", e.Podiums), white, true)
				drawTableCellText(hdc, cols[4], cols[5], y, rowH, fmt.Sprintf("%d", e.Firsts), gold, true)
				drawTableCellText(hdc, cols[5], cols[6], y, rowH, fmt.Sprintf("%d", e.Seconds), silver, true)
				drawTableCellText(hdc, cols[6], cols[7], y, rowH, fmt.Sprintf("%d", e.Thirds), bronze, true)
			default:
				e := current[idx]
				drawTableCellText(hdc, cols[2], cols[3], y, rowH, fmt.Sprintf("%d", e.EasyPoints), white, true)
				drawTableCellText(hdc, cols[3], cols[4], y, rowH, fmt.Sprintf("%d", e.NormalPoints), white, true)
				drawTableCellText(hdc, cols[4], cols[5], y, rowH, fmt.Sprintf("%d", e.HardPoints), white, true)
				drawTableCellText(hdc, cols[5], cols[6], y, rowH, fmt.Sprintf("%d", e.InsanePoints), white, true)
				drawTableCellText(hdc, cols[6], cols[7], y, rowH, fmt.Sprintf("%d", e.TotalPoints), gold, true)
			}
		}
		restoreDC.Call(hdc, saved)
		for _, x := range cols {
			drawLineSimple(hdc, x, tableTop, x, tableTop+rowH*int32(rows+1), 1, rgb(20, 72, 106))
		}
		for i := 0; i <= rows+1; i++ {
			y := tableTop + rowH*int32(i)
			if y > maxBottom {
				y = maxBottom
			}
			drawLineSimple(hdc, tableLeft, y, contentRight, y, 1, rgb(20, 72, 106))
		}
		selectObject.Call(hdc, old)
	}
	track := RECT{contentRight + sx(4, w), tableTop + rowH + 1, tableRight - 1, maxBottom}
	drawLeaderboardScrollbar(hdc, track, len(entries), visible, globalLeaderboardScroll, rgb(192, 111, 255))
	if len(entries) == 0 && hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		if loading {
			setTextColor.Call(hdc, cyan)
		} else {
			setTextColor.Call(hdc, muted)
		}
		if statusText == "" {
			switch view {
			case competitionViewPrevious:
				statusText = "NO PREVIOUS COMPETITION WINNERS YET"
			case competitionViewAllTime:
				statusText = "NO WEEKLY WINNERS YET"
			default:
				statusText = "NO COMPETITION SCORES YET"
			}
		}
		centeredTextOut(hdc, left, right, tableTop+sy(120, hgt), statusText)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		footer := "PRECISION POINTS ACCUMULATE ALL WEEK • MOUSE WHEEL / SCROLLBAR • GUIDE FOR RULES"
		if view == competitionViewPrevious {
			footer = "FINAL TOP 3 FROM THE PREVIOUS WEEK • REWARDS ARE CREDITED AUTOMATICALLY"
		} else if view == competitionViewAllTime {
			footer = "ALL-TIME WEEKLY CHAMPIONS • RANKED BY 1ST-PLACE WINS, THEN TOTAL PODIUMS"
		}
		centeredTextOut(hdc, left, right, bottom-sy(20, hgt), footer)
		selectObject.Call(hdc, old)
	}
	drawCompetitionGuidePanel(hdc, left, right, top, bottom, w, hgt)
}

func drawGlobalLeaderboardOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayGlobalLeaderboard {
		return
	}
	if globalLeaderboardScope == leaderboardScopeCompetition {
		drawPrecisionCompetitionLeaderboard(hdc, w, hgt)
		return
	}
	if leaderboardFilterName(globalLeaderboardFilter) == "ENDURANCE" {
		drawEnduranceGlobalLeaderboard(hdc, w, hgt)
		return
	}
	left, right, top, bottom, tableLeft, tableRight, tableTop, rowH := globalLeaderboardGeometry(w, hgt)
	baseFace, themeAccent, themeDark := themedHUDPanelPalette(true)
	headerBG := blendColor(themeDark, baseFace, .30)
	rowA := blendColor(baseFace, themeDark, .52)
	rowB := blendColor(baseFace, themeDark, .40)
	cyan := themeAccent
	white := rgb(239, 246, 252)
	muted := rgb(150, 185, 211)
	gold := rgb(247, 195, 62)
	silver := rgb(205, 216, 228)
	bronze := rgb(177, 118, 68)
	green := rgb(70, 220, 125)
	drawBevelPanel(hdc, RECT{left, top, right, bottom}, baseFace, themeAccent, themeDark, 4)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, cyan)
		centeredTextOut(hdc, left, right, top+10, "GLOBAL LEADERBOARD // "+leaderboardScopeName(globalLeaderboardScope))
		selectObject.Call(hdc, old)
	}
	filterLeft := left + sx(18, w)
	filterRight := right - sx(18, w)
	filterTop := top + sy(51, hgt)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		drawLeaderboardFilterTabs(hdc, leaderboardFilterRects(filterLeft, filterRight, filterTop, w, hgt), globalLeaderboardFilter)
		drawGlobalLeaderboardScopeTabs(hdc, filterLeft, filterRight, top+sy(20, hgt), w, hgt)
		if globalLeaderboardScope == leaderboardScopeWeekly {
			setTextColor.Call(hdc, gold)
			text := competitionResetCountdownText()
			sz := textPixelSize(hdc, hudTinyFont, text)
			textOut(hdc, filterRight-sz.Cx, top+sy(23, hgt), text)
		}
		selectObject.Call(hdc, old)
	}
	filterName := leaderboardFilterName(globalLeaderboardFilter)
	overall := filterName == "OVERALL"
	entries := globalFilteredEntries()
	visible := globalLeaderboardVisibleRows(w, hgt)
	globalLeaderboardScroll = clampLeaderboardScroll(globalLeaderboardScroll, len(entries), visible)
	start := globalLeaderboardScroll
	end := start + visible
	if end > len(entries) {
		end = len(entries)
	}
	rows := end - start
	pinnedTop := bottom - sy(66, hgt)
	pinnedHeaderTop := pinnedTop - rowH
	maxBottom := pinnedHeaderTop - sy(8, hgt)
	scrollW := sx(14, w)
	if scrollW < 10 {
		scrollW = 10
	}
	contentRight := tableRight - scrollW - sx(3, w)
	total := contentRight - tableLeft
	var cols []int32
	var headers []string
	if overall {
		cols = []int32{tableLeft, tableLeft + total*6/100, tableLeft + total*35/100, tableLeft + total*48/100, tableLeft + total*59/100, tableLeft + total*69/100, tableLeft + total*79/100, tableLeft + total*89/100, contentRight}
		headers = []string{"#", "PLAYER", "RANK", "EASY", "NORMAL", "HARD", "INSANE", "TOTAL"}
	} else {
		cols = []int32{tableLeft, tableLeft + total*6/100, tableLeft + total*35/100, tableLeft + total*54/100, tableLeft + total*68/100, tableLeft + total*82/100, contentRight}
		headers = []string{"#", "PLAYER", "SCORE", "STREAK", "ACC", "RANK"}
	}
	fillSolidRect(hdc, RECT{tableLeft, tableTop, contentRight, tableTop + rowH}, headerBG)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		for i, h := range headers {
			drawTableCellText(hdc, cols[i], cols[i+1], tableTop, rowH, h, cyan, true)
		}
		saved, _, _ := saveDC.Call(hdc)
		intersectClipRect.Call(hdc, uintptr(tableLeft), uintptr(tableTop+rowH), uintptr(contentRight), uintptr(maxBottom))
		for vis := 0; vis < rows; vis++ {
			idx := start + vis
			y := tableTop + rowH*int32(vis+1)
			e := entries[idx]
			bg := rowA
			if idx%2 == 1 {
				bg = rowB
			}
			if e.Position == 1 {
				bg = rgb(66, 50, 17)
			} else if e.Position == 2 {
				bg = rgb(38, 48, 62)
			} else if e.Position == 3 {
				bg = rgb(54, 36, 28)
			}
			if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(localPlayerName())) {
				bg = rgb(5, 55, 66)
			}
			fillSolidRect(hdc, RECT{tableLeft, y, contentRight, y + rowH}, bg)
			pc := white
			if e.Position == 1 {
				pc = gold
			} else if e.Position == 2 {
				pc = silver
			} else if e.Position == 3 {
				pc = bronze
			}
			drawTableCellText(hdc, cols[0], cols[1], y, rowH, fmt.Sprintf("%d", e.Position), pc, true)
			nc := profileNameColour(e.NameColour)
			if e.NameColour == 0 {
				nc = cyan
			}
			drawTableCellText(hdc, cols[1], cols[2], y, rowH, cleanDiscordDisplayName(e.Name)+"  >", nc, false)
			if overall {
				drawTableCellText(hdc, cols[2], cols[3], y, rowH, e.Rank, white, true)
				drawTableCellText(hdc, cols[3], cols[4], y, rowH, fmt.Sprintf("%d", e.EasyClears), white, true)
				drawTableCellText(hdc, cols[4], cols[5], y, rowH, fmt.Sprintf("%d", e.NormalClears), white, true)
				drawTableCellText(hdc, cols[5], cols[6], y, rowH, fmt.Sprintf("%d", e.HardClears), white, true)
				drawTableCellText(hdc, cols[6], cols[7], y, rowH, fmt.Sprintf("%d", e.InsaneClears), white, true)
				drawTableCellText(hdc, cols[7], cols[8], y, rowH, fmt.Sprintf("%d", e.TotalClears), gold, true)
			} else {
				drawTableCellText(hdc, cols[2], cols[3], y, rowH, fmt.Sprintf("%d", e.Score), white, true)
				drawTableCellText(hdc, cols[3], cols[4], y, rowH, fmt.Sprintf("%d", e.Streak), white, true)
				drawTableCellText(hdc, cols[4], cols[5], y, rowH, fmt.Sprintf("%.1f%%", e.Accuracy), white, true)
				drawTableCellText(hdc, cols[5], cols[6], y, rowH, e.Rank, white, true)
			}
		}
		restoreDC.Call(hdc, saved)
		for _, x := range cols {
			drawLineSimple(hdc, x, tableTop, x, tableTop+rowH*int32(rows+1), 1, rgb(20, 72, 106))
		}
		for i := 0; i <= rows+1; i++ {
			y := tableTop + rowH*int32(i)
			if y > maxBottom {
				y = maxBottom
			}
			drawLineSimple(hdc, tableLeft, y, contentRight, y, 1, rgb(20, 72, 106))
		}
		fillSolidRect(hdc, RECT{tableLeft, pinnedHeaderTop, contentRight, pinnedHeaderTop + rowH}, rgb(5, 44, 54))
		setTextColor.Call(hdc, green)
		textOut(hdc, tableLeft+sx(10, w), pinnedHeaderTop+3, "YOUR GLOBAL PROGRESS")
		fillSolidRect(hdc, RECT{tableLeft, pinnedTop, contentRight, pinnedTop + rowH}, rgb(7, 36, 42))
		serverEntry, has := localPlayerGlobalEntry()
		pos := "--"
		if has && serverEntry.Position > 0 {
			pos = fmt.Sprintf("%d", serverEntry.Position)
		}
		drawTableCellText(hdc, cols[0], cols[1], pinnedTop, rowH, pos, green, true)
		drawTableCellText(hdc, cols[1], cols[2], pinnedTop, rowH, cleanDiscordDisplayName(localPlayerName())+"  (YOU)", profileNameColour(gameMeta.SelectedNameColour), false)
		if overall && has {
			drawTableCellText(hdc, cols[2], cols[3], pinnedTop, rowH, serverEntry.Rank, green, true)
			drawTableCellText(hdc, cols[3], cols[4], pinnedTop, rowH, fmt.Sprintf("%d", serverEntry.EasyClears), white, true)
			drawTableCellText(hdc, cols[4], cols[5], pinnedTop, rowH, fmt.Sprintf("%d", serverEntry.NormalClears), white, true)
			drawTableCellText(hdc, cols[5], cols[6], pinnedTop, rowH, fmt.Sprintf("%d", serverEntry.HardClears), white, true)
			drawTableCellText(hdc, cols[6], cols[7], pinnedTop, rowH, fmt.Sprintf("%d", serverEntry.InsaneClears), white, true)
			drawTableCellText(hdc, cols[7], cols[8], pinnedTop, rowH, fmt.Sprintf("%d", serverEntry.TotalClears), gold, true)
		} else if !overall && has {
			drawTableCellText(hdc, cols[2], cols[3], pinnedTop, rowH, fmt.Sprintf("%d", serverEntry.Score), white, true)
			drawTableCellText(hdc, cols[3], cols[4], pinnedTop, rowH, fmt.Sprintf("%d", serverEntry.Streak), white, true)
			drawTableCellText(hdc, cols[4], cols[5], pinnedTop, rowH, fmt.Sprintf("%.1f%%", serverEntry.Accuracy), white, true)
			drawTableCellText(hdc, cols[5], cols[6], pinnedTop, rowH, serverEntry.Rank, green, true)
		}
		selectObject.Call(hdc, old)
	}
	track := RECT{contentRight + sx(4, w), tableTop + rowH + 1, tableRight - 1, maxBottom}
	drawLeaderboardScrollbar(hdc, track, len(entries), visible, globalLeaderboardScroll, cyan)
	globalMu.Lock()
	statusText := globalLeaderboardStatus
	loading := globalLeaderboardLoading
	if globalLeaderboardScope != leaderboardScopeTop20 {
		statusText = competitiveLeaderboardStatus
		loading = competitiveLeaderboardLoading
	}
	globalMu.Unlock()
	if len(entries) == 0 && hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		if loading {
			setTextColor.Call(hdc, cyan)
		} else {
			setTextColor.Call(hdc, muted)
		}
		if statusText == "" {
			statusText = "NO GLOBAL SCORES YET"
		}
		centeredTextOut(hdc, left, right, tableTop+sy(120, hgt), statusText)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, left, right, bottom-sy(20, hgt), "CLICK A PLAYER • MOUSE WHEEL / SCROLLBAR • G / ESC = CLOSE")
		selectObject.Call(hdc, old)
	}
}

func selectedGlobalEntry() (GlobalLeaderboardEntry, bool) {
	if selectedGlobalOverrideOn {
		return selectedGlobalOverride, true
	}
	entries := globalFilteredEntries()
	if selectedGlobalPlayer < 0 || selectedGlobalPlayer >= len(entries) {
		return GlobalLeaderboardEntry{}, false
	}
	return entries[selectedGlobalPlayer], true
}

func globalEntryByName(entries []GlobalLeaderboardEntry, name string) (GlobalLeaderboardEntry, bool) {
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(name)) {
			return e, true
		}
	}
	return GlobalLeaderboardEntry{}, false
}

func profilePositions(name string) map[string]int {
	globalMu.Lock()
	defer globalMu.Unlock()
	out := map[string]int{"OVERALL": 0, "EASY": 0, "NORMAL": 0, "HARD": 0, "INSANE": 0, "ENDURANCE": 0}
	sets := map[string][]GlobalLeaderboardEntry{"OVERALL": globalLeaderboardOverall, "EASY": globalLeaderboardEasy, "NORMAL": globalLeaderboardNormal, "HARD": globalLeaderboardHard, "INSANE": globalLeaderboardInsane, "ENDURANCE": globalLeaderboardEndurance}
	for k, list := range sets {
		for i, e := range list {
			if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(name)) {
				if e.Position > 0 {
					out[k] = e.Position
				} else {
					out[k] = i + 1
				}
				break
			}
		}
	}
	return out
}

func profilePBs(name string) map[string]GlobalLeaderboardEntry {
	globalMu.Lock()
	defer globalMu.Unlock()
	out := map[string]GlobalLeaderboardEntry{}
	sets := map[string][]GlobalLeaderboardEntry{"EASY": globalLeaderboardEasy, "NORMAL": globalLeaderboardNormal, "HARD": globalLeaderboardHard, "INSANE": globalLeaderboardInsane, "ENDURANCE": globalLeaderboardEndurance}
	for k, list := range sets {
		for _, e := range list {
			if strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(name)) {
				out[k] = e
				break
			}
		}
	}
	return out
}

func profileDataPBs(p RemoteProfileData) map[string]GlobalLeaderboardEntry {
	out := map[string]GlobalLeaderboardEntry{}
	for _, m := range p.Scores {
		e := globalEntryFromMap(m, 0)
		d := strings.ToUpper(strings.TrimSpace(e.Difficulty))
		if d != "" {
			out[d] = e
		}
	}
	return out
}

func drawProfileAvatar(hdc uintptr, r RECT, data []byte, iw, ih int32, name string) {
	drawBevelPanel(hdc, r, rgb(4, 31, 69), rgb(59, 220, 255), rgb(2, 14, 35), 3)
	inner := RECT{r.Left + 4, r.Top + 4, r.Right - 4, r.Bottom - 4}
	if len(data) > 0 && iw > 0 && ih > 0 {
		drawRawBGRAFit(hdc, data, iw, ih, inner)
		return
	}
	fillSolidRect(hdc, inner, rgb(12, 52, 100))
	initial := "?"
	n := strings.TrimSpace(name)
	if n != "" {
		initial = strings.ToUpper(string([]rune(n)[0]))
	}
	if hudStatFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudStatFont)
		setTextColor.Call(hdc, rgb(240, 248, 255))
		centeredTextOut(hdc, inner.Left, inner.Right, verticallyCenteredTextY(hdc, hudTitleFont, inner, initial), initial)
		selectObject.Call(hdc, old)
	}
}

func drawProfileAvatarFlat(hdc uintptr, r RECT, data []byte, iw, ih int32, name string) {
	// v347: Global Profile avatar uses a clean square frame with no bevel/drop
	// shadow so themed avatar overlays can sit flush against it later.
	fillSolidRect(hdc, r, rgb(4, 31, 69))
	drawOutlineRect(hdc, r, rgb(59, 220, 255), 2)
	inner := RECT{r.Left + 3, r.Top + 3, r.Right - 3, r.Bottom - 3}
	if len(data) > 0 && iw > 0 && ih > 0 {
		drawRawBGRAFit(hdc, data, iw, ih, inner)
		return
	}
	fillSolidRect(hdc, inner, rgb(12, 52, 100))
	initial := "?"
	n := strings.TrimSpace(name)
	if n != "" {
		initial = strings.ToUpper(string([]rune(n)[0]))
	}
	if hudStatFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudStatFont)
		setTextColor.Call(hdc, rgb(240, 248, 255))
		centeredTextOut(hdc, inner.Left, inner.Right, verticallyCenteredTextY(hdc, hudTitleFont, inner, initial), initial)
		selectObject.Call(hdc, old)
	}
}

func profileDifficultyColor(d string) uintptr {
	switch strings.ToUpper(strings.TrimSpace(d)) {
	case "EASY":
		return diffs[0].color
	case "NORMAL":
		return diffs[1].color
	case "HARD":
		return diffs[2].color
	case "INSANE":
		return diffs[3].color
	case "ENDURANCE":
		// Endurance uses the cyan HUD colourway throughout profiles.
		return rgb(80, 231, 255)
	default:
		return rgb(80, 231, 255)
	}
}

func drawShipCollectionStrip(hdc uintptr, r RECT, unlocked []int, selected int, w, hgt int32) {
	drawBevelPanel(hdc, r, rgb(5, 29, 61), rgb(36, 137, 190), rgb(2, 16, 38), 2)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, rgb(80, 231, 255))
		headerBand := RECT{r.Left + sx(14, w), r.Top + sy(4, hgt), r.Right - sx(14, w), r.Top + sy(36, hgt)}
		textOut(hdc, headerBand.Left, verticallyCenteredTextY(hdc, hudSmallFont, headerBand, "SPACESHIP COLLECTION"), "SPACESHIP COLLECTION")
		selectObject.Call(hdc, old)
	}
	owned := map[int]bool{0: true}
	for _, id := range unlocked {
		owned[id] = true
	}
	count := 0
	for _, id := range garageShipOrder {
		if owned[id] {
			count++
		}
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, rgb(174, 208, 232))
		text := fmt.Sprintf("%d / %d UNLOCKED", count, len(garageShipOrder))
		sz := textPixelSize(hdc, hudTinyFont, text)
		headerBand := RECT{r.Left + sx(14, w), r.Top + sy(4, hgt), r.Right - sx(14, w), r.Top + sy(36, hgt)}
		textOut(hdc, headerBand.Right-sz.Cx, verticallyCenteredTextY(hdc, hudTinyFont, headerBand, text), text)
		selectObject.Call(hdc, old)
	}

	// Compact 6 x 2 ship grid. Names are intentionally omitted so the ships
	// remain large and easy to scan at a glance.
	left := r.Left + sx(14, w)
	right := r.Right - sx(14, w)
	top := r.Top + sy(40, hgt)
	bottom := r.Bottom - sy(10, hgt)
	cols := int32(6)
	rows := int32(2)
	gapX := sx(8, w)
	gapY := sy(7, hgt)
	cw := (right - left - gapX*(cols-1)) / cols
	ch := (bottom - top - gapY*(rows-1)) / rows
	for i, id := range garageShipOrder {
		col := int32(i) % cols
		row := int32(i) / cols
		if row >= rows {
			break
		}
		x := left + col*(cw+gapX)
		y := top + row*(ch+gapY)
		cr := RECT{x, y, x + cw, y + ch}
		face := rgb(7, 35, 70)
		border := rgb(38, 108, 151)
		if id == selected {
			face = rgb(16, 74, 111)
			border = rgb(255, 211, 55)
		}
		drawBevelPanel(hdc, cr, face, border, rgb(2, 14, 34), 2)
		img := RECT{cr.Left + sx(7, w), cr.Top + sy(4, hgt), cr.Right - sx(7, w), cr.Bottom - sy(4, hgt)}
		if owned[id] {
			drawShipTextureFit(hdc, id, img)
		} else {
			fillSolidRect(hdc, img, rgb(4, 15, 30))
			if hudSmallFont != 0 {
				old, _, _ := selectObject.Call(hdc, hudSmallFont)
				setTextColor.Call(hdc, rgb(66, 82, 103))
				centeredTextOut(hdc, img.Left, img.Right, verticallyCenteredTextY(hdc, hudSmallFont, img, "?"), "?")
				selectObject.Call(hdc, old)
			}
		}
	}
}

func drawRankingCards(hdc uintptr, r RECT, positions map[string]int, w, hgt int32) {
	labels := []string{"OVERALL", "EASY", "NORMAL", "HARD", "INSANE", "ENDURANCE"}
	gap := sx(7, w)
	cw := (r.Right - r.Left - gap*5) / 6
	for i, l := range labels {
		x := r.Left + int32(i)*(cw+gap)
		cr := RECT{x, r.Top, x + cw, r.Bottom}
		accent := profileDifficultyColor(l)
		if l == "OVERALL" {
			accent = rgb(255, 211, 55)
		}
		drawBevelPanel(hdc, cr, rgb(7, 34, 70), accent, rgb(2, 16, 38), 2)
		fillSolidRect(hdc, RECT{cr.Left + 2, cr.Top + 2, cr.Right - 2, cr.Top + sy(5, hgt)}, accent)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			v := "—"
			if positions[l] > 0 {
				v = fmt.Sprintf("#%d", positions[l])
			}
			labelH := textPixelSize(hdc, hudTinyFont, l).Cy
			valueH := textPixelSize(hdc, hudTinyFont, v).Cy
			if labelH <= 0 {
				labelH = 13
			}
			if valueH <= 0 {
				valueH = 13
			}
			groupGap := sy(7, hgt)
			groupTop := cr.Top + ((cr.Bottom-cr.Top)-(labelH+groupGap+valueH))/2
			setTextColor.Call(hdc, accent)
			centeredTextOut(hdc, cr.Left, cr.Right, groupTop, l)
			setTextColor.Call(hdc, rgb(250, 250, 250))
			centeredTextOut(hdc, cr.Left, cr.Right, groupTop+labelH+groupGap, v)
			selectObject.Call(hdc, old)
		}
	}
}

func drawDifficultyLockedOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayDifficultyLocked {
		return
	}
	drawModalBackdrop(hdc, w, hgt)

	i := lockedDifficultyPopup
	if i < 1 || i > 3 {
		i = 1
	}
	label, current, required := difficultyUnlockRequirement(i)
	remaining := required - current
	if remaining < 0 {
		remaining = 0
	}

	r := centeredPanel(w, hgt, 700, 320)
	drawPanelFrame(hdc, r, "DIFFICULTY LOCKED", "PROGRESSION REQUIRED")

	accent := diffs[i].color
	white := rgb(245, 249, 252)
	muted := rgb(159, 194, 219)
	gold := rgb(255, 202, 47)

	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, accent)
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(92, hgt), diffs[i].name+" IS LOCKED")
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, white)
		centeredTextOut(hdc, r.Left+sx(30, w), r.Right-sx(30, w), r.Top+sy(145, hgt), fmt.Sprintf("You need %d more %s to unlock this difficulty.", remaining, strings.ToLower(label)))
		setTextColor.Call(hdc, gold)
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(190, hgt), fmt.Sprintf("%d / %d  %s", current, required, label))
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, r.Left, r.Right, r.Bottom-sy(52, hgt), "CLICK / ENTER / ESC = CLOSE")
		selectObject.Call(hdc, old)
	}
}

func drawRemoteProfileOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayRemoteProfile {
		return
	}
	drawModalBackdrop(hdc, w, hgt)
	e, ok := selectedGlobalEntry()
	if !ok {
		setOverlay(OverlayGlobalLeaderboard)
		return
	}
	r := centeredPanel(w, hgt, 1180, 840)
	white := rgb(245, 249, 253)

	remoteProfileMu.Lock()
	p := remoteProfile
	loaded := remoteProfileLoaded
	loading := remoteProfileLoading
	status := remoteProfileStatus
	av := append([]byte(nil), avatarAnimationFrame(remoteAvatarAnim, remoteAvatarBGRA)...)
	avw, avh := remoteAvatarW, remoteAvatarH
	remoteProfileMu.Unlock()
	profileFontStyle := 0
	profileNameFontStyle := 0
	profilePrimary := 1
	profileSecondary := 2
	profileShadow := true
	profileShadowColour := 0
	profileGradientVertical := false
	profileAnim := 0
	profileEXP := 0
	if loaded {
		profileFontStyle = p.SelectedProfileFont
		profileNameFontStyle = p.SelectedProfileNameFont
		profilePrimary = p.ProfilePrimaryColour
		profileSecondary = p.ProfileSecondaryColour
		profileShadow = p.ProfileNameShadow
		profileShadowColour = p.ProfileShadowColour
		profileGradientVertical = p.ProfileGradientVertical
		profileAnim = p.ProfileAnimation
		profileEXP = p.EXP
	}
	if remoteProfileIsSelf() {
		profileFontStyle = gameMeta.SelectedProfileFont
		profileNameFontStyle = gameMeta.SelectedProfileNameFont
		profilePrimary = gameMeta.ProfilePrimaryColour
		profileSecondary = gameMeta.ProfileSecondaryColour
		profileShadow = gameMeta.ProfileNameShadow
		profileShadowColour = gameMeta.ProfileShadowColour
		profileGradientVertical = gameMeta.ProfileGradientVertical
		profileAnim = gameMeta.ProfileAnimation
		profileEXP = globalProfileEXPForUnlocks()
	}
	if profileEXP < 4000 {
		profileFontStyle = 0
	}
	if profileEXP < 30000 {
		profileNameFontStyle = 0
		profilePrimary = 1
		profileSecondary = 1
		profileShadow = false
		profileGradientVertical = false
	}
	if profileEXP < 75000 {
		profileAnim = 0
	}
	gpTitle := publicProfileFont(profileFontStyle, 0)
	gpName := publicProfileFont(profileFontStyle, 1)
	gpSmall := publicProfileFont(profileFontStyle, 2)
	gpTiny := publicProfileFont(profileFontStyle, 3)
	themeID := 0
	if loaded {
		themeID = p.SelectedProfileFrame
	}
	if remoteProfileIsSelf() {
		themeID = gameMeta.SelectedProfileFrame
	}
	theme := profileThemeForSkin(themeID)
	cyan := theme.Accent
	muted := theme.Muted
	gold := theme.Gold
	drawBevelPanel(hdc, r, theme.Outer, theme.Accent, theme.Deep, 5)
	name := strings.TrimSuffix(e.Name, "#0")
	rank := e.Rank
	exp := 0
	clears := []int{e.EasyClears, e.NormalClears, e.HardClears, e.InsaneClears, e.TotalClears}
	unlocked := []int{}
	selected := 0
	remoteShowcase := []string{}
	pbs := profilePBs(name)
	if loaded {
		if strings.TrimSpace(p.DisplayName) != "" {
			name = strings.TrimSuffix(p.DisplayName, "#0")
		}
		if strings.TrimSpace(p.EXPRank) != "" {
			rank = p.EXPRank
		}
		exp = p.EXP
		clears = []int{p.EasyClears, p.NormalClears, p.HardClears, p.InsaneClears, p.TotalClears}
		unlocked = p.UnlockedShips
		selected = p.SelectedShip
		remoteShowcase = append([]string(nil), p.AchievementShowcase...)
		for d, v := range profileDataPBs(p) {
			pbs[d] = v
		}
	}
	positions := profilePositions(name)
	if loaded && p.Positions != nil {
		for k, v := range p.Positions {
			if v > 0 {
				positions[strings.ToUpper(strings.TrimSpace(k))] = v
			}
		}
	}

	header := RECT{r.Left + sx(18, w), r.Top + sy(18, hgt), r.Right - sx(18, w), r.Top + sy(78, hgt)}
	fillSolidRect(hdc, header, theme.Panel2)
	drawLineSimple(hdc, header.Left, header.Bottom, header.Right, header.Bottom, 3, cyan)
	if gpTitle != 0 {
		old, _, _ := selectObject.Call(hdc, gpTitle)
		setTextColor.Call(hdc, white)
		textOut(hdc, header.Left+sx(18, w), header.Top+sy(12, hgt), "GLOBAL PLAYER PROFILE")
		selectObject.Call(hdc, old)
	}

	identity := RECT{r.Left + sx(28, w), r.Top + sy(96, hgt), r.Right - sx(28, w), r.Top + sy(270, hgt)}
	skinID := 0
	if loaded || remoteProfileIsSelf() {
		skinID = p.SelectedProfileFrame
		if remoteProfileIsSelf() {
			skinID = gameMeta.SelectedProfileFrame
		}
	}
	// v345: once a profile skin is equipped, the banner itself becomes the identity
	// container. The old bevel panel is only retained for profiles without a skin.
	hasSkin := skinID >= profileSkinFirstID && skinID <= profileSkinLastID && len(profileSkinAsset(skinID)) == int(globalProfileSkinWidth*globalProfileSkinHeight*4)
	if !hasSkin {
		drawBevelPanel(hdc, identity, theme.Panel, theme.Accent, theme.Deep, 3)
	}
	if hasSkin {
		drawGlobalProfileSkin(hdc, identity, skinID, w, hgt)
	}
	if remoteProfileIsSelf() {
		uiButton(hdc, profileSkinEditButtonRect(w, hgt), "EDIT", "", false)
	}
	// v340 compact identity cluster: the profile banner is rendered at a smaller,
	// right-aligned natural-aspect display size for sharper artwork.  Avatar, rank
	// and all identity text move with that banner instead of remaining anchored to
	// the old full-width card.
	skinR := profileSkinDisplayRect(identity, w, hgt)
	// v344 identity grid: avatar is a true square, then a fixed 20px gutter to
	// the rank emblem, then a 22px gutter to the text column. This gives every
	// identity element its own stable lane and provides an exact square target
	// for the upcoming themed avatar-frame overlays.
	avatarSize := sy(132, hgt)
	if sx(132, w) < avatarSize {
		avatarSize = sx(132, w)
	}
	avatarTop := skinR.Top + (skinR.Bottom-skinR.Top-avatarSize)/2 - sy(2, hgt)
	avatarR := RECT{skinR.Left + sx(89, w), avatarTop, skinR.Left + sx(89, w) + avatarSize, avatarTop + avatarSize}
	drawProfileAvatarFlat(hdc, avatarR, av, avw, avh, name)
	badgeSize := sy(148, hgt)
	if sx(148, w) < badgeSize {
		badgeSize = sx(148, w)
	}
	badgeTop := skinR.Top + (skinR.Bottom-skinR.Top-badgeSize)/2
	badgeLeft := avatarR.Right + sx(15, w)
	badge := RECT{badgeLeft, badgeTop, badgeLeft + badgeSize, badgeTop + badgeSize}
	drawRankBadge(hdc, badge, rank)
	nameX := badge.Right + sx(12, w)
	advancedNameDrawn := false
	if profileEXP >= 30000 && profileNameFontStyle > 0 {
		// v381: decorative names use exactly the same origin as the default
		// profile-name font. Different font metrics must not shift the name.
		advancedNameDrawn = drawAdvancedProfileTextAt(hdc, nameX, skinR.Top+sy(28, hgt), skinR.Right-sx(26, w), name, profileNameFontStyle, profilePrimary, profileSecondary, profileGradientVertical, profileShadow, profileShadowColour)
	}
	if !advancedNameDrawn && gpName != 0 {
		if profileEXP >= 30000 {
			drawGradientProfileTextStyled(hdc, nameX, skinR.Top+sy(28, hgt), name, profilePrimary, profileSecondary, gpName, profileGradientVertical, profileShadow, profileShadowColour)
		} else {
			old, _, _ := selectObject.Call(hdc, gpName)
			setBkMode.Call(hdc, TRANSPARENT)
			setTextColor.Call(hdc, rgb(245, 249, 253))
			textOut(hdc, nameX, skinR.Top+sy(28, hgt), name)
			selectObject.Call(hdc, old)
		}
	}
	if loaded && strings.TrimSpace(p.SelectedTitle) != "" && gpTiny != 0 {
		old, _, _ := selectObject.Call(hdc, gpTiny)
		setTextColor.Call(hdc, gold)
		textOut(hdc, nameX, skinR.Top+sy(79, hgt), p.SelectedTitle)
		selectObject.Call(hdc, old)
	}
	if loaded && gpTiny != 0 && (strings.TrimSpace(p.CompetitiveBadge) != "" || strings.TrimSpace(p.SeasonBest) != "") {
		old, _, _ := selectObject.Call(hdc, gpTiny)
		setTextColor.Call(hdc, rgb(255, 211, 69))
		badgeText := strings.TrimSpace(p.CompetitiveBadge)
		if strings.TrimSpace(p.SeasonBest) != "" {
			if badgeText != "" {
				badgeText += "  •  "
			}
			badgeText += strings.TrimSpace(p.SeasonBest)
		}
		badgeText = fitTextEllipsis(hdc, gpTiny, badgeText, identity.Right-nameX-sx(22, w))
		textOut(hdc, nameX, skinR.Top+sy(50, hgt), badgeText)
		selectObject.Call(hdc, old)
	}
	if gpTiny != 0 {
		old, _, _ := selectObject.Call(hdc, gpTiny)
		setTextColor.Call(hdc, profileRankNameColour(rank))
		textOut(hdc, nameX, skinR.Top+sy(103, hgt), rank)
		selectObject.Call(hdc, old)
	}
	if gpTiny != 0 {
		old, _, _ := selectObject.Call(hdc, gpTiny)
		setTextColor.Call(hdc, cyan)
		if exp > 0 {
			textOut(hdc, nameX, skinR.Top+sy(128, hgt), fmt.Sprintf("%d EXP", exp))
		}
		selectObject.Call(hdc, old)
	}
	if gpTiny != 0 {
		old, _, _ := selectObject.Call(hdc, gpTiny)
		setTextColor.Call(hdc, muted)
		if loading {
			textOut(hdc, identity.Right-sx(250, w), identity.Top+sy(22, hgt), "SYNCING PROFILE...")
		} else if status != "" {
			textOut(hdc, identity.Right-sx(280, w), identity.Top+sy(22, hgt), status)
		}
		selectObject.Call(hdc, old)
	}

	rankR := RECT{r.Left + sx(28, w), r.Top + sy(286, hgt), r.Right - sx(28, w), r.Top + sy(352, hgt)}
	drawRankingCardsThemed(hdc, rankR, positions, w, hgt, theme)

	// PB cards use a compact internal grid and each mode keeps the same accent
	// colour as its main-HUD difficulty.
	cardTop := r.Top + sy(370, hgt)
	left := r.Left + sx(28, w)
	right := r.Right - sx(28, w)
	gap := sx(9, w)
	cw := (right - left - gap*4) / 5
	ch := sy(145, hgt)
	diffsList := []string{"EASY", "NORMAL", "HARD", "INSANE", "ENDURANCE"}
	for i, d := range diffsList {
		x := left + int32(i)*(cw+gap)
		cr := RECT{x, cardTop, x + cw, cardTop + ch}
		accent := profileDifficultyColor(d)
		drawBevelPanel(hdc, cr, theme.Panel2, theme.Accent, theme.Deep, 2)
		fillSolidRect(hdc, RECT{cr.Left + 2, cr.Top + 2, cr.Right - 2, cr.Top + sy(5, hgt)}, theme.Accent)
		pe, has := pbs[d]
		if gpSmall != 0 {
			old, _, _ := selectObject.Call(hdc, gpSmall)
			setTextColor.Call(hdc, accent)
			centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(10, hgt), d)
			selectObject.Call(hdc, old)
		}
		gridTop := cr.Top + sy(39, hgt)
		gridBottom := cr.Bottom - sy(8, hgt)
		rowH := (gridBottom - gridTop) / 3
		for row := int32(1); row < 3; row++ {
			drawLineSimple(hdc, cr.Left+sx(7, w), gridTop+row*rowH, cr.Right-sx(7, w), gridTop+row*rowH, 1, theme.Accent2)
		}
		mid := cr.Left + (cr.Right-cr.Left)*54/100
		drawLineSimple(hdc, mid, gridTop, mid, gridBottom, 1, theme.Accent2)
		if gpTiny != 0 {
			old, _, _ := selectObject.Call(hdc, gpTiny)
			setTextColor.Call(hdc, muted)
			labels := []string{"SCORE", "STREAK", "ACCURACY"}
			values := []string{"—", "—", "—"}
			if has {
				if d == "ENDURANCE" {
					dist := pe.Distance
					if dist <= 0 {
						dist = float64(pe.Score) / 10
					}
					labels = []string{"DISTANCE", "TARGETS", "ACCURACY"}
					values = []string{fmt.Sprintf("%.0fm", dist), fmt.Sprintf("%d", func() int {
						if pe.TargetsHit > 0 {
							return pe.TargetsHit
						}
						return pe.Streak
					}()), fmt.Sprintf("%.1f%%", pe.Accuracy)}
				} else {
					values = []string{fmt.Sprintf("%d", pe.Score), fmt.Sprintf("%d", pe.Streak), fmt.Sprintf("%.1f%%", pe.Accuracy)}
				}
			}
			for row := int32(0); row < 3; row++ {
				y := gridTop + row*rowH + sy(8, hgt)
				setTextColor.Call(hdc, muted)
				textOut(hdc, cr.Left+sx(9, w), y, labels[row])
				setTextColor.Call(hdc, white)
				centeredTextOut(hdc, mid+sx(2, w), cr.Right-sx(5, w), y, values[row])
			}
			selectObject.Call(hdc, old)
		}
	}

	prog := RECT{r.Left + sx(28, w), r.Top + sy(530, hgt), r.Right - sx(28, w), r.Top + sy(602, hgt)}
	drawBevelPanel(hdc, prog, theme.Panel2, theme.Accent2, theme.Deep, 2)
	if gpTiny != 0 {
		old, _, _ := selectObject.Call(hdc, gpTiny)
		vals := []string{fmt.Sprintf("EASY %d", clears[0]), fmt.Sprintf("NORMAL %d", clears[1]), fmt.Sprintf("HARD %d", clears[2]), fmt.Sprintf("INSANE %d", clears[3]), fmt.Sprintf("TOTAL %d", clears[4])}
		seg := (prog.Right - prog.Left) / 5
		for i, v := range vals {
			setTextColor.Call(hdc, func() uintptr {
				if i == 4 {
					return gold
				}
				return white
			}())
			centeredTextOut(hdc, prog.Left+int32(i)*seg, prog.Left+int32(i+1)*seg, prog.Top+sy(25, hgt), v)
		}
		selectObject.Call(hdc, old)
	}

	// Public achievement showcase. This is deliberately compact: status identity
	// sits between progression and the ship collection without crowding gameplay PBs.
	showTop := r.Top + sy(614, hgt)
	showBottom := r.Top + sy(682, hgt)
	showGap := sx(10, w)
	showW := (right - left - showGap*2) / 3
	for i := 0; i < 3; i++ {
		x := left + int32(i)*(showW+showGap)
		cr := RECT{x, showTop, x + showW, showBottom}
		accent := theme.Accent
		if i < len(remoteShowcase) && strings.TrimSpace(remoteShowcase[i]) != "" {
			id := strings.TrimSpace(remoteShowcase[i])
			if a, ok := achievementByID(id); ok {
				switch achievementDifficultyLevel(id, a.Tier) {
				case 0:
					accent = rgb(59, 130, 246)
				case 1:
					accent = rgb(245, 158, 11)
				case 2:
					accent = rgb(239, 68, 68)
				case 3:
					accent = rgb(192, 38, 211)
				}
			}
		}
		showBG := blendColor(rgb(4, 16, 30), accent, 0.22)
		showDeep := blendColor(rgb(1, 8, 18), accent, 0.10)
		drawBevelPanel(hdc, cr, showBG, accent, showDeep, 2)
		if gpTiny != 0 {
			old, _, _ := selectObject.Call(hdc, gpTiny)
			if i < len(remoteShowcase) && strings.TrimSpace(remoteShowcase[i]) != "" {
				id := strings.TrimSpace(remoteShowcase[i])
				title := strings.ReplaceAll(id, "_", " ")
				desc := ""
				difficulty := ""
				if a, ok := achievementByID(id); ok {
					title = a.Title
					desc = a.Description
					difficulty = achievementDifficultyName(id, a.Tier)
				}
				setTextColor.Call(hdc, accent)
				title = fitTextEllipsis(hdc, gpTiny, title, cr.Right-cr.Left-sx(14, w))
				centeredTextOut(hdc, cr.Left+sx(7, w), cr.Right-sx(7, w), cr.Top+sy(7, hgt), title)
				if desc != "" {
					setTextColor.Call(hdc, muted)
					desc = fitTextEllipsis(hdc, gpTiny, desc, cr.Right-cr.Left-sx(14, w))
					centeredTextOut(hdc, cr.Left+sx(7, w), cr.Right-sx(7, w), cr.Top+sy(27, hgt), desc)
				}
				if difficulty != "" {
					setTextColor.Call(hdc, accent)
					centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(48, hgt), difficulty)
				}
			} else {
				setTextColor.Call(hdc, muted)
				centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(34, hgt), "NO SHOWCASE")
			}
			selectObject.Call(hdc, old)
		}
	}

	ships := RECT{r.Left + sx(28, w), r.Top + sy(692, hgt), r.Right - sx(28, w), r.Bottom - sy(42, hgt)}
	drawShipCollectionStripThemed(hdc, ships, unlocked, selected, w, hgt, theme)
	drawBackHint(hdc, r, w, hgt)
	if profileAnim > 0 {
		drawGlobalProfileAnimation(hdc, r, profileAnim, profilePrimary, profileSecondary)
	}

}
