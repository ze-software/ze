//go:build linux

// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- Linux raw-socket probe for the doctor check
//
// rawSocketAvailable opens and immediately closes an AF_INET/SOCK_RAW socket for
// protocol 112, the exact rx capability the VRRP transport needs. EPERM (no
// CAP_NET_RAW) makes it return false, which the doctor check surfaces as
// doctor-vrrp-raw-socket.

package transport

import (
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// rawSocketAvailable reports whether a raw proto-112 IP socket can be opened.
func rawSocketAvailable() bool {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, int(packet.ProtoNumber))
	if err != nil {
		return false
	}
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("vrrp/transport: close raw-socket probe", "err", cerr)
	}
	return true
}
