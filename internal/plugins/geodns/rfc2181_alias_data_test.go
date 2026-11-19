// Design: docs/architecture/dns/geodns.md -- RFC 2181 Section 10.1: no CNAME beside other data

package geodns

import (
	"net/netip"
	"testing"

	"github.com/miekg/dns"
)

// TestRFC2181NoAliasBesideOtherData proves RFC 2181 Section 10.1, "An alias
// name (label of a CNAME record) may, if DNSSEC is in use, have SIG, NXT, and
// KEY RRs, but may have no other data.", on the authoritative side. A name
// GeoDNS serves holds data (here an A and an AAAA record), so it is the
// second of the four cases the section lists, "one or more records exist,
// none being CNAME records".
//
// Method: one configured host with an IPv4 and an IPv6 address, asked
// directly through answerQuestions. An ANY query returns every record the
// name owns, so no record of type CNAME may be among them, and a CNAME query
// is answered NODATA with the zone SOA. The negative takes the case just
// outside the clause: a name that is not an alias carries its data, and an A
// query returns its A record, so a server that answered nothing would fail.
func TestRFC2181NoAliasBesideOtherData(t *testing.T) {
	t.Parallel()
	st := answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+
		`"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.1","2001:db8::1"]}}}},`+
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)
	ask := func(qtype uint16) *dns.Msg {
		r := new(dns.Msg)
		r.SetQuestion("www.t.example.", qtype)
		msg := new(dns.Msg)
		msg.SetReply(r)
		answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))
		return msg
	}

	// RFC requirement: RFC2181-10.1-4 positive -- a name holding an A and an AAAA record answers an ANY query with both and with no record of type CNAME, and answers a CNAME query with an empty Answer section and the zone SOA in Authority.
	anyReply := ask(dns.TypeANY)
	if len(anyReply.Answer) != 2 {
		t.Fatalf("ANY answer = %v, want the A and the AAAA record", anyReply.Answer)
	}
	for _, rr := range anyReply.Answer {
		if rr.Header().Rrtype == dns.TypeCNAME {
			t.Errorf("ANY answer carries a CNAME beside the name's other data: %v", rr)
		}
	}
	cname := ask(dns.TypeCNAME)
	if cname.Rcode != dns.RcodeSuccess || len(cname.Answer) != 0 {
		t.Errorf("CNAME query: rcode %s with answer %v, want NOERROR and an empty Answer", dns.RcodeToString[cname.Rcode], cname.Answer)
	}
	if len(cname.Ns) != 1 || cname.Ns[0].Header().Rrtype != dns.TypeSOA {
		t.Errorf("CNAME query authority = %v, want the zone SOA alone", cname.Ns)
	}

	// RFC requirement: RFC2181-10.1-4 negative -- the name is no alias, so the rule leaves its data in place: an A query answers exactly the configured A record 10.0.0.1, owned by the queried name.
	a := ask(dns.TypeA)
	if len(a.Answer) != 1 {
		t.Fatalf("A answer = %v, want the one A record", a.Answer)
	}
	rec, ok := a.Answer[0].(*dns.A)
	if !ok || rec.Hdr.Rrtype != dns.TypeA || rec.A.String() != "10.0.0.1" || rec.Hdr.Name != "www.t.example." {
		t.Errorf("A answer = %v, want www.t.example. A 10.0.0.1", a.Answer[0])
	}
}
