//go:build windows

package main

import "strings"

const (
	globalProfileSkinWidth  int32 = 1124
	globalProfileSkinHeight int32 = 174
)

// profileSkinDisplayRect renders the selected skin centered inside the identity
// card. v343 increases the v340 compact presentation by 20% while keeping the
// banner centered and within the 1124x174 identity design area.
func profileSkinDisplayRect(identity RECT, w, h int32) RECT {
	// v347: fit the banner inside the actual runtime identity rectangle, not just
	// the nominal 1536x1024 design size. centeredPanel can clamp at smaller
	// resolutions, so the old fixed scaled size could extend outside the profile
	// interface even though it was valid at the reference resolution.
	desiredW := sx(1102, w)
	desiredH := sy(170, h)
	availW := identity.Right - identity.Left
	availH := identity.Bottom - identity.Top
	if desiredW > availW || desiredH > availH {
		// Preserve the banner aspect ratio while fitting both axes.
		scaleW := float64(availW) / float64(desiredW)
		scaleH := float64(availH) / float64(desiredH)
		scale := scaleW
		if scaleH < scale {
			scale = scaleH
		}
		if scale < 0 {
			scale = 0
		}
		desiredW = int32(float64(desiredW) * scale)
		desiredH = int32(float64(desiredH) * scale)
	}
	left := identity.Left + (availW-desiredW)/2
	top := identity.Top + (availH-desiredH)/2
	return RECT{left, top, left + desiredW, top + desiredH}
}

// drawGlobalProfileSkin overlays the equipped premultiplied-BGRA skin around
// the Global Profile identity box.
func drawGlobalProfileSkin(hdc uintptr, identity RECT, frameID int, w, h int32) {
	if !profileSkinUnlocked(frameID) {
		return
	}
	data := profileSkinAsset(frameID)
	if len(data) == int(globalProfileSkinWidth*globalProfileSkinHeight*4) {
		drawRawBGRAFit(hdc, data, globalProfileSkinWidth, globalProfileSkinHeight, profileSkinDisplayRect(identity, w, h))
	}
}

// profileRankNameColour keeps the rank label visually tied to the emblem tier.
func profileRankNameColour(rank string) uintptr {
	u := strings.ToUpper(strings.TrimSpace(rank))
	switch {
	case strings.HasPrefix(u, "BRONZE"):
		return rgb(177, 118, 68)
	case strings.HasPrefix(u, "SILVER"):
		return rgb(205, 216, 228)
	case strings.HasPrefix(u, "GOLD"):
		return rgb(242, 192, 55)
	case strings.HasPrefix(u, "PLATINUM"):
		return rgb(86, 213, 201)
	case strings.HasPrefix(u, "DIAMOND"):
		return rgb(89, 155, 255)
	case strings.HasPrefix(u, "MASTER"):
		return rgb(204, 83, 245)
	default:
		return rgb(245, 249, 253)
	}
}
