// Design: docs/architecture/core-design.md -- sysctl no-op backend
// Overview: backend.go -- backend interface

//go:build !linux && !darwin

package sysctl

import (
	"errors"
)

var errSysctlNotSupportedOnThisPlatform = errors.New("sysctl: not supported on this platform")

type otherBackend struct{}

func newBackend() backend {
	return &otherBackend{}
}

func (b *otherBackend) read(key string) (string, error) {
	return "", errSysctlNotSupportedOnThisPlatform
}

func (b *otherBackend) write(key, value string) error {
	return errSysctlNotSupportedOnThisPlatform
}
