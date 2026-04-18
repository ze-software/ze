// Design: docs/architecture/core-design.md -- nftables backend stub for non-Linux

//go:build !linux

package firewallnft

import (
	"fmt"
	"runtime"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

func newBackend() (firewall.Backend, error) {
	return nil, fmt.Errorf("firewallnft: not supported on %s", runtime.GOOS)
}
