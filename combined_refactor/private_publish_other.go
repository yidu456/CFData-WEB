//go:build !android

package main

import "os"

func publishNewPrivateFile(source, destination string) error {
	return os.Link(source, destination)
}
