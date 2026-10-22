// Design: docs/architecture/exabgp-bridge.md -- the commands a 4.2 or 5.0 script writes
// Overview: bridge_command.go -- Translator.Line, which resolves the escapes under test

package bridge

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// TestLineEscapesResolved verifies that a backslash escape in a route line is
// read as ExaBGP reads it before splitting the line into words.
//
// VALIDATES: 4.2 and 5.0 run a route line through `unescape` before
// tokenising it (configuration/core/format.py, tokens: `line =
// unescape(letters)`, reached through Tokeniser.set_api), and the pin keeps it
// for the lines its configuration lexer reads ("Resolve the backslash escapes
// of a line, as the legacy tokeniser does before splitting it", unescape in
// configuration/grammar/lexer.py at 685f772e). A neighbor control line is
// not one of them (TestLineNeighborControlEscapesRaw). `\n`, `\r` and `\t` become
// characters in SPACES, so they end a word: 5.0 sent MED 100 for `med 100\n
// nlri ...` in 44 recorded commands (testdata/exabgp-old/commands-5.0.json.gz).
// Any other escaped character is itself, `\uXXXX` is the character it names,
// and a backslash ending the line escapes the space the pin adds before the
// command's terminator (read_command, configuration/grammar/read.py), so 5.0
// sent `announce route 10.0.0.0/8 next-hop self \` as the route alone.
// PREVENTS: the bridge handing `100\n` to ze as the MED, which ze refuses.
func TestLineEscapesResolved(t *testing.T) {
	for _, tc := range []struct {
		line    string
		contain string
	}{
		{`announce route 10.0.0.0/24 next-hop 1.2.3.4 med 100\n`, " med 100 "},
		{`announce attributes next-hop 1.2.3.4 med 100\n nlri 10.0.1.0/24`, " med 100 "},
		{`announce route 10.0.0.0/24 next-hop 1.2.3.4\tmed\r200`, " med 200 "},
		{`announce route 10.0.0.0/24 next-hop self\n`, " nhop self "},
		{`announce route 10.0.0.0/24 next-hop 1.2.3.4 m\ed 300`, " med 300 "},
		{`announce route 10.0.0.0/24 next-hop 1.2.3.4 \u006ded 400`, " med 400 "},
		{`announce route 10.0.0.0/24 next-hop self \`, " nhop self "},
		{`announce route 10.0.0.0/24 next-hop self \ med 500`, " med 500 "},
	} {
		translation, err := TranslateLine(tc.line)
		if err != nil {
			t.Errorf("%q: %v", tc.line, err)
			continue
		}
		if len(translation.Commands) == 0 {
			t.Errorf("%q: no command", tc.line)
			continue
		}
		text := translation.Commands[0].Text + " "
		if !strings.Contains(text, tc.contain) {
			t.Errorf("%q became %q, want %q", tc.line, text, tc.contain)
		}
		if strings.Contains(text, `\`) {
			t.Errorf("%q became %q, which still carries a backslash", tc.line, text)
		}
	}
}

// TestLineBadUnicodeEscapeRefused verifies that a `\u` escape with fewer than
// four hexadecimal digits refuses the line, as the pin's unescape refuses it
// ("unicode escape \u... needs 4 hexadecimal digits", "is not hexadecimal"),
// rather than reaching ze as some other word.
func TestLineBadUnicodeEscapeRefused(t *testing.T) {
	for _, line := range []string{
		`announce route 10.0.0.0/24 next-hop 1.2.3.4 med 1\u00`,
		`announce route 10.0.0.0/24 next-hop 1.2.3.4 med 1\u00zz`,
	} {
		_, err := TranslateLine(line)
		if err == nil {
			t.Errorf("%q: accepted, want refused", line)
			continue
		}
		if !errors.Is(err, errBadEscape) {
			t.Errorf("%q: %v, want errBadEscape", line, err)
		}
		if !strings.Contains(err.Error(), strconv.Quote(line)) {
			t.Errorf("%q: the refusal %q does not quote the line", line, err)
		}
		// The refusal says why a backslash matters, and both ways to fix it.
		for _, part := range []string{"the start of an escape", `A`, `write \\ where you meant a backslash`} {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%q: the refusal %q does not say %q", line, err, part)
			}
		}
	}
}

// TestLineNeighborControlEscapesRaw verifies that a neighbor control line
// reaches its converter with its backslashes as the script wrote them.
//
// VALIDATES: the pin resolves escapes only on the lines its configuration
// lexer reads (route, flow, vpls and attribute commands, through lex_command
// and read_command in configuration/grammar/lexer.py at 685f772e). Its
// teardown and disable read the raw line with shlex.split, where a backslash
// inside double quotes stays a backslash unless it escapes `"` or `\`. The
// test compares what Line answers with what convertDisable and convertTeardown
// answer for the raw words, so a resolved escape shows as a different
// command. Where shlex reads `\\` inside double quotes as one backslash and
// the converter writes the word as text, the test also checks the backslash
// reaches ze. A single `\t` there is shlex's to read, and a teardown text is
// encoded for the wire, so those cases assert the comparison alone.
// PREVENTS: `disable "a\tb"` arriving at ze as a reason holding a tab, and a
// teardown text losing the backslash the operator wrote.
func TestLineNeighborControlEscapesRaw(t *testing.T) {
	for _, tc := range []struct {
		line    string
		rest    string
		convert func(selector, rest string) (Translation, bool, error)
		contain string
	}{
		{`neighbor 1.2.3.4 disable "a\tb"`, `disable "a\tb"`, convertDisable, ""},
		{`neighbor 1.2.3.4 disable "a\\tb"`, `disable "a\\tb"`, convertDisable, `a\tb`},
		{`neighbor 1.2.3.4 teardown 6 2 "shut\\down"`, `teardown 6 2 "shut\\down"`, convertTeardown, `notify 6 2 hex 09736875745c646f776e`},
		{`neighbor 1.2.3.4 teardown 6 2 "a\tb"`, `teardown 6 2 "a\tb"`, convertTeardown, `notify 6 2 hex 04615c7462`},
	} {
		got, err := TranslateLine(tc.line)
		if err != nil {
			t.Errorf("%q: %v", tc.line, err)
			continue
		}
		want, handled, err := tc.convert("1.2.3.4", tc.rest)
		if !handled {
			t.Errorf("%q: the converter did not handle %q", tc.line, tc.rest)
			continue
		}
		if err != nil {
			t.Errorf("%q: the converter refused %q: %v", tc.line, tc.rest, err)
			continue
		}
		if len(got.Commands) != 1 {
			t.Errorf("%q became %d commands, want 1", tc.line, len(got.Commands))
			continue
		}
		if len(want.Commands) != 1 {
			t.Errorf("%q: the converter wrote %d commands, want 1", tc.rest, len(want.Commands))
			continue
		}
		if got.Commands[0].Text != want.Commands[0].Text {
			t.Errorf("%q became %q, want %q", tc.line, got.Commands[0].Text, want.Commands[0].Text)
		}
		if tc.contain == "" {
			continue
		}
		if !strings.Contains(got.Commands[0].Text, tc.contain) {
			t.Errorf("%q became %q, which lost %q", tc.line, got.Commands[0].Text, tc.contain)
		}
	}
}
