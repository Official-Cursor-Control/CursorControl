//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unsafe"
)

func localLeaderboardPath() string {
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
	return filepath.Join(dir, "leaderboard.json")
}

func loadLeaderboard() {
	leaderboardFile = localLeaderboardPath()
	if err := readJSONWithRecovery(leaderboardFile, &leaderboard); err != nil {
		leaderboard = nil
		return
	}

	// Backward compatibility with older leaderboard files where Date stored
	// both values as "YYYY-MM-DD HH:MM".
	changed := false
	for i := range leaderboard {
		if leaderboard[i].Time == "" {
			parts := strings.Fields(leaderboard[i].Date)
			if len(parts) >= 2 {
				oldDate := parts[0]
				oldTime := parts[1]

				// Convert old ISO date into DD/MM/YYYY where possible.
				if parsed, err := time.Parse("2006-01-02", oldDate); err == nil {
					leaderboard[i].Date = parsed.Format("02/01/2006")
				} else {
					leaderboard[i].Date = oldDate
				}
				leaderboard[i].Time = oldTime
				changed = true
			}
		}
	}

	sortLeaderboard()

	if changed {
		if migrated, err := json.MarshalIndent(leaderboard, "", "  "); err == nil {
			_ = atomicWriteFile(leaderboardFile, migrated, 0644)
		}
	}
}

func sortLeaderboard() {
	groups := map[string][]LeaderboardEntry{}
	order := []string{"ENDURANCE", "INSANE", "HARD", "NORMAL", "EASY"}
	for _, e := range leaderboard {
		key := strings.ToUpper(strings.TrimSpace(e.Difficulty))
		groups[key] = append(groups[key], e)
	}
	out := make([]LeaderboardEntry, 0, len(leaderboard))
	seen := map[string]bool{}
	for _, key := range order {
		g := groups[key]
		if len(g) == 0 {
			continue
		}
		sort.SliceStable(g, func(i, j int) bool { return leaderboardEntryIsBetter(g[i], g[j]) })
		if len(g) > 100 {
			g = g[:100]
		}
		out = append(out, g...)
		seen[key] = true
	}
	// Preserve any legacy/unknown difficulty rows too.
	for key, g := range groups {
		if seen[key] {
			continue
		}
		sort.SliceStable(g, func(i, j int) bool { return leaderboardEntryIsBetter(g[i], g[j]) })
		if len(g) > 100 {
			g = g[:100]
		}
		out = append(out, g...)
	}
	leaderboard = out
}

const (
	supabaseProjectURL     = "https://fobeyfnqmbslywqapfkb.supabase.co"
	supabasePublishableKey = "sb_publishable_yOHUqChckXx_6_tbVjJm1A_uGFTX7kO"
	desktopAuthCallback    = "http://127.0.0.1:8765/auth/callback"
)

func authSessionPath() string {
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
	return filepath.Join(dir, "auth_session.json")
}

func saveAuthSession() {
	authMu.Lock()
	defer authMu.Unlock()
	if authSessionFile == "" {
		authSessionFile = authSessionPath()
	}
	data, err := json.MarshalIndent(authSession, "", "  ")
	if err == nil {
		_ = atomicWriteFile(authSessionFile, data, 0600)
	}
}

func clearAuthSession() {
	authMu.Lock()
	authSession = AuthSession{}
	discordConnected = false
	discordDisplayName = ""
	discordAuthStatus = ""
	discordUserID = ""
	discordAvatarURL = ""
	discordAvatarBGRA = nil
	discordAvatarAnim = AvatarAnimation{}
	discordAvatarW, discordAvatarH = 0, 0
	discordCreatedAt = ""
	path := authSessionFile
	if path == "" {
		path = authSessionPath()
	}
	authMu.Unlock()
	globalMu.Lock()
	globalMyOverall = GlobalLeaderboardEntry{}
	globalMyOverallValid = false
	globalMu.Unlock()
	_ = os.Remove(path)
	_ = os.Remove(path + ".bak")
	afkCloudResetLease()
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

var sharedAuthHTTPClient = &http.Client{Timeout: 12 * time.Second}
var sharedAdminHTTPClient = &http.Client{Timeout: 8 * time.Second}

func authHTTPClient() *http.Client {
	return sharedAuthHTTPClient
}

func getDiscordUser(accessToken string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, supabaseProjectURL+"/auth/v1/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := authHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("session validation failed (%d)", resp.StatusCode)
	}

	var user struct {
		ID           string         `json:"id"`
		Email        string         `json:"email"`
		CreatedAt    string         `json:"created_at"`
		UserMetadata map[string]any `json:"user_metadata"`
	}
	if err := json.Unmarshal(body, &user); err != nil {
		return "", err
	}
	authMu.Lock()
	discordUserID = strings.TrimSpace(user.ID)
	discordCreatedAt = strings.TrimSpace(user.CreatedAt)
	avatarURL := ""
	for _, k := range []string{"avatar_url", "picture", "avatar"} {
		if v, ok := user.UserMetadata[k]; ok {
			if a, ok := v.(string); ok && strings.TrimSpace(a) != "" {
				avatarURL = strings.TrimSpace(a)
				break
			}
		}
	}
	discordAvatarURL = avatarURL
	authMu.Unlock()
	if avatarURL != "" {
		go loadDiscordAvatar(avatarURL, false)
	}

	keys := []string{"user_name", "preferred_username", "name", "full_name", "username"}
	for _, k := range keys {
		if v, ok := user.UserMetadata[k]; ok {
			if name, ok := v.(string); ok && strings.TrimSpace(name) != "" {
				return strings.TrimSpace(name), nil
			}
		}
	}
	if user.Email != "" {
		if at := strings.IndexByte(user.Email, '@'); at > 0 {
			return user.Email[:at], nil
		}
		return user.Email, nil
	}
	return "DISCORD PLAYER", nil
}

func decodeImageToBGRA(data []byte) ([]byte, int32, int32, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || w > 2048 || h > 2048 {
		return nil, 0, 0, fmt.Errorf("invalid image size")
	}
	out := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bv, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := (y*w + x) * 4
			out[i+0] = byte(bv >> 8)
			out[i+1] = byte(g >> 8)
			out[i+2] = byte(r >> 8)
			out[i+3] = byte(a >> 8)
		}
	}
	return out, int32(w), int32(h), nil
}

func loadDiscordAvatar(rawURL string, remote bool) {
	if strings.TrimSpace(rawURL) == "" {
		return
	}
	requestURL := discordAnimatedAvatarURL(rawURL)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "image/gif,image/*;q=0.9,*/*;q=0.1")
	req.Header.Set("User-Agent", "CursorControl/"+clientBuildVersion)
	resp, err := authHTTPClient().Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
	pix, w, h, anim, err := decodeDiscordAvatar(body)
	if err != nil {
		return
	}
	// Discord animated avatar hashes begin with a_. If the CDN/proxy ever serves a
	// single-frame variant on the first request, retry the canonical GIF at a smaller
	// size. This keeps Nitro avatars animated instead of silently falling back to frame 1.
	if len(anim.Frames) < 2 && strings.Contains(strings.ToLower(requestURL), "/a_") {
		u2 := discordAnimatedAvatarURL(rawURL)
		if parsed, e := url.Parse(u2); e == nil {
			q := parsed.Query()
			q.Set("size", "128")
			parsed.RawQuery = q.Encode()
			if req2, e := http.NewRequest(http.MethodGet, parsed.String(), nil); e == nil {
				req2.Header.Set("Accept", "image/gif,image/*;q=0.9,*/*;q=0.1")
				req2.Header.Set("User-Agent", "CursorControl/"+clientBuildVersion)
				if resp2, e := authHTTPClient().Do(req2); e == nil {
					defer resp2.Body.Close()
					if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
						b2, _ := io.ReadAll(io.LimitReader(resp2.Body, 12<<20))
						if p2, w2, h2, a2, e2 := decodeDiscordAvatar(b2); e2 == nil && len(a2.Frames) > 1 {
							pix, w, h, anim = p2, w2, h2, a2
						}
					}
				}
			}
		}
	}
	if remote {
		remoteProfileMu.Lock()
		remoteAvatarBGRA, remoteAvatarW, remoteAvatarH = pix, w, h
		remoteAvatarAnim = anim
		remoteProfileMu.Unlock()
	} else {
		authMu.Lock()
		discordAvatarBGRA, discordAvatarW, discordAvatarH = pix, w, h
		discordAvatarAnim = anim
		authMu.Unlock()
	}
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

func syncPlayerProfile() {
	var colorDirtyAtStart, sizeDirtyAtStart, showcaseDirtyAtStart uint64
	var titleDirtyAtStart, nameColourDirtyAtStart, frameDirtyAtStart uint64
	var localSelectedFireColor, localSelectedFireSize int
	var localSelectedNameColour, localSelectedProfileFrame int
	var localAchievementShowcase []string
	var localSelectedTitle string
	var payload, legacyPayload []byte

	// Profile state belongs to the Win32/UI thread. Capture one coherent snapshot
	// there, then perform only HTTP work on this background sync worker.
	postMainThreadTaskAndWait(func() {
		colorDirtyAtStart = garageFireColorSelectionDirty.Load()
		sizeDirtyAtStart = garageFireSizeSelectionDirty.Load()
		showcaseDirtyAtStart = achievementShowcaseSelectionDirty.Load()
		titleDirtyAtStart = profileTitleSelectionDirty.Load()
		nameColourDirtyAtStart = profileNameColourSelectionDirty.Load()
		frameDirtyAtStart = profileFrameSelectionDirty.Load()
		localSelectedFireColor = gameMeta.SelectedFireColor
		localSelectedFireSize = gameMeta.SelectedFireSize
		localAchievementShowcase = append([]string(nil), normalizedAchievementShowcase()...)
		localSelectedTitle = selectedProfileTitle()
		localSelectedNameColour = gameMeta.SelectedNameColour
		localSelectedProfileFrame = gameMeta.SelectedProfileFrame

		// Materialise rank/achievement-derived permanent cosmetics before snapshotting
		// the payload so newly earned ownership transfers immediately.
		unlockProfileCosmeticsFromProgress()
		payload = playerProfileSyncPayload(true)
		legacyPayload = playerProfileSyncPayload(false)
	})

	token, err := validAuthAccessToken()
	if err != nil || strings.TrimSpace(token) == "" {
		return
	}
	body, ok := postPlayerProfileSync(token, payload)
	if !ok {
		// Backward-compatible rollout: until the v320 profile migration/Edge Function
		// is deployed, retry the exact legacy contract so existing cloud sync never
		// breaks merely because this newer client understands showcase statistics.
		body, ok = postPlayerProfileSync(token, legacyPayload)
		if !ok {
			return
		}
	}
	// v159: server returns the UNION of the local and cloud collections.
	// Apply that merged state locally so ships unlocked on another device appear immediately.
	var env struct {
		OK                      bool     `json:"ok"`
		UnlockedShips           []int    `json:"unlocked_ships"`
		SelectedShip            int      `json:"selected_ship"`
		UnlockedFireColors      []int    `json:"unlocked_fire_colors"`
		SelectedFireColor       *int     `json:"selected_fire_color"`
		UnlockedFireSizes       []int    `json:"unlocked_fire_sizes"`
		SelectedFireSize        *int     `json:"selected_fire_size"`
		UnlockedTitles          []string `json:"unlocked_titles"`
		SelectedTitle           *string  `json:"selected_title"`
		UnlockedNameColours     []int    `json:"unlocked_name_colours"`
		SelectedNameColour      *int     `json:"selected_name_colour"`
		UnlockedProfileFrames   []int    `json:"unlocked_profile_frames"`
		SelectedProfileFrame    *int     `json:"selected_profile_frame"`
		UnlockedProfileSkins    []int    `json:"unlocked_profile_skins"`
		SelectedProfileSkin     *int     `json:"selected_profile_skin"`
		SelectedProfileFont     *int     `json:"selected_profile_font"`
		SelectedProfileNameFont *int     `json:"selected_profile_name_font"`
		ProfilePrimaryColour    *int     `json:"selected_profile_primary_colour"`
		ProfileSecondaryColour  *int     `json:"selected_profile_secondary_colour"`
		ProfileNameShadow       *bool    `json:"profile_name_shadow"`
		ProfileShadowColour     *int     `json:"profile_shadow_colour"`
		ProfileGradientVertical *bool    `json:"profile_gradient_vertical"`
		ProfileAnimation        *int     `json:"selected_profile_animation"`
		SentinelDefeats         int      `json:"sentinel_defeats"`
		SerpentDefeats          int      `json:"serpent_defeats"`
		ArrayDefeats            int      `json:"array_defeats"`
		CompetitiveBadge        *string  `json:"competitive_badge"`
		SeasonBest              *string  `json:"season_best"`
		AchievementShowcase     []string `json:"achievement_showcase"`
		BestSurvivalWave        int      `json:"best_survival_wave"`
		BestSurvivalKills       int      `json:"best_survival_kills"`
		SurvivalCheckpoint      int      `json:"survival_checkpoint"`
	}
	if json.Unmarshal(body, &env) != nil || !env.OK {
		return
	}
	postMainThreadTask(func() {
		changed := false
		if env.UnlockedShips != nil {
			merged := mergeShipCollections(gameMeta.UnlockedShips, env.UnlockedShips)
			gameMeta.UnlockedShips = merged
			changed = true
		}
		if env.SelectedShip >= 0 {
			// Never let a stale cloud selection force a locally locked/invalid ship.
			if env.SelectedShip == 0 || shipUnlocked(env.SelectedShip) {
				gameMeta.SelectedShip = env.SelectedShip
				changed = true
			}
		}
		if env.UnlockedFireColors != nil {
			gameMeta.UnlockedFireColors = mergeCosmeticIDCollections(gameMeta.UnlockedFireColors, env.UnlockedFireColors, len(fireColorDefs)-1)
			changed = true
		}
		if colorDirtyAtStart > 0 {
			// This sync was caused by a local garage equip/purchase. Preserve exactly
			// what the player chose while still merging cloud ownership above.
			if localSelectedFireColor >= 0 && localSelectedFireColor < len(fireColorDefs) &&
				(localSelectedFireColor == 0 || fireColorUnlocked(localSelectedFireColor)) {
				gameMeta.SelectedFireColor = localSelectedFireColor
			}
		} else if env.SelectedFireColor != nil {
			remote := *env.SelectedFireColor
			if remote >= 0 && remote < len(fireColorDefs) && (remote == 0 || fireColorUnlocked(remote)) {
				gameMeta.SelectedFireColor = remote
				changed = true
			}
		}
		if env.UnlockedFireSizes != nil {
			gameMeta.UnlockedFireSizes = mergeCosmeticIDCollections(gameMeta.UnlockedFireSizes, env.UnlockedFireSizes, len(fireSizeDefs)-1)
			changed = true
		}
		if sizeDirtyAtStart > 0 {
			if localSelectedFireSize >= 0 && localSelectedFireSize < len(fireSizeDefs) &&
				(localSelectedFireSize == 0 || fireSizeUnlocked(localSelectedFireSize)) {
				gameMeta.SelectedFireSize = localSelectedFireSize
			}
		} else if env.SelectedFireSize != nil {
			remote := *env.SelectedFireSize
			if remote >= 0 && remote < len(fireSizeDefs) && (remote == 0 || fireSizeUnlocked(remote)) {
				gameMeta.SelectedFireSize = remote
				changed = true
			}
		}
		if env.UnlockedTitles != nil {
			for _, t := range env.UnlockedTitles {
				addUnlockedTitle(t)
			}
			changed = true
		}
		if env.UnlockedNameColours != nil {
			for _, id := range env.UnlockedNameColours {
				addUnlockedNameColour(id)
			}
			changed = true
		}
		if env.UnlockedProfileFrames != nil {
			for _, id := range env.UnlockedProfileFrames {
				addUnlockedProfileFrame(id)
			}
			changed = true
		}
		if env.UnlockedProfileSkins != nil {
			for _, id := range env.UnlockedProfileSkins {
				addUnlockedProfileFrame(id)
			}
			changed = true
		}
		if titleDirtyAtStart > 0 {
			gameMeta.SelectedTitle = localSelectedTitle
			changed = true
			if env.SelectedTitle != nil && strings.TrimSpace(*env.SelectedTitle) == localSelectedTitle {
				profileTitleSelectionDirty.CompareAndSwap(titleDirtyAtStart, 0)
			}
		} else if env.SelectedTitle != nil && strings.TrimSpace(*env.SelectedTitle) != "" {
			for _, t := range unlockedProfileTitles() {
				if t == strings.TrimSpace(*env.SelectedTitle) {
					gameMeta.SelectedTitle = t
					changed = true
					break
				}
			}
		}
		if nameColourDirtyAtStart > 0 {
			if nameColourUnlocked(localSelectedNameColour) {
				gameMeta.SelectedNameColour = localSelectedNameColour
				changed = true
			}
			if env.SelectedNameColour != nil && *env.SelectedNameColour == localSelectedNameColour {
				profileNameColourSelectionDirty.CompareAndSwap(nameColourDirtyAtStart, 0)
			}
		} else if env.SelectedNameColour != nil && nameColourUnlocked(*env.SelectedNameColour) {
			gameMeta.SelectedNameColour = *env.SelectedNameColour
			changed = true
		}
		if frameDirtyAtStart > 0 {
			if profileFrameUnlocked(localSelectedProfileFrame) {
				gameMeta.SelectedProfileFrame = localSelectedProfileFrame
				changed = true
			}
			confirmed := (env.SelectedProfileSkin != nil && *env.SelectedProfileSkin == localSelectedProfileFrame) ||
				(env.SelectedProfileFrame != nil && *env.SelectedProfileFrame == localSelectedProfileFrame)
			if confirmed {
				profileFrameSelectionDirty.CompareAndSwap(frameDirtyAtStart, 0)
			}
		} else {
			remote := -1
			if env.SelectedProfileSkin != nil {
				remote = *env.SelectedProfileSkin
			} else if env.SelectedProfileFrame != nil {
				remote = *env.SelectedProfileFrame
			}
			if remote == 0 || profileFrameUnlocked(remote) {
				gameMeta.SelectedProfileFrame = remote
				changed = true
			}
		}
		if env.SelectedProfileFont != nil && *env.SelectedProfileFont >= 0 && *env.SelectedProfileFont < len(uiFontFaces) {
			gameMeta.SelectedProfileFont = *env.SelectedProfileFont
			changed = true
		}
		if env.SelectedProfileNameFont != nil && *env.SelectedProfileNameFont >= 0 && *env.SelectedProfileNameFont <= 12 {
			gameMeta.SelectedProfileNameFont = *env.SelectedProfileNameFont
			changed = true
		}
		if env.ProfilePrimaryColour != nil && *env.ProfilePrimaryColour >= 0 && *env.ProfilePrimaryColour < len(profileStyleColours) {
			gameMeta.ProfilePrimaryColour = *env.ProfilePrimaryColour
			changed = true
		}
		if env.ProfileSecondaryColour != nil && *env.ProfileSecondaryColour >= 0 && *env.ProfileSecondaryColour < len(profileStyleColours) {
			gameMeta.ProfileSecondaryColour = *env.ProfileSecondaryColour
			changed = true
		}
		if env.ProfileNameShadow != nil {
			gameMeta.ProfileNameShadow = *env.ProfileNameShadow
			changed = true
		}
		if env.ProfileShadowColour != nil && *env.ProfileShadowColour >= 0 && *env.ProfileShadowColour < len(profileStyleColours) {
			gameMeta.ProfileShadowColour = *env.ProfileShadowColour
			changed = true
		}
		if env.ProfileGradientVertical != nil {
			gameMeta.ProfileGradientVertical = *env.ProfileGradientVertical
			changed = true
		}
		if env.ProfileAnimation != nil && *env.ProfileAnimation >= 0 && *env.ProfileAnimation <= 3 {
			gameMeta.ProfileAnimation = *env.ProfileAnimation
			changed = true
		}
		if env.SentinelDefeats > gameMeta.SentinelDefeats {
			gameMeta.SentinelDefeats = env.SentinelDefeats
			changed = true
		}
		if env.SerpentDefeats > gameMeta.SerpentDefeats {
			gameMeta.SerpentDefeats = env.SerpentDefeats
			changed = true
		}
		if env.ArrayDefeats > gameMeta.ArrayDefeats {
			gameMeta.ArrayDefeats = env.ArrayDefeats
			changed = true
		}
		// Pointer fields distinguish "old backend omitted this field" from "v320+
		// backend explicitly cleared an expired temporary prestige reward".
		if env.CompetitiveBadge != nil {
			badge := strings.TrimSpace(*env.CompetitiveBadge)
			if badge != gameMeta.CompetitiveBadge {
				gameMeta.CompetitiveBadge = badge
				changed = true
			}
		}
		if env.SeasonBest != nil {
			season := strings.TrimSpace(*env.SeasonBest)
			if season != gameMeta.SeasonBest {
				gameMeta.SeasonBest = season
				changed = true
			}
		}
		if showcaseDirtyAtStart > 0 {
			// A showcase click is locally authoritative until the server echoes the same
			// slots back. This prevents an older/stale profile response from immediately
			// erasing the achievement the player just selected.
			gameMeta.AchievementShowcase = append([]string(nil), localAchievementShowcase...)
			changed = true
			remoteNormalized := normalizeAchievementShowcaseValues(env.AchievementShowcase)
			if achievementShowcasesEqual(remoteNormalized, localAchievementShowcase) {
				achievementShowcaseSelectionDirty.CompareAndSwap(showcaseDirtyAtStart, 0)
			}
		} else if env.AchievementShowcase != nil {
			// Only accept achievements this device has actually unlocked. The cloud
			// selection cannot manufacture local achievement ownership.
			gameMeta.AchievementShowcase = append([]string(nil), env.AchievementShowcase...)
			gameMeta.AchievementShowcase = normalizedAchievementShowcase()
			changed = true
		}
		if env.BestSurvivalWave > gameMeta.BestSurvivalWave {
			gameMeta.BestSurvivalWave = env.BestSurvivalWave
			changed = true
		}
		if env.BestSurvivalKills > gameMeta.BestSurvivalKills {
			gameMeta.BestSurvivalKills = env.BestSurvivalKills
			changed = true
		}
		if env.SurvivalCheckpoint > gameMeta.SurvivalCheckpoint {
			gameMeta.SurvivalCheckpoint = env.SurvivalCheckpoint
			survivalCheckpoint = env.SurvivalCheckpoint
			changed = true
		}
		if colorDirtyAtStart > 0 {
			garageFireColorSelectionDirty.CompareAndSwap(colorDirtyAtStart, 0)
		}
		if sizeDirtyAtStart > 0 {
			garageFireSizeSelectionDirty.CompareAndSwap(sizeDirtyAtStart, 0)
		}
		if changed {
			normalizeGameMeta()
			saveGameMeta()
			if mainHwnd != 0 {
				invalidateRect.Call(mainHwnd, 0, 0)
			}
		}
	})
}

func prepareRemoteProfile() {
	e, ok := selectedGlobalEntry()
	if !ok {
		return
	}

	// Seed the panel instantly from information we already have. In particular,
	// when the player opens their own global profile, reuse the Discord avatar and
	// auth metadata already cached by the client instead of waiting for another HTTP round-trip.
	seed := RemoteProfileData{
		UserID: e.UserID, DisplayName: e.Name, EXPRank: e.Rank,
		EasyClears: e.EasyClears, NormalClears: e.NormalClears,
		HardClears: e.HardClears, InsaneClears: e.InsaneClears, TotalClears: e.TotalClears,
	}
	var seedAvatar []byte
	var seedAvatarAnim AvatarAnimation
	var seedAvatarW, seedAvatarH int32
	authMu.Lock()
	isSelf := (strings.TrimSpace(e.UserID) != "" && strings.EqualFold(strings.TrimSpace(e.UserID), strings.TrimSpace(discordUserID))) ||
		(strings.TrimSpace(discordDisplayName) != "" && strings.EqualFold(strings.TrimSpace(e.Name), strings.TrimSpace(discordDisplayName)))
	if isSelf {
		seed.AvatarURL = discordAvatarURL
		seed.CreatedAt = discordCreatedAt
		seedAvatar = append([]byte(nil), discordAvatarBGRA...)
		seedAvatarAnim = discordAvatarAnim
		seedAvatarW, seedAvatarH = discordAvatarW, discordAvatarH
	}
	authMu.Unlock()

	remoteProfileMu.Lock()
	remoteProfile = seed
	remoteProfileLoaded = false
	remoteProfileLoading = true
	remoteProfileStatus = ""
	remoteAvatarBGRA = seedAvatar
	remoteAvatarAnim = seedAvatarAnim
	remoteAvatarW, remoteAvatarH = seedAvatarW, seedAvatarH
	remoteProfileMu.Unlock()
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
	go fetchRemoteProfile(e)
}

func fetchRemoteProfile(e GlobalLeaderboardEntry) {
	q := url.Values{}
	if strings.TrimSpace(e.UserID) != "" {
		q.Set("user_id", strings.TrimSpace(e.UserID))
	} else {
		q.Set("name", strings.TrimSpace(e.Name))
	}
	req, err := http.NewRequest(http.MethodGet, supabaseProjectURL+"/functions/v1/player-profile?"+q.Encode(), nil)
	if err != nil {
		return
	}
	req.Header.Set("apikey", supabasePublishableKey)
	if tok, tokErr := validAuthAccessToken(); tokErr == nil && strings.TrimSpace(tok) != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	} else {
		req.Header.Set("Authorization", "Bearer "+supabasePublishableKey)
	}
	resp, err := authHTTPClient().Do(req)
	if err != nil {
		remoteProfileMu.Lock()
		remoteProfileLoading = false
		remoteProfileStatus = "PROFILE SERVER UNAVAILABLE"
		remoteProfileMu.Unlock()
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		remoteProfileMu.Lock()
		remoteProfileLoading = false
		remoteProfileStatus = "PROFILE DATA NOT AVAILABLE"
		remoteProfileMu.Unlock()
		return
	}
	var env struct {
		OK      bool              `json:"ok"`
		Profile RemoteProfileData `json:"profile"`
	}
	if json.Unmarshal(body, &env) != nil || !env.OK {
		remoteProfileMu.Lock()
		remoteProfileLoading = false
		remoteProfileStatus = "PROFILE DATA FORMAT ERROR"
		remoteProfileMu.Unlock()
		return
	}
	remoteProfileMu.Lock()
	remoteProfile = env.Profile
	remoteProfileLoaded = true
	remoteProfileLoading = false
	remoteProfileStatus = ""
	remoteProfileMu.Unlock()
	if strings.TrimSpace(env.Profile.AvatarURL) != "" {
		go loadDiscordAvatar(env.Profile.AvatarURL, true)
	}
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}
}

func refreshSupabaseSession(refreshToken string) (AuthSession, error) {
	payload, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/auth/v1/token?grant_type=refresh_token", bytes.NewReader(payload))
	if err != nil {
		return AuthSession{}, err
	}
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := authHTTPClient().Do(req)
	if err != nil {
		return AuthSession{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AuthSession{}, fmt.Errorf("refresh failed (%d)", resp.StatusCode)
	}
	var v struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return AuthSession{}, err
	}
	if v.AccessToken == "" {
		return AuthSession{}, fmt.Errorf("missing access token")
	}
	if v.RefreshToken == "" {
		v.RefreshToken = refreshToken
	}
	return AuthSession{AccessToken: v.AccessToken, RefreshToken: v.RefreshToken, ExpiresAt: time.Now().Unix() + v.ExpiresIn}, nil
}

func loadAuthSession() {
	authSessionFile = authSessionPath()
	var sess AuthSession
	if err := readJSONWithRecovery(authSessionFile, &sess); err != nil || sess.AccessToken == "" {
		return
	}

	// Refresh proactively when the stored access token is close to expiry.
	if sess.ExpiresAt > 0 && time.Now().Unix()+90 >= sess.ExpiresAt && sess.RefreshToken != "" {
		if refreshed, err := refreshSupabaseSession(sess.RefreshToken); err == nil {
			sess = refreshed
		}
	}

	name, err := getDiscordUser(sess.AccessToken)
	if err != nil && sess.RefreshToken != "" {
		if refreshed, rerr := refreshSupabaseSession(sess.RefreshToken); rerr == nil {
			sess = refreshed
			name, err = getDiscordUser(sess.AccessToken)
		}
	}
	if err != nil {
		_ = os.Remove(authSessionFile)
		_ = os.Remove(authSessionFile + ".bak")
		return
	}

	authMu.Lock()
	authSession = sess
	discordConnected = true
	discordDisplayName = cleanDiscordDisplayName(name)
	discordAuthStatus = ""
	authMu.Unlock()
	saveAuthSession()
	afkCloudClaimDeviceAsync()
	go autoSyncPrecisionCompetitionRewards()
}

func validAuthAccessToken() (string, error) {
	authMu.Lock()
	sess := authSession
	connected := discordConnected
	authMu.Unlock()
	if !connected || sess.AccessToken == "" {
		return "", fmt.Errorf("not logged in")
	}

	if sess.ExpiresAt > 0 && time.Now().Unix()+90 >= sess.ExpiresAt && sess.RefreshToken != "" {
		refreshed, err := refreshSupabaseSession(sess.RefreshToken)
		if err != nil {
			return "", err
		}
		authMu.Lock()
		authSession = refreshed
		authMu.Unlock()
		saveAuthSession()
		sess = refreshed
	}
	return sess.AccessToken, nil
}

func canonicalOnlineDifficulty(diffName string) string {
	u := strings.ToUpper(strings.TrimSpace(diffName))
	switch u {
	case "EASY", "NORMAL", "HARD", "INSANE":
		return u
	default:
		i := activeDifficultyIndex()
		if i < 0 {
			i = 0
		}
		if i >= len(diffs) {
			i = len(diffs) - 1
		}
		return strings.ToUpper(diffs[i].name)
	}
}

func loadSyncedEndurancePB() {
	if syncedEndurancePBFile == "" {
		return
	}
	data, err := os.ReadFile(syncedEndurancePBFile)
	if err != nil {
		return
	}
	var e GlobalLeaderboardEntry
	if json.Unmarshal(data, &e) != nil || e.Distance <= 0 {
		return
	}
	globalMu.Lock()
	globalEnduranceSyncedPB = e
	globalEnduranceSyncedValid = true
	globalMu.Unlock()
}

func rememberSyncedEndurancePB(result ResultData) {
	e := GlobalLeaderboardEntry{
		Position:   0,
		Name:       localPlayerName(),
		Score:      int(math.Round(result.Distance * 10)),
		Streak:     result.TargetsHit,
		Accuracy:   result.CombinedAcc,
		Difficulty: "ENDURANCE",
		Rank:       result.Rank,
		Distance:   result.Distance,
		TargetsHit: result.TargetsHit,
	}
	globalMu.Lock()
	shouldSave := !globalEnduranceSyncedValid ||
		e.Distance > globalEnduranceSyncedPB.Distance+0.001 ||
		(math.Abs(e.Distance-globalEnduranceSyncedPB.Distance) <= 0.001 && e.TargetsHit > globalEnduranceSyncedPB.TargetsHit)
	if shouldSave {
		globalEnduranceSyncedPB = e
		globalEnduranceSyncedValid = true
	}
	globalMu.Unlock()
	if shouldSave && syncedEndurancePBFile != "" {
		if data, err := json.MarshalIndent(e, "", "  "); err == nil {
			_ = atomicWriteFile(syncedEndurancePBFile, data, 0644)
		}
	}
}

type PendingEnduranceSubmission struct {
	Result   ResultData `json:"result"`
	QueuedAt string     `json:"queued_at"`
}

func enduranceResultIsBetter(a, b ResultData) bool {
	if math.Abs(a.Distance-b.Distance) > 0.001 {
		return a.Distance > b.Distance
	}
	if a.TargetsHit != b.TargetsHit {
		return a.TargetsHit > b.TargetsHit
	}
	if math.Abs(a.CombinedAcc-b.CombinedAcc) > 0.001 {
		return a.CombinedAcc > b.CombinedAcc
	}
	return a.TotalScore > b.TotalScore
}

func queuePendingEnduranceSubmission(result ResultData) {
	if pendingEnduranceSubmitFile == "" || result.Distance <= 0 {
		return
	}
	pending := PendingEnduranceSubmission{
		Result:   result,
		QueuedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if data, err := os.ReadFile(pendingEnduranceSubmitFile); err == nil {
		var existing PendingEnduranceSubmission
		if json.Unmarshal(data, &existing) == nil &&
			existing.Result.Distance > 0 &&
			!enduranceResultIsBetter(result, existing.Result) {
			return
		}
	}
	if data, err := json.MarshalIndent(pending, "", "  "); err == nil {
		_ = atomicWriteFile(pendingEnduranceSubmitFile, data, 0644)
	}
}

func clearPendingEnduranceSubmissionIfCovered(result ResultData) {
	if pendingEnduranceSubmitFile == "" {
		return
	}
	data, err := os.ReadFile(pendingEnduranceSubmitFile)
	if err != nil {
		return
	}
	var pending PendingEnduranceSubmission
	if json.Unmarshal(data, &pending) != nil {
		_ = os.Remove(pendingEnduranceSubmitFile)
		return
	}
	// Remove only if the successfully submitted run is at least as good as
	// the currently queued one. A newer/better run must remain queued.
	if !enduranceResultIsBetter(pending.Result, result) {
		_ = os.Remove(pendingEnduranceSubmitFile)
	}
}

func retryPendingEnduranceSubmission() {
	if pendingEnduranceSubmitFile == "" {
		return
	}
	data, err := os.ReadFile(pendingEnduranceSubmitFile)
	if err != nil {
		return
	}
	var pending PendingEnduranceSubmission
	if json.Unmarshal(data, &pending) != nil || pending.Result.Distance <= 0 {
		_ = os.Remove(pendingEnduranceSubmitFile)
		return
	}
	submitGlobalClear(pending.Result, "ENDURANCE")
}

func logEnduranceSyncError(status int, body []byte) {
	if logRoot == "" {
		return
	}
	msg := fmt.Sprintf(
		"%s status=%d body=%s\n",
		time.Now().Format(time.RFC3339),
		status,
		strings.TrimSpace(string(body)),
	)
	f, err := os.OpenFile(filepath.Join(logRoot, "endurance_sync.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(msg)
}

func submitGlobalClear(result ResultData, onlineDifficulty string) {
	isEndurance := strings.EqualFold(onlineDifficulty, "ENDURANCE")
	if isEndurance {
		queuePendingEnduranceSubmission(result)
	}

	// Serialize submissions so two Endurance failures cannot race the pending PB file.
	globalSubmitMu.Lock()
	defer globalSubmitMu.Unlock()

	token, err := validAuthAccessToken()
	if err != nil {
		if isEndurance {
			globalMu.Lock()
			globalLeaderboardStatus = "ENDURANCE PB QUEUED — LOGIN TO SYNC"
			globalMu.Unlock()
			if mainHwnd != 0 {
				invalidateRect.Call(mainHwnd, 0, 0)
			}
		}
		return
	}

	submitScore := result.TotalScore
	submitStreak := result.Streak
	if isEndurance {
		submitScore = int(math.Round(result.Distance * 10))
		submitStreak = result.TargetsHit
	}
	payload := map[string]any{
		"difficulty":     onlineDifficulty,
		"score":          submitScore,
		"streak":         submitStreak,
		"accuracy":       math.Round(result.CombinedAcc*100) / 100,
		"exp":            result.TotalEXP,
		"exp_rank":       result.Rank,
		"client_version": clientBuildVersion,
		"run_time_ms":    int(math.Round(result.Time * 1000)),
		"target_count": func() int {
			if isEndurance {
				return result.TargetsHit
			}
			return result.TargetCount
		}(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, supabaseProjectURL+"/functions/v1/submit-score", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", supabasePublishableKey)
	req.Header.Set("Authorization", "Bearer "+token)

	if isEndurance {
		globalMu.Lock()
		globalLeaderboardStatus = "SYNCING ENDURANCE PB..."
		globalMu.Unlock()
	}

	resp, err := authHTTPClient().Do(req)
	if err != nil {
		if isEndurance {
			globalMu.Lock()
			globalLeaderboardStatus = "ENDURANCE PB QUEUED — RETRY ON NEXT RUN/LEADERBOARD"
			globalMu.Unlock()
			if mainHwnd != 0 {
				invalidateRect.Call(mainHwnd, 0, 0)
			}
		}
		return
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if isEndurance {
			clearPendingEnduranceSubmissionIfCovered(result)
			rememberSyncedEndurancePB(result)
			globalMu.Lock()
			globalLeaderboardStatus = "ENDURANCE PB SYNCED"
			globalMu.Unlock()

			// If a better PB was queued while this request was in flight, send it next.
			if pendingEnduranceSubmitFile != "" {
				if _, err := os.Stat(pendingEnduranceSubmitFile); err == nil {
					go retryPendingEnduranceSubmission()
				}
			}
		}
		go fetchGlobalLeaderboard()
		return
	}

	if isEndurance {
		logEnduranceSyncError(resp.StatusCode, responseBody)
		globalMu.Lock()
		if resp.StatusCode == 500 {
			globalLeaderboardStatus = "ENDURANCE SERVER 500 — BACKEND PATCH REQUIRED • PB QUEUED"
		} else {
			globalLeaderboardStatus = fmt.Sprintf("ENDURANCE SYNC ERROR %d — PB QUEUED", resp.StatusCode)
		}
		globalMu.Unlock()
		if mainHwnd != 0 {
			invalidateRect.Call(mainHwnd, 0, 0)
		}
	}
	_ = responseBody
}

func discordCallbackHTML() string {
	return `<!doctype html>
<html>
<head><meta charset="utf-8"><title>Cursor Control Trainer</title></head>
<body style="margin:0;background:#061323;color:#eaf7ff;font-family:"Lucida Console",Consolas,monospace;display:grid;place-items:center;height:100vh">
<div style="max-width:520px;text-align:center;padding:36px;border:1px solid #1acdef;background:#071d31">
<h2 style="color:#1acdef">CURSOR CONTROL TRAINER</h2>
<p id="status">Completing Discord login…</p>
</div>
<script>
(async()=>{
  const h=new URLSearchParams(location.hash.slice(1));
  const q=new URLSearchParams(location.search);
  const error=h.get('error_description')||q.get('error_description')||h.get('error')||q.get('error');
  if(error){document.getElementById('status').textContent='Discord login failed: '+error;return;}
  const access_token=h.get('access_token');
  const refresh_token=h.get('refresh_token')||'';
  const expires_in=Number(h.get('expires_in')||3600);
  if(!access_token){document.getElementById('status').textContent='No login token was returned. Check the Supabase redirect URL configuration.';return;}
  try{
    const r=await fetch('/auth/session',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({access_token,refresh_token,expires_in})});
    if(!r.ok) throw new Error(await r.text());
    document.getElementById('status').innerHTML='<b style="color:#52dc8b">Login successful.</b><br><br>You can close this browser tab and return to the game.';
    history.replaceState(null,'',location.pathname);
  }catch(e){
    document.getElementById('status').textContent='Could not return the session to the game: '+e.message;
  }
})();
</script></body></html>`
}

func startDiscordCallbackServer() error {
	authMu.Lock()
	if authServer != nil {
		authMu.Unlock()
		return nil
	}
	authMu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:8765")
	if err != nil {
		return fmt.Errorf("callback port 8765 is unavailable: %w", err)
	}

	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}
	authMu.Lock()
	authServer = server
	authMu.Unlock()

	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, discordCallbackHTML())
	})
	mux.HandleFunc("/auth/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var v struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&v); err != nil || v.AccessToken == "" {
			http.Error(w, "invalid session payload", http.StatusBadRequest)
			return
		}
		name, err := getDiscordUser(v.AccessToken)
		if err != nil {
			http.Error(w, "Supabase session validation failed", http.StatusUnauthorized)
			return
		}
		if v.ExpiresIn <= 0 {
			v.ExpiresIn = 3600
		}

		authMu.Lock()
		authSession = AuthSession{
			AccessToken:  v.AccessToken,
			RefreshToken: v.RefreshToken,
			ExpiresAt:    time.Now().Unix() + v.ExpiresIn,
		}
		discordConnected = true
		discordDisplayName = cleanDiscordDisplayName(name)
		discordAuthStatus = ""
		authMu.Unlock()
		saveAuthSession()
		afkCloudClaimDeviceAsync()
		requestPlayerProfileSync()
		go retryPendingEnduranceSubmission()
		go autoSyncPrecisionCompetitionRewards()
		if mainHwnd != 0 {
			invalidateRect.Call(mainHwnd, 0, 0)
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok")
		go func() {
			time.Sleep(400 * time.Millisecond)
			_ = server.Close()
			authMu.Lock()
			if authServer == server {
				authServer = nil
			}
			authMu.Unlock()
		}()
	})

	go func() {
		_ = server.Serve(ln)
		authMu.Lock()
		if authServer == server {
			authServer = nil
		}
		authMu.Unlock()
	}()
	return nil
}

func openExternalURL(raw string) error {
	operation := utf16ptr("open")
	target := utf16ptr(raw)
	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		SW_SHOW,
	)
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW failed with code %d", ret)
	}
	return nil
}

func beginDiscordLogin() {
	authMu.Lock()
	if discordConnected {
		authMu.Unlock()
		clearAuthSession()
		return
	}
	discordAuthStatus = "WAITING..."
	authMu.Unlock()
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
	}

	if err := startDiscordCallbackServer(); err != nil {
		authMu.Lock()
		discordAuthStatus = "ERROR"
		authMu.Unlock()
		msg := utf16ptr("Discord login could not start because the local callback port 8765 is unavailable. Close any other copy of the game and try again.")
		title := utf16ptr("Discord Login")
		messageBoxW.Call(mainHwnd, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), MB_ICONWARNING)
		return
	}

	authURL := supabaseProjectURL + "/auth/v1/authorize?provider=discord&redirect_to=" + url.QueryEscape(desktopAuthCallback)
	if err := openExternalURL(authURL); err != nil {
		authMu.Lock()
		discordAuthStatus = "ERROR"
		authMu.Unlock()
		msg := utf16ptr("Windows could not open your default browser for Discord login.")
		title := utf16ptr("Discord Login")
		messageBoxW.Call(mainHwnd, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), MB_ICONWARNING)
	}
}

func localPlayerName() string {
	if discordConnected && strings.TrimSpace(discordDisplayName) != "" {
		return strings.ToUpper(strings.TrimSpace(discordDisplayName))
	}

	name := strings.TrimSpace(os.Getenv("USERNAME"))
	if name == "" {
		name = strings.TrimSpace(os.Getenv("USER"))
	}
	if name == "" {
		name = "LOCAL PLAYER"
	}
	if len([]rune(name)) > 16 {
		name = string([]rune(name)[:16])
	}
	return strings.ToUpper(name)
}

func leaderboardEntryIsBetter(a, b LeaderboardEntry) bool {
	if strings.EqualFold(a.Difficulty, "ENDURANCE") && strings.EqualFold(b.Difficulty, "ENDURANCE") {
		if math.Abs(a.Distance-b.Distance) > 0.001 {
			return a.Distance > b.Distance
		}
		if a.TargetsHit != b.TargetsHit {
			return a.TargetsHit > b.TargetsHit
		}
		if math.Abs(a.Accuracy-b.Accuracy) > 0.001 {
			return a.Accuracy > b.Accuracy
		}
		return a.Score > b.Score
	}
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if math.Abs(a.Accuracy-b.Accuracy) > 0.001 {
		return a.Accuracy > b.Accuracy
	}
	return a.Streak > b.Streak
}

func saveLocalBestAutomatically() bool {
	if leaderboardFile == "" {
		leaderboardFile = localLeaderboardPath()
	}

	now := time.Now().Local()
	entry := LeaderboardEntry{
		Name:       localPlayerName(),
		Score:      lastResult.TotalScore,
		Streak:     lastResult.Streak,
		Accuracy:   lastResult.CombinedAcc,
		Difficulty: lastResult.Difficulty,
		Rank:       lastResult.Rank,
		Date:       now.Format("02/01/2006"),
		Time:       now.Format("15:04:05"),
		Distance:   lastResult.Distance,
		TargetsHit: lastResult.TargetsHit,
	}

	// Endurance is a run-history leaderboard locally: every completed run is
	// recorded, then the Top 10 view ranks those runs by distance.
	if strings.EqualFold(entry.Difficulty, "ENDURANCE") {
		leaderboard = append(leaderboard, entry)
		sortLeaderboard()
		if data, err := json.MarshalIndent(leaderboard, "", "  "); err == nil {
			_ = atomicWriteFile(leaderboardFile, data, 0644)
		}
		return true
	}

	// Standard difficulties retain one PB per local player/difficulty.
	updated := false
	found := -1
	for i, e := range leaderboard {
		if strings.EqualFold(e.Name, entry.Name) && strings.EqualFold(e.Difficulty, entry.Difficulty) {
			found = i
			break
		}
	}
	if found >= 0 {
		if leaderboardEntryIsBetter(entry, leaderboard[found]) {
			leaderboard[found] = entry
			updated = true
		}
	} else {
		leaderboard = append(leaderboard, entry)
		updated = true
	}
	if !updated {
		return false
	}

	sortLeaderboard()
	if data, err := json.MarshalIndent(leaderboard, "", "  "); err == nil {
		_ = atomicWriteFile(leaderboardFile, data, 0644)
	}
	return true
}

func leaderboardFilterName(filter int) string {
	switch filter {
	case 1:
		return "EASY"
	case 2:
		return "NORMAL"
	case 3:
		return "HARD"
	case 4:
		return "INSANE"
	case 5:
		return "ENDURANCE"
	default:
		return "OVERALL"
	}
}

func filteredLocalLeaderboard() []LeaderboardEntry {
	filterName := leaderboardFilterName(localLeaderboardFilter)
	out := make([]LeaderboardEntry, 0, len(leaderboard))
	for _, e := range leaderboard {
		end := strings.EqualFold(strings.TrimSpace(e.Difficulty), "ENDURANCE")
		if filterName == "OVERALL" {
			if end {
				continue
			}
			out = append(out, e)
		} else if strings.EqualFold(e.Difficulty, filterName) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return leaderboardEntryIsBetter(out[i], out[j]) })
	return out
}

func saveLeaderboardEntry(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Player"
	}
	if len([]rune(name)) > 16 {
		name = string([]rune(name)[:16])
	}

	// time.Now().Local() uses the local clock/timezone configured on this PC.
	now := time.Now().Local()

	entry := LeaderboardEntry{
		Name:       name,
		Score:      lastResult.TotalScore,
		Streak:     lastResult.Streak,
		Accuracy:   lastResult.CombinedAcc,
		Difficulty: lastResult.Difficulty,
		Rank:       lastResult.Rank,
		Date:       now.Format("02/01/2006"),
		Time:       now.Format("15:04:05"),
		Distance:   lastResult.Distance,
		TargetsHit: lastResult.TargetsHit,
	}

	leaderboard = append(leaderboard, entry)
	sortLeaderboard()

	data, err := json.MarshalIndent(leaderboard, "", "  ")
	if err == nil {
		_ = atomicWriteFile(leaderboardFile, data, 0644)
	}
}

// requestGlobalAccountEXP refreshes the authenticated player's server-side EXP for
// the compact HUD LOCAL/GLOBAL rank toggle. It deliberately reads player-profile
// rather than inferring EXP from leaderboard position.
func requestGlobalAccountEXP() {
	if !discordConnected || globalAccountEXPLoading {
		return
	}
	authMu.Lock()
	uid := strings.TrimSpace(discordUserID)
	name := strings.TrimSpace(discordDisplayName)
	authMu.Unlock()
	if uid == "" && name == "" {
		return
	}
	globalAccountEXPLoading = true
	go func() {
		loaded := false
		exp := 0
		defer func() {
			postMainThreadTask(func() {
				if loaded {
					globalAccountEXP = exp
					globalAccountEXPLoaded = true
				}
				globalAccountEXPLoading = false
				if mainHwnd != 0 {
					invalidateRect.Call(mainHwnd, 0, 0)
				}
			})
		}()
		q := url.Values{}
		if uid != "" {
			q.Set("user_id", uid)
		} else {
			q.Set("name", name)
		}
		req, err := http.NewRequest(http.MethodGet, supabaseProjectURL+"/functions/v1/player-profile?"+q.Encode(), nil)
		if err != nil {
			return
		}
		req.Header.Set("apikey", supabasePublishableKey)
		if tok, tokErr := validAuthAccessToken(); tokErr == nil && strings.TrimSpace(tok) != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		} else {
			return
		}
		resp, err := authHTTPClient().Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return
		}
		var env struct {
			OK      bool              `json:"ok"`
			Profile RemoteProfileData `json:"profile"`
		}
		if json.Unmarshal(body, &env) != nil || !env.OK {
			return
		}
		exp = env.Profile.EXP
		loaded = true
	}()
}
