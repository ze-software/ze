// Design: docs/architecture/testing/ci-format.md -- multi-peer loopback alias setup
//
// OpenBSD binds only 127.0.0.1 on lo0 and routes the rest of 127.0.0.0/8 to a
// reject route, so multi-peer tests need aliases as on macOS. This file probes
// and never adds: OpenBSD has removed the raw syscall(2) entry the darwin and
// FreeBSD ioctl relies on, and an unprivileged run could not add it anyway. The
// error names the command that adds the address.

package runner

import (
	"fmt"
	"net"
)

// ensureLoopbackAlias reports whether ip is usable on lo0, and never changes the
// interface. The caller fails the test on any error: a missing address surfaces
// later as a bind failure or a whole-test timeout with nothing pointing at the
// cause.
func ensureLoopbackAlias(ip net.IP) error {
	ip4 := ip.To4()
	if ip4 == nil {
		if loopbackBindable(ip) {
			return nil
		}
		return loopbackMissing(ip)
	}
	if ip4[0] != 127 {
		return fmt.Errorf("ensureLoopbackAlias: %v is not in 127.0.0.0/8", ip)
	}
	if loopbackBindable(ip) {
		return nil
	}
	return fmt.Errorf("ensureLoopbackAlias: %v is not on lo0 and this runner does not add"+
		" addresses on OpenBSD: add it as root with `ifconfig lo0 alias %v`", ip, ip)
}

// loopbackIPv6AddCommand is the command an operator runs to put an IPv6 address
// on the loopback interface. /128 keeps it a host address, so no route toward
// the rest of its block is created.
func loopbackIPv6AddCommand(ip net.IP) string {
	return "ifconfig lo0 inet6 " + ip.String() + "/128 alias"
}
