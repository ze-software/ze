// Design: plan/spec-iface-2-manage.md — Interface CLI commands
//
// Package iface provides the ze interface subcommand for managing
// OS network interfaces (dummy, veth, VLAN units, addresses).
package iface

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"

	// Register the netlink backend so ifacepkg.LoadBackend("netlink")
	// below resolves. Without this blank import, every subcommand that
	// calls into the backend (show, create, delete, unit, addr, migrate)
	// fails with "iface: no backend loaded".
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/ifacenetlink"
)

// Run executes the interface subcommand with the given arguments.
// Returns exit code.
//
// Help is handled before the backend is loaded so `ze interface help`
// works on platforms where the netlink backend stub returns errors
// (non-Linux). Every other subcommand requires the backend; LoadBackend
// is called once here and released via defer so individual command
// handlers do not repeat the boilerplate.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" { //nolint:goconst // consistent pattern across cmd files
		usage()
		return 0
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
	case "show":
		return cmdShow(subArgs)
	case "scan":
		return cmdScan(subArgs)
	case "create":
		return cmdCreate(subArgs)
	case "delete":
		return cmdDelete(subArgs)
	case "unit":
		return cmdUnit(subArgs)
	case "addr":
		return cmdAddr(subArgs)
	case "migrate":
		return cmdMigrate(subArgs)
	}

	fmt.Fprintf(os.Stderr, "error: unknown interface subcommand: %s\n", subcmd)
	if s := suggest.Command(subcmd, []string{
		"show", "scan", "create", "delete", "unit", "addr", "migrate", "help",
	}); s != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
	}
	usage()
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
				{Name: "delete <name>", Desc: "Delete an interface"},
				{Name: "unit add <name> <id> [vlan-id <vid>]", Desc: "Add a logical unit (VLAN subinterface)"},
				{Name: "unit del <name> <id>", Desc: "Delete a logical unit"},
				{Name: "addr add <name> unit <id> <cidr>", Desc: "Add an IP address to a unit"},
				{Name: "addr del <name> unit <id> <cidr>", Desc: "Remove an IP address from a unit"},
				{Name: "migrate --from .. --to .. --address", Desc: "Make-before-break IP migration"},
				{Name: "help", Desc: "Show this help"},
			}},
		},
		Examples: []string{
			"ze interface show",
			"ze interface show eth0",
			"ze interface create dummy lo1",
			"ze interface create veth ze0 ze1",
			"ze interface delete lo1",
			"ze interface unit add eth0 100 vlan-id 100",
			"ze interface unit del eth0 100",
			"ze interface addr add eth0 unit 0 10.0.0.1/24",
			"ze interface addr del eth0 unit 100 192.168.1.1/24",
		},
	}
	p.Write()
}
