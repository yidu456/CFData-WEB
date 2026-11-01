package main

import (
	"errors"

	"golang.org/x/sys/unix"
)

func publishNewPrivateFile(source, destination string) error {
	// Android emulated storage has no hard links; preserve atomic no-overwrite publication.
	err := unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EOPNOTSUPP) {
		// shortcut: old sdcardfs needs exclusive copy; an interrupted first migration may leave an incomplete new file.
		return copyNewPrivateFile(source, destination)
	}
	return err
}
