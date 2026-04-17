// Design: docs/architecture/l2tp.md -- subsystem read/write façade for CLI
// Related: subsystem.go -- owns the reactor slice this file reads from
// Related: snapshot.go -- reactor-scoped read API this file aggregates

package l2tp

import (
	"errors"
	"fmt"
)

// ErrSubsystemNotStarted is returned by façade methods when they are
// called before Start or after Stop. Keeps CLI handlers from panicking
// on a partially-initialized Subsystem.
var ErrSubsystemNotStarted = errors.New("l2tp: subsystem not started")

// Snapshot returns the aggregated read-only state across every reactor
// owned by the subsystem. Aggregation is a simple concatenation of
// per-reactor snapshots plus top-level counters; per-reactor snapshots
// already sort their own tunnels.
//
// Safe for concurrent use. Returns a zero-value Snapshot when the
// subsystem has not been started.
func (s *Subsystem) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return Snapshot{}
	}
	var agg Snapshot
	for _, r := range s.reactors {
		snap := r.Snapshot()
		if agg.CapturedAt.IsZero() {
			agg.CapturedAt = snap.CapturedAt
		}
		agg.Tunnels = append(agg.Tunnels, snap.Tunnels...)
		agg.TunnelCount += snap.TunnelCount
		agg.SessionCount += snap.SessionCount
	}
	return agg
}

// LookupTunnel walks every reactor in registration order and returns
// the first tunnel matching localTID. Returns false when no reactor
// owns a tunnel with that TID.
func (s *Subsystem) LookupTunnel(localTID uint16) (TunnelSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return TunnelSnapshot{}, false
	}
	for _, r := range s.reactors {
		if ts, ok := r.LookupTunnel(localTID); ok {
			return ts, true
		}
	}
	return TunnelSnapshot{}, false
}

// LookupSession walks every reactor and returns the first session
// matching localSID. L2TP SIDs are tunnel-scoped in RFC 2661, but
// operators expect one lookup; collisions across tunnels are unlikely
// in practice (16-bit random allocation) and the reactor returns the
// first hit deterministically.
func (s *Subsystem) LookupSession(localSID uint16) (SessionSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return SessionSnapshot{}, false
	}
	for _, r := range s.reactors {
		if ss, ok := r.LookupSession(localSID); ok {
			return ss, true
		}
	}
	return SessionSnapshot{}, false
}

// Listeners returns the bound endpoints in registration order. Used by
// `show l2tp listeners`. Returns nil when the subsystem is stopped.
func (s *Subsystem) Listeners() []ListenerSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	out := make([]ListenerSnapshot, 0, len(s.listeners))
	for _, l := range s.listeners {
		out = append(out, ListenerSnapshot{Addr: l.Addr()})
	}
	return out
}

// EffectiveConfig returns the Parameters the subsystem is currently
// running with. `shared-secret` is returned as "<set>" / "<unset>" so
// CLI output does not leak the secret.
func (s *Subsystem) EffectiveConfig() ConfigSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs := ConfigSnapshot{
		Enabled:       s.params.Enabled,
		MaxTunnels:    s.params.MaxTunnels,
		MaxSessions:   s.params.MaxSessions,
		HelloInterval: s.params.HelloInterval,
		SharedSecret:  redactSecret(s.params.SharedSecret),
	}
	cs.ListenAddrs = append(cs.ListenAddrs, s.params.ListenAddrs...)
	return cs
}

// TeardownTunnel fans an operator-initiated StopCCN across every
// reactor and returns nil on the first reactor that owned the TID.
// Returns ErrTunnelNotFound when no reactor owned it.
func (s *Subsystem) TeardownTunnel(localTID uint16) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return ErrSubsystemNotStarted
	}
	reactors := make([]*L2TPReactor, len(s.reactors))
	copy(reactors, s.reactors)
	s.mu.Unlock()

	for _, r := range reactors {
		err := r.TeardownTunnelByID(localTID)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrTunnelNotFound) {
			continue
		}
		return err
	}
	return fmt.Errorf("%w: local-tid=%d", ErrTunnelNotFound, localTID)
}

// TeardownSession fans an operator-initiated CDN across every reactor.
func (s *Subsystem) TeardownSession(localSID uint16) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return ErrSubsystemNotStarted
	}
	reactors := make([]*L2TPReactor, len(s.reactors))
	copy(reactors, s.reactors)
	s.mu.Unlock()

	for _, r := range reactors {
		err := r.TeardownSessionByID(localSID)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrSessionNotFound) {
			continue
		}
		return err
	}
	return fmt.Errorf("%w: local-sid=%d", ErrSessionNotFound, localSID)
}

// TeardownAllTunnels tears down every tunnel across every reactor and
// returns the total count torn down.
func (s *Subsystem) TeardownAllTunnels() int {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return 0
	}
	reactors := make([]*L2TPReactor, len(s.reactors))
	copy(reactors, s.reactors)
	s.mu.Unlock()

	n := 0
	for _, r := range reactors {
		n += r.TeardownAllTunnels()
	}
	return n
}

// TeardownAllSessions tears down every session across every reactor
// and returns the total count torn down.
func (s *Subsystem) TeardownAllSessions() int {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return 0
	}
	reactors := make([]*L2TPReactor, len(s.reactors))
	copy(reactors, s.reactors)
	s.mu.Unlock()

	n := 0
	for _, r := range reactors {
		n += r.TeardownAllSessions()
	}
	return n
}

// redactSecret maps a non-empty secret to "<set>" and the empty string
// to "<unset>" so CLI output does not leak the shared secret.
func redactSecret(s string) string {
	if s == "" {
		return "<unset>"
	}
	return "<set>"
}
