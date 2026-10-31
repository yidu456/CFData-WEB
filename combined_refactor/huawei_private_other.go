//go:build !windows

package main

import "errors"

const legacyHuaweiPlaintextSupported = true

func unprotectLegacyHuaweiPrivate([]byte) ([]byte, error) {
	return nil, errors.New("legacy DPAPI private data is unavailable on this platform")
}
