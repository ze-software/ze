//go:build linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- Linux raw-socket probe

package transport

import "golang.org/x/sys/unix"

func rawSocketAvailable() bool {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, Protocol)
	if err != nil {
		return false
	}
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("ospf/transport: close raw-socket probe", "err", cerr)
	}
	return true
}
