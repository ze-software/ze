//go:build !linux

// Design: docs/labs/pppoe-interop.md -- raw sockets execute only in Linux peers.
package testvpppppoe

import (
	"context"
	"errors"
)

func runControl(context.Context, string, string) (controlResult, error) {
	return controlResult{}, errors.New("the control helper requires Linux; use the QEMU docker-lab route")
}

func injectFrames(string, string, bool) error {
	return errors.New("the wire injection helper requires Linux; use the QEMU docker-lab route")
}
