package report

import (
	"encoding/json"
	"io"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// jsonEvent is the NDJSON representation of a peer event.
// All keys use kebab-case per project JSON convention.
type jsonEvent struct {
	EventType   string `json:"event-type"`
	PeerIndex   int    `json:"peer-index"`
	Timestamp   string `json:"timestamp"`
	Prefix      string `json:"prefix,omitempty"`
	Count       int    `json:"count,omitempty"`
	ChaosAction string `json:"chaos-action,omitempty"`
	Error       string `json:"error,omitempty"`
}

// JSONLog writes events as NDJSON (one JSON object per line) to a writer.
// It implements the Consumer interface.
// Encode errors are tracked and returned from Close().
type JSONLog struct {
	enc *json.Encoder
	err error // first encode error
}

// NewJSONLog creates a JSONLog that writes NDJSON to w.
func NewJSONLog(w io.Writer) *JSONLog {
	return &JSONLog{enc: json.NewEncoder(w)}
}

// ProcessEvent serializes the event as a single JSON line.
func (j *JSONLog) ProcessEvent(ev peer.Event) {
	entry := jsonEvent{
		EventType: ev.Type.String(),
		PeerIndex: ev.PeerIndex,
		Timestamp: ev.Time.Format(time.RFC3339Nano),
	}

	if ev.Prefix.IsValid() {
		entry.Prefix = ev.Prefix.String()
	}

	if ev.Count > 0 {
		entry.Count = ev.Count
	}

	if ev.ChaosAction != "" {
		entry.ChaosAction = ev.ChaosAction
	}

	if ev.Err != nil {
		entry.Error = ev.Err.Error()
	}

	// json.Encoder.Encode appends a newline, producing NDJSON format.
	// Track the first error; return it from Close().
	if err := j.enc.Encode(entry); err != nil && j.err == nil {
		j.err = err
	}
}

// Close returns the first encode error encountered, if any.
// The caller owns the underlying writer.
func (j *JSONLog) Close() error {
	return j.err
}
