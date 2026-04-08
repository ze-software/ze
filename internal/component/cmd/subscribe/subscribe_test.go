package subscribe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// newTestContext creates a CommandContext with a Server (including SubscriptionManager)
// and an optional Process. Pass nil proc for tests that need no process context.
func newTestContext(proc *process.Process) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	return &pluginserver.CommandContext{
		Server:  server,
		Process: proc,
	}
}

// validSubscribeArgs returns args that ParseSubscription accepts.
func validSubscribeArgs() []string {
	return []string{"bgp", "event", "update"}
}

// =============================================================================
// Subscribe Handler Tests
// =============================================================================

// TestSubscribeInvalidArgs verifies handleSubscribe rejects invalid arguments.
//
// VALIDATES: Bad args produce error response with StatusError.
// PREVENTS: Malformed subscribe commands accepted silently.
func TestSubscribeInvalidArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"empty", nil},
		{"missing_event_keyword", []string{"bgp", "update"}},
		{"invalid_namespace", []string{"bmp", "event", "update"}},
		{"missing_event_type", []string{"bgp", "event"}},
		{"invalid_event_type", []string{"bgp", "event", "unknown"}},
		{"invalid_direction", []string{"bgp", "event", "update", "direction", "inbound"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(process.NewProcess(plugin.PluginConfig{Name: "test"}))

			resp, err := handleSubscribe(ctx, tt.args)
			require.Error(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
		})
	}
}

// TestSubscribeNoProcess verifies handleSubscribe fails when Process is nil.
//
// VALIDATES: Nil Process returns error with descriptive message.
// PREVENTS: Nil pointer dereference when no process context available.
func TestSubscribeNoProcess(t *testing.T) {
	ctx := newTestContext(nil)

	resp, err := handleSubscribe(ctx, validSubscribeArgs())
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, "subscribe requires a process context", resp.Data)
}

// TestSubscribeNoSubscriptionManager verifies handleSubscribe fails when Server is nil.
//
// VALIDATES: Nil Server (so Subscriptions() returns nil) returns error.
// PREVENTS: Nil pointer dereference on subscription manager access.
func TestSubscribeNoSubscriptionManager(t *testing.T) {
	ctx := &pluginserver.CommandContext{
		Process: process.NewProcess(plugin.PluginConfig{Name: "test"}),
		// Server is nil, so Subscriptions() returns nil
	}

	resp, err := handleSubscribe(ctx, validSubscribeArgs())
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, "subscription manager not available", resp.Data)
}

// TestSubscribeSuccess verifies handleSubscribe succeeds with valid context.
//
// VALIDATES: Subscription is added and response contains correct data fields.
// PREVENTS: Successful subscribe returning wrong status or missing data.
func TestSubscribeSuccess(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		namespace string
		event     string
		direction string
	}{
		{
			name:      "bgp_update",
			args:      []string{"bgp", "event", "update"},
			namespace: "bgp",
			event:     "update",
			direction: "both",
		},
		{
			name:      "bgp_state_received",
			args:      []string{"bgp", "event", "state", "direction", "received"},
			namespace: "bgp",
			event:     "state",
			direction: "received",
		},
		{
			name:      "rib_cache",
			args:      []string{"bgp-rib", "event", "cache"},
			namespace: "bgp-rib",
			event:     "cache",
			direction: "both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := process.NewProcess(plugin.PluginConfig{Name: "test-plugin"})
			ctx := newTestContext(proc)

			resp, err := handleSubscribe(ctx, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusDone, resp.Status)

			data, ok := resp.Data.(map[string]any)
			require.True(t, ok, "response data should be map[string]any")
			assert.Equal(t, tt.namespace, data["namespace"])
			assert.Equal(t, tt.event, data["event"])
			assert.Equal(t, tt.direction, data["direction"])

			// Verify subscription was actually added to the manager.
			assert.Equal(t, 1, ctx.Subscriptions().Count(proc))
		})
	}
}

// =============================================================================
// Unsubscribe Handler Tests
// =============================================================================

// TestUnsubscribeInvalidArgs verifies handleUnsubscribe rejects invalid arguments.
//
// VALIDATES: Bad args produce error response with StatusError.
// PREVENTS: Malformed unsubscribe commands accepted silently.
func TestUnsubscribeInvalidArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"empty", nil},
		{"missing_event_keyword", []string{"bgp", "update"}},
		{"invalid_namespace", []string{"bmp", "event", "update"}},
		{"missing_event_type", []string{"bgp", "event"}},
		{"invalid_event_type", []string{"bgp-rib", "event", "update"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext(process.NewProcess(plugin.PluginConfig{Name: "test"}))

			resp, err := handleUnsubscribe(ctx, tt.args)
			require.Error(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
		})
	}
}

// TestUnsubscribeNoProcess verifies handleUnsubscribe fails when Process is nil.
//
// VALIDATES: Nil Process returns error with descriptive message.
// PREVENTS: Nil pointer dereference when no process context available.
func TestUnsubscribeNoProcess(t *testing.T) {
	ctx := newTestContext(nil)

	resp, err := handleUnsubscribe(ctx, validSubscribeArgs())
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Equal(t, "unsubscribe requires a process context", resp.Data)
}

// TestUnsubscribeSuccess verifies handleUnsubscribe succeeds after subscribing.
//
// VALIDATES: Subscribe then unsubscribe returns removed=true and correct data.
// PREVENTS: Unsubscribe failing to remove existing subscription.
func TestUnsubscribeSuccess(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-plugin"})
	ctx := newTestContext(proc)

	// Subscribe first.
	args := validSubscribeArgs()
	_, err := handleSubscribe(ctx, args)
	require.NoError(t, err)
	assert.Equal(t, 1, ctx.Subscriptions().Count(proc))

	// Unsubscribe with same args.
	resp, err := handleUnsubscribe(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "response data should be map[string]any")
	assert.Equal(t, true, data["removed"])
	assert.Equal(t, "bgp", data["namespace"])
	assert.Equal(t, "update", data["event"])

	// Verify subscription was actually removed.
	assert.Equal(t, 0, ctx.Subscriptions().Count(proc))
}

// TestUnsubscribeNotFound verifies handleUnsubscribe returns removed=false for missing subscription.
//
// VALIDATES: Unsubscribe without prior subscribe returns removed=false, not an error.
// PREVENTS: Unsubscribe of non-existent subscription causing error or panic.
func TestUnsubscribeNotFound(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-plugin"})
	ctx := newTestContext(proc)

	// Unsubscribe without subscribing first.
	resp, err := handleUnsubscribe(ctx, validSubscribeArgs())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "response data should be map[string]any")
	assert.Equal(t, false, data["removed"])
	assert.Equal(t, "bgp", data["namespace"])
	assert.Equal(t, "update", data["event"])
}
