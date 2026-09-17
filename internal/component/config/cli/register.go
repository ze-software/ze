// Design: docs/architecture/api/commands.md — config command ownership
//
// Register the `config` root command and its `show config *` / `validate
// config` offline shortcuts with the importable command registry. This is the
// owner package: the offline configuration CLI lives with
// internal/component/config, not under cmd/ze.
//
// Every handler parses its operands before resolving an optional store. Loose
// file readers never resolve runtime storage.
package cli

import (
	"slices"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands, derived from storageHandlers and subcommandHandlers (the
// dispatch maps in main.go).
func subcommands() string {
	cmds := make([]string, 0, len(storageHandlers)+len(subcommandHandlers))
	for k := range storageHandlers {
		cmds = append(cmds, k)
	}
	for k := range subcommandHandlers {
		cmds = append(cmds, k)
	}
	slices.Sort(cmds)
	return textbuf.Join(cmds, ", ")
}

// storageShortcut shares argument parsing with the config root command.
func storageShortcut(sub string) registry.LocalHandler {
	return func(args []string) int {
		return Run(append([]string{sub}, args...))
	}
}

func init() {
	registry.MustRegisterRootHandler("config", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		ShortHelp: "Configuration editing, formatting, validation, and history",
		Mode:      modeOffline,
		Section:   registry.SectionConfiguration,
		Subs:      subcommands(),
	})

	// Flag inventory for shell completion (registration over hardcoding).
	// Mirrors the flag.FlagSet declarations in cmd_import.go.
	registry.RegisterCommandFlags("config import", []registry.FlagSpec{
		{Name: "--name", Description: "destination config name (one input only)", ValueHint: registry.FlagValueNone},
		{Name: "--dir", Description: "destination store folder", ValueHint: registry.FlagValueFile},
	})

	// Read-only shortcuts read their explicit candidate/running config source.
	// Each of these answers with DATA, so the operator's pipe chain renders it
	// and no command carries a rendering flag of its own. They printed and
	// returned an exit code before, which is why
	// `ze cli -c "show config dump x.conf | json"` answered `unknown command`.
	registry.MustRegisterLocalData("show config dump", dataDump, registry.Meta{
		ShortHelp: "Show the fully resolved config tree. What you see is exactly what the daemon uses.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show config diff", dataDiff, registry.Meta{
		ShortHelp: "Show what changed between the running and candidate configurations.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalMeta("show config fmt", func(args []string) int {
		return Run(append([]string{"fmt"}, args...))
	}, registry.Meta{ShortHelp: "Pretty-print the config with consistent formatting and ordering."})
	registry.MustRegisterLocalData("validate config", dataValidate, registry.Meta{
		ShortHelp: "Check a config for errors without applying it.",
		Description: "Both the grammar of the file and the meaning of its values are checked, and each " +
			"problem is reported with the diagnostic code that explains it.",
		Mode: modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalMeta("show config graph", func(args []string) int {
		return Run(append([]string{"graph"}, args...))
	}, registry.Meta{
		ShortHelp: "Show how components and peers depend on each other, as JSON.",
		Description: "It takes a config file path, or - to read the file on stdin. Inactive blocks are " +
			"pruned before the graph is built, so a deactivated peer contributes no edge and the " +
			"answer describes the config as it would run.",
	})

	// History shortcuts resolve the store lazily after parsing arguments.
	registry.MustRegisterLocalData("show config history", dataHistory, registry.Meta{
		ShortHelp: "List config snapshots with timestamps and commit messages.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show config list", dataList, registry.Meta{
		ShortHelp: "List stored config snapshots and loose config files.",
		Mode:      modeOffline,
	}, command.RenderLocalAnswer)

	// dump, diff and the validation verdict are each ONE document, so the row
	// operators are refused over them by name. history and list answer rows.
	command.RegisterShape([]string{
		"show config dump", "show config diff", "validate config",
	}, command.ShapeDoc)
	command.RegisterShape([]string{
		"show config history", "show config list",
	}, command.ShapeTab)
	command.RegisterColumns([]string{"show config history"},
		command.ColumnOrder{keyRevision, "timestamp", keyPath, "state"})
	command.RegisterColumns([]string{"show config list"}, command.ColumnOrder{keySource, keyPath})
	registry.MustRegisterLocalMeta("show config cat", storageShortcut("cat"),
		registry.Meta{ShortHelp: "Print the full configuration text for a stored snapshot."})
}
