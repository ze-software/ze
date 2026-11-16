// Design: docs/guide/radius.md -- Client.Close joins its original answer reader
// Related: panic_guard_test.go -- the actual unsigned-EAP discard panic seam

package radius

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// closeReaderGate holds the real answer reader after dispatchResponse's
// deferred mutex unlock, inside the contained-panic ERROR log call. Its owner
// MUST release it before joining the reader; Handle MUST wait for that release.
type closeReaderGate struct {
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (g *closeReaderGate) Enabled(context.Context, slog.Level) bool { return true }

func (g *closeReaderGate) Handle(_ context.Context, record slog.Record) error { //nolint:gocritic // hugeParam: slog.Handler requires Record by value.
	if record.Level == slog.LevelError {
		g.once.Do(func() { close(g.entered) })
		<-g.release
	}
	return nil
}

func (g *closeReaderGate) WithAttrs([]slog.Attr) slog.Handler { return g }
func (g *closeReaderGate) WithGroup(string) slog.Handler      { return g }

// TestConcurrentClientCloseJoinsOriginalReader proves the second caller joins
// the reader even after the first caller has consumed the UDP socket. The real
// NewClient reader is gated in its contained-panic logger, outside c.mu, while
// its original done remains open. Nothing fabricates or replaces done.
// METHOD: after the real datagram reaches that gate, synctest.Wait schedules
// each Close until every worker is durably blocked. Both joins and the gate
// use channels created inside the bubble, so an omitted join must have returned
// before the absence assertion; no timer or sleep supplies that inference.
// MUTATION: remove only the closed branch's receive on done in Client.Close,
// retaining the captured done variable with _ = done. The second caller then
// returns while the actual reader remains gated, and this test must fail.
func TestConcurrentClientCloseJoinsOriginalReader(t *testing.T) {
	// Package tests MUST remain serial while the task and metrics seam is set.
	previousTask := readTask
	previousMetrics := radiusMetricsPtr.Swap(&radiusMetrics{})
	defer func() {
		readTask = previousTask
		radiusMetricsPtr.Store(previousMetrics)
	}()
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	// Network I/O is not durably blocked in synctest. Both computing AND
	// installing this setup deadline MUST happen outside the bubble: net's
	// SetReadDeadline calls time.Until, which otherwise uses the fake clock.
	// The single setter MUST be canceled and joined even if NewClient fails.
	deadlineSocket := make(chan *net.UDPConn)
	deadlineResult := make(chan error, 1)
	deadlineSetterDone := make(chan struct{})
	go func() {
		defer close(deadlineSetterDone)
		if socket, ok := <-deadlineSocket; ok {
			deadlineResult <- socket.SetReadDeadline(time.Now().Add(5 * time.Second))
		}
	}()
	defer func() {
		close(deadlineSocket)
		<-deadlineSetterDone
	}()

	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		releaseReader := sync.OnceFunc(func() { close(release) })
		defer releaseReader()
		gate := &closeReaderGate{entered: make(chan struct{}), release: release}
		guard := taskguard.New("radius", slog.New(gate))
		readTask = guard.Task(taskRead, readRestart)

		client, err := NewClient(ClientConfig{})
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		defer func() {
			releaseReader()
			if closeErr := client.Close(); closeErr != nil {
				t.Errorf("cleanup Close: %v", closeErr)
			}
			// Join both Close workers as well, including every failure path.
			synctest.Wait()
		}()
		deadlineSocket <- client.conn
		if err = <-deadlineResult; err != nil {
			t.Fatalf("SetReadDeadline: %v", err)
		}
		server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatalf("server ListenUDP: %v", err)
		}
		defer closeSilent(server)
		secret := []byte("testing123")
		request := &Packet{Code: CodeAccessRequest, Identifier: client.NextID()}
		key := responseKey{server: server.LocalAddr().String(), id: request.Identifier}
		waiter, err := client.registerWaiter(key, request.Authenticator, secret, true, "")
		if err != nil {
			t.Fatalf("registerWaiter: %v", err)
		}
		defer client.unregisterWaiter(key, waiter)
		requestWire := make([]byte, MaxPacketLen)
		requestLen, err := request.EncodeTo(requestWire, 0)
		if err != nil {
			t.Fatalf("request EncodeTo: %v", err)
		}
		answer := unsignedReply(CodeAccessAccept, requestWire[:requestLen], secret, true)
		if len(answer) < MinPacketLen {
			t.Fatal("unsigned EAP answer was not encoded")
		}
		local, ok := client.conn.LocalAddr().(*net.UDPAddr)
		if !ok {
			t.Fatalf("client socket address has type %T, want *net.UDPAddr", client.conn.LocalAddr())
		}
		destination := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: local.Port}
		written, err := server.WriteToUDP(answer, destination)
		if err != nil {
			t.Fatalf("server WriteToUDP: %v", err)
		}
		if written != len(answer) {
			t.Fatalf("server wrote %d bytes, want %d", written, len(answer))
		}
		select {
		case <-gate.entered:
		case <-client.done:
			t.Fatal("actual reader ended before reaching the contained-panic log gate")
		}
		count, declared := guard.Panics(taskRead)
		if !declared {
			t.Fatalf("reader task %q was not declared", taskRead)
		}
		if count != 1 {
			t.Fatalf("contained reader panics = %d, want 1", count)
		}

		first := make(chan error, 1)
		go func() { first <- client.Close() }()
		synctest.Wait()
		client.mu.Lock()
		closed := client.closed
		client.mu.Unlock()
		if !closed {
			t.Fatal("first Close did not fence requests while the reader was gated")
		}
		select {
		case <-client.done:
			t.Fatal("actual reader ended before its gate was released")
		default:
		}
		select {
		case err = <-first:
			t.Fatalf("first Close returned before reader release: %v", err)
		default:
		}
		second := make(chan error, 1)
		go func() { second <- client.Close() }()
		synctest.Wait()
		select {
		case err = <-second:
			t.Fatalf("concurrent second Close returned before original reader release: %v", err)
		default:
		}

		releaseReader()
		if err = <-first; err != nil {
			t.Fatalf("first Close after reader release: %v", err)
		}
		if err = <-second; err != nil {
			t.Fatalf("second Close after reader release: %v", err)
		}
		select {
		case <-client.done:
		default:
			t.Fatal("Close returned without the original reader's completion")
		}
	})
}
