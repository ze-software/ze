// Register the config root command and its `show config *` / `validate
// config` offline shortcuts with the cmd/ze dispatcher. Storage-backed
// subcommands are bound separately via BindStorageCommands so that the
// main binary controls when the blob store is opened (after global flag
// parsing).

package config

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands, derived from storageHandlers and subcommandHandlers
// (the dispatch maps in main.go).
func subcommands() string {
	cmds := make([]string, 0, len(storageHandlers)+len(subcommandHandlers))
	for k := range storageHandlers {
		cmds = append(cmds, k)
	}
	for k := range subcommandHandlers {
		cmds = append(cmds, k)
	}
	sort.Strings(cmds)
	return strings.Join(cmds, ", ")
}

func init() {
	cmdregistry.RegisterRoot("config", cmdregistry.Meta{
		Description: "Configuration editing, formatting, validation, and history",
		Mode:        "offline",
		Section:     cmdregistry.SectionConfiguration,
		Subs:        subcommands(),
	})
	cmdregistry.MustRegisterLocalMeta("show config dump", func(args []string) int {
		return Run(append([]string{"dump"}, args...))
	}, cmdregistry.Meta{Description: "Show the fully resolved config tree as JSON. What you see is exactly what the daemon uses."})
	cmdregistry.MustRegisterLocalMeta("show config diff", func(args []string) int {
		return Run(append([]string{"diff"}, args...))
	}, cmdregistry.Meta{Description: "Show what changed between the running and candidate configurations."})
	cmdregistry.MustRegisterLocalMeta("show config fmt", func(args []string) int {
		return Run(append([]string{"fmt"}, args...))
	}, cmdregistry.Meta{Description: "Pretty-print the config with consistent formatting and ordering."})
	cmdregistry.MustRegisterLocalMeta("validate config", func(args []string) int {
		return Run(append([]string{"validate"}, args...))
	}, cmdregistry.Meta{Description: "Check your config for errors without applying anything. Reports syntax and semantic issues."})
	cmdregistry.MustRegisterLocalMeta("show config graph", func(args []string) int {
		return Run(append([]string{"graph"}, args...))
	}, cmdregistry.Meta{Description: "Show how components and peers depend on each other (DOT graph format)."})
}

// StorageResolver is the thunk supplied by cmd/ze/main.go so that the
// storage-dependent local commands (history, ls, cat) can open the
// blob store at dispatch time rather than package-load time.
type StorageResolver func() storage.Storage

// BindStorageCommands wires the three storage-dependent `show config
// *` commands. Must be called once from cmd/ze/main.go after global
// flag parsing.
func BindStorageCommands(resolve StorageResolver) {
	cmdregistry.MustRegisterLocalMeta("show config history", func(args []string) int {
		return runStorageClose(resolve, append([]string{"history"}, args...))
	}, cmdregistry.Meta{Description: "List config snapshots with timestamps and commit messages."})
	cmdregistry.MustRegisterLocalMeta("show config ls", func(args []string) int {
		return runStorageClose(resolve, append([]string{"ls"}, args...))
	}, cmdregistry.Meta{Description: "List all config snapshots stored in the blob store."})
	cmdregistry.MustRegisterLocalMeta("show config cat", func(args []string) int {
		return runStorageClose(resolve, append([]string{"cat"}, args...))
	}, cmdregistry.Meta{Description: "Print the full configuration text for a stored snapshot."})
}

// runStorageClose opens storage via the resolver, runs the command,
// and closes the store. Errors from Close are logged only; exit code
// comes from the command run.
func runStorageClose(resolve StorageResolver, args []string) int {
	store := resolve()
	defer func() {
		if err := store.Close(); err != nil {
			_ = err // best-effort cleanup before exit
		}
	}()
	return RunWithStorage(store, args)
}
