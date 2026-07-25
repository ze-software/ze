// Design: docs/architecture/core-design.md — CLI help formatting
//
// Package helpfmt provides structured, color-aware help output for ze CLI commands.
// Subcommands build a Page struct and call WriteTo to render with semantic coloring.
// Color is controlled by slogutil.UseColor.
package helpfmt

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Role styles -- map UI roles to palette colors. Change here to restyle globally.
const (
	styleHeader     = textbuf.ColorBoldMagenta  // section titles
	styleCommand    = textbuf.ColorBoldCyan     // top-level command path
	styleSubcommand = textbuf.ColorBrightGreen  // entry names in lists
	styleFlag       = textbuf.ColorBrightYellow // flag names
	styleArg        = textbuf.ColorDim          // placeholders <file>, [options]
	styleExample    = textbuf.ColorDim          // example lines
	styleError      = textbuf.ColorBoldRed      // error messages
	styleHint       = textbuf.ColorBrightYellow // hint/suggestion messages
	styleSummary    = textbuf.ColorDim          // command summary after dash
	colorReset      = textbuf.ColorReset
)

// Page is a structured help page for a CLI command.
type Page struct {
	Command  string        // e.g. "ze bgp"
	Summary  string        // e.g. "BGP protocol tools" (subcommand description)
	Software string        // e.g. "ze Software" (top-level only, styled differently)
	Usage    []string      // usage patterns
	Sections []HelpSection // groups of entries
	Examples []string      // example command lines
	SeeAlso  []string      // related commands
}

// HelpSection is a named group of entries in a help page.
type HelpSection struct {
	Title   string      // e.g. "Commands", "Options"
	Entries []HelpEntry // name + description pairs
}

// HelpEntry is a single command, flag, or option in a help section.
type HelpEntry struct {
	Name string // e.g. "decode <hex>" or "--verbose"
	Desc string // description text
}

// WriteErr renders the help page to stderr with automatic color detection.
func (p *Page) WriteErr() {
	color := slogutil.UseColor(os.Stderr)
	p.WriteTo(os.Stderr, color)
}

// WriteOut renders the help page to stdout with automatic color detection.
func (p *Page) WriteOut() {
	color := slogutil.UseColor(os.Stdout)
	p.WriteTo(os.Stdout, color)
}

// WriteTo renders the help page to w. If color is true, applies ANSI codes.
// Output routes through a RenderWriter so a broken pipe stops the render loop;
// the byte stream is identical to the previous fmt.Fprintf implementation.
func (p *Page) WriteTo(w io.Writer, color bool) {
	rw := NewRenderWriter(w)
	var b textbuf.Buffer

	// Header: "command - software" or "command - summary"
	switch {
	case p.Software != "":
		rw.Line(b.Reset().Str(styled(color, styleCommand, p.Command)).Str(" - ").Str(p.Software).String())
	case p.Summary != "":
		rw.Line(b.Reset().Str(styled(color, styleCommand, p.Command)).Str(" - ").Str(styled(color, styleSummary, p.Summary)).String())
	default:
		rw.Line(styled(color, styleCommand, p.Command))
	}

	// Usage
	if len(p.Usage) > 0 {
		rw.Str("\n")
		rw.Line(styled(color, styleHeader, "Usage:"))
		for _, u := range p.Usage {
			rw.Line(b.Reset().Str("  ").Str(highlightArgs(color, u)).String())
		}
	}

	// Sections
	for _, s := range p.Sections {
		if len(s.Entries) == 0 {
			continue
		}
		rw.Str("\n")
		rw.Line(styled(color, styleHeader, b.Reset().Str(s.Title).Byte(':').String()))
		width := entryWidth(s.Entries)
		for _, e := range s.Entries {
			// Pad based on raw name length, then apply color.
			// ANSI codes add bytes, so pad the raw name first.
			padded := b.Reset().PadRight(e.Name, width).String()
			rw.Line(b.Reset().Str("  ").Str(styleEntry(color, padded)).Byte(' ').Str(Summary(e.Desc)).String())
		}
	}

	// Examples
	if len(p.Examples) > 0 {
		rw.Str("\n")
		rw.Line(styled(color, styleHeader, "Examples:"))
		for _, ex := range p.Examples {
			rw.Line(b.Reset().Str("  ").Str(styled(color, styleExample, ex)).String())
		}
	}

	// See also
	if len(p.SeeAlso) > 0 {
		rw.Str("\n")
		rw.Line(styled(color, styleHeader, "See also:"))
		for _, sa := range p.SeeAlso {
			rw.Line(b.Reset().Str("  ").Str(styled(color, styleExample, sa)).String())
		}
	}

	rw.Str("\n")
}

// WriteError writes a colored error message to w.
func WriteError(w io.Writer, color bool, format string, a ...any) {
	prefix := styled(color, styleError, "error:")
	fmt.Fprintf(w, "%s %s\n", prefix, fmt.Sprintf(format, a...)) //nolint:errcheck // help output to stderr
}

// WriteHint writes a colored hint message to w.
func WriteHint(w io.Writer, color bool, format string, a ...any) {
	prefix := styled(color, styleHint, "hint:")
	fmt.Fprintf(w, "%s %s\n", prefix, fmt.Sprintf(format, a...)) //nolint:errcheck // help output to stderr
}

// Summary returns the first sentence or first line of a description.
// Multi-line YANG descriptions carry grammar and action details
// that belong in per-command help, not top-level listings.
func Summary(s string) string {
	if s == "" {
		return ""
	}

	firstLineEnd := -1
	for i := range len(s) {
		switch s[i] {
		case '\n':
			if firstLineEnd < 0 {
				firstLineEnd = i
			}
		case '.', '!', '?':
			if s[i] == '.' && ((i > 0 && s[i-1] == '.') || (i+1 < len(s) && s[i+1] == '.')) {
				continue
			}
			if i+1 == len(s) || isSummarySpace(s[i+1]) {
				return cleanSummary(s[:i+1])
			}
		}
	}
	if firstLineEnd >= 0 {
		return cleanSummary(s[:firstLineEnd])
	}
	return cleanSummary(s)
}

func cleanSummary(s string) string {
	s = strings.TrimRight(s, " \t\r\n")
	if !needsSummarySpaceCollapse(s) {
		return s
	}

	var b textbuf.Buffer
	pendingSpace := false
	for i := range len(s) {
		if isSummarySpace(s[i]) {
			if b.Len() > 0 {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace {
			b.Byte(' ')
			pendingSpace = false
		}
		b.Byte(s[i])
	}
	return b.String()
}

func needsSummarySpaceCollapse(s string) bool {
	previousSpace := false
	for i := range len(s) {
		if isSummarySpace(s[i]) {
			if s[i] != ' ' || previousSpace {
				return true
			}
			previousSpace = true
			continue
		}
		previousSpace = false
	}
	return false
}

func isSummarySpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t' || c == '\r'
}

// entryWidth returns the column width for a section's entries.
// Uses the longest entry name, with a minimum of 16 and ANSI codes excluded from measurement.
func entryWidth(entries []HelpEntry) int {
	w := 16
	for _, e := range entries {
		if n := len(e.Name); n > w {
			w = n
		}
	}
	return w
}

// styled wraps s in the ANSI code if color is enabled.
func styled(color bool, code, s string) string {
	if !color {
		return s
	}
	return code + s + colorReset
}

// styleEntry colors an entry name based on whether it's a flag or subcommand.
// Flags (starting with -) get styleFlag, subcommands get styleSubcommand.
func styleEntry(color bool, name string) string {
	if !color {
		return name
	}
	var b textbuf.Buffer
	if strings.HasPrefix(name, "-") {
		return b.Str(styleFlag).Str(name).Str(colorReset).String()
	}
	return b.Str(styleSubcommand).Str(name).Str(colorReset).String()
}

// highlightArgs colors angle-bracket and square-bracket placeholders in a usage line.
func highlightArgs(color bool, line string) string {
	if !color {
		return line
	}
	var b textbuf.Buffer
	b.Grow(len(line) + 40)
	i := 0
	for i < len(line) {
		switch line[i] {
		case '<':
			end := strings.IndexByte(line[i:], '>')
			if end == -1 {
				b.Str(line[i:])
				return b.String()
			}
			b.Str(styleArg).Str(line[i : i+end+1]).Str(colorReset)
			i += end + 1
		case '[':
			end := strings.IndexByte(line[i:], ']')
			if end == -1 {
				b.Str(line[i:])
				return b.String()
			}
			b.Str(styleArg).Str(line[i : i+end+1]).Str(colorReset)
			i += end + 1
		default:
			b.Byte(line[i])
			i++
		}
	}
	return b.String()
}
