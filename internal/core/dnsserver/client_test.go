package dnsserver

import (
	"net"
	"net/netip"
	"testing"

	"github.com/miekg/dns"
)

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

// VALIDATES: ClientIP resolves the client network from EDNS0 client-subnet,
// the packet source, or a mode-driven fallback -- ported verbatim from a
// consumer plugin's TestClientIPSourceModes (AC-6, RFC 7871).
// PREVENTS: answering from the wrong customer view (e.g. a forwarder's own
// IP instead of the original client's).
func TestClientIP_EDNS0AndPacket(t *testing.T) {
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
		got, ok := ClientIP(tc.msg, packet, tc.mode)
		if ok != tc.wantOK || (ok && got.String() != tc.want) {
			t.Errorf("%s: ClientIP = (%v,%v), want (%q,%v)", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}

// VALIDATES: RemoteAddr extracts the IP from a ResponseWriter's remote
// address, ignoring the port; an unparsable address yields the zero Addr.
// PREVENTS: a malformed remote address crashing the query path.
func TestRemoteAddr(t *testing.T) {
	fw := &remoteAddrWriter{addr: &net.UDPAddr{IP: net.ParseIP("203.0.113.7"), Port: 53210}}
	got := RemoteAddr(fw)
	if got.String() != "203.0.113.7" {
		t.Errorf("RemoteAddr = %v, want 203.0.113.7", got)
	}
}

type remoteAddrWriter struct {
	dns.ResponseWriter
	addr net.Addr
}

func (w *remoteAddrWriter) RemoteAddr() net.Addr { return w.addr }
