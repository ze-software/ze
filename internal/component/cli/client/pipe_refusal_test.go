// Detail: answer.go -- daemonOutput, which decides where a streamed answer goes

package client

import (
	"io"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	cmd "github.com/ze-software/ze/internal/component/command"
)

// TestARefusedPipeChainExitsNonZero drives the surface a script reads: one
// command through the client, answered by a daemon that refused the operator's
// pipe chain after running the command.
//
// VALIDATES: the refusal reaches stderr and the process exits 1, which is what
// every other pipe surface already does (runPipe in cmd/ze/ze_core_pipe.go,
// emitLocalResult in main.go, leroot.go).
// PREVENTS:  the refusal arriving on stdout as if it were the formatted answer,
// with exit 0, so a script collects the diagnostic as data and reads success
// (plan/journal/silent-fall-through.md, 2026-09-05).
func TestARefusedPipeChainExitsNonZero(t *testing.T) {
	const refusal = cmd.PipeErrorPrefix + "count needs rows and this answer has none"

	var sent string
	client := &cliClient{stream: func(_ sshclient.Credentials, command string, body io.Writer) (sshclient.Answer, error) {
		sent = command
		_, err := io.WriteString(body, refusal+"\n")
		return sshclient.Answer{}, err
	}}

	var code int
	var stdout string
	stderr := captureOutput(t, true, func() {
		stdout = captureOutput(t, false, func() {
			code = client.Execute("show bgp peer list | count", "")
		})
	})

	if !strings.Contains(sent, "| count") {
		t.Fatalf("the daemon was sent %q, want the operator's chain", sent)
	}
	if code != 1 {
		t.Errorf("a refused chain exited %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout carried %q, want nothing: a refusal is not an answer", stdout)
	}
	if !strings.Contains(stderr, refusal) {
		t.Errorf("stderr carried %q, want the refusal", stderr)
	}
}

// TestAStreamedAnswerStillReachesStdout holds the other side of the same
// decision: an answer that is not a refusal is unchanged by the reading, and
// an answer shorter than the refusal prefix still arrives.
func TestAStreamedAnswerStillReachesStdout(t *testing.T) {
	for _, answer := range []string{"3", "ze 26.09.20"} {
		t.Run(answer, func(t *testing.T) {
			client := &cliClient{stream: func(_ sshclient.Credentials, _ string, body io.Writer) (sshclient.Answer, error) {
				_, err := io.WriteString(body, answer)
				return sshclient.Answer{}, err
			}}

			var code int
			stdout := captureOutput(t, false, func() {
				code = client.Execute("show bgp peer list | count", "")
			})

			if code != 0 {
				t.Errorf("an answer exited %d, want 0", code)
			}
			if stdout != answer+"\n" {
				t.Errorf("the operator saw %q, want %q", stdout, answer+"\n")
			}
		})
	}
}
