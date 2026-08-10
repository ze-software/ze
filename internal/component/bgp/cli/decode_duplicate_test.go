package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// dupOriginUpdateHex is a BGP UPDATE with TWO ORIGIN attributes:
//
//	header: FF*16, length 0x0031 (49), type 0x02 (UPDATE)
//	body: withdrawn-len 0x0000, attr-len 0x0016 (22)
//	  attrs: ORIGIN=IGP(0x00), AS_PATH=[65001], NEXT_HOP=192.0.2.1, ORIGIN=EGP(0x01, dup)
//	  NLRI: 192.0.2.0/24
//
// The two ORIGINs are individually valid; the second is a keep-first duplicate.
const dupOriginUpdateHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0031020000001640010100" +
	"4002040201FDE9" + "400304C0000201" + "40010101" + "18C00002"

// TestDecodeUpdateDuplicateOriginKeepFirst pins RFC 7606 Section 3.g keep-first for the
// offline `ze bgp decode` diagnostic path (D-4b): a duplicated ORIGIN is decoded once,
// and the surviving value is the FIRST occurrence (IGP), matching what an established
// session keeps via enforceRFC7606. Before this fix decode was last-write-wins on the
// attribute map, so `ze bgp decode` disagreed with on-session behavior.
//
// VALIDATES: decode keeps the first ORIGIN (igp) for a duplicate-ORIGIN UPDATE.
// PREVENTS: decode diagnostics diverging from the session's keep-first policy.
func TestDecodeUpdateDuplicateOriginKeepFirst(t *testing.T) {
	out, err := decodeHexPacket(dupOriginUpdateHex, msgTypeUpdate, "", true)
	require.NoError(t, err, "a duplicate-but-valid ORIGIN must decode, not error")

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))

	bgp, ok := env["bgp"].(map[string]any)
	require.True(t, ok, "bgp envelope")
	update, ok := bgp["update"].(map[string]any)
	require.True(t, ok, "update section")
	attr, ok := update["attr"].(map[string]any)
	require.True(t, ok, "attr section")
	origin, ok := attr["origin"].(map[string]any)
	require.True(t, ok, "origin attribute")
	require.Equal(t, "igp", origin["value"], "keep-first: the FIRST ORIGIN (IGP) wins, not the EGP duplicate")

	// Log the exact JSON so the test/decode/*.ci expectation can be pinned to reality.
	t.Logf("decode JSON: %s", out)
}
