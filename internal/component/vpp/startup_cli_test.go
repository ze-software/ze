// Design: docs/guide/vpp.md -- original-generation synchronous startup CLI
// Related: startup_cli.go -- shared binary API primitive and manager barrier
package vpp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/memclnt"
)

// This initialized transport records the exact request and context. Every
// returned stream closes once even when the native reply or Close refuses.
type startupCLITestStream struct {
	ctx                                 context.Context
	request                             api.Message
	sendError, receiveError, closeError error
	reply                               api.Message
	receive                             func() (api.Message, error)
	close                               func() error
	closes                              int
	sends                               int
}

func (s *startupCLITestStream) Context() context.Context { return s.ctx }
func (s *startupCLITestStream) SendMsg(message api.Message) error {
	s.sends++
	s.request = message
	return s.sendError
}
func (s *startupCLITestStream) RecvMsg() (api.Message, error) {
	if s.receive != nil {
		return s.receive()
	}
	return s.reply, s.receiveError
}
func (s *startupCLITestStream) Close() error {
	s.closes++
	if s.close != nil {
		return s.close()
	}
	return s.closeError
}

func startupCLITestOwner(t *testing.T, stream *startupCLITestStream) (*Connector, *recordingConnection) {
	t.Helper()
	owner := NewConnector("/original/api.sock")
	transport := &recordingConnection{open: func(ctx context.Context) (api.Stream, error) { stream.ctx = ctx; return stream, nil }}
	owner.conn, owner.connected, owner.generation = transport, true, 7
	return owner, transport
}

// One call sends exactly the original file on the supplied owner rather than
// a globally active foreign connector, then checks the actual native result.
func TestVPPStartupCLIUsesOriginalGenerationOnce(t *testing.T) {
	stream := &startupCLITestStream{reply: &startupCLIReply{}}
	owner, transport := startupCLITestOwner(t, stream)
	old := GetActiveConnector()
	foreign := NewConnector("foreign")
	foreignTransport := &recordingConnection{}
	foreign.conn = foreignTransport
	foreign.connected = true
	foreign.generation = 7
	setActiveConnector(foreign)
	t.Cleanup(func() { setActiveConnector(old) })
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	if err := ExecuteStartupCLI(ctx, owner, 7, "/original/bootstrap.cli"); err != nil {
		t.Fatal(err)
	}
	request, ok := stream.request.(*startupCLIRequest)
	if !ok {
		t.Fatalf("wrong request %T", stream.request)
	}
	if request.Cmd != "exec /original/bootstrap.cli" {
		t.Fatal("original path was rewritten", request.Cmd)
	}
	if stream.ctx != ctx {
		t.Fatal("request lost the caller context")
	}
	if transport.streams.Load() != 1 {
		t.Fatal("startup CLI repeated or did not use original transport", transport.streams.Load())
	}
	if stream.sends != 1 {
		t.Fatal("original startup file was submitted more than once", stream.sends)
	}
	if stream.closes != 1 {
		t.Fatal("original stream did not close once", stream.closes)
	}
	if foreignTransport.streams.Load() != 0 {
		t.Fatal("global replacement connector was selected")
	}
}

// A native command error is a failure even with a correctly typed reply and
// successful transport. Native output and exact Retval remain diagnostic data.
func TestVPPStartupCLIRejectsNativeCommandError(t *testing.T) {
	stream := &startupCLITestStream{reply: &startupCLIReply{Retval: -7, Reply: "original native command failed"}}
	owner, _ := startupCLITestOwner(t, stream)
	err := ExecuteStartupCLI(t.Context(), owner, 7, "/original/bootstrap.cli")
	if err == nil {
		t.Fatal("native CLI error admitted startup")
	}
	for _, want := range []string{"/original/bootstrap.cli", "-7", "original native command failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatal("native refusal lost original diagnostic", want, err)
		}
	}
	if stream.closes != 1 {
		t.Fatal("refused reply leaked original stream")
	}
}

// Wrong, nil and typed-nil native replies cannot impersonate completion.
func TestVPPStartupCLIRequiresExactReply(t *testing.T) {
	for name, reply := range map[string]api.Message{"wrong": &memclnt.ControlPingReply{}, "nil": nil, "typed-nil": (*startupCLIReply)(nil)} {
		t.Run(name, func(t *testing.T) {
			stream := &startupCLITestStream{reply: reply}
			owner, _ := startupCLITestOwner(t, stream)
			if err := ExecuteStartupCLI(t.Context(), owner, 7, "/original/bootstrap.cli"); err == nil {
				t.Fatal("missing exact completion admitted")
			}
			if stream.closes != 1 {
				t.Fatal("invalid reply leaked original stream")
			}
		})
	}
}

// Stream open/send/receive/close errors keep their original identities; no
// retry can repeat a native command whose transport outcome is uncertain.
func TestVPPStartupCLIPreservesOriginalFailures(t *testing.T) {
	for _, stage := range []string{"open", "send", "receive", "close"} {
		t.Run(stage, func(t *testing.T) {
			cause := errors.New("original " + stage + " failure")
			stream := &startupCLITestStream{reply: &startupCLIReply{}}
			owner, transport := startupCLITestOwner(t, stream)
			switch stage {
			case "open":
				transport.open = func(context.Context) (api.Stream, error) { return nil, cause }
			case "send":
				stream.sendError = cause
			case "receive":
				stream.receiveError = cause
			case "close":
				stream.closeError = cause
			}
			err := ExecuteStartupCLI(t.Context(), owner, 7, "/original/bootstrap.cli")
			if !errors.Is(err, cause) {
				t.Fatal("original native failure lost", err)
			}
			if transport.streams.Load() != 1 {
				t.Fatal("uncertain command was retried")
			}
			if stream.sends > 1 {
				t.Fatal("uncertain command was replayed on its original stream")
			}
			want := 1
			if stage == "open" {
				want = 0
			}
			if stream.closes != want {
				t.Fatal("original stream close count", stream.closes)
			}
		})
	}
}

// Generation changes at either boundary refuse and never route an old
// operation to the replacement transport, including change during Close.
func TestVPPStartupCLIRejectsChangedGeneration(t *testing.T) {
	for _, stage := range []string{"before", "during", "close"} {
		t.Run(stage, func(t *testing.T) {
			stream := &startupCLITestStream{reply: &startupCLIReply{}}
			owner, transport := startupCLITestOwner(t, stream)
			if stage == "before" {
				owner.generation = 8
			}
			if stage == "during" {
				stream.receive = func() (api.Message, error) {
					owner.mu.Lock()
					owner.generation = 8
					owner.mu.Unlock()
					return &startupCLIReply{}, nil
				}
			}
			if stage == "close" {
				stream.close = func() error { owner.mu.Lock(); owner.generation = 8; owner.mu.Unlock(); return nil }
			}
			err := ExecuteStartupCLI(t.Context(), owner, 7, "/original/bootstrap.cli")
			if !errors.Is(err, errVppGenerationChanged) {
				t.Fatal("changed native owner admitted", err)
			}
			want := int32(1)
			if stage == "before" {
				want = 0
			}
			if transport.streams.Load() != want {
				t.Fatal("wrong transport selected", transport.streams.Load())
			}
		})
	}
}

// Cancellation before allocation, while receiving and after the typed reply
// all refuse readiness and preserve the original caller cancellation cause.
func TestVPPStartupCLICancellationIsNotCompletion(t *testing.T) {
	for _, stage := range []string{"before", "receiving", "after-reply"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			stream := &startupCLITestStream{reply: &startupCLIReply{}}
			owner, transport := startupCLITestOwner(t, stream)
			if stage == "before" {
				cancel()
			}
			if stage == "receiving" {
				stream.receive = func() (api.Message, error) { cancel(); <-ctx.Done(); return nil, ctx.Err() }
			}
			if stage == "after-reply" {
				stream.receive = func() (api.Message, error) { cancel(); return &startupCLIReply{}, nil }
			}
			err := ExecuteStartupCLI(ctx, owner, 7, "/original/bootstrap.cli")
			if !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation became native success", err)
			}
			want := int32(1)
			if stage == "before" {
				want = 0
			}
			if transport.streams.Load() != want {
				t.Fatal("canceled operation selected unexpected transport")
			}
		})
	}
}

// Direct callers cannot bypass native-token validation or supply an absent
// owner/epoch. Only the manager's absent-leaf branch is an optional no-op.
func TestVPPStartupCLIRejectsInvalidBoundary(t *testing.T) {
	stream := &startupCLITestStream{reply: &startupCLIReply{}}
	owner, transport := startupCLITestOwner(t, stream)
	for _, path := range []string{"", "relative.cli", "/tmp/a b", "/tmp/a\nother", "/tmp/a;other"} {
		if err := ExecuteStartupCLI(t.Context(), owner, 7, path); err == nil {
			t.Fatal("invalid original native path admitted", path)
		}
	}
	//nolint:staticcheck // SA1012: boundary refusal is the assertion.
	if err := ExecuteStartupCLI(nil, owner, 7, "/original/bootstrap.cli"); err == nil {
		t.Fatal("nil context admitted")
	}
	if err := ExecuteStartupCLI(t.Context(), nil, 7, "/original/bootstrap.cli"); err == nil {
		t.Fatal("absent original connector admitted")
	}
	if err := ExecuteStartupCLI(t.Context(), owner, 0, "/original/bootstrap.cli"); err == nil {
		t.Fatal("zero generation admitted")
	}
	if transport.streams.Load() != 0 {
		t.Fatal("invalid input reached native transport")
	}
}

// An unavailable original transport or a missing stream cannot be treated as
// successful native execution. The connected positive case lives above.
func TestVPPStartupCLIRefusesUnavailableTransport(t *testing.T) {
	for _, mode := range []string{"disconnected", "missing-connection", "missing-stream"} {
		t.Run(mode, func(t *testing.T) {
			stream := &startupCLITestStream{reply: &startupCLIReply{}}
			owner, transport := startupCLITestOwner(t, stream)
			switch mode {
			case "disconnected":
				owner.connected = false
			case "missing-connection":
				owner.conn = nil
			case "missing-stream":
				transport.open = func(context.Context) (api.Stream, error) {
					return nil, nil //nolint:nilnil // Deliberately malformed success must refuse an absent stream.
				}
			}
			err := ExecuteStartupCLI(t.Context(), owner, 7, "/original/bootstrap.cli")
			if err == nil {
				t.Fatal("unavailable native transport admitted startup")
			}
			if mode != "missing-stream" {
				if !errors.Is(err, errVppNotConnected) {
					t.Fatal("original disconnect cause lost", err)
				}
			}
			if stream.request != nil {
				t.Fatal("unavailable original transport received a command")
			}
		})
	}
}
