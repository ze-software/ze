// Design: docs/features/interfaces.md -- Linux Control Plane pairs (VPP-only)
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import "fmt"

// SetupLCPPair and RemoveLCPPair are VPP-only: an LCP pair mirrors a VPP
// dataplane interface into a Linux TAP. The netlink backend owns real Linux
// interfaces directly, so there is nothing to shadow; it rejects the request
// rather than silently no-op. The ze:backend commit gate normally prevents LCP
// config from reaching a non-vpp backend, so these are defense in depth.
func (b *netlinkBackend) SetupLCPPair(vppIface, _ string) error {
	return fmt.Errorf("iface: LCP pair for %q is only supported on the vpp backend", vppIface)
}

func (b *netlinkBackend) RemoveLCPPair(vppIface string) error {
	return fmt.Errorf("iface: LCP pair for %q is only supported on the vpp backend", vppIface)
}
