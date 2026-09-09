// Design: docs/architecture/api/commands.md — where a command is served
// Related: resolve.go — InternalPluginInfo, the registry walk this answers with
// Related: registry/setup.go — SetupResults, the record each row's outcome comes from
//
// register.go registers the two commands on the plugin noun. `show plugin list`
// answers which plugins this binary carries and what each plugin's own init()
// recorded about its setup. Both facts describe one set, so they are one row
// rather than two commands: InternalPluginInfo walks registry.All, SetupResults
// walks the same map, and a reader asking either question was being made to run
// both. `show plugin declarations` answers the other question, what each plugin
// declares, and its rows are built in declarations.go.
//
// The command answers with DATA, so `| json`, `| yaml` and `| table` are three
// renderings of one payload (ai/rules/cli.md). It is registered here rather
// than in cmd/ze because this package owns plugin introspection: removing the
// plugin host removes the command with it.

package plugin

import (
	"github.com/ze-software/ze/internal/component/command"
	cmdregistry "github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// modeOffline is the help tag for a command that answers with no running
// daemon. `show plugin list` reads a registry this process filled at init, so it is
// one.
const modeOffline = "offline"

// keyPlugins is the envelope key the rows travel under, so a caller parses one
// shape whichever format it asked for.
const keyPlugins = "plugins"

// descriptionUnregistered describes a plugin that recorded a setup outcome and
// whose Register call never completed. It stands where the registration's own
// description would be, because that registration does not exist: an empty
// cell among named ones is the row a reader skips, and this is the row the
// whole setup record exists to show.
const descriptionUnregistered = "recorded a setup outcome and did not register"

// pluginRow is one row of `show plugin list`: what the plugin IS, from its
// Registration, and what its own init() achieved, from the setup record.
//
// PluginInfo is embedded, so encoding/json writes its fields beside outcome
// and reason and every pipe operator sees one flat row.
type pluginRow struct {
	PluginInfo
	Outcome registry.SetupOutcome `json:"outcome"`
	Reason  string                `json:"reason,omitempty"`
}

// The command path is written as a literal at each call, not through a const.
// `./le docvalid command-contract` parses this file to check that every YANG
// command has a handler, and it reads a string literal; a const identifier
// reaches it as no path at all, so the command would be reported as declared in
// YANG and served by nobody.
func init() {
	cmdregistry.MustRegisterLocalData("show plugin list", dataPlugins, cmdregistry.Meta{
		Description: "Every plugin compiled into this binary, with its setup outcome.",
		LongHelp: "One row is written for each plugin the binary links. The row names the " +
			"families it registers, the RFCs it implements and the capability codes it " +
			"negotiates, beside the outcome its own init() recorded and the reason for it.",
		Mode: modeOffline,
	}, command.RenderLocalAnswer)

	// The answer is rows read against declared column names, so every row
	// operator applies and the published page can say so before the command
	// runs. reason is last because it is free text an operator acts on, and a
	// long cell in the middle pushes the short ones off the terminal.
	command.RegisterShape([]string{"show plugin list"}, command.ShapeTab)
	command.RegisterColumns([]string{"show plugin list"},
		command.ColumnOrder{"name", "description", "outcome", "families", "rfcs", "capabilities", "reason"},
	)

	// `show plugin declarations` is the second command on this noun: the list
	// answers which plugins the binary carries, and this one answers what each
	// plugin declares. Its rows and its states are declarations.go.
	//
	// The `config <path>` form is a command of its own because the keyword is a
	// command word rather than a value: the CLI tree carries it, so completion
	// offers it and the usage line states the grammar. Both forms answer one
	// payload, so both declare the same shape and the same columns.
	cmdregistry.MustRegisterLocalData("show plugin declarations", dataDeclarations, cmdregistry.Meta{
		Description: "What each plugin declares: the commands it serves and the pipe aliases it puts on them.",
		LongHelp: "One row is written for each plugin this binary carries. A plugin whose " +
			"declaration could not be read keeps its row and says why in the state field, so a " +
			"plugin is never missing from the answer.",
		Mode: modeOffline,
	}, command.RenderLocalAnswer)
	cmdregistry.MustRegisterLocalData("show plugin declarations config", dataDeclarationsConfig, cmdregistry.Meta{
		Description: "The same answer, plus a row for each plugin a config file names.",
		LongHelp: "The file is read with the daemon's own loader, so the answer covers the plugins " +
			"the daemon would start from it, external ones included. A plugin the file names and " +
			"this binary also carries keeps one row rather than two.",
		Mode: modeOffline,
	}, command.RenderLocalAnswer)

	command.RegisterShape([]string{
		"show plugin declarations",
		"show plugin declarations config",
	}, command.ShapeTab)
	command.RegisterColumns([]string{
		"show plugin declarations",
		"show plugin declarations config",
	}, command.ColumnOrder{"name", "kind", "state", "commands", "pipes", "reason"})
}

// dataPlugins answers `show plugin list` with every plugin this binary carries and
// the setup outcome it recorded, in name order.
//
// It takes no arguments: the answer is the whole set, and a reader who wants
// one plugin narrows it with `| match <name>`.
func dataPlugins(_ []string) (any, int) {
	return Map{keyPlugins: pluginRows()}, 0
}

// pluginRows joins each recorded setup outcome to the registration of the same
// name, in name order.
//
// registry.SetupResults decides the rows, not InternalPluginInfo. The two hold
// the same names by construction, because SetupResults reads the map that
// registry.All sorts, with ONE divergence: SetupResults also carries a plugin
// that RECORDED and then never completed its Register call. Dropping that row
// would leave its absence reading as "not built into this binary", which is
// the silence the setup record exists to remove.
//
// A registered plugin that recorded nothing keeps a row too, with the unknown
// outcome, for the same reason.
func pluginRows() []pluginRow {
	described := make(map[string]PluginInfo)
	for _, info := range InternalPluginInfo() {
		described[info.Name] = info
	}

	results := registry.SetupResults()
	rows := make([]pluginRow, len(results))
	for index, result := range results {
		info, registered := described[result.Plugin]
		if !registered {
			info = PluginInfo{Name: result.Plugin, Description: descriptionUnregistered}
		}
		rows[index] = pluginRow{PluginInfo: info, Outcome: result.Outcome, Reason: result.Reason}
	}
	return rows
}
