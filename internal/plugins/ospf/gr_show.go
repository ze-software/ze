// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- `show ospf graceful-restart` renderer.
// Related: gr.go (grManager state), cmd_show.go + register.go (command wiring).
// RFC: rfc/short/rfc3623.md (restarter + helper state), rfc/short/rfc5187.md (OSPFv3).
package ospf

import (
	"sort"
	"time"
)

// grHelperView is one helper session in the `show ospf graceful-restart` output.
type grHelperView struct {
	Interface        string `json:"interface"`
	Router           string `json:"router"`
	RemainingSeconds int64  `json:"remaining-seconds"`
	WasDR            bool   `json:"was-dr"`
}

// grShowSnapshot is the `show ospf graceful-restart` / `show ospf ipv6 graceful-restart` view:
// the restarter state (enabled, in-restart, grace end, reason) and the per-neighbor helper
// sessions (RFC 3623 / RFC 5187, AC-26).
type grShowSnapshot struct {
	Family            string         `json:"family"`
	RestarterEnabled  bool           `json:"restarter-enabled"`
	HelperEnabled     bool           `json:"helper-enabled"`
	RestartInterval   uint16         `json:"restart-interval"`
	StrictLSAChecking bool           `json:"strict-lsa-checking"`
	Restarting        bool           `json:"restarting"`
	GraceEndUnix      int64          `json:"grace-end-unix"`
	Reason            uint8          `json:"reason"`
	Helpers           []grHelperView `json:"helpers"`
}

// grSnapshot renders the Graceful Restart state for this engine's address family (AC-26).
func (e *engine) grSnapshot() grShowSnapshot {
	m := e.gr
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	snap := grShowSnapshot{
		Family:            e.grFamilyLabel(),
		RestarterEnabled:  m.cfg.restarterEnabled(),
		HelperEnabled:     m.cfg.HelperEnabled,
		RestartInterval:   m.cfg.RestartInterval,
		StrictLSAChecking: m.cfg.StrictLSAChecking,
		Restarting:        m.restarting,
		Reason:            m.reason,
		Helpers:           []grHelperView{},
	}
	if m.restarting {
		snap.GraceEndUnix = m.graceEnd.Unix()
	}
	for key, s := range m.helping {
		remaining := max(int64(s.graceEnd.Sub(now)/time.Second), 0)
		snap.Helpers = append(snap.Helpers, grHelperView{
			Interface:        key.iface,
			Router:           key.router.String(),
			RemainingSeconds: remaining,
			WasDR:            s.wasDR,
		})
	}
	sort.Slice(snap.Helpers, func(i, j int) bool {
		if snap.Helpers[i].Interface != snap.Helpers[j].Interface {
			return snap.Helpers[i].Interface < snap.Helpers[j].Interface
		}
		return snap.Helpers[i].Router < snap.Helpers[j].Router
	})
	return snap
}
