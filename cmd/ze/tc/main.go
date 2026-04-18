// Design: docs/architecture/core-design.md -- Traffic control CLI entry point

// Package tc provides the ze traffic-control subcommand for viewing
// tc qdisc, class, and filter state on network interfaces.
package tc

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	tcpkg "codeberg.org/thomas-mangin/ze/internal/component/traffic"
	tccmd "codeberg.org/thomas-mangin/ze/internal/component/traffic/cmd"

	// Register the tc backend so tcpkg.LoadBackend("tc") resolves.
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/traffic/netlink"
)

// Run executes the traffic-control subcommand. Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]

	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" { //nolint:goconst // consistent pattern
		usage()
		return 0
	}

	if err := tcpkg.LoadBackend("tc"); err != nil {
		fmt.Fprintf(os.Stderr, "error: load tc backend: %v\n", err)
		return 1
	}
	defer func() {
		if err := tcpkg.CloseBackend(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: close tc backend: %v\n", err)
		}
	}()

	b := tcpkg.GetBackend()
	if b == nil {
		fmt.Fprintln(os.Stderr, "error: no traffic control backend loaded")
		return 1
	}

	if subcmd == "show" {
		return cmdShow(b, args[1:])
	}

	fmt.Fprintf(os.Stderr, "unknown command: ze traffic-control %s\n", subcmd)
	usage()
	return 1
}

func cmdShow(b tcpkg.Backend, args []string) int {
	if len(args) > 0 && args[0] == "interface" && len(args) > 1 {
		ifaceName := args[1]
		qos, err := b.ListQdiscs(ifaceName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Print(tccmd.FormatQoS(qos))
		return 0
	}

	// No per-interface filter: show all would require listing all interfaces.
	// For now, require an interface name.
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "No traffic control configured.")
		fmt.Fprintln(os.Stderr, "Use: ze traffic-control show interface <name>")
		return 0
	}

	fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[0])
	usage()
	return 1
}

func usage() {
	p := helpfmt.Page{
		Command: "ze traffic-control",
		Summary: "Traffic control visibility",
		Usage:   []string{"ze traffic-control <command> [options]"},
		Sections: []helpfmt.HelpSection{{
			Title: "Commands",
			Entries: []helpfmt.HelpEntry{
				{Name: "show interface <name>", Desc: "Show tc state for a specific interface"},
			},
		}},
	}
	p.Write()
}
