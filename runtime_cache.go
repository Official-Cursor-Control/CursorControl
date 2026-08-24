package main

import (
	"sync"
	"unsafe"
)

// RuntimeSpriteCache keeps immutable BGRA assets in persistent GDI DIBs.
// Previously drawRawBGRAFit created a DC, bitmap, and copied the complete
// source image every time the HUD painted. With multi-megabyte HUD assets,
// that became the dominant CPU/memory-bandwidth cost.
type runtimeSpriteKey struct {
	ptr uintptr
	n   int
	w   int32
	h   int32
}

type runtimeSpriteEntry struct {
	dc      uintptr
	bitmap  uintptr
	old     uintptr
	w       int32
	h       int32
	bytes   int64
	lastUse uint64
}

var runtimeSpriteCache = struct {
	sync.Mutex
	items   map[runtimeSpriteKey]*runtimeSpriteEntry
	bytes   int64
	tick    uint64
	maxByte int64
}{
	items:   make(map[runtimeSpriteKey]*runtimeSpriteEntry),
	maxByte: 160 << 20, // hard cap: 160 MiB
}

func runtimeSpriteKeyFor(data []byte, w, h int32) (runtimeSpriteKey, bool) {
	if len(data) == 0 || w <= 0 || h <= 0 || len(data) < int(w*h*4) {
		return runtimeSpriteKey{}, false
	}
	return runtimeSpriteKey{
		ptr: uintptr(unsafe.Pointer(&data[0])),
		n:   len(data),
		w:   w,
		h:   h,
	}, true
}

func releaseRuntimeSpriteEntry(e *runtimeSpriteEntry) {
	if e == nil || e.dc == 0 {
		return
	}
	if e.old != 0 {
		selectObject.Call(e.dc, e.old)
	}
	if e.bitmap != 0 {
		deleteObject.Call(e.bitmap)
	}
	deleteDC.Call(e.dc)
	e.dc = 0
	e.bitmap = 0
	e.old = 0
}

func evictRuntimeSpriteCacheLocked(need int64, protect runtimeSpriteKey) {
	for runtimeSpriteCache.bytes+need > runtimeSpriteCache.maxByte && len(runtimeSpriteCache.items) > 0 {
		var victimKey runtimeSpriteKey
		var victim *runtimeSpriteEntry
		for k, e := range runtimeSpriteCache.items {
			if k == protect {
				continue
			}
			if victim == nil || e.lastUse < victim.lastUse {
				victimKey = k
				victim = e
			}
		}
		if victim == nil {
			break
		}
		releaseRuntimeSpriteEntry(victim)
		runtimeSpriteCache.bytes -= victim.bytes
		delete(runtimeSpriteCache.items, victimKey)
	}
}

func ensureRuntimeSprite(hdc uintptr, data []byte, w, h int32) *runtimeSpriteEntry {
	key, ok := runtimeSpriteKeyFor(data, w, h)
	if !ok || hdc == 0 {
		return nil
	}

	runtimeSpriteCache.Lock()
	defer runtimeSpriteCache.Unlock()
	runtimeSpriteCache.tick++
	if e := runtimeSpriteCache.items[key]; e != nil {
		e.lastUse = runtimeSpriteCache.tick
		return e
	}

	bytesNeeded := int64(w) * int64(h) * 4
	evictRuntimeSpriteCacheLocked(bytesNeeded, key)

	dc, _, _ := createCompatibleDC.Call(hdc)
	if dc == 0 {
		return nil
	}

	var bits uintptr
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       w,
		BiHeight:      -h,
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BI_RGB,
	}}
	bmp, _, _ := createDIBSection.Call(
		dc,
		uintptr(unsafe.Pointer(&bmi)),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if bmp == 0 || bits == 0 {
		deleteDC.Call(dc)
		return nil
	}
	old, _, _ := selectObject.Call(dc, bmp)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(bits)), int(bytesNeeded)), data[:bytesNeeded])

	e := &runtimeSpriteEntry{
		dc:      dc,
		bitmap:  bmp,
		old:     old,
		w:       w,
		h:       h,
		bytes:   bytesNeeded,
		lastUse: runtimeSpriteCache.tick,
	}
	runtimeSpriteCache.items[key] = e
	runtimeSpriteCache.bytes += bytesNeeded
	return e
}

func releaseRuntimeSpriteCache() {
	runtimeSpriteCache.Lock()
	defer runtimeSpriteCache.Unlock()
	for _, e := range runtimeSpriteCache.items {
		releaseRuntimeSpriteEntry(e)
	}
	runtimeSpriteCache.items = make(map[runtimeSpriteKey]*runtimeSpriteEntry)
	runtimeSpriteCache.bytes = 0
	runtimeSpriteCache.tick = 0
}
