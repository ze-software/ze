// Design: docs/architecture/iface/management.md -- Interface CLI commands
//
// Package iface provides the ze interface subcommand for managing
// OS network interfaces (dummy, veth, VLAN units, addresses).
package cli

import (
	"fmt"
	"os"
	"slices"

	ifacepkg "github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"

	// Register the netlink backend so ifacepkg.LoadBackend("netlink")
	// below resolves. Without this blank import, every subcommand that
	// calls into the backend (show, create, delete, unit, addr, migrate)
	// fails with "iface: no backend loaded".
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
)

// Subcommand names. ifaceCommands below and the Run dispatch each list the
// whole set, and both read from here so neither can grow a name the other
// does not have.
const (
	subcmdShow      = "show"
	subcmdScan      = "scan"
	subcmdCreate    = "create"
	subcmdDelete    = "delete"
	subcmdUnit      = "unit"
	subcmdAddr      = "addr"
	subcmdMigrate   = "migrate"
	subcmdUp        = "up"
	subcmdDown      = "down"
	subcmdMTU       = "mtu"
	subcmdMAC       = "mac"
	subcmdNeighbors = "neighbors"
	subcmdRoutes    = "routes"
	subcmdClear     = "clear"
	subcmdHelp      = "help"
)

// Help tokens every interface subcommand accepts.
const (
	flagHelpShort = "-h"
	flagHelpLong  = "--help"
)

// Interface type names an operator types after `create` or `--create`. They
// spell the same words as the zeType* constants of internal/component/iface,
// which are unexported and so cannot be shared with this package.
const (
	ifaceTypeDummy  = "dummy"
	ifaceTypeVeth   = "veth"
	ifaceTypeBridge = "bridge"
)

// Help page vocabulary shared by every subcommand that takes --json.
const (
	helpSectionOptions = "Options"
	flagJSONLong       = "--json"
	helpDescJSON       = "Output in JSON format"
)

// Full command paths. Each is spelled in this page's Examples and again in the
// subcommand's own flag set and help page.
const (
	cmdPathShow      = "ze interface show"
	cmdPathNeighbors = "ze interface neighbors"
	cmdPathRoutes    = "ze interface routes"
)

// ifaceCommands lists the user-facing subcommand names, kept in sync
// with the switch cases in Run below. Used by the known-subcommand gate,
// suggestion hints, and Meta.Subs in register.go.
var ifaceCommands = []string{
	subcmdShow, subcmdScan, subcmdCreate, subcmdDelete, subcmdUnit, subcmdAddr, subcmdMigrate,
	subcmdUp, subcmdDown, subcmdMTU, subcmdMAC, subcmdNeighbors, subcmdRoutes, subcmdClear,
}

// Run executes the interface subcommand with the given arguments.
// Returns exit code.
//
// Help and unknown-subcommand rejection run BEFORE the backend is loaded.
// Loading+closing the backend for a bogus subcommand perturbs global
// backend state in ways that race with unit tests calling individual
// cmdXxx handlers in parallel (they depend on the backend loaded in
// init()). Gating the backend load on a known-subcommand check keeps
// the perturbation window closed to the actual command dispatch path.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	if subcmd == subcmdHelp || subcmd == flagHelpShort || subcmd == flagHelpLong {
		usage()
		return 0
	}

	// Validate subcommand BEFORE touching the backend. Otherwise a bogus
	// subcommand triggers LoadBackend + defer CloseBackend, mutating
	// package-global state that parallel unit tests rely on.
	if !slices.Contains(ifaceCommands, subcmd) {
		fmt.Fprintf(os.Stderr, "error: unknown interface subcommand: %s\n", subcmd)
		if s := suggest.Command(subcmd, append(ifaceCommands, subcmdHelp)); s != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
		}
		usage()
		return 1
	}

	if err := ifacepkg.LoadBackend("netlink"); err != nil {
		fmt.Fprintf(os.Stderr, "error: load netlink backend: %v\n", err)
		return 1
	}
	defer func() {
		if err := ifacepkg.CloseBackend(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: close netlink backend: %v\n", err)
		}
	}()

	switch subcmd {
	case subcmdShow:
		return cmdShow(subArgs)
	case subcmdScan:
		return cmdScan(subArgs)
	case subcmdCreate:
		return cmdCreate(subArgs)
	case subcmdDelete:
		return cmdDelete(subArgs)
	case subcmdUnit:
		return cmdUnit(subArgs)
	case subcmdAddr:
		return cmdAddr(subArgs)
	case subcmdMigrate:
		return cmdMigrate(subArgs)
	case subcmdUp:
		return cmdUp(subArgs)
	case subcmdDown:
		return cmdDown(subArgs)
	case subcmdMTU:
		return cmdMTU(subArgs)
	case subcmdMAC:
		return cmdMAC(subArgs)
	case subcmdNeighbors:
		return cmdNeighbors(subArgs)
	case subcmdRoutes:
		return cmdRoutes(subArgs)
	case subcmdClear:
		return cmdClear(subArgs)
	}
	// Unreachable: known-subcommand gate above.
	return 1
}

func usage() {
	p := helpfmt.Page{
		Command: "ze interface",
		Summary: "manage OS network interfaces",
		Usage:   []string{"ze interface <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "show [name]", Desc: "List interfaces or show one"},
				{Name: "scan [--config|--json|--yaml]", Desc: "Scan OS for interfaces and classify by Ze type"},
				{Name: "create dummy <name>", Desc: "Create a dummy interface"},
				{Name: "create veth <name> <peer>", Desc: "Create a veth pair"},
				{Name: "create bridge <name>", Desc: "Create a Linux bridge"},
				{Name: "delete <name>", Desc: "Delete an interface"},
				{Name: "up <name>", Desc: "Bring an interface administratively up"},
				{Name: "down <name>", Desc: "Bring an interface administratively down"},
				{Name: "mtu <name> <mtu>", Desc: "Set the MTU on an interface (68..65535)"},
				{Name: "mac <name> <mac>", Desc: "Set the MAC address on an interface"},
				{Name: "neighbors [ipv4|ipv6]", Desc: "List kernel neighbor table (ARP + ND)"},
				{Name: "routes [cidr] [--limit N]", Desc: "List kernel routing table entries"},
				{Name: "clear counters [name]", Desc: "Clear RX/TX counters (all or one)"},
				{Name: "unit add <name> <id> [vlan-id <vid>]", Desc: "Add a logical unit (VLAN subinterface)"},
				{Name: "unit del <name> <id>", Desc: "Delete a logical unit"},
				{Name: "addr add <name> unit <id> <cidr>", Desc: "Add an IP address to a unit"},
				{Name: "addr del <name> unit <id> <cidr>", Desc: "Remove an IP address from a unit"},
				{Name: "migrate from .. to .. address ..", Desc: "Make-before-break IP migration"},
				{Name: subcmdHelp, Desc: "Show this help"},
			}},
		},
		Examples: []string{
			cmdPathShow,
			"ze interface show eth0",
			"ze interface create dummy lo1",
			"ze interface create veth ze0 ze1",
			"ze interface create bridge br0",
			"ze interface delete lo1",
			"ze interface up eth0",
			"ze interface down eth0",
			"ze interface mtu eth0 9000",
			"ze interface mac eth0 02:00:00:00:00:01",
			cmdPathNeighbors,
			"ze interface neighbors ipv6",
			cmdPathRoutes,
			"ze interface routes 10.0.0.0/8",
			"ze interface clear counters",
			"ze interface clear counters eth0",
			"ze interface unit add eth0 100 vlan-id 100",
			"ze interface unit del eth0 100",
			"ze interface addr add eth0 unit 0 10.0.0.1/24",
			"ze interface addr del eth0 unit 100 192.168.1.1/24",
		},
	}
	p.WriteErr()
}
