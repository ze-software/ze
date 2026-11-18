// Design: docs/guide/route-reflection.md -- departure recovery on peer down
// Related: rr.go -- enqueueDeparture counts each departure it queues
// Related: source_policy.go -- prepare waits for the source's departures

package rr

import (
	"context"
	"errors"
)

// departureWait is one source's departures queued or running: count of them,
// and idle, closed when the count returns to zero. Guarded by departureMu.
type departureWait struct {
	count int
	idle  chan struct{}
}

// sendDeparture runs one departure on a departure worker (sendPeerDown), then
// counts it done, waking a source-policy prepare waiting for the source.
// departureWork keeps the destinations captured on original DOWN separate
// from comparable withdrawalInfo values used by source-policy pins.
type departureBatch struct {
	entries map[withdrawalKey]withdrawalInfo
	targets []peerTarget
}
type departureWork struct {
	entries map[withdrawalKey]withdrawalInfo
	batches []departureBatch
}

func (rr *routeReflector) sendDeparture(source peerIdentity, work departureWork) {
	ctx := rr.replayParent()
	err := rr.forwards.wait(ctx)
	blocked := false
	if err == nil {
		err = rr.unsettledWork()
		blocked = err != nil
	}
	if err == nil {
		err = rr.retainSource(ctx, source)
	}
	if err != nil {
		blocked = true
	}
	if err == nil {
		err = rr.sendPeerDown(ctx, source, work)
	}
	if blocked {
		rr.mu.RLock()
		state := rr.peers[source]
		rr.mu.RUnlock()
		if state != nil {
			state.departureCall.Lock()
			rr.mu.Lock()
			if state.departureOwed == nil {
				held := work
				state.departureOwed = &held
			} else {
				if state.departureOwed.entries == nil {
					state.departureOwed.entries = make(map[withdrawalKey]withdrawalInfo)
				}
				for key, info := range work.entries {
					state.departureOwed.entries[key] = info
				}
				state.departureOwed.batches = append(state.departureOwed.batches, work.batches...)
			}
			rr.mu.Unlock()
			state.departureCall.Unlock()
		}
	}
	if err != nil {
		rr.reportUpdateFailure(source.address.String(), "original source departure", err)
	}
	if err == nil {
		rr.withdrawalMu.Lock()
		entries := rr.withdrawals[source.address.String()]
		for key, closed := range work.entries {
			if current, present := entries[key]; present {
				if current == closed {
					delete(entries, key)
				}
			}
		}
		if len(entries) == 0 {
			delete(rr.withdrawals, source.address.String())
		}
		rr.withdrawalMu.Unlock()
		err = rr.releaseEndedSource(ctx, source)
		if err != nil {
			rr.reportUpdateFailure(source.address.String(), "normal DOWN lease release", err)
		}
	}
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
func (rr *routeReflector) awaitDepartures(ctx context.Context, owner uint64) error {
	rr.departureMu.Lock()
	pending := make(map[peerIdentity]<-chan struct{})
	for source, wait := range rr.departing {
		if source.owner == owner {
			pending[source] = wait.idle
		}
	}
	rr.departureMu.Unlock()
	for source, idle := range pending {
		select {
		case <-idle:
		case <-ctx.Done():
			return errors.New("bgp-rr: source " + source.address.String() + ": departure recovery in flight did not reach the engine before the source-policy deadline: " + ctx.Err().Error())
		}
	}
	return nil
}
