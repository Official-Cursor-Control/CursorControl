//go:build windows

package main

// Shared Starbase management chrome. Keeping all management screens on the same
// frame/padding/title rhythm makes the mode feel like one interface instead of a
// collection of unrelated minigames.
func afkManagementPanelRect(w, hgt int32) RECT {
	f := afkMainFieldRect(w, hgt)
	return RECT{f.Left + sx(34, w), f.Top + sy(28, hgt), f.Right - sx(34, w), f.Bottom - sy(28, hgt)}
}

func drawAFKManagementChrome(hdc uintptr, p RECT, title, subtitle string, accent uintptr, w, hgt int32) {
	// Every Starbase management surface follows the player's selected HUD
	// background theme. Keep each subsystem's accent colour for identity, but
	// derive the large background, rim and shadow from the shared HUD palette.
	face, light, dark := themedHUDPanelPalette(true)
	panelFace := blendColor(face, dark, 0.34)
	panelLight := blendColor(light, accent, 0.28)
	drawBevelPanel(hdc, p, panelFace, panelLight, dark, 4)
	drawOutlineRect(hdc, RECT{p.Left + sx(6, w), p.Top + sy(6, hgt), p.Right - sx(6, w), p.Bottom - sy(6, hgt)}, blendColor(light, accent, 0.18), 1)
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 218, 79))
		t := fitTextEllipsis(hdc, hudTitleFont, title, p.Right-p.Left-sx(48, w))
		centeredTextOut(hdc, p.Left+sx(24, w), p.Right-sx(24, w), p.Top+sy(18, hgt), t)
		selectObject.Call(hdc, old)
	}
	if subtitle != "" && hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(130, 188, 215))
		t := fitTextEllipsis(hdc, hudTinyFont, subtitle, p.Right-p.Left-sx(64, w))
		centeredTextOut(hdc, p.Left+sx(32, w), p.Right-sx(32, w), p.Top+sy(58, hgt), t)
		selectObject.Call(hdc, old)
	}
}

func drawAFKManagementBackdrop(hdc uintptr, w, hgt int32) {
	f := afkMainFieldRect(w, hgt)
	face, _, dark := themedHUDPanelPalette(false)
	overlaySolidAlphaRect(hdc, f, blendColor(dark, face, 0.18), 188)
}
