// Design: docs/architecture/plugin/rib-storage-design.md -- ROA cache for RPKI validation
// Related: roa_cache.go -- ROACache.Entries, the producer under test
package rpki

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// seedOrderedROACache fills a cache with ten VRPs whose insertion order is
// neither the address order nor the text order of their prefixes.
//
// "192.0.2.10/32" sorts before "192.0.2.2/32" as text and after it as an
// address, so an implementation that sorts the map keys as strings fails.
// 192.0.2.0/24 carries three VRPs, which pins the tie-break inside one prefix:
// max length first, then ASN. 192.0.2.0/25 shares its address with
// 192.0.2.0/24, so the prefix length breaks that pair. The IPv6 rows do the
// same for the second family table.
func seedOrderedROACache() *ROACache {
	c := newROACache()
	c.Add(makeVRP("2001:db8:1::/48", 48, 65101))
	c.Add(makeVRP("192.0.2.10/32", 32, 65010))
	c.Add(makeVRP("192.0.2.0/24", 32, 65002))
	c.Add(makeVRP("10.0.0.0/8", 24, 64500))
	c.Add(makeVRP("2001:db8::/32", 32, 65100))
	c.Add(makeVRP("192.0.2.2/32", 32, 65002))
	c.Add(makeVRP("192.0.2.0/24", 24, 65001))
	c.Add(makeVRP("2001:db8::/48", 48, 65148))
	c.Add(makeVRP("192.0.2.0/25", 25, 65025))
	c.Add(makeVRP("192.0.2.0/24", 24, 65000))
	return c
}

// orderedROARows is the one order seedOrderedROACache must answer: every IPv4
// row, then every IPv6 row, each family by address, then by prefix length, then
// by max length, then by ASN.
var orderedROARows = []DiagEntry{
	{Prefix: "10.0.0.0/8", MaxLength: 24, ASN: 64500},
	{Prefix: "192.0.2.0/24", MaxLength: 24, ASN: 65000},
	{Prefix: "192.0.2.0/24", MaxLength: 24, ASN: 65001},
	{Prefix: "192.0.2.0/24", MaxLength: 32, ASN: 65002},
	{Prefix: "192.0.2.0/25", MaxLength: 25, ASN: 65025},
	{Prefix: "192.0.2.2/32", MaxLength: 32, ASN: 65002},
	{Prefix: "192.0.2.10/32", MaxLength: 32, ASN: 65010},
	{Prefix: "2001:db8::/32", MaxLength: 32, ASN: 65100},
	{Prefix: "2001:db8::/48", MaxLength: 48, ASN: 65148},
	{Prefix: "2001:db8:1::/48", MaxLength: 48, ASN: 65101},
}

// Go randomizes where a map range starts. The seeded cache holds five IPv4
// prefix keys and three IPv6 ones, so a map-ranging implementation answers the
// sorted order by luck with probability at most 1/5 * 1/3 per call. Thirty-two
// calls put that ceiling at 15^-32, near 1e-38, so such an implementation fails
// this test rather than flaking it.
const roaOrderCallCount = 32

// TestROAEntriesAnswersSortedOrder proves that Entries answers one order.
//
// Goal: the row operators that `show bgp rpki roa` publishes ("first", "last",
// "display") must select the same VRP on every run. Method: seed ten VRPs in an
// order that is not sorted, call Entries many times, and require the full row
// sequence on every call.
//
// VALIDATES: ROACache.Entries answers IPv4 rows then IPv6 rows, each by address,
// prefix length, max length and ASN.
// PREVENTS: Go map iteration randomization reaching the CLI, where one unchanged
// cache answers a different row order on each call.
func TestROAEntriesAnswersSortedOrder(t *testing.T) {
	c := seedOrderedROACache()
	v4, v6 := c.Count()
	require.Equal(t, len(orderedROARows), v4+v6, "every seeded VRP is stored")

	for call := range roaOrderCallCount {
		require.Equal(t, orderedROARows, c.Entries(0), "call %d answers sorted order", call)
	}
}

// TestROAEntriesLimitAnswersDeterministicPrefix proves that a truncated answer
// is the same rows, in the same order, on every call.
//
// Goal: `show bgp rpki roa` answers "truncated":true beside the FIRST limit
// rows, not beside a random sample of them, so an operator diagnosing an RPKI
// problem reads one VRP set rather than a new one on each run. Method: seed
// more VRPs than the limit and require the sorted prefix of the row sequence.
//
// VALIDATES: ROACache.Entries(limit) answers the first limit rows of its sorted
// order, and the limit counts across both families rather than per family.
// PREVENTS: a map range stopping at the limit, which answers a random SUBSET of
// the VRPs and says nothing about which ones it dropped.
func TestROAEntriesLimitAnswersDeterministicPrefix(t *testing.T) {
	c := seedOrderedROACache()

	cases := []struct {
		name  string
		limit int
		rows  int
	}{
		// 3 stops inside the three VRPs of 192.0.2.0/24, so the order within one
		// prefix is pinned as well as the order of the prefixes.
		{"inside one prefix", 3, 3},
		{"first row only", 1, 1},
		// 8 stops one row into the IPv6 table, which is where a per-family limit
		// would answer a different count from a limit across the concatenation.
		{"one row past the family boundary", 8, 8},
		{"limit above the row count", 99, len(orderedROARows)},
		{"no limit", 0, len(orderedROARows)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for call := range roaOrderCallCount {
				got := c.Entries(tc.limit)
				require.Equal(t, orderedROARows[:tc.rows], got, "call %d answers the sorted prefix", call)
			}
		})
	}
}
