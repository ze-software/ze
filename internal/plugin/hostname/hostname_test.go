// Package hostname implements hostname (FQDN) capability plugin for ze.
package hostname

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHostnamePluginParseConfig verifies config parsing.
//
// VALIDATES: Plugin parses "config json bgp <json>" correctly.
// PREVENTS: Config values being ignored or misassigned.
func TestHostnamePluginParseConfig(t *testing.T) {
	tests := []struct {
		name          string
		lines         []string
		wantHost      string
		wantDomain    string
		wantPeerCount int
	}{
		{
			name: "bgp_json_both",
			lines: []string{
				`config json bgp {"peer":{"192.168.1.1":{"capability":{"hostname":{"host":"router1","domain":"example.com"}}}}}`,
				"config done",
			},
			wantHost:      "router1",
			wantDomain:    "example.com",
			wantPeerCount: 1,
		},
		{
			name: "bgp_json_host_only",
			lines: []string{
				`config json bgp {"peer":{"10.0.0.1":{"capability":{"hostname":{"host":"myhost"}}}}}`,
				"config done",
			},
			wantHost:      "myhost",
			wantDomain:    "",
			wantPeerCount: 1,
		},
		{
			name: "bgp_json_domain_only",
			lines: []string{
				`config json bgp {"peer":{"10.0.0.1":{"capability":{"hostname":{"domain":"mydomain.net"}}}}}`,
				"config done",
			},
			wantHost:      "",
			wantDomain:    "mydomain.net",
			wantPeerCount: 1,
		},
		{
			name: "bgp_json_no_hostname",
			lines: []string{
				`config json bgp {"peer":{"10.0.0.1":{"capability":{}}}}`,
				"config done",
			},
			wantHost:      "",
			wantDomain:    "",
			wantPeerCount: 0,
		},
		{
			name: "bgp_json_no_peers",
			lines: []string{
				`config json bgp {"router-id":"1.2.3.4"}`,
				"config done",
			},
			wantHost:      "",
			wantDomain:    "",
			wantPeerCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.Join(tt.lines, "\n") + "\n"
			output := &bytes.Buffer{}

			plugin := NewHostnamePlugin(strings.NewReader(input), output)
			plugin.parseConfig()

			assert.Len(t, plugin.hostConfig, tt.wantPeerCount)

			if tt.wantPeerCount > 0 {
				// Get first peer (we know which one from the test)
				var peerAddr string
				for addr := range plugin.hostConfig {
					peerAddr = addr
					break
				}
				cfg := plugin.hostConfig[peerAddr]
				assert.Equal(t, tt.wantHost, cfg.hostname)
				assert.Equal(t, tt.wantDomain, cfg.domain)
			}
		})
	}
}

// TestHostnamePluginEncode verifies wire encoding.
//
// VALIDATES: Plugin generates correct hex capability bytes.
// PREVENTS: Wrong wire format breaking OPEN message.
func TestHostnamePluginEncode(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		domain   string
		wantHex  string // Capability value only (without code/length prefix)
	}{
		{
			name:     "both_values",
			hostname: "router1",
			domain:   "example.com",
			// hostname-len(1) + "router1"(7) + domain-len(1) + "example.com"(11)
			// 07 726f75746572 31 0b 6578616d706c652e636f6d
			wantHex: "07726f7574657231" + "0b" + "6578616d706c652e636f6d",
		},
		{
			name:     "host_only",
			hostname: "test",
			domain:   "",
			// 04 + "test" + 00
			wantHex: "0474657374" + "00",
		},
		{
			name:     "domain_only",
			hostname: "",
			domain:   "dom.net",
			// 00 + 07 + "dom.net"
			wantHex: "00" + "07" + "646f6d2e6e6574",
		},
		{
			name:     "empty_both",
			hostname: "",
			domain:   "",
			wantHex:  "0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &fqdnConfig{hostname: tt.hostname, domain: tt.domain}
			got := cfg.encodeValue()
			assert.Equal(t, tt.wantHex, got)
		})
	}
}

// TestHostnamePluginMultiplePeers verifies per-peer config handling.
//
// VALIDATES: Different peers get different hostname/domain values.
// PREVENTS: Config values leaking between peers.
func TestHostnamePluginMultiplePeers(t *testing.T) {
	input := strings.Join([]string{
		`config json bgp {"peer":{"192.168.1.1":{"capability":{"hostname":{"host":"router1","domain":"example.com"}}},"10.0.0.1":{"capability":{"hostname":{"host":"router2","domain":"test.org"}}}}}`,
		"config done",
	}, "\n") + "\n"
	output := &bytes.Buffer{}

	plugin := NewHostnamePlugin(strings.NewReader(input), output)
	plugin.parseConfig()

	require.Len(t, plugin.hostConfig, 2)

	cfg1 := plugin.hostConfig["192.168.1.1"]
	require.NotNil(t, cfg1)
	assert.Equal(t, "router1", cfg1.hostname)
	assert.Equal(t, "example.com", cfg1.domain)

	cfg2 := plugin.hostConfig["10.0.0.1"]
	require.NotNil(t, cfg2)
	assert.Equal(t, "router2", cfg2.hostname)
	assert.Equal(t, "test.org", cfg2.domain)
}

// TestHostnamePluginEmptyValues verifies handling of missing values.
//
// VALIDATES: Plugin handles peers with no hostname/domain gracefully.
// PREVENTS: Panic or invalid capability when config is incomplete.
func TestHostnamePluginEmptyValues(t *testing.T) {
	// No config for any peer
	input := "config done\n"
	output := &bytes.Buffer{}

	plugin := NewHostnamePlugin(strings.NewReader(input), output)
	plugin.parseConfig()

	assert.Empty(t, plugin.hostConfig)
}

// TestHostnamePluginBoundary verifies boundary conditions.
//
// VALIDATES: Hostname/domain at 255 bytes works, 256 bytes truncates.
// PREVENTS: Buffer overflow or silent data loss.
func TestHostnamePluginBoundary(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantLen  int
	}{
		{
			name:     "max_valid_255",
			hostname: strings.Repeat("a", 255),
			wantLen:  255,
		},
		{
			name:     "truncate_256",
			hostname: strings.Repeat("b", 256),
			wantLen:  255, // Truncated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &fqdnConfig{hostname: tt.hostname, domain: ""}
			hex := cfg.encodeValue()
			// First byte is length
			lenByte := hex[0:2]
			assert.Equal(t, "ff", lenByte) // 255 = 0xff
			// Content should be wantLen bytes (2 hex chars per byte)
			// Plus 1 byte for hostname len, plus 1 byte for domain len (00)
			// Total = 1 + wantLen + 1 = wantLen + 2 bytes
			expectedHexLen := (1 + tt.wantLen + 1) * 2
			assert.Len(t, hex, expectedHexLen)
		})
	}
}

// TestHostnamePluginYANG verifies --yang output.
//
// VALIDATES: Plugin outputs valid YANG schema on --yang flag.
// PREVENTS: Missing or malformed YANG breaking discovery.
func TestHostnamePluginYANG(t *testing.T) {
	yang := GetYANG()

	// Must contain required elements
	assert.Contains(t, yang, "module ze-hostname")
	assert.Contains(t, yang, "namespace")
	assert.Contains(t, yang, "augment") // YANG keyword is "augment" not "augments"
	assert.Contains(t, yang, "leaf host")
	assert.Contains(t, yang, "leaf domain")
	// Legacy paths for trigger detection
	assert.Contains(t, yang, "host-name")
	assert.Contains(t, yang, "domain-name")
}

// TestHostnamePluginDeclarations verifies startup protocol.
//
// VALIDATES: Plugin sends correct declare lines.
// PREVENTS: Config not being delivered to plugin.
func TestHostnamePluginDeclarations(t *testing.T) {
	// Simulate startup with immediate config done and registry done
	input := strings.Join([]string{
		"config done",
		"registry done",
	}, "\n") + "\n"
	output := &bytes.Buffer{}

	plugin := NewHostnamePlugin(strings.NewReader(input), output)
	plugin.doStartupProtocol()

	out := output.String()

	// Check new JSON config declaration (root-based delivery)
	assert.Contains(t, out, "declare wants config bgp")

	assert.Contains(t, out, "declare done")
	assert.Contains(t, out, "capability done")
	assert.Contains(t, out, "ready")
}
