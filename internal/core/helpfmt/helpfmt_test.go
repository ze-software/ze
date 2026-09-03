package helpfmt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPageWriteColored verifies colored output contains ANSI codes for each role.
//
// VALIDATES: AC-1: section headers, command names, flags get semantic color.
// PREVENTS: Color logic being silently broken.
func TestPageWriteColored(t *testing.T) {
	p := Page{
		Command: "ze bgp",
		Summary: "BGP protocol tools",
		Usage:   []string{"ze bgp <command> [options]"},
		Sections: []HelpSection{
			{Title: "Commands", Entries: []HelpEntry{
				{Name: "decode", Desc: "Decode BGP message"},
				{Name: "--verbose", Desc: "Enable verbose output"},
			}},
		},
		Examples: []string{"ze bgp decode update <hex>"},
	}

	var buf bytes.Buffer
	p.WriteTo(&buf, true)
	out := buf.String()

	// Header "Commands:" should have header style
	assert.Contains(t, out, styleHeader+"Commands:"+colorReset)
	// Command name "ze bgp" should have command style
	assert.Contains(t, out, styleCommand+"ze bgp"+colorReset)
	// Subcommand "decode" should have subcommand style
	assert.Contains(t, out, styleSubcommand)
	// Flag "--verbose" should have flag style
	assert.Contains(t, out, styleFlag)
	// Example should have example style
	assert.Contains(t, out, styleExample)
}

// TestPageWritePlain verifies plain output has no ANSI codes.
//
// VALIDATES: AC-2: NO_COLOR / non-TTY produces clean text.
// PREVENTS: ANSI codes leaking into piped output.
func TestPageWritePlain(t *testing.T) {
	p := Page{
		Command: "ze bgp",
		Summary: "BGP protocol tools",
		Usage:   []string{"ze bgp <command> [options]"},
		Sections: []HelpSection{
			{Title: "Commands", Entries: []HelpEntry{
				{Name: "decode", Desc: "Decode BGP message"},
			}},
		},
		Examples: []string{"ze bgp decode update <hex>"},
	}

	var buf bytes.Buffer
	p.WriteTo(&buf, false)
	out := buf.String()

	assert.NotContains(t, out, "\033[")
	assert.Contains(t, out, "ze bgp")
	assert.Contains(t, out, "Commands:")
	assert.Contains(t, out, "decode")
}

// TestFlagDetection verifies entries starting with - get flag color, not subcommand.
//
// VALIDATES: Flags auto-detected by prefix and colored yellow.
// PREVENTS: Flags rendered in subcommand green.
func TestFlagDetection(t *testing.T) {
	p := Page{
		Command: "ze test",
		Summary: "test",
		Sections: []HelpSection{
			{Title: "Options", Entries: []HelpEntry{
				{Name: "-v, --verbose", Desc: "Verbose"},
				{Name: "run", Desc: "Run something"},
			}},
		},
	}

	var buf bytes.Buffer
	p.WriteTo(&buf, true)
	out := buf.String()

	// Flag should use flag style (padded with trailing spaces inside ANSI)
	assert.Contains(t, out, styleFlag+"-v, --verbose")
	// Subcommand should use subcommand style
	assert.Contains(t, out, styleSubcommand+"run")
}

// TestArgHighlighting verifies angle brackets in usage lines get dim styling.
//
// VALIDATES: Placeholders like <file> are visually distinct.
// PREVENTS: Placeholders blending with literal text.
func TestArgHighlighting(t *testing.T) {
	p := Page{
		Command: "ze test",
		Summary: "test",
		Usage:   []string{"ze test <file> [options]"},
	}

	var buf bytes.Buffer
	p.WriteTo(&buf, true)
	out := buf.String()

	assert.Contains(t, out, styleArg+"<file>"+colorReset)
	assert.Contains(t, out, styleArg+"[options]"+colorReset)
}

// TestEmptySection verifies sections with no entries are omitted.
//
// VALIDATES: Empty sections don't produce blank headers.
// PREVENTS: Orphan section titles in output.
func TestEmptySection(t *testing.T) {
	p := Page{
		Command: "ze test",
		Summary: "test",
		Sections: []HelpSection{
			{Title: "Empty", Entries: nil},
			{Title: "Full", Entries: []HelpEntry{{Name: "cmd", Desc: "desc"}}},
		},
	}

	var buf bytes.Buffer
	p.WriteTo(&buf, false)
	out := buf.String()

	assert.NotContains(t, out, "Empty:")
	assert.Contains(t, out, "Full:")
}

// TestWriteError verifies error output in bold red when color enabled.
//
// VALIDATES: Error messages are visually distinct.
// PREVENTS: Errors blending with normal output.
func TestWriteError(t *testing.T) {
	var buf bytes.Buffer
	WriteError(&buf, true, "file not found: %s", "test.conf")
	out := buf.String()

	assert.Contains(t, out, styleError+"error:"+colorReset)
	assert.Contains(t, out, "file not found: test.conf")
}

// TestWriteHint verifies hint output in yellow when color enabled.
//
// VALIDATES: Hints are visually distinct from errors.
// PREVENTS: Hints looking like normal text.
func TestWriteHint(t *testing.T) {
	var buf bytes.Buffer
	WriteHint(&buf, true, "did you mean '%s'?", "decode")
	out := buf.String()

	assert.Contains(t, out, styleHint+"hint:"+colorReset)
	assert.Contains(t, out, "did you mean 'decode'?")
}

// TestSeeAlso verifies See also section is rendered.
//
// VALIDATES: Cross-references appear in output.
// PREVENTS: SeeAlso field being silently ignored.
func TestSeeAlso(t *testing.T) {
	p := Page{
		Command: "ze test",
		Summary: "test",
		SeeAlso: []string{"ze config validate", "ze schema"},
	}

	var buf bytes.Buffer
	p.WriteTo(&buf, false)
	out := buf.String()

	assert.Contains(t, out, "See also:")
	assert.Contains(t, out, "ze config validate")
}

// TestPageRendersEachDeclaredHelpTextWhole renders both declared texts and
// asserts neither is shortened. The entry summary keeps its second sentence.
// The long explanation keeps the newlines its author wrote.
//
// VALIDATES: AC-2, AC-4 -- the page shortens nothing it is given.
// PREVENTS: the deleted first-sentence cut coming back. It dropped every word
// after the first full stop on five shipped help pages.
func TestPageRendersEachDeclaredHelpTextWhole(t *testing.T) {
	p := Page{
		Command:  "ze bgp",
		Summary:  "Inspect the BGP protocol engine.",
		LongHelp: "One subtree per session.\nEach answers over the negotiated families.",
		Sections: []HelpSection{{Title: "Commands", Entries: []HelpEntry{
			{Name: "rib", Desc: "Show one row per route. The row carries the next hop."},
		}}},
	}

	var buf bytes.Buffer
	p.WriteTo(&buf, false)
	out := buf.String()

	assert.Contains(t, out, "Inspect the BGP protocol engine.")
	assert.Contains(t, out, "  One subtree per session.")
	assert.Contains(t, out, "  Each answers over the negotiated families.")
	assert.Contains(t, out, "Show one row per route. The row carries the next hop.")
}

// TestPageWithoutHelpPrintsNoBodyBlock pins the empty-is-a-floor rule: a
// command nobody wrote an explanation for prints its summary alone.
func TestPageWithoutHelpPrintsNoBodyBlock(t *testing.T) {
	p := Page{Command: "ze bgp", Summary: "Inspect the BGP protocol engine."}

	var buf bytes.Buffer
	p.WriteTo(&buf, false)

	assert.Equal(t, "ze bgp - Inspect the BGP protocol engine.\n\n", buf.String())
}

func TestEntryWidth(t *testing.T) {
	entries := []HelpEntry{
		{Name: "short", Desc: "x"},
		{Name: "a-longer-command-name", Desc: "y"},
	}
	w := entryWidth(entries)
	assert.Equal(t, 21, w)

	small := []HelpEntry{{Name: "hi", Desc: "x"}}
	w = entryWidth(small)
	assert.Equal(t, 16, w)
}

func TestHighlightArgsUnclosed(t *testing.T) {
	got := highlightArgs(true, "ze test <file")
	assert.Contains(t, got, "<file")
	assert.NotContains(t, got, styleArg)

	got = highlightArgs(true, "ze test [opt")
	assert.Contains(t, got, "[opt")
	assert.NotContains(t, got, styleArg)
}

func TestHighlightArgsMultiple(t *testing.T) {
	got := highlightArgs(true, "ze <cmd> [opt] <file>")
	assert.Equal(t, 3, strings.Count(got, styleArg))
	assert.Equal(t, 3, strings.Count(got, colorReset))
}

func TestExamplesSection(t *testing.T) {
	p := Page{
		Command:  "ze test",
		Summary:  "test",
		Examples: []string{"ze test run", "ze test --all"},
	}
	var buf bytes.Buffer
	p.WriteTo(&buf, false)
	out := buf.String()
	assert.Contains(t, out, "Examples:")
	assert.Contains(t, out, "  ze test run\n")
	assert.Contains(t, out, "  ze test --all\n")
}

func TestNoExamplesNoSection(t *testing.T) {
	p := Page{
		Command: "ze test",
		Summary: "test",
	}
	var buf bytes.Buffer
	p.WriteTo(&buf, false)
	assert.NotContains(t, buf.String(), "Examples:")
}

// TestOutputMatchesLayout verifies the overall layout structure.
//
// VALIDATES: AC-7: output layout matches expected format.
// PREVENTS: Structural regressions.
func TestOutputMatchesLayout(t *testing.T) {
	p := Page{
		Command: "ze bgp",
		Summary: "BGP protocol tools",
		Usage:   []string{"ze bgp <command> [options]"},
		Sections: []HelpSection{
			{Title: "Commands", Entries: []HelpEntry{
				{Name: "decode <hex>", Desc: "Decode BGP message from hex to JSON"},
			}},
		},
		Examples: []string{"ze bgp decode update <hex>"},
	}

	var buf bytes.Buffer
	p.WriteTo(&buf, false)
	lines := strings.Split(buf.String(), "\n")

	// First line: "command - summary"
	assert.Equal(t, "ze bgp - BGP protocol tools", lines[0])
	// Blank line after header
	assert.Equal(t, "", lines[1])
	// Usage header
	assert.Equal(t, "Usage:", lines[2])
}
