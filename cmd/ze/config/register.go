// Register the config root command and its `show config *` / `validate
// config` offline shortcuts with the cmd/ze dispatcher. Storage-backed
// subcommands are bound separately via BindStorageCommands so that the
// main binary controls when the blob store is opened (after global flag
// parsing).

package config

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

func init() {
	cmdregistry.RegisterRoot("config", cmdregistry.Meta{
		Description: "Configuration management",
		Mode:        "offline",
		Subs:        "edit, set, deactivate, activate, migrate, rollback, archive, import, rename",
	})
	cmdregistry.MustRegisterLocal("show config dump", func(args []string) int {
		return Run(append([]string{"dump"}, args...))
	})
	cmdregistry.MustRegisterLocal("show config diff", func(args []string) int {
		return Run(append([]string{"diff"}, args...))
	})
	cmdregistry.MustRegisterLocal("show config fmt", func(args []string) int {
		return Run(append([]string{"fmt"}, args...))
	})
	cmdregistry.MustRegisterLocal("validate config", func(args []string) int {
		return Run(append([]string{"validate"}, args...))
	})
}

// StorageResolver is the thunk supplied by cmd/ze/main.go so that the
// storage-dependent local commands (history, ls, cat) can open the
// blob store at dispatch time rather than package-load time.
type StorageResolver func() storage.Storage

// BindStorageCommands wires the three storage-dependent `show config
// *` commands. Must be called once from cmd/ze/main.go after global
// flag parsing.
func BindStorageCommands(resolve StorageResolver) {
	cmdregistry.MustRegisterLocal("show config history", func(args []string) int {
		return runStorageClose(resolve, append([]string{"history"}, args...))
	})
	cmdregistry.MustRegisterLocal("show config ls", func(args []string) int {
		return runStorageClose(resolve, append([]string{"ls"}, args...))
	})
	cmdregistry.MustRegisterLocal("show config cat", func(args []string) int {
		return runStorageClose(resolve, append([]string{"cat"}, args...))
	})
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
