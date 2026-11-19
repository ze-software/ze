// Design: docs/architecture/dns/geodns.md -- the client-subnet option geodns
// validates and echoes as an RFC 7871 Authoritative Nameserver.
// RFC: rfc/short/rfc7871.md -- EDNS0 Client Subnet, Section 7.2.1
// Related: rfc7871_echo_test.go -- the positives of the rows this file adds
// the negatives of; storeEchoConfig, ecsQuery, subnet, onWire, replySubnet

package geodns

import (
	"testing"

	"github.com/miekg/dns"
)

// TestRFC7871_EchoTiedToEnabledSupport proves that the missing option of
// RFC 7871 Section 7.2.1 ("Such a server MUST NOT include an ECS option
// within replies to indicate lack of support for it.") is tied to support
// being off: the query that draws no option under client-ip-source packet
// draws one when geodns has ECS enabled, the default edns0-then-packet mode.
// Method: the same tailored A query, carrying 203.0.113.0/24, under the
// default mode, its reply packed and unpacked.
func TestRFC7871_EchoTiedToEnabledSupport(t *testing.T) {
	storeEchoConfig(t, echoConfig, "")
	reply := onWire(t, ecsQuery("www.t.example.", dns.TypeA, subnet(1, 24, 0, "203.0.113.0")))
	// RFC requirement: RFC7871-7.2.1-2 negative -- with ECS enabled (the
	// default client-ip-source, which reads the option), the query carrying
	// 203.0.113.0/24 that draws no option under packet draws a reply carrying
	// an ECS option.
	if firstA(reply) != "10.0.0.5" {
		t.Fatalf("A = %q, want the tailored 10.0.0.5", firstA(reply))
	}
	if replySubnet(reply) == nil {
		t.Errorf("ECS enabled: the reply carries no ECS option; only a server without ECS support omits it")
	}
}

// TestRFC7871_EchoMatchesOnlyTheThreeFields proves that the match of RFC 7871
// Section 7.2.1 ("FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response
// MUST match those in the query.") binds those three fields and no fourth:
// SCOPE PREFIX-LENGTH, which a query sets to 0, is the answer's own in the
// reply, so the echo is not a copy of the query's option. Method: a query
// carrying 203.0.113.0/24 with SCOPE 0 draws the A chosen from the
// configured 203.0.113.0/24, and the echo is read field by field.
func TestRFC7871_EchoMatchesOnlyTheThreeFields(t *testing.T) {
	storeEchoConfig(t, echoConfig, "")
	query := subnet(1, 24, 0, "203.0.113.0")
	reply := onWire(t, ecsQuery("www.t.example.", dns.TypeA, query))
	echo := replySubnet(reply)
	if firstA(reply) != "10.0.0.5" || echo == nil {
		t.Fatalf("A %q echo %v, want the tailored 10.0.0.5 and an echo", firstA(reply), echo)
	}
	if echo.Family != query.Family || echo.SourceNetmask != query.SourceNetmask || !echo.Address.Equal(query.Address) {
		t.Fatalf("echo %s, want FAMILY, SOURCE PREFIX-LENGTH and ADDRESS of the query %s", echo, query)
	}
	// RFC requirement: RFC7871-7.2.1-8 negative -- the field the match does
	// not bind differs from the query's: the query's SCOPE PREFIX-LENGTH is 0,
	// and the reply's is 24, the configured source prefix the answer came
	// from, while FAMILY, SOURCE PREFIX-LENGTH and ADDRESS equal the query's.
	if echo.SourceScope != 24 {
		t.Errorf("echo SCOPE PREFIX-LENGTH %d, want 24, not the query's %d", echo.SourceScope, query.SourceScope)
	}
}
