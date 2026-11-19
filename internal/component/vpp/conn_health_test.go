// Design: docs/research/vpp-deployment-reference.md -- connection loss ownership
// VALIDATES: loss invalidates before publication and Close joins its one worker.
package vpp

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A failed probe must invalidate the captured generation before its loss
// notification can start replacement work. Old streams must then refuse.
func TestConnectorHealthLossInvalidatesBeforeNotification(t *testing.T) {
	proofs := observeProcessExits(t)
	c := NewConnector("unused")
	conn := new(recordingConnection)
	c.conn, c.connected, c.generation = conn, true, 7
	ticks := make(chan time.Time, 1)
	want := errors.New("old Unix connection lost")
	c.health = c.startHealth(conn, 7, ticks, func() {}, func(context.Context, connectorConnection) error { return want })
	t.Cleanup(c.Close)
	loss, err := c.ConnectionLoss()
	if err != nil {
		t.Fatal(err)
	}
	ticks <- time.Time{}
	select {
	case err := <-loss:
		if !errors.Is(err, want) {
			t.Fatalf("loss = %v", err)
		}
	case <-t.Context().Done():
		t.Fatal("connection loss was not published")
	}
	if c.IsConnected() {
		t.Fatal("loss notification preceded invalidation")
	}
	if stream, err := c.NewStreamForGeneration(t.Context(), 7); stream != nil || !errors.Is(err, errVppGenerationChanged) {
		t.Fatalf("old stream = %v, %v", stream, err)
	}
	if len(proofs()) != 0 {
		t.Fatal("connection loss manufactured process death")
	}
	if conn.disconnects.Load() != 0 {
		t.Fatal("health worker closed the API before its owner's statistics join")
	}
	c.Close()
	if conn.disconnects.Load() != 1 {
		t.Fatalf("native disconnects = %d", conn.disconnects.Load())
	}
}

// A delayed old worker cannot retire a connection that another explicit
// publication already owns, or put its error into the replacement's channel.
func TestConnectorOldHealthCannotRetireReplacement(t *testing.T) {
	c := NewConnector("unused")
	old := new(recordingConnection)
	oldHealth := &connectorHealth{lost: make(chan error, 1)}
	replacement := new(recordingConnection)
	replacementHealth := &connectorHealth{lost: make(chan error, 1)}
	c.conn, c.health, c.connected, c.generation = replacement, replacementHealth, true, 9
	c.connectionLost(old, 7, oldHealth, errors.New("late old probe"))
	if !c.IsConnected() {
		t.Fatal("old worker retired replacement")
	}
	if generation, err := c.ConnectionGeneration(); generation != 9 || err != nil {
		t.Fatalf("generation = %d, %v", generation, err)
	}
	select {
	case err := <-replacementHealth.lost:
		t.Fatalf("replacement loss = %v", err)
	default:
	}
	if stream, err := c.NewStreamForGeneration(t.Context(), 7); stream != nil || !errors.Is(err, errVppGenerationChanged) {
		t.Fatalf("old stream selected replacement: %v, %v", stream, err)
	}
	if replacement.disconnects.Load() != 0 || replacement.streams.Load() != 0 {
		t.Fatal("old worker touched replacement transport")
	}
}

// Cancellation must finish the connection worker before Close returns, while
// preserving the distinction between API disconnection and process exit.
func TestConnectorHealthCloseJoinsWorker(t *testing.T) {
	proofs := observeProcessExits(t)
	c := NewConnector("unused")
	conn := new(recordingConnection)
	c.conn, c.connected, c.generation = conn, true, 7
	ticks := make(chan time.Time, 1)
	entered, finished := make(chan struct{}), make(chan struct{})
	c.health = c.startHealth(conn, 7, ticks, func() {}, func(ctx context.Context, _ connectorConnection) error {
		close(entered)
		<-ctx.Done()
		close(finished)
		return ctx.Err()
	})
	ticks <- time.Time{}
	select {
	case <-entered:
	case <-t.Context().Done():
		t.Fatal("probe did not start")
	}
	c.Close()
	select {
	case <-finished:
	default:
		t.Fatal("Close forgot its live health worker")
	}
	if len(proofs()) != 0 {
		t.Fatal("Close manufactured a process-exit proof")
	}
	if conn.disconnects.Load() != 1 {
		t.Fatalf("native disconnects = %d", conn.disconnects.Load())
	}
}

// Connection retries are bounded by the caller's declared attempt count; a
// cancellation is observed before another dial rather than after publication.
func TestConnectorSynchronousRetryIsBounded(t *testing.T) {
	c := NewConnector("unused")
	want := errors.New("socket refused")
	attempts := 0
	err := c.connect(t.Context(), 3, time.Nanosecond, func(context.Context) (connectorConnection, error) { attempts++; return nil, want })
	if !errors.Is(err, want) || attempts != 3 {
		t.Fatalf("attempts = %d, error = %v", attempts, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = c.connect(ctx, 3, time.Nanosecond, func(context.Context) (connectorConnection, error) { t.Fatal("canceled dial ran"); return nil, want })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled connect = %v", err)
	}
}

// A concurrent Close must prevent a completed old dial from publishing after
// shutdown, including one whose successful return was already in flight.
func TestConnectorCloseRejectsInFlightPublication(t *testing.T) {
	c := NewConnector("unused")
	entered, release := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	transport := new(recordingConnection)
	go func() {
		result <- c.connect(t.Context(), 1, 0, func(context.Context) (connectorConnection, error) {
			close(entered)
			<-release
			return transport, nil
		})
	}()
	select {
	case <-entered:
	case <-t.Context().Done():
		t.Fatal("dial did not enter")
	}
	c.Close()
	close(release)
	select {
	case err := <-result:
		if !errors.Is(err, errVppGenerationChanged) {
			t.Fatalf("publication = %v", err)
		}
	case <-t.Context().Done():
		t.Fatal("old dial did not return")
	}
	if c.IsConnected() {
		t.Fatal("old dial revived a closed connector")
	}
	if transport.disconnects.Load() != 1 {
		t.Fatalf("unpublished transport disconnects = %d", transport.disconnects.Load())
	}
}

// Invalid retry bounds and an unavailable worker cannot be mistaken for a
// healthy connection with no possible loss notification.
func TestConnectorHealthRefusesInvalidStartup(t *testing.T) {
	c := NewConnector("unused")
	if _, err := c.ConnectionLoss(); err == nil {
		t.Fatal("missing health owner accepted")
	}
	unreachableDial := errors.New("invalid retry bounds must refuse before dialing")
	for _, item := range []struct {
		attempts int
		interval time.Duration
	}{{0, 0}, {2, 0}} {
		if err := c.connect(t.Context(), item.attempts, item.interval, func(context.Context) (connectorConnection, error) {
			t.Fatal("invalid dial ran")
			return nil, unreachableDial
		}); err == nil {
			t.Fatal("invalid retry bounds accepted")
		}
	}
}
