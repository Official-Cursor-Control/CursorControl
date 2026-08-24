//go:build windows

package main

import (
	"archive/zip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Existing Cursor Control artwork; no launcher-specific generated art.
//
//go:embed assets/launcher_bg.jpg
var launcherBackground []byte

//go:embed assets/app_icon.png
var launcherIcon []byte

const (
	launcherVersion    = "1.0.2"
	defaultManifestURL = "https://raw.githubusercontent.com/Official-Cursor-Control/CursorControl/main/release/manifest.json"

	CS_HREDRAW         = 0x0002
	CS_VREDRAW         = 0x0001
	IDC_ARROW          = 32512
	WS_OVERLAPPED      = 0x00000000
	WS_CAPTION         = 0x00C00000
	WS_SYSMENU         = 0x00080000
	WS_MINIMIZEBOX     = 0x00020000
	WS_VISIBLE         = 0x10000000
	CW_USEDEFAULT      = 0x80000000
	SW_SHOW            = 5
	SW_SHOWNORMAL      = 1
	WM_DESTROY         = 0x0002
	WM_PAINT           = 0x000F
	WM_ERASEBKGND      = 0x0014
	WM_LBUTTONDOWN     = 0x0201
	WM_MOUSEMOVE       = 0x0200
	WM_SETCURSOR       = 0x0020
	WM_TIMER           = 0x0113
	WM_APP             = 0x8000
	WM_APP_REFRESH     = WM_APP + 1
	WM_APP_TASKDONE    = WM_APP + 2
	WM_APP_PROGRESS    = WM_APP + 3
	PM_REMOVE          = 0x0001
	MB_OK              = 0x00000000
	MB_ICONERROR       = 0x00000010
	MB_ICONINFORMATION = 0x00000040
	MB_YESNO           = 0x00000004
	MB_ICONQUESTION    = 0x00000020
	IDYES              = 6
	TRANSPARENT        = 1
	SRCCOPY            = 0x00CC0020
	DIB_RGB_COLORS     = 0
	BI_RGB             = 0
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procBeginPaint       = user32.NewProc("BeginPaint")
	procEndPaint         = user32.NewProc("EndPaint")
	procInvalidateRect   = user32.NewProc("InvalidateRect")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procSetCursor        = user32.NewProc("SetCursor")
	procMessageBoxW      = user32.NewProc("MessageBoxW")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procSetTimer         = user32.NewProc("SetTimer")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procFillRect         = user32.NewProc("FillRect")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procTextOutW         = gdi32.NewProc("TextOutW")
	procDrawTextW        = user32.NewProc("DrawTextW")
	procStretchDIBits    = gdi32.NewProc("StretchDIBits")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procMoveToEx         = gdi32.NewProc("MoveToEx")
	procLineTo           = gdi32.NewProc("LineTo")

	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}
type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}
type RGBQUAD struct{ B, G, R, A byte }
type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]RGBQUAD
}

type LauncherConfig struct {
	ManifestURL string `json:"manifest_url"`
}
type ReleaseManifest struct {
	Version       string     `json:"version"`
	PackageURL    string     `json:"package_url"`
	PackageSHA256 string     `json:"package_sha256"`
	GameExeSHA256 string     `json:"game_exe_sha256,omitempty"`
	PackageBytes  int64      `json:"package_bytes,omitempty"`
	News          []NewsItem `json:"news,omitempty"`
}
type NewsItem struct {
	Version string `json:"version"`
	Title   string `json:"title"`
	Detail  string `json:"detail"`
}
type InstallState struct {
	Version       string `json:"version"`
	InstalledAt   string `json:"installed_at"`
	PackageSHA256 string `json:"package_sha256"`
}

type AppState struct {
	mu               sync.Mutex
	hwnd             uintptr
	manifest         *ReleaseManifest
	installedVersion string
	status           string
	substatus        string
	progress         float64
	busy             bool
	hoverPlay        bool
	hoverRepair      bool
	lastErr          string
}

var app = &AppState{status: "STARTING LAUNCHER", substatus: "Checking local installation..."}

var bgImg, iconImg *rgbaImage

type rgbaImage struct {
	W, H int
	Pix  []byte
}

var playRect = RECT{70, 545, 510, 625}
var repairRect = RECT{540, 545, 800, 625}

var fallbackNews = []NewsItem{
	{"v462", "Stability & Windows hardening", "Safer atomic saves, main-thread cloud state application, GUI subsystem and embedded Windows icon fixes."},
	{"v461", "Expedition rewards + materials", "Six Ship Module materials added with icons; Expedition Complete now clearly displays Starbits, NAV Data and material rewards."},
	{"v460", "Void Serpent combo feedback", "Five-hit Hunt nodes now use escalating hit sounds and a flashing remaining-hit counter beside the active node."},
	{"v459", "Ship Module artwork", "All 72 Ship Module pieces now use their authored sprites instead of generic placeholders."},
	{"v458", "Void Serpent five-hit Hunt", "Each Hunt node now requires five correct hits before destruction while preserving the three-segment safety pocket."},
	{"v457", "Intro first-frame fix", "The publisher intro now owns the startup splash so the game window cannot appear before the intro."},
	{"v456", "Startup initialization stability", "Preserved the corrected startup/game initialization path used by the later splash-window implementation."},
	{"v455", "Publisher video playback", "The supplied KongGames MP4 is attempted unchanged first, with the compatibility fallback retained for safer playback."},
	{"v454", "Startup compatibility", "Established the working game initialization path preserved by the subsequent intro revisions."},
	{"v453", "Weekly reset boundary", "Weekly and competition reset timing corrected to Monday 00:00 Asia/Ho_Chi_Minh; Sunday remains active."},
}

func main() {
	runtime.LockOSThread()
	bgImg = decodeImage(launcherBackground)
	iconImg = decodeImage(launcherIcon)
	hInstance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	cls, _ := syscall.UTF16PtrFromString("CursorControlLauncherWindow")
	title, _ := syscall.UTF16PtrFromString("Cursor Control Launcher")
	cursor, _, _ := procLoadCursorW.Call(0, IDC_ARROW)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), Style: CS_HREDRAW | CS_VREDRAW, LpfnWndProc: syscall.NewCallback(wndProc), HInstance: hInstance, HCursor: cursor, LpszClassName: cls}
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		panic("RegisterClassExW failed")
	}
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX|WS_VISIBLE, CW_USEDEFAULT, CW_USEDEFAULT, 1120, 735, 0, 0, hInstance, 0)
	if hwnd == 0 {
		panic("CreateWindowExW failed")
	}
	app.hwnd = hwnd
	procShowWindow.Call(hwnd, SW_SHOW)
	procUpdateWindow.Call(hwnd)
	go refreshManifest()
	var msg MSG
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	case WM_SETCURSOR:
		cur, _, _ := procLoadCursorW.Call(0, IDC_ARROW)
		procSetCursor.Call(cur)
		return 1
	case WM_ERASEBKGND:
		// Full-frame painting in WM_PAINT means erasing first only creates visible flicker.
		return 1
	case WM_APP_REFRESH, WM_APP_TASKDONE, WM_APP_PROGRESS:
		procInvalidateRect.Call(hwnd, 0, 0)
		return 0
	case WM_MOUSEMOVE:
		x, y := mouseXY(lParam)
		hp := ptIn(playRect, x, y)
		hr := ptIn(repairRect, x, y)
		app.mu.Lock()
		changed := hp != app.hoverPlay || hr != app.hoverRepair
		app.hoverPlay = hp
		app.hoverRepair = hr
		app.mu.Unlock()
		if changed {
			procInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case WM_LBUTTONDOWN:
		x, y := mouseXY(lParam)
		if ptIn(playRect, x, y) {
			onPlay()
		} else if ptIn(repairRect, x, y) {
			onRepair()
		}
		return 0
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		paint(hdc)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func paint(hdc uintptr) {
	drawImage(hdc, bgImg, 0, 0, 1100, 700)
	// dark HUD veil
	fill(hdc, RECT{0, 0, 1100, 700}, rgb(3, 8, 18))
	// Repaint selected background areas with image so the source art remains visible but subdued.
	drawImageAlphaApprox(hdc, bgImg, 0, 0, 1100, 700)
	fill(hdc, RECT{0, 0, 1100, 92}, rgb(5, 12, 26))
	fill(hdc, RECT{0, 92, 1100, 96}, rgb(255, 116, 34))
	if iconImg != nil {
		drawImage(hdc, iconImg, 24, 15, 62, 62)
	}
	text(hdc, 105, 22, "CURSOR", 32, 700, rgb(72, 205, 255))
	text(hdc, 248, 22, "CONTROL", 32, 700, rgb(255, 125, 38))
	text(hdc, 105, 58, "LAUNCHER // BOOTSTRAP v"+launcherVersion, 12, 600, rgb(125, 160, 190))

	// Status card
	fill(hdc, RECT{55, 120, 815, 205}, rgb(7, 18, 37))
	outline(hdc, RECT{55, 120, 815, 205}, rgb(36, 128, 176))
	app.mu.Lock()
	st := app.status
	sub := app.substatus
	p := app.progress
	busy := app.busy
	installed := app.installedVersion
	mf := app.manifest
	hp := app.hoverPlay
	hr := app.hoverRepair
	app.mu.Unlock()
	text(hdc, 75, 138, st, 18, 700, rgb(233, 245, 255))
	text(hdc, 75, 167, sub, 13, 400, rgb(139, 177, 204))
	if busy {
		fill(hdc, RECT{75, 191, 795, 198}, rgb(16, 42, 62))
		fill(hdc, RECT{75, 191, 75 + int32(720*p), 198}, rgb(53, 195, 255))
	}

	// Install information
	fill(hdc, RECT{835, 120, 1050, 205}, rgb(7, 18, 37))
	outline(hdc, RECT{835, 120, 1050, 205}, rgb(36, 128, 176))
	text(hdc, 855, 135, "INSTALLATION", 12, 700, rgb(127, 206, 244))
	if installed == "" {
		installed = "NOT INSTALLED"
	}
	text(hdc, 855, 158, installed, 17, 700, rgb(245, 245, 245))
	latest := "HOST PENDING"
	if mf != nil && mf.Version != "" {
		latest = mf.Version
	}
	text(hdc, 855, 183, "LATEST  "+latest, 11, 500, rgb(150, 168, 184))

	// News
	fill(hdc, RECT{55, 228, 1050, 520}, rgb(6, 14, 29))
	outline(hdc, RECT{55, 228, 1050, 520}, rgb(34, 103, 145))
	text(hdc, 75, 245, "LATEST FIXES // LAST 10 VERSIONS", 15, 700, rgb(255, 137, 47))
	news := fallbackNews
	if mf != nil && len(mf.News) >= 10 {
		news = mf.News[:10]
	}
	y := 278
	for i, n := range news {
		if i >= 10 {
			break
		}
		text(hdc, 75, int32(y), n.Version, 12, 700, rgb(67, 201, 255))
		text(hdc, 130, int32(y), n.Title, 12, 700, rgb(235, 241, 247))
		textClip(hdc, 355, int32(y), 675, 19, n.Detail, 11, 400, rgb(135, 159, 179))
		y += 23
	}

	// Buttons
	drawButton(hdc, playRect, "PLAY", hp && !busy, true)
	drawButton(hdc, repairRect, "REPAIR / REINSTALL", hr && !busy, false)
	fill(hdc, RECT{825, 545, 1050, 625}, rgb(8, 18, 34))
	outline(hdc, RECT{825, 545, 1050, 625}, rgb(42, 84, 113))
	text(hdc, 845, 561, "INSTALL PATH", 10, 700, rgb(119, 164, 194))
	textClip(hdc, 845, 582, 185, 30, installRoot(), 10, 400, rgb(205, 215, 223))
	text(hdc, 55, 658, "PLAY automatically installs/updates when a release host is connected. REPAIR forces a clean reinstall and verifies the package hash.", 11, 400, rgb(116, 143, 163))
}

func drawButton(hdc uintptr, r RECT, label string, hover, primary bool) {
	bg := rgb(10, 30, 47)
	border := rgb(54, 173, 222)
	fg := rgb(235, 249, 255)
	if primary {
		bg = rgb(16, 89, 126)
		border = rgb(74, 218, 255)
	}
	if hover {
		if primary {
			bg = rgb(22, 116, 157)
		} else {
			bg = rgb(16, 48, 70)
		}
	}
	fill(hdc, r, bg)
	outline(hdc, r, border)
	outline(hdc, RECT{r.Left + 4, r.Top + 4, r.Right - 4, r.Bottom - 4}, rgb(21, 62, 84))
	size := 20
	if !primary {
		size = 15
	}
	tw := approxTextWidth(label, size)
	x := (r.Left+r.Right)/2 - int32(tw/2)
	y := (r.Top+r.Bottom)/2 - int32(size/2) - 2
	text(hdc, x, y, label, size, 700, fg)
}

func onPlay() {
	app.mu.Lock()
	if app.busy {
		app.mu.Unlock()
		return
	}
	mf := app.manifest
	installed := app.installedVersion
	app.mu.Unlock()
	exe := filepath.Join(gameDir(), "CursorControl.exe")
	if mf == nil {
		if fileExists(exe) {
			launchGame(exe)
			return
		}
		message("Cursor Control Launcher", "The launcher is ready, but the release host has not been connected yet.\n\nWhen we add the GitHub repository URL, PLAY will automatically download and install the game.", MB_OK|MB_ICONINFORMATION)
		return
	}
	if installed != mf.Version || !fileExists(exe) {
		go installOrRepair(false, true)
		return
	}
	launchGame(exe)
}

func onRepair() {
	app.mu.Lock()
	if app.busy {
		app.mu.Unlock()
		return
	}
	mf := app.manifest
	app.mu.Unlock()
	if mf == nil {
		message("Repair / Reinstall", "The repair system is implemented, but it needs the online release manifest before it can download a clean game package.", MB_OK|MB_ICONINFORMATION)
		return
	}
	if message("Repair / Reinstall", "This will download the latest Cursor Control build and replace the current game installation.\n\nYour user data in AppData is not deleted. Continue?", MB_YESNO|MB_ICONQUESTION) != IDYES {
		return
	}
	go installOrRepair(true, false)
}

func refreshManifest() {
	local := readInstallState()
	app.mu.Lock()
	app.installedVersion = local.Version
	app.mu.Unlock()
	manifestURL := configuredManifestURL()
	if manifestURL == "" {
		app.mu.Lock()
		app.status = "LAUNCHER READY // HOST NOT CONNECTED"
		app.substatus = "GitHub release endpoint will be connected later. Local launcher systems are ready."
		app.mu.Unlock()
		post(WM_APP_REFRESH)
		return
	}
	app.mu.Lock()
	app.status = "CHECKING FOR UPDATES"
	app.substatus = "Contacting release manifest..."
	app.mu.Unlock()
	post(WM_APP_REFRESH)
	m, err := fetchManifest(manifestURL)
	if err != nil {
		app.mu.Lock()
		app.status = "OFFLINE // UPDATE CHECK FAILED"
		app.substatus = err.Error()
		app.lastErr = err.Error()
		app.mu.Unlock()
		post(WM_APP_REFRESH)
		return
	}
	app.mu.Lock()
	app.manifest = m
	if local.Version == "" {
		app.status = "READY TO INSTALL"
		app.substatus = "Latest release: " + m.Version + " // Press PLAY to install and launch."
	} else if local.Version != m.Version {
		app.status = "UPDATE AVAILABLE // " + m.Version
		app.substatus = "Installed: " + local.Version + " // Press PLAY to update automatically."
	} else {
		app.status = "READY TO PLAY"
		app.substatus = "Installation is current and verified by launcher state."
	}
	app.mu.Unlock()
	post(WM_APP_REFRESH)
}

func installOrRepair(force, launchAfter bool) {
	app.mu.Lock()
	if app.busy {
		app.mu.Unlock()
		return
	}
	app.busy = true
	mf := app.manifest
	app.progress = 0
	if force {
		app.status = "REPAIRING CURSOR CONTROL"
	} else {
		app.status = "INSTALLING / UPDATING CURSOR CONTROL"
	}
	app.substatus = "Preparing download..."
	app.mu.Unlock()
	post(WM_APP_PROGRESS)
	defer func() { app.mu.Lock(); app.busy = false; app.mu.Unlock(); post(WM_APP_TASKDONE) }()
	if mf == nil {
		setFailure(errors.New("release manifest unavailable"))
		return
	}
	if err := os.MkdirAll(downloadDir(), 0755); err != nil {
		setFailure(err)
		return
	}
	pkg := filepath.Join(downloadDir(), "cursor-control-"+safeName(mf.Version)+".ccpkg")
	appSetSub("Downloading " + mf.Version + "...")
	if err := downloadFile(resolvePackageURL(configuredManifestURL(), mf.PackageURL), pkg, mf.PackageBytes); err != nil {
		setFailure(err)
		return
	}
	appSetSub("Verifying package SHA-256...")
	got, err := sha256File(pkg)
	if err != nil {
		setFailure(err)
		return
	}
	if !equalHash(got, mf.PackageSHA256) {
		setFailure(fmt.Errorf("package verification failed: expected %s, got %s", shortHash(mf.PackageSHA256), shortHash(got)))
		return
	}
	stage := filepath.Join(installRoot(), "_staging")
	os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0755); err != nil {
		setFailure(err)
		return
	}
	appSetSub("Installing game files...")
	if err := unzipSafe(pkg, stage); err != nil {
		setFailure(err)
		return
	}
	root, err := normalizeExtractedRoot(stage)
	if err != nil {
		setFailure(err)
		return
	}
	exe := filepath.Join(root, "CursorControl.exe")
	if !fileExists(exe) {
		setFailure(errors.New("package does not contain CursorControl.exe"))
		return
	}
	if mf.GameExeSHA256 != "" {
		h, e := sha256File(exe)
		if e != nil {
			setFailure(e)
			return
		}
		if !equalHash(h, mf.GameExeSHA256) {
			setFailure(errors.New("game executable verification failed"))
			return
		}
	}
	appSetSub("Activating installation...")
	if err := activateInstall(root); err != nil {
		setFailure(err)
		return
	}
	state := InstallState{Version: mf.Version, InstalledAt: time.Now().UTC().Format(time.RFC3339), PackageSHA256: mf.PackageSHA256}
	if err := writeJSONAtomic(statePath(), state); err != nil {
		setFailure(err)
		return
	}
	os.Remove(pkg)
	app.mu.Lock()
	app.installedVersion = mf.Version
	app.progress = 1
	app.status = "READY TO PLAY"
	app.substatus = mf.Version + " installed successfully."
	app.mu.Unlock()
	post(WM_APP_TASKDONE)
	if launchAfter {
		time.Sleep(300 * time.Millisecond)
		launchGame(filepath.Join(gameDir(), "CursorControl.exe"))
	}
}

func activateInstall(root string) error {
	final := gameDir()
	backup := filepath.Join(installRoot(), "_previous")
	userBackup := filepath.Join(installRoot(), "_userdata_preserve")
	os.RemoveAll(backup)
	os.RemoveAll(userBackup)

	// Cursor Control retains a few local runtime files under Game\data. Repair must
	// never destroy these. Preserve the complete existing data directory and restore
	// it over the clean package after activation. Cloud/user-config saves remain
	// outside Game and are unaffected by the reinstall.
	oldData := filepath.Join(final, "data")
	if fileExists(oldData) {
		if err := copyDir(oldData, userBackup); err != nil {
			return fmt.Errorf("could not preserve local game data: %w", err)
		}
	}

	if fileExists(final) {
		if err := os.Rename(final, backup); err != nil {
			return fmt.Errorf("cannot replace existing game (is Cursor Control running?): %w", err)
		}
	}
	if err := os.Rename(root, final); err != nil {
		if fileExists(backup) {
			_ = os.Rename(backup, final)
		}
		return err
	}
	_ = os.RemoveAll(filepath.Join(installRoot(), "_staging"))

	if fileExists(userBackup) {
		if err := copyDir(userBackup, filepath.Join(final, "data")); err != nil {
			// Roll back rather than accepting an install that lost preserved user data.
			os.RemoveAll(final)
			if fileExists(backup) {
				_ = os.Rename(backup, final)
			}
			return fmt.Errorf("could not restore local game data: %w", err)
		}
	}
	_ = os.RemoveAll(userBackup)
	_ = os.RemoveAll(backup)
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			in.Close()
			return err
		}
		_, cpErr := io.Copy(out, in)
		closeOut := out.Close()
		closeIn := in.Close()
		if cpErr != nil {
			return cpErr
		}
		if closeOut != nil {
			return closeOut
		}
		return closeIn
	})
}

func normalizeExtractedRoot(stage string) (string, error) {
	if fileExists(filepath.Join(stage, "CursorControl.exe")) {
		return stage, nil
	}
	ents, err := os.ReadDir(stage)
	if err != nil {
		return "", err
	}
	dirs := []string{}
	for _, e := range ents {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(stage, e.Name()))
		}
	}
	if len(dirs) == 1 && fileExists(filepath.Join(dirs[0], "CursorControl.exe")) {
		return dirs[0], nil
	}
	return "", errors.New("unable to locate CursorControl.exe in downloaded package")
}

func unzipSafe(zipPath, dst string) error {
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer z.Close()
	cleanDst, _ := filepath.Abs(dst)
	for _, f := range z.File {
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == "." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || filepath.IsAbs(name) {
			return fmt.Errorf("unsafe package path: %s", f.Name)
		}
		target := filepath.Join(cleanDst, name)
		abs, _ := filepath.Abs(target)
		if abs != cleanDst && !strings.HasPrefix(abs, cleanDst+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe package path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			return err
		}
		_, cpErr := io.Copy(out, rc)
		c1 := out.Close()
		c2 := rc.Close()
		if cpErr != nil {
			return cpErr
		}
		if c1 != nil {
			return c1
		}
		if c2 != nil {
			return c2
		}
	}
	return nil
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many HTTP redirects")
			}
			// GitHub release assets redirect to release-assets.githubusercontent.com.
			// Preserve the launcher user agent across each redirect hop.
			req.Header.Set("User-Agent", "CursorControlLauncher/"+launcherVersion)
			return nil
		},
	}
}

func fetchManifest(raw string) (*ReleaseManifest, error) {
	client := newHTTPClient(20 * time.Second)
	req, err := http.NewRequest("GET", raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CursorControlLauncher/"+launcherVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("manifest HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var m ReleaseManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if strings.TrimSpace(m.Version) == "" || strings.TrimSpace(m.PackageURL) == "" || len(strings.TrimSpace(m.PackageSHA256)) < 32 {
		return nil, errors.New("release manifest is incomplete")
	}
	return &m, nil
}

func downloadFile(raw, path string, expected int64) error {
	if raw == "" {
		return errors.New("package URL is empty")
	}
	client := newHTTPClient(0)
	req, err := http.NewRequest("GET", raw, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "CursorControlLauncher/"+launcherVersion)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download HTTP %s", resp.Status)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = expected
	}
	tmp := path + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	buf := make([]byte, 256*1024)
	var done int64
	for {
		n, e := resp.Body.Read(buf)
		if n > 0 {
			if _, w := out.Write(buf[:n]); w != nil {
				return w
			}
			done += int64(n)
			if total > 0 {
				setProgress(float64(done) / float64(total) * 0.72)
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
	}
	if err := out.Sync(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmp, path)
}

func configuredManifestURL() string {
	if v := strings.TrimSpace(os.Getenv("CURSOR_CONTROL_MANIFEST_URL")); v != "" {
		return v
	}
	exe, _ := os.Executable()
	p := filepath.Join(filepath.Dir(exe), "launcher_config.json")
	if b, err := os.ReadFile(p); err == nil {
		var c LauncherConfig
		if json.Unmarshal(b, &c) == nil && strings.TrimSpace(c.ManifestURL) != "" {
			return strings.TrimSpace(c.ManifestURL)
		}
	}
	return defaultManifestURL
}
func resolvePackageURL(manifestURL, packageURL string) string {
	u, err := url.Parse(packageURL)
	if err == nil && u.IsAbs() {
		return u.String()
	}
	base, err := url.Parse(manifestURL)
	if err != nil {
		return packageURL
	}
	rel, err := url.Parse(packageURL)
	if err != nil {
		return packageURL
	}
	return base.ResolveReference(rel).String()
}

func installRoot() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "Cursor Control")
}
func gameDir() string     { return filepath.Join(installRoot(), "Game") }
func downloadDir() string { return filepath.Join(installRoot(), "_downloads") }
func statePath() string   { return filepath.Join(installRoot(), "launcher_state.json") }
func readInstallState() InstallState {
	var s InstallState
	b, err := os.ReadFile(statePath())
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Version != "" && !fileExists(filepath.Join(gameDir(), "CursorControl.exe")) {
		s.Version = ""
	}
	return s
}
func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmp, path)
}

func launchGame(path string) {
	if !fileExists(path) {
		message("Cursor Control", "CursorControl.exe was not found. Use REPAIR / REINSTALL to restore the game.", MB_OK|MB_ICONERROR)
		return
	}
	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		message("Cursor Control", err.Error(), MB_OK|MB_ICONERROR)
	}
}
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func equalHash(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12] + "..."
	}
	return s
}
func safeName(s string) string {
	r := strings.NewReplacer("/", "-", "\\", "-", ":", "-")
	return r.Replace(s)
}
func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
func setFailure(err error) {
	app.mu.Lock()
	app.status = "INSTALLATION FAILED"
	app.substatus = err.Error()
	app.lastErr = err.Error()
	app.progress = 0
	app.mu.Unlock()
	post(WM_APP_TASKDONE)
	message("Cursor Control Launcher", err.Error(), MB_OK|MB_ICONERROR)
}
func appSetSub(s string) { app.mu.Lock(); app.substatus = s; app.mu.Unlock(); post(WM_APP_PROGRESS) }
func setProgress(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	app.mu.Lock()
	app.progress = v
	app.mu.Unlock()
	post(WM_APP_PROGRESS)
}
func post(msg uint32) {
	if app.hwnd != 0 {
		procPostMessageW.Call(app.hwnd, uintptr(msg), 0, 0)
	}
}

func decodeImage(b []byte) *rgbaImage {
	img, _, err := image.Decode(strings.NewReader(string(b)))
	if err != nil {
		return nil
	}
	r := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, r.Min, draw.Src)
	pix := make([]byte, r.Dx()*r.Dy()*4)
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			i := rgba.PixOffset(x, y)
			j := (y*r.Dx() + x) * 4
			pix[j] = rgba.Pix[i+2]
			pix[j+1] = rgba.Pix[i+1]
			pix[j+2] = rgba.Pix[i]
			pix[j+3] = 0
		}
	}
	return &rgbaImage{r.Dx(), r.Dy(), pix}
}
func drawImage(hdc uintptr, img *rgbaImage, x, y, w, h int32) {
	if img == nil {
		return
	}
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{BiSize: uint32(unsafe.Sizeof(BITMAPINFOHEADER{})), BiWidth: int32(img.W), BiHeight: -int32(img.H), BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB}}
	procStretchDIBits.Call(hdc, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, uintptr(img.W), uintptr(img.H), uintptr(unsafe.Pointer(&img.Pix[0])), uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, SRCCOPY)
}

// drawImageAlphaApprox intentionally uses the existing art at full opacity and then dark HUD panels; no generated visual layer.
func drawImageAlphaApprox(hdc uintptr, img *rgbaImage, x, y, w, h int32) {
	drawImage(hdc, img, x, y, w, h)
	fill(hdc, RECT{0, 0, 1100, 700}, rgb(3, 7, 16))
}
func fill(hdc uintptr, r RECT, c uintptr) {
	b, _, _ := procCreateSolidBrush.Call(c)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), b)
	procDeleteObject.Call(b)
}
func outline(hdc uintptr, r RECT, c uintptr) {
	p, _, _ := procCreatePen.Call(0, 1, c)
	old, _, _ := procSelectObject.Call(hdc, p)
	procMoveToEx.Call(hdc, uintptr(r.Left), uintptr(r.Top), 0)
	procLineTo.Call(hdc, uintptr(r.Right-1), uintptr(r.Top))
	procLineTo.Call(hdc, uintptr(r.Right-1), uintptr(r.Bottom-1))
	procLineTo.Call(hdc, uintptr(r.Left), uintptr(r.Bottom-1))
	procLineTo.Call(hdc, uintptr(r.Left), uintptr(r.Top))
	procSelectObject.Call(hdc, old)
	procDeleteObject.Call(p)
}
func text(hdc uintptr, x, y int32, s string, size, weight int, c uintptr) {
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	font, _, _ := procCreateFontW.Call(uintptr(-size), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	old, _, _ := procSelectObject.Call(hdc, font)
	procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, c)
	u, _ := syscall.UTF16FromString(s)
	if len(u) > 1 {
		procTextOutW.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1))
	}
	procSelectObject.Call(hdc, old)
	procDeleteObject.Call(font)
}
func textClip(hdc uintptr, x, y, w, h int32, s string, size, weight int, c uintptr) {
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	font, _, _ := procCreateFontW.Call(uintptr(-size), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	old, _, _ := procSelectObject.Call(hdc, font)
	procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, c)
	u, _ := syscall.UTF16FromString(s)
	r := RECT{x, y, x + w, y + h}
	const DT_SINGLELINE = 0x20
	const DT_END_ELLIPSIS = 0x8000
	const DT_NOPREFIX = 0x800
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_END_ELLIPSIS|DT_NOPREFIX)
	procSelectObject.Call(hdc, old)
	procDeleteObject.Call(font)
}
func approxTextWidth(s string, size int) int { return len([]rune(s)) * size * 55 / 100 }
func rgb(r, g, b byte) uintptr               { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }
func mouseXY(lp uintptr) (int32, int32) {
	return int32(int16(lp & 0xffff)), int32(int16((lp >> 16) & 0xffff))
}
func ptIn(r RECT, x, y int32) bool { return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom }
func message(title, body string, flags uintptr) uintptr {
	t, _ := syscall.UTF16PtrFromString(title)
	b, _ := syscall.UTF16PtrFromString(body)
	r, _, _ := procMessageBoxW.Call(app.hwnd, uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(t)), flags)
	return r
}
