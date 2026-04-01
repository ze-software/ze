// Design: docs/architecture/config/syntax.md — config CLI commands
// Detail: cmd_edit.go — edit subcommand handler
// Detail: cmd_validate.go — validate subcommand handler
// Detail: cmd_migrate.go — migrate subcommand handler
// Detail: cmd_fmt.go — fmt subcommand handler
// Detail: cmd_dump.go — dump subcommand handler
// Detail: cmd_diff.go — diff subcommand handler
// Detail: cmd_completion.go — completion query handler
// Detail: cmd_history.go — history subcommand handler
// Detail: cmd_rollback.go — rollback subcommand handler
// Detail: cmd_set.go — set subcommand handler
// Detail: cmd_archive.go — archive subcommand handler
// Detail: cmd_import.go — import subcommand handler
// Detail: cmd_rename.go — rename subcommand handler
//
// Package config provides the ze config subcommand.
package config

import (
	"fmt"
	"io"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// Exit codes for config commands.
const (
	exitOK    = 0 // Success
	exitError = 2 // Error (file not found, parse error, etc.)
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
	"import":   cmdImportWithStorage,
	"rename":   cmdRenameWithStorage,
}

// subcommandHandlers maps subcommand names to their handler functions.
// Using a map avoids both if-else chains (gocritic lint) and switch default
// (hook false positive for /config/ path).
var subcommandHandlers = map[string]func([]string) int{
	"validate":   cmdValidate,
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
	p := helpfmt.Page{
		Command: "ze config",
		Summary: "Create and manage Ze configurations",
		Usage:   []string{"ze config <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Editing", Entries: []helpfmt.HelpEntry{
				{Name: "edit [file]", Desc: "Interactive editor (default: <identity>.conf)"},
				{Name: "set <file> <path> <val>", Desc: "Set a value by path"},
			}},
			{Title: "Storage", Entries: []helpfmt.HelpEntry{
				{Name: "import [--name n] <file>...", Desc: "Import files into the database"},
				{Name: "rename <old> <new>", Desc: "Rename a config in the database"},
				{Name: "ls [prefix]", Desc: "List configs in the database"},
				{Name: "cat <key>", Desc: "Print a database entry"},
			}},
			{Title: "Inspection", Entries: []helpfmt.HelpEntry{
				{Name: "validate <file>", Desc: "Validate configuration file"},
				{Name: "dump <file>", Desc: "Parse and display config"},
				{Name: "diff <f1> <f2>", Desc: "Compare two configs"},
				{Name: "diff <N> <file>", Desc: "Compare rollback revision N against current"},
				{Name: "fmt <file>", Desc: "Format and normalize"},
			}},
			{Title: "History", Entries: []helpfmt.HelpEntry{
				{Name: "history <file>", Desc: "List rollback revisions"},
				{Name: "rollback <N> <file>", Desc: "Restore from rollback revision N"},
				{Name: "archive <name> <file>", Desc: "Archive to a named destination"},
			}},
			{Title: "Migration", Entries: []helpfmt.HelpEntry{
				{Name: "migrate <file>", Desc: "Convert old format to current"},
			}},
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "-f", Desc: "Bypass database, use filesystem directly"},
			}},
		},
		Examples: []string{
			"ze config edit",
			"ze config import router.conf",
			"ze config import --name production.conf /etc/ze/router.conf",
			"ze config validate router.conf",
			"ze config set router.conf bgp local as 65000",
		},
	}
	p.Write()
}
