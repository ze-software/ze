// Design: docs/architecture/l2tp/bng-5-pppoe.md -- Linux LCP control prerequisite
// Related: prepare_linux.go -- namespace-specific CP link readback

//go:build !linux

package vpp

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/iface"
)

func readControlLink(context.Context, iface.LCPPair) (controlLink, error) {
	return controlLink{}, errors.New("VPP PPPoE access requires Linux LCP control devices")
}
