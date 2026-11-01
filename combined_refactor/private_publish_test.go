package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExclusivePrivatePublicationPreservesSourceAndExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "synced-private.tmp"), filepath.Join(dir, "web-state.private")
	ciphertext, err := protectHuaweiPrivate([]byte(`{"theme":"dark"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, ciphertext, 0600); err != nil {
		t.Fatal(err)
	}
	if err := copyNewPrivateFile(source, destination); err != nil {
		t.Fatal("sdcardfs-compatible private publication failed", err)
	}
	if got, err := os.ReadFile(destination); err != nil || !bytes.Equal(got, ciphertext) {
		t.Fatal("ciphertext changed while publishing")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("exclusive publication removed its source")
	}
	if err := os.WriteFile(source, []byte("must not overwrite"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := copyNewPrivateFile(source, destination); !os.IsExist(err) {
		t.Fatal("existing destination was not protected", err)
	}
	if got, _ := os.ReadFile(destination); !bytes.Equal(got, ciphertext) {
		t.Fatal("existing private state was overwritten")
	}
}

func TestExclusivePrivatePublicationRemovesFailedNewCopies(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "private")
	for _, source := range []string{filepath.Join(dir, "absent"), dir} {
		if err := copyNewPrivateFile(source, destination); err == nil {
			t.Fatal("invalid source was published")
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Fatal("failed new publication left an incomplete destination")
		}
	}
}
