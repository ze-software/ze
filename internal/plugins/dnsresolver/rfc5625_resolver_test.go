// Design: docs/architecture/dns/lan-resolver.md -- the LAN resolver against RFC 5625
// Related: rfc1035_resolver_test.go -- the RFC 1035 rows on the same path

package dnsresolver

import (
	"testing"

	"github.com/miekg/dns"
)

// TestRFC5625ResolverUnknownFlags proves RFC 5625 Section 4.1 on the resolver
// path: a query carrying flags the resolver does not act on (AD, CD and the
// reserved Z bit) is answered as usual, not refused or dropped. Method: one
// query with the three flags set and one with none, both against an upstream
// that answers one A record.
func TestRFC5625ResolverUnknownFlags(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		return []fakeReply{{msg: answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1")}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	q := query("flags.test", dns.TypeA)
	q.AuthenticatedData = true
	q.CheckingDisabled = true
	q.Zero = true
	reply := ask(r, q, "127.0.0.1", false)
	// RFC requirement: RFC5625-4.1-1 positive -- "proxies MUST ignore any
	// unknown DNS flags and proxy those packets as usual." A query with AD, CD
	// and Z set is answered NOERROR with the upstream's record.
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
		t.Fatalf("reply %v to a query with AD, CD and Z set, want NOERROR with one A record", reply)
	}
}

// TestRFC5625ResolverUnknownTypeAndClass proves RFC 5625 Section 4.3 on the
// resolver path: a question of a type and a class the resolver has no
// knowledge of reaches the upstream with that type and class, and the answer
// record of an unknown type comes back to the client. Method: a TYPE65280
// question in class IN, and a TXT question in class CH, each against an
// upstream that echoes the question in its answer.
func TestRFC5625ResolverUnknownTypeAndClass(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		if q.Question[0].Qclass == dns.ClassCHAOS {
			return []fakeReply{{msg: answerWith(q, q.Question[0].Name+` 300 CH TXT "ze"`)}}
		}
		return []fakeReply{{msg: answerWith(q, q.Question[0].Name+` 300 IN TYPE65280 \# 4 01020304`)}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	reply := ask(r, query("private.test", 65280), "127.0.0.1", false)
	// RFC requirement: RFC5625-4.3-1 positive -- "All requests and responses
	// MUST be proxied regardless of the values of the QTYPE and QCLASS
	// fields." A TYPE65280 question reaches the upstream as TYPE65280.
	if seen := up.lastQuery(); seen == nil || seen.Question[0].Qtype != 65280 {
		t.Fatalf("upstream saw %v, want the TYPE65280 question", seen)
	}
	// RFC requirement: RFC5625-4.3-2 positive -- "all responses MUST be
	// proxied regardless of the values of the TYPE and CLASS fields of any
	// Resource Record therein." The TYPE65280 record reaches the client.
	if reply == nil || len(reply.Answer) != 1 || reply.Answer[0].Header().Rrtype != 65280 {
		t.Fatalf("reply %v, want the one TYPE65280 record", reply)
	}

	q := query("version.test", dns.TypeTXT)
	q.Question[0].Qclass = dns.ClassCHAOS
	reply = ask(r, q, "127.0.0.1", false)
	// RFC requirement: RFC5625-4.3-1 positive -- a class CH question reaches
	// the upstream in class CH, and its CH answer reaches the client.
	if seen := up.lastQuery(); seen == nil || seen.Question[0].Qclass != dns.ClassCHAOS {
		t.Fatalf("upstream saw %v, want the class CH question", seen)
	}
	if reply == nil || len(reply.Answer) != 1 || reply.Answer[0].Header().Class != dns.ClassCHAOS {
		t.Fatalf("reply %v, want the one class CH record", reply)
	}
}

// TestRFC5625ResolverCompressedReply proves RFC 5625 Section 4.2 on the
// resolver path: an upstream reply whose names are compressed is forwarded to
// the client. Method: an upstream that packs its reply with compression on.
func TestRFC5625ResolverCompressedReply(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		m := answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1", q.Question[0].Name+" 300 IN A 192.0.2.2")
		m.Compress = true
		return []fakeReply{{msg: m}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	reply := ask(r, query("compressed.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC5625-4.2-1 positive -- "Proxies MUST forward packets
	// regardless of the presence or absence of compressed labels therein." A
	// compressed upstream reply reaches the client with both records.
	if reply == nil || len(reply.Answer) != 2 {
		t.Fatalf("reply %v to a compressed upstream reply, want both A records", reply)
	}
}

// TestRFC5625ProxyTCPAndOPT proves RFC 5625 Sections 4.4.1 and 4.4.2 on the
// resolver path: a query that arrives over TCP is answered over TCP, and a
// query that carries an OPT RR is answered, not refused with FORMERR.
// Method: one TCP query, and one UDP query with EDNS0 at 4096 octets and DO
// set, against an upstream that answers one A record.
func TestRFC5625ProxyTCPAndOPT(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		return []fakeReply{{msg: answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1")}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	reply := ask(r, query("tcp.test", dns.TypeA), "127.0.0.1", true)
	// RFC requirement: RFC5625-4.4.1-1 positive -- "DNS proxies MUST therefore
	// be prepared to receive and forward queries over TCP." A query received
	// over TCP is forwarded and its answer returned.
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
		t.Fatalf("reply %v to a TCP query, want NOERROR with one A record", reply)
	}

	q := query("opt.test", dns.TypeA)
	q.SetEdns0(4096, true)
	reply = ask(r, q, "127.0.0.1", false)
	// RFC requirement: RFC5625-4.4.2-1 positive -- "As per Section 4.1,
	// proxies MUST NOT refuse to proxy such packets." A query with an OPT RR
	// is answered NOERROR with the upstream's record, not FORMERR.
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
		t.Fatalf("reply %v to a query with an OPT RR, want NOERROR with one A record", reply)
	}
}
