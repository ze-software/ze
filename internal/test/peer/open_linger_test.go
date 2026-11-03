// Design: docs/architecture/testing/ci-format.md -- completed OPEN expectations and linger.

package peer

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// TestOpenCompletionLinger proves an OPEN-only script holds its completed
// connection when requested. A wire round trip proves liveness, cancellation
// ends the hold, and the control proves ordinary completion still closes.
func TestOpenCompletionLinger(t *testing.T) {
	const open = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D01040001005A0A141E2800"
	for _, linger := range []bool{false, true} {
		name := "without-linger"
		if linger {
			name = "with-linger"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			p, err := New(&Config{Mode: ModeCheck, Linger: linger, InspectOpenMessage: true,
				Expect: []string{"expect=bgp:conn=1:seq=1:hex=" + open}, Output: new(bytes.Buffer)})
			if err != nil {
				t.Fatal(err)
			}
			local, remote := net.Pipe()
			done := make(chan Result, 1)
			go func() {
				result := p.handleConnection(ctx, local)
				local.Close() //nolint:errcheck // test owns both socket ends
				done <- result
			}()
			defer func() {
				cancel()
				remote.Close() //nolint:errcheck // release a blocked test worker on failure
				result := <-done
				if !result.Success {
					t.Errorf("completed peer failed: %v", result.Error)
				}
			}()
			if err := remote.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			wire, err := hex.DecodeString(open)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := remote.Write(wire); err != nil {
				t.Fatal(err)
			}
			reader := newMessageReader(remote)
			for _, kind := range []byte{MsgOPEN, MsgKEEPALIVE} {
				header, _, err := reader.readMessage()
				if err != nil {
					t.Fatal(err)
				}
				if header[18] != kind {
					t.Fatalf("handshake kind %d, want %d", header[18], kind)
				}
			}
			if !linger {
				var one [1]byte
				if _, err := remote.Read(one[:]); !errors.Is(err, io.EOF) {
					t.Fatalf("ordinary completion did not close: %v", err)
				}
				return
			}
			if _, err := remote.Write(KeepaliveMsg()); err != nil {
				t.Fatalf("completed OPEN did not linger: %v", err)
			}
			header, _, err := reader.readMessage()
			if err != nil {
				t.Fatal(err)
			}
			if header[18] != MsgKEEPALIVE {
				t.Fatalf("linger reply kind %d, want KEEPALIVE", header[18])
			}
			cancel()
			var one [1]byte
			if _, err := remote.Read(one[:]); !errors.Is(err, io.EOF) {
				t.Fatalf("canceled hold did not close: %v", err)
			}
		})
	}
}

// TestPreOpenNotificationWithLingerCloses keeps a refused handshake out of the
// completed-session hold: a notification is terminal, even with linger set.
func TestPreOpenNotificationWithLingerCloses(t *testing.T) {
	p, err := New(&Config{Mode: ModeCheck, Linger: true, InspectOpenMessage: true,
		Expect: []string{"expect=bgp:conn=1:seq=1:hex=" + ceaseConnectionRejected}, Output: new(bytes.Buffer)})
	if err != nil {
		t.Fatal(err)
	}
	local, remote := net.Pipe()
	defer remote.Close() //nolint:errcheck // test socket
	answered := make(chan error, 1)
	go func() { answered <- refuseConnection(remote) }()
	result := p.handleConnection(t.Context(), local)
	local.Close() //nolint:errcheck // the reader must observe EOF after the handler returns
	if err := <-answered; err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected notification failed: %v", result.Error)
	}
}
