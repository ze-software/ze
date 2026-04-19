// Design: docs/features/interfaces.md -- Interface admin state + link props

package iface

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"
	ifacecmd "codeberg.org/thomas-mangin/ze/internal/component/iface/cmd"
)

// maxIfaceNameLen matches IFNAMSIZ (16 bytes including the NUL) so the
// CLI rejects over-long names with a clear message before the kernel
// returns a less specific EINVAL. Mirrors the early-validation done
// for MTU and MAC on these same handlers.
const maxIfaceNameLen = 15

// validateIfaceName enforces the IFNAMSIZ bound. Empty names are
// rejected; length > 15 is rejected with a message naming the limit.
// Deeper character-set checks are left to the netlink backend, which
// surfaces the kernel's own reject reason.
func validateIfaceName(name string) error {
	if name == "" {
		return fmt.Errorf("interface name must not be empty")
	}
	if len(name) > maxIfaceNameLen {
		return fmt.Errorf("interface name %q exceeds %d-byte limit (IFNAMSIZ)", name, maxIfaceNameLen)
	}
	return nil
}

// hasHelpFlag scans args for a help token anywhere in the slice. The
// flag package only recognizes `-h` / `--help` BEFORE the first
// positional, so `ze interface up eth0 --help` would otherwise miss
// the flag entirely. Scanning first closes the review I4 gap.
func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

// parseNameOnly runs a flag.FlagSet expecting exactly one positional
// argument (the interface name). Scans for `--help` / `-h` anywhere
// in args up front; the flag package's own ErrHelp handles the case
// where help appears before a positional. Returns name, exit code,
// and whether the caller should proceed: true = use name, false =
// exit with the returned code (help = 0, shape error = 1).
func parseNameOnly(verb string, args []string, usage func()) (string, int, bool) {
	if hasHelpFlag(args) {
		usage()
		return "", 0, false
	}
	fs := flag.NewFlagSet("ze interface "+verb, flag.ContinueOnError)
	fs.Usage = usage
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return "", 0, false
		}
		return "", 1, false
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintf(os.Stderr, "error: %s requires exactly one argument: <name>\n", verb)
		usage()
		return "", 1, false
	}
	if err := validateIfaceName(rest[0]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return "", 1, false
	}
	return rest[0], 0, true
}

// cmdUp brings an interface administratively up.
// Returns exit code.
func cmdUp(args []string) int {
	name, code, ok := parseNameOnly("up", args, func() { upDownUsage("up") })
	if !ok {
		return code
	}
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
	name, code, ok := parseNameOnly("down", args, func() { upDownUsage("down") })
	if !ok {
		return code
	}
	if err := ifacepkg.SetAdminDown(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("interface %s down\n", name)
	return 0
}

// cmdMTU sets the MTU on an interface. Validates 68..65535 before
// reaching the backend; mirrors the daemon-side handler's validation
// so both entry points reject identically. Uses hasHelpFlag + FlagSet
// so `--help` / `-h` are recognized anywhere in args.
func cmdMTU(args []string) int {
	if hasHelpFlag(args) {
		mtuUsage()
		return 0
	}
	fs := flag.NewFlagSet("ze interface mtu", flag.ContinueOnError)
	fs.Usage = mtuUsage
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprintf(os.Stderr, "error: mtu requires exactly two arguments: <name> <mtu>\n")
		mtuUsage()
		return 1
	}
	name := rest[0]
	if err := validateIfaceName(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	mtu, err := strconv.Atoi(rest[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid MTU %q: %v\n", rest[1], err)
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
// via ifacecmd.IsValidMACAddress before calling the backend. Uses
// hasHelpFlag + FlagSet so `--help` / `-h` are recognized anywhere
// in args.
func cmdMAC(args []string) int {
	if hasHelpFlag(args) {
		macUsage()
		return 0
	}
	fs := flag.NewFlagSet("ze interface mac", flag.ContinueOnError)
	fs.Usage = macUsage
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprintf(os.Stderr, "error: mac requires exactly two arguments: <name> <mac>\n")
		macUsage()
		return 1
	}
	name := rest[0]
	if err := validateIfaceName(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	mac := rest[1]
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
