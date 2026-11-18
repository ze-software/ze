// Design: docs/architecture/hub-architecture.md -- API cleanup participates in the stop result.
package hub

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHubCleanupAPISuccessPreservesRestart reaches actual API shutdown through
// runYANGConfig and proves a successfully joined callback still permits 75.
func TestHubCleanupAPISuccessPreservesRestart(t *testing.T) {
	exit, _, calls := runCleanupResult(t, "request restart", nil, nil, nil,
		func(context.Context) error { return nil })
	require.Equal(t, 75, exit)
	require.Equal(t, []string{"api", "engine", "service-a", "service-b"}, calls)
}

// TestHubCleanupAPIFailureRefusesRestart reaches a failing actual API callback
// during both accepted stop kinds; every later cleanup still runs and exit 1
// retains the cause rather than permitting a recovery after an unjoined API.
func TestHubCleanupAPIFailureRefusesRestart(t *testing.T) {
	for _, line := range []string{"request shutdown", "request restart"} {
		t.Run(line, func(t *testing.T) {
			failure := errors.New("API cleanup result sentinel")
			exit, diagnostic, calls := runCleanupResult(t, line, nil, nil, nil,
				func(context.Context) error { return failure })
			require.Equal(t, 1, exit)
			require.Contains(t, diagnostic, failure.Error())
			require.Contains(t, diagnostic, "ze doctor")
			require.Equal(t, []string{"api", "engine", "service-a", "service-b"}, calls)
		})
	}
}
