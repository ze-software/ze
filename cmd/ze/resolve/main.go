// Design: docs/architecture/resolve.md -- CLI for resolution services
//
// Package resolve provides the ze resolve subcommand for querying DNS,
// Team Cymru, PeeringDB, and IRR resolution services.
package resolve

import (
	"context"
	"fmt"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
)

const (
	exitOK    = 0
	exitError = 1

	defaultTimeout = 30 * time.Second
)

var subcommands = []string{"dns", "cymru", "peeringdb", "irr"}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

// Run executes the resolve subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return exitError
	}

	subcmd := args[0]
	subArgs := args[1:]

	if isHelp(subcmd) {
		usage()
		return exitOK
	}

	switch subcmd {
	case "dns":
		return cmdDNS(subArgs)
	case "cymru":
		return cmdCymru(subArgs)
	case "peeringdb":
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		return cmdPeeringDB(ctx, subArgs)
	case "irr":
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		return cmdIRR(ctx, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "unknown resolve subcommand: %s\n", subcmd)
		if s := suggest.Command(subcmd, subcommands); s != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
		}
		usage()
		return exitError
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze resolve",
		Summary: "query resolution services",
		Usage:   []string{"ze resolve <service> <operation> [args]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Services", Entries: []helpfmt.HelpEntry{
				{Name: "dns <op> <name>", Desc: "DNS record queries (a, aaaa, txt, ptr)"},
				{Name: "cymru asn-name <asn>", Desc: "Team Cymru ASN-to-name resolution"},
				{Name: "peeringdb <op> <asn>", Desc: "PeeringDB prefix count and AS-SET queries"},
				{Name: "irr <op> <name|asn>", Desc: "IRR AS-SET expansion and prefix lookup"},
			}},
		},
		Examples: []string{
			"ze resolve dns a example.com",
			"ze resolve dns ptr 8.8.8.8",
			"ze resolve cymru asn-name 13335",
			"ze resolve peeringdb max-prefix 13335",
			"ze resolve irr as-set AS-CLOUDFLARE",
		},
	}
	p.Write()
}
