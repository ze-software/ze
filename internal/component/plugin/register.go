// Design: docs/architecture/api/commands.md — where a command is served
// Related: resolve.go — InternalPluginInfo, the registry walk this answers with
// Related: registry/setup.go — SetupResults, the record `show module list` reads
//
// register.go registers `show plugins`, the command that answers which plugins
// this binary carries, and `show module list`, the command that answers what
// each module's own init() recorded about its setup.
//
// Each command answers with DATA, so `| json`, `| yaml` and `| table` are
// three renderings of one payload (ai/rules/cli.md). They are registered here
// rather than in cmd/ze because this package owns plugin introspection:
// removing the plugin host removes both commands with it.

package plugin

import (
	"github.com/ze-software/ze/internal/component/command"
	cmdregistry "github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// modeOffline is the help tag for a command that answers with no running
// daemon. `show plugins` reads a registry this process filled at init, so it is
// one.
const modeOffline = "offline"

// keyPlugins and keyModules are the envelope keys the rows travel under, so a
// caller parses one shape whichever format it asked for.
const (
	keyPlugins = "plugins"
	keyModules = "modules"
)

// The command path is written as a literal at each call, not through a const.
// `./le docvalid command-contract` parses this file to check that every YANG
// command has a handler, and it reads a string literal; a const identifier
// reaches it as no path at all, so the command would be reported as declared in
// YANG and served by nobody.
func init() {
	cmdregistry.MustRegisterLocalData("show plugins", dataPlugins, cmdregistry.Meta{
		Description: "Every plugin compiled into this binary, with its families, RFCs and capability codes.",
		Mode:        modeOffline,
	}, command.RenderLocalAnswer)

	// The answer is rows read against declared column names, so every row
	// operator applies and the published page can say so before the command
	// runs.
	command.RegisterShape([]string{"show plugins"}, command.ShapeTab)
	command.RegisterColumns([]string{"show plugins"},
		command.ColumnOrder{"name", "description", "families", "rfcs", "capabilities"},
	)

	cmdregistry.MustRegisterLocalData("show module list", dataModules, cmdregistry.Meta{
		Description: "Every module with the setup outcome its own init() recorded, replayed rather than probed.",
		Mode:        modeOffline,
	}, command.RenderLocalAnswer)

	command.RegisterShape([]string{"show module list"}, command.ShapeTab)
	command.RegisterColumns([]string{"show module list"},
		command.ColumnOrder{"module", "outcome", "reason"},
	)
}

// dataModules answers `show module list` with every module and the setup
// outcome it recorded, in name order. registry.SetupResults derives the module
// set from the registry, so a module that recorded nothing is listed as
// unknown rather than dropped.
//
// It takes no arguments: the answer is the whole set, and a reader who wants
// one module narrows it with `| match <name>`.
func dataModules(_ []string) (any, int) {
	return Map{keyModules: registry.SetupResults()}, 0
}

// dataPlugins answers `show plugins` with every plugin this binary carries, in
// name order. InternalPluginInfo walks the registry, and registry.All sorts.
//
// It takes no arguments: the answer is the whole set, and a reader who wants
// one plugin narrows it with `| match <name>`.
func dataPlugins(_ []string) (any, int) {
	return Map{keyPlugins: InternalPluginInfo()}, 0
}
