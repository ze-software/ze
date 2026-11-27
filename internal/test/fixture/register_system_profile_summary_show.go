// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- fixture registration for show system profile | summary
// Related: plugin_system_profile_summary_show.go -- scenario body

package fixture

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/system-profile-summary-show", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "system-profile-summary-show-test", &sdk.Registration{}, plugin16AfterEOR(pluginSystemProfileSummary))
	}, StopsDaemon)
}
