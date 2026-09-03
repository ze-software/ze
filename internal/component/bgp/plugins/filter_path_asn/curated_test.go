package filter_path_asn

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCuratedTableHasSourcesAndDate holds the provenance of a table nothing can
// refresh. There is no machine-readable authority for the transit-free set, so
// the header IS the staleness signal: two named sources and one curated date,
// which `show bgp reject-asn known transit-free` prints for the operator.
//
// VALIDATES: curated.go names both sources and carries a Curated date, and says
// Curated rather than Generated, because no verb regenerates it.
// PREVENTS: the header being dropped or reworded in an edit, leaving a table
// whose age and origin no reader can recover.
func TestCuratedTableHasSourcesAndDate(t *testing.T) {
	header := curatedFileHeader(t)

	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, curatedDate, "the curated date is a date")
	assert.Contains(t, header, "// Curated: "+curatedDate,
		"the header must carry the date the table was curated")
	assert.NotContains(t, header, "\n// Generated:",
		"nothing generates this table; a Generated line would promise a refresh verb")

	// The constants are what `show bgp reject-asn known transit-free` prints, so
	// the header and the constants must be one fact rather than two copies.
	require.Len(t, curatedSources, 2, "the table is built from two sources")
	for _, source := range curatedSources {
		assert.Contains(t, header, "// Source: "+source,
			"the header and curatedSources must name the same source")
	}
	assert.Contains(t, header, "en.wikipedia.org/wiki/Tier_1_network",
		"the undated source must be named")
	assert.Contains(t, header, "anuragbhatia.com",
		"the dated source must be named")
	assert.Contains(t, header, "2026-08-06",
		"the dated source's own date must be recorded")
}

// TestCuratedTableDecidesNothing is the whole design in one assertion. The
// curated set feeds completion and the show annotation, and NO path that
// decides anything reads it.
//
// A curated set that decided would go wrong in silence: a network that leaves
// the transit-free club keeps having its routes dropped, and nothing in the
// operator's config names it. A stale suggestion costs one keystroke.
//
// VALIDATES: neither the config parse nor the AS_PATH scan nor the filter
// decision mentions the curated table.
// PREVENTS: a later edit reaching for curatedLookup inside parseRejectASNLists
// or matchPositions, which would ship an ASN set Ze acts on.
func TestCuratedTableDecidesNothing(t *testing.T) {
	// Named, not globbed. A glob that matches nothing passes, and this test
	// exists precisely to fail when one of these files changes.
	for _, name := range []string{"config.go", "match.go", "filter_path_asn.go"} {
		body, err := os.ReadFile(name)
		require.NoError(t, err, "%s is the file this test exists to read", name)
		assert.NotContains(t, string(body), "curated",
			"%s decides something, so it must not read the curated table", name)
	}
}

// TestCuratedTableShape pins what the two sources say, so an edit that adds,
// drops or renames an entry is a deliberate act with a red test in front of it.
//
// VALIDATES: 15 entries, sorted by ASN, each ASN once, each with a network
// name; AS174 alone is contested and carries the reason.
// PREVENTS: a duplicate ASN shadowing an entry, an unnamed entry rendering an
// empty annotation, and a contested entry losing the dispute that makes it
// contested (R-3, which is discharged by RECORDING the disagreement).
func TestCuratedTableShape(t *testing.T) {
	require.Len(t, curatedTransitFree, 15,
		"14 networks from both sources plus AS174 from the dated one")

	seen := make(map[uint32]bool, len(curatedTransitFree))
	previous := uint32(0)
	for _, network := range curatedTransitFree {
		assert.NotEmpty(t, network.name, "AS%d has no network name", network.asn)
		assert.False(t, seen[network.asn], "AS%d appears twice", network.asn)
		assert.Greater(t, network.asn, previous, "the table is sorted by ASN")
		seen[network.asn] = true
		previous = network.asn

		if network.contested {
			assert.NotEmpty(t, network.note,
				"AS%d is contested and must say why", network.asn)
			continue
		}
		assert.Empty(t, network.note,
			"AS%d is not contested, so a note would have no subject", network.asn)
	}

	for _, asn := range []uint32{174, 701, 1299, 2914, 3257, 3320, 3356, 3491,
		5511, 6453, 6461, 6762, 6830, 7018, 12956} {
		_, found := curatedLookup(asn)
		assert.True(t, found, "AS%d is named by the sources and must be in the table", asn)
	}
}

// TestCuratedContestedEntryRecordsTheDispute holds R-3 to its resolution: the
// two sources disagree about AS174, and the table records the disagreement
// rather than picking a side. Picking one quietly would put an editorial
// judgement on a surface that cannot show its working.
//
// VALIDATES: AS174 is contested, and its note names both halves of the reason
// the dated source gives.
// PREVENTS: the dispute being resolved in a later edit with no discussion.
func TestCuratedContestedEntryRecordsTheDispute(t *testing.T) {
	cogent, found := curatedLookup(174)
	require.True(t, found)
	require.True(t, cogent.contested, "the sources disagree about AS174")

	assert.Contains(t, cogent.note, "paid-peering",
		"the note must carry the declared paid-peering arrangement")
	assert.Contains(t, cogent.note, "IPv6",
		"the note must carry the IPv6 peering gap")

	contested := 0
	for _, network := range curatedTransitFree {
		if network.contested {
			contested++
		}
	}
	assert.Equal(t, 1, contested, "AS174 is the only entry the sources disagree about")
}

// TestCuratedLookupAnswersAbsence proves a miss is an answer and not a zero the
// caller cannot tell from a hit.
//
// VALIDATES: curatedLookup reports found=false for an ASN outside the table,
// including ASN 0.
// PREVENTS: an empty curatedNetwork read as a real entry, which would render an
// annotation column with a blank network name instead of no annotation.
func TestCuratedLookupAnswersAbsence(t *testing.T) {
	for _, asn := range []uint32{0, 64512, 65000, 4294967295} {
		network, found := curatedLookup(asn)
		assert.False(t, found, "AS%d is not in the curated table", asn)
		assert.Empty(t, network.name, "a miss returns no name to print")
	}

	cogent, found := curatedLookup(174)
	require.True(t, found)
	assert.Equal(t, "Cogent Communications", cogent.name)
}

// curatedFileHeader returns the comment block at the top of curated.go, up to
// the package clause. Reading the file is the point: the header is prose that
// no compiler checks, and this is the only thing that can notice its loss.
func curatedFileHeader(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("curated.go")
	require.NoError(t, err, "curated.go is the file this test exists to read")

	header, _, found := strings.Cut(string(body), "\npackage ")
	require.True(t, found, "curated.go must have a package clause after its header")
	return header
}
