package hub

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
)

// TestHubCollectsSchemas verifies schemas collected from plugins.
//
// VALIDATES: Schemas from plugin declarations are registered.
// PREVENTS: Lost schema declarations during startup.
func TestHubCollectsSchemas(t *testing.T) {
	cfg := &HubConfig{}
	o := NewOrchestrator(cfg)

	// Simulate plugin schema registration
	schema := &plugin.Schema{
		Module:    "ze-bgp",
		Namespace: "urn:ze:bgp",
		Handlers:  []string{"bgp"},
		Plugin:    "bgp",
		Priority:  100,
	}

	err := o.Registry().Register(schema)
	require.NoError(t, err)

	// Verify schema registered
	assert.Equal(t, 1, o.Registry().Count())

	// Verify can retrieve by handler
	found, handlerPath := o.Registry().FindHandler("bgp")
	require.NotNil(t, found)
	assert.Equal(t, "bgp", handlerPath)
	assert.Equal(t, "ze-bgp", found.Module)
}

// TestHubRoutesConfigByHandler verifies longest prefix match routing.
//
// VALIDATES: FindHandler returns correct plugin for path.
// PREVENTS: Wrong plugin receiving config.
func TestHubRoutesConfigByHandler(t *testing.T) {
	cfg := &HubConfig{}
	o := NewOrchestrator(cfg)

	// Register two handlers
	err := o.Registry().Register(&plugin.Schema{
		Module:   "ze-bgp",
		Handlers: []string{"bgp"},
		Plugin:   "bgp",
	})
	require.NoError(t, err)

	err = o.Registry().Register(&plugin.Schema{
		Module:   "ze-gr",
		Handlers: []string{"bgp.peer.capability.graceful-restart"},
		Plugin:   "gr",
	})
	require.NoError(t, err)

	// "bgp.peer.capability.graceful-restart.timers" → gr plugin
	found, handlerPath := o.Registry().FindHandler("bgp.peer.capability.graceful-restart.timers")
	require.NotNil(t, found)
	assert.Equal(t, "bgp.peer.capability.graceful-restart", handlerPath)
	assert.Equal(t, "ze-gr", found.Module)

	// "bgp.peer.remote-as" → bgp plugin
	found, handlerPath = o.Registry().FindHandler("bgp.peer.remote-as")
	require.NotNil(t, found)
	assert.Equal(t, "bgp", handlerPath)
	assert.Equal(t, "ze-bgp", found.Module)
}

// TestHubConfigStore verifies live/edit config storage.
//
// VALIDATES: Config store maintains live and edit states.
// PREVENTS: Config state corruption.
func TestHubConfigStore(t *testing.T) {
	store := NewConfigStore()

	// Set edit config
	editCfg := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.2.3.4",
			"local-as":  "65000",
		},
	}
	store.SetEdit(editCfg)

	// Query edit
	result, err := store.Query(ConfigEdit, "bgp")
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Live should be empty
	_, err = store.Query(ConfigLive, "bgp")
	require.Error(t, err) // Not found in live

	// Apply makes edit become live
	store.Apply()

	// Now live has the config
	result, err = store.Query(ConfigLive, "bgp")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestHubQueryConfigPath verifies path-based config query.
//
// VALIDATES: Query returns subtree for path.
// PREVENTS: Full config returned instead of subtree.
func TestHubQueryConfigPath(t *testing.T) {
	store := NewConfigStore()

	// Note: Use simple keys without dots. List key bracket notation
	// (e.g., peer[address=192.0.2.1]) is a future enhancement.
	cfg := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.2.3.4",
			"peer": map[string]any{
				"peer1": map[string]any{
					"address":   "192.0.2.1",
					"remote-as": "65001",
				},
			},
		},
	}
	store.SetEdit(cfg)
	store.Apply()

	// Query specific path
	result, err := store.Query(ConfigLive, "bgp.router-id")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3.4", result)

	// Query nested path
	result, err = store.Query(ConfigLive, "bgp.peer.peer1.remote-as")
	require.NoError(t, err)
	assert.Equal(t, "65001", result)

	// Query non-existent path
	_, err = store.Query(ConfigLive, "bgp.nonexistent")
	require.Error(t, err)
}

// TestHubDeliversJSON verifies config conversion to JSON.
//
// VALIDATES: Config block converts to valid JSON.
// PREVENTS: JSON encoding errors.
func TestHubDeliversJSON(t *testing.T) {
	store := NewConfigStore()

	cfg := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.2.3.4",
			"local-as":  65000,
		},
	}
	store.SetEdit(cfg)
	store.Apply()

	result, err := store.Query(ConfigLive, "bgp")
	require.NoError(t, err)

	// Should be JSON-encodable
	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(data), "router-id")
	assert.Contains(t, string(data), "1.2.3.4")
}

// TestHubSubRootHandler verifies sub-root handler gets subtree only.
//
// VALIDATES: GR plugin gets only graceful-restart subtree.
// PREVENTS: Sub-root handler receiving full tree.
func TestHubSubRootHandler(t *testing.T) {
	store := NewConfigStore()

	// Note: Use simple keys without dots. List key bracket notation
	// (e.g., peer[address=192.0.2.1]) is a future enhancement.
	cfg := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.2.3.4",
			"peer": map[string]any{
				"peer1": map[string]any{
					"address":   "192.0.2.1",
					"remote-as": "65001",
					"capability": map[string]any{
						"graceful-restart": map[string]any{
							"restart-time": "120",
						},
					},
				},
			},
		},
	}
	store.SetEdit(cfg)
	store.Apply()

	// Query sub-root path
	result, err := store.Query(ConfigLive, "bgp.peer.peer1.capability.graceful-restart")
	require.NoError(t, err)

	// Should be just the graceful-restart subtree
	grCfg, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "120", grCfg["restart-time"])

	// Should not contain peer-level data
	_, hasRemoteAS := grCfg["remote-as"]
	assert.False(t, hasRemoteAS)
}

// TestHubConfigStoreEmpty verifies empty config handling.
//
// VALIDATES: Empty config doesn't panic.
// PREVENTS: Nil pointer on empty config.
func TestHubConfigStoreEmpty(t *testing.T) {
	store := NewConfigStore()

	// Query on empty store
	_, err := store.Query(ConfigLive, "bgp")
	require.Error(t, err)

	_, err = store.Query(ConfigEdit, "bgp")
	require.Error(t, err)
}

// TestHubSchemasByPriority verifies priority ordering.
//
// VALIDATES: Schemas returned in priority order.
// PREVENTS: Out-of-order config processing.
func TestHubSchemasByPriority(t *testing.T) {
	cfg := &HubConfig{}
	o := NewOrchestrator(cfg)

	// Register schemas with different priorities
	err := o.Registry().Register(&plugin.Schema{
		Module:   "ze-rib",
		Handlers: []string{"rib"},
		Plugin:   "rib",
		Priority: 200,
	})
	require.NoError(t, err)

	err = o.Registry().Register(&plugin.Schema{
		Module:   "ze-bgp",
		Handlers: []string{"bgp"},
		Plugin:   "bgp",
		Priority: 100,
	})
	require.NoError(t, err)

	err = o.Registry().Register(&plugin.Schema{
		Module:   "ze-gr",
		Handlers: []string{"bgp.peer.capability.graceful-restart"},
		Plugin:   "gr",
		Priority: 300,
	})
	require.NoError(t, err)

	// Get schemas by priority
	schemas := o.SchemasByPriority()

	require.Len(t, schemas, 3)
	assert.Equal(t, "ze-bgp", schemas[0].Module) // Priority 100
	assert.Equal(t, "ze-rib", schemas[1].Module) // Priority 200
	assert.Equal(t, "ze-gr", schemas[2].Module)  // Priority 300
}
