// Design: docs/architecture/doctor-and-health-checks.md -- plugin external blocks the daemon cannot start
// Related: register_external.go -- registers the two checks below
// Related: ../ipc/plugin_name.go -- the one plugin name declaration the name check reads
// Related: ../registry/registry.go -- Registration.RunsInProcessOnly, the field the in-process check reads

package doctor

import (
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// externalNameCheckName reports a `plugin external` name the connect-back
// refuses.
const externalNameCheckName = "plugin-external-name"

// externalInProcessCheckName reports a `plugin external` block that runs a
// plugin declared in-process only.
const externalInProcessCheckName = "plugin-external-in-process"

// checkExternalPluginNames reports each `plugin external` block whose name the
// connect-back refuses. The commit refuses such a name now, so the row is for
// a config written before that refusal: it fails its load, and the daemon
// never starts the plugin. It runs at the parsed-config phase, on the tree
// that ReadConfig's refusal does not hide.
func checkExternalPluginNames(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	// The parsed-config phase hands every check the tree a successful parse
	// produced (internal/component/doctor runChecks), so a missing tree is a
	// defect in the runner. The check then says it could not look, rather than
	// answering "no refused name" for a config it never read.
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return []diagnostic.Diagnostic{externalNameUnreadRow()}
	}
	if tree == nil {
		return []diagnostic.Diagnostic{externalNameUnreadRow()}
	}
	plugins := tree.GetContainer("plugin")
	if plugins == nil {
		return nil // No plugin block, so no external block to judge.
	}
	blocks := plugins.GetList("external")
	var rows []diagnostic.Diagnostic
	for _, name := range slices.Sorted(maps.Keys(blocks)) {
		if ipc.ValidPluginName(name) {
			continue
		}
		rows = append(rows, externalNameRow(name))
	}
	return rows
}

// externalNameUnreadRow is the answer when the check was given no parsed
// tree: it judged no external name, and says so.
func externalNameUnreadRow() diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginConnectBackRefused,
		Severity: diagnostic.SeverityError,
		Message: "plugin external names were not checked: the doctor run gave this check no parsed config tree, " +
			"so a name the connect-back refuses would go unreported",
		Fix: "ze config validate, which refuses an external name the connect-back refuses",
	}
}

// externalNameRow is the row for one refused external name.
func externalNameRow(name string) diagnostic.Diagnostic {
	var message, fix textbuf.Buffer
	message.Str("plugin external ").Quoted(name).
		Str(": the connect-back refuses this name, because the hub binds the plugin's token to it and a name is ").
		Str(ipc.PluginNameForm).Str("; ze refuses the config, and the plugin never attaches")
	fix.Str("rename the block to a name of that form, then run ze config validate")
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginConnectBackRefused,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}

// checkExternalInProcess reports each `plugin external` block that runs a
// plugin whose registration declares RunsInProcessOnly. Such a plugin reaches
// daemon state a forked process does not hold, so the process starts and
// silently does nothing. The check reads the field of the registration the
// block resolves to, never a list of plugin names.
func checkExternalInProcess(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for _, block := range ctx.Plugins {
		if block.Internal {
			continue // An internal block, or an external one with use, runs in the daemon.
		}
		if reg := inProcessOnlyRegistration(block); reg != nil {
			rows = append(rows, externalInProcessRow(block.Name, reg.Name))
		}
	}
	return rows
}

// inProcessOnlyRegistration returns the in-process-only registration an
// external block runs, or nil when the block runs none: a third-party binary,
// or a plugin that also works forked.
func inProcessOnlyRegistration(block zeplugin.PluginConfig) *registry.Registration {
	for _, name := range zeplugin.RegistryNames(block) {
		reg := registry.Lookup(name)
		if reg == nil {
			continue // Not a plugin this binary carries.
		}
		if reg.RunsInProcessOnly {
			return reg
		}
	}
	return nil
}

// externalInProcessRow is the row for one external block running an
// in-process-only plugin.
func externalInProcessRow(block, plugin string) diagnostic.Diagnostic {
	var message, fix textbuf.Buffer
	message.Str("plugin external ").Quoted(block).Str(" runs ").Str(plugin).
		Str(", which works only inside the daemon: a forked process does not hold the daemon state it reaches, ").
		Str("so it starts and does nothing")
	fix.Str("move it to an internal block: plugin { internal ").Str(block).Str(" { use ").Str(plugin).Str("; } }")
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginExternalUnsupported,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}
