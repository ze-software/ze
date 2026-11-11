// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: register_lifecycle.go -- registers this check at the pre-config phase
// Related: ../registry/setup.go -- RecordSetup and SetupResults, the records it reads

package doctor

import (
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// pluginSetupFixDefault is the fix of a recorded failure whose reason names
// no remedy of its own: the command that replays every recorded outcome.
const pluginSetupFixDefault = "ze show plugin list"

// checkPluginSetup answers one row per plugin whose init() setup recorded a
// failure: an error for a hard failure, which the boot gate refuses, and a
// warning for a soft one, which leaves the plugin running degraded. A plugin
// recorded succeeded, or that recorded nothing, is not reported.
func checkPluginSetup(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return pluginSetupRows(registry.SetupResults())
}

// pluginSetupRows builds the rows from the recorded results.
func pluginSetupRows(results []registry.SetupResult) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for i := range results {
		result := &results[i]
		switch result.Outcome {
		case registry.SetupFailedHard:
			rows = append(rows, pluginSetupRow(result, diagnostic.SeverityError, "so ze refuses to start"))
		case registry.SetupFailedSoft:
			rows = append(rows, pluginSetupRow(result, diagnostic.SeverityWarning, "so the plugin runs degraded"))
		case registry.SetupUnknown, registry.SetupSucceeded:
			// Nothing failed, or nothing was recorded.
		default:
			// RecordSetup refuses an outcome outside the enumeration, so a
			// value here is a registry defect: reported, never skipped.
			rows = append(rows, pluginSetupRow(result, diagnostic.SeverityError, "an outcome ze does not know"))
		}
	}
	return rows
}

// pluginSetupRow writes one failed setup. A reason carries its remedy after
// its last "; " (RecordSetup): the part before is the message, the remedy is
// the fix. A reason with no remedy keeps the whole reason in the message and
// takes the default fix.
func pluginSetupRow(result *registry.SetupResult, severity diagnostic.Severity, effect string) diagnostic.Diagnostic {
	cause, fix := result.Reason, pluginSetupFixDefault
	if at := strings.LastIndex(result.Reason, "; "); at >= 0 {
		cause, fix = result.Reason[:at], result.Reason[at+2:]
	}
	var message textbuf.Buffer
	message.Str("plugin ").Str(result.Plugin).Str(" setup ").Str(result.Outcome.String()).
		Str(", ").Str(effect)
	if cause != "" {
		message.Str(": ").Str(cause)
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginSetupFailed,
		Severity: severity,
		Message:  message.String(),
		Fix:      fix,
	}
}
