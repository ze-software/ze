// Design: docs/architecture/config/syntax.md — config CLI commands
// Detail: cmd_edit.go — edit subcommand handler
// Detail: cmd_check.go — check subcommand handler
// Detail: cmd_migrate.go — migrate subcommand handler
// Detail: cmd_fmt.go — fmt subcommand handler
// Detail: cmd_dump.go — dump subcommand handler
// Detail: cmd_diff.go — diff subcommand handler
// Detail: cmd_completion.go — completion query handler
// Detail: cmd_history.go — history subcommand handler
// Detail: cmd_rollback.go — rollback subcommand handler
// Detail: cmd_set.go — set subcommand handler
// Detail: cmd_archive.go — archive subcommand handler
//
// Package config provides the ze config subcommand.
package config

import (
	"fmt"
	"io"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// Exit codes for config commands.
const (
	exitOK              = 0 // Success
	exitMigrationNeeded = 1 // Config needs migration (check command)
	exitError           = 2 // Error (file not found, parse error, etc.)
)

// storageHandlers maps subcommand names to handler functions that receive storage.
var storageHandlers = map[string]func(storage.Storage, []string) int{
	"edit":     cmdEditWithStorage,
	"set":      cmdSetWithStorage,
	"history":  cmdHistoryWithStorage,
	"rollback": cmdRollbackWithStorage,
	"archive":  cmdArchiveWithStorage,
	"diff":     cmdDiffWithStorage,
	"ls":       cmdLsWithStorage,
	"cat":      cmdCatWithStorage,
}

// subcommandHandlers maps subcommand names to their handler functions.
// Using a map avoids both if-else chains (gocritic lint) and switch default
// (hook false positive for /config/ path).
var subcommandHandlers = map[string]func([]string) int{
	"check":      cmdCheck,
	"migrate":    cmdMigrate,
	"fmt":        cmdFmt,
	"dump":       cmdDump,
	"completion": cmdCompletion,
}

// Run executes the config subcommand with filesystem storage (backward compat).
// Returns exit code.
func Run(args []string) int {
	return RunWithStorage(storage.NewFilesystem(), args)
}

// RunWithStorage executes the config subcommand with the given storage backend.
// Returns exit code.
func RunWithStorage(store storage.Storage, args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	// Check for help first
	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" {
		usage()
		return 0
	}

	// Look up storage-aware handler first
	if handler, ok := storageHandlers[subcmd]; ok {
		return handler(store, subArgs)
	}

	// Look up plain handler
	if handler, ok := subcommandHandlers[subcmd]; ok {
		return handler(subArgs)
	}

	// Unknown subcommand
	fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", subcmd)
	allCmds := make([]string, 0, len(storageHandlers)+len(subcommandHandlers))
	for k := range storageHandlers {
		allCmds = append(allCmds, k)
	}
	for k := range subcommandHandlers {
		allCmds = append(allCmds, k)
	}
	if s := suggest.Command(subcmd, allCmds); s != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
	}
	usage()
	return 1
}

// loadConfigData reads config from the given path via storage, or stdin if path is "-".
func loadConfigData(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path) //nolint:gosec // Config path from user CLI
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze config <command> [options]

Configuration management commands.

Commands:
  edit [file]       Interactive configuration editor (default: <identity>.conf)
  check <file>      Check config status and deprecated patterns
  migrate <file>    Convert configuration to current format
  fmt <file>        Format and normalize configuration file
  dump <file>       Dump parsed configuration
  diff <f1> <f2>    Compare two configuration files
  diff <N> <file>   Compare rollback revision N against current
  history <file>    List rollback revisions
  rollback <N> <file>  Restore from rollback revision N
  set <file> <path> <value>  Set a configuration value
  archive <name> <file>  Archive config to named destination
  ls [prefix]         List config files in database (default: file/)
  cat <key>           Print content of a database key
  completion <file>   Query completion engine (testing/debugging)

Options:
  -f                Use filesystem directly, bypass blob store

Examples:
  ze config edit                         Edit default config from blob
  ze config edit router.conf             Edit router.conf from blob
  ze config edit -f router.conf          Edit router.conf from filesystem
  ze config check config.conf
  ze config migrate config.conf -o new.conf
  ze config set config.conf bgp local as 65000
`)
}
