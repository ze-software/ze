// Design: plan/spec-iface-2-manage.md — Interface unit subcommand

package iface

import (
	"fmt"
	"os"
	"strconv"

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

// cmdUnitAdd handles: unit add <name> <id> [vlan-id <vid>]
//
// Creates a VLAN subinterface. The VLAN ID defaults to the unit ID
// unless overridden with "vlan-id <vid>".
func cmdUnitAdd(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "error: unit add requires at least: <name> <id>\n")
		unitUsage()
		return 1
	}

	name := args[0]
	unitID, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid unit id %q: %v\n", args[1], err)
		return 1
	}

	// Default VLAN ID to unit ID.
	vlanID := unitID
	remaining := args[2:]

	// Parse optional "vlan-id <vid>".
	if len(remaining) >= 2 && remaining[0] == "vlan-id" {
		vid, err := strconv.Atoi(remaining[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid vlan-id %q: %v\n", remaining[1], err)
			return 1
		}
		vlanID = vid
		remaining = remaining[2:]
	}

	if len(remaining) > 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected arguments after unit add: %v\n", remaining)
		unitUsage()
		return 1
	}

	if err := mgr.CreateVLAN(name, vlanID); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("added unit %d on %s (vlan-id %d)\n", unitID, name, vlanID)
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
	fmt.Fprintf(os.Stderr, `Usage: ze interface unit <action> <args>

Manage logical units (VLAN subinterfaces) on an interface.

Actions:
  add <name> <id> [vlan-id <vid>]    Add a VLAN subinterface
  del <name> <id>                     Delete a VLAN subinterface

The VLAN ID defaults to the unit ID unless overridden with vlan-id.
The OS interface name for unit N on parent P is "P.N".

Examples:
  ze interface unit add eth0 100
  ze interface unit add eth0 100 vlan-id 200
  ze interface unit del eth0 100
`)
}
