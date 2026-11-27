// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/plugin/process.

package process

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorPluginCleanup = "doctor-plugin-cleanup"
	// CodeDoctorPluginStderrRelayStopped is the doctor diagnostic code "doctor-plugin-stderr-relay-stopped".
	CodeDoctorPluginStderrRelayStopped = "doctor-plugin-stderr-relay-stopped"
	// CodeDoctorPluginRestartOverlap is the doctor diagnostic code "doctor-plugin-restart-overlap".
	CodeDoctorPluginRestartOverlap = "doctor-plugin-restart-overlap"
	// CodeDoctorPluginStopUnfinished is the doctor diagnostic code "doctor-plugin-stop-unfinished".
	CodeDoctorPluginStopUnfinished = "doctor-plugin-stop-unfinished"
	// CodeDoctorPluginDeliveryFailed is the doctor diagnostic code "doctor-plugin-delivery-failed".
	CodeDoctorPluginDeliveryFailed = "doctor-plugin-delivery-failed"
	// CodeDoctorPluginStopCleanupLate is the doctor diagnostic code "doctor-plugin-stop-cleanup-late".
	CodeDoctorPluginStopCleanupLate = "doctor-plugin-stop-cleanup-late"
	// CodeDoctorPluginShutdownOrder is the doctor diagnostic code "doctor-plugin-shutdown-order".
	CodeDoctorPluginShutdownOrder = "doctor-plugin-shutdown-order"
	// CodeDoctorPluginGroupSignalFailed is the doctor diagnostic code "doctor-plugin-group-signal-failed".
	CodeDoctorPluginGroupSignalFailed = "doctor-plugin-group-signal-failed"
	// CodeDoctorPluginStderrDrainUnbounded is the doctor diagnostic code "doctor-plugin-stderr-drain-unbounded".
	CodeDoctorPluginStderrDrainUnbounded = "doctor-plugin-stderr-drain-unbounded"
	exampleShowDoctor                    = "ze show doctor"
	exampleShowErrors                    = "ze show errors"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorPluginCleanup,
		Title:       "Plugin resource cleanup is unconfirmed",
		Description: "A plugin that declares terminal cleanup ownership ended without confirming resource release. The daemon retains the original process's failure cause and exit outcome after its RPC connection ends. A healthy replacement cannot clear that warning. Inspect the retained cause and confirm the original resources are released before restarting service.",
		Examples:    []string{exampleShowDoctor, "ze show warnings"},
	},
	{
		Code:  CodeDoctorPluginStderrRelayStopped,
		Title: "Plugin error output no longer logged",
		Description: "The relay that copies a plugin's standard error into ze's log stopped while the plugin was running, so what the plugin writes there is lost. " +
			"The event names the plugin and the cause.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-plugin-stderr-relay-stopped"},
	},
	{
		Code:  CodeDoctorPluginRestartOverlap,
		Title: "Plugin restarted beside its previous process",
		Description: "When ze restarted a plugin, the process it replaced had not finished stopping within its bound (2s after the stop for a forked plugin, 1s for one that runs in the daemon), so the replacement started anyway. " +
			"The event says what was left. An engine that had not returned, or a forked process that had not exited, may hold the same resources as the replacement, such as sockets, routes or kernel state. " +
			"When the engine or the process had ended, only its event delivery or the relay of its stderr was left, and a process the plugin started may hold that stderr open. " +
			"ze takes no further step to end what is left. The fix is the command that shows it: the goroutine dump for a plugin that runs in the daemon, and the relayed plugin output for a forked plugin. A second restart replaces the new process, not the previous one.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-plugin-restart-overlap"},
	},
	{
		Code:  CodeDoctorPluginStopUnfinished,
		Title: "Plugin stopped at boot had not finished stopping",
		Description: "At boot ze starts each external plugin the configuration declares before it parses the whole configuration, to read the plugin's own config schema. " +
			"It stops one that sent no declaration, one the parsed configuration no longer names, and the others of a startup phase that was refused. " +
			"This plugin had not finished stopping within its bound (2s after the stop for a forked plugin, 1s for one that runs in the daemon), and ze went on and removed it from the plugins it runs. " +
			"The event says what was left. A forked process that had not exited was sent the kill for its process group and may still be running; " +
			"when the process had ended, only its event delivery or the relay of its stderr was left, and a process the plugin started may hold that stderr open. " +
			"ze takes no further step to end what is left. The fix is the command that shows it: the relayed plugin output for a forked plugin, and the goroutine dump for a plugin that runs in the daemon.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-plugin-stop-unfinished"},
	},
	{
		Code:  CodeDoctorPluginDeliveryFailed,
		Title: "Event delivery to a plugin failing or slow",
		Description: "Batches of events sent to a plugin failed, or took more than 500 ms to deliver, in the recent window, so the plugin runs behind or without those events. " +
			"The row names the plugin, the failures and the slowest delivery.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-delivery-failed"},
	},
	{
		Code:  CodeDoctorPluginStopCleanupLate,
		Title: "Cleanup at stop did not finish",
		Description: "When ze last stopped, a plugin engine or the config transaction did not finish its cleanup in time, so state it installed may still be present. " +
			"The reason is recorded before the stop and reported on the next run. Each owner's own doctor check reports the state that was left behind.",
		Examples: []string{"ze doctor --json", "ze show diagnostic doctor-plugin-stop-cleanup-late"},
	},
	{
		Code:  CodeDoctorPluginShutdownOrder,
		Title: "Shutdown dependency order could not be preserved",
		Description: "The stop could not resolve the selected plugins' dependency order, or was given no resolver, so it stopped the remaining plugins together. " +
			"A provider may therefore have stopped before its consumers finished cleanup. This does not mean cleanup ran late or installed state was left behind. " +
			"The report event names the cause, and the daemon writes it to its last-stop record so local doctor can report it after the stop. Read journalctl -u ze before correcting the plugin declarations or server wiring.",
		Examples: []string{"ze doctor --json", exampleShowErrors, "ze show diagnostic doctor-plugin-shutdown-order"},
	},
	{
		Code:  CodeDoctorPluginGroupSignalFailed,
		Title: "Plugin process group could not be signaled",
		Description: "When ze stopped or aborted a forked plugin, sending SIGKILL to its process group failed with an error other than ESRCH, " +
			"so the plugin or child processes it started may still be running and holding sockets or other host resources. " +
			"The event names the plugin and the kernel error. Inspect the plugin's relayed output and recent errors before restarting it.",
		Examples:     []string{exampleShowDoctor, exampleShowErrors, restartOverlapRelayLogFix, "ze show diagnostic doctor-plugin-group-signal-failed"},
		RelatedCodes: []string{CodeDoctorPluginRestartOverlap, CodeDoctorPluginStopUnfinished},
	},
	{
		Code:  CodeDoctorPluginStderrDrainUnbounded,
		Title: "Plugin stderr drain bound could not be armed",
		Description: "After forking or reaping a forked plugin, ze could not close its parent copy of the stderr write pipe or set the post-reap idle read deadline on the stderr pipe, " +
			"so a lingering child holding stderr open can delay stop or restart waiting on the stderr relay. " +
			"The event names the plugin and the operating system error. Inspect recent errors and relayed plugin output.",
		Examples:     []string{exampleShowDoctor, exampleShowErrors, restartOverlapRelayLogFix, "ze show diagnostic doctor-plugin-stderr-drain-unbounded"},
		RelatedCodes: []string{CodeDoctorPluginStderrRelayStopped, CodeDoctorPluginRestartOverlap},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: process: doctor code registration refused: " + err.Error())
		}
	}
}
