// Design: docs/architecture/api/ipc_protocol.md -- SSH warning verdicts.
// Related: answer_progress_test.go -- completed failed output and transcripts.

package client

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCLIClientStatusOnlyWarning reaches the real SSH client through one-shot
// execution. A warning without diagnostic text must remain visible, without
// changing its successful exit, body, transcript or original answer metadata.
func TestCLIClientStatusOnlyWarning(t *testing.T) {
	for _, count := range []uint64{0, 2, 259} {
		for _, outcome := range []string{"healthy", "explicit warning", "status-only warning"} {
			t.Run(fmt.Sprintf("%s/%d", outcome, count), func(t *testing.T) {
				dataHome := configureOneShotTranscript(t, true)
				var body strings.Builder
				for ordinal := range count {
					fmt.Fprintf(&body, "{\"ordinal\":%d}\n", ordinal)
				}
				warning := ""
				if outcome == "explicit warning" {
					warning = "cleanup retained one owner"
				}
				creds := cliStatusWarningServer(t, body.String(), outcome, warning, count)
				client := newCLIClient(creds)
				var original sshclient.Answer
				client.stream = func(creds sshclient.Credentials, command string, output io.Writer) (sshclient.Answer, error) {
					answer, err := sshclient.ExecCommandStream(creds, command, output)
					original = answer
					return answer, err
				}
				var code int
				var stdout string
				stderr := captureOutput(t, true, func() {
					stdout = captureOutput(t, false, func() {
						code = client.runOne("show cleanup progress", "")
					})
				})
				if code != 0 {
					t.Errorf("completed warning exit = %d, want 0", code)
				}
				wantBody := body.String()
				if count == 0 {
					wantBody = "OK\n"
				}
				if stdout != wantBody {
					t.Errorf("stdout = %q, want %q", stdout, wantBody)
				}
				wantDiagnostic := ""
				wantVerdict := rpc.VerdictDone
				if outcome != "healthy" {
					wantVerdict = rpc.VerdictWarning
					wantDiagnostic = "Warning\n"
				}
				if warning != "" {
					wantDiagnostic = warning + "\n"
				}
				if stderr != wantDiagnostic {
					t.Errorf("stderr = %q, want %q", stderr, wantDiagnostic)
				}
				if original.Verdict != wantVerdict {
					t.Errorf("original verdict = %q, want %q", original.Verdict, wantVerdict)
				}
				if original.Warnings != warning {
					t.Errorf("original warnings = %q, want %q", original.Warnings, warning)
				}
				if original.Message != "" {
					t.Errorf("invented original failure = %q", original.Message)
				}
				if original.Count != count {
					t.Errorf("original count = %d, want %d", original.Count, count)
				}
				if original.Faults != 0 {
					t.Errorf("invented original faults = %d", original.Faults)
				}
				assertOneShotTranscript(t, dataHome, "show cleanup progress", strings.TrimSuffix(body.String(), "\n"))
			})
		}
	}
}

// cliStatusWarningServer owns one signed SSH session and joins it at cleanup.
// Its deadline bounds malformed clients; no polling or daemon is involved.
func cliStatusWarningServer(t *testing.T, body, outcome, warning string, count uint64) sshclient.Credentials {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cliStatusWarningServe(listener, signer, body, outcome, warning, count)
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close warning listener: %v", err)
		}
		if err := <-done; err != nil {
			t.Errorf("warning SSH fixture: %v", err)
		}
	})
	return sshclient.Credentials{Host: "127.0.0.1", Port: port, Username: "operator", Auth: "warning-proof"}
}

func cliStatusWarningServe(listener net.Listener, signer ssh.Signer, body, outcome, warning string, count uint64) error {
	connection, err := listener.Accept()
	if err != nil {
		return err
	}
	defer connection.Close() //nolint:errcheck // Owned test connection cleanup.
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	config := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() != "operator" {
			return nil, errors.New("unexpected warning fixture user")
		}
		if string(password) != "warning-proof" {
			return nil, errors.New("unexpected warning fixture password")
		}
		return &ssh.Permissions{}, nil
	}}
	config.AddHostKey(signer)
	server, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return err
	}
	defer server.Close() //nolint:errcheck // Owned test server cleanup.
	go ssh.DiscardRequests(requests)
	proposed, ok := <-channels
	if !ok {
		return errors.New("warning fixture received no SSH session")
	}
	if proposed.ChannelType() != "session" {
		return fmt.Errorf("warning fixture received channel %q", proposed.ChannelType())
	}
	channel, commands, err := proposed.Accept()
	if err != nil {
		return err
	}
	defer channel.Close() //nolint:errcheck // Owned test channel cleanup.
	request, ok := <-commands
	if !ok {
		return errors.New("warning fixture received no command")
	}
	if request.Type != "exec" {
		return fmt.Errorf("warning fixture received request %q", request.Type)
	}
	var input struct{ Command string }
	if err := ssh.Unmarshal(request.Payload, &input); err != nil {
		return err
	}
	if input.Command != "show cleanup progress" {
		return fmt.Errorf("warning fixture received command %q", input.Command)
	}
	if err := request.Reply(true, nil); err != nil {
		return err
	}
	if _, err := io.WriteString(channel, body); err != nil {
		return err
	}
	if warning != "" {
		if _, err := fmt.Fprintln(channel.Stderr(), warning); err != nil {
			return err
		}
	}
	frame := rpc.AppendAnswerHead(nil, rpc.AnswerNoID, rpc.AnswerTypeDocument, "", nil)
	frame = append(frame, '\n')
	if outcome == "healthy" {
		frame = rpc.AppendAnswerTerminator(frame, rpc.AnswerNoID, count, 0, "")
	} else {
		frame = rpc.AppendAnswerWarning(frame, rpc.AnswerNoID, count, 0, "")
	}
	frame = append(frame, '\n')
	if _, err := channel.Stderr().Write(frame); err != nil {
		return err
	}
	_, err = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
	return err
}
