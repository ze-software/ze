// Design: docs/architecture/l2tp/bng-5-pppoe.md -- observability snapshots for CLI

package pppoe

import (
	"net"
	"time"
)

// SessionSnapshot is a point-in-time copy of a PPPoE session, safe to
// read without holding locks.
type SessionSnapshot struct {
	SID         uint16
	MAC         net.HardwareAddr
	IfName      string
	ServiceName string
	State       SessionState
	UnitNum     int
	CreatedAt   time.Time
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
			SID:         s.SID,
			MAC:         append(net.HardwareAddr(nil), s.MAC...),
			IfName:      s.IfName,
			ServiceName: s.ServiceName,
			State:       s.State,
			UnitNum:     s.UnitNum,
			CreatedAt:   s.CreatedAt,
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
// which mutates State/UnitNum/PppoxFD after Add().
func (st *SessionTable) LookupSnapshot(sid uint16) (SessionSnapshot, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()

	s, ok := st.sessions[sid]
	if !ok {
		return SessionSnapshot{}, false
	}
	return SessionSnapshot{
		SID:         s.SID,
		MAC:         append(net.HardwareAddr(nil), s.MAC...),
		IfName:      s.IfName,
		ServiceName: s.ServiceName,
		State:       s.State,
		UnitNum:     s.UnitNum,
		CreatedAt:   s.CreatedAt,
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
