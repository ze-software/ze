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

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// Color palette -- ANSI codes by name.
const (
	colorReset        = "\033[0m"
	colorDim          = "\033[2m"
	colorBoldRed      = "\033[1;31m"
	colorBrightGreen  = "\033[92m"
	colorBrightYellow = "\033[93m"
	colorBrightCyan   = "\033[96m"
	colorBoldCyan     = "\033[1;96m"
	colorBoldMagenta  = "\033[1;95m"
)

// Role styles -- map UI roles to palette colors. Change here to restyle globally.
const (
	styleHeader     = colorBoldMagenta  // section titles
	styleCommand    = colorBoldCyan     // top-level command path
	styleSubcommand = colorBrightGreen  // entry names in lists
	styleFlag       = colorBrightYellow // flag names
	styleArg        = colorDim          // placeholders <file>, [options]
	styleExample    = colorDim          // example lines
	styleError      = colorBoldRed      // error messages
	styleHint       = colorBrightYellow // hint/suggestion messages
	styleSummary    = colorDim          // command summary after dash
	styleSoftware   = colorBrightCyan   // software name in top-level header
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

// Write renders the help page to stderr with automatic color detection.
func (p *Page) Write() {
	color := slogutil.UseColor(os.Stderr)
	p.WriteTo(os.Stderr, color)
}

// WriteTo renders the help page to w. If color is true, applies ANSI codes.
func (p *Page) WriteTo(w io.Writer, color bool) {
	wr := func(format string, a ...any) { fmt.Fprintf(w, format, a...) } //nolint:errcheck // help output to stderr

	// Header: "command - software" or "command - summary"
	switch {
	case p.Software != "":
		wr("%s - %s\n", styled(color, styleCommand, p.Command), p.Software)
	case p.Summary != "":
		wr("%s - %s\n", styled(color, styleCommand, p.Command), styled(color, styleSummary, p.Summary))
	default:
		wr("%s\n", styled(color, styleCommand, p.Command))
	}

	// Usage
	if len(p.Usage) > 0 {
		wr("\n%s\n", styled(color, styleHeader, "Usage:"))
		for _, u := range p.Usage {
			wr("  %s\n", highlightArgs(color, u))
		}
	}

	// Sections
	for _, s := range p.Sections {
		if len(s.Entries) == 0 {
			continue
		}
		wr("\n%s\n", styled(color, styleHeader, s.Title+":"))
		width := entryWidth(s.Entries)
		for _, e := range s.Entries {
			// Pad based on raw name length, then apply color.
			// ANSI codes add bytes that fmt.Sprintf counts, so pad first.
			padded := fmt.Sprintf("%-*s", width, e.Name)
			wr("  %s %s\n", styleEntry(color, padded), e.Desc)
		}
	}

	// Examples
	if len(p.Examples) > 0 {
		wr("\n%s\n", styled(color, styleHeader, "Examples:"))
		for _, ex := range p.Examples {
			wr("  %s\n", styled(color, styleExample, ex))
		}
	}

	// See also
	if len(p.SeeAlso) > 0 {
		wr("\n%s\n", styled(color, styleHeader, "See also:"))
		for _, sa := range p.SeeAlso {
			wr("  %s\n", styled(color, styleExample, sa))
		}
	}
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
	if strings.HasPrefix(name, "-") {
		return styleFlag + name + colorReset
	}
	return styleSubcommand + name + colorReset
}

// highlightArgs colors angle-bracket and square-bracket placeholders in a usage line.
func highlightArgs(color bool, line string) string {
	if !color {
		return line
	}
	var b strings.Builder
	b.Grow(len(line) + 40)
	i := 0
	for i < len(line) {
		switch line[i] {
		case '<':
			end := strings.IndexByte(line[i:], '>')
			if end == -1 {
				b.WriteString(line[i:])
				return b.String()
			}
			b.WriteString(styleArg)
			b.WriteString(line[i : i+end+1])
			b.WriteString(colorReset)
			i += end + 1
		case '[':
			end := strings.IndexByte(line[i:], ']')
			if end == -1 {
				b.WriteString(line[i:])
				return b.String()
			}
			b.WriteString(styleArg)
			b.WriteString(line[i : i+end+1])
			b.WriteString(colorReset)
			i += end + 1
		default:
			b.WriteByte(line[i])
			i++
		}
	}
	return b.String()
}
