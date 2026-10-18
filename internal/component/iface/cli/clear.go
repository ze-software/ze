// Design: docs/features/interfaces.md -- offline clear verb for counters

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	ifacepkg "github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

// cmdClear implements `ze interface clear <subject>`. Today only
// `counters` is supported -- the same subject accepted by the daemon
// RPC `ze-iface:clear-interface-counters`. Extension points (flush addresses,
// flush neighbors, ...) slot in under the same switch.
func cmdClear(args []string) int {
	if len(args) == 0 {
		clearUsage()
		return 1
	}
	switch args[0] {
	case subcmdHelp, flagHelpShort, flagHelpLong:
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

// cmdClearCounters zeros RX/TX counters. `all` clears every managed
// interface and a name clears that interface only; with no target the call
// is refused (clearCountersTarget).
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
	name, err := clearCountersTarget(fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
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

// errClearCountersNoTarget refuses `ze interface clear counters` with no
// target: it names nothing to act on, and the everything form must be typed
// (the explicit-all CLI rule, owner decision 2026-10-17).
var errClearCountersNoTarget = errors.New("clear counters needs a target: it names no interface, " +
	"and Ze never resets every interface unless you type all. " +
	"Run ze interface clear counters all to reset every interface, or ze interface clear counters <name> for one")

// errClearCountersTooMany refuses a second target word.
var errClearCountersTooMany = errors.New("clear counters takes one target, all or <name>: " +
	"run ze interface clear counters all, or ze interface clear counters <name>")

// clearCountersTarget answers the interface a `ze interface clear counters`
// call targets, where "" means every interface, as iface.ResetCounters reads
// it. Only the explicit word all yields "".
func clearCountersTarget(rest []string) (string, error) {
	switch len(rest) {
	case 0:
		return "", errClearCountersNoTarget
	case 1:
		if rest[0] == "all" {
			return "", nil
		}
		if err := validateIfaceName(rest[0]); err != nil {
			return "", err
		}
		return rest[0], nil
	}
	return "", errClearCountersTooMany
}

func clearUsage() {
	p := helpfmt.Page{
		Command:   "ze interface clear",
		ShortHelp: "Clear per-interface operational state",
		Usage:     []string{"ze interface clear <subject> all|<name>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Subjects", Entries: []helpfmt.HelpEntry{
				{Name: "counters all|<name>", Desc: "Zero RX/TX counters on every interface (all), or on one"},
			}},
		},
		Examples: []string{
			"ze interface clear counters all",
			"ze interface clear counters eth0",
		},
	}
	p.WriteErr()
}
