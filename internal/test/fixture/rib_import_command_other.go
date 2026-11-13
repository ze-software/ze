//go:build !linux

// Design: docs/functional-tests.md -- route-policy kernel fixtures require Linux.

package fixture

import (
	"errors"
	"syscall"
)

func importCommandAttributes() (*syscall.SysProcAttr, error) {
	return nil, errors.New("route-policy kernel fixture commands require Linux; run this carrier through ./le test qemu ci test <ci-path>")
}
