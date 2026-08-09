// Design: docs/architecture/iface/logical-name-resolution.md -- consumers resolve through iface

package pppoe

import (
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/iface"
)

// resolveInterface looks up a logical interface name through the shared iface
// resolver and returns its kernel index, hardware address, and MTU. It replaces
// the per-package SIOCGIF ioctl wrapper (the twin of the one IS-IS dropped) so a
// PPPoE source interface honors the os-name / mac-match selectors instead of
// assuming name == kernel device. Cross-platform: on a host with no iface
// backend loaded, iface.Resolve returns the error (mirroring the old non-Linux
// stub) rather than a bogus zero binding.
func resolveInterface(name string) (ifindex int, hwaddr [EthALen]byte, mtu int, err error) {
	b, rerr := iface.Resolve(name)
	if rerr != nil {
		return 0, hwaddr, 0, fmt.Errorf("pppoe: resolve %s: %w", name, rerr)
	}
	if b.OperMAC != "" {
		mac, perr := net.ParseMAC(b.OperMAC)
		if perr != nil {
			return 0, hwaddr, 0, fmt.Errorf("pppoe: parse MAC %q for %s: %w", b.OperMAC, name, perr)
		}
		if len(mac) != EthALen {
			return 0, hwaddr, 0, fmt.Errorf("pppoe: %s MAC has %d bytes, want %d", name, len(mac), EthALen)
		}
		copy(hwaddr[:], mac)
	}
	return b.Ifindex, hwaddr, b.MTU, nil
}

// ResolveInterface looks up a logical interface name (cross-package use by the
// PPPoE client dialer).
func ResolveInterface(name string) (int, [EthALen]byte, int, error) { return resolveInterface(name) }
