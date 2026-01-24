package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseVerifyCommand verifies verify command parsing.
//
// VALIDATES: Verify command is correctly parsed.
// PREVENTS: Command parsing errors.
func TestParseVerifyCommand(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantHandler string
		wantAction  string
		wantPath    string
		wantData    string
		wantErr     bool
	}{
		{
			name:        "basic_create",
			input:       `config verify handler "bgp.peer" action create path "bgp.peer[address=192.0.2.1]" data '{"peer-as":65002}'`,
			wantHandler: "bgp.peer",
			wantAction:  "create",
			wantPath:    "bgp.peer[address=192.0.2.1]",
			wantData:    `{"peer-as":65002}`,
		},
		{
			name:        "modify_action",
			input:       `config verify handler "bgp" action modify path "bgp" data '{"local-as":65001}'`,
			wantHandler: "bgp",
			wantAction:  "modify",
			wantPath:    "bgp",
			wantData:    `{"local-as":65001}`,
		},
		{
			name:        "delete_action",
			input:       `config verify handler "bgp.peer" action delete path "bgp.peer[address=192.0.2.1]" data '{}'`,
			wantHandler: "bgp.peer",
			wantAction:  "delete",
			wantPath:    "bgp.peer[address=192.0.2.1]",
			wantData:    `{}`,
		},
		{
			name:    "missing_handler",
			input:   `config verify action create path "test" data '{}'`,
			wantErr: true,
		},
		{
			name:    "missing_action",
			input:   `config verify handler "test" path "test" data '{}'`,
			wantErr: true,
		},
		{
			name:    "missing_data",
			input:   `config verify handler "test" action create path "test"`,
			wantErr: true,
		},
		{
			name:    "not_config_verify",
			input:   `config apply handler "test"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseVerifyCommand(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHandler, req.Handler)
			assert.Equal(t, tt.wantAction, req.Action)
			assert.Equal(t, tt.wantPath, req.Path)
			assert.Equal(t, tt.wantData, req.Data)
		})
	}
}

// TestHub_RouteVerifyToHandler verifies routing by handler prefix.
//
// VALIDATES: Verify requests are routed to correct handler.
// PREVENTS: Misrouting config to wrong plugin.
func TestHub_RouteVerifyToHandler(t *testing.T) {
	registry := NewSchemaRegistry()

	// Register schemas
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

	// Test handler lookup
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

// TestHub_VerifyUnknownHandler verifies error on unknown handler.
//
// VALIDATES: Unknown handlers return error.
// PREVENTS: Silent failure on misconfigured handlers.
func TestHub_VerifyUnknownHandler(t *testing.T) {
	registry := NewSchemaRegistry()
	subsystems := NewSubsystemManager()

	hub := NewHub(registry, subsystems)

	req := &VerifyRequest{
		Handler: "unknown.handler",
		Action:  "create",
		Path:    "unknown.handler",
		Data:    "{}",
	}

	err := hub.RouteVerify(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown handler")
}

// TestHub_ProcessConfig verifies transaction handling.
//
// VALIDATES: All verify pass before any apply.
// PREVENTS: Partial configuration application.
func TestHub_ProcessConfig(t *testing.T) {
	// This test verifies the transaction logic at the Hub level
	// Without actual plugins, we can only test the error paths

	registry := NewSchemaRegistry()
	subsystems := NewSubsystemManager()

	hub := NewHub(registry, subsystems)

	// Empty blocks should succeed
	err := hub.ProcessConfig(context.Background(), []ConfigBlock{})
	require.NoError(t, err)

	// Unknown handler should fail at verify
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
	assert.Contains(t, err.Error(), "verify failed")
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
		Data:    `{"peer-as":65002}`,
	}

	assert.Equal(t, "bgp.peer", block.Handler)
	assert.Equal(t, "create", block.Action)
	assert.Equal(t, "bgp.peer[address=192.0.2.1]", block.Path)
	assert.Contains(t, block.Data, "peer-as")
}

// TestParseQuotedOrWord verifies parsing helper.
//
// VALIDATES: Quoted and unquoted strings are parsed correctly.
// PREVENTS: Parsing errors in command handling.
func TestParseQuotedOrWord(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		rest    string
		wantErr bool
	}{
		{"quoted", `"hello world" rest`, "hello world", " rest", false},
		{"unquoted", `hello rest`, "hello", " rest", false},
		{"empty", ``, "", "", true},
		{"unclosed", `"hello`, "", "", true},
		{"escaped", `"hello\"world" rest`, `hello"world`, " rest", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rest, err := parseQuotedOrWord(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.rest, rest)
		})
	}
}

// TestParseQuotedData verifies JSON data parsing.
//
// VALIDATES: Single-quoted JSON data is parsed correctly.
// PREVENTS: JSON data corruption during parsing.
func TestParseQuotedData(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"simple", `'{"key":"value"}'`, `{"key":"value"}`, false},
		{"empty_object", `'{}'`, `{}`, false},
		{"with_escape", `'{"key":"value\'s"}'`, `{"key":"value's"}`, false},
		{"not_quoted", `{"key":"value"}`, "", true},
		{"unclosed", `'{"key":"value"`, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := parseQuotedData(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
