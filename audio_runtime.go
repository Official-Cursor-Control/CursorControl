//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// v306 AUDIO REWRITE
// ------------------
// Music and sound effects are deliberately separated:
//   * MUSIC BUS: MCI/mpegvideo, used only for long looping MP3 music tracks.
//   * SFX BUS:   waveOut PCM playback, used only for WAV effects.
//
// This removes the old fragmented architecture where many short effects shared
// MCI aliases with MP3 music. Each one-shot opens an independent waveOut handle,
// so music can never steal/overwrite the SFX playback path and multiple effects
// may overlap naturally through the Windows mixer.

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

type waveHeader struct {
	Data          *byte
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          *waveHeader
	Reserved      uintptr
}

const (
	waveMapper       = 0xffffffff
	callbackNull     = 0
	whdrDone         = 0x00000001
	womDone          = 0x3bd
	maxConcurrentSFX = 40
)

var (
	waveOutOpenProc      = winmm.NewProc("waveOutOpen")
	waveOutCloseProc     = winmm.NewProc("waveOutClose")
	waveOutPrepareProc   = winmm.NewProc("waveOutPrepareHeader")
	waveOutUnprepareProc = winmm.NewProc("waveOutUnprepareHeader")
	waveOutWriteProc     = winmm.NewProc("waveOutWrite")
	waveOutResetProc     = winmm.NewProc("waveOutReset")
	waveOutSetVolumeProc = winmm.NewProc("waveOutSetVolume")
)

type pcmEffect struct {
	Name     string
	Path     string
	Format   waveFormatEx
	Data     []byte
	Duration time.Duration
}

var sfxBus = struct {
	sync.RWMutex
	effects     map[string]*pcmEffect
	sem         chan struct{}
	criticalSem chan struct{}
	loops       map[string]*sfxLoop
	active      map[string]int
}{
	effects:     make(map[string]*pcmEffect),
	sem:         make(chan struct{}, maxConcurrentSFX-8),
	criticalSem: make(chan struct{}, 8),
	loops:       make(map[string]*sfxLoop),
	active:      make(map[string]int),
}

type sfxLoop struct {
	stop chan struct{}
	once sync.Once
}

func loadPCMEffect(name, path string) bool {
	if name == "" || path == "" {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	fx, err := parsePCM16Wave(name, path, raw)
	if err != nil {
		return false
	}
	sfxBus.Lock()
	sfxBus.effects[name] = fx
	sfxBus.Unlock()
	return true
}

func parsePCM16Wave(name, path string, raw []byte) (*pcmEffect, error) {
	if len(raw) < 44 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return nil, fmt.Errorf("invalid wav: %s", filepath.Base(path))
	}
	var fmtChunk []byte
	var dataChunk []byte
	for off := 12; off+8 <= len(raw); {
		id := string(raw[off : off+4])
		n := int(binary.LittleEndian.Uint32(raw[off+4 : off+8]))
		start := off + 8
		end := start + n
		if n < 0 || end > len(raw) {
			break
		}
		switch id {
		case "fmt ":
			fmtChunk = raw[start:end]
		case "data":
			dataChunk = raw[start:end]
		}
		off = end
		if off&1 != 0 {
			off++
		}
	}
	if len(fmtChunk) < 16 || len(dataChunk) == 0 {
		return nil, fmt.Errorf("missing fmt/data chunk: %s", filepath.Base(path))
	}
	formatTag := binary.LittleEndian.Uint16(fmtChunk[0:2])
	channels := binary.LittleEndian.Uint16(fmtChunk[2:4])
	sampleRate := binary.LittleEndian.Uint32(fmtChunk[4:8])
	avgBytes := binary.LittleEndian.Uint32(fmtChunk[8:12])
	blockAlign := binary.LittleEndian.Uint16(fmtChunk[12:14])
	bits := binary.LittleEndian.Uint16(fmtChunk[14:16])
	if formatTag != 1 || channels == 0 || sampleRate == 0 || avgBytes == 0 || bits != 16 {
		return nil, fmt.Errorf("unsupported wav format: %s", filepath.Base(path))
	}
	// Copy only PCM payload so the backing array stays owned by the effect cache.
	pcm := append([]byte(nil), dataChunk...)
	dur := time.Duration(float64(len(pcm)) / float64(avgBytes) * float64(time.Second))
	return &pcmEffect{
		Name: name,
		Path: path,
		Format: waveFormatEx{
			FormatTag:      formatTag,
			Channels:       channels,
			SamplesPerSec:  sampleRate,
			AvgBytesPerSec: avgBytes,
			BlockAlign:     blockAlign,
			BitsPerSample:  bits,
		},
		Data:     pcm,
		Duration: dur,
	}, nil
}

func sfxLoaded(name string) bool {
	sfxBus.RLock()
	_, ok := sfxBus.effects[name]
	sfxBus.RUnlock()
	return ok
}

func sfxPriority(name string) int {
	// 3 = gameplay-critical cue, 2 = progression/reward, 1 = action feedback.
	// Critical sounds have reserved voices so rapid hit spam cannot steal them.
	switch name {
	case "buzzer", "endurance_fail", "warp_ready", "alien_charge", "alien_impact",
		"boss1_roar", "boss2_serpent_start", "boss2_serpent_attack_1", "boss2_serpent_attack_2",
		"boss2_serpent_attack_3", "boss2_meteor_rumble", "boss2_energy_ball":
		return 3
	case "levelup", "space_cache", "garage_buy", "afk_tier_up", "afk_prestige", "afk_orbital_fire":
		return 2
	default:
		return 1
	}
}

func sfxVoiceLimit(name string) int {
	switch name {
	case "boss_click_effect":
		return 4
	case "survival_hit", "hit", "endurance_explode":
		return 7
	case "ui_button_click":
		return 3
	case "afk_starbit_collect", "afk_drone_mining":
		return 2
	default:
		return 10
	}
}

var criticalAudioDuckUntil atomic.Int64
var operatorVoiceDuckUntil atomic.Int64

func criticalAudioDuckActive() bool {
	return time.Now().UnixMilli() < criticalAudioDuckUntil.Load()
}

func operatorVoiceDuckActive() bool {
	return time.Now().UnixMilli() < operatorVoiceDuckUntil.Load()
}

// Operator quips are deliberately given a much stronger, longer music duck than
// normal combat SFX. The duck begins before the voice starts and releases only
// after the line/celebration slot has ended, so the spoken quip is always clear.
func requestOperatorVoiceDuck(duration time.Duration) {
	if duration < 1200*time.Millisecond {
		duration = 1200 * time.Millisecond
	}
	until := time.Now().Add(duration).UnixMilli()
	for {
		old := operatorVoiceDuckUntil.Load()
		if old >= until || operatorVoiceDuckUntil.CompareAndSwap(old, until) {
			break
		}
	}
	applyAudioVolumes()
	go func(target int64, wait time.Duration) {
		time.Sleep(wait + 90*time.Millisecond)
		if operatorVoiceDuckUntil.Load() <= target {
			applyAudioVolumes()
		}
	}(until, duration)
}

func requestCriticalAudioDuck() {
	until := time.Now().Add(850 * time.Millisecond).UnixMilli()
	for {
		old := criticalAudioDuckUntil.Load()
		if old >= until || criticalAudioDuckUntil.CompareAndSwap(old, until) {
			break
		}
	}
	go func(target int64) {
		applyAudioVolumes()
		time.Sleep(900 * time.Millisecond)
		if criticalAudioDuckUntil.Load() <= target {
			applyAudioVolumes()
		}
	}(until)
}

func effectVolumeWord(name string) uintptr {
	v := gameMeta.EffectsVolume
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	gain := 1.0
	switch name {
	case "endurance_explode":
		gain = 0.50
	case "alien_charge":
		gain = 0.35
	case "shield_protect":
		gain = 0.10
	case "survival_1hp_siren":
		gain = 0.14
	case "boss2_final_roar_1":
		gain = 0.50
	case "boss2_final_roar_2":
		gain = 0.75
	case "boss2_final_roar_3":
		gain = 1.00
	case "afk_starbit_collect":
		gain = 0.30
	case "afk_drone_mining":
		gain = 0.16
	case "afk_orbital_charge":
		gain = 0.55
	case "afk_orbital_fire":
		gain = 0.85
	case "afk_expedition_launch":
		gain = 0.65
	}
	level := uint32(float64((uint64(v)*0xffff)/100) * gain)
	if level > 0xffff {
		level = 0xffff
	}
	return uintptr(level | (level << 16))
}

func playSFX(name string) {
	if !audioReady || name == "" || gameMeta.EffectsVolume <= 0 {
		return
	}
	sfxBus.RLock()
	fx := sfxBus.effects[name]
	active := sfxBus.active[name]
	sfxBus.RUnlock()
	if fx == nil || active >= sfxVoiceLimit(name) {
		return
	}

	priority := sfxPriority(name)
	sem := sfxBus.sem
	if priority >= 3 {
		sem = sfxBus.criticalSem
		requestCriticalAudioDuck()
	}
	select {
	case sem <- struct{}{}:
		sfxBus.Lock()
		// Recheck under lock so two simultaneous callers cannot exceed the cap.
		if sfxBus.active[name] >= sfxVoiceLimit(name) {
			sfxBus.Unlock()
			<-sem
			return
		}
		sfxBus.active[name]++
		sfxBus.Unlock()
		go func() {
			defer func() {
				sfxBus.Lock()
				if sfxBus.active[name] > 0 {
					sfxBus.active[name]--
				}
				sfxBus.Unlock()
				<-sem
			}()
			_ = playPCMBlocking(fx, nil)
		}()
	default:
		// Never stall gameplay because the user generated an extreme SFX burst.
	}
}

func playPCMBlocking(fx *pcmEffect, stop <-chan struct{}) bool {
	if fx == nil || len(fx.Data) == 0 {
		return false
	}
	var handle uintptr
	r, _, _ := waveOutOpenProc.Call(
		uintptr(unsafe.Pointer(&handle)),
		uintptr(waveMapper),
		uintptr(unsafe.Pointer(&fx.Format)),
		0,
		0,
		callbackNull,
	)
	if r != 0 || handle == 0 {
		return false
	}
	defer waveOutCloseProc.Call(handle)
	waveOutSetVolumeProc.Call(handle, effectVolumeWord(fx.Name))

	hdr := waveHeader{
		Data:         &fx.Data[0],
		BufferLength: uint32(len(fx.Data)),
	}
	if r, _, _ = waveOutPrepareProc.Call(handle, uintptr(unsafe.Pointer(&hdr)), unsafe.Sizeof(hdr)); r != 0 {
		return false
	}
	defer waveOutUnprepareProc.Call(handle, uintptr(unsafe.Pointer(&hdr)), unsafe.Sizeof(hdr))
	if r, _, _ = waveOutWriteProc.Call(handle, uintptr(unsafe.Pointer(&hdr)), unsafe.Sizeof(hdr)); r != 0 {
		return false
	}

	tick := time.NewTicker(8 * time.Millisecond)
	defer tick.Stop()
	deadline := time.NewTimer(fx.Duration + 2*time.Second)
	defer deadline.Stop()
	for {
		if atomic.LoadUint32(&hdr.Flags)&whdrDone != 0 {
			return true
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			waveOutResetProc.Call(handle)
			return true
		case <-stop:
			waveOutResetProc.Call(handle)
			return true
		}
	}
}

func startLoopSFX(name string) {
	if !audioReady || name == "" || gameMeta.EffectsVolume <= 0 {
		return
	}
	sfxBus.RLock()
	fx := sfxBus.effects[name]
	sfxBus.RUnlock()
	if fx == nil {
		return
	}

	sfxBus.Lock()
	if _, exists := sfxBus.loops[name]; exists {
		sfxBus.Unlock()
		return
	}
	loop := &sfxLoop{stop: make(chan struct{})}
	sfxBus.loops[name] = loop
	sfxBus.Unlock()

	go func() {
		defer func() {
			sfxBus.Lock()
			if sfxBus.loops[name] == loop {
				delete(sfxBus.loops, name)
			}
			sfxBus.Unlock()
		}()
		for {
			select {
			case <-loop.stop:
				return
			default:
			}
			if !playPCMBlocking(fx, loop.stop) {
				return
			}
		}
	}()
}

func stopLoopSFX(name string) {
	sfxBus.Lock()
	loop := sfxBus.loops[name]
	if loop != nil {
		delete(sfxBus.loops, name)
	}
	sfxBus.Unlock()
	if loop != nil {
		loop.once.Do(func() { close(loop.stop) })
	}
}

func stopAllSFXLoops() {
	sfxBus.Lock()
	loops := make([]*sfxLoop, 0, len(sfxBus.loops))
	for _, loop := range sfxBus.loops {
		loops = append(loops, loop)
	}
	sfxBus.loops = make(map[string]*sfxLoop)
	sfxBus.Unlock()
	for _, loop := range loops {
		loop.once.Do(func() { close(loop.stop) })
	}
}

// Kept as the single compatibility entry point used by existing gameplay code.
// All callers now route into the centralized SFX bus instead of MCI aliases.
func playOneShotAsync(alias string) {
	playSFX(alias)
}

func initSFXBus(audioAssets string) {
	// Exact audio-role registry preserved from the approved v305 map.
	registry := map[string]string{
		"buzzer":                    "buzzer_loud.wav",
		"buzz_fortnite":             "Fortnite.wav",
		"buzz_roblox":               "Roblox.wav",
		"buzz_minecraft":            "Minecraft.wav",
		"buzz_amongus":              "Among Us.wav",
		"hit":                       "hit.wav",
		"levelup":                   "levelup.wav",
		"rechamber":                 "rechamber.wav",
		"konggames_intro":           "konggames_intro.wav",
		"warp_ready":                "ready_go.wav",
		"warp_rocket":               "rocket_sound.wav",
		"endurance_explode":         "meteorite_explode.wav",
		"endurance_fail":            "endurance_fail.wav",
		"space_cache":               "space_cache_open.wav",
		"alien_charge":              "alien_charge_up.wav",
		"alien_impact":              "alien_impact_laser.wav",
		"shield_protect":            "shield_protect.wav",
		"powerup_pickup":            "powerup_pickup.wav",
		"ui_button_click":           "button_click.wav",
		"garage_buy":                "buy.wav",
		"survival_hit":              "survival_hit.wav",
		"survival_damage_taken":     "damage_taken.wav",
		"survival_1hp_siren":        "1_hp_siren.wav",
		"boss_click_effect":         "boss_click_effect.wav",
		"boss1_roar":                "boss_1_roar.wav",
		"boss2_serpent_start":       "serpent_roar_start.wav",
		"boss2_serpent_attack_1":    "serpent_attack_1.wav",
		"boss2_serpent_attack_2":    "serpent_attack_2.wav",
		"boss2_serpent_attack_3":    "serpent_attack_3.wav",
		"boss2_final_roar_1":        "serpent_attack_1.wav",
		"boss2_final_roar_2":        "serpent_attack_2.wav",
		"boss2_final_roar_3":        "serpent_attack_3.wav",
		"boss2_meteor_rumble":       "meteor_rumble.wav",
		"boss2_meteor_smash":        "meteor_smash.wav",
		"boss2_energy_ball":         "energy_ball.wav",
		"boss2_hunt_hit_1":          "sboss2_hit_1.wav",
		"boss2_hunt_hit_2":          "sboss2_hit_2.wav",
		"boss2_hunt_hit_3":          "sboss2_hit_3.wav",
		"boss2_hunt_hit_4":          "sboss2_hit_4.wav",
		"boss2_hunt_hit_5":          "sboss2_hit_5.wav",
		"afk_starbit_collect":       "starbase_starbit_collect.wav",
		"afk_construction":          "starbase_construction.wav",
		"afk_tier_up":               "starbase_tier_up.wav",
		"afk_expedition_launch":     "starbase_expedition_launch.wav",
		"afk_research_complete":     "starbase_research_complete.wav",
		"afk_operator_recruited":    "starbase_operator_recruited.wav",
		"afk_operator_nova_quip_1":  "operator_nova_quip_1.wav",
		"afk_operator_nova_quip_2":  "operator_nova_quip_2.wav",
		"afk_operator_nova_quip_3":  "operator_nova_quip_3.wav",
		"afk_operator_vega_quip_1":  "operator_vega_quip_1.wav",
		"afk_operator_vega_quip_2":  "operator_vega_quip_2.wav",
		"afk_operator_vega_quip_3":  "operator_vega_quip_3.wav",
		"afk_operator_byte_quip_1":  "operator_byte_quip_1.wav",
		"afk_operator_byte_quip_2":  "operator_byte_quip_2.wav",
		"afk_operator_byte_quip_3":  "operator_byte_quip_3.wav",
		"afk_operator_astra_quip_1": "operator_astra_quip_1.wav",
		"afk_operator_astra_quip_2": "operator_astra_quip_2.wav",
		"afk_operator_astra_quip_3": "operator_astra_quip_3.wav",
		"afk_operator_flux_quip_1":  "operator_flux_quip_1.wav",
		"afk_operator_flux_quip_2":  "operator_flux_quip_2.wav",
		"afk_operator_flux_quip_3":  "operator_flux_quip_3.wav",
		"afk_module_crafted":        "starbase_module_crafted.wav",
		"afk_drone_deploy":          "starbase_drone_deploy.wav",
		"afk_drone_mining":          "starbase_drone_mining.wav",
		"afk_overdrive":             "starbase_overdrive.wav",
		"afk_orbital_charge":        "starbase_orbital_charge.wav",
		"afk_orbital_fire":          "starbase_orbital_fire.wav",
		"afk_prestige":              "starbase_prestige.wav",
		"afk_upgrade":               "starbase_upgrade.wav",
		"afk_collect_item":          "starbase_collect_item.wav",
	}
	for name, file := range registry {
		loadPCMEffect(name, filepath.Join(audioAssets, file))
	}

	hitAudioReady = sfxLoaded("hit")
	levelAudioReady = sfxLoaded("levelup")
	rechamberAudioReady = sfxLoaded("rechamber")
	warpCueAudioReady = sfxLoaded("warp_ready")
	warpRocketAudioReady = sfxLoaded("warp_rocket")
	enduranceExplodeAudioReady = sfxLoaded("endurance_explode")
	enduranceFailAudioReady = sfxLoaded("endurance_fail")
	spaceCacheAudioReady = sfxLoaded("space_cache")
	alienChargeAudioReady = sfxLoaded("alien_charge")
	alienImpactAudioReady = sfxLoaded("alien_impact")
	shieldProtectAudioReady = sfxLoaded("shield_protect")
	powerupPickupAudioReady = sfxLoaded("powerup_pickup")
	buttonClickAudioReady = sfxLoaded("ui_button_click")
	buyAudioReady = sfxLoaded("garage_buy")
	survivalHitAudioReady = sfxLoaded("survival_hit")
	bossClickAudioReady = sfxLoaded("boss_click_effect")
	boss1RoarAudioReady = sfxLoaded("boss1_roar")
}

// Ensure syscall remains referenced on all supported Go versions where the
// Windows build tag is compiled with older syscall internals.
var _ = syscall.Errno(0)
