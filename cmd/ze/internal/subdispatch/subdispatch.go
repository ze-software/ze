// Design: docs/architecture/cli/plugin-modes.md — shared action-first dispatch for install/uninstall

package subdispatch

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
)

type SubMeta struct {
	Desc string
}

type Dispatcher struct {
	Command  string
	Summary  string
	handlers map[string]func([]string) int
	metas    map[string]SubMeta
}

func New(command, summary string) *Dispatcher {
	return &Dispatcher{
		Command:  command,
		Summary:  summary,
		handlers: make(map[string]func([]string) int),
		metas:    make(map[string]SubMeta),
	}
}

func (d *Dispatcher) Register(name string, handler func([]string) int, meta SubMeta) {
	d.handlers[name] = handler
	d.metas[name] = meta
}

func (d *Dispatcher) Dispatch(args []string) int {
	if len(args) == 0 {
		d.usage()
		return 1
	}

	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		d.usage()
		return 0
	}

	h, ok := d.handlers[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown %s target: %s\n", d.Command, args[0])
		if s := suggest.Command(args[0], d.targetNames()); s != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
		}
		d.usage()
		return 1
	}
	return h(args[1:])
}

func (d *Dispatcher) Subcommands() string {
	return strings.Join(d.targetNames(), ", ")
}

func (d *Dispatcher) targetNames() []string {
	names := make([]string, 0, len(d.handlers))
	for k := range d.handlers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func (d *Dispatcher) usage() {
	names := d.targetNames()
	entries := make([]helpfmt.HelpEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, helpfmt.HelpEntry{Name: n, Desc: d.metas[n].Desc})
	}

	p := helpfmt.Page{
		Command: "ze " + d.Command,
		Summary: d.Summary,
		Usage:   []string{"ze " + d.Command + " <target> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Targets", Entries: entries},
		},
	}
	p.Write()
}
