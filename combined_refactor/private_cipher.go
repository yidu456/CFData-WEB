package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

const (
	privateEnvelopeMagic   = "CFDataPrivate\x00"
	privateEnvelopeVersion = byte(1)
)

// This fixed application key prevents casual plaintext inspection, not reverse engineering.
var privateCipherKey = sha256.Sum256([]byte("CFData portable private storage public application key v1"))

const huaweiPrivateEncryptionSupported = true

func protectHuaweiPrivate(plain []byte) ([]byte, error) {
	gcm, err := privateGCM()
	if err != nil {
		return nil, err
	}
	prefix := append([]byte(privateEnvelopeMagic), privateEnvelopeVersion)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(prefix)+len(nonce)+len(plain)+gcm.Overhead())
	out = append(out, prefix...)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plain, prefix), nil
}

func unprotectHuaweiPrivate(data []byte) ([]byte, error) {
	plain, recognized, err := unprotectPortablePrivate(data)
	if recognized {
		return plain, err
	}
	return unprotectLegacyHuaweiPrivate(data)
}

func unprotectHuaweiPrivateFormat(data []byte) ([]byte, string, error) {
	plain, recognized, err := unprotectPortablePrivate(data)
	if recognized {
		if err != nil {
			return nil, "", err
		}
		return plain, "encrypted", nil
	}
	plain, err = unprotectLegacyHuaweiPrivate(data)
	if err != nil {
		return nil, "", err
	}
	return plain, "legacy-dpapi", nil
}

func unprotectPortablePrivate(data []byte) ([]byte, bool, error) {
	magic := []byte(privateEnvelopeMagic)
	if !bytes.HasPrefix(data, magic) {
		return nil, false, errors.New("unrecognized private data")
	}
	if len(data) <= len(magic) || data[len(magic)] != privateEnvelopeVersion {
		return nil, true, errors.New("unsupported private data version")
	}
	gcm, err := privateGCM()
	if err != nil {
		return nil, true, err
	}
	prefixLen := len(magic) + 1
	if len(data) < prefixLen+gcm.NonceSize()+gcm.Overhead() {
		return nil, true, errors.New("invalid private data")
	}
	prefix := data[:prefixLen]
	nonce := data[prefixLen : prefixLen+gcm.NonceSize()]
	plain, err := gcm.Open(nil, nonce, data[prefixLen+gcm.NonceSize():], prefix)
	if err != nil {
		return nil, true, errors.New("private data authentication failed")
	}
	return plain, true, nil
}

func privateGCM() (cipher.AEAD, error) {
	block, err := aes.NewCipher(privateCipherKey[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
