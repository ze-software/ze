// Design: docs/architecture/cli/plugin-modes.md — shared action-first dispatch for install/uninstall

package subdispatch

import (
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func writeStderr(s string) {
	os.Stderr.WriteString(s) //nolint:errcheck // best-effort CLI output
}

type SubMeta struct {
	Desc    string
	Options func() []helpfmt.HelpEntry
}

type Dispatcher struct {
	Command        string
	Summary        string
	handlers       map[string]func([]string) int
	metas          map[string]SubMeta
	defaultHandler func([]string) int
	defaultMeta    SubMeta
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

// RegisterDefault declares the owner of the bare command and its offline options.
// Registration MUST finish before Dispatch is called.
func (d *Dispatcher) RegisterDefault(handler func([]string) int, meta SubMeta) {
	if handler == nil {
		panic("BUG: nil default command handler")
	}
	if d.defaultHandler != nil {
		panic("BUG: duplicate default command handler")
	}
	d.defaultHandler = handler
	d.defaultMeta = meta
}

func (d *Dispatcher) Dispatch(args []string) int {
	if len(args) == 0 {
		if d.defaultHandler != nil {
			return d.defaultHandler(args)
		}
		d.usage()
		return 1
	}

	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		d.usage()
		return 0
	}
	if strings.HasPrefix(args[0], "-") {
		if d.defaultHandler != nil {
			return d.defaultHandler(args)
		}
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

// Target is one registered subcommand: the word typed and its one-line
// description.
type Target struct {
	Name string `json:"name"`
	Desc string `json:"description"`
}

// Targets answers every registered subcommand, sorted by name, so a caller that
// lists them states the registry rather than a copy of it.
func (d *Dispatcher) Targets() []Target {
	names := d.targetNames()
	targets := make([]Target, 0, len(names))
	for _, name := range names {
		targets = append(targets, Target{Name: name, Desc: d.metas[name].Desc})
	}
	return targets
}

func (d *Dispatcher) targetNames() []string {
	names := make([]string, 0, len(d.handlers))
	for k := range d.handlers {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

func (d *Dispatcher) usage() {
	names := d.targetNames()
	entries := make([]helpfmt.HelpEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, helpfmt.HelpEntry{Name: n, Desc: d.metas[n].Desc})
	}
	if d.defaultHandler != nil {
		entries = append([]helpfmt.HelpEntry{{Name: "(no target)", Desc: d.defaultMeta.Desc}}, entries...)
	}

	var tb textbuf.Buffer
	usage := tb.Str("ze ").Str(d.Command).Str(" <target> [options]").String()
	if d.defaultHandler != nil {
		usage = tb.Reset().Str("ze ").Str(d.Command).Str(" [target] [options]").String()
	}
	sections := []helpfmt.HelpSection{{Title: "Targets", Entries: entries}}
	if d.defaultMeta.Options != nil {
		sections = append(sections, helpfmt.HelpSection{Title: "Default options", Entries: d.defaultMeta.Options()})
	}
	p := helpfmt.Page{
		Command:   tb.Str("ze ").Str(d.Command).String(),
		ShortHelp: d.Summary,
		Usage:     []string{usage},
		Sections:  sections,
	}
	p.WriteErr()
}
