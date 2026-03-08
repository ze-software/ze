package bgpcmdmeta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/commit"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// newTestContext creates a CommandContext for handler tests.
func newTestContext() *pluginserver.CommandContext {
	server := pluginserver.NewServer(&pluginserver.ServerConfig{
		CommitManager: commit.NewCommitManager(),
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
