// Design: docs/architecture/exabgp-bridge.md -- Q-E3 servedSelector narrowing and Q-E4 wire invariants.
// Related: bridge_negotiated.go -- Sessions keyed by (peer, connection-id) and asn4 derivation.
// Related: bridge_command.go -- Translator.servedSelector and Translator.NoPeers.

package bridge

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateASN4DecodesASPathWithoutNegotiated verifies that a `bgp:update`
// event carrying `"asn4":true` or `"asn4":false` decodes AS_PATH segments from
// raw bytes even when the session never received `bgp:negotiated` (Q-E4).
//
// VALIDATES: Q-E4 wire invariant (1) — 4-byte and 2-byte AS_PATH wire decoding
// works directly from `bgp:update`'s `asn4` field without a hidden
// `bgp:negotiated` subscription.
// PREVENTS: lossy flattened AS_PATH or corrupted 2-byte/4-byte ASN decoding
// when `receive > allow` excludes `bgp:negotiated`.
func TestUpdateASN4DecodesASPathWithoutNegotiated(t *testing.T) {
	t.Parallel()

	t.Run("asn4-true-without-negotiated", func(t *testing.T) {
		t.Parallel()
		raw := wireUpdateEvent()
		raw["bgp"].(map[string]any)["asn4"] = true

		var sessions Sessions
		ev := ReadEvent(raw)
		require.True(t, ev.HasASN4)
		require.True(t, ev.ASN4)

		sessions.Note(&ev)
		attrs := wireUpdateAttributes(t, &ev)
		wantPath := map[string]any{
			"0": map[string]any{"element": "as-sequence", "value": []any{float64(65001)}},
		}
		assert.Equal(t, wantPath, attrs["as-path"])
	})

	t.Run("asn4-false-2byte-aspath-without-negotiated", func(t *testing.T) {
		t.Parallel()
		// Withdrawn=0 (0000), PathAttrLen=12 (000C):
		// ORIGIN IGP: 40 01 01 00 (4 bytes)
		// AS_PATH 2-byte AS_SEQUENCE [65001, 65002]: 40 02 06 02 02 FDE9 FDEA (9 bytes -> total 13 = 0x000D)
		raw := map[string]any{
			"type": "bgp",
			"bgp": map[string]any{
				"message": map[string]any{"type": "update", "direction": "received"},
				"peer": map[string]any{
					"remote": map[string]any{"address": "10.0.0.1", "as": float64(65001)},
					"local":  map[string]any{"address": "10.0.0.2", "as": float64(65002)},
				},
				"asn4": false,
				"update": map[string]any{
					"attr": map[string]any{
						"origin":  "igp",
						"as-path": []any{float64(65001), float64(65002)},
					},
				},
				"raw": map[string]any{
					"update": "0000" + "000D" + "40010100" + "4002060202FDE9FDEA",
				},
			},
		}

		var sessions Sessions
		ev := ReadEvent(raw)
		require.True(t, ev.HasASN4)
		require.False(t, ev.ASN4)

		sessions.Note(&ev)
		attrs := wireUpdateAttributes(t, &ev)
		wantPath := map[string]any{
			"0": map[string]any{"element": "as-sequence", "value": []any{float64(65001), float64(65002)}},
		}
		assert.Equal(t, wantPath, attrs["as-path"])
	})
}

// TestSessionsInvalidateNegotiatedOnReconnectWithoutState verifies that
// Sessions keys each peer's cached negotiated state by (peer, connection-id) so
// a reconnected session never reuses a stale negotiated block or stale ASN4
// context even when `bgp:state` was excluded from `receive > allow` (Q-E4).
//
// VALIDATES: Q-E4 wire invariant (2) — connection-id change invalidates cached
// negotiated block and encoding context immediately without bgp:state down.
// PREVENTS: a reconnected session carrying the previous session's negotiated
// parameters or decoding AS_PATH with the previous session's ASN width.
func TestSessionsInvalidateNegotiatedOnReconnectWithoutState(t *testing.T) {
	t.Parallel()

	peerObj := map[string]any{
		"remote": map[string]any{"address": "192.0.2.1", "as": float64(65001)},
		"local":  map[string]any{"address": "192.0.2.2", "as": float64(65002)},
	}

	var sessions Sessions
	neg1 := ReadEvent(map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message":       map[string]any{"type": "negotiated"},
			"peer":          peerObj,
			"connection-id": "101",
			"negotiated": map[string]any{
				"timer":         map[string]any{"hold-time": float64(90)},
				"message-size":  float64(4096),
				"route-refresh": "normal",
				"asn4":          true,
				"families":      []any{"ipv4/unicast"},
			},
		},
	})
	require.Equal(t, "101", neg1.ConnectionID)
	sessions.Note(&neg1)

	// Same connection-id keeps the cached block.
	upd1 := ReadEvent(map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message":       map[string]any{"type": "update", "direction": "received"},
			"peer":          peerObj,
			"connection-id": "101",
			"asn4":          true,
			"update":        map[string]any{"attr": map[string]any{"origin": "igp"}},
		},
	})
	sessions.Note(&upd1)
	require.NotNil(t, upd1.Negotiated)
	assert.Equal(t, float64(90), upd1.Negotiated["hold_time"])

	// Reconnect on connection-id 202 with NO bgp:state down event and 2-byte ASN4=false.
	upd2 := ReadEvent(map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message":       map[string]any{"type": "update", "direction": "received"},
			"peer":          peerObj,
			"connection-id": "202",
			"asn4":          false,
			"update": map[string]any{
				"attr": map[string]any{
					"origin":  "igp",
					"as-path": []any{float64(65001), float64(65002)},
				},
			},
			"raw": map[string]any{
				"update": "0000" + "000D" + "40010100" + "4002060202FDE9FDEA",
			},
		},
	})
	sessions.Note(&upd2)
	assert.Nil(t, upd2.Negotiated, "reconnect must invalidate stale negotiated block from connection 101")

	// ForGrant(true) falls back to unexchangedNegotiated (hold_time=0), never stale 90.
	granted := upd2.ForGrant(true)
	assert.Equal(t, float64(0), granted.Negotiated["hold_time"])

	// AS_PATH still decodes accurately using upd2's own asn4=false (2-byte width).
	attrs := wireUpdateAttributes(t, &upd2)
	wantPath := map[string]any{
		"0": map[string]any{"element": "as-sequence", "value": []any{float64(65001), float64(65002)}},
	}
	if !reflect.DeepEqual(attrs["as-path"], wantPath) {
		t.Errorf("reconnected 2-byte as-path = %#v, want %#v", attrs["as-path"], wantPath)
	}

	// New negotiated for connection-id 202 populates cache for subsequent events.
	neg2 := ReadEvent(map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message":       map[string]any{"type": "negotiated"},
			"peer":          peerObj,
			"connection-id": "202",
			"negotiated": map[string]any{
				"timer":         map[string]any{"hold-time": float64(120)},
				"message-size":  float64(4096),
				"route-refresh": "enhanced",
				"asn4":          false,
				"families":      []any{"ipv4/unicast"},
			},
		},
	})
	sessions.Note(&neg2)

	upd3 := ReadEvent(map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message":       map[string]any{"type": "update", "direction": "received"},
			"peer":          peerObj,
			"connection-id": "202",
			"update":        map[string]any{"attr": map[string]any{"origin": "igp"}},
		},
	})
	sessions.Note(&upd3)
	require.NotNil(t, upd3.Negotiated)
	assert.Equal(t, float64(120), upd3.Negotiated["hold_time"])
}

// TestServedSelectorNarrowsWildcardAndHonorsNoPeers verifies Q-E3: when
// Translator.Peers is non-empty, an explicit wildcard (`peer *` / `neighbor *`)
// narrows to the script's scoped peers rather than passing `*` through; and
// when Translator.NoPeers is true (`send { to none; }`), every peer-scoped
// command is rejected while peer-independent local controls remain usable.
//
// VALIDATES: Q-E3 `servedSelector` narrowing and `Translator.NoPeers`.
// PREVENTS: explicit `*` triggering per-peer WARN refusals in
// filterPermittedPeers, or `send { to none; }` falling back to unscoped `*`.
func TestServedSelectorNarrowsWildcardAndHonorsNoPeers(t *testing.T) {
	t.Parallel()

	t.Run("scoped-translator-narrows-explicit-wildcard", func(t *testing.T) {
		t.Parallel()
		tr := Translator{Peers: []string{"192.0.2.1", "192.0.2.2"}}

		for _, line := range []string{
			"announce route 10.0.0.0/24 next-hop 1.1.1.1",
			"peer * announce route 10.0.0.0/24 next-hop 1.1.1.1",
			"neighbor * announce route 10.0.0.0/24 next-hop 1.1.1.1",
			"peer [192.0.2.99, *] announce route 10.0.0.0/24 next-hop 1.1.1.1",
		} {
			got, err := tr.Line(line)
			require.NoError(t, err, line)
			require.Len(t, got.Commands, 1, line)
			assert.Equal(t, "192.0.2.1,192.0.2.2", got.Selector, line)
			assert.Equal(t, "send bgp 192.0.2.1,192.0.2.2 update text nhop 1.1.1.1 nlri add 10.0.0.0/24",
				got.Commands[0].Text, line)
		}

		// Specific permitted peer narrows to that peer alone.
		single, err := tr.Line("peer 192.0.2.1 announce route 10.0.0.0/24 next-hop 1.1.1.1")
		require.NoError(t, err)
		assert.Equal(t, "192.0.2.1", single.Selector)

		// Unpermitted peer is still rejected.
		_, err = tr.Line("peer 192.0.2.99 announce route 10.0.0.0/24 next-hop 1.1.1.1")
		require.ErrorIs(t, err, errSelectorMatchesNoPeer)
	})

	t.Run("no-peers-rejects-peer-commands-but-keeps-local-controls", func(t *testing.T) {
		t.Parallel()
		tr := Translator{NoPeers: true}

		for _, line := range []string{
			"announce route 10.0.0.0/24 next-hop 1.1.1.1",
			"peer * announce route 10.0.0.0/24 next-hop 1.1.1.1",
			"neighbor * announce route 10.0.0.0/24 next-hop 1.1.1.1",
			"neighbor 192.0.2.1 announce route 10.0.0.0/24 next-hop 1.1.1.1",
			"clear adj-rib out",
		} {
			got, err := tr.Line(line)
			require.ErrorIs(t, err, errSelectorMatchesNoPeer, line)
			assert.Empty(t, got.Commands, line)
		}

		// Peer-independent local controls stay usable on a receive-only endpoint.
		for _, tc := range []struct {
			line string
			want LocalAction
		}{
			{"enable-ack", LocalAckEnable},
			{"disable-ack", LocalAckDisableAfter},
			{"silence-ack", LocalAckDisableNow},
			{"ping", LocalPing},
			{"status", LocalStatus},
			{"api version", LocalAPIVersion},
		} {
			got, err := tr.Line(tc.line)
			require.NoError(t, err, tc.line)
			assert.Equal(t, tc.want, got.Local, tc.line)
		}
	})

	t.Run("unscoped-translator-keeps-wildcard", func(t *testing.T) {
		t.Parallel()
		tr := Translator{}
		got, err := tr.Line("peer * announce route 10.0.0.0/24 next-hop 1.1.1.1")
		require.NoError(t, err)
		assert.Equal(t, "*", got.Selector)
		assert.Equal(t, "send bgp * update text nhop 1.1.1.1 nlri add 10.0.0.0/24", got.Commands[0].Text)
	})
}
