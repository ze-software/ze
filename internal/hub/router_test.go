package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/plugin/server"
)

// TestHubCommandRouting verifies command routing by handler prefix.
//
// VALIDATES: Commands routed to correct plugin.
// PREVENTS: Wrong plugin receiving command.
func TestHubCommandRouting(t *testing.T) {
	cfg := &HubConfig{}
	o := NewOrchestrator(cfg)

	// Register schemas
	err := o.Registry().Register(&pluginserver.Schema{
		Module:   "ze-bgp",
		Handlers: []string{"bgp", "bgp.peer"},
		Plugin:   "bgp",
		Priority: 100,
	})
	require.NoError(t, err)

	err = o.Registry().Register(&pluginserver.Schema{
		Module:   "ze-rib",
		Handlers: []string{"rib"},
		Plugin:   "rib",
		Priority: 200,
	})
	require.NoError(t, err)

	// Test routing
	schema, handler := o.Registry().FindHandler("bgp.peer.list")
	require.NotNil(t, schema)
	assert.Equal(t, "bgp.peer", handler)
	assert.Equal(t, "bgp", schema.Plugin)

	schema, handler = o.Registry().FindHandler("rib.show")
	require.NotNil(t, schema)
	assert.Equal(t, "rib", handler)
	assert.Equal(t, "rib", schema.Plugin)
}

// TestHubEventPattern verifies event pattern matching.
//
// VALIDATES: Event patterns match correctly.
// PREVENTS: Missed event subscriptions.
func TestHubEventPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		event    string
		expected bool
	}{
		{"exact_match", "bgp.peer.up", "bgp.peer.up", true},
		{"wildcard_all", "bgp.*", "bgp.peer", true},
		{"wildcard_prefix", "bgp.peer.*", "bgp.peer.up", true},
		{"no_match", "rib.*", "bgp.peer.up", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchEventPattern(tt.pattern, tt.event)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestSocketPathResolution verifies socket path selection.
//
// VALIDATES: Socket path follows precedence order.
// PREVENTS: Socket creation in non-writable location.
func TestSocketPathResolution(t *testing.T) {
	// Default path when no config
	path := resolveSocketPath("")
	assert.NotEmpty(t, path)

	// Explicit path from config
	path = resolveSocketPath("/custom/path.sock")
	assert.Equal(t, "/custom/path.sock", path)
}
