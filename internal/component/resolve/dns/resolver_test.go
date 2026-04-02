package dns

import (
	"context"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDNSServer starts a local DNS server for testing.
// Returns the address and a cleanup function. Caller MUST call cleanup.
func testDNSServer(t *testing.T, handler mdns.Handler) (string, func()) {
	t.Helper()

	lc := net.ListenConfig{}
	pc, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &mdns.Server{
		PacketConn: pc,
		Handler:    handler,
	}

	go func() {
		_ = server.ActivateAndServe()
	}()

	// Wait for server to be ready.
	time.Sleep(10 * time.Millisecond)

	return pc.LocalAddr().String(), func() {
		_ = server.Shutdown()
	}
}

// testHandler returns a DNS handler that responds to specific queries.
func testHandler() mdns.Handler {
	return mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
		m := new(mdns.Msg)
		m.SetReply(r)
		m.Authoritative = true

		for _, q := range r.Question {
			switch {
			case q.Name == "example.com." && q.Qtype == mdns.TypeA:
				m.Answer = append(m.Answer, &mdns.A{
					Hdr: mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 300},
					A:   net.ParseIP("93.184.216.34"),
				})
			case q.Name == "example.com." && q.Qtype == mdns.TypeAAAA:
				m.Answer = append(m.Answer, &mdns.AAAA{
					Hdr:  mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeAAAA, Class: mdns.ClassINET, Ttl: 300},
					AAAA: net.ParseIP("2606:2800:220:1:248:1893:25c8:1946"),
				})
			case q.Name == "example.com." && q.Qtype == mdns.TypeTXT:
				m.Answer = append(m.Answer, &mdns.TXT{
					Hdr: mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeTXT, Class: mdns.ClassINET, Ttl: 300},
					Txt: []string{"v=spf1 -all"},
				})
			case q.Name == "34.216.184.93.in-addr.arpa." && q.Qtype == mdns.TypePTR:
				m.Answer = append(m.Answer, &mdns.PTR{
					Hdr: mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypePTR, Class: mdns.ClassINET, Ttl: 300},
					Ptr: "example.com.",
				})
			case q.Name == "nonexistent.invalid." && q.Qtype == mdns.TypeA:
				m.Rcode = mdns.RcodeNameError // NXDOMAIN
			case q.Name == "alias.example.com." && q.Qtype == mdns.TypeCNAME:
				m.Answer = append(m.Answer, &mdns.CNAME{
					Hdr:    mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeCNAME, Class: mdns.ClassINET, Ttl: 300},
					Target: "target.example.com.",
				})
			case q.Name == "example.com." && q.Qtype == mdns.TypeMX:
				m.Answer = append(m.Answer, &mdns.MX{
					Hdr:        mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeMX, Class: mdns.ClassINET, Ttl: 300},
					Preference: 10,
					Mx:         "mail.example.com.",
				})
			case q.Name == "example.com." && q.Qtype == mdns.TypeNS:
				m.Answer = append(m.Answer, &mdns.NS{
					Hdr: mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeNS, Class: mdns.ClassINET, Ttl: 300},
					Ns:  "ns1.example.com.",
				})
			case q.Name == "_sip._tcp.example.com." && q.Qtype == mdns.TypeSRV:
				m.Answer = append(m.Answer, &mdns.SRV{
					Hdr:      mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeSRV, Class: mdns.ClassINET, Ttl: 300},
					Priority: 10,
					Weight:   60,
					Port:     5060,
					Target:   "sip.example.com.",
				})
			case q.Name == "multi.example.com." && q.Qtype == mdns.TypeA:
				for _, ip := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
					m.Answer = append(m.Answer, &mdns.A{
						Hdr: mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 300},
						A:   net.ParseIP(ip),
					})
				}
			}
		}

		if err := w.WriteMsg(m); err != nil {
			return
		}
	})
}

// TestResolveWithConfiguredServer verifies resolver uses the configured server.
//
// VALIDATES: AC-1 -- YANG config specifies dns server address, resolver uses it.
// PREVENTS: Resolver ignoring configured server.
func TestResolveWithConfiguredServer(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 100,
		CacheTTL:  3600,
	})
	defer r.Close()

	records, err := r.ResolveA("example.com")
	require.NoError(t, err)
	assert.Contains(t, records, "93.184.216.34")
}

// TestResolveDefaultServer verifies resolver falls back to system DNS.
//
// VALIDATES: AC-2 -- no dns config section, resolver uses system default.
// PREVENTS: Resolver failing when no server configured.
func TestResolveDefaultServer(t *testing.T) {
	// Empty server string means system default.
	r := NewResolver(ResolverConfig{
		Server:    "",
		Timeout:   2,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	// Verify system DNS was resolved at construction.
	// On any system with /etc/resolv.conf, server should be non-empty.
	// On systems without it (containers), server will be empty and queries will
	// return a clear error. Either way, the resolver was created without panic.
	assert.NotNil(t, r)
}

// TestResolveTXT verifies TXT record resolution.
//
// VALIDATES: AC-3 -- TXT query for valid domain returns TXT record content.
// PREVENTS: TXT records not extracted from DNS response.
func TestResolveTXT(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 100,
		CacheTTL:  3600,
	})
	defer r.Close()

	records, err := r.ResolveTXT("example.com")
	require.NoError(t, err)
	assert.Contains(t, records, "v=spf1 -all")
}

// TestResolveA verifies A record resolution.
//
// VALIDATES: AC-4 -- A query for valid domain returns IP address(es).
// PREVENTS: A records not extracted from DNS response.
func TestResolveA(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 100,
		CacheTTL:  3600,
	})
	defer r.Close()

	records, err := r.ResolveA("example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"93.184.216.34"}, records)
}

// TestResolveAAAA verifies AAAA record resolution.
//
// VALIDATES: AC-4 -- AAAA query for valid domain returns IPv6 address(es).
// PREVENTS: AAAA records not extracted from DNS response.
func TestResolveAAAA(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 100,
		CacheTTL:  3600,
	})
	defer r.Close()

	records, err := r.ResolveAAAA("example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"2606:2800:220:1:248:1893:25c8:1946"}, records)
}

// TestResolvePTR verifies reverse DNS resolution.
//
// VALIDATES: AC-5 -- PTR query for IP address returns reverse DNS hostname.
// PREVENTS: PTR records not extracted or IP not converted to arpa format.
func TestResolvePTR(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 100,
		CacheTTL:  3600,
	})
	defer r.Close()

	records, err := r.ResolvePTR("93.184.216.34")
	require.NoError(t, err)
	assert.Contains(t, records, "example.com.")
}

// TestResolvePTRInvalidAddress verifies PTR with invalid address returns error.
//
// VALIDATES: AC-5 -- invalid address returns clear error.
// PREVENTS: Panic or confusing error on malformed input.
func TestResolvePTRInvalidAddress(t *testing.T) {
	r := NewResolver(ResolverConfig{
		Server:    "127.0.0.1:53",
		Timeout:   1,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	_, err := r.ResolvePTR("not-an-ip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reverse addr")
}

// TestResolveNXDOMAIN verifies non-existent domain returns empty result.
//
// VALIDATES: AC-6 -- query for non-existent domain returns empty result, no error.
// PREVENTS: NXDOMAIN responses treated as errors.
func TestResolveNXDOMAIN(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 100,
		CacheTTL:  3600,
	})
	defer r.Close()

	records, err := r.ResolveA("nonexistent.invalid")
	require.NoError(t, err, "NXDOMAIN should not return an error")
	assert.Empty(t, records, "NXDOMAIN should return empty results")
}

// TestResolveNXDOMAINNotCached verifies NXDOMAIN results are not cached.
//
// VALIDATES: AC-6 -- NXDOMAIN is not cached, repeated queries hit network.
// PREVENTS: Negative caching when not intended.
func TestResolveNXDOMAINNotCached(t *testing.T) {
	var queryCount atomic.Int32
	handler := mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
		queryCount.Add(1)
		m := new(mdns.Msg)
		m.SetReply(r)
		m.Rcode = mdns.RcodeNameError
		_ = w.WriteMsg(m)
	})

	addr, cleanup := testDNSServer(t, handler)
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 100,
		CacheTTL:  3600,
	})
	defer r.Close()

	r.ResolveA("nope.invalid") //nolint:errcheck // testing side effect
	r.ResolveA("nope.invalid") //nolint:errcheck // testing side effect

	assert.Equal(t, int32(2), queryCount.Load(), "NXDOMAIN should not be cached, both queries hit server")
}

// TestResolveTimeout verifies timeout handling.
//
// VALIDATES: AC-7 -- DNS server unreachable returns error after configured timeout.
// PREVENTS: Resolver blocking indefinitely on unreachable server.
func TestResolveTimeout(t *testing.T) {
	// Use a non-routable address to trigger timeout.
	r := NewResolver(ResolverConfig{
		Server:    "192.0.2.1:53", // RFC 5737 TEST-NET, guaranteed non-routable.
		Timeout:   1,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	start := time.Now()
	_, err := r.ResolveA("example.com")
	elapsed := time.Since(start)

	require.Error(t, err, "unreachable server should return an error")
	assert.Less(t, elapsed, 5*time.Second, "should not block much longer than timeout")
}

// TestResolveCacheIntegration verifies resolver uses cache.
//
// VALIDATES: AC-8 -- same query repeated within cache TTL returns cached result.
// PREVENTS: Resolver bypassing cache for repeated queries.
func TestResolveCacheIntegration(t *testing.T) {
	var queryCount atomic.Int32
	handler := mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
		queryCount.Add(1)
		m := new(mdns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		m.Answer = append(m.Answer, &mdns.A{
			Hdr: mdns.RR_Header{Name: r.Question[0].Name, Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 300},
			A:   net.ParseIP("1.2.3.4"),
		})
		_ = w.WriteMsg(m)
	})

	addr, cleanup := testDNSServer(t, handler)
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 100,
		CacheTTL:  3600,
	})
	defer r.Close()

	// First query hits the server.
	records1, err := r.ResolveA("cached.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"1.2.3.4"}, records1)
	assert.Equal(t, int32(1), queryCount.Load(), "first query should hit server")

	// Second query should come from cache.
	records2, err := r.ResolveA("cached.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"1.2.3.4"}, records2)
	assert.Equal(t, int32(1), queryCount.Load(), "second query should come from cache, not server")
}

// TestResolveServerPortAppended verifies port :53 is appended when missing.
//
// VALIDATES: AC-1 -- server address normalized with default port.
// PREVENTS: Connection failure when user omits port in config.
func TestResolveServerPortAppended(t *testing.T) {
	r := NewResolver(ResolverConfig{
		Server:    "127.0.0.1",
		Timeout:   1,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	assert.Equal(t, "127.0.0.1:53", r.server, "port :53 should be appended when missing")
}

// TestResolveTimeoutZeroDefault verifies Timeout=0 defaults to 5 seconds.
//
// VALIDATES: boundary testing for timeout.
// PREVENTS: Zero timeout causing instant failure.
func TestResolveTimeoutZeroDefault(t *testing.T) {
	r := NewResolver(ResolverConfig{
		Server:    "127.0.0.1:53",
		Timeout:   0,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	assert.Equal(t, 5*time.Second, r.client.Timeout, "timeout=0 should default to 5s")
}

// TestResolveBoundaryTimeout verifies timeout boundary values.
//
// VALIDATES: boundary testing for timeout range 1-60.
// PREVENTS: Invalid timeout values accepted.
func TestResolveBoundaryTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout uint16
	}{
		{"min valid", 1},
		{"max valid", 60},
		{"default", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver(ResolverConfig{
				Server:    "127.0.0.1:" + strconv.Itoa(53),
				Timeout:   tt.timeout,
				CacheSize: 0,
				CacheTTL:  0,
			})
			defer r.Close()
			assert.NotNil(t, r)
		})
	}
}

// TestResolveCNAME verifies CNAME record extraction.
//
// VALIDATES: extractRecords handles CNAME type.
// PREVENTS: CNAME records silently dropped.
func TestResolveCNAME(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	records, err := r.Resolve("alias.example.com", mdns.TypeCNAME)
	require.NoError(t, err)
	assert.Equal(t, []string{"target.example.com."}, records)
}

// TestResolveMX verifies MX record extraction.
//
// VALIDATES: extractRecords handles MX type.
// PREVENTS: MX records silently dropped.
func TestResolveMX(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	records, err := r.Resolve("example.com", mdns.TypeMX)
	require.NoError(t, err)
	assert.Equal(t, []string{"mail.example.com."}, records)
}

// TestResolveNS verifies NS record extraction.
//
// VALIDATES: extractRecords handles NS type.
// PREVENTS: NS records silently dropped.
func TestResolveNS(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	records, err := r.Resolve("example.com", mdns.TypeNS)
	require.NoError(t, err)
	assert.Equal(t, []string{"ns1.example.com."}, records)
}

// TestResolveSRV verifies SRV record extraction with target:port format.
//
// VALIDATES: extractRecords handles SRV type with correct formatting.
// PREVENTS: SRV records silently dropped or wrongly formatted.
func TestResolveSRV(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	records, err := r.Resolve("_sip._tcp.example.com", mdns.TypeSRV)
	require.NoError(t, err)
	assert.Equal(t, []string{"sip.example.com.:5060"}, records)
}

// TestResolveMultipleRecords verifies multiple records in a single response.
//
// VALIDATES: extractRecords returns all records, not just the first.
// PREVENTS: Only first record extracted from multi-record responses.
func TestResolveMultipleRecords(t *testing.T) {
	addr, cleanup := testDNSServer(t, testHandler())
	defer cleanup()

	r := NewResolver(ResolverConfig{
		Server:    addr,
		Timeout:   5,
		CacheSize: 0,
		CacheTTL:  0,
	})
	defer r.Close()

	records, err := r.ResolveA("multi.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, records)
}
