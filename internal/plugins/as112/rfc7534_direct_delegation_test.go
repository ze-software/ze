// Design: docs/architecture/dns/as112.md -- RFC 7534 section 3.5 over every served zone

package as112

import (
	"testing"

	"github.com/miekg/dns"
)

// VALIDATES: RFC 7534 section 3.5 over EVERY Direct-Delegation zone the node
// serves, through the real handler (which sets AA): each zone is answered
// authoritatively from its own data, and no query type at its apex draws a
// record other than its SOA and NS. The hostname zones are left out: section
// 3.4 has them carry TXT records, so they are not Direct-Delegation zones.
// PREVENTS: a zone of the nominated set answered without authority, or a
// Direct-Delegation zone hosting a TXT, A, PTR or any other record.
func TestRFC7534EveryDirectDelegationZoneAuthoritativeAndBare(t *testing.T) {
	h := as112Handler(t)
	probed := 0
	for _, zone := range servedZones() {
		name := dns.Fqdn(zone.Name)
		if isHostnameZone(name) {
			continue
		}
		probed++
		soa := askAS112(t, h, name, dns.TypeSOA)
		// RFC requirement: RFC7534-3.5-1 positive -- every served Direct-Delegation zone answers a SOA query at its apex with AA set, NOERROR, and its own SOA in the Answer section.
		if !soa.Authoritative || soa.Rcode != dns.RcodeSuccess {
			t.Errorf("%s SOA: AA=%v rcode=%s, want an authoritative NOERROR", name, soa.Authoritative, dns.RcodeToString[soa.Rcode])
		}
		if len(soa.Answer) != 1 || soa.Answer[0].Header().Rrtype != dns.TypeSOA || dns.CanonicalName(soa.Answer[0].Header().Name) != dns.CanonicalName(name) {
			t.Errorf("%s SOA answer = %v, want the zone's own SOA", name, soa.Answer)
		}
		// RFC requirement: RFC7534-3.5-2 positive -- at the apex of every Direct-Delegation zone, queries for A, AAAA, TXT, PTR, MX, CNAME and ANY return no record of a type other than SOA or NS in any section.
		for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT, dns.TypePTR, dns.TypeMX, dns.TypeCNAME, dns.TypeANY} {
			reply := askAS112(t, h, name, qtype)
			for _, section := range [][]dns.RR{reply.Answer, reply.Ns, reply.Extra} {
				for _, rr := range section {
					if kind := rr.Header().Rrtype; kind != dns.TypeSOA && kind != dns.TypeNS {
						t.Errorf("%s %s: zone holds a %s record: %v", name, dns.TypeToString[qtype], dns.TypeToString[kind], rr)
					}
				}
			}
		}
	}
	if probed < 20 {
		t.Fatalf("probed %d Direct-Delegation zones, want the 19 reverse zones and EMPTY.AS112.ARPA", probed)
	}
}
