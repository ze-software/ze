// Design: docs/architecture/testing/ci-format.md -- multi-peer loopback alias setup
//
// Linux routes the entire 127.0.0.0/8 subnet to the loopback interface
// automatically, so no IPv4 alias setup is needed. IPv6 has no equivalent: the
// interface carries ::1 and nothing else, so a second address is real
// configuration -- see loopback.go for why this file reports rather than adds.

//go:build linux

package runner

import (
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ensureLoopbackAlias reports whether ip is usable on lo, and never changes the
// interface.
//
// IPv4 needs no work at all: 127.0.0.0/8 routes to lo by default, so any
// 127.x.x.x address binds. Input validation is shared across platforms.
//
// IPv6 is probed, because ::1 is the only address a host carries by default and
// adding another needs CAP_NET_ADMIN, which the test runner does not have. The
// error names the command that adds it.
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
	return nil
}

// loopbackIPv6AddCommand is the command an operator runs to put an IPv6 address
// on the loopback interface. /128 keeps it a host address, so no route toward
// the rest of its block is created.
func loopbackIPv6AddCommand(ip net.IP) string {
	var tb textbuf.Buffer
	return tb.Str("sudo ip -6 addr add ").Str(ip.String()).Str("/128 dev lo").String()
}
