package hub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/api"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// VALIDATES: API execute wiring preserves request context and remote address into dispatcher context.
// PREVENTS: REST/gRPC metadata reaching APIEngine but being dropped before Dispatcher.Dispatch().
func TestAPIExecutorPropagatesRequestContextAndRemoteAddr(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	type ctxKey struct{}

	var seen *pluginserver.CommandContext
	server.Dispatcher().Register("test api", func(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		seen = ctx
		return &plugin.Response{Status: plugin.StatusDone, Data: "ok"}, nil
	}, "test api")

	exec := apiExecutor(server)
	requestCtx := context.WithValue(context.Background(), ctxKey{}, "trace-id")

	output, err := exec(requestCtx, api.CallerIdentity{
		Username:   "alice",
		RemoteAddr: "198.51.100.10:4444",
	}, "test api")
	require.NoError(t, err)
	assert.Equal(t, "ok", output)

	require.NotNil(t, seen)
	assert.Equal(t, "alice", seen.Username)
	assert.Equal(t, "198.51.100.10:4444", seen.RemoteAddr)
	assert.Same(t, requestCtx, seen.Context())
	assert.Equal(t, "trace-id", seen.Context().Value(ctxKey{}))
}
