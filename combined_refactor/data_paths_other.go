//go:build !android

package main

import "os"

func platformDataDir() (string, bool, error) {
	return "", false, nil
}

func chmodPrivateFile(f *os.File) error {
	return f.Chmod(0600)
}

func chmodAtomicFile(name string, perm os.FileMode) error {
	return os.Chmod(name, perm)
}
