// Design: docs/architecture/config/environment.md — ze env CLI command

package env

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/envcatalog"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage()
		return 0
	case "registered":
		return cmdRegistered(args[1:])
	case "list":
		return cmdList(args[1:])
	case "get":
		if len(args) < 2 {
			writeErr("error: ze env get requires a key")
			return 1
		}
		return showOne(args[1])
	}

	var tb textbuf.Buffer
	writeErr(tb.Str("error: unknown env command: ").Str(args[0]).String())
	usage()
	return 1
}

func cmdRegistered(args []string) int {
	if len(args) > 0 {
		return showOne(args[0])
	}
	return showAll(false)
}

func cmdList(args []string) int {
	fs := flag.NewFlagSet("ze env list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	verbose := fs.Bool("v", false, "show current effective values")
	fs.BoolVar(verbose, "verbose", false, "show current effective values")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		var tb textbuf.Buffer
		writeErr(tb.Str("error: unexpected argument \"").Str(fs.Arg(0)).Byte('"').String())
		return 1
	}
	return showAll(*verbose)
}

func showAll(verbose bool) int {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	if verbose {
		printRow(w, "KEY", "TYPE", "DEFAULT", "CURRENT", "DESCRIPTION")
		printRow(w, "---", "----", "-------", "-------", "-----------")
	} else {
		printRow(w, "KEY", "TYPE", "DEFAULT", "DESCRIPTION")
		printRow(w, "---", "----", "-------", "-----------")
	}

	entries := env.Entries()
	slices.SortFunc(entries, func(a, b env.EnvEntry) int {
		return strings.Compare(a.Key, b.Key)
	})
	for _, e := range entries {
		dflt := valueOrDash(e.Default)
		if verbose {
			current := currentValue(e.Key)
			printRow(w, e.Key, e.Type, dflt, current, e.Description)
		} else {
			printRow(w, e.Key, e.Type, dflt, e.Description)
		}
	}

	if err := w.Flush(); err != nil {
		var tb textbuf.Buffer
		writeErr(tb.Str("error: ").Err(err).String())
		return 1
	}

	if subs := slogutil.Subsystems(); len(subs) > 0 {
		out := bufio.NewWriter(os.Stdout)
		writeLine(out, "")
		writeLine(out, "Log subsystems (ze.log.<name>=<level>, hierarchical: ze.log.bgp sets all bgp.* subsystems):")
		writeLine(out, "Levels: disabled, debug, info, warn, err")
		writeLine(out, "")
		sw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		for _, sub := range subs {
			desc := sub.Description
			if desc == "" {
				desc = "-"
			}
			var tb textbuf.Buffer
			printRow(sw, tb.Str("  ze.log.").Str(sub.Name).String(), desc)
		}
		if err := sw.Flush(); err != nil {
			var tb textbuf.Buffer
			writeErr(tb.Str("error: ").Err(err).String())
			return 1
		}
		if err := out.Flush(); err != nil {
			var tb textbuf.Buffer
			writeErr(tb.Str("error: ").Err(err).String())
			return 1
		}
	}

	return 0
}

func printRow(w *tabwriter.Writer, cols ...string) {
	if _, err := fmt.Fprintln(w, textbuf.Join(cols, "\t")); err != nil { //nolint:errcheck // output
		return
	}
}

func writeLine(w *bufio.Writer, s string) {
	var tb textbuf.Buffer
	if _, err := w.WriteString(tb.Str(s).Byte('\n').Slice()); err != nil {
		return
	}
}

func currentValue(key string) string {
	if strings.Contains(key, "<") {
		return "-"
	}
	if env.IsSecret(key) {
		v := env.Get(key)
		if v != "" {
			return "****"
		}
		return "-"
	}
	return valueOrDash(env.Get(key))
}

func showOne(key string) int {
	normalized := strings.ReplaceAll(strings.ToLower(key), "_", ".")

	for _, e := range env.AllEntries() {
		if e.Key != normalized && e.Key != key {
			continue
		}

		current := currentValue(e.Key)
		out := bufio.NewWriter(os.Stdout)
		var tb textbuf.Buffer
		writeLine(out, tb.Str("Key:         ").Str(e.Key).Slice())
		writeLine(out, tb.Reset().Str("Type:        ").Str(e.Type).Slice())
		writeLine(out, tb.Reset().Str("Default:     ").Str(valueOrDash(e.Default)).Slice())
		writeLine(out, tb.Reset().Str("Current:     ").Str(valueOrDash(current)).Slice())
		writeLine(out, tb.Reset().Str("Description: ").Str(e.Description).Slice())
		if e.Private {
			writeLine(out, "Private:     yes")
		}
		if e.Secret {
			writeLine(out, "Secret:      yes")
		}

		under := strings.ReplaceAll(e.Key, ".", "_")
		writeLine(out, "")
		writeLine(out, "Accepted forms:")
		writeLine(out, tb.Reset().Str("  ").Str(e.Key).Slice())
		writeLine(out, tb.Reset().Str("  ").Str(under).Slice())
		writeLine(out, tb.Reset().Str("  ").Str(strings.ToUpper(under)).Slice())
		if err := out.Flush(); err != nil {
			return 1
		}
		return 0
	}

	if sub, ok := envcatalog.LookupLogSubsystem(normalized); ok {
		return showLogSubsystem(normalized, sub)
	}

	var tb textbuf.Buffer
	writeErr(tb.Str("error: unknown env var \"").Str(key).Byte('"').Slice())
	return 1
}

func showLogSubsystem(key string, sub slogutil.SubsystemInfo) int {
	out := bufio.NewWriter(os.Stdout)
	var tb textbuf.Buffer
	desc := sub.Description
	if desc == "" {
		desc = tb.Reset().Str("Log level for ").Str(sub.Name).String()
	}
	writeLine(out, tb.Reset().Str("Key:         ").Str(key).Slice())
	writeLine(out, "Type:        string")
	writeLine(out, "Default:     -")
	writeLine(out, tb.Reset().Str("Current:     ").Str(valueOrDash(env.Get(key))).Slice())
	writeLine(out, tb.Reset().Str("Description: ").Str(desc).Slice())

	under := strings.ReplaceAll(key, ".", "_")
	writeLine(out, "")
	writeLine(out, "Accepted forms:")
	writeLine(out, tb.Reset().Str("  ").Str(key).Slice())
	writeLine(out, tb.Reset().Str("  ").Str(under).Slice())
	writeLine(out, tb.Reset().Str("  ").Str(strings.ToUpper(under)).Slice())
	if err := out.Flush(); err != nil {
		return 1
	}
	return 0
}

func valueOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func writeErr(s string) {
	var tb textbuf.Buffer
	if err := tb.Str(s).Byte('\n').StdErr(); err != nil {
		return
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze env",
		Summary: "List and inspect Ze environment variables",
		Usage:   []string{"ze env <command> [args...]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "registered [key]", Desc: "List all registered env vars, or show one by key"},
				{Name: "list [-v]", Desc: "List all registered env vars (with optional values)"},
				{Name: "get <key>", Desc: "Show details for a specific env var"},
				{Name: "help", Desc: "Show this help"},
			}},
			{Title: "Notes", Entries: []helpfmt.HelpEntry{
				{Name: "-v", Desc: "With list, shows the current effective value alongside defaults"},
			}},
			{Title: "All Ze env vars accept three notation forms", Entries: []helpfmt.HelpEntry{
				{Name: "ze.foo.bar", Desc: "(dot notation, highest priority)"},
				{Name: "ze_foo_bar", Desc: "(lowercase underscore)"},
				{Name: "ZE_FOO_BAR", Desc: "(uppercase underscore, shell convention)"},
			}},
		},
		Examples: []string{
			"ze env registered                # List all registered env vars",
			"ze env registered ze.log         # Show details for ze.log",
			"ze env list -v                   # List with current values",
			"ze env get ZE_PLUGIN_HUB_HOST    # Look up by any notation",
		},
	}
	p.WriteErr()
}
