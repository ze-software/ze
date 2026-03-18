// Design: docs/architecture/api/ipc_protocol.md — batched event delivery
// Overview: framing.go — newline-delimited frame reader/writer

package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
)

// maxPoolBufSize is the maximum buffer capacity returned to batchBufPool.
// Buffers that grew beyond this (16x the initial 4KB) are dropped to prevent
// a single large batch from permanently inflating the pool.
const maxPoolBufSize = 64 * 1024

// batchBufPool provides reusable buffers for constructing batch RPC frames.
// Initial capacity 4KB matches buildBufPool used elsewhere.
var batchBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 4096)
		return &buf
	},
}

// WriteBatchFrame writes a deliver-batch line to w using a pooled buffer.
// Events must be valid JSON values (strings, objects, arrays, etc.) -- they are
// embedded directly into the events array. Callers must JSON-quote plain text
// events (e.g., via json.Marshal) before passing them here.
// The frame is newline-terminated.
func WriteBatchFrame(w io.Writer, id uint64, events [][]byte) error {
	bp, ok := batchBufPool.Get().(*[]byte)
	if !ok {
		b := make([]byte, 0, 4096)
		bp = &b
	}
	buf := (*bp)[:0]

	// Build line: #<id> ze-plugin-callback:deliver-batch {"events":[...]}
	buf = append(buf, '#')
	buf = strconv.AppendUint(buf, id, 10)
	buf = append(buf, ` ze-plugin-callback:deliver-batch {"events":[`...)
	for i, event := range events {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, event...)
	}
	buf = append(buf, "]}\n"...)

	_, err := w.Write(buf)

	// Don't return oversized buffers to pool (prevents memory leak from large batches).
	if cap(buf) <= maxPoolBufSize {
		*bp = buf[:0]
		batchBufPool.Put(bp)
	}

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
