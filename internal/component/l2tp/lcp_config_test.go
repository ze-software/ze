// Design: docs/guide/l2tp.md -- LCP configuration reaches new sessions
// Integration: the reactor fixture binds a live Linux SO_REUSEPORT UDP socket.
//go:build integration && linux

package l2tp

import (
	"context"
	"net/netip"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// TestL2TPParseLCPContainer drives both startup and reload parsers from the
// real configuration tree and checks every non-default LCP setting.
func TestL2TPParseLCPContainer(t *testing.T) {
	tree, err := zeconfig.ParseTreeForValidation("set l2tp lcp restart-timer 500\nset l2tp lcp max-terminate 3\nset l2tp lcp max-configure 4\nset l2tp lcp max-failure 6\n")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ExtractParameters(tree)
	if err != nil {
		t.Fatal(err)
	}
	l2tpRoot, ok := tree.ToMap()["l2tp"].(map[string]any)
	if !ok {
		t.Fatalf("L2TP root = %T, want map[string]any", tree.ToMap()["l2tp"])
	}
	provider := &fakeConfigProvider{trees: map[string]map[string]any{"l2tp": l2tpRoot}}
	reloaded, err := extractFromProvider(provider)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []Parameters{parsed, reloaded} {
		if p.LCP.RestartTimer() != 500*time.Millisecond {
			t.Fatalf("RestartTimer = %s", p.LCP.RestartTimer())
		}
		if p.LCP.MaxTerminate() != 3 {
			t.Fatalf("MaxTerminate = %d", p.LCP.MaxTerminate())
		}
		if p.LCP.MaxConfigure() != 4 {
			t.Fatalf("MaxConfigure = %d", p.LCP.MaxConfigure())
		}
		if p.LCP.MaxFailure() != 6 {
			t.Fatalf("MaxFailure = %d", p.LCP.MaxFailure())
		}
	}
}

// TestL2TPStartSessionCarriesLCPRestartConfig exercises the production reactor
// dispatch, then reloads it and proves an earlier StartSession retains its value.
func TestL2TPStartSessionCarriesLCPRestartConfig(t *testing.T) {
	_, r, stop := newUnstartedReactor(t)
	defer stop()
	driver := newFakePPPDriver()
	r.setPPPDriver(driver)
	first, err := zeconfig.ParseTreeForValidation("set l2tp lcp restart-timer 500\nset l2tp lcp max-terminate 3\nset l2tp lcp max-configure 4\nset l2tp lcp max-failure 6\n")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ExtractParameters(first)
	if err != nil {
		t.Fatal(err)
	}
	r.setLCPConfig(p.LCP)
	mkTunnel(r, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	deliverOwnedKernelSuccess(t, r, kernelSetupSucceeded{localTID: 100, localSID: 1001, fds: pppSessionFDs{pppoxFD: 30, chanFD: 31, unitFD: 32, unitNum: 7}}, protocolFixtureIdentity("ppp7", 0x80000007))
	start := awaitStartSession(t, driver.sessionsIn, "session 1001 before the reload")
	if start.LCP != p.LCP {
		t.Fatalf("StartSession lost configured LCP values: %+v", start.LCP)
	}
	s := newStartedSubsystem(t, &p)
	s.reactors = []*l2tpReactor{r}
	provider := &fakeConfigProvider{trees: map[string]map[string]any{"l2tp": {"lcp": map[string]any{"restart-timer": "6000", "max-terminate": "7", "max-configure": "8", "max-failure": "9"}}}}
	if err := s.Reload(context.Background(), provider); err != nil {
		t.Fatal(err)
	}
	deliverOwnedKernelSuccess(t, r, kernelSetupSucceeded{localTID: 100, localSID: 1002, fds: pppSessionFDs{pppoxFD: 40, chanFD: 41, unitFD: 42, unitNum: 8}}, protocolFixtureIdentity("ppp8", 0x80000008))
	next := awaitStartSession(t, driver.sessionsIn, "session 1002 after the LCP reload")
	if start.LCP != p.LCP {
		t.Fatal("reload changed the earlier session snapshot")
	}
	if next.LCP.RestartTimer() != 6*time.Second {
		t.Fatalf("new timer = %s", next.LCP.RestartTimer())
	}
	if next.LCP.MaxTerminate() != 7 {
		t.Fatalf("new MaxTerminate = %d", next.LCP.MaxTerminate())
	}
	if next.LCP.MaxConfigure() != 8 {
		t.Fatalf("new MaxConfigure = %d", next.LCP.MaxConfigure())
	}
	if next.LCP.MaxFailure() != 9 {
		t.Fatalf("new MaxFailure = %d", next.LCP.MaxFailure())
	}
}
