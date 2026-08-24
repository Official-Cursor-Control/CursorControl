//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

func comMethod(obj uintptr, index int) uintptr {
	if obj == 0 {
		return 0
	}
	vt := *(*uintptr)(unsafe.Pointer(obj))
	return *(*uintptr)(unsafe.Pointer(vt + uintptr(index)*unsafe.Sizeof(uintptr(0))))
}

func comCall(obj uintptr, index int, args ...uintptr) (uintptr, uintptr, syscall.Errno) {
	fn := comMethod(obj, index)
	if fn == 0 {
		return 0, 0, syscall.Errno(1)
	}
	call := make([]uintptr, 0, len(args)+1)
	call = append(call, obj)
	call = append(call, args...)
	return syscall.SyscallN(fn, call...)
}

func comRelease(obj uintptr) {
	if obj != 0 {
		comCall(obj, 2)
	}
}

func d2dOK(hr uintptr) bool { return int32(hr) >= 0 }

func d2dColor(r, g, b, a float32) D2D1ColorF {
	return D2D1ColorF{R: r, G: g, B: b, A: a}
}

func d2dCreateSolidBrush(c D2D1ColorF) uintptr {
	if d2dRenderTarget == 0 {
		return 0
	}
	var brush uintptr
	hr, _, _ := comCall(
		d2dRenderTarget, 8,
		uintptr(unsafe.Pointer(&c)),
		0,
		uintptr(unsafe.Pointer(&brush)),
	)
	if !d2dOK(hr) {
		return 0
	}
	return brush
}

func d2dPackSize(w, h uint32) uintptr {
	return uintptr(uint64(w) | uint64(h)<<32)
}

const d2dPixelDPI float32 = 96

func d2dCreateBitmapFromBGRA(data []byte, w, h uint32) uintptr {
	if d2dRenderTarget == 0 || len(data) < int(w*h*4) {
		return 0
	}
	props := D2D1BitmapProperties{
		PixelFormat: D2D1PixelFormat{Format: 87, AlphaMode: 1}, // BGRA8 premultiplied
		DpiX:        d2dPixelDPI, DpiY: d2dPixelDPI,
	}
	var bmp uintptr
	hr, _, _ := comCall(
		d2dRenderTarget, 4,
		d2dPackSize(w, h),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(w*4),
		uintptr(unsafe.Pointer(&props)),
		uintptr(unsafe.Pointer(&bmp)),
	)
	if !d2dOK(hr) {
		return 0
	}
	return bmp
}

func d2dFillRect(r D2D1RectF, brush uintptr) {
	if d2dRenderTarget == 0 || brush == 0 {
		return
	}
	comCall(d2dRenderTarget, 17, uintptr(unsafe.Pointer(&r)), brush)
}

func d2dFillEllipse(e D2D1Ellipse, brush uintptr) {
	if d2dRenderTarget == 0 || brush == 0 {
		return
	}
	comCall(d2dRenderTarget, 21, uintptr(unsafe.Pointer(&e)), brush)
}

func d2dDrawEllipse(e D2D1Ellipse, brush uintptr, strokeWidth float32) {
	if d2dRenderTarget == 0 || brush == 0 || strokeWidth <= 0 {
		return
	}
	comCall(d2dRenderTarget, 20,
		uintptr(unsafe.Pointer(&e)),
		brush,
		uintptr(math.Float32bits(strokeWidth)),
		0)
}

func d2dDrawBitmap(bmp uintptr, dst, src D2D1RectF, opacity float32) {
	if d2dRenderTarget == 0 || bmp == 0 {
		return
	}
	comCall(
		d2dRenderTarget, 26,
		bmp,
		uintptr(unsafe.Pointer(&dst)),
		uintptr(math.Float32bits(opacity)),
		0,
		uintptr(unsafe.Pointer(&src)),
	)
}

func d2dSetTransform(m D2D1Matrix3x2F) {
	if d2dRenderTarget == 0 {
		return
	}
	comCall(d2dRenderTarget, 30, uintptr(unsafe.Pointer(&m)))
}

func d2dIdentity() D2D1Matrix3x2F {
	return D2D1Matrix3x2F{M11: 1, M22: 1}
}

func d2dTranslation(x, y float32) D2D1Matrix3x2F {
	return D2D1Matrix3x2F{M11: 1, M22: 1, Dx: x, Dy: y}
}

func d2dPackPoint(x, y float32) uintptr {
	return uintptr(uint64(math.Float32bits(x)) | uint64(math.Float32bits(y))<<32)
}

func rebuildMergedEnduranceD2DGeometry() bool {
	if d2dFactory == 0 || len(path) < 2 {
		return false
	}
	if mainHwnd != 0 {
		w, hgt := getClient(mainHwnd)
		clampEndurancePathToArena(w, hgt)
	}

	var geom uintptr
	hr, _, _ := comCall(d2dFactory, 10, uintptr(unsafe.Pointer(&geom)))
	if !d2dOK(hr) || geom == 0 {
		return false
	}

	var sink uintptr
	hr, _, _ = comCall(geom, 17, uintptr(unsafe.Pointer(&sink)))
	if !d2dOK(hr) || sink == 0 {
		comRelease(geom)
		return false
	}

	pts := make([]D2D1PointF, len(path))
	for i, p := range path {
		pts[i] = D2D1PointF{X: float32(p.X), Y: float32(p.Y)}
	}
	comCall(sink, 5, d2dPackPoint(pts[0].X, pts[0].Y), 1)
	if len(pts) > 1 {
		comCall(sink, 6, uintptr(unsafe.Pointer(&pts[1])), uintptr(len(pts)-1))
	}
	comCall(sink, 8, 0)
	hr, _, _ = comCall(sink, 9)
	comRelease(sink)
	if !d2dOK(hr) {
		comRelease(geom)
		return false
	}

	comRelease(d2dMergedRailGeometry)
	d2dMergedRailGeometry = geom
	d2dMergedRailDirty = false
	return true
}

func markMergedEnduranceRailDirty() {
	d2dMergedRailDirty = true
}

func appendEnduranceD2DGeometry(points []FPoint) {
	if d2dFactory == 0 || len(points) < 2 {
		return
	}
	var geom uintptr
	hr, _, _ := comCall(d2dFactory, 10, uintptr(unsafe.Pointer(&geom)))
	if !d2dOK(hr) || geom == 0 {
		return
	}

	var sink uintptr
	hr, _, _ = comCall(geom, 17, uintptr(unsafe.Pointer(&sink)))
	if !d2dOK(hr) || sink == 0 {
		comRelease(geom)
		return
	}

	pts := make([]D2D1PointF, len(points))
	for i, p := range points {
		pts[i] = D2D1PointF{X: float32(p.X), Y: float32(p.Y)}
	}

	comCall(sink, 5, d2dPackPoint(pts[0].X, pts[0].Y), 1) // hollow figure
	if len(pts) > 1 {
		comCall(sink, 6, uintptr(unsafe.Pointer(&pts[1])), uintptr(len(pts)-1))
	}
	comCall(sink, 8, 0) // open figure
	hr, _, _ = comCall(sink, 9)
	comRelease(sink)
	if !d2dOK(hr) {
		comRelease(geom)
		return
	}

	enduranceD2DGeometries = append(enduranceD2DGeometries, EnduranceD2DGeometry{
		Geometry: geom,
		MinX:     points[0].X,
		MaxX:     points[len(points)-1].X,
	})
}

func releaseEnduranceD2DGeometries() {
	for _, t := range enduranceD2DGeometries {
		comRelease(t.Geometry)
	}
	enduranceD2DGeometries = nil
}

func dropFirstEnduranceD2DGeometry() {
	if len(enduranceD2DGeometries) == 0 {
		return
	}
	comRelease(enduranceD2DGeometries[0].Geometry)
	enduranceD2DGeometries = enduranceD2DGeometries[1:]
}

func releaseD2DResources() {
	if d2dReady || d2dRenderTarget != 0 || d2dFactory != 0 {
		logRuntimeEvent("d2d_release", "releasing Direct2D resources")
	}
	releaseEnduranceD2DGeometries()
	for i := range d2dPowerupBitmaps {
		comRelease(d2dPowerupBitmaps[i])
		d2dPowerupBitmaps[i] = 0
	}
	for i := range d2dShipBitmaps {
		comRelease(d2dShipBitmaps[i])
		d2dShipBitmaps[i] = 0
	}
	for i := range d2dThrusterColorBrushes {
		for j := range d2dThrusterColorBrushes[i] {
			comRelease(d2dThrusterColorBrushes[i][j])
			d2dThrusterColorBrushes[i][j] = 0
		}
	}
	comRelease(d2dMergedRailGeometry)
	d2dMergedRailGeometry = 0
	d2dMergedRailDirty = false
	for _, b := range []uintptr{
		d2dBackgroundBitmap, d2dHazardBlueBitmap, d2dHazardOrangeBitmap, d2dRocketBitmap,
		d2dWarpPortalTopBitmap, d2dAlienMinionBitmap, d2dAlienBossBitmap, d2dUFOWarningBitmap, d2dPowerupKeyQBitmap, d2dPowerupKeyWBitmap,
		d2dRailGlowBrush, d2dRailDarkBrush, d2dRailMidBrush, d2dRailSilverBrush,
		d2dRailSafetyBrush, d2dRailCoreBrush, d2dRailStrokeStyle,
		d2dTargetGlowBrush, d2dTargetBrush, d2dTargetDarkBrush,
		d2dExplosionCoreBrush, d2dExplosionHotBrush, d2dExplosionFireBrush, d2dExplosionEmberBrush, d2dCrosshairBrush,
		d2dPowerupGlowBrush, d2dPowerupBlueBrush, d2dPowerupLightBrush, d2dPowerupYellowBrush, d2dPowerupRedBrush, d2dShieldFlashBrush, d2dShieldAuraBrush, d2dShieldEdgeBrush, d2dShieldCoreBrush, d2dShipHitboxBrush, d2dShipHitboxGlowBrush,
		d2dWarpWarmBrush, d2dWarpMagentaBrush, d2dWarpBlueBrush, d2dAlienWarningBrush, d2dAlienLaserGlowBrush, d2dAlienLaserCoreBrush, d2dAlienChargeFlashBrush, d2dAlienCautionFlashBrush, d2dPowerupFullPulseBrush,
		d2dParticleBrushes[0], d2dParticleBrushes[1], d2dParticleBrushes[2],
	} {
		comRelease(b)
	}
	d2dBackgroundBitmap = 0
	d2dHazardBlueBitmap = 0
	d2dHazardOrangeBitmap = 0
	d2dRocketBitmap = 0
	d2dWarpPortalTopBitmap = 0
	d2dAlienMinionBitmap = 0
	d2dAlienBossBitmap = 0
	d2dUFOWarningBitmap = 0
	d2dPowerupKeyQBitmap = 0
	d2dPowerupKeyWBitmap = 0
	d2dRailGlowBrush = 0
	d2dRailDarkBrush = 0
	d2dRailMidBrush = 0
	d2dRailSilverBrush = 0
	d2dRailSafetyBrush = 0
	d2dRailCoreBrush = 0
	d2dRailStrokeStyle = 0
	d2dTargetGlowBrush = 0
	d2dTargetBrush = 0
	d2dTargetDarkBrush = 0
	d2dExplosionCoreBrush = 0
	d2dExplosionHotBrush = 0
	d2dExplosionFireBrush = 0
	d2dExplosionEmberBrush = 0
	d2dCrosshairBrush = 0
	d2dPowerupGlowBrush = 0
	d2dPowerupBlueBrush = 0
	d2dPowerupLightBrush = 0
	d2dPowerupYellowBrush = 0
	d2dPowerupRedBrush = 0
	d2dShieldFlashBrush = 0
	d2dShieldAuraBrush = 0
	d2dShieldEdgeBrush = 0
	d2dShieldCoreBrush = 0
	d2dShipHitboxBrush = 0
	d2dShipHitboxGlowBrush = 0
	d2dWarpWarmBrush = 0
	d2dWarpMagentaBrush = 0
	d2dWarpBlueBrush = 0
	d2dAlienWarningBrush = 0
	d2dAlienLaserGlowBrush = 0
	d2dAlienLaserCoreBrush = 0
	d2dAlienChargeFlashBrush = 0
	d2dAlienCautionFlashBrush = 0
	d2dPowerupFullPulseBrush = 0
	d2dParticleBrushes = [3]uintptr{}
	comRelease(d2dRenderTarget)
	d2dRenderTarget = 0
	comRelease(d2dFactory)
	d2dFactory = 0
	d2dReady = false
}

func initD2D(h uintptr) bool {
	if h == 0 {
		return false
	}
	iid := GUID{
		Data1: 0x06152247,
		Data2: 0x6f50,
		Data3: 0x465a,
		Data4: [8]byte{0x92, 0x45, 0x11, 0x8b, 0xfd, 0x3b, 0x60, 0x07},
	}
	var factory uintptr
	hr, _, _ := d2d1CreateFactory.Call(
		0,
		uintptr(unsafe.Pointer(&iid)),
		0,
		uintptr(unsafe.Pointer(&factory)),
	)
	if !d2dOK(hr) || factory == 0 {
		return false
	}
	d2dFactory = factory

	w, hgt := getClient(h)
	props := D2D1RenderTargetProperties{
		Type:        0,
		PixelFormat: D2D1PixelFormat{Format: 0, AlphaMode: 0},
		// Cursor Control's path, collision and GDI HUD all use physical client
		// pixels. Keep Direct2D in that exact coordinate space as well.
		DpiX: d2dPixelDPI, DpiY: d2dPixelDPI, Usage: 0, MinLevel: 0,
	}
	hprops := D2D1HwndRenderTargetProperties{
		Hwnd:           h,
		PixelSize:      D2D1SizeU{Width: uint32(w), Height: uint32(hgt)},
		PresentOptions: 0,
	}
	var rt uintptr
	hr, _, _ = comCall(
		d2dFactory, 14,
		uintptr(unsafe.Pointer(&props)),
		uintptr(unsafe.Pointer(&hprops)),
		uintptr(unsafe.Pointer(&rt)),
	)
	if !d2dOK(hr) || rt == 0 {
		releaseD2DResources()
		return false
	}
	d2dRenderTarget = rt

	strokeProps := D2D1StrokeStyleProperties{
		StartCap:   0,
		EndCap:     0,
		DashCap:    0,
		LineJoin:   2,
		MiterLimit: 10,
		DashStyle:  0,
		DashOffset: 0,
	}
	var strokeStyle uintptr
	hr, _, _ = comCall(
		d2dFactory, 11,
		uintptr(unsafe.Pointer(&strokeProps)),
		0, 0,
		uintptr(unsafe.Pointer(&strokeStyle)),
	)
	if !d2dOK(hr) || strokeStyle == 0 {
		releaseD2DResources()
		return false
	}
	d2dRailStrokeStyle = strokeStyle

	// Fixed rail palette and lightweight playfield brushes.
	d2dRailGlowBrush = d2dCreateSolidBrush(d2dColor(42.0/255.0, 196.0/255.0, 1, 0.18))
	d2dRailDarkBrush = d2dCreateSolidBrush(d2dColor(8.0/255.0, 20.0/255.0, 40.0/255.0, 1))
	d2dRailMidBrush = d2dCreateSolidBrush(d2dColor(62.0/255.0, 82.0/255.0, 108.0/255.0, 1))
	d2dRailSilverBrush = d2dCreateSolidBrush(d2dColor(226.0/255.0, 237.0/255.0, 246.0/255.0, 1))
	d2dRailSafetyBrush = d2dCreateSolidBrush(d2dColor(46.0/255.0, 126.0/255.0, 247.0/255.0, 1))
	d2dRailCoreBrush = d2dCreateSolidBrush(d2dColor(220.0/255.0, 243.0/255.0, 1, 1))
	d2dParticleBrushes[0] = d2dCreateSolidBrush(d2dColor(26.0/255.0, 120.0/255.0, 1, 0.28))
	d2dParticleBrushes[1] = d2dCreateSolidBrush(d2dColor(45.0/255.0, 181.0/255.0, 1, 0.42))
	d2dParticleBrushes[2] = d2dCreateSolidBrush(d2dColor(93.0/255.0, 221.0/255.0, 1, 0.58))
	d2dTargetGlowBrush = d2dCreateSolidBrush(d2dColor(1, 48.0/255.0, 66.0/255.0, 0.34))
	d2dTargetBrush = d2dCreateSolidBrush(d2dColor(244.0/255.0, 53.0/255.0, 66.0/255.0, 1))
	d2dTargetDarkBrush = d2dCreateSolidBrush(d2dColor(5.0/255.0, 20.0/255.0, 46.0/255.0, 1))
	// Compact meteor-fire palette for successful target destruction. These are
	// intentionally small/brief so the FX never masks the rail on tight turns.
	d2dExplosionCoreBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.96, 0.72, 1.0))
	d2dExplosionHotBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.72, 0.16, 0.95))
	d2dExplosionFireBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.25, 0.04, 0.86))
	d2dExplosionEmberBrush = d2dCreateSolidBrush(d2dColor(0.72, 0.06, 0.02, 0.74))
	// Garage-selectable thruster palettes. Kept as cached brushes so switching
	// fire colors has zero resource creation cost during gameplay.
	thrusterPalettes := [7][4]D2D1ColorF{
		{d2dColor(0.62, 0.02, 0.01, 0.76), d2dColor(1.0, 0.18, 0.03, 0.90), d2dColor(1.0, 0.72, 0.16, 1.0), d2dColor(1.0, 0.94, 0.72, 1.0)},
		{d2dColor(0.02, 0.40, 0.06, 0.76), d2dColor(0.08, 0.90, 0.18, 0.90), d2dColor(0.50, 1.0, 0.45, 1.0), d2dColor(0.88, 1.0, 0.82, 1.0)},
		{d2dColor(0.02, 0.12, 0.58, 0.76), d2dColor(0.04, 0.42, 1.0, 0.90), d2dColor(0.36, 0.78, 1.0, 1.0), d2dColor(0.88, 0.97, 1.0, 1.0)},
		{d2dColor(0.52, 0.02, 0.28, 0.76), d2dColor(1.0, 0.16, 0.62, 0.90), d2dColor(1.0, 0.52, 0.82, 1.0), d2dColor(1.0, 0.90, 0.97, 1.0)},
		{d2dColor(0.28, 0.02, 0.55, 0.76), d2dColor(0.55, 0.10, 1.0, 0.90), d2dColor(0.76, 0.44, 1.0, 1.0), d2dColor(0.95, 0.87, 1.0, 1.0)},
		{d2dColor(0.55, 0.27, 0.01, 0.76), d2dColor(1.0, 0.62, 0.02, 0.90), d2dColor(1.0, 0.88, 0.24, 1.0), d2dColor(1.0, 0.98, 0.78, 1.0)},
		{d2dColor(0.34, 0.40, 0.48, 0.76), d2dColor(0.70, 0.78, 0.90, 0.90), d2dColor(0.90, 0.95, 1.0, 1.0), d2dColor(1.0, 1.0, 1.0, 1.0)},
	}
	for i := range thrusterPalettes {
		for j := 0; j < 4; j++ {
			d2dThrusterColorBrushes[i][j] = d2dCreateSolidBrush(thrusterPalettes[i][j])
		}
	}
	d2dCrosshairBrush = d2dCreateSolidBrush(d2dColor(0.02, 0.02, 0.025, 1))
	// Endurance power-ups share the rail's blue language but use distinct icon colors.
	d2dPowerupGlowBrush = d2dCreateSolidBrush(d2dColor(0.10, 0.72, 1.0, 0.30))
	d2dPowerupBlueBrush = d2dCreateSolidBrush(d2dColor(0.04, 0.36, 0.92, 0.98))
	d2dPowerupLightBrush = d2dCreateSolidBrush(d2dColor(0.45, 0.90, 1.0, 1.0))
	d2dPowerupYellowBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.91, 0.20, 1.0))
	d2dPowerupRedBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.20, 0.24, 1.0))
	d2dShieldFlashBrush = d2dCreateSolidBrush(d2dColor(0.34, 0.88, 1.0, 0.16))
	d2dShieldAuraBrush = d2dCreateSolidBrush(d2dColor(0.18, 0.72, 1.0, 0.10))
	d2dShieldEdgeBrush = d2dCreateSolidBrush(d2dColor(0.25, 0.88, 1.0, 0.72))
	d2dShieldCoreBrush = d2dCreateSolidBrush(d2dColor(0.76, 0.96, 1.0, 0.95))
	d2dShipHitboxBrush = d2dCreateSolidBrush(d2dColor(0.20, 1.0, 0.36, 0.95))
	d2dShipHitboxGlowBrush = d2dCreateSolidBrush(d2dColor(0.20, 1.0, 0.36, 0.20))
	// v175 wormhole palette: warm core + magenta/cyan edge energy.
	d2dWarpWarmBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.34, 0.06, 0.14))
	d2dWarpMagentaBrush = d2dCreateSolidBrush(d2dColor(0.90, 0.12, 1.0, 0.12))
	d2dWarpBlueBrush = d2dCreateSolidBrush(d2dColor(0.12, 0.72, 1.0, 0.10))
	d2dAlienWarningBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.82, 0.12, 0.70))
	d2dAlienLaserGlowBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.04, 0.08, 0.34))
	d2dAlienLaserCoreBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.84, 0.78, 1.0))
	d2dAlienChargeFlashBrush = d2dCreateSolidBrush(d2dColor(1.0, 1.0, 1.0, 0.15))
	d2dAlienCautionFlashBrush = d2dCreateSolidBrush(d2dColor(1.0, 0.78, 0.0, 0.18))
	d2dPowerupFullPulseBrush = d2dCreateSolidBrush(d2dColor(0.15, 0.88, 1.0, 0.24))

	if textureRoot == "" {
		releaseD2DResources()
		return false
	}
	bgData, err := os.ReadFile(filepath.Join(textureRoot, "endurance_background.bgra"))
	if err != nil || len(bgData) < 1942*809*4 {
		releaseD2DResources()
		return false
	}
	blueData, err := os.ReadFile(filepath.Join(textureRoot, "hazard_blue.bgra"))
	if err != nil || len(blueData) < 64*160*4 {
		releaseD2DResources()
		return false
	}
	orangeData, err := os.ReadFile(filepath.Join(textureRoot, "hazard_orange.bgra"))
	if err != nil || len(orangeData) < 64*160*4 {
		releaseD2DResources()
		return false
	}
	rocketData, err := os.ReadFile(filepath.Join(textureRoot, "rocket_cursor.bgra"))
	if err != nil || len(rocketData) < 32*22*4 {
		releaseD2DResources()
		return false
	}
	portalTopData, portalTopErr := os.ReadFile(filepath.Join(textureRoot, "warp_portal_top.bgra"))
	alienMinionData, alienMinionErr := os.ReadFile(filepath.Join(textureRoot, "alien_minion.bgra"))
	alienBossData, alienBossErr := os.ReadFile(filepath.Join(textureRoot, "alien_boss.bgra"))
	ufoWarningData, ufoWarningErr := os.ReadFile(filepath.Join(textureRoot, "ufo_warning_overlay.bgra"))
	powerupKeyQData, powerupKeyQErr := os.ReadFile(filepath.Join(textureRoot, "powerup_key_q.bgra"))
	powerupKeyWData, powerupKeyWErr := os.ReadFile(filepath.Join(textureRoot, "powerup_key_w.bgra"))
	powerNames := []string{"powerup_distance.bgra", "powerup_shield.bgra", "powerup_slow.bgra"}
	for i, name := range powerNames {
		data, e := os.ReadFile(filepath.Join(textureRoot, name))
		if e == nil && len(data) >= 64*64*4 {
			d2dPowerupBitmaps[i] = d2dCreateBitmapFromBGRA(data, 64, 64)
		}
	}
	for i := 1; i <= 12; i++ {
		data, e := os.ReadFile(filepath.Join(textureRoot, fmt.Sprintf("ship_%d.bgra", i)))
		sw, sh := shipTextureW[i], shipTextureH[i]
		if e == nil && sw > 0 && sh > 0 && len(data) >= int(sw*sh*4) {
			d2dShipBitmaps[i] = d2dCreateBitmapFromBGRA(data, uint32(sw), uint32(sh))
		}
	}

	d2dBackgroundBitmap = d2dCreateBitmapFromBGRA(bgData, 1942, 809)
	d2dHazardBlueBitmap = d2dCreateBitmapFromBGRA(blueData, 64, 160)
	d2dHazardOrangeBitmap = d2dCreateBitmapFromBGRA(orangeData, 64, 160)
	if len(rocketData) >= int(shipTextureW[0]*shipTextureH[0]*4) {
		d2dRocketBitmap = d2dCreateBitmapFromBGRA(rocketData, uint32(shipTextureW[0]), uint32(shipTextureH[0]))
	}
	if portalTopErr == nil && len(portalTopData) >= 144*256*4 {
		d2dWarpPortalTopBitmap = d2dCreateBitmapFromBGRA(portalTopData, 144, 256)
	}
	if alienMinionErr == nil && len(alienMinionData) >= 96*64*4 {
		d2dAlienMinionBitmap = d2dCreateBitmapFromBGRA(alienMinionData, 96, 64)
	}
	if alienBossErr == nil && len(alienBossData) >= 260*150*4 {
		d2dAlienBossBitmap = d2dCreateBitmapFromBGRA(alienBossData, 260, 150)
	}
	if ufoWarningErr == nil && len(ufoWarningData) >= 1000*300*4 {
		d2dUFOWarningBitmap = d2dCreateBitmapFromBGRA(ufoWarningData, 1000, 300)
	}
	if powerupKeyQErr == nil && len(powerupKeyQData) >= 44*28*4 {
		d2dPowerupKeyQBitmap = d2dCreateBitmapFromBGRA(powerupKeyQData, 44, 28)
	}
	if powerupKeyWErr == nil && len(powerupKeyWData) >= 44*28*4 {
		d2dPowerupKeyWBitmap = d2dCreateBitmapFromBGRA(powerupKeyWData, 44, 28)
	}

	if d2dBackgroundBitmap == 0 || d2dRailSafetyBrush == 0 {
		releaseD2DResources()
		return false
	}
	d2dReady = true
	return true
}

func d2dDrawEnduranceRail(ar RECT) {
	if d2dRenderTarget == 0 {
		return
	}
	if d2dMergedRailDirty || d2dMergedRailGeometry == 0 {
		if !rebuildMergedEnduranceD2DGeometry() {
			return
		}
	}

	d2dSetTransform(d2dTranslation(
		-float32(float64(ar.Left)+enduranceCameraX),
		-float32(ar.Top),
	))
	const base = 27.0
	g := d2dMergedRailGeometry
	comCall(d2dRenderTarget, 22, g, d2dRailGlowBrush, uintptr(math.Float32bits(base+26)), d2dRailStrokeStyle)
	comCall(d2dRenderTarget, 22, g, d2dRailDarkBrush, uintptr(math.Float32bits(base+19)), d2dRailStrokeStyle)
	comCall(d2dRenderTarget, 22, g, d2dRailMidBrush, uintptr(math.Float32bits(base+14)), d2dRailStrokeStyle)
	comCall(d2dRenderTarget, 22, g, d2dRailSilverBrush, uintptr(math.Float32bits(base+8)), d2dRailStrokeStyle)
	comCall(d2dRenderTarget, 22, g, d2dRailSafetyBrush, uintptr(math.Float32bits(base+2)), d2dRailStrokeStyle)
	comCall(d2dRenderTarget, 22, g, d2dRailCoreBrush, uintptr(math.Float32bits(4)), d2dRailStrokeStyle)
	d2dSetTransform(d2dIdentity())
}

func d2dDrawEnduranceParticles(ar RECT) {
	if gameMeta.ParticleQuality == 0 {
		return
	}
	if particleEpoch.IsZero() {
		particleEpoch = time.Now()
	}
	t := enduranceParticleClockNow()
	w := float64(ar.Right - ar.Left)
	h := float64(ar.Bottom - ar.Top)
	count := 190
	if gameMeta.ParticleQuality == 1 {
		count = 110
	}
	depth := enduranceWorldDepth(enduranceProgressDistance())
	if gameMeta.ParticleQuality == 1 {
		count += int(28 * depth)
	} else {
		count += int(52 * depth)
	}
	warpVisual := enduranceParticleSpeedMultiplierNow()
	if warpVisual > 1.05 {
		if gameMeta.ParticleQuality == 1 {
			count = 190
		} else {
			count = 340
		}
	}
	for i := 0; i < count; i++ {
		seedX := math.Mod(math.Abs(math.Sin(float64(i)*12.731+0.4)*43758.5453), 1)
		seedY := math.Mod(math.Abs(math.Sin(float64(i)*41.117+3.2)*24634.6345), 1)
		layer := i % 3
		speed := 38.0 + float64((i*19)%74)
		travel := math.Mod(seedX*w+t*speed, w)
		x := w - travel
		y := seedY * h
		length := float32(7+(i*5)%18) * float32(1.0+0.30*depth)
		if warpVisual > 1.0 {
			// v175: longer streaks create a tunnel/warp sensation instead of
			// ordinary star particles merely moving faster.
			length *= float32(1.0 + 0.78*(warpVisual-1.0))
		}
		thickness := float32(1.5)
		if layer == 1 {
			length += 5
			thickness = 2.2
		}
		if layer == 2 {
			length += 9
			thickness = 2.8
		}
		r := D2D1RectF{
			Left:   float32(x) - length,
			Top:    float32(y) - thickness/2,
			Right:  float32(x),
			Bottom: float32(y) + thickness/2,
		}
		brush := d2dParticleBrushes[layer]
		if warpVisual > 1.15 {
			switch i % 9 {
			case 0, 6:
				brush = d2dWarpWarmBrush
			case 2, 7:
				brush = d2dWarpMagentaBrush
			case 4:
				brush = d2dWarpBlueBrush
			}
		}
		d2dFillRect(r, brush)
	}
}

func d2dDrawEnduranceTargets(ar RECT) {
	for i, t := range targets {
		if t.Clicked {
			continue
		}
		p := targetCurrentPoint(i)
		x := float32(p.X - float64(ar.Left))
		y := float32(p.Y - float64(ar.Top))
		if x < -30 || x > float32(ar.Right-ar.Left)+30 || y < -30 || y > float32(ar.Bottom-ar.Top)+30 {
			continue
		}
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: 22, RadiusY: 22}, d2dTargetGlowBrush)
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: 15, RadiusY: 15}, d2dTargetBrush)
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: 10, RadiusY: 10}, d2dTargetDarkBrush)
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: 3, RadiusY: 3}, d2dTargetBrush)
	}
}

func addEnduranceTargetExplosion(targetScreenPoint FPoint) {
	// Capture the destroyed target centre in D2D playfield-local coordinates at
	// the exact hit frame. The explosion is therefore anchored to the target
	// that was destroyed and can never follow subsequent cursor movement.
	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)
	local := FPoint{
		X: targetScreenPoint.X - float64(ar.Left),
		Y: targetScreenPoint.Y - float64(ar.Top),
	}
	enduranceTargetExplosions = append(enduranceTargetExplosions, TargetExplosion{
		Point: local, Started: time.Now(), Seed: float64(len(enduranceTargetExplosions)+1)*1.731 + local.X*0.013 + local.Y*0.007,
	})
	// Warp zones can create targets quickly. Bound retained FX so rendering cost
	// stays constant even if the player clicks extremely fast.
	if len(enduranceTargetExplosions) > 18 {
		enduranceTargetExplosions = enduranceTargetExplosions[len(enduranceTargetExplosions)-18:]
	}
}

func d2dDrawEnduranceTargetExplosions(ar RECT) {
	if len(enduranceTargetExplosions) == 0 {
		return
	}
	now := time.Now()
	kept := enduranceTargetExplosions[:0]
	for _, e := range enduranceTargetExplosions {
		age := now.Sub(e.Started).Seconds()
		const life = 0.42
		if age < 0 || age >= life {
			continue
		}
		kept = append(kept, e)
		p := age / life
		x := float32(e.Point.X)
		y := float32(e.Point.Y)

		// Meteor-like fireball rather than a generic circular pop. The hot center
		// flashes first, then irregular orange/red flame tongues peel outward.
		outer := float32(6.0 + p*15.0)
		mid := outer * 0.68
		core := outer * float32(0.34*(1.0-p*0.55))
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y + outer*0.10}, RadiusX: outer * 0.92, RadiusY: outer * 0.72}, d2dExplosionEmberBrush)
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: mid, RadiusY: mid * 0.74}, d2dExplosionFireBrush)
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y - core*0.10}, RadiusX: core, RadiusY: core * 0.86}, d2dExplosionCoreBrush)

		// Uneven flame lobes make the burst read as burning meteor fire.
		for i := 0; i < 7; i++ {
			a := e.Seed + float64(i)*0.897597901
			d := 3.0 + p*(8.0+float64((i*5)%9))
			lx := x + float32(math.Cos(a)*d)
			ly := y + float32(math.Sin(a)*d*0.62) - float32((1.0-p)*float64(i%3))
			rx := float32(3.8+float64((i+1)%3)) * float32(1.0-p*0.45)
			ry := rx * float32(0.72+0.18*math.Abs(math.Sin(a)))
			brush := d2dExplosionFireBrush
			if i%3 == 0 {
				brush = d2dExplosionHotBrush
			}
			d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{lx, ly}, RadiusX: rx, RadiusY: ry}, brush)
		}

		// Small embers spit out beyond the flame body, like the meteor trails.
		for i := 0; i < 10; i++ {
			a := e.Seed*1.31 + float64(i)*0.6283185307
			d := 5.0 + p*(17.0+float64((i*7)%13))
			sx := x + float32(math.Cos(a)*d)
			sy := y + float32(math.Sin(a)*d*0.68)
			r := float32(2.2 - p*1.35)
			if r < 0.65 {
				r = 0.65
			}
			brush := d2dExplosionHotBrush
			if i%2 == 1 {
				brush = d2dExplosionEmberBrush
			}
			d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{sx, sy}, RadiusX: r, RadiusY: r * 0.82}, brush)
		}
	}
	enduranceTargetExplosions = kept
}

func d2dDrawUFOWarningOverlay(ar RECT) {
	if enduranceAlienBossState != alienBossWarning || d2dUFOWarningBitmap == 0 {
		return
	}
	age := time.Since(enduranceAlienBossStateStarted).Seconds()
	if age < 0 || age >= 3.0 {
		return
	}

	playW := float32(ar.Right - ar.Left)
	playH := float32(ar.Bottom - ar.Top)

	// Two short yellow game-area flashes during the three-second warning.
	// Flash windows are deliberately brief so the rail remains readable.
	flash1 := age >= 0.18 && age <= 0.48
	flash2 := age >= 1.48 && age <= 1.78
	if flash1 || flash2 {
		d2dFillRect(
			D2D1RectF{Left: 0, Top: 0, Right: playW, Bottom: playH},
			d2dAlienCautionFlashBrush,
		)
	}

	// Hazard sign: large enough to notice instantly, but kept shallow and
	// only 54% opaque so the track remains clearly visible underneath.
	w := playW * 0.62
	h := w * 300.0 / 1000.0
	maxH := playH * 0.30
	if h > maxH {
		h = maxH
		w = h * 1000.0 / 300.0
	}
	x := (playW - w) * 0.5
	y := (playH - h) * 0.44

	dst := D2D1RectF{Left: x, Top: y, Right: x + w, Bottom: y + h}
	src := D2D1RectF{Left: 0, Top: 0, Right: 1000, Bottom: 300}

	comCall(d2dRenderTarget, 26, d2dUFOWarningBitmap,
		uintptr(unsafe.Pointer(&dst)),
		uintptr(math.Float32bits(0.54)),
		1,
		uintptr(unsafe.Pointer(&src)))
}

func d2dDrawEnduranceAliens(ar RECT) {
	if d2dAlienMinionBitmap != 0 {
		src := D2D1RectF{Left: 0, Top: 0, Right: 96, Bottom: 64}
		for _, a := range enduranceAlienMinions {
			x := float32(a.X - float64(ar.Left))
			y := float32(a.Y - float64(ar.Top))
			trailY := y + float32(a.Height)*0.55
			trailLen := float32(18.0 + math.Min(34.0, a.Speed*0.045))
			d2dFillRect(D2D1RectF{Left: x + float32(a.Width) - 4, Top: trailY - 1.2, Right: x + float32(a.Width) + trailLen, Bottom: trailY + 1.2}, d2dPowerupBlueBrush)
			d2dDrawBitmap(d2dAlienMinionBitmap,
				D2D1RectF{Left: x, Top: y, Right: x + float32(a.Width), Bottom: y + float32(a.Height)},
				src, 1)
		}
	}
	if enduranceAlienBossState >= alienBossEntering && enduranceAlienBossState < alienBossDone && d2dAlienBossBitmap != 0 {
		x := float32(enduranceAlienBossX - float64(ar.Left))
		y := float32(enduranceAlienBossY - float64(ar.Top))
		if enduranceAlienBossState == alienBossEntering {
			pulse := float32(0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*11.0))
			d2dFillEllipse(D2D1Ellipse{
				Point:   D2D1PointF{x + float32(enduranceAlienBossWidth)*0.82, y + float32(enduranceAlienBossHeight)*0.73},
				RadiusX: 14 + pulse*7, RadiusY: 6 + pulse*3,
			}, d2dPowerupBlueBrush)
		}
		d2dDrawBitmap(d2dAlienBossBitmap,
			D2D1RectF{Left: x, Top: y, Right: x + float32(enduranceAlienBossWidth), Bottom: y + float32(enduranceAlienBossHeight)},
			D2D1RectF{Left: 0, Top: 0, Right: 260, Bottom: 150}, 1)
	}
	if enduranceAlienBossState == alienBossAim1 || enduranceAlienBossState == alienBossAim2 || enduranceAlienBossState == alienBossAim3 || enduranceAlienBossState == alienBossAim4 {
		// Power-up effect: rapid subtle white flashes while the cannon charges.
		chargeAge := time.Since(enduranceAlienBossStateStarted).Seconds()
		flashPulse := 0.5 + 0.5*math.Sin(chargeAge*30.0)
		if flashPulse > 0.42 {
			d2dFillRect(
				D2D1RectF{Left: 0, Top: 0, Right: float32(ar.Right - ar.Left), Bottom: float32(ar.Bottom - ar.Top)},
				d2dAlienChargeFlashBrush,
			)
		}

		x2 := float32(alienBossCannonX() - float64(ar.Left))
		if x2 > 0 {
			y := float32(alienBossCannonY() - float64(ar.Top))
			chargeAge := time.Since(enduranceAlienBossStateStarted).Seconds()
			pulse := float32(0.5 + 0.5*math.Sin(chargeAge*22))
			chargeP := chargeAge / alienBossAimDuration()
			if chargeP < 0 {
				chargeP = 0
			}
			if chargeP > 1 {
				chargeP = 1
			}
			th := 1.0 + pulse*1.25 + float32(easeSmoothStep(chargeP)*1.35)
			d2dFillRect(D2D1RectF{Left: 0, Top: y - th, Right: x2, Bottom: y + th}, d2dAlienWarningBrush)
		}
	}
	if x1, x2, active := alienLaserVisibleSegment(ar); active && x2 > x1 {
		l := float32(x1 - float64(ar.Left))
		r := float32(x2 - float64(ar.Left))
		y := float32(alienBossCannonY() - float64(ar.Top))
		d2dFillRect(D2D1RectF{Left: l, Top: y - 15, Right: r, Bottom: y + 15}, d2dAlienLaserGlowBrush)
		d2dFillRect(D2D1RectF{Left: l, Top: y - 7, Right: r, Bottom: y + 7}, d2dTargetBrush)
		d2dFillRect(D2D1RectF{Left: l, Top: y - 2.5, Right: r, Bottom: y + 2.5}, d2dAlienLaserCoreBrush)
	}
}

func d2dDrawEnduranceHazards(ar RECT) {
	// The sprite is 64x160. Its rock body occupies the lower portion while the
	// flame trail extends upward. Keep the rock aligned to the original square
	// collision box and draw the full trail above it instead of squeezing or
	// cropping the whole meteor into a square.
	src := D2D1RectF{Left: 0, Top: 0, Right: 64, Bottom: 160}
	const spriteAspect = float32(160.0 / 64.0)
	for _, b := range enduranceBlocks {
		x := float32(b.X - float64(ar.Left))
		y := float32(b.Y - float64(ar.Top))
		size := float32(b.Width)
		spriteH := size * spriteAspect
		// Bottom-align the sprite to the hazard's collision square. This keeps the
		// meteor rock where gameplay expects it while the full fire trail remains visible.
		dst := D2D1RectF{Left: x, Top: y + size - spriteH, Right: x + size, Bottom: y + size}
		bmp := d2dHazardBlueBitmap
		if b.Orange {
			bmp = d2dHazardOrangeBitmap
		}
		d2dDrawBitmap(bmp, dst, src, 1)
	}
}

func d2dDrawEndurancePowerups(ar RECT) {
	if len(endurancePowerups) == 0 {
		return
	}
	for _, pu := range endurancePowerups {
		x := float32(pu.Point.X - enduranceCameraX - float64(ar.Left))
		y := float32(pu.Point.Y - float64(ar.Top))
		if x < -38 || x > float32(ar.Right-ar.Left)+38 || y < -38 || y > float32(ar.Bottom-ar.Top)+38 {
			continue
		}
		bmp := uintptr(0)
		if pu.Kind >= 0 && pu.Kind < len(d2dPowerupBitmaps) {
			bmp = d2dPowerupBitmaps[pu.Kind]
		}
		if bmp != 0 {
			// Preserve the square icon proportions and keep pickups compact enough
			// that they do not visually swallow the rail. Nearest-neighbour keeps
			// the 8-bit art crisp instead of smearing/stretching pixels.
			dst := D2D1RectF{Left: x - 19, Top: y - 19, Right: x + 19, Bottom: y + 19}
			src := D2D1RectF{Left: 0, Top: 0, Right: 64, Bottom: 64}
			comCall(d2dRenderTarget, 26, bmp, uintptr(unsafe.Pointer(&dst)), uintptr(math.Float32bits(1.0)), 1, uintptr(unsafe.Pointer(&src)))
		} else {
			d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: 18, RadiusY: 18}, d2dPowerupBlueBrush)
		}
	}
}

func d2dDrawEndurancePowerupInventory(ar RECT) {
	if !enduranceActive() {
		return
	}

	playH := float32(ar.Bottom - ar.Top)

	const boxW = float32(58)
	const boxH = float32(78)
	const gap = float32(8)
	const margin = float32(14)

	baseY := playH - boxH - margin
	shieldX := margin
	timeX := shieldX + boxW + gap

	drawSlot := func(x float32, kind int, count int, keyBmp uintptr, isShield bool) {
		// Slim dark-neon card: deep navy body, crisp cyan perimeter, minimal clutter.
		d2dFillRect(D2D1RectF{Left: x, Top: baseY, Right: x + boxW, Bottom: baseY + boxH}, d2dTargetDarkBrush)
		d2dFillRect(D2D1RectF{Left: x + 1, Top: baseY + 1, Right: x + boxW - 1, Bottom: baseY + boxH - 1}, d2dRailMidBrush)
		d2dFillRect(D2D1RectF{Left: x + 3, Top: baseY + 3, Right: x + boxW - 3, Bottom: baseY + boxH - 3}, d2dRailDarkBrush)

		// Full-capacity reminder pulse stays subtle and only brightens the card edge.
		if count >= 2 && d2dPowerupFullPulseBrush != 0 {
			phase := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*math.Pi*1.6)
			if phase > 0.48 {
				d2dDrawEllipse(
					D2D1Ellipse{Point: D2D1PointF{x + boxW/2, baseY + 31}, RadiusX: 23, RadiusY: 23},
					d2dPowerupFullPulseBrush, 2.0,
				)
			}
		}

		bmp := uintptr(0)
		if kind >= 0 && kind < len(d2dPowerupBitmaps) {
			bmp = d2dPowerupBitmaps[kind]
		}
		if bmp != 0 {
			dst := D2D1RectF{Left: x + 11, Top: baseY + 7, Right: x + 47, Bottom: baseY + 43}
			src := D2D1RectF{Left: 0, Top: 0, Right: 64, Bottom: 64}
			comCall(d2dRenderTarget, 26, bmp, uintptr(unsafe.Pointer(&dst)),
				uintptr(math.Float32bits(1.0)), 1, uintptr(unsafe.Pointer(&src)))
		}

		// Two strong colored capacity bars, matching the power-up's own color language.
		for i := 0; i < 2; i++ {
			left := x + 9 + float32(i)*22
			outer := D2D1RectF{Left: left, Top: baseY + 48, Right: left + 17, Bottom: baseY + 55}
			d2dFillRect(outer, d2dTargetDarkBrush)
			inner := D2D1RectF{Left: left + 1.5, Top: baseY + 49.5, Right: left + 15.5, Bottom: baseY + 53.5}
			if i < count {
				if isShield {
					d2dFillRect(inner, d2dPowerupLightBrush)
				} else {
					d2dFillRect(inner, d2dPowerupRedBrush)
				}
			} else {
				d2dFillRect(inner, d2dRailMidBrush)
			}
		}

		// Compact Q/W control label centred at the foot of the card.
		if keyBmp != 0 {
			dst := D2D1RectF{Left: x + 18, Top: baseY + 59, Right: x + 40, Bottom: baseY + 73}
			src := D2D1RectF{Left: 0, Top: 0, Right: 44, Bottom: 28}
			comCall(d2dRenderTarget, 26, keyBmp, uintptr(unsafe.Pointer(&dst)),
				uintptr(math.Float32bits(0.92)), 1, uintptr(unsafe.Pointer(&src)))
		}
	}

	drawSlot(shieldX, endurancePowerupShield, enduranceStoredShields, d2dPowerupKeyQBitmap, true)
	drawSlot(timeX, endurancePowerupSlow, enduranceStoredTime, d2dPowerupKeyWBitmap, false)
}

func enduranceThrusterWarpIntensity() float64 {
	// Tie the afterburner directly to the physical Warp acceleration curve.
	// 0 = normal flight, 1 = full Warp. Recovery shrinks it smoothly as speed falls.
	span := enduranceWarpSpeedMultiplier - 1.0
	if span <= 0 {
		return 0
	}
	t := (enduranceWarpSpeedMultiplierNow() - 1.0) / span
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t
}

func selectedThrusterBrushes() (uintptr, uintptr, uintptr, uintptr) {
	id := gameMeta.SelectedFireColor
	if id < 0 || id >= len(fireColorDefs) || !fireColorUnlocked(id) {
		id = 0
	}
	if id == 7 {
		// Rainbow changes continuously but only reuses cached palette brushes.
		base := int(enduranceParticleClockNow()*5.0) % 7
		return d2dThrusterColorBrushes[base][0], d2dThrusterColorBrushes[(base+1)%7][1], d2dThrusterColorBrushes[(base+2)%7][2], d2dThrusterColorBrushes[(base+3)%7][3]
	}
	b := d2dThrusterColorBrushes[id]
	return b[0], b[1], b[2], b[3]
}

func selectedThrusterColorID() int {
	id := gameMeta.SelectedFireColor
	if id < 0 || id >= len(fireColorDefs) || !fireColorUnlocked(id) {
		return 0
	}
	return id
}

func d2dDrawEnduranceThrusters(x, y float32) {
	if d2dRenderTarget == 0 {
		return
	}

	// v286: every Garage ship now uses one centred rear thruster. The artwork is
	// cosmetic only; the exhaust origin is fixed to the shared 32x22 render box,
	// so changing ship skins never changes gameplay geometry.
	const (
		shipW     = float32(32.0)
		rearInset = float32(2.0)
	)

	nozzleX := x - shipW/2 + rearInset
	nozzleY := y
	warp := enduranceThrusterWarpIntensity()
	sizeMul := selectedFireSizeMultiplier()
	emberBrush, flameBrush, coreBrush, flareBrush := selectedThrusterBrushes()

	t := enduranceParticleClockNow()
	phase := t * 19.0
	flicker := 0.5 + 0.5*math.Sin(phase)
	fine := 0.5 + 0.5*math.Sin(t*37.0+1.31)
	pulse := 0.72*flicker + 0.28*fine

	outerLen := float32(9.0+3.0*pulse+27.0*warp) * sizeMul
	hotLen := outerLen * float32(0.69+0.04*pulse)
	coreLen := outerLen * float32(0.39+0.03*fine)
	outerHalf := float32(2.45+0.72*pulse+1.05*warp) * sizeMul
	hotHalf := outerHalf * 0.68
	coreHalf := outerHalf * 0.34

	if warp > 0.04 {
		haloLen := outerLen * float32(1.08+0.22*warp)
		haloHalf := outerHalf * float32(1.08+0.28*warp)
		d2dFillRect(
			D2D1RectF{Left: nozzleX - haloLen, Top: nozzleY - haloHalf, Right: nozzleX + 0.5, Bottom: nozzleY + haloHalf},
			d2dWarpBlueBrush,
		)
	}

	// v349: tapered pixel-rocket exhaust. Build the flame from stepped segments
	// so it narrows toward the tail instead of reading as one rectangular bar.
	drawTaper := func(length, half float32, brush uintptr, tipInset float32) {
		if length <= 0 || half <= 0 {
			return
		}
		segments := []struct{ a, b, scale float32 }{
			{0.00, 0.26, 0.34},
			{0.26, 0.55, 0.58},
			{0.55, 0.80, 0.80},
			{0.80, 1.00, 1.00},
		}
		for _, seg := range segments {
			x0 := nozzleX - length*(1.0-seg.a) + tipInset
			x1 := nozzleX - length*(1.0-seg.b) + tipInset
			h := half * seg.scale
			d2dFillRect(D2D1RectF{Left: x0, Top: nozzleY - h, Right: x1 + 0.7, Bottom: nozzleY + h}, brush)
		}
	}
	drawTaper(outerLen, outerHalf, emberBrush, 0)
	drawTaper(hotLen, hotHalf, flameBrush, 0.6)
	drawTaper(coreLen, coreHalf, coreBrush, 1.0)
	// Bright rectangular nozzle core matches the reference's clean white-hot centre.
	flareHalf := float32(0.9+0.22*pulse+0.15*warp) * sizeMul
	d2dFillRect(D2D1RectF{Left: nozzleX - 2.2*sizeMul, Top: nozzleY - flareHalf, Right: nozzleX + 1.4, Bottom: nozzleY + flareHalf}, flareBrush)

	// v286: premium Silver, Gold and Rainbow exhausts shed a richer cloud of
	// tiny deterministic micro-particles. No particle objects are allocated or
	// retained, keeping this cosmetic upgrade lightweight at Endurance speed.
	colorID := selectedThrusterColorID()
	if colorID == 5 || colorID == 6 || colorID == 7 {
		count := 12
		if gameMeta.ParticleQuality == 1 {
			count = 7
		} else if gameMeta.ParticleQuality == 0 {
			count = 3
		}
		for i := 0; i < count; i++ {
			seed := float64(i)*1.731 + t*(7.5+float64(i%3)*0.55)
			wave := 0.5 + 0.5*math.Sin(seed*2.11)
			trail := float32(outerLen) + float32(4+i*4) + float32(wave*8.0) + float32(warp*float64(i*2))
			py := nozzleY + float32(math.Sin(seed*3.07))*float32(2.0+0.42*float64(i))
			pr := float32(0.34 + 0.18*float64((i%3)+1) + 0.18*warp)
			brush := flareBrush
			if colorID == 7 {
				brush = d2dThrusterColorBrushes[(i+int(t*5.0))%7][3]
			}
			d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{nozzleX - trail, py}, RadiusX: pr, RadiusY: pr}, brush)
		}
	}

	// A small detached ember keeps normal colours alive without recreating the
	// old two-engine look.
	emberX := nozzleX - outerLen - float32(1.5+3.0*fine+5.0*warp)
	emberR := float32(0.55 + 0.35*flicker)
	d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{emberX, nozzleY + float32((fine-0.5)*1.8)}, RadiusX: emberR, RadiusY: emberR}, flareBrush)
}

func rareGarageShipGlowLevel(id int) int {
	// v300: only the final four Garage slots glow. Their effects step up in
	// intensity so rarity is visible without changing sprite size or collision.
	switch id {
	case 2: // plane 9
		return 1
	case 8: // plane 10
		return 2
	case 12: // plane 11
		return 3
	case 9: // plane 12: rarest / strongest treatment
		return 4
	default:
		return 0
	}
}

func d2dDrawRareGarageShipAura(x, y float32, id int) {
	level := rareGarageShipGlowLevel(id)
	if level == 0 || d2dRenderTarget == 0 {
		return
	}

	t := enduranceParticleClockNow()
	pulse := float32(0.5 + 0.5*math.Sin(t*3.6+float64(id)*0.71))
	slowPulse := float32(0.5 + 0.5*math.Sin(t*1.85+float64(id)*0.37))
	quality := gameMeta.ParticleQuality

	// v349 Aurora Bloom (ship 9): sparkle-only treatment. No halos, rings or
	// orbitals. Stars continuously loop in a tight field around the ship.
	if id == 9 {
		count := 9
		if quality == 0 {
			count = 5
		} else if quality == 1 {
			count = 7
		}
		for i := 0; i < count; i++ {
			a := t*(0.85+0.045*float64(i)) + float64(i)*2*math.Pi/float64(count)
			rx := float32(18.0 + float32(i%3)*2.5)
			ry := float32(9.0 + float32((i+1)%3)*1.5)
			sx := x + float32(math.Cos(a))*rx
			sy := y + float32(math.Sin(a))*ry
			p := float32(0.55 + 0.45*math.Sin(t*3.4+float64(i)*1.3))
			arm := float32(1.8 + 2.0*p)
			d2dFillRect(D2D1RectF{Left: sx - arm, Top: sy - 0.55, Right: sx + arm, Bottom: sy + 0.55}, d2dPowerupYellowBrush)
			d2dFillRect(D2D1RectF{Left: sx - 0.55, Top: sy - arm, Right: sx + 0.55, Bottom: sy + arm}, d2dShieldFlashBrush)
		}
		return
	}

	// Replace the cheap noisy sparkle cloud with tighter, premium-looking energy
	// structures: layered halos, controlled orbitals and a compact hero effect for
	// the final four rarest ships. These remain purely visual.
	baseW := float32(19.0 + float32(level)*1.5 + pulse*1.8)
	baseH := float32(10.2 + float32(level)*0.9 + pulse*0.9)
	d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x - 1.2, y}, RadiusX: baseW + 7, RadiusY: baseH + 4}, d2dTargetGlowBrush)
	d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW + 3, RadiusY: baseH + 1.6}, d2dShieldAuraBrush)
	d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW, RadiusY: baseH}, d2dShieldEdgeBrush, 1.0+0.3*pulse)

	satBrush := d2dShieldCoreBrush
	glowBrush := d2dShieldAuraBrush
	edgeBrush := d2dShieldEdgeBrush
	accentBrush := d2dPowerupLightBrush
	if id == 8 {
		glowBrush = d2dWarpMagentaBrush
		edgeBrush = d2dShieldCoreBrush
		accentBrush = d2dWarpBlueBrush
	} else if id == 12 {
		glowBrush = d2dWarpBlueBrush
		edgeBrush = d2dPowerupLightBrush
		accentBrush = d2dShieldFlashBrush
	} else if id == 9 {
		glowBrush = d2dWarpWarmBrush
		edgeBrush = d2dPowerupYellowBrush
		accentBrush = d2dShieldCoreBrush
	}
	_ = satBrush

	switch id {
	case 2: // sleek ion halo
		trailLen := float32(7.0 + 4.0*slowPulse)
		d2dFillRect(D2D1RectF{Left: x - baseW - trailLen, Top: y - 2.3, Right: x - baseW*0.15, Bottom: y + 2.3}, d2dWarpBlueBrush)
		d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW + 4.2, RadiusY: baseH + 2.2}, edgeBrush, 0.8)
		for i := 0; i < 4; i++ {
			a := t*1.6 + float64(i)*math.Pi/2
			sx := x + float32(math.Cos(a))*(baseW+3.2)
			sy := y + float32(math.Sin(a))*(baseH+1.4)
			d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{sx, sy}, RadiusX: 1.4, RadiusY: 1.4}, d2dShieldCoreBrush)
		}
	case 8: // premium phase lattice
		d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW + 2.2, RadiusY: baseH + 3.4}, edgeBrush, 0.85)
		d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW + 5.4, RadiusY: baseH - 1.3}, d2dShieldEdgeBrush, 0.7)
		for i := 0; i < 6; i++ {
			a := t*1.25 + float64(i)*2*math.Pi/6
			rx := baseW + 4 + float32(i%2)*1.5
			ry := baseH + 2 - float32(i%2)*0.5
			sx := x + float32(math.Cos(a))*rx
			sy := y + float32(math.Sin(a))*ry
			d2dFillRect(D2D1RectF{Left: sx - 1.2, Top: sy - 1.2, Right: sx + 1.2, Bottom: sy + 1.2}, d2dShieldCoreBrush)
		}
		if quality > 0 {
			for i := 0; i < 2+quality; i++ {
				a := -t*1.9 + float64(i)*math.Pi/(1.0+float64(quality))
				sx := x + float32(math.Cos(a))*(baseW+1.8)
				sy := y + float32(math.Sin(a))*(baseH+4.6)
				d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{sx, sy}, RadiusX: 0.95, RadiusY: 0.95}, d2dPowerupLightBrush)
			}
		}
	case 12: // refined prism wake
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x - 2.2, y}, RadiusX: baseW + 4.6, RadiusY: baseH + 2.2}, glowBrush)
		d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW + 5.0, RadiusY: baseH + 2.8}, edgeBrush, 0.95)
		for i := 0; i < 4; i++ {
			off := float32(i-1) * 5.0
			d2dFillRect(D2D1RectF{Left: x - baseW - 6 - float32(i), Top: y + off - 0.7, Right: x - baseW + 1.5, Bottom: y + off + 0.7}, d2dShieldFlashBrush)
		}
		for i := 0; i < 6; i++ {
			a := t*0.95 + float64(i)*2*math.Pi/6
			sx := x + float32(math.Cos(a))*(baseW+5.4)
			sy := y + float32(math.Sin(a))*(baseH+1.8)
			r := float32(1.0)
			if i%3 == 0 {
				r = 1.6
			}
			d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{sx, sy}, RadiusX: r, RadiusY: r}, d2dShieldCoreBrush)
		}
	case 9: // mythic contraband crown
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW + 8.2, RadiusY: baseH + 4.0}, glowBrush)
		d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW + 6.2, RadiusY: baseH + 3.2}, edgeBrush, 1.2)
		d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: baseW + 2.8, RadiusY: baseH + 1.1}, d2dShieldCoreBrush, 0.85)
		// four polished crown flares
		for _, pt := range []D2D1PointF{{x - baseW*0.72, y - baseH - 2.6}, {x, y - baseH - 4.4}, {x + baseW*0.72, y - baseH - 2.6}, {x + baseW + 2.6, y}} {
			d2dFillRect(D2D1RectF{Left: pt.X - 0.8, Top: pt.Y - 4.5, Right: pt.X + 0.8, Bottom: pt.Y + 4.5}, d2dPowerupYellowBrush)
			d2dFillRect(D2D1RectF{Left: pt.X - 4.5, Top: pt.Y - 0.8, Right: pt.X + 4.5, Bottom: pt.Y + 0.8}, accentBrush)
		}
		sparks := 6
		if quality == 0 {
			sparks = 4
		} else if quality == 2 {
			sparks = 8
		}
		for i := 0; i < sparks; i++ {
			a := t*0.8 + float64(i)*2*math.Pi/float64(sparks)
			rx := baseW + 8 + float32(i%2)*2
			ry := baseH + 5 + float32((i+1)%2)
			sx := x + float32(math.Cos(a))*rx
			sy := y + float32(math.Sin(a))*ry
			d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{sx, sy}, RadiusX: 1.55, RadiusY: 1.55}, d2dShieldCoreBrush)
		}
	}

	// Common premium finish: restrained moving micro-gems instead of noisy dust.
	orbiters := 2 + level
	if quality == 0 && orbiters > 3 {
		orbiters = 3
	} else if quality == 1 && orbiters > 4 {
		orbiters = 4
	}
	for i := 0; i < orbiters; i++ {
		a := -t*(0.7+0.07*float64(i)) + float64(i)*2*math.Pi/float64(orbiters)
		rx := baseW + 2.5 + float32((i%2)*2)
		ry := baseH + 0.8 + float32(((i+1)%2)*2)
		sx := x + float32(math.Cos(a))*rx
		sy := y + float32(math.Sin(a))*ry
		r := float32(0.9 + 0.25*slowPulse)
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{sx, sy}, RadiusX: r, RadiusY: r}, d2dShieldCoreBrush)
	}
}

func d2dDrawEnduranceShipPulseOutline(shipBmp uintptr, dst, src D2D1RectF, shipID int) {
	if shipBmp == 0 || d2dRenderTarget == 0 {
		return
	}
	level := rareGarageShipGlowLevel(shipID)
	// Every ship gets the clean trace; rarity only increases its strength slightly.
	strength := float32(0.16 + 0.035*float32(level))
	t := enduranceParticleClockNow()
	pulse := float32(0.5 + 0.5*math.Sin(t*3.4+float64(shipID)*0.31))
	opacity := strength + pulse*0.13
	spread := float32(1.2 + 0.35*float32(level) + 0.45*pulse)
	for _, o := range []D2D1PointF{{-spread, 0}, {spread, 0}, {0, -spread}, {0, spread}, {-spread * 0.72, -spread * 0.72}, {spread * 0.72, -spread * 0.72}, {-spread * 0.72, spread * 0.72}, {spread * 0.72, spread * 0.72}} {
		g := D2D1RectF{Left: dst.Left + o.X, Top: dst.Top + o.Y, Right: dst.Right + o.X, Bottom: dst.Bottom + o.Y}
		d2dDrawBitmap(shipBmp, g, src, opacity)
	}
}

func d2dDrawEnduranceCrosshair(ar RECT) {
	if !cursorInArena {
		return
	}
	shipID := gameMeta.SelectedShip
	shipBmp := d2dRocketBitmap
	if shipID > 0 && shipID < len(d2dShipBitmaps) && d2dShipBitmaps[shipID] != 0 {
		shipBmp = d2dShipBitmaps[shipID]
	}
	if shipBmp == 0 {
		return
	}
	x := float32(cursorPos.X - float64(ar.Left))
	y := float32(cursorPos.Y - float64(ar.Top))

	// Animated single-engine exhaust is part of the existing ship render pass.
	// It creates no gameplay objects and scales directly with Warp acceleration.
	d2dDrawEnduranceThrusters(x, y)

	// Preserve each cosmetic's artwork aspect ratio inside one common visual box.
	// Gameplay collision remains identical for every ship.
	sw, sh := shipTextureW[shipID], shipTextureH[shipID]
	if sw <= 0 || sh <= 0 {
		sw, sh = shipTextureW[0], shipTextureH[0]
	}
	boxW, boxH := float32(38), float32(30)
	aspect := float32(sw) / float32(sh)
	rw, rh := boxW, boxW/aspect
	if rh > boxH {
		rh = boxH
		rw = boxH * aspect
	}
	dst := D2D1RectF{Left: x - rw/2, Top: y - rh/2, Right: x + rw/2, Bottom: y + rh/2}
	src := D2D1RectF{Left: 0, Top: 0, Right: float32(sw), Bottom: float32(sh)}

	// v431: all ship cosmetics use a clean pulsing silhouette trace only.
	d2dDrawEnduranceShipPulseOutline(shipBmp, dst, src, shipID)

	// Light-blue protection bubble: a true perfect circle around the ship.
	if enduranceShieldActive() {
		age := float64(time.Now().UnixMilli()) / 1000.0
		pulse := float32(0.5 + 0.5*math.Sin(age*5.2))
		radius := float32(27.0 + 1.8*pulse)

		// Soft circular energy volume behind the ship.
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: radius, RadiusY: radius}, d2dShieldAuraBrush)
		d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: radius - 4, RadiusY: radius - 4}, d2dShieldFlashBrush)

		// Crisp circular luminous shell and inner refraction ring.
		d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: radius, RadiusY: radius}, d2dShieldEdgeBrush, 2.4)
		d2dDrawEllipse(D2D1Ellipse{Point: D2D1PointF{x, y}, RadiusX: radius - 4.5, RadiusY: radius - 4.5}, d2dShieldCoreBrush, 1.15)

		// Four tiny sparks orbit on the same circular radius.
		for i := 0; i < 4; i++ {
			a := age*1.8 + float64(i)*math.Pi/2
			sx := x + float32(math.Cos(a))*radius
			sy := y + float32(math.Sin(a))*radius
			d2dFillEllipse(D2D1Ellipse{Point: D2D1PointF{sx, sy}, RadiusX: 1.7, RadiusY: 1.7}, d2dShieldCoreBrush)
		}
	}
	d2dDrawBitmap(shipBmp, dst, src, 1)
	d2dDrawShipHitbox(x, y)
}

func enduranceWarpPortalPlacement(ar RECT) (D2D1RectF, D2D1RectF, float32, bool) {
	if d2dRenderTarget == 0 || enduranceNextWarpAt <= 0 {
		return D2D1RectF{}, D2D1RectF{}, 0, false
	}
	checkpoint := enduranceNextWarpAt
	if enduranceWarpCheckpoint > 0 && (enduranceWarpCueActive || enduranceWarpActive || enduranceWarpRecoveryUntil > 0) {
		checkpoint = enduranceWarpCheckpoint
	}
	track := enduranceTrackDistance()
	ahead := checkpoint - track
	// Keep the portal alive briefly after the checkpoint so the rocket can visibly
	// pass between its back and front layers instead of the marker disappearing
	// the instant the Ready/Go sequence starts.
	if ahead < -18.0 || ahead > enduranceWarpMarkerShowMeters {
		return D2D1RectF{}, D2D1RectF{}, 0, false
	}
	x := float32(42.0 + ahead*10.0)
	y := float32(ar.Bottom-ar.Top) * 0.5
	if x < -120 || x > float32(ar.Right-ar.Left)+120 {
		return D2D1RectF{}, D2D1RectF{}, 0, false
	}
	// Keep the portal at a fixed size. v137 continuously rescaled both portal
	// bitmaps every frame for a pulse effect, which caused avoidable GPU work
	// exactly when players needed a stable frame rate for the warp entry.
	// Both portal layers use this exact same destination rectangle. Keeping the
	// aspect ratio identical prevents the front rim from drifting off the back layer.
	// v222: portal artwork is cropped around the portal body rather than the
	// trailing debris. Size is tuned so the inner opening comfortably surrounds
	// the ~50px visible Endurance rail instead of looking pinched around it.
	pw := float32(146.0)
	ph := float32(260.0)
	dst := D2D1RectF{Left: x - pw/2, Top: y - ph/2, Right: x + pw/2, Bottom: y + ph/2}
	src := D2D1RectF{Left: 0, Top: 0, Right: 144, Bottom: 256}
	return dst, src, 1.0, true
}

func d2dDrawEnduranceWarpPortalTop(ar RECT) {
	if d2dWarpPortalTopBitmap == 0 {
		return
	}
	dst, src, opacity, ok := enduranceWarpPortalPlacement(ar)
	if !ok {
		return
	}
	// v220: single portal layer only. The portal is intentionally rendered over
	// the rail and player; there is no separate back layer to drift/misalign.
	d2dDrawBitmap(d2dWarpPortalTopBitmap, dst, src, opacity)
}

func d2dDrawEnduranceWarpAtmosphere(ar RECT) {
	if !enduranceInWarpTransition() && !(enduranceWarpRecoveryUntil > 0 && enduranceTrackDistance() < enduranceWarpRecoveryUntil) {
		return
	}

	w := float32(ar.Right - ar.Left)
	h := float32(ar.Bottom - ar.Top)
	if w <= 0 || h <= 0 {
		return
	}

	// The overlay intensity follows actual rail acceleration, so the visual
	// tunnel builds and collapses with the ship rather than switching abruptly.
	rail := enduranceWarpSpeedMultiplierNow()
	span := enduranceWarpSpeedMultiplier - 1.0
	intensity := 0.0
	if span > 0 {
		intensity = (rail - 1.0) / span
	}
	if enduranceWarpCueActive && intensity < 0.18 {
		// Subtle pre-entry pulse during READY/GO.
		intensity = 0.18 + 0.10*(math.Sin(time.Since(enduranceWarpCueStarted).Seconds()*7.0)+1.0)*0.5
	}
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}

	// Warm centre haze: suggests hot matter stretching through the wormhole.
	coreH := h * float32(0.16+0.22*intensity)
	cy := h * 0.5
	core := D2D1RectF{Left: 0, Top: cy - coreH, Right: w, Bottom: cy + coreH}
	d2dFillRect(core, d2dWarpWarmBrush)

	// Magenta/cyan tunnel bands pulse at the top and bottom edges.
	band := float32(10.0 + 18.0*intensity)
	pulse := float32((math.Sin(enduranceParticleClockNow()*5.5) + 1.0) * 0.5)
	top1 := D2D1RectF{Left: 0, Top: 0, Right: w, Bottom: band + pulse*8}
	bot1 := D2D1RectF{Left: 0, Top: h - (band + pulse*8), Right: w, Bottom: h}
	d2dFillRect(top1, d2dWarpMagentaBrush)
	d2dFillRect(bot1, d2dWarpBlueBrush)

	// Repeating inner bands move toward the centre, giving the impression that
	// the playfield is inside a tunnel rather than simply scrolling faster.
	phase := float32(math.Mod(enduranceParticleClockNow()*120.0, 120.0))
	for i := 0; i < 4; i++ {
		yOff := float32(i*52) + phase*0.30
		th := float32(3.0 + 3.0*intensity)
		if cy-yOff > 0 {
			d2dFillRect(D2D1RectF{Left: 0, Top: cy - yOff - th, Right: w, Bottom: cy - yOff}, d2dWarpMagentaBrush)
		}
		if cy+yOff < h {
			d2dFillRect(D2D1RectF{Left: 0, Top: cy + yOff, Right: w, Bottom: cy + yOff + th}, d2dWarpBlueBrush)
		}
	}
}

func renderEnduranceD2D() {
	if !d2dReady || d2dRenderTarget == 0 || state != StatePlaying || !enduranceActive() {
		return
	}
	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)
	localW := float32(ar.Right - ar.Left)
	localH := float32(ar.Bottom - ar.Top)
	if localW <= 0 || localH <= 0 {
		return
	}

	comCall(d2dRenderTarget, 48) // BeginDraw

	// Background travels right -> left, the same direction as Endurance particles.
	// Two full-width copies tile seamlessly so there is never an exposed edge.
	if particleEpoch.IsZero() {
		particleEpoch = time.Now()
	}
	bgSpeed := 18.0 + 12.0*enduranceWorldDepth(enduranceProgressDistance())
	bgPhase := math.Mod(enduranceAmbientClockNow()*bgSpeed, float64(localW))
	src := D2D1RectF{Left: 0, Top: 0, Right: 1942, Bottom: 809}
	dst1 := D2D1RectF{Left: -float32(bgPhase), Top: 0, Right: localW - float32(bgPhase), Bottom: localH}
	dst2 := D2D1RectF{Left: localW - float32(bgPhase), Top: 0, Right: 2*localW - float32(bgPhase), Bottom: localH}
	// Full opacity prevents the D2D child target from darkening the source
	// image relative to the waiting GDI renderer.
	bgOpacity := float32(1.0)
	if enduranceInWarpTransition() {
		// Pulse the playfield toward transparency during the warp sequence.
		pulse := (math.Sin(time.Since(enduranceWarpCueStarted).Seconds()*8.0) + 1.0) * 0.5
		bgOpacity = float32(0.48 + pulse*0.34)
	}
	d2dDrawBitmap(d2dBackgroundBitmap, dst1, src, bgOpacity)
	d2dDrawBitmap(d2dBackgroundBitmap, dst2, src, bgOpacity)

	d2dDrawDeepSpaceAtmosphere(ar)
	d2dDrawEnduranceWarpAtmosphere(ar)
	d2dDrawEnduranceParticles(ar)
	d2dDrawEnduranceRail(ar)

	// Z-order policy: passive/danger hazards sit beneath gameplay-critical
	// click targets and pickups so required interactions cannot be visually
	// hidden by a meteor. Boss/laser layers remain above normal interactives.
	d2dDrawEnduranceHazards(ar)
	d2dDrawEnduranceTargets(ar)
	d2dDrawEndurancePowerups(ar)
	d2dDrawEnduranceTargetExplosions(ar)
	d2dDrawPolishVFX(ar)
	d2dDrawUFOWarningOverlay(ar)
	d2dDrawEnduranceAliens(ar)
	d2dDrawEnduranceCrosshair(ar)
	d2dDrawEnduranceWarpPortalTop(ar)

	hr, _, _ := comCall(d2dRenderTarget, 49, 0, 0) // EndDraw / Present
	if !d2dOK(hr) {
		// Device-loss recovery: briefly use the sprite-based software fallback,
		// then rebuild Direct2D instead of getting stuck in the old square-hazard renderer.
		logRuntimeEvent("d2d_device_loss", fmt.Sprintf("present_hr=%d", int32(hr)))
		releaseD2DResources()
		d2dRetryAfter = time.Now().Add(250 * time.Millisecond)
		setD2DPlayfieldVisible(false)
		if mainHwnd != 0 && state == StatePlaying && enduranceActive() {
			setTimer.Call(mainHwnd, TIMER_GAME, 8, 0)
			invalidateRect.Call(mainHwnd, 0, 0)
		}
	}
}

func d2dChildWndProc(h uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_NCHITTEST:
		// Input passes through to the parent game window.
		return ^uintptr(0)
	case WM_SETCURSOR:
		setCursor.Call(0)
		return 1
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := beginPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			endPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
		}
		return 0
	}
	r, _, _ := defWindowProcW.Call(h, uintptr(msg), wParam, lParam)
	return r
}

func setD2DPlayfieldVisible(v bool) {
	if d2dChildHwnd == 0 || d2dChildVisible == v {
		return
	}
	if v {
		showWindow.Call(d2dChildHwnd, SW_SHOW)
	} else {
		showWindow.Call(d2dChildHwnd, 0)
	}
	d2dChildVisible = v
}
