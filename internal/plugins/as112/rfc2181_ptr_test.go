// Design: docs/architecture/dns/as112.md -- RFC 2181 Section 10.2: the AS112 reverse zones serve no PTR

package as112

import (
	"testing"

	"github.com/miekg/dns"
)

// TestRFC2181ReverseZonesServeNoPTR proves the authoritative half of RFC 2181
// Section 10.2, "Note that while the value of a PTR record must not be an
// alias, there is no requirement that the process of resolving a PTR record
// not encounter any aliases.": an AS112 node serves no PTR record at all, so
// no PTR value it sends can be an alias.
//
// Method: a PTR query at the apex of the 168.192.in-addr.arpa reverse zone,
// and one for an address inside it, through answerQuestions. RFC 7534 Section
// 3.5 has these zones hold an SOA and an NS set at the apex and nothing else,
// so the apex query is NODATA and the address query is NXDOMAIN, each with an
// empty Answer section and the zone SOA in Authority. The second half of the
// sentence, an alias met on the way to a PTR, binds the resolver and is
// proven in internal/plugins/dnsresolver.
func TestRFC2181ReverseZonesServeNoPTR(t *testing.T) {
	t.Parallel()
	ask := func(name string) *dns.Msg {
		r := new(dns.Msg)
		r.SetQuestion(name, dns.TypePTR)
		msg := new(dns.Msg)
		msg.SetReply(r)
		answerQuestions(msg, r, 1, "", "", "")
		return msg
	}

	// RFC requirement: RFC2181-10.2-1 positive -- a PTR query at the reverse zone apex 168.192.in-addr.arpa. is answered NOERROR, and one for 1.0.168.192.in-addr.arpa. NXDOMAIN, each with an empty Answer section and the zone SOA alone in Authority, so the node serves no PTR value.
	cases := []struct {
		name  string
		rcode int
	}{
		{"168.192.in-addr.arpa.", dns.RcodeSuccess},
		{"1.0.168.192.in-addr.arpa.", dns.RcodeNameError},
	}
	for _, tc := range cases {
		msg := ask(tc.name)
		if len(msg.Answer) != 0 {
			t.Errorf("PTR %s: answer %v, want an empty Answer section", tc.name, msg.Answer)
		}
		if msg.Rcode != tc.rcode {
			t.Errorf("PTR %s: rcode %s, want %s", tc.name, dns.RcodeToString[msg.Rcode], dns.RcodeToString[tc.rcode])
		}
		if len(msg.Ns) != 1 || msg.Ns[0].Header().Rrtype != dns.TypeSOA {
			t.Errorf("PTR %s: authority %v, want the zone SOA alone", tc.name, msg.Ns)
		}
	}
}
