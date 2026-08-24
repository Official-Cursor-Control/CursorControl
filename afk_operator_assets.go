//go:build windows

package main

type afkOperatorSprite struct {
	data   []byte
	locked []byte
	w, h   int32
}

type afkOperatorPetSprite struct {
	data []byte
	w, h int32
}

// v442 uses the user's newly separated Operator artwork. Bodies no longer contain
// their companion drones/pets, so companions can be animated independently.
var afkOperatorSprites = [...]*afkOperatorSprite{
	{w: 512, h: 896}, // Nova body, companion removed
	{w: 512, h: 896}, // Vega body, companion removed
	{w: 512, h: 896}, // Byte + flying chair, companion removed
	{w: 512, h: 896}, // Astra body, companion removed
	{w: 512, h: 896}, // Flux body, both companions removed
}

var afkOperatorSpriteNames = [...]string{"nova", "vega", "byte", "astra", "flux"}

const (
	afkOperatorPetNova = iota
	afkOperatorPetVega
	afkOperatorPetByte
	afkOperatorPetAstra
	afkOperatorPetFluxHover
	afkOperatorPetFluxSpider
	afkOperatorPetCount
)

var afkOperatorPetSprites = [...]*afkOperatorPetSprite{
	{w: 146, h: 180}, // Nova floating companion
	{w: 182, h: 168}, // Vega scout drone
	{w: 345, h: 168}, // Byte floating companion cluster
	{w: 197, h: 241}, // Astra cargo drone
	{w: 238, h: 138}, // Flux hovering drone
	{w: 173, h: 125}, // Flux floor spider
}

var afkOperatorPetSpriteNames = [...]string{
	"nova_pet",
	"vega_pet",
	"byte_pet",
	"astra_pet",
	"flux_pet_1",
	"flux_pet_2",
}

func loadAFKOperatorAssets() {
	for i, s := range afkOperatorSprites {
		if s == nil || i >= len(afkOperatorSpriteNames) {
			continue
		}
		s.data = readExternalBytes("ui", "operators", afkOperatorSpriteNames[i]+".bgra")
		if len(s.data) >= int(s.w*s.h*4) {
			s.locked = makeAFKOperatorLockedBGRA(s.data, s.w, s.h)
		}
	}
	for i, s := range afkOperatorPetSprites {
		if s == nil || i >= len(afkOperatorPetSpriteNames) {
			continue
		}
		s.data = readExternalBytes("ui", "operators", afkOperatorPetSpriteNames[i]+".bgra")
	}
}

func makeAFKOperatorLockedBGRA(src []byte, w, h int32) []byte {
	if len(src) < int(w*h*4) {
		return nil
	}
	out := make([]byte, len(src))
	// Preserve the supplied NPC silhouette but suppress almost all colour so an
	// unrecruited specialist still looks like the real character rather than a
	// generic placeholder. RGB values are premultiplied for AlphaBlend.
	for i := int32(0); i < w*h; i++ {
		o := int(i * 4)
		a := src[o+3]
		if a == 0 {
			continue
		}
		out[o+3] = a
		out[o+0] = byte(uint16(28) * uint16(a) / 255) // B
		out[o+1] = byte(uint16(22) * uint16(a) / 255) // G
		out[o+2] = byte(uint16(15) * uint16(a) / 255) // R
	}
	return out
}

func drawAFKOperatorSprite(hdc uintptr, r RECT, i int, recruited bool) bool {
	if i < 0 || i >= len(afkOperatorSprites) {
		return false
	}
	s := afkOperatorSprites[i]
	if s == nil || r.Right <= r.Left || r.Bottom <= r.Top {
		return false
	}
	data := s.data
	if !recruited {
		data = s.locked
	}
	if len(data) < int(s.w*s.h*4) {
		return false
	}
	drawRawBGRATrimmedFit(hdc, data, s.w, s.h, r)
	return true
}

func drawAFKOperatorPetSprite(hdc uintptr, r RECT, petIndex int) bool {
	if petIndex < 0 || petIndex >= len(afkOperatorPetSprites) {
		return false
	}
	s := afkOperatorPetSprites[petIndex]
	if s == nil || r.Right <= r.Left || r.Bottom <= r.Top || len(s.data) < int(s.w*s.h*4) {
		return false
	}
	drawRawBGRATrimmedFit(hdc, s.data, s.w, s.h, r)
	return true
}
