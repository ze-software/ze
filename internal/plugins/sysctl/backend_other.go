// Design: docs/architecture/core-design.md -- sysctl no-op backend
// Overview: backend.go -- backend interface

//go:build !linux && !darwin

package sysctl

import "fmt"

type otherBackend struct{}

func newBackend() backend {
	return &otherBackend{}
}

func (b *otherBackend) read(key string) (string, error) {
	return "", fmt.Errorf("sysctl: not supported on this platform")
}

func (b *otherBackend) write(key, value string) error {
	return fmt.Errorf("sysctl: not supported on this platform")
}
