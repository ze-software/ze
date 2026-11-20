// Design: docs/architecture/api/commands.md -- a data caller's pipe chain, applied by the engine
// Related: plugin_fixture_16_pipe_chain.go -- the body of this observer

package fixture

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The observer test/plugin/dispatch-pipe-chain-applied.ci forks: a plugin
// that dispatches piped lines and judges the answers. Its body is
// plugin_fixture_16_pipe_chain.go.
func init() {
	Register("plugin/dispatch-pipe-chain-applied", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "dispatch-pipe-chain-applied-test", &sdk.Registration{}, plugin16DispatchPipeChainApplied)
	}, StopsDaemon)
}
