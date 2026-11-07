// Design: docs/architecture/iface/logical-name-resolution.md -- consumers resolve through iface

package pppoe

import (
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/iface"
)

// InterfaceBinding is one resolution of a logical interface name: the kernel
// device it named at that moment and the facts read from that device. Every
// kernel operation a PPPoE endpoint makes on the interface takes its identity
// from one binding, so the discovery socket (Ifindex) and the session socket
// (Device) always reach the same NIC. A value type, safe to copy.
type InterfaceBinding struct {
	// Device is the kernel device name the logical name resolved to, through
	// the os-name or mac match selector when the interface carries one. It is
	// the name connect(AF_PPPOX) takes; the configured name never is.
	Device string
	// Ifindex is the kernel index of Device, used by the discovery socket.
	Ifindex int
	// HWAddr is Device's operational hardware address, the PPPoE source MAC.
	HWAddr [EthALen]byte
	// MTU is Device's MTU.
	MTU int
}

// resolveInterface looks up a logical interface name through the shared iface
// resolver. It replaces the per-package SIOCGIF ioctl wrapper (the twin of the
// one IS-IS dropped) so a PPPoE interface honors the os-name / mac-match
// selectors instead of assuming name == kernel device. Cross-platform: on a
// host with no iface backend loaded, iface.Resolve returns the error
// (mirroring the old non-Linux stub) rather than a bogus zero binding.
func resolveInterface(name string) (InterfaceBinding, error) {
	b, rerr := iface.Resolve(name)
	if rerr != nil {
		return InterfaceBinding{}, fmt.Errorf("pppoe: resolve %s: %w", name, rerr)
	}
	// A binding with no device name cannot name the session socket's device.
	// Falling back to the logical name is the defect this type exists to stop.
	if b.OsName == "" {
		return InterfaceBinding{}, fmt.Errorf("pppoe: resolve %s: the resolver named no kernel device", name)
	}
	binding := InterfaceBinding{Device: b.OsName, Ifindex: b.Ifindex, MTU: b.MTU}
	if b.OperMAC != "" {
		mac, perr := net.ParseMAC(b.OperMAC)
		if perr != nil {
			return InterfaceBinding{}, fmt.Errorf("pppoe: parse MAC %q for %s: %w", b.OperMAC, name, perr)
		}
		if len(mac) != EthALen {
			return InterfaceBinding{}, fmt.Errorf("pppoe: %s MAC has %d bytes, want %d", name, len(mac), EthALen)
		}
		copy(binding.HWAddr[:], mac)
	}
	return binding, nil
}

// ResolveInterface looks up a logical interface name (cross-package use by the
// PPPoE client dialer).
func ResolveInterface(name string) (InterfaceBinding, error) { return resolveInterface(name) }
