// Design: plan/spec-appliance-1-builder.md — appliance CLI dispatch
//
// Package appliance provides the ze appliance subcommand for managing
// gokrazy-based Ze appliance images.
package appliance

import (
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
)

const (
	exitOK    = 0
	exitError = 1
)

var handlers = map[string]func([]string) int{
	"init":         cmdInit,
	"assemble":     cmdAssemble,
	"build":        cmdBuild,
	"passwd":       cmdPasswd,
	"replace-cert": cmdReplaceCert,
	"rekey":        cmdRekey,
	"clone":        cmdClone,
	"list":         cmdList,
	"show":         cmdShow,
	"run":          cmdRun,
	"unlock":       cmdUnlock,
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
	cmdPasswd      = stub
	cmdReplaceCert = stub
	cmdRekey       = stub
	cmdClone       = stub
	cmdList        = stub
	cmdShow        = stub
	cmdRun         = stub
	cmdUnlock      = stub
)

func usage() {
	p := helpfmt.Page{
		Command: "ze appliance",
		Summary: "Manage gokrazy-based Ze appliance images",
		Usage:   []string{"ze appliance [--dir <path>] <command> [args...]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "init <name>", Desc: "Create a new appliance with config and secrets"},
				{Name: "assemble <name>", Desc: "Build ZeFS database only (fast path)"},
				{Name: "build <name>", Desc: "Build full disk image (assemble + gok + ext4)"},
				{Name: "passwd <name>", Desc: "Change SSH password"},
				{Name: "replace-cert <name>", Desc: "Replace TLS certificate"},
				{Name: "rekey <name>", Desc: "Change encryption passphrase"},
				{Name: "clone <src> <dst>", Desc: "Copy config (not secrets) to new appliance"},
				{Name: "list", Desc: "List appliances"},
				{Name: "show <name>", Desc: "Show config summary and cert expiry"},
				{Name: "run <name>", Desc: "Boot in QEMU"},
				{Name: "unlock", Desc: "Start passphrase agent"},
			}},
			{Title: "Flags", Entries: []helpfmt.HelpEntry{
				{Name: "--dir <path>", Desc: "Appliance directory (default: $ZE_APPLIANCE_DIR or ~/.config/ze/appliances)"},
			}},
		},
		Examples: []string{
			"ze appliance init lab",
			"ze appliance build lab",
			"ze appliance list",
			"ze appliance show lab",
		},
	}
	p.Write()
}
