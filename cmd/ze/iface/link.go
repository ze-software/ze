// Design: docs/features/interfaces.md -- Interface admin state + link props

package iface

import (
	"fmt"
	"os"
	"strconv"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"
	ifacecmd "codeberg.org/thomas-mangin/ze/internal/component/iface/cmd"
)

// cmdUp brings an interface administratively up.
// Returns exit code.
func cmdUp(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "error: up requires one argument: <name>\n")
		upDownUsage("up")
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" { //nolint:goconst // consistent pattern across cmd files
		upDownUsage("up")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "error: up requires exactly one argument: <name>\n")
		upDownUsage("up")
		return 1
	}
	name := args[0]
	if err := ifacepkg.SetAdminUp(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("interface %s up\n", name)
	return 0
}

// cmdDown brings an interface administratively down.
// Returns exit code.
func cmdDown(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "error: down requires one argument: <name>\n")
		upDownUsage("down")
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		upDownUsage("down")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "error: down requires exactly one argument: <name>\n")
		upDownUsage("down")
		return 1
	}
	name := args[0]
	if err := ifacepkg.SetAdminDown(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("interface %s down\n", name)
	return 0
}

// cmdMTU sets the MTU on an interface. Validates 68..65535 before
// reaching the backend; mirrors the daemon-side handler's validation
// so both entry points reject identically.
func cmdMTU(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		mtuUsage()
		if len(args) == 0 {
			return 1
		}
		return 0
	}
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "error: mtu requires exactly two arguments: <name> <mtu>\n")
		mtuUsage()
		return 1
	}
	name := args[0]
	mtu, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid MTU %q: %v\n", args[1], err)
		return 1
	}
	if mtu < ifacecmd.MTUMin || mtu > ifacecmd.MTUMax {
		fmt.Fprintf(os.Stderr, "error: MTU %d out of range %d..%d\n", mtu, ifacecmd.MTUMin, ifacecmd.MTUMax)
		return 1
	}
	if err := ifacepkg.SetMTU(name, mtu); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("interface %s mtu %d\n", name, mtu)
	return 0
}

// cmdMAC sets the MAC address on an interface. Validates the format
// via ifacecmd.IsValidMACAddress before calling the backend.
func cmdMAC(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		macUsage()
		if len(args) == 0 {
			return 1
		}
		return 0
	}
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "error: mac requires exactly two arguments: <name> <mac>\n")
		macUsage()
		return 1
	}
	name := args[0]
	mac := args[1]
	if !ifacecmd.IsValidMACAddress(mac) {
		fmt.Fprintf(os.Stderr, "error: invalid MAC address %q (expected xx:xx:xx:xx:xx:xx)\n", mac)
		return 1
	}
	if err := ifacepkg.SetMACAddress(name, mac); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("interface %s mac %s\n", name, mac)
	return 0
}

func upDownUsage(verb string) {
	p := helpfmt.Page{
		Command: "ze interface " + verb,
		Summary: "Bring an interface administratively " + verb,
		Usage:   []string{"ze interface " + verb + " <name>"},
		Examples: []string{
			"ze interface " + verb + " eth0",
		},
	}
	p.Write()
}

func mtuUsage() {
	p := helpfmt.Page{
		Command: "ze interface mtu",
		Summary: "Set the MTU on an interface",
		Usage:   []string{"ze interface mtu <name> <mtu>"},
		Examples: []string{
			"ze interface mtu eth0 1500",
			"ze interface mtu eth0 9000",
		},
	}
	p.Write()
}

func macUsage() {
	p := helpfmt.Page{
		Command: "ze interface mac",
		Summary: "Set the MAC address on an interface",
		Usage:   []string{"ze interface mac <name> <mac>"},
		Examples: []string{
			"ze interface mac eth0 02:00:00:00:00:01",
		},
	}
	p.Write()
}
