//go:build linux

// Design: docs/functional-tests.md -- isolated route-policy command authority.

package fixture

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func importCommandAttributes() (*syscall.SysProcAttr, error) {
	return &syscall.SysProcAttr{AmbientCaps: []uintptr{unix.CAP_NET_ADMIN}}, nil
}
