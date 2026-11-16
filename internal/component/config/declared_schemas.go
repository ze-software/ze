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
	"github.com/ze-software/ze/internal/core/opererr"
)

// ConfigSource is one config a command is about to parse: its text, and the
// path it was read from, which is "" or "-" for a text with no file.
type ConfigSource struct {
	Text string
	Path string
}

// daemonSession is the command that opens a session a running ze serves, and
// daemonValidation the command typed there that checks a config file with the
// schema that ze holds. `ze cli -c "show config validation <file>"` is not the
// fix: `ze cli -c` answers that command in its own process
// (command.ServeLocal), which is the process this file refuses in.
const (
	daemonSession    = "ze cli"
	daemonValidation = "show config validation "
	restartDaemon    = "restart ze"
)

// DeclaredSchemas is what SchemaWithDeclaredPlugins learned about the external
// plugins its configs declare. The zero value holds no plugin. Its fields are
// never written after construction.
type DeclaredSchemas struct {
	// plugins holds the name of each declared external plugin this process
	// holds no config schema of, each once, in source then config order.
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

// WithoutSchema answers the name of each declared external plugin this
// process holds no config schema of, each once, in source then config order.
func (d *DeclaredSchemas) WithoutSchema() []string {
	return slices.Clone(d.plugins)
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
	return opererr.New(d.problem(path), d.why(), d.fix())
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
	var held *opererr.Error
	if errors.As(refusal, &held) {
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
	return opererr.Wrap(refusal, d.problem(path), d.why()+". What the parser refused", d.fix())
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
func (d *DeclaredSchemas) why() string {
	declares := "the config declares " + pluginNames(d.plugins)
	if !d.offline {
		return declares + ", of which this ze holds no config schema: ze reads the config schema of an " +
			"external plugin once, when it starts, from the plugins declared then. A section that such a " +
			"plugin defines is refused until ze starts with the plugin declared and the plugin declares its schema"
	}
	file := "<config-file>"
	if namesFile(d.path) {
		file = d.path
	}
	return declares + "; the config schema of an external plugin is not compiled into ze: ze reads it " +
		"from the plugin when `ze start` runs it. This command does not ask a running ze and starts no " +
		"plugin, so it cannot check a section that such a plugin defines. A running ze that started with " +
		"these plugins can: type `" + daemonValidation + file + "` in the session the command below opens"
}

// fix is the one command that reaches a ze holding the schema.
func (d *DeclaredSchemas) fix() string {
	if d.offline {
		return daemonSession
	}
	return restartDaemon
}

// namesFile reports whether path names a file, as opposed to a text with no
// file ("" and "-").
func namesFile(path string) bool {
	if path == "" {
		return false
	}
	return path != "-"
}

// pluginNames writes names as `external plugin "a"`, or `external plugins "a",
// "b"` for several. names MUST hold at least one name.
func pluginNames(names []string) string {
	text := "external plugin"
	if len(names) > 1 {
		text += "s"
	}
	for index, name := range names {
		if index > 0 {
			text += ","
		}
		text += " \"" + name + "\""
	}
	return text
}

// declaredStep records what this file did to the process's module set.
var declaredStep struct {
	mu sync.Mutex
	// fixedHere is true once this file fixed the process's module set to the
	// compiled-in one, which only a process with no daemon in it lets it do.
	fixedHere bool
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
// reaches a running ze. Inside a daemon the same section parses when the
// daemon started with the plugin, and is refused naming the restart when it
// did not.
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
	names := declaredExternalPlugins(sources)

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

	published, _ := yang.PublishedExternalModules()
	for _, name := range names {
		ofPlugin := func(module yang.ExternalModule) bool { return module.Plugin == name }
		if slices.ContainsFunc(published, ofPlugin) {
			continue
		}
		declared.plugins = append(declared.plugins, name)
	}
	return declared, nil
}

// declaredExternalPlugins answers the name of each external plugin sources
// declare, each once, in source then config order.
func declaredExternalPlugins(sources []ConfigSource) []string {
	var names []string
	for _, source := range sources {
		firstPass, err := FirstPass(source.Text, source.Path, nil)
		if err != nil {
			// refuse-by-default: guards SchemaWithDeclaredPlugins -- a source the first pass refuses declares no plugin here, and the caller's full parse refuses the same block
			continue
		}
		for _, external := range firstPass.External {
			if slices.Contains(names, external.Name) {
				continue
			}
			names = append(names, external.Name)
		}
	}
	return names
}

// ResetDeclaredSchemasForTest puts the process back to one that has published
// no module set (yang.ResetPublishedModulesForTest) and that this file fixed
// nothing in, and returns the function that restores what it found. A test
// that calls SchemaWithDeclaredPlugins, or an entry that does, with a config
// that declares an external plugin calls it first.
// MUST be paired with the returned restore, typically through t.Cleanup. Not
// safe for concurrent use: such a test MUST NOT run in parallel with another
// test that loads YANG.
func ResetDeclaredSchemasForTest() (restore func()) {
	restorePublished := yang.ResetPublishedModulesForTest()
	declaredStep.mu.Lock()
	saved := declaredStep.fixedHere
	declaredStep.fixedHere = false
	declaredStep.mu.Unlock()
	return func() {
		declaredStep.mu.Lock()
		declaredStep.fixedHere = saved
		declaredStep.mu.Unlock()
		restorePublished()
	}
}
