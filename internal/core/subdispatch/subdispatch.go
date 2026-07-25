// Design: docs/architecture/cli/plugin-modes.md — shared action-first dispatch for install/uninstall

package subdispatch

import (
	"os"
	"sort"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func writeStderr(s string) {
	os.Stderr.WriteString(s) //nolint:errcheck // best-effort CLI output
}

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
		var tb textbuf.Buffer
		writeStderr(tb.Str("unknown ").Str(d.Command).Str(" target: ").Str(args[0]).Byte('\n').Slice())
		if s := suggest.Command(args[0], d.targetNames()); s != "" {
			writeStderr(tb.Reset().Str("hint: did you mean '").Str(s).Str("'?\n").Slice())
		}
		d.usage()
		return 1
	}
	return h(args[1:])
}

func (d *Dispatcher) Subcommands() string {
	return textbuf.Join(d.targetNames(), ", ")
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

	var tb textbuf.Buffer
	p := helpfmt.Page{
		Command: tb.Str("ze ").Str(d.Command).String(),
		Summary: d.Summary,
		Usage:   []string{tb.Reset().Str("ze ").Str(d.Command).Str(" <target> [options]").String()},
		Sections: []helpfmt.HelpSection{
			{Title: "Targets", Entries: entries},
		},
	}
	p.WriteErr()
}
