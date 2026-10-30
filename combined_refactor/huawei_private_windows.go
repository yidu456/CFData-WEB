package main

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

const huaweiPrivateEncryptionSupported = true

func protectHuaweiPrivate(data []byte) ([]byte, error)   { return huaweiDPAPI(data, true) }
func unprotectHuaweiPrivate(data []byte) ([]byte, error) { return huaweiDPAPI(data, false) }
func huaweiDPAPI(data []byte, encrypt bool) ([]byte, error) {
	input := windows.DataBlob{Size: uint32(len(data))}
	if len(data) > 0 {
		input.Data = &data[0]
	}
	var output windows.DataBlob
	var err error
	// No machine scope: only the current Windows user can decrypt this file.
	if encrypt {
		err = windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	} else {
		err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return append([]byte(nil), unsafe.Slice(output.Data, int(output.Size))...), nil
}
