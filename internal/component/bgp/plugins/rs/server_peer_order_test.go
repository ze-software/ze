// Design: docs/architecture/core-design.md — peer event handlers for route server
// Related: server_handlers.go — peerStatus, the producer under test
package rs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// peerRows answers the "address" field of each row of a peerStatus answer, in
// the order the answer carries them.
//
// The row order is the whole subject here, so the addresses are read out in
// sequence rather than collected into a set.
func peerRows(t *testing.T, answer any) []string {
	t.Helper()

	envelope, isEnvelope := answer.(map[string]any)
	require.True(t, isEnvelope, "peerStatus answers an envelope")
	rows, isRows := envelope["peers"].([]map[string]any)
	require.True(t, isRows, "the envelope carries its rows under \"peers\"")

	addresses := make([]string, 0, len(rows))
	for _, row := range rows {
		address, isText := row["address"].(string)
		require.True(t, isText, "each row carries an address")
		addresses = append(addresses, address)
	}
	return addresses
}

// TestPeerStatusAnswersSortedOrder proves that peerStatus answers one
// deterministic order, and that the order is the sorted one.
//
// Goal: `show bgp rs peers` declares a row shape, and a declared row shape
// publishes "first", "last" and "display" as supported, so each of them must
// select the same peer on every run. Method: register eight peers in an order
// that is neither the sorted one nor the text order of their addresses, call
// peerStatus many times, and require ascending address order on every call.
//
// VALIDATES: peerStatus answers in netip.Addr order.
// PREVENTS: Go map iteration randomization reaching the CLI, where `| first 1`
// answers a different peer on each call and nothing in the answer says so.
func TestPeerStatusAnswersSortedOrder(t *testing.T) {
	rs := newTestRouteServer(t)

	// The insertion order, the lexical order of the address strings, and the
	// address order all disagree here, so an implementation answering either of
	// the other two fails. "192.0.2.10" sorts before "192.0.2.2" as text and
	// after it as an address, and netip.Addr.Compare orders by bit length first,
	// so every IPv4 peer precedes every IPv6 one where the text order puts
	// "2001:db8::1" between "198.51.100.1" and "203.0.113.5".
	inserted := []string{
		"198.51.100.1", "192.0.2.10", "2001:db8::1", "192.0.2.2",
		"203.0.113.5", "10.0.0.1", "2001:db8::2", "192.0.2.9",
	}
	expected := []string{
		"10.0.0.1", "192.0.2.2", "192.0.2.9", "192.0.2.10",
		"198.51.100.1", "203.0.113.5", "2001:db8::1", "2001:db8::2",
	}

	for _, address := range inserted {
		rs.peers[address] = &PeerState{Address: address, ASN: 65000, Up: true}
	}
	require.Len(t, rs.peers, len(inserted), "every peer address is distinct")

	// Go randomizes where a map range starts, so with eight peers a single call
	// answers the sorted order by luck with probability at most 1/8. Thirty-two
	// calls put that ceiling at 8^-32, near 5e-29, so a map-ranging
	// implementation fails this test rather than flaking it. Asserting the whole
	// sequence rather than its stability is what makes insertion order fail too.
	const callCount = 32

	for call := range callCount {
		require.Equal(t, expected, peerRows(t, rs.peerStatus()),
			"call %d answers ascending address order", call)
	}
}

// TestComparePeerAddressOrdersAnUnparseableKeyLast proves the sort key is total
// for whatever the engine sent.
//
// Goal: a key arrives from the engine as a string, so peerStatus cannot assume
// it parses as an address. A comparison answering 0 for two unparseable keys
// would leave them in the map's own order, which is the defect this file exists
// to close. Method: compare the three pairs an unparseable key can form.
//
// VALIDATES: comparePeerAddress is a total order over arbitrary strings.
// PREVENTS: one malformed peer address reintroducing a random row order for the
// whole answer.
func TestComparePeerAddressOrdersAnUnparseableKeyLast(t *testing.T) {
	require.Negative(t, comparePeerAddress("203.0.113.5", "not-an-address"),
		"an address sorts before a key that does not parse")
	require.Positive(t, comparePeerAddress("not-an-address", "203.0.113.5"),
		"a key that does not parse sorts after an address")
	require.Negative(t, comparePeerAddress("alpha", "beta"),
		"two keys that do not parse are ordered by their text")
	require.Zero(t, comparePeerAddress("192.0.2.1", "192.0.2.1"),
		"one address equals itself")
}
