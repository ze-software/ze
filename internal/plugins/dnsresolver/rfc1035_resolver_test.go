package dnsresolver

import (
	"strconv"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

// TestRFC1035ResolverNameCase proves RFC 1035 Sections 2.3.3 and 3.1 on the
// resolver path: names and labels are compared without regard to case, in
// the cache key and in the match of an upstream reply to its query, and a
// name that differs in a letter rather than in case stays a different name.
// Method: an upstream that answers every query with the question name upper
// cased, except one name it answers with another name in the question; the
// client asks through dnsserver.Recursive and the upstream query count
// tells a cache hit from a miss.
func TestRFC1035ResolverNameCase(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		name := strings.ToUpper(q.Question[0].Name)
		if rest, found := strings.CutPrefix(name, "WEB-9."); found {
			name = "WEB-8." + rest
		}
		m := new(dns.Msg)
		m.SetReply(q)
		m.Question[0].Name = name
		m.Answer = append(m.Answer, mustRR(name+" 300 IN A 192.0.2.1"))
		return []fakeReply{{msg: m}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	first := ask(r, query("web-1.example.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-3.1-5 positive -- "Name servers and resolvers
	// must compare labels in a case-insensitive manner (i.e., A=a)". An
	// upstream reply whose question spells web-1.example.test in upper case is
	// taken as the reply to the lower-case query: the client gets NOERROR and
	// the one A record.
	if first == nil || first.Rcode != dns.RcodeSuccess || len(first.Answer) != 1 {
		t.Fatalf("reply %v to web-1.example.test, want NOERROR with one A record", first)
	}

	again := ask(r, query("WEB-1.Example.TEST", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-2.3.3-1 positive -- "all comparisons between
	// character strings (e.g., labels, domain names, etc.) are done in a
	// case-insensitive manner". WEB-1.Example.TEST, asked after
	// web-1.example.test, is answered with one A record from the cache: the
	// upstream saw one query for the two spellings.
	if again == nil || len(again.Answer) != 1 {
		t.Fatalf("reply %v to WEB-1.Example.TEST, want one A record", again)
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries for two spellings of one name, want 1", n)
	}

	other := ask(r, query("web-2.example.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-2.3.3-1 negative -- case-insensitive comparison
	// makes two spellings of one name equal, not two names: web-2.example.test,
	// which differs from the cached web-1.example.test in a letter, is a cache
	// miss that reaches the upstream, and its answer is owned by web-2.
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries after web-2.example.test, want 2: a different name hit the cache", n)
	}
	if other == nil || len(other.Answer) != 1 || !strings.EqualFold(other.Answer[0].Header().Name, "web-2.example.test.") {
		t.Fatalf("reply %v to web-2.example.test, want the one record web-2 owns", other)
	}

	wrong := ask(r, query("web-9.example.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-3.1-5 negative -- "A=a" is the whole of the
	// equivalence: an upstream reply whose question names WEB-8 is not the
	// reply to a query for web-9, so the client gets SERVFAIL with no answer.
	if wrong == nil || wrong.Rcode != dns.RcodeServerFailure || len(wrong.Answer) != 0 {
		t.Fatalf("reply %v to web-9.example.test, want SERVFAIL with no answer: a reply for another name was taken", wrong)
	}
}

// TestRFC1035ResolverHeaderZ proves RFC 1035 Section 4.1.1 Z on the
// resolver path: the reply's reserved Z bit is zero, whatever the upstream
// reply and the client query carried. Method: an upstream that sets Z in its
// reply, asked once with a query whose Z is clear and once with Z set.
func TestRFC1035ResolverHeaderZ(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		m := answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1")
		m.Zero = true
		return []fakeReply{{msg: m}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	plain := ask(r, query("z.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-4.1.1-1 positive -- "Z               Reserved for
	// future use.  Must be zero in all queries and responses." The reply to a
	// query with Z clear, answered from an upstream reply with Z set, has Z
	// clear and carries the answer.
	if plain == nil || len(plain.Answer) != 1 {
		t.Fatalf("reply %v, want one A record", plain)
	}
	if plain.Zero {
		t.Fatal("reply has Z set, want Z clear")
	}

	q := query("z2.test", dns.TypeA)
	q.Zero = true
	echoed := ask(r, q, "127.0.0.1", false)
	// RFC requirement: RFC1035-4.1.1-1 negative -- "Must be zero in all queries
	// and responses" binds the response independently of the query: a query
	// that arrives with Z set is answered with Z clear rather than echoed.
	if echoed == nil || echoed.Zero {
		t.Fatalf("reply %v to a query with Z set, want Z clear", echoed)
	}
}

// TestRFC1035ResolverClearsAA proves RFC 1035 Section 4.1.1 AA on the
// resolver path: a forwarding resolver is no authority for any name, so its
// reply has AA clear even when the upstream that answered set it, and
// clearing AA leaves the neighboring RD bit tracking the query. Method: an
// upstream that sets AA, asked with RD set.
func TestRFC1035ResolverClearsAA(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())

	reply := ask(r, query("aa.test", dns.TypeA), "127.0.0.1", false)
	if reply == nil || len(reply.Answer) != 1 {
		t.Fatalf("reply %v, want one A record", reply)
	}
	// RFC requirement: RFC1035-4.1.1-2 positive -- "AA              Authoritative
	// Answer - this bit is valid in responses, and specifies that the responding
	// name server is an authority for the domain name in question section." The
	// resolver's reply, built from an upstream reply with AA set, has AA clear.
	if reply.Authoritative {
		t.Fatal("reply has AA set, though the resolver holds no zone")
	}
	// RFC requirement: RFC1035-4.1.1-2 negative -- "this bit is valid in
	// responses" scopes the rule to the AA position alone: the same reply still
	// has RD set, copied from the query.
	if !reply.RecursionDesired {
		t.Fatal("reply has RD clear, though the query set it")
	}
}

// TestRFC1035ResolverUnderstandsPointers proves RFC 1035 Section 4.1.4 on
// the resolver path: an upstream reply that uses compression is understood.
// Method: an upstream that packs its reply compressed, so the answer's owner
// name is a pointer to the question name at offset 12.
func TestRFC1035ResolverUnderstandsPointers(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		m := answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1")
		m.Compress = true
		return []fakeReply{{msg: m}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	reply := ask(r, query("pointer.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-4.1.4-5 positive -- "However all programs are
	// required to understand arriving messages that contain pointers." An
	// upstream reply whose answer owner is a compression pointer is answered to
	// the client as NOERROR with that A record, owned by pointer.test.
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
		t.Fatalf("reply %v, want NOERROR with one A record", reply)
	}
	if got := reply.Answer[0].Header().Name; got != "pointer.test." {
		t.Fatalf("answer owner %q, want pointer.test.", got)
	}
}

// bigUpstream answers big.test with count A records and every other name
// with one, over both transports.
func bigUpstream(t *testing.T, count int) *fakeUpstream {
	t.Helper()
	return newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		if q.Question[0].Name != "big.test." {
			return []fakeReply{{msg: answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1")}}
		}
		var rrs []string
		for i := 1; i <= count; i++ {
			rrs = append(rrs, "big.test. 300 IN A 192.0.2."+strconv.Itoa(i))
		}
		m := answerWith(q, rrs...)
		m.Compress = true
		return []fakeReply{{msg: m}}
	})
}

// TestRFC1035ResolverDatagramBound proves RFC 1035 Section 4.2.1 on the
// resolver path: a datagram reply to a client that offered no larger buffer
// is held to 512 octets and marked truncated when shortened, a reply that
// fits is not marked, and the bound is the datagram's alone. Method: an
// upstream holding a 50-record A RRset (more than 512 octets) and a
// one-record one, asked without EDNS over UDP, then over TCP.
func TestRFC1035ResolverDatagramBound(t *testing.T) {
	const records = 50
	up := bigUpstream(t, records)
	r := newTestResolver(t, testConfig(up), newTestClock())

	reply := ask(r, query("big.test", dns.TypeA), "127.0.0.1", false)
	if reply == nil {
		t.Fatal("no UDP reply for big.test")
	}
	wire, err := reply.Pack()
	if err != nil {
		t.Fatalf("pack UDP reply: %v", err)
	}
	// RFC requirement: RFC1035-4.2.1-1 positive -- "Messages carried by UDP are
	// restricted to 512 bytes (not counting the IP or UDP headers)." The UDP
	// reply to a query carrying no OPT, for a 50-record RRset, packs to at most
	// 512 octets.
	if len(wire) > dns.MinMsgSize {
		t.Fatalf("UDP reply of %d octets, want at most %d", len(wire), dns.MinMsgSize)
	}
	// RFC requirement: RFC1035-4.2.1-2 positive -- "Longer messages are
	// truncated and the TC bit is set in the header". The same UDP reply, which
	// carries fewer than the 50 records, has TC set.
	if len(reply.Answer) >= records {
		t.Fatalf("UDP reply carries %d records, want fewer than %d", len(reply.Answer), records)
	}
	if !reply.Truncated {
		t.Fatal("shortened UDP reply has TC clear, want TC set")
	}

	small := ask(r, query("small.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-4.2.1-2 negative -- only a longer message is
	// truncated: the UDP reply for a one-record RRset, which fits in 512
	// octets, carries that record with TC clear.
	if small == nil || len(small.Answer) != 1 || small.Truncated {
		t.Fatalf("UDP reply %v for small.test, want one record with TC clear", small)
	}

	stream := ask(r, query("big.test", dns.TypeA), "127.0.0.1", true)
	if stream == nil {
		t.Fatal("no TCP reply for big.test")
	}
	streamWire, err := stream.Pack()
	if err != nil {
		t.Fatalf("pack TCP reply: %v", err)
	}
	// RFC requirement: RFC1035-4.2.1-1 negative -- the 512-octet restriction is
	// on messages carried by UDP: the TCP reply for the same 50-record RRset
	// carries all 50 records, packs to more than 512 octets and has TC clear.
	if len(stream.Answer) != records || stream.Truncated || len(streamWire) <= dns.MinMsgSize {
		t.Fatalf("TCP reply of %d octets with %d records, tc=%v, want all %d records, over %d octets, TC clear",
			len(streamWire), len(stream.Answer), stream.Truncated, records, dns.MinMsgSize)
	}
}

// resolverConfigJSON is the delivered JSON of an enabled resolver with one
// listener and one forward server, each written as the operator wrote it.
func resolverConfigJSON(listener, server string) string {
	return `{"service":{"dns-resolver":{"enabled":"true",` +
		`"listener":{"lan":` + listener + `},` +
		`"allow-from":["10.0.0.0/8"],` +
		`"forward":{"server":[` + server + `]}}}}`
}

// TestRFC1035ResolverListensOnServerPort53 proves RFC 1035 Section 4.2.1 on
// the resolver's listener: a listener the operator gave no port receives
// client datagrams on server port 53. Method: parse the delivered config of a
// listener written with an address alone.
func TestRFC1035ResolverListensOnServerPort53(t *testing.T) {
	cfg, err := parseConfig(resolverConfigJSON(`{"ip":"127.0.0.1"}`, `{"ip":"192.0.2.1"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cfg.Listeners) != 1 {
		t.Fatalf("%d listeners, want 1", len(cfg.Listeners))
	}
	// RFC requirement: RFC1035-4.2.1-3 positive -- "Messages sent using UDP user
	// server port 53 (decimal)." A resolver listener configured with an address
	// and no port listens on port 53.
	if got := cfg.Listeners[0].Port; got != 53 {
		t.Fatalf("listener port %d, want 53", got)
	}
}

// TestRFC1035ResolverForwardsToServerPort53 proves RFC 1035 Section 4.2.1 on
// the resolver's upstream side: a forward server the operator gave no port is
// sent its queries on server port 53, and 53 is that default rather than a
// constant overriding the operator. Method: parse the delivered config of a
// forward server written with an address alone, then with port 5300.
func TestRFC1035ResolverForwardsToServerPort53(t *testing.T) {
	cfg, err := parseConfig(resolverConfigJSON(`{"ip":"127.0.0.1","port":"5353"}`, `{"ip":"192.0.2.1"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cfg.Forward) != 1 {
		t.Fatalf("%d forward servers, want 1", len(cfg.Forward))
	}
	// RFC requirement: RFC1035-4.2.1-3 positive -- "Messages sent using UDP user
	// server port 53 (decimal)." A forward server configured with an address and
	// no port is the upstream address with port 53.
	if got := cfg.Forward[0].Port; got != 53 {
		t.Fatalf("forward server port %d, want 53", got)
	}

	named, err := parseConfig(resolverConfigJSON(`{"ip":"127.0.0.1","port":"5353"}`, `{"ip":"192.0.2.1","port":"5300"}`))
	if err != nil {
		t.Fatalf("parse with port 5300: %v", err)
	}
	// RFC requirement: RFC1035-4.2.1-3 negative -- port 53 is the server port a
	// forward server takes when the operator names none, not a constant: a
	// forward server configured with port 5300 is the upstream address with
	// port 5300.
	if len(named.Forward) != 1 || named.Forward[0].Port != 5300 {
		t.Fatalf("forward servers %v, want one on port 5300", named.Forward)
	}
}

// TestRFC1035ResolverStreamFraming proves RFC 1035 Section 4.2.2 on the
// resolver's upstream exchanges: a query sent over TCP carries the two-byte
// length prefix, and a query sent over UDP does not. Method: an upstream
// that marks its UDP reply for big.test truncated, so the resolver retries
// over TCP, and reads each query with the framing of its transport, so a
// query framed for the other transport is not read at all.
func TestRFC1035ResolverStreamFraming(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, tcp bool) []fakeReply {
		if q.Question[0].Name != "big.test." {
			return []fakeReply{{msg: answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1")}}
		}
		if !tcp {
			m := new(dns.Msg)
			m.SetReply(q)
			m.Truncated = true
			return []fakeReply{{msg: m}}
		}
		return []fakeReply{{msg: answerWith(q, "big.test. 300 IN A 192.0.2.1", "big.test. 300 IN A 192.0.2.2")}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	datagram := ask(r, query("small.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-4.2.2-1 negative -- the two byte length field is
	// the stream framing of section 4.2.2. A datagram carries its own boundary,
	// so the resolver's UDP query is not prefixed: an upstream that unpacks the
	// datagram from its first octet reads it and answers it over UDP, so the
	// client gets the one record and the upstream sees one query, none over
	// TCP.
	if datagram == nil || len(datagram.Answer) != 1 {
		t.Fatalf("reply %v, want the one record the UDP exchange returned", datagram)
	}
	if n, tcp := up.queries.Load(), up.tcp.Load(); n != 1 || tcp != 0 {
		t.Fatalf("upstream read %d queries, %d over TCP, want 1 and 0", n, tcp)
	}

	reply := ask(r, query("big.test", dns.TypeA), "127.0.0.1", true)
	// RFC requirement: RFC1035-4.2.2-1 positive -- "The message is prefixed with
	// a two byte length field which gives the message length, excluding the two
	// byte length field." The resolver's TCP query, after a truncated UDP reply,
	// is read by an upstream that takes the first two octets as the length of
	// the message that follows: the upstream sees one TCP query and the client
	// gets both records.
	if reply == nil || len(reply.Answer) != 2 {
		t.Fatalf("reply %v, want the two records the TCP exchange returned", reply)
	}
	if n := up.tcp.Load(); n != 1 {
		t.Fatalf("upstream read %d TCP queries, want 1", n)
	}
}

// TestRFC1035ResolverInverseQueryNotImplemented proves RFC 1035 Section 6.4
// on the resolver path: an inverse query draws Not Implemented without
// reaching the upstream, and a standard query is still answered. Method: an
// IQUERY and a QUERY through dnsserver.Recursive over the resolver, with the
// upstream query count read after each.
func TestRFC1035ResolverInverseQueryNotImplemented(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())

	standard := ask(r, query("standard.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC1035-6.4-1 negative -- the Not Implemented reply is
	// owed for the inverse query only: a standard query is forwarded once and
	// answered NOERROR with the upstream's A record.
	if standard == nil || standard.Rcode != dns.RcodeSuccess || len(standard.Answer) != 1 {
		t.Fatalf("reply %v to a standard query, want NOERROR with one A record", standard)
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1", n)
	}

	q := query("inverse.test", dns.TypeA)
	q.Opcode = dns.OpcodeIQuery
	reply := ask(r, q, "127.0.0.1", false)
	// RFC requirement: RFC1035-6.4-1 positive -- "While inverse query support is
	// optional, all name servers must be at least able to return the error
	// response". An IQUERY to the resolver is answered NOTIMP with no answer,
	// and the upstream sees no further query.
	if reply == nil || reply.Rcode != dns.RcodeNotImplemented || len(reply.Answer) != 0 {
		t.Fatalf("reply %v to an IQUERY, want NOTIMP with no answer", reply)
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries after an IQUERY, want the 1 the standard query made", n)
	}
}
