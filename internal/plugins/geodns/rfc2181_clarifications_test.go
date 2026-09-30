// Design: docs/architecture/dns/geodns.md -- RFC 2181 clarifications GeoDNS must honor

package geodns

import (
	"encoding/binary"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// VALIDATES: RFC 2181 section 4.1 -- a UDP reply leaves from the address the
// query was sent to, on a listener that is NOT the loopback primary. The query
// goes to 127.0.0.2 from a client on 127.0.0.1; a server that let the kernel
// pick the source from the route back to 127.0.0.1 would answer from
// 127.0.0.1, so the assertion discriminates.
// PREVENTS: a multi-homed server answering from an address the client never
// queried, which the client then drops.
func TestRFC2181ReplySourcedFromTheQueriedAddress(t *testing.T) {
	port := freePort(t)
	cfg := resolveTestConfig(t, port)
	cfg.Listeners = []listenerEndpoint{{IP: netip.MustParseAddr("127.0.0.2"), Port: port}}
	storeApplied(cfg, 1)
	mgr := newServerManager(testLogger())
	if err := mgr.apply(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	t.Cleanup(mgr.stopAll)

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("client socket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	q := subnetMsg("proxy.test.example.", dns.TypeA, "1.1.1.1")
	packed, err := q.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}
	if _, err := conn.WriteToUDP(packed, &net.UDPAddr{IP: net.ParseIP("127.0.0.2"), Port: int(port)}); err != nil {
		t.Fatalf("send query: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 4096)
	_, source, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	// RFC requirement: RFC2181-4.1-1 positive -- a UDP query sent to 127.0.0.2 from 127.0.0.1 is answered with IP source 127.0.0.2, the query's destination, not the 127.0.0.1 the route back would pick.
	if !source.IP.Equal(net.ParseIP("127.0.0.2")) {
		t.Errorf("reply source IP = %s, want 127.0.0.2 (the query's destination address)", source.IP)
	}
}

// VALIDATES: RFC 2181 section 10.3 -- every NS target GeoDNS names is itself
// answered as a name with address records: asked for a CNAME it returns none,
// asked for its A record it returns the configured nameserver address.
// PREVENTS: an NS record whose target is an alias, which resolvers may refuse.
func TestRFC2181NSTargetIsNotAnAlias(t *testing.T) {
	t.Parallel()
	st := answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],"nameserver":["10.0.0.1","10.0.0.2"]}}}`)
	ask := func(name string, qtype uint16) *dns.Msg {
		r := new(dns.Msg)
		r.SetQuestion(name, qtype)
		msg := new(dns.Msg)
		msg.SetReply(r)
		answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))
		return msg
	}

	var targets []string
	for _, rr := range ask("t.example.", dns.TypeNS).Answer {
		if ns, ok := rr.(*dns.NS); ok {
			targets = append(targets, ns.Ns)
		}
	}
	if len(targets) != 2 {
		t.Fatalf("want 2 NS targets, got %v", targets)
	}
	addresses := map[string]bool{}
	// RFC requirement: RFC2181-10.3-1 positive -- each NS target, queried for CNAME, has no CNAME record in the answer, and queried for A, answers its own A record holding a configured nameserver address.
	for _, target := range targets {
		for _, rr := range ask(target, dns.TypeCNAME).Answer {
			if _, alias := rr.(*dns.CNAME); alias {
				t.Errorf("NS target %q is an alias: %v", target, rr)
			}
		}
		found := false
		for _, rr := range ask(target, dns.TypeA).Answer {
			a, ok := rr.(*dns.A)
			if !ok {
				t.Errorf("NS target %q answers a %T for an A query", target, rr)
				continue
			}
			if !strings.EqualFold(a.Hdr.Name, target) {
				t.Errorf("A record owner %q is not the NS target %q", a.Hdr.Name, target)
			}
			addresses[a.A.String()] = true
			found = true
		}
		if !found {
			t.Errorf("NS target %q has no A record of its own", target)
		}
	}
	if !addresses["10.0.0.1"] || !addresses["10.0.0.2"] {
		t.Errorf("NS target addresses = %v, want the configured 10.0.0.1 and 10.0.0.2", addresses)
	}
}

// VALIDATES: RFC 2181 section 11 -- a name GeoDNS accepts, and so can answer
// with, has labels of 1 to 63 octets and at most 255 octets in all. The bounds
// are checked on both sides of each edge, at the configuration boundary
// (parseConfig) and in the wire codec answers are packed through.
// PREVENTS: a configured name the codec cannot pack, which silently answers
// nothing for that name.
func TestRFC2181NameLimitsBothSidesOfEachBound(t *testing.T) {
	t.Parallel()
	hostConfig := func(host string) string {
		return `{"service":{"geodns":{"enabled":"true","zone":["t.example."],` +
			`"host-set":{"web":{"host":{"` + host + `":{"address":["10.0.0.1"]}}}},` +
			`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`
	}
	label63 := strings.Repeat("a", 63)
	// RFC requirement: RFC2181-11-1 positive -- a host with a 63-octet label is accepted and its answer packs; a 255-octet name is accepted by checkName and packs through the codec.
	if _, err := parseConfig(hostConfig(label63 + ".t.example.")); err != nil {
		t.Errorf("a 63-octet label was refused: %v", err)
	}
	name255 := zoneOfWireOctets(t, 255)
	if err := checkName("host", name255); err != nil {
		t.Errorf("a 255-octet name was refused: %v", err)
	}
	fits := new(dns.Msg)
	fits.SetQuestion(name255, dns.TypeA)
	if _, err := fits.Pack(); err != nil {
		t.Errorf("the codec refused a 255-octet name: %v", err)
	}

	// RFC requirement: RFC2181-11-1 negative -- a 64-octet label, an empty label and a 256-octet name are each refused by the configuration; the codec refuses to pack a 256-octet name.
	if _, err := parseConfig(hostConfig(label63 + "a.t.example.")); err == nil {
		t.Error("a 64-octet label was accepted")
	}
	if _, err := parseConfig(hostConfig("www..t.example.")); err == nil {
		t.Error("a name with an empty label was accepted")
	}
	name256 := "a" + zoneOfWireOctets(t, 255)
	if err := checkName("host", name256); err == nil {
		t.Errorf("a %d-octet name was accepted", nameWireOctets(name256))
	}
	over := new(dns.Msg)
	over.SetQuestion(name256, dns.TypeA)
	if _, err := over.Pack(); err == nil {
		t.Error("the codec packed a 256-octet name")
	}
}

// VALIDATES: RFC 2181 section 8 -- a TTL is 0 to 2^31-1 and is sent with the
// sign bit clear. The configured maximum is carried into the packed answer and
// read back from the TTL field of every record GeoDNS emits for the zone (A,
// NS, SOA), and 0 is a legal configured TTL.
// PREVENTS: a TTL reaching the wire with its top bit set, which a resolver
// reads as 0 or as negative.
func TestRFC2181TTLTransmittedWithSignBitClear(t *testing.T) {
	t.Parallel()
	// RFC requirement: RFC2181-8-1 positive -- a configured TTL of 0 is accepted as 0, and with default-ttl 2147483647 every A, NS and SOA record in the packed answer carries a TTL field whose most significant bit is zero and whose value is at most 2147483647.
	if ttl, err := parseTTL("0"); err != nil || ttl != 0 {
		t.Errorf("parseTTL(0) = %d, %v; want 0, nil", ttl, err)
	}
	st := answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],"nameserver":["10.0.0.1"],`+
		`"default-ttl":"2147483647",`+
		`"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.5"]}}}},`+
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)
	for _, question := range []struct {
		name  string
		qtype uint16
	}{{"www.t.example.", dns.TypeA}, {"t.example.", dns.TypeNS}, {"t.example.", dns.TypeSOA}} {
		r := new(dns.Msg)
		r.SetQuestion(question.name, question.qtype)
		msg := new(dns.Msg)
		msg.SetReply(r)
		answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))
		if len(msg.Answer) == 0 {
			t.Fatalf("no answer for %s %d", question.name, question.qtype)
		}
		wire, err := msg.Pack()
		if err != nil {
			t.Fatalf("pack: %v", err)
		}
		back := new(dns.Msg)
		if err := back.Unpack(wire); err != nil {
			t.Fatalf("unpack: %v", err)
		}
		for _, rr := range append(back.Answer, back.Extra...) {
			field := make([]byte, 4)
			binary.BigEndian.PutUint32(field, rr.Header().Ttl)
			if field[0]&0x80 != 0 {
				t.Errorf("%s TTL field %x has the sign bit set", rr.Header().Name, field)
			}
			if rr.Header().Ttl > 2147483647 {
				t.Errorf("%s TTL %d exceeds 2147483647", rr.Header().Name, rr.Header().Ttl)
			}
		}
	}
}
