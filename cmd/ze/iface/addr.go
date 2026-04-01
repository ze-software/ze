// Design: plan/spec-iface-2-manage.md — Interface addr subcommand

package iface

import (
	"fmt"
	"os"
	"strconv"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	mgr "codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// cmdAddr manages IP addresses on interface units.
// Returns exit code.
func cmdAddr(args []string) int {
	if len(args) < 1 {
		addrUsage()
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		addrUsage()
		return 0
	case "add":
		return cmdAddrAdd(args[1:])
	case "del":
		return cmdAddrDel(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "error: unknown addr action: %s (expected add or del)\n", args[0])
		addrUsage()
		return 1
	}
}

// cmdAddrAdd handles: addr add <name> unit <id> <cidr>.
func cmdAddrAdd(args []string) int {
	ifaceName, cidr, ok := parseAddrArgs("add", args)
	if !ok {
		return 1
	}

	if err := mgr.AddAddress(ifaceName, cidr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("added address %s on %s\n", cidr, ifaceName)
	return 0
}

// cmdAddrDel handles: addr del <name> unit <id> <cidr>.
func cmdAddrDel(args []string) int {
	ifaceName, cidr, ok := parseAddrArgs("del", args)
	if !ok {
		return 1
	}

	if err := mgr.RemoveAddress(ifaceName, cidr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("removed address %s from %s\n", cidr, ifaceName)
	return 0
}

// parseAddrArgs parses the common argument pattern: <name> unit <id> <cidr>
// Returns the resolved OS interface name, CIDR, and true on success.
// On failure, prints an error to stderr and returns false.
func parseAddrArgs(action string, args []string) (string, string, bool) {
	// Expected: <name> unit <id> <cidr>
	if len(args) != 4 {
		fmt.Fprintf(os.Stderr, "error: addr %s requires: <name> unit <id> <cidr>\n", action)
		addrUsage()
		return "", "", false
	}

	name := args[0]
	if args[1] != "unit" {
		fmt.Fprintf(os.Stderr, "error: expected 'unit' keyword, got %q\n", args[1])
		addrUsage()
		return "", "", false
	}

	unitID, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid unit id %q: %v\n", args[2], err)
		return "", "", false
	}
	if unitID < 0 {
		fmt.Fprintf(os.Stderr, "error: unit id must be >= 0\n")
		return "", "", false
	}

	cidr := args[3]

	// Resolve OS interface name: unit 0 is the parent, unit N is "<name>.<N>".
	ifaceName := name
	if unitID != 0 {
		ifaceName = fmt.Sprintf("%s.%d", name, unitID)
	}

	return ifaceName, cidr, true
}

func addrUsage() {
	p := helpfmt.Page{
		Command: "ze interface addr",
		Summary: "Manage IP addresses on interface units",
		Usage:   []string{"ze interface addr <action> <name> unit <id> <cidr>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Actions", Entries: []helpfmt.HelpEntry{
				{Name: "add <name> unit <id> <cidr>", Desc: "Add an IP address to a unit"},
				{Name: "del <name> unit <id> <cidr>", Desc: "Remove an IP address from a unit"},
			}},
		},
		Examples: []string{
			"ze interface addr add eth0 unit 0 10.0.0.1/24",
			"ze interface addr add eth0 unit 100 192.168.1.1/24",
			"ze interface addr del eth0 unit 0 10.0.0.1/24",
		},
	}
	p.Write()
	fmt.Fprintf(os.Stderr, "\nUnit 0 refers to the parent interface itself.\nUnit N (N > 0) refers to VLAN subinterface \"<name>.<N>\".\n")
}
