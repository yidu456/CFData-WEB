//go:build !windows

package main

func protectHuaweiPrivate(data []byte) ([]byte, error)   { return data, nil }
func unprotectHuaweiPrivate(data []byte) ([]byte, error) { return data, nil }
