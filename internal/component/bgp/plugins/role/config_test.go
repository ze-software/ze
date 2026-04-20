package role

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// TestImportDeclaresRole verifies "import" keyword sets the local role.
//
// VALIDATES: import <role> sets role correctly for all 5 role values.
// PREVENTS: import keyword being silently ignored, causing no Role capability.
func TestImportDeclaresRole(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantRole string
	}{
		{"customer", `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"customer"}}}}}`, "customer"},
		{"provider", `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider"}}}}}`, "provider"},
		{"peer", `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"peer"}}}}}`, "peer"},
		{"rs", `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"rs"}}}}}`, "rs"},
		{"rs-client", `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"rs-client"}}}}}`, "rs-client"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, _ := extractPeerRoleConfigs(tt.json)
			require.Contains(t, configs, "10.0.0.1")
			assert.Equal(t, tt.wantRole, configs["10.0.0.1"].role)
		})
	}
}

// TestImportReplacesName verifies "import" is accepted and "name" is rejected.
//
// VALIDATES: import replaces the Phase 1 "name" keyword completely.
// PREVENTS: Old "name" keyword silently working after migration.
func TestImportReplacesName(t *testing.T) {
	// import keyword should work.
	importJSON := `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"customer"}}}}}`
	configs, _ := extractPeerRoleConfigs(importJSON)
	require.Contains(t, configs, "10.0.0.1")
	assert.Equal(t, "customer", configs["10.0.0.1"].role)

	// name keyword should no longer be accepted.
	nameJSON := `{"bgp":{"peer":{"10.0.0.1":{"role":{"name":"customer"}}}}}`
	configs, _ = extractPeerRoleConfigs(nameJSON)
	assert.Empty(t, configs, "name keyword should no longer be accepted")
}

// TestParseExportConfig verifies export token parsing from config JSON.
//
// VALIDATES: export tokens parsed correctly as string or string array.
// PREVENTS: Export config being silently ignored.
func TestParseExportConfig(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantExport []string
	}{
		{
			name:       "single_default_string",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider","export":"default"}}}}}`,
			wantExport: []string{"default"},
		},
		{
			name:       "array_tokens",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider","export":["default","unknown"]}}}}}`,
			wantExport: []string{"default", "unknown"},
		},
		{
			name:       "explicit_roles",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider","export":["customer","peer"]}}}}}`,
			wantExport: []string{"customer", "peer"},
		},
		{
			name:       "no_export",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider"}}}}}`,
			wantExport: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, _ := extractPeerRoleConfigs(tt.json)
			require.Contains(t, configs, "10.0.0.1")
			assert.Equal(t, tt.wantExport, configs["10.0.0.1"].export)
			// Verify resolvedExport is pre-computed at config time and matches runtime resolution.
			expected := resolveExport(configs["10.0.0.1"].role, tt.wantExport)
			assert.Equal(t, expected, configs["10.0.0.1"].resolvedExport,
				"resolvedExport should match resolveExport(role, export)")
		})
	}
}

// TestResolveExportEdgeCases verifies edge cases in export token resolution.
//
// VALIDATES: resolveExport handles empty, nil, unknown role, duplicate tokens.
// PREVENTS: Panic or incorrect expansion on malformed input.
func TestResolveExportEdgeCases(t *testing.T) {
	t.Run("nil_tokens", func(t *testing.T) {
		assert.Nil(t, resolveExport("provider", nil))
	})
	t.Run("empty_tokens", func(t *testing.T) {
		assert.Nil(t, resolveExport("provider", []string{}))
	})
	t.Run("unknown_local_role", func(t *testing.T) {
		// "default" for unknown role expands to nothing (no entry in exportDefaults).
		result := resolveExport("bogus", []string{"default"})
		assert.Empty(t, result)
	})
	t.Run("duplicate_tokens_deduped", func(t *testing.T) {
		result := resolveExport("provider", []string{"customer", "customer"})
		assert.Equal(t, []string{"customer"}, result)
	})
	t.Run("default_with_overlap", func(t *testing.T) {
		// "default" for provider = {customer, rs-client}. Adding explicit "customer" should not duplicate.
		result := resolveExport("provider", []string{"default", "customer"})
		assert.ElementsMatch(t, []string{"customer", "rs-client"}, result)
	})
	t.Run("default_twice", func(t *testing.T) {
		result := resolveExport("provider", []string{"default", "default"})
		assert.ElementsMatch(t, []string{"customer", "rs-client"}, result)
	})
}

// TestParseExportConfigEdgeCases verifies edge cases in export config parsing.
//
// VALIDATES: Export parsing handles invalid tokens, empty arrays, mixed types.
// PREVENTS: Panic or silent misconfiguration from malformed export values.
func TestParseExportConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantExport []string
	}{
		{
			name:       "export_empty_string",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider","export":""}}}}}`,
			wantExport: nil,
		},
		{
			name:       "export_empty_array",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider","export":[]}}}}}`,
			wantExport: nil,
		},
		{
			name:       "export_number_ignored",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider","export":42}}}}}`,
			wantExport: nil,
		},
		{
			name:       "export_bool_ignored",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider","export":true}}}}}`,
			wantExport: nil,
		},
		{
			name:       "import_with_strict_and_export",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"customer","strict":true,"export":"default"}}}}}`,
			wantExport: []string{"default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, _ := extractPeerRoleConfigs(tt.json)
			if tt.wantExport == nil {
				if configs != nil && configs["10.0.0.1"] != nil {
					assert.Nil(t, configs["10.0.0.1"].export)
				}
			} else {
				require.Contains(t, configs, "10.0.0.1")
				assert.Equal(t, tt.wantExport, configs["10.0.0.1"].export)
			}
		})
	}
}

// TestImportWithStrictAndExportCombined verifies all three fields parsed together.
//
// VALIDATES: import + strict + export are all parsed from the same role container.
// PREVENTS: One field overwriting or suppressing another.
func TestImportWithStrictAndExportCombined(t *testing.T) {
	jsonStr := `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"customer","strict":true,"export":["default","unknown"]}}}}}`
	configs, _ := extractPeerRoleConfigs(jsonStr)
	require.Contains(t, configs, "10.0.0.1")
	cfg := configs["10.0.0.1"]
	assert.Equal(t, "customer", cfg.role)
	assert.True(t, cfg.strict)
	assert.Equal(t, []string{"default", "unknown"}, cfg.export)
}

// TestExportGroupInheritance verifies export config inherits from group.
//
// VALIDATES: Group-level export config applies to all peers in the group.
// PREVENTS: Export config being lost during group->peer inheritance.
func TestExportGroupInheritance(t *testing.T) {
	jsonStr := `{"bgp":{"group":{"transit":{"role":{"import":"provider","export":"default"},"peer":{
		"10.0.0.1":{"peer-as":65001},
		"10.0.0.2":{"role":{"import":"customer","export":["default","unknown"]}}
	}}}}}`
	configs, _ := extractPeerRoleConfigs(jsonStr)
	require.Len(t, configs, 2)

	// 10.0.0.1 inherits group export.
	assert.Equal(t, []string{"default"}, configs["10.0.0.1"].export)
	assert.Equal(t, "provider", configs["10.0.0.1"].role)
	assert.ElementsMatch(t, []string{"customer", "rs-client"}, configs["10.0.0.1"].resolvedExport,
		"provider default resolvedExport should expand to customer + rs-client")

	// 10.0.0.2 uses its own export (override).
	assert.Equal(t, []string{"default", "unknown"}, configs["10.0.0.2"].export)
	assert.Equal(t, "customer", configs["10.0.0.2"].role)
	assert.ElementsMatch(t, []string{"provider", "rs", "peer", "unknown"}, configs["10.0.0.2"].resolvedExport,
		"customer default+unknown resolvedExport should expand correctly")
}

// TestExportDefault_Provider verifies RFC 9234 default export expansion for Provider.
//
// VALIDATES: export default for Provider allows sending to customer and rs-client.
// PREVENTS: Wrong default expansion causing routes to leak or be suppressed.
func TestExportDefault_Provider(t *testing.T) {
	resolved := resolveExport("provider", []string{"default"})
	assert.ElementsMatch(t, []string{"customer", "rs-client"}, resolved)
}

// TestExportDefault_Customer verifies RFC 9234 default export expansion for Customer.
//
// VALIDATES: export default for Customer allows sending to provider, rs, peer.
// PREVENTS: Customer routes not reaching upstream providers.
func TestExportDefault_Customer(t *testing.T) {
	resolved := resolveExport("customer", []string{"default"})
	assert.ElementsMatch(t, []string{"provider", "rs", "peer"}, resolved)
}

// TestExportDefaultUnknown verifies default + unknown combined.
//
// VALIDATES: export default unknown expands default AND includes "unknown".
// PREVENTS: "unknown" token being lost during default expansion.
func TestExportDefaultUnknown(t *testing.T) {
	resolved := resolveExport("provider", []string{"default", "unknown"})
	assert.ElementsMatch(t, []string{"customer", "rs-client", "unknown"}, resolved)
}

// TestExportExplicitRoles verifies explicit export role list without default.
//
// VALIDATES: Explicit role list is used as-is, no default expansion.
// PREVENTS: Explicit overrides being expanded like defaults.
func TestExportExplicitRoles(t *testing.T) {
	resolved := resolveExport("provider", []string{"customer", "peer"})
	assert.ElementsMatch(t, []string{"customer", "peer"}, resolved)
}

// TestExportDefaultAllRoles verifies default expansion for all 5 roles.
//
// VALIDATES: RFC 9234 Section 5 default egress rules for each local role.
// PREVENTS: Missing role in default expansion.
func TestExportDefaultAllRoles(t *testing.T) {
	tests := []struct {
		localRole string
		want      []string
	}{
		{"provider", []string{"customer", "rs-client"}},
		{"customer", []string{"provider", "rs", "peer"}},
		{"rs", []string{"rs-client"}},
		{"rs-client", []string{"rs", "provider"}},
		{"peer", []string{"customer", "rs-client"}},
	}

	for _, tt := range tests {
		t.Run(tt.localRole, func(t *testing.T) {
			resolved := resolveExport(tt.localRole, []string{"default"})
			assert.ElementsMatch(t, tt.want, resolved)
		})
	}
}

// TestExtractRoleCapabilities_ParseBGPConfig verifies JSON config parsing.
//
// VALIDATES: extractRoleCapabilities correctly parses BGP config JSON and returns
// CapabilityDecl with correct code (9), encoding (hex), and single-byte payload.
// PREVENTS: Config being silently ignored, causing missing Role capability.
func TestExtractRoleCapabilities_ParseBGPConfig(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		wantPeer    string
		wantPayload string
		wantParsed  bool
	}{
		{
			name:        "provider_role_0",
			json:        `{"bgp":{"peer":{"192.168.1.1":{"role":{"import":"provider"}}}}}`,
			wantPeer:    "192.168.1.1",
			wantPayload: "00",
			wantParsed:  true,
		},
		{
			name:        "rs_role_1",
			json:        `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"rs"}}}}}`,
			wantPeer:    "10.0.0.1",
			wantPayload: "01",
			wantParsed:  true,
		},
		{
			name:        "rs_client_role_2",
			json:        `{"bgp":{"peer":{"10.0.0.2":{"role":{"import":"rs-client"}}}}}`,
			wantPeer:    "10.0.0.2",
			wantPayload: "02",
			wantParsed:  true,
		},
		{
			name:        "customer_role_3",
			json:        `{"bgp":{"peer":{"10.0.0.3":{"role":{"import":"customer"}}}}}`,
			wantPeer:    "10.0.0.3",
			wantPayload: "03",
			wantParsed:  true,
		},
		{
			name:        "peer_role_4",
			json:        `{"bgp":{"peer":{"10.0.0.4":{"role":{"import":"peer"}}}}}`,
			wantPeer:    "10.0.0.4",
			wantPayload: "04",
			wantParsed:  true,
		},
		{
			name:       "no_role_capability",
			json:       `{"bgp":{"peer":{"192.168.1.1":{"capability":{"route-refresh":{}}}}}}`,
			wantParsed: false,
		},
		{
			name:       "no_capability_section",
			json:       `{"bgp":{"peer":{"192.168.1.1":{"peer-as":65001}}}}`,
			wantParsed: false,
		},
		{
			name:       "no_peer_section",
			json:       `{"bgp":{"router-id":"1.2.3.4"}}`,
			wantParsed: false,
		},
		{
			name:       "invalid_role_name",
			json:       `{"bgp":{"peer":{"192.168.1.1":{"role":{"import":"invalid"}}}}}`,
			wantParsed: false,
		},
		{
			name:       "empty_role_name",
			json:       `{"bgp":{"peer":{"192.168.1.1":{"role":{"import":""}}}}}`,
			wantParsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := extractRoleCapabilities(tt.json)

			if tt.wantParsed {
				require.Len(t, caps, 1, "should return exactly one capability")
				cap := caps[0]
				assert.Equal(t, uint8(roleCapCode), cap.Code, "capability code must be 9 (Role)")
				assert.Equal(t, sdk.CapEncodingHex, cap.Encoding, "encoding must be hex")
				assert.Equal(t, tt.wantPayload, cap.Payload, "payload hex mismatch")
				require.Len(t, cap.Peers, 1, "should have exactly one peer")
				assert.Equal(t, tt.wantPeer, cap.Peers[0], "peer address mismatch")
			} else {
				assert.Empty(t, caps, "should return no capabilities")
			}
		})
	}
}

// TestExtractRoleCapabilities_MultiplePeers verifies multiple peer config extraction.
//
// VALIDATES: Each peer with Role config produces a separate CapabilityDecl.
// PREVENTS: Only first peer being extracted when multiple peers have Role config.
func TestExtractRoleCapabilities_MultiplePeers(t *testing.T) {
	json := `{"bgp":{"peer":{
		"192.168.1.1":{"role":{"import":"customer"}},
		"10.0.0.1":{"role":{"import":"provider"}}
	}}}`

	caps := extractRoleCapabilities(json)

	require.Len(t, caps, 2, "should return one capability per peer")

	peerPayload := make(map[string]string)
	for _, cap := range caps {
		assert.Equal(t, uint8(roleCapCode), cap.Code)
		assert.Equal(t, sdk.CapEncodingHex, cap.Encoding)
		require.Len(t, cap.Peers, 1)
		peerPayload[cap.Peers[0]] = cap.Payload
	}

	assert.Equal(t, "03", peerPayload["192.168.1.1"], "192.168.1.1 role=customer (3)")
	assert.Equal(t, "00", peerPayload["10.0.0.1"], "10.0.0.1 role=provider (0)")
}

// TestExtractRoleCapabilities_InvalidJSON verifies graceful handling of bad input.
//
// VALIDATES: Invalid JSON does not panic, returns empty slice.
// PREVENTS: Crash on malformed config data.
func TestExtractRoleCapabilities_InvalidJSON(t *testing.T) {
	caps := extractRoleCapabilities(`not valid json`)
	assert.Empty(t, caps, "invalid JSON should return no capabilities")
}

// TestExtractPeerRoleConfigs_StrictParsing verifies strict mode is parsed into peerRoleConfig.
//
// VALIDATES: extractPeerRoleConfigs extracts strict flag from config JSON.
// PREVENTS: Strict mode being silently ignored in config parsing.
func TestExtractPeerRoleConfigs_StrictParsing(t *testing.T) {
	strictJSON := `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"customer","strict":true}}}}}`
	configs, _ := extractPeerRoleConfigs(strictJSON)
	require.Contains(t, configs, "10.0.0.1")
	assert.True(t, configs["10.0.0.1"].strict, "strict should be true when strict is true")

	normalJSON := `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"customer"}}}}}`
	configs, _ = extractPeerRoleConfigs(normalJSON)
	require.Contains(t, configs, "10.0.0.1")
	assert.False(t, configs["10.0.0.1"].strict, "strict should be false when role-strict is absent")
}

// TestExtractPeerRoleConfigs_GroupWithPeerOverride verifies per-peer role overrides group role.
//
// VALIDATES: When a group has role "customer" and one peer has role "provider",
// the per-peer role takes precedence.
// PREVENTS: Group-level role config suppressing per-peer overrides.
func TestExtractPeerRoleConfigs_GroupWithPeerOverride(t *testing.T) {
	jsonStr := `{"bgp":{"group":{"transit":{"role":{"import":"customer"},"peer":{
		"10.0.0.1":{"role":{"import":"provider"}},
		"10.0.0.2":{}
	}}}}}`

	configs, _ := extractPeerRoleConfigs(jsonStr)
	require.Len(t, configs, 2, "both peers should have role configs")

	cfg1 := configs["10.0.0.1"]
	require.NotNil(t, cfg1)
	assert.Equal(t, "provider", cfg1.role, "per-peer role should override group role")

	cfg2 := configs["10.0.0.2"]
	require.NotNil(t, cfg2)
	assert.Equal(t, "customer", cfg2.role, "peer without role should inherit group role")
}

// TestExtractRoleCapabilities_GroupOnly verifies group-level role applies to all peers.
//
// VALIDATES: Group-level role config is applied to all peers in the group.
// PREVENTS: Group-level role being ignored when no per-peer role is set.
func TestExtractRoleCapabilities_GroupOnly(t *testing.T) {
	jsonStr := `{"bgp":{"group":{"transit":{"role":{"import":"customer","strict":true},"peer":{
		"10.0.0.1":{"peer-as":65001},
		"10.0.0.2":{"peer-as":65002}
	}}}}}`

	caps := extractRoleCapabilities(jsonStr)
	require.Len(t, caps, 2, "both peers should get role capabilities from group")

	peerPayload := make(map[string]string)
	for _, cap := range caps {
		require.Len(t, cap.Peers, 1)
		peerPayload[cap.Peers[0]] = cap.Payload
	}

	assert.Equal(t, "03", peerPayload["10.0.0.1"], "10.0.0.1 role=customer (3)")
	assert.Equal(t, "03", peerPayload["10.0.0.2"], "10.0.0.2 role=customer (3)")
}

// TestExtractRoleCapabilities_StrictMode verifies strict mode config extraction.
//
// VALIDATES: extractPeerRoleConfigs correctly extracts strict mode flag per peer.
// PREVENTS: Strict mode being silently ignored, allowing sessions without Role.
func TestExtractRoleCapabilities_StrictMode(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantStrict bool
		wantRole   string
	}{
		{
			name:       "strict_true",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"customer","strict":true}}}}}`,
			wantStrict: true,
			wantRole:   "customer",
		},
		{
			name:       "strict_false_explicit",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"provider","strict":false}}}}}`,
			wantStrict: false,
			wantRole:   "provider",
		},
		{
			name:       "strict_absent_defaults_false",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"role":{"import":"peer"}}}}}`,
			wantStrict: false,
			wantRole:   "peer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, _ := extractPeerRoleConfigs(tt.json)
			require.Len(t, configs, 1, "should return exactly one peer config")

			cfg := configs["10.0.0.1"]
			require.NotNil(t, cfg, "should have config for 10.0.0.1")
			assert.Equal(t, tt.wantRole, cfg.role, "role mismatch")
			assert.Equal(t, tt.wantStrict, cfg.strict, "strict mode mismatch")
		})
	}
}

// TestExtractRemoteIP verifies IP extraction from peer and group config maps.
//
// VALIDATES: extractRemoteIP correctly extracts remote.ip from peer or group config.
// PREVENTS: Key mismatch between config (keyed by name) and filters (keyed by IP).
func TestExtractRemoteIP(t *testing.T) {
	tests := []struct {
		name     string
		peerMap  map[string]any
		groupMap map[string]any
		wantIP   string
	}{
		{"peer_has_ip", map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}}, nil, "10.0.0.1"},
		{"group_has_ip", nil, map[string]any{"remote": map[string]any{"ip": "10.0.0.2"}}, "10.0.0.2"},
		{"peer_overrides_group", map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}}, map[string]any{"remote": map[string]any{"ip": "10.0.0.2"}}, "10.0.0.1"},
		{"no_remote", map[string]any{"peer-as": "65001"}, nil, ""},
		{"remote_no_ip", map[string]any{"remote": map[string]any{"as": "65001"}}, nil, ""},
		{"both_nil", nil, nil, ""},
		{"empty_ip", map[string]any{"remote": map[string]any{"ip": ""}}, nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRemoteIP(tt.peerMap, tt.groupMap)
			assert.Equal(t, tt.wantIP, got)
		})
	}
}

// TestExtractPeerRoleConfigs_NamedPeerKeyedByIP verifies named peers are keyed by IP.
//
// VALIDATES: extractPeerRoleConfigs keys configs by remote.ip, not peer name.
// PREVENTS: Filter lookups by IP missing config stored by name.
func TestExtractPeerRoleConfigs_NamedPeerKeyedByIP(t *testing.T) {
	jsonStr := `{"bgp":{"peer":{"my-upstream":{"remote":{"ip":"10.0.0.1"},"role":{"import":"provider"}}}}}`
	configs, nameToIP := extractPeerRoleConfigs(jsonStr)

	// Config should be keyed by IP, not name.
	require.Contains(t, configs, "10.0.0.1", "config should be keyed by IP")
	assert.Nil(t, configs["my-upstream"], "config should NOT be keyed by name")
	assert.Equal(t, "provider", configs["10.0.0.1"].role)

	// Name-to-IP mapping should be populated.
	require.Contains(t, nameToIP, "my-upstream")
	assert.Equal(t, "10.0.0.1", nameToIP["my-upstream"])
}

// TestNameToIPResolution verifies setFilterRemoteRole resolves names to IPs.
//
// VALIDATES: setFilterRemoteRole uses filterNameToIP to convert peer name to IP.
// PREVENTS: Remote role stored by name when filters look up by IP.
func TestNameToIPResolution(t *testing.T) {
	// Set up name-to-IP mapping.
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider},
	}, map[string]string{"my-upstream": "10.0.0.1"}, 0)
	defer setFilterState(nil, nil, 0)

	// Store remote role by NAME (as OnValidateOpen does).
	setFilterRemoteRole("my-upstream", roleCustomer)
	defer func() {
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	}()

	// Look up by IP (as filters do).
	cfg, remoteRole := getFilterConfig("10.0.0.1")
	require.NotNil(t, cfg, "config should be found by IP")
	assert.Equal(t, roleProvider, cfg.role)
	assert.Equal(t, roleCustomer, remoteRole, "remote role should be found by IP after name resolution")
}

// VALIDATES: extractLocalASN parses local-as from BGP config JSON.
// PREVENTS: Wrong ASN used for OTC egress stamping.
func TestExtractLocalASN(t *testing.T) {
	tests := []struct {
		name string
		json string
		want uint32
	}{
		{"valid_float64", `{"bgp":{"local-as":65001}}`, 65001},
		{"max_asn", `{"bgp":{"local-as":4294967295}}`, 4294967295},
		{"zero", `{"bgp":{"local-as":0}}`, 0},
		{"missing_key", `{"bgp":{}}`, 0},
		{"string_value", `{"bgp":{"local-as":"not-a-number"}}`, 0},
		{"invalid_json", `not json`, 0},
		{"empty_json", `{}`, 0},
		{"negative", `{"bgp":{"local-as":-1}}`, 0},
		{"overflow", `{"bgp":{"local-as":4294967296}}`, 0},
		{"very_large", `{"bgp":{"local-as":5000000000}}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractLocalASN(tt.json))
		})
	}
}
