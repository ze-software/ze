package bgp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestParseEvent_ZeBGPUpdateFormat verifies parsing of ze-bgp JSON update events.
// The ze-bgp format nests event data under {"type":"bgp","bgp":{...,"update":{...}}}.
//
// VALIDATES: ParseEvent extracts peer, message, family ops from ze-bgp nested format.
// PREVENTS: Breaking shared event parsing after extraction from bgp-rib.
func TestParseEvent_ZeBGPUpdateFormat(t *testing.T) {
	input := `{"type":"bgp","bgp":{
		"peer":{"address":"10.0.0.1","local":{"address":"10.0.0.2","as":65002},"remote":{"as":65001}},
		"message":{"type":"update","id":42,"direction":"received"},
		"update":{
			"attributes":{"origin":"igp","as-path":[65001]},
			"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}
		}
	}}`

	event, err := ParseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "update", event.GetEventType())
	assert.Equal(t, uint64(42), event.GetMsgID())
	assert.Equal(t, "received", event.GetDirection())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.Equal(t, uint32(65001), event.GetPeerASN())
	assert.Equal(t, "igp", event.Origin)
	assert.Equal(t, []uint32{65001}, event.ASPath)

	require.Contains(t, event.FamilyOps, family.IPv4Unicast)
	ops := event.FamilyOps[family.IPv4Unicast]
	require.Len(t, ops, 1)
	assert.Equal(t, "add", ops[0].Action)
	assert.Equal(t, "10.0.0.1", ops[0].NextHop)
	require.Len(t, ops[0].NLRIs, 1)
	assert.Equal(t, "10.0.0.0/24", ops[0].NLRIs[0])
}

// TestParseEvent_StateFormat verifies parsing of peer state events.
//
// VALIDATES: ParseEvent extracts peer state from flat format.
// PREVENTS: State events silently ignored after extraction.
func TestParseEvent_StateFormat(t *testing.T) {
	input := `{"type":"state","peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"up"}`

	event, err := ParseEvent([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "state", event.GetEventType())
	assert.Equal(t, "10.0.0.1", event.GetPeerAddress())
	assert.Equal(t, "up", event.GetPeerState())
}

// TestParseEvent_FormatFullRawFields verifies raw hex fields from format=full events.
// Tests both possible positions: raw inside update (legacy) and raw at bgp level
// (actual format produced by formatFullFromResult).
//
// VALIDATES: ParseEvent extracts raw.attributes, raw.nlri, raw.withdrawn.
// PREVENTS: Raw hex bytes lost after extraction — adj-rib-in depends on these.
func TestParseEvent_FormatFullRawFields(t *testing.T) {
	// Raw inside update object (legacy test — kept for backwards compatibility).
	t.Run("raw inside update", func(t *testing.T) {
		input := `{"type":"bgp","bgp":{
			"peer":{"address":"10.0.0.1","local":{"address":"10.0.0.2","as":65002},"remote":{"as":65001}},
			"message":{"type":"update","id":1,"direction":"received"},
			"update":{
				"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]},
				"raw":{"attributes":"40010100","nlri":{"ipv4/unicast":"180a0000"},"withdrawn":{"ipv4/unicast":"180a0100"}}
			}
		}}`

		event, err := ParseEvent([]byte(input))
		require.NoError(t, err)

		assert.Equal(t, "40010100", event.RawAttributes)
		require.Contains(t, event.RawNLRI, family.IPv4Unicast)
		assert.Equal(t, "180a0000", event.RawNLRI[family.IPv4Unicast])
		require.Contains(t, event.RawWithdrawn, family.IPv4Unicast)
		assert.Equal(t, "180a0100", event.RawWithdrawn[family.IPv4Unicast])

		assert.Equal(t, []byte{0x40, 0x01, 0x01, 0x00}, event.GetRawAttributesBytes())
		assert.Equal(t, []byte{0x18, 0x0a, 0x00, 0x00}, event.GetRawNLRIBytes(family.IPv4Unicast))
		assert.Nil(t, event.GetRawNLRIBytes(family.IPv6Unicast), "missing family returns nil")
	})

	// Raw at bgp level (sibling of update) — actual format from formatFullFromResult.
	// formatFullFromResult injects raw at the bgp level, not inside update.
	// ParseEvent must extract it BEFORE narrowing payloadData to the update object.
	t.Run("raw at bgp level", func(t *testing.T) {
		input := `{"type":"bgp","bgp":{
			"peer":{"address":"10.0.0.1","local":{"address":"10.0.0.2","as":65002},"remote":{"as":65001}},
			"message":{"type":"update","id":1,"direction":"received"},
			"update":{
				"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}
			},
			"raw":{"attributes":"40010100","nlri":{"ipv4/unicast":"180a0000"},"withdrawn":{"ipv4/unicast":"180a0100"}}
		}}`

		event, err := ParseEvent([]byte(input))
		require.NoError(t, err)

		assert.Equal(t, "40010100", event.RawAttributes)
		require.Contains(t, event.RawNLRI, family.IPv4Unicast)
		assert.Equal(t, "180a0000", event.RawNLRI[family.IPv4Unicast])
		require.Contains(t, event.RawWithdrawn, family.IPv4Unicast)
		assert.Equal(t, "180a0100", event.RawWithdrawn[family.IPv4Unicast])

		assert.Equal(t, []byte{0x40, 0x01, 0x01, 0x00}, event.GetRawAttributesBytes())
		assert.Equal(t, []byte{0x18, 0x0a, 0x00, 0x00}, event.GetRawNLRIBytes(family.IPv4Unicast))
		assert.Nil(t, event.GetRawNLRIBytes(family.IPv6Unicast), "missing family returns nil")
	})
}

// TestParseEvent_AddPathField verifies ADD-PATH per-family flags from format=full events.
//
// VALIDATES: ParseEvent extracts raw.add-path map into Event.AddPath.
// PREVENTS: RIB plugin unable to determine ADD-PATH state from event JSON.
func TestParseEvent_AddPathField(t *testing.T) {
	t.Run("add_path_present", func(t *testing.T) {
		input := `{"type":"bgp","bgp":{
			"peer":{"address":"10.0.0.1","local":{"address":"10.0.0.2","as":65002},"remote":{"as":65001}},
			"message":{"type":"update","id":1,"direction":"received"},
			"update":{
				"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}
			},
			"raw":{"attributes":"40010100","nlri":{"ipv4/unicast":"180a0000"},"add-path":{"ipv4/unicast":true}}
		}}`

		event, err := ParseEvent([]byte(input))
		require.NoError(t, err)

		require.NotNil(t, event.AddPath, "AddPath map should be populated")
		assert.True(t, event.AddPath[family.IPv4Unicast], "IPv4 unicast should be true")
		_, hasIPv6 := event.AddPath[family.IPv6Unicast]
		assert.False(t, hasIPv6, "IPv6 unicast should be absent")
	})

	t.Run("add_path_absent", func(t *testing.T) {
		input := `{"type":"bgp","bgp":{
			"peer":{"address":"10.0.0.1","local":{"address":"10.0.0.2","as":65002},"remote":{"as":65001}},
			"message":{"type":"update","id":1,"direction":"received"},
			"update":{
				"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}
			},
			"raw":{"attributes":"40010100","nlri":{"ipv4/unicast":"180a0000"}}
		}}`

		event, err := ParseEvent([]byte(input))
		require.NoError(t, err)

		assert.Empty(t, event.AddPath, "AddPath map should be nil/empty when no add-path in JSON")
	})
}

// TestParseEvent_PeerFormats verifies both flat and nested peer formats.
//
// VALIDATES: GetPeerAddress/GetPeerASN work for both sent (flat) and received (nested) events.
// PREVENTS: Peer info extraction failing for one format after extraction.
func TestParseEvent_PeerFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
		addr  string
		asn   uint32
	}{
		{
			name:  "flat format (sent/state events)",
			input: `{"type":"state","peer":{"address":"192.168.1.1","remote":{"as":64512}},"state":"down"}`,
			addr:  "192.168.1.1",
			asn:   64512,
		},
		{
			name: "nested format (received events)",
			input: `{"type":"bgp","bgp":{
				"peer":{"address":"192.168.1.1","local":{"address":"10.0.0.2","as":65002},"remote":{"as":64512}},
				"message":{"type":"update","id":1,"direction":"received"},
				"update":{"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}}
			}}`,
			addr: "192.168.1.1",
			asn:  64512,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseEvent([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.addr, event.GetPeerAddress())
			assert.Equal(t, tt.asn, event.GetPeerASN())
		})
	}
}

// TestParseEvent_MultipleFamilies verifies multi-family UPDATE parsing.
//
// VALIDATES: Multiple address families extracted from single event.
// PREVENTS: Only first family parsed, others silently dropped.
func TestParseEvent_MultipleFamilies(t *testing.T) {
	input := `{"type":"bgp","bgp":{
		"peer":{"address":"10.0.0.1","remote":{"as":65001}},
		"message":{"type":"update","id":1,"direction":"received"},
		"update":{
			"nlri":{
				"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}],
				"ipv6/unicast":[{"next-hop":"::1","action":"add","nlri":["2001:db8::/32"]}]
			}
		}
	}}`

	event, err := ParseEvent([]byte(input))
	require.NoError(t, err)

	require.Len(t, event.FamilyOps, 2)
	assert.Contains(t, event.FamilyOps, family.IPv4Unicast)
	assert.Contains(t, event.FamilyOps, family.IPv6Unicast)
}

// TestParseEvent_InvalidJSON verifies error on malformed input.
//
// VALIDATES: ParseEvent returns error for invalid JSON.
// PREVENTS: Panic on malformed input.
func TestParseEvent_InvalidJSON(t *testing.T) {
	_, err := ParseEvent([]byte(`{not json`))
	require.Error(t, err)
}

// TestParseNLRIValue verifies NLRI value extraction from both formats.
//
// VALIDATES: ParseNLRIValue handles string and structured NLRI formats.
// PREVENTS: Path-ID silently lost from structured format.
func TestParseNLRIValue(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		wantPrefix string
		wantPathID uint32
	}{
		{"string format", "10.0.0.0/24", "10.0.0.0/24", 0},
		{"structured with path-id", map[string]any{"prefix": "10.0.0.0/24", "path-id": float64(42)}, "10.0.0.0/24", 42},
		{"structured without path-id", map[string]any{"prefix": "10.0.0.0/24"}, "10.0.0.0/24", 0},
		{"nil input", nil, "", 0},
		{"integer input", 42, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, pathID := ParseNLRIValue(tt.input)
			assert.Equal(t, tt.wantPrefix, prefix)
			assert.Equal(t, tt.wantPathID, pathID)
		})
	}
}

// TestRouteKey verifies unique key generation for routes.
//
// VALIDATES: RouteKey produces unique keys; path-id creates distinct entries.
// PREVENTS: Route collisions between same prefix with different path-ids.
func TestRouteKey(t *testing.T) {
	tests := []struct {
		name   string
		family string
		prefix string
		pathID uint32
		want   string
	}{
		{"no path-id", "ipv4/unicast", "10.0.0.0/24", 0, "ipv4/unicast:10.0.0.0/24"},
		{"with path-id", "ipv4/unicast", "10.0.0.0/24", 42, "ipv4/unicast:10.0.0.0/24:42"},
		{"ipv6", "ipv6/unicast", "2001:db8::/32", 0, "ipv6/unicast:2001:db8::/32"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RouteKey(tt.family, tt.prefix, tt.pathID)
			assert.Equal(t, tt.want, got)
		})
	}
}
