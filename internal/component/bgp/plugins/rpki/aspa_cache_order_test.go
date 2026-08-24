// Design: docs/architecture/plugin/rib-storage-design.md -- ASPA record cache
// Related: aspa_cache.go -- aSPACache.Entries and lookupCustomer, the producers under test
package rpki

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// seedOrderedASPACache fills a cache with five ASPA records whose insertion
// order is not the customer-AS order, and whose provider sets are each stored
// unsorted. 65535 and 4200000000 sort one way as numbers and the other way as
// text, so an implementation that orders either set as strings fails.
func seedOrderedASPACache() *aSPACache {
	c := newASPACache()
	c.Set(65010, []uint32{65100, 100})
	c.Set(100, []uint32{65535, 3, 4200000000, 12, 64500})
	c.Set(4200000000, []uint32{1})
	c.Set(65002, []uint32{65003, 65001, 65002})
	c.Set(65535, []uint32{7})
	return c
}

// orderedASPARows is the one order seedOrderedASPACache must answer: records by
// ascending customer AS, each provider set ascending.
var orderedASPARows = []ASPADiagEntry{
	{CustomerAS: 100, Providers: []uint32{3, 12, 64500, 65535, 4200000000}},
	{CustomerAS: 65002, Providers: []uint32{65001, 65002, 65003}},
	{CustomerAS: 65010, Providers: []uint32{100, 65100}},
	{CustomerAS: 65535, Providers: []uint32{7}},
	{CustomerAS: 4200000000, Providers: []uint32{1}},
}

// Go randomizes where a map range starts. The seeded cache holds five customer
// records, and the customer whose providers the lookup test reads holds five
// providers, so a map-ranging implementation answers the sorted order by luck
// with probability at most 1/5 per call. Thirty-two calls put that ceiling at
// 5^-32, near 4e-23, so such an implementation fails rather than flakes.
const aspaOrderCallCount = 32

// TestASPAEntriesAnswersSortedOrder proves that Entries answers one order.
//
// Goal: the row operators that `show bgp rpki aspa` publishes ("first", "last",
// "display") must select the same record on every run, and the provider list
// inside a row must not move either. Method: seed five records in an order that
// is not sorted, call Entries many times, and require the full row sequence on
// every call.
//
// VALIDATES: aSPACache.Entries answers records in ascending customer-AS order,
// each with its providers ascending.
// PREVENTS: Go map iteration randomization reaching the CLI, where one unchanged
// cache answers a different record order, and a different provider order inside
// a record, on each call.
func TestASPAEntriesAnswersSortedOrder(t *testing.T) {
	c := seedOrderedASPACache()
	require.Equal(t, len(orderedASPARows), c.count(), "every seeded record is stored")

	for call := range aspaOrderCallCount {
		require.Equal(t, orderedASPARows, c.Entries(0), "call %d answers sorted order", call)
	}
}

// TestASPAEntriesLimitAnswersDeterministicPrefix proves that a truncated answer
// is the same records, in the same order, on every call.
//
// Goal: `show bgp rpki aspa` answers "truncated":true beside the FIRST limit
// records rather than beside a random sample of them. Method: seed more records
// than the limit and require the sorted prefix of the row sequence.
//
// VALIDATES: aSPACache.Entries(limit) answers the first limit records of its
// sorted order.
// PREVENTS: a map range stopping at the limit, which answers a random SUBSET of
// the records and says nothing about which ones it dropped.
func TestASPAEntriesLimitAnswersDeterministicPrefix(t *testing.T) {
	c := seedOrderedASPACache()

	cases := []struct {
		name  string
		limit int
		rows  int
	}{
		{"first record only", 1, 1},
		{"inside the record set", 3, 3},
		{"limit above the record count", 99, len(orderedASPARows)},
		{"no limit", 0, len(orderedASPARows)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for call := range aspaOrderCallCount {
				got := c.Entries(tc.limit)
				require.Equal(t, orderedASPARows[:tc.rows], got, "call %d answers the sorted prefix", call)
			}
		})
	}
}

// TestASPALookupCustomerAnswersSortedProviders proves that the one-customer
// branch answers its providers in one order.
//
// Goal: `show bgp rpki aspa <customer-asn>` writes the provider list into the
// same row shape as the no-argument branch (rpki.go, aspaCommand), so it owes
// the same order. Method: read a customer holding five providers many times and
// require the ascending order on every call.
//
// VALIDATES: aSPACache.lookupCustomer answers providers in ascending order.
// PREVENTS: Go map iteration randomization reaching the CLI, where one unchanged
// record answers a different provider order on each call.
func TestASPALookupCustomerAnswersSortedProviders(t *testing.T) {
	c := seedOrderedASPACache()

	for call := range aspaOrderCallCount {
		require.Equal(t, orderedASPARows[0].Providers, c.lookupCustomer(100),
			"call %d answers sorted providers", call)
	}
}

// TestASPALookupCustomerSeparatesEmptyFromAbsent proves that sorting the
// providers did not turn an empty provider set into a missing record.
//
// Goal: aspaCommand reads a nil answer as "found":false (rpki.go), so a record
// whose provider set is empty must still answer a non-nil slice. Method: store a
// record with no providers and read it back beside a customer that has none.
//
// VALIDATES: lookupCustomer answers non-nil for a stored record and nil for an
// absent one.
// PREVENTS: `show bgp rpki aspa <asn>` reporting "found":false for a customer
// whose ASPA record authorizes no provider.
func TestASPALookupCustomerSeparatesEmptyFromAbsent(t *testing.T) {
	c := newASPACache()
	c.Set(64500, nil)

	providers := c.lookupCustomer(64500)
	require.NotNil(t, providers, "a stored record answers a non-nil provider slice")
	require.Empty(t, providers, "the stored record authorizes no provider")
	require.Nil(t, c.lookupCustomer(64501), "an absent record answers nil")
}
