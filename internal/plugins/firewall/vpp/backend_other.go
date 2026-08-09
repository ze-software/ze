// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- VPP firewall backend stub for non-Linux

//go:build !linux

package firewallvpp

import (
	"fmt"
	"runtime"

	"github.com/ze-software/ze/internal/component/firewall"
)

func newBackend() (firewall.Backend, error) {
	return nil, fmt.Errorf("firewallvpp: not supported on %s", runtime.GOOS)
}
