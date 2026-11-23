// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/plugin/server.

package server

import (
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const (
	// CodeDoctorPluginAnswerUndelivered is the doctor diagnostic code "doctor-plugin-answer-undelivered".
	CodeDoctorPluginAnswerUndelivered = "doctor-plugin-answer-undelivered"
	// CodeDoctorPluginDrainIncomplete is the doctor diagnostic code "doctor-plugin-drain-incomplete".
	CodeDoctorPluginDrainIncomplete = "doctor-plugin-drain-incomplete"
	// CodeDoctorPluginSchemaChanged is the doctor diagnostic code "doctor-plugin-schema-changed".
	CodeDoctorPluginSchemaChanged = "doctor-plugin-schema-changed"
	// CodeDoctorManagedClientNotifyDropped is the doctor diagnostic code "doctor-managed-client-notify-dropped".
	CodeDoctorManagedClientNotifyDropped = "doctor-managed-client-notify-dropped"
	// CodeDoctorCommandEnsureRollbackFailed is the doctor diagnostic code "doctor-command-ensure-rollback-failed".
	CodeDoctorCommandEnsureRollbackFailed = "doctor-command-ensure-rollback-failed"
	// CodeDoctorCommandDeprecatedUsed is the doctor diagnostic code "doctor-command-deprecated-used".
	CodeDoctorCommandDeprecatedUsed = "doctor-command-deprecated-used"
	// CodeDoctorConfigReloadRefused is the doctor diagnostic code "doctor-config-reload-refused".
	CodeDoctorConfigReloadRefused = "doctor-config-reload-refused"
	// CodeDoctorHubManagedCollision is the doctor diagnostic code "doctor-hub-managed-collision".
	CodeDoctorHubManagedCollision = "doctor-hub-managed-collision"
	// CodeDoctorPluginConfigUnapplied is the doctor diagnostic code "doctor-plugin-config-unapplied".
	CodeDoctorPluginConfigUnapplied = "doctor-plugin-config-unapplied"
	// CodeDoctorPluginFatalStop is the doctor diagnostic code "doctor-plugin-fatal-stop".
	CodeDoctorPluginFatalStop = "doctor-plugin-fatal-stop"
	// CodeDoctorPluginCommandNotRegistered is the doctor diagnostic code "doctor-plugin-command-not-registered".
	CodeDoctorPluginCommandNotRegistered = "doctor-plugin-command-not-registered"
	// CodeDoctorPluginSubscriptionDropped is the doctor diagnostic code "doctor-plugin-subscription-dropped".
	CodeDoctorPluginSubscriptionDropped = "doctor-plugin-subscription-dropped"
	// CodeDoctorPluginDeliveryDisagreement is the doctor diagnostic code "doctor-plugin-delivery-disagreement".
	CodeDoctorPluginDeliveryDisagreement = "doctor-plugin-delivery-disagreement"
	// CodeDoctorPluginClaimUnbacked is the doctor diagnostic code "doctor-plugin-claim-unbacked".
	CodeDoctorPluginClaimUnbacked = "doctor-plugin-claim-unbacked"
	// CodeDoctorPluginPostStartupUndelivered is the doctor diagnostic code "doctor-plugin-poststartup-undelivered".
	CodeDoctorPluginPostStartupUndelivered = "doctor-plugin-poststartup-undelivered"
	// CodeDoctorPluginSchemaUndeclared is the doctor diagnostic code "doctor-plugin-schema-undeclared".
	CodeDoctorPluginSchemaUndeclared = "doctor-plugin-schema-undeclared"
	// CodeDoctorPluginSchemaRefused is the doctor diagnostic code "doctor-plugin-schema-refused".
	CodeDoctorPluginSchemaRefused = "doctor-plugin-schema-refused"
	// CodeDoctorPluginEnricherUnavailable is the doctor diagnostic code "doctor-plugin-enricher-unavailable".
	CodeDoctorPluginEnricherUnavailable = "doctor-plugin-enricher-unavailable"
	// CodeDoctorManagedListenerUnbound is the doctor diagnostic code "doctor-managed-listener-unbound".
	CodeDoctorManagedListenerUnbound = "doctor-managed-listener-unbound"
	// CodeDoctorManagedClientRejected is the doctor diagnostic code "doctor-managed-client-rejected".
	CodeDoctorManagedClientRejected = "doctor-managed-client-rejected"
	// CodeDoctorPluginRemovalUnrecovered is the doctor diagnostic code "doctor-plugin-removal-unrecovered".
	CodeDoctorPluginRemovalUnrecovered = "doctor-plugin-removal-unrecovered"
	// CodeDoctorPluginCapabilitiesNotRestored is the doctor diagnostic code "doctor-plugin-capabilities-not-restored".
	CodeDoctorPluginCapabilitiesNotRestored = "doctor-plugin-capabilities-not-restored"
	exampleShowDoctor                       = "ze show doctor"
	exampleShowErrors                       = "ze show errors"
	exampleDoctorJSON                       = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
// ipc.ConnectBackRefusedCode is registered here because diagnostic imports plugin/ipc
// through internal/component/plugin, so plugin/ipc cannot import diagnostic.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  ipc.ConnectBackRefusedCode,
		Title: "External plugin connection refused",
		Description: "An external plugin connected back to the daemon and was refused, because its name is not a valid plugin name, it is not a plugin the daemon started, or its token did not match. " +
			"The plugin does not start, and its start waits until it times out. Correct the name in the plugin's `external` block.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, "ze show diagnostic doctor-plugin-connect-back-refused"},
	},
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
		Code:  CodeDoctorPluginSchemaChanged,
		Title: "Restarted plugin declared other YANG modules",
		Description: "ze reads the YANG modules of an external plugin (its config, command and rpc modules) once, when ze starts, joins them to the schema of the whole configuration and to the command tree, and every reader of the configuration and of the commands keeps them. " +
			"This plugin was started again while ze ran, after it ended or by `request plugin restart`, and its new declaration carries other modules: another module text, name or namespace, a module where it declared none, or none where it declared one. " +
			"ze refused the plugin at its first startup stage and does not run it. The running configuration, the modules ze holds and every other plugin are unchanged. " +
			"The warning says what differs. Restart ze to load the new modules, or put back the program that declares the modules ze started with and restart the plugin.",
		Examples: []string{exampleShowDoctor, "ze show warnings", "ze show diagnostic doctor-plugin-schema-changed"},
	},
	{
		Code:  CodeDoctorManagedClientNotifyDropped,
		Title: "Managed client not told of a config change",
		Description: "The daemon could not tell a connected managed client that its configuration changed, because the notice could not be built or the client did not accept it, " +
			"so the client keeps the configuration it had until it fetches again. A client that did not accept the notice is disconnected, and it fetches when it reconnects. " +
			"The event names the client and the cause.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-managed-client-notify-dropped"},
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
		Code:  CodeDoctorConfigReloadRefused,
		Title: "Configuration reload refused",
		Description: "The daemon refused a configuration reload: the configuration transaction did not commit (a plugin refused the change or did not answer its verify or apply step), " +
			"a plugin the new configuration needs could not be resolved or did not start, the daemon refused the configuration once those plugins had started, " +
			"or the new configuration declares other external plugins than the ones the daemon started with, which change only when ze restarts. " +
			"The daemon stops the plugins the reload started and keeps the configuration it was running; an event raised while it restored that configuration says the restore did not complete. " +
			"The event says what refused the change and why: correct the configuration and reload again.",
		Examples:     []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-config-reload-refused"},
		RelatedCodes: []string{diagnostic.CodeConfigPluginVerify, diagnostic.CodeDoctorPluginStopped},
	},
	{
		Code:  CodeDoctorHubManagedCollision,
		Title: "Managed client listener cannot bind",
		Description: "A plugin/hub/server block declares managed clients on the address the plugin acceptor binds. " +
			"The acceptor takes the FIRST server block and the managed listener takes every block that declares client entries, so a shared address is asked of the kernel twice. " +
			"The acceptor binds it, the managed listener is skipped, and every managed client that dials that address is dropped by an acceptor which routes only to a plugin waiter. " +
			"Move the client entries to a server block with its own port.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-hub-managed-collision"},
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
		Code:  CodeDoctorPluginFatalStop,
		Title: "Ze stopped because a plugin failed",
		Description: "When ze last ran, a plugin whose failure policy is fatal failed, or the boot refused a plugin, and ze stopped itself. " +
			"The record written before the stop names the plugin, the reason and the time. Read that plugin's log lines, then change its configuration or its failure policy.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-fatal-stop"},
		RelatedCodes: []string{"doctor-plugin-stop-cleanup-late"},
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
		Code:  CodeDoctorPluginSchemaUndeclared,
		Title: "External plugin gave ze no config schema",
		Description: "At start ze runs each external plugin the configuration declares before it parses the whole configuration, and reads the plugin's config schema (its YANG module) from the plugin's declaration. " +
			"ze holds no schema of this plugin: its program could not be started, it sent no declaration within its bound, its connection ended first, or its declaration carries no YANG module. " +
			"A configuration section that only this plugin's schema defines is then an unknown keyword, and ze refuses to start with it; the refusal names this code beside the unknown keyword, and `ze doctor` reports the plugin from the record that refusal leaves, until ze next starts cleanly. " +
			"A configuration with no such section starts, the plugin that sent no declaration does not run, and the warning stays until ze restarts. " +
			"The fix is the conformance check of the plugin's run line, which starts the program the way ze does and shows what its declaration lacks. Correct the program or the run line, then restart ze.",
		Examples:     []string{exampleShowDoctor, "ze show warnings", "ze show diagnostic doctor-plugin-schema-undeclared"},
		RelatedCodes: []string{diagnostic.CodeDoctorPluginStopped, CodeDoctorPluginSchemaRefused},
	},
	{
		Code:  CodeDoctorPluginSchemaRefused,
		Title: "External plugin's YANG module refused at start",
		Description: "At start ze joins the YANG modules each external plugin declares (its config, command and rpc modules) to the modules compiled into ze, before it parses the configuration. " +
			"ze refused a module: it does not compile, its name or namespace is held by another module, it defines a top-level node another module already defines, " +
			"it names a validator or a reference source ze does not hold, the name or namespace the plugin declared is not the one in the text, " +
			"it declares a command path or a wire method another module declares, a ze:rpc names an rpc no module defines, " +
			"or it declares definitions ze reads nowhere (config outside a -conf module, commands outside a -cmd module, a typedef no module imports). " +
			"ze does not start with a module it refused: the refusal ends the start, on the terminal and in the log, and names the plugin, the module, the cause and the command that shows it. " +
			"No daemon runs afterwards. ze records the refusal before it exits, and `ze doctor` reports it from that record, with the time, until ze next starts cleanly. Correct the plugin's module, then start ze again.",
		Examples:     []string{"ze doctor", "ze show diagnostic doctor-plugin-schema-refused", "ze show plugin conformance run \"./my-plugin\""},
		RelatedCodes: []string{CodeDoctorPluginSchemaUndeclared, CodeDoctorPluginSchemaChanged, diagnostic.CodeDoctorBuildRegistrationUnheld},
	},
	{
		Code:  CodeDoctorPluginEnricherUnavailable,
		Title: "Show command answers without a plugin's fields",
		Description: "A plugin's enricher, which adds the plugin's fields to a show command's answer, failed to register or failed when it was called, " +
			"so that command answers without them. The row names the plugin, the command and the field.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-enricher-unavailable"},
	},
	{
		Code:  CodeDoctorManagedListenerUnbound,
		Title: "Managed client listener did not bind",
		Description: "A hub server block declares managed clients, and its address could not be bound for a reason other than the plugin acceptor holding it, " +
			"so managed clients that dial that address are not served. The row names the address and the kernel's error.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic doctor-managed-listener-unbound"},
		RelatedCodes: []string{CodeDoctorHubManagedCollision},
	},
	{
		Code:  CodeDoctorManagedClientRejected,
		Title: "Managed client refused or rejected its config",
		Description: "A managed client was refused because a client of the same name is already connected, or the client rejected the configuration the daemon sent it, " +
			"so it runs without its current configuration. The row names the client and its last answer.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-managed-client-rejected"},
	},
	{
		Code:  CodeDoctorPluginRemovalUnrecovered,
		Title: "Plugin stopped by a refused reload was not restored",
		Description: "A reload that removed a plugin was refused, and the daemon could not start again the plugin it had stopped, so the plugin stays stopped while the configuration still names it. " +
			"Reload the configuration to retry.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-removal-unrecovered"},
		RelatedCodes: []string{diagnostic.CodeDoctorPluginStopped},
	},
	{
		Code:  CodeDoctorPluginCapabilitiesNotRestored,
		Title: "Plugin capabilities not restored after a refused reload",
		Description: "A reload was refused and the daemon could not restore the capabilities the plugins had declared before it, " +
			"so what the daemon advertises may not match the running configuration until the next reload commits.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-plugin-capabilities-not-restored"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: server: doctor code registration refused: " + err.Error())
		}
	}
}
