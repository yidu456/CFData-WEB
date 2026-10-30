//go:build !windows

package main

import "errors"

const huaweiPrivateEncryptionSupported = false

func protectHuaweiPrivate(data []byte) ([]byte, error) {
	return nil, errors.New("encrypted Huawei configuration saving is unsupported on this platform; use memory-only Apply")
}

// Legacy plaintext remains readable, but new persistent writes must fail closed.
func unprotectHuaweiPrivate(data []byte) ([]byte, error) { return data, nil }
