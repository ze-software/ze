package gr

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGRPlugin_ParseConfigLine verifies config line parsing.
//
// VALIDATES: Config lines in format "config peer <addr> restart-time <value>" are parsed.
// PREVENTS: Config being silently ignored, causing missing GR capability.
func TestGRPlugin_ParseConfigLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantPeer   string
		wantTime   uint16
		wantParsed bool
	}{
		{
			name:       "valid_restart_time_120",
			line:       "config peer 192.168.1.1 restart-time 120",
			wantPeer:   "192.168.1.1",
			wantTime:   120,
			wantParsed: true,
		},
		{
			name:       "valid_restart_time_zero",
			line:       "config peer 10.0.0.1 restart-time 0",
			wantPeer:   "10.0.0.1",
			wantTime:   0,
			wantParsed: true,
		},
		{
			name:       "valid_restart_time_max_4095",
			line:       "config peer 127.0.0.1 restart-time 4095",
			wantPeer:   "127.0.0.1",
			wantTime:   4095,
			wantParsed: true,
		},
		{
			name:       "clamped_above_max_4096",
			line:       "config peer 127.0.0.1 restart-time 4096",
			wantPeer:   "127.0.0.1",
			wantTime:   4095, // Clamped to max
			wantParsed: true,
		},
		{
			name:       "clamped_above_max_65535",
			line:       "config peer 127.0.0.1 restart-time 65535",
			wantPeer:   "127.0.0.1",
			wantTime:   4095, // Clamped to max
			wantParsed: true,
		},
		{
			name:       "ignore_non_peer_config",
			line:       "config global some-setting value",
			wantParsed: false,
		},
		{
			name:       "ignore_other_capability",
			line:       "config peer 192.168.1.1 some-other-cap value",
			wantParsed: false,
		},
		{
			name:       "malformed_too_few_parts",
			line:       "config peer 192.168.1.1",
			wantParsed: false,
		},
		{
			name:       "invalid_value_not_a_number",
			line:       "config peer 192.168.1.1 restart-time abc",
			wantParsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GRPlugin{grConfig: make(map[string]uint16)}
			g.parseConfigLine(tt.line)

			if tt.wantParsed {
				require.Contains(t, g.grConfig, tt.wantPeer, "peer should be in grConfig")
				assert.Equal(t, tt.wantTime, g.grConfig[tt.wantPeer], "restart-time mismatch")
			} else {
				assert.Empty(t, g.grConfig, "grConfig should be empty for ignored/invalid lines")
			}
		})
	}
}

// TestGRPlugin_RegisterCapabilities verifies capability registration output.
//
// VALIDATES: Capability hex output matches RFC 4724 wire format.
// PREVENTS: Malformed GR capability causing OPEN rejection.
func TestGRPlugin_RegisterCapabilities(t *testing.T) {
	tests := []struct {
		name         string
		grConfig     map[string]uint16
		wantContains []string
	}{
		{
			name: "single_peer_120",
			grConfig: map[string]uint16{
				"192.168.1.1": 120,
			},
			// RFC 4724: restart-time 120 = 0x0078
			wantContains: []string{"capability hex 64 0078 peer 192.168.1.1"},
		},
		{
			name: "single_peer_max_4095",
			grConfig: map[string]uint16{
				"10.0.0.1": 4095,
			},
			// RFC 4724: restart-time 4095 = 0x0FFF
			wantContains: []string{"capability hex 64 0fff peer 10.0.0.1"},
		},
		{
			name: "single_peer_zero",
			grConfig: map[string]uint16{
				"127.0.0.1": 0,
			},
			// RFC 4724: restart-time 0 = 0x0000
			wantContains: []string{"capability hex 64 0000 peer 127.0.0.1"},
		},
		{
			name:         "empty_config",
			grConfig:     map[string]uint16{},
			wantContains: []string{"capability done"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			g := &GRPlugin{
				output:   &buf,
				grConfig: tt.grConfig,
			}

			g.registerCapabilities()
			output := buf.String()

			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "output should contain: %s", want)
			}
			assert.Contains(t, output, "capability done", "output must end with capability done")
		})
	}
}

// TestGRPlugin_CapabilityWireFormat verifies RFC 4724 wire encoding.
//
// VALIDATES: Restart-time value is correctly encoded as 12-bit big-endian.
// PREVENTS: Byte order errors or bit-shift mistakes in capability encoding.
// BOUNDARY: Tests 0 (min), 4095 (max 12-bit), intermediate values.
func TestGRPlugin_CapabilityWireFormat(t *testing.T) {
	tests := []struct {
		name        string
		restartTime uint16
		wantHex     string
	}{
		// RFC 4724: [Restart Flags:4 bits][Restart Time:12 bits] = 2 bytes
		// Flags = 0, so just the restart time in lower 12 bits
		{"zero", 0, "0000"},
		{"one", 1, "0001"},
		{"120_default", 120, "0078"},       // Common default
		{"255_byte_boundary", 255, "00ff"}, // 0xFF
		{"256_byte_boundary", 256, "0100"}, // 0x100
		{"4095_max", 4095, "0fff"},         // Max 12-bit value
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			g := &GRPlugin{
				output: &buf,
				grConfig: map[string]uint16{
					"192.168.1.1": tt.restartTime,
				},
			}

			g.registerCapabilities()
			output := buf.String()

			wantLine := "capability hex 64 " + tt.wantHex + " peer 192.168.1.1"
			assert.Contains(t, output, wantLine, "wire format mismatch")
		})
	}
}

// TestGRPlugin_StartupProtocol verifies the 5-stage startup sequence.
//
// VALIDATES: Plugin sends correct declarations and waits for expected markers.
// PREVENTS: Protocol handshake failures blocking plugin startup.
func TestGRPlugin_StartupProtocol(t *testing.T) {
	// Simulate engine input
	input := strings.NewReader(`config peer 192.168.1.1 restart-time 120
config done
registry done
`)

	var output bytes.Buffer
	g := NewGRPlugin(input, &output)

	// Run startup protocol only (not event loop - that would block)
	g.doStartupProtocol()

	out := output.String()

	// Stage 1: Declaration
	assert.Contains(t, out, "declare conf peer * capability graceful-restart:restart-time <restart-time:\\d+>")
	assert.Contains(t, out, "declare done")

	// Stage 3: Capability registration
	assert.Contains(t, out, "capability hex 64 0078 peer 192.168.1.1")
	assert.Contains(t, out, "capability done")

	// Stage 5: Ready
	assert.Contains(t, out, "ready")

	// Verify config was parsed
	assert.Equal(t, uint16(120), g.grConfig["192.168.1.1"])
}
