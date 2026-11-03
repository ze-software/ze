// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- receive failure diagnostics.
// Related: receive.go -- actual receive worker; dialer.go -- supervisor shutdown.

package pppoeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestNetworkReceiveFailureDiagnostic feeds terminal results through the real
// reader and supervisor. Active failures must retain their cause and client
// identity at WARN; owner shutdown must close exactly once without a warning.
func TestNetworkReceiveFailureDiagnostic(t *testing.T) {
	for _, readErr := range []error{syscall.EIO, syscall.EBADF, io.EOF, io.ErrUnexpectedEOF, io.ErrClosedPipe, context.Canceled, nil} {
		name := "zero-read"
		if readErr != nil {
			name = readErr.Error()
		}
		for _, stopped := range []bool{false, true} {
			mode := "active/"
			if stopped {
				mode = "stopped/"
			}
			t.Run(mode+name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					channel := newConnectedExchange()
					transportCloses := 0
					link := &sessionLink{channel: channel, stopped: make(chan struct{}),
						closeTransport: func() { transportCloses++ }}
					entered, release := make(chan struct{}), make(chan struct{})
					reader, err := newFrameReader(func([]byte) (int, error) {
						close(entered)
						<-release
						return 0, readErr
					}, link.stopped)
					if err != nil {
						t.Fatal(err)
					}
					initial := sessionResult{reader: reader, localIP: netip.MustParseAddr("192.0.2.2"),
						peerIP: netip.MustParseAddr("192.0.2.1"), negMTU: 1492}
					session, err := newConnectedSession(link, initial,
						sessionConfig{name: "wan-receive", mtu: 1492}, func(ppp.NPMode) error {
							t.Error("receive failure changed forwarding mode")
							return nil
						})
					if err != nil {
						t.Fatal(err)
					}
					var output bytes.Buffer
					logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelWarn}))
					phaseDone := make(chan struct{})
					go superviseNetworkPhase(link, session, phaseDone, logger)
					<-entered
					if stopped {
						if err := link.Close(); err != nil {
							t.Fatal(err)
						}
					}
					close(release)
					<-phaseDone
					<-link.stopped
					<-reader.done
					synctest.Wait()
					if channel.closes != 1 || transportCloses != 1 {
						t.Fatalf("closure counts: channel=%d transport=%d", channel.closes, transportCloses)
					}
					if _, err := link.Write([]byte{1}); !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("write after Done: %v", err)
					}
					if stopped {
						if output.Len() != 0 {
							t.Fatalf("intentional stop warned: %s", &output)
						}
						return
					}
					var record struct {
						Level string `json:"level"`
						Msg   string `json:"msg"`
						Name  string `json:"name"`
						Error string `json:"error"`
					}
					if err := json.Unmarshal(output.Bytes(), &record); err != nil {
						t.Fatalf("expected one receive warning: %v; log=%s", err, &output)
					}
					cause := io.EOF
					if readErr != nil {
						cause = readErr
					}
					if record.Level != "WARN" || record.Name != "wan-receive" || record.Error != cause.Error() {
						t.Fatalf("receive warning lost severity, client or cause: %+v", record)
					}
					if record.Msg != "pppoe-client: cannot receive session packets; the client will reconnect" {
						t.Fatalf("receive warning lost problem or recovery action: %q", record.Msg)
					}
				})
			})
		}
	}
}

// TestNetworkReceiveWarningAfterStop checks the delivery/stop race boundary
// directly: a delivered error must not warn once Close has published stop.
// The supervisor test above covers production delivery and transport closure.
func TestNetworkReceiveWarningAfterStop(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelWarn}))
	session := &connectedSession{cfg: sessionConfig{name: "wan-stopped"}}
	for _, err := range []error{syscall.EIO, io.EOF, io.ErrUnexpectedEOF, io.ErrClosedPipe, context.Canceled} {
		session.warnReceiveFailure(err, stop, logger)
	}
	if output.Len() != 0 {
		t.Fatalf("delivered error warned after stop: %s", &output)
	}
}
