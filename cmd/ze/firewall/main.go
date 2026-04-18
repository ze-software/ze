// Design: docs/architecture/core-design.md -- Firewall CLI entry point

// Package firewall provides the ze firewall subcommand for viewing
// nftables firewall state (tables, chains, rules, counters).
package firewall

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	fwpkg "codeberg.org/thomas-mangin/ze/internal/component/firewall"
	fwcmd "codeberg.org/thomas-mangin/ze/internal/component/firewall/cmd"

	// Register the nft backend so fwpkg.LoadBackend("nft") resolves.
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/firewall/nft"
)

// Run executes the firewall subcommand. Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]

	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" { //nolint:goconst // consistent pattern
		usage()
		return 0
	}

	if err := fwpkg.LoadBackend("nft"); err != nil {
		fmt.Fprintf(os.Stderr, "error: load nft backend: %v\n", err)
		return 1
	}
	defer func() {
		if err := fwpkg.CloseBackend(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: close nft backend: %v\n", err)
		}
	}()

	b := fwpkg.GetBackend()
	if b == nil {
		fmt.Fprintln(os.Stderr, "error: no firewall backend loaded")
		return 1
	}

	switch subcmd {
	case "show":
		return cmdShow(b, args[1:])
	case "counters":
		return cmdCounters(b, args[1:])
	}

	fmt.Fprintf(os.Stderr, "unknown command: ze firewall %s\n", subcmd)
	usage()
	return 1
}

func cmdShow(b fwpkg.Backend, args []string) int {
	tables, err := b.ListTables()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if len(args) > 0 && args[0] == "table" && len(args) > 1 {
		// Filter to single table.
		name := "ze_" + args[1]
		for _, t := range tables {
			if t.Name == name {
				fmt.Print(fwcmd.FormatTables([]fwpkg.Table{t}))
				return 0
			}
		}
		fmt.Fprintf(os.Stderr, "error: table %q not found\n", args[1])
		return 1
	}

	fmt.Print(fwcmd.FormatTables(tables))
	return 0
}

func cmdCounters(b fwpkg.Backend, args []string) int {
	tableName := ""
	if len(args) > 0 {
		tableName = "ze_" + args[0]
	}
	if tableName == "" {
		// Show counters for all tables.
		tables, err := b.ListTables()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		for _, t := range tables {
			counters, err := b.GetCounters(t.Name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Printf("table %s:\n%s", fwcmd.StripPrefix(t.Name), fwcmd.FormatCounters(counters))
		}
		return 0
	}

	counters, err := b.GetCounters(tableName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Print(fwcmd.FormatCounters(counters))
	return 0
}

func usage() {
	p := helpfmt.Page{
		Command: "ze firewall",
		Summary: "Firewall table visibility",
		Usage:   []string{"ze firewall <command> [options]"},
		Sections: []helpfmt.HelpSection{{
			Title: "Commands",
			Entries: []helpfmt.HelpEntry{
				{Name: "show", Desc: "Show all ze_* firewall tables"},
				{Name: "show table <name>", Desc: "Show a single table"},
				{Name: "counters", Desc: "Show packet/byte counters"},
				{Name: "counters <table>", Desc: "Show counters for a specific table"},
			},
		}},
	}
	p.Write()
}
