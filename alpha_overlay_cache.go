package main

import "sync"

type solidPixelResource struct {
	DC     uintptr
	Bitmap uintptr
	Old    uintptr
}

var solidPixelCache = struct {
	sync.Mutex
	items map[uintptr]*solidPixelResource
}{items: make(map[uintptr]*solidPixelResource)}

func ensureSolidPixel(hdc uintptr, color uintptr) *solidPixelResource {
	if hdc == 0 {
		return nil
	}
	solidPixelCache.Lock()
	defer solidPixelCache.Unlock()
	if e := solidPixelCache.items[color]; e != nil && e.DC != 0 && e.Bitmap != 0 {
		return e
	}
	dc, _, _ := createCompatibleDC.Call(hdc)
	if dc == 0 {
		return nil
	}
	bmp, _, _ := createCompatibleBitmap.Call(hdc, 1, 1)
	if bmp == 0 {
		deleteDC.Call(dc)
		return nil
	}
	old, _, _ := selectObject.Call(dc, bmp)
	fillSolidRect(dc, RECT{0, 0, 1, 1}, color)
	e := &solidPixelResource{DC: dc, Bitmap: bmp, Old: old}
	solidPixelCache.items[color] = e
	return e
}

func alphaSolidRect(hdc uintptr, r RECT, color uintptr, alpha byte) {
	if alpha == 0 || r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	e := ensureSolidPixel(hdc, color)
	if e == nil {
		return
	}
	blend := uintptr(uint32(alpha) << 16)
	alphaBlend.Call(
		hdc,
		uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top),
		e.DC,
		0, 0, 1, 1,
		blend,
	)
}

func releaseSolidPixelCache() {
	solidPixelCache.Lock()
	defer solidPixelCache.Unlock()
	for _, e := range solidPixelCache.items {
		if e == nil || e.DC == 0 {
			continue
		}
		if e.Old != 0 {
			selectObject.Call(e.DC, e.Old)
		}
		if e.Bitmap != 0 {
			deleteObject.Call(e.Bitmap)
		}
		deleteDC.Call(e.DC)
	}
	solidPixelCache.items = make(map[uintptr]*solidPixelResource)
}
