// Design: docs/architecture/api/commands.md -- clear bgp rib advertised
// Related: reactor_api_advertised.go -- WithdrawAdvertised
// Overview: group_updates_framing_test.go -- the peer, reactor and frame-reading harness
//
// `clear bgp rib advertised` withdraws what the session's Adj-RIB-Out holds.
// These tests read the BYTES the peer's connection received, because the
// withdrawal reaching the wire is the behavior.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestWithdrawAdvertisedPutsTheWithdrawalOnTheWire announces two prefixes on
// the API rail, clears the peer's Adj-RIB-Out, and reads the UPDATE the clear
// wrote.
//
// VALIDATES: WithdrawAdvertised writes one UPDATE whose withdrawn routes field
// names both prefixes the session advertised, empties the Adj-RIB-Out, and a
// second clear writes nothing and withdraws nothing.
// PREVENTS: a clear that empties ze's tables without telling the peer, and a
// clear that withdraws routes the session no longer advertises.
func TestWithdrawAdvertisedPutsTheWithdrawalOnTheWire(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)
	require.NoError(t, adapter.AnnounceNLRIBatch(t.Context(), selector.All(), groupUpdatesBatch(), plugin.OperatorSender()))
	announced := len(updateBodies(t, conn.written()))
	require.Positive(t, announced, "the announce reached the wire")

	withdrawn, err := adapter.WithdrawAdvertised(t.Context(), selector.All(), 0, 0, plugin.OperatorSender())
	require.NoError(t, err)
	assert.Equal(t, len(groupUpdatesPrefixes), withdrawn, "every advertised prefix is withdrawn")

	bodies := updateBodies(t, conn.written())
	require.Len(t, bodies, announced+1, "the clear wrote one UPDATE")
	withdrawnField, attributes, reachable := updateSections(t, bodies[announced])
	assert.Empty(t, attributes, "an IPv4 unicast withdrawal carries no path attribute")
	assert.Empty(t, reachable, "the clear announces nothing")
	var want []byte
	for _, prefix := range groupUpdatesPrefixes {
		want = append(want, byte(prefix.Bits()))
		addr := prefix.Addr().As4()
		want = append(want, addr[:(prefix.Bits()+7)/8]...)
	}
	assert.ElementsMatch(t, splitIPv4Prefixes(t, want), splitIPv4Prefixes(t, withdrawnField),
		"the withdrawn routes field names exactly the advertised prefixes")
	left, err := peer.adjOut.withdrawalBatches(peer.currentSession(), family.Family{})
	require.NoError(t, err)
	assert.Empty(t, left, "the Adj-RIB-Out is empty after the clear")

	again, err := adapter.WithdrawAdvertised(t.Context(), selector.All(), 0, 0, plugin.OperatorSender())
	require.NoError(t, err)
	assert.Zero(t, again, "a second clear has nothing left to withdraw")
	assert.Len(t, updateBodies(t, conn.written()), announced+1, "a second clear writes nothing")
}

// splitIPv4Prefixes reads an RFC 4271 Section 4.3 withdrawn routes field as
// IPv4 prefixes, failing on a length that runs past the field.
func splitIPv4Prefixes(t *testing.T, field []byte) []netip.Prefix {
	t.Helper()
	var prefixes []netip.Prefix
	for off := 0; off < len(field); {
		bits := int(field[off])
		octets := (bits + 7) / 8
		require.LessOrEqual(t, off+1+octets, len(field), "a prefix runs past the field")
		var addr [4]byte
		copy(addr[:], field[off+1:off+1+octets])
		prefixes = append(prefixes, netip.PrefixFrom(netip.AddrFrom4(addr), bits))
		off += 1 + octets
	}
	return prefixes
}
