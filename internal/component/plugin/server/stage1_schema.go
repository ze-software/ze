// Design: docs/architecture/hub-architecture.md -- an external plugin's YANG modules change only when ze restarts
// Related: stage1_checks.go -- the Stage 1 table, whose schema-unchanged entry calls refuseChangedSchema
// Related: reload_declarations.go -- the same rule for the plugin declarations of a reload
// Related: ../../config/yang/publish.go -- PublishedModule, the set a declaration is compared with
// Related: ../../../../cmd/ze/hub/declared_plugins.go -- declaredModules, which builds what boot published

package server

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// schemaFixedWhy says why a plugin cannot bring other YANG modules while ze
// runs, whether they add config, commands or rpcs, and what the refusal left
// as it was.
const schemaFixedWhy = "ze reads the YANG modules of an external plugin once, when ze starts, and every reader of the config " +
	"and of the commands keeps them. ze does not run the plugin with other ones; the running config, the modules ze holds " +
	"and every other plugin are unchanged"

// refuseChangedSchema refuses the Stage 1 declaration of an external plugin
// whose YANG modules (config, command or rpc modules) are not the ones the
// process published for it when ze started (yang.PublishExternalModules). It
// runs for every Stage 1 of the
// plugin: the first, which declares what boot read and published, and each
// later one, after a respawn or `request plugin restart`.
//
// The refusal is an operator error that says what differs and names the
// restart of ze. It is logged and raised as doctor-plugin-schema-changed
// here, and the published set is never written. A declaration that carries
// the published modules clears that warning.
func refuseChangedSchema(cfg plugin.PluginConfig, schemas []rpc.SchemaDecl) error {
	if !cfg.RunsExternalProgram() {
		// A plugin compiled into ze declares a module ze registered at
		// init(): the published external layer never holds it, and it cannot
		// change while the binary runs.
		return nil
	}
	difference, differs := schemaDifference(cfg.Name, schemas)
	if !differs {
		report.ClearWarning(serverEventSource, CodeDoctorPluginSchemaChanged, fitSubject(cfg.Name))
		return nil
	}
	var problem textbuf.Buffer
	problem.Str("plugin ").Quoted(cfg.Name).
		Str(" was started again with other YANG modules, and ze refused it: ").Str(difference)
	refusal := opererr.New(problem.String(), schemaFixedWhy, restartDaemonFix)

	logger().Warn("plugin restarted with other YANG modules", "plugin", cfg.Name, "error", refusal,
		"fix", restartDaemonFix, "code", CodeDoctorPluginSchemaChanged)
	var message textbuf.Buffer
	message.Str(refusal.Problem()).Str(". ").Str(refusal.Why())
	report.RaiseWarningFix(serverEventSource, CodeDoctorPluginSchemaChanged, fitSubject(cfg.Name),
		report.FitMessage(message.String()), restartDaemonFix)
	return refusal
}

// schemaDifference compares the modules the plugin named name declares now
// with the modules the process published for it, and says how they differ.
// differs is false when they are the same: the same modules, or none on both
// sides. The first difference is named: a module declared now that differs
// from the published one of its name or was not published, then a published
// module the plugin no longer declares.
func schemaDifference(name string, schemas []rpc.SchemaDecl) (difference string, differs bool) {
	published, fixed := yang.PublishedExternalModules()
	if !fixed {
		// refuse-by-default: guards refuseChangedSchema -- a process that published no external layer started no external plugin before its full parse, so it holds no schema this declaration could differ from; boot publishes, with no module too, whenever it starts one (cmd/ze/hub joinDeclaredPlugins)
		return "", false
	}
	var held []yang.ExternalModule
	for _, module := range published {
		if module.Plugin == name {
			held = append(held, module)
		}
	}
	declared := DeclaredModules(name, schemas)

	var text textbuf.Buffer
	if len(declared) == 0 {
		if len(held) == 0 {
			return "", false
		}
		text.Str("it declares no YANG module now, and declared module ").Quoted(held[0].Module).
			Str(" when ze started")
		return text.String(), true
	}
	for _, module := range declared {
		switch yang.PublishedModule(module) {
		case yang.ModulePublished:
			continue
		case yang.ModuleUndeclared:
			text.Str("it declares YANG module ").Quoted(module.Module).
				Str(" now, and declared no module when ze started")
			return text.String(), true
		case yang.ModuleChanged:
			return changedModuleDifference(held, declared, module), true
		case yang.ModuleSetUnpublished:
			panic("BUG: the published module set went back to unpublished")
		case yang.ModulePublicationUnspecified:
			panic("BUG: yang.PublishedModule answered the unspecified publication")
		}
		panic("BUG: unknown module publication")
	}
	for _, module := range held {
		if slices.ContainsFunc(declared, func(now yang.ExternalModule) bool { return now.Module == module.Module }) {
			continue
		}
		text.Str("it does not declare YANG module ").Quoted(module.Module).
			Str(" now, and declared it when ze started")
		return text.String(), true
	}
	return "", false
}

// changedModuleDifference says how module, declared now, differs from what
// the plugin published: from the published module of the same name, or, when
// the plugin brings one module on each side, from that one module. held MUST
// hold at least one module (yang.ModuleChanged).
func changedModuleDifference(held, declared []yang.ExternalModule, module yang.ExternalModule) string {
	same := slices.IndexFunc(held, func(published yang.ExternalModule) bool { return published.Module == module.Module })
	if same >= 0 {
		return moduleDifference(held[same], module)
	}
	if len(held) == 1 && len(declared) == 1 {
		return moduleDifference(held[0], module)
	}
	var text textbuf.Buffer
	text.Str("it declares YANG module ").Quoted(module.Module).Str(" now, which it did not declare when ze started")
	return text.String()
}

// DeclaredModules answers the modules a Stage 1 declaration of the plugin
// named name carries, in the order it lists them: one for each entry with a
// module text. Boot publishes what it answers (cmd/ze/hub declaredModules), and
// a later Stage 1 of the plugin is compared with it, so both read one rule. An
// entry with no text brings no module.
func DeclaredModules(name string, schemas []rpc.SchemaDecl) []yang.ExternalModule {
	var modules []yang.ExternalModule
	for i := range schemas {
		if schemas[i].YANGText == "" {
			continue
		}
		modules = append(modules, yang.ExternalModule{
			Plugin:    name,
			Module:    schemas[i].Module,
			Namespace: schemas[i].Namespace,
			Text:      schemas[i].YANGText,
		})
	}
	return modules
}

// moduleDifference says in which field declared differs from published, the
// module of the same plugin: the name first, then the namespace, then the
// text. The caller MUST pass two modules that differ.
func moduleDifference(published, declared yang.ExternalModule) string {
	var text textbuf.Buffer
	if published.Module != declared.Module {
		text.Str("it declares YANG module ").Quoted(declared.Module).Str(" now, and declared module ").
			Quoted(published.Module).Str(" when ze started")
		return text.String()
	}
	if published.Namespace != declared.Namespace {
		text.Str("the namespace of YANG module ").Quoted(declared.Module).Str(" is ").Quoted(declared.Namespace).
			Str(" now, and was ").Quoted(published.Namespace).Str(" when ze started")
		return text.String()
	}
	text.Str("the text of YANG module ").Quoted(declared.Module).
		Str(" is not the text the plugin declared when ze started")
	return text.String()
}
