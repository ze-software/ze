// Design: docs/contributing/committing.md -- the message contract
// Related: input.go -- Message, the one producer of a commit message.
package commit

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestMessageKeepsTrailerLinesWhole proves A-2: a trailer line is appended
// after the wrapped body, separated by a blank line, and is never wrapped
// however long the owner's words are.
func TestMessageKeepsTrailerLinesWhole(t *testing.T) {
	reason := strings.Repeat("the owner said yes ", 6)
	trailer := "RFC-approved: pkg.TestThing: " + strings.TrimSpace(reason)
	if len(trailer) <= messageWidth {
		t.Fatalf("the fixture trailer is %d characters, which the wrap would keep anyway", len(trailer))
	}

	message, err := Message("a subject", []string{"a body " + strings.Repeat("word ", 30)}, []string{trailer})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(message, "\n"), "\n")
	last := lines[len(lines)-1]
	if last != trailer {
		t.Fatalf("the last line is %q, want the whole trailer", last)
	}
	if lines[len(lines)-2] != "" {
		t.Fatalf("no blank line separates the body from the trailer:\n%s", message)
	}
	for _, line := range lines[:len(lines)-2] {
		if len(line) > messageWidth {
			t.Fatalf("the body line %q escaped the wrap", line)
		}
	}

	if without, err := Message("a subject", []string{"a body"}, nil); err != nil || strings.Contains(without, "RFC-approved") {
		t.Fatalf("no trailer gave %q, %v", without, err)
	}
}

// TestMessageRefusesAHandWrittenApprovalTrailer proves the record cannot be
// forged from the body: a body line that starts with the trailer key is
// refused, the error names the line, and it says who writes the trailer. A
// subject starting with the key is refused for the same reason, because
// git prints it in the same message the gate reads back.
func TestMessageRefusesAHandWrittenApprovalTrailer(t *testing.T) {
	forged := rfc.ApprovalTrailer + "pkg.TestThing: the author says the owner said yes"
	_, err := Message("a subject", []string{"a body", "", "  " + forged}, nil)
	if err == nil {
		t.Fatal("a hand-written approval trailer in the body was accepted")
	}
	for _, want := range []string{forged, "./le rfc approve"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal %q does not carry %q", err.Error(), want)
		}
	}
	if _, err := Message(forged, []string{"a body"}, nil); err == nil {
		t.Fatal("a subject written as an approval trailer was accepted")
	}
	if _, err := Message("a subject", []string{"the RFC-approved: key inside a line is prose"}, nil); err != nil {
		t.Fatalf("the key inside a body line was refused: %v", err)
	}
	// The wrap can move a key that sits past column 72 to the start of the
	// next emitted line, which is where the record is read from.
	prefix := strings.Repeat("word ", 14)
	if _, err := Message("a subject", []string{prefix + forged}, nil); err == nil {
		t.Fatal("an approval trailer the wrap moved to the start of a line was accepted")
	}
}

// TestMessageRefusesDeclaredTrailerInBody proves no declared trailer can be
// forged from prose: a body line starting with any key the derivation reads
// back is refused, naming the one route that writes it. A hand-written
// `Owner-attested:` would otherwise answer an obligation the owner never saw.
// VALIDATES: AC-13, AC-14, AC-15 (Security Review Checklist, trailer forgery).
func TestMessageRefusesDeclaredTrailerInBody(t *testing.T) {
	for _, declared := range declaredTrailers {
		forged := declared.Key + "x: y"
		_, err := Message("a subject", []string{"a body", forged}, nil)
		if err == nil {
			t.Fatalf("a body line starting with %q was accepted", declared.Key)
		}
		if !strings.Contains(err.Error(), declared.WrittenBy) {
			t.Fatalf("the refusal %q does not name %q", err, declared.WrittenBy)
		}
		if _, err := Message(forged, nil, nil); err == nil {
			t.Fatalf("a subject starting with %q was accepted", declared.Key)
		}
	}
	for _, key := range []string{"Review-owed: ", "RFC-approval-owed: ", "Owner-attested: ", "Ledger-carried: "} {
		if _, err := Message("a subject", []string{key + "x: y"}, nil); err == nil {
			t.Fatalf("the spec's trailer key %q is not declared", key)
		}
	}
}

// TestMessageWritesOwedTrailers proves create records each admission it grants
// as a trailer in the spelling the derivation reads: review-override as
// `Review-owed: <stem>: <reason>`, rfc-change-ok as one
// `RFC-approval-owed: <package>.<Test>: <reason>` per unit.
// VALIDATES: AC-13, AC-14, AC-20.
func TestMessageWritesOwedTrailers(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "owed-trailer-fixture")
	t.Run("review-override", func(t *testing.T) {
		root := newCommitRepository(t)
		configureCommitAuthor(t, root)
		commitFixture(t, root, "the spec lands", map[string]string{owedSpecPath: specText("in-progress", "")}, nil)
		prepared, err := Create(root, &Options{Subject: "close the spec", Remove: []string{owedSpecPath},
			ReviewOverride: "no reviewer free", NoNews: "fixture: no operator-visible change", DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		assertTrailer(t, &prepared, "Review-owed: "+owedSpecStem+": no reviewer free")
	})
	t.Run("rfc-change-ok", func(t *testing.T) {
		root := approvalRepository(t)
		prepared, err := Create(root, &Options{Session: approvalSession, Subject: "change the tagged unit",
			Files: []string{approvalTagged}, NoTest: "the fixture carries tests only",
			RFCChangeOK: "owner away", DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		assertTrailer(t, &prepared, "RFC-approval-owed: "+approvalUnit+": owner away")
	})
}

func assertTrailer(t *testing.T, prepared *Prepared, trailer string) {
	t.Helper()
	if !strings.Contains(prepared.MessageText, "\n"+trailer+"\n") {
		t.Fatalf("the message does not carry %q as one line:\n%s", trailer, prepared.MessageText)
	}
}

// bodyLines answers the lines of a message between the blank line under the
// subject and the end, for a message that carries no trailer.
func bodyLines(t *testing.T, message string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(message, "\n"), "\n")
	if len(lines) < 3 || lines[1] != "" {
		t.Fatalf("the message has no body under a blank line:\n%s", message)
	}
	return lines[2:]
}

// TestMessagePutsAnOverLongWordOnItsOwnLine proves AC-9: a body word longer
// than the line (a URL here) is accepted rather than refused, sits on a line
// of its own with the indent of its input line, and the words around it wrap
// at 72 as before. Method: build the message and read each emitted line.
// VALIDATES: AC-9 (spec-agent-economy-2-scoped-test).
func TestMessagePutsAnOverLongWordOnItsOwnLine(t *testing.T) {
	url := "https://example.com/" + strings.Repeat("a", 70)
	if len(url) <= messageWidth {
		t.Fatalf("the fixture word is %d characters, which the wrap would keep anyway", len(url))
	}
	words := strings.TrimSpace(strings.Repeat("word ", 20))

	message, err := Message("a subject", []string{"see " + url + " " + words}, nil)
	if err != nil {
		t.Fatalf("a body holding one over-long word was refused: %v", err)
	}
	lines := bodyLines(t, message)
	if lines[0] != "see" || lines[1] != url {
		t.Fatalf("the over-long word does not sit alone after the words before it:\n%s", message)
	}
	if strings.Join(lines[2:], " ") != words {
		t.Fatalf("the words after the over-long one were changed:\n%s", message)
	}
	for _, line := range lines[2:] {
		if len(line) > messageWidth {
			t.Fatalf("the body line %q escaped the wrap", line)
		}
	}
	if len(lines) < 4 {
		t.Fatalf("the words after the over-long one did not wrap onto several lines:\n%s", message)
	}

	indented, err := Message("a subject", []string{"  indented " + url + " tail"}, nil)
	if err != nil {
		t.Fatalf("an indented over-long word was refused: %v", err)
	}
	if got := bodyLines(t, indented); len(got) != 3 || got[0] != "  indented" || got[1] != "  "+url || got[2] != "  tail" {
		t.Fatalf("the indented over-long word did not keep its indent on its own line: %q", got)
	}
}

// TestMessageOverLongWordAddsNoEmptyLine proves an over-long word that opens
// its input line, indented or not, starts the emitted line itself: the wrap
// does not first flush an empty or indent-only line above it. It also pins the
// boundary: a word of exactly 72 columns with its indent wraps as before, and
// 73 takes its own line.
// VALIDATES: AC-9 (spec-agent-economy-2-scoped-test), Boundary Tests "Body word width".
func TestMessageOverLongWordAddsNoEmptyLine(t *testing.T) {
	long := strings.Repeat("x", messageWidth+8)
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"first word", long + " tail", []string{long, "tail"}},
		{"first word, indented", "    " + long + " tail", []string{"    " + long, "    tail"}},
		{"alone", long, []string{long}},
		// A later input line, where no leading-empty trim can hide a flushed
		// empty line above the word.
		{"first word of a later line", "first line\n" + long, []string{"first line", long}},
		{"alone, indented", "  " + long, []string{"  " + long}},
		{"72 columns with its indent", "  " + strings.Repeat("y", messageWidth-2), []string{"  " + strings.Repeat("y", messageWidth-2)}},
		{"73 columns with its indent", "  " + strings.Repeat("z", messageWidth-1) + " tail", []string{"  " + strings.Repeat("z", messageWidth-1), "  tail"}},
		{"72 then a word", strings.Repeat("y", messageWidth) + " tail", []string{strings.Repeat("y", messageWidth), "tail"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message, err := Message("a subject", []string{tc.body}, nil)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			got := bodyLines(t, message)
			for _, line := range got {
				if strings.TrimSpace(line) == "" {
					t.Fatalf("an empty line was emitted: %q", got)
				}
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestMessageStillRefusesAnOverLongSubject proves what AC-9 keeps: the subject
// limit holds even for a subject that is one word (only a body word is
// exempt), and a body line starting with a declared trailer key is refused
// when it is an over-long word on its own line.
// VALIDATES: AC-9 survivors (spec-agent-economy-2-scoped-test).
func TestMessageStillRefusesAnOverLongSubject(t *testing.T) {
	word := strings.Repeat("s", messageWidth+1)
	_, err := Message(word, []string{"a body"}, nil)
	if err == nil {
		t.Fatal("a one-word subject over the limit was accepted")
	}
	if !strings.Contains(err.Error(), "1 over the 72 limit") {
		t.Fatalf("the refusal does not give the overage: %v", err)
	}
	if _, err := Message(strings.Repeat("s", messageWidth), []string{"a body"}, nil); err != nil {
		t.Fatalf("a subject of exactly %d characters was refused: %v", messageWidth, err)
	}

	// One word with no space, so the key opens the over-long word's own line.
	forged := strings.TrimSpace(rfc.ApprovalTrailer) + strings.Repeat("p", messageWidth)
	if _, err := Message("a subject", []string{"a body " + forged}, nil); err == nil {
		t.Fatal("an over-long hand-written trailer the wrap put on its own line was accepted")
	}
}

// TestHandWrittenNoNewsTrailerRefused proves the no-news admission cannot be
// forged from prose: a body line `No-news: <reason>` is refused by the
// hand-written trailer check, and the refusal names the no-news keyword, the
// one route that writes the trailer (AC-8, R-7).
// PREVENTS: a closure that skips the news-fragment gate by writing the
// trailer the gate would have admitted.
func TestHandWrittenNoNewsTrailerRefused(t *testing.T) {
	_, err := Message("plan: close x", []string{"a body", trailerNoNews + "tooling: no operator-visible change"}, nil)
	if err == nil {
		t.Fatal("a hand-written No-news: line was accepted")
	}
	if !strings.Contains(err.Error(), keywordNoNews+` "<reason>"`) {
		t.Fatalf("the refusal does not name the %s keyword: %v", keywordNoNews, err)
	}
}
