//go:build !windows

package main

import "testing"

func TestHuaweiUnsupportedProtectFailsClosedAndLegacyReadRemains(t *testing.T) {
	plain := []byte(`{"ak":"legacy-ak","sk":"legacy-sk","region":"cn-north-4","projectId":"0123456789abcdef0123456789abcdef"}`)
	protected, err := protectHuaweiPrivate(plain)
	if err == nil || len(protected) != 0 {
		t.Fatal("unsupported platform returned plaintext as encrypted data")
	}
	legacy, err := unprotectHuaweiPrivate(plain)
	if err != nil || string(legacy) != string(plain) {
		t.Fatal("legacy plaintext read compatibility lost")
	}
}
