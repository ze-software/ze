package monitor

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"os"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

func TestMain(m *testing.M) {
	// Register bgp namespace since bgp/server/register.go init() doesn't run in this test package.
	_ = events.RegisterNamespace(bgpevents.Namespace,
		bgpevents.EventUpdate, bgpevents.EventOpen, bgpevents.EventNotification,
		bgpevents.EventKeepalive, bgpevents.EventRefresh, bgpevents.EventState,
		bgpevents.EventNegotiated, bgpevents.EventEOR, bgpevents.EventCongested,
		bgpevents.EventResumed, bgpevents.EventRPKI, bgpevents.EventListenerReady,
		bgpevents.EventUpdateNotification, events.DirectionSent,
	)
	os.Exit(m.Run())
}

// syncBuffer is a thread-safe bytes.Buffer for concurrent read/write in tests.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *syncBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

// =============================================================================
// Monitor Arg Parsing Tests
// =============================================================================

// TestParseMonitorArgs verifies basic keyword parsing.
//
// VALIDATES: Individual keywords (peer, event, direction) parse correctly.
// PREVENTS: Keyword parsing failures for valid inputs.
func TestParseMonitorArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPeer  string
		wantDir   string
		wantTypes []string
	}{
		{
			name:      "peer_filter",
			args:      []string{"peer", "10.0.0.1"},
			wantPeer:  "10.0.0.1",
			wantDir:   "",
			wantTypes: nil,
		},
		{
			name:      "event_filter",
			args:      []string{"event", "update"},
			wantPeer:  "",
			wantDir:   "",
			wantTypes: []string{"update"},
		},
		{
			name:      "direction_received",
			args:      []string{"direction", "received"},
			wantPeer:  "",
			wantDir:   "received",
			wantTypes: nil,
		},
		{
			name:      "direction_sent",
			args:      []string{"direction", "sent"},
			wantPeer:  "",
			wantDir:   "sent",
			wantTypes: nil,
		},
		{
			name:      "all_keywords",
			args:      []string{"peer", "10.0.0.1", "event", "update", "direction", "received"},
			wantPeer:  "10.0.0.1",
			wantDir:   "received",
			wantTypes: []string{"update"},
		},
		{
			name:      "peer_glob",
			args:      []string{"peer", "*"},
			wantPeer:  "*",
			wantDir:   "",
			wantTypes: nil,
		},
		{
			name:      "peer_name",
			args:      []string{"peer", "upstream-1"},
			wantPeer:  "upstream-1",
			wantDir:   "",
			wantTypes: nil,
		},
		{
			name:      "peer_exclusion",
			args:      []string{"peer", "!10.0.0.1"},
			wantPeer:  "!10.0.0.1",
			wantDir:   "",
			wantTypes: nil,
		},
		{
			name:      "peer_name_exclusion",
			args:      []string{"peer", "!upstream-1"},
			wantPeer:  "!upstream-1",
			wantDir:   "",
			wantTypes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseMonitorArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPeer, opts.peer)
			assert.Equal(t, tt.wantDir, opts.direction)
			assert.Equal(t, tt.wantTypes, opts.eventTypes)
		})
	}
}

// TestParseMonitorArgsMultipleEvents verifies comma-separated event types.
//
// VALIDATES: "event update,state" expands to two event types.
// PREVENTS: Comma-separated events not split correctly.
func TestParseMonitorArgsMultipleEvents(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantTypes []string
	}{
		{
			name:      "two_events",
			args:      []string{"event", "update,state"},
			wantTypes: []string{"update", "state"},
		},
		{
			name:      "three_events",
			args:      []string{"event", "update,state,open"},
			wantTypes: []string{"update", "state", "open"},
		},
		{
			name:      "all_bgp_events",
			args:      []string{"event", "update,open,notification,keepalive,refresh,state,negotiated,eor"},
			wantTypes: []string{"update", "open", "notification", "keepalive", "refresh", "state", "negotiated", "eor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseMonitorArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTypes, opts.eventTypes)
		})
	}
}

// TestParseMonitorArgsInvalid verifies error handling for bad inputs.
//
// VALIDATES: Invalid keywords, values, and missing arguments return errors.
// PREVENTS: Invalid monitor args silently accepted.
func TestParseMonitorArgsInvalid(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown_keyword", []string{"foo", "bar"}},
		{"missing_peer_value", []string{"peer"}},
		{"missing_event_value", []string{"event"}},
		{"missing_direction_value", []string{"direction"}},
		{"invalid_direction", []string{"direction", "inbound"}},
		{"invalid_event_type", []string{"event", "unknown"}},
		{"invalid_event_in_list", []string{"event", "update,bogus"}},
		{"empty_event_in_list", []string{"event", "update,,state"}},
		{"duplicate_keyword", []string{"peer", "10.0.0.1", "peer", "10.0.0.2"}},
		{"empty_exclusion", []string{"peer", "!"}},
		{"double_exclusion", []string{"peer", "!!10.0.0.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseMonitorArgs(tt.args)
			require.Error(t, err, "expected error for args: %v", tt.args)
		})
	}
}

// TestParseMonitorArgsDefaults verifies no-args defaults.
//
// VALIDATES: No args → all events, all peers, both directions.
// PREVENTS: Defaults not applied correctly.
func TestParseMonitorArgsDefaults(t *testing.T) {
	opts, err := parseMonitorArgs(nil)
	require.NoError(t, err)
	assert.Empty(t, opts.peer, "default peer should be empty (all peers)")
	assert.Empty(t, opts.direction, "default direction should be empty (both)")
	assert.Nil(t, opts.eventTypes, "default event types should be nil (all events)")
}

// TestParseMonitorArgsKeywordOrder verifies keywords work in any order.
//
// VALIDATES: "direction received peer 10.0.0.1 event update" parses same as canonical order.
// PREVENTS: Parser depends on keyword ordering.
func TestParseMonitorArgsKeywordOrder(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"canonical", []string{"peer", "10.0.0.1", "event", "update", "direction", "received"}},
		{"reversed", []string{"direction", "received", "event", "update", "peer", "10.0.0.1"}},
		{"mixed", []string{"event", "update", "peer", "10.0.0.1", "direction", "received"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseMonitorArgs(tt.args)
			require.NoError(t, err)
			assert.Equal(t, "10.0.0.1", opts.peer)
			assert.Equal(t, "received", opts.direction)
			assert.Equal(t, []string{"update"}, opts.eventTypes)
		})
	}
}

// =============================================================================
// handleMonitor Tests (Fix #9)
// =============================================================================

// TestHandleMonitor verifies the RPC handler returns parsed config or error.
//
// VALIDATES: Valid args return StatusDone with config data; invalid args return StatusError.
// PREVENTS: handleMonitor silently accepting bad input or returning wrong status.
func TestHandleMonitor(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStatus string
		wantErr    bool
	}{
		{
			name:       "no_args_returns_done",
			args:       nil,
			wantStatus: plugin.StatusDone,
		},
		{
			name:       "valid_peer_returns_done",
			args:       []string{"peer", "10.0.0.1"},
			wantStatus: plugin.StatusDone,
		},
		{
			name:       "valid_event_returns_done",
			args:       []string{"event", "update"},
			wantStatus: plugin.StatusDone,
		},
		{
			name:       "all_filters_returns_done",
			args:       []string{"peer", "10.0.0.1", "event", "update,state", "direction", "received"},
			wantStatus: plugin.StatusDone,
		},
		{
			name:       "invalid_args_returns_error",
			args:       []string{"bogus"},
			wantStatus: plugin.StatusError,
			wantErr:    true,
		},
		{
			name:       "peer_name_returns_done",
			args:       []string{"peer", "upstream-1"},
			wantStatus: plugin.StatusDone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleMonitor(nil, tt.args)
			if tt.wantErr {
				require.Error(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.wantStatus, resp.Status)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.wantStatus, resp.Status)

			// Verify response data contains expected fields.
			data, ok := resp.Data.(map[string]any)
			require.True(t, ok, "response data should be a map")
			assert.Equal(t, "monitor-configured", data["status"])
		})
	}
}

// TestHandleMonitorResponseContent verifies the response data fields match parsed args.
//
// VALIDATES: Response data reflects the parsed peer, event-types, and direction.
// PREVENTS: Response data out of sync with parsed options.
func TestHandleMonitorResponseContent(t *testing.T) {
	resp, err := handleMonitor(nil, []string{"peer", "10.0.0.1", "event", "update,state", "direction", "received"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", data["peer"])
	assert.Equal(t, "received", data["direction"])
	assert.Equal(t, []string{"update", "state"}, data["event-types"])
}

// =============================================================================
// buildSubscriptions Tests (Fix #9)
// =============================================================================

// TestBuildSubscriptions verifies subscription construction from parsed options.
//
// VALIDATES: Correct subscription count, direction, namespace, and peer filter.
// PREVENTS: Wrong number of subscriptions or missing peer filter.
func TestBuildSubscriptions(t *testing.T) {
	tests := []struct {
		name        string
		opts        *monitorOpts
		wantCount   int
		wantDir     string
		wantPeerNil bool
		wantPeerSel string
	}{
		{
			name:        "no_filters_subscribes_all_events",
			opts:        &monitorOpts{},
			wantCount:   len(allBGPEventTypes),
			wantDir:     events.DirectionBoth,
			wantPeerNil: true,
		},
		{
			name:        "specific_events",
			opts:        &monitorOpts{eventTypes: []string{"update", "state"}},
			wantCount:   2,
			wantDir:     events.DirectionBoth,
			wantPeerNil: true,
		},
		{
			name:        "with_peer_filter",
			opts:        &monitorOpts{peer: "10.0.0.1"},
			wantCount:   len(allBGPEventTypes),
			wantDir:     events.DirectionBoth,
			wantPeerNil: false,
			wantPeerSel: "10.0.0.1",
		},
		{
			name:        "with_direction",
			opts:        &monitorOpts{direction: "received"},
			wantCount:   len(allBGPEventTypes),
			wantDir:     "received",
			wantPeerNil: true,
		},
		{
			name:        "all_filters",
			opts:        &monitorOpts{peer: "10.0.0.2", eventTypes: []string{"update"}, direction: "sent"},
			wantCount:   1,
			wantDir:     "sent",
			wantPeerNil: false,
			wantPeerSel: "10.0.0.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subs := buildSubscriptions(tt.opts)
			require.Len(t, subs, tt.wantCount)

			for _, sub := range subs {
				assert.Equal(t, "bgp", sub.Namespace, "namespace should be bgp")
				assert.Equal(t, tt.wantDir, sub.Direction, "direction mismatch")
				if tt.wantPeerNil {
					assert.Nil(t, sub.PeerFilter, "peer filter should be nil")
				} else {
					require.NotNil(t, sub.PeerFilter, "peer filter should not be nil")
					assert.Equal(t, tt.wantPeerSel, sub.PeerFilter.Selector)
				}
			}
		})
	}
}

// =============================================================================
// formatHeader Tests (Fix #9)
// =============================================================================

// TestFormatHeader verifies header line construction from parsed options.
//
// VALIDATES: Header reflects active filters or "all events, all peers" when none.
// PREVENTS: Missing or incorrect filter display in header.
func TestFormatHeader(t *testing.T) {
	tests := []struct {
		name string
		opts *monitorOpts
		want string
	}{
		{
			name: "no_filters",
			opts: &monitorOpts{},
			want: "monitoring: all events, all peers",
		},
		{
			name: "peer_only",
			opts: &monitorOpts{peer: "10.0.0.1"},
			want: "monitoring: peer=10.0.0.1",
		},
		{
			name: "event_only",
			opts: &monitorOpts{eventTypes: []string{"update", "state"}},
			want: "monitoring: event=update,state",
		},
		{
			name: "direction_only",
			opts: &monitorOpts{direction: "received"},
			want: "monitoring: direction=received",
		},
		{
			name: "all_filters",
			opts: &monitorOpts{peer: "10.0.0.1", eventTypes: []string{"update", "state"}, direction: "received"},
			want: "monitoring: peer=10.0.0.1 event=update,state direction=received",
		},
		{
			name: "peer_glob",
			opts: &monitorOpts{peer: "*"},
			want: "monitoring: peer=*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHeader(tt.opts)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// StreamMonitor Integration Test (Fix #10)
// =============================================================================

// TestStreamMonitor verifies end-to-end streaming: register, deliver, cancel, output.
//
// VALIDATES: StreamMonitor writes header, receives delivered events, and exits on cancel.
// PREVENTS: StreamMonitor failing to register, deliver, or clean up.
func TestStreamMonitor(t *testing.T) {
	mm := pluginserver.NewMonitorManager()
	ctx, cancel := context.WithCancel(t.Context())

	var buf syncBuffer
	done := make(chan error, 1)
	go func() {
		done <- StreamMonitor(ctx, mm, &buf, nil) // no filters = all events
	}()

	// Wait for the monitor client to register.
	require.Eventually(t, func() bool {
		return mm.Count() == 1
	}, 2*time.Second, 10*time.Millisecond, "monitor client should register")

	// Deliver an event.
	eventJSON := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","remote":{"as":65001}},"message":{"type":"update","direction":"received"}}}`
	mm.Deliver(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", eventJSON)

	// Wait for the event to appear in output.
	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), eventJSON)
	}, 2*time.Second, 10*time.Millisecond, "event should appear in output")

	// Cancel and collect.
	cancel()
	err := <-done
	require.NoError(t, err)

	// Verify output contains header + event.
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "output should have header + at least one event")
	assert.Equal(t, "monitoring: all events, all peers", lines[0])
	assert.Contains(t, lines[1], eventJSON)

	// Monitor should have been removed after StreamMonitor returns.
	assert.Equal(t, 0, mm.Count(), "monitor client should be removed after exit")
}

// TestStreamMonitorWithFilters verifies streaming with peer and event filters.
//
// VALIDATES: Only matching events are delivered when filters are set.
// PREVENTS: Filters ignored during streaming.
func TestStreamMonitorWithFilters(t *testing.T) {
	mm := pluginserver.NewMonitorManager()
	ctx, cancel := context.WithCancel(t.Context())

	var buf syncBuffer
	done := make(chan error, 1)
	go func() {
		done <- StreamMonitor(ctx, mm, &buf, []string{"peer", "10.0.0.1", "event", "update"})
	}()

	// Wait for registration.
	require.Eventually(t, func() bool {
		return mm.Count() == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Deliver matching event.
	matchEvent := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","remote":{"as":65001}},"message":{"type":"update","direction":"received"}}}`
	mm.Deliver(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", matchEvent)

	// Deliver non-matching event (different peer).
	noMatchEvent := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.2","remote":{"as":65002}},"message":{"type":"update","direction":"received"}}}`
	mm.Deliver(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.2", "", noMatchEvent)

	// Wait for the matching event.
	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), matchEvent)
	}, 2*time.Second, 10*time.Millisecond)

	// Verify non-matching event does NOT arrive within a reasonable window.
	require.Never(t, func() bool {
		return strings.Contains(buf.String(), noMatchEvent)
	}, 50*time.Millisecond, 10*time.Millisecond, "non-matching event should not appear")

	cancel()
	err := <-done
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, matchEvent, "matching event should appear")
	assert.NotContains(t, output, noMatchEvent, "non-matching event should not appear")
}

// TestStreamMonitorInvalidArgs verifies StreamMonitor returns error for bad args.
//
// VALIDATES: Invalid args cause immediate error return without registering.
// PREVENTS: StreamMonitor blocking forever on bad input.
func TestStreamMonitorInvalidArgs(t *testing.T) {
	mm := pluginserver.NewMonitorManager()
	var buf bytes.Buffer
	err := StreamMonitor(t.Context(), mm, &buf, []string{"bogus"})
	require.Error(t, err)
	assert.Equal(t, 0, mm.Count(), "no client should be registered on error")
}
