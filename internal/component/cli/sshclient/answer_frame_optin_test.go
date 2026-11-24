// Design: docs/architecture/api/ipc_protocol.md -- opt-in SSH exec answer framing.

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

const (
	frameOptInCommand = "show version | json"
	frameOptInPayload = `{"version":"1.0"}`
)

// TestExecClientsRequestAnswerFrame proves that both ExecCommandStream and
// ExecCommand send the ZE_ANSWER_FRAME=1 SSH env request before starting exec,
// and that ExecCommandStream detects a missing frame as ErrAnswerTruncated.
//
// VALIDATES: AC-25 -- Ze's own SSH exec clients explicitly opt into stderr
// answer framing so the server can withhold frame lines from plain SSH clients.
// PREVENTS: sshclient forgetting to send ZE_ANSWER_FRAME=1 after the server
// gates framing on opt-in, which would break every ze cli -c invocation with
// ErrAnswerTruncated.
func TestExecClientsRequestAnswerFrame(t *testing.T) {
	t.Run("ExecCommandStream sends opt-in and reads framed answer", func(t *testing.T) {
		creds, envCh := startFrameEnvServer(t, true)
		var body strings.Builder
		answer, err := ExecCommandStream(creds, frameOptInCommand, &body)
		if err != nil {
			t.Fatalf("ExecCommandStream: %v", err)
		}
		sawEnv := <-envCh
		if sawEnv[EnvAnswerFrame] != AnswerFrameEnabled {
			t.Errorf("ExecCommandStream sent %s=%q, want %q", EnvAnswerFrame, sawEnv[EnvAnswerFrame], AnswerFrameEnabled)
		}
		if answer.Verdict != rpc.VerdictDone {
			t.Errorf("verdict = %q, want %q", answer.Verdict, rpc.VerdictDone)
		}
		if answer.Count != 1 {
			t.Errorf("count = %d, want 1", answer.Count)
		}
		if strings.TrimSpace(body.String()) != frameOptInPayload {
			t.Errorf("body = %q, want JSON payload", body.String())
		}
	})

	t.Run("ExecCommand sends opt-in and returns payload", func(t *testing.T) {
		creds, envCh := startFrameEnvServer(t, true)
		output, err := ExecCommand(creds, frameOptInCommand)
		if err != nil {
			t.Fatalf("ExecCommand: %v", err)
		}
		sawEnv := <-envCh
		if sawEnv[EnvAnswerFrame] != AnswerFrameEnabled {
			t.Errorf("ExecCommand sent %s=%q, want %q", EnvAnswerFrame, sawEnv[EnvAnswerFrame], AnswerFrameEnabled)
		}
		if output != frameOptInPayload {
			t.Errorf("output = %q, want JSON payload", output)
		}
	})

	t.Run("ExecCommandStream reports truncation when server emits no frame", func(t *testing.T) {
		creds, envCh := startFrameEnvServer(t, false)
		var body strings.Builder
		answer, err := ExecCommandStream(creds, frameOptInCommand, &body)
		if !errors.Is(err, ErrAnswerTruncated) {
			t.Fatalf("err = %v, want ErrAnswerTruncated", err)
		}
		sawEnv := <-envCh
		if sawEnv[EnvAnswerFrame] != AnswerFrameEnabled {
			t.Errorf("ExecCommandStream sent %s=%q, want %q", EnvAnswerFrame, sawEnv[EnvAnswerFrame], AnswerFrameEnabled)
		}
		if answer.Verdict != rpc.VerdictTruncated {
			t.Errorf("verdict = %q, want %q", answer.Verdict, rpc.VerdictTruncated)
		}
	})
}

func startFrameEnvServer(t *testing.T, emitFrame bool) (Credentials, <-chan map[string]string) {
	t.Helper()
	hostKeyTestEnvironment(t)
	signer := hostKeyTestSigner(t)
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	envCh := make(chan map[string]string, 1)
	done := make(chan error, 1)
	go func() {
		done <- serveFrameEnvOnce(listener, signer, emitFrame, envCh)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		if err := <-done; err != nil {
			t.Errorf("SSH fixture: %v", err)
		}
	})
	return Credentials{Host: "127.0.0.1", Port: port, Username: "operator", Auth: "frame-proof"}, envCh
}

func serveFrameEnvOnce(listener net.Listener, signer ssh.Signer, emitFrame bool, envCh chan<- map[string]string) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // owned fixture transport cleanup
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	config := &ssh.ServerConfig{PasswordCallback: func(meta ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
		if meta.User() != "operator" || string(pass) != "frame-proof" {
			return nil, errors.New("unexpected fixture credentials")
		}
		return &ssh.Permissions{}, nil
	}}
	config.AddHostKey(signer)
	server, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return err
	}
	defer server.Close() //nolint:errcheck // owned fixture transport cleanup
	go ssh.DiscardRequests(requests)

	proposed, ok := <-channels
	if !ok {
		return errors.New("fixture received no SSH session")
	}
	channel, chanReqs, err := proposed.Accept()
	if err != nil {
		return err
	}
	defer channel.Close() //nolint:errcheck // owned fixture channel cleanup

	recorded := make(map[string]string)
	for req := range chanReqs {
		if req.Type == "env" {
			var envReq struct{ Name, Value string }
			if err := ssh.Unmarshal(req.Payload, &envReq); err != nil {
				return err
			}
			recorded[envReq.Name] = envReq.Value
			if err := req.Reply(true, nil); err != nil {
				return err
			}
			continue
		}
		if req.Type != "exec" {
			return fmt.Errorf("unexpected request %q", req.Type)
		}
		envCh <- recorded
		if err := req.Reply(true, nil); err != nil {
			return err
		}
		if _, err := io.WriteString(channel, frameOptInPayload+"\n"); err != nil {
			return err
		}
		if emitFrame {
			frame := rpc.AppendAnswerHead(nil, rpc.AnswerNoID, rpc.AnswerTypeDocument, "", nil)
			frame = append(frame, '\n')
			frame = rpc.AppendAnswerTerminator(frame, rpc.AnswerNoID, 1, 0, "")
			frame = append(frame, '\n')
			if _, err := channel.Stderr().Write(frame); err != nil {
				return err
			}
		}
		_, err = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
		return err
	}
	return errors.New("fixture received no exec request")
}
