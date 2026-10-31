// Design: docs/features/interfaces.md -- Netlink backend Linux implementation
// Overview: ifacenetlink.go -- package hub
// Related: tunnel_linux.go -- CreateTunnel implementation for all 8 tunnel kinds

//go:build linux

package ifacenetlink

import (
	"github.com/ze-software/ze/internal/component/iface"
)

// netlinkBackend implements iface.Backend using Linux netlink. Its owner MUST
// call Close before discarding it to release monitoring, neighbor watch and
// counter sockets.
type netlinkBackend struct {
	mon      *monitor
	counters counterSource
	// closed ends every neighbor watch this backend started
	// (neighbor_watch_linux.go), so no watch outlives its backend.
	closed closeSignal
}

func newNetlinkBackend() (iface.Backend, error) {
	return &netlinkBackend{}, nil
}

// Close MUST be called before discarding the backend. It permanently stops
// counter reads and waits for any bounded snapshot already in progress. It
// signals every neighbor watch to stop; each closes its channel once its
// kernel subscription is released.
func (b *netlinkBackend) Close() error {
	b.StopMonitor()
	b.closed.close()
	b.counters.close()
	return nil
}
