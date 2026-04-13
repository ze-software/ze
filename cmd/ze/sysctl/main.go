// Design: docs/architecture/core-design.md -- sysctl offline CLI
//
// Package sysctl provides the ze sysctl subcommand for inspecting and
// setting kernel tunables without a running daemon.
package sysctl

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	sysctlreg "codeberg.org/thomas-mangin/ze/internal/core/sysctl"
)

// Run executes the sysctl subcommand. Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" { //nolint:goconst // consistent pattern across cmd files
		usage()
		return 0
	}

	switch subcmd {
	case "list":
		return cmdList(subArgs)
	case "describe":
		return cmdDescribe(subArgs)
	case "show":
		return cmdShow(subArgs)
	case "set":
		return cmdSet(subArgs)
	}

	fmt.Fprintf(os.Stderr, "error: unknown sysctl subcommand: %s\n", subcmd)
	if s := suggest.Command(subcmd, []string{
		"list", "describe", "show", "set", "help",
	}); s != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
	}
	usage()
	return 1
}

func cmdList(_ []string) int {
	all := sysctlreg.All()
	if len(all) == 0 {
		fmt.Println("no known sysctl keys registered")
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "KEY\tTYPE\tDESCRIPTION"); err != nil {
		return 1
	}
	for _, k := range all {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", k.Name, typeName(k.Type), k.Description); err != nil {
			return 1
		}
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return 0
}

func cmdDescribe(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "error: sysctl describe requires a key argument\n")
		return 1
	}
	key := args[0]

	k, ok := sysctlreg.Lookup(key)
	if !ok {
		k, ok = sysctlreg.MatchTemplate(key)
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown key: %s (not in known-keys registry)\n", key)
		return 1
	}

	type detail struct {
		Key         string `json:"key"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Min         int    `json:"min,omitempty"`
		Max         int    `json:"max,omitempty"`
		Platform    string `json:"platform"`
		Template    bool   `json:"template,omitempty"`
	}
	d := detail{
		Key:         key,
		Description: k.Description,
		Type:        typeName(k.Type),
		Platform:    platformName(k.Platform),
		Template:    k.Template,
	}
	if k.Type == sysctlreg.TypeIntRange {
		d.Min = k.Min
		d.Max = k.Max
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal describe result: %v\n", err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}

func cmdShow(_ []string) int {
	fmt.Fprintf(os.Stderr, "error: 'ze sysctl show' requires a running daemon\n")
	fmt.Fprintf(os.Stderr, "hint: start ze, then use 'ze cli sysctl show' or the SSH CLI\n")
	fmt.Fprintf(os.Stderr, "hint: use 'ze sysctl list' to see known keys without a daemon\n")
	return 1
}

func cmdSet(_ []string) int {
	fmt.Fprintf(os.Stderr, "error: 'ze sysctl set' requires a running daemon\n")
	fmt.Fprintf(os.Stderr, "hint: start ze, then use 'ze cli sysctl set <key> <value>'\n")
	return 1
}

func typeName(t sysctlreg.ValueType) string {
	switch t {
	case sysctlreg.TypeBool:
		return "bool"
	case sysctlreg.TypeInt:
		return "int"
	case sysctlreg.TypeIntRange:
		return "int-range"
	}
	return "unknown"
}

func platformName(p sysctlreg.Platform) string {
	switch p {
	case sysctlreg.PlatformAll:
		return "all"
	case sysctlreg.PlatformLinux:
		return "linux"
	case sysctlreg.PlatformDarwin:
		return "darwin"
	}
	return "unknown"
}

func usage() {
	p := helpfmt.Page{
		Command: "ze sysctl",
		Summary: "inspect and manage kernel tunables",
		Usage:   []string{"ze sysctl <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Offline Commands (no daemon needed)", Entries: []helpfmt.HelpEntry{
				{Name: "list", Desc: "List all known sysctl keys with descriptions"},
				{Name: "describe <key>", Desc: "Show detail for one known key"},
			}},
			{Title: "Daemon Commands (requires running ze)", Entries: []helpfmt.HelpEntry{
				{Name: "show", Desc: "Show active keys (via daemon CLI)"},
				{Name: "set <key> <value>", Desc: "Set a transient value (via daemon CLI)"},
			}},
		},
		Examples: []string{
			"ze sysctl list",
			"ze sysctl describe net.ipv4.conf.all.forwarding",
			"ze cli sysctl show      # requires running daemon",
			"ze cli sysctl set net.core.somaxconn 4096",
		},
	}
	p.Write()
}
