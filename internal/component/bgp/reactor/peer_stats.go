// Design: docs/architecture/api/architecture.md — peer statistics for operational commands
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"sync/atomic"
	"time"
)

// PeerStats holds a snapshot of per-peer counters.
// Updates = per UPDATE message (engine level, no content parsing).
// Keepalives = per KEEPALIVE message.
// EOR = End-of-RIB markers (RFC 4724).
// NLRI-level counters (announce vs withdraw) belong in the RIB plugin.
type PeerStats struct {
	UpdatesReceived    uint32
	UpdatesSent        uint32
	KeepalivesReceived uint32
	KeepalivesSent     uint32
	EORReceived        uint32
	EORSent            uint32
}

// peerCounters holds atomic counters for per-peer statistics.
// Embedded in Peer for lock-free increment from hot paths.
// NLRI-level counters (announce vs withdraw) are tracked by the RIB plugin.
type peerCounters struct {
	updatesReceived    atomic.Uint32
	updatesSent        atomic.Uint32
	keepalivesReceived atomic.Uint32
	keepalivesSent     atomic.Uint32
	eorReceived        atomic.Uint32
	eorSent            atomic.Uint32
	establishedAt      atomic.Int64 // UnixNano; 0 = not established
}

// Stats returns a snapshot of the peer's counters.
func (p *Peer) Stats() PeerStats {
	return PeerStats{
		UpdatesReceived:    p.counters.updatesReceived.Load(),
		UpdatesSent:        p.counters.updatesSent.Load(),
		KeepalivesReceived: p.counters.keepalivesReceived.Load(),
		KeepalivesSent:     p.counters.keepalivesSent.Load(),
		EORReceived:        p.counters.eorReceived.Load(),
		EORSent:            p.counters.eorSent.Load(),
	}
}

// IncrUpdatesReceived increments the received UPDATE counter.
func (p *Peer) IncrUpdatesReceived() {
	p.counters.updatesReceived.Add(1)
}

// IncrUpdatesSent increments the sent UPDATE counter.
func (p *Peer) IncrUpdatesSent() {
	p.counters.updatesSent.Add(1)
}

// IncrKeepalivesReceived increments the received KEEPALIVE counter.
func (p *Peer) IncrKeepalivesReceived() {
	p.counters.keepalivesReceived.Add(1)
}

// IncrKeepalivesSent increments the sent KEEPALIVE counter.
func (p *Peer) IncrKeepalivesSent() {
	p.counters.keepalivesSent.Add(1)
}

// IncrEORReceived increments the received End-of-RIB counter.
func (p *Peer) IncrEORReceived() {
	p.counters.eorReceived.Add(1)
}

// IncrEORSent increments the sent End-of-RIB counter.
func (p *Peer) IncrEORSent() {
	p.counters.eorSent.Add(1)
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
	p.counters.updatesReceived.Store(0)
	p.counters.updatesSent.Store(0)
	p.counters.keepalivesReceived.Store(0)
	p.counters.keepalivesSent.Store(0)
	p.counters.eorReceived.Store(0)
	p.counters.eorSent.Store(0)
	p.counters.establishedAt.Store(0)
}
