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

// peerAddrLabel returns the peer address string for Prometheus labels.
// settings is set at peer creation and never nil, but guard defensively.
func (p *Peer) peerAddrLabel() string {
	if p.settings == nil {
		return "unknown"
	}
	return p.settings.Address.String()
}

// IncrUpdatesReceived increments the received UPDATE counter.
// Also increments the per-peer Prometheus counter if metrics are configured.
func (p *Peer) IncrUpdatesReceived() {
	p.counters.updatesReceived.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel()).Inc()
	}
}

// IncrUpdatesSent increments the sent UPDATE counter.
// Also increments the per-peer Prometheus counter if metrics are configured.
func (p *Peer) IncrUpdatesSent() {
	p.counters.updatesSent.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel()).Inc()
	}
}

// IncrKeepalivesReceived increments the received KEEPALIVE counter.
// Also increments the per-peer Prometheus counter if metrics are configured.
func (p *Peer) IncrKeepalivesReceived() {
	p.counters.keepalivesReceived.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel()).Inc()
	}
}

// IncrKeepalivesSent increments the sent KEEPALIVE counter.
// Also increments the per-peer Prometheus counter if metrics are configured.
func (p *Peer) IncrKeepalivesSent() {
	p.counters.keepalivesSent.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel()).Inc()
	}
}

// IncrEORReceived increments the received End-of-RIB counter.
// Also increments the per-peer Prometheus counter if metrics are configured.
func (p *Peer) IncrEORReceived() {
	p.counters.eorReceived.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel()).Inc()
	}
}

// IncrEORSent increments the sent End-of-RIB counter.
// Also increments the per-peer Prometheus counter if metrics are configured.
func (p *Peer) IncrEORSent() {
	p.counters.eorSent.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel()).Inc()
	}
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

// updatePeerStateMetric updates the ze_peer_state Prometheus gauge.
func (p *Peer) updatePeerStateMetric(state PeerState) {
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerState.With(p.peerAddrLabel()).Set(float64(state))
	}
}
