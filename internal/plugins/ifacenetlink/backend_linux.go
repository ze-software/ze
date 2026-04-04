// Design: docs/features/interfaces.md -- Netlink backend Linux implementation
// Overview: ifacenetlink.go -- package hub

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
