package bgp_role

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
