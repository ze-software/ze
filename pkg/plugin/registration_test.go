package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseRFCAdd verifies parsing of "declare rfc <number>" command.
//
// VALIDATES: RFC numbers are parsed correctly for human-readable feature tracking.
// PREVENTS: Plugin registration failures due to RFC command parsing errors.
func TestParseRFCAdd(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantRFC uint16
		wantErr bool
	}{
		{
			name:    "basic_rfc_4271",
			input:   "declare rfc 4271",
			wantRFC: 4271,
		},
		{
			name:    "rfc_9234",
			input:   "declare rfc 9234",
			wantRFC: 9234,
		},
		{
			name:    "missing_number",
			input:   "declare rfc",
			wantErr: true,
		},
		{
			name:    "invalid_number",
			input:   "declare rfc notanumber",
			wantErr: true,
		},
		{
			name:    "negative_number",
			input:   "declare rfc -1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &PluginRegistration{}
			err := reg.ParseLine(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, reg.RFCs, tt.wantRFC)
		})
	}
}

// TestParseEncodingAdd verifies parsing of "declare encoding <enc>" command.
//
// VALIDATES: Supported encodings (text, b64, hex) are tracked per plugin.
// PREVENTS: Invalid encoding names being accepted.
func TestParseEncodingAdd(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantEncoding string
		wantErr      bool
	}{
		{
			name:         "text_encoding",
			input:        "declare encoding text",
			wantEncoding: "text",
		},
		{
			name:         "b64_encoding",
			input:        "declare encoding b64",
			wantEncoding: "b64",
		},
		{
			name:         "hex_encoding",
			input:        "declare encoding hex",
			wantEncoding: "hex",
		},
		{
			name:    "missing_encoding",
			input:   "declare encoding",
			wantErr: true,
		},
		{
			name:    "invalid_encoding",
			input:   "declare encoding xml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &PluginRegistration{}
			err := reg.ParseLine(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, reg.Encodings, tt.wantEncoding)
		})
	}
}

// TestParseFamilyAdd verifies parsing of "declare family <afi> <safi>" command.
//
// VALIDATES: Address families are parsed and tracked for update delivery.
// PREVENTS: Plugin missing updates due to family registration errors.
func TestParseFamilyAdd(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFamily string
		wantErr    bool
	}{
		{
			name:       "ipv4_unicast",
			input:      "declare family ipv4 unicast",
			wantFamily: "ipv4/unicast",
		},
		{
			name:       "ipv6_unicast",
			input:      "declare family ipv6 unicast",
			wantFamily: "ipv6/unicast",
		},
		{
			name:       "all_families",
			input:      "declare family all",
			wantFamily: "all",
		},
		{
			name:    "missing_safi",
			input:   "declare family ipv4",
			wantErr: true,
		},
		{
			name:    "invalid_afi",
			input:   "declare family ipv12 unicast",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &PluginRegistration{}
			err := reg.ParseLine(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, reg.Families, tt.wantFamily)
		})
	}
}

// TestParseConfigPattern verifies parsing of "declare conf <pattern>" command.
//
// VALIDATES: Config patterns with globs and regex captures are parsed correctly.
// PREVENTS: Invalid patterns causing startup failures.
func TestParseConfigPattern(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPattern string
		wantErr     bool
	}{
		{
			name:        "hostname_pattern",
			input:       "declare conf peer * capability hostname <hostname:.*>",
			wantPattern: "peer * capability hostname <hostname:.*>",
		},
		{
			name:        "graceful_restart_pattern",
			input:       "declare conf peer * capability graceful-restart <restart-time:\\d+>",
			wantPattern: "peer * capability graceful-restart <restart-time:\\d+>",
		},
		{
			name:    "missing_pattern",
			input:   "declare conf",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &PluginRegistration{}
			err := reg.ParseLine(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, reg.ConfigPatterns, 1)
			assert.Equal(t, tt.wantPattern, reg.ConfigPatterns[0].Pattern)
		})
	}
}

// TestParseCommandAdd verifies parsing of "declare cmd <command>" command.
//
// VALIDATES: Commands are registered for routing to plugins.
// PREVENTS: Command conflict detection failures.
func TestParseCommandAdd(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantCmd string
		wantErr bool
	}{
		{
			name:    "rib_show_command",
			input:   "declare cmd rib adjacent in show",
			wantCmd: "rib adjacent in show",
		},
		{
			name:    "peer_refresh_command",
			input:   "declare cmd peer * refresh",
			wantCmd: "peer * refresh",
		},
		{
			name:    "missing_command",
			input:   "declare cmd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &PluginRegistration{}
			err := reg.ParseLine(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, reg.Commands, tt.wantCmd)
		})
	}
}

// TestParseRegistrationDone verifies "declare done" signals completion.
//
// VALIDATES: Stage 1 completion is signaled correctly.
// PREVENTS: Startup hangs waiting for registration.
func TestParseRegistrationDone(t *testing.T) {
	reg := &PluginRegistration{}

	// Add some registrations first
	require.NoError(t, reg.ParseLine("declare rfc 4271"))
	require.NoError(t, reg.ParseLine("declare encoding text"))
	require.NoError(t, reg.ParseLine("declare family ipv4 unicast"))

	// Then signal done
	err := reg.ParseLine("declare done")
	require.NoError(t, err)
	assert.True(t, reg.Done)
}

// TestParseCapabilitySet verifies "capability <enc> <code> <payload>" command.
//
// VALIDATES: Plugin capability bytes are captured for OPEN message injection.
// PREVENTS: Malformed capability declarations.
func TestParseCapabilitySet(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCode    uint8
		wantPayload string
		wantErr     bool
	}{
		{
			name:        "hostname_capability",
			input:       "capability b64 73 cm91dGVyMS5leGFtcGxlLmNvbQ==",
			wantCode:    73,
			wantPayload: "cm91dGVyMS5leGFtcGxlLmNvbQ==",
		},
		{
			name:        "gr_capability",
			input:       "capability b64 64 AAAA",
			wantCode:    64,
			wantPayload: "AAAA",
		},
		{
			name:        "empty_payload",
			input:       "capability b64 64",
			wantCode:    64,
			wantPayload: "", // Empty payload is valid (RFC 2918 route-refresh).
		},
		{
			name:    "invalid_code",
			input:   "capability b64 abc AAAA",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := &PluginCapabilities{}
			err := caps.ParseLine(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, caps.Capabilities, 1)
			assert.Equal(t, tt.wantCode, caps.Capabilities[0].Code)
			assert.Equal(t, tt.wantPayload, caps.Capabilities[0].Payload)
		})
	}
}

// TestParseCapabilityDone verifies "capability done" signals capability stage completion.
//
// VALIDATES: Stage 3 completion is signaled correctly.
// PREVENTS: Startup hangs waiting for capabilities.
func TestParseCapabilityDone(t *testing.T) {
	caps := &PluginCapabilities{}

	err := caps.ParseLine("capability done")
	require.NoError(t, err)
	assert.True(t, caps.Done)
}

// TestConfigPatternMatching verifies config patterns match correctly.
//
// VALIDATES: Glob wildcards and regex captures work as specified.
// PREVENTS: Config delivery to wrong plugins or missing captures.
func TestConfigPatternMatching(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		config     string
		wantMatch  bool
		wantValues map[string]string
	}{
		{
			name:       "hostname_match",
			pattern:    "peer * capability hostname <hostname:.*>",
			config:     "peer 192.168.1.1 capability hostname router1.example.com",
			wantMatch:  true,
			wantValues: map[string]string{"hostname": "router1.example.com"},
		},
		{
			name:      "no_match_wrong_path",
			pattern:   "peer * capability hostname <hostname:.*>",
			config:    "peer 192.168.1.1 capability graceful-restart 120",
			wantMatch: false,
		},
		{
			name:       "restart_time_match",
			pattern:    "peer * capability graceful-restart <restart-time:\\d+>",
			config:     "peer 192.168.1.1 capability graceful-restart 120",
			wantMatch:  true,
			wantValues: map[string]string{"restart-time": "120"},
		},
		{
			name:       "multiple_captures",
			pattern:    "peer * capability graceful-restart <restart-time:\\d+> <forwarding:(true|false)>",
			config:     "peer 192.168.1.1 capability graceful-restart 120 true",
			wantMatch:  true,
			wantValues: map[string]string{"restart-time": "120", "forwarding": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pat, err := CompileConfigPattern(tt.pattern)
			require.NoError(t, err)

			match := pat.Match(tt.config)
			if !tt.wantMatch {
				assert.Nil(t, match)
				return
			}
			require.NotNil(t, match)
			for k, v := range tt.wantValues {
				assert.Equal(t, v, match.Captures[k], "capture %s", k)
			}
		})
	}
}

// TestConflictDetection verifies command/capability conflict detection.
//
// VALIDATES: Duplicate registrations are rejected at startup.
// PREVENTS: Silent command/capability overwrites.
func TestConflictDetection(t *testing.T) {
	reg := NewPluginRegistry()

	// First plugin registers a command
	plugin1 := &PluginRegistration{
		Name:     "plugin1",
		Commands: []string{"rib show"},
	}
	require.NoError(t, reg.Register(plugin1))

	// Second plugin tries same command - should fail
	plugin2 := &PluginRegistration{
		Name:     "plugin2",
		Commands: []string{"rib show"},
	}
	err := reg.Register(plugin2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command conflict")
	assert.Contains(t, err.Error(), "rib show")
}

// TestCapabilityConflictDetection verifies capability type conflict detection.
//
// VALIDATES: Duplicate capability codes are rejected.
// PREVENTS: Two plugins sending conflicting OPEN capabilities.
func TestCapabilityConflictDetection(t *testing.T) {
	reg := NewPluginRegistry()

	// First plugin registers capability 73 (hostname)
	caps1 := &PluginCapabilities{
		PluginName: "plugin1",
		Capabilities: []PluginCapability{
			{Code: 73, Encoding: "b64", Payload: "dGVzdA=="},
		},
	}
	require.NoError(t, reg.RegisterCapabilities(caps1))

	// Second plugin tries same capability code - should fail
	caps2 := &PluginCapabilities{
		PluginName: "plugin2",
		Capabilities: []PluginCapability{
			{Code: 73, Encoding: "b64", Payload: "b3RoZXI="},
		},
	}
	err := reg.RegisterCapabilities(caps2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capability conflict")
	assert.Contains(t, err.Error(), "73")
}
