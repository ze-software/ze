//go:build linux

// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- Linux raw IPv6 socket probe
// RFC: rfc/short/rfc5340.md (§2.9 raw IPv6 proto 89)

package transport

import "golang.org/x/sys/unix"

// rawSocketAvailable opens and immediately closes a raw IPv6 proto-89 socket to
// confirm CAP_NET_RAW is present before the daemon starts.
func rawSocketAvailable() bool {
	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW, Protocol)
	if err != nil {
		return false
	}
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("ospfv3/transport: close raw-socket probe", "err", cerr)
	}
	return true
}
