// Design: docs/architecture/cli/error-surface.md -- preserve the producer's final fix.
// Related: answer_progress_test.go -- failed command output and transcripts.

package client

import (
	"io"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCLIClientOutputFailureKeepsTheProducerFix closes a real stdout pipe before
// the held answer is flushed. Both causes must remain visible, with the original
// producer's complete error and fix last; a healthy sink adds no output failure.
func TestCLIClientOutputFailureKeepsTheProducerFix(t *testing.T) {
	for _, test := range []struct {
		name   string
		closed bool
	}{
		{name: "closed destination", closed: true},
		{name: "healthy destination", closed: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dataHome := configureOneShotTranscript(t, true)
			producerFailure := opererr.New(
				"cleanup incomplete", "one retained resource still needs review",
				"ze cli -c \"show cleanup progress\"",
			)
			var closedWriter *os.File
			if test.closed {
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := writer.Close(); err != nil {
						t.Error(err)
					}
				})
				if err := reader.Close(); err != nil {
					t.Fatal(err)
				}
				closedWriter = writer
			}
			calls := 0
			client := &cliClient{
				creds: sshclient.Credentials{Host: "127.0.0.1", Port: "1"},
				stream: func(_ sshclient.Credentials, command string, output io.Writer) (sshclient.Answer, error) {
					calls++
					if command != "show cleanup progress" {
						t.Errorf("transport command = %q", command)
					}
					// This is shorter than the refusal prefix, so no destination
					// write happens until execute closes the failed answer.
					if _, err := io.WriteString(output, "1"); err != nil {
						t.Errorf("short answer reached the destination before close: %v", err)
						return sshclient.Answer{}, err
					}
					return sshclient.Answer{Verdict: rpc.VerdictAborted, Count: 1, Faults: 1, Warnings: "cleanup retained one owner"}, producerFailure
				},
			}
			var code int
			var stdout string
			stderr := captureOutput(t, true, func() {
				if !test.closed {
					stdout = captureOutput(t, false, func() {
						code = client.runOne("show cleanup progress", "")
					})
					return
				}
				beforeOutput := os.Stdout
				os.Stdout = closedWriter
				defer func() { os.Stdout = beforeOutput }()
				code = client.runOne("show cleanup progress", "")
			})
			if code != 1 {
				t.Errorf("failed command exit = %d, want 1", code)
			}
			if calls != 1 {
				t.Errorf("command executions = %d, want exactly one", calls)
			}
			if !strings.HasPrefix(stderr, "cleanup retained one owner\n") {
				t.Errorf("original warning lost: %q", stderr)
			}
			if !strings.HasSuffix(stderr, "error: "+producerFailure.Error()+"\n") {
				t.Errorf("original failure and final fix changed or followed by other text: %q", stderr)
			}
			const evidence = "command output could not be completed: "
			if test.closed {
				if !strings.Contains(stderr, evidence) {
					t.Errorf("output completion evidence lost: %q", stderr)
				}
				if !strings.Contains(stderr, syscall.EPIPE.Error()) {
					t.Errorf("actual broken-pipe cause lost: %q", stderr)
				}
			} else {
				if strings.Contains(stderr, evidence) {
					t.Errorf("healthy destination acquired an output failure: %q", stderr)
				}
				if stdout != "1\n" {
					t.Errorf("completed stdout = %q, want one retained row", stdout)
				}
			}
			assertOneShotTranscript(t, dataHome, "show cleanup progress", "1")
		})
	}
}
