// Design: docs/architecture/api/architecture.md — peer statistics for operational commands
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"strconv"
	"sync/atomic"
	"time"
)

// msgTypeUpdate is the Prometheus label value for UPDATE messages.
// Shared across message-type counters and notification code mapping.
const msgTypeUpdate = "update"

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
// Uses a cached string computed at peer creation to avoid repeated
// netip.Addr.String() allocations on the hot path.
func (p *Peer) peerAddrLabel() string {
	if p.addrString == "" {
		return "unknown"
	}
	return p.addrString
}

// IncrUpdatesReceived increments the received UPDATE counter.
// Also increments the per-peer Prometheus counter with type label.
func (p *Peer) IncrUpdatesReceived() {
	p.counters.updatesReceived.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel(), msgTypeUpdate).Inc()
	}
}

// IncrUpdatesSent increments the sent UPDATE counter.
// Also increments the per-peer Prometheus counter with type label.
func (p *Peer) IncrUpdatesSent() {
	p.counters.updatesSent.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel(), msgTypeUpdate).Inc()
	}
}

// IncrKeepalivesReceived increments the received KEEPALIVE counter.
// Also increments the per-peer Prometheus counter with type label.
func (p *Peer) IncrKeepalivesReceived() {
	p.counters.keepalivesReceived.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel(), "keepalive").Inc()
	}
}

// IncrKeepalivesSent increments the sent KEEPALIVE counter.
// Also increments the per-peer Prometheus counter with type label.
func (p *Peer) IncrKeepalivesSent() {
	p.counters.keepalivesSent.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel(), "keepalive").Inc()
	}
}

// IncrEORReceived increments the received End-of-RIB counter.
// Also increments the per-peer Prometheus counter with type label.
func (p *Peer) IncrEORReceived() {
	p.counters.eorReceived.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel(), "eor").Inc()
	}
}

// IncrEORSent increments the sent End-of-RIB counter.
// Also increments the per-peer Prometheus counter with type label.
func (p *Peer) IncrEORSent() {
	p.counters.eorSent.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel(), "eor").Inc()
	}
}

// notificationCodeLabel maps a BGP notification error code (RFC 4271 Section 4.5)
// to a bounded label string for Prometheus. Unknown codes map to "other" to prevent
// unbounded label cardinality from malformed or future code values.
func notificationCodeLabel(code uint8) string {
	switch code {
	case 1:
		return "header"
	case 2:
		return "open"
	case 3:
		return msgTypeUpdate
	case 4:
		return "hold-timer"
	case 5:
		return "fsm"
	case 6:
		return "cease"
	default: // Intentional: unknown/future codes bucketed to bound cardinality.
		return "other"
	}
}

// IncrNotificationSent increments the sent NOTIFICATION counter with code/subcode
// labels and pushes a notification-sent error event onto the report bus.
// Sets p.notificationExchanged so the FSM Established->Idle transition handler
// in peer_run.go can suppress the duplicate session-dropped error.
func (p *Peer) IncrNotificationSent(code, subcode uint8) {
	p.notificationExchanged.Store(true)
	raiseNotificationError("sent", p.peerAddrLabel(), code, subcode)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.notifSent.With(
			p.peerAddrLabel(),
			notificationCodeLabel(code),
			strconv.FormatUint(uint64(subcode), 10),
		).Inc()
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel(), "notification").Inc()
	}
}

// IncrNotificationReceived increments the received NOTIFICATION counter with
// code/subcode labels and pushes a notification-received error event onto the
// report bus. Sets p.notificationExchanged so the FSM Established->Idle
// transition handler in peer_run.go can suppress the duplicate session-dropped
// error.
func (p *Peer) IncrNotificationReceived(code, subcode uint8) {
	p.notificationExchanged.Store(true)
	raiseNotificationError("received", p.peerAddrLabel(), code, subcode)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.notifRecv.With(
			p.peerAddrLabel(),
			notificationCodeLabel(code),
			strconv.FormatUint(uint64(subcode), 10),
		).Inc()
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel(), "notification").Inc()
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

// peerStateNames lists all PeerState.String() values for metric label cleanup.
var peerStateNames = []string{"stopped", "connecting", "active", "established", "unknown"}

// notifCodeNames lists all notification code label values produced by
// notificationCodeLabel. Used for metric cleanup when a peer is removed.
var notifCodeNames = []string{"header", "open", msgTypeUpdate, "hold-timer", "fsm", "cease", "other"}

// notifSubcodeNames lists common subcodes for metric cleanup.
// Covers 0-14, which spans all standard subcodes across all error codes.
var notifSubcodeNames = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14"}

// updatePeerStateMetric updates the ze_peer_state Prometheus gauge and
// increments session lifecycle counters (transitions, established, flaps).
func (p *Peer) updatePeerStateMetric(oldState, newState PeerState) {
	if p.reactor == nil || p.reactor.rmetrics == nil {
		return
	}
	m := p.reactor.rmetrics
	addr := p.peerAddrLabel()

	m.peerState.With(addr).Set(float64(newState))
	m.stateTransitions.With(addr, oldState.String(), newState.String()).Inc()

	if newState == PeerStateEstablished {
		m.sessionsEstablished.With(addr).Inc()
	}
	if oldState == PeerStateEstablished && newState != PeerStateEstablished {
		m.sessionFlaps.With(addr).Inc()
		m.sessionDuration.With(addr).Set(0)
	}
}
