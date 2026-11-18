// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: register_lifecycle.go -- registers both checks
// Related: ../server/last_stop.go -- ReadLastStopFrom, the record the fatal-stop check reads
// Related: ../server/startup_autoload.go -- the boot auto-load resolution the auto-load check repeats

package doctor

import (
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// checkPluginFatalStop answers one error per recorded reason the previous run
// of ze stopped for: a plugin whose failure policy is fatal failed, or the
// boot was refused. It reads the record the stop wrote (server.RecordLastStop)
// from the store the check is handed, so it answers on the local route while
// ze is down, which is when an operator asks. A clean start deletes the
// record, so a row means the stop has not been followed by a clean start.
//
// A stop-time cleanup that ran late is reported from the same record as a
// doctor-plugin-stop-cleanup-late warning (AC-32), written by the server when
// the config transaction or a plugin engine ran late (stop_cleanup_late.go).
//
// A shutdown ordering refusal is retained separately as
// doctor-plugin-shutdown-order: it does not imply cleanup ran late.
//
// A start refused for a YANG module an external plugin declared is reported from the
// same record under its own code: doctor-plugin-schema-refused for a module
// the join refused, doctor-plugin-schema-undeclared for a config name no
// schema defines while ze held no schema of a declared plugin
// (server.RecordRefusedSchemaStop, server.RecordUndeclaredSchemaStop).
func checkPluginFatalStop(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Store == nil {
		return nil // The store open diagnostic already names an absent store.
	}
	record, found, err := server.ReadLastStopFrom(ctx.Store)
	if err != nil {
		var message textbuf.Buffer
		message.Str("cannot read why ze last stopped, so a fatal plugin stop would go unreported: ").Err(err)
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorPluginFatalStop,
			Severity: diagnostic.SeverityWarning,
			Message:  message.String(),
			Fix:      "ze data list",
		}}
	}
	// refuse-by-default: guards fatalStopRows -- no record is no stop to report: the server writes one before each fatal stop and a clean start deletes it
	if !found {
		return nil
	}
	return fatalStopRows(&record)
}

// fatalStopRows builds the rows from a record.
func fatalStopRows(record *server.LastStop) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for i := range record.Entries {
		entry := &record.Entries[i]
		switch entry.Reason {
		case server.LastStopFatalPolicy, server.LastStopBootRefused:
			rows = append(rows, fatalStopRow(entry))
		case server.LastStopCleanupLate:
			rows = append(rows, cleanupLateRow(entry))
		case server.LastStopShutdownOrder:
			rows = append(rows, shutdownOrderRow(entry))
		case server.LastStopSchemaRefused:
			rows = append(rows, schemaRefusedRow(entry))
		case server.LastStopSchemaUndeclared:
			rows = append(rows, schemaUndeclaredRow(entry))
		case server.LastStopBuildRegistrationUnheld:
			rows = append(rows, buildRegistrationUnheldRow(entry))
		case server.LastStopUnspecified:
			// RecordLastStop refuses an unspecified reason.
		default:
			rows = append(rows, fatalStopRow(entry))
		}
	}
	if record.Dropped > 0 && len(rows) > 0 {
		last := &rows[len(rows)-1]
		var tail textbuf.Buffer
		tail.Str(last.Message).Str("; ").Int(int64(record.Dropped)).Str(" later stop reasons were not kept")
		last.Message = tail.String()
	}
	return rows
}

// fatalStopRow writes one recorded stop: which plugin, why and when, and the
// fix: the plugin's own log lines before the stop, then the config change.
//
// A boot refusal that is not a missing dependency records no plugin
// (settleLastStop), because the refusal's own text is the whole cause. Its
// row then names no plugin, and its fix reads ze's log up to that time with
// no grep, because no name narrows it.
func fatalStopRow(entry *server.LastStopEntry) diagnostic.Diagnostic {
	at := entry.Time.UTC().Format(time.RFC3339)
	var message, fix textbuf.Buffer
	if entry.Reason == server.LastStopBootRefused {
		message.Str("ze refused to start at ").Str(at)
		if entry.Plugin != "" {
			message.Str(" (").Str(entry.Plugin).Byte(')')
		}
		message.Str(": ").Str(entry.Detail)
	} else {
		subject := "a plugin ze did not record"
		if entry.Plugin != "" {
			subject = "plugin " + entry.Plugin
		}
		message.Str("ze stopped itself at ").Str(at).Str(" because ").Str(subject).
			Str(" failed and its failure policy stops ze: ").Str(entry.Detail)
	}
	fix.Str("journalctl -u ze --until ").Str(at)
	if entry.Plugin == "" {
		fix.Str(", then change the configuration the refusal names and start ze")
	} else {
		fix.Str(" | grep ").Str(entry.Plugin).
			Str(", then change that plugin's configuration and start ze")
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginFatalStop,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}

// cleanupLateRow writes one stop whose cleanup ran late: which owner, what
// did not finish and when; the fix reads that owner's log lines.
func cleanupLateRow(entry *server.LastStopEntry) diagnostic.Diagnostic {
	at := entry.Time.UTC().Format(time.RFC3339)
	var message, fix textbuf.Buffer
	message.Str("the stop at ").Str(at).Str(" went on before ").Str(entry.Plugin).
		Str(" finished its cleanup, so state it installs may remain: ").Str(entry.Detail)
	fix.Str("journalctl -u ze --until ").Str(at).Str(" | grep ").Str(entry.Plugin)
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginStopCleanupLate,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}

// shutdownOrderRow names the distinct historical loss of dependency ordering.
// It does not claim that cleanup ran late or that installed state remains.
func shutdownOrderRow(entry *server.LastStopEntry) diagnostic.Diagnostic {
	at := entry.Time.UTC().Format(time.RFC3339)
	var message textbuf.Buffer
	message.Str("the stop at ").Str(at).
		Str(" could not preserve plugin dependency order, so providers may have stopped before consumers finished cleanup: ").
		Str(entry.Detail)
	fix := entry.Fix
	if fix == "" {
		fix = "journalctl -u ze --until " + at
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginShutdownOrder,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(), Fix: fix,
	}
}

// schemaRefusedRow writes one start that ended because ze refused the YANG
// module an external plugin declared: when, and the refusal's own problem and
// why, which name the plugin, the module and the cause. The module may be a
// config, command or rpc module, so the message names a YANG module and not
// the config schema. The fix is the one the refusal named.
func schemaRefusedRow(entry *server.LastStopEntry) diagnostic.Diagnostic {
	at := entry.Time.UTC().Format(time.RFC3339)
	var message textbuf.Buffer
	message.Str("ze refused to start at ").Str(at).
		Str(" because it refused a YANG module an external plugin declared: ").Str(entry.Detail)
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginSchemaRefused,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      schemaStopFix(entry, at),
	}
}

// schemaUndeclaredRow writes one start that ended on a config name no schema
// defines while ze held no config schema of a declared external plugin: when,
// which plugin, and why ze held none. A section written for that plugin is
// such a name. The fix is the one the refusal named.
func schemaUndeclaredRow(entry *server.LastStopEntry) diagnostic.Diagnostic {
	at := entry.Time.UTC().Format(time.RFC3339)
	var message textbuf.Buffer
	message.Str("ze refused to start at ").Str(at).
		Str(" on a configuration name no schema defines, and it held no config schema of external plugin ").
		Quoted(entry.Plugin).Str(", so a section written for that plugin is such a name: ").Str(entry.Detail)
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginSchemaUndeclared,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      schemaStopFix(entry, at),
	}
}

// buildRegistrationUnheldRow writes one start that ended because a module
// compiled into ze names a ze:validate function or a ze:reference source this
// ze does not hold: when, and the refusal's own problem and why, which name
// the module, the extension and the name. The fix is the one the refusal
// named.
func buildRegistrationUnheldRow(entry *server.LastStopEntry) diagnostic.Diagnostic {
	at := entry.Time.UTC().Format(time.RFC3339)
	var message textbuf.Buffer
	message.Str("ze refused to start at ").Str(at).
		Str(" because this ze was built without a registration its own YANG names: ").Str(entry.Detail)
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorBuildRegistrationUnheld,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      schemaStopFix(entry, at),
	}
}

// schemaStopFix answers the fix of a schema row: the command the refusal
// named, or, for an entry that kept none, ze's log up to the refusal.
func schemaStopFix(entry *server.LastStopEntry, at string) string {
	if entry.Fix != "" {
		return entry.Fix
	}
	return "journalctl -u ze --until " + at
}

// checkPluginAutoload resolves the plugins the config's roots load, as boot
// does (Server.getConfigPathPlugins, startup_autoload.go), and reports the
// refusal boot would give: a plugin the config selects whose hard dependency
// this build does not carry. The text is the boot refusal's own
// (registry.MissingDependencyError).
//
// Boot also adds the data-plane writer a NeedsDataPlane producer needs
// (appendDataPlaneWriter); plugin-dataplane-writer reports that refusal
// (check_dataplane_writer.go), so it is not repeated here.
func checkPluginAutoload(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	// A context holding no *config.Tree, and a nil one, are both no parsed
	// config: one fact, one guard.
	tree, _ := ctx.Tree.(*config.Tree)
	if tree == nil {
		return nil // No parsed config: config validation's verdict, not this one.
	}
	_, err := registry.ResolveDependencies(autoloadSet(tree, ctx.Plugins), nil, config.ConfigNeedHolds(tree.ToMap()))
	if err == nil {
		return nil
	}
	return []diagnostic.Diagnostic{autoloadRow(err)}
}

// autoloadSet is the plugins the config's roots select that no plugin block
// configures: boot excludes a configured plugin before it resolves.
func autoloadSet(tree *config.Tree, configured []zeplugin.PluginConfig) []string {
	names := make(map[string]bool)
	for i := range configured {
		for _, name := range zeplugin.RegistryNames(configured[i]) {
			names[name] = true
		}
	}
	var needed []string
	for _, name := range registry.PluginsForConfigPaths(config.CollectContainerPaths(tree)) {
		if names[name] {
			continue
		}
		needed = append(needed, name)
	}
	return needed
}

// autoloadRow writes the refusal. Its fix is the one the refusal names.
func autoloadRow(err error) diagnostic.Diagnostic {
	var message textbuf.Buffer
	message.Str("the plugins this config auto-loads do not resolve in this build, so ze refuses to start: ").Err(err)
	fix := "ze show plugin list"
	if refusal, ok := errors.AsType[*opererr.Error](err); ok {
		fix = refusal.Fix()
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginAutoloadUnresolved,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      fix,
	}
}
