//go:build windows

package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Survival boss dossier cinematic.
// Production flow:
// cleared wave -> fade to black/cut sector music -> dossier + intro audio ->
// glitch/fade to black/cut intro audio -> fade directly into active boss fight.
const (
	survivalBossIntroNone = iota
	survivalBossIntroWaveFade
	survivalBossIntroPoster
	survivalBossIntroBossFadeIn
)

const (
	survivalBossIntroWaveFadeDur = 850 * time.Millisecond
	survivalBossIntroPosterDur   = 5 * time.Second
	survivalBossIntroFightFade   = 850 * time.Millisecond
)

var (
	survivalBossIntroStage int
	survivalBossIntroBoss  int
	survivalBossIntroAt    time.Time

	survivalBossIntroSharp [3][]byte
	survivalBossIntroBlur  [3][]byte
	survivalBossIntroRed   []byte
	survivalBossIntroCyan  []byte

	survivalBossIntroAudioReady bool
	survivalBossIntroFont       uintptr
	survivalBossIntroOnce       sync.Once

	// True only while the shared dossier cinematic hands control to an existing
	// boss initializer. Boss entry functions use this to skip their legacy
	// wave->boss fades, because the dossier has already performed that handoff.
	survivalBossIntroHandoff bool
)

func survivalBossIntroActive() bool { return survivalBossIntroStage != survivalBossIntroNone }

func survivalBossIntroEnsureAssets() {
	survivalBossIntroOnce.Do(func() {
		survivalBossIntroSharp[0] = readExternalBytes("survival", "boss_intro", "sentinel_sharp.bgra")
		survivalBossIntroSharp[1] = readExternalBytes("survival", "boss_intro", "void_serpent_sharp.bgra")
		survivalBossIntroSharp[2] = readExternalBytes("survival", "boss_intro", "terminus_sharp.bgra")
		survivalBossIntroBlur[0] = readExternalBytes("survival", "boss_intro", "sentinel_blur.bgra")
		survivalBossIntroBlur[1] = readExternalBytes("survival", "boss_intro", "void_serpent_blur.bgra")
		survivalBossIntroBlur[2] = readExternalBytes("survival", "boss_intro", "terminus_blur.bgra")

		if survivalBossIntroFont == 0 {
			survivalBossIntroFont = makeFontForFace("Consolas", 50, 700)
		}
		if audioReady {
			p := externalAsset("audio", "survival_boss_intro.mp3")
			mci("stop survival_boss_intro")
			mci("close survival_boss_intro")
			survivalBossIntroAudioReady = mci(`open "` + p + `" type mpegvideo alias survival_boss_intro`)
		}
	})
}

func survivalBossIntroBuildRGBSplit() {
	idx := survivalBossIntroBoss - 1
	if idx < 0 || idx >= len(survivalBossIntroSharp) {
		survivalBossIntroRed = nil
		survivalBossIntroCyan = nil
		return
	}
	src := survivalBossIntroSharp[idx]
	if len(src) < 1280*720*4 {
		survivalBossIntroRed = nil
		survivalBossIntroCyan = nil
		return
	}
	red := make([]byte, len(src))
	cyan := make([]byte, len(src))
	for i := 0; i+3 < len(src); i += 4 {
		b, g, r, a := src[i], src[i+1], src[i+2], src[i+3]
		red[i+2], red[i+3] = r, a
		cyan[i], cyan[i+1], cyan[i+3] = b, g, a
	}
	survivalBossIntroRed = red
	survivalBossIntroCyan = cyan
}

func survivalBossIntroTitle() string {
	switch survivalBossIntroBoss {
	case 1:
		return "THE SENTINEL"
	case 2:
		return "THE VOID SERPENT"
	case 3:
		return "THE TERMINUS 1337"
	default:
		return "UNKNOWN CONTACT"
	}
}

func survivalBossIntroWaveAlias() string {
	return survivalMusicAliasForWave(survivalWave)
}

func survivalBossIntroTargetVolume() int {
	v := gameMeta.MusicVolume * 10
	if v < 0 {
		return 0
	}
	if v > 1000 {
		return 1000
	}
	return v
}

func survivalBossIntroPlayAudio() {
	if !audioReady {
		return
	}
	if !survivalBossIntroAudioReady {
		// The first attempt can occur before the global audio bootstrap has fully
		// settled. Re-open once here rather than failing the cinematic silently.
		p := externalAsset("audio", "survival_boss_intro.mp3")
		mci("stop survival_boss_intro")
		mci("close survival_boss_intro")
		survivalBossIntroAudioReady = mci(`open "` + p + `" type mpegvideo alias survival_boss_intro`)
	}
	if !survivalBossIntroAudioReady {
		return
	}
	mci("stop survival_boss_intro")
	mci("seek survival_boss_intro to start")
	mci(fmt.Sprintf("setaudio survival_boss_intro volume to %d", survivalBossIntroTargetVolume()))
	mci("play survival_boss_intro")
}

func survivalBossIntroStopAudio() {
	if survivalBossIntroAudioReady {
		mci("stop survival_boss_intro")
		mci("seek survival_boss_intro to start")
	}
}

func beginSurvivalBossDossier(h uintptr, boss int, now time.Time) {
	if boss < 1 || boss > 3 || survivalBossIntroActive() || survivalBoss1Active() || survivalBoss2Active() || survivalBoss3Active() {
		return
	}
	survivalBossIntroEnsureAssets()
	survivalBossIntroBoss = boss
	survivalBossIntroStage = survivalBossIntroWaveFade
	survivalBossIntroAt = now
	survivalBossIntroBuildRGBSplit()

	// Lock the cleared battlefield and remove transient targets before fading it.
	survivalEnemies = nil
	survivalPickup = nil
	survivalPickup2 = nil
	survivalEliminations = nil
	survivalArcFX = nil
	survivalWaveBreakUntil = time.Time{}
	survivalNextSpawn = time.Time{}
	survivalNextPickupAt = time.Time{}
	survivalWaveSpawned = survivalWaveBudget // permanently close the cleared normal-wave spawn budget
	resetSurvivalSector3FieldEvent()

	alias := survivalBossIntroWaveAlias()
	if alias != "" {
		survivalBoss1FadeAlias(alias, survivalBoss1TargetMusicVolume(), 0, survivalBossIntroWaveFadeDur, false)
	}
	survivalWaveBannerText = ""
	survivalWaveBannerUntil = time.Time{}
	status = "BOSS SIGNAL // CLASSIFIED DOSSIER"
	if h != 0 {
		invalidateRect.Call(h, 0, 0)
	}
}

func survivalBossIntroStartBossBehindBlack(h uintptr, now time.Time) {
	// Ensure the normal music is genuinely silent before the boss owner takes over.
	mci("stop survival_music")
	mci("stop survival_section2")
	mci("stop survival_section3")
	survivalBossIntroStopAudio()

	survivalBossIntroHandoff = true
	switch survivalBossIntroBoss {
	case 1:
		beginSurvivalBoss1Transition(h, now)
	case 2:
		beginSurvivalBoss2(h, now)
	case 3:
		beginSurvivalBoss3(h, now)
	}
	survivalBossIntroHandoff = false
	// All three bosses currently use the dedicated Survival boss music owner.
	survivalBoss1StartMusic()
}

func survivalBossIntroReleaseCombat(now time.Time, w, h int32) {
	switch survivalBossIntroBoss {
	case 1:
		// The dossier replaces the legacy warning/appearance delay. Start the accelerated
		// combat clock only now, so cinematic time never consumes a fight window.
		survivalBoss1StartedAt = now
		survivalBoss1FightAt = now
		survivalBoss1BeginPhase(survivalBoss1Phase1, now)
		survivalWaveBannerText = "TARGET WEAK POINTS"
		survivalWaveBannerUntil = now.Add(1200 * time.Millisecond)
	case 2:
		survivalBoss2StartedAt = now
		survivalBoss2LastUpdate = now
		survivalBoss2SetStage(survivalBoss2Hunt, now)
		survivalBoss2ComboAwaitEntry = true
		survivalBoss2ComboEntryAt = time.Time{}
		playSurvivalBoss2SerpentStart()
		status = "VOID SERPENT // ENGAGED"
	case 3:
		survivalBoss3StartedAt = now
		survivalBoss3LastUpdate = now
		survivalBoss3ShipLastTick = now
		survivalBoss3SetStage(survivalBoss3Combat, now)
		terminusBeginCombatLoop(now, w, h)
		survivalBoss3MistakeGraceUntil = now.Add(600 * time.Millisecond)
		status = "TERMINUS 1337 // ENGAGED"
	}
}

func updateSurvivalBossIntro(h uintptr, now time.Time, w, hgt int32) bool {
	if !survivalBossIntroActive() {
		return false
	}
	age := now.Sub(survivalBossIntroAt)
	switch survivalBossIntroStage {
	case survivalBossIntroWaveFade:
		if age >= survivalBossIntroWaveFadeDur {
			alias := survivalBossIntroWaveAlias()
			if alias != "" {
				mci("stop " + alias)
				mci("seek " + alias + " to start")
			}
			survivalBossIntroStage = survivalBossIntroPoster
			survivalBossIntroAt = now
			survivalBossIntroPlayAudio()
		}
	case survivalBossIntroPoster:
		if age >= survivalBossIntroPosterDur {
			survivalBossIntroStage = survivalBossIntroBossFadeIn
			survivalBossIntroAt = now
			survivalBossIntroStartBossBehindBlack(h, now)
		}
	case survivalBossIntroBossFadeIn:
		if age >= survivalBossIntroFightFade {
			survivalBossIntroReleaseCombat(now, w, hgt)
			survivalBossIntroStage = survivalBossIntroNone
			survivalBossIntroBoss = 0
			survivalBossIntroAt = time.Time{}
			survivalBossIntroRed = nil
			survivalBossIntroCyan = nil
		}
	}
	if h != 0 {
		invalidateRect.Call(h, 0, 0)
	}
	return true
}

func drawRawBGRAContainAlpha(hdc uintptr, data []byte, srcW, srcH int32, box RECT, alpha byte, dx, dy int32) {
	if alpha == 0 || srcW <= 0 || srcH <= 0 || box.Right <= box.Left || box.Bottom <= box.Top || len(data) < int(srcW*srcH*4) {
		return
	}
	sprite := ensureRuntimeSprite(hdc, data, srcW, srcH)
	if sprite == nil || sprite.dc == 0 {
		return
	}
	boxW := box.Right - box.Left
	boxH := box.Bottom - box.Top
	scale := math.Min(float64(boxW)/float64(srcW), float64(boxH)/float64(srcH))
	dstW := int32(math.Round(float64(srcW) * scale))
	dstH := int32(math.Round(float64(srcH) * scale))
	if dstW < 1 || dstH < 1 {
		return
	}
	dstX := box.Left + (boxW-dstW)/2 + dx
	dstY := box.Top + (boxH-dstH)/2 + dy
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		sprite.dc, 0, 0, uintptr(srcW), uintptr(srcH), blend)
}

func survivalBossIntroGlitchStrength(t float64) float64 {
	// Short, authored corruption bursts: mostly stable imagery punctuated by hard
	// interference, matching the supplied military-area reference better than
	// continuous random noise.
	bursts := [][3]float64{
		{.03, .13, .95}, {.39, .48, .62}, {.91, 1.03, .72}, {1.62, 1.72, .46},
		{2.34, 2.43, .58}, {3.18, 3.25, .38}, {4.12, 4.26, .72}, {4.43, 4.72, 1.0},
	}
	best := 0.0
	for _, b := range bursts {
		if t < b[0] || t > b[1] {
			continue
		}
		p := (t - b[0]) / (b[1] - b[0])
		s := math.Sin(math.Pi*p) * b[2]
		if s > best {
			best = s
		}
	}
	return best
}

func survivalBossIntroDrawTears(hdc uintptr, w, h int32, t, strength float64) {
	if strength <= .05 {
		return
	}
	// Shift narrow horizontal bands on the already-rendered frame. Coordinates are
	// deterministic from time so the effect is violent without being visually noisy.
	for i := 0; i < 7; i++ {
		phase := t*31.0 + float64(i)*2.73 + float64(survivalBossIntroBoss)*.91
		yf := .10 + math.Mod(math.Abs(math.Sin(phase*1.37))*1.7+float64(i)*.111, .80)
		y := int32(float64(h) * yf)
		band := int32(math.Max(2, float64(h)*(.006+.012*math.Abs(math.Sin(phase*2.11)))))
		off := int32(math.Round(math.Sin(phase*3.9) * float64(w) * .018 * strength))
		if off == 0 {
			continue
		}
		if off > 0 {
			bitBlt.Call(hdc, uintptr(off), uintptr(y), uintptr(w-off), uintptr(band), hdc, 0, uintptr(y), SRCCOPY)
		} else {
			shift := -off
			bitBlt.Call(hdc, 0, uintptr(y), uintptr(w-shift), uintptr(band), hdc, uintptr(shift), uintptr(y), SRCCOPY)
		}
	}
}

func survivalBossIntroDrawScan(hdc uintptr, w, h int32, t float64) {
	// Sparse CRT scan bands and one brighter travelling scanner line.
	for y := int32(0); y < h; y += max32(4, h/180) {
		alphaSolidRect(hdc, RECT{0, y, w, y + 1}, rgb(0, 0, 0), 24)
	}
	scanY := int32(math.Mod(t*138.0, float64(h+80))) - 40
	if scanY >= 0 && scanY < h {
		alphaSolidRect(hdc, RECT{0, scanY, w, min32(h, scanY+2)}, rgb(215, 232, 240), 36)
		alphaSolidRect(hdc, RECT{0, max32(0, scanY-7), w, min32(h, scanY+9)}, rgb(185, 210, 225), 10)
	}
}

func survivalBossIntroDrawVignette(hdc uintptr, w, h int32) {
	for i := int32(0); i < 8; i++ {
		a := byte(12 + i*6)
		th := max32(2, h/70)
		top := i * th
		alphaSolidRect(hdc, RECT{0, top, w, top + th}, rgb(0, 0, 0), a)
		alphaSolidRect(hdc, RECT{0, h - top - th, w, h - top}, rgb(0, 0, 0), a)
		vw := max32(2, w/100)
		left := i * vw
		alphaSolidRect(hdc, RECT{left, 0, left + vw, h}, rgb(0, 0, 0), a)
		alphaSolidRect(hdc, RECT{w - left - vw, 0, w - left, h}, rgb(0, 0, 0), a)
	}
}

func survivalBossIntroTypedTitle(t float64) (string, bool) {
	full := survivalBossIntroTitle()
	const typingStart = .72
	const charPeriod = .068
	if t < typingStart {
		return "", false
	}
	n := int((t-typingStart)/charPeriod) + 1
	if n < 0 {
		n = 0
	}
	if n > len(full) {
		n = len(full)
	}
	text := full[:n]
	cursor := math.Mod(t*2.7, 1.0) < .68
	return text, cursor
}

func survivalBossIntroDrawTitle(hdc uintptr, w, h int32, t, glitch float64, overallAlpha byte) {
	text, cursor := survivalBossIntroTypedTitle(t)
	if text == "" || overallAlpha == 0 {
		return
	}
	if cursor {
		text += "_"
	}
	font := survivalBossIntroFont
	if font == 0 {
		font = hudStatFont
	}
	old, _, _ := selectObject.Call(hdc, font)
	defer selectObject.Call(hdc, old)
	setBkMode.Call(hdc, TRANSPARENT)
	cy := h/2 - sy(20, h)
	bandH := sy(88, h)
	alphaSolidRect(hdc, RECT{0, cy - bandH/2, w, cy + bandH/2}, rgb(0, 0, 0), byte(float64(overallAlpha)*.38))

	// During hard corruption bursts, the title itself tears into cyan/red ghosts.
	// Vertically centre the terminal title inside the cinematic black strip.
	// TextOut treats Y as the glyph top, so use a higher top edge than the old
	// baseline to keep all three boss names optically centred in the strip.
	titleY := cy - sy(36, h)
	if glitch > .12 {
		off := int32(math.Round(float64(sx(4, w)) * glitch))
		setTextColor.Call(hdc, rgb(0, 205, 255))
		centeredTextOut(hdc, off, w+off, titleY, text)
		setTextColor.Call(hdc, rgb(255, 45, 38))
		centeredTextOut(hdc, -off, w-off, titleY, text)
	}
	setTextColor.Call(hdc, rgb(245, 247, 248))
	centeredTextOut(hdc, 0, w, titleY, text)
}

func drawSurvivalBossIntroOverlay(hdc uintptr, w, h int32) {
	if !survivalBossIntroActive() {
		return
	}
	r := RECT{0, 0, w, h}
	t := time.Since(survivalBossIntroAt).Seconds()
	switch survivalBossIntroStage {
	case survivalBossIntroWaveFade:
		p := math.Min(1, t/survivalBossIntroWaveFadeDur.Seconds())
		// Ease-in keeps the first half readable, then rapidly closes to black.
		p = p * p * (3 - 2*p)
		alphaSolidRect(hdc, r, rgb(0, 0, 0), byte(p*255))
		return
	case survivalBossIntroBossFadeIn:
		p := math.Min(1, t/survivalBossIntroFightFade.Seconds())
		p = p * p * (3 - 2*p)
		alphaSolidRect(hdc, r, rgb(0, 0, 0), byte((1-p)*255))
		return
	case survivalBossIntroPoster:
		// Preserve the supplied dossier artwork exactly. The game window is taller
		// than 16:9, so use cinematic black bars instead of stretching/cropping it.
		alphaSolidRect(hdc, r, rgb(0, 0, 0), 255)
		idx := survivalBossIntroBoss - 1
		if idx < 0 || idx >= 3 {
			alphaSolidRect(hdc, r, rgb(0, 0, 0), 255)
			return
		}
		fadeIn := math.Min(1, t/.34)
		fadeOut := 1.0
		if t > 4.48 {
			fadeOut = math.Max(0, (5.0-t)/.52)
		}
		overall := math.Max(0, math.Min(1, fadeIn*fadeOut))
		alpha := byte(overall * 255)
		focus := math.Max(0, math.Min(1, (t-.08)/.58))
		blurAlpha := byte(float64(alpha) * (1 - focus))
		sharpAlpha := byte(float64(alpha) * focus)
		if blurAlpha > 0 {
			drawRawBGRAContainAlpha(hdc, survivalBossIntroBlur[idx], 1280, 720, r, blurAlpha, 0, 0)
		}
		if sharpAlpha > 0 {
			drawRawBGRAContainAlpha(hdc, survivalBossIntroSharp[idx], 1280, 720, r, sharpAlpha, 0, 0)
		}

		glitch := survivalBossIntroGlitchStrength(t)
		if glitch > .04 {
			off := int32(math.Round(float64(sx(7, w)) * glitch))
			ghostAlpha := byte(math.Min(120, 42+glitch*72) * overall)
			if len(survivalBossIntroCyan) > 0 {
				drawRawBGRAContainAlpha(hdc, survivalBossIntroCyan, 1280, 720, r, ghostAlpha, -off, 0)
			}
			if len(survivalBossIntroRed) > 0 {
				drawRawBGRAContainAlpha(hdc, survivalBossIntroRed, 1280, 720, r, ghostAlpha, off, 0)
			}
			survivalBossIntroDrawTears(hdc, w, h, t, glitch)
			// Brief white sync flashes at the peak of a corruption burst.
			if glitch > .78 {
				flash := byte(math.Min(80, (glitch-.78)*260) * overall)
				alphaSolidRect(hdc, r, rgb(235, 242, 246), flash)
			}
		}
		survivalBossIntroDrawScan(hdc, w, h, t)
		survivalBossIntroDrawVignette(hdc, w, h)
		survivalBossIntroDrawTitle(hdc, w, h, t, glitch, alpha)
		if overall < 1 {
			alphaSolidRect(hdc, r, rgb(0, 0, 0), byte((1-overall)*255))
		}
	}
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
