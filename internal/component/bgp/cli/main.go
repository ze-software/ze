// Design: docs/architecture/core-design.md — BGP CLI commands
// Detail: cmd_plugin.go — plugin CLI simulator subcommand
//
// Package bgp provides the Ze BGP daemon subcommand.
package cli

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
)

// helpSectionOptions titles the flag section of every help page in this package.
const helpSectionOptions = "Options"

// Run executes the bgp subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	arg := args[0]

	// Check for known commands first
	switch arg {
	case bgpCmdDecode:
		return cmdDecode(args[1:])
	case bgpCmdEncode:
		return cmdEncode(args[1:])
	case bgpCmdPlugin:
		return cmdPlugin(args[1:])
	case bgpCmdHelp, "-h", "--help":
		usage()
		return 0
	}

	// Unknown command
	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	if s := suggest.Command(arg, append(bgpCommands, bgpCmdHelp)); s != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
	}
	usage()
	return 1
}

func usage() {
	p := helpfmt.Page{
		Command: "ze bgp",
		Summary: "BGP protocol tools",
		Usage:   []string{"ze bgp <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "decode <hex>", Desc: "Decode BGP message from hex to JSON"},
				{Name: "encode <route>", Desc: "Encode API route command to BGP hex"},
				{Name: bgpCmdPlugin, Desc: "Interactive plugin simulator"},
				{Name: bgpCmdHelp, Desc: "Show this help"},
			}},
		},
		Examples: []string{
			"ze bgp decode update <hex>",
			`ze bgp encode "nlri ipv4/unicast add 10.0.0.0/24"`,
		},
		SeeAlso: []string{
			"ze config validate   Validate configuration",
			"ze config            Configuration management",
			"ze plugin            Plugin system (rib, rr, gr, etc.)",
			"ze schema            Schema discovery",
			"ze version           Show version",
		},
	}
	p.WriteErr()
}
