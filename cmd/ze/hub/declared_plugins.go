// Design: docs/architecture/hub-architecture.md -- Startup Flow: the declared plugins run and declare before the full parse.
// Related: main.go -- runYANGConfig calls joinDeclaredPlugins before LoadConfig.
// Related: ../../../internal/component/plugin/server/early_stage1.go -- HoldDeclaredPlugins reads and keeps each Stage 1.
// Related: ../../../internal/component/config/first_pass.go -- FirstPass answers the declared plugins.

package hub

import (
	"context"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/yang"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ConfigRefused, when set, is called once when boot refuses, at its full
// parse, the config it was started with, before the process exits 1. It is
// for a caller that handed boot a config it could not judge itself: a config
// that declares an external plugin can only be judged once that plugin has
// run and declared its schema, which happens inside boot. The caller undoes
// what it did on the strength of that config (a pushed config it applied, a
// fetched config it cached).
//
// MUST be set before Run or RunWithManagedClient is called, and never after.
var ConfigRefused func(err error)

// loadHubAuthority answers the certificate authority the plugin hub takes its
// leaf from: the root in the store, generated on first start, or a root for
// the life of the process when the daemon has no store. Boot MUST call it
// once and keep the answer: a second storeless call would answer another
// root than the one the forked plugins were told to trust.
func loadHubAuthority(store storage.Storage) (*zepki.Root, error) {
	if store == nil {
		return zepki.NewEphemeralRoot()
	}
	return zepki.LoadOrGenerateRoot(store)
}

// pluginServerConfigs answers plugins as the plugin server and the process
// manager take them: each with workDir as the directory its run string
// resolves against. Boot builds the declared external plugins and the full
// plugin list through this one function, so the config a held plugin was
// forked with is the config its startup phase names.
func pluginServerConfigs(plugins []zePlugin.PluginConfig, workDir string) []zePlugin.PluginConfig {
	configs := make([]zePlugin.PluginConfig, 0, len(plugins))
	for _, pc := range plugins {
		configs = append(configs, zePlugin.PluginConfig{
			Name:          pc.Name,
			Run:           pc.Run,
			Encoder:       pc.Encoder,
			Respawn:       pc.Respawn,
			WorkDir:       workDir,
			ReceiveUpdate: pc.ReceiveUpdate,
			StageTimeout:  pc.StageTimeout,
			Internal:      pc.Internal,
		})
	}
	return configs
}

// joinDeclaredPlugins is steps 2 to 5 of the load order, between the first
// pass and the full parse: it runs each external plugin the first pass found,
// reads and keeps its Stage 1 declaration with the plugin held at the barrier,
// then joins the modules those declarations carry to the compiled-in set and
// publishes the joined set to the process. It answers what each plugin
// declared, for the refusal of a section still unknown at the full parse.
//
// With no declared external plugin it does nothing at all: it calls neither
// prepare nor spawner, waits for nothing and publishes nothing, so such a
// boot is the boot it always was.
//
// prepare gives the spawner what a fork needs (the hub listener and its
// certificate authority) and runs only when a plugin is to be forked. An
// error is a refusal of the boot: prepare's, or the publish's when a declared
// module does not compile, clashes with a module ze holds, or is published
// after a value that keeps the module set was built. On a publish error the
// declarations are answered too, and the plugins are still held: the caller
// MUST stop them.
//
// The caller MUST call it before it parses the full config, before
// pluginserver.NewServer and before any gNMI server exists: each of those
// keeps the module set it finds (yang.KeepsModuleSet).
func joinDeclaredPlugins(ctx context.Context, firstPass *zeconfig.FirstPassResult, spawner pluginserver.DeclaredSpawner, prepare func() error) ([]pluginserver.EarlyDeclaration, error) {
	if len(firstPass.External) == 0 {
		return nil, nil
	}
	if err := prepare(); err != nil {
		return nil, err
	}
	// The `environment` block reaches each forked plugin here, in the child's
	// environment alone. The daemon's own is written by the full parse and no
	// earlier: a block applied now would outlive a config that parse refuses,
	// and its values would then win over the recovered config's.
	declarations := pluginserver.HoldDeclaredPlugins(ctx, spawner,
		pluginServerConfigs(firstPass.External, firstPass.ConfigDir),
		zeconfig.EnvironmentEntries(firstPass.Environment))
	// Published even when no plugin declared a module: the set is then
	// recorded as holding no external module, which is what a plugin
	// restarted later is compared with.
	if err := zeconfig.PublishExternalModules(declaredModules(declarations)); err != nil {
		return declarations, err
	}
	return declarations, nil
}

// declaredModules answers the YANG modules the declarations carry, in config
// order and, within one plugin, in the order its declaration lists them: one
// for each entry with a module text, so a plugin that brings a config module
// and a command module contributes both. A plugin that did not declare, and an
// entry with no module text, contribute none. A later Stage 1 of the plugin is
// compared with what this answers (pluginserver.DeclaredModules).
func declaredModules(declarations []pluginserver.EarlyDeclaration) []yang.ExternalModule {
	var modules []yang.ExternalModule
	for _, declaration := range declarations {
		if declaration.Err != nil {
			continue
		}
		modules = append(modules, pluginserver.DeclaredModules(declaration.Plugin, declaration.Schemas)...)
	}
	return modules
}

// startedPlugins answers what boot learned of the config schema of each
// external plugin the first pass found, for the record a served command reads
// when it names the plugins that could explain a name no schema defines
// (zeconfig.RecordStartedPlugins). external is FirstPassResult.External and
// declarations what joinDeclaredPlugins answered for it. A plugin with no
// declaration read, for any reason, is recorded as unread: boot does not know
// whether it defines a config section.
func startedPlugins(external []zePlugin.PluginConfig, declarations []pluginserver.EarlyDeclaration) []zeconfig.StartedPlugin {
	started := make([]zeconfig.StartedPlugin, 0, len(external))
	for _, declared := range external {
		started = append(started, zeconfig.StartedPlugin{
			Declaration: declared,
			Schema:      schemaOutcome(declared.Name, declarations),
		})
	}
	return started
}

// schemaOutcome answers what the declaration boot read from the plugin named
// name says of its config schema. A plugin whose modules add only commands and
// rpcs defines no config section, so it is SchemaNone, as one that declared no
// module is. A module text that does not parse alone leaves boot not knowing
// whether the plugin defines config, so it is SchemaUnread; the join refuses
// such a text before any caller asks.
func schemaOutcome(name string, declarations []pluginserver.EarlyDeclaration) zeconfig.SchemaOutcome {
	for _, declaration := range declarations {
		if declaration.Plugin != name {
			continue
		}
		if declaration.Err != nil {
			return zeconfig.SchemaUnread
		}
		config, err := pluginserver.DeclaresConfig(declaration.Plugin, declaration.Schemas)
		if err != nil {
			return zeconfig.SchemaUnread
		}
		if !config {
			return zeconfig.SchemaNone
		}
		return zeconfig.SchemaDeclared
	}
	return zeconfig.SchemaUnread
}

// schemalessPlugins answers the declared external plugins whose declaration
// boot read and that carries no YANG module, in config order. external is
// FirstPassResult.External and declarations what joinDeclaredPlugins answered
// for it. Such a plugin defines no config section, which is no problem by
// itself: it matters only beside the refusal of a name no schema defines.
func schemalessPlugins(external []zePlugin.PluginConfig, declarations []pluginserver.EarlyDeclaration) []zePlugin.PluginConfig {
	var schemaless []zePlugin.PluginConfig
	for _, declared := range external {
		if schemaOutcome(declared.Name, declarations) == zeconfig.SchemaNone {
			schemaless = append(schemaless, declared)
		}
	}
	return schemaless
}

// schemalessPluginLines answers, for a config load refused for a name no
// schema defines, one operator error for each plugin of schemaless: the
// plugin declared no YANG module, what follows, and the command that shows
// what its program declares. It answers "" for no plugin.
func schemalessPluginLines(schemaless []zePlugin.PluginConfig) string {
	var tb textbuf.Buffer
	for _, declared := range schemaless {
		tb.Str("error: ").Err(pluginserver.SchemalessRefusal(declared)).Byte('\n')
	}
	return tb.String()
}

// recordUndeclaredSchemaStops writes to the last-stop record, for a config load
// refused for a name no schema defines, each declared external plugin ze holds
// no config schema of: the ones of declarations that sent no declaration, then
// the ones of schemaless, whose declaration carries no YANG module. They are
// the plugins undeclaredPluginLines and schemalessPluginLines name on the
// terminal.
func recordUndeclaredSchemaStops(declarations []pluginserver.EarlyDeclaration, schemaless []zePlugin.PluginConfig) {
	for _, declaration := range declarations {
		if declaration.Err != nil {
			pluginserver.RecordUndeclaredSchemaStop(declaration.Plugin, declaration.Err)
		}
	}
	for _, declared := range schemaless {
		pluginserver.RecordUndeclaredSchemaStop(declared.Name, pluginserver.SchemalessRefusal(declared))
	}
}

// declaredPluginNames answers the names of plugins, for DropDeclaredExcept.
func declaredPluginNames(plugins []zePlugin.PluginConfig) []string {
	names := make([]string, 0, len(plugins))
	for _, pc := range plugins {
		names = append(names, pc.Name)
	}
	return names
}

// undeclaredPluginLines answers, for a refused config load, one line for each
// declared external plugin that gave the daemon no declaration, with why and
// the command that fixes it. A section only such a plugin's schema defines is
// refused by the load as an unknown keyword, and the load's own message
// cannot name the plugin: nothing ties an unknown keyword to a schema the
// daemon never received. It answers "" when every plugin declared.
func undeclaredPluginLines(declarations []pluginserver.EarlyDeclaration) string {
	var tb textbuf.Buffer
	for _, declaration := range declarations {
		if declaration.Err == nil {
			continue
		}
		tb.Str("error: a config section that only plugin \"").Str(declaration.Plugin).
			Str("\" defines is an unknown keyword to ze: ").Err(declaration.Err).Byte('\n')
	}
	return tb.String()
}
