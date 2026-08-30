// Design: docs/architecture/api/commands.md — where a command is served
// Related: resolve.go — InternalPluginInfo, the registry walk this answers with
//
// register.go registers `show plugins`, the command that answers which plugins
// this binary carries.
//
// The command answers with DATA, so `| json`, `| yaml` and `| table` are three
// renderings of one payload (ai/rules/cli.md). It is registered here rather
// than in cmd/ze because this package owns plugin introspection: removing the
// plugin host removes the command with it.

package plugin

import (
	"github.com/ze-software/ze/internal/component/command"
	cmdregistry "github.com/ze-software/ze/internal/component/command/registry"
)

// modeOffline is the help tag for a command that answers with no running
// daemon. `show plugins` reads a registry this process filled at init, so it is
// one.
const modeOffline = "offline"

// keyPlugins is the envelope key the rows travel under, so a caller parses one
// shape whichever format it asked for.
const keyPlugins = "plugins"

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
}

// dataPlugins answers `show plugins` with every plugin this binary carries, in
// name order. InternalPluginInfo walks the registry, and registry.All sorts.
//
// It takes no arguments: the answer is the whole set, and a reader who wants
// one plugin narrows it with `| match <name>`.
func dataPlugins(_ []string) (any, int) {
	return Map{keyPlugins: InternalPluginInfo()}, 0
}
