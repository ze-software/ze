// Design: docs/architecture/api/commands.md -- retain completed command progress.

package client

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	cmd "github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCLIClientFailureKeepsCompletedProgress drives the command entry point
// with a failed transport answer. Short held output, diagnostics and transcripts
// must survive; an empty failure must never print the success placeholder.
func TestCLIClientFailureKeepsCompletedProgress(t *testing.T) {
	for _, test := range []struct {
		name, body, stdout, diagnostic string
	}{
		{"short progress", "1", "1\n", ""},
		{"long progress", "completed cleanup rows", "completed cleanup rows\n", ""},
		{"empty refusal", "", "", ""},
		{"pipe refusal", cmd.PipeErrorPrefix + "cleanup count unavailable", "", cmd.PipeErrorPrefix + "cleanup count unavailable\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dataHome := configureOneShotTranscript(t, true)
			client := &cliClient{
				creds: sshclient.Credentials{Host: "127.0.0.1", Port: "1"},
				stream: func(_ sshclient.Credentials, command string, body io.Writer) (sshclient.Answer, error) {
					if command != "show cleanup progress" {
						t.Errorf("transport command = %q", command)
					}
					if _, err := io.WriteString(body, test.body); err != nil {
						return sshclient.Answer{}, err
					}
					return sshclient.Answer{Verdict: rpc.VerdictAborted, Count: 1, Faults: 1, Warnings: "cleanup retained one owner"}, errors.New("cleanup incomplete")
				},
			}
			var code int
			var stdout string
			stderr := captureOutput(t, true, func() {
				stdout = captureOutput(t, false, func() {
					code = client.runOne("show cleanup progress", "")
				})
			})
			if code != 1 {
				t.Errorf("failed command status = %d, want 1", code)
			}
			if stdout != test.stdout {
				t.Errorf("stdout = %q, want %q", stdout, test.stdout)
			}
			if !strings.Contains(stderr, "cleanup retained one owner\n") {
				t.Errorf("warning lost: %q", stderr)
			}
			if !strings.Contains(stderr, "error: cleanup incomplete\n") {
				t.Errorf("failure lost: %q", stderr)
			}
			if !strings.Contains(stderr, test.diagnostic) {
				t.Errorf("pipe refusal lost: %q", stderr)
			}
			assertOneShotTranscript(t, dataHome, "show cleanup progress", test.body)
		})
	}
}
