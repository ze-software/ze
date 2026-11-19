// Design: docs/architecture/dns/as112.md -- the answers a served zone gives,
// and the zone SOA a negative answer carries
// RFC: rfc/short/rfc2308.md -- RFC 2308 Section 3, the SOA of a negative answer
// Related: zones_test.go -- as112Handler, askAS112, hasSOAIn

package as112

import (
	"strings"
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
		if strings.EqualFold(soa.Hdr.Name, zone) {
			return soa
		}
	}
	return nil
}

// TestRFC2308_NegativeAnswerCarriesZoneSOA proves RFC 2308 Section 3 for
// AS112 as the authoritative server of its zones: an NXDOMAIN and a no-data
// answer from 10.in-addr.arpa. each carry that zone's SOA in the authority
// section, and an answer holding the data asked for carries none. Method:
// the full harness, asked for a name below the apex, for a type the apex
// holds no record of, and for the apex NS set.
func TestRFC2308_NegativeAnswerCarriesZoneSOA(t *testing.T) {
	handler := as112Handler(t)
	const zone = "10.in-addr.arpa."

	for _, tc := range []struct {
		what  string
		qname string
		qtype uint16
		rcode int
	}{
		{"name error", "1.0.0.10.in-addr.arpa.", dns.TypePTR, dns.RcodeNameError},
		{"no data of the type", zone, dns.TypeA, dns.RcodeSuccess},
	} {
		reply := askAS112(t, handler, tc.qname, tc.qtype)
		// RFC requirement: RFC2308-3-1 positive -- AS112, authoritative for
		// 10.in-addr.arpa., includes the 10.in-addr.arpa. SOA in the authority
		// section of an NXDOMAIN (1.0.0.10.in-addr.arpa. PTR) and of the
		// no-data answer for the apex A.
		if reply.Rcode != tc.rcode || len(reply.Answer) != 0 {
			t.Fatalf("%s: rcode %s answer %v, want %s with no answer", tc.what,
				dns.RcodeToString[reply.Rcode], reply.Answer, dns.RcodeToString[tc.rcode])
		}
		if zoneSOAIn(reply.Ns, zone) == nil {
			t.Errorf("%s: authority %v, want the %s SOA", tc.what, reply.Ns, zone)
		}
	}

	positive := askAS112(t, handler, zone, dns.TypeNS)
	// RFC requirement: RFC2308-3-1 negative -- an answer that reports neither
	// NXDOMAIN nor no data, the apex NS set of 10.in-addr.arpa., carries no
	// SOA in the authority section.
	if positive.Rcode != dns.RcodeSuccess || len(positive.Answer) == 0 {
		t.Fatalf("apex NS: rcode %s answer %v, want NOERROR with the NS set",
			dns.RcodeToString[positive.Rcode], positive.Answer)
	}
	if hasSOAIn(positive.Ns) {
		t.Errorf("apex NS: authority %v, want no SOA in a positive answer", positive.Ns)
	}
}
