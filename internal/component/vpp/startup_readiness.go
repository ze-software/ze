// Design: docs/research/vpp-deployment-reference.md -- runtime startup readiness
// Related: vpp.go -- one manager owns each readiness lifetime

package vpp

import vppevents "github.com/ze-software/ze/internal/core/vpp/events"

// notifyStartup is owned by Run. Its caller MUST configure a capacity-one
// receipt before starting Run and MUST NOT replace it while Run is active.
// Clearing it before sending makes both setup failure and readiness one-shot.
func (m *VPPManager) notifyStartup(failure error) {
	if m.startup == nil {
		return
	}
	receipt := m.startup
	m.startup = nil
	receipt <- failure
}

// retireReadiness ends only the readiness this manager actually published.
// Cancellation and replacement take this same path as a failed connection;
// an initial candidate which never became ready emits no disconnected event.
func (m *VPPManager) retireReadiness() {
	if !m.ready {
		return
	}
	m.ready = false
	m.emitEvent(vppevents.EventDisconnected)
}
