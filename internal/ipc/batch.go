// Design: docs/architecture/api/ipc_protocol.md — batched event delivery
// Overview: framing.go — NUL-delimited frame reader/writer

package ipc

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
)

// batchBufPool provides reusable buffers for constructing batch RPC frames.
// Initial capacity 4KB matches buildBufPool used elsewhere.
var batchBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 4096)
		return &buf
	},
}

// WriteBatchFrame writes a deliver-batch JSON-RPC frame to w using a pooled buffer.
// Events must be valid JSON values (strings, objects, arrays, etc.) — they are
// embedded directly into the events array. Callers must JSON-quote plain text
// events (e.g., via json.Marshal) before passing them here.
// The frame is NUL-terminated, compatible with the existing FrameReader.
//
// Frame format: JSON-RPC envelope with events array, NUL-terminated.
func WriteBatchFrame(w io.Writer, id uint64, events [][]byte) error {
	bp, ok := batchBufPool.Get().(*[]byte)
	if !ok {
		b := make([]byte, 0, 4096)
		bp = &b
	}
	buf := (*bp)[:0]

	// Build JSON-RPC envelope manually — avoids json.Marshal allocation.
	buf = append(buf, `{"method":"ze-plugin-callback:deliver-batch","params":{"events":[`...)
	for i, event := range events {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, event...)
	}
	buf = append(buf, `]},"id":`...)
	buf = strconv.AppendUint(buf, id, 10)
	// Close JSON envelope + NUL terminator for FrameReader compatibility
	buf = append(buf, '}', 0)

	_, err := w.Write(buf)

	// Return buffer to pool (keep capacity, reset length)
	*bp = buf[:0]
	batchBufPool.Put(bp)

	if err != nil {
		return fmt.Errorf("write batch frame: %w", err)
	}
	return nil
}

// ParseBatchEvents extracts individual event payloads from deliver-batch params.
// Each returned json.RawMessage references the parsed buffer.
func ParseBatchEvents(params []byte) ([]json.RawMessage, error) {
	var input struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("unmarshal batch events: %w", err)
	}
	return input.Events, nil
}
