package report

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// helper: split NDJSON output into parsed lines.
func parseNDJSON(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	result := make([]map[string]any, len(lines))
	for i, line := range lines {
		require.NoError(t, json.Unmarshal(line, &result[i]), "line %d invalid JSON: %s", i, line)
	}
	return result
}

// TestJSONLogHeader verifies the first line is a header with metadata.
//
// VALIDATES: First line has record-type "header" with version, seed, peers.
// PREVENTS: Missing metadata making replay impossible.
func TestJSONLogHeader(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	jlog := NewJSONLog(&buf, JSONLogConfig{
		Start:     start,
		Seed:      42,
		Peers:     4,
		ChaosRate: 0.1,
	})
	require.NoError(t, jlog.Close())

	parsed := parseNDJSON(t, &buf)
	require.Len(t, parsed, 1, "header only, no events")

	hdr := parsed[0]
	assert.Equal(t, "header", hdr["record-type"])
	assert.Equal(t, float64(1), hdr["version"])
	assert.Equal(t, float64(42), hdr["seed"])
	assert.Equal(t, float64(4), hdr["peers"])
	assert.Equal(t, 0.1, hdr["chaos-rate"])
	assert.Contains(t, hdr, "start-time")
}

// TestJSONLogFormat verifies NDJSON output: header + one event line, kebab-case keys.
//
// VALIDATES: Each event produces valid JSON with record-type, seq, time-offset-ms.
// PREVENTS: Multi-line JSON output breaking post-mortem tools (jq, grep).
func TestJSONLogFormat(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	jlog := NewJSONLog(&buf, JSONLogConfig{Start: start, Seed: 1, Peers: 4})

	ev := peer.Event{
		Type:      peer.EventRouteSent,
		PeerIndex: 3,
		Time:      start.Add(150 * time.Millisecond),
		Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
	}
	jlog.ProcessEvent(ev)
	require.NoError(t, jlog.Close())

	parsed := parseNDJSON(t, &buf)
	require.Len(t, parsed, 2, "header + 1 event")

	// Event line (index 1).
	event := parsed[1]
	assert.Equal(t, "event", event["record-type"])
	assert.Equal(t, float64(1), event["seq"])
	assert.Equal(t, float64(150), event["time-offset-ms"])
	assert.Equal(t, "route-sent", event["event-type"])
	assert.Equal(t, float64(3), event["peer-index"])
	assert.Equal(t, "10.0.0.0/24", event["prefix"])

	// No timestamp field (replaced by time-offset-ms).
	assert.NotContains(t, event, "timestamp")
}

// TestJSONLogSequence verifies sequence numbers are monotonically increasing.
//
// VALIDATES: seq starts at 1 and increments by 1 for each event.
// PREVENTS: Duplicate or missing sequence numbers breaking replay ordering.
func TestJSONLogSequence(t *testing.T) {
	var buf bytes.Buffer
	start := time.Now()
	jlog := NewJSONLog(&buf, JSONLogConfig{Start: start, Seed: 1, Peers: 2})

	for i := range 5 {
		jlog.ProcessEvent(peer.Event{
			Type:      peer.EventRouteSent,
			PeerIndex: i % 2,
			Time:      start.Add(time.Duration(i) * time.Millisecond),
			Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
		})
	}
	require.NoError(t, jlog.Close())

	parsed := parseNDJSON(t, &buf)
	require.Len(t, parsed, 6, "header + 5 events")

	for i := 1; i <= 5; i++ {
		assert.Equal(t, float64(i), parsed[i]["seq"], "event %d should have seq %d", i, i)
	}
}

// TestJSONLogTimeOffset verifies time-offset-ms is relative to start.
//
// VALIDATES: time-offset-ms is non-negative and reflects elapsed time from start.
// PREVENTS: Absolute wall-clock timestamps that break cross-machine replay.
func TestJSONLogTimeOffset(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	jlog := NewJSONLog(&buf, JSONLogConfig{Start: start, Seed: 1, Peers: 1})

	jlog.ProcessEvent(peer.Event{
		Type:      peer.EventEstablished,
		PeerIndex: 0,
		Time:      start.Add(500 * time.Millisecond),
	})
	jlog.ProcessEvent(peer.Event{
		Type:      peer.EventRouteSent,
		PeerIndex: 0,
		Time:      start.Add(1500 * time.Millisecond),
		Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
	})
	require.NoError(t, jlog.Close())

	parsed := parseNDJSON(t, &buf)
	require.Len(t, parsed, 3)

	assert.Equal(t, float64(500), parsed[1]["time-offset-ms"])
	assert.Equal(t, float64(1500), parsed[2]["time-offset-ms"])
}

// TestJSONLogAllEvents verifies all 10 event types serialize correctly.
//
// VALIDATES: Every EventType produces valid JSON with correct event-type string.
// PREVENTS: Missing event type in serialization causing empty or wrong output.
func TestJSONLogAllEvents(t *testing.T) {
	allTypes := []struct {
		typ  peer.EventType
		name string
	}{
		{peer.EventEstablished, "established"},
		{peer.EventRouteSent, "route-sent"},
		{peer.EventRouteReceived, "route-received"},
		{peer.EventRouteWithdrawn, "route-withdrawn"},
		{peer.EventEORSent, "eor-sent"},
		{peer.EventDisconnected, "disconnected"},
		{peer.EventError, "error"},
		{peer.EventChaosExecuted, "chaos-executed"},
		{peer.EventReconnecting, "reconnecting"},
		{peer.EventWithdrawalSent, "withdrawal-sent"},
	}

	for _, tt := range allTypes {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			jlog := NewJSONLog(&buf, JSONLogConfig{Start: time.Now(), Seed: 1, Peers: 2})

			ev := peer.Event{
				Type:      tt.typ,
				PeerIndex: 1,
				Time:      time.Now(),
			}
			jlog.ProcessEvent(ev)
			require.NoError(t, jlog.Close())

			parsed := parseNDJSON(t, &buf)
			require.Len(t, parsed, 2) // header + event
			assert.Equal(t, tt.name, parsed[1]["event-type"])
			assert.Equal(t, "event", parsed[1]["record-type"])
		})
	}
}

// TestJSONLogOptionalFields verifies optional fields are included when set.
//
// VALIDATES: Prefix, Count, ChaosAction, Err appear only when relevant.
// PREVENTS: Null/zero fields cluttering output for events that don't use them.
func TestJSONLogOptionalFields(t *testing.T) {
	t.Run("chaos-action", func(t *testing.T) {
		var buf bytes.Buffer
		jlog := NewJSONLog(&buf, JSONLogConfig{Start: time.Now(), Seed: 1, Peers: 3})

		jlog.ProcessEvent(peer.Event{
			Type:        peer.EventChaosExecuted,
			PeerIndex:   2,
			Time:        time.Now(),
			ChaosAction: "tcp-disconnect",
		})
		require.NoError(t, jlog.Close())

		parsed := parseNDJSON(t, &buf)
		require.Len(t, parsed, 2)
		assert.Equal(t, "tcp-disconnect", parsed[1]["chaos-action"])
	})

	t.Run("count", func(t *testing.T) {
		var buf bytes.Buffer
		jlog := NewJSONLog(&buf, JSONLogConfig{Start: time.Now(), Seed: 1, Peers: 1})

		jlog.ProcessEvent(peer.Event{
			Type:      peer.EventEORSent,
			PeerIndex: 0,
			Time:      time.Now(),
			Count:     500,
		})
		require.NoError(t, jlog.Close())

		parsed := parseNDJSON(t, &buf)
		require.Len(t, parsed, 2)
		assert.Equal(t, float64(500), parsed[1]["count"])
	})

	t.Run("error", func(t *testing.T) {
		var buf bytes.Buffer
		jlog := NewJSONLog(&buf, JSONLogConfig{Start: time.Now(), Seed: 1, Peers: 6})

		jlog.ProcessEvent(peer.Event{
			Type:      peer.EventError,
			PeerIndex: 5,
			Time:      time.Now(),
			Err:       assert.AnError,
		})
		require.NoError(t, jlog.Close())

		parsed := parseNDJSON(t, &buf)
		require.Len(t, parsed, 2)
		assert.Contains(t, parsed[1], "error")
	})
}

// TestJSONLogControlRecord verifies LogControl writes a "control" record type.
//
// VALIDATES: Control events have record-type "control", seq, command, and value fields.
// PREVENTS: Control events being serialized as "event" records (breaking replay).
func TestJSONLogControlRecord(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	jlog := NewJSONLog(&buf, JSONLogConfig{Start: start, Seed: 1, Peers: 4})

	// Log a control event.
	jlog.LogControl("pause", "", start.Add(2*time.Second))
	jlog.LogControl("rate", "0.50", start.Add(3*time.Second))
	require.NoError(t, jlog.Close())

	parsed := parseNDJSON(t, &buf)
	require.Len(t, parsed, 3, "header + 2 control records")

	// First control record: pause.
	ctrl1 := parsed[1]
	assert.Equal(t, "control", ctrl1["record-type"])
	assert.Equal(t, float64(1), ctrl1["seq"])
	assert.Equal(t, float64(2000), ctrl1["time-offset-ms"])
	assert.Equal(t, "pause", ctrl1["command"])
	assert.NotContains(t, ctrl1, "value", "empty value should be omitted")

	// Second control record: rate with value.
	ctrl2 := parsed[2]
	assert.Equal(t, "control", ctrl2["record-type"])
	assert.Equal(t, float64(2), ctrl2["seq"])
	assert.Equal(t, float64(3000), ctrl2["time-offset-ms"])
	assert.Equal(t, "rate", ctrl2["command"])
	assert.Equal(t, "0.50", ctrl2["value"])
}

// TestJSONLogControlSequenceShared verifies control and event records share a single sequence.
//
// VALIDATES: Sequence numbers are global across events and controls.
// PREVENTS: Duplicate seq values causing ordering ambiguity in replay.
func TestJSONLogControlSequenceShared(t *testing.T) {
	var buf bytes.Buffer
	start := time.Now()
	jlog := NewJSONLog(&buf, JSONLogConfig{Start: start, Seed: 1, Peers: 2})

	jlog.ProcessEvent(peer.Event{
		Type: peer.EventEstablished, PeerIndex: 0, Time: start,
	})
	jlog.LogControl("pause", "", start.Add(time.Second))
	jlog.ProcessEvent(peer.Event{
		Type: peer.EventRouteSent, PeerIndex: 0, Time: start.Add(2 * time.Second),
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
	})
	require.NoError(t, jlog.Close())

	parsed := parseNDJSON(t, &buf)
	require.Len(t, parsed, 4, "header + event + control + event")

	assert.Equal(t, float64(1), parsed[1]["seq"])
	assert.Equal(t, "event", parsed[1]["record-type"])
	assert.Equal(t, float64(2), parsed[2]["seq"])
	assert.Equal(t, "control", parsed[2]["record-type"])
	assert.Equal(t, float64(3), parsed[3]["seq"])
	assert.Equal(t, "event", parsed[3]["record-type"])
}

// TestJSONLogMultipleEvents verifies multiple events produce header + N event lines.
//
// VALIDATES: N events produce 1 header + N event lines of NDJSON.
// PREVENTS: Events overwriting each other instead of appending.
func TestJSONLogMultipleEvents(t *testing.T) {
	var buf bytes.Buffer
	start := time.Now()
	jlog := NewJSONLog(&buf, JSONLogConfig{Start: start, Seed: 1, Peers: 5})

	for i := range 5 {
		jlog.ProcessEvent(peer.Event{
			Type:      peer.EventRouteSent,
			PeerIndex: i,
			Time:      start.Add(time.Duration(i) * time.Millisecond),
			Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
		})
	}
	require.NoError(t, jlog.Close())

	parsed := parseNDJSON(t, &buf)
	assert.Len(t, parsed, 6, "header + 5 events")

	// Each event line should have record-type "event".
	for i := 1; i <= 5; i++ {
		assert.Equal(t, "event", parsed[i]["record-type"])
	}
}

// TestJSONLogConcurrentSafety verifies ProcessEvent and LogControl are safe
// for concurrent use from multiple goroutines.
//
// VALIDATES: No data race when event loop and HTTP handlers write simultaneously.
// PREVENTS: Corrupted NDJSON output, duplicate/missing sequence numbers, panics.
func TestJSONLogConcurrentSafety(t *testing.T) {
	var buf bytes.Buffer
	start := time.Now()
	jlog := NewJSONLog(&buf, JSONLogConfig{Start: start, Seed: 1, Peers: 4})

	const numEvents = 50
	const numControls = 20

	var wg sync.WaitGroup
	wg.Add(2)

	// Simulate event loop goroutine.
	go func() {
		defer wg.Done()
		for i := range numEvents {
			jlog.ProcessEvent(peer.Event{
				Type:      peer.EventRouteSent,
				PeerIndex: i % 4,
				Time:      start.Add(time.Duration(i) * time.Millisecond),
				Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
			})
		}
	}()

	// Simulate HTTP handler goroutine.
	go func() {
		defer wg.Done()
		for i := range numControls {
			jlog.LogControl("pause", "", start.Add(time.Duration(i)*time.Second))
		}
	}()

	wg.Wait()
	require.NoError(t, jlog.Close())

	parsed := parseNDJSON(t, &buf)
	// header + numEvents + numControls
	require.Len(t, parsed, 1+numEvents+numControls)

	// Collect all sequence numbers — must be unique and cover 1..N.
	seqs := make(map[float64]bool)
	for i := 1; i < len(parsed); i++ {
		seq, ok := parsed[i]["seq"].(float64)
		require.True(t, ok, "line %d missing seq", i)
		require.False(t, seqs[seq], "duplicate seq %v at line %d", seq, i)
		seqs[seq] = true
	}
	assert.Len(t, seqs, numEvents+numControls, "all sequence numbers unique")
}
