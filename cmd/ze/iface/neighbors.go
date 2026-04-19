// Design: docs/features/interfaces.md -- neighbor table CLI entry point

package iface

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// cmdNeighbors lists the kernel neighbor table (IPv4 ARP + IPv6 ND).
// Optional positional argument "ipv4" or "ipv6" narrows the dump.
// Returns exit code.
func cmdNeighbors(args []string) int {
	fs := flag.NewFlagSet("ze interface neighbors", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze interface neighbors",
			Summary: "List the kernel neighbor table (IPv4 ARP + IPv6 ND)",
			Usage:   []string{"ze interface neighbors [ipv4|ipv6] [--json]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--json", Desc: "Output in JSON format"},
				}},
			},
			Examples: []string{
				"ze interface neighbors",
				"ze interface neighbors ipv4",
				"ze interface neighbors ipv6 --json",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	remaining := fs.Args()
	family := ifacepkg.NeighborFamilyAny
	switch len(remaining) {
	case 0:
		// default: both families
	case 1:
		switch strings.ToLower(remaining[0]) {
		case "ipv4":
			family = ifacepkg.NeighborFamilyIPv4
		case "ipv6":
			family = ifacepkg.NeighborFamilyIPv6
		case "any", "all":
			family = ifacepkg.NeighborFamilyAny
		default:
			fmt.Fprintf(os.Stderr, "error: unknown family %q (expected ipv4, ipv6, or any)\n", remaining[0])
			fs.Usage()
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "error: too many arguments\n")
		fs.Usage()
		return 1
	}

	neighbors, err := ifacepkg.ListNeighbors(family)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(neighbors); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	printRow(w, "ADDRESS", "MAC", "DEVICE", "FAMILY", "STATE")
	printRow(w, "-------", "---", "------", "------", "-----")
	for _, n := range neighbors {
		mac := n.MAC
		if mac == "" {
			mac = "-"
		}
		printRow(w, n.Address, mac, n.Device, n.Family, n.State)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
