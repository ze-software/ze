// Design: docs/architecture/iface/management.md -- Interface show subcommand

package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	ifacepkg "github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
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
		p.WriteErr()
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

	fmt.Fprint(os.Stdout, formatInterfaceDetail(info)) //nolint:errcheck // CLI output

	if info.Type == "xfrm" {
		showXFRMDetail(name)
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

// formatInterfaceDetail renders the human-readable detail block for an
// interface (everything except the netlink-backed XFRM and statistics
// sections). Split out from showOne so the rendering -- including the OS-name
// and permanent-MAC lines -- is unit-testable without a netlink backend.
func formatInterfaceDetail(info *ifacepkg.InterfaceInfo) string {
	var b textbuf.Buffer
	b.Str("Name:       ").Str(info.Name).Byte('\n')
	if info.OsName != "" {
		b.Str("OS Name:    ").Str(info.OsName).Byte('\n')
	}
	b.Str("Index:      ").Int(int64(info.Index)).Byte('\n')
	if info.Type != "" {
		b.Str("Type:       ").Str(info.Type).Byte('\n')
	}
	b.Str("State:      ").Str(info.State).Byte('\n')
	b.Str("MTU:        ").Int(int64(info.MTU)).Byte('\n')
	if info.MAC != "" {
		b.Str("MAC:        ").Str(info.MAC).Byte('\n')
	}
	if info.PermanentMAC != "" {
		b.Str("Perm MAC:   ").Str(info.PermanentMAC).Byte('\n')
	}
	if info.VlanID != 0 {
		b.Str("VLAN ID:    ").Int(int64(info.VlanID)).Byte('\n')
	}
	if len(info.Addresses) > 0 {
		b.Str("Addresses:\n")
		for _, a := range info.Addresses {
			b.Str("  ").Str(a.Address).Byte('/').Int(int64(a.PrefixLength)).Str(" (").Str(a.Family).Str(")\n")
		}
	}
	return b.String()
}

func showXFRMDetail(name string) {
	xi, err := ifacepkg.GetXFRMInfo(name)
	if err != nil {
		return
	}
	if _, err = fmt.Fprintf(os.Stdout, "XFRM if-id: %d\n", xi.IfID); err != nil { //nolint:errcheck // output
		return
	}
	if xi.ParentDev != "" {
		if _, err = fmt.Fprintf(os.Stdout, "XFRM dev:   %s\n", xi.ParentDev); err != nil { //nolint:errcheck // output
			return
		}
	}
	if len(xi.Policies) > 0 {
		if _, err = fmt.Fprintln(os.Stdout, "XFRM Policies:"); err != nil { //nolint:errcheck // output
			return
		}
		for _, p := range xi.Policies {
			if _, err = fmt.Fprintf(os.Stdout, "  %s %s -> %s proto=%s mode=%s\n", p.Dir, p.Src, p.Dst, p.Proto, p.Mode); err != nil { //nolint:errcheck // output
				return
			}
		}
	}
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
	return textbuf.Join(parts, ", ")
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
	if _, err := fmt.Fprintln(w, textbuf.Join(cols, "\t")); err != nil { //nolint:errcheck // output
		return
	}
}
