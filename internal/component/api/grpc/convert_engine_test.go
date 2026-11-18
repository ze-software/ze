// Design: docs/architecture/api/architecture.md -- domain adapters preserve execution and streaming parameters.
package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

func convertedExecuteCommand(t *testing.T, req *api.ExecuteRequest) string {
	t.Helper()
	var built string
	engine := api.NewAPIEngine(func(_ context.Context, _ api.CallerIdentity, input string) (*plugin.Response, error) {
		built = input
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, nil, nil, nil)
	response, err := engine.Execute(t.Context(), req)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	return built
}

func convertedStreamCommand(t *testing.T, req *api.StreamRequest) string {
	t.Helper()
	var built string
	events := make(chan string)
	close(events)
	engine := api.NewAPIEngine(nil, nil, nil, func(_ context.Context, _ api.CallerIdentity, input string) (<-chan string, func(), error) {
		built = input
		return events, func() {}, nil
	})
	_, cancel, err := engine.Stream(t.Context(), req)
	require.NoError(t, err)
	defer cancel()
	return built
}
