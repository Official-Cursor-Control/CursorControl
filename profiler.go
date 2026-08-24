package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type perfTelemetry struct {
	Enabled      bool
	UpdateMS     float64
	RenderMS     float64
	HUDMS        float64
	FrameMS      float64
	FPS          float64
	MaxFrameMS   float64
	MaxUpdateMS  float64
	MaxRenderMS  float64
	MaxHUDMS     float64
	SlowFrames   uint64
	SevereFrames uint64
	DroppedSteps uint64

	HeapAllocMB float64
	HeapSysMB   float64
	HeapObjects uint64
	TotalAlloc  uint64
	AllocMBPS   float64
	NumGC       uint32
	LastGCPause float64
	Goroutines  int
	PeakHeapMB  float64

	lastFrame       time.Time
	smoothFPS       float64
	lastRuntimePoll time.Time
	lastTotalAlloc  uint64
}

var perfStats perfTelemetry

func perfBeginFrame() time.Time { return time.Now() }

func perfPollRuntime(now time.Time) {
	// runtime.ReadMemStats is intentionally sampled only once per second so the
	// diagnostics never become a source of frame-time noise themselves.
	if !perfStats.lastRuntimePoll.IsZero() && now.Sub(perfStats.lastRuntimePoll) < time.Second {
		return
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	perfStats.HeapAllocMB = float64(ms.HeapAlloc) / (1024 * 1024)
	perfStats.HeapSysMB = float64(ms.HeapSys) / (1024 * 1024)
	perfStats.HeapObjects = ms.HeapObjects
	perfStats.NumGC = ms.NumGC
	perfStats.Goroutines = runtime.NumGoroutine()
	if ms.NumGC > 0 {
		idx := (ms.NumGC - 1) % uint32(len(ms.PauseNs))
		perfStats.LastGCPause = float64(ms.PauseNs[idx]) / 1e6
	}
	if perfStats.HeapAllocMB > perfStats.PeakHeapMB {
		perfStats.PeakHeapMB = perfStats.HeapAllocMB
	}
	if !perfStats.lastRuntimePoll.IsZero() && ms.TotalAlloc >= perfStats.lastTotalAlloc {
		dt := now.Sub(perfStats.lastRuntimePoll).Seconds()
		if dt > 0 {
			instant := float64(ms.TotalAlloc-perfStats.lastTotalAlloc) / (1024 * 1024) / dt
			if perfStats.AllocMBPS == 0 {
				perfStats.AllocMBPS = instant
			} else {
				perfStats.AllocMBPS = perfStats.AllocMBPS*0.75 + instant*0.25
			}
		}
	}
	perfStats.TotalAlloc = ms.TotalAlloc
	perfStats.lastTotalAlloc = ms.TotalAlloc
	perfStats.lastRuntimePoll = now
}

func perfEndFrame(start time.Time) {
	now := time.Now()
	ms := now.Sub(start).Seconds() * 1000
	perfStats.FrameMS = perfStats.FrameMS*0.88 + ms*0.12
	if ms > perfStats.MaxFrameMS {
		perfStats.MaxFrameMS = ms
	}
	if !perfStats.lastFrame.IsZero() {
		dt := now.Sub(perfStats.lastFrame).Seconds()
		if dt > 0 {
			f := 1 / dt
			if perfStats.smoothFPS == 0 {
				perfStats.smoothFPS = f
			} else {
				perfStats.smoothFPS = perfStats.smoothFPS*0.90 + f*0.10
			}
			perfStats.FPS = perfStats.smoothFPS
		}
	}
	perfStats.lastFrame = now
	if ms > 25 {
		perfStats.SlowFrames++
	}
	if ms > 50 {
		perfStats.SevereFrames++
	}
	if perfStats.Enabled {
		perfPollRuntime(now)
	}
}

func perfMeasureUpdate(start time.Time) {
	ms := time.Since(start).Seconds() * 1000
	perfStats.UpdateMS = perfStats.UpdateMS*0.88 + ms*0.12
	if ms > perfStats.MaxUpdateMS {
		perfStats.MaxUpdateMS = ms
	}
}
func perfMeasureRender(start time.Time) {
	ms := time.Since(start).Seconds() * 1000
	perfStats.RenderMS = perfStats.RenderMS*0.88 + ms*0.12
	if ms > perfStats.MaxRenderMS {
		perfStats.MaxRenderMS = ms
	}
}
func perfMeasureHUD(start time.Time) {
	ms := time.Since(start).Seconds() * 1000
	perfStats.HUDMS = perfStats.HUDMS*0.88 + ms*0.12
	if ms > perfStats.MaxHUDMS {
		perfStats.MaxHUDMS = ms
	}
}

func perfModeText() string {
	if survivalActive() {
		return fmt.Sprintf("SURVIVAL W%d HP%d ENEMIES%d", survivalWave, survivalHP, len(survivalEnemies))
	}
	if enduranceActive() {
		return fmt.Sprintf("ENDURANCE %.0fm T%d M%d A%d P%d", enduranceDistance, len(targets), len(enduranceBlocks), len(enduranceAlienMinions), len(endurancePowerups))
	}
	return fmt.Sprintf("PRECISION T%d FX%d", len(targets), len(polishEffects))
}

func perfStatusText() string {
	return fmt.Sprintf("FPS %.0f | FRAME %.2fms MAX %.2f | UPDATE %.2f | RENDER %.2f | HUD %.2f | SLOW %d | >50ms %d | DROPPED %d | HEAP %.1fMB | ALLOC %.2fMB/s | GC %d | G %d",
		perfStats.FPS, perfStats.FrameMS, perfStats.MaxFrameMS, perfStats.UpdateMS, perfStats.RenderMS, perfStats.HUDMS,
		perfStats.SlowFrames, perfStats.SevereFrames, perfStats.DroppedSteps, perfStats.HeapAllocMB, perfStats.AllocMBPS, perfStats.NumGC, perfStats.Goroutines)
}

func drawDeveloperPerfOverlay(hdc uintptr, w, hgt int32) {
	if !perfStats.Enabled || hdc == 0 {
		return
	}
	r := RECT{sx(8, w), sy(182, hgt), sx(760, w), sy(284, hgt)}
	fillSolidRect(hdc, r, rgb(2, 10, 28))
	drawLineSimple(hdc, r.Left, r.Top, r.Right, r.Top, 2, rgb(42, 220, 255))
	drawLineSimple(hdc, r.Left, r.Bottom-2, r.Right, r.Bottom-2, 2, rgb(255, 159, 24))
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(225, 246, 255))
		textOut(hdc, r.Left+sx(10, w), r.Top+sy(8, hgt), fmt.Sprintf("FPS %.0f  FRAME %.2fms (MAX %.2f)  UPDATE %.2f  RENDER %.2f  HUD %.2f", perfStats.FPS, perfStats.FrameMS, perfStats.MaxFrameMS, perfStats.UpdateMS, perfStats.RenderMS, perfStats.HUDMS))
		setTextColor.Call(hdc, rgb(255, 205, 82))
		textOut(hdc, r.Left+sx(10, w), r.Top+sy(29, hgt), fmt.Sprintf("SLOW >25ms %d  SEVERE >50ms %d  DROPPED %d  %s", perfStats.SlowFrames, perfStats.SevereFrames, perfStats.DroppedSteps, perfModeText()))
		setTextColor.Call(hdc, rgb(136, 240, 178))
		textOut(hdc, r.Left+sx(10, w), r.Top+sy(50, hgt), fmt.Sprintf("HEAP %.1fMB  PEAK %.1fMB  SYS %.1fMB  OBJECTS %d  ALLOC %.2fMB/s  GC %d (LAST %.2fms)  GOROUTINES %d", perfStats.HeapAllocMB, perfStats.PeakHeapMB, perfStats.HeapSysMB, perfStats.HeapObjects, perfStats.AllocMBPS, perfStats.NumGC, perfStats.LastGCPause, perfStats.Goroutines))
		setTextColor.Call(hdc, rgb(90, 225, 255))
		textOut(hdc, r.Left+sx(10, w), r.Top+sy(73, hgt), "DEV TELEMETRY  /PERF RESET  /PERF SAVE  /PERF OFF")
		selectObject.Call(hdc, old)
	}
}

func writeRunPerfSnapshot(event string) {
	if logRoot == "" {
		return
	}
	perfPollRuntime(time.Now())
	_ = os.MkdirAll(logRoot, 0755)
	line := fmt.Sprintf(
		"%s event=%s course=%s mode=%q distance=%.1f fps=%.1f frame_ms=%.2f frame_max_ms=%.2f update_ms=%.2f update_max_ms=%.2f render_ms=%.2f render_max_ms=%.2f hud_ms=%.2f hud_max_ms=%.2f slow=%d severe=%d dropped=%d heap_mb=%.2f peak_heap_mb=%.2f heap_sys_mb=%.2f heap_objects=%d alloc_mb_s=%.3f num_gc=%d last_gc_pause_ms=%.3f goroutines=%d meteors=%d aliens=%d fx=%d\n",
		time.Now().Format(time.RFC3339),
		event,
		lastResult.Course,
		perfModeText(),
		enduranceDistance,
		perfStats.FPS,
		perfStats.FrameMS,
		perfStats.MaxFrameMS,
		perfStats.UpdateMS,
		perfStats.MaxUpdateMS,
		perfStats.RenderMS,
		perfStats.MaxRenderMS,
		perfStats.HUDMS,
		perfStats.MaxHUDMS,
		perfStats.SlowFrames,
		perfStats.SevereFrames,
		perfStats.DroppedSteps,
		perfStats.HeapAllocMB,
		perfStats.PeakHeapMB,
		perfStats.HeapSysMB,
		perfStats.HeapObjects,
		perfStats.AllocMBPS,
		perfStats.NumGC,
		perfStats.LastGCPause,
		perfStats.Goroutines,
		len(enduranceBlocks),
		len(enduranceAlienMinions),
		len(polishEffects),
	)
	f, err := os.OpenFile(filepath.Join(logRoot, "run_performance.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

func resetPerfForRun() {
	enabled := perfStats.Enabled
	perfStats = perfTelemetry{Enabled: enabled}
}
