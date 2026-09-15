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
