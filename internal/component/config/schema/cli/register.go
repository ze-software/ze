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
	registry.MustRegisterLocal("show schema list", func(args []string) int {
		return Run(append([]string{"list"}, args...), nil)
	})
	registry.MustRegisterLocal("show schema methods", func(args []string) int {
		return Run(append([]string{"methods"}, args...), nil)
	})
	registry.MustRegisterLocal("show schema events", func(args []string) int {
		return Run(append([]string{"events"}, args...), nil)
	})
	registry.MustRegisterLocal("show schema handlers", func(args []string) int {
		return Run(append([]string{"handlers"}, args...), nil)
	})
	registry.MustRegisterLocal("show schema protocol", func(_ []string) int {
		return Run([]string{"protocol"}, nil)
	})
}
