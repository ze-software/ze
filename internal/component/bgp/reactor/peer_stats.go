// Design: docs/architecture/api/architecture.md — peer statistics for operational commands
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"sync/atomic"
	"time"
)

// PeerStats holds a snapshot of per-peer message and route counters.
type PeerStats struct {
	MessagesReceived uint64
	MessagesSent     uint64
	RoutesReceived   uint32
	RoutesSent       uint32
}

// peerCounters holds atomic counters for message and route statistics.
// Embedded in Peer for lock-free increment from hot paths.
type peerCounters struct {
	messagesReceived atomic.Uint64
	messagesSent     atomic.Uint64
	routesReceived   atomic.Uint32
	routesSent       atomic.Uint32
	establishedAt    atomic.Int64 // UnixNano; 0 = not established
}

// Stats returns a snapshot of the peer's message and route counters.
func (p *Peer) Stats() PeerStats {
	return PeerStats{
		MessagesReceived: p.counters.messagesReceived.Load(),
		MessagesSent:     p.counters.messagesSent.Load(),
		RoutesReceived:   p.counters.routesReceived.Load(),
		RoutesSent:       p.counters.routesSent.Load(),
	}
}

// IncrMessageReceived increments the received message counter.
func (p *Peer) IncrMessageReceived() {
	p.counters.messagesReceived.Add(1)
}

// IncrMessageSent increments the sent message counter.
func (p *Peer) IncrMessageSent() {
	p.counters.messagesSent.Add(1)
}

// IncrRoutesReceived adds n to the received route counter.
func (p *Peer) IncrRoutesReceived(n uint32) {
	p.counters.routesReceived.Add(n)
}

// IncrRoutesSent adds n to the sent route counter.
func (p *Peer) IncrRoutesSent(n uint32) {
	p.counters.routesSent.Add(n)
}

// SetEstablishedNow records the current time as session establishment time.
func (p *Peer) SetEstablishedNow() {
	p.counters.establishedAt.Store(p.clock.Now().UnixNano())
}

// EstablishedAt returns the time the session was established.
// Returns zero time if not established.
func (p *Peer) EstablishedAt() time.Time {
	ns := p.counters.establishedAt.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// ClearStats resets all counters and the established timestamp.
// Called on session teardown to start fresh for the next session.
func (p *Peer) ClearStats() {
	p.counters.messagesReceived.Store(0)
	p.counters.messagesSent.Store(0)
	p.counters.routesReceived.Store(0)
	p.counters.routesSent.Store(0)
	p.counters.establishedAt.Store(0)
}
