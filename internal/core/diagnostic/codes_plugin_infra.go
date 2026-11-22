// Design: docs/architecture/doctor-and-health-checks.md -- cross-cutting doctor codes shared across unrelated packages
// Related: codes.go -- builtinCodes, which appends this table
// Related: event_codes.go -- RegisterEventCode, which an owner of a bus-raised code calls
//
// Cross-cutting doctor codes shared by unrelated packages that cannot import
// each other without introducing an import cycle. Single-owner plugin-infra
// codes live and register in their own packages (plugin/ipc, plugin/process,
// plugin/server, plugin/doctor). A code an owner raises on the report bus is
// also registered as an event code from that owner's init(), so the
// recent-events check declares it.

package diagnostic

// The spelling of each code, declared once (see the const block in codes.go).
const (
	CodeDoctorInternalDefect            = "doctor-internal-defect"
	CodeDoctorPluginStopped             = "doctor-plugin-stopped"
	CodeDoctorPluginCrashed             = "doctor-plugin-crashed"
	CodeDoctorPluginStartupPhaseFailed  = "doctor-plugin-startup-phase-failed"
	CodeDoctorPluginExternalUnsupported = "doctor-plugin-external-unsupported"
	CodeDoctorAuditRecordFailed         = "doctor-audit-record-failed"
	CodeDoctorPluginRequestRefused      = "doctor-plugin-request-refused"
	CodeDoctorReportBusFull             = "doctor-report-bus-full"
	CodeDoctorPluginPanic               = "doctor-plugin-panic"
	CodeDoctorBuildRegistrationUnheld   = "doctor-build-registration-unheld"
	CodeDoctorEnvInvalidValue           = "doctor-env-invalid-value"
)

// exampleShowErrors lists the report bus's recent error events, the record
// behind every event code in this table.
const exampleShowErrors = "ze show errors"

var pluginInfraCodes = []CodeMeta{
	// The cross-cutting codes.
	{
		Code:  CodeDoctorInternalDefect,
		Title: "Internal defect in Ze",
		Description: "A component met a state that only a defect in Ze can produce, such as a config transaction or engine payload it could not decode, " +
			"a registration that failed when the binary started, or a typed event whose payload did not match its registered type. " +
			"The work that met it was dropped and an error event was raised on the report bus. Nothing in the configuration causes it. " +
			"Report it with the log lines around the time ze show errors gives for the event.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-internal-defect"},
	},
	{
		Code:  CodeDoctorPluginStopped,
		Title: "Configured plugin is not running",
		Description: "A plugin the configuration names is not running: it exited under the ignore or an unspecified failure policy, its restart failed, " +
			"its startup handshake failed, or the respawn limit disabled it. Every feature the plugin provides is absent until it runs again. " +
			"The row names the plugin and why it stopped; the plugin's log lines give the cause.",
		Examples:     []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-plugin-stopped"},
		RelatedCodes: []string{CodeDoctorPluginCrashed},
	},
	{
		Code:  CodeDoctorPluginCrashed,
		Title: "Plugin process ended unexpectedly",
		Description: "A plugin process exited when nothing asked it to stop. The event names the plugin, its exit status and the failure policy applied to it. " +
			"A plugin that was restarted may have recovered; one that stays stopped is also reported as doctor-plugin-stopped.",
		Examples:     []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-plugin-crashed"},
		RelatedCodes: []string{CodeDoctorPluginStopped},
	},
	{
		Code:  CodeDoctorPluginStartupPhaseFailed,
		Title: "Plugin failed its startup",
		Description: "A plugin failed one of the five startup stages it runs with the daemon, or its run returned before it reported ready. " +
			"One plugin failure stops the startup tier it belongs to, so the plugins after it in that tier did not start either. " +
			"The row names the plugin and the stage; the plugin's log lines give the cause.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-startup-phase-failed"},
		RelatedCodes: []string{"doctor-plugin-setup-failed"},
	},
	{
		Code:  CodeDoctorPluginExternalUnsupported,
		Title: "Plugin cannot run as an external process",
		Description: "The configuration starts a plugin in an `external` block, and that plugin declares it runs only inside the daemon, because it needs state a separate process cannot reach. " +
			"Run as a subprocess it would start and do nothing. Move it to an `internal` block.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-external-unsupported"},
	},
	{
		Code:  CodeDoctorAuditRecordFailed,
		Title: "Audit record could not be written",
		Description: "Writing an entry to the command audit log failed, so at least one command ran with no audit record. " +
			"The row names the last error and when it happened. Check the free space and the permissions of the storage the audit log writes to.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-audit-record-failed"},
	},
	{
		Code:  CodeDoctorPluginRequestRefused,
		Title: "Plugin request refused",
		Description: "The daemon refused a request a plugin sent: its payload did not decode (a typed event whose payload is not its registered type among them), it named something the daemon does not know, " +
			"it arrived before the plugin's startup allowed it, a command it ran failed, or it raised more problems on the report bus than its rate or its share of active warnings allows. " +
			"The plugin did not get what it asked for and runs without it. The event names the plugin and the reason; " +
			"it is raised at most once per plugin in a minute, so a plugin that repeats a bad request cannot fill the error ring.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-plugin-request-refused"},
	},

	// The plugin infrastructure classes.
	{
		Code:  CodeDoctorReportBusFull,
		Title: "Report bus dropped entries",
		Description: "The report bus reached its warning cap and dropped its oldest active warning, or its error ring wrapped and overwrote its oldest error event. " +
			"show warnings, show errors and the health rows that read the bus no longer see what was dropped. " +
			"Raise ze.report.warnings.max or ze.report.errors.max, both clamped to 10000.",
		Examples: []string{exampleShowDoctor, exampleShowWarnings, "ze show diagnostic doctor-report-bus-full"},
	},
	{
		Code:  CodeDoctorPluginPanic,
		Title: "Event handler failed",
		Description: "An event handler panicked and the daemon recovered it, or a plugin process failed to apply an event, so that event was lost for the handler. " +
			"The row names the plugin, or the engine handler's namespace and event type, and the first error. Report it with the log lines around that time.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-panic"},
		RelatedCodes: []string{CodeDoctorInternalDefect},
	},
	{
		Code:  CodeDoctorBuildRegistrationUnheld,
		Title: "This ze was built without a validator its YANG names",
		Description: "A YANG module compiled into ze names, with ze:validate or ze:reference, a validator function or a reference source that no package linked into this ze registers. " +
			"Such a leaf is not checked at all (ze:validate) or refuses every value (ze:reference), so ze start refuses the build before it parses the configuration: " +
			"the refusal, on the terminal and in the log, names the module, the extension, the name and the schema path. The configuration is not at fault. " +
			"ze records the refusal before it exits, and `ze doctor` reports it from that record until ze next starts cleanly. " +
			"A command that runs with no daemon logs the same names under this code and goes on. Rebuild ze with the package that registers the name linked wherever the module is.",
		Examples:     []string{"ze doctor", "ze show diagnostic doctor-build-registration-unheld"},
		RelatedCodes: []string{"doctor-plugin-schema-refused"},
	},

	// The core class.
	{
		Code:  CodeDoctorEnvInvalidValue,
		Title: "Environment value cannot be used",
		Description: "An environment variable holds a value its registered type cannot parse, so ze used the default instead, or a value outside its range: below its minimum, so ze used the default, or above its maximum, so ze clamped it. " +
			"The row names the key, the value, the type and the value used; a secret key's value is not shown. Correct the export and restart ze.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-env-invalid-value"},
	},
}
