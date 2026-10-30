package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
)

type huaweiMigrationReceipt struct {
	source, destination         [32]byte
	sourceInfo, destinationInfo os.FileInfo
}

func privatePathStatus(path string) string {
	if path == "" {
		return "unavailable"
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "absent"
	}
	if err != nil || !info.Mode().IsRegular() {
		return "unavailable"
	}
	return "available"
}

func readHuaweiPrivateConfig(path string) (huaweiConfigRequest, []byte, os.FileInfo, error) {
	var c huaweiConfigRequest
	info, err := os.Lstat(path)
	if err != nil {
		return c, nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16384 {
		return c, nil, nil, errors.New("invalid private configuration file")
	}
	f, err := os.Open(path)
	if err != nil {
		return c, nil, nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return c, nil, nil, errors.New("private configuration changed")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(raw) > 16384 {
		return c, nil, nil, errors.New("private configuration read failed")
	}
	plain, err := unprotectHuaweiPrivate(raw)
	if err != nil {
		return c, nil, nil, errors.New("private configuration decryption failed")
	}
	if json.Unmarshal(plain, &c) != nil || validateHuaweiConfig(c) != nil {
		return c, nil, nil, errors.New("invalid private configuration")
	}
	return c, raw, opened, nil
}

func (s *huaweiDNSStore) migrateLegacy() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !huaweiPrivateEncryptionSupported {
		return errors.New("encrypted legacy migration unsupported on this platform")
	}
	if s.path == "" || s.legacyPath == "" || privatePathStatus(s.path) != "absent" {
		return errors.New("legacy migration requires an absent executable-local destination")
	}
	c, source, sourceInfo, err := readHuaweiPrivateConfig(s.legacyPath)
	if err != nil {
		return errors.New("legacy configuration could not be decrypted and validated")
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return errors.New("legacy configuration encoding failed")
	}
	// A hard link publishes the synced temporary file only if the destination is absent.
	if err := writeHuaweiPrivate(s.path, raw, false); err != nil {
		return errors.New("legacy migration save failed; source preserved")
	}
	loaded, destination, destinationInfo, err := readHuaweiPrivateConfig(s.path)
	if err != nil || loaded != c {
		return errors.New("legacy migration verification failed; source preserved")
	}
	receipt := &huaweiMigrationReceipt{sha256.Sum256(source), sha256.Sum256(destination), sourceInfo, destinationInfo}
	if !s.migrationFilesMatchLocked(receipt) {
		return errors.New("legacy migration files changed; source preserved")
	}
	s.config, s.savedConfig = loaded, loaded
	s.savedConfig.Persist = false
	s.privateFileStatus = "encrypted"
	s.revision++
	s.legacyMigration = receipt
	return nil
}

func (s *huaweiDNSStore) migrationFilesMatchLocked(r *huaweiMigrationReceipt) bool {
	if r == nil || !huaweiPrivateEncryptionSupported {
		return false
	}
	_, source, sourceInfo, err := readHuaweiPrivateConfig(s.legacyPath)
	if err != nil || !os.SameFile(r.sourceInfo, sourceInfo) || !r.sourceInfo.ModTime().Equal(sourceInfo.ModTime()) || sha256.Sum256(source) != r.source {
		return false
	}
	_, destination, destinationInfo, err := readHuaweiPrivateConfig(s.path)
	return err == nil && os.SameFile(r.destinationInfo, destinationInfo) && r.destinationInfo.ModTime().Equal(destinationInfo.ModTime()) && sha256.Sum256(destination) == r.destination
}

func (s *huaweiDNSStore) legacyUnchangedLocked() bool {
	return s.migrationFilesMatchLocked(s.legacyMigration)
}

func (s *huaweiDNSStore) cleanupLegacy() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.legacyUnchangedLocked() {
		return errors.New("legacy cleanup requires a successful migration and unchanged verified files")
	}
	if err := os.Remove(s.legacyPath); err != nil {
		return errors.New("legacy cleanup failed; source preserved")
	}
	s.legacyMigration = nil
	return nil
}
