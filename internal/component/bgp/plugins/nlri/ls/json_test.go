package ls

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AppendJSON on each BGP-LS NLRI type emits the same structured
// JSON shape that the RPC decode path produces (ls-nlri-type, protocol-id,
// l3-routing-topology, type-specific fields).
// PREVENTS: BGP-LS falling through formatNLRIJSON and emitting the
// String()-as-prefix placeholder instead of structured topology data.
func TestBGPLSAppendJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		nlri        BGPLSNLRI
		wantType    string
		wantFields  []string // keys that must appear in the parsed object
		extraChecks func(t *testing.T, parsed map[string]any)
	}{
		{
			name: "node",
			nlri: NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
				ASN:             65001,
				BGPLSIdentifier: 0x12345678,
				IGPRouterID:     []byte{1, 1, 1, 1},
			}),
			wantType:   "bgpls-node",
			wantFields: []string{"ls-nlri-type", "protocol-id", "l3-routing-topology", "node-descriptors"},
			extraChecks: func(t *testing.T, p map[string]any) {
				descs, ok := p["node-descriptors"].([]any)
				require.True(t, ok, "node-descriptors must be array")
				assert.NotEmpty(t, descs)
			},
		},
		{
			name: "link",
			nlri: NewBGPLSLink(
				ProtoISISL2, 0x200,
				NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
				NodeDescriptor{ASN: 65002, IGPRouterID: []byte{2, 2, 2, 2}},
				LinkDescriptor{LinkLocalID: 100, LinkRemoteID: 200},
			),
			wantType:   "bgpls-link",
			wantFields: []string{"ls-nlri-type", "local-node-descriptors", "remote-node-descriptors", "link-identifiers"},
		},
		{
			name: "prefix-v4",
			nlri: NewBGPLSPrefixV4(
				ProtoOSPFv2, 0x100,
				NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
				PrefixDescriptor{IPReachabilityInfo: []byte{24, 10, 0, 0}},
			),
			wantType:   "bgpls-prefix-v4",
			wantFields: []string{"ls-nlri-type", "node-descriptors"},
		},
		{
			name: "prefix-v6",
			nlri: NewBGPLSPrefixV6(
				ProtoOSPFv3, 0x200,
				NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
				PrefixDescriptor{IPReachabilityInfo: []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}},
			),
			wantType:   "bgpls-prefix-v6",
			wantFields: []string{"ls-nlri-type", "node-descriptors"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf []byte
			switch n := tt.nlri.(type) {
			case *BGPLSNode:
				buf = n.AppendJSON(buf)
			case *BGPLSLink:
				buf = n.AppendJSON(buf)
			case *BGPLSPrefix:
				buf = n.AppendJSON(buf)
			case *BGPLSSRv6SID:
				buf = n.AppendJSON(buf)
			default:
				t.Fatalf("unexpected NLRI type %T", n)
			}

			output := string(buf)
			var parsed map[string]any
			require.NoError(t, json.Unmarshal([]byte(output), &parsed), "output must be valid JSON: %s", output)

			assert.Equal(t, tt.wantType, parsed["ls-nlri-type"])
			for _, f := range tt.wantFields {
				assert.Contains(t, parsed, f, "missing field %q in %s", f, output)
			}
			if tt.extraChecks != nil {
				tt.extraChecks(t, parsed)
			}
		})
	}
}

// VALIDATES: AppendJSON on an SRv6 SID NLRI emits bgpls-srv6-sid with the srv6-sid field.
// PREVENTS: RFC 9514 SRv6 SID information dropped from JSON output.
func TestBGPLSAppendJSONSRv6SID(t *testing.T) {
	t.Parallel()

	sid := NewBGPLSSRv6SID(
		ProtoOSPFv3, 0x300,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		SRv6SIDDescriptor{SRv6SID: []byte{
			0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0x42,
		}},
	)

	buf := sid.AppendJSON(nil)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf, &parsed))

	assert.Equal(t, "bgpls-srv6-sid", parsed["ls-nlri-type"])
	assert.Contains(t, parsed, "srv6-sid")
}

// VALIDATES: BGP-LS Prefix NLRI with empty IPReachabilityInfo omits
// ip-reach-prefix/ip-reachability-tlv and still produces valid JSON.
// PREVENTS: missing prefix branch emitting malformed JSON (trailing comma,
// unclosed brace) or accidentally emitting empty-string prefix fields.
func TestBGPLSAppendJSONPrefixEmpty(t *testing.T) {
	t.Parallel()

	prefix := NewBGPLSPrefixV4(
		ProtoOSPFv2, 0x400,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		PrefixDescriptor{IPReachabilityInfo: nil},
	)

	buf := prefix.AppendJSON(nil)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf, &parsed), "output must be valid JSON: %s", string(buf))
	assert.Equal(t, "bgpls-prefix-v4", parsed["ls-nlri-type"])
	assert.NotContains(t, parsed, "ip-reach-prefix")
	assert.NotContains(t, parsed, "ip-reachability-tlv")
}

// VALIDATES: AppendJSON produces the same semantic JSON as the RPC decode path
// (bgplsToJSON + json.Marshal). Two implementations must stay in sync -- the
// json.go file doc says "update both if the shape changes" -- and this test
// makes the divergence mechanical to detect.
// PREVENTS: silent drift between the in-process fast path and the RPC decode
// path when one of them gains a field or renames a key.
func TestBGPLSAppendJSONMatchesRPCDecode(t *testing.T) {
	t.Parallel()

	cases := []BGPLSNLRI{
		NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
			ASN: 65001, BGPLSIdentifier: 0x12345678, IGPRouterID: []byte{1, 1, 1, 1},
		}),
		NewBGPLSLink(
			ProtoISISL2, 0x200,
			NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
			NodeDescriptor{ASN: 65002, IGPRouterID: []byte{2, 2, 2, 2}},
			LinkDescriptor{LinkLocalID: 100, LinkRemoteID: 200},
		),
		NewBGPLSPrefixV4(
			ProtoOSPFv2, 0x100,
			NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
			PrefixDescriptor{IPReachabilityInfo: []byte{24, 10, 0, 0}},
		),
		NewBGPLSPrefixV6(
			ProtoOSPFv3, 0x200,
			NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
			PrefixDescriptor{IPReachabilityInfo: []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}},
		),
		NewBGPLSSRv6SID(
			ProtoOSPFv3, 0x300,
			NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
			SRv6SIDDescriptor{SRv6SID: []byte{
				0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0x42,
			}},
		),
	}

	for _, n := range cases {
		t.Run(bgplsNLRITypeString(uint16(n.NLRIType())), func(t *testing.T) {
			t.Parallel()

			var buf []byte
			switch c := n.(type) {
			case *BGPLSNode:
				buf = c.AppendJSON(buf)
			case *BGPLSLink:
				buf = c.AppendJSON(buf)
			case *BGPLSPrefix:
				buf = c.AppendJSON(buf)
			case *BGPLSSRv6SID:
				buf = c.AppendJSON(buf)
			default:
				t.Fatalf("unsupported NLRI type %T", c)
			}

			// RPC decode reference.
			refMap := bgplsToJSON(n, n.Bytes())
			refBytes, err := json.Marshal(refMap)
			require.NoError(t, err)

			// Byte-for-byte comparison: both paths must sort keys alphabetically.
			assert.Equal(t, string(refBytes), string(buf),
				"AppendJSON output must match json.Marshal(bgplsToJSON(...)) byte-for-byte")
		})
	}
}
