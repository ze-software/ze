//go:build linux

// Design: docs/architecture/l2tp/subscriber-session-model.md -- Linux replacement-assignment fixture
// Related: current_core_driver_fixture_test.go -- shared genuine current-core protocol owner
package l2tp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// Each deliberately malformed assignment case gets a fresh real receipt on
// the SAME worker/Attempt through an actual LCP/auth/NCP restart. Prior receipt
// must already have been acknowledged by the consumer under test.
func (w *currentCoreWire) replacementAssignment(t *testing.T) ppp.EventSessionIPAssigned {
	t.Helper()
	withdrawal := w.beginRestart(t)
	withdrawal.Acknowledge() // protocol-only preparation, no setup NAS notice
	w.finishRestart(t)
	w.prepareNetwork(t, "grace", ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.7")}, [8]byte{})
	return w.assign(t, ppp.AddressFamilyIPv4)
}
