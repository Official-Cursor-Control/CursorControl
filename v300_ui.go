package main

import (
	"math"
	"time"
)

// garageRarityColour matches the research-backed four-tier garage presentation.
// Names mirror the familiar CS-style rarity ladder without changing gameplay.
func garageRarityColour(rarity string) uintptr {
	switch rarity {
	case "CELESTIAL":
		return rgb(228, 174, 57)
	case "NOVA":
		return rgb(235, 75, 75)
	case "NEBULA":
		return rgb(136, 71, 255)
	case "ORBITAL":
		return rgb(75, 105, 255)
	default:
		return rgb(48, 214, 255)
	}
}

func garageSlotBackdropColour(slot int) uintptr {
	switch {
	case slot == 11:
		return rgb(228, 174, 57)
	case slot >= 8:
		return rgb(235, 75, 75)
	case slot >= 4:
		return rgb(136, 71, 255)
	default:
		return rgb(75, 105, 255)
	}
}

func garageSlotFaceColour(slot int) uintptr {
	return blendColor(rgb(7, 15, 36), garageSlotBackdropColour(slot), 0.23)
}

func drawGarageLegendaryTwinkle(hdc uintptr, r RECT, w, hgt int32) {
	// Aurora Bloom: continuous, compact sparkle loop around the ship artwork only.
	// The Garage TIMER_UI repaint keeps this moving even when the mouse is idle.
	phase := float64(time.Now().UnixMilli()) / 1000.0
	base := rgb(255, 255, 255)
	glow := rgb(255, 222, 92)
	// Ship artwork occupies roughly the upper 64 px of the card. Keep stars close
	// to that area instead of scattering them across labels/rarity text.
	cx := (r.Left + r.Right) / 2
	cy := r.Top + sy(46, hgt)
	rx := sx(48, w)
	ry := sy(25, hgt)
	for i := 0; i < 7; i++ {
		a := phase*(0.75+0.08*float64(i)) + float64(i)*2*math.Pi/7
		x := cx + int32(math.Round(math.Cos(a)*float64(rx)))
		y := cy + int32(math.Round(math.Sin(a)*float64(ry)))
		pulse := 0.68 + 0.32*math.Sin(phase*(2.6+0.12*float64(i))+float64(i))
		sz := int32(math.Round((3.0 + float64(i%3)) * pulse))
		if sz < 2 {
			sz = 2
		}
		drawLineSimple(hdc, x-sz, y, x+sz, y, 1, base)
		drawLineSimple(hdc, x, y-sz, x, y+sz, 1, base)
		fillSolidRect(hdc, RECT{x - 1, y - 1, x + 2, y + 2}, glow)
	}
}
