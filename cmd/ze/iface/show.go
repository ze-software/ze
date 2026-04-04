// Design: plan/learned/491-iface-2-manage.md — Interface show subcommand

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

// cmdShow lists interfaces or shows details for a specific one.
// Returns exit code.
func cmdShow(args []string) int {
	fs := flag.NewFlagSet("ze interface show", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze interface show",
			Summary: "List all interfaces or show details for a specific interface",
			Usage:   []string{"ze interface show [options] [name]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--json", Desc: "Output in JSON format"},
				}},
			},
			Examples: []string{
				"ze interface show",
				"ze interface show eth0",
				"ze interface show --json",
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

	switch len(remaining) {
	case 0:
		return showAll(*jsonOutput)
	case 1:
		return showOne(remaining[0], *jsonOutput)
	default:
		fmt.Fprintf(os.Stderr, "error: too many arguments\n")
		fs.Usage()
		return 1
	}
}

// showAll lists all interfaces.
func showAll(jsonOut bool) int {
	ifaces, err := ifacepkg.ListInterfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if jsonOut {
		return encodeJSON(ifaces)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	printRow(w, "NAME", "INDEX", "TYPE", "STATE", "MTU", "MAC", "ADDRESSES")
	printRow(w, "----", "-----", "----", "-----", "---", "---", "---------")

	for i := range ifaces {
		addrs := formatAddrs(ifaces[i].Addresses)
		mac := ifaces[i].MAC
		if mac == "" {
			mac = "-"
		}
		typ := ifaces[i].Type
		if typ == "" {
			typ = "-"
		}
		printRow(w, ifaces[i].Name, fmt.Sprint(ifaces[i].Index), typ, ifaces[i].State,
			fmt.Sprint(ifaces[i].MTU), mac, addrs)
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// showOne shows details for a specific interface.
func showOne(name string, jsonOut bool) int {
	info, err := ifacepkg.GetInterface(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if jsonOut {
		return encodeJSON(info)
	}

	fmt.Printf("Name:       %s\n", info.Name)
	fmt.Printf("Index:      %d\n", info.Index)
	if info.Type != "" {
		fmt.Printf("Type:       %s\n", info.Type)
	}
	fmt.Printf("State:      %s\n", info.State)
	fmt.Printf("MTU:        %d\n", info.MTU)
	if info.MAC != "" {
		fmt.Printf("MAC:        %s\n", info.MAC)
	}
	if info.VlanID != 0 {
		fmt.Printf("VLAN ID:    %d\n", info.VlanID)
	}

	if len(info.Addresses) > 0 {
		fmt.Println("Addresses:")
		for _, a := range info.Addresses {
			fmt.Printf("  %s/%d (%s)\n", a.Address, a.PrefixLength, a.Family)
		}
	}

	if info.Stats != nil {
		fmt.Println("Statistics:")
		fmt.Printf("  RX: %d bytes, %d packets, %d errors, %d dropped\n",
			info.Stats.RxBytes, info.Stats.RxPackets, info.Stats.RxErrors, info.Stats.RxDropped)
		fmt.Printf("  TX: %d bytes, %d packets, %d errors, %d dropped\n",
			info.Stats.TxBytes, info.Stats.TxPackets, info.Stats.TxErrors, info.Stats.TxDropped)
	}

	return 0
}

// formatAddrs returns a compact string of addresses.
func formatAddrs(addrs []ifacepkg.AddrInfo) string {
	if len(addrs) == 0 {
		return "-"
	}
	parts := make([]string, len(addrs))
	for i, a := range addrs {
		parts[i] = fmt.Sprintf("%s/%d", a.Address, a.PrefixLength)
	}
	return strings.Join(parts, ", ")
}

func encodeJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding JSON: %v\n", err)
		return 1
	}
	return 0
}

func printRow(w *tabwriter.Writer, cols ...string) {
	if _, err := fmt.Fprintln(w, strings.Join(cols, "\t")); err != nil {
		return
	}
}
