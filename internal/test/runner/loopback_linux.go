// Design: docs/architecture/testing/ci-format.md -- multi-peer loopback alias setup
//
// Linux routes the entire 127.0.0.0/8 subnet to the loopback interface
// automatically. No alias setup is needed.

//go:build linux

package runner

import "net"

// ensureLoopbackAlias is a no-op on Linux. The 127.0.0.0/8 subnet is
// routed to lo by default, so any 127.x.x.x address can be bound without
// explicit alias configuration.
func ensureLoopbackAlias(_ net.IP) error {
	return nil
}
