// Design: docs/architecture/config/yang-config-design.md — what a command that parses a config with no daemon knows of the external plugins the config declares
// Related: first_pass.go — FirstPass, which answers the plugins a config declares
// Related: yang_schema.go — PublishExternalModules and YANGSchema, which this file calls
// Related: line_error.go — RefusesUnknownNode, the refusals this file explains
// Related: ../../../cmd/ze/hub/declared_plugins.go — joinDeclaredPlugins, the step at boot, which reads each plugin's schema from its live Stage 1

package config

import (
	"errors"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ConfigSource is one config a command is about to parse: its text, and the
// path it was read from, which is "" or "-" for a text with no file.
type ConfigSource struct {
	Text string
	Path string
}

// validateCommand is the command that asks a running ze to check a config
// file: `ze config validate <file>` sends the file's content to the daemon,
// which holds the schema of the external plugins it started with, and answers
// in its own process only when no daemon answers. It is the fix a command that
// asked no daemon names. A session command (`show config validation <file>`
// typed in `ze cli`) is not: the daemon reads a file a caller names only from
// its own config directories (command.CallerPath), never from the operator's
// working directory.
const (
	validateCommand = "ze config validate "
	restartDaemon   = "restart ze"
)

// DaemonUnanswered is what a command that asked a running ze to check a config
// learned when none answered. A command that asked no daemon holds none, and
// its message says so (DeclaredSchemas.Unanswered).
type DaemonUnanswered struct {
	// Why says why no running ze answered, naming its address when one was
	// resolved.
	Why string
	// Fix is the one command that gets a running ze to answer.
	Fix string
	// Rerun is true when Fix makes a ze answer and the operator then runs the
	// command again, and false when Fix is that command itself.
	Rerun bool
	// Again is the command the operator runs again after Fix, as they typed
	// it, for a command other than `ze config validate`. Empty means the
	// validate command for the config's path. It is read only when Rerun is
	// true.
	Again string
}

// SchemaOutcome is what a daemon learned of the config schema of one external
// plugin when it started the plugin, before it parsed the config. The zero
// value is invalid.
type SchemaOutcome uint8

const (
	SchemaOutcomeUnspecified SchemaOutcome = iota
	// SchemaDeclared: the plugin declared a YANG module that adds config
	// (yang.DefinesConfig), and the module set the daemon published holds it.
	SchemaDeclared
	// SchemaNone: the plugin sent its declaration, and the declaration
	// carries no YANG module that adds config: none at all, or only modules
	// that add commands and rpcs. The plugin defines no config section, so no
	// name a parse refuses is explained by it.
	SchemaNone
	// SchemaUnread: the plugin gave the daemon no declaration before the
	// config was parsed (it could not be run, it stalled, or it ended), so
	// the daemon does not know whether it defines a config section.
	SchemaUnread
)

// StartedPlugin is one external plugin a daemon started at boot, and what it
// learned of the plugin's config schema then.
type StartedPlugin struct {
	// Declaration is the plugin's declaration as FirstPass answered it, with
	// no work directory: a config read later is compared with it whole.
	Declaration plugin.PluginConfig
	Schema      SchemaOutcome
}

// DeclaredSchemas is what SchemaWithDeclaredPlugins learned about the external
// plugins its configs declare. The zero value holds no plugin. Its fields are
// never written after construction.
type DeclaredSchemas struct {
	// unanswered is why no running ze checked the config, for a command that
	// asked one before it parsed here. It is nil for a command that asked
	// none.
	unanswered *DaemonUnanswered
	// plugins holds the name of each declared external plugin that could
	// define a name this process's schema does not hold, each once, in
	// source then config order: a plugin this process holds no config schema
	// of, less the ones the daemon started with that declaration and that
	// declared no module (RecordStartedPlugins).
	plugins []string
	// offline is true in a process whose module set this file fixed to the
	// compiled-in one: a command that runs with no daemon. It is false in a
	// process whose set was fixed before this file read a config, as a
	// running daemon's is.
	offline bool
	// path is the path of the one source the step was given, and "" when it
	// was given several or none.
	path string
}

// A plugin in the list refuses nothing by itself: a config with no name the
// schema does not define parses. The list matters only beside the refusal of
// such a name, and nothing ties that name to one of the plugins, because ze
// holds no schema of them: the message says which plugins the config declares
// and never that one of them owns the name.

// WithoutSchema answers the name of each declared external plugin that could
// define a name this process's schema does not hold, each once, in source
// then config order. A plugin this process holds the module of is absent, and
// so is one the daemon started with the same declaration that declared no
// module: neither can explain a refused name.
func (d *DeclaredSchemas) WithoutSchema() []string {
	return slices.Clone(d.plugins)
}

// Offline reports whether the step ran in a command with no daemon: a process
// whose module set this file fixed to the compiled-in one. It is false in a
// running daemon, whose set was fixed before this file read a config.
//
// It is the one answer to "is there a daemon in this process" for a caller
// that reads a declared plugin from its program only inside one: with no
// daemon ze starts no external plugin (`show plugin declarations config`,
// through readConfiguredPlugins).
func (d *DeclaredSchemas) Offline() bool {
	return d.offline
}

// Unanswered answers a copy of d for a command that asked a running ze to
// check the config before it parsed here, and got no answer: every message the
// copy builds says a running ze was asked, why none answered, and the one
// command that gets an answer. d itself is unchanged, and a command that asked
// no daemon uses it as it is: its messages say it asked none and name the
// command that does.
func (d *DeclaredSchemas) Unanswered(asked DaemonUnanswered) *DeclaredSchemas {
	answered := *d
	answered.unanswered = &asked
	return &answered
}

// Unchecked answers the operator error a command adds beside the refusal of a
// name the schema does not define, when it reports that refusal as an entry of
// its own: which external plugins the config at path declares, why ze holds no
// schema of them here, and the one command that fixes it. It answers nil when
// the config declares no such plugin.
//
// The caller MUST add it only beside a refusal RefusesUnknownNode holds for.
func (d *DeclaredSchemas) Unchecked(path string) *opererr.Error {
	if len(d.plugins) == 0 {
		return nil
	}
	return opererr.New(d.problem(path), d.why(path), d.fix(path))
}

// Explain answers refusal, the refusal of a parse over the schema this value
// came with. A refusal of a name the schema does not define
// (RefusesUnknownNode), in a config that declares an external plugin ze holds
// no schema of here, is answered as one operator error that holds it: what was
// refused, the declared plugins, why ze cannot check a section of theirs here,
// and the command that can. Every other refusal, and nil, is answered
// unchanged: a value out of range or a syntax mistake is about a node ze
// knows, and no plugin's schema explains it.
func (d *DeclaredSchemas) Explain(refusal error) error {
	if refusal == nil {
		return nil
	}
	if !d.explains(refusal) {
		return refusal
	}
	return d.unchecked(d.path, refusal)
}

// ParseRefusal answers refusal, the refusal of a parse of the config at path
// over the schema this value came with, as the one operator error a command
// prints: what went wrong, why, and one fix on the last line. why says what
// the command could not do without the parsed config.
//
// Three cases, in this order:
//
//   - refusal is of a name the schema does not define (RefusesUnknownNode) and
//     the config declares an external plugin ze holds no schema of here: the
//     error Explain answers, whose fix is the command that reaches a ze that
//     holds the schema.
//   - refusal already holds an operator error: that error is answered as it
//     is, with its own fix and no second one.
//   - any other refusal: the fix is the command that lists every problem of
//     the config with its line.
func (d *DeclaredSchemas) ParseRefusal(path string, refusal error, why string) *opererr.Error {
	if d.explains(refusal) {
		return d.unchecked(path, refusal)
	}
	if held, holds := errors.AsType[*opererr.Error](refusal); holds {
		return held
	}
	return opererr.Wrap(refusal, "config "+path+" does not parse", why, "ze config validate "+path)
}

// explains reports whether the declared plugins are what a command says
// beside refusal: the refusal is of a name the schema does not define, and the
// config declares an external plugin ze holds no schema of here.
func (d *DeclaredSchemas) explains(refusal error) bool {
	if len(d.plugins) == 0 {
		return false
	}
	return RefusesUnknownNode(refusal)
}

// unchecked is Unchecked with the parser's refusal held as the cause.
func (d *DeclaredSchemas) unchecked(path string, refusal error) *opererr.Error {
	return opererr.Wrap(refusal, d.problem(path), d.why(path)+". What the parser refused", d.fix(path))
}

// problem says what was refused, in the terms of the process that refused it.
func (d *DeclaredSchemas) problem(path string) string {
	subject := "the config"
	if namesFile(path) {
		subject = "config " + path
	}
	if d.offline {
		return subject + " does not parse: it holds a name that no config schema compiled into ze defines"
	}
	return subject + " does not parse: it holds a name that no config schema this ze holds defines"
}

// why says which external plugins the config declares and why this process
// holds no schema of them. It claims nothing about which plugin, if any,
// defines the refused name.
func (d *DeclaredSchemas) why(path string) string {
	declares := "the config declares " + ExternalPluginNames(d.plugins)
	if !d.offline {
		return declares + ", of which this ze holds no config schema: ze reads the config schema of an " +
			"external plugin once, when it starts, from the plugins declared then. A section that such a " +
			"plugin defines is refused until ze starts with the plugin declared and the plugin declares its schema"
	}
	lead := declares + "; the config schema of an external plugin is not compiled into ze: ze reads it " +
		"from the plugin when `ze start` runs it. This command starts no plugin, so only a running ze " +
		"that started with these plugins checks a section that such a plugin defines"
	if d.unanswered == nil {
		return lead + ". This command checked the config in its own process and asked no running ze; " +
			"the command below asks one"
	}
	asked := lead + ", and none answered: " + d.unanswered.Why
	if !d.unanswered.Rerun {
		return asked
	}
	again := d.unanswered.Again
	if again == "" {
		again = validateFile(path)
	}
	return asked + ". Run the command below, then `" + again + "` again"
}

// fix is the one command that reaches a ze holding the schema: inside a
// daemon, its restart; with no daemon, the command that gets a running ze to
// answer when one was asked, and the command that asks one when none was.
func (d *DeclaredSchemas) fix(path string) string {
	if !d.offline {
		return restartDaemon
	}
	if d.unanswered == nil {
		return validateFile(path)
	}
	return d.unanswered.Fix
}

// validateFile is the command that asks a running ze to check the config at
// path, as the caller named it, with a placeholder for a text with no file.
func validateFile(path string) string {
	if namesFile(path) {
		return validateCommand + path
	}
	return validateCommand + "<config-file>"
}

// namesFile reports whether path names a file, as opposed to a text with no
// file ("" and "-").
func namesFile(path string) bool {
	if path == "" {
		return false
	}
	return path != "-"
}

// ExternalPluginNames writes names as `external plugin "a"`, or `external
// plugins "a", "b"` for several, as every refusal that says which external
// plugins a config declares names them. names MUST hold at least one name.
func ExternalPluginNames(names []string) string {
	var text textbuf.Buffer
	text.Str("external plugin")
	if len(names) > 1 {
		text.Byte('s')
	}
	for index, name := range names {
		if index > 0 {
			text.Byte(',')
		}
		text.Str(` "`).Str(name).Byte('"')
	}
	return text.String()
}

// declaredStep records what this file did to the process's module set.
var declaredStep struct {
	mu sync.Mutex
	// fixedHere is true once this file fixed the process's module set to the
	// compiled-in one, which only a process with no daemon in it lets it do.
	fixedHere bool
	// started is the external plugins the daemon in this process started at
	// boot, with what each declared (RecordStartedPlugins). It is nil in a
	// process that booted no daemon, and in a daemon that started none.
	started []StartedPlugin
	// unanswered is why no running ze answered the command this process is,
	// for a command that asked one before it judged the config here
	// (RecordDaemonUnanswered). It is nil in a process that asked none.
	unanswered *DaemonUnanswered
}

// RecordDaemonUnanswered records that this process, a command with no daemon
// in it, asked a running ze about the config it is about to judge and got no
// answer. Every DeclaredSchemas the step builds afterwards carries it, so the
// refusal of a name no compiled-in schema defines says a running ze was asked,
// why none answered, and the one command that gets an answer, in place of
// saying that none was asked.
//
// It exists for the commands whose producer builds its own DeclaredSchemas
// (`ze config fmt`, `dump`, `diff`, `graph`): the route that asked cannot hand
// the producer the failure, and the producer serves the daemon too, where
// nothing is asked. A command that holds the DeclaredSchemas itself uses
// DeclaredSchemas.Unanswered. Safe for concurrent use.
func RecordDaemonUnanswered(asked DaemonUnanswered) {
	declaredStep.mu.Lock()
	defer declaredStep.mu.Unlock()
	declaredStep.unanswered = &asked
}

// RecordStartedPlugins records the external plugins a daemon started at boot
// and what it learned of each one's config schema. The step reads the record
// when it names the plugins that could explain a refused name: a plugin the
// daemon started with the declaration a config holds, and that declared no
// module, defines no config section, so it is not named beside a name the
// schema does not define. Without the record every declared plugin the
// process holds no module of is named.
//
// The daemon calls it once, at boot, after it published the modules its
// plugins declared and before it serves a command (cmd/ze/hub runYANGConfig).
// Each entry MUST carry a specified outcome. Safe for concurrent use.
func RecordStartedPlugins(started []StartedPlugin) {
	for _, entry := range started {
		if entry.Schema == SchemaOutcomeUnspecified {
			panic("BUG: a started plugin was recorded with no schema outcome: " + entry.Declaration.Name)
		}
	}
	declaredStep.mu.Lock()
	defer declaredStep.mu.Unlock()
	declaredStep.started = slices.Clone(started)
}

// startedWithoutSchemaLocked reports whether the daemon started external,
// with this exact declaration, and the plugin declared no module. A plugin
// declared with another run line or another leaf is not the plugin the daemon
// started: a restart with that declaration can bring a schema.
// The caller MUST hold declaredStep.mu.
func startedWithoutSchemaLocked(external plugin.PluginConfig) bool {
	for _, entry := range declaredStep.started {
		if entry.Declaration != external {
			continue
		}
		return entry.Schema == SchemaNone
	}
	return false
}

// SchemaWithDeclaredPlugins answers the schema a command parses sources
// against, and which external plugins they declare that this process holds no
// config schema of. It is the one step every command that parses a config
// with no daemon calls, in place of YANGSchema, before it parses.
//
// It starts no program, in any process (owner ruling: with no daemon running
// ze starts no external plugin, and what needs one is not available then).
//
//  1. FirstPass reads the plugin declarations of each source.
//  2. In a process whose module set is not fixed yet (yang.ModuleSetFixed), a
//     command that runs with no daemon, the set is fixed to the compiled-in
//     one. In a process whose set is already fixed, a running daemon that
//     joined its plugins' modules when it started, nothing is published.
//  3. The schema of the process's set is answered, for the full parse.
//
// A section only an external plugin's schema defines is therefore refused by
// the full parse of a command with no daemon, as a name the schema does not
// define, and DeclaredSchemas.ParseRefusal says why and names the command that
// gets a running ze to check it: `ze config validate <file>`, which asks the
// daemon first, for a command that asked none, and the fix of the failed ask
// for one that did (Unanswered). Inside a daemon the same section parses when the
// daemon started with the plugin, and is refused naming the restart when it
// did not. A plugin the daemon started that declared no module is never named
// there: it defines no section (RecordStartedPlugins).
//
// A source the first pass refuses declares nothing here: the caller's full
// parse refuses the same `plugin` or `environment` block with the message it
// always gave.
//
// The error is a schema that does not build.
//
// The caller MUST call it before the first full parse of the process. Safe
// for concurrent use.
func SchemaWithDeclaredPlugins(sources ...ConfigSource) (*Schema, *DeclaredSchemas, error) {
	declared, err := publishDeclaredSchemas(sources)
	if err != nil {
		return nil, nil, err
	}
	schema, err := YANGSchema()
	if err != nil {
		return nil, nil, err
	}
	return schema, declared, nil
}

// PublishDeclaredSchemas is SchemaWithDeclaredPlugins for a caller that parses
// through a function that takes the schema itself (ReadConfig,
// ParseTreeForValidation): steps 1 and 2, with no schema answered.
func PublishDeclaredSchemas(sources ...ConfigSource) (*DeclaredSchemas, error) {
	return publishDeclaredSchemas(sources)
}

// publishDeclaredSchemas is steps 1 and 2 of SchemaWithDeclaredPlugins.
func publishDeclaredSchemas(sources []ConfigSource) (*DeclaredSchemas, error) {
	declared := &DeclaredSchemas{}
	if len(sources) == 1 {
		declared.path = sources[0].Path
	}
	externals := declaredExternalPlugins(sources)

	declaredStep.mu.Lock()
	defer declaredStep.mu.Unlock()
	if !yang.ModuleSetFixed() {
		// No external module: the set of a command with no daemon is the
		// compiled-in one, and it is fixed here so every later reader of the
		// process answers from it.
		if err := PublishExternalModules(nil); err != nil {
			return nil, err
		}
		declaredStep.fixedHere = true
	}
	declared.offline = declaredStep.fixedHere
	if declaredStep.unanswered != nil {
		asked := *declaredStep.unanswered
		declared.unanswered = &asked
	}

	published, _ := yang.PublishedExternalModules()
	for _, external := range externals {
		// A plugin the process published a module of is not named: either
		// that module defines its section, or its modules add only commands
		// and rpcs and it defines no section (SchemaNone). Neither explains a
		// refused name.
		ofPlugin := func(module yang.ExternalModule) bool { return module.Plugin == external.Name }
		if slices.ContainsFunc(published, ofPlugin) {
			continue
		}
		if startedWithoutSchemaLocked(external) {
			continue
		}
		declared.plugins = append(declared.plugins, external.Name)
	}
	return declared, nil
}

// declaredExternalPlugins answers each external plugin sources declare, as
// the first pass answers it, each name once, in source then config order.
func declaredExternalPlugins(sources []ConfigSource) []plugin.PluginConfig {
	var externals []plugin.PluginConfig
	for _, source := range sources {
		firstPass, err := FirstPass(source.Text, source.Path, nil)
		if err != nil {
			// refuse-by-default: guards SchemaWithDeclaredPlugins -- a source the first pass refuses declares no plugin here, and the caller's full parse refuses the same block
			continue
		}
		for _, external := range firstPass.External {
			sameName := func(held plugin.PluginConfig) bool { return held.Name == external.Name }
			if slices.ContainsFunc(externals, sameName) {
				continue
			}
			externals = append(externals, external)
		}
	}
	return externals
}

// ResetDeclaredSchemasForTest puts the process back to one that has published
// no module set (yang.ResetPublishedModulesForTest), that this file fixed
// nothing in and that recorded no started plugin, and returns the function
// that restores what it found. A test
// that calls SchemaWithDeclaredPlugins, or an entry that does, with a config
// that declares an external plugin calls it first.
// MUST be paired with the returned restore, typically through t.Cleanup. Not
// safe for concurrent use: such a test MUST NOT run in parallel with another
// test that loads YANG.
func ResetDeclaredSchemasForTest() (restore func()) {
	restorePublished := yang.ResetPublishedModulesForTest()
	declaredStep.mu.Lock()
	saved := declaredStep.fixedHere
	savedStarted := declaredStep.started
	savedUnanswered := declaredStep.unanswered
	declaredStep.fixedHere = false
	declaredStep.started = nil
	declaredStep.unanswered = nil
	declaredStep.mu.Unlock()
	return func() {
		declaredStep.mu.Lock()
		declaredStep.fixedHere = saved
		declaredStep.started = savedStarted
		declaredStep.unanswered = savedUnanswered
		declaredStep.mu.Unlock()
		restorePublished()
	}
}
