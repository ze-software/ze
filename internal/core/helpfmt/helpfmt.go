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
//
// ShortHelp and Description are the command's two declared help texts. Neither is
// derived from the other. ShortHelp goes on the header line. Description is the long
// explanation, and it goes in the body block. A command with no long
// explanation leaves Description empty and prints no block.
type Page struct {
	Command     string        // e.g. "ze bgp"
	ShortHelp   string        // e.g. "BGP protocol tools" (the one-line summary)
	Description string        // the long explanation, printed as the body block
	Software    string        // e.g. "ze Software" (top-level only, styled differently)
	Usage       []string      // usage patterns
	Sections    []HelpSection // groups of entries
	Examples    []string      // example command lines
	SeeAlso     []string      // related commands
	// Width is the column a rendered line stops at. Zero renders every line
	// whole, which is the page ze has always printed. A positive width wraps the
	// description, each usage line, and each entry's text at a space, and an
	// entry's continuation starts under its description column. A word longer
	// than the room left stays whole on its own line, so no token is ever cut.
	Width int
}

// HelpSection is a named group of entries in a help page.
type HelpSection struct {
	Title string // e.g. "Commands", "Options"
	// Summary is one line saying what the section holds, rendered after the
	// title on the heading line. Empty renders the title alone.
	Summary string
	Entries []HelpEntry // name + description pairs
}

// HelpEntry is a single command, flag, or option in a help section.
type HelpEntry struct {
	Name string // e.g. "decode <hex>" or "--verbose"
	// Desc is the one-line summary, rendered whole. A caller with a long
	// explanation puts it in Page.Description, never here. A section entry is one row,
	// and this package shortens nothing.
	Desc string
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
	case p.ShortHelp != "":
		rw.Line(b.Reset().Str(styled(color, styleCommand, p.Command)).Str(" - ").Str(styled(color, styleSummary, p.ShortHelp)).String())
	default:
		// A header with no summary can be a whole usage line, as a refusal's
		// is, so it wraps as one.
		for _, part := range wrap(p.Command, p.Width, 0, 6) {
			rw.Line(styled(color, styleCommand, part))
		}
	}

	// The long explanation, indented, keeping the newlines its author wrote.
	if p.Description != "" {
		rw.Str("\n")
		for line := range strings.SplitSeq(p.Description, "\n") {
			for _, part := range wrap(strings.TrimRight(line, " \t"), p.Width, 2, 2) {
				rw.Line(part)
			}
		}
	}

	// Usage
	if len(p.Usage) > 0 {
		rw.Str("\n")
		rw.Line(styled(color, styleHeader, "Usage:"))
		for _, u := range p.Usage {
			for _, part := range wrap(u, p.Width, 2, 6) {
				rw.Line(highlightArgs(color, part))
			}
		}
	}

	// Sections
	for _, s := range p.Sections {
		if len(s.Entries) == 0 {
			continue
		}
		rw.Str("\n")
		heading := b.Reset().Str(s.Title).Byte(':').String()
		if s.Summary == "" {
			rw.Line(styled(color, styleHeader, heading))
		} else {
			// The summary wraps like a description: its first line follows
			// the title, and it continues under the entries' indent.
			parts := wrap(s.Summary, p.Width, len(heading)+1, 2)
			rw.Line(b.Reset().Str(styled(color, styleHeader, heading)).Byte(' ').Str(parts[0][len(heading)+1:]).String())
			for _, part := range parts[1:] {
				rw.Line(part)
			}
		}
		width := entryWidth(s.Entries)
		for _, e := range s.Entries {
			// Pad based on raw name length, then apply color.
			// ANSI codes add bytes, so pad the raw name first.
			padded := b.Reset().PadRight(e.Name, width).String()
			column := 2 + width + 1
			// Below the description column plus narrowColumns there is too
			// little room beside the name, so the description takes its own
			// lines, indented under the name, and continues at that indent.
			if p.Width > 0 && p.Width < column+narrowColumns {
				rw.Line(b.Reset().Str("  ").Str(styleEntry(color, e.Name)).String())
				if e.Desc == "" {
					continue
				}
				for _, part := range wrap(e.Desc, p.Width, narrowIndent, narrowIndent) {
					rw.Line(part)
				}
				continue
			}
			// The first part carries the description column's indent, which
			// the name already fills, so it is cut off before the name goes on.
			parts := wrap(e.Desc, p.Width, column, column)
			rw.Line(b.Reset().Str("  ").Str(styleEntry(color, padded)).Byte(' ').Str(parts[0][column:]).String())
			for _, part := range parts[1:] {
				rw.Line(part)
			}
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

// narrowColumns is the least room a description is given beside its name. A
// page narrower than the description column plus this moves each description
// onto its own lines, indented by narrowIndent.
const (
	narrowColumns = 20
	narrowIndent  = 4
)

// wrap breaks text at spaces into lines of at most width columns, the first
// indented by first spaces and every later one by next. A width of zero or less
// keeps the text on one line, so a page that sets no Width renders as it always
// has. A word longer than the room left goes on a line of its own, whole.
func wrap(text string, width, first, next int) []string {
	var b textbuf.Buffer
	if width <= 0 {
		return []string{b.Repeat(" ", first).Str(text).String()}
	}
	words := wrapWords(text)
	if len(words) == 0 {
		return []string{b.Repeat(" ", first).String()}
	}
	lines := make([]string, 0, 2)
	b.Repeat(" ", first).Str(words[0])
	used := first + len(words[0])
	for _, word := range words[1:] {
		if used+1+len(word) <= width {
			b.Byte(' ').Str(word)
			used += 1 + len(word)
			continue
		}
		lines = append(lines, b.String())
		b.Reset().Repeat(" ", next).Str(word)
		used = next + len(word)
	}
	return append(lines, b.String())
}

// wrapWords splits text at spaces, except inside brackets: a usage line's
// `[timeout <duration>]` or `[| json | yaml | table]` is one thing a reader
// types or skips, so a line never breaks inside it.
func wrapWords(text string) []string {
	fields := strings.Fields(text)
	words := make([]string, 0, len(fields))
	depth := 0
	for _, field := range fields {
		if depth > 0 {
			words[len(words)-1] += " " + field
		} else {
			words = append(words, field)
		}
		// A group opens only on a word that starts with a bracket, so a
		// comparison in prose ("a < b") never glues the rest of a line.
		if depth == 0 && !strings.HasPrefix(field, "[") && !strings.HasPrefix(field, "<") {
			continue
		}
		depth += strings.Count(field, "[") + strings.Count(field, "<")
		depth -= strings.Count(field, "]") + strings.Count(field, ">")
		depth = max(depth, 0)
	}
	return words
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
