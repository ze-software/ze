// Design: docs/architecture/dns/as112.md -- end-to-end DNS-serving proof against the real privileged port 53
//
// Requires CAP_NET_BIND_SERVICE / root: binding UDP/TCP port 53 needs
// elevated privilege on Linux (and everywhere else). These assertions run under
// `./le test integration as112`, not the standard `test/plugin/*.ci` functional
// suite, which is unprivileged and has no precedent for a privileged-port bind.
// The as112-*.ci functional tests still verify
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
	err := mgr.mgr.ApplyWithSecure(true, []dnsserver.Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: 53}}, dnsserver.SecureConfig{}, dnsserver.BindLenient, testLogger())
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

// wantZoneSOA fails unless resp carries an empty Answer and exactly the SOA of
// zone in Authority. RFC 2308 Section 3: "Name servers authoritative for a zone
// MUST include the SOA record of the zone in the authority section of the
// response when reporting an NXDOMAIN or indicating that no data of the
// requested type exists." The helper checks
// both halves.
func wantZoneSOA(t *testing.T, resp *dns.Msg, zone string) {
	t.Helper()
	if len(resp.Answer) != 0 || len(resp.Ns) != 1 {
		t.Fatalf("Answer=%v Ns=%v, want empty Answer and exactly the %s SOA in Ns", resp.Answer, resp.Ns, zone)
	}
	soa, ok := resp.Ns[0].(*dns.SOA)
	if !ok || soa.Hdr.Name != zone {
		t.Fatalf("Ns[0] = %v, want the SOA of %s", resp.Ns[0], zone)
	}
}

// VALIDATES: AC-2 -- a real wire PTR query at the apex of a Direct-Delegation
// reverse zone gets NODATA: NOERROR, empty Answer, the zone SOA in Authority.
// The apex is the one node the zone holds (RFC 7534 Section 3.5 db.dd-empty),
// and it holds no PTR.
func TestIntegration_ReverseZoneNoData(t *testing.T) {
	addr, cleanup := bindLoopback53(t, as112Config{Enabled: true})
	defer cleanup()

	resp := exchange(t, addr, "10.in-addr.arpa.", dns.TypePTR)
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("Rcode = %v, want NOERROR (NODATA at the apex)", dns.RcodeToString[resp.Rcode])
	}
	wantZoneSOA(t, resp, "10.in-addr.arpa.")
	if resp.RecursionAvailable {
		t.Fatal("RecursionAvailable = true, want false (AC-6)")
	}
}

// VALIDATES: a real wire query for a private address's reverse name below the
// apex of a Direct-Delegation zone gets NXDOMAIN with the zone SOA in Authority,
// authoritatively. The zone holds nothing below its apex, so the name does not
// exist: RFC 1035 Section 4.1.1, "Name Error - Meaningful only for responses
// from an authoritative name server, this code signifies that the domain name
// referenced in the query does not exist." The test asks for a name
// below a served apex.
func TestIntegration_ReverseZoneNameError(t *testing.T) {
	addr, cleanup := bindLoopback53(t, as112Config{Enabled: true})
	defer cleanup()

	resp := exchange(t, addr, "1.0.10.in-addr.arpa.", dns.TypePTR)
	if resp.Rcode != dns.RcodeNameError {
		t.Fatalf("Rcode = %v, want NXDOMAIN", dns.RcodeToString[resp.Rcode])
	}
	if !resp.Authoritative {
		t.Fatal("AA clear on a name inside a served zone, want an authoritative NXDOMAIN")
	}
	wantZoneSOA(t, resp, "10.in-addr.arpa.")
	if resp.RecursionAvailable {
		t.Fatal("RecursionAvailable = true, want false (AC-6)")
	}
}

// VALIDATES: AC-3 -- a real wire query for a name within empty.as112.arpa.
// gets NXDOMAIN with the zone SOA in Authority. The zone holds its SOA and NS
// at the apex and nothing below it (RFC 7534 Section 3.5 db.dr-empty), so a
// redirected name does not exist, and the SOA is what lets the resolver cache
// that: RFC 7535 Section 6, "The negative caching [RFC2308] of the CNAME
// target follows the parameters defined in the target zone, EMPTY.AS112.ARPA." The
// answer carries that zone's SOA.
func TestIntegration_EmptyAS112Arpa(t *testing.T) {
	addr, cleanup := bindLoopback53(t, as112Config{Enabled: true})
	defer cleanup()

	resp := exchange(t, addr, "foo.empty.as112.arpa.", dns.TypeA)
	if resp.Rcode != dns.RcodeNameError {
		t.Fatalf("Rcode = %v, want NXDOMAIN", dns.RcodeToString[resp.Rcode])
	}
	wantZoneSOA(t, resp, "empty.as112.arpa.")
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

// VALIDATES: AC-5 -- a real wire query outside every served zone is REFUSED
// with AA clear and nothing in Answer or Authority. The node holds no data for
// the name, so it makes no claim about it: RFC 1035 Section 4.1.1, "Refused -
// The name server refuses to perform the specified operation for policy
// reasons." A name outside every served zone gets that answer.
func TestIntegration_OutOfZoneRefused(t *testing.T) {
	addr, cleanup := bindLoopback53(t, as112Config{Enabled: true})
	defer cleanup()

	resp := exchange(t, addr, "example.com.", dns.TypeA)
	if resp.Rcode != dns.RcodeRefused {
		t.Fatalf("Rcode = %v, want REFUSED", dns.RcodeToString[resp.Rcode])
	}
	if resp.Authoritative {
		t.Fatal("AA set on a name under no served zone; the node claims authority it does not have")
	}
	if len(resp.Answer) != 0 || len(resp.Ns) != 0 {
		t.Fatalf("Answer=%v Ns=%v, want both empty for a refused name", resp.Answer, resp.Ns)
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

	// A dropped query never gets a reply, so exchange fails on its timeout;
	// an answered one carries the zone's own verdict on the name, which for a
	// name below the apex is NXDOMAIN with the zone SOA.
	resp := exchange(t, addr, "1.0.10.in-addr.arpa.", dns.TypePTR)
	if resp.Rcode != dns.RcodeNameError {
		t.Fatalf("loopback query Rcode = %v, want NXDOMAIN answered from the zone (H1/M4 carve-out)", dns.RcodeToString[resp.Rcode])
	}
	wantZoneSOA(t, resp, "10.in-addr.arpa.")
}
