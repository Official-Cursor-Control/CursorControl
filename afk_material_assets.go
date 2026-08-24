//go:build windows

package main

// v461: authored Ship Module crafting-material sprites supplied by the user.
// The index order intentionally matches afkCraftComponentDefs:
// Pixel Wire, Star Alloy, Power Cell, Quantum Glass, Void Crystal, Nova Core.
type afkCraftMaterialSprite struct {
	file string
	data []byte
	w, h int32
}

var afkCraftMaterialSprites = [...]afkCraftMaterialSprite{
	{file: "pixel_wire.bgra", w: 234, h: 279},
	{file: "star_alloy.bgra", w: 208, h: 196},
	{file: "power_cell.bgra", w: 133, h: 270},
	{file: "quantum_glass.bgra", w: 218, h: 263},
	{file: "void_crystal.bgra", w: 181, h: 257},
	{file: "nova_core.bgra", w: 229, h: 249},
}

func loadAFKCraftMaterialAssets() {
	for i := range afkCraftMaterialSprites {
		s := &afkCraftMaterialSprites[i]
		s.data = readExternalBytes("ui", "ship_materials", s.file)
	}
}

func drawAFKCraftMaterialSprite(hdc uintptr, r RECT, index int, alpha byte) bool {
	if index < 0 || index >= len(afkCraftMaterialSprites) || r.Right <= r.Left || r.Bottom <= r.Top {
		return false
	}
	s := &afkCraftMaterialSprites[index]
	if len(s.data) < int(s.w*s.h*4) {
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
	boxW, boxH := r.Right-r.Left, r.Bottom-r.Top
	dstW := boxW
	dstH := int32(float64(dstW) * float64(cropH) / float64(cropW))
	if dstH > boxH {
		dstH = boxH
		dstW = int32(float64(dstH) * float64(cropW) / float64(cropH))
	}
	dstX := r.Left + (boxW-dstW)/2
	dstY := r.Top + (boxH-dstH)/2
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		sprite.dc, uintptr(crop.Left), uintptr(crop.Top), uintptr(cropW), uintptr(cropH), blend)
	return true
}
