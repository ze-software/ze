// Goal: prove the zone answers the response code each name declares, and that a
// name it holds without a record of the queried type answers NODATA rather than
// NXDOMAIN. Method: start a real server on an ephemeral port and query it over
// UDP, which is the path a daemon takes.
//
// VALIDATES: AC-3, AC-4, AC-5.
// PREVENTS: the failure this stub exists to avoid making. resolveAndRecord
// (internal/component/firewall/plugins/domain/domain.go) empties a name's
// contribution on NXDOMAIN and keeps the last good answer on SERVFAIL and
// REFUSED, so a stub that answered NXDOMAIN for the AAAA of an IPv4-only name
// would delete a live address from a firewall set and the test would call that
// correct.

package dns

import (
	"testing"

	mdns "github.com/miekg/dns"
)

func TestZoneAnswersEveryRcode(t *testing.T) {
	server := startForTest(t)

	for _, test := range []struct {
		name  string
		qtype uint16
		rcode int
	}{
		{"web.example.test", mdns.TypeA, mdns.RcodeSuccess},
		{"web.example.test", mdns.TypeAAAA, mdns.RcodeSuccess},
		{"v4only.example.test", mdns.TypeA, mdns.RcodeSuccess},
		{"cached.example.test", mdns.TypeA, mdns.RcodeSuccess},
		{"broken.example.test", mdns.TypeA, mdns.RcodeServerFailure},
		{"broken.example.test", mdns.TypeAAAA, mdns.RcodeServerFailure},
		{"refused.example.test", mdns.TypeA, mdns.RcodeRefused},
		{"refused.example.test", mdns.TypeAAAA, mdns.RcodeRefused},
		{"absent.example.test", mdns.TypeA, mdns.RcodeNameError},
		{"absent.example.test", mdns.TypeAAAA, mdns.RcodeNameError},
	} {
		reply := ask(t, server, test.name, test.qtype)
		if reply.Rcode != test.rcode {
			t.Errorf("%s %s: rcode %s, want %s", test.name, mdns.TypeToString[test.qtype],
				mdns.RcodeToString[reply.Rcode], mdns.RcodeToString[test.rcode])
		}
		if test.rcode != mdns.RcodeSuccess && len(reply.Answer) != 0 {
			t.Errorf("%s %s: %d records with rcode %s, want none",
				test.name, mdns.TypeToString[test.qtype], len(reply.Answer), mdns.RcodeToString[test.rcode])
		}
	}
}

// A name that holds only an A record answers AAAA with NOERROR and an empty
// answer section, which RFC 2308 Section 2.2 states as "NODATA is indicated by
// an answer with the RCODE set to NOERROR and no relevant answers in the answer
// section".
func TestNameWithoutThatTypeAnswersNoData(t *testing.T) {
	server := startForTest(t)

	reply := ask(t, server, "v4only.example.test", mdns.TypeAAAA)
	if reply.Rcode != mdns.RcodeSuccess {
		t.Fatalf("AAAA of an IPv4-only name: rcode %s, want NOERROR -- a caller reads anything else as the name being gone",
			mdns.RcodeToString[reply.Rcode])
	}
	if len(reply.Answer) != 0 {
		t.Fatalf("AAAA of an IPv4-only name: %d records, want none", len(reply.Answer))
	}

	// The same name still answers A, so the NODATA above is about the TYPE and
	// not about the name having been dropped from the zone.
	reply = ask(t, server, "v4only.example.test", mdns.TypeA)
	if len(reply.Answer) != 1 {
		t.Fatalf("A of an IPv4-only name: %d records, want 1", len(reply.Answer))
	}
}

// Every name the usage text can name is a name the server answers for, so the
// zone table and the answers cannot disagree.
func TestEveryZoneNameResolves(t *testing.T) {
	server := startForTest(t)

	zone := defaultZone()
	if len(zone) == 0 {
		t.Fatal("the default zone is empty")
	}
	for name, entry := range zone {
		reply := ask(t, server, name, mdns.TypeA)
		if reply.Rcode == mdns.RcodeNameError {
			t.Errorf("%s: NXDOMAIN for a name the zone declares", name)
		}
		if reply.Rcode != entry.Rcode {
			t.Errorf("%s: rcode %s, want %s", name, mdns.RcodeToString[reply.Rcode], mdns.RcodeToString[entry.Rcode])
		}
	}
}
