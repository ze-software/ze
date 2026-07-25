// VALIDATES: `show ospf graceful-restart` / `show ospf ipv6 graceful-restart` render the
// restarter state (in-restart or not, grace end, reason) and the per-neighbor helper sessions
// (which neighbors are being helped and their remaining grace) for the queried family (AC-26).
// PREVENTS: a GR state view that omits the restarter phase or the active helper relationships.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestGRShowState (AC-26): the snapshot reports restarter + helper state.
func TestGRShowState(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)

	// Not restarting, no helpers yet.
	snap := e.grSnapshot()
	if snap.Family != "ipv4" || !snap.RestarterEnabled || snap.Restarting {
		t.Fatalf("initial snapshot wrong: %+v", snap)
	}
	if len(snap.Helpers) != 0 {
		t.Fatalf("expected no helpers initially: %+v", snap.Helpers)
	}

	// Enter restart.
	e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, []ospftypes.RouterID{{10, 0, 0, 2}})
	// Add a helper session.
	x := ospftypes.RouterID{10, 0, 0, 9}
	e.gr.helperEnter(helperKey{iface: "eth0", router: x}, graceReceived{iface: "eth0", advRouter: x, gracePeriod: 90}, true, netip.MustParseAddr("10.0.0.9"), 5)

	snap = e.grSnapshot()
	if !snap.Restarting {
		t.Fatalf("snapshot must report the restarter active")
	}
	if snap.Reason != grReasonReload || snap.GraceEndUnix != now.Add(120*time.Second).Unix() {
		t.Fatalf("restarter fields wrong: %+v", snap)
	}
	if len(snap.Helpers) != 1 {
		t.Fatalf("expected one helper session: %+v", snap.Helpers)
	}
	h := snap.Helpers[0]
	if h.Interface != "eth0" || h.Router != x.String() || !h.WasDR {
		t.Fatalf("helper view wrong: %+v", h)
	}
	if h.RemainingSeconds <= 0 || h.RemainingSeconds > 90 {
		t.Fatalf("helper remaining grace out of range: %d", h.RemainingSeconds)
	}
}
