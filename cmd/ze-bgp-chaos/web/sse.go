package web

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SSEEvent is a server-sent event with a named event type and data payload.
type SSEEvent struct {
	Event string // SSE event field (e.g., "peer-update", "tick", "stats")
	Data  string // SSE data field (HTML fragment or JSON)
}

// sseClient represents a connected SSE client.
type sseClient struct {
	ch   chan SSEEvent
	done chan struct{}
}

// SSEBroker manages SSE client connections and broadcasts events.
// Clients register via the HTTP handler. The broker's Run goroutine
// reads dirty flags from DashboardState at a configurable interval
// and broadcasts rendered HTML fragments to all connected clients.
type SSEBroker struct {
	mu       sync.Mutex
	clients  map[*sseClient]struct{}
	closed   bool
	interval time.Duration
}

// NewSSEBroker creates a broker with the given debounce interval.
func NewSSEBroker(debounceInterval time.Duration) *SSEBroker {
	if debounceInterval < 50*time.Millisecond {
		debounceInterval = 50 * time.Millisecond
	}
	return &SSEBroker{
		clients:  make(map[*sseClient]struct{}),
		interval: debounceInterval,
	}
}

// Subscribe registers a new client and returns its event channel and done signal.
// The caller should call Unsubscribe when the client disconnects.
func (b *SSEBroker) Subscribe() *sseClient {
	c := &sseClient{
		ch:   make(chan SSEEvent, 64),
		done: make(chan struct{}),
	}
	b.mu.Lock()
	b.clients[c] = struct{}{}
	b.mu.Unlock()
	return c
}

// Unsubscribe removes a client from the broker.
// If Close() already removed and signaled this client, this is a no-op.
func (b *SSEBroker) Unsubscribe(c *sseClient) {
	b.mu.Lock()
	_, exists := b.clients[c]
	delete(b.clients, c)
	b.mu.Unlock()
	if exists {
		close(c.done)
	}
}

// Broadcast sends an event to all connected clients.
// Non-blocking: if a client's buffer is full, the event is dropped for that client.
func (b *SSEBroker) Broadcast(ev SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.clients {
		select {
		case c.ch <- ev:
		default:
			// Client buffer full — drop event to avoid blocking.
		}
	}
}

// ClientCount returns the number of connected clients.
func (b *SSEBroker) ClientCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients)
}

// Close marks the broker as closed and drains all clients.
func (b *SSEBroker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for c := range b.clients {
		close(c.done)
		delete(b.clients, c)
	}
}

// Interval returns the debounce interval.
func (b *SSEBroker) Interval() time.Duration {
	return b.interval
}

// ServeHTTP handles SSE client connections. It sets the appropriate headers,
// registers the client, and streams events until the client disconnects
// or the broker is closed.
func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	client := b.Subscribe()
	defer b.Unsubscribe(client)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.done:
			return
		case ev := <-client.ch:
			var err error
			if ev.Event != "" {
				_, err = fmt.Fprintf(w, "event: %s\n", ev.Event)
			}
			if err == nil {
				// SSE requires each line of multi-line data to be
				// prefixed with "data: ". Newlines in HTML fragments
				// would otherwise terminate the data field prematurely.
				for line := range strings.SplitSeq(ev.Data, "\n") {
					if err != nil {
						break
					}
					_, err = fmt.Fprintf(w, "data: %s\n", line)
				}
				if err == nil {
					_, err = fmt.Fprintf(w, "\n") // Blank line terminates the event.
				}
			}
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
