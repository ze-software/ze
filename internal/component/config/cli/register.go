// Design: docs/architecture/api/commands.md — config command ownership
//
// Register the `config` root command and its `show config *` / `validate
// config` offline shortcuts with the importable command registry. This is the
// owner package: the offline configuration CLI lives with
// internal/component/config, not under cmd/ze.
//
// The root command and the snapshot shortcuts (history, list, cat) need the blob
// store, which is opened only after global flag parsing. The root handler
// receives it through the RuntimeContext; the local shortcuts (which have the
// func(args)int signature and so get no context) resolve it lazily through the
// registry's runtime storage resolver, which cmd/ze/main.go installs.
package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/storage"
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
	sort.Strings(cmds)
	return textbuf.Join(cmds, ", ")
}

// storageShortcut builds a local handler for a storage-backed `show config
// <sub>` shortcut. It resolves the blob store lazily through the registry's
// runtime storage resolver, runs the command, and closes the store.
func storageShortcut(sub string) registry.LocalHandler {
	return func(args []string) int {
		store, ok := registry.RuntimeStorage().(storage.Storage)
		if !ok {
			fmt.Fprintln(os.Stderr, "error: config storage unavailable")
			return 1
		}
		defer func() {
			if err := store.Close(); err != nil {
				_ = err // best-effort cleanup before exit
			}
		}()
		return RunWithStorage(store, append([]string{sub}, args...))
	}
}

func init() {
	registry.MustRegisterRootHandler("config", func(rctx *registry.RuntimeContext, args []string) int {
		store, ok := registry.StorageAs[storage.Storage](rctx)
		if !ok {
			fmt.Fprintln(os.Stderr, "error: config requires storage")
			return 1
		}
		code := RunWithStorage(store, args)
		if err := store.Close(); err != nil {
			_ = err // best-effort cleanup before exit
		}
		return code
	}, registry.Meta{
		Description: "Configuration editing, formatting, validation, and history",
		Mode:        modeOffline,
		Section:     registry.SectionConfiguration,
		Subs:        subcommands(),
	})

	// Non-storage shortcuts: read the candidate/running config without the blob.
	// Each of these answers with DATA, so the operator's pipe chain renders it
	// and no command carries a rendering flag of its own. They printed and
	// returned an exit code before, which is why
	// `ze cli -c "show config dump x.conf | json"` answered `unknown command`.
	registry.MustRegisterLocalData("show config dump", dataDump, registry.Meta{
		Description: "Show the fully resolved config tree. What you see is exactly what the daemon uses.",
		Mode:        modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show config diff", dataDiff, registry.Meta{
		Description: "Show what changed between the running and candidate configurations.",
		Mode:        modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalMeta("show config fmt", func(args []string) int {
		return Run(append([]string{"fmt"}, args...))
	}, registry.Meta{Description: "Pretty-print the config with consistent formatting and ordering."})
	registry.MustRegisterLocalData("validate config", dataValidate, registry.Meta{
		Description: "Check a config for errors without applying it.",
		LongHelp: "Both the grammar of the file and the meaning of its values are checked, and each " +
			"problem is reported with the diagnostic code that explains it.",
		Mode: modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalMeta("show config graph", func(args []string) int {
		return Run(append([]string{"graph"}, args...))
	}, registry.Meta{Description: "Show how components and peers depend on each other (DOT graph format)."})

	// Storage-backed shortcuts: resolve the blob store lazily at dispatch.
	registry.MustRegisterLocalData("show config history", dataHistory, registry.Meta{
		Description: "List config snapshots with timestamps and commit messages.",
		Mode:        modeOffline,
	}, command.RenderLocalAnswer)
	registry.MustRegisterLocalData("show config list", dataList, registry.Meta{
		Description: "List all config snapshots stored in the blob store.",
		Mode:        modeOffline,
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
		registry.Meta{Description: "Print the full configuration text for a stored snapshot."})
}
