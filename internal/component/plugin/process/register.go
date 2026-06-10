// Design: docs/architecture/api/process-protocol.md -- plugin process health
// Related: manager.go -- raises the plugin-down warning this row reports

package process

import (
	"codeberg.org/thomas-mangin/ze/internal/core/health"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
)

func init() {
	// manager.go raises plugin-down when a managed plugin process dies; the
	// process manager owns its health row (ai/rules/plugin-self-containment.md).
	health.Register("plugins", report.HealthProbeDown("plugin-down"))
}
