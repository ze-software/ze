// Design: docs/architecture/dns/geodns.md -- the client-subnet option geodns
// validates and echoes as an RFC 7871 Authoritative Nameserver.
// RFC: rfc/short/rfc7871.md -- EDNS0 Client Subnet
// Related: rfc7871_server_test.go -- answerViaEntry, rfc7871Peer

package geodns

import (
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// echoConfig serves www.t.example. from the host set "web" to 203.0.113.0/24
// and 2001:db8::/32, so a query whose option names either draws a tailored A.
const echoConfig = `{"service":{"geodns":{"enabled":"true","zone":["t.example."],` +
	`"nameserver":["192.0.2.53"],` +
	`"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.5"]}}}},` +
	`"source":{"203.0.113.0/24":{"host-set":"web"},"2001:db8::/32":{"host-set":"web"}}}}}`

// storeEchoConfig parses raw, with client-ip-source set to mode when mode is
// not empty, and publishes it as the state answerQuery reads.
func storeEchoConfig(t *testing.T, raw, mode string) {
	t.Helper()
	cfg, err := parseConfig(raw)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if mode != "" {
		cfg.ClientIPSource = mode
	}
	storeApplied(cfg, 1)
}

// ecsQuery builds a query for qname and qtype carrying the client-subnet
// options given, in one OPT record. No option gives a query with no OPT.
func ecsQuery(qname string, qtype uint16, subnets ...dns.EDNS0) *dns.Msg {
	r := new(dns.Msg)
	r.SetQuestion(qname, qtype)
	if len(subnets) == 0 {
		return r
	}
	opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	opt.SetUDPSize(4096)
	opt.Option = subnets
	r.Extra = append(r.Extra, opt)
	return r
}

// subnet builds a client-subnet option from its four fields.
func subnet(family uint16, source, scope uint8, address string) *dns.EDNS0_SUBNET {
	var ip net.IP
	if address != "" {
		ip = net.ParseIP(address)
	}
	return &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET, Family: family, SourceNetmask: source, SourceScope: scope, Address: ip}
}

// v4Packet is the packet source the in-process queries arrive from. It lies
// inside 203.0.113.0/24, so edns0-then-packet selects "web" from it too.
var v4Packet = &net.UDPAddr{IP: net.ParseIP("203.0.113.7"), Port: 5353}

// onWire answers r through answerQuery, then packs and unpacks the reply, so
// what the test reads is what the client would read off the socket.
func onWire(t *testing.T, r *dns.Msg) *dns.Msg {
	t.Helper()
	wire, err := answerViaEntry(r, v4Packet).Pack()
	if err != nil {
		t.Fatalf("pack reply: %v", err)
	}
	back := new(dns.Msg)
	if err := back.Unpack(wire); err != nil {
		t.Fatalf("unpack reply: %v", err)
	}
	return back
}

// replySubnet returns the reply's client-subnet option, or nil.
func replySubnet(msg *dns.Msg) *dns.EDNS0_SUBNET {
	opt := msg.IsEdns0()
	if opt == nil {
		return nil
	}
	for _, o := range opt.Option {
		if s, ok := o.(*dns.EDNS0_SUBNET); ok {
			return s
		}
	}
	return nil
}

// VALIDATES: the reply to a query carrying a client-subnet option carries one,
// whose FAMILY, SOURCE PREFIX-LENGTH and ADDRESS are the query's, for an IPv4
// and an IPv6 option (RFC 7871 Sections 7.2.1 and 11.2). Method: two queries
// with different options, each reply packed and unpacked, each echo compared
// with its own query, so a constant echo cannot pass.
// PREVENTS: geodns tailoring an answer from the option and giving the resolver
// no option to cache it by.
func TestRFC7871_ECSQueryEchoesOption(t *testing.T) {
	storeEchoConfig(t, echoConfig, "")

	for _, query := range []*dns.EDNS0_SUBNET{
		subnet(1, 24, 0, "203.0.113.0"),
		subnet(2, 40, 0, "2001:db8:1200::"),
	} {
		reply := onWire(t, ecsQuery("www.t.example.", dns.TypeA, query))
		if got := firstA(reply); got != "10.0.0.5" {
			t.Fatalf("option %s: A = %q, want the tailored 10.0.0.5", query, got)
		}
		echo := replySubnet(reply)
		// RFC requirement: RFC7871-7.2.1-5 positive -- a query carrying an ECS option, IPv4 or IPv6, draws a reply that carries an ECS option.
		if echo == nil {
			t.Fatalf("option %s: the reply carries no ECS option", query)
		}
		// RFC requirement: RFC7871-7.2.1-8 positive -- the reply's FAMILY, SOURCE PREFIX-LENGTH and ADDRESS equal the query's, for two queries whose options differ in all three.
		// RFC requirement: RFC7871-11.2-1 positive -- the ECS option in the reply carries the full FAMILY, ADDRESS and SOURCE PREFIX-LENGTH of the query it answers.
		if echo.Family != query.Family || echo.SourceNetmask != query.SourceNetmask || !echo.Address.Equal(query.Address) {
			t.Errorf("echo %s family %d, want the query's %s family %d", echo, echo.Family, query, query.Family)
		}
	}
}

// VALIDATES: no client-subnet option is echoed to a query that carried none,
// and none under client-ip-source packet, which does not read the option
// (RFC 7871 Section 7.2.1). Method: the tailored A query of the test above,
// once without an option and once with one under packet.
// PREVENTS: an echo that does not depend on the query, and a server that has
// not enabled ECS answering as though it had.
func TestRFC7871_NoEchoWithoutECSOrInPacketMode(t *testing.T) {
	storeEchoConfig(t, echoConfig, "")
	bare := onWire(t, ecsQuery("www.t.example.", dns.TypeA))
	// RFC requirement: RFC7871-7.2.1-5 negative -- a query carrying no ECS option draws a reply with no ECS option, though the answer is tailored.
	if replySubnet(bare) != nil || firstA(bare) != "10.0.0.5" {
		t.Errorf("ECS-less query: echo %v, A %q; want no option and the tailored 10.0.0.5", replySubnet(bare), firstA(bare))
	}

	storeEchoConfig(t, echoConfig, clientIPSourcePacket)
	ignored := onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(1, 24, 0, "203.0.113.0")))
	// RFC requirement: RFC7871-7.2.1-2 positive -- with client-ip-source packet, which does not read the option, a query carrying an ECS option draws a reply with no ECS option.
	if echo := replySubnet(ignored); echo != nil {
		t.Errorf("packet mode echoed %s; a server that does not read the option must not include one", echo)
	}
}

// VALIDATES: every reply geodns gives to a query carrying a client-subnet
// option carries one: the tailored answer, the apex SOA and NS, a name error,
// a no-data answer and a refusal for a name under no zone (RFC 7871 Section
// 12.1). Method: the same six queries with and without an option.
// PREVENTS: a resolver deciding from one untailored reply that geodns does not
// support ECS and dropping the option from its later queries.
func TestRFC7871_EveryReplyToAnECSQueryCarriesTheOption(t *testing.T) {
	storeEchoConfig(t, echoConfig, "")
	queries := []struct {
		name  string
		qtype uint16
	}{
		{"www.t.example.", dns.TypeA},
		{"t.example.", dns.TypeSOA},
		{"t.example.", dns.TypeNS},
		{"nope.t.example.", dns.TypeA},
		{"www.t.example.", dns.TypeMX},
		{"www.other.example.", dns.TypeA},
	}
	for _, q := range queries {
		with := onWire(t, ecsQuery(q.name, q.qtype, subnet(1, 24, 0, "203.0.113.0")))
		// RFC requirement: RFC7871-12.1-4 positive -- each of six replies to an ECS query (tailored A, apex SOA, apex NS, NXDOMAIN, no-data, REFUSED) carries an ECS option.
		if replySubnet(with) == nil {
			t.Errorf("%s %s (rcode %s): the reply to an ECS query carries no ECS option",
				q.name, dns.TypeToString[q.qtype], dns.RcodeToString[with.Rcode])
		}
		without := onWire(t, ecsQuery(q.name, q.qtype))
		// RFC requirement: RFC7871-12.1-4 negative -- the same six queries without an ECS option draw replies with no ECS option.
		if echo := replySubnet(without); echo != nil {
			t.Errorf("%s %s: the reply to a query with no option carries %s", q.name, dns.TypeToString[q.qtype], echo)
		}
	}
}

// VALIDATES: SCOPE PREFIX-LENGTH is 0 when the answer did not come from the
// option: an apex SOA, and a tailored A chosen from the packet source for an
// option with SOURCE PREFIX-LENGTH 0; it is the matched /24 when the A came
// from the option's ADDRESS (RFC 7871 Section 12.1).
// PREVENTS: a resolver caching an answer for one network as though it held for
// every client, or the reverse.
func TestRFC7871_ScopeZeroWhenTheOptionIsUnused(t *testing.T) {
	storeEchoConfig(t, echoConfig, "")

	soa := replySubnet(onWire(t, ecsQuery("t.example.", dns.TypeSOA, subnet(1, 24, 0, "203.0.113.0"))))
	masked := onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(1, 0, 0, "")))
	if firstA(masked) != "10.0.0.5" {
		t.Fatalf("/0 option: A = %q, want 10.0.0.5 chosen from the packet source", firstA(masked))
	}
	// RFC requirement: RFC7871-12.1-5 positive -- SCOPE PREFIX-LENGTH is 0 in the reply to an ECS query for the apex SOA, and to a /0 ECS query whose A geodns chose from the packet source.
	if soa == nil || soa.SourceScope != 0 || replySubnet(masked) == nil || replySubnet(masked).SourceScope != 0 {
		t.Errorf("SOA echo %v, /0 echo %v; want both with SCOPE PREFIX-LENGTH 0", soa, replySubnet(masked))
	}

	used := replySubnet(onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(1, 24, 0, "203.0.113.0"))))
	// RFC requirement: RFC7871-12.1-5 negative -- when the A comes from the option's ADDRESS, SCOPE PREFIX-LENGTH is the matched source prefix length, 24, not 0.
	if used == nil || used.SourceScope != 24 {
		t.Errorf("tailored echo %v; want SCOPE PREFIX-LENGTH 24, the matched 203.0.113.0/24", used)
	}
}

// VALIDATES: the scope geodns returns never covers an address another source
// prefix answers. With 10.0.0.0/16 to "web" and the exception 10.0.5.0/24 to
// "lab", a query for 10.0.1.0/24 draws web's A with scope 22, the shortest
// network round 10.0.1.0 that leaves 10.0.5.0/24 out; with no exception the
// scope is the /16 itself (RFC 7871 Section 7.2.1).
// PREVENTS: a resolver caching web's answer for all of 10.0.0.0/16 and handing
// it to the 10.0.5.0/24 clients the exception exists for.
func TestRFC7871_ScopeOverlapsNoOtherPrefix(t *testing.T) {
	sets := `"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.5"]}}},` +
		`"lab":{"host":{"www.t.example.":{"address":["10.0.0.9"]}}}},`
	storeEchoConfig(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+sets+
		`"source":{"10.0.0.0/16":{"host-set":"web"},"10.0.5.0/24":{"host-set":"lab"}}}}}`, "")

	wide := onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(1, 24, 0, "10.0.1.0")))
	echo := replySubnet(wide)
	if firstA(wide) != "10.0.0.5" || echo == nil {
		t.Fatalf("10.0.1.0/24: A %q echo %v; want web's 10.0.0.5 and an echo", firstA(wide), echo)
	}
	scope := netip.PrefixFrom(netip.MustParseAddr("10.0.1.0"), int(echo.SourceScope)).Masked()
	// RFC requirement: RFC7871-7.2.1-12 positive -- with 10.0.0.0/16 and the exception 10.0.5.0/24 configured, the answer for 10.0.1.0/24 carries SCOPE PREFIX-LENGTH 22, a network that does not overlap 10.0.5.0/24.
	if echo.SourceScope != 22 || scope.Overlaps(netip.MustParsePrefix("10.0.5.0/24")) {
		t.Errorf("scope %s overlaps the 10.0.5.0/24 exception; want 10.0.0.0/22", scope)
	}

	storeEchoConfig(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+sets+
		`"source":{"10.0.0.0/16":{"host-set":"web"}}}}}`, "")
	alone := replySubnet(onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(1, 24, 0, "10.0.1.0"))))
	// RFC requirement: RFC7871-7.2.1-12 negative -- with 10.0.0.0/16 alone, no prefix lies inside it, and the same query carries SCOPE PREFIX-LENGTH 16.
	if alone == nil || alone.SourceScope != 16 {
		t.Errorf("echo %v with no exception configured; want SCOPE PREFIX-LENGTH 16", alone)
	}
}

// VALIDATES: a query whose option gives SOURCE PREFIX-LENGTH 0 and FAMILY 0
// (the `dig +subnet=0` form) draws an echo whose FAMILY is that of the packet
// source geodns answered from, while a query that gave FAMILY 2 keeps FAMILY 2
// (RFC 7871 Section 7.3). The ADDRESS has no octets either way: SOURCE
// PREFIX-LENGTH 0 truncates it to none.
// PREVENTS: an echo that tells a resolver nothing about the family the answer
// was chosen for.
func TestRFC7871_PrivacyMaskedQueryGetsTheFamilyUsed(t *testing.T) {
	storeEchoConfig(t, echoConfig, "")

	filled := replySubnet(onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(0, 0, 0, ""))))
	// RFC requirement: RFC7871-7.3-4 positive -- a FAMILY 0, SOURCE PREFIX-LENGTH 0 ECS query answered from an IPv4 packet source draws an echo with FAMILY 1 and SOURCE PREFIX-LENGTH 0.
	if filled == nil || filled.Family != 1 || filled.SourceNetmask != 0 {
		t.Errorf("echo %v; want FAMILY 1, the family of the packet source used", filled)
	}

	kept := replySubnet(onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(2, 0, 0, ""))))
	// RFC requirement: RFC7871-7.3-4 negative -- a query that gave FAMILY 2 with SOURCE PREFIX-LENGTH 0 keeps FAMILY 2 in the echo, though the packet source is IPv4.
	if kept == nil || kept.Family != 2 {
		t.Errorf("echo %v; want the query's FAMILY 2 kept", kept)
	}
}

// VALIDATES: a query whose client-subnet option is wrongly formatted draws
// FORMERR with nothing in the Answer and no option: an unknown FAMILY, a
// non-zero SCOPE PREFIX-LENGTH, ADDRESS bits past SOURCE PREFIX-LENGTH, a
// SOURCE PREFIX-LENGTH wider than the family, and two options in one query. A
// well-formed option is answered, and under client-ip-source packet a
// malformed one is ignored (RFC 7871 Sections 6 and 7.2.1).
// PREVENTS: geodns tailoring an answer from an option it cannot read, or
// refusing what a server that does not read the option ought to ignore.
func TestRFC7871_MalformedOptionDrawsFormErr(t *testing.T) {
	storeEchoConfig(t, echoConfig, "")
	malformed := []struct {
		name    string
		options []dns.EDNS0
	}{
		{"unknown FAMILY", []dns.EDNS0{subnet(3, 0, 0, "")}},
		{"SCOPE PREFIX-LENGTH in a query", []dns.EDNS0{subnet(1, 24, 8, "203.0.113.0")}},
		{"ADDRESS bits past SOURCE PREFIX-LENGTH", []dns.EDNS0{subnet(1, 24, 0, "203.0.113.7")}},
		{"SOURCE PREFIX-LENGTH wider than IPv4", []dns.EDNS0{subnet(1, 33, 0, "203.0.113.0")}},
		{"two options", []dns.EDNS0{subnet(1, 24, 0, "203.0.113.0"), subnet(1, 16, 0, "203.0.0.0")}},
	}
	for _, tc := range malformed {
		reply := onWire(t, ecsQuery("www.t.example.", dns.TypeA, tc.options...))
		// RFC requirement: RFC7871-7.2.1-3 positive -- a query whose ECS option is wrongly formatted (unknown FAMILY, non-zero SCOPE PREFIX-LENGTH, ADDRESS bits past SOURCE PREFIX-LENGTH, SOURCE PREFIX-LENGTH wider than the family, two options) is rejected: no Answer and no ECS option.
		if len(reply.Answer) != 0 || replySubnet(reply) != nil {
			t.Errorf("%s: answer %v echo %v; want the query rejected", tc.name, reply.Answer, replySubnet(reply))
		}
		// RFC requirement: RFC7871-7.2.1-4 positive -- the reply to each of those five wrongly formatted queries has RCODE FORMERR.
		if reply.Rcode != dns.RcodeFormatError {
			t.Errorf("%s: rcode %s, want FORMERR", tc.name, dns.RcodeToString[reply.Rcode])
		}
	}

	good := onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(1, 24, 0, "203.0.113.0")))
	// RFC requirement: RFC7871-7.2.1-4 negative -- a well-formed ECS option is answered NOERROR with the tailored A, not FORMERR.
	if good.Rcode != dns.RcodeSuccess || firstA(good) != "10.0.0.5" {
		t.Errorf("well-formed option: rcode %s A %q; want NOERROR and 10.0.0.5", dns.RcodeToString[good.Rcode], firstA(good))
	}

	storeEchoConfig(t, echoConfig, clientIPSourcePacket)
	ignored := onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(1, 24, 8, "203.0.113.0")))
	// RFC requirement: RFC7871-7.2.1-3 negative -- with client-ip-source packet, which does not read the option, a query whose option has a non-zero SCOPE PREFIX-LENGTH is answered NOERROR from the packet source, not rejected.
	if ignored.Rcode != dns.RcodeSuccess || firstA(ignored) != "10.0.0.5" {
		t.Errorf("packet mode: rcode %s A %q; want the option ignored", dns.RcodeToString[ignored.Rcode], firstA(ignored))
	}
}

// VALIDATES: over a real UDP socket, a query whose option carries a non-zero
// SCOPE PREFIX-LENGTH draws FORMERR from geodns, and the same option with SCOPE
// PREFIX-LENGTH 0 is answered. A query whose option has an unknown FAMILY draws
// FORMERR too: miekg/dns refuses that option while unpacking and its server
// answers FORMERR before geodns runs, which is the stack's answer on the wire.
// The options are sent as raw bytes, so no client-side check rewrites them.
// PREVENTS: the FORMERR path existing only for in-process messages.
func TestRFC7871_MalformedOptionOnTheWire(t *testing.T) {
	port := freePort(t)
	cfg := resolveTestConfig(t, port)
	storeApplied(cfg, 1)
	mgr := newServerManager(testLogger())
	if err := mgr.reconcile(cfg, dnsserver.BindLenient); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))

	exchange := func(data []byte) *dns.Msg {
		t.Helper()
		raw := &dns.EDNS0_LOCAL{Code: dns.EDNS0SUBNET, Data: data}
		c := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
		resp, _, err := c.Exchange(ecsQuery("proxy.test.example.", dns.TypeA, raw), addr)
		if err != nil {
			t.Fatalf("exchange: %v", err)
		}
		return resp
	}

	// FAMILY 1, SOURCE PREFIX-LENGTH 24, SCOPE PREFIX-LENGTH 8, ADDRESS 82.219.4.
	scoped := exchange([]byte{0, 1, 24, 8, 82, 219, 4})
	// RFC requirement: RFC7871-7.2.1-4 positive -- over UDP, a query whose ECS option has SCOPE PREFIX-LENGTH 8 draws RCODE FORMERR.
	if scoped.Rcode != dns.RcodeFormatError {
		t.Errorf("SCOPE PREFIX-LENGTH 8: rcode %s, want FORMERR", dns.RcodeToString[scoped.Rcode])
	}
	answered := exchange([]byte{0, 1, 24, 0, 82, 219, 4})
	if answered.Rcode == dns.RcodeFormatError || replySubnet(answered) == nil {
		t.Errorf("well-formed option: rcode %s echo %v; want an answer with an echo",
			dns.RcodeToString[answered.Rcode], replySubnet(answered))
	}
	// FAMILY 3, which RFC 7871 Section 6 defines no address format for.
	if unknown := exchange([]byte{0, 3, 0, 0}); unknown.Rcode != dns.RcodeFormatError {
		t.Errorf("unknown FAMILY: rcode %s, want FORMERR", dns.RcodeToString[unknown.Rcode])
	}
}
