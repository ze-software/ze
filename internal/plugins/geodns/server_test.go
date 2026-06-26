package geodns

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
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

// VALIDATES: client IP comes from EDNS0 subnet, the packet source, or a
// fallback, per client-ip-source mode.
// PREVENTS: answering from the wrong customer view (e.g. CoreDNS's own IP).
func TestClientIPSourceModes(t *testing.T) {
	t.Parallel()
	packet := netip.MustParseAddr("9.9.9.9")
	withECS := subnetMsg("a.example.", dns.TypeA, "82.219.4.10")
	noECS := subnetMsg("a.example.", dns.TypeA, "")

	cases := []struct {
		name   string
		mode   string
		msg    *dns.Msg
		want   string
		wantOK bool
	}{
		{"edns0 present", "edns0", withECS, "82.219.4.10", true},
		{"edns0 absent", "edns0", noECS, "", false},
		{"packet ignores ecs", "packet", withECS, "9.9.9.9", true},
		{"edns0-then-packet uses ecs", "edns0-then-packet", withECS, "82.219.4.10", true},
		{"edns0-then-packet falls back", "edns0-then-packet", noECS, "9.9.9.9", true},
	}
	for _, tc := range cases {
		got, ok := clientIP(tc.msg, packet, tc.mode)
		if ok != tc.wantOK || (ok && got.String() != tc.want) {
			t.Errorf("%s: clientIP = (%v,%v), want (%q,%v)", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}

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
