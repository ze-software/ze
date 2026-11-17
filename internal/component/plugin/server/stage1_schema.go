// Design: docs/architecture/hub-architecture.md -- an external plugin's config schema changes only when ze restarts
// Related: stage1_checks.go -- the Stage 1 table, whose schema-unchanged entry calls refuseChangedSchema
// Related: reload_declarations.go -- the same rule for the plugin declarations of a reload
// Related: ../../config/yang/publish.go -- PublishedModule, the set a declaration is compared with
// Related: ../../../../cmd/ze/hub/declared_plugins.go -- declaredModules, which builds what boot published

package server

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// schemaFixedWhy says why a plugin cannot bring another config schema while
// ze runs, and what the refusal left as it was.
const schemaFixedWhy = "ze reads the config schema of an external plugin once, when ze starts, and every reader of the config " +
	"keeps that schema. ze does not run the plugin with another one; the running config, the schema ze holds and every " +
	"other plugin are unchanged"

// refuseChangedSchema refuses the Stage 1 declaration of an external plugin
// whose config schema is not the one the process published for it when ze
// started (yang.PublishExternalModules). It runs for every Stage 1 of the
// plugin: the first, which declares what boot read and published, and each
// later one, after a respawn or `request plugin restart`.
//
// The refusal is an operator error that says what differs and names the
// restart of ze. It is logged and raised as doctor-plugin-schema-changed
// here, and the published set is never written. A declaration that carries
// the published schema clears that warning.
func refuseChangedSchema(cfg plugin.PluginConfig, schema *rpc.SchemaDecl) error {
	if !cfg.RunsExternalProgram() {
		// A plugin compiled into ze declares a module ze registered at
		// init(): the published external layer never holds it, and it cannot
		// change while the binary runs.
		return nil
	}
	difference, differs := schemaDifference(cfg.Name, schema)
	if !differs {
		report.ClearWarning(serverEventSource, diagnostic.CodeDoctorPluginSchemaChanged, fitSubject(cfg.Name))
		return nil
	}
	var problem textbuf.Buffer
	problem.Str("plugin ").Quoted(cfg.Name).
		Str(" was started again with another config schema, and ze refused it: ").Str(difference)
	refusal := opererr.New(problem.String(), schemaFixedWhy, restartDaemonFix)

	logger().Warn("plugin restarted with another config schema", "plugin", cfg.Name, "error", refusal,
		"fix", restartDaemonFix, "code", diagnostic.CodeDoctorPluginSchemaChanged)
	var message textbuf.Buffer
	message.Str(refusal.Problem()).Str(". ").Str(refusal.Why())
	report.RaiseWarningFix(serverEventSource, diagnostic.CodeDoctorPluginSchemaChanged, fitSubject(cfg.Name),
		report.FitMessage(message.String()), restartDaemonFix)
	return refusal
}

// schemaDifference compares the schema the plugin named name declares now
// with the module the process published for it, and says how they differ.
// differs is false when they are the same: the same module, or none on both
// sides.
func schemaDifference(name string, schema *rpc.SchemaDecl) (difference string, differs bool) {
	published, fixed := yang.PublishedExternalModules()
	if !fixed {
		// refuse-by-default: guards refuseChangedSchema -- a process that published no external layer started no external plugin before its full parse, so it holds no schema this declaration could differ from; boot publishes, with no module too, whenever it starts one (cmd/ze/hub joinDeclaredPlugins)
		return "", false
	}
	held := slices.IndexFunc(published, func(module yang.ExternalModule) bool { return module.Plugin == name })
	declared, declares := declaredModule(name, schema)

	var text textbuf.Buffer
	if !declares {
		if held < 0 {
			return "", false
		}
		text.Str("it declares no YANG module now, and declared module ").Quoted(published[held].Module).
			Str(" when ze started")
		return text.String(), true
	}

	switch yang.PublishedModule(declared) {
	case yang.ModulePublished:
		return "", false
	case yang.ModuleUndeclared:
		text.Str("it declares YANG module ").Quoted(declared.Module).
			Str(" now, and declared no module when ze started")
		return text.String(), true
	case yang.ModuleChanged:
		return moduleDifference(published[held], declared), true
	case yang.ModuleSetUnpublished:
		panic("BUG: the published module set went back to unpublished")
	case yang.ModulePublicationUnspecified:
		panic("BUG: yang.PublishedModule answered the unspecified publication")
	}
	panic("BUG: unknown module publication")
}

// declaredModule answers the module a Stage 1 schema declares, as boot builds
// the one it publishes (cmd/ze/hub declaredModules): declares is false for a
// declaration with no schema or with no module text.
func declaredModule(name string, schema *rpc.SchemaDecl) (module yang.ExternalModule, declares bool) {
	if schema == nil {
		return yang.ExternalModule{}, false
	}
	if schema.YANGText == "" {
		return yang.ExternalModule{}, false
	}
	return yang.ExternalModule{
		Plugin:    name,
		Module:    schema.Module,
		Namespace: schema.Namespace,
		Text:      schema.YANGText,
	}, true
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
