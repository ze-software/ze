// Design: docs/architecture/core-design.md -- Traffic control CLI entry point

// Package cli provides the ze traffic control subcommand for viewing
// tc qdisc, class, and filter state on network interfaces.
package cli

import (
	"fmt"
	"os"

	tcpkg "github.com/ze-software/ze/internal/component/traffic"
	tccmd "github.com/ze-software/ze/internal/component/traffic/cmd"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
)

// Run executes the traffic control subcommand. Returns exit code.
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

	if err := tcpkg.LoadBackend("kernel"); err != nil {
		msg := opererr.New("load kernel traffic control backend: "+err.Error(),
			"The kernel traffic control backend could not be loaded on this host.",
			"ze doctor")
		os.Stderr.WriteString("error: " + msg.Error() + "\n") //nolint:errcheck // CLI stderr output
		return 1
	}
	defer func() {
		if err := tcpkg.CloseBackend(); err != nil {
			msg := opererr.New("close kernel traffic control backend: "+err.Error(),
				"The kernel traffic control backend handles may not have been released cleanly.",
				"ze doctor")
			os.Stderr.WriteString("warning: " + msg.Error() + "\n") //nolint:errcheck // CLI stderr output
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

	fmt.Fprintf(os.Stderr, "unknown command: ze traffic control %s\n", subcmd)
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
		fmt.Fprintln(os.Stderr, "Use: ze traffic control show interface <name>")
		return 0
	}

	fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[0])
	usage()
	return 1
}

func usage() {
	p := helpfmt.Page{
		Command:   "ze traffic control",
		ShortHelp: "Traffic control visibility",
		Usage:     []string{"ze traffic control <command> [options]"},
		Sections: []helpfmt.HelpSection{{
			Title: "Commands",
			Entries: []helpfmt.HelpEntry{
				{Name: "show interface <name>", Desc: "Show traffic control state for a specific interface"},
			},
		}},
	}
	p.WriteErr()
}
