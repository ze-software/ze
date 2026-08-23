// Design: docs/architecture/api/commands.md — schema command ownership
//
// Register the `schema` root command and its `show schema *` offline shortcuts
// with the importable command registry. This is the owner package: the offline
// schema-discovery CLI lives alongside the YANG tooling under
// internal/component/config, not under cmd/ze. The root handler forwards the
// process --plugin list from the runtime context, matching the old
// `schema.Run(args, plugins)` dispatch.
package cli

import (
	"sort"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// schemaCommands lists the user-facing subcommand names, kept in sync with the
// switch cases in Run (main.go). "help" and "show" (which takes a module
// argument) are excluded from Meta.Subs.
var schemaCommands = []string{"list", "methods", "events", "handlers", "protocol"}

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands (single source of truth for Meta.Subs and error messages).
func subcommands() string {
	sorted := make([]string, len(schemaCommands))
	copy(sorted, schemaCommands)
	sort.Strings(sorted)
	return textbuf.Join(sorted, ", ")
}

func init() {
	registry.MustRegisterRootHandler("schema", func(rctx *registry.RuntimeContext, args []string) int {
		var plugins []string
		if rctx != nil {
			plugins = rctx.Plugins
		}
		return Run(args, plugins)
	}, registry.Meta{
		Description: "Schema discovery",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        subcommands(),
	})
	// These five answer with DATA, so their answers go through the pipe layer
	// like any other command's. They printed a table and returned an exit code
	// before, which is why `ze cli -c "show schema list | json"` answered
	// `unknown command`: YANG declares a wire method for each and no daemon
	// handler implements one.
	registry.MustRegisterLocalData("show schema list", dataList, registry.Meta{
		Description: "Every registered schema module, with its namespace.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show schema methods", dataMethods, registry.Meta{
		Description: "Every RPC a schema module declares. Narrow it with a module name.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show schema events", dataEvents, registry.Meta{
		Description: "Every notification a schema module declares. Narrow it with a module name.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show schema handlers", dataHandlers, registry.Meta{
		Description: "Which module serves each handler path.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show schema protocol", dataProtocol, registry.Meta{
		Description: "The hub architecture protocol version.",
		Mode:        "offline",
	}, command.RenderLocalAnswer)

	// Four answer rows read against declared column names; `show schema
	// protocol` answers ONE document, so it declares doc and the row operators
	// are refused over it by name rather than answering something plausible.
	command.RegisterShape([]string{"show schema list", "show schema methods",
		"show schema events", "show schema handlers"}, command.ShapeTab)
	command.RegisterShape([]string{"show schema protocol"}, command.ShapeDoc)

	command.RegisterColumns([]string{"show schema list"},
		command.ColumnOrder{"module", "namespace", "wants-config", "imports"})
	command.RegisterColumns([]string{"show schema methods", "show schema events"},
		command.ColumnOrder{"method", "module", "description"})
	command.RegisterColumns([]string{"show schema handlers"},
		command.ColumnOrder{"handler", "module"})
}
