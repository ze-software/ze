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
		{"ipv4_flowspec", "ipv4/flowspec", true},
		{"ipv6_flowspec", "ipv6/flowspec", true},
		{"ipv4_flowspec_space", "ipv4 flowspec", true},

		// Unsupported families (deferred)
		{"l2vpn_evpn", "l2vpn/evpn", false},
		{"ipv4_mpls_vpn", "ipv4/mpls-vpn", false},
		{"ipv4_flowspec_vpn", "ipv4/flowspec-vpn", false},
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
// VALIDATES: Zebgp decode envelope format transforms to plugin format correctly.
// PREVENTS: Broken JSON validation due to format mismatch.
func TestTransformEnvelopeToPlugin_IPv4Announce(t *testing.T) {
	// Zebgp decode envelope format
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "127.0.0.1", "peer": "127.0.0.1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{
					"attribute": map[string]any{
						"origin":           "igp",
						"local-preference": float64(200),
					},
					"announce": map[string]any{
						"ipv4/unicast": map[string]any{
							"10.0.1.254": []any{
								map[string]any{"nlri": "10.0.1.0/24"},
							},
						},
					},
				},
			},
		},
	}

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/unicast", family)

	// Check transformed structure
	msg, ok := result["message"].(map[string]any)
	require.True(t, ok, "missing message")
	assert.Equal(t, "update", msg["type"])

	// Attributes at top level
	assert.Equal(t, "igp", result["origin"])
	assert.Equal(t, float64(200), result["local-preference"])

	// NLRI transformed to plugin format
	nlriList, ok := result["ipv4/unicast"].([]map[string]any)
	require.True(t, ok, "missing ipv4/unicast")
	require.Len(t, nlriList, 1)
	assert.Equal(t, "10.0.1.254", nlriList[0]["next-hop"])
	assert.Equal(t, "add", nlriList[0]["action"])
	assert.Equal(t, []string{"10.0.1.0/24"}, nlriList[0]["nlri"])
}

// TestTransformEnvelopeToPlugin_IPv4Withdraw verifies IPv4 withdraw transformation.
// Zebgp decode produces ["prefix"] format for IPv4 unicast withdraws.
//
// VALIDATES: Withdrawn routes get action:del in plugin format.
// PREVENTS: Withdrawals showing as announces.
func TestTransformEnvelopeToPlugin_IPv4Withdraw(t *testing.T) {
	// Zebgp decode format uses ["prefix"] for IPv4 unicast withdraws
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "127.0.0.1", "peer": "127.0.0.1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{
					"withdraw": map[string]any{
						"ipv4/unicast": []any{"10.0.1.0/24"},
					},
				},
			},
		},
	}

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/unicast", family)

	// NLRI transformed to plugin format with action:del
	nlriList, ok := result["ipv4/unicast"].([]map[string]any)
	require.True(t, ok, "missing ipv4/unicast")
	require.Len(t, nlriList, 1)
	assert.Equal(t, "del", nlriList[0]["action"])
	assert.Equal(t, []string{"10.0.1.0/24"}, nlriList[0]["nlri"])
}

// TestTransformEnvelopeToPlugin_IPv6Withdraw verifies IPv6 withdraw transformation.
// Zebgp decode produces [{"nlri":"prefix"}] format for IPv6/MP withdraws.
//
// VALIDATES: MP_UNREACH withdraw format transforms correctly.
// PREVENTS: IPv6 withdrawals failing transformation.
func TestTransformEnvelopeToPlugin_IPv6Withdraw(t *testing.T) {
	// Zebgp decode format uses [{"nlri": "prefix"}] for IPv6/MP withdraws
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "::1", "peer": "::1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{
					"withdraw": map[string]any{
						"ipv6/unicast": []any{
							map[string]any{"nlri": "fc00:1::/64"},
						},
					},
				},
			},
		},
	}

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv6/unicast", family)

	// NLRI transformed to plugin format with action:del
	nlriList, ok := result["ipv6/unicast"].([]map[string]any)
	require.True(t, ok, "missing ipv6/unicast")
	require.Len(t, nlriList, 1)
	assert.Equal(t, "del", nlriList[0]["action"])
	assert.Equal(t, []string{"fc00:1::/64"}, nlriList[0]["nlri"])
}

// TestTransformEnvelopeToPlugin_IPv6Announce verifies IPv6 unicast transformation.
//
// VALIDATES: IPv6 unicast transforms like IPv4.
// PREVENTS: Family-specific transformation bugs.
func TestTransformEnvelopeToPlugin_IPv6Announce(t *testing.T) {
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "::1", "peer": "::1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{
					"attribute": map[string]any{
						"origin":           "igp",
						"local-preference": float64(200),
					},
					"announce": map[string]any{
						"ipv6/unicast": map[string]any{
							"2001::11": []any{
								map[string]any{"nlri": "fc00:1::/64"},
							},
						},
					},
				},
			},
		},
	}

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv6/unicast", family)

	// NLRI transformed to plugin format
	nlriList, ok := result["ipv6/unicast"].([]map[string]any)
	require.True(t, ok, "missing ipv6/unicast")
	require.Len(t, nlriList, 1)
	assert.Equal(t, "2001::11", nlriList[0]["next-hop"])
	assert.Equal(t, "add", nlriList[0]["action"])
	assert.Equal(t, []string{"fc00:1::/64"}, nlriList[0]["nlri"])
}

// TestTransformEnvelopeToPlugin_EOR verifies End-of-RIB transformation.
//
// VALIDATES: Empty UPDATE (EOR) transforms correctly.
// PREVENTS: Panic on empty message content.
func TestTransformEnvelopeToPlugin_EOR(t *testing.T) {
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "127.0.0.1", "peer": "127.0.0.1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{},
			},
		},
	}

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "", family) // No family for EOR

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

// TestComparePluginJSON_IgnoresContextFields verifies peer/direction ignored.
//
// VALIDATES: Context-dependent fields don't affect comparison.
// PREVENTS: False failures due to test context differences.
func TestComparePluginJSON_IgnoresContextFields(t *testing.T) {
	// Actual has peer/direction
	actual := map[string]any{
		"message":   map[string]any{"type": "update"},
		"direction": "in",
		"peer":      map[string]any{"address": "127.0.0.1", "asn": 65533},
		"origin":    "igp",
	}

	// Expected has different peer/direction
	expected := `{"message":{"type":"update"},"direction":"out","peer":{"address":"10.0.0.1","asn":65000},"origin":"igp"}`

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
	// Zebgp decode envelope format for FlowSpec with no-nexthop
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "127.0.0.1", "peer": "127.0.0.1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{
					"attribute": map[string]any{
						"origin":           "igp",
						"local-preference": float64(100),
						"extended-community": []any{
							map[string]any{"value": float64(9225060886715039744), "string": "rate-limit:0"},
						},
					},
					"announce": map[string]any{
						"ipv4/flowspec": map[string]any{
							"no-nexthop": []any{
								map[string]any{
									"tcp-flags": []any{"=rst", "=fin+push"},
									"string":    "flow tcp-flags [ =rst =fin+push ]",
								},
							},
						},
					},
				},
			},
		},
	}

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/flowspec", family)

	// Check transformed structure
	msg, ok := result["message"].(map[string]any)
	require.True(t, ok, "missing message")
	assert.Equal(t, "update", msg["type"])

	// Attributes at top level
	assert.Equal(t, "igp", result["origin"])
	assert.Equal(t, float64(100), result["local-preference"])

	// FlowSpec NLRI transformed to plugin format
	nlriList, ok := result["ipv4/flowspec"].([]map[string]any)
	require.True(t, ok, "missing ipv4/flowspec")
	require.Len(t, nlriList, 1)
	assert.Equal(t, "add", nlriList[0]["action"])
	assert.Nil(t, nlriList[0]["next-hop"]) // no-nexthop means no next-hop field

	// FlowSpec components preserved
	nlri, ok := nlriList[0]["nlri"].(map[string]any)
	require.True(t, ok, "missing nlri")
	assert.Equal(t, []any{"=rst", "=fin+push"}, nlri["tcp-flags"])
	assert.Equal(t, "flow tcp-flags [ =rst =fin+push ]", nlri["string"])
}

// TestTransformEnvelopeToPlugin_FlowSpecWithNextHop verifies FlowSpec with redirect next-hop.
//
// VALIDATES: FlowSpec with next-hop includes it in transformation.
// PREVENTS: Missing next-hop in redirect rules.
func TestTransformEnvelopeToPlugin_FlowSpecWithNextHop(t *testing.T) {
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "127.0.0.1", "peer": "127.0.0.1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{
					"attribute": map[string]any{
						"origin":           "igp",
						"local-preference": float64(100),
					},
					"announce": map[string]any{
						"ipv4/flowspec": map[string]any{
							"1.2.3.4": []any{
								map[string]any{
									"destination-ipv4": []any{"192.168.0.1/32"},
									"string":           "flow destination-ipv4 192.168.0.1/32",
								},
							},
						},
					},
				},
			},
		},
	}

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/flowspec", family)

	// FlowSpec NLRI with next-hop inside nlri
	nlriList, ok := result["ipv4/flowspec"].([]map[string]any)
	require.True(t, ok, "missing ipv4/flowspec")
	require.Len(t, nlriList, 1)
	assert.Equal(t, "add", nlriList[0]["action"])
	assert.Nil(t, nlriList[0]["next-hop"]) // next-hop is inside nlri, not at top level

	// FlowSpec components preserved with next-hop inside
	nlri, ok := nlriList[0]["nlri"].(map[string]any)
	require.True(t, ok, "missing nlri")
	assert.Equal(t, []any{"192.168.0.1/32"}, nlri["destination-ipv4"])
	assert.Equal(t, "1.2.3.4", nlri["next-hop"])
}

// TestTransformEnvelopeToPlugin_IPv6FlowSpec verifies IPv6 FlowSpec transformation.
//
// VALIDATES: IPv6 FlowSpec transforms correctly with all components.
// PREVENTS: IPv6 FlowSpec validation failures.
func TestTransformEnvelopeToPlugin_IPv6FlowSpec(t *testing.T) {
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "::1", "peer": "::1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{
					"attribute": map[string]any{
						"origin":           "igp",
						"local-preference": float64(100),
					},
					"announce": map[string]any{
						"ipv6/flowspec": map[string]any{
							"no-nexthop": []any{
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
		},
	}

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv6/flowspec", family)

	// IPv6 FlowSpec NLRI
	nlriList, ok := result["ipv6/flowspec"].([]map[string]any)
	require.True(t, ok, "missing ipv6/flowspec")
	require.Len(t, nlriList, 1)
	assert.Equal(t, "add", nlriList[0]["action"])

	// All components preserved
	nlri, ok := nlriList[0]["nlri"].(map[string]any)
	require.True(t, ok, "missing nlri")
	assert.Equal(t, []any{"2a02:29b8:1925::2e69/128/0"}, nlri["destination-ipv6"])
	assert.Equal(t, []any{"beef:f00e::/64/0"}, nlri["source-ipv6"])
	assert.Equal(t, []any{"=tcp"}, nlri["next-header"])
	assert.Equal(t, []any{"first-fragment", "is-fragment", "last-fragment"}, nlri["fragment"])
}

// TestTransformEnvelopeToPlugin_FlowSpecWithdraw verifies FlowSpec withdraw transformation.
// FlowSpec withdraws have component objects like announces, but no next-hop.
//
// VALIDATES: FlowSpec withdraw produces action:del with components in nlri.
// PREVENTS: FlowSpec withdrawals failing transformation.
func TestTransformEnvelopeToPlugin_FlowSpecWithdraw(t *testing.T) {
	envelope := map[string]any{
		"exabgp": "5.0.0",
		"type":   "update",
		"neighbor": map[string]any{
			"address":   map[string]any{"local": "127.0.0.1", "peer": "127.0.0.1"},
			"asn":       map[string]any{"local": 65533, "peer": 65533},
			"direction": "in",
			"message": map[string]any{
				"update": map[string]any{
					"withdraw": map[string]any{
						"ipv4/flowspec": []any{
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

	result, family := transformEnvelopeToPlugin(envelope)
	assert.Equal(t, "ipv4/flowspec", family)

	// FlowSpec withdraw with action:del
	nlriList, ok := result["ipv4/flowspec"].([]map[string]any)
	require.True(t, ok, "missing ipv4/flowspec")
	require.Len(t, nlriList, 1)
	assert.Equal(t, "del", nlriList[0]["action"])

	// Components preserved in nlri
	nlri, ok := nlriList[0]["nlri"].(map[string]any)
	require.True(t, ok, "missing nlri")
	assert.Equal(t, []any{"192.168.0.1/32"}, nlri["destination"])
	assert.Equal(t, []any{"10.0.0.2/32"}, nlri["source"])
	assert.Equal(t, []any{"=tcp"}, nlri["protocol"])
}

// TestExtractFamily verifies family extraction from envelope.
//
// VALIDATES: Family string extracted from announce/withdraw sections.
// PREVENTS: Wrong family detection causing skipped validation.
func TestExtractFamily(t *testing.T) {
	tests := []struct {
		name     string
		envelope map[string]any
		want     string
	}{
		{
			name: "ipv4_announce",
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
		{
			name: "ipv6_withdraw",
			envelope: map[string]any{
				"neighbor": map[string]any{
					"message": map[string]any{
						"update": map[string]any{
							"withdraw": map[string]any{
								"ipv6/unicast": []any{},
							},
						},
					},
				},
			},
			want: "ipv6/unicast",
		},
		{
			name: "eor_empty",
			envelope: map[string]any{
				"neighbor": map[string]any{
					"message": map[string]any{
						"update": map[string]any{},
					},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFamily(tt.envelope)
			assert.Equal(t, tt.want, got)
		})
	}
}
