// Design: plan/spec-iface-2-manage.md — Interface create subcommand

package iface

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	mgr "codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// cmdCreate creates a new interface: dummy or veth pair.
// Returns exit code.
func cmdCreate(args []string) int {
	if len(args) < 1 {
		createUsage()
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		createUsage()
		return 0
	case "dummy": //nolint:goconst // CLI dispatch strings, constants in internal/component/iface
		return cmdCreateDummy(args[1:])
	case "veth": //nolint:goconst // CLI dispatch strings, constants in internal/component/iface
		return cmdCreateVeth(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "error: unknown interface type: %s (expected dummy or veth)\n", args[0])
		createUsage()
		return 1
	}
}

func cmdCreateDummy(args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "error: create dummy requires exactly one argument: <name>\n")
		createUsage()
		return 1
	}

	name := args[0]
	if err := mgr.CreateDummy(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("created dummy interface %s\n", name)
	return 0
}

func cmdCreateVeth(args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "error: create veth requires exactly two arguments: <name> <peer>\n")
		createUsage()
		return 1
	}

	name := args[0]
	peer := args[1]
	if err := mgr.CreateVeth(name, peer); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("created veth pair %s <-> %s\n", name, peer)
	return 0
}

func createUsage() {
	p := helpfmt.Page{
		Command: "ze interface create",
		Summary: "Create a new network interface",
		Usage:   []string{"ze interface create <type> <args>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Types", Entries: []helpfmt.HelpEntry{
				{Name: "dummy <name>", Desc: "Create a dummy interface and bring it up"},
				{Name: "veth <name> <peer>", Desc: "Create a veth pair and bring both ends up"},
			}},
		},
		Examples: []string{
			"ze interface create dummy lo1",
			"ze interface create veth ze0 ze1",
		},
	}
	p.Write()
}
