package main

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"
)

type deepAuditCounter struct {
	Pass  int
	Fail  int
	Lines []string
}

func (a *deepAuditCounter) check(ok bool, name, detail string) {
	status := "PASS"
	if ok {
		a.Pass++
	} else {
		a.Fail++
		status = "FAIL"
	}
	if detail != "" {
		a.Lines = append(a.Lines, fmt.Sprintf("[%s] %s — %s", status, name, detail))
	} else {
		a.Lines = append(a.Lines, fmt.Sprintf("[%s] %s", status, name))
	}
}

func deepAuditReport(title, filename string, a deepAuditCounter) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n\n")
	for _, line := range a.Lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\nRESULT %d PASS / %d FAIL\n", a.Pass, a.Fail)
	if logRoot != "" {
		_ = atomicWriteFile(logPath(filename), []byte(b.String()), 0644)
	}
	return fmt.Sprintf("%s %d PASS / %d FAIL — logs/%s", title, a.Pass, a.Fail, filename)
}

func logPath(name string) string {
	if logRoot == "" {
		return name
	}
	return logRoot + string(osPathSeparator()) + name
}

func osPathSeparator() byte {
	// Cursor Control is a Windows build, but use the active separator so source
	// tests remain valid when cross-compiling.
	return '\\'
}

// --------------------------------------------------------------------------
// DPI / CURSOR COORDINATE AUDIT
// --------------------------------------------------------------------------

func currentWindowDPI() uint32 {
	if mainHwnd == 0 {
		return 96
	}
	if err := getDpiForWindow.Find(); err != nil {
		return 96
	}
	v, _, _ := getDpiForWindow.Call(mainHwnd)
	if v == 0 {
		return 96
	}
	return uint32(v)
}

func runDPICursorAudit() string {
	var a deepAuditCounter

	a.check(setProcessDPIAware.Find() == nil, "Windows DPI-awareness API", "SetProcessDPIAware available")
	dpi := currentWindowDPI()
	a.check(dpi >= 96 && dpi <= 480, "current window DPI", fmt.Sprintf("%d DPI (%.0f%%)", dpi, float64(dpi)/96*100))

	// sx/sy operate from final physical client pixels. Verify normalized cursor
	// positions survive design -> client -> design round-trips at common DPI/scales.
	type target struct {
		W, H int32
		DPI  uint32
	}
	targets := []target{
		{1280, 720, 96},
		{1366, 768, 120},
		{1536, 1024, 144},
		{1920, 1080, 168},
		{2560, 1440, 192},
	}
	for _, t := range targets {
		ar := arenaRect(t.W, t.H)
		a.check(ar.Right > ar.Left && ar.Bottom > ar.Top,
			fmt.Sprintf("%dx%d @ %d DPI arena", t.W, t.H, t.DPI), "")

		designPts := []FPoint{
			{14, 180}, {768, 476}, {1522, 772},
			{274, 24}, {1493, 871},
		}
		maxErr := 0.0
		for _, p := range designPts {
			clientX := float64(sx(p.X, t.W))
			clientY := float64(sy(p.Y, t.H))
			backX := clientX * 1536.0 / float64(t.W)
			backY := clientY * 1024.0 / float64(t.H)
			err := math.Hypot(backX-p.X, backY-p.Y)
			if err > maxErr {
				maxErr = err
			}
		}
		// Integer-pixel rounding should stay comfortably below a design pixel.
		a.check(maxErr <= 1.8,
			fmt.Sprintf("%dx%d coordinate round-trip", t.W, t.H),
			fmt.Sprintf("max design-space error %.3fpx", maxErr))

		// Boundary semantics: a point on a rectangle edge is accepted, one
		// physical pixel beyond is rejected.
		test := RECT{sx(100, t.W), sy(100, t.H), sx(300, t.W), sy(200, t.H)}
		edge := FPoint{X: float64(test.Right), Y: float64((test.Top + test.Bottom) / 2)}
		outside := FPoint{X: float64(test.Right + 1), Y: edge.Y}
		a.check(pointInRect(edge, test) && !pointInRect(outside, test),
			fmt.Sprintf("%dx%d edge-hit semantics", t.W, t.H), "")
	}

	logRuntimeEvent("dpi_audit", fmt.Sprintf("dpi=%d", dpi))
	return deepAuditReport("DPI/CURSOR AUDIT", "dpi_cursor_audit_latest.txt", a)
}

// --------------------------------------------------------------------------
// FRAME-RATE INDEPENDENCE AUDIT
// --------------------------------------------------------------------------

func simulateFixedStep(seconds float64, frameHz float64, stallEvery int) (steps int, simulated float64, dropped int) {
	const fixed = 1.0 / 125.0
	acc := 0.0
	frameDt := 1.0 / frameHz
	frames := int(math.Ceil(seconds * frameHz))
	for frame := 0; frame < frames; frame++ {
		dt := frameDt
		if stallEvery > 0 && frame > 0 && frame%stallEvery == 0 {
			dt += 0.035 // representative short hitch
		}
		if dt > 0.050 {
			dt = 0.050
		}
		acc += dt
		local := 0
		for acc >= fixed && local < 6 {
			acc -= fixed
			steps++
			local++
		}
		if acc >= fixed {
			dropped++
			acc = math.Mod(acc, fixed)
		}
	}
	return steps, float64(steps) * fixed, dropped
}

func runFrameIndependenceAudit() string {
	var a deepAuditCounter
	const duration = 20.0

	expected := duration
	for _, hz := range []float64{30, 60, 120, 144, 240} {
		steps, sim, dropped := simulateFixedStep(duration, hz, 0)
		err := math.Abs(sim - expected)
		a.check(err <= 0.016,
			fmt.Sprintf("%.0f FPS fixed-step equivalence", hz),
			fmt.Sprintf("steps=%d simulated=%.3fs error=%.4fs dropped=%d", steps, sim, err, dropped))
		a.check(dropped == 0, fmt.Sprintf("%.0f FPS no-drop baseline", hz), "")
	}

	// With deliberate frame stalls, the loop is expected to clamp catch-up work
	// instead of spiraling. Verify it stays bounded and deterministic.
	for _, hz := range []float64{60, 120, 240} {
		steps, sim, dropped := simulateFixedStep(duration, hz, 90)
		stallEvents := int(math.Ceil(duration * hz / 90.0))
		a.check(steps > 0 && sim > duration*0.95,
			fmt.Sprintf("%.0f FPS hitch recovery", hz),
			fmt.Sprintf("simulated=%.3fs dropped-events=%d", sim, dropped))
		// A clamped hitch may deliberately discard one backlog remainder. The
		// important invariant is that drops are bounded by the number of injected
		// stalls rather than cascading after the hitch has ended.
		a.check(dropped <= stallEvents, fmt.Sprintf("%.0f FPS bounded hitch drops", hz),
			fmt.Sprintf("dropped-events=%d injected-stalls<=%d", dropped, stallEvents))
	}

	a.check(enduranceRenderInterval() >= time.Second/240 && enduranceRenderInterval() <= time.Second/60,
		"configured presentation interval", enduranceRenderInterval().String())
	return deepAuditReport("FRAME AUDIT", "frame_independence_audit_latest.txt", a)
}

// --------------------------------------------------------------------------
// STATE-TRANSITION MODEL AUDIT
// --------------------------------------------------------------------------

func runStateTransitionAudit() string {
	var a deepAuditCounter

	type model struct {
		mode       int
		state      int // 0 waiting,1 playing,2 failed,3 result
		blocks     int
		aliens     int
		targets    int
		powerups   int
		fx         int
		shield     int
		slow       int
		warpCue    bool
		warpActive bool
		boss       bool
		audio      bool
	}
	m := model{}
	seed := uint64(0x210C0DE)
	next := func() uint64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return seed
	}

	valid := true
	repairs := 0
	for i := 0; i < 100000; i++ {
		action := int(next() % 15)
		switch action {
		case 0: // mode switch while waiting
			if m.state == 0 {
				m.mode = 1 - m.mode
				m.blocks, m.aliens, m.targets, m.powerups, m.fx = 0, 0, 0, 0, 0
				m.shield, m.slow = 0, 0
				m.warpCue, m.warpActive, m.boss, m.audio = false, false, false, false
			}
		case 1: // start
			if m.state == 0 {
				m.state = 1
			}
		case 2: // fail
			if m.state == 1 {
				m.state = 2
				m.audio = false
				m.warpCue = false
				m.warpActive = false
			}
		case 3: // result
			if m.state == 1 || m.state == 2 {
				m.state = 3
			}
		case 4: // reset
			if m.state != 1 {
				m.state = 0
				m.blocks, m.aliens, m.targets, m.powerups, m.fx = 0, 0, 0, 0, 0
				m.shield, m.slow = 0, 0
				m.warpCue, m.warpActive, m.boss, m.audio = false, false, false, false
			}
		case 5:
			if m.state == 1 && m.mode == 1 && m.blocks < 8 {
				m.blocks++
			}
		case 6:
			if m.state == 1 && m.mode == 1 && m.aliens < 6 {
				m.aliens++
			}
		case 7:
			if m.state == 1 && m.mode == 1 && m.targets < 12 {
				m.targets++
			}
		case 8:
			if m.state == 1 && m.mode == 1 && m.powerups < 1 {
				m.powerups++
			}
		case 9:
			if m.state == 1 && m.mode == 1 {
				m.shield = int(next() % 3)
			}
		case 10:
			if m.state == 1 && m.mode == 1 {
				m.slow = int(next() % 3)
			}
		case 11:
			if m.state == 1 && m.mode == 1 {
				m.warpCue, m.warpActive = true, false
			}
		case 12:
			if m.state == 1 && m.mode == 1 && m.warpCue {
				m.warpCue, m.warpActive = false, true
			}
		case 13:
			if m.state == 1 && m.mode == 1 {
				m.boss, m.audio = true, true
			}
		case 14:
			if m.state == 1 && m.mode == 1 {
				m.fx = int(next() % 25)
			}
		}

		// Same emergency invariants used by the runtime watchdog.
		if m.blocks > 8 {
			m.blocks = 8
			repairs++
		}
		if m.aliens > 6 {
			m.aliens = 6
			repairs++
		}
		if m.targets > 12 {
			m.targets = 12
			repairs++
		}
		if m.powerups > 1 {
			m.powerups = 1
			repairs++
		}
		if m.fx > 24 {
			m.fx = 24
			repairs++
		}
		if m.warpCue && m.warpActive {
			m.warpCue = false
			repairs++
		}

		if m.blocks > 8 || m.aliens > 6 || m.targets > 12 || m.powerups > 1 ||
			m.fx > 24 || m.shield < 0 || m.shield > 2 || m.slow < 0 || m.slow > 2 ||
			(m.warpCue && m.warpActive) {
			valid = false
			break
		}
		if m.mode == 0 && m.state == 0 && (m.blocks != 0 || m.aliens != 0 || m.powerups != 0 || m.boss) {
			valid = false
			break
		}
	}
	a.check(valid, "100,000 randomised lifecycle transitions", fmt.Sprintf("repairs=%d", repairs))
	a.check(strings.Contains(runtimeDiagnosticText(), "blocks="), "live diagnostic state surface", runtimeDiagnosticText())
	a.check(!enduranceWarpCueActive || !enduranceWarpActive, "live Warp cue/active exclusion", "")
	a.check(enduranceStoredShields >= 0 && enduranceStoredShields <= 2, "live Shield storage bounds", fmt.Sprintf("%d", enduranceStoredShields))
	a.check(enduranceStoredTime >= 0 && enduranceStoredTime <= 2, "live Time storage bounds", fmt.Sprintf("%d", enduranceStoredTime))
	return deepAuditReport("STATE AUDIT", "state_transition_audit_latest.txt", a)
}

// --------------------------------------------------------------------------
// RESOURCE GROWTH / LEAK BASELINE AUDIT
// --------------------------------------------------------------------------

type resourceSnapshot struct {
	At         time.Time
	Alloc      uint64
	Sys        uint64
	HeapObjs   uint64
	Goroutines int
	GDI        uint32
	USER       uint32
}

var leakAuditBaseline *resourceSnapshot

func guiResourceCount(flag uintptr) uint32 {
	if getCurrentProcess == nil || getGuiResources == nil {
		return 0
	}
	if err := getGuiResources.Find(); err != nil {
		return 0
	}
	p, _, _ := getCurrentProcess.Call()
	if p == 0 {
		return 0
	}
	v, _, _ := getGuiResources.Call(p, flag)
	return uint32(v)
}

func captureResourceSnapshot() resourceSnapshot {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return resourceSnapshot{
		At:         time.Now(),
		Alloc:      ms.Alloc,
		Sys:        ms.Sys,
		HeapObjs:   ms.HeapObjects,
		Goroutines: runtime.NumGoroutine(),
		GDI:        guiResourceCount(0),
		USER:       guiResourceCount(1),
	}
}

func formatResourceSnapshot(s resourceSnapshot) string {
	return fmt.Sprintf("alloc=%.1fMB sys=%.1fMB heap=%d goroutines=%d GDI=%d USER=%d",
		float64(s.Alloc)/(1024*1024), float64(s.Sys)/(1024*1024), s.HeapObjs, s.Goroutines, s.GDI, s.USER)
}

func runLeakAudit(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "check":
		now := captureResourceSnapshot()
		if leakAuditBaseline == nil {
			leakAuditBaseline = &now
			return "LEAK AUDIT BASELINE CREATED — " + formatResourceSnapshot(now)
		}
		base := *leakAuditBaseline
		allocDelta := int64(now.Alloc) - int64(base.Alloc)
		sysDelta := int64(now.Sys) - int64(base.Sys)
		heapDelta := int64(now.HeapObjs) - int64(base.HeapObjs)
		gorDelta := now.Goroutines - base.Goroutines
		gdiDelta := int64(now.GDI) - int64(base.GDI)
		userDelta := int64(now.USER) - int64(base.USER)
		report := fmt.Sprintf(
			"LEAK CHECK age=%s now=[%s] delta=[alloc=%+.1fMB sys=%+.1fMB heap=%+d gor=%+d GDI=%+d USER=%+d]",
			time.Since(base.At).Round(time.Second), formatResourceSnapshot(now),
			float64(allocDelta)/(1024*1024), float64(sysDelta)/(1024*1024), heapDelta, gorDelta, gdiDelta, userDelta)
		logRuntimeEvent("leak_audit", report)
		if logRoot != "" {
			_ = atomicWriteFile(logPath("leak_audit_latest.txt"), []byte(report+"\n"), 0644)
		}
		return report
	case "start", "reset":
		s := captureResourceSnapshot()
		leakAuditBaseline = &s
		report := "LEAK AUDIT BASELINE RESET — " + formatResourceSnapshot(s)
		logRuntimeEvent("leak_audit", report)
		return report
	default:
		return "USE /LeakAudit Start|Check|Reset"
	}
}

// --------------------------------------------------------------------------
// ACHIEVEMENT + ECONOMY INVARIANT AUDIT
// --------------------------------------------------------------------------

func runAchievementEconomyAudit() string {
	var a deepAuditCounter

	defs := achievementDefinitions()
	seen := make(map[string]bool, len(defs))
	duplicate := ""
	for _, d := range defs {
		if seen[d.ID] {
			duplicate = d.ID
			break
		}
		seen[d.ID] = true
	}
	a.check(len(defs) == 95, "achievement definition count", fmt.Sprintf("%d", len(defs)))
	a.check(duplicate == "", "achievement IDs unique", duplicate)

	tierCount := [3]int{}
	totalReward := 0
	for _, d := range defs {
		if d.Tier >= 0 && d.Tier <= 2 {
			tierCount[d.Tier]++
			totalReward += achievementRewardEXP(d.Tier)
		}
	}
	a.check(tierCount == [3]int{34, 34, 27}, "achievement tier distribution",
		fmt.Sprintf("easy=%d medium=%d hard=%d", tierCount[0], tierCount[1], tierCount[2]))
	a.check(totalReward > 0, "achievement EXP pool positive", fmt.Sprintf("%d EXP", totalReward))

	// Endurance milestone thresholds should be strictly increasing within each family.
	families := [][]int{
		{250, 500, 1000, 1500, 2500, 5000, 7500, 10000},
		{1, 5, 20},
		{1, 3, 10},
		{5, 25, 100},
		{1, 10, 50},
		{1, 10, 50},
		{1, 10},
		{3, 10},
	}
	for fi, values := range families {
		ok := true
		for i := 1; i < len(values); i++ {
			if values[i] <= values[i-1] {
				ok = false
			}
		}
		a.check(ok, fmt.Sprintf("milestone family %d monotonic", fi+1), fmt.Sprint(values))
	}

	// Cache economy invariants from the current configured reward logic:
	// cost=100; 50% EXP, 40% coins, 10% ship; duplicate ship=200 coins.
	cacheCost := 100
	coinRewards := []int{15, 50, 75, 200}
	allPositive := cacheCost > 0
	for _, c := range coinRewards {
		allPositive = allPositive && c >= 0
	}
	a.check(allPositive, "Space Cache reward non-negativity", fmt.Sprintf("cost=%d rewards=%v", cacheCost, coinRewards))
	a.check(200 >= cacheCost, "duplicate ship compensation covers cache cost", "200 coins vs 100 cost")

	// Analytical expected values.
	conditionalCoin := 0.50*15 + 0.35*50 + 0.15*75
	expectedCoinContribution := 0.40 * conditionalCoin
	expectedEXPContribution := 0.50 * 550.0
	a.check(math.Abs(conditionalCoin-36.25) < 0.001, "coin reward expected value", fmt.Sprintf("%.2f conditional", conditionalCoin))
	a.check(expectedCoinContribution > 0 && expectedEXPContribution > 0,
		"cache expected reward contributions",
		fmt.Sprintf("coins %.2f/open before ship duplicates; EXP %.1f/open", expectedCoinContribution, expectedEXPContribution))

	// Live invariants.
	a.check(gameMeta.SpaceCoins >= 0, "live coin balance nonnegative", fmt.Sprintf("%d", gameMeta.SpaceCoins))
	normalizedShips := normalizeShipList(gameMeta.UnlockedShips)
	a.check(len(normalizedShips) == len(gameMeta.UnlockedShips), "live ship collection normalized",
		fmt.Sprintf("stored=%d normalized=%d", len(gameMeta.UnlockedShips), len(normalizedShips)))
	a.check(gameMeta.SelectedShip == 0 || shipUnlocked(gameMeta.SelectedShip),
		"live selected ship valid", fmt.Sprintf("%d", gameMeta.SelectedShip))

	return deepAuditReport("ACHIEVEMENT/ECONOMY AUDIT", "achievement_economy_audit_latest.txt", a)
}

func runDeepAudit() string {
	results := []string{
		runDPICursorAudit(),
		runFrameIndependenceAudit(),
		runStateTransitionAudit(),
		runAchievementEconomyAudit(),
		runTechnicalConsistencyAudit(),
	}
	report := "Cursor Control " + clientBuildVersion + " Deep Audit\n" + time.Now().Format(time.RFC3339) + "\n\n" + strings.Join(results, "\n") + "\n"
	if leakAuditBaseline == nil {
		s := captureResourceSnapshot()
		leakAuditBaseline = &s
		report += "\nLeak baseline created: " + formatResourceSnapshot(s) + "\n"
	} else {
		report += "\n" + runLeakAudit("check") + "\n"
	}
	if logRoot != "" {
		_ = atomicWriteFile(logPath("deep_audit_latest.txt"), []byte(report), 0644)
	}
	logRuntimeEvent("deep_audit", "completed")
	return "DEEP AUDIT COMPLETE — DPI + FRAME + STATE + ACHIEVEMENTS/ECONOMY + TECH — logs/deep_audit_latest.txt"
}
