package role

import (
	"bytes"
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

// TestRoleValidPairs verifies RFC 9234 Table 2 valid role pair matching.
//
// VALIDATES: All 5 valid local↔peer role pairs are accepted.
// PREVENTS: False rejection of valid BGP sessions.
func TestRoleValidPairs(t *testing.T) {
	// RFC 9234 Table 2: valid pairs
	validPairs := []struct {
		local string
		peer  string
	}{
		{"provider", "customer"},
		{"customer", "provider"},
		{"rs", "rs-client"},
		{"rs-client", "rs"},
		{"peer", "peer"},
	}

	for _, tt := range validPairs {
		t.Run(tt.local+"_"+tt.peer, func(t *testing.T) {
			assert.True(t, validRolePair(tt.local, tt.peer),
				"pair (%s, %s) should be valid", tt.local, tt.peer)
		})
	}
}

// TestRoleInvalidPairs verifies RFC 9234 Table 2 invalid role pair rejection.
//
// VALIDATES: All 20 invalid local↔peer role pairs are rejected.
// PREVENTS: Accepting sessions with mismatched roles (route leak risk).
func TestRoleInvalidPairs(t *testing.T) {
	allRoles := []string{"provider", "rs", "rs-client", "customer", "peer"}

	// Valid pairs to exclude
	valid := map[[2]string]bool{
		{"provider", "customer"}: true,
		{"customer", "provider"}: true,
		{"rs", "rs-client"}:      true,
		{"rs-client", "rs"}:      true,
		{"peer", "peer"}:         true,
	}

	for _, local := range allRoles {
		for _, peer := range allRoles {
			if valid[[2]string{local, peer}] {
				continue
			}
			t.Run(local+"_"+peer, func(t *testing.T) {
				assert.False(t, validRolePair(local, peer),
					"pair (%s, %s) should be invalid", local, peer)
			})
		}
	}
}

// TestRoleValueBoundary verifies role value encoding boundaries.
//
// VALIDATES: Role values 0-4 are valid, values outside range are rejected.
// PREVENTS: Off-by-one in role value validation.
// BOUNDARY: 4 (last valid), 5 (first invalid above).
func TestRoleValueBoundary(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		want   string
		wantOK bool
	}{
		{"provider_0", 0, "provider", true},
		{"rs_1", 1, "rs", true},
		{"rs_client_2", 2, "rs-client", true},
		{"customer_3", 3, "customer", true},
		{"peer_4", 4, "peer", true},
		{"invalid_5", 5, "", false},
		{"invalid_255", 255, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := roleValueToName(tt.value)
			assert.Equal(t, tt.wantOK, ok, "valid flag mismatch")
			if tt.wantOK {
				assert.Equal(t, tt.want, got, "role name mismatch")
			}
		})
	}
}

// TestRoleNameToValue verifies role name to wire value mapping.
//
// VALIDATES: All role name strings map to correct wire values.
// PREVENTS: Wrong capability value being sent in OPEN.
func TestRoleNameToValue(t *testing.T) {
	tests := []struct {
		name   string
		want   uint8
		wantOK bool
	}{
		{"provider", 0, true},
		{"rs", 1, true},
		{"rs-client", 2, true},
		{"customer", 3, true},
		{"peer", 4, true},
		{"invalid", 0, false},
		{"", 0, false},
		{"Provider", 0, false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := roleNameToValue(tt.name)
			assert.Equal(t, tt.wantOK, ok, "valid flag mismatch for %q", tt.name)
			if tt.wantOK {
				assert.Equal(t, tt.want, got, "value mismatch for %q", tt.name)
			}
		})
	}
}

// TestRunCLIDecode verifies CLI decode mode for Role capability.
//
// VALIDATES: RunCLIDecode correctly parses Role capability hex and outputs JSON/text.
// PREVENTS: CLI decode returning wrong format or failing on valid input.
func TestRunCLIDecode(t *testing.T) {
	tests := []struct {
		name         string
		hexInput     string
		textOutput   bool
		wantExitCode int
		wantContains []string
		wantErr      string
	}{
		{
			name:         "json_provider_0",
			hexInput:     "00",
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"name":"role"`, `"value":"provider"`},
		},
		{
			name:         "json_rs_1",
			hexInput:     "01",
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"value":"rs"`},
		},
		{
			name:         "json_rs_client_2",
			hexInput:     "02",
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"value":"rs-client"`},
		},
		{
			name:         "json_customer_3",
			hexInput:     "03",
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"value":"customer"`},
		},
		{
			name:         "json_peer_4",
			hexInput:     "04",
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"value":"peer"`},
		},
		{
			name:         "json_unknown_5",
			hexInput:     "05",
			textOutput:   false,
			wantExitCode: 0,
			wantContains: []string{`"value":"unknown(5)"`},
		},
		{
			name:         "text_customer",
			hexInput:     "03",
			textOutput:   true,
			wantExitCode: 0,
			wantContains: []string{"role", "customer"},
		},
		{
			name:         "invalid_hex",
			hexInput:     "ZZZZ",
			textOutput:   false,
			wantExitCode: 1,
			wantErr:      "invalid hex",
		},
		{
			name:         "empty_input",
			hexInput:     "",
			textOutput:   false,
			wantExitCode: 1,
			wantErr:      "empty",
		},
		{
			name:         "too_long",
			hexInput:     "0001",
			textOutput:   false,
			wantExitCode: 1,
			wantErr:      "length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := RunCLIDecode(tt.hexInput, tt.textOutput, &stdout, &stderr)

			assert.Equal(t, tt.wantExitCode, exitCode, "exit code mismatch")

			if tt.wantErr != "" {
				assert.Contains(t, stderr.String(), tt.wantErr, "stderr should contain error")
				return
			}

			output := stdout.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "output should contain: %s", want)
			}
		})
	}
}

// TestDecodeRole verifies wire byte decoding of Role capability.
//
// VALIDATES: decodeRole correctly parses 1-byte role value.
// PREVENTS: Wrong role name returned for valid wire values.
// BOUNDARY: 0 (min valid), 4 (max valid), 5 (first invalid).
func TestDecodeRole(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    string
		wantErr bool
	}{
		{"provider", []byte{0x00}, "provider", false},
		{"rs", []byte{0x01}, "rs", false},
		{"rs-client", []byte{0x02}, "rs-client", false},
		{"customer", []byte{0x03}, "customer", false},
		{"peer", []byte{0x04}, "peer", false},
		{"unknown_5", []byte{0x05}, "unknown(5)", false},
		{"unknown_255", []byte{0xff}, "unknown(255)", false},
		{"empty", []byte{}, "", true},
		{"too_long", []byte{0x03, 0x00}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeRole(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
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
