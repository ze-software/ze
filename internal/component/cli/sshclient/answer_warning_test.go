package sshclient

import (
	"strings"
	"testing"
)

// TestReadAnswerFrameReadsAWarningTerminator checks that the exec client reads
// a terminator spelled `wrn` as the end of the answer and as a warning. The
// method: the frame the SSH server writes for a warning Response (a document
// head and a warning terminator counting its one record) is read, and the
// verdict, the count and the error answerError derives are checked.
//
// VALIDATES: journal row 2026-10-20 (silent-fall-through) on the exec channel:
// a warning arrives as warning, never as done, and is not a failure.
// PREVENTS: the reader skipping the warning terminator as a line that does not
// end the answer, which reads a completed answer as truncated.
func TestReadAnswerFrameReadsAWarningTerminator(t *testing.T) {
	frame := strings.Join([]string{
		"top doc 0: 0:",
		"wrn 1 0 0:",
		"",
	}, "\n")

	answer, text := readAnswerFrame(strings.NewReader(frame))

	if answer.Verdict != "warning" {
		t.Errorf("the frame reports verdict %q, want warning", answer.Verdict)
	}
	if answer.Count != 1 {
		t.Errorf("the frame reports count %d, want the terminator's 1", answer.Count)
	}
	if text != "" {
		t.Errorf("every line parsed as an answer line, so none is operator text; got %q", text)
	}
	if err := answerError(answer, text, nil, nil); err != nil {
		t.Errorf("a warning is an outcome the command completed with, not a failure: %v", err)
	}
}
