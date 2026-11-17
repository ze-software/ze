package sshclient

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReadAnswerFrameMarksANotUnderstoodAnswer checks that the exec client
// tells the daemon's "this names no command" apart from a command that ran and
// failed. The method: the two frames the SSH server writes for the two
// failures (writeExecFailure, internal/component/ssh/answer.go) are read, the
// operator's plain line first and the frame after it, as the server writes
// them.
//
// VALIDATES: spec-plugin-external-yang-at-startup AC-36: `ze <verb> <path>`
// for a path the binary does not hold reads the running ze's not-understood
// answer as "the ze holds no such command either", and refuses the path as
// unknown.
// PREVENTS: a failed command read as an unknown one, which would replace the
// daemon's refusal of a value with "unknown command".
func TestReadAnswerFrameMarksANotUnderstoodAnswer(t *testing.T) {
	nay := string(rpc.AppendAnswerNotUnderstood(nil, rpc.AnswerNoID, "", "unknown command"))
	answer, text := readAnswerFrame(strings.NewReader("error: unknown command\n" + nay + "\n"))
	if !answer.NotUnderstood {
		t.Errorf("a nay answer must read as not understood: %+v", answer)
	}
	if answer.Verdict != rpc.VerdictError {
		t.Errorf("a nay answer is a failure: verdict %q, want %q", answer.Verdict, rpc.VerdictError)
	}
	if answer.Message != "unknown command" {
		t.Errorf("the daemon's text is the message: got %q", answer.Message)
	}
	if err := answerError(answer, text, nil, nil); err == nil {
		t.Errorf("a nay answer must be an error")
	}

	failed := strings.Join([]string{
		"error: name accepts 1..16 characters",
		"top doc 0: 0:",
		string(rpc.AppendAnswerTerminator(nil, rpc.AnswerNoID, 0, 0, "name accepts 1..16 characters")),
		"",
	}, "\n")
	answer, _ = readAnswerFrame(strings.NewReader(failed))
	if answer.NotUnderstood {
		t.Errorf("a command that ran and failed is understood: %+v", answer)
	}
	if answer.Verdict != rpc.VerdictError {
		t.Errorf("the failed command's frame must read as an error: verdict %q", answer.Verdict)
	}
}
