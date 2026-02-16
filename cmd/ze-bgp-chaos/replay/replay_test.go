package replay

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: build NDJSON from header + event lines.
func buildNDJSON(header map[string]any, events ...map[string]any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	_ = enc.Encode(header)
	for _, ev := range events {
		_ = enc.Encode(ev)
	}
	return buf.String()
}

func makeHeader(seed uint64, peers int) map[string]any {
	return map[string]any{
		"record-type": "header",
		"version":     1,
		"seed":        seed,
		"peers":       peers,
		"chaos-rate":  0.0,
		"start-time":  "2025-01-15T10:30:00Z",
	}
}

func makeEvent(seq int, typ string, peerIdx int, opts ...func(map[string]any)) map[string]any {
	ev := map[string]any{
		"record-type":    "event",
		"seq":            seq,
		"time-offset-ms": seq * 10,
		"event-type":     typ,
		"peer-index":     peerIdx,
	}
	for _, opt := range opts {
		opt(ev)
	}
	return ev
}

func withPrefix(p string) func(map[string]any) {
	return func(m map[string]any) { m["prefix"] = p }
}

func withCount(c int) func(map[string]any) {
	return func(m map[string]any) { m["count"] = c }
}

func withChaosAction(a string) func(map[string]any) {
	return func(m map[string]any) { m["chaos-action"] = a }
}

// TestReplayBasic verifies that replaying a simple passing run produces PASS.
//
// VALIDATES: Replay feeds events through validation model and reports PASS.
// PREVENTS: Replay engine failing to reconstruct validation state.
func TestReplayBasic(t *testing.T) {
	// Scenario: 2 peers, peer 0 establishes, sends route, peer 1 establishes, receives route.
	input := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "established", 1),
		makeEvent(3, "route-sent", 0, withPrefix("10.0.0.0/24")),
		makeEvent(4, "route-received", 1, withPrefix("10.0.0.0/24")),
	)

	var out bytes.Buffer
	exitCode := Run(strings.NewReader(input), &out)

	assert.Equal(t, 0, exitCode, "should PASS")
	assert.Contains(t, out.String(), "PASS")
}

// TestReplayMissingRoute verifies that replay detects missing routes.
//
// VALIDATES: Replay reports FAIL when expected route was not received.
// PREVENTS: Replay silently passing when routes are missing.
func TestReplayMissingRoute(t *testing.T) {
	// Scenario: peer 0 announces, peer 1 never receives → FAIL.
	input := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "established", 1),
		makeEvent(3, "route-sent", 0, withPrefix("10.0.0.0/24")),
		// No route-received for peer 1.
	)

	var out bytes.Buffer
	exitCode := Run(strings.NewReader(input), &out)

	assert.Equal(t, 1, exitCode, "should FAIL")
	assert.Contains(t, out.String(), "FAIL")
}

// TestReplayWithDisconnect verifies disconnect clears expected routes.
//
// VALIDATES: Disconnect removes source peer's routes from expected set.
// PREVENTS: Stale routes causing false failures after disconnect.
func TestReplayWithDisconnect(t *testing.T) {
	// Scenario: peer 0 announces, peer 1 receives, peer 0 disconnects → model clears.
	input := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "established", 1),
		makeEvent(3, "route-sent", 0, withPrefix("10.0.0.0/24")),
		makeEvent(4, "route-received", 1, withPrefix("10.0.0.0/24")),
		makeEvent(5, "disconnected", 0),
	)

	var out bytes.Buffer
	exitCode := Run(strings.NewReader(input), &out)

	// After disconnect, peer 0's routes are removed from expected.
	// Peer 1 still has the route in actual → extra. That's expected behavior
	// (tracker doesn't auto-clear on source disconnect — RR sends withdrawals).
	// But model no longer expects it, so it shows as extra → FAIL.
	assert.Equal(t, 1, exitCode, "extra route after disconnect should FAIL")
}

// TestReplayCounters verifies aggregate counters in the summary.
//
// VALIDATES: Announced/Received/ChaosEvents counters are correct.
// PREVENTS: Counter logic bugs in replay path.
func TestReplayCounters(t *testing.T) {
	input := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "established", 1),
		makeEvent(3, "route-sent", 0, withPrefix("10.0.0.0/24")),
		makeEvent(4, "route-sent", 0, withPrefix("10.0.1.0/24")),
		makeEvent(5, "route-received", 1, withPrefix("10.0.0.0/24")),
		makeEvent(6, "route-received", 1, withPrefix("10.0.1.0/24")),
		makeEvent(7, "chaos-executed", 0, withChaosAction("tcp-disconnect")),
		makeEvent(8, "withdrawal-sent", 0, withCount(5)),
	)

	var out bytes.Buffer
	exitCode := Run(strings.NewReader(input), &out)

	output := out.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "2 announced")
	assert.Contains(t, output, "2 received")
	assert.Contains(t, output, "1 events") // chaos
}

// TestReplayBadInput verifies error handling for invalid input.
//
// VALIDATES: Replay fails gracefully on malformed NDJSON.
// PREVENTS: Panic on invalid input.
func TestReplayBadInput(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var out bytes.Buffer
		exitCode := Run(strings.NewReader(""), &out)
		assert.Equal(t, 2, exitCode)
	})

	t.Run("no-header", func(t *testing.T) {
		// Event line without header.
		input := `{"record-type":"event","seq":1,"event-type":"established","peer-index":0}` + "\n"
		var out bytes.Buffer
		exitCode := Run(strings.NewReader(input), &out)
		assert.Equal(t, 2, exitCode)
	})

	t.Run("invalid-json", func(t *testing.T) {
		input := "not json\n"
		var out bytes.Buffer
		exitCode := Run(strings.NewReader(input), &out)
		assert.Equal(t, 2, exitCode)
	})
}

// TestReplayEventTypes verifies all event types are handled without error.
//
// VALIDATES: All 10 event types are recognized during replay.
// PREVENTS: Unknown event type causing replay to fail.
func TestReplayEventTypes(t *testing.T) {
	eventTypes := []string{
		"established", "route-sent", "route-received", "route-withdrawn",
		"eor-sent", "disconnected", "error", "chaos-executed",
		"reconnecting", "withdrawal-sent",
	}

	events := make([]map[string]any, len(eventTypes))
	for i, et := range eventTypes {
		ev := makeEvent(i+1, et, 0)
		if et == "route-sent" || et == "route-received" || et == "route-withdrawn" {
			ev["prefix"] = "10.0.0.0/24"
		}
		if et == "withdrawal-sent" || et == "eor-sent" {
			ev["count"] = 5
		}
		events[i] = ev
	}

	input := buildNDJSON(makeHeader(1, 1), events...)

	var out bytes.Buffer
	exitCode := Run(strings.NewReader(input), &out)

	// Exit code doesn't matter — just verify no panic.
	require.True(t, exitCode == 0 || exitCode == 1, "should not return error exit code 2")
}
