// Design: docs/architecture/diagnostics/production-diagnostics.md -- fixture registration for show system trace
// Related: plugin_system_trace_show.go -- scenario body

package fixture

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/system-trace-show", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "system-trace-show-test", &sdk.Registration{}, plugin16AfterEOR(pluginSystemTrace))
	}, StopsDaemon)
}
