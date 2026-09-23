// Design: docs/features/interfaces.md -- Netlink backend Linux implementation
// Overview: ifacenetlink.go -- package hub
// Related: tunnel_linux.go -- CreateTunnel implementation for all 8 tunnel kinds

//go:build linux

package ifacenetlink

import (
	"github.com/ze-software/ze/internal/component/iface"
)

// netlinkBackend implements iface.Backend using Linux netlink. Its owner MUST
// call Close before discarding it to release monitoring and counter sockets.
type netlinkBackend struct {
	mon      *monitor
	counters counterSource
}

func newNetlinkBackend() (iface.Backend, error) {
	return &netlinkBackend{}, nil
}

// Close MUST be called before discarding the backend. It permanently stops
// counter reads and waits for any bounded snapshot already in progress.
func (b *netlinkBackend) Close() error {
	b.StopMonitor()
	b.counters.close()
	return nil
}
