//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

var moveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

// atomicReplaceFile replaces dst without first removing it. MOVEFILE_WRITE_THROUGH
// asks Windows to flush the rename metadata before returning, which is the safest
// available primitive for local progression/config saves without extra packages.
func atomicReplaceFile(src, dst string) error {
	src16, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	dst16, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	r, _, callErr := moveFileExW.Call(
		uintptr(unsafe.Pointer(src16)),
		uintptr(unsafe.Pointer(dst16)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if r == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("MoveFileExW failed")
	}
	return nil
}
