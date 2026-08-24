package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func runEndurancePathBoundsAudit() string {
	type resolution struct{ W, H int32 }
	resolutions := []resolution{
		{1280, 720},
		{1366, 768},
		{1536, 1024},
		{1920, 1080},
		{2560, 1440},
	}

	var lines []string
	pass, fail := 0, 0
	check := func(ok bool, name, detail string) {
		status := "PASS"
		if ok {
			pass++
		} else {
			fail++
			status = "FAIL"
		}
		lines = append(lines, fmt.Sprintf("[%s] %s — %s", status, name, detail))
	}

	for _, res := range resolutions {
		ar := arenaRect(res.W, res.H)
		top, bottom := endurancePathCenterBounds(ar)
		halfVisibleRail := (24.0 + 26.0) / 2.0
		check(top-halfVisibleRail >= float64(ar.Top),
			fmt.Sprintf("%dx%d top visual containment", res.W, res.H),
			fmt.Sprintf("arena=%d centerMin=%.1f visibleTop=%.1f", ar.Top, top, top-halfVisibleRail))
		check(bottom+halfVisibleRail <= float64(ar.Bottom),
			fmt.Sprintf("%dx%d bottom visual containment", res.W, res.H),
			fmt.Sprintf("arena=%d centerMax=%.1f visibleBottom=%.1f", ar.Bottom, bottom, bottom+halfVisibleRail))
		check(bottom > top,
			fmt.Sprintf("%dx%d usable path band", res.W, res.H),
			fmt.Sprintf("%.1fpx", bottom-top))
	}

	if mainHwnd != 0 && enduranceActive() && len(path) > 0 {
		w, hgt := getClient(mainHwnd)
		ar := arenaRect(w, hgt)
		top, bottom := endurancePathCenterBounds(ar)
		minY, maxY := math.MaxFloat64, -math.MaxFloat64
		for _, p := range path {
			if p.Y < minY {
				minY = p.Y
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
		check(endurancePathWithinArena(w, hgt),
			"live Endurance path centre containment",
			fmt.Sprintf("pathY=%.1f..%.1f allowed=%.1f..%.1f", minY, maxY, top, bottom))
		check(minY-25.0 >= float64(ar.Top) && maxY+25.0 <= float64(ar.Bottom),
			"live complete rail visual containment",
			fmt.Sprintf("visibleY=%.1f..%.1f arena=%d..%d", minY-25.0, maxY+25.0, ar.Top, ar.Bottom))
	}

	var b strings.Builder
	b.WriteString("Cursor Control " + clientBuildVersion + " Endurance Path Bounds Audit\n")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n\n")
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\nRESULT %d PASS / %d FAIL\n", pass, fail)
	if logRoot != "" {
		_ = atomicWriteFile(logPath("path_bounds_audit_latest.txt"), []byte(b.String()), 0644)
	}
	logRuntimeEvent("path_bounds_audit", fmt.Sprintf("pass=%d fail=%d", pass, fail))
	return fmt.Sprintf("PATH AUDIT %d PASS / %d FAIL — logs/path_bounds_audit_latest.txt", pass, fail)
}
