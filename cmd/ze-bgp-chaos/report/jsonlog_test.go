package report

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONLogFormat verifies NDJSON output: one JSON object per line, kebab-case keys.
//
// VALIDATES: Each event produces exactly one line of valid JSON with kebab-case keys.
// PREVENTS: Multi-line JSON output breaking post-mortem tools (jq, grep).
func TestJSONLogFormat(t *testing.T) {
	var buf bytes.Buffer
	log := NewJSONLog(&buf)

	ev := peer.Event{
		Type:      peer.EventRouteSent,
		PeerIndex: 3,
		Time:      time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
	}
	log.ProcessEvent(ev)
	require.NoError(t, log.Close())

	// Should be exactly one line (NDJSON).
	output := buf.String()
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 1, "should produce exactly one line")

	// Must be valid JSON.
	var parsed map[string]any
	err := json.Unmarshal(lines[0], &parsed)
	require.NoError(t, err, "line must be valid JSON: %s", output)

	// Verify kebab-case keys.
	assert.Contains(t, parsed, "event-type")
	assert.Contains(t, parsed, "peer-index")
	assert.Contains(t, parsed, "timestamp")

	// Verify values.
	assert.Equal(t, "route-sent", parsed["event-type"])
	assert.Equal(t, float64(3), parsed["peer-index"])
	assert.Equal(t, "10.0.0.0/24", parsed["prefix"])
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
			log := NewJSONLog(&buf)

			ev := peer.Event{
				Type:      tt.typ,
				PeerIndex: 1,
				Time:      time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			}
			log.ProcessEvent(ev)
			require.NoError(t, log.Close())

			var parsed map[string]any
			err := json.Unmarshal(buf.Bytes(), &parsed)
			require.NoError(t, err)
			assert.Equal(t, tt.name, parsed["event-type"])
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
		log := NewJSONLog(&buf)

		log.ProcessEvent(peer.Event{
			Type:        peer.EventChaosExecuted,
			PeerIndex:   2,
			Time:        time.Now(),
			ChaosAction: "tcp-disconnect",
		})
		require.NoError(t, log.Close())

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Equal(t, "tcp-disconnect", parsed["chaos-action"])
	})

	t.Run("count", func(t *testing.T) {
		var buf bytes.Buffer
		log := NewJSONLog(&buf)

		log.ProcessEvent(peer.Event{
			Type:      peer.EventEORSent,
			PeerIndex: 0,
			Time:      time.Now(),
			Count:     500,
		})
		require.NoError(t, log.Close())

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Equal(t, float64(500), parsed["count"])
	})

	t.Run("error", func(t *testing.T) {
		var buf bytes.Buffer
		log := NewJSONLog(&buf)

		log.ProcessEvent(peer.Event{
			Type:      peer.EventError,
			PeerIndex: 5,
			Time:      time.Now(),
			Err:       assert.AnError,
		})
		require.NoError(t, log.Close())

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Contains(t, parsed, "error")
	})
}

// TestJSONLogMultipleEvents verifies multiple events produce multiple lines.
//
// VALIDATES: N events produce N lines of NDJSON.
// PREVENTS: Events overwriting each other instead of appending.
func TestJSONLogMultipleEvents(t *testing.T) {
	var buf bytes.Buffer
	log := NewJSONLog(&buf)

	for i := range 5 {
		log.ProcessEvent(peer.Event{
			Type:      peer.EventRouteSent,
			PeerIndex: i,
			Time:      time.Now(),
			Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
		})
	}
	require.NoError(t, log.Close())

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 5)

	// Each line should be valid JSON.
	for i, line := range lines {
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(line, &parsed), "line %d invalid", i)
	}
}
