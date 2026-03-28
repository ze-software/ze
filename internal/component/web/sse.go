// Design: docs/architecture/web-interface.md -- Server-Sent Events for live updates
// Related: handler_config.go -- Commit handler triggers SSE broadcast
// Related: render.go -- Template rendering

package web

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
)

// sseEvent is a server-sent event with a named event type and data payload.
type sseEvent struct {
	eventType string // SSE event field (e.g., "config-change")
	data      string // SSE data field (pre-rendered HTML fragment)
}

// sseClient represents a connected SSE client with a buffered event channel.
type sseClient struct {
	ch   chan sseEvent
	done chan struct{}
}

// EventBroker manages SSE client connections for the web interface and
// broadcasts events to all connected clients. It is safe for concurrent use.
//
// Caller MUST call Close when the broker is no longer needed to release
// all connected clients.
type EventBroker struct {
	mu         sync.Mutex
	clients    map[*sseClient]struct{}
	closed     bool
	maxClients int
}

// NewEventBroker creates a broker that limits the number of concurrent SSE
// clients. A maxClients value of 0 or negative defaults to 100.
func NewEventBroker(maxClients int) *EventBroker {
	if maxClients <= 0 {
		maxClients = 100
	}

	return &EventBroker{
		clients:    make(map[*sseClient]struct{}),
		maxClients: maxClients,
	}
}

// Subscribe registers a new client and returns it. The client's channel has
// a buffer of 16 events. Returns nil if the broker is closed or the maximum
// number of clients has been reached.
func (b *EventBroker) Subscribe() *sseClient {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed || len(b.clients) >= b.maxClients {
		return nil
	}

	c := &sseClient{
		ch:   make(chan sseEvent, 16),
		done: make(chan struct{}),
	}
	b.clients[c] = struct{}{}

	return c
}

// Unsubscribe removes a client from the broker and closes its done channel.
// If Close() already removed this client, this is a no-op.
func (b *EventBroker) Unsubscribe(c *sseClient) {
	b.mu.Lock()
	_, exists := b.clients[c]
	delete(b.clients, c)
	b.mu.Unlock()

	if exists {
		close(c.done)
	}
}

// broadcastEvent sends an event to a single client. Returns true if the event
// was enqueued, false if the client buffer was full (event dropped by design;
// SSE clients that fall behind lose events rather than blocking the broker).
func broadcastEvent(c *sseClient, ev sseEvent) bool {
	select {
	case c.ch <- ev:
		return true
	default: // Non-blocking send: drop event for slow client (by design).
		return false
	}
}

// Broadcast sends an event to all connected clients. Non-blocking: if a
// client's buffer is full, the event is dropped for that client.
func (b *EventBroker) Broadcast(eventType, data string) {
	ev := sseEvent{eventType: eventType, data: data}

	b.mu.Lock()
	defer b.mu.Unlock()

	for c := range b.clients {
		broadcastEvent(c, ev)
	}
}

// ClientCount returns the number of connected clients.
func (b *EventBroker) ClientCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.clients)
}

// Close marks the broker as closed and drains all clients.
func (b *EventBroker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true

	for c := range b.clients {
		close(c.done)
		delete(b.clients, c)
	}
}

// ServeHTTP handles SSE client connections. It sets the appropriate headers,
// registers the client, and streams events until the client disconnects
// or the broker is closed.
func (b *EventBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	client := b.Subscribe()
	if client == nil {
		http.Error(w, "too many SSE clients", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	defer b.Unsubscribe(client)

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-client.done:
			return
		case ev := <-client.ch:
			if writeSSEEvent(w, ev) != nil {
				return
			}

			flusher.Flush()
		}
	}
}

// writeSSEEvent writes a single SSE event to the writer in the standard
// SSE wire format: "event: <type>\ndata: <line1>\ndata: <line2>\n\n".
func writeSSEEvent(w http.ResponseWriter, ev sseEvent) error {
	var err error

	if ev.eventType != "" {
		_, err = fmt.Fprintf(w, "event: %s\n", ev.eventType)
	}

	if err == nil {
		// SSE requires each line of multi-line data to be
		// prefixed with "data: ". Newlines in HTML fragments
		// would otherwise terminate the data field prematurely.
		for line := range strings.SplitSeq(ev.data, "\n") {
			if err != nil {
				break
			}

			_, err = fmt.Fprintf(w, "data: %s\n", line)
		}

		if err == nil {
			_, err = fmt.Fprintf(w, "\n") // Blank line terminates the event.
		}
	}

	return err
}

// notificationBannerSource is the template for rendering config change
// notification banners. Rendered server-side for SSE delivery via
// html/template which auto-escapes the reason text.
// SSE event data is swapped into #notification-bar innerHTML by the HTMX SSE
// extension (sse-swap="config-change"). No OOB wrapper needed.
const notificationBannerSource = `<div class="notification-banner">` +
	`<span class="notification-reason">{{.Reason}}</span>` +
	`<button class="btn btn-sm" hx-get="{{.RefreshURL}}" hx-target="#content-area" hx-swap="innerHTML">Refresh</button>` +
	`<button class="btn btn-sm btn-dismiss" data-action="dismiss-banner">Dismiss</button>` +
	`</div>`

// notificationBannerTmpl is the pre-compiled template for config change
// notification banners delivered via SSE. It uses html/template for
// automatic escaping of the reason text.
var notificationBannerTmpl = template.Must(template.New("notification_banner").Parse(
	notificationBannerSource,
))

// notificationBannerData holds the template data for rendering the notification banner.
type notificationBannerData struct {
	Reason     string
	RefreshURL string
}

// BroadcastConfigChange renders the notification banner template with the
// given username and reason, then broadcasts it as a "config-change" SSE
// event to all connected clients. The reason text is auto-escaped by
// html/template.
func BroadcastConfigChange(broker *EventBroker, username, reason string) {
	if broker == nil {
		return
	}

	data := notificationBannerData{
		Reason:     fmt.Sprintf("Config changed by %s: %s", username, reason),
		RefreshURL: "/show/",
	}

	var buf bytes.Buffer
	if err := notificationBannerTmpl.Execute(&buf, data); err != nil {
		serverLogger.Warn("notification banner render failed", "error", err)
		return
	}

	broker.Broadcast("config-change", buf.String())
}
