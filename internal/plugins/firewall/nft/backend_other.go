// Design: docs/architecture/core-design.md -- nftables backend stub for non-Linux

//go:build !linux

package firewallnft

import (
	"fmt"
	"runtime"

	"github.com/ze-software/ze/internal/component/firewall"
)

func newBackend() (firewall.Backend, error) {
	return nil, fmt.Errorf("firewallnft: not supported on %s", runtime.GOOS)
}
