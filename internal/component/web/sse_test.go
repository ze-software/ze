package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestEventBrokerSubscribe verifies that subscribing creates a client and
// increments the client count.
//
// VALIDATES: Subscribe creates client with buffered channel.
// PREVENTS: nil client returned on valid subscribe.
func TestEventBrokerSubscribe(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(100)
	defer broker.Close()

	c := broker.Subscribe()
	if c == nil {
		t.Fatal("Subscribe returned nil")
	}

	if broker.ClientCount() != 1 {
		t.Fatalf("ClientCount() = %d, want 1", broker.ClientCount())
	}

	c2 := broker.Subscribe()
	if c2 == nil {
		t.Fatal("second Subscribe returned nil")
	}

	if broker.ClientCount() != 2 {
		t.Fatalf("ClientCount() = %d, want 2", broker.ClientCount())
	}
}

// TestEventBrokerBroadcast verifies that broadcast delivers events to all
// subscribed clients.
//
// VALIDATES: Broadcast sends event to all subscribers, each receives it.
// PREVENTS: Events lost or delivered to wrong clients.
func TestEventBrokerBroadcast(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(100)
	defer broker.Close()

	c1 := broker.Subscribe()
	c2 := broker.Subscribe()

	broker.Broadcast("test", "hello")

	for _, tc := range []struct {
		name   string
		client *sseClient
	}{
		{"c1", c1},
		{"c2", c2},
	} {
		select {
		case ev := <-tc.client.ch:
			if ev.eventType != "test" || ev.data != "hello" {
				t.Fatalf("%s got (%q, %q), want (\"test\", \"hello\")", tc.name, ev.eventType, ev.data)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s did not receive event", tc.name)
		}
	}
}

// TestEventBrokerUnsubscribe verifies that unsubscribed clients are removed
// and their done channel is closed.
//
// VALIDATES: Unsubscribe removes client from broker, done channel closed.
// PREVENTS: Leaked client map entries or goroutines.
func TestEventBrokerUnsubscribe(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(100)
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

// TestEventBrokerNonBlocking verifies that broadcast does not block when
// a client's buffer is full.
//
// VALIDATES: Full client buffer causes event drop, not blocking.
// PREVENTS: Slow client blocking all other clients.
func TestEventBrokerNonBlocking(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(100)
	defer broker.Close()

	c := broker.Subscribe()

	// Fill the client buffer (capacity 16).
	for range 16 {
		broker.Broadcast("fill", "data")
	}

	// Next broadcast should not block.
	done := make(chan struct{})

	go func() {
		broker.Broadcast("overflow", "data")
		close(done)
	}()

	select {
	case <-done:
		// Good -- broadcast returned without blocking.
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on full client buffer")
	}

	// Drain and verify we got the fill events, not the overflow.
	count := 0
	for range 16 {
		select {
		case <-c.ch:
			count++
		default:
		}
	}

	if count != 16 {
		t.Fatalf("drained %d events, want 16", count)
	}
}

// TestEventBrokerMaxClients verifies that Subscribe returns nil when the
// maximum number of clients is reached.
//
// VALIDATES: Broker enforces maxClients limit.
// PREVENTS: Unbounded client registrations causing resource exhaustion.
func TestEventBrokerMaxClients(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(2)
	defer broker.Close()

	c1 := broker.Subscribe()
	if c1 == nil {
		t.Fatal("first Subscribe returned nil")
	}

	c2 := broker.Subscribe()
	if c2 == nil {
		t.Fatal("second Subscribe returned nil")
	}

	c3 := broker.Subscribe()
	if c3 != nil {
		t.Fatal("third Subscribe should return nil (max 2 clients)")
	}

	if broker.ClientCount() != 2 {
		t.Fatalf("ClientCount() = %d, want 2", broker.ClientCount())
	}
}

// TestEventBrokerClose verifies that closing the broker removes all clients
// and closes their done channels.
//
// VALIDATES: Close marks broker closed, all client done channels closed.
// PREVENTS: Leaked goroutines after shutdown.
func TestEventBrokerClose(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(100)

	c1 := broker.Subscribe()
	c2 := broker.Subscribe()

	broker.Close()

	if broker.ClientCount() != 0 {
		t.Fatalf("ClientCount() after Close = %d, want 0", broker.ClientCount())
	}

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

	// Subscribe after close should return nil.
	if c := broker.Subscribe(); c != nil {
		t.Fatal("Subscribe after Close should return nil")
	}
}

// TestEventBrokerDefaultMaxClients verifies that a zero or negative
// maxClients defaults to 100.
//
// VALIDATES: Default maxClients value when invalid input provided.
// PREVENTS: Zero maxClients preventing all subscriptions.
func TestEventBrokerDefaultMaxClients(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(0)
	defer broker.Close()

	c := broker.Subscribe()
	if c == nil {
		t.Fatal("Subscribe returned nil with default maxClients")
	}
}

// TestEventBrokerServeHTTP verifies the SSE HTTP handler sends events in
// the correct SSE wire format.
//
// VALIDATES: Content-Type text/event-stream, event/data format, cleanup on disconnect.
// PREVENTS: Malformed SSE output that browsers cannot parse.
func TestEventBrokerServeHTTP(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(100)
	defer broker.Close()

	ts := httptest.NewServer(broker)
	defer ts.Close()

	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, http.NoBody)
	if reqErr != nil {
		t.Fatalf("new request: %v", reqErr)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Log("close response body:", closeErr)
		}
	}()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	// Send an event while connected.
	broker.Broadcast("config-change", "<div>test</div>")

	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, "event: config-change") {
		t.Errorf("response missing 'event: config-change', got: %q", body)
	}

	if !strings.Contains(body, "data: <div>test</div>") {
		t.Errorf("response missing 'data: <div>test</div>', got: %q", body)
	}
}

// TestEventBrokerServeHTTPMaxClients verifies that the HTTP handler returns
// 503 when the broker is at capacity.
//
// VALIDATES: HTTP 503 response when max clients reached.
// PREVENTS: Silent failure when connection limit exceeded.
func TestEventBrokerServeHTTPMaxClients(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(1)
	defer broker.Close()

	// Fill the single slot.
	_ = broker.Subscribe()

	ts := httptest.NewServer(broker)
	defer ts.Close()

	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, http.NoBody)
	if reqErr != nil {
		t.Fatalf("new request: %v", reqErr)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Log("close response body:", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// TestBroadcastConfigChange verifies that BroadcastConfigChange renders
// the notification banner with HTML-escaped reason text and broadcasts it.
//
// VALIDATES: Reason text is HTML-escaped, SSE event type is "config-change".
// PREVENTS: XSS via unescaped reason text in notification banner.
func TestBroadcastConfigChange(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(100)
	defer broker.Close()

	client := broker.Subscribe()

	// Use a reason with HTML special characters to verify escaping.
	BroadcastConfigChange(broker, "alice", "added <script>alert('xss')</script>")

	select {
	case ev := <-client.ch:
		if ev.eventType != "config-change" {
			t.Fatalf("event type = %q, want %q", ev.eventType, "config-change")
		}

		// Verify the reason text is HTML-escaped.
		if strings.Contains(ev.data, "<script>") {
			t.Fatal("reason text was NOT HTML-escaped: contains <script>")
		}

		if !strings.Contains(ev.data, "&lt;script&gt;") {
			t.Fatal("expected HTML-escaped <script> as &lt;script&gt;")
		}

		// Verify the notification structure.
		if !strings.Contains(ev.data, "Config changed by alice") {
			t.Error("missing username in reason text")
		}

		if !strings.Contains(ev.data, "Refresh") {
			t.Error("missing Refresh button")
		}

		if !strings.Contains(ev.data, "Dismiss") {
			t.Error("missing Dismiss button")
		}

		if !strings.Contains(ev.data, "hx-swap-oob") {
			t.Error("missing hx-swap-oob attribute for OOB swap")
		}

		if !strings.Contains(ev.data, `hx-target="#content-area"`) {
			t.Error("missing hx-target for content area refresh")
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive config-change event")
	}
}

// TestBroadcastConfigChangeNilBroker verifies that BroadcastConfigChange
// is a no-op when broker is nil.
//
// VALIDATES: Nil broker does not panic.
// PREVENTS: Nil pointer dereference when SSE is not configured.
func TestBroadcastConfigChangeNilBroker(t *testing.T) {
	t.Parallel()

	// Should not panic.
	BroadcastConfigChange(nil, "alice", "test")
}

// TestSSEEventMultiLine verifies multi-line data is sent with proper
// SSE framing where each line gets its own "data: " prefix.
//
// VALIDATES: Multi-line SSE data formatted correctly.
// PREVENTS: Newlines in HTML breaking SSE event framing.
func TestSSEEventMultiLine(t *testing.T) {
	t.Parallel()

	broker := NewEventBroker(100)
	defer broker.Close()

	ts := httptest.NewServer(broker)
	defer ts.Close()

	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, http.NoBody)
	if reqErr != nil {
		t.Fatalf("new request: %v", reqErr)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Log("close response body:", closeErr)
		}
	}()

	multiLine := "<div class=\"banner\">\n  <span>hello</span>\n</div>"
	broker.Broadcast("config-change", multiLine)

	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, "data: <div class=\"banner\">") {
		t.Errorf("missing first data line, got: %q", body)
	}

	if !strings.Contains(body, "data:   <span>hello</span>") {
		t.Errorf("missing second data line, got: %q", body)
	}

	if !strings.Contains(body, "data: </div>") {
		t.Errorf("missing third data line, got: %q", body)
	}
}
