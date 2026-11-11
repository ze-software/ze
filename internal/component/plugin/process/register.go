// Design: docs/architecture/api/process-protocol.md -- plugin process health
// Related: manager.go -- raises the doctor-plugin-stopped warning this row reports

package process

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

// processEventCodes are the report-bus codes the process manager raises as
// events, which the recent-events doctor check reports: a plugin process that
// ended unasked, and a stderr relay that stopped while its plugin ran.
var processEventCodes = []diagnostic.EventCode{
	{Code: diagnostic.CodeDoctorPluginCrashed},
	{Code: diagnostic.CodeDoctorPluginStderrRelayStopped},
}

func init() {
	// manager.go raises doctor-plugin-stopped when the respawn limit disables
	// a plugin; the process manager owns its health row (ai/rules/plugins.md).
	health.Register("plugins", report.HealthProbeDown(diagnostic.CodeDoctorPluginStopped))
	for _, event := range processEventCodes {
		if err := diagnostic.RegisterEventCode(event); err != nil {
			panic("BUG: plugin process event code registration refused: " + err.Error())
		}
	}
}
