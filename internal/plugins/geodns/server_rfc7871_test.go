// Design: docs/architecture/dns/server-harness.md -- geodns consumes the EDNS0
// client-subnet ADDRESS via dnsserver.ClientIP for source selection; this file
// pins geodns's RFC 7871 response-side behavior at the answerQuery entry point.
// RFC: rfc/short/rfc7871.md -- EDNS0 Client Subnet

package geodns

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

// rfc7871Peer is a minimal dnsserver.Peer exposing a fixed packet source, so an
// answerQuery call can exercise client-IP resolution (RFC 7871) without a socket.
type rfc7871Peer struct{ addr net.Addr }

func (p rfc7871Peer) RemoteAddr() net.Addr { return p.addr }

// responseHasECS reports whether msg carries an EDNS0 client-subnet option.
func responseHasECS(msg *dns.Msg) bool {
	opt := msg.IsEdns0()
	if opt == nil {
		return false
	}
	for _, o := range opt.Option {
		if _, ok := o.(*dns.EDNS0_SUBNET); ok {
			return true
		}
	}
	return false
}

// answerViaEntry drives geodns's answerQuery entry point against the published
// state, mimicking the dnsserver harness's msg.SetReply(r) shaping, and returns
// the response geodns built. The caller must have published state first.
func answerViaEntry(r *dns.Msg, src net.Addr) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetReply(r)
	answerQuery(msg, r, rfc7871Peer{addr: src})
	return msg
}

// VALIDATES: geodns emits no ECS option in its response even when the query
// carried none and geodns still produced a Tailored Response from the packet
// source (RFC 7871 sections 7.2.1 and 7.2.2). The negative polarity is
// unreachable: geodns adds no ECS option to any response (server.go
// answerQuestions builds only A/AAAA/SRV/SOA/NS records), so a response with an
// ECS option present when the query had none cannot be constructed -- annotated
// {single-polarity: positive} in rfc/short/rfc7871.md.
// PREVENTS: geodns fabricating an ECS option a downstream cache would treat as a
// scoped answer for a client that never asked for one.
func TestRFC7871_NoECSQueryNoECSResponse(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"geodns":{"enabled":"true","zone":["t.example."],` +
		`"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.5"]}}}},` +
		`"source":{"203.0.113.0/24":{"host-set":"web"}}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	storeApplied(cfg, 1)

	// No ECS option in the query; the packet source (203.0.113.7, inside the
	// configured /24) still selects the "web" host-set, so geodns returns a
	// Tailored Response and must nonetheless add no ECS option.
	r := subnetMsg("www.t.example.", dns.TypeA, "")
	msg := answerViaEntry(r, &net.UDPAddr{IP: net.ParseIP("203.0.113.7"), Port: 5353})

	if got := firstA(msg); got != "10.0.0.5" {
		t.Fatalf("packet-source Tailored Response A = %q, want 10.0.0.5 (a tailored answer was produced)", got)
	}
	// RFC requirement: RFC7871-7.2.1-7 positive -- a query without an ECS option
	// that still yields a Tailored Response draws a response with no ECS option.
	if responseHasECS(msg) {
		t.Error("response carries an ECS option though the query had none, with a Tailored Response")
	}
	// RFC requirement: RFC7871-7.2.2-1 positive -- a client query with no ECS
	// option draws a response with no ECS option.
	if responseHasECS(msg) {
		t.Error("response carries an ECS option though the client query had none")
	}
}

// VALIDATES: a query whose ECS option provides 0 address bits (SOURCE
// PREFIX-LENGTH 0, empty ADDRESS) is answered normally, never refused (RFC 7871
// section 7.5). The negative polarity is unreachable: geodns refuses a query only
// when the service is disabled (server.go answerQuery), never on ECS content, so
// a refusal driven solely by 0 address bits cannot be constructed -- annotated
// {single-polarity: positive} in rfc/short/rfc7871.md.
// PREVENTS: geodns rejecting a privacy-masked (0-bit) client-subnet query that
// should fall back to the packet source.
func TestRFC7871_ZeroAddressBitsNotRefused(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"geodns":{"enabled":"true","zone":["t.example."],` +
		`"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.5"]}}}},` +
		`"source":{"203.0.113.0/24":{"host-set":"web"}}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	storeApplied(cfg, 1)

	// ECS option supplying 0 address bits: SOURCE PREFIX-LENGTH 0 and an empty
	// ADDRESS. ClientIP finds no usable ECS network and falls back to the packet
	// source under the default edns0-then-packet mode.
	r := new(dns.Msg)
	r.SetQuestion("www.t.example.", dns.TypeA)
	opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	opt.SetUDPSize(4096)
	opt.Option = append(opt.Option, &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET, Family: 1, SourceNetmask: 0, SourceScope: 0, Address: net.IP{}})
	r.Extra = append(r.Extra, opt)

	msg := answerViaEntry(r, &net.UDPAddr{IP: net.ParseIP("203.0.113.7"), Port: 5353})

	// RFC requirement: RFC7871-7.5-6 positive -- a query providing 0 address bits
	// is answered normally, never refused; geodns refuses only when disabled.
	if msg.Rcode == dns.RcodeRefused {
		t.Errorf("query with 0 address bits was REFUSED; want a normal answer (rcode=%s)", dns.RcodeToString[msg.Rcode])
	}
	if got := firstA(msg); got != "10.0.0.5" {
		t.Errorf("0-bit ECS query fell back to packet source A = %q, want 10.0.0.5", got)
	}
}
