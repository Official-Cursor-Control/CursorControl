package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var atomicWriteMu sync.Mutex

// atomicWriteFile prevents partially-written progression/config files if the
// process exits during a save. The temporary file is created beside the target
// so rename stays on the same filesystem.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	atomicWriteMu.Lock()
	defer atomicWriteMu.Unlock()
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	writeTemp := func(prefix string, payload []byte) (string, error) {
		tmp, err := os.CreateTemp(dir, prefix)
		if err != nil {
			return "", err
		}
		name := tmp.Name()
		ok := false
		defer func() {
			_ = tmp.Close()
			if !ok {
				_ = os.Remove(name)
			}
		}()
		if _, err = tmp.Write(payload); err != nil {
			return "", err
		}
		if err = tmp.Sync(); err != nil {
			return "", err
		}
		if err = tmp.Chmod(perm); err != nil {
			return "", err
		}
		if err = tmp.Close(); err != nil {
			return "", err
		}
		ok = true
		return name, nil
	}

	// Preserve the previous complete file as a last-known-good recovery point.
	// This backup is refreshed before the live path is replaced, so a crash during
	// backup creation still leaves the current live file untouched.
	backup := path + ".bak"
	if old, err := os.ReadFile(path); err == nil && len(old) > 0 {
		backupTmp, tmpErr := writeTemp("."+filepath.Base(path)+".bak.tmp-*", old)
		if tmpErr == nil {
			if replaceErr := atomicReplaceFile(backupTmp, backup); replaceErr != nil {
				_ = os.Remove(backupTmp)
			} else {
				_ = os.Remove(backupTmp)
			}
		}
	}

	tmpName, err := writeTemp("."+filepath.Base(path)+".tmp-*", data)
	if err != nil {
		return err
	}
	defer os.Remove(tmpName)
	if err = atomicReplaceFile(tmpName, path); err != nil {
		return err
	}
	return nil
}

func readJSONWithRecovery(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(data, dst); err == nil {
			return nil
		}
	}
	backup := path + ".bak"
	if b, backupErr := os.ReadFile(backup); backupErr == nil {
		if backupErr = json.Unmarshal(b, dst); backupErr == nil {
			_ = atomicWriteFile(path, b, 0644)
			return nil
		}
	}
	return err
}
