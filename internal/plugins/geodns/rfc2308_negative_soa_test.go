// Design: docs/architecture/dns/geodns.md -- the answer policy, and the zone
// SOA a negative answer carries
// RFC: rfc/short/rfc2308.md -- RFC 2308 Section 3, the SOA of a negative answer
// Related: rfc1035_negative_test.go -- negZone, askFrom

package geodns

import (
	"testing"

	"github.com/miekg/dns"
)

// zoneSOAIn returns the SOA in rrs owned by zone, or nil when rrs holds none.
func zoneSOAIn(rrs []dns.RR, zone string) *dns.SOA {
	for _, rr := range rrs {
		soa, ok := rr.(*dns.SOA)
		if !ok {
			continue
		}
		if equalName(soa.Hdr.Name, zone) {
			return soa
		}
	}
	return nil
}

// TestRFC2308_NegativeAnswerCarriesZoneSOA proves RFC 2308 Section 3 for
// geodns as the authoritative server of t.example.: an NXDOMAIN and a no-data
// answer each carry the zone's SOA in the authority section, and an answer
// that holds the data asked for carries none. Method: the negZone harness,
// asked from a client in the "us" host set for a name the zone does not own,
// a name configured only for another host set, a type www holds no record of,
// and www's own A record.
func TestRFC2308_NegativeAnswerCarriesZoneSOA(t *testing.T) {
	handler := negZone(t)
	const us = "198.51.100.7"

	for _, tc := range []struct {
		what  string
		qname string
		qtype uint16
		rcode int
	}{
		{"name error", "nope.t.example.", dns.TypeA, dns.RcodeNameError},
		{"no data for this client", "eu-only.t.example.", dns.TypeA, dns.RcodeSuccess},
		{"no data of the type", "www.t.example.", dns.TypeMX, dns.RcodeSuccess},
	} {
		reply := askFrom(t, handler, us, tc.qname, tc.qtype)
		// RFC requirement: RFC2308-3-1 positive -- geodns, authoritative for
		// t.example., includes the t.example. SOA in the authority section of
		// an NXDOMAIN (nope.t.example. A) and of the no-data answers for
		// eu-only.t.example. A and www.t.example. MX.
		if reply.Rcode != tc.rcode || len(reply.Answer) != 0 {
			t.Fatalf("%s: rcode %s answer %v, want %s with no answer", tc.what,
				dns.RcodeToString[reply.Rcode], reply.Answer, dns.RcodeToString[tc.rcode])
		}
		if zoneSOAIn(reply.Ns, "t.example.") == nil {
			t.Errorf("%s: authority %v, want the t.example. SOA", tc.what, reply.Ns)
		}
	}

	positive := askFrom(t, handler, us, "www.t.example.", dns.TypeA)
	// RFC requirement: RFC2308-3-1 negative -- an answer that reports neither
	// NXDOMAIN nor no data, www.t.example. A answered 10.0.0.5 for this
	// client, carries no SOA in the authority section.
	if positive.Rcode != dns.RcodeSuccess || firstA(positive) != "10.0.0.5" {
		t.Fatalf("www.t.example. A: rcode %s A %q, want NOERROR and 10.0.0.5",
			dns.RcodeToString[positive.Rcode], firstA(positive))
	}
	if hasSOA(positive.Ns) {
		t.Errorf("www.t.example. A: authority %v, want no SOA in a positive answer", positive.Ns)
	}
}
