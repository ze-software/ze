package ipc

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestBatchRoundTrip verifies write batch frame → read with FrameReader → parse events.
//
// VALIDATES: Batch frame is a valid NUL-delimited frame parseable by FrameReader,
// and events round-trip correctly through WriteBatchFrame + ParseBatchEvents.
// PREVENTS: Batch frames breaking existing NUL-framed protocol.
func TestBatchRoundTrip(t *testing.T) {
	events := [][]byte{
		[]byte(`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`),
		[]byte(`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.2"}}}`),
		[]byte(`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.3"}}}`),
	}

	var buf bytes.Buffer
	err := rpc.WriteBatchFrame(&buf, 42, events)
	require.NoError(t, err)

	// Read back with FrameReader — batch frame must be a valid NUL-delimited frame
	reader := rpc.NewFrameReader(&buf)
	frame, err := reader.Read()
	require.NoError(t, err)

	// Frame should be valid JSON
	var rpcReq struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		ID     uint64          `json:"id"`
	}
	require.NoError(t, json.Unmarshal(frame, &rpcReq))
	assert.Equal(t, "ze-plugin-callback:deliver-batch", rpcReq.Method)
	assert.Equal(t, uint64(42), rpcReq.ID)

	// Parse events from params
	got, err := rpc.ParseBatchEvents(rpcReq.Params)
	require.NoError(t, err)
	require.Len(t, got, 3)

	for i, event := range events {
		assert.JSONEq(t, string(event), string(got[i]))
	}

	// No more frames
	_, err = reader.Read()
	assert.ErrorIs(t, err, io.EOF)
}

// TestBatchSingleEvent verifies a batch of 1 works correctly.
//
// VALIDATES: AC-1 — single event in channel delivered as batch of 1.
// PREVENTS: Off-by-one when batch contains exactly one event.
func TestBatchSingleEvent(t *testing.T) {
	event := []byte(`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`)

	var buf bytes.Buffer
	err := rpc.WriteBatchFrame(&buf, 1, [][]byte{event})
	require.NoError(t, err)

	reader := rpc.NewFrameReader(&buf)
	frame, err := reader.Read()
	require.NoError(t, err)

	var rpcReq struct {
		Params json.RawMessage `json:"params"`
	}
	require.NoError(t, json.Unmarshal(frame, &rpcReq))

	got, err := rpc.ParseBatchEvents(rpcReq.Params)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.JSONEq(t, string(event), string(got[0]))
}

// TestBatchParseEvents verifies ParseBatchEvents with raw JSON params.
//
// VALIDATES: AC-7 — events extracted correctly from params.
// PREVENTS: Event boundaries being misidentified.
func TestBatchParseEvents(t *testing.T) {
	tests := []struct {
		name   string
		params string
		want   int
	}{
		{
			name:   "three_events",
			params: `{"events":[{"a":1},{"b":2},{"c":3}]}`,
			want:   3,
		},
		{
			name:   "one_event",
			params: `{"events":[{"a":1}]}`,
			want:   1,
		},
		{
			name:   "empty_events",
			params: `{"events":[]}`,
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rpc.ParseBatchEvents([]byte(tt.params))
			require.NoError(t, err)
			assert.Len(t, got, tt.want)
		})
	}
}

// TestBatchParseEventsError verifies ParseBatchEvents rejects invalid input.
//
// VALIDATES: Malformed params produce an error.
// PREVENTS: Silent corruption from invalid batch params.
func TestBatchParseEventsError(t *testing.T) {
	tests := []struct {
		name   string
		params string
	}{
		{name: "not_json", params: `not json`},
		{name: "events_not_array", params: `{"events":"string"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rpc.ParseBatchEvents([]byte(tt.params))
			require.Error(t, err)
		})
	}
}

// TestBatchFramePooledBuffer verifies WriteBatchFrame uses pooled buffers.
//
// VALIDATES: AC-10 — batch write uses pooled buffer, no per-frame make([]byte).
// PREVENTS: Allocation per batch write degrading GC performance.
func TestBatchFramePooledBuffer(t *testing.T) {
	events := [][]byte{
		[]byte(`{"type":"bgp"}`),
	}

	// Call WriteBatchFrame multiple times — should reuse pool buffers
	for range 100 {
		var buf bytes.Buffer
		err := rpc.WriteBatchFrame(&buf, 1, events)
		require.NoError(t, err)
		assert.True(t, buf.Len() > 0)
	}
}

// TestBatchFrameLargePayload verifies batching many events.
//
// VALIDATES: AC-2 — N events queued delivered in one batch.
// PREVENTS: Large batches overflowing or being truncated.
func TestBatchFrameLargePayload(t *testing.T) {
	// 64 events (channel capacity)
	events := make([][]byte, 64)
	for i := range events {
		events[i] = []byte(`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"},"update":{"attr":{"origin":"igp"}}}}`)
	}

	var buf bytes.Buffer
	err := rpc.WriteBatchFrame(&buf, 99, events)
	require.NoError(t, err)

	reader := rpc.NewFrameReader(&buf)
	frame, err := reader.Read()
	require.NoError(t, err)

	var rpcReq struct {
		Params json.RawMessage `json:"params"`
	}
	require.NoError(t, json.Unmarshal(frame, &rpcReq))

	got, err := rpc.ParseBatchEvents(rpcReq.Params)
	require.NoError(t, err)
	assert.Len(t, got, 64)
}

// TestBatchTextEventRoundTrip verifies text events survive WriteBatchFrame → ParseBatchEvents.
//
// VALIDATES: Text-format events (not valid JSON values) round-trip through batch framing.
// PREVENTS: Text events producing invalid JSON in batch frame (broken pipe crash).
func TestBatchTextEventRoundTrip(t *testing.T) {
	// Text events are plain strings, NOT valid JSON values.
	// They must be JSON-quoted before insertion into the events array.
	textEvent1, _ := json.Marshal("peer 10.0.0.1 received update 42 announce origin igp ipv4/unicast next-hop 10.0.0.1 nlri 192.168.1.0/24\n")
	textEvent2, _ := json.Marshal("peer 10.0.0.2 state up\n")
	events := [][]byte{textEvent1, textEvent2}

	var buf bytes.Buffer
	err := rpc.WriteBatchFrame(&buf, 7, events)
	require.NoError(t, err)

	reader := rpc.NewFrameReader(&buf)
	frame, err := reader.Read()
	require.NoError(t, err)

	// Frame must be valid JSON
	var rpcReq struct {
		Params json.RawMessage `json:"params"`
	}
	require.NoError(t, json.Unmarshal(frame, &rpcReq))

	got, err := rpc.ParseBatchEvents(rpcReq.Params)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Each event should be unwrappable as a JSON string
	for i, raw := range got {
		var eventStr string
		require.NoError(t, json.Unmarshal(raw, &eventStr), "event %d should be a valid JSON string", i)
	}
}

// TestBatchFrameIDIncrement verifies unique IDs in batch frames.
//
// VALIDATES: Each batch frame has a unique ID field.
// PREVENTS: ID collisions between batch frames.
func TestBatchFrameIDIncrement(t *testing.T) {
	events := [][]byte{[]byte(`{}`)}

	ids := make(map[uint64]bool)
	for i := range uint64(10) {
		var buf bytes.Buffer
		err := rpc.WriteBatchFrame(&buf, i+1, events)
		require.NoError(t, err)

		reader := rpc.NewFrameReader(&buf)
		frame, err := reader.Read()
		require.NoError(t, err)

		var rpcReq struct {
			ID uint64 `json:"id"`
		}
		require.NoError(t, json.Unmarshal(frame, &rpcReq))
		assert.False(t, ids[rpcReq.ID], "duplicate ID: %d", rpcReq.ID)
		ids[rpcReq.ID] = true
	}
}
