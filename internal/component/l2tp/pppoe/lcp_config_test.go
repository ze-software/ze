// Design: docs/guide/pppoe.md -- LCP configuration and reload snapshot

package pppoe

import (
	"testing"
	"time"
)

// TestPPPoEParseLCPContainer uses the real config parser and server config
// mapper, so a disconnected leaf or server copy loses a distinct value.
func TestPPPoEParseLCPContainer(t *testing.T) {
	p := extractFromConfigText(t, "set pppoe interface eth0\nset pppoe lcp restart-timer 6000\nset pppoe lcp max-terminate 3\nset pppoe lcp max-configure 4\nset pppoe lcp max-failure 6\n")
	if p.LCP.RestartTimer() != 6*time.Second {
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
	server := &InterfaceServer{}
	applyInterfaceConfig(server, &p.Interfaces[0], &p)
	if server.lcpConfig != p.LCP {
		t.Fatalf("server lost configured values: %+v", server.lcpConfig)
	}
	old := server.lcpConfig
	next := extractFromConfigText(t, "set pppoe interface eth0\nset pppoe lcp restart-timer 500\n")
	applyInterfaceConfig(server, &next.Interfaces[0], &next)
	if old != p.LCP {
		t.Fatal("reload mutated the earlier snapshot")
	}
	if server.lcpConfig.RestartTimer() != 500*time.Millisecond {
		t.Fatalf("new server timer = %s", server.lcpConfig.RestartTimer())
	}
}
