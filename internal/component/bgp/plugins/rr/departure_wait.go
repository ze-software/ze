// Design: docs/guide/route-reflection.md -- departure recovery on peer down
// Related: rr.go -- enqueueDeparture counts each departure it queues
// Related: source_policy.go -- prepare waits for the source's departures

package rr

import (
	"context"
	"errors"
	"net/netip"
)

// departureWait is one source's departures queued or running: count of them,
// and idle, closed when the count returns to zero. Guarded by departureMu.
type departureWait struct {
	count int
	idle  chan struct{}
}

// sendDeparture runs one departure on a departure worker (sendPeerDown), then
// counts it done, waking a source-policy prepare waiting for the source.
func (rr *routeReflector) sendDeparture(source netip.Addr, entries map[string]withdrawalInfo) {
	rr.sendPeerDown(source, entries)
	rr.departureMu.Lock()
	defer rr.departureMu.Unlock()
	wait := rr.departing[source]
	if wait == nil {
		return
	}
	wait.count--
	if wait.count > 0 {
		return
	}
	close(wait.idle)
	delete(rr.departing, source)
}

// awaitDepartures returns once no departure of source is queued or running,
// so a source-policy prepare pins the source's routes behind every departure
// recovery the reflector already owed. It refuses when ctx ends first.
func (rr *routeReflector) awaitDepartures(ctx context.Context, source string) error {
	addr, err := netip.ParseAddr(source)
	if err != nil {
		return errors.New("bgp-rr: source-policy peer " + source + " is not an IP address: " + err.Error())
	}
	rr.departureMu.Lock()
	wait := rr.departing[addr]
	rr.departureMu.Unlock()
	// refuse-by-default: guards sendDeparture -- no registered wait is the idle state sendDeparture leaves when the source's last departure is answered
	if wait == nil {
		return nil
	}
	select {
	case <-wait.idle:
		return nil
	case <-ctx.Done():
		return errors.New("bgp-rr: source " + source + " still has a departure recovery in flight from its last session " +
			"down, and it did not finish before the source-policy deadline; commit the change again once it has")
	}
}
