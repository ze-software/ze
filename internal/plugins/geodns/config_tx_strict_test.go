package geodns

import (
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
)

// VALIDATES: R-13 for geodns. A reload whose listener cannot be bound (its
// port is in use) fails the install with an error naming the endpoint and
// the fix; the previous snapshot stays published and the listener of the
// running config still answers. The Stage 2 install binds leniently.
// PREVENTS: a SIGHUP moving geodns to a busy port reported as committed while
// DNS is down, the failure only in the log.
// Not parallel: stateP is package state.
func TestInstallReloadToUnbindableListenerIsRefused(t *testing.T) {
	data := `{"service":{"geodns":{"enabled":"true","zone":["test.example."],"nameserver":["127.0.0.1"],
		"host-set":{"external":{"host":{"proxy.test.example.":{"address":["10.7.7.7"]}}}},
		"source":{"0.0.0.0/0":{"host-set":"external"}}}}}`
	running, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	candidate, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	loopback := netip.MustParseAddr("127.0.0.1")
	a, b := freePort(t), freePort(t)
	running.Listeners = []listenerEndpoint{{IP: loopback, Port: a}}
	candidate.Listeners = []listenerEndpoint{{IP: loopback, Port: b}}

	inst := &geodnsInstaller{mgr: newServerManager(testLogger()), log: testLogger()}
	t.Cleanup(func() {
		inst.mgr.stopAll()
		stateP.Store(nil)
	})
	inst.boot = true
	if err := inst.install(running); err != nil {
		t.Fatalf("boot install: %v", err)
	}
	inst.boot = false
	installed := loadState()

	busy := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(b)))
	holder, err := net.ListenPacket("udp", busy)
	if err != nil {
		t.Fatalf("occupy %s: %v", busy, err)
	}
	t.Cleanup(func() { _ = holder.Close() })

	err = inst.install(candidate)
	if err == nil {
		t.Fatal("reload to an occupied port succeeded")
	}
	for _, want := range []string{"geodns: dnsserver: reload refused", "cannot listen on " + busy, "commit again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if loadState() != installed {
		t.Error("a refused reload left its snapshot published")
	}
	resp := queryA(t, "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(a))), "proxy.test.example.", "1.2.3.4")
	if got := firstA(resp); got != "10.7.7.7" {
		t.Fatalf("running listener after the refusal: A = %q, want 10.7.7.7", got)
	}
}
