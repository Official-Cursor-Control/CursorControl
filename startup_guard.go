package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
	"unsafe"
)

func startupLogPath() string {
	exe, err := os.Executable()
	if err == nil && exe != "" {
		return filepath.Join(filepath.Dir(exe), "startup_error.log")
	}
	return "startup_error.log"
}

func writeStartupFailure(v any) {
	body := fmt.Sprintf("Cursor Control %s startup failure\nTime: %s\nError: %v\n\n%s\n",
		clientBuildVersion, time.Now().Format(time.RFC3339), v, string(debug.Stack()))
	_ = os.WriteFile(startupLogPath(), []byte(body), 0644)
	title := utf16ptr("Cursor Control - Startup Error")
	msg := utf16ptr("Cursor Control could not start. A startup_error.log file has been created next to the game executable.")
	messageBoxW.Call(0, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), MB_ICONWARNING)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			writeStartupFailure(r)
		}
	}()
	runGame()
}
