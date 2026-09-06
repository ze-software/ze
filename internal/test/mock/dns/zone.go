// Design: docs/functional-tests.md -- the zone `ze-test dns` answers
// Related: server.go -- the server that serves it; dns.go -- the subcommand
// RFC: rfc/short/rfc1035.md -- the RCODE values; rfc/full/rfc2308.txt -- NODATA

package dns

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	mdns "github.com/miekg/dns"
)

// maxAddressesPerAnswer bounds how many addresses one name answers for one
// record type. It is the cap the first consumer of this stub applies to what
// one name contributes to one family (maxAddressesPerName,
// internal/component/firewall/plugins/domain/sets.go), so a larger answer would
// exercise a path that product truncates anyway.
const maxAddressesPerAnswer = 64

// Answer is the address set the stub holds for one name and one record type.
type Answer struct {
	// TTL is what every record in this answer carries.
	//
	// Zero is the useful default and not an unset value: cache.put
	// (internal/component/resolve/dns/cache.go) returns before storing a
	// zero-TTL answer, so a fixture that changes an answer sees the change on
	// the next lookup instead of reading the daemon's own cache back. A zero
	// TTL does not disarm a TTL-driven refresh either, because refreshInterval
	// (internal/component/firewall/plugins/domain/schedule.go) takes the
	// maximum of the TTL and the group's floor.
	TTL uint32
	// Addresses are the records the reply carries. Empty is legal and answers
	// NOERROR with an empty answer section.
	Addresses []netip.Addr
}

// Name is everything the stub answers for one name.
//
// The response code belongs to the NAME and the addresses belong to the TYPE. A
// server that cannot answer cannot answer for any type, while a name that
// exists and holds no record of the queried type answers NOERROR with an empty
// answer section (RFC 2308 Section 2.2) rather than NXDOMAIN. A stub that
// collapsed the two would tell a caller the name is GONE.
type Name struct {
	// Rcode is the response code every query for this name is answered with:
	// mdns.RcodeSuccess, mdns.RcodeServerFailure or mdns.RcodeRefused. A name
	// absent from the zone answers mdns.RcodeNameError and needs no entry, so
	// that code never appears here.
	Rcode int
	// Answers holds the records for each record type this name carries. A type
	// absent from it answers NODATA.
	Answers map[uint16]Answer
}

// defaultZone is what a stub answers before a test changes anything. It is a
// function rather than a package var so every Server owns its own maps: two
// servers in one process, and two tests in a row, must not share state a Set
// can reach.
//
// The addresses come from the documentation ranges of RFC 5737 (203.0.113.0/24,
// 198.51.100.0/24) and RFC 3849 (2001:db8::/32). The names sit under `.test`,
// which RFC 6761 Section 6.2 reserves for testing.
func defaultZone() map[string]Name {
	return map[string]Name{
		// The dual-family name the firewall domain-group tests already name.
		canonical("web.example.test"): {
			Rcode: mdns.RcodeSuccess,
			Answers: map[uint16]Answer{
				mdns.TypeA:    {Addresses: []netip.Addr{netip.MustParseAddr("203.0.113.10")}},
				mdns.TypeAAAA: {Addresses: []netip.Addr{netip.MustParseAddr("2001:db8::10")}},
			},
		},
		// One family only, so a query for the other answers NODATA. This is the
		// case a stub that answered NXDOMAIN would get wrong, and the one that
		// would empty a live firewall set.
		canonical("v4only.example.test"): {
			Rcode: mdns.RcodeSuccess,
			Answers: map[uint16]Answer{
				mdns.TypeA: {Addresses: []netip.Addr{netip.MustParseAddr("198.51.100.10")}},
			},
		},
		// A non-zero TTL, for a test of the resolver cache or of a TTL-driven
		// refresh schedule.
		canonical("cached.example.test"): {
			Rcode: mdns.RcodeSuccess,
			Answers: map[uint16]Answer{
				mdns.TypeA: {TTL: 300, Addresses: []netip.Addr{netip.MustParseAddr("203.0.113.20")}},
			},
		},
		// The server-side failure. statusFromRcode
		// (internal/component/resolve/dns/resolver.go) maps it to
		// StatusServerFailure, which is not Authoritative, so a caller keeps
		// its last good answer instead of emptying what it programmed.
		canonical("broken.example.test"): {Rcode: mdns.RcodeServerFailure},
		// The second non-authoritative code, which reaches the caller through a
		// different branch of statusFromRcode.
		canonical("refused.example.test"): {Rcode: mdns.RcodeRefused},
	}
}

// canonical is the key a name is stored and looked up under: lower case, with
// the trailing dot a query carries. Declaring it once is what lets a fixture
// write `web.example.test` and a query arrive as `WEB.example.test.`.
func canonical(name string) string {
	return strings.ToLower(mdns.Fqdn(name))
}

// checkAnswer refuses an answer the stub cannot serve for qtype.
//
// The zone is Go source and Set is called from compiled fixtures, so every
// input here is written by a test author and a mismatch is a programmer error.
// No DNS query reaches this function: a peer cannot make it panic. It refuses
// rather than skipping the address, because a stub that quietly served fewer
// records than it was given would make the test reading them assert the wrong
// thing (ai/rules/principles.md).
func checkAnswer(qtype uint16, answer Answer) {
	if qtype != mdns.TypeA && qtype != mdns.TypeAAAA {
		panic("BUG: ze-test dns: the stub serves A and AAAA only, not " + mdns.TypeToString[qtype])
	}
	if len(answer.Addresses) > maxAddressesPerAnswer {
		panic(fmt.Sprintf("BUG: ze-test dns: %d addresses in one answer, at most %d", len(answer.Addresses), maxAddressesPerAnswer))
	}
	for _, address := range answer.Addresses {
		if !address.IsValid() {
			panic("BUG: ze-test dns: an answer carries the zero address")
		}
		if qtype == mdns.TypeA && !address.Is4() {
			panic("BUG: ze-test dns: " + address.String() + " is not an IPv4 address, so it cannot answer an A query")
		}
		if qtype == mdns.TypeAAAA && address.Is4() {
			panic("BUG: ze-test dns: " + address.String() + " is not an IPv6 address, so it cannot answer an AAAA query")
		}
	}
}

// zoneUsage renders the zone for the usage text, one line per name.
//
// It is derived from the table rather than written beside it, so a name added
// to the zone appears in `--help` with no second edit and no chance of the two
// disagreeing (ai/rules/evidence.md).
func zoneUsage(zone map[string]Name) string {
	names := make([]string, 0, len(zone))
	for name := range zone {
		names = append(names, name)
	}
	slices.Sort(names)

	var out strings.Builder
	for _, name := range names {
		entry := zone[name]
		fmt.Fprintf(&out, "  %-22s %-8s", strings.TrimSuffix(name, "."), mdns.RcodeToString[entry.Rcode])

		types := make([]uint16, 0, len(entry.Answers))
		for qtype := range entry.Answers {
			types = append(types, qtype)
		}
		slices.Sort(types)
		if len(types) == 0 {
			out.WriteString(" no records")
		}
		for _, qtype := range types {
			answer := entry.Answers[qtype]
			addresses := make([]string, 0, len(answer.Addresses))
			for _, address := range answer.Addresses {
				addresses = append(addresses, address.String())
			}
			fmt.Fprintf(&out, " %s %s ttl=%d;", mdns.TypeToString[qtype], strings.Join(addresses, ","), answer.TTL)
		}
		out.WriteByte('\n')
	}
	return out.String()
}
