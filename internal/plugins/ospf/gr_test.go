// VALIDATES: the shared GR control plane drives BOTH address families through the same
// restarter/helper code paths (A-1, R-13): a v4 engine and a v6 engine enter and exit
// in-restart mode identically, and the in-restart flag gates origination + install for both.
// PREVENTS: family divergence creeping into the state machines (an IsV6 branch inside the
// restarter/helper rather than only at the wire seam).
package ospf

import (
	"testing"
	"time"

	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

func grTestConfig() gracefulRestartConfig {
	return gracefulRestartConfig{
		present:           true,
		RestartInterval:   120,
		RestarterSupport:  grSupportPlanned,
		HelperEnabled:     true,
		StrictLSAChecking: true,
	}
}

// grEnableEngine returns a v4 or v6 engine with GR enabled and a deterministic clock.
func grEnableEngine(t *testing.T, v6 bool, now time.Time) *engine {
	t.Helper()
	var e *engine
	if v6 {
		e = newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	} else {
		e = newEngine(nil)
	}
	e.gr.now = func() time.Time { return now }
	e.gr.configure(grTestConfig())
	return e
}

// TestGRControlPlaneSharedAcrossFamilies (A-1, R-13): the restarter FSM enters and exits the
// same way for IPv4 and IPv6; only the wire seam differs.
func TestGRControlPlaneSharedAcrossFamilies(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	for _, v6 := range []bool{false, true} {
		e := grEnableEngine(t, v6, now)
		x := ospftypes.RouterID{10, 0, 0, 2}
		e.gr.enterRestart(now.Add(120*time.Second), grReasonReload, []ospftypes.RouterID{x})
		if !e.gr.inRestart() {
			t.Fatalf("v6=%v: engine should be in restart after enterRestart", v6)
		}
		if !e.gr.suppressOrigination() || !e.gr.suppressInstall() {
			t.Fatalf("v6=%v: in-restart must suppress origination and install", v6)
		}
		// All pre-restart adjacencies re-reach Full -> exit (RFC 3623 sec 2.2 trigger 1).
		e.gr.noteAdjacencyFull(x)
		if e.gr.inRestart() {
			t.Fatalf("v6=%v: engine should have exited restart once all adjacencies are Full", v6)
		}
		if e.gr.suppressOrigination() || e.gr.suppressInstall() {
			t.Fatalf("v6=%v: suppression must clear on exit", v6)
		}
	}
}

// TestGRDisabledNoGraceLSA (AC-25, A-13): with GR disabled (the default), prepareRestart
// originates no Grace-LSA and never enters in-restart.
func TestGRDisabledNoGraceLSA(t *testing.T) {
	e := newEngine(nil)
	// default config: GracefulRestart not present -> RestarterEnabled false.
	if e.gr.cfg.restarterEnabled() {
		t.Fatalf("default config must not enable the restarter")
	}
	if err := e.gr.prepareRestart(grReasonReload); err == nil {
		t.Fatalf("prepareRestart should refuse when the restarter is disabled")
	}
	if e.gr.inRestart() {
		t.Fatalf("engine must not enter restart when GR is disabled")
	}
}
