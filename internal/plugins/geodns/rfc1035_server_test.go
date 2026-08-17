// Design: docs/architecture/dns/geodns.md -- geodns owns answer policy, and the
// TTL that reaches the wire is part of it. This file pins that the zone's SOA
// MINIMUM is NOT a floor on it. The file keeps its rfc1035 name because RFC 1035
// section 3.3.13 is the requirement it settles, by recording that Ze does not
// implement it.
// RFC: rfc/short/rfc2181.md -- TTL bounds and RRSet TTL equality

// rfc-test-change-approved: 2026-08-12 Thomas approved replacing the two
// RFC1035-3.3.13-1 tagged tests, and their three tags, with tests that pin the
// ABSENCE of the floor. Grounds: RFC 2308 (Standards Track, "Updates: 1034,
// 1035") section 4 deprecates RFC 1035 section 3.3.13's zone-wide TTL floor.
// These tests pinned the floor. They now pin its absence. He preferred the
// rewrite over deleting the file so the ruling becomes an enforced invariant,
// and so nobody re-adds the floor when you read only section 3.3.13.

package geodns

import (
	"net/netip"
	"testing"

	"github.com/miekg/dns"
)

// answerTTLs returns the TTL of every record of type rrtype across all three
// sections of msg.
func answerTTLs(msg *dns.Msg, rrtype uint16) []uint32 {
	var ttls []uint32
	for _, section := range [][]dns.RR{msg.Answer, msg.Ns, msg.Extra} {
		for _, rr := range section {
			if rr.Header().Rrtype == rrtype {
				ttls = append(ttls, rr.Header().Ttl)
			}
		}
	}
	return ttls
}

// answerA drives answerQuestions for one A query against st and returns the
// reply, mimicking the harness's msg.SetReply(r) shaping.
func answerA(st *resolverState, name string) *dns.Msg {
	r := new(dns.Msg)
	r.SetQuestion(name, dns.TypeA)
	msg := new(dns.Msg)
	msg.SetReply(r)
	answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))
	return msg
}

// VALIDATES: every emitted TTL is the one the operator configured, under a zone
// whose SOA MINIMUM is 300. A record at 120 serves at 120 and a record at 900
// serves at 900, so the emitted value tracks the config in both directions
// rather than matching one constant. The synthesized NS records and their glue
// serve at the default TTL. The SOA's own TTL and its MINTTL field are still
// 300, because MINIMUM keeps its one remaining meaning there.
//
// RFC 1035 section 3.3.13 asks for the opposite: "Whenever a RR is sent in a
// response to a query, the TTL field is set to the maximum of the TTL field from
// the RR and the MINIMUM field in the appropriate SOA." RFC 2308 (Standards
// Track, "Updates: 1034, 1035") section 4 withdraws it: "Despite being the
// original defined meaning, the first of these, the minimum TTL value of all RRs
// in a zone, has never in practice been used and is hereby deprecated." What
// MINIMUM still means is the negative-caching TTL of RFC 2308 section 5, which
// the last two assertions hold.
//
// These tests carry no `RFC requirement:` tag. RFC1035-3.3.13-1 would republish
// a withdrawn obligation as proven, and RFC 2308 has no summary in rfc/short/
// and is not enrolled, so a tag naming it would resolve to nothing.
//
// PREVENTS: the floor being reintroduced by a reader who finds RFC 1035 section
// 3.3.13 and not the update that withdrew it. It was implemented once, on
// 2026-08-12, and reverted the same day. Its operator-visible cost is that the
// default MINIMUM of 300 silently lengthens every record configured below 300
// seconds, which is what turned TestRFC2181_RRSetEqualTTL red.
func TestRFC2308_NoZoneWideTTLFloor(t *testing.T) {
	t.Parallel()

	st := answerState(t, `{"service":{"geodns":{"enabled":"true","zone":["t.example."],`+
		`"default-ttl":"120","soa":{"minimum":"300"},`+
		`"nameserver":["10.0.0.1"],`+
		`"host-set":{"web":{"host":{`+
		`"low.t.example.":{"ttl":"120","address":["10.0.0.5"]},`+
		`"high.t.example.":{"ttl":"900","address":["10.0.0.6"]}}}},`+
		`"source":{"0.0.0.0/0":{"host-set":"web"}}}}}`)

	below := answerTTLs(answerA(st, "low.t.example."), dns.TypeA)
	if len(below) != 1 {
		t.Fatalf("got %d A records for the host less than MINIMUM, want 1", len(below))
	}
	if below[0] != 120 {
		t.Errorf("A TTL = %d for a record configured at 120 under a MINIMUM of 300, want 120", below[0])
	}

	// The second host is what stops the first assertion passing against a TTL
	// hard-coded at 120: 900 is above the MINIMUM, so only a path that emits the
	// configured value satisfies both.
	above := answerTTLs(answerA(st, "high.t.example."), dns.TypeA)
	if len(above) != 1 {
		t.Fatalf("got %d A records for the above-MINIMUM host, want 1", len(above))
	}
	if above[0] != 900 {
		t.Errorf("A TTL = %d for a record configured at 900 under a MINIMUM of 300, want 900", above[0])
	}

	r := new(dns.Msg)
	r.SetQuestion("t.example.", dns.TypeNS)
	msg := new(dns.Msg)
	msg.SetReply(r)
	answerQuestions(msg, r, st, netip.MustParseAddr("203.0.113.7"))

	ns := answerTTLs(msg, dns.TypeNS)
	if len(ns) == 0 {
		t.Fatal("no NS record in the reply")
	}
	for i, ttl := range ns {
		if ttl != 120 {
			t.Errorf("NS record %d TTL = %d under a MINIMUM of 300 and default-ttl 120, want 120", i, ttl)
		}
	}
	glue := answerTTLs(msg, dns.TypeA)
	if len(glue) == 0 {
		t.Fatal("no A glue in the reply")
	}
	for i, ttl := range glue {
		if ttl != 120 {
			t.Errorf("glue A record %d TTL = %d under a MINIMUM of 300 and default-ttl 120, want 120", i, ttl)
		}
	}

	// MINIMUM still reaches the wire, as the negative-caching TTL of the SOA in
	// the Authority section of a negative answer (RFC 2308 section 5). Asserting
	// it is what stops every assertion above passing vacuously: delete the TTL
	// path entirely and each one becomes 0, so these fail too.
	negative := answerA(st, "absent.t.example.")
	soa := answerTTLs(negative, dns.TypeSOA)
	if len(soa) != 1 {
		t.Fatalf("got %d SOA records in the negative answer, want 1", len(soa))
	}
	if soa[0] != 300 {
		t.Errorf("SOA TTL = %d, want the configured MINIMUM of 300", soa[0])
	}
	for _, rr := range negative.Ns {
		if s, ok := rr.(*dns.SOA); ok && s.Minttl != 300 {
			t.Errorf("SOA MINTTL = %d, want the configured MINIMUM of 300", s.Minttl)
		}
	}
}
