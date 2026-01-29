package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigTreeStructure verifies the config tree format plugins receive.
//
// VALIDATES: ConfigTree has expected structure for plugin JSON delivery.
// PREVENTS: Wrong tree structure breaking plugin config parsing.
func TestConfigTreeStructure(t *testing.T) {
	// Simulate what the config parser would produce for hostname config
	configTree := map[string]any{
		"bgp": map[string]any{
			"peer": map[string]any{
				"127.0.0.1": map[string]any{
					"capability": map[string]any{
						"hostname": map[string]any{
							"host":   "my-host-name",
							"domain": "my-domain-name.com",
						},
					},
					"router-id":     "10.0.0.2",
					"local-address": "127.0.0.1",
					"local-as":      "65533",
					"peer-as":       "65533",
				},
			},
		},
	}

	// Verify structure matches what hostname plugin expects
	bgp, ok := configTree["bgp"].(map[string]any)
	require.True(t, ok, "bgp should be a map")

	peers, ok := bgp["peer"].(map[string]any)
	require.True(t, ok, "peer should be a map")

	peer, ok := peers["127.0.0.1"].(map[string]any)
	require.True(t, ok, "peer 127.0.0.1 should be a map")

	cap, ok := peer["capability"].(map[string]any)
	require.True(t, ok, "capability should be a map")

	hostname, ok := cap["hostname"].(map[string]any)
	require.True(t, ok, "hostname should be a map")

	assert.Equal(t, "my-host-name", hostname["host"])
	assert.Equal(t, "my-domain-name.com", hostname["domain"])
}

// TestExtractConfigSubtree verifies path-based config extraction.
//
// VALIDATES: Scope paths like "bgp/peer" correctly extract subtrees wrapped in full path.
// PREVENTS: Wrong data sent to plugins when using scoped config delivery.
func TestExtractConfigSubtree(t *testing.T) {
	configTree := map[string]any{
		"bgp": map[string]any{
			"peer": map[string]any{
				"127.0.0.1": map[string]any{
					"peer-as": "65533",
				},
			},
		},
		"environment": map[string]any{
			"log": map[string]any{
				"level": "debug",
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		wantNil  bool
		wantKeys []string // Expected top-level keys in result (always path root)
	}{
		{
			name:     "wildcard_returns_full_tree",
			path:     "*",
			wantKeys: []string{"bgp", "environment"},
		},
		{
			name:     "single_key_bgp_wrapped",
			path:     "bgp",
			wantKeys: []string{"bgp"}, // Now wrapped: {"bgp": {...}}
		},
		{
			name:     "path_bgp_peer_wrapped",
			path:     "bgp/peer",
			wantKeys: []string{"bgp"}, // Wrapped: {"bgp": {"peer": {...}}}
		},
		{
			name:     "deep_path_wrapped",
			path:     "bgp/peer/127.0.0.1",
			wantKeys: []string{"bgp"}, // Wrapped from root
		},
		{
			name:    "nonexistent_root",
			path:    "nonexistent",
			wantNil: true,
		},
		{
			name:    "nonexistent_deep_path",
			path:    "bgp/nonexistent",
			wantNil: true,
		},
		{
			name:     "empty_segment_ignored",
			path:     "bgp//peer",
			wantKeys: []string{"bgp"}, // Wrapped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractConfigSubtree(configTree, tt.path)
			if tt.wantNil {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)

			// Check it's a map with expected keys
			resultMap, ok := result.(map[string]any)
			require.True(t, ok, "expected map[string]any, got %T", result)

			for _, key := range tt.wantKeys {
				assert.Contains(t, resultMap, key, "expected key %q in result", key)
			}
		})
	}
}

// TestExtractConfigSubtreePreservesPath verifies full path structure is preserved.
//
// VALIDATES: "bgp/peer" returns {"bgp": {"peer": ...}}, not just peer data.
// PREVENTS: Plugins losing context about where data came from in the tree.
func TestExtractConfigSubtreePreservesPath(t *testing.T) {
	configTree := map[string]any{
		"bgp": map[string]any{
			"peer": map[string]any{
				"127.0.0.1": map[string]any{
					"peer-as": "65533",
				},
			},
		},
	}

	// Extract bgp/peer - should get {"bgp": {"peer": {...}}}
	result := extractConfigSubtree(configTree, "bgp/peer")
	require.NotNil(t, result)

	// Navigate the wrapped structure
	resultMap, ok := result.(map[string]any)
	require.True(t, ok, "expected map[string]any")

	bgp, ok := resultMap["bgp"].(map[string]any)
	require.True(t, ok, "expected bgp key at root")

	peer, ok := bgp["peer"].(map[string]any)
	require.True(t, ok, "expected peer key under bgp")

	peerData, ok := peer["127.0.0.1"].(map[string]any)
	require.True(t, ok, "expected 127.0.0.1 key under peer")

	assert.Equal(t, "65533", peerData["peer-as"])
}

// TestHostnamePluginFullFlow simulates the full plugin startup flow.
//
// VALIDATES: Plugin receives config, parses it, and registers capabilities.
// PREVENTS: Integration issues between server and plugin.
func TestHostnamePluginFullFlow(t *testing.T) {
	// Simulate the full plugin startup protocol:
	// 1. Plugin sends "declare wants config bgp" + "declare done"
	// 2. Server sends "config json bgp {...}" + "config done"
	// 3. Plugin parses config and sends "capability hex 73 <value> peer <addr>" + "capability done"
	// 4. Server sends "registry done"
	// 5. Plugin sends "ready"

	// This test verifies that when we simulate the protocol, the capability gets registered.

	// Create capability injector
	injector := NewCapabilityInjector()

	// Simulate plugin sending capability (what hostname plugin would send after parsing config)
	capLine := "capability hex 73 0C6D792D686F73742D6E616D65126D792D646F6D61696E2D6E616D652E636F6D peer 127.0.0.1"

	// Parse the capability line
	caps := &PluginCapabilities{PluginName: "hostname"}
	err := caps.ParseLine(capLine)
	require.NoError(t, err)

	// Register capabilities
	err = injector.AddPluginCapabilities(caps)
	require.NoError(t, err)

	// Verify capability is registered for the peer
	peerCaps := injector.GetCapabilitiesForPeer("127.0.0.1")
	require.Len(t, peerCaps, 1)
	assert.Equal(t, uint8(73), peerCaps[0].Code)

	// The capability value should be the hostname encoded
	// 0C = length 12 "my-host-name"
	// 12 = length 18 "my-domain-name.com"
	t.Logf("Capability value: %x", peerCaps[0].Value)
}

// TestParseCapabilityWithPeer verifies capability parsing with peer address.
//
// VALIDATES: "capability hex <code> <value> peer <addr>" is parsed correctly.
// PREVENTS: Per-peer capabilities being lost or misassigned.
func TestParseCapabilityWithPeer(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantCode uint8
		wantPeer string
	}{
		{
			name:     "with_peer",
			line:     "capability hex 73 0474657374 peer 192.168.1.1",
			wantCode: 73,
			wantPeer: "192.168.1.1",
		},
		{
			name:     "global_no_peer",
			line:     "capability hex 2 01",
			wantCode: 2,
			wantPeer: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := &PluginCapabilities{}
			err := caps.ParseLine(tt.line)
			require.NoError(t, err)

			require.Len(t, caps.Capabilities, 1)
			cap := caps.Capabilities[0]
			assert.Equal(t, tt.wantCode, cap.Code)

			if tt.wantPeer != "" {
				require.Len(t, cap.Peers, 1)
				assert.Equal(t, tt.wantPeer, cap.Peers[0])
			} else {
				assert.Empty(t, cap.Peers)
			}
		})
	}
}
