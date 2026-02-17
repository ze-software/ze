package report

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// JSONLogConfig holds configuration for the NDJSON event log.
type JSONLogConfig struct {
	// Start is the reference time for computing time-offset-ms.
	Start time.Time

	// Seed is the scenario seed recorded in the header.
	Seed uint64

	// Peers is the number of peers recorded in the header.
	Peers int

	// ChaosRate is the chaos probability recorded in the header.
	ChaosRate float64
}

// jsonHeader is the NDJSON header line (first line of event log).
type jsonHeader struct {
	RecordType string  `json:"record-type"`
	Version    int     `json:"version"`
	Seed       uint64  `json:"seed"`
	Peers      int     `json:"peers"`
	ChaosRate  float64 `json:"chaos-rate"`
	StartTime  string  `json:"start-time"`
}

// jsonControl is the NDJSON representation of a dashboard control event.
// Used for pause/resume/rate/trigger/stop — informational, skipped by replay.
type jsonControl struct {
	RecordType   string `json:"record-type"`
	Seq          uint64 `json:"seq"`
	TimeOffsetMS int64  `json:"time-offset-ms"`
	Command      string `json:"command"`
	Value        string `json:"value,omitempty"`
}

// jsonEvent is the NDJSON representation of a peer event.
// All keys use kebab-case per project JSON convention.
type jsonEvent struct {
	RecordType   string `json:"record-type"`
	Seq          uint64 `json:"seq"`
	TimeOffsetMS int64  `json:"time-offset-ms"`
	EventType    string `json:"event-type"`
	PeerIndex    int    `json:"peer-index"`
	Prefix       string `json:"prefix,omitempty"`
	Count        int    `json:"count,omitempty"`
	ChaosAction  string `json:"chaos-action,omitempty"`
	Error        string `json:"error,omitempty"`
}

// JSONLog writes events as NDJSON (one JSON object per line) to a writer.
// The first line is a header with scenario metadata; subsequent lines are events
// with monotonically increasing sequence numbers and relative time offsets.
// It implements the Consumer interface.
// Encode errors are tracked and returned from Close().
//
// JSONLog is safe for concurrent use: ProcessEvent (event loop goroutine)
// and LogControl (HTTP handler goroutines) are synchronized via mu.
type JSONLog struct {
	mu    sync.Mutex
	enc   *json.Encoder
	start time.Time
	seq   uint64
	err   error // first encode error
}

// NewJSONLog creates a JSONLog that writes NDJSON to w.
// It immediately writes the header line with scenario metadata.
func NewJSONLog(w io.Writer, cfg JSONLogConfig) *JSONLog {
	j := &JSONLog{
		enc:   json.NewEncoder(w),
		start: cfg.Start,
	}

	hdr := jsonHeader{
		RecordType: "header",
		Version:    1,
		Seed:       cfg.Seed,
		Peers:      cfg.Peers,
		ChaosRate:  cfg.ChaosRate,
		StartTime:  cfg.Start.Format(time.RFC3339Nano),
	}
	if err := j.enc.Encode(hdr); err != nil {
		j.err = err
	}

	return j
}

// ProcessEvent serializes the event as a single JSON line with sequence number
// and relative time offset.
func (j *JSONLog) ProcessEvent(ev peer.Event) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.seq++

	entry := jsonEvent{
		RecordType:   "event",
		Seq:          j.seq,
		TimeOffsetMS: ev.Time.Sub(j.start).Milliseconds(),
		EventType:    ev.Type.String(),
		PeerIndex:    ev.PeerIndex,
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

// LogControl writes a "control" record for dashboard control events
// (pause, resume, rate, trigger, stop). These are informational and
// skipped by replay.
func (j *JSONLog) LogControl(command, value string, t time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.seq++
	entry := jsonControl{
		RecordType:   "control",
		Seq:          j.seq,
		TimeOffsetMS: t.Sub(j.start).Milliseconds(),
		Command:      command,
		Value:        value,
	}
	if err := j.enc.Encode(entry); err != nil && j.err == nil {
		j.err = err
	}
}

// Close returns the first encode error encountered, if any.
// The caller owns the underlying writer.
func (j *JSONLog) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	return j.err
}
