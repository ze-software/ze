// Design: docs/architecture/doctor-and-health-checks.md -- the cross-cutting codes every area child names
// Related: codes.go -- builtinCodes, which appends this table
// Related: event_codes.go -- RegisterEventCode, which an owner of a bus-raised code calls
//
// The plugin infrastructure and core codes. The first ten are the
// cross-cutting codes every area of the doctor coverage work names, so they
// exist before the checks and the raises that carry them; the rest are the
// problem classes of the plugin infrastructure and of core. A code an owner
// raises on the report bus is also registered as an event code from that
// owner's init(), so the recent-events check declares it.

package diagnostic

// The spelling of each code, declared once (see the const block in codes.go).
const (
	CodeDoctorInternalDefect                = "doctor-internal-defect"
	CodeDoctorPluginStopped                 = "doctor-plugin-stopped"
	CodeDoctorPluginCrashed                 = "doctor-plugin-crashed"
	CodeDoctorPluginConfigUnapplied         = "doctor-plugin-config-unapplied"
	CodeDoctorPluginSetupFailed             = "doctor-plugin-setup-failed"
	CodeDoctorPluginStartupPhaseFailed      = "doctor-plugin-startup-phase-failed"
	CodeDoctorPluginExternalUnsupported     = "doctor-plugin-external-unsupported"
	CodeDoctorAuditRecordFailed             = "doctor-audit-record-failed"
	CodeDoctorPluginRequestRefused          = "doctor-plugin-request-refused"
	CodeDoctorPluginStopCleanupLate         = "doctor-plugin-stop-cleanup-late"
	CodeDoctorPluginAnswerUndelivered       = "doctor-plugin-answer-undelivered"
	CodeDoctorPluginDrainIncomplete         = "doctor-plugin-drain-incomplete"
	CodeDoctorReportBusFull                 = "doctor-report-bus-full"
	CodeDoctorPluginFatalStop               = "doctor-plugin-fatal-stop"
	CodeDoctorPluginAutoloadUnresolved      = "doctor-plugin-autoload-unresolved"
	CodeDoctorPluginCommandNotRegistered    = "doctor-plugin-command-not-registered"
	CodeDoctorPluginSubscriptionDropped     = "doctor-plugin-subscription-dropped"
	CodeDoctorPluginDeliveryDisagreement    = "doctor-plugin-delivery-disagreement"
	CodeDoctorPluginClaimUnbacked           = "doctor-plugin-claim-unbacked"
	CodeDoctorPluginPostStartupUndelivered  = "doctor-plugin-poststartup-undelivered"
	CodeDoctorPluginPanic                   = "doctor-plugin-panic"
	CodeDoctorPluginDeliveryFailed          = "doctor-plugin-delivery-failed"
	CodeDoctorPluginStderrRelayStopped      = "doctor-plugin-stderr-relay-stopped"
	CodeDoctorPluginRestartOverlap          = "doctor-plugin-restart-overlap"
	CodeDoctorPluginEnricherUnavailable     = "doctor-plugin-enricher-unavailable"
	CodeDoctorPluginConnectBackRefused      = "doctor-plugin-connect-back-refused"
	CodeDoctorManagedListenerUnbound        = "doctor-managed-listener-unbound"
	CodeDoctorManagedClientRejected         = "doctor-managed-client-rejected"
	CodeDoctorManagedClientNotifyDropped    = "doctor-managed-client-notify-dropped"
	CodeDoctorPluginRemovalUnrecovered      = "doctor-plugin-removal-unrecovered"
	CodeDoctorPluginCapabilitiesNotRestored = "doctor-plugin-capabilities-not-restored"
	CodeDoctorCommandEnsureRollbackFailed   = "doctor-command-ensure-rollback-failed"
	CodeDoctorCommandDeprecatedUsed         = "doctor-command-deprecated-used"
	CodeDoctorPluginNoDataPlaneWriter       = "doctor-plugin-no-dataplane-writer"
	CodeDoctorEnvInvalidValue               = "doctor-env-invalid-value"
	CodeDoctorConfigReloadRefused           = "doctor-config-reload-refused"
)

// exampleShowErrors lists the report bus's recent error events, the record
// behind every event code in this table.
const exampleShowErrors = "ze show errors"

// codeDoctorHubManagedCollision is the managed listener's collision code,
// named here for a RelatedCodes cross-reference.
const codeDoctorHubManagedCollision = "doctor-hub-managed-collision"

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
		Code:  CodeDoctorPluginConfigUnapplied,
		Title: "Plugin was not told a config change committed",
		Description: "The daemon committed a config transaction and the commit notice did not reach a plugin, so the plugin still runs the configuration it had before. " +
			"The row names the plugin, the last transaction it was told of and the daemon's. Restart the plugin (ze request plugin restart <name>), which starts it again with the running configuration; " +
			"a reload does not clear the row, because a reload of an unchanged configuration starts no transaction.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-config-unapplied"},
	},
	{
		Code:  CodeDoctorPluginSetupFailed,
		Title: "Plugin registration or setup failed",
		Description: "A plugin's registration or setup, which runs when the binary starts and before any config is read, failed and recorded why. " +
			"A hard failure stops the daemon from starting; a soft one leaves the plugin without the part that failed. " +
			"The row names the plugin and the recorded reason, and the reason names its fix.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-setup-failed"},
		RelatedCodes: []string{CodeDoctorPluginStartupPhaseFailed},
	},
	{
		Code:  CodeDoctorPluginStartupPhaseFailed,
		Title: "Plugin failed its startup",
		Description: "A plugin failed one of the five startup stages it runs with the daemon, or its run returned before it reported ready. " +
			"One plugin failure stops the startup tier it belongs to, so the plugins after it in that tier did not start either. " +
			"The row names the plugin and the stage; the plugin's log lines give the cause.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-startup-phase-failed"},
		RelatedCodes: []string{CodeDoctorPluginSetupFailed},
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
	{
		Code:  CodeDoctorPluginStopCleanupLate,
		Title: "Cleanup at stop did not finish",
		Description: "When ze last stopped, a plugin engine or the config transaction did not finish its cleanup in time, so state it installed may still be present. " +
			"The reason is recorded before the stop and reported on the next run. Each owner's own doctor check reports the state that was left behind.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-stop-cleanup-late"},
	},

	// The plugin infrastructure classes.
	{
		Code:  CodeDoctorPluginAnswerUndelivered,
		Title: "Answer to a plugin not delivered",
		Description: "The daemon could not deliver its answer to a plugin's request, so the plugin waits for it until its timeout or goes on without it. " +
			"The event names the plugin and the request.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-plugin-answer-undelivered"},
	},
	{
		Code:  CodeDoctorPluginDrainIncomplete,
		Title: "Plugin work not drained",
		Description: "At a stop or a reload the daemon did not finish draining the work queued for a plugin before it went on, so what was still queued was dropped. " +
			"The event names the plugin and what was left.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-plugin-drain-incomplete"},
	},
	{
		Code:  CodeDoctorReportBusFull,
		Title: "Report bus dropped entries",
		Description: "The report bus reached its warning cap and dropped its oldest active warning, or its error ring wrapped and overwrote its oldest error event. " +
			"show warnings, show errors and the health rows that read the bus no longer see what was dropped. " +
			"Raise ze.report.warnings.max or ze.report.errors.max, both clamped to 10000.",
		Examples: []string{exampleShowDoctor, "ze show warnings", "ze show diagnostic doctor-report-bus-full"},
	},
	{
		Code:  CodeDoctorPluginFatalStop,
		Title: "Ze stopped because a plugin failed",
		Description: "When ze last ran, a plugin whose failure policy is fatal failed, or the boot refused a plugin, and ze stopped itself. " +
			"The record written before the stop names the plugin, the reason and the time. Read that plugin's log lines, then change its configuration or its failure policy.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-fatal-stop"},
		RelatedCodes: []string{CodeDoctorPluginStopCleanupLate},
	},
	{
		Code:  CodeDoctorPluginAutoloadUnresolved,
		Title: "Plugin dependencies missing from this build",
		Description: "The configuration selects a plugin that depends on another plugin this binary does not carry, because its build tag compiled it out. " +
			"The daemon refuses to start with this configuration. The row names the plugin, the dependency and the build tag: " +
			"use a build that carries the dependency, or remove the configuration that selects the plugin.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-autoload-unresolved"},
	},
	{
		Code:  CodeDoctorPluginCommandNotRegistered,
		Title: "Plugin command not registered",
		Description: "A running plugin declared a command, or a deprecated alias of one, whose name another plugin already owns, so the command was refused and its name reaches the other plugin's handler. " +
			"The row names both plugins and the command. Remove or rename the command in one of them.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-command-not-registered"},
	},
	{
		Code:  CodeDoctorPluginSubscriptionDropped,
		Title: "Plugin event subscription dropped",
		Description: "A plugin asked for events in a namespace the daemon does not know, gave no default namespace, or sent ready parameters that did not parse, " +
			"so the daemon dropped the subscription and the plugin receives none of those events. The row names the plugin and the cause.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-subscription-dropped"},
	},
	{
		Code:  CodeDoctorPluginDeliveryDisagreement,
		Title: "Event delivery and attach process disagree",
		Description: "The `attach process` configuration and the events the plugins declare disagree, so some events are sent to a plugin that does not handle them, or never reach the plugin that does. " +
			"The row names each disagreement. Change the attach process configuration to match the plugin.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-delivery-disagreement"},
	},
	{
		Code:  CodeDoctorPluginClaimUnbacked,
		Title: "Exclusive role claimed and nobody performs it",
		Description: "The configuration needs an exclusive role that a plugin claims, and no plugin holding that claim is running, so nothing performs the role. " +
			"The row names the role and the plugin that claims it. Start that plugin, or remove the configuration that needs the role.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-claim-unbacked"},
	},
	{
		Code:  CodeDoctorPluginPostStartupUndelivered,
		Title: "Plugin never got its post-startup callback",
		Description: "A running plugin finished its startup, and the callback that tells it every plugin is running never reached it, " +
			"so the work a plugin starts at that point, such as a command to another plugin, never ran. The row names the plugin.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-poststartup-undelivered"},
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
		Code:  CodeDoctorPluginDeliveryFailed,
		Title: "Event delivery to a plugin failing or slow",
		Description: "Batches of events sent to a plugin failed, or took more than 500 ms to deliver, in the recent window, so the plugin runs behind or without those events. " +
			"The row names the plugin, the failures and the slowest delivery.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-delivery-failed"},
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
		Code:  CodeDoctorPluginEnricherUnavailable,
		Title: "Show command answers without a plugin's fields",
		Description: "A plugin's enricher, which adds the plugin's fields to a show command's answer, failed to register or failed when it was called, " +
			"so that command answers without them. The row names the plugin, the command and the field.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-enricher-unavailable"},
	},
	{
		Code:  CodeDoctorPluginConnectBackRefused,
		Title: "External plugin connection refused",
		Description: "An external plugin connected back to the daemon and was refused, because its name is not a valid plugin name, it is not a plugin the daemon started, or its token did not match. " +
			"The plugin does not start, and its start waits until it times out. Correct the name in the plugin's `external` block.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, "ze show diagnostic doctor-plugin-connect-back-refused"},
	},
	{
		Code:  CodeDoctorManagedListenerUnbound,
		Title: "Managed client listener did not bind",
		Description: "A hub server block declares managed clients, and its address could not be bound for a reason other than the plugin acceptor holding it, " +
			"so managed clients that dial that address are not served. The row names the address and the kernel's error.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic doctor-managed-listener-unbound"},
		RelatedCodes: []string{codeDoctorHubManagedCollision},
	},
	{
		Code:  CodeDoctorManagedClientRejected,
		Title: "Managed client refused or rejected its config",
		Description: "A managed client was refused because a client of the same name is already connected, or the client rejected the configuration the daemon sent it, " +
			"so it runs without its current configuration. The row names the client and its last answer.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-managed-client-rejected"},
	},
	{
		Code:  CodeDoctorManagedClientNotifyDropped,
		Title: "Managed client not told of a config change",
		Description: "The daemon could not tell a managed client that its configuration changed, because the notice queue was full or the notice could not be built, " +
			"so the client keeps the configuration it had. The event names the client and the cause.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-managed-client-notify-dropped"},
	},
	{
		Code:  CodeDoctorPluginRemovalUnrecovered,
		Title: "Plugin stopped by a refused reload was not restored",
		Description: "A reload that removed a plugin was refused, and the daemon could not start again the plugin it had stopped, so the plugin stays stopped while the configuration still names it. " +
			"Reload the configuration to retry.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-removal-unrecovered"},
		RelatedCodes: []string{CodeDoctorPluginStopped},
	},
	{
		Code:  CodeDoctorPluginCapabilitiesNotRestored,
		Title: "Plugin capabilities not restored after a refused reload",
		Description: "A reload was refused and the daemon could not restore the capabilities the plugins had declared before it, " +
			"so what the daemon advertises may not match the running configuration until the next reload commits.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-capabilities-not-restored"},
	},
	{
		Code:  CodeDoctorCommandEnsureRollbackFailed,
		Title: "Config parent created for a command was not removed",
		Description: "A command created a missing config parent before it ran, the command failed, and removing that parent again failed, so an empty parent stays in the configuration. " +
			"The event names its path; delete it with the delete command for that path.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-command-ensure-rollback-failed"},
	},
	{
		Code:  CodeDoctorCommandDeprecatedUsed,
		Title: "Deprecated command name used",
		Description: "A client ran a command by a deprecated name. It still runs, as an alias of the current name, which the event names. " +
			"Change the script or client to use the current name.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-command-deprecated-used"},
	},
	{
		Code:  CodeDoctorPluginNoDataPlaneWriter,
		Title: "Data plane has no writer",
		Description: "A configured plugin produces routes for a data plane, and no plugin in the configuration programs that data plane, or its writer is compiled out of this build. " +
			"The daemon refuses to start or reload with this configuration. The row names the producer and the data plane: add a fib block, or configure the writer for that data plane.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-no-dataplane-writer"},
	},
	{
		Code:  CodeDoctorConfigReloadRefused,
		Title: "Configuration reload refused",
		Description: "The daemon refused a configuration reload: the configuration transaction did not commit (a plugin refused the change or did not answer its verify or apply step), " +
			"a plugin the new configuration needs could not be resolved or did not start, or the daemon refused the configuration once those plugins had started. " +
			"The daemon stops the plugins the reload started and keeps the configuration it was running; an event raised while it restored that configuration says the restore did not complete. " +
			"The event says what refused the change and why: correct the configuration and reload again.",
		Examples:     []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-config-reload-refused"},
		RelatedCodes: []string{CodeConfigPluginVerify, CodeDoctorPluginStopped},
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
