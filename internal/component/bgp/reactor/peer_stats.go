// Design: docs/architecture/api/architecture.md — peer statistics for operational commands
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// msgTypeUpdate is the Prometheus label value for UPDATE messages.
// Shared across message-type counters and notification code mapping.
const msgTypeUpdate = "update"

// PeerStats holds a snapshot of per-peer counters.
// Updates = per UPDATE message (engine level, no content parsing).
// Keepalives = per KEEPALIVE message.
// EOR = End-of-RIB markers (RFC 4724) that reached the socket; see IncrEORSent
// for the contract and for why the value is per-peer lifetime, not per-session.
// NLRI-level counters (announce vs withdraw) belong in the RIB plugin.
type PeerStats struct {
	UpdatesReceived    uint32
	UpdatesSent        uint32
	KeepalivesReceived uint32
	KeepalivesSent     uint32
	EORReceived        uint32
	EORSent            uint32

	OpensReceived         uint32
	OpensSent             uint32
	NotificationsReceived uint32
	NotificationsSent     uint32
	RefreshReceived       uint32
	RefreshSent           uint32

	// Lifetime counters (survive ClearStats).
	ConnectionsEstablished uint32
	ConnectionsDropped     uint32

	// Last notification details (survive ClearStats).
	LastNotifCode    uint8
	LastNotifSubcode uint8
	LastNotifRecv    bool
	LastNotifTime    time.Time

	// Activity timestamps.
	LastReadTime  time.Time
	LastWriteTime time.Time
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

	opensReceived         atomic.Uint32
	opensSent             atomic.Uint32
	notificationsReceived atomic.Uint32
	notificationsSent     atomic.Uint32
	refreshReceived       atomic.Uint32
	refreshSent           atomic.Uint32

	// Lifetime counters (not reset by ClearStats).
	connectionsEstablished atomic.Uint32
	connectionsDropped     atomic.Uint32

	// Last notification details (not reset by ClearStats).
	lastNotifCode    atomic.Uint32 // uint8 stored in uint32
	lastNotifSubcode atomic.Uint32
	lastNotifRecv    atomic.Bool
	lastNotifTime    atomic.Int64 // UnixNano

	// Activity timestamps.
	lastReadTime  atomic.Int64 // UnixNano
	lastWriteTime atomic.Int64 // UnixNano
}

// Stats returns a snapshot of the peer's counters.
func (p *Peer) Stats() PeerStats {
	stats := PeerStats{
		UpdatesReceived:        p.counters.updatesReceived.Load(),
		UpdatesSent:            p.counters.updatesSent.Load(),
		KeepalivesReceived:     p.counters.keepalivesReceived.Load(),
		KeepalivesSent:         p.counters.keepalivesSent.Load(),
		EORReceived:            p.counters.eorReceived.Load(),
		EORSent:                p.counters.eorSent.Load(),
		OpensReceived:          p.counters.opensReceived.Load(),
		OpensSent:              p.counters.opensSent.Load(),
		NotificationsReceived:  p.counters.notificationsReceived.Load(),
		NotificationsSent:      p.counters.notificationsSent.Load(),
		RefreshReceived:        p.counters.refreshReceived.Load(),
		RefreshSent:            p.counters.refreshSent.Load(),
		ConnectionsEstablished: p.counters.connectionsEstablished.Load(),
		ConnectionsDropped:     p.counters.connectionsDropped.Load(),
		LastNotifCode:          uint8(p.counters.lastNotifCode.Load()),
		LastNotifSubcode:       uint8(p.counters.lastNotifSubcode.Load()),
		LastNotifRecv:          p.counters.lastNotifRecv.Load(),
	}
	if ns := p.counters.lastNotifTime.Load(); ns != 0 {
		stats.LastNotifTime = time.Unix(0, ns)
	}
	if ns := p.counters.lastReadTime.Load(); ns != 0 {
		stats.LastReadTime = time.Unix(0, ns)
	}
	if ns := p.counters.lastWriteTime.Load(); ns != 0 {
		stats.LastWriteTime = time.Unix(0, ns)
	}
	return stats
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
//
// CONTRACT: call this ONLY after the EOR send returned nil. eorSent counts
// End-of-RIB markers that reached the socket, never sends that were attempted:
// operators read it as "the peer has been told the initial RIB is complete"
// (`show bgp peer <sel> detail`, `show bgp summary`, the CLI dashboard,
// ze_peer_msg_sent{type="eor"}), and the functional suite reads it as the
// "end-of-RIB is on the wire" barrier before asserting the frame
// (test/scripts/ze_api.py wait_peer_eor_sent). Incrementing on a discarded
// error makes both claims false while looking healthy.
//
// It is a per-peer LIFETIME counter, not per-session: it is reset only by
// ClearStats, which runs when the peer object stops (peer_run.go cleanup), so
// it accumulates across session flaps. A value above the negotiated family count
// means the peer re-established, or that a second producer emitted an
// establishment EOR for a family this peer already covered.
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

// IncrOpensReceived increments the received OPEN counter.
func (p *Peer) IncrOpensReceived() {
	p.counters.opensReceived.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel(), "open").Inc()
	}
}

// IncrOpensSent increments the sent OPEN counter.
func (p *Peer) IncrOpensSent() {
	p.counters.opensSent.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel(), "open").Inc()
	}
}

func (p *Peer) recordNotification(code, subcode uint8, recv bool) {
	p.counters.lastNotifCode.Store(uint32(code))
	p.counters.lastNotifSubcode.Store(uint32(subcode))
	p.counters.lastNotifRecv.Store(recv)
	p.counters.lastNotifTime.Store(p.clock.Now().UnixNano())
	p.notificationExchanged.Store(true)
}

// IncrNotificationSent increments the sent NOTIFICATION counter with code/subcode
// labels and pushes a notification-sent error event onto the report bus.
// Sets p.notificationExchanged so the FSM Established->Idle transition handler
// in peer_run.go can suppress the duplicate session-dropped error.
func (p *Peer) IncrNotificationSent(code, subcode uint8) {
	p.counters.notificationsSent.Add(1)
	p.recordNotification(code, subcode, false)
	raiseNotificationError("sent", p.peerAddrLabel(), code, subcode)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.notifSent.With(
			p.peerAddrLabel(),
			notificationCodeLabel(code),
			textbuf.StringUint8(subcode),
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
	p.counters.notificationsReceived.Add(1)
	p.recordNotification(code, subcode, true)
	raiseNotificationError("received", p.peerAddrLabel(), code, subcode)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.notifRecv.With(
			p.peerAddrLabel(),
			notificationCodeLabel(code),
			textbuf.StringUint8(subcode),
		).Inc()
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel(), "notification").Inc()
	}
}

// IncrRefreshReceived increments the received ROUTE-REFRESH counter.
func (p *Peer) IncrRefreshReceived() {
	p.counters.refreshReceived.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgRecv.With(p.peerAddrLabel(), "refresh").Inc()
	}
}

// IncrRefreshSent increments the sent ROUTE-REFRESH counter.
func (p *Peer) IncrRefreshSent() {
	p.counters.refreshSent.Add(1)
	if p.reactor != nil && p.reactor.rmetrics != nil {
		p.reactor.rmetrics.peerMsgSent.With(p.peerAddrLabel(), "refresh").Inc()
	}
}

// IncrConnectionsEstablished increments the lifetime connections-established counter.
func (p *Peer) IncrConnectionsEstablished() {
	p.counters.connectionsEstablished.Add(1)
}

// IncrConnectionsDropped increments the lifetime connections-dropped counter.
func (p *Peer) IncrConnectionsDropped() {
	p.counters.connectionsDropped.Add(1)
}

// TouchLastRead records the current time as the last message read time.
func (p *Peer) TouchLastRead() {
	p.counters.lastReadTime.Store(p.clock.Now().UnixNano())
}

// TouchLastWrite records the current time as the last message write time.
func (p *Peer) TouchLastWrite() {
	p.counters.lastWriteTime.Store(p.clock.Now().UnixNano())
}

// SetEstablishedNow records the current time as session establishment time
// and increments the lifetime connections-established counter.
func (p *Peer) SetEstablishedNow() {
	p.counters.establishedAt.Store(p.clock.Now().UnixNano())
	p.IncrConnectionsEstablished()
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

// ClearStats resets per-session counters and the established timestamp.
// Called on session teardown to start fresh for the next session.
// Lifetime counters (connections, flaps, last notification) are preserved.
func (p *Peer) ClearStats() {
	p.counters.updatesReceived.Store(0)
	p.counters.updatesSent.Store(0)
	p.counters.keepalivesReceived.Store(0)
	p.counters.keepalivesSent.Store(0)
	p.counters.eorReceived.Store(0)
	p.counters.eorSent.Store(0)
	p.counters.establishedAt.Store(0)
	p.counters.opensReceived.Store(0)
	p.counters.opensSent.Store(0)
	p.counters.notificationsReceived.Store(0)
	p.counters.notificationsSent.Store(0)
	p.counters.refreshReceived.Store(0)
	p.counters.refreshSent.Store(0)
	p.counters.lastReadTime.Store(0)
	p.counters.lastWriteTime.Store(0)
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
