// Design: docs/architecture/l2tp.md -- CLI snapshot data shape
// Related: reactor.go -- snapshot source; owns the tunnel and session maps

package l2tp

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"
)

// Snapshot captures the reactor's read-only operational state at one
// instant. All time values are wall clock (time.Time.UTC). Consumers
// (CLI handlers, tests) MUST treat the returned value as immutable --
// the reactor copies every field under tunnelsMu so later FSM activity
// does not mutate the snapshot.
//
// Caller MUST NOT hold tunnelsMu; Snapshot acquires it internally.
type Snapshot struct {
	ListenAddr   netip.AddrPort
	Tunnels      []TunnelSnapshot
	CapturedAt   time.Time
	TunnelCount  int
	SessionCount int
}

// TunnelSnapshot is the read-only view of one tunnel.
type TunnelSnapshot struct {
	LocalTID       uint16
	RemoteTID      uint16
	PeerAddr       netip.AddrPort
	PeerHostName   string
	PeerFraming    uint32
	PeerBearer     uint32
	PeerRecvWindow uint16
	State          string
	CreatedAt      time.Time
	LastActivity   time.Time
	Sessions       []SessionSnapshot
	MaxSessions    uint16
	SessionCount   int
}

// ListenerSnapshot is the read-only view of one bound UDP endpoint.
type ListenerSnapshot struct {
	Addr netip.AddrPort
}

// ConfigSnapshot renders the Parameters the subsystem is currently
// running with. SharedSecret is redacted to "<set>" / "<unset>" so it
// does not leak through CLI JSON output.
type ConfigSnapshot struct {
	Enabled       bool
	MaxTunnels    uint16
	MaxSessions   uint16
	HelloInterval time.Duration
	SharedSecret  string
	ListenAddrs   []netip.AddrPort
}

// SessionSnapshot is the read-only view of one session.
type SessionSnapshot struct {
	LocalSID           uint16
	RemoteSID          uint16
	TunnelLocalTID     uint16
	State              string
	StateNum           int
	CreatedAt          time.Time
	Username           string
	AssignedAddr       netip.Addr
	Family             string // "ipv4" / "ipv6" / "" when no NCP assignment yet
	TxConnectSpeed     uint32
	RxConnectSpeed     uint32
	FramingType        uint32
	SequencingRequired bool
	LNSMode            bool
	KernelSetupNeeded  bool
	PppInterface       string
}

// Snapshot returns a deep copy of the reactor's current state.
// The copy is taken under tunnelsMu so the returned value is safe to
// hand to any goroutine. Order: tunnels sorted by LocalTID, sessions
// within each tunnel sorted by LocalSID -- deterministic output for
// test assertions and human-readable CLI output.
func (r *L2TPReactor) Snapshot() Snapshot {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	return r.snapshotLocked()
}

// snapshotLocked produces a Snapshot assuming tunnelsMu is held by the
// caller. Used internally by aggregate façades that want one lock
// acquire across several reactors.
func (r *L2TPReactor) snapshotLocked() Snapshot {
	snap := Snapshot{
		CapturedAt:  r.params.Clock(),
		TunnelCount: len(r.tunnelsByLocalID),
	}
	if r.listener != nil {
		snap.ListenAddr = r.listener.Addr()
	}
	tids := make([]uint16, 0, len(r.tunnelsByLocalID))
	for tid := range r.tunnelsByLocalID {
		tids = append(tids, tid)
	}
	slices.Sort(tids)

	snap.Tunnels = make([]TunnelSnapshot, 0, len(tids))
	for _, tid := range tids {
		t := r.tunnelsByLocalID[tid]
		ts := TunnelSnapshot{
			LocalTID:       t.localTID,
			RemoteTID:      t.remoteTID,
			PeerAddr:       t.peerAddr,
			PeerHostName:   t.peerHostName,
			PeerFraming:    t.peerFraming,
			PeerBearer:     t.peerBearer,
			PeerRecvWindow: t.peerRecvWindow,
			State:          t.state.String(),
			CreatedAt:      t.createdAt,
			LastActivity:   t.lastActivity,
			MaxSessions:    t.maxSessions,
			SessionCount:   t.sessionCount(),
		}
		sids := make([]uint16, 0, len(t.sessions))
		for sid := range t.sessions {
			sids = append(sids, sid)
		}
		slices.Sort(sids)
		ts.Sessions = make([]SessionSnapshot, 0, len(sids))
		for _, sid := range sids {
			s := t.sessions[sid]
			ts.Sessions = append(ts.Sessions, sessionSnapshot(t.localTID, s))
			snap.SessionCount++
		}
		snap.Tunnels = append(snap.Tunnels, ts)
	}
	return snap
}

// sessionSnapshot copies one session's read-only fields into a snapshot
// value. Caller MUST hold the owning reactor's tunnelsMu.
func sessionSnapshot(tunnelTID uint16, s *L2TPSession) SessionSnapshot {
	family := ""
	if s.assignedAddr.IsValid() {
		if s.assignedAddr.Is4() {
			family = "ipv4"
		} else {
			family = "ipv6"
		}
	}
	return SessionSnapshot{
		LocalSID:           s.localSID,
		RemoteSID:          s.remoteSID,
		TunnelLocalTID:     tunnelTID,
		State:              s.state.String(),
		StateNum:           int(s.state),
		CreatedAt:          s.createdAt,
		Username:           s.username,
		AssignedAddr:       s.assignedAddr,
		Family:             family,
		TxConnectSpeed:     s.txConnectSpeed,
		RxConnectSpeed:     s.rxConnectSpeed,
		FramingType:        s.framingType,
		SequencingRequired: s.sequencingRequired,
		LNSMode:            s.lnsMode,
		KernelSetupNeeded:  s.kernelSetupNeeded,
		PppInterface:       s.pppInterface,
	}
}

// LookupTunnel returns a deep copy of the named tunnel, or false if no
// tunnel is registered under that local TID. Follows the same locking
// contract as Snapshot: caller MUST NOT hold tunnelsMu.
func (r *L2TPReactor) LookupTunnel(localTID uint16) (TunnelSnapshot, bool) {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	t, ok := r.tunnelsByLocalID[localTID]
	if !ok {
		return TunnelSnapshot{}, false
	}
	ts := TunnelSnapshot{
		LocalTID:       t.localTID,
		RemoteTID:      t.remoteTID,
		PeerAddr:       t.peerAddr,
		PeerHostName:   t.peerHostName,
		PeerFraming:    t.peerFraming,
		PeerBearer:     t.peerBearer,
		PeerRecvWindow: t.peerRecvWindow,
		State:          t.state.String(),
		CreatedAt:      t.createdAt,
		LastActivity:   t.lastActivity,
		MaxSessions:    t.maxSessions,
		SessionCount:   t.sessionCount(),
	}
	sids := make([]uint16, 0, len(t.sessions))
	for sid := range t.sessions {
		sids = append(sids, sid)
	}
	slices.Sort(sids)
	ts.Sessions = make([]SessionSnapshot, 0, len(sids))
	for _, sid := range sids {
		ts.Sessions = append(ts.Sessions, sessionSnapshot(t.localTID, t.sessions[sid]))
	}
	return ts, true
}

// LookupSession walks every tunnel for a session with the given local
// SID. L2TP session IDs are tunnel-scoped per RFC 2661, but operators
// expect a single global lookup; the reactor walks the map once and
// returns the first match. Returns false when no session has the given
// SID on any tunnel.
func (r *L2TPReactor) LookupSession(localSID uint16) (SessionSnapshot, bool) {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	for tid, t := range r.tunnelsByLocalID {
		if s, ok := t.sessions[localSID]; ok {
			return sessionSnapshot(tid, s), true
		}
	}
	return SessionSnapshot{}, false
}

// ReliableStats returns a snapshot of the reliable engine state for a
// tunnel. Returns nil when the tunnel does not exist.
func (r *L2TPReactor) ReliableStats(localTID uint16) *ReliableStats {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	t, ok := r.tunnelsByLocalID[localTID]
	if !ok {
		return nil
	}
	stats := t.engine.Stats()
	return &stats
}

// TunnelFSMHistory returns the FSM transition history for a tunnel.
// Returns nil when the tunnel does not exist.
func (r *L2TPReactor) TunnelFSMHistory(localTID uint16) []FSMTransition {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	t, ok := r.tunnelsByLocalID[localTID]
	if !ok {
		return nil
	}
	if t.fsmHistory == nil {
		return []FSMTransition{}
	}
	return t.fsmHistory.snapshot()
}

// SessionFSMHistory returns the FSM transition history for a session.
// Returns nil when the session does not exist.
func (r *L2TPReactor) SessionFSMHistory(localSID uint16) []FSMTransition {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	for _, t := range r.tunnelsByLocalID {
		if sess, ok := t.sessions[localSID]; ok {
			if sess.fsmHistory == nil {
				return []FSMTransition{}
			}
			return sess.fsmHistory.snapshot()
		}
	}
	return nil
}

// FormatFraming renders an RFC 2661 Framing Capabilities bitmap as a
// human-readable string for CLI output. "-" when zero.
func FormatFraming(v uint32) string {
	if v == 0 {
		return "-"
	}
	parts := make([]string, 0, 2)
	if v&0x00000001 != 0 {
		parts = append(parts, "async")
	}
	if v&0x00000002 != 0 {
		parts = append(parts, "sync")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("0x%08x", v)
	}
	return strings.Join(parts, "+")
}
