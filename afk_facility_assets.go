//go:build windows

package main

import (
	"fmt"
)

type afkFacilitySprite struct {
	data []byte
	glow []byte
	w, h int32
}

var afkFacilitySprites = map[string]*afkFacilitySprite{
	"cursor_core_t1":       {w: 255, h: 272},
	"cursor_core_t2":       {w: 269, h: 299},
	"cursor_core_t3":       {w: 273, h: 369},
	"cursor_core_t4":       {w: 303, h: 385},
	"cursor_core_t5":       {w: 364, h: 408},
	"scout_pad_t1":         {w: 228, h: 244},
	"scout_pad_t2":         {w: 286, h: 231},
	"scout_pad_t3":         {w: 323, h: 259},
	"tech_lab":             {w: 263, h: 259},
	"operations":           {w: 259, h: 250},
	"module_fabricator":    {w: 269, h: 225},
	"drone_bay_t1":         {w: 183, h: 197},
	"drone_bay_t2":         {w: 228, h: 233},
	"drone_bay_t3":         {w: 263, h: 267},
	"orbital_extractor_t1": {w: 152, h: 173},
	"orbital_extractor_t2": {w: 169, h: 223},
	"orbital_extractor_t3": {w: 213, h: 296},
	"scout_ship":           {w: 246, h: 149},
	"mining_drone_01":      {w: 122, h: 114},
	"mining_drone_02":      {w: 122, h: 133},
	"mining_drone_03":      {w: 130, h: 119},
	"mining_drone_04":      {w: 136, h: 146},
	"cargo_crate_01":       {w: 109, h: 69},
	"cargo_crate_02":       {w: 119, h: 79},
	"cargo_crate_03":       {w: 86, h: 69},
	"communications_dish":  {w: 111, h: 167},
	"solar_panel":          {w: 107, h: 109},
	"starbase_flag":        {w: 105, h: 165},
	"energy_canister":      {w: 66, h: 113},
	"heavy_cargo_crate":    {w: 128, h: 111},
}

func loadAFKFacilityAssets() {
	for name, s := range afkFacilitySprites {
		if s == nil {
			continue
		}
		s.data = readExternalBytes("ui", "starbase_facilities", name+".bgra")
		if len(s.data) >= int(s.w*s.h*4) {
			s.glow = makeAFKFacilityGlowBGRA(s.data, s.w, s.h)
		}
	}
}

func makeAFKFacilityGlowBGRA(src []byte, w, h int32) []byte {
	if len(src) < int(w*h*4) {
		return nil
	}
	out := make([]byte, len(src))
	// Keep only the sprite silhouette alpha. RGB is a light Starbase cyan;
	// rendering expanded/offset copies gives an irregular edge halo instead of
	// a rectangular frame around the logical hit box.
	for i := int32(0); i < w*h; i++ {
		o := int(i * 4)
		a := src[o+3]
		if a < 8 {
			continue
		}
		// Premultiplied BGRA for AlphaBlend.
		ga := uint16(a) * 150 / 255
		out[o+3] = byte(ga)
		out[o+0] = byte(uint16(255) * ga / 255)
		out[o+1] = byte(uint16(218) * ga / 255)
		out[o+2] = byte(uint16(83) * ga / 255)
	}
	return out
}

func drawAFKFacilityGlowSpriteGrounded(hdc uintptr, name string, box RECT, alpha byte) bool {
	s := afkFacilitySprites[name]
	if s == nil || len(s.glow) < int(s.w*s.h*4) || box.Right <= box.Left || box.Bottom <= box.Top {
		return false
	}
	sprite := ensureRuntimeSprite(hdc, s.glow, s.w, s.h)
	if sprite == nil || sprite.dc == 0 {
		return false
	}
	crop := alphaBoundsBGRA(s.data, s.w, s.h, 5)
	cropW, cropH := crop.Right-crop.Left, crop.Bottom-crop.Top
	if cropW <= 0 || cropH <= 0 {
		return false
	}
	boxW, boxH := box.Right-box.Left, box.Bottom-box.Top
	dstW := boxW
	dstH := int32(float64(dstW) * float64(cropH) / float64(cropW))
	if dstH > boxH {
		dstH = boxH
		dstW = int32(float64(dstH) * float64(cropW) / float64(cropH))
	}
	dstX := box.Left + (boxW-dstW)/2
	dstY := box.Bottom - dstH
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		sprite.dc, uintptr(crop.Left), uintptr(crop.Top), uintptr(cropW), uintptr(cropH), blend)
	return true
}

func drawAFKFacilitySprite(hdc uintptr, name string, box RECT) bool {
	s := afkFacilitySprites[name]
	if s == nil || len(s.data) < int(s.w*s.h*4) {
		return false
	}
	drawRawBGRATrimmedFit(hdc, s.data, s.w, s.h, box)
	return true
}

func drawAFKFacilitySpriteGrounded(hdc uintptr, name string, box RECT) bool {
	s := afkFacilitySprites[name]
	if s == nil || len(s.data) < int(s.w*s.h*4) || box.Right <= box.Left || box.Bottom <= box.Top {
		return false
	}
	sprite := ensureRuntimeSprite(hdc, s.data, s.w, s.h)
	if sprite == nil || sprite.dc == 0 {
		return false
	}
	crop := alphaBoundsBGRA(s.data, s.w, s.h, 5)
	cropW, cropH := crop.Right-crop.Left, crop.Bottom-crop.Top
	if cropW <= 0 || cropH <= 0 {
		return false
	}
	boxW, boxH := box.Right-box.Left, box.Bottom-box.Top
	dstW := boxW
	dstH := int32(float64(dstW) * float64(cropH) / float64(cropW))
	if dstH > boxH {
		dstH = boxH
		dstW = int32(float64(dstH) * float64(cropW) / float64(cropH))
	}
	dstX := box.Left + (boxW-dstW)/2
	// Grounded sprites always sit on the lunar surface instead of being
	// vertically centred inside their logical hit box.
	dstY := box.Bottom - dstH
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		sprite.dc, uintptr(crop.Left), uintptr(crop.Top), uintptr(cropW), uintptr(cropH), blend)
	return true
}

func afkFacilityAssetStatus() string {
	loaded := 0
	for _, s := range afkFacilitySprites {
		if s != nil && len(s.data) >= int(s.w*s.h*4) {
			loaded++
		}
	}
	return fmt.Sprintf("STARBASE FACILITIES %d/%d", loaded, len(afkFacilitySprites))
}

// drawAFKFacilityDecor places the supplied support sprites into the physical
// Starbase without turning them into extra UI controls. They are decorative
// world feedback tied to systems the player has actually unlocked.
func afkDecorJitter(index int, span int32) int32 {
	if span <= 0 {
		return 0
	}
	// Stable for this launch, different on the next launch. This gives the moon
	// strip a naturally scattered settlement layout without props wandering every
	// frame or overlapping facilities unpredictably.
	x := uint32((gameMeta.Sessions+1)*1664525 + (index+17)*1013904223)
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	width := uint32(span*2 + 1)
	return int32(x%width) - span
}

func afkDecorPoint(field RECT, w, hgt int32, frac float64, index int) (int32, int32) {
	fieldW := field.Right - field.Left
	x := field.Left + int32(float64(fieldW)*frac) + afkDecorJitter(index, sx(10, w))
	// Plant props slightly into the moon strip, with a small per-session height
	// variation so the decoration does not form another ruler-straight line.
	y := afkMoonSurfaceY(w, hgt) + sy(52, hgt) + afkDecorJitter(index+50, sy(6, hgt))
	return x, y
}

// drawAFKFacilityDecor places support props in the natural gaps between the
// seven major facilities. Decorative props are deliberately independent from
// facility hit boxes and never sit directly on top of the Tech Lab (or any
// other facility). Their positions are pseudo-random once per game session.
func drawAFKFacilityDecor(hdc uintptr, w, hgt int32) {
	if !afkPrimaryWorldVisible() {
		return
	}
	field := afkMainFieldRect(w, hgt)

	drawGround := func(name string, frac float64, index int, ww, hh int32) {
		x, y := afkDecorPoint(field, w, hgt, frac, index)
		drawAFKFacilitySpriteGrounded(hdc, name, RECT{x - ww/2, y - hh, x + ww/2, y})
	}

	// Persistent base decoration. Fractions sit in the spaces between the seven
	// facility centres (0.11, .24, .37, .50, .63, .76, .89), so the props make
	// the strip feel inhabited without obscuring the buildings or their labels.
	drawGround("starbase_flag", 0.045, 0, sx(22, w), sy(42, hgt))
	drawGround("communications_dish", 0.175, 1, sx(25, w), sy(43, hgt))
	drawGround("starbase_flag", 0.305, 2, sx(21, w), sy(40, hgt))
	drawGround("communications_dish", 0.565, 3, sx(24, w), sy(42, hgt))
	drawGround("starbase_flag", 0.695, 4, sx(22, w), sy(42, hgt))
	drawGround("communications_dish", 0.825, 5, sx(25, w), sy(44, hgt))
	drawGround("starbase_flag", 0.955, 6, sx(21, w), sy(40, hgt))

	// Unlocked systems add smaller functional clutter into the remaining gaps.
	// These replace the old fixed Tech Lab props that could render directly over
	// the laboratory sprite.
	if afkTechLabUnlocked() {
		drawGround("solar_panel", 0.435, 7, sx(27, w), sy(26, hgt))
		drawGround("energy_canister", 0.585, 8, sx(15, w), sy(29, hgt))
	}
	if afkModulesAvailable() {
		drawGround("cargo_crate_01", 0.410, 9, sx(22, w), sy(16, hgt))
		drawGround("cargo_crate_02", 0.455, 10, sx(23, w), sy(17, hgt))
		drawGround("heavy_cargo_crate", 0.720, 11, sx(26, w), sy(22, hgt))
	}
}
