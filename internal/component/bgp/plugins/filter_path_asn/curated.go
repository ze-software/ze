// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// Detail: register_completion.go -- the completion this table feeds
// Detail: command.go -- the show annotation and the pasteable block it prints
// Related: config.go -- the config parse, which does NOT read this table
//
// Curated: 2026-09-02
// Source: https://en.wikipedia.org/wiki/Tier_1_network -- 14 networks, no "as of" date
// Source: anuragbhatia.com (2026-08-06) -- adds AS174 and gives a departure history
//
// The transit-free networks, as an annotation and a suggestion. A network is in
// this table when both sources describe it as selling transit and buying none,
// so it reaches every destination through settlement-free peering alone.
//
// This table DECIDES NOTHING. It feeds three surfaces and no others, all of
// them read-only: the completion on the `asn` leaf-list
// (register_completion.go), the annotation column of `show bgp reject-asn`, and
// the pasteable block `show bgp reject-asn known transit-free` prints
// (command.go). No config path, no filter path and no commit rule reads it, and
// `TestCuratedTableDecidesNothing` holds that line.
//
// The separation is the whole design. Ze ships no ASN set that anything acts
// on, because a curated set that DECIDED would go wrong in silence: a network
// that leaves the transit-free club keeps having its routes dropped, with
// nothing in the operator's config to point at. A stale suggestion costs one
// keystroke. An operator who wants these ASNs pastes them in, and after the
// paste the config holds NUMBERS, so a later edit to this table cannot change
// what that config does.
//
// There is no refresh verb, and that is deliberate rather than unfinished. No
// authority publishes this set in a machine-readable form, so the header says
// `Curated:` where a generated table would say `Generated:`, and the date is
// the only staleness signal there is. `show bgp reject-asn known transit-free`
// prints it.

package filter_path_asn

import "github.com/ze-software/ze/internal/core/textbuf"

// curatedNetwork is one transit-free network as the two sources describe it.
//
// contested is a NAMED fact and not an absence: it says the two sources
// disagree about the entry, and note says why. Recording the dispute is what
// the annotation owes a reader. Resolving it quietly, in either direction,
// would put an editorial judgement on a surface that has no way to show its
// working.
type curatedNetwork struct {
	asn       uint32
	name      string
	contested bool
	note      string
}

// curatedDate is the day the table below was curated, and curatedSources names
// where it came from. Both are also written in this file's header, because a
// reader opening the file is owed them there.
//
// They are DECLARED here so `show bgp reject-asn known transit-free` prints the
// same provenance the header carries, rather than a second copy that would drift
// from it. TestCuratedTableHasSourcesAndDate compares the two.
const curatedDate = "2026-09-02"

var curatedSources = []string{
	`https://en.wikipedia.org/wiki/Tier_1_network -- 14 networks, no "as of" date`,
	`anuragbhatia.com (2026-08-06) -- adds AS174 and gives a departure history`,
}

// curatedTransitFree holds the 15 networks the two sources name, sorted by ASN.
// Fourteen come from both sources. AS174 comes from the dated source alone, and
// carries the dispute it records.
var curatedTransitFree = []curatedNetwork{
	{
		asn:       174,
		name:      "Cogent Communications",
		contested: true,
		note: "the dated source records AS174 added to and removed from the " +
			"transit-free set repeatedly, over a declared paid-peering " +
			"arrangement and a gap in its IPv6 peering. The undated source " +
			"does not list it.",
	},
	{asn: 701, name: "Verizon Business"},
	{asn: 1299, name: "Arelion"},
	{asn: 2914, name: "NTT Global IP Network"},
	{asn: 3257, name: "GTT Communications"},
	{asn: 3320, name: "Deutsche Telekom Global Carrier"},
	{asn: 3356, name: "Lumen Technologies"},
	{asn: 3491, name: "PCCW Global"},
	{asn: 5511, name: "Orange International Carriers"},
	{asn: 6453, name: "Tata Communications"},
	{asn: 6461, name: "Zayo Group"},
	{asn: 6762, name: "Telecom Italia Sparkle"},
	{asn: 6830, name: "Liberty Global"},
	{asn: 7018, name: "AT&T Services"},
	{asn: 12956, name: "Telxius"},
}

// curatedLookup returns the table entry for asn.
//
// The scan is linear over 15 entries and no index is built, because both
// callers are control plane: an operator pressing Tab, and one `show` command
// rendering its annotation column. An index would be machinery that buys
// nothing on either path.
func curatedLookup(asn uint32) (curatedNetwork, bool) {
	for _, network := range curatedTransitFree {
		if network.asn == asn {
			return network, true
		}
	}
	return curatedNetwork{}, false
}

// curatedAnnotation names the network behind one ASN, and returns the EMPTY
// string for an ASN this table does not hold.
//
// Empty rather than a guess, and empty rather than nothing at all: the two
// surfaces that read it (the completion dropdown and the `show bgp reject-asn`
// annotation column) both print the ASN whatever this answers, so an unknown ASN
// is listed with no annotation rather than dropped (AC-25).
//
// A contested entry carries the word in the annotation. The reason is one
// sentence long and lives in the table; the word here is the flag that sends a
// reader to it, because a dropdown row and a table cell each have one line.
func curatedAnnotation(asn uint32) string {
	network, found := curatedLookup(asn)
	if !found {
		return ""
	}
	if !network.contested {
		return network.name
	}
	var tb textbuf.Buffer
	return tb.Str(network.name).Str(" (contested)").String()
}
