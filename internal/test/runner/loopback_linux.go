// Design: docs/architecture/testing/ci-format.md -- multi-peer loopback alias setup
//
// Linux routes the entire 127.0.0.0/8 subnet to the loopback interface
// automatically. No alias setup is needed.

//go:build linux

package runner

import (
	"fmt"
	"net"
)

// ensureLoopbackAlias is a no-op on Linux. The 127.0.0.0/8 subnet is
// routed to lo by default, so any 127.x.x.x address can be bound without
// explicit alias configuration. Input validation is shared across platforms.
func ensureLoopbackAlias(ip net.IP) error {
	ip4 := ip.To4()
	if ip4 == nil {
		return fmt.Errorf("ensureLoopbackAlias: %v is not IPv4", ip)
	}
	if ip4[0] != 127 {
		return fmt.Errorf("ensureLoopbackAlias: %v is not in 127.0.0.0/8", ip)
	}
	return nil
}
