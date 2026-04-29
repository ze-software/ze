package route_refresh

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunDecodeMode verifies the IPC decode protocol for Route Refresh.
//
// VALIDATES: RunDecodeMode parses IPC decode lines and returns correct JSON/text.
// PREVENTS: Broken IPC decode for codes 2 (route-refresh) and 70 (enhanced-route-refresh).
func TestRunDecodeMode(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "route_refresh_json",
			input:  "decode capability 2 \n",
			expect: "decoded json {\"name\":\"route-refresh\"}\n",
		},
		{
			name:   "route_refresh_text",
			input:  "decode text capability 2 \n",
			expect: "decoded text route-refresh       \n",
		},
		{
			name:   "enhanced_route_refresh_json",
			input:  "decode capability 70 \n",
			expect: "decoded json {\"name\":\"enhanced-route-refresh\"}\n",
		},
		{
			name:   "enhanced_route_refresh_text",
			input:  "decode text capability 70 \n",
			expect: "decoded text enhanced-route-refresh\n",
		},
		{
			name:   "explicit_json_format",
			input:  "decode json capability 2 \n",
			expect: "decoded json {\"name\":\"route-refresh\"}\n",
		},
		{
			name:   "unknown_code",
			input:  "decode capability 99 \n",
			expect: "decoded unknown\n",
		},
		{
			name:   "not_capability",
			input:  "decode update 2 \n",
			expect: "decoded unknown\n",
		},
		{
			name:   "empty_line_skipped",
			input:  "\ndecode capability 2 \n",
			expect: "decoded json {\"name\":\"route-refresh\"}\n",
		},
		{
			name:   "too_few_fields",
			input:  "decode capability\n",
			expect: "decoded unknown\n",
		},
		{
			name:   "not_decode_command",
			input:  "something else 2 00\n",
			expect: "decoded unknown\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			var output bytes.Buffer
			code := RunDecodeMode(input, &output)
			assert.Equal(t, 0, code, "exit code should be 0")
			assert.Equal(t, tt.expect, output.String())
		})
	}
}

// TestRunCLIDecode verifies CLI decode for route-refresh.
//
// VALIDATES: RunCLIDecode returns correct JSON/text for route-refresh.
// PREVENTS: Broken CLI decode output formatting.
func TestRunCLIDecode(t *testing.T) {
	tests := []struct {
		name       string
		capCode    uint8
		hexData    string
		textOutput bool
		wantOut    string
		wantErr    string
		wantCode   int
	}{
		{
			name:       "json_output",
			capCode:    2,
			hexData:    "",
			textOutput: false,
			wantOut:    `{"code":2,"name":"route-refresh"}` + "\n",
			wantCode:   0,
		},
		{
			name:       "text_output",
			capCode:    2,
			hexData:    "",
			textOutput: true,
			wantOut:    "route-refresh       \n",
			wantCode:   0,
		},
		{
			name:       "hex_data_ignored",
			capCode:    2,
			hexData:    "DEADBEEF",
			textOutput: false,
			wantOut:    `{"code":2,"name":"route-refresh"}` + "\n",
			wantCode:   0,
		},
		{
			name:       "enhanced_route_refresh_json",
			capCode:    70,
			hexData:    "",
			textOutput: false,
			wantOut:    `{"code":70,"name":"enhanced-route-refresh"}` + "\n",
			wantCode:   0,
		},
		{
			name:       "enhanced_route_refresh_text",
			capCode:    70,
			hexData:    "",
			textOutput: true,
			wantOut:    "enhanced-route-refresh\n",
			wantCode:   0,
		},
		{
			name:       "unknown_code",
			capCode:    99,
			hexData:    "",
			textOutput: false,
			wantErr:    "error: unsupported route-refresh capability code 99\n",
			wantCode:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunCLIDecodeWithCode(tt.capCode, tt.hexData, tt.textOutput, &stdout, &stderr)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantOut, stdout.String())
			assert.Equal(t, tt.wantErr, stderr.String())
		})
	}
}

func TestRunCLIDecodeDefaultsToRouteRefresh(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCLIDecode("", false, &stdout, &stderr)
	assert.Equal(t, 0, code)
	assert.Equal(t, `{"code":2,"name":"route-refresh"}`+"\n", stdout.String())
	assert.Empty(t, stderr.String())
}

// TestGetYANG verifies YANG schema is embedded.
//
// VALIDATES: GetYANG returns non-empty YANG containing expected module.
// PREVENTS: Missing or incorrect YANG embedding.
func TestGetYANG(t *testing.T) {
	yang := GetYANG()
	require.NotEmpty(t, yang)
	assert.Contains(t, yang, "module ze-route-refresh")
	assert.Contains(t, yang, "route-refresh")
}
