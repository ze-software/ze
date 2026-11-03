// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- transport admission before use.

package pppoeclient

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
)

// TestSessionNilTransports supplies a valid reader so neither nil form can hide
// behind receive-owner validation. Admission must return an error, never panic.
func TestSessionNilTransports(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transport io.ReadWriteCloser
	}{
		{name: "nil"},
		{name: "typed-nil", transport: (*sessionLink)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				defer func() {
					if value := recover(); value != nil {
						t.Errorf("transport admission panicked: %v", value)
					}
				}()
				frames := make(chan readFrame)
				reader := testFrameReader(t, frames, nil)
				_, err := negotiateSession(tc.transport, reader, -1, -1,
					sessionConfig{mtu: 1492, chanFD: -1}, make(chan struct{}),
					slog.New(slog.NewTextHandler(io.Discard, nil)))
				if err == nil {
					t.Fatal("nil transport admitted")
				}
				if !strings.Contains(err.Error(), "sessionLink") {
					t.Fatalf("unexpected refusal: %v", err)
				}
				if reader.pending {
					t.Fatal("nil transport authorized a packet read")
				}
			})
		})
	}
}
