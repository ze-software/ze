// Design: docs/architecture/appliance/builder.md -- appliance CLI dispatch
//
// Package appliance is the self-contained command provider for ze appliance.
// It owns the entire appliance command surface and registers it through the
// importable offline command registry. It has no daemon, bus, or engine
// presence; all commands are offline shell commands for build-host tooling.
package appliance

import (
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
)

const (
	exitOK    = 0
	exitError = 1
)

type applianceCommandInfo struct {
	Key     string
	Usage   string
	Desc    string
	Handler func([]string) int
}

func applianceCommands() []applianceCommandInfo {
	return []applianceCommandInfo{
		{Key: "init", Usage: "init <name>", Desc: "Create a new appliance with config and secrets", Handler: cmdInit},
		{Key: "init", Usage: "init --batch <manifest>", Desc: "Batch init from JSON manifest", Handler: cmdInit},
		{Key: "assemble", Usage: "assemble <name>", Desc: "Build ZeFS database only (fast path)", Handler: cmdAssemble},
		{Key: "build", Usage: "build <name>", Desc: "Build full disk image (assemble + gok + ext4)", Handler: cmdBuild},
		{Key: "iso", Usage: "iso <name>", Desc: "Build bootable installer ISO from an existing appliance image", Handler: cmdIso},
		{Key: "push", Usage: "push <name>", Desc: "Push image to device via OTA update", Handler: cmdPush},
		{Key: "push", Usage: "push --all", Desc: "Push to all appliances with device.address", Handler: cmdPush},
		{Key: "config", Usage: "config <name> --merged", Desc: "Show effective config (base + overlay)", Handler: cmdConfig},
		{Key: "config-push", Usage: "config-push <name>", Desc: "Push config to running device via SSH", Handler: cmdConfigPush},
		{Key: "config-push", Usage: "config-push --all", Desc: "Push config to all addressed devices", Handler: cmdConfigPush},
		{Key: "passwd", Usage: "passwd <name>", Desc: "Change SSH password", Handler: cmdPasswd},
		{Key: "replace-cert", Usage: "replace-cert <name>", Desc: "Replace TLS certificate", Handler: cmdReplaceCert},
		{Key: "rekey", Usage: "rekey <name>", Desc: "Change encryption passphrase", Handler: cmdRekey},
		{Key: "clone", Usage: "clone <src> <dst>", Desc: "Copy config (not secrets) to new appliance", Handler: cmdClone},
		{Key: "list", Usage: "list", Desc: "List appliances", Handler: cmdList},
		{Key: "show", Usage: "show <name>", Desc: "Show config summary and cert expiry", Handler: cmdShow},
		{Key: "run", Usage: "run <name>", Desc: "Boot in QEMU", Handler: cmdRun},
		{Key: "unlock", Usage: "unlock", Desc: "Start passphrase agent", Handler: cmdUnlock},
		{Key: "export", Usage: "export <name>", Desc: "Export appliance to encrypted archive", Handler: cmdExport},
		{Key: "export", Usage: "export --all", Desc: "Export all appliances to single encrypted archive", Handler: cmdExport},
		{Key: "import", Usage: "import <archive>", Desc: "Import appliance from encrypted archive", Handler: cmdImport},
		{Key: "kernel", Usage: "kernel [options] [<name>]", Desc: "Download or build the installer kernel", Handler: cmdKernel},
		{Key: "initrd", Usage: "initrd", Desc: "Download or build the installer initrd", Handler: cmdInitrd},
	}
}

// dispatchTable maps each subcommand to its handler. It is built at call time,
// NOT as a package-level var: the cmd*.go files install their real handlers
// into the cmd* vars from func init(), and a map literal evaluated during
// package-variable initialization would capture the stub values before those
// init functions run (Go runs var initializers before init funcs), leaving
// every subcommand permanently stubbed. Building the map inside Run() reads the
// vars after all init funcs have completed.
func dispatchTable() map[string]func([]string) int {
	handlers := make(map[string]func([]string) int)
	for _, cmd := range applianceCommands() {
		handlers[cmd.Key] = cmd.Handler
	}
	return handlers
}

func applianceHelpEntries() []helpfmt.HelpEntry {
	commands := applianceCommands()
	entries := make([]helpfmt.HelpEntry, 0, len(commands))
	for _, cmd := range commands {
		entries = append(entries, helpfmt.HelpEntry{Name: cmd.Usage, Desc: cmd.Desc})
	}
	return entries
}

// baseDir holds the resolved appliance directory for the current invocation.
// Set once in Run() before dispatching. Not safe for concurrent use (CLI is
// single-threaded; tests that call Run() must not use t.Parallel).
var baseDir string

func Run(args []string) int {
	var flagDir string
	args, flagDir = extractDirFlag(args)
	baseDir = ResolveDir(flagDir)

	if len(args) == 0 {
		usage()
		return exitError
	}

	subcmd := args[0]
	subArgs := args[1:]

	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" {
		usage()
		return exitOK
	}

	handlers := dispatchTable()
	if handler, ok := handlers[subcmd]; ok {
		return handler(subArgs)
	}

	fmt.Fprintf(os.Stderr, "unknown appliance subcommand: %s\n", subcmd)
	candidates := make([]string, 0, len(handlers))
	for k := range handlers {
		candidates = append(candidates, k)
	}
	if s := suggest.Command(subcmd, candidates); s != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
	}
	usage()
	return exitError
}

func extractDirFlag(args []string) ([]string, string) {
	var dir string
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--dir" && i+1 < len(args):
			dir = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--dir="):
			dir = args[i][len("--dir="):]
		default:
			remaining = append(remaining, args[i])
		}
	}
	return remaining, dir
}

func getBaseDir() string { return baseDir }

func stub(_ []string) int {
	fmt.Fprintf(os.Stderr, "error: not yet implemented (dir=%s)\n", getBaseDir())
	return exitError
}

var (
	cmdInit        = stub
	cmdAssemble    = stub
	cmdBuild       = stub
	cmdIso         = stub
	cmdPasswd      = stub
	cmdReplaceCert = stub
	cmdRekey       = stub
	cmdClone       = stub
	cmdList        = stub
	cmdShow        = stub
	cmdRun         = stub
	cmdUnlock      = stub
	cmdExport      = stub
	cmdImport      = stub
	cmdPush        = stub
	cmdConfig      = stub
	cmdConfigPush  = stub
	cmdKernel      = stub
	cmdInitrd      = stub
)

func usage() {
	p := helpfmt.Page{
		Command: "ze appliance",
		Summary: "Manage gokrazy-based Ze appliance images",
		Usage:   []string{"ze appliance [--dir <path>] <command> [args...]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: applianceHelpEntries()},
			{Title: "Flags", Entries: []helpfmt.HelpEntry{
				{Name: "--dir <path>", Desc: "Appliance directory (default: $ZE_APPLIANCE_DIR or ~/.config/ze/appliances)"},
			}},
		},
		Examples: []string{
			"ze appliance init lab",
			"ze appliance build lab",
			"ze appliance iso lab",
			"ze appliance list",
			"ze appliance show lab",
		},
	}
	p.WriteErr()
}
