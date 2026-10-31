package main

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWebStateLockedDestinationRetainsBytesAndMemory(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows locked replacement")
	}
	s := &webStateStore{path: filepath.Join(t.TempDir(), "web-state.private")}
	if e := statePatch(t, s, 0, `{"theme":"dark"}`); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(s.path)
	name, _ := windows.UTF16PtrFromString(s.path)
	h, e := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer windows.CloseHandle(h)
	if e := statePatch(t, s, 1, `{"theme":"light"}`); e == nil {
		t.Fatal("locked write succeeded")
	}
	after, _ := os.ReadFile(s.path)
	if string(before) != string(after) || s.snapshot().Revision != 1 || string(s.snapshot().Sections["theme"]) != `"dark"` {
		t.Fatal("failed write destroyed state")
	}
}
