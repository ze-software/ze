// Design: docs/architecture/l2tp/bng-5-pppoe.md -- observability snapshots for CLI

package pppoe

import (
	"net"
	"strconv"
	"strings"

	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// SessionSnapshot is a point-in-time copy of a PPPoE session, safe to
// read without holding locks.
type SessionSnapshot struct {
	PADSPublication PADSPublication
	SubscriberID    string
	AccessIfIndex   int
	SID             uint16
	MAC             net.HardwareAddr
	IfName          string
	ServiceName     string
	State           SessionState
	UnitNum         int // Kernel display only; -1 when the interface is not pppN.
	Identity        ppp.SessionIdentity
	CreatedAt       time.Time
}

// InterfaceSnapshot is a point-in-time copy of per-interface state.
type InterfaceSnapshot struct {
	Name         string
	IfIndex      int
	SessionCount int
	MaxSessions  int
	ServiceNames []string
}

// Snapshot is a point-in-time view of the PPPoE subsystem.
type Snapshot struct {
	SessionCount   int
	InterfaceCount int
	Sessions       []SessionSnapshot
	Interfaces     []InterfaceSnapshot
	CapturedAt     time.Time
}

// Sessions returns a snapshot of all sessions in the table.
func (st *SessionTable) Sessions() []SessionSnapshot {
	st.mu.Lock()
	defer st.mu.Unlock()

	out := make([]SessionSnapshot, 0, len(st.sessions))
	for _, s := range st.sessions {
		out = append(out, SessionSnapshot{
			PADSPublication: s.padsPublication,
			SubscriberID:    s.SubscriberID,
			AccessIfIndex:   s.AccessIfIndex,
			SID:             s.SID,
			MAC:             append(net.HardwareAddr(nil), s.MAC...),
			IfName:          s.IfName,
			ServiceName:     s.ServiceName,
			State:           s.State,
			UnitNum:         sessionUnitDisplay(s),
			Identity:        sessionIdentity(s),
			CreatedAt:       s.CreatedAt,
		})
	}
	return out
}

// Snapshot returns a point-in-time view of the entire PPPoE subsystem.
func (s *Subsystem) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := Snapshot{
		InterfaceCount: len(s.servers),
		CapturedAt:     time.Now(),
	}

	for _, srv := range s.servers {
		sessions := srv.sessions.Sessions()
		snap.SessionCount += len(sessions)
		snap.Sessions = append(snap.Sessions, sessions...)

		svcNames := make([]string, len(srv.serviceNames))
		copy(svcNames, srv.serviceNames)
		snap.Interfaces = append(snap.Interfaces, InterfaceSnapshot{
			Name:         srv.ifName,
			IfIndex:      srv.ifIndex,
			SessionCount: len(sessions),
			MaxSessions:  srv.sessions.maxSessions,
			ServiceNames: svcNames,
		})
	}

	return snap
}

// LookupSnapshot returns a snapshot of the session with the given SID,
// copying all fields under the lock to avoid racing with handlePADR
// which publishes the admitted owner after Add().
func (st *SessionTable) LookupSnapshot(sid uint16) (SessionSnapshot, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[sid]
	if !ok {
		return SessionSnapshot{}, false
	}
	return SessionSnapshot{
		PADSPublication: s.padsPublication,
		SubscriberID:    s.SubscriberID,
		AccessIfIndex:   s.AccessIfIndex,
		SID:             s.SID,
		MAC:             append(net.HardwareAddr(nil), s.MAC...),
		IfName:          s.IfName,
		ServiceName:     s.ServiceName,
		State:           s.State,
		UnitNum:         sessionUnitDisplay(s),
		Identity:        sessionIdentity(s),
		CreatedAt:       s.CreatedAt,
	}, true
}

// LookupSession searches all interfaces for a session with the given SID.
func (s *Subsystem) LookupSession(sid uint16) (SessionSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, srv := range s.servers {
		snap, ok := srv.sessions.LookupSnapshot(sid)
		if ok {
			return snap, true
		}
	}
	return SessionSnapshot{}, false
}

// sessionIdentity is read while the session table lock is held. The backend
// identity is immutable after the admitted owner is published.
func sessionIdentity(session *Session) ppp.SessionIdentity {
	if session.Opened == nil {
		return ppp.SessionIdentity{}
	}
	return session.Opened.Dataplane().Identity()
}

// sessionUnitDisplay preserves the kernel's existing human-facing unit field.
// A VPP interface never acquires a fabricated kernel unit number.
func sessionUnitDisplay(session *Session) int {
	name, kernel := strings.CutPrefix(sessionIdentity(session).InterfaceName, "ppp")
	if !kernel {
		return -1
	}
	unit, err := strconv.Atoi(name)
	if err != nil {
		return -1
	}
	if unit < 0 {
		return -1
	}
	return unit
}

// snapshotAdmission refuses a departed or replaced owner under the same lock
// that copies its immutable identity and mutable transport snapshot.
func (st *SessionTable) snapshotAdmission(expected *Session) (SessionSnapshot, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil || st.sessions[expected.SID] != expected {
		return SessionSnapshot{}, false
	}
	s := expected
	return SessionSnapshot{
		PADSPublication: s.padsPublication,
		SubscriberID:    s.SubscriberID,
		AccessIfIndex:   s.AccessIfIndex,
		SID:             s.SID,
		MAC:             append(net.HardwareAddr(nil), s.MAC...),
		IfName:          s.IfName,
		ServiceName:     s.ServiceName,
		State:           s.State,
		UnitNum:         sessionUnitDisplay(s),
		Identity:        sessionIdentity(s),
		CreatedAt:       s.CreatedAt,
	}, true
}
