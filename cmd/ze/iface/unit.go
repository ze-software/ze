// Design: plan/spec-iface-2-manage.md — Interface unit subcommand

package iface

import (
	"fmt"
	"os"
	"strconv"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	mgr "codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// cmdUnit manages logical units (VLAN subinterfaces) on an interface.
// Returns exit code.
func cmdUnit(args []string) int {
	if len(args) < 1 {
		unitUsage()
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		unitUsage()
		return 0
	case "add":
		return cmdUnitAdd(args[1:])
	case "del":
		return cmdUnitDel(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "error: unknown unit action: %s (expected add or del)\n", args[0])
		unitUsage()
		return 1
	}
}

// cmdUnitAdd handles: unit add <name> <id>
//
// Creates a VLAN subinterface. The unit ID is the VLAN ID (consistent with
// JunOS where unit N on a VLAN-tagged interface is VLAN N). The OS interface
// is named "<parent>.<id>".
func cmdUnitAdd(args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "error: unit add requires exactly: <name> <id>\n")
		unitUsage()
		return 1
	}

	name := args[0]
	unitID, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid unit id %q: %v\n", args[1], err)
		return 1
	}
	if unitID <= 0 {
		fmt.Fprintf(os.Stderr, "error: unit id must be > 0 (unit 0 is the parent interface)\n")
		return 1
	}

	if err := mgr.CreateVLAN(name, unitID); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("added unit %d on %s\n", unitID, name)
	return 0
}

// cmdUnitDel handles: unit del <name> <id>
//
// Deletes a VLAN subinterface named "<name>.<id>".
func cmdUnitDel(args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "error: unit del requires exactly: <name> <id>\n")
		unitUsage()
		return 1
	}

	name := args[0]
	unitID, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid unit id %q: %v\n", args[1], err)
		return 1
	}
	if unitID <= 0 {
		fmt.Fprintf(os.Stderr, "error: unit id must be > 0 (use 'ze interface delete' for the parent)\n")
		return 1
	}

	// VLAN subinterface OS name is "<parent>.<unit>".
	osName := fmt.Sprintf("%s.%d", name, unitID)
	if err := mgr.DeleteInterface(osName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("deleted unit %d on %s\n", unitID, name)
	return 0
}

func unitUsage() {
	p := helpfmt.Page{
		Command: "ze interface unit",
		Summary: "Manage logical units (VLAN subinterfaces) on an interface",
		Usage:   []string{"ze interface unit <action> <args>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Actions", Entries: []helpfmt.HelpEntry{
				{Name: "add <name> <id>", Desc: "Add a VLAN unit (creates <name>.<id> subinterface)"},
				{Name: "del <name> <id>", Desc: "Delete a VLAN unit"},
			}},
		},
		Examples: []string{
			"ze interface unit add eth0 100",
			"ze interface unit del eth0 100",
		},
	}
	p.Write()
	fmt.Fprintf(os.Stderr, "\nUnit ID must be > 0. Unit 0 is the parent interface (implicit).\nThe OS interface name for unit N on parent P is \"P.N\".\n")
}
