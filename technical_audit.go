package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type auditCounter struct {
	Pass  int
	Fail  int
	Lines []string
}

func (a *auditCounter) check(ok bool, name, detail string) {
	status := "PASS"
	if ok {
		a.Pass++
	} else {
		status = "FAIL"
		a.Fail++
	}
	if detail != "" {
		a.Lines = append(a.Lines, fmt.Sprintf("[%s] %s — %s", status, name, detail))
	} else {
		a.Lines = append(a.Lines, fmt.Sprintf("[%s] %s", status, name))
	}
}

func auditRectValid(r RECT) bool { return r.Right > r.Left && r.Bottom > r.Top }
func auditRectW(r RECT) int32    { return r.Right - r.Left }
func auditRectH(r RECT) int32    { return r.Bottom - r.Top }
func auditRectCY(r RECT) int32   { return r.Top + (r.Bottom-r.Top)/2 }
func auditRectWithin(inner, outer RECT) bool {
	return inner.Left >= outer.Left && inner.Top >= outer.Top && inner.Right <= outer.Right && inner.Bottom <= outer.Bottom
}
func auditRectsOverlap(a, b RECT) bool {
	return a.Left < b.Right && a.Right > b.Left && a.Top < b.Bottom && a.Bottom > b.Top
}
func auditEqualSize(rects []RECT) bool {
	if len(rects) < 2 {
		return true
	}
	w, h := auditRectW(rects[0]), auditRectH(rects[0])
	for _, r := range rects[1:] {
		if auditRectW(r) != w || auditRectH(r) != h {
			return false
		}
	}
	return true
}
func auditNoOverlap(rects []RECT) bool {
	for i := 0; i < len(rects); i++ {
		for j := i + 1; j < len(rects); j++ {
			if auditRectsOverlap(rects[i], rects[j]) {
				return false
			}
		}
	}
	return true
}

func auditUIResolution(w, h int32, a *auditCounter) {
	screen := RECT{0, 0, w, h}
	prefix := fmt.Sprintf("%dx%d", w, h)

	ar := arenaRect(w, h)
	a.check(auditRectValid(ar) && auditRectWithin(ar, screen), prefix+" arena bounds", fmt.Sprintf("%dx%d", auditRectW(ar), auditRectH(ar)))

	// Current/default HUD controls all use the same normalized rect factories as hit testing.
	mode := enduranceModeButtonRect(w, h)
	a.check(auditRectWithin(mode, screen), prefix+" mode-switch bounds", "")

	quick := quickAccessRects(w, h)
	q := []RECT{quick[0], quick[1], quick[2]}
	a.check(auditNoOverlap(q), prefix+" profile/local/global non-overlap", "")
	a.check(auditRectCY(q[0]) == auditRectCY(q[1]) && auditRectCY(q[1]) == auditRectCY(q[2]),
		prefix+" profile/local/global Y-centre", fmt.Sprintf("%d/%d/%d", auditRectCY(q[0]), auditRectCY(q[1]), auditRectCY(q[2])))
	for i, r := range q {
		a.check(auditRectWithin(r, screen), fmt.Sprintf("%s quick button %d bounds", prefix, i+1), "")
	}

	bug := quickBugReportRect(w, h)
	support := quickSupportDevRect(w, h)
	discord := quickDiscordLoginRect(w, h)
	actions := []RECT{bug, support, discord}
	a.check(auditEqualSize(actions), prefix+" Bug/Support/Discord identical boxes",
		fmt.Sprintf("%dx%d", auditRectW(bug), auditRectH(bug)))
	a.check(bug.Left == support.Left && support.Left == discord.Left, prefix+" Bug/Support/Discord X alignment", "")
	a.check(auditNoOverlap(actions), prefix+" Bug/Support/Discord non-overlap", "")
	g1 := support.Top - bug.Bottom
	g2 := discord.Top - support.Bottom
	a.check(g1 == g2, prefix+" Bug/Support/Discord equal spacing", fmt.Sprintf("%d/%d", g1, g2))

	coin, cache, garage := enduranceSpaceUIRects(w, h)
	spaceControls := []RECT{coin, cache, garage}
	a.check(auditNoOverlap(spaceControls), prefix+" Endurance lower-HUD controls non-overlap", "")
	a.check(auditRectCY(coin) == auditRectCY(cache) && auditRectCY(cache) == auditRectCY(garage),
		prefix+" Endurance lower-HUD Y-centre", "")
	for i, r := range spaceControls {
		a.check(auditRectWithin(r, screen), fmt.Sprintf("%s Endurance HUD control %d bounds", prefix, i+1), "")
	}

	menuBtn := menuButtonRect(w, h)
	menuPanel := menuPanelRect(w, h)
	a.check(auditRectWithin(menuBtn, ar), prefix+" MENU button inside arena", "")
	a.check(auditRectWithin(menuPanel, ar), prefix+" quick menu panel inside arena", "")
	a.check(!auditRectsOverlap(menuBtn, menuPanel), prefix+" MENU button/panel separation", "")

	mainPanel := centeredPanel(w, h, 940, 820)
	menuRects := mainMenuRects(w, h)
	menuSlice := make([]RECT, 0, len(menuRects))
	for _, r := range menuRects {
		menuSlice = append(menuSlice, r)
	}
	a.check(auditEqualSize(menuSlice), prefix+" main-menu button sizing", "")
	a.check(auditNoOverlap(menuSlice), prefix+" main-menu button spacing", "")
	inside := true
	for _, r := range menuSlice {
		if !auditRectWithin(r, mainPanel) {
			inside = false
		}
	}
	a.check(inside, prefix+" main-menu buttons inside panel", "")

	settingsPanel := centeredPanel(w, h, 900, 840)
	visibleSettings := settingsVisibleRows(w, h)
	setSlice := make([]RECT, 0, len(visibleSettings))
	for _, row := range visibleSettings {
		setSlice = append(setSlice, row.Rect)
	}
	a.check(auditEqualSize(setSlice), prefix+" settings row sizing", "")
	a.check(auditNoOverlap(setSlice), prefix+" settings row spacing", "")
	inside = true
	for _, r := range setSlice {
		if !auditRectWithin(r, settingsPanel) {
			inside = false
		}
	}
	a.check(inside, prefix+" settings rows inside panel", "")

	garagePanel := garagePanelRect(w, h)
	cards := garageShipCardRects(w, h)
	cardSlice := make([]RECT, 0, len(cards))
	for _, r := range cards {
		cardSlice = append(cardSlice, r)
	}
	a.check(auditEqualSize(cardSlice), prefix+" Garage card sizing", "")
	a.check(auditNoOverlap(cardSlice), prefix+" Garage card spacing", "")
	inside = true
	for _, r := range cardSlice {
		if !auditRectWithin(r, garagePanel) {
			inside = false
		}
	}
	a.check(inside, prefix+" Garage cards inside panel", "")

	ach := achievementPanelRect(w, h)
	track, thumb := achievementScrollbarRects(w, h)
	a.check(auditRectWithin(ach, screen), prefix+" Achievements panel bounds", "")
	a.check(auditRectWithin(track, ach) && auditRectWithin(thumb, track), prefix+" Achievement scrollbar bounds", "")

	filterLeft := ar.Left + sx(73, w)
	filterRight := ar.Right - sx(73, w)
	filterTop := ar.Top + sy(96, h)
	tabs := leaderboardFilterRects(filterLeft, filterRight, filterTop, w, h)
	tabSlice := make([]RECT, 0, len(tabs))
	for _, r := range tabs {
		tabSlice = append(tabSlice, r)
	}
	a.check(auditNoOverlap(tabSlice), prefix+" leaderboard tabs non-overlap", "")
	equalHeight := true
	for _, r := range tabSlice[1:] {
		if auditRectH(r) != auditRectH(tabSlice[0]) {
			equalHeight = false
		}
	}
	a.check(equalHeight, prefix+" leaderboard tab height consistency", "")

	// Fundamental hitbox semantics used by every rectangle-based UI control.
	test := RECT{10, 20, 110, 70}
	a.check(pointInRect(FPoint{60, 45}, test), prefix+" hitbox centre accepted", "")
	a.check(!pointInRect(FPoint{9, 45}, test) && !pointInRect(FPoint{111, 45}, test),
		prefix+" hitbox outside rejected", "")
}

func runUIConsistencyAudit() string {
	savedCfg := hudLayoutConfig
	savedLoaded := hudLayoutLoaded
	savedMotion := gameMeta.ReducedMotion
	defer func() {
		hudLayoutConfig = savedCfg
		hudLayoutLoaded = savedLoaded
		gameMeta.ReducedMotion = savedMotion
	}()

	gameMeta.ReducedMotion = true
	hudLayoutConfig = applyNormalizedHUDLayout(defaultHUDLayoutConfig())
	hudLayoutLoaded = true

	var a auditCounter
	resolutions := [][2]int32{{1024, 600}, {1152, 648}, {1280, 720}, {1366, 768}, {1536, 864}, {1536, 1024}, {1600, 900}, {1920, 1080}, {2560, 1440}, {3840, 2160}}
	if mainHwnd != 0 {
		w, h := getClient(mainHwnd)
		if w > 0 && h > 0 {
			resolutions = append([][2]int32{{w, h}}, resolutions...)
		}
	}
	seen := map[[2]int32]bool{}
	for _, res := range resolutions {
		if seen[res] {
			continue
		}
		seen[res] = true
		auditUIResolution(res[0], res[1], &a)
	}

	var b strings.Builder
	b.WriteString("Cursor Control " + clientBuildVersion + " UI / Geometry Audit\n")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n\n")
	for _, line := range a.Lines {
		b.WriteString(line + "\n")
	}
	fmt.Fprintf(&b, "\nRESULT %d PASS / %d FAIL\n", a.Pass, a.Fail)
	report := b.String()
	if logRoot != "" {
		_ = atomicWriteFile(filepath.Join(logRoot, "ui_audit_latest.txt"), []byte(report), 0644)
	}
	logRuntimeEvent("ui_audit", fmt.Sprintf("pass=%d fail=%d", a.Pass, a.Fail))
	return fmt.Sprintf("UI AUDIT %d PASS / %d FAIL — logs/ui_audit_latest.txt", a.Pass, a.Fail)
}

type auditBGRAAsset struct {
	Name    string
	Data    []byte
	W, H    int32
	Trimmed bool
}

func auditAlphaAsset(asset auditBGRAAsset, a *auditCounter) {
	expected := int(asset.W * asset.H * 4)
	if len(asset.Data) != expected {
		a.check(false, "asset "+asset.Name+" dimensions", fmt.Sprintf("bytes=%d expected=%d", len(asset.Data), expected))
		return
	}
	bounds := alphaBoundsBGRA(asset.Data, asset.W, asset.H, 5)
	bw, bh := auditRectW(bounds), auditRectH(bounds)
	cx := float64(bounds.Left+bounds.Right)/2 - float64(asset.W)/2
	cy := float64(bounds.Top+bounds.Bottom)/2 - float64(asset.H)/2
	coverage := float64(bw*bh) / float64(asset.W*asset.H) * 100
	a.check(bw > 0 && bh > 0, "asset "+asset.Name+" alpha bounds",
		fmt.Sprintf("visible=%dx%d coverage=%.1f%% raw-centre-offset=(%.1f,%.1f) trim-aware=%t", bw, bh, coverage, cx, cy, asset.Trimmed))
}

func runAssetConsistencyAudit() string {
	var a auditCounter
	if err := validateRuntimeAssets(); err != nil {
		a.check(false, "critical runtime assets", err.Error())
	} else {
		a.check(true, "critical runtime assets", "validateRuntimeAssets PASS")
	}
	if _, err := os.Stat(filepath.Join(cacheRoot, "textures")); os.IsNotExist(err) {
		a.check(true, "legacy cache/textures absent", "")
	} else {
		a.check(false, "legacy cache/textures absent", "immutable textures must live in assets/textures")
	}

	assets := []auditBGRAAsset{
		{"Profile button", profileButtonBGRA, 1200, 700, true},
		{"Local button", localButtonBGRA, 1200, 700, true},
		{"Global button", globalButtonBGRA, 1200, 700, true},
		{"Bug button", bugReportButtonBGRA, 1200, 290, true},
		{"Support button", supportDevButtonBGRA, 1200, 290, true},
		{"Discord login", discordLoginButtonBGRA, 1200, 290, true},
		{"Discord logged in", discordLoggedInButtonBGRA, 1200, 290, true},
		{"Precision mode", precisionModeButtonBGRA, 2048, 682, true},
		{"Endurance mode", enduranceModeButtonBGRA, 2048, 682, true},
		{"Space Coin", spaceCoinBGRA, 200, 203, false},
		{"Space Coin bar", spaceCoinBarBGRA, 468, 177, false},
		{"Space Cache button", spaceCacheButtonBGRA, 515, 205, false},
		{"Garage button", garageButtonBGRA, 470, 176, false},
		{"Shield power-up", powerupShieldBGRA, 64, 64, false},
		{"Time power-up", powerupTimeBGRA, 64, 64, false},
		{"Default ship", defaultShipBGRA, 32, 22, false},
		{"Blue meteor", hazardBlueBGRA, 64, 160, false},
		{"Orange meteor", hazardOrangeBGRA, 64, 160, false},
	}
	for _, asset := range assets {
		auditAlphaAsset(asset, &a)
	}

	// Action images deliberately share one source canvas and one runtime box size.
	a.check(len(bugReportButtonBGRA) == len(supportDevButtonBGRA) && len(supportDevButtonBGRA) == len(discordLoginButtonBGRA),
		"Bug/Support/Discord source canvas consistency", "1200x290 expected")
	a.check(sharedAuthHTTPClient != nil && sharedAdminHTTPClient != nil, "shared HTTP clients", "network clients reused outside gameplay render path")

	var b strings.Builder
	b.WriteString("Cursor Control " + clientBuildVersion + " Asset / Runtime Resource Audit\n")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n\n")
	for _, line := range a.Lines {
		b.WriteString(line + "\n")
	}
	fmt.Fprintf(&b, "\nRESULT %d PASS / %d FAIL\n", a.Pass, a.Fail)
	report := b.String()
	if logRoot != "" {
		_ = atomicWriteFile(filepath.Join(logRoot, "asset_audit_latest.txt"), []byte(report), 0644)
	}
	logRuntimeEvent("asset_audit", fmt.Sprintf("pass=%d fail=%d", a.Pass, a.Fail))
	return fmt.Sprintf("ASSET AUDIT %d PASS / %d FAIL — logs/asset_audit_latest.txt", a.Pass, a.Fail)
}

func runTechnicalConsistencyAudit() string {
	ui := runUIConsistencyAudit()
	assets := runAssetConsistencyAudit()
	self := runRuntimeSelfTest()
	diag := runtimeDiagnosticText()
	report := fmt.Sprintf("Cursor Control "+clientBuildVersion+" Technical Consistency Audit\n%s\n\n%s\n%s\n%s\n\n%s\n",
		time.Now().Format(time.RFC3339), ui, assets, self, diag)
	if logRoot != "" {
		_ = atomicWriteFile(filepath.Join(logRoot, "technical_audit_latest.txt"), []byte(report), 0644)
	}
	logRuntimeEvent("technical_audit", "completed")
	return "TECH AUDIT COMPLETE — UI + ASSETS + SELFTEST + DIAG — logs/technical_audit_latest.txt"
}
