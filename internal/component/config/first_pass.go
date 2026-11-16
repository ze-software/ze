// Design: docs/architecture/config/syntax.md — the first pass over a config
// Related: loader.go — the full load, whose extractors the first pass calls
// Related: parser.go — Parser.SkipUnknownRoots, the brace form's skip
// Related: setparser.go — SetParser.SkipUnknownRoots, the set form's skip
// Related: yang_schema.go — PluginOnlySchema, the schema of the first pass

package config

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
)

// firstPassRoots names the roots Ze reads before any plugin runs: `plugin`,
// read by ExtractPluginsFromTree and ExtractHubConfig, and `environment`, read
// by ExtractEnvironment. PluginOnlySchema holds these and no other.
var firstPassRoots = []string{"plugin", "environment"}

// FirstPassResult is what a config and the command line say about the plugins
// to run, read before the full config can be parsed. FirstPass builds it.
type FirstPassResult struct {
	// External holds each declared plugin that runs as another program: an
	// `external` block with a `run` string, or a command line named with
	// --plugin. A plugin compiled into ze is absent: its module is already
	// registered. An inactive entry is absent too.
	External []plugin.PluginConfig
	// Hub is the listener a forked plugin connects back to, from `plugin hub`.
	// The zero value means the config declares none.
	Hub plugin.HubConfig
	// Environment holds the values of the `environment` block, as
	// ExtractEnvironment answers them; nil when the config holds no block.
	// FirstPass applies none of them to the running process.
	Environment map[string]map[string]string
	// ConfigDir is the directory the plugins' work directory resolves
	// against, the one LoadConfigResult.ConfigDir answers for the same path.
	ConfigDir string
}

// FirstPass reads the plugin declarations of a config before any protocol
// starts: the `plugin` block of input and the plugins cliPlugins names. It is
// the first step of the load order, and the only one that can run while the
// modules of the external plugins are unknown.
//
// input is parsed by the same Parser or SetParser as the full load, over
// PluginOnlySchema, in SkipUnknownRoots mode: a top-level section the schema
// does not hold is skipped whole, in the brace form and in the set form. The
// roots it does hold are parsed strictly, so a refusal inside `plugin` or
// `environment` is the refusal the full load gives. It resolves no YANG
// module of its own, and a config that declares no external plugin answers an
// empty External.
//
// A daemon that boots MUST, before it parses the full config (LoadConfig,
// ReadConfig):
//
//   - obtain the Stage 1 declaration of every plugin in External by running
//     it, join the modules they declare to the compiled-in set and publish
//     the joined set. With an empty External there is nothing to run, join or
//     publish, and the caller MUST go straight to the full parse;
//   - keep the outcome of a plugin that did not declare, with its reason, so
//     the refusal of a root still unknown at the full parse can name it.
//
// A command with no daemon MUST NOT run a plugin of External (owner ruling):
// it parses against the compiled-in set and says which plugins the config
// declares beside the refusal of a name that set does not define.
//
// The caller MUST NOT take External for the plugins the daemon runs. The full
// load answers that list (LoadConfigResult.Plugins): it adds the compiled-in
// plugins, the plugins a section declares inline and their dependencies, none
// of which the first pass can see. A plugin declared inline under a root the
// first pass skips is not in External.
//
// The caller MUST NOT publish Environment as the config's environment: the
// validators have not judged this config, and LoadConfig publishes the block
// once they have. It is here for what a plugin needs to be forked.
//
// The daemon calls it first at boot (cmd/ze/hub runYANGConfig, then
// joinDeclaredPlugins for the run, the join and the publish), and the start
// path uses it where it needs the `plugin` block before boot (cmd/ze
// judgeConfigBeforeBoot, extractManagedClientConfig). The commands that parse
// a config with no daemon call it through SchemaWithDeclaredPlugins
// (declared_schemas.go), which runs no plugin and keeps the names it read.
func FirstPass(input, configPath string, cliPlugins []string) (*FirstPassResult, error) {
	schema, err := PluginOnlySchema()
	if err != nil {
		return nil, fmt.Errorf("YANG schema: %w", err)
	}

	tree, err := parseFirstPass(schema, input)
	if err != nil {
		return nil, &parseConfigError{cause: err}
	}
	// As the full load does before any reader: an `inactive:` declaration is
	// one the operator switched off, and its plugin MUST NOT be run.
	PruneInactive(tree, schema)

	declared, err := ExtractPluginsFromTree(tree)
	if err != nil {
		return nil, err
	}
	declared, err = MergeCliPlugins(declared, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("resolve plugins: %w", err)
	}

	hub, err := ExtractHubConfig(tree)
	if err != nil {
		return nil, err
	}
	configDir, err := configDirOf(configPath)
	if err != nil {
		return nil, err
	}

	external := make([]plugin.PluginConfig, 0, len(declared))
	for _, candidate := range declared {
		if candidate.RunsExternalProgram() {
			external = append(external, candidate)
		}
	}

	return &FirstPassResult{
		External:    external,
		Hub:         hub,
		Environment: ExtractEnvironment(tree),
		ConfigDir:   configDir,
	}, nil
}

// parseFirstPass parses input over schema in the form DetectFormat finds,
// each parser skipping the roots schema does not hold. The tree holds the
// first-pass roots alone and never leaves this file.
func parseFirstPass(schema *Schema, input string) (*Tree, error) {
	switch DetectFormat(input) {
	case FormatHierarchical:
		parser := NewParser(schema)
		parser.SkipUnknownRoots()
		return parser.Parse(input)
	case FormatSet:
		parser := NewSetParser(schema)
		parser.SkipUnknownRoots()
		return parser.Parse(input)
	case FormatSetMeta:
		parser := NewSetParser(schema)
		parser.SkipUnknownRoots()
		tree, _, err := parser.ParseWithMeta(input)
		return tree, err
	}
	panic("BUG: invalid detected config format")
}
