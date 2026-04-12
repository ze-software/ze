package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractConfigSubtreeEnvironmentWrapped verifies that environment subtree
// is correctly extracted and wrapped from a full config tree.
//
// VALIDATES: Plugins declaring WantsConfig: ["bgp", "environment"] receive
// both subtrees wrapped with their root key.
// PREVENTS: BMP plugin not receiving environment config for receiver listener.
func TestExtractConfigSubtreeEnvironmentWrapped(t *testing.T) {
	t.Parallel()

	configTree := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.2.3.4",
			"bmp": map[string]any{
				"sender": map[string]any{
					"collector": []any{
						map[string]any{"name": "station1", "address": "10.0.0.100", "port": float64(11019)},
					},
				},
			},
		},
		"environment": map[string]any{
			"bmp": map[string]any{
				"enabled": true,
				"server": []any{
					map[string]any{"name": "default", "ip": "0.0.0.0", "port": float64(11019)},
				},
				"max-sessions": float64(100),
			},
			"log": map[string]any{
				"level": "warn",
			},
		},
	}

	// Extract "bgp" subtree -- should be wrapped: {"bgp": {...}}
	bgpResult := ExtractConfigSubtree(configTree, "bgp")
	require.NotNil(t, bgpResult, "bgp subtree should exist")
	bgpWrap, ok := bgpResult.(map[string]any)
	require.True(t, ok)
	bgpInner, ok := bgpWrap["bgp"].(map[string]any)
	require.True(t, ok, "result should be wrapped with bgp key")
	assert.Equal(t, "1.2.3.4", bgpInner["router-id"])

	// Extract "environment" subtree -- should be wrapped: {"environment": {...}}
	envResult := ExtractConfigSubtree(configTree, "environment")
	require.NotNil(t, envResult, "environment subtree should exist")
	envWrap, ok := envResult.(map[string]any)
	require.True(t, ok)
	envInner, ok := envWrap["environment"].(map[string]any)
	require.True(t, ok, "result should be wrapped with environment key")

	// Verify environment contains bmp config.
	bmpCfg, ok := envInner["bmp"].(map[string]any)
	require.True(t, ok, "environment should contain bmp key")
	assert.Equal(t, true, bmpCfg["enabled"])
	assert.Equal(t, float64(100), bmpCfg["max-sessions"])

	// Extract non-existent subtree.
	missing := ExtractConfigSubtree(configTree, "nonexistent")
	assert.Nil(t, missing, "missing key should return nil")
}
