package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCommand verifies command parsing.
//
// VALIDATES: Command format parsed correctly.
// PREVENTS: Command parsing errors.
func TestParseCommand(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantHandler string
		wantAction  string
		wantData    string
		wantErr     bool
	}{
		{
			name:        "basic_create",
			input:       `bgp peer create {"remote":{"as":65002,"ip":"192.0.2.1"}}`,
			wantHandler: "bgp.peer",
			wantAction:  "create",
			wantData:    `{"remote":{"as":65002,"ip":"192.0.2.1"}}`,
		},
		{
			name:        "modify_action",
			input:       `bgp peer modify {"address":"192.0.2.1","receive-hold-time":90}`,
			wantHandler: "bgp.peer",
			wantAction:  "modify",
			wantData:    `{"address":"192.0.2.1","receive-hold-time":90}`,
		},
		{
			name:        "delete_action",
			input:       `bgp peer delete {"address":"192.0.2.1"}`,
			wantHandler: "bgp.peer",
			wantAction:  "delete",
			wantData:    `{"address":"192.0.2.1"}`,
		},
		{
			name:        "commit",
			input:       `bgp commit`,
			wantHandler: "bgp",
			wantAction:  "commit",
		},
		{
			name:        "rollback",
			input:       `bgp rollback`,
			wantHandler: "bgp",
			wantAction:  "rollback",
		},
		{
			name:        "namespace_only_create",
			input:       `bgp create {"router-id":"1.2.3.4"}`,
			wantHandler: "bgp",
			wantAction:  "create",
			wantData:    `{"router-id":"1.2.3.4"}`,
		},
		{
			name:        "namespace_only_modify",
			input:       `rib modify {"max-entries":1000}`,
			wantHandler: "rib",
			wantAction:  "modify",
			wantData:    `{"max-entries":1000}`,
		},
		{
			name:    "missing_json",
			input:   `bgp peer create`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := ParseCommand(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHandler, block.Handler)
			assert.Equal(t, tt.wantAction, block.Action)
			assert.Equal(t, tt.wantData, block.Data)
		})
	}
}

// TestHub_RouteToHandler verifies routing by handler prefix.
//
// VALIDATES: Commands are routed to correct handler.
// PREVENTS: Misrouting config to wrong plugin.
func TestHub_RouteToHandler(t *testing.T) {
	registry := NewSchemaRegistry()

	// Register schemas.
	err := registry.Register(&Schema{
		Module:   "ze-bgp",
		Handlers: []string{"bgp", "bgp.peer"},
		Plugin:   "bgp",
	})
	require.NoError(t, err)

	err = registry.Register(&Schema{
		Module:   "ze-rib",
		Handlers: []string{"rib"},
		Plugin:   "rib",
	})
	require.NoError(t, err)

	// Test handler lookup.
	schema, match := registry.FindHandler("bgp.peer")
	require.NotNil(t, schema)
	assert.Equal(t, "bgp.peer", match)
	assert.Equal(t, "bgp", schema.Plugin)

	schema, match = registry.FindHandler("bgp.peer[address=192.0.2.1]")
	require.NotNil(t, schema)
	assert.Equal(t, "bgp.peer", match)

	schema, match = registry.FindHandler("rib")
	require.NotNil(t, schema)
	assert.Equal(t, "rib", match)
	assert.Equal(t, "rib", schema.Plugin)
}

// TestHub_UnknownHandler verifies error on unknown handler.
//
// VALIDATES: Unknown handlers return error.
// PREVENTS: Silent failure on misconfigured handlers.
func TestHub_UnknownHandler(t *testing.T) {
	registry := NewSchemaRegistry()
	subsystems := NewSubsystemManager()

	hub := NewHub(registry, subsystems)

	block := &ConfigBlock{
		Handler: "unknown.handler",
		Action:  "create",
		Path:    "unknown.handler",
		Data:    "{}",
	}

	err := hub.RouteCommand(context.Background(), block)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown handler")
}

// TestHub_ProcessConfig verifies transaction handling.
//
// VALIDATES: All commands sent before commit.
// PREVENTS: Partial configuration application.
func TestHub_ProcessConfig(t *testing.T) {
	registry := NewSchemaRegistry()
	subsystems := NewSubsystemManager()

	hub := NewHub(registry, subsystems)

	// Empty blocks should succeed.
	err := hub.ProcessConfig(context.Background(), []ConfigBlock{})
	require.NoError(t, err)

	// Unknown handler should fail.
	blocks := []ConfigBlock{
		{
			Handler: "unknown",
			Action:  "create",
			Path:    "unknown",
			Data:    "{}",
		},
	}
	err = hub.ProcessConfig(context.Background(), blocks)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config failed")
}

// TestConfigBlock verifies ConfigBlock structure.
//
// VALIDATES: ConfigBlock holds all required fields.
// PREVENTS: Missing configuration data.
func TestConfigBlock(t *testing.T) {
	block := ConfigBlock{
		Handler: "bgp.peer",
		Action:  "create",
		Path:    "bgp.peer[address=192.0.2.1]",
		Data:    `{"remote":{"as":65002}}`,
	}

	assert.Equal(t, "bgp.peer", block.Handler)
	assert.Equal(t, "create", block.Action)
	assert.Equal(t, "bgp.peer[address=192.0.2.1]", block.Path)
	assert.Contains(t, block.Data, "remote")
}

// TestSplitHandler verifies handler splitting.
//
// VALIDATES: Handler split into namespace and path.
// PREVENTS: Incorrect namespace extraction.
func TestSplitHandler(t *testing.T) {
	tests := []struct {
		handler       string
		wantNamespace string
		wantPath      string
	}{
		{"bgp.peer", "bgp", "peer"},
		{"bgp.peer.timers", "bgp", "peer.timers"},
		{"bgp", "bgp", ""},
		{"rib.route", "rib", "route"},
		{"acme-monitor.endpoint", "acme-monitor", "endpoint"},
	}

	for _, tt := range tests {
		t.Run(tt.handler, func(t *testing.T) {
			ns, path := splitHandler(tt.handler)
			assert.Equal(t, tt.wantNamespace, ns)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}
