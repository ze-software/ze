// Design: docs/architecture/doctor-and-health-checks.md -- event codes
// Related: early_stage1.go -- EarlyDeclaration, the outcome of each plugin boot ran before the full parse
// Related: register.go -- serverEventCodes, which declares the two codes these findings carry
// Related: stage1_schema.go -- doctor-plugin-schema-changed, the third schema code
// Related: last_stop.go -- the record a refused start leaves for the next ze doctor
// Related: ../../../../cmd/ze/hub/main.go -- boot calls these between the early Stage 1 and the full parse

package server

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config/yang"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// schemaJoinSubject names the join of the declared modules in an event that no
// one plugin owns: a refused module can clash with the module of another
// plugin, so the refusal can name two.
const schemaJoinSubject = "external-plugin-schema"

// schemalessWhy says what follows from a declaration that carries no YANG
// module, for the operator who wrote a config section meant for that plugin.
const schemalessWhy = "an external plugin brings its config section to ze as a YANG module in its declaration; " +
	"with none declared, ze holds no section of this plugin, and a section written for it is refused as an unknown keyword"

// SchemalessRefusal answers the operator error boot adds, beside the refusal
// of a name no schema defines, for a declared external plugin that sent its
// declaration with no YANG module: the plugin, what follows, and the command
// that shows what the program declares.
//
// The caller MUST add it only beside a refusal config.RefusesUnknownNode holds
// for: a plugin that declares no module is no problem by itself.
//
// A plugin whose modules the process published, and that is named here, brought
// modules that add only commands and rpcs (DeclaresConfig): the problem then
// names those modules, so it never says that the plugin declared none.
func SchemalessRefusal(cfg plugin.PluginConfig) *opererr.Error {
	var problem textbuf.Buffer
	problem.Str("external plugin ").Quoted(cfg.Name)
	var modules []string
	published, _ := yang.PublishedExternalModules()
	for _, module := range published {
		if module.Plugin == cfg.Name {
			modules = append(modules, module.Module)
		}
	}
	if len(modules) == 0 {
		problem.Str(" declared no YANG module, so it defines no config section")
		return opererr.New(problem.String(), schemalessWhy, conformanceFix(cfg))
	}
	problem.Str(" declared YANG modules that define commands and no config (")
	for index, module := range modules {
		if index > 0 {
			problem.Str(", ")
		}
		problem.Quoted(module)
	}
	problem.Str("), so it defines no config section")
	return opererr.New(problem.String(), commandsOnlyWhy, conformanceFix(cfg))
}

// commandsOnlyWhy says what follows from a declaration whose modules add only
// commands and rpcs, for the operator who wrote a config section meant for
// that plugin.
const commandsOnlyWhy = "an external plugin brings its config section to ze as a YANG module whose name ends in -conf, " +
	"or that augments a config node, in its declaration; with none such declared, ze holds no section of this plugin, " +
	"and a section written for it is refused as an unknown keyword"

// ReportUndeclaredSchema logs and raises doctor-plugin-schema-undeclared about
// the declared external plugin named name: ze holds no config schema of it,
// and refusal says why and names the command that fixes it (the Err of the
// plugin's EarlyDeclaration, or SchemalessRefusal).
//
// The warning is never cleared: the schema ze holds changes only when ze
// restarts, and a restarted process holds an empty report bus.
func ReportUndeclaredSchema(name string, refusal error) {
	message, fix := refusalParts(refusal, fixShowWarnings)
	logger().Warn("external plugin gave ze no config schema", "plugin", name, "error", refusal,
		"fix", fix, "code", diagnostic.CodeDoctorPluginSchemaUndeclared)
	report.RaiseWarningFix(serverEventSource, diagnostic.CodeDoctorPluginSchemaUndeclared, fitSubject(name),
		report.FitMessage(message), busFix(fix, fixShowWarnings))
}

// ReportRefusedSchema logs and raises doctor-plugin-schema-refused: the join
// of the modules the declared external plugins brought refused one, and
// refusal names the plugin, the module, the cause and the fix. The caller MUST
// end the start after it: ze does not run with a module it refused.
func ReportRefusedSchema(refusal error) {
	message, fix := refusalParts(refusal, fixShowErrors)
	logger().Error("external plugin YANG module refused, ze does not start", "error", refusal,
		"fix", fix, "code", diagnostic.CodeDoctorPluginSchemaRefused)
	report.RaiseErrorFix(serverEventSource, diagnostic.CodeDoctorPluginSchemaRefused, schemaJoinSubject,
		report.FitMessage(message), busFix(fix, fixShowErrors))
}

// RecordRefusedSchemaStop writes to the last-stop record that this start ended
// because the join refused a declared module: the refusal's problem and why,
// and its fix. The report bus ends with the process, so this record is what
// the next `ze doctor` reads, with no daemon running, to report
// doctor-plugin-schema-refused; the next clean start deletes it
// (Server.settleLastStop). It reports whether the record was written; a failed
// write is logged under the same code.
//
// The caller MUST call it only where the start ends: the record says why ze
// did not start.
func RecordRefusedSchemaStop(refusal error) bool {
	detail, fix := refusalParts(refusal, "")
	return recordLastStopLogged(LastStopEntry{Reason: LastStopSchemaRefused, Detail: detail, Fix: fix})
}

// RecordUndeclaredSchemaStop writes to the last-stop record that this start
// ended on a config name no schema defines while ze held no config schema of
// the declared external plugin named name: refusal says why ze holds none and
// names the fix (the Err of the plugin's EarlyDeclaration, or
// SchemalessRefusal). The next `ze doctor` reads it to report
// doctor-plugin-schema-undeclared with no daemon running, and the next clean
// start deletes it. It reports whether the record was written.
//
// The caller MUST call it only where the load refused such a name and the
// start ends. A plugin that gave no schema refuses nothing by itself, and a
// daemon that starts reports it from the report bus (ReportUndeclaredSchema).
func RecordUndeclaredSchemaStop(name string, refusal error) bool {
	detail, fix := refusalParts(refusal, "")
	return recordLastStopLogged(LastStopEntry{Reason: LastStopSchemaUndeclared, Plugin: name, Detail: detail, Fix: fix})
}

// refusalParts answers the one-line message and the fix of refusal for an
// event: the problem and why of the operator error its chain carries, with
// that error's fix, and the error's own text with fallback when the chain
// carries none.
func refusalParts(refusal error, fallback string) (message, fix string) {
	operator, ok := errors.AsType[*opererr.Error](refusal)
	if !ok {
		return refusal.Error(), fallback
	}
	var text textbuf.Buffer
	text.Str(operator.Problem()).Str(". ").Str(operator.Why())
	return text.String(), operator.Fix()
}
