// Design: docs/architecture/dns/as112.md -- AC-11 / finding B2: IP_FREEBIND wiring proof

//go:build linux

package as112

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/dnsserver"
	"github.com/ze-software/ze/internal/test/userns"
)

// VALIDATES: AC-11 -- newServerManager wires Options{Freebind: true} through
// to the harness, so bind() succeeds against an address not locally assigned
// (TEST-NET-3, RFC 5737, never routed to this host) -- the same proof
// internal/core/dnsserver's own freebind_integration_linux_test.go uses for
// the harness itself, run here against AS112'S OWN newServerManager
// construction to prove AS112 actually opts in, not just that the harness
// mechanism works in isolation.
// PREVENTS: newServerManager silently reverting to Options{} (Freebind:
// false), which would make as112's bind() fail with EADDRNOTAVAIL whenever
// it starts before iface has applied the anycast address to lo (finding
// B2's whole reason for existing).
func TestListener_FreebindBindsWithoutAddress(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	resetAS112State(t)
	storeState(buildState(as112Config{Enabled: true}, 1))

	mgr := newServerManager(testLogger(), nil)
	const nonLocalAddr = "203.0.113.5"
	err := mgr.mgr.ApplyWithSecure(true, []dnsserver.Endpoint{{IP: netip.MustParseAddr(nonLocalAddr), Port: 5391}}, dnsserver.SecureConfig{}, dnsserver.BindLenient, testLogger())
	if err != nil {
		t.Fatalf("bind %s:5391 through newServerManager with IP_FREEBIND: %v", nonLocalAddr, err)
	}
	t.Cleanup(mgr.stopAll)
}
