package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// runningTree is a running configuration as the daemon holds it: DDoS FlowSpec
// at the level the fixture boots with, and a login user whose bcrypt hash is a
// secret.
func runningTree(level string) map[string]any {
	return map[string]any{
		"ddos": map[string]any{"flowspec": map[string]any{"response-level": level, "action": "discard"}},
		"system": map[string]any{"authentication": map[string]any{"user": map[string]any{
			"ci": map[string]any{"password": "$2a$10$abcdefghijklmnopqrstuv", "profile": []any{"admin"}},
		}}},
	}
}

// dispatchRunning sends `show config running` through the production
// dispatcher of a server whose running tree the coordinator holds.
func dispatchRunning(t *testing.T, coordinator *plugin.Coordinator) *plugin.Response {
	t.Helper()
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, coordinator)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: server}
	resp, err := server.Dispatcher().Dispatch(ctx, "show config running")
	require.NoError(t, err)
	require.NotNil(t, resp)
	return resp
}

// flowspecLevel reads ddos/flowspec/response-level out of an answer.
func flowspecLevel(t *testing.T, resp *plugin.Response) string {
	t.Helper()
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "show config running answered %T, want plugin.Map", resp.Data)
	ddos, ok := data["ddos"].(map[string]any)
	require.True(t, ok, "no ddos root in %v", data)
	flowspec, ok := ddos["flowspec"].(map[string]any)
	require.True(t, ok, "no ddos flowspec in %v", ddos)
	level, ok := flowspec["response-level"].(string)
	require.True(t, ok, "no response-level in %v", flowspec)
	return level
}

// VALIDATES: `show config running`, dispatched by its YANG path, answers the
// tree the coordinator holds as running, and follows that tree when a committed
// reload replaces it (SetConfigTree). It never reads a file: the server here has
// no config path at all.
// PREVENTS: a running-config read answered from the config file, which after a
// refused reload already holds the refused text.
func TestShowConfigRunningAnswersTheAppliedTree(t *testing.T) {
	coordinator := plugin.NewCoordinator(runningTree("alert"))

	resp := dispatchRunning(t, coordinator)
	require.Equal(t, plugin.StatusDone, resp.Status, resp.Error)
	assert.Equal(t, "alert", flowspecLevel(t, resp))

	coordinator.SetConfigTree(runningTree("enforce"))
	assert.Equal(t, "enforce", flowspecLevel(t, dispatchRunning(t, coordinator)))
}

// VALIDATES: a secret leaf (the user's bcrypt hash) is answered as the
// placeholder, and the daemon's own running tree keeps the real value.
// PREVENTS: the running config leaking credentials over SSH, REST or gRPC, and
// the mask writing into the live tree the next reload diffs against.
func TestShowConfigRunningMasksSecretsWithoutTouchingTheRunningTree(t *testing.T) {
	tree := runningTree("alert")
	coordinator := plugin.NewCoordinator(tree)

	resp := dispatchRunning(t, coordinator)
	require.Equal(t, plugin.StatusDone, resp.Status, resp.Error)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	user := mapAt(t, data, "system", "authentication", "user", "ci")
	assert.Equal(t, config.SecretDataPlaceholder, user["password"])
	assert.Equal(t, []any{"admin"}, user["profile"])

	live := mapAt(t, tree, "system", "authentication", "user", "ci")
	assert.Equal(t, "$2a$10$abcdefghijklmnopqrstuv", live["password"])
}

// mapAt walks nested maps by key and fails the test where one is missing.
func mapAt(t *testing.T, tree map[string]any, keys ...string) map[string]any {
	t.Helper()
	node := tree
	for _, key := range keys {
		next, ok := node[key].(map[string]any)
		require.True(t, ok, "no map at %q in %v", key, node)
		node = next
	}
	return node
}

// VALIDATES: a daemon with no running tree refuses, naming the command to run
// once start-up completes.
// PREVENTS: an empty map answered as "the daemon runs an empty configuration".
func TestShowConfigRunningRefusesWithoutARunningTree(t *testing.T) {
	resp := dispatchRunning(t, plugin.NewCoordinator(nil))
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "holds no running configuration")
	assert.Contains(t, resp.Error, "show config running")
}
