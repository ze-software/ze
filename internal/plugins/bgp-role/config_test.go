package bgp_role

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			json:        `{"bgp":{"peer":{"192.168.1.1":{"capability":{"role":{"role":"provider"}}}}}}`,
			wantPeer:    "192.168.1.1",
			wantPayload: "00",
			wantParsed:  true,
		},
		{
			name:        "rs_role_1",
			json:        `{"bgp":{"peer":{"10.0.0.1":{"capability":{"role":{"role":"rs"}}}}}}`,
			wantPeer:    "10.0.0.1",
			wantPayload: "01",
			wantParsed:  true,
		},
		{
			name:        "rs_client_role_2",
			json:        `{"bgp":{"peer":{"10.0.0.2":{"capability":{"role":{"role":"rs-client"}}}}}}`,
			wantPeer:    "10.0.0.2",
			wantPayload: "02",
			wantParsed:  true,
		},
		{
			name:        "customer_role_3",
			json:        `{"bgp":{"peer":{"10.0.0.3":{"capability":{"role":{"role":"customer"}}}}}}`,
			wantPeer:    "10.0.0.3",
			wantPayload: "03",
			wantParsed:  true,
		},
		{
			name:        "peer_role_4",
			json:        `{"bgp":{"peer":{"10.0.0.4":{"capability":{"role":{"role":"peer"}}}}}}`,
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
			json:       `{"bgp":{"peer":{"192.168.1.1":{"capability":{"role":{"role":"invalid"}}}}}}`,
			wantParsed: false,
		},
		{
			name:       "empty_role_name",
			json:       `{"bgp":{"peer":{"192.168.1.1":{"capability":{"role":{"role":""}}}}}}`,
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
				assert.Equal(t, "hex", cap.Encoding, "encoding must be hex")
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
		"192.168.1.1":{"capability":{"role":{"role":"customer"}}},
		"10.0.0.1":{"capability":{"role":{"role":"provider"}}}
	}}}`

	caps := extractRoleCapabilities(json)

	require.Len(t, caps, 2, "should return one capability per peer")

	peerPayload := make(map[string]string)
	for _, cap := range caps {
		assert.Equal(t, uint8(roleCapCode), cap.Code)
		assert.Equal(t, "hex", cap.Encoding)
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
	strictJSON := `{"bgp":{"peer":{"10.0.0.1":{"capability":{"role":{"role":"customer","role-strict":true}}}}}}`
	configs := extractPeerRoleConfigs(strictJSON)
	require.Contains(t, configs, "10.0.0.1")
	assert.True(t, configs["10.0.0.1"].strict, "strict should be true when role-strict is true")

	normalJSON := `{"bgp":{"peer":{"10.0.0.1":{"capability":{"role":{"role":"customer"}}}}}}`
	configs = extractPeerRoleConfigs(normalJSON)
	require.Contains(t, configs, "10.0.0.1")
	assert.False(t, configs["10.0.0.1"].strict, "strict should be false when role-strict is absent")
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
			json:       `{"bgp":{"peer":{"10.0.0.1":{"capability":{"role":{"role":"customer","role-strict":true}}}}}}`,
			wantStrict: true,
			wantRole:   "customer",
		},
		{
			name:       "strict_false_explicit",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"capability":{"role":{"role":"provider","role-strict":false}}}}}}`,
			wantStrict: false,
			wantRole:   "provider",
		},
		{
			name:       "strict_absent_defaults_false",
			json:       `{"bgp":{"peer":{"10.0.0.1":{"capability":{"role":{"role":"peer"}}}}}}`,
			wantStrict: false,
			wantRole:   "peer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs := extractPeerRoleConfigs(tt.json)
			require.Len(t, configs, 1, "should return exactly one peer config")

			cfg := configs["10.0.0.1"]
			require.NotNil(t, cfg, "should have config for 10.0.0.1")
			assert.Equal(t, tt.wantRole, cfg.role, "role mismatch")
			assert.Equal(t, tt.wantStrict, cfg.strict, "strict mode mismatch")
		})
	}
}
