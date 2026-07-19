package geodns

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// storeApplied builds and publishes a resolver snapshot with an explicit serial.
// Test-only helper for the in-process server.
func storeApplied(cfg geodnsConfig, serial uint32) {
	st := buildState(cfg)
	st.serial = serial
	storeState(st)
}

// VALIDATES: SOA serial generation per mode — auto-epoch is strictly monotonic
// (max(unix, prev+1)), auto-datetime is YYYYMMDDnn, fixed echoes the leaf.
// PREVENTS: a serial that stalls across sub-second reloads, or a datetime that
// overflows uint32.
func TestComputeSerial(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	unix := uint32(base.Unix())

	if s := computeSerial(soaConfig{SerialMode: "fixed", Serial: 2018122500}, 0, base); s != 2018122500 {
		t.Errorf("fixed serial = %d, want 2018122500", s)
	}
	if s := computeSerial(soaConfig{SerialMode: "auto-epoch"}, 0, base); s != unix {
		t.Errorf("epoch serial = %d, want %d", s, unix)
	}
	if s := computeSerial(soaConfig{SerialMode: "auto-epoch"}, unix, base); s != unix+1 {
		t.Errorf("epoch collision serial = %d, want %d", s, unix+1)
	}
	if s := computeSerial(soaConfig{SerialMode: "auto-datetime"}, 0, base); s != 2026062600 {
		t.Errorf("datetime serial = %d, want 2026062600", s)
	}
	if s := computeSerial(soaConfig{SerialMode: "auto-datetime"}, 2026062600, base); s != 2026062601 {
		t.Errorf("datetime same-day serial = %d, want 2026062601", s)
	}
}

func subnetMsg(qname string, qtype uint16, subnet string) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(qname), qtype)
	if subnet != "" {
		opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
		opt.SetUDPSize(4096)
		ip := net.ParseIP(subnet)
		fam := uint16(1)
		mask := uint8(32)
		if ip.To4() == nil {
			fam, mask = 2, 128
		}
		opt.Option = append(opt.Option, &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET, Family: fam, SourceNetmask: mask, Address: ip})
		m.Extra = append(m.Extra, opt)
	}
	return m
}

// test-relax: TestClientIPSourceModes unit-tested the package-local clientIP
// function directly. plan/learned/1027-dns-server-harness.md moved that
// function to dnsserver.ClientIP and explicitly directs the unit test to be
// "ported" there -- it now lives, verbatim in scenario coverage, as
// TestClientIP_EDNS0AndPacket in internal/core/dnsserver/client_test.go.
// geodns has no local function left to unit-test directly; the equivalent
// client-IP-driven source selection is still proven end-to-end over the wire
// by TestServerResolvesPerSource below, so this is a relocation of test
// coverage, not a removal of it.

// VALIDATES: nsID recognizes ns1..nsN.<zone> within the nameserver count.
// PREVENTS: serving glue for a non-existent nameserver index.
func TestNsID(t *testing.T) {
	t.Parallel()
	ns := []netip.Addr{netip.MustParseAddr("10.0.0.1")}
	zone := "geodns.example."
	if n := nsID("ns1.geodns.example.", zone, ns); n != 1 {
		t.Errorf("nsID(ns1) = %d, want 1", n)
	}
	if n := nsID("ns2.geodns.example.", zone, ns); n != 0 {
		t.Errorf("nsID(ns2) with 1 ns = %d, want 0", n)
	}
	if n := nsID("proxy.geodns.example.", zone, ns); n != 0 {
		t.Errorf("nsID(proxy) = %d, want 0", n)
	}
}

// VALIDATES: matchZone returns the longest configured zone suffix.
// PREVENTS: a query being answered under a broader zone than it belongs to.
func TestMatchZone(t *testing.T) {
	t.Parallel()
	zones := []string{"example.", "geodns.example."}
	if z := matchZone("proxy.geodns.example.", zones); z != "geodns.example." {
		t.Errorf("matchZone = %q, want geodns.example.", z)
	}
	if z := matchZone("host.other.", zones); z != "" {
		t.Errorf("matchZone(other) = %q, want empty", z)
	}
}

func resolveTestConfig(t *testing.T, port uint16) geodnsConfig {
	t.Helper()
	data := `{"service":{"geodns":{
		"enabled":"true",
		"zone":["test.example."],
		"nameserver":["127.0.0.1"],
		"host-set":{
			"internal":{"host":{"proxy.test.example.":{"address":["10.0.0.1"]}}},
			"external":{"host":{"proxy.test.example.":{"address":["10.0.0.2"]}}}
		},
		"source":{"82.219.0.0/16":{"host-set":"internal"},"0.0.0.0/0":{"host-set":"external"}}
	}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	cfg.Listeners = []listenerEndpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}
	return cfg
}

func freePort(t *testing.T) uint16 {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	udpAddr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("expected *net.UDPAddr, got %T", pc.LocalAddr())
	}
	port := uint16(udpAddr.Port)
	if cerr := pc.Close(); cerr != nil {
		t.Fatalf("close probe socket: %v", cerr)
	}
	return port
}

func queryA(t *testing.T, proto, addr, qname, subnet string) *dns.Msg {
	t.Helper()
	c := &dns.Client{Net: proto, Timeout: 2 * time.Second}
	resp, _, err := c.Exchange(subnetMsg(qname, dns.TypeA, subnet), addr)
	if err != nil {
		t.Fatalf("%s exchange: %v", proto, err)
	}
	return resp
}

// VALIDATES: end-to-end resolution over real UDP+TCP sockets — most-specific
// source wins, SOA query is synthesized, an unknown name is a NOERROR negative
// answer with SOA in authority, and reload swaps answers without rebinding.
// PREVENTS: the whole feature silently not working off-wire.
func TestServerResolvesPerSource(t *testing.T) {
	port := freePort(t)
	cfg := resolveTestConfig(t, port)
	storeApplied(cfg, 1)
	mgr := newServerManager(testLogger())
	if err := mgr.apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))

	for _, proto := range []string{"udp", "tcp"} {
		// internal source
		resp := queryA(t, proto, addr, "proxy.test.example.", "82.219.4.10")
		if got := firstA(resp); got != "10.0.0.1" {
			t.Errorf("%s internal A = %q, want 10.0.0.1", proto, got)
		}
		// external (catch-all)
		resp = queryA(t, proto, addr, "proxy.test.example.", "1.1.1.1")
		if got := firstA(resp); got != "10.0.0.2" {
			t.Errorf("%s external A = %q, want 10.0.0.2", proto, got)
		}
		// unknown name -> NOERROR + SOA authority
		resp = queryA(t, proto, addr, "nope.test.example.", "1.1.1.1")
		if resp.Rcode != dns.RcodeSuccess {
			t.Errorf("%s unknown rcode = %s, want NOERROR", proto, dns.RcodeToString[resp.Rcode])
		}
		if len(resp.Answer) != 0 || !hasSOA(resp.Ns) {
			t.Errorf("%s unknown: want empty answer + SOA authority, got answer=%v ns=%v", proto, resp.Answer, resp.Ns)
		}
		// SOA query for zone
		c := &dns.Client{Net: proto, Timeout: 2 * time.Second}
		soaResp, _, err := c.Exchange(subnetMsg("test.example.", dns.TypeSOA, "1.1.1.1"), addr)
		if err != nil {
			t.Fatalf("soa exchange: %v", err)
		}
		if !hasSOA(soaResp.Answer) {
			t.Errorf("%s SOA query: no SOA in answer; got %v", proto, soaResp.Answer)
		}
	}

	// reload: change external answer, same port -> no rebind, new answer
	data2 := `{"service":{"geodns":{"enabled":"true","zone":["test.example."],"nameserver":["127.0.0.1"],
		"host-set":{"external":{"host":{"proxy.test.example.":{"address":["10.9.9.9"]}}}},
		"source":{"0.0.0.0/0":{"host-set":"external"}}}}}`
	cfg2, err := parseConfig(data2)
	if err != nil {
		t.Fatalf("parseConfig reload: %v", err)
	}
	cfg2.Listeners = cfg.Listeners
	storeApplied(cfg2, 2)
	if err := mgr.apply(cfg2); err != nil {
		t.Fatalf("apply reload: %v", err)
	}
	resp := queryA(t, "udp", addr, "proxy.test.example.", "1.1.1.1")
	if got := firstA(resp); got != "10.9.9.9" {
		t.Errorf("after reload A = %q, want 10.9.9.9", got)
	}
}

func firstA(m *dns.Msg) string {
	for _, rr := range m.Answer {
		if a, ok := rr.(*dns.A); ok {
			return a.A.String()
		}
	}
	return ""
}

func hasSOA(rrs []dns.RR) bool {
	for _, rr := range rrs {
		if _, ok := rr.(*dns.SOA); ok {
			return true
		}
	}
	return false
}

// answerState builds and publishes nothing; it returns an in-process resolver
// snapshot for driving answerQuestions without a socket.
func answerState(t *testing.T, jsonCfg string) *resolverState {
	t.Helper()
	cfg, err := parseConfig(jsonCfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	st := buildState(cfg)
	st.serial = 1
	return st
}

// TestRFC2181_UDPReplySourceAndPort verifies that a UDP reply is sourced from the
// address and port the query was sent to, and directed back to the query's source
// port, over a real socket.
//
// VALIDATES: RFC 2181 sections 4.1 and 4.2 -- the reply's source IP is the query's
// destination address, replies leave from the port they were sent to, and the
// query's source port is used as the reply's destination port.
// PREVENTS: a reply sourced from the wrong address/port being dropped by the client.
func TestRFC2181_UDPReplySourceAndPort(t *testing.T) {
	port := freePort(t)
	cfg := resolveTestConfig(t, port) // listener bound to 127.0.0.1:port
	storeApplied(cfg, 1)
	mgr := newServerManager(testLogger())
	if err := mgr.apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)

	serverAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)}
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("client socket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	q := subnetMsg("proxy.test.example.", dns.TypeA, "1.1.1.1")
	q.Id = 0x2181
	packed, err := q.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}
	if _, err := conn.WriteToUDP(packed, serverAddr); err != nil {
		t.Fatalf("send query: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 4096)
	n, src, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}

	// RFC requirement: RFC2181-4.1-1 positive -- the reply's source IP equals the
	// address the query was sent to (127.0.0.1), because the listener binds it.
	if !src.IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("reply source IP = %s, want 127.0.0.1 (the query destination address)", src.IP)
	}
	// RFC requirement: RFC2181-4.2-1 positive -- the reply is directed from the port
	// the query was sent to (the server port).
	if src.Port != int(port) {
		t.Errorf("reply source port = %d, want %d (the server port)", src.Port, port)
	}
	// RFC requirement: RFC2181-4.2-2 positive -- the server used the query's UDP
	// source port as the reply destination, so the datagram arrived on this client
	// socket and matches our query id.
	var reply dns.Msg
	if err := reply.Unpack(buf[:n]); err != nil {
		t.Fatalf("unpack reply: %v", err)
	}
	if reply.Id != q.Id {
		t.Errorf("reply id = %d, want %d", reply.Id, q.Id)
	}
	if got := firstA(&reply); got != "10.0.0.2" {
		t.Errorf("reply A = %q, want 10.0.0.2 (external host-set)", got)
	}
}

// TestRFC2181_RRSetEqualTTL verifies every record in an A RRSet geodns emits for
// one host carries an identical TTL.
//
// VALIDATES: RFC 2181 section 5.2 -- the TTLs of all RRs in an RRSet must be equal;
// a server must never send an RRSet with unequal TTLs.
// PREVENTS: a multi-address host emitting records with differing TTLs.
func TestRFC2181_RRSetEqualTTL(t *testing.T) {
	t.Parallel()
	st := answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+
		`"host-set":{"web":{"host":{"www.t.example.":{"ttl":"120","address":["10.0.0.1","10.0.0.2","10.0.0.3"]}}}},`+
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)

	r := new(dns.Msg)
	r.SetQuestion("www.t.example.", dns.TypeA)
	msg := new(dns.Msg)
	msg.SetReply(r)
	answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))

	var ttls []uint32
	for _, rr := range msg.Answer {
		if a, ok := rr.(*dns.A); ok {
			ttls = append(ttls, a.Hdr.Ttl)
		}
	}
	if len(ttls) < 2 {
		t.Fatalf("want a multi-record A RRSet, got %d A records", len(ttls))
	}
	// RFC requirement: RFC2181-5.2-1 positive -- every A record in the RRSet shares
	// the single configured TTL of 120 seconds.
	for i, ttl := range ttls {
		if ttl != ttls[0] {
			t.Errorf("A record %d TTL = %d, want %d (all equal)", i, ttl, ttls[0])
		}
	}
	if ttls[0] != 120 {
		t.Errorf("RRSet TTL = %d, want the configured 120", ttls[0])
	}
}

// TestRFC2181_NSCanonicalWithGlue verifies geodns's synthesized NS targets are
// canonical names (never aliases/CNAMEs) and carry A glue.
//
// VALIDATES: RFC 2181 section 10.3 -- an NS/MX value must not be an alias and must
// never have a CNAME RR, and that name must have one or more address records.
// PREVENTS: an NS record pointing at an alias, or lacking address records.
func TestRFC2181_NSCanonicalWithGlue(t *testing.T) {
	t.Parallel()
	st := answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],"nameserver":["10.0.0.1","10.0.0.2"]}}}`)

	r := new(dns.Msg)
	r.SetQuestion("t.example.", dns.TypeNS)
	msg := new(dns.Msg)
	msg.SetReply(r)
	answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))

	var nsTargets []string
	for _, rr := range msg.Answer {
		switch v := rr.(type) {
		case *dns.NS:
			nsTargets = append(nsTargets, v.Ns)
		case *dns.CNAME:
			t.Fatalf("NS answer contains a CNAME %q; an alias must never appear here", v.Target)
		}
	}
	if len(nsTargets) != 2 {
		t.Fatalf("want 2 NS records, got %d (%v)", len(nsTargets), nsTargets)
	}
	// RFC requirement: RFC2181-10.3-1 positive -- each NS target is a canonical
	// ns<n>.<zone> name, never an alias/CNAME.
	for _, ns := range nsTargets {
		if !strings.HasPrefix(ns, "ns") || !strings.HasSuffix(ns, ".t.example.") {
			t.Errorf("NS target %q is not a canonical ns<n>.<zone> name", ns)
		}
	}

	glue := map[string]bool{}
	for _, rr := range msg.Extra {
		switch v := rr.(type) {
		case *dns.A:
			glue[v.Hdr.Name] = true
		case *dns.CNAME:
			t.Errorf("glue for %q is a CNAME; an NS target must resolve via address records", v.Hdr.Name)
		}
	}
	// RFC requirement: RFC2181-10.3-2 positive -- every NS target has an A (address)
	// glue record.
	for _, ns := range nsTargets {
		if !glue[ns] {
			t.Errorf("NS target %q has no A glue record; glue=%v", ns, glue)
		}
	}
}

// TestRFC2181_WireNameLimits verifies geodns's emitted names respect the DNS wire
// limits and that the codec ze writes through rejects an over-limit label.
//
// VALIDATES: RFC 2181 section 11 -- any one label is limited to 1..63 octets and a
// full domain name to 255 octets.
// PREVENTS: geodns emitting a name the wire codec cannot represent.
func TestRFC2181_WireNameLimits(t *testing.T) {
	t.Parallel()
	st := answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+
		`"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.1"]}}}},`+
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)

	r := new(dns.Msg)
	r.SetQuestion("www.t.example.", dns.TypeA)
	msg := new(dns.Msg)
	msg.SetReply(r)
	answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))

	// RFC requirement: RFC2181-11-1 positive -- geodns's synthesized answer packs
	// through the wire codec, and every emitted name obeys the 1..63 octet label and
	// 255 octet name limits.
	packed, err := msg.Pack()
	if err != nil {
		t.Fatalf("geodns answer failed to pack: %v", err)
	}
	if len(packed) == 0 {
		t.Fatal("packed message is empty")
	}
	if len(msg.Answer) == 0 {
		t.Fatal("no answer records to check")
	}
	for _, rr := range msg.Answer {
		name := rr.Header().Name
		if len(name) > 255 {
			t.Errorf("name %q exceeds 255 octets", name)
		}
		for label := range strings.SplitSeq(strings.TrimSuffix(name, "."), ".") {
			if l := len(label); l < 1 || l > 63 {
				t.Errorf("label %q in %q is %d octets, must be 1..63", label, name, l)
			}
		}
	}

	// The same codec refuses a 64-octet label, so an out-of-bounds name can never
	// reach the wire from ze.
	over := new(dns.Msg)
	over.SetQuestion(strings.Repeat("a", 64)+".t.example.", dns.TypeA)
	if _, err := over.Pack(); err == nil {
		t.Error("a 64-octet label packed; the codec did not enforce the 63-octet limit")
	}
}
