// Design: docs/architecture/dns/geodns.md -- RFC 4035 section 3 for a query with DO clear

package geodns

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

// VALIDATES: RFC 4035 section 3, the second arm of its condition: a query that
// carries an EDNS OPT pseudo-RR with the DO bit CLEAR draws the same records as
// the OPT-less query, with no RRSIG, NSEC or DNSKEY record in any section, so
// no DNSSEC additional processing runs for it.
// PREVENTS: DNSSEC records leaking into a response for a client that did not
// ask for them by setting DO.
func TestRFC4035DOClearQueryGetsNoDNSSECProcessing(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"geodns":{"enabled":"true","zone":["t.example."],"nameserver":["10.0.0.1"],` +
		`"host-set":{"web":{"host":{"www.t.example.":{"address":["10.0.0.5"]}}}},` +
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	storeApplied(cfg, 1)
	source := &net.UDPAddr{IP: net.ParseIP("203.0.113.7"), Port: 5353}

	for _, qtype := range []uint16{dns.TypeA, dns.TypeDNSKEY, dns.TypeRRSIG, dns.TypeNSEC} {
		bare := new(dns.Msg)
		bare.SetQuestion("www.t.example.", qtype)
		withOPT := new(dns.Msg)
		withOPT.SetQuestion("www.t.example.", qtype)
		withOPT.SetEdns0(4096, false)

		plain := answerViaEntry(bare, source)
		clear := answerViaEntry(withOPT, source)
		// RFC requirement: RFC4035-3-6 positive -- a query with an EDNS OPT whose DO bit is clear, for A, DNSKEY, RRSIG and NSEC, gets the same Answer and Authority as the OPT-less query and no RRSIG, NSEC or DNSKEY record in any section.
		if len(plain.Answer) != len(clear.Answer) || len(plain.Ns) != len(clear.Ns) {
			t.Errorf("type %d: DO-clear response (%d answer, %d authority) differs from the OPT-less one (%d, %d)",
				qtype, len(clear.Answer), len(clear.Ns), len(plain.Answer), len(plain.Ns))
		}
		for _, section := range [][]dns.RR{clear.Answer, clear.Ns, clear.Extra} {
			for _, rr := range section {
				switch rr.Header().Rrtype {
				case dns.TypeRRSIG, dns.TypeNSEC, dns.TypeDNSKEY:
					t.Errorf("type %d: DO-clear response carries DNSSEC record %v", qtype, rr)
				}
			}
		}
	}
}
