package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSSEBroadcast verifies events are delivered to all connected clients.
//
// VALIDATES: Broadcast sends to all subscribers, each receives the event.
// PREVENTS: Events lost or delivered to wrong clients.
func TestSSEBroadcast(t *testing.T) {
	t.Parallel()

	broker := NewSSEBroker(200 * time.Millisecond)
	defer broker.Close()

	c1 := broker.Subscribe()
	c2 := broker.Subscribe()

	broker.Broadcast(SSEEvent{Event: "test", Data: "hello"})

	select {
	case ev := <-c1.ch:
		if ev.Event != "test" || ev.Data != "hello" {
			t.Fatalf("c1 got (%q, %q), want (\"test\", \"hello\")", ev.Event, ev.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("c1 did not receive event")
	}

	select {
	case ev := <-c2.ch:
		if ev.Event != "test" || ev.Data != "hello" {
			t.Fatalf("c2 got (%q, %q), want (\"test\", \"hello\")", ev.Event, ev.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("c2 did not receive event")
	}
}

// TestSSEClientCleanup verifies disconnected clients are removed.
//
// VALIDATES: Unsubscribe removes client, done channel is closed.
// PREVENTS: Leaked client goroutines or map entries.
func TestSSEClientCleanup(t *testing.T) {
	t.Parallel()

	broker := NewSSEBroker(200 * time.Millisecond)
	defer broker.Close()

	c := broker.Subscribe()
	if broker.ClientCount() != 1 {
		t.Fatalf("ClientCount() = %d, want 1", broker.ClientCount())
	}

	broker.Unsubscribe(c)
	if broker.ClientCount() != 0 {
		t.Fatalf("ClientCount() after unsubscribe = %d, want 0", broker.ClientCount())
	}

	// done channel should be closed.
	select {
	case <-c.done:
		// OK
	default:
		t.Fatal("done channel should be closed after Unsubscribe")
	}
}

// TestSSEDebounce verifies the broker's debounce interval is respected.
//
// VALIDATES: Debounce interval clamped to minimum 50ms.
// PREVENTS: Overly aggressive SSE updates overwhelming the browser.
func TestSSEDebounce(t *testing.T) {
	t.Parallel()

	// Below minimum — should clamp to 50ms.
	broker := NewSSEBroker(10 * time.Millisecond)
	if broker.Interval() != 50*time.Millisecond {
		t.Fatalf("Interval() = %v, want 50ms (clamped)", broker.Interval())
	}

	// Valid interval.
	broker = NewSSEBroker(200 * time.Millisecond)
	if broker.Interval() != 200*time.Millisecond {
		t.Fatalf("Interval() = %v, want 200ms", broker.Interval())
	}
}

// TestSSEBroadcastDropsOnFullBuffer verifies non-blocking send behavior.
//
// VALIDATES: Full client buffer causes event drop, not blocking.
// PREVENTS: Slow client blocking all other clients.
func TestSSEBroadcastDropsOnFullBuffer(t *testing.T) {
	t.Parallel()

	broker := NewSSEBroker(200 * time.Millisecond)
	defer broker.Close()

	c := broker.Subscribe()

	// Fill the client buffer (capacity 64).
	for range 64 {
		broker.Broadcast(SSEEvent{Data: "fill"})
	}

	// Next broadcast should not block — event is dropped for this client.
	done := make(chan struct{})
	go func() {
		broker.Broadcast(SSEEvent{Data: "overflow"})
		close(done)
	}()

	select {
	case <-done:
		// Good — broadcast returned without blocking.
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on full client buffer")
	}

	// Drain and verify we got the fill events, not the overflow.
	count := 0
	for range 64 {
		select {
		case <-c.ch:
			count++
		default:
		}
	}
	if count != 64 {
		t.Fatalf("drained %d events, want 64", count)
	}
}

// TestSSEClose verifies broker shutdown closes all clients.
//
// VALIDATES: Close marks broker closed, all client done channels closed.
// PREVENTS: Leaked goroutines after dashboard shutdown.
func TestSSEClose(t *testing.T) {
	t.Parallel()

	broker := NewSSEBroker(200 * time.Millisecond)

	c1 := broker.Subscribe()
	c2 := broker.Subscribe()

	broker.Close()

	if broker.ClientCount() != 0 {
		t.Fatalf("ClientCount() after Close = %d, want 0", broker.ClientCount())
	}

	// Both done channels should be closed.
	select {
	case <-c1.done:
	default:
		t.Fatal("c1.done should be closed after Close")
	}
	select {
	case <-c2.done:
	default:
		t.Fatal("c2.done should be closed after Close")
	}
}

// TestSSEServeHTTP verifies the SSE HTTP handler sends events in SSE format.
//
// VALIDATES: Correct Content-Type, event/data format, client cleanup on disconnect.
// PREVENTS: Malformed SSE output that browsers can't parse.
func TestSSEServeHTTP(t *testing.T) {
	t.Parallel()

	broker := NewSSEBroker(200 * time.Millisecond)
	defer broker.Close()

	// Start the handler in a goroutine — it blocks until client disconnects.
	ts := httptest.NewServer(broker)
	defer ts.Close()

	// Send an event before connecting (should not matter).
	broker.Broadcast(SSEEvent{Event: "early", Data: "skip"})

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	// Send an event while connected.
	broker.Broadcast(SSEEvent{Event: "peer-update", Data: "<div>test</div>"})

	// Read the response — we need a small buffer read since the connection stays open.
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, "event: peer-update") {
		t.Errorf("response missing 'event: peer-update', got: %q", body)
	}
	if !strings.Contains(body, "data: <div>test</div>") {
		t.Errorf("response missing 'data: <div>test</div>', got: %q", body)
	}
}
