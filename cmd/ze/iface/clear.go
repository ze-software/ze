// Design: docs/features/interfaces.md -- offline clear verb for counters

package iface

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// cmdClear implements `ze interface clear <subject>`. Today only
// `counters` is supported -- the same subject accepted by the daemon
// RPC `ze-clear:interface-counters`. Extension points (flush addresses,
// flush neighbors, ...) slot in under the same switch.
func cmdClear(args []string) int {
	if len(args) == 0 {
		clearUsage()
		return 1
	}
	switch args[0] {
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		clearUsage()
		return 0
	case "counters":
		return cmdClearCounters(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "error: unknown clear subject: %s (expected: counters)\n", args[0])
		clearUsage()
		return 1
	}
}

// cmdClearCounters zeros RX/TX counters. With no argument, clears every
// managed interface; with one argument, clears that interface only.
// Backends that cannot reset kernel counters fall back to a per-iface
// baseline inside iface.ResetCounters -- from the operator's view the
// counters read zero immediately on the next `show interface`.
//
// Uses hasHelpFlag + FlagSet so `--help` / `-h` are recognized
// anywhere in args (including after a positional interface name).
// Interface names are length-checked up front against IFNAMSIZ.
func cmdClearCounters(args []string) int {
	if hasHelpFlag(args) {
		clearUsage()
		return 0
	}
	fs := flag.NewFlagSet("ze interface clear counters", flag.ContinueOnError)
	fs.Usage = clearUsage
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	rest := fs.Args()
	name := ""
	switch len(rest) {
	case 0:
		// clear all
	case 1:
		if err := validateIfaceName(rest[0]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		name = rest[0]
	default:
		fmt.Fprintf(os.Stderr, "error: clear counters accepts at most one argument: [<name>]\n")
		clearUsage()
		return 1
	}
	if err := ifacepkg.ResetCounters(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	scope := name
	if scope == "" {
		scope = "all"
	}
	fmt.Printf("cleared counters: %s\n", scope)
	return 0
}

func clearUsage() {
	p := helpfmt.Page{
		Command: "ze interface clear",
		Summary: "Clear per-interface operational state",
		Usage:   []string{"ze interface clear <subject> [<name>]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Subjects", Entries: []helpfmt.HelpEntry{
				{Name: "counters [<name>]", Desc: "Zero RX/TX counters (all interfaces, or one)"},
			}},
		},
		Examples: []string{
			"ze interface clear counters",
			"ze interface clear counters eth0",
		},
	}
	p.Write()
}
