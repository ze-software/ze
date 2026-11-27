// Design: docs/architecture/api/process-protocol.md -- plugin process health
// Related: stopped.go -- recordStoppedLocked raises the doctor-plugin-stopped warning this row reports

package process

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

// processEventCodes are the report-bus codes the process manager raises as
// events, which the recent-events doctor check reports: a plugin process that
// ended unasked, a stderr relay that stopped while its plugin ran, a respawn
// whose replacement started before the process it replaces ended, a plugin
// stopped at boot that had not finished stopping when ze went on, a terminal
// stop that could not resolve plugin dependency order, a plugin process group
// that could not be signaled, and a forked plugin stderr drain whose parent
// write-end close or post-reap idle deadline could not be armed.
var processEventCodes = []diagnostic.EventCode{
	{Code: diagnostic.CodeDoctorPluginCrashed},
	{Code: codeDoctorPluginCleanup},
	{Code: CodeDoctorPluginStderrRelayStopped},
	{Code: CodeDoctorPluginRestartOverlap},
	{Code: CodeDoctorPluginStopUnfinished},
	{Code: CodeDoctorPluginShutdownOrder},
	{Code: CodeDoctorPluginGroupSignalFailed},
	{Code: CodeDoctorPluginStderrDrainUnbounded},
}

func init() {
	registerDoctorCodes()
	// recordStoppedLocked (stopped.go) raises doctor-plugin-stopped on every
	// path that leaves a configured plugin down, the respawn limit, a policy
	// exit, a failed restart and a failed handshake; the process manager owns
	// its health row (ai/rules/plugins.md).
	health.Register("plugins", report.HealthProbeDown(diagnostic.CodeDoctorPluginStopped))
	for _, event := range processEventCodes {
		if err := diagnostic.RegisterEventCode(event); err != nil {
			panic("BUG: plugin process event code registration refused: " + err.Error())
		}
	}
}
