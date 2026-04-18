// Design: docs/features/interfaces.md -- Netlink backend Linux implementation
// Overview: ifacenetlink.go -- package hub
// Related: tunnel_linux.go -- CreateTunnel implementation for all 8 tunnel kinds

//go:build linux

package ifacenetlink

import (
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// netlinkBackend implements iface.Backend using Linux netlink.
type netlinkBackend struct {
	mon *monitor
}

func newNetlinkBackend() (iface.Backend, error) {
	return &netlinkBackend{}, nil
}

func (b *netlinkBackend) Close() error {
	b.StopMonitor()
	return nil
}
