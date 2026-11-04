// Design: docs/architecture/chaos-web-dashboard.md -- one run clock at HTTP and SSE consumers.
package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/report"
	"github.com/ze-software/ze/internal/core/clock"
)

// consumerClock controls the producer time, not browser/editor execution time.
// RealClock supplies unused timer operations; the broadcast ticker is owned here.
type consumerClock struct {
	clock.RealClock
	mu    sync.Mutex
	at    time.Time
	reads int
	ticks chan time.Time
	ready chan struct{}
}

func (c *consumerClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reads++
	return c.at
}

func (c *consumerClock) reset(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = at
	c.reads = 0
}

func (c *consumerClock) samples() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

func (c *consumerClock) NewTicker(time.Duration) clock.Ticker {
	select {
	case c.ready <- struct{}{}:
	default:
	}
	return consumerTicker{ticks: c.ticks}
}

type consumerTicker struct{ ticks <-chan time.Time }

func (consumerTicker) Stop()                 {}
func (t consumerTicker) C() <-chan time.Time { return t.ticks }

func newConsumerClock() *consumerClock {
	return &consumerClock{
		at:    time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		ticks: make(chan time.Time, 1), ready: make(chan struct{}, 1),
	}
}

func clockConsumerDashboard(t *testing.T, c *consumerClock) (*Dashboard, *http.ServeMux) {
	t.Helper()
	mux := http.NewServeMux()
	d, err := New(Config{Mux: mux, PeerCount: 24, MaxVisible: 10, Clock: c,
		DebounceInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	return d, mux
}

func serveClockConsumer(t *testing.T, mux *http.ServeMux, method, path string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), method, path, http.NoBody))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s %s: status %d: %s", method, path, recorder.Code, recorder.Body.String())
	}
	return recorder.Body.String()
}

// TestDashboardConstructorUsesRunClock checks the timestamp and initial activity
// written by the public constructor, not a timestamp patched by a capture fixture.
func TestDashboardConstructorUsesRunClock(t *testing.T) {
	c := newConsumerClock()
	mux := http.NewServeMux()
	d, err := New(Config{Mux: mux, PeerCount: 2, Clock: c, DebounceInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	if !d.state.StartTime.Equal(c.at) {
		t.Errorf("start = %v, want run clock %v", d.state.StartTime, c.at)
	}
	for _, idx := range d.state.Active.Indices() {
		if !d.state.Active.Entry(idx).LastActive.Equal(c.at) {
			t.Errorf("initial peer %d did not use run clock", idx)
		}
	}
	if got := c.samples(); got != 1 {
		t.Errorf("constructor sampled clock %d times, want once", got)
	}
}

// TestDashboardHTTPClockConsumers advances one controlled run across all reached
// HTTP renderers. Exact ages/durations and a single sample prohibit wall-clock
// reads in nested layouts, panel dispatch and the two timeline windows.
func TestDashboardHTTPClockConsumers(t *testing.T) {
	c := newConsumerClock()
	d, mux := clockConsumerDashboard(t, c)
	base := c.at
	d.state.StartTime = base
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: base.Add(5 * time.Second)})
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, Time: base.Add(5 * time.Second), ChaosAction: "tcp-disconnect"})
	for _, elapsed := range []time.Duration{20 * time.Second, 80 * time.Second} {
		age := formatElapsed(elapsed - 5*time.Second)
		for _, entry := range []struct{ path, want string }{
			{"/", `data-start="` + itoa(int(elapsed.Seconds())) + `">` + FormatDuration(elapsed)},
			{"/peer/0", age + " ago"},
			{"/sidebar/events", `class="event-time">` + age + "</span>"},
			{"/viz/events", age + " ago"},
			{"/viz/chaos-timeline", `class="stat-value">` + FormatDuration(elapsed) + "</span>"},
			{"/viz/peer-timeline", `--timeline-duration:` + itoa(int(elapsed.Seconds()))},
			{"/viz/panel-content?panel=0&viz=events", age + " ago"},
			{"/viz/panels?p0=events&p1=peer-timeline&p2=chaos-timeline", "panel-grid"},
		} {
			t.Run(entry.path+"/"+elapsed.String(), func(t *testing.T) {
				c.reset(base.Add(elapsed))
				body := serveClockConsumer(t, mux, http.MethodGet, entry.path)
				if !strings.Contains(body, entry.want) {
					t.Errorf("render does not contain controlled value %q", entry.want)
				}
				if got := c.samples(); got != 1 {
					t.Errorf("render sampled clock %d times, want once", got)
				}
			})
		}
	}
}

// TestDashboardSyncAndManualActivityUseRunClock checks state reached by an EOR
// and registered POST routes, including protection from immediately expiring a
// manually promoted peer when broadcast decay runs in the same time domain.
func TestDashboardSyncAndManualActivityUseRunClock(t *testing.T) {
	c := newConsumerClock()
	d, mux := clockConsumerDashboard(t, c)
	base := c.at
	d.state.StartTime = base
	c.reset(base.Add(4500 * time.Millisecond))
	for idx := range 24 {
		d.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: idx, Time: base})
	}
	if got := d.state.SyncDuration; got != 4500*time.Millisecond {
		t.Errorf("sync = %v, want 4.5s", got)
	}
	for _, path := range []string{"/peers/promote?id=1", "/peers/2/pin"} {
		c.reset(base.Add(10 * time.Second))
		serveClockConsumer(t, mux, http.MethodPost, path)
	}
	for _, idx := range []int{1, 2} {
		if got := d.state.Active.Entry(idx).LastActive; !got.Equal(base.Add(10 * time.Second)) {
			t.Errorf("manual peer %d activity = %v", idx, got)
		}
	}
	c.reset(base.Add(131 * time.Second))
	d.broadcastDirty(false)
	if d.state.Active.Contains(1) {
		t.Error("manual peer survived controlled 121-second idle age")
	}
	if !d.state.Active.Contains(2) {
		t.Error("pinned peer expired")
	}
}

// TestDashboardBroadcastUsesRunClock proves the broadcast producer timestamps
// its rendered event ages and throughput with the same single sample.
func TestDashboardBroadcastUsesRunClock(t *testing.T) {
	c := newConsumerClock()
	d, _ := clockConsumerDashboard(t, c)
	base := c.at
	d.state.StartTime = base
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: base})
	client := d.broker.Subscribe()
	defer d.broker.Unsubscribe(client)
	c.reset(base.Add(20 * time.Second))
	d.broadcastDirty(false)
	var events string
	for len(client.ch) > 0 {
		message := <-client.ch
		if strings.Contains(message.Data, `id="events"`) {
			events = message.Data
		}
	}
	if !strings.Contains(events, `class="event-time">20s</span>`) {
		t.Errorf("SSE producer did not render controlled 20s age: %s", events)
	}
	if got := c.samples(); got != 1 {
		t.Errorf("broadcast sampled clock %d times, want once", got)
	}
	if !d.state.lastThroughputAt.Equal(base.Add(20 * time.Second)) {
		t.Error("throughput sample used another time domain")
	}
}

// TestDashboardBroadcastTickerUsesRunClock drives the actual owned broadcast
// loop through its injected ticker and joins it before returning.
func TestDashboardBroadcastTickerUsesRunClock(t *testing.T) {
	c := newConsumerClock()
	d := newTestDashboard(24)
	d.state.clock = c
	defer d.broker.Close()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); d.runBroadcastLoop(ctx) }()
	defer func() { cancel(); <-done }()
	select {
	case <-c.ready:
	case <-time.After(time.Second):
		t.Fatal("broadcast loop did not subscribe to the run clock ticker")
	}
	client := d.broker.Subscribe()
	defer d.broker.Unsubscribe(client)
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: c.at})
	c.reset(c.at.Add(20 * time.Second))
	c.ticks <- c.at
	select {
	case <-client.ch:
	case <-time.After(time.Second):
		t.Fatal("controlled tick did not publish dirty state")
	}
}

// TestDashboardHTTPStreamUsesRunClock observes the actual registered /events
// stream, including its wire framing, after a controlled producer-clock advance.
func TestDashboardHTTPStreamUsesRunClock(t *testing.T) {
	c := newConsumerClock()
	d, mux := clockConsumerDashboard(t, c)
	base := c.at
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: base})
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/events", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if got := response.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("stream content type = %q", got)
	}
	waitForClient(t, d.broker)
	c.reset(base.Add(20 * time.Second))
	d.broadcastDirty(false)
	reader := bufio.NewReader(response.Body)
	var message strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE events: %v", err)
		}
		if line != "\n" {
			if !strings.HasPrefix(line, "data: ") {
				t.Fatalf("unexpected SSE field %q", line)
			}
			message.WriteString(strings.TrimPrefix(line, "data: "))
			continue
		}
		if strings.Contains(message.String(), `id="events"`) {
			break
		}
		message.Reset()
	}
	if !strings.Contains(message.String(), `class="event-time">20s</span>`) {
		t.Errorf("HTTP SSE did not preserve controlled producer age: %s", message.String())
	}
}

// TestDashboardControlLogUsesRunClock consumes the real NDJSON producer after
// a registered HTTP control action, so the persisted offset shares the run clock.
func TestDashboardControlLogUsesRunClock(t *testing.T) {
	c := newConsumerClock()
	var output bytes.Buffer
	log := report.NewJSONLog(&output, report.JSONLogConfig{Start: c.at})
	mux := http.NewServeMux()
	commands := make(chan ControlCommand, 1)
	d, err := New(Config{Mux: mux, PeerCount: 1, Clock: c, Control: commands,
		ControlLogger: log, DebounceInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	c.reset(c.at.Add(2 * time.Second))
	serveClockConsumer(t, mux, http.MethodPost, "/control/pause")
	if got := <-commands; got.Type != "pause" {
		t.Errorf("control command = %q", got.Type)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var header json.RawMessage
	if err := decoder.Decode(&header); err != nil {
		t.Fatal(err)
	}
	var record struct {
		Kind    string `json:"record-type"`
		Command string `json:"command"`
		Offset  int64  `json:"time-offset-ms"`
	}
	if err := decoder.Decode(&record); err != nil {
		t.Fatal(err)
	}
	if record.Kind != "control" || record.Command != "pause" || record.Offset != 2000 {
		t.Errorf("control log = %+v, want pause at controlled offset 2000ms", record)
	}
	if got := c.samples(); got != 1 {
		t.Errorf("control action sampled clock %d times, want once", got)
	}
}
