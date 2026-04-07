package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsSupportedFamily verifies family detection for JSON validation.
//
// VALIDATES: Unicast and FlowSpec families are supported, others deferred.
// PREVENTS: Attempting to validate unsupported families (EVPN, VPN, BGP-LS).
func TestIsSupportedFamily(t *testing.T) {
	tests := []struct {
		name   string
		family string
		want   bool
	}{
		// Supported families (Phase 1 + FlowSpec)
		{"ipv4_unicast", "ipv4/unicast", true},
		{"ipv6_unicast", "ipv6/unicast", true},
		{"ipv4_unicast_space", "ipv4 unicast", true},
		{"ipv4_flowspec", "ipv4/flow", true},
		{"ipv6_flowspec", "ipv6/flow", true},
		{"ipv4_flowspec_space", "ipv4 flow", true},

		// Unsupported families (deferred)
		{"l2vpn_evpn", "l2vpn/evpn", false},
		{"ipv4_mpls_vpn", "ipv4/mpls-vpn", false},
		{"ipv4_flowspec_vpn", "ipv4/flow-vpn", false},
		{"bgpls", "bgp-ls/bgp-ls", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSupportedFamily(tt.family)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTransformEnvelopeToPlugin_IPv4Announce verifies IPv4 announce transformation.
//
// VALIDATES: ze-bgp JSON envelope format transforms to plugin format correctly.
// PREVENTS: Broken JSON validation due to format mismatch.
func TestTransformEnvelopeToPlugin_IPv4Announce(t *testing.T) {
	// ze-bgp JSON format from ze bgp decode
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{
				"type":      "update",
				"id":        float64(0),
				"direction": "received",
			},
			"peer": map[string]any{
				"address": "127.0.0.1",
				"remote":  map[string]any{"as": float64(65533)},
			},
			"update": map[string]any{
				"attr": map[string]any{
					"origin":           "igp",
					"local-preference": float64(200),
				},
				"ipv4/unicast": []any{
					map[string]any{
						"next-hop": "10.0.1.254",
						"action":   "add",
						"nlri":     []any{"10.0.1.0/24"},
					},
				},
			},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/unicast", fam)

	// Check transformed structure
	msg, ok := result["message"].(map[string]any)
	require.True(t, ok, "missing message")
	assert.Equal(t, "update", msg["type"])

	// Attributes at top level
	assert.Equal(t, "igp", result["origin"])
	assert.Equal(t, float64(200), result["local-preference"])

	// NLRI passed through (already in plugin format)
	nlriList, ok := result["ipv4/unicast"].([]any)
	require.True(t, ok, "missing ipv4/unicast")
	require.Len(t, nlriList, 1)
	nlri, ok := nlriList[0].(map[string]any)
	require.True(t, ok, "nlri entry must be map")
	assert.Equal(t, "10.0.1.254", nlri["next-hop"])
	assert.Equal(t, "add", nlri["action"])
}

// TestTransformEnvelopeToPlugin_IPv4Withdraw verifies IPv4 withdraw transformation.
//
// VALIDATES: Withdrawn routes with action:del in ze-bgp format.
// PREVENTS: Withdrawals showing as announces.
func TestTransformEnvelopeToPlugin_IPv4Withdraw(t *testing.T) {
	// ze-bgp JSON format
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "update"},
			"peer":    map[string]any{"address": "127.0.0.1", "remote": map[string]any{"as": float64(65533)}},
			"update": map[string]any{
				"ipv4/unicast": []any{
					map[string]any{"action": "del", "nlri": []any{"10.0.1.0/24"}},
				},
			},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/unicast", fam)

	// NLRI passed through
	nlriList, ok := result["ipv4/unicast"].([]any)
	require.True(t, ok, "missing ipv4/unicast")
	require.Len(t, nlriList, 1)
	nlri, ok := nlriList[0].(map[string]any)
	require.True(t, ok, "nlri entry must be map")
	assert.Equal(t, "del", nlri["action"])
}

// TestTransformEnvelopeToPlugin_IPv6Withdraw verifies IPv6 withdraw transformation.
//
// VALIDATES: MP_UNREACH withdraw format transforms correctly.
// PREVENTS: IPv6 withdrawals failing transformation.
func TestTransformEnvelopeToPlugin_IPv6Withdraw(t *testing.T) {
	// ze-bgp JSON format
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "update"},
			"peer":    map[string]any{"address": "::1", "remote": map[string]any{"as": float64(65533)}},
			"update": map[string]any{
				"ipv6/unicast": []any{
					map[string]any{"action": "del", "nlri": []any{"fc00:1::/64"}},
				},
			},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv6/unicast", fam)

	// NLRI passed through
	nlriList, ok := result["ipv6/unicast"].([]any)
	require.True(t, ok, "missing ipv6/unicast")
	require.Len(t, nlriList, 1)
	nlri, ok := nlriList[0].(map[string]any)
	require.True(t, ok, "nlri entry must be map")
	assert.Equal(t, "del", nlri["action"])
}

// TestTransformEnvelopeToPlugin_IPv6Announce verifies IPv6 unicast transformation.
//
// VALIDATES: IPv6 unicast transforms like IPv4.
// PREVENTS: Family-specific transformation bugs.
func TestTransformEnvelopeToPlugin_IPv6Announce(t *testing.T) {
	// ze-bgp JSON format
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "update"},
			"peer":    map[string]any{"address": "::1", "remote": map[string]any{"as": float64(65533)}},
			"update": map[string]any{
				"attr": map[string]any{
					"origin":           "igp",
					"local-preference": float64(200),
				},
				"ipv6/unicast": []any{
					map[string]any{
						"next-hop": "2001::11",
						"action":   "add",
						"nlri":     []any{"fc00:1::/64"},
					},
				},
			},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv6/unicast", fam)

	// NLRI passed through
	nlriList, ok := result["ipv6/unicast"].([]any)
	require.True(t, ok, "missing ipv6/unicast")
	require.Len(t, nlriList, 1)
	nlri, ok := nlriList[0].(map[string]any)
	require.True(t, ok, "nlri entry must be map")
	assert.Equal(t, "2001::11", nlri["next-hop"])
	assert.Equal(t, "add", nlri["action"])
}

// TestTransformEnvelopeToPlugin_EOR verifies End-of-RIB transformation.
//
// VALIDATES: Empty UPDATE (EOR) transforms correctly.
// PREVENTS: Panic on empty message content.
func TestTransformEnvelopeToPlugin_EOR(t *testing.T) {
	// ze-bgp JSON format - EOR has empty update
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "update"},
			"peer":    map[string]any{"address": "127.0.0.1", "remote": map[string]any{"as": float64(65533)}},
			"update":  map[string]any{},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "", fam) // No family for EOR

	// Should have message type
	msg, ok := result["message"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "update", msg["type"])
}

// TestComparePluginJSON_Match verifies matching JSON passes.
//
// VALIDATES: Identical JSON content passes comparison.
// PREVENTS: False negatives in JSON comparison.
func TestComparePluginJSON_Match(t *testing.T) {
	actual := map[string]any{
		"message":          map[string]any{"type": "update"},
		"origin":           "igp",
		"local-preference": float64(200),
		"ipv4/unicast": []map[string]any{
			{"next-hop": "10.0.1.254", "action": "add", "nlri": []string{"10.0.1.0/24"}},
		},
	}

	expected := `{"message":{"type":"update"},"origin":"igp","local-preference":200,"ipv4/unicast":[{"next-hop":"10.0.1.254","action":"add","nlri":["10.0.1.0/24"]}]}`

	err := comparePluginJSON(actual, expected)
	assert.NoError(t, err)
}

// TestComparePluginJSON_Mismatch verifies mismatch fails with diff.
//
// VALIDATES: Different JSON content fails comparison.
// PREVENTS: False positives in JSON comparison.
func TestComparePluginJSON_Mismatch(t *testing.T) {
	actual := map[string]any{
		"message":          map[string]any{"type": "update"},
		"origin":           "egp", // Different!
		"local-preference": float64(200),
	}

	expected := `{"message":{"type":"update"},"origin":"igp","local-preference":200}`

	err := comparePluginJSON(actual, expected)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

// TestComparePluginJSON_MismatchFieldLevel verifies field-level diff in mismatch errors.
// Instead of dumping two full JSON blobs, the error should name the differing fields.
//
// VALIDATES: AC-5 — JSON mismatch error names differing fields.
// PREVENTS: Developer scanning two large JSON dumps to find differences.
func TestComparePluginJSON_MismatchFieldLevel(t *testing.T) {
	actual := map[string]any{
		"message":          map[string]any{"type": "update"},
		"origin":           "egp",
		"local-preference": float64(300),
		"extra-field":      "unexpected",
	}

	expected := `{"message":{"type":"update"},"origin":"igp","local-preference":200,"missing-field":"value"}`

	err := comparePluginJSON(actual, expected)
	require.Error(t, err)

	errMsg := err.Error()
	// Error must contain structured diff markers (changed/added/removed), not just two full dumps.
	// The old format was "JSON mismatch:\nExpected:\n...\nActual:\n..." — reject that.
	assert.NotContains(t, errMsg, "Expected:\n", "error should use field-level diff, not full dumps")
	assert.NotContains(t, errMsg, "Actual:\n", "error should use field-level diff, not full dumps")

	// Must identify the kind of difference for each field
	assert.Contains(t, errMsg, "changed", "error should label changed fields")
	assert.Contains(t, errMsg, "added", "error should label added fields (in actual but not expected)")
	assert.Contains(t, errMsg, "removed", "error should label removed fields (in expected but not actual)")
}

// TestComparePluginJSON_ArrayElementDiff verifies field-level diff recurses into array elements.
// When arrays differ in nested map fields, the diff should name the specific sub-field,
// not dump the entire array element as "changed".
//
// VALIDATES: AC-5 — array element diff recurses into nested maps.
// PREVENTS: Opaque "changed: ipv4/unicast[0] = {...} (expected {...})" dumps.
func TestComparePluginJSON_ArrayElementDiff(t *testing.T) {
	actual := map[string]any{
		"message": map[string]any{"type": "update"},
		"origin":  "igp",
		"ipv4/unicast": []any{
			map[string]any{"next-hop": "10.0.1.1", "action": "add", "nlri": []any{"10.0.0.0/24"}},
		},
	}

	// Expected has different next-hop inside the array element
	expected := `{"message":{"type":"update"},"origin":"igp","ipv4/unicast":[{"next-hop":"10.0.2.2","action":"add","nlri":["10.0.0.0/24"]}]}`

	err := comparePluginJSON(actual, expected)
	require.Error(t, err)

	errMsg := err.Error()
	// Diff must identify the specific field within the array element
	assert.Contains(t, errMsg, "next-hop", "diff should name the differing field inside the array element")
	assert.Contains(t, errMsg, "changed", "diff should label the field as changed")
}

// TestComparePluginJSON_IgnoresContextFields verifies peer/direction ignored.
//
// VALIDATES: Context-dependent fields don't affect comparison.
// PREVENTS: False failures due to test context differences.
func TestComparePluginJSON_IgnoresContextFields(t *testing.T) {
	// Actual has peer/direction
	actual := map[string]any{
		"message":   map[string]any{"type": "update"},
		"direction": "in",
		"peer":      map[string]any{"address": "127.0.0.1", "remote": map[string]any{"as": 65533}},
		"origin":    "igp",
	}

	// Expected has different peer/direction
	expected := `{"message":{"type":"update"},"direction":"out","peer":{"address":"10.0.0.1","remote":{"as":65000}},"origin":"igp"}`

	err := comparePluginJSON(actual, expected)
	assert.NoError(t, err)
}

// TestComparePluginJSON_OrderIndependent verifies key order doesn't matter.
//
// VALIDATES: JSON comparison is order-independent.
// PREVENTS: Failures due to Go map iteration order.
func TestComparePluginJSON_OrderIndependent(t *testing.T) {
	actual := map[string]any{
		"origin":           "igp",
		"local-preference": float64(200),
		"message":          map[string]any{"type": "update"},
	}

	// Same fields, different order
	expected := `{"message":{"type":"update"},"local-preference":200,"origin":"igp"}`

	err := comparePluginJSON(actual, expected)
	assert.NoError(t, err)
}

// TestTransformEnvelopeToPlugin_FlowSpecAnnounce verifies FlowSpec announce transformation.
// FlowSpec NLRI contains components (destination-ipv4, tcp-flags, etc.) rather than simple prefixes.
//
// VALIDATES: FlowSpec components are preserved in plugin format.
// PREVENTS: FlowSpec validation failures due to format mismatch.
func TestTransformEnvelopeToPlugin_FlowSpecAnnounce(t *testing.T) {
	// ze-bgp JSON format for FlowSpec
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "update"},
			"peer":    map[string]any{"address": "127.0.0.1", "remote": map[string]any{"as": float64(65533)}},
			"update": map[string]any{
				"attr": map[string]any{
					"origin":           "igp",
					"local-preference": float64(100),
				},
				"ipv4/flow": []any{
					map[string]any{
						"action": "add",
						"nlri": []any{
							map[string]any{
								"tcp-flags": []any{"=rst", "=fin+push"},
								"string":    "flow tcp-flags [ =rst =fin+push ]",
							},
						},
					},
				},
			},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/flow", fam)

	// Check transformed structure
	msg, ok := result["message"].(map[string]any)
	require.True(t, ok, "missing message")
	assert.Equal(t, "update", msg["type"])

	// Attributes at top level
	assert.Equal(t, "igp", result["origin"])
	assert.Equal(t, float64(100), result["local-preference"])

	// FlowSpec NLRI passed through
	nlriList, ok := result["ipv4/flow"].([]any)
	require.True(t, ok, "missing ipv4/flow")
	require.Len(t, nlriList, 1)
	nlri, ok := nlriList[0].(map[string]any)
	require.True(t, ok, "nlri entry must be map")
	assert.Equal(t, "add", nlri["action"])
}

// TestTransformEnvelopeToPlugin_FlowSpecWithNextHop verifies FlowSpec with redirect next-hop.
//
// VALIDATES: FlowSpec with next-hop includes it in transformation.
// PREVENTS: Missing next-hop in redirect rules.
func TestTransformEnvelopeToPlugin_FlowSpecWithNextHop(t *testing.T) {
	// ze-bgp JSON format
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "update"},
			"peer":    map[string]any{"address": "127.0.0.1", "remote": map[string]any{"as": float64(65533)}},
			"update": map[string]any{
				"attr": map[string]any{
					"origin":           "igp",
					"local-preference": float64(100),
				},
				"ipv4/flow": []any{
					map[string]any{
						"next-hop": "1.2.3.4",
						"action":   "add",
						"nlri": []any{
							map[string]any{
								"destination-ipv4": []any{"192.168.0.1/32"},
								"string":           "flow destination-ipv4 192.168.0.1/32",
							},
						},
					},
				},
			},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/flow", fam)

	// FlowSpec NLRI passed through
	nlriList, ok := result["ipv4/flow"].([]any)
	require.True(t, ok, "missing ipv4/flow")
	require.Len(t, nlriList, 1)
	nlri, ok := nlriList[0].(map[string]any)
	require.True(t, ok, "nlri entry must be map")
	assert.Equal(t, "add", nlri["action"])
	assert.Equal(t, "1.2.3.4", nlri["next-hop"])
}

// TestTransformEnvelopeToPlugin_IPv6FlowSpec verifies IPv6 FlowSpec transformation.
//
// VALIDATES: IPv6 FlowSpec transforms correctly with all components.
// PREVENTS: IPv6 FlowSpec validation failures.
func TestTransformEnvelopeToPlugin_IPv6FlowSpec(t *testing.T) {
	// ze-bgp JSON format
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "update"},
			"peer":    map[string]any{"address": "::1", "remote": map[string]any{"as": float64(65533)}},
			"update": map[string]any{
				"attr": map[string]any{
					"origin":           "igp",
					"local-preference": float64(100),
				},
				"ipv6/flow": []any{
					map[string]any{
						"action": "add",
						"nlri": []any{
							map[string]any{
								"destination-ipv6": []any{"2a02:29b8:1925::2e69/128/0"},
								"source-ipv6":      []any{"beef:f00e::/64/0"},
								"next-header":      []any{"=tcp"},
								"fragment":         []any{"first-fragment", "is-fragment", "last-fragment"},
								"string":           "flow destination-ipv6 2a02:29b8:1925::2e69/128/0 source-ipv6 beef:f00e::/64/0 next-header =tcp fragment [ first-fragment is-fragment last-fragment ]",
							},
						},
					},
				},
			},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv6/flow", fam)

	// IPv6 FlowSpec NLRI passed through
	nlriList, ok := result["ipv6/flow"].([]any)
	require.True(t, ok, "missing ipv6/flow")
	require.Len(t, nlriList, 1)
	nlri, ok := nlriList[0].(map[string]any)
	require.True(t, ok, "nlri entry must be map")
	assert.Equal(t, "add", nlri["action"])
}

// TestTransformEnvelopeToPlugin_FlowSpecWithdraw verifies FlowSpec withdraw transformation.
// FlowSpec withdraws have component objects like announces, but no next-hop.
//
// VALIDATES: FlowSpec withdraw produces action:del with components in nlri.
// PREVENTS: FlowSpec withdrawals failing transformation.
func TestTransformEnvelopeToPlugin_FlowSpecWithdraw(t *testing.T) {
	// ze-bgp JSON format
	envelope := map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"message": map[string]any{"type": "update"},
			"peer":    map[string]any{"address": "127.0.0.1", "remote": map[string]any{"as": float64(65533)}},
			"update": map[string]any{
				"ipv4/flow": []any{
					map[string]any{
						"action": "del",
						"nlri": []any{
							map[string]any{
								"destination": []any{"192.168.0.1/32"},
								"source":      []any{"10.0.0.2/32"},
								"protocol":    []any{"=tcp"},
								"string":      "flow destination 192.168.0.1/32 source 10.0.0.2/32 protocol =tcp",
							},
						},
					},
				},
			},
		},
	}

	result, fam := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/flow", fam)

	// FlowSpec withdraw with action:del
	nlriList, ok := result["ipv4/flow"].([]any)
	require.True(t, ok, "missing ipv4/flow")
	require.Len(t, nlriList, 1)
	nlriEntry, ok := nlriList[0].(map[string]any)
	require.True(t, ok, "nlri entry must be map")
	assert.Equal(t, "del", nlriEntry["action"])
}

// TestExtractFamily verifies family extraction from envelope.
//
// VALIDATES: Family string extracted from ze-bgp update section.
// PREVENTS: Wrong family detection causing skipped validation.
func TestExtractFamily(t *testing.T) {
	tests := []struct {
		name     string
		envelope map[string]any
		want     string
	}{
		{
			name: "ipv4_unicast",
			envelope: map[string]any{
				"type": "bgp",
				"bgp": map[string]any{
					"message": map[string]any{"type": "update"},
					"update": map[string]any{
						"ipv4/unicast": []any{
							map[string]any{"action": "add", "nlri": []any{"10.0.0.0/24"}},
						},
					},
				},
			},
			want: "ipv4/unicast",
		},
		{
			name: "ipv6_unicast",
			envelope: map[string]any{
				"type": "bgp",
				"bgp": map[string]any{
					"message": map[string]any{"type": "update"},
					"update": map[string]any{
						"ipv6/unicast": []any{
							map[string]any{"action": "del", "nlri": []any{"fc00::/64"}},
						},
					},
				},
			},
			want: "ipv6/unicast",
		},
		{
			name: "ipv4_flowspec",
			envelope: map[string]any{
				"type": "bgp",
				"bgp": map[string]any{
					"message": map[string]any{"type": "update"},
					"update": map[string]any{
						"ipv4/flow": []any{
							map[string]any{"action": "add", "nlri": []any{}},
						},
					},
				},
			},
			want: "ipv4/flow",
		},
		{
			name: "eor_empty",
			envelope: map[string]any{
				"type": "bgp",
				"bgp": map[string]any{
					"message": map[string]any{"type": "update"},
					"update":  map[string]any{},
				},
			},
			want: "",
		},
		{
			name: "legacy_ipv4_announce",
			envelope: map[string]any{
				"neighbor": map[string]any{
					"message": map[string]any{
						"update": map[string]any{
							"announce": map[string]any{
								"ipv4/unicast": map[string]any{},
							},
						},
					},
				},
			},
			want: "ipv4/unicast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFamily(tt.envelope)
			assert.Equal(t, tt.want, got)
		})
	}
}
