package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var programExecutable = os.Executable

func programDataDir() (string, error) {
	exe, err := programExecutable()
	if err != nil || strings.TrimSpace(exe) == "" || !filepath.IsAbs(exe) {
		return "", errors.New("executable data directory unavailable")
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable data directory: %w", err)
	}
	return filepath.Dir(exe), nil
}

func defaultDataPath(name string) (string, error) {
	if strings.TrimSpace(name) == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\:`) {
		return "", errors.New("default data filename must be a basename")
	}
	dir, err := programDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
