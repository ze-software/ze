// VALIDATES: `le spec design away` and `return` are Thomas's terminal's
// alone, away needs an until date of today or later, and the record lapses on
// its own after that date
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-27, C-4).
// PREVENTS: an agent declaring Thomas away to start a spec without him, and
// an absence with no end.
package specdesign_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/le/human/humantest"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// awayRecord is the away record of the fixture checkout.
const awayRecord = "tmp/spec-design/away.md"

// runDesign runs one `le spec design` action and answers its rendered answer
// and stderr, and its exit code.
func runDesign(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var answer any
	var code int
	stderr := captureStderr(t, func() {
		answer, code = specdesign.Answer(args)
	})
	if answer == nil {
		return stderr, code
	}
	return answerText(t, answer) + stderr, code
}

// TestAwayRequiresTerminalAndUntil is AC-27. Method: with piped streams, away
// and return each exit 2 with the owner-terminal refusal and write nothing;
// from a real terminal, away without until, with a date that is not one, and
// with yesterday each refuse and write nothing; away until today writes the
// record with From, Until and the words, and return removes it. A record
// whose until is past reads as no absence, and one that is unreadable is an
// error, never an absence.
func TestAwayRequiresTerminalAndUntil(t *testing.T) {
	root := widgetRoot(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "away-session")
	today := time.Now().Format(time.DateOnly)
	yesterday := time.Now().AddDate(0, 0, -1).Format(time.DateOnly)

	humantest.Streams(t, "piped-both")
	for _, args := range [][]string{{"away", "until", today, "words", "away today"}, {"return"}} {
		output, code := runDesign(t, append([]string{}, args...)...)
		if code != 2 {
			t.Errorf("%s without a terminal answered %d, want 2:\n%s", args[0], code, output)
		}
		requireContains(t, args[0]+" without a terminal", output,
			"owner authorization requires interactive stdin and stdout", "spec design "+args[0])
		requireAbsent(t, root, "tmp/spec-design")
	}

	humantest.Streams(t, "terminal")
	for _, args := range [][]string{
		{"away", "words", "away"},
		{"away", "until", "next week", "words", "away"},
		{"away", "until", yesterday, "words", "away"},
	} {
		output, code := runDesign(t, args...)
		if code == 0 {
			t.Errorf("%q was admitted:\n%s", strings.Join(args, " "), output)
		}
		requireAbsent(t, root, awayRecord)
	}

	output, code := runDesign(t, "away", "until", today, "words", "away until tonight")
	if code != 0 {
		t.Fatalf("away until today answered %d, want 0:\n%s", code, output)
	}
	requireContains(t, "the away record", readFixture(t, root, awayRecord),
		"From: "+today+"\n", "Until: "+today+"\n", "Owner words: away until tonight\n")
	away, found, err := specdesign.ActiveAway(root, time.Now())
	if err != nil || !found || away.Until != today {
		t.Errorf("ActiveAway after away: %+v, found %v, err %v", away, found, err)
	}

	output, code = runDesign(t, "return")
	if code != 0 {
		t.Fatalf("return answered %d, want 0:\n%s", code, output)
	}
	requireAbsent(t, root, awayRecord)

	writeTree(t, root, map[string]string{awayRecord: "From: 2026-01-01\nUntil: " + yesterday + "\nOwner words: gone\n"})
	if _, found, err := specdesign.ActiveAway(root, time.Now()); err != nil || found {
		t.Errorf("a record past its until reads as found %v, err %v; want lapsed", found, err)
	}
	writeTree(t, root, map[string]string{awayRecord: "From: 2026-01-01\nOwner words: gone\n"})
	if _, found, err := specdesign.ActiveAway(root, time.Now()); err == nil || found {
		t.Errorf("a record with no Until reads as found %v, err %v; want an error", found, err)
	}
}
