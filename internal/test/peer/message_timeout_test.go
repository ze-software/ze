// Design: docs/architecture/testing/ci-format.md — complete frames across idle timeouts.
// Related: message.go, peer.go, reject.go — frame reader and its retrying consumers.

package peer

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMessageLoopPreservesPartialFrame requires the real checker and echo
// consumer to receive two exact wire frames despite a timeout inside the first.
// Scripted reads fix the byte/error boundary without scheduling or sleeps.
func TestMessageLoopPreservesPartialFrame(t *testing.T) {
	first := wireMessage(t, "0000000740050400000064180A0000")
	second := wireMessage(t, "00000007400504000000C818C00002")
	for _, mode := range []Mode{ModeCheck, ModeEcho} {
		for _, split := range []struct {
			name string
			at   int
		}{{"header", 7}, {"body", HeaderLen + 3}} {
			for _, combined := range []bool{false, true} {
				name := split.name + "/separate-error"
				if combined {
					name = split.name + "/data-with-error"
				}
				t.Run(mode.String()+"/"+name, func(t *testing.T) {
					conn := timeoutFrameConn(t, first, second, split.at, combined)
					p, err := New(&Config{
						Mode: mode,
						Expect: []string{
							"expect=bgp:conn=1:seq=1:hex=" + first.Stream(),
							"expect=bgp:conn=1:seq=2:hex=" + second.Stream(),
						},
						Silent: true,
						Output: io.Discard,
					})
					if err != nil {
						t.Fatal(err)
					}
					p.checker.Init()
					result := p.runMessageLoop(t.Context(), conn, 0)
					if mode == ModeCheck {
						if !result.Success {
							t.Fatalf("exact wire expectations failed after partial-frame timeout: %v", result.Error)
						}
						if !p.checker.Completed() {
							t.Fatal("success without consuming both complete wire frames")
						}
					}
					if mode == ModeEcho {
						want, err := hex.DecodeString(first.Stream() + second.Stream())
						if err != nil {
							t.Fatal(err)
						}
						if !bytes.Equal(conn.written.Bytes(), want) {
							t.Fatalf("echo consumer wrote %X, want exact frames %X (result: %v)", conn.written.Bytes(), want, result.Error)
						}
					}
					if len(conn.steps) != 0 {
						t.Fatalf("consumer left %d scripted reads unconsumed", len(conn.steps))
					}
				})
			}
		}
	}
}

// TestHoldSessionPreservesPartialFrame requires the linger consumer to process
// the interrupted frame, then reject a following complete forbidden frame.
func TestHoldSessionPreservesPartialFrame(t *testing.T) {
	first := wireMessage(t, "0000000740050400000064180A0000")
	second := wireMessage(t, "00000007400504000000C818C00002")
	for _, split := range []struct {
		name string
		at   int
	}{{"header", 7}, {"body", HeaderLen + 3}} {
		t.Run(split.name, func(t *testing.T) {
			conn := timeoutFrameConn(t, first, second, split.at, false)
			var output bytes.Buffer
			p, err := New(&Config{
				Mode: ModeCheck,
				Expect: []string{
					"expect=bgp:conn=1:seq=1:hex=" + first.Stream(),
					"reject=bgp:conn=1:pattern=" + second.Stream(),
				},
				Output: &output,
			})
			if err != nil {
				t.Fatal(err)
			}
			p.checker.Init()
			result, closed := p.holdSession(t.Context(), conn)
			if result.Success {
				t.Fatal("linger lost the following forbidden wire frame")
			}
			if closed {
				t.Fatal("rejection was reported as remote closure")
			}
			if !strings.Contains(output.String(), RejectionMarker) {
				t.Fatalf("missing runner-visible rejection: %s", output.String())
			}
			if !bytes.Equal(conn.written.Bytes(), KeepaliveMsg()) {
				t.Fatalf("linger did not process exactly the first complete frame: %X", conn.written.Bytes())
			}
		})
	}
}

// TestPartialFrameTimeoutCancellation cancels at the timeout boundary itself;
// neither consumer may retry a partial frame without returning control to its
// context check. The finite read script also bounds a broken consumer.
func TestPartialFrameTimeoutCancellation(t *testing.T) {
	first := wireMessage(t, "0000000740050400000064180A0000")
	for _, hold := range []bool{false, true} {
		for _, split := range []struct {
			name string
			at   int
		}{{"header", 7}, {"body", HeaderLen + 3}} {
			name := "message/" + split.name
			if hold {
				name = "hold/" + split.name
			}
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				wire, err := hex.DecodeString(first.Stream())
				if err != nil {
					t.Fatal(err)
				}
				conn := &frameTimeoutConn{steps: []frameReadStep{
					{data: wire[:split.at]},
					{err: os.ErrDeadlineExceeded, before: cancel},
					{data: wire[split.at:]},
				}}
				p, err := New(&Config{
					Mode:   ModeCheck,
					Expect: []string{"expect=bgp:conn=1:seq=1:hex=" + first.Stream()},
					Silent: true,
					Output: io.Discard,
				})
				if err != nil {
					t.Fatal(err)
				}
				p.checker.Init()
				if hold {
					result, closed := p.holdSession(ctx, conn)
					if !result.Success {
						t.Fatalf("teardown failed: %v", result.Error)
					}
					if closed {
						t.Fatal("context cancellation mistaken for remote close")
					}
				} else {
					result := p.runMessageLoop(ctx, conn, 0)
					if !errors.Is(result.Error, errContextCanceled) {
						t.Fatalf("want context cancellation, got %v", result.Error)
					}
				}
				if len(conn.steps) != 1 {
					t.Fatalf("consumer read past cancellation: %d steps remain, want 1", len(conn.steps))
				}
			})
		}
	}
}

func timeoutFrameConn(t *testing.T, first, second *Message, split int, combined bool) *frameTimeoutConn {
	t.Helper()
	wire, err := hex.DecodeString(first.Stream() + second.Stream())
	if err != nil {
		t.Fatal(err)
	}
	if combined {
		return &frameTimeoutConn{steps: []frameReadStep{
			{data: wire[:split], err: os.ErrDeadlineExceeded},
			{data: wire[split:]},
		}}
	}
	return &frameTimeoutConn{steps: []frameReadStep{
		{data: wire[:split]},
		{err: os.ErrDeadlineExceeded},
		{data: wire[split:]},
	}}
}

type frameReadStep struct {
	data   []byte
	err    error
	before func()
}

// frameTimeoutConn is a single-consumer byte stream, not a message mock. Reads
// obey the caller's buffer size and deliver the scripted error only after all
// bytes in that step. Writes record the actual consumer's output verbatim.
type frameTimeoutConn struct {
	steps   []frameReadStep
	written bytes.Buffer
}

func (c *frameTimeoutConn) Read(dst []byte) (int, error) {
	if len(c.steps) == 0 {
		return 0, io.EOF
	}
	step := &c.steps[0]
	if step.before != nil {
		step.before()
		step.before = nil
	}
	n := copy(dst, step.data)
	step.data = step.data[n:]
	if len(step.data) != 0 {
		return n, nil
	}
	err := step.err
	c.steps = c.steps[1:]
	return n, err
}

func (c *frameTimeoutConn) Write(src []byte) (int, error)  { return c.written.Write(src) }
func (*frameTimeoutConn) Close() error                     { return nil }
func (*frameTimeoutConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (*frameTimeoutConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (*frameTimeoutConn) SetDeadline(time.Time) error      { return nil }
func (*frameTimeoutConn) SetReadDeadline(time.Time) error  { return nil }
func (*frameTimeoutConn) SetWriteDeadline(time.Time) error { return nil }
