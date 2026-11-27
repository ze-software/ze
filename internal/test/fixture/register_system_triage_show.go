// Design: docs/architecture/diagnostics/production-diagnostics.md -- fixture registration for show system triage
// Related: plugin_system_triage_show.go -- scenario body

package fixture

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/system-triage-show", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "system-triage-show-test", &sdk.Registration{}, plugin16AfterEOR(pluginSystemTriage))
	}, StopsDaemon)
}
