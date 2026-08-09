//go:build linux

// Design: docs/architecture/isis/isis-3-l2-transport.md -- raw-socket probe for the doctor check
//
// rawSocketAvailable opens and immediately closes an AF_PACKET/SOCK_RAW socket,
// the exact capability the IS-IS transport needs. EPERM (no CAP_NET_RAW) makes
// it return false, which the doctor check surfaces as doctor-isis-raw-socket.

package transport

import "golang.org/x/sys/unix"

// rawSocketAvailable reports whether a raw AF_PACKET socket can be opened.
func rawSocketAvailable() bool {
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		return false
	}
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("isis/transport: close raw-socket probe", "err", cerr)
	}
	return true
}
