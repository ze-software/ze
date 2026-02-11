package ipc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMethodNameParsing verifies module:rpc-name method parsing.
//
// VALIDATES: ParseMethod splits method string into module and RPC name.
// PREVENTS: Method dispatch failing due to incorrect parsing of colon separator.
func TestMethodNameParsing(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantModule string
		wantRPC    string
	}{
		{
			name:       "bgp_peer_list",
			method:     "ze-bgp:peer-list",
			wantModule: "ze-bgp",
			wantRPC:    "peer-list",
		},
		{
			name:       "system_daemon_status",
			method:     "ze-system:daemon-status",
			wantModule: "ze-system",
			wantRPC:    "daemon-status",
		},
		{
			name:       "system_version",
			method:     "ze-system:version",
			wantModule: "ze-system",
			wantRPC:    "version",
		},
		{
			name:       "system_api_module",
			method:     "ze-system-api:command-list",
			wantModule: "ze-system-api",
			wantRPC:    "command-list",
		},
		{
			name:       "plugin_api_module",
			method:     "ze-plugin-api:session-ready",
			wantModule: "ze-plugin-api",
			wantRPC:    "session-ready",
		},
		{
			name:       "rib_api_module",
			method:     "ze-rib-api:show-in",
			wantModule: "ze-rib-api",
			wantRPC:    "show-in",
		},
		{
			name:       "single_word_rpc",
			method:     "ze-bgp:subscribe",
			wantModule: "ze-bgp",
			wantRPC:    "subscribe",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			module, rpc, err := ParseMethod(tc.method)
			require.NoError(t, err)
			assert.Equal(t, tc.wantModule, module)
			assert.Equal(t, tc.wantRPC, rpc)
		})
	}
}

// TestMethodNameValidation verifies invalid method names are rejected.
//
// VALIDATES: ParseMethod rejects empty, missing-colon, and malformed methods.
// PREVENTS: Panic or silent misbehavior on invalid IPC method strings.
func TestMethodNameValidation(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"empty_string", ""},
		{"no_colon", "ze-bgp-peer-list"},
		{"empty_module", ":peer-list"},
		{"empty_rpc", "ze-bgp:"},
		{"double_colon", "ze-bgp::peer-list"},
		{"spaces_in_module", "ze bgp:peer-list"},
		{"spaces_in_rpc", "ze-bgp:peer list"},
		{"multiple_colons", "ze-bgp:peer:list"},
		{"too_long", string(make([]byte, 257))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ParseMethod(tc.method)
			require.Error(t, err, "should reject: %q", tc.method)
		})
	}
}

// TestMethodNameBoundary verifies method name length limits.
//
// VALIDATES: Method names at max length (256) succeed, above fail.
// PREVENTS: Off-by-one in length validation.
// BOUNDARY: 256 is last valid, 257 is first invalid.
func TestMethodNameBoundary(t *testing.T) {
	// Build a valid method at exactly 256 chars: "module:rpc..."
	// "m:" is 2 chars, so rpc part needs to be 254 chars
	module := "m"
	rpc := make([]byte, 254)
	for i := range rpc {
		rpc[i] = 'a'
	}
	atLimit := module + ":" + string(rpc)
	require.Len(t, atLimit, 256)

	mod, rpcName, err := ParseMethod(atLimit)
	require.NoError(t, err, "256-char method should succeed")
	assert.Equal(t, module, mod)
	assert.Equal(t, string(rpc), rpcName)

	// 257 chars should fail
	overLimit := atLimit + "x"
	_, _, err = ParseMethod(overLimit)
	require.Error(t, err, "257-char method should fail")
}

// TestFormatMethod verifies method string construction.
//
// VALIDATES: FormatMethod produces valid module:rpc-name strings.
// PREVENTS: Method string construction producing unparseable results.
func TestFormatMethod(t *testing.T) {
	tests := []struct {
		name   string
		module string
		rpc    string
		want   string
	}{
		{"bgp_peer_list", "ze-bgp", "peer-list", "ze-bgp:peer-list"},
		{"system_version", "ze-system", "version", "ze-system:version"},
		{"plugin_ready", "ze-plugin-api", "session-ready", "ze-plugin-api:session-ready"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatMethod(tc.module, tc.rpc)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestMethodRoundTrip verifies parse then format produces the original string.
//
// VALIDATES: FormatMethod(ParseMethod(s)) == s for valid methods.
// PREVENTS: Data loss in method name conversions.
func TestMethodRoundTrip(t *testing.T) {
	methods := []string{
		"ze-bgp:peer-list",
		"ze-system-api:command-list",
		"ze-rib-api:show-in",
		"ze-plugin-api:session-ready",
		"ze-bgp:subscribe",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			module, rpc, err := ParseMethod(method)
			require.NoError(t, err)
			got := FormatMethod(module, rpc)
			assert.Equal(t, method, got)
		})
	}
}
