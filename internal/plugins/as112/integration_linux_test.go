// Design: docs/architecture/dns/as112.md -- end-to-end DNS-serving proof against the real privileged port 53
//
// Requires CAP_NET_BIND_SERVICE / root: binding UDP/TCP port 53 needs
// elevated privilege on Linux (and everywhere else). This is why these
// assertions live in the sudo-gated `make ze-integration-as112-test` target
// (mk/test-integration.mk), not the standard `test/plugin/*.ci` functional
// suite, which runs unprivileged and has no precedent for a privileged-port
// bind. The as112-*.ci functional tests (test/plugin/) still verify
// config-application and `show as112` state; THIS file is the only place
// that proves the server actually answers real wire queries on port 53.

//go:build integration && linux

package as112

import (
	"net/netip"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// bindLoopback53 starts a real as112 server bound to 127.0.0.1:53 (the
// standard DNS port) and returns a cleanup func. Skips (not fails) if the
// bind fails, so this test degrades gracefully when run without
// CAP_NET_BIND_SERVICE instead of reporting a false regression.
func bindLoopback53(t *testing.T, cfg as112Config) (addr string, cleanup func()) {
	t.Helper()
	resetAS112State(t)
	storeState(buildState(cfg, 1))

	mgr := newServerManager(testLogger(), nil)
	err := mgr.apply(true, []dnsserver.Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: 53}})
	if err != nil {
		mgr.stopAll()
		t.Skipf("bind 127.0.0.1:53 failed (needs CAP_NET_BIND_SERVICE/root): %v", err)
	}
	return "127.0.0.1:53", mgr.stopAll
}

func exchange(t *testing.T, addr, name string, qtype uint16) *dns.Msg {
	t.Helper()
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	c := &dns.Client{Net: "udp", Timeout: 3 * time.Second}
	resp, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("query %s %s: %v", name, dns.TypeToString[qtype], err)
	}
	return resp
}

// VALIDATES: AC-2 -- a real wire query for a name within a Direct-Delegation
// reverse zone gets NOERROR, empty Answer, zone SOA in Authority.
func TestIntegration_ReverseZoneNoData(t *testing.T) {
	addr, cleanup := bindLoopback53(t, as112Config{Enabled: true})
	defer cleanup()

	resp := exchange(t, addr, "1.0.10.in-addr.arpa.", dns.TypePTR)
	if resp.Rcode != dns.RcodeSuccess || len(resp.Answer) != 0 || len(resp.Ns) != 1 {
		t.Fatalf("response = %+v, want NOERROR, empty Answer, 1 Ns (SOA)", resp)
	}
	if resp.RecursionAvailable {
		t.Fatal("RecursionAvailable = true, want false (AC-6)")
	}
}

// VALIDATES: AC-3 -- a real wire query within empty.as112.arpa.
func TestIntegration_EmptyAS112Arpa(t *testing.T) {
	addr, cleanup := bindLoopback53(t, as112Config{Enabled: true})
	defer cleanup()

	resp := exchange(t, addr, "foo.empty.as112.arpa.", dns.TypeA)
	if resp.Rcode != dns.RcodeSuccess || len(resp.Answer) != 0 || len(resp.Ns) != 1 {
		t.Fatalf("response = %+v, want NOERROR, empty Answer, 1 Ns (SOA)", resp)
	}
}

// VALIDATES: AC-4 / M3 -- HOSTNAME.AS112.NET TXT over the real wire, with
// max-length hostname/facility/location, fits <=512 octets with TC=0.
func TestIntegration_HostnameTXTUnder512(t *testing.T) {
	cfg := as112Config{
		Enabled:  true,
		Hostname: repeatString("h", maxHostnameLen),
		Facility: repeatString("f", maxFacilityLen),
		Location: repeatString("l", maxLocationLen),
	}
	addr, cleanup := bindLoopback53(t, cfg)
	defer cleanup()

	resp := exchange(t, addr, "hostname.as112.net.", dns.TypeTXT)
	if resp.Truncated {
		t.Fatal("Truncated = true, want false (TC=0) at max field lengths")
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("Answer = %v, want exactly one TXT record", resp.Answer)
	}
}

// VALIDATES: AC-5 -- a real wire query outside every served zone is NXDOMAIN.
func TestIntegration_OutOfZoneNXDOMAIN(t *testing.T) {
	addr, cleanup := bindLoopback53(t, as112Config{Enabled: true})
	defer cleanup()

	resp := exchange(t, addr, "example.com.", dns.TypeA)
	if resp.Rcode != dns.RcodeNameError {
		t.Fatalf("Rcode = %v, want NXDOMAIN", dns.RcodeToString[resp.Rcode])
	}
}

// VALIDATES: AC-13 -- SOA timers/MNAME over the real wire.
func TestIntegration_SOAOverWire(t *testing.T) {
	addr, cleanup := bindLoopback53(t, as112Config{Enabled: true})
	defer cleanup()

	resp := exchange(t, addr, "10.in-addr.arpa.", dns.TypeSOA)
	if len(resp.Answer) != 1 {
		t.Fatalf("Answer = %v, want exactly one SOA record", resp.Answer)
	}
	soa, ok := resp.Answer[0].(*dns.SOA)
	if !ok || soa.Refresh != soaRefresh || soa.Retry != soaRetry || soa.Ns != directDelegationMName {
		t.Fatalf("SOA = %+v, want RFC-mandated timers and MNAME %q", soa, directDelegationMName)
	}
}

// VALIDATES: AC-16 -- loopback/on-box source is always permitted over the
// real wire even when allow-from does not include it (H1/M4 carve-out for
// the healthcheck probe). The out-of-range-drop half of AC-15 is proven at
// the unit level (TestAllowFrom_DropsOutOfRange), since a real remote,
// out-of-range source cannot be simulated in-process without raw-socket
// spoofing; this integration test's job is to prove the loopback carve-out
// specifically holds over a real wire query, not a fabricated Peer.
func TestIntegration_LoopbackAlwaysPermittedOverWire(t *testing.T) {
	cfg := as112Config{Enabled: true, AllowFrom: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}}
	addr, cleanup := bindLoopback53(t, cfg)
	defer cleanup()

	resp := exchange(t, addr, "1.0.10.in-addr.arpa.", dns.TypePTR)
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("loopback query Rcode = %v, want NOERROR (H1/M4 carve-out)", dns.RcodeToString[resp.Rcode])
	}
}
