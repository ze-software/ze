package meta

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/transaction"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// newTestContext creates a CommandContext for handler tests.
func newTestContext() *pluginserver.CommandContext {
	server := pluginserver.NewServer(&pluginserver.ServerConfig{
		CommitManager: transaction.NewCommitManager(),
	}, nil)
	return &pluginserver.CommandContext{Server: server}
}

// TestHandlerEventList verifies event list returns BGP event types.
//
// VALIDATES: Event list handler returns all known event types.
// PREVENTS: Missing event types in API response.
func TestHandlerEventList(t *testing.T) {
	resp, err := handleBgpEventList(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	events, ok := data["events"].([]string)
	require.True(t, ok)
	assert.Contains(t, events, "update")
	assert.Contains(t, events, "state")
	assert.Contains(t, events, "negotiated")
}

// TestHandlerPluginEncoding verifies encoding handler accepts valid encodings.
//
// VALIDATES: Encoding handler accepts json/text, rejects invalid.
// PREVENTS: Accepting unknown encoding names silently.
func TestHandlerPluginEncoding(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "json", input: "json"},
		{name: "text", input: "text"},
		{name: "invalid", input: "xml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext()
			resp, err := handleBgpPluginEncoding(ctx, []string{tt.input})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusDone, resp.Status)
		})
	}
}

// TestHandlerPluginEncodingMissingArg verifies encoding handler rejects no args.
//
// VALIDATES: Encoding handler requires argument.
// PREVENTS: Panic on empty args.
func TestHandlerPluginEncodingMissingArg(t *testing.T) {
	ctx := newTestContext()
	_, err := handleBgpPluginEncoding(ctx, nil)
	require.Error(t, err)
}

// TestHandlerPluginFormat verifies format handler accepts valid formats.
//
// VALIDATES: Format handler accepts hex/base64/parsed/full, rejects invalid.
// PREVENTS: Accepting unknown format names silently.
func TestHandlerPluginFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "hex", input: "hex"},
		{name: "base64", input: "base64"},
		{name: "parsed", input: "parsed"},
		{name: "full", input: "full"},
		{name: "invalid", input: "yaml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext()
			resp, err := handleBgpPluginFormat(ctx, []string{tt.input})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusDone, resp.Status)
		})
	}
}

// TestHandlerPluginAck verifies ack handler accepts sync/async.
//
// VALIDATES: Ack handler accepts sync/async, rejects invalid.
// PREVENTS: Accepting unknown ack modes.
func TestHandlerPluginAck(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "sync", input: "sync"},
		{name: "async", input: "async"},
		{name: "invalid", input: "maybe", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext()
			resp, err := handleBgpPluginAck(ctx, []string{tt.input})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusDone, resp.Status)
		})
	}
}

// TestHandlerPluginAckMissingArg verifies ack handler rejects no args.
//
// VALIDATES: Ack handler requires argument.
// PREVENTS: Panic on empty args.
func TestHandlerPluginAckMissingArg(t *testing.T) {
	ctx := newTestContext()
	_, err := handleBgpPluginAck(ctx, nil)
	require.Error(t, err)
}

// TestHandlerHelp verifies bgp help returns registered bgp commands.
//
// VALIDATES: Help handler returns command list from dispatcher.
// PREVENTS: Help returning empty results when commands are registered.
func TestHandlerHelp(t *testing.T) {
	ctx := newTestContext()
	resp, err := handleBgpHelp(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	commands, ok := data["commands"].([]string)
	require.True(t, ok)
	assert.NotEmpty(t, commands, "should list registered bgp commands")

	// At least "bgp help" itself should be listed (registered by init())
	found := false
	for _, c := range commands {
		if strings.HasPrefix(c, "bgp help") {
			found = true
			break
		}
	}
	assert.True(t, found, "bgp help command should appear in help output")
}

// TestHandlerHelpNilDispatcher verifies bgp help handles nil dispatcher without panic.
//
// VALIDATES: Help handler is nil-safe when server is nil.
// PREVENTS: Nil pointer dereference when dispatcher is unavailable.
func TestHandlerHelpNilDispatcher(t *testing.T) {
	ctx := &pluginserver.CommandContext{} // nil Server → nil Dispatcher
	resp, err := handleBgpHelp(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	// commands should be nil (no dispatcher to populate)
	assert.Nil(t, data["commands"])
}

// TestHandlerCommandList verifies command list returns bgp commands.
//
// VALIDATES: Command list handler returns Completion structs for registered commands.
// PREVENTS: Command list returning empty when commands exist.
func TestHandlerCommandList(t *testing.T) {
	ctx := newTestContext()
	resp, err := handleBgpCommandList(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	commands, ok := data["commands"].([]pluginserver.Completion)
	require.True(t, ok)
	assert.NotEmpty(t, commands, "should list registered bgp commands")

	// Verify completions have Value and Help populated
	for _, c := range commands {
		assert.NotEmpty(t, c.Value, "completion value should not be empty")
		assert.True(t, strings.HasPrefix(c.Value, "bgp "), "command should start with 'bgp '")
		// Source should be empty in non-verbose mode
		assert.Empty(t, c.Source, "source should be empty in non-verbose mode")
	}
}

// TestHandlerCommandListVerbose verifies verbose mode sets Source field.
//
// VALIDATES: Verbose flag populates Source field as "builtin".
// PREVENTS: Source field missing in verbose command list output.
func TestHandlerCommandListVerbose(t *testing.T) {
	ctx := newTestContext()
	resp, err := handleBgpCommandList(ctx, []string{"verbose"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	commands, ok := data["commands"].([]pluginserver.Completion)
	require.True(t, ok)
	assert.NotEmpty(t, commands)

	for _, c := range commands {
		assert.Equal(t, "builtin", c.Source, "verbose mode should set source to 'builtin'")
	}
}

// TestHandlerCommandHelp verifies command help returns details for known commands.
//
// VALIDATES: Command help returns correct name, description, and source for a known command.
// PREVENTS: Command help failing for validly registered commands.
func TestHandlerCommandHelp(t *testing.T) {
	ctx := newTestContext()
	resp, err := handleBgpCommandHelp(ctx, []string{"bgp help"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bgp help", data["command"])
	assert.NotEmpty(t, data["description"])
	assert.Equal(t, "builtin", data["source"])
}

// TestHandlerCommandHelpMissingArg verifies command help rejects empty args.
//
// VALIDATES: Command help handler requires a command name argument.
// PREVENTS: Panic or silent failure when no argument is provided.
func TestHandlerCommandHelpMissingArg(t *testing.T) {
	_, err := handleBgpCommandHelp(newTestContext(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage:")
}

// TestHandlerCommandHelpUnknown verifies command help rejects unknown commands.
//
// VALIDATES: Command help returns error for non-existent command names.
// PREVENTS: Returning empty help data for unknown commands.
func TestHandlerCommandHelpUnknown(t *testing.T) {
	_, err := handleBgpCommandHelp(newTestContext(), []string{"bgp nonexistent-command-xyz"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown bgp command")
}

// TestHandlerCommandComplete verifies completion matching for partial input.
//
// VALIDATES: Command complete returns matching bgp commands for partial input.
// PREVENTS: Completion returning no results for valid partial matches.
func TestHandlerCommandComplete(t *testing.T) {
	ctx := newTestContext()
	resp, err := handleBgpCommandComplete(ctx, []string{"bgp h"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	completions, ok := data["completions"].([]pluginserver.Completion)
	require.True(t, ok)
	assert.NotEmpty(t, completions, "should match at least 'bgp help'")

	found := false
	for _, c := range completions {
		if c.Value == "bgp help" {
			found = true
			break
		}
	}
	assert.True(t, found, "'bgp help' should appear in completions for 'bgp h'")
}

// TestHandlerCommandCompleteMissingArg verifies completion rejects empty args.
//
// VALIDATES: Command complete handler requires a partial input argument.
// PREVENTS: Panic or silent failure when no argument is provided.
func TestHandlerCommandCompleteMissingArg(t *testing.T) {
	_, err := handleBgpCommandComplete(newTestContext(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage:")
}
