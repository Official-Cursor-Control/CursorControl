//go:build windows

package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

//go:embed assets/profile_fonts/*.ttf assets/profile_fonts/*.otf
var advancedFontFS embed.FS

const frPrivate = 0x10

var (
	addFontResourceExW    = gdi32.NewProc("AddFontResourceExW")
	removeFontResourceExW = gdi32.NewProc("RemoveFontResourceExW")

	advancedFontOnce        sync.Once
	advancedFontPaths       [12]string
	advancedFontLoaded      [12]bool
	advancedFontHandleCache = map[string]uintptr{}
)

var advancedFontFiles = []string{
	"01_chinese_rocks.otf",
	"02_overwave.otf",
	"03_manolete.ttf",
	"04_corrupted_file.ttf",
	"05_hellbone.otf",
	"06_american_captain.otf",
	"07_verve.ttf",
	"08_evil_empire.otf",
	"09_nothing_magic.ttf",
	"10_ancient.ttf",
	"11_storm_gust.otf",
	"12_golden_varsity.ttf",
}

// These are the actual family names embedded in the supplied TTF/OTF files.
var advancedFontFaces = []string{
	"Chinese Rocks Rg",
	"Overwave",
	"MANOLETE",
	"Corrupted File",
	"Hellbone-Demo",
	"American Captain",
	"Verve",
	"Evil Empire",
	"Nothing Magic",
	"Ancient",
	"Storm Gust",
	"Golden Varsity",
}

func ensureAdvancedProfileFontsLoaded() {
	advancedFontOnce.Do(func() {
		cacheDir := filepath.Join(os.TempDir(), "CursorControl", "profile_fonts")
		_ = os.MkdirAll(cacheDir, 0o755)
		for i, name := range advancedFontFiles {
			b, err := advancedFontFS.ReadFile(filepath.ToSlash(filepath.Join("assets", "profile_fonts", name)))
			if err != nil || len(b) == 0 {
				continue
			}
			p := filepath.Join(cacheDir, name)
			if err := os.WriteFile(p, b, 0o600); err != nil {
				continue
			}
			advancedFontPaths[i] = p
			u := utf16ptr(p)
			if u == nil {
				continue
			}
			r, _, _ := addFontResourceExW.Call(uintptr(unsafe.Pointer(u)), frPrivate, 0)
			advancedFontLoaded[i] = r != 0
		}
	})
}

func advancedProfileFontHandle(fontChoice int, pixelHeight int32, weight uintptr) uintptr {
	if fontChoice <= 0 || fontChoice > len(advancedFontFaces) || pixelHeight <= 0 {
		return 0
	}
	ensureAdvancedProfileFontsLoaded()
	idx := fontChoice - 1
	if !advancedFontLoaded[idx] {
		return 0
	}
	key := fmt.Sprintf("%d|%d|%d", fontChoice, pixelHeight, weight)
	if h := advancedFontHandleCache[key]; h != 0 {
		return h
	}
	h := makeFontForFace(advancedFontFaces[idx], uint32(pixelHeight), weight)
	if h != 0 {
		advancedFontHandleCache[key] = h
	}
	return h
}

func releaseAdvancedProfileFonts() {
	for k, h := range advancedFontHandleCache {
		if h != 0 {
			deleteObject.Call(h)
		}
		delete(advancedFontHandleCache, k)
	}
	for i, p := range advancedFontPaths {
		if !advancedFontLoaded[i] || p == "" {
			continue
		}
		u := utf16ptr(p)
		if u != nil {
			removeFontResourceExW.Call(uintptr(unsafe.Pointer(u)), frPrivate, 0)
		}
		advancedFontLoaded[i] = false
	}
}

func profileColorChannels(c uintptr) (byte, byte, byte) {
	return byte(c & 0xff), byte((c >> 8) & 0xff), byte((c >> 16) & 0xff)
}

// drawAdvancedProfileTextInBox draws using the real user-supplied TTF/OTF font.
// It dynamically reduces the font height until the text fits the box, so a
// decorative face can never make the player's name disappear or overflow.
func drawAdvancedProfileTextInBox(hdc uintptr, box RECT, text string, fontChoice, p1, p2 int, vertical, shadow bool, shadowC int) RECT {
	if fontChoice <= 0 || fontChoice > len(advancedFontFaces) || text == "" || box.Right <= box.Left || box.Bottom <= box.Top {
		return RECT{}
	}
	maxH := box.Bottom - box.Top
	if maxH < 10 {
		return RECT{}
	}
	fontH := maxH - 6
	if fontH > 58 {
		fontH = 58
	}
	if fontH < 12 {
		fontH = 12
	}
	bw := box.Right - box.Left
	var font uintptr
	var size SIZE
	for fontH >= 12 {
		font = advancedProfileFontHandle(fontChoice, fontH, 700)
		if font == 0 {
			return RECT{}
		}
		size = textPixelSize(hdc, font, text)
		if size.Cx <= bw-8 && size.Cy <= maxH-4 {
			break
		}
		fontH -= 2
	}
	if font == 0 || size.Cx <= 0 || size.Cy <= 0 || size.Cx > bw {
		return RECT{}
	}
	x := box.Left + (bw-size.Cx)/2
	y := box.Top + (maxH-size.Cy)/2
	drawGradientProfileTextStyled(hdc, x, y, text, p1, p2, font, vertical, shadow, shadowC)
	return RECT{x, y, x + size.Cx, y + size.Cy}
}

// drawAdvancedProfileTextAt renders a decorative name from the same top-left
// origin used by the normal Global Profile name. Font choice may change glyph
// shape, but never the name column position or baseline lane. The face shrinks
// only when necessary to stay inside maxRight.
func drawAdvancedProfileTextAt(hdc uintptr, x, y, maxRight int32, text string, fontChoice, p1, p2 int, vertical, shadow bool, shadowC int) bool {
	if fontChoice <= 0 || fontChoice > len(advancedFontFaces) || text == "" || maxRight <= x {
		return false
	}
	fontH := int32(30)
	var font uintptr
	var size SIZE
	for fontH >= 12 {
		font = advancedProfileFontHandle(fontChoice, fontH, 700)
		if font == 0 {
			return false
		}
		size = textPixelSize(hdc, font, text)
		if size.Cx > 0 && size.Cy > 0 && x+size.Cx <= maxRight {
			break
		}
		fontH -= 2
	}
	if font == 0 || size.Cx <= 0 || size.Cy <= 0 || x+size.Cx > maxRight {
		return false
	}
	drawGradientProfileTextStyled(hdc, x, y, text, p1, p2, font, vertical, shadow, shadowC)
	return true
}
