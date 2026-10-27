package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestGRLockReleasedAfterContainedPanic proves the graceful-restart manager
// releases m.mu when a call under it panics. Method: the receive worker reaches
// helperEnter (a Grace-LSA), helperExit (an LSDB content change) and
// exitRestart (a neighbor reaching Full); each holds m.mu across the metric
// calls, so a nil metric, a state only a Ze defect produces, panics there. The
// test recovers the panic as the ospf/receive boundary does, then m.mu must
// be free, or the next Grace-LSA and every origination would block.
func TestGRLockReleasedAfterContainedPanic(t *testing.T) {
	x := ospftypes.RouterID{10, 0, 0, 9}
	key := helperKey{iface: "eth0", router: x}
	grace := graceReceived{iface: "eth0", advRouter: x, gracePeriod: 120}

	cases := []struct {
		name string
		arm  func(m *grManager)
		run  func(m *grManager)
	}{
		{
			name: "helper enter",
			arm:  func(m *grManager) { m.metrics.helperSessions = nil },
			run:  func(m *grManager) { m.helperEnter(key, grace, false, netip.Addr{}, 0) },
		},
		{
			name: "helper exit",
			arm: func(m *grManager) {
				m.helperEnter(key, grace, false, netip.Addr{}, 0)
				m.metrics.helperExits = nil
			},
			run: func(m *grManager) { m.helperExit(key, nil, grExitGraceExpiry) },
		},
		{
			name: "restart exit",
			arm: func(m *grManager) {
				m.mu.Lock()
				m.restarting = true
				m.mu.Unlock()
				m.metrics.restarterExits = nil
			},
			run: func(m *grManager) { m.exitRestart(grExitAdjacencies) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := grHelperManager(t, time.Now())
			tc.arm(m)

			recovered := func() (value any) {
				defer func() { value = recover() }()
				tc.run(m)
				return nil
			}()
			if recovered == nil {
				t.Fatal("the nil metric did not panic: the test reaches no call under m.mu")
			}
			if !m.mu.TryLock() {
				t.Fatal("m.mu is still held after the panic under it was recovered")
			}
			m.mu.Unlock()
		})
	}
}
