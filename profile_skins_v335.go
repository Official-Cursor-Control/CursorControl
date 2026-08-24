//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	profileSkinFirstID = 101
	profileSkinLastID  = 111
)

type profileSkinDef struct {
	ID          int
	Name        string
	Requirement string
}

var profileSkinDefs = []profileSkinDef{
	{101, "WATER", "Complete 100 Precision courses."},
	{102, "THUNDER", "Complete 50 Normal courses."},
	{103, "FIRE", "Complete 30 Insane courses."},
	{104, "CRYSTAL", "Unlock 20 achievements."},
	{105, "EARTH", "Complete 200 Precision courses."},
	{106, "SKY", "Reach 1,200m in Endurance."},
	{107, "SPACE", "Complete 3 Endurance warps."},
	{108, "OCEAN", "Reach 2,500m in Endurance."},
	{109, "NODE", "Defeat The Terminus 10 times."},
	{110, "SENTINEL", "Defeat The Sentinel 15 times."},
	{111, "SERPENT", "Defeat The Void Serpent 10 times."},
}
var profileSkinScroll int

func profileSkinDefByID(id int) (profileSkinDef, bool) {
	for _, d := range profileSkinDefs {
		if d.ID == id {
			return d, true
		}
	}
	return profileSkinDef{}, false
}

func countUnlockedAchievements() int {
	seen := map[string]bool{}
	n := 0
	for _, id := range gameMeta.Achievements {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			n++
		}
	}
	return n
}

func profileSkinRequirementMet(id int) bool {
	switch id {
	case 101:
		return playerProgress.EasyCompleted+playerProgress.NormalCompleted+playerProgress.HardCompleted+playerProgress.InsaneCompleted >= 100
	case 102:
		return playerProgress.NormalCompleted >= 50
	case 103:
		return playerProgress.InsaneCompleted >= 30
	case 104:
		return countUnlockedAchievements() >= 20
	case 105:
		return playerProgress.EasyCompleted+playerProgress.NormalCompleted+playerProgress.HardCompleted+playerProgress.InsaneCompleted >= 200
	case 106:
		return gameMeta.BestEnduranceDistance >= 1200
	case 107:
		return gameMeta.EnduranceWarpsCompleted >= 3
	case 108:
		return gameMeta.BestEnduranceDistance >= 2500
	case 109:
		return gameMeta.ArrayDefeats >= 10
	case 110:
		return gameMeta.SentinelDefeats >= 15
	case 111:
		return gameMeta.SerpentDefeats >= 10
	}
	return false
}

func globalProfileEXPForUnlocks() int {
	// Rank-gated public-profile cosmetics are based on the authenticated GLOBAL
	// account only. Never fall back to a device-local EXP value or a fresh/high local
	// profile could temporarily unlock a server feature that the Discord account has
	// not earned (or hide one it has earned on another device).
	if remoteProfileIsSelf() {
		remoteProfileMu.Lock()
		exp := remoteProfile.EXP
		loaded := remoteProfileLoaded
		remoteProfileMu.Unlock()
		if loaded {
			return exp
		}
	}
	if globalAccountEXPLoaded {
		return globalAccountEXP
	}
	return 0
}

func profileFontFeatureUnlocked() bool        { return globalProfileEXPForUnlocks() >= 4000 }
func profileBannersFeatureUnlocked() bool     { return globalProfileEXPForUnlocks() >= 12000 }
func profileNameStylingFeatureUnlocked() bool { return globalProfileEXPForUnlocks() >= 30000 }
func profileAnimationFeatureUnlocked() bool   { return globalProfileEXPForUnlocks() >= 75000 }
func refreshProfileSkinUnlocks() bool {
	changed := false
	// v381: restore the proven v360 ownership/equip model. Banner ownership is
	// permanent once present in local/cloud state and is not invalidated while
	// global rank data is still loading. Platinum I gates the editor/equip action,
	// not persistence of an already-equipped banner.
	cleaned := make([]int, 0, len(gameMeta.UnlockedProfileFrames)+len(profileSkinDefs))
	seen := map[int]bool{}
	for _, id := range gameMeta.UnlockedProfileFrames {
		if id >= profileSkinFirstID && id <= profileSkinLastID && !seen[id] {
			seen[id] = true
			cleaned = append(cleaned, id)
		}
	}
	for _, d := range profileSkinDefs {
		if profileSkinRequirementMet(d.ID) && !seen[d.ID] {
			seen[d.ID] = true
			cleaned = append(cleaned, d.ID)
			changed = true
		}
	}
	if len(cleaned) != len(gameMeta.UnlockedProfileFrames) {
		changed = true
	}
	gameMeta.UnlockedProfileFrames = cleaned
	if gameMeta.SelectedProfileFrame != 0 && !containsInt(cleaned, gameMeta.SelectedProfileFrame) {
		gameMeta.SelectedProfileFrame = 0
		changed = true
	}
	return changed
}

func profileSkinUnlocked(id int) bool {
	if id == 0 {
		return true
	}
	refreshProfileSkinUnlocks()
	return id >= profileSkinFirstID && id <= profileSkinLastID && containsInt(gameMeta.UnlockedProfileFrames, id)
}

type profileTheme struct{ Outer, Accent, Accent2, Panel, Panel2, Deep, Muted, Gold uintptr }

func darkerProfileThemeColour(c uintptr) uintptr {
	// COLORREF is 0x00BBGGRR. Keep the hue, lower each channel to 42% so the
	// offset remains visibly related to the equipped banner without overpowering it.
	r := int(c & 0xff)
	g := int((c >> 8) & 0xff)
	b := int((c >> 16) & 0xff)
	return rgb(byte(r*42/100), byte(g*42/100), byte(b*42/100))
}

func profileThemeForSkin(id int) profileTheme {
	// Dark fills stay readable; accents follow the equipped banner palette.
	switch id {
	case 101, 108:
		return profileTheme{rgb(3, 45, 94), rgb(35, 185, 255), rgb(31, 102, 235), rgb(4, 52, 98), rgb(4, 35, 74), rgb(1, 18, 44), rgb(148, 205, 234), rgb(94, 232, 255)}
	case 102:
		return profileTheme{rgb(57, 39, 4), rgb(255, 190, 35), rgb(255, 225, 90), rgb(67, 47, 7), rgb(42, 31, 8), rgb(23, 17, 3), rgb(225, 198, 128), rgb(255, 220, 70)}
	case 103:
		return profileTheme{rgb(87, 19, 10), rgb(255, 71, 30), rgb(255, 174, 31), rgb(82, 24, 12), rgb(55, 18, 11), rgb(27, 8, 5), rgb(230, 165, 139), rgb(255, 201, 58)}
	case 104:
		return profileTheme{rgb(54, 17, 88), rgb(209, 67, 255), rgb(83, 211, 255), rgb(58, 19, 94), rgb(37, 13, 65), rgb(19, 6, 34), rgb(205, 164, 229), rgb(238, 119, 255)}
	case 105:
		return profileTheme{rgb(25, 59, 20), rgb(86, 224, 72), rgb(188, 153, 78), rgb(27, 65, 24), rgb(20, 46, 19), rgb(9, 24, 9), rgb(171, 205, 157), rgb(174, 225, 90)}
	case 106:
		return profileTheme{rgb(7, 60, 111), rgb(73, 197, 255), rgb(235, 248, 255), rgb(10, 69, 125), rgb(6, 48, 94), rgb(3, 24, 50), rgb(176, 216, 239), rgb(255, 220, 102)}
	case 107:
		return profileTheme{rgb(40, 18, 87), rgb(171, 64, 255), rgb(46, 220, 255), rgb(44, 22, 94), rgb(28, 16, 66), rgb(12, 7, 34), rgb(193, 170, 229), rgb(255, 101, 224)}
	case 109:
		return profileTheme{rgb(57, 14, 86), rgb(213, 55, 255), rgb(69, 219, 255), rgb(62, 18, 94), rgb(38, 13, 65), rgb(19, 7, 35), rgb(212, 165, 229), rgb(248, 111, 255)}
	case 110:
		return profileTheme{rgb(78, 21, 9), rgb(255, 83, 28), rgb(255, 184, 45), rgb(83, 24, 12), rgb(52, 17, 10), rgb(27, 8, 5), rgb(229, 161, 133), rgb(255, 201, 60)}
	case 111:
		return profileTheme{rgb(49, 15, 83), rgb(190, 61, 255), rgb(245, 78, 255), rgb(56, 18, 92), rgb(34, 13, 62), rgb(17, 6, 34), rgb(205, 166, 231), rgb(239, 112, 255)}
	}
	return profileTheme{rgb(4, 45, 105), rgb(47, 211, 255), rgb(80, 231, 255), rgb(5, 54, 125), rgb(8, 38, 78), rgb(2, 22, 57), rgb(157, 194, 221), rgb(255, 220, 70)}
}

var (
	profileSkinAssetMu      sync.Mutex
	profileSkinAssets       = map[int][]byte{}
	profileSkinAssetChecked = map[int]bool{}
)

func profileSkinAsset(id int) []byte {
	if id < profileSkinFirstID || id > profileSkinLastID || assetRoot == "" {
		return nil
	}
	profileSkinAssetMu.Lock()
	defer profileSkinAssetMu.Unlock()
	if !profileSkinAssetChecked[id] {
		profileSkinAssetChecked[id] = true
		p := filepath.Join(assetRoot, "ui", "profile_frames", fmt.Sprintf("profile_frame_%d.bgra", id))
		if b, err := os.ReadFile(p); err == nil && len(b) == int(globalProfileSkinWidth*globalProfileSkinHeight*4) {
			profileSkinAssets[id] = b
		}
	}
	return profileSkinAssets[id]
}

func drawProfileSkinPreview(hdc uintptr, r RECT, id int) {
	if b := profileSkinAsset(id); len(b) == int(globalProfileSkinWidth*globalProfileSkinHeight*4) {
		drawRawBGRAFit(hdc, b, globalProfileSkinWidth, globalProfileSkinHeight, r)
	}
}

func profileSkinEditButtonRect(w, h int32) RECT {
	r := centeredPanel(w, h, 1180, 840)
	identity := RECT{r.Left + sx(28, w), r.Top + sy(96, h), r.Right - sx(28, w), r.Top + sy(270, h)}
	bw, bh := sx(104, w), sy(28, h)
	return RECT{identity.Right - bw, identity.Top - sy(64, h), identity.Right, identity.Top - sy(64, h) + bh}
}

func remoteProfileIsSelf() bool {
	e, ok := selectedGlobalEntry()
	if !ok {
		return false
	}
	if strings.TrimSpace(discordUserID) != "" && strings.EqualFold(strings.TrimSpace(e.UserID), strings.TrimSpace(discordUserID)) {
		return true
	}
	return strings.EqualFold(cleanDiscordDisplayName(e.Name), cleanDiscordDisplayName(localPlayerName()))
}

func profileSkinPanelRect(w, h int32) RECT { return centeredPanel(w, h, 1180, 840) }
func profileSkinBackRect(w, h int32) RECT {
	r := profileSkinPanelRect(w, h)
	return RECT{r.Left + sx(24, w), r.Top + sy(20, h), r.Left + sx(128, w), r.Top + sy(52, h)}
}

func profileSkinCardRects(w, h int32) []RECT {
	r := profileSkinPanelRect(w, h)
	left := r.Left + sx(30, w)
	top := r.Top + sy(148, h)
	gapX := sx(20, w)
	gapY := sy(24, h)
	cw := (r.Right - r.Left - sx(80, w) - gapX) / 2
	ch := sy(134, h)
	out := make([]RECT, 0, len(profileSkinDefs))
	visibleTop := top - int32(profileSkinScroll)*sy(158, h)
	for i := range profileSkinDefs {
		col := int32(i % 2)
		row := int32(i / 2)
		x := left + col*(cw+gapX)
		y := visibleTop + row*(ch+gapY)
		out = append(out, RECT{x, y, x + cw, y + ch})
	}
	return out
}

func drawProfileSkinGallery(hdc uintptr, w, h int32) {
	drawGlobalProfileCustomizer(hdc, w, h)
}

func equipProfileSkin(id int) {
	if !profileBannersFeatureUnlocked() || !profileSkinUnlocked(id) {
		return
	}
	gameMeta.SelectedProfileFrame = id
	// Update the already-loaded self profile immediately. The server sync remains
	// authoritative for cross-device persistence, but the UI must not wait for it.
	if remoteProfileIsSelf() {
		remoteProfileMu.Lock()
		remoteProfile.SelectedProfileFrame = id
		remoteProfileMu.Unlock()
	}
	markProfileFrameSelectionDirty()
	saveGameMeta()
	requestPlayerProfileSync()
	analyticsEvent("profile_customized", map[string]any{"type": "profile_skin", "id": id})
	if mainHwnd != 0 {
		invalidateRect.Call(mainHwnd, 0, 0)
		updateWindow.Call(mainHwnd)
	}
}

func drawRankingCardsThemed(hdc uintptr, r RECT, positions map[string]int, w, hgt int32, theme profileTheme) {
	labels := []string{"OVERALL", "EASY", "NORMAL", "HARD", "INSANE", "ENDURANCE"}
	gap := sx(8, w)
	cw := (r.Right - r.Left - gap*int32(len(labels)-1)) / int32(len(labels))
	for i, l := range labels {
		x := r.Left + int32(i)*(cw+gap)
		cr := RECT{x, r.Top, x + cw, r.Bottom}
		border := theme.Accent
		if i%2 == 1 {
			border = theme.Accent2
		}
		drawBevelPanel(hdc, cr, theme.Panel2, border, theme.Deep, 2)
		fillSolidRect(hdc, RECT{cr.Left + 2, cr.Top + 2, cr.Right - 2, cr.Top + sy(5, hgt)}, border)
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, theme.Muted)
			centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(9, hgt), l)
			v := "—"
			if positions[l] > 0 {
				v = fmt.Sprintf("#%d", positions[l])
			}
			setTextColor.Call(hdc, rgb(245, 249, 253))
			centeredTextOut(hdc, cr.Left, cr.Right, cr.Top+sy(34, hgt), v)
			selectObject.Call(hdc, old)
		}
	}
}

func drawShipCollectionStripThemed(hdc uintptr, r RECT, unlocked []int, selected int, w, hgt int32, theme profileTheme) {
	drawBevelPanel(hdc, r, theme.Panel2, theme.Accent2, theme.Deep, 2)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, theme.Accent)
		textOut(hdc, r.Left+sx(14, w), r.Top+sy(10, hgt), "SPACESHIP COLLECTION")
		selectObject.Call(hdc, old)
	}
	owned := map[int]bool{0: true}
	for _, id := range unlocked {
		owned[id] = true
	}
	left, right := r.Left+sx(14, w), r.Right-sx(14, w)
	top, bottom := r.Top+sy(38, hgt), r.Bottom-sy(8, hgt)
	cols := int32(6)
	gapX, gapY := sx(8, w), sy(7, hgt)
	cw := (right - left - gapX*5) / cols
	ch := (bottom - top - gapY) / 2
	for i, id := range garageShipOrder {
		row := int32(i) / cols
		if row >= 2 {
			break
		}
		col := int32(i) % cols
		x := left + col*(cw+gapX)
		y := top + row*(ch+gapY)
		cr := RECT{x, y, x + cw, y + ch}
		face, border := theme.Deep, theme.Accent2
		if id == selected {
			face = theme.Panel
			border = theme.Gold
		}
		drawBevelPanel(hdc, cr, face, border, theme.Deep, 2)
		img := RECT{cr.Left + sx(7, w), cr.Top + sy(4, hgt), cr.Right - sx(7, w), cr.Bottom - sy(4, hgt)}
		if owned[id] {
			drawShipTextureFit(hdc, id, img)
		} else {
			fillSolidRect(hdc, img, theme.Deep)
			if hudSmallFont != 0 {
				old, _, _ := selectObject.Call(hdc, hudSmallFont)
				setTextColor.Call(hdc, theme.Muted)
				centeredTextOut(hdc, img.Left, img.Right, verticallyCenteredTextY(hdc, hudSmallFont, img, "?"), "?")
				selectObject.Call(hdc, old)
			}
		}
	}
}
