// Design: docs/architecture/api/process-protocol.md -- plugin process health
// Related: manager.go -- raises the plugin-down warning this row reports

package process

import (
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

func init() {
	// manager.go raises plugin-down when a managed plugin process dies; the
	// process manager owns its health row (ai/rules/plugins.md).
	health.Register("plugins", report.HealthProbeDown("plugin-down"))
}
