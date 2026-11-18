// Design: docs/architecture/api/ipc_protocol.md -- preserve completed SSH answers.

package sshclient

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSSHClientStreamKeepsCompletedProgress drives the public streaming client
// over a signed SSH connection. Partial warnings and late failures must retain
// the bytes, counts, warning lines and authoritative failure the server sent.
func TestSSHClientStreamKeepsCompletedProgress(t *testing.T) {
	for _, test := range []struct {
		name, body, warning, message, verdict string
		count, faults                         uint64
		status                                uint32
	}{
		{"partial warning", "1", "cleanup retained one owner", "", rpc.VerdictPartial, 1, 1, 1},
		{"failed progress", "1", "cleanup retained one owner", "cleanup incomplete", rpc.VerdictAborted, 1, 1, 1},
		{"multiline failed progress", "1", "cleanup retained one owner", "cleanup incomplete:\n  route still owned\n\nretry cleanup", rpc.VerdictAborted, 1, 1, 1},
		{"failure ending in space", "1", "cleanup retained one owner", "cleanup incomplete ", rpc.VerdictAborted, 1, 1, 1},
		{"failure ending in newline", "1", "cleanup retained one owner", "cleanup incomplete\n", rpc.VerdictAborted, 1, 1, 1},
		{"ordinary refusal", "", "", "command refused", rpc.VerdictError, 0, 0, 1},
		{"success warning", "1", "cleanup retained one owner", "", rpc.VerdictWarning, 1, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			creds := sshClientProgressServer(t, test.body, test.warning, test.message, test.count, test.faults, test.status)
			var body strings.Builder
			answer, err := ExecCommandStream(creds, "show cleanup progress", &body)
			if body.String() != test.body {
				t.Errorf("completed body = %q, want %q", body.String(), test.body)
			}
			if answer.Verdict != test.verdict {
				t.Errorf("verdict = %q, want %q", answer.Verdict, test.verdict)
			}
			if answer.Count != test.count {
				t.Errorf("count = %d, want %d", answer.Count, test.count)
			}
			if answer.Faults != test.faults {
				t.Errorf("faults = %d, want %d", answer.Faults, test.faults)
			}
			if answer.Warnings != test.warning {
				t.Errorf("warnings = %q, want %q", answer.Warnings, test.warning)
			}
			if answer.Message != test.message {
				t.Errorf("message = %q, want %q", answer.Message, test.message)
			}
			if test.message != "" {
				if err == nil {
					t.Fatal("failed frame returned success")
				}
				if err.Error() != test.message {
					t.Errorf("failure = %q, want %q", err, test.message)
				}
			} else if test.status != 0 {
				var exit *ssh.ExitError
				if !errors.As(err, &exit) {
					t.Fatalf("partial frame lost SSH status: %v", err)
				}
				if exit.ExitStatus() != int(test.status) {
					t.Errorf("SSH status = %d, want %d", exit.ExitStatus(), test.status)
				}
			} else if err != nil {
				t.Errorf("completed warning failed: %v", err)
			}
		})
	}
}

// TestSSHClientCollectedFailureKeepsCompletedOutput holds the collected sibling
// to the same contract: nonzero SSH status must not discard observed stdout.
func TestSSHClientCollectedFailureKeepsCompletedOutput(t *testing.T) {
	for _, body := range []string{"1", ""} {
		t.Run(fmt.Sprintf("body-%d", len(body)), func(t *testing.T) {
			count := uint64(0)
			if body != "" {
				count = 1
			}
			creds := sshClientProgressServer(t, body, "", "cleanup incomplete", count, 0, 1)
			output, err := ExecCommand(creds, "show cleanup progress")
			if output != body {
				t.Errorf("completed output = %q, want %q", output, body)
			}
			if err == nil {
				t.Fatal("failed frame returned success")
			}
			if err.Error() != "cleanup incomplete" {
				t.Errorf("failure = %q", err)
			}
		})
	}
	t.Run("original SSH status", func(t *testing.T) {
		creds := sshClientProgressServer(t, "1", "cleanup retained one owner", "", 1, 1, 1)
		output, err := ExecCommand(creds, "show cleanup progress")
		if output != "1" {
			t.Errorf("completed output = %q, want 1", output)
		}
		var exit *ssh.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("partial frame lost SSH status: %v", err)
		}
		if exit.ExitStatus() != 1 {
			t.Errorf("SSH status = %d, want 1", exit.ExitStatus())
		}
	})
}

// TestSSHClientFrameSeparatesOnlyTheStatedFailure checks the failure/warning
// split against writeExecError's exact suffix. Unrelated stderr remains error
// evidence and must not be relabeled as a warning by its position alone.
func TestSSHClientFrameSeparatesOnlyTheStatedFailure(t *testing.T) {
	for _, test := range []struct {
		name, text, warning, remaining, message string
	}{
		{"matching failure", "cleanup warning\nerror: cleanup incomplete", "cleanup warning", "error: cleanup incomplete", "cleanup incomplete"},
		{"different failure", "cleanup warning\nerror: another failure", "", "cleanup warning\nerror: another failure", "cleanup incomplete"},
		{"plain diagnostic", "unclassified daemon diagnostic", "", "unclassified daemon diagnostic", "cleanup incomplete"},
		{"multiline failure", "cleanup warning\nerror: cleanup incomplete:\n  route still owned\n\nretry cleanup", "cleanup warning", "error: cleanup incomplete:\n  route still owned\n\nretry cleanup", "cleanup incomplete:\n  route still owned\n\nretry cleanup"},
		{"failure ending in space", "cleanup warning\nerror: cleanup incomplete ", "cleanup warning", "error: cleanup incomplete ", "cleanup incomplete "},
		{"failure ending in newline", "cleanup warning\nerror: cleanup incomplete\n", "cleanup warning", "error: cleanup incomplete\n", "cleanup incomplete\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			frame := rpc.AppendAnswerTerminator(nil, rpc.AnswerNoID, 1, 1, test.message)
			answer, remaining := readAnswerFrame(strings.NewReader(test.text + "\n" + string(frame) + "\n"))
			if answer.Warnings != test.warning {
				t.Errorf("warnings = %q, want %q", answer.Warnings, test.warning)
			}
			if remaining != test.remaining {
				t.Errorf("failure evidence = %q, want %q", remaining, test.remaining)
			}
			err := answerError(answer, remaining, nil, nil)
			if err == nil {
				t.Fatal("failed frame returned success")
			}
			if err.Error() != test.message {
				t.Errorf("authoritative failure = %q", err)
			}
		})
	}
}

// sshClientProgressServer owns one bounded SSH session. The signed transport
// exercises the real clients; the scripted completion follows writeExecAnswer's
// stdout/stderr grammar, without claiming daemon authentication or admission.
func sshClientProgressServer(t *testing.T, body, warning, message string, count, faults uint64, status uint32) Credentials {
	t.Helper()
	hostKeyTestEnvironment(t)
	signer := hostKeyTestSigner(t)
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
		done <- sshClientProgressServe(listener, signer, body, warning, message, count, faults, status)
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close SSH listener: %v", err)
		}
		if err := <-done; err != nil {
			t.Errorf("SSH fixture: %v", err)
		}
	})
	return Credentials{Host: "127.0.0.1", Port: port, Username: "operator", Auth: "cleanup-proof"}
}

func sshClientProgressServe(listener net.Listener, signer ssh.Signer, body, warning, message string, count, faults uint64, status uint32) error {
	connection, err := listener.Accept()
	if err != nil {
		return err
	}
	defer connection.Close() //nolint:errcheck // owned fixture transport cleanup
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	config := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() != "operator" {
			return nil, errors.New("unexpected fixture user")
		}
		if string(password) != "cleanup-proof" {
			return nil, errors.New("unexpected fixture password")
		}
		return &ssh.Permissions{}, nil
	}}
	config.AddHostKey(signer)
	server, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return err
	}
	defer server.Close() //nolint:errcheck // owned fixture transport cleanup
	go ssh.DiscardRequests(requests)
	proposed, ok := <-channels
	if !ok {
		return errors.New("fixture received no SSH session")
	}
	if proposed.ChannelType() != "session" {
		return fmt.Errorf("fixture received channel %q", proposed.ChannelType())
	}
	channel, commands, err := proposed.Accept()
	if err != nil {
		return err
	}
	defer channel.Close() //nolint:errcheck // owned fixture channel cleanup
	request, ok := <-commands
	if !ok {
		return errors.New("fixture received no command")
	}
	if request.Type != "exec" {
		return fmt.Errorf("fixture received request %q", request.Type)
	}
	var input struct{ Command string }
	if err := ssh.Unmarshal(request.Payload, &input); err != nil {
		return err
	}
	if input.Command != "show cleanup progress" {
		return fmt.Errorf("fixture received command %q", input.Command)
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
	if message != "" {
		if _, err := fmt.Fprintln(channel.Stderr(), errorPrefix+message); err != nil {
			return err
		}
	}
	frame := rpc.AppendAnswerHead(nil, rpc.AnswerNoID, rpc.AnswerTypeDocument, "", nil)
	frame = append(frame, '\n')
	if message != "" {
		frame = rpc.AppendAnswerTerminator(frame, rpc.AnswerNoID, count, faults, message)
	} else if warning != "" {
		frame = rpc.AppendAnswerWarning(frame, rpc.AnswerNoID, count, faults, "")
	} else {
		frame = rpc.AppendAnswerTerminator(frame, rpc.AnswerNoID, count, faults, "")
	}
	frame = append(frame, '\n')
	if _, err := channel.Stderr().Write(frame); err != nil {
		return err
	}
	_, err = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
	return err
}
