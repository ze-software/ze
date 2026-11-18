// Design: docs/architecture/api/architecture.md -- converters preserve domain parameters.
package rest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

// convertedExecuteCommand observes the final command at the production engine
// boundary. Converter fixtures keep their original text expectations while
// allowing metadata-dependent construction to happen in its actual owner.
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
