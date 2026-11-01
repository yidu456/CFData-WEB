package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func platformDataDir() (string, bool, error) {
	dir := os.Getenv("CFDATA_DATA_DIR")
	if dir == "" {
		dir = "/storage/emulated/0/Download/cfdata-web"
	}
	if strings.TrimSpace(dir) == "" || !filepath.IsAbs(dir) {
		return "", true, errors.New("Android data directory must be absolute")
	}
	return filepath.Clean(dir), true, nil
}

func androidModeError(name string, err error) error {
	if err == nil || (!errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.EOPNOTSUPP) && !errors.Is(err, syscall.ENOSYS)) {
		return err
	}
	dir, _, dirErr := platformDataDir()
	rel, relErr := filepath.Rel(dir, name)
	if dirErr != nil || relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return err
	}
	// Emulated storage fixes file modes; successful writes still require Android storage permission.
	return nil
}

func chmodPrivateFile(f *os.File) error {
	return androidModeError(f.Name(), f.Chmod(0600))
}

func chmodAtomicFile(name string, perm os.FileMode) error {
	return androidModeError(name, os.Chmod(name, perm))
}
