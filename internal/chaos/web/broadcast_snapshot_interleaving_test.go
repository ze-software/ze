// Design: docs/architecture/chaos-web-dashboard.md -- atomic broadcast snapshots.
package web

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/core/clock"
)

// broadcastBoundaryClock makes one sampled-time boundary available to the test
// scheduler. It never changes the already sampled value when the callback runs.
type broadcastBoundaryClock struct {
	clock.RealClock
	at       time.Time
	reads    int
	boundary func()
}

func (c *broadcastBoundaryClock) Now() time.Time {
	at := c.at
	c.reads++
	if boundary := c.boundary; boundary != nil {
		c.boundary = nil
		boundary()
	}
	return at
}

// TestDashboardBroadcastSerializesClockAndEventSnapshot observes complete real
// broker batches for the same two authored events and fixed producer times.
// TryLock is only an interleaving controller: it probes whether the writer gate
// is available at the sampled-clock boundary and releases a successful probe
// before calling the real ProcessEvent. A held gate defers that same event until
// the first batch finishes. No blocked-goroutine entry or elapsed-time evidence
// is claimed, and no sleep or goroutine scheduling establishes the interleaving.
// MUTATION: sample Now before the state lock and restore the old separate
// consume/render/clear lock windows. The newer event then enters before the old
// sample is rendered, producing a negative age and consuming its next batch.
func TestDashboardBroadcastSerializesClockAndEventSnapshot(t *testing.T) {
	base := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	initial := peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: base.Add(5 * time.Second)}
	newer := peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, Time: base.Add(20 * time.Second), ChaosAction: "snapshot-newer-event"}
	for _, controlled := range []bool{false, true} {
		name := "serial-control"
		if controlled {
			name = "writer-at-sample"
		}
		t.Run(name, func(t *testing.T) {
			dashboard := newTestDashboard(1)
			defer dashboard.broker.Close()
			runClock := &broadcastBoundaryClock{at: base.Add(10 * time.Second)}
			dashboard.state.clock = runClock
			dashboard.state.StartTime = base
			dashboard.state.Active.Promote(0, PriorityManual, base)
			dashboard.ProcessEvent(initial)
			client := dashboard.broker.Subscribe()
			defer dashboard.broker.Unsubscribe(client)

			delivered := false
			probes := 0
			if controlled {
				runClock.boundary = func() {
					probes++
					if dashboard.state.mu.TryLock() {
						dashboard.state.mu.Unlock()
						dashboard.ProcessEvent(newer)
						delivered = true
					}
				}
			}

			// broadcastDirty is synchronous and this fixture starts no broadcast
			// loop. Its return makes this a complete batch, not a timed sample.
			dashboard.broadcastDirty(false)
			first := takeBroadcastBatch(client)
			assertSnapshotBatch(t, "first", first, 0, "10", "10s", []string{
				`<span class="event-time">5s</span>`, "p0 established",
			}, 1)
			if strings.Contains(strings.Join(first, "\n"), "snapshot-newer-event") {
				t.Error("the first sampled batch contains the later authored chaos event")
			}
			if runClock.reads != 1 {
				t.Errorf("first batch sampled producer time %d times, want 1", runClock.reads)
			}
			wantProbes := 0
			if controlled {
				wantProbes = 1
			}
			if probes != wantProbes {
				t.Errorf("sample-boundary writer probes = %d, want %d", probes, wantProbes)
			}
			if !delivered {
				dashboard.ProcessEvent(newer)
			}
			if !strings.Contains(dashboard.renderPeerCell(0), " pulse") {
				t.Error("the older batch cleared the newer event's unrendered transient pulse")
			}

			runClock.at = base.Add(20 * time.Second)
			dashboard.broadcastDirty(false)
			second := takeBroadcastBatch(client)
			assertSnapshotBatch(t, "second", second, 1, "20", "20s", []string{
				`<span class="event-time">0ms</span>`, "p0 chaos",
				`<span class="event-time">15s</span>`, "p0 established",
			}, 2)
			if count := strings.Count(strings.Join(second, "\n"), "snapshot-newer-event"); count != 1 {
				t.Errorf("second batch's exact newer-event toast count = %d, want 1", count)
			}
			if runClock.reads != 2 {
				t.Errorf("two batches sampled producer time %d times, want 2", runClock.reads)
			}
			if strings.Contains(dashboard.renderPeerCell(0), " pulse") {
				t.Error("the completed newer batch retained its transient pulse")
			}
			dashboard.broadcastDirty(false)
			if repeated := takeBroadcastBatch(client); len(repeated) != 0 {
				t.Errorf("a clean third batch repeated consumed output: %q", repeated)
			}
		})
	}
}

func takeBroadcastBatch(client *sseClient) []string {
	var batch []string
	for len(client.ch) > 0 {
		batch = append(batch, (<-client.ch).Data)
	}
	return batch
}

// assertSnapshotBatch keeps the externally emitted toast, stats/uptime, events
// and peer-update population explicit. It does not regenerate expected HTML by
// calling the same renderer that produced the observed batch.
func assertSnapshotBatch(t *testing.T, name string, batch []string, chaosCount int, uptimeSeconds, uptime string, eventParts []string, eventCount int) {
	t.Helper()
	wantMessages := 3
	if chaosCount != 0 {
		wantMessages++
	}
	if len(batch) != wantMessages {
		t.Errorf("%s complete message count = %d, want %d: %q", name, len(batch), wantMessages, batch)
	}
	counts := make(map[string]int)
	for _, message := range batch {
		switch {
		case strings.HasPrefix(message, `<div hx-swap-oob="beforeend:#toast-container">`):
			counts["toast"]++
			if !strings.Contains(message, "p0 chaos — snapshot-newer-event") {
				t.Errorf("%s toast lost its exact event identity: %s", name, message)
			}
		case strings.HasPrefix(message, `<div id="stats"`):
			counts["stats"]++
			if !strings.Contains(message, `<span id="uptime" data-start="`+uptimeSeconds+`" hx-swap-oob="outerHTML">`+uptime+`</span>`) {
				t.Errorf("%s stats lost its exact sampled uptime: %s", name, message)
			}
		case strings.HasPrefix(message, `<div id="events"`):
			counts["events"]++
			for _, part := range eventParts {
				if !strings.Contains(message, part) {
					t.Errorf("%s event output lacks %q: %s", name, part, message)
				}
			}
			if count := strings.Count(message, `class="event-row"`); count != eventCount {
				t.Errorf("%s event rows = %d, want %d: %s", name, count, eventCount, message)
			}
			if strings.Contains(message, `class="event-time">-`) {
				t.Errorf("%s batch rendered an event newer than its sampled clock: %s", name, message)
			}
		case strings.HasPrefix(message, `<tr id="peer-0" hx-swap-oob="outerHTML">`):
			counts["peer"]++
			lastCell := `<td>0</td></tr>`
			if chaosCount == 1 {
				lastCell = `<td>1</td></tr>`
			}
			if !strings.HasSuffix(message, lastCell) {
				t.Errorf("%s peer update lost its exact chaos count: %s", name, message)
			}
		default:
			t.Errorf("%s batch has an unexpected message: %s", name, message)
		}
	}
	for _, kind := range []string{"stats", "events", "peer"} {
		if counts[kind] != 1 {
			t.Errorf("%s %s message count = %d, want 1", name, kind, counts[kind])
		}
	}
	wantToasts := 0
	if chaosCount != 0 {
		wantToasts = 1
	}
	if counts["toast"] != wantToasts {
		t.Errorf("%s toast message count = %d, want %d", name, counts["toast"], wantToasts)
	}
}
