package bridge

import (
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestWireUpdateRendersTheFixtureItIsMeasuredAgainst proves the renderer against
// the one pair test/exabgp-compat states both halves of: api-api.ci carries a
// raw frame and, on the next line, the ExaBGP JSON that frame owes.
//
// VALIDATES: an UPDATE's wire body renders to the document the fixture expects,
// once the members no two runs agree on are dropped.
// PREVENTS: the 61 :json: expectations staying decoration. readExaBGPCase
// (internal/le/interoplab/bgp) keeps a step only when its second field is `raw`,
// so every one of them is read and discarded, and two defects in this file's own
// output reached a verification sweep because of it.
func TestWireUpdateRendersTheFixtureItIsMeasuredAgainst(t *testing.T) {
	// test/exabgp-compat/api/api-api.ci, the announce of 6.6.6.0/24: the body of
	// `1:raw:FFFF...:0030:02:` with its header removed.
	payload, err := hex.DecodeString("0000001540010100400200400304010101014005040000006418060606")
	require.NoError(t, err)

	got, err := WireUpdateToExabgpJSON(payload, SessionFacts{
		Local:    netip.MustParseAddr("127.0.0.1"),
		Peer:     netip.MustParseAddr("127.0.0.1"),
		LocalAS:  65535,
		PeerAS:   65535,
		RouterID: 0x7f00007b, // 127.0.0.123, as the fixture's config sets it
	}, rpc.DirectionReceived)
	require.NoError(t, err)
	DropVolatile(got)

	// The expectation the fixture writes, minus the same volatile members.
	const expected = `{"exabgp":"6.0.0","type":"update","neighbor":{"address":{"local":"127.0.0.1",` +
		`"peer":"127.0.0.1"},"asn":{"local":65535,"peer":65535},"router-id":"127.0.0.123",` +
		`"direction":"in","message":{"update":{"attribute":{"origin":"igp","local-preference":100},` +
		`"announce":{"ipv4 unicast":{"1.1.1.1":[{"nlri":"6.6.6.0/24"}]}}}}}}`
	var want map[string]any
	require.NoError(t, json.Unmarshal([]byte(expected), &want))
	DropVolatile(want)

	gotJSON, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON),
		"the rendered frame does not match the expectation the fixture states for it")
}

// TestASPathKeepsItsSegments pins the member ze's own JSON cannot carry.
//
// VALIDATES: an AS_PATH renders as ExaBGP states it, keyed by segment index
// with each segment's element type named.
// PREVENTS: the information loss that made this necessary. appendASPathJSON
// (internal/core/bgp/attribute/json.go) walks every segment and appends every
// ASN, discarding seg.Type and the boundaries, so `[65533]` cannot say whether
// the path was an AS_SEQUENCE or an AS_SET -- and RFC 4271 Section 4.3 makes a
// set UNORDERED, which is a different statement about the path.
func TestASPathKeepsItsSegments(t *testing.T) {
	// ORIGIN igp, AS_PATH (one as-sequence holding 65533), NEXT_HOP 1.1.1.1,
	// NLRI 10.0.0.0/24. Attribute length 0x18 covers the four attributes.
	payload, err := hex.DecodeString("0000001840010100" + "40020602010000FFFD" + "40030401010101" + "180A0000")
	require.NoError(t, err)

	got, err := WireUpdateToExabgpJSON(payload, SessionFacts{
		Local:   netip.MustParseAddr("127.0.0.1"),
		Peer:    netip.MustParseAddr("127.0.0.1"),
		LocalAS: 65535, PeerAS: 65535,
	}, rpc.DirectionReceived)
	require.NoError(t, err)

	update := got["neighbor"].(map[string]any)["message"].(map[string]any)["update"].(map[string]any)
	attributes, ok := update["attribute"].(map[string]any)
	require.True(t, ok, "the update states no attributes")

	segments, ok := attributes["as-path"].(map[string]any)
	require.True(t, ok, "as-path is not the segment map ExaBGP states, it is %T", attributes["as-path"])
	first, ok := segments["0"].(map[string]any)
	require.True(t, ok, "no segment 0 in %v", segments)
	assert.Equal(t, "as-sequence", first["element"],
		"the segment type is what tells an ordered path from an unordered set")
	assert.Equal(t, []any{float64(65533)}, first["value"])
}
