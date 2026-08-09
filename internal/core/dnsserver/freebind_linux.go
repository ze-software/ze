// Design: docs/architecture/dns/server-harness.md -- IP_FREEBIND opt-in bind, Linux only
//
// Mirrors internal/plugins/dhcpserver/socket_linux.go's plain-syscall Control
// hook pattern (SO_BINDTODEVICE there, IP_FREEBIND here).

//go:build linux

package dnsserver

import (
	"net"
	"syscall"
)

// ipFreebind is IP_FREEBIND from <linux/in.h>; it is not exposed by the
// standard syscall package.
const ipFreebind = 15

// listenConfig returns a net.ListenConfig that sets IP_FREEBIND on the socket
// before bind when freebind is true, allowing a bind to an address not yet
// present on any local interface (e.g. an anycast VIP added after startup).
func listenConfig(freebind bool) net.ListenConfig {
	if !freebind {
		return net.ListenConfig{}
	}
	return net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var sErr error
			err := c.Control(func(fd uintptr) {
				sErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, ipFreebind, 1)
			})
			if err != nil {
				return err
			}
			return sErr
		},
	}
}
