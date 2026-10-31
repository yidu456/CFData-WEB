package main

import (
	"bytes"
	"testing"
)

func TestPortablePrivateCipherRoundTripAndRandomNonce(t *testing.T) {
	secret := []byte("portable private fixture")
	first, err := protectHuaweiPrivate(secret)
	if err != nil {
		t.Fatal(err)
	}
	second, err := protectHuaweiPrivate(secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, secret) || bytes.Equal(first, second) {
		t.Fatal("plaintext or reused nonce")
	}
	plain, err := unprotectHuaweiPrivate(first)
	if err != nil || !bytes.Equal(plain, secret) {
		t.Fatal("portable ciphertext did not round trip", err)
	}
}

func TestPortablePrivateCipherEmptyPayload(t *testing.T) {
	ciphertext, err := protectHuaweiPrivate(nil)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := unprotectHuaweiPrivate(ciphertext)
	if err != nil || len(plain) != 0 {
		t.Fatal("empty payload did not round trip", err)
	}
}

func TestPortablePrivateCipherRejectsDamageAndPlaintext(t *testing.T) {
	ciphertext, err := protectHuaweiPrivate([]byte("authenticated fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(ciphertext, []byte("CFDataPrivate\x00\x01")) {
		t.Fatal("portable version prefix missing")
	}
	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 1
	unknownVersion := append([]byte(nil), ciphertext...)
	unknownVersion[len("CFDataPrivate\x00")]++
	for name, raw := range map[string][]byte{
		"empty":           nil,
		"truncated":       ciphertext[:len(ciphertext)-1],
		"prefix-only":     ciphertext[:len("CFDataPrivate\x00\x01")],
		"tampered":        tampered,
		"unknown-version": unknownVersion,
		"plaintext":       []byte(`{"version":1,"secret":"fixture"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if plain, err := unprotectHuaweiPrivate(raw); err == nil || len(plain) != 0 {
				t.Fatal("invalid private data accepted")
			}
		})
	}
}
