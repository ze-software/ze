// Design: docs/architecture/chaos-web-dashboard.md — chaos reporting and metrics

package report

import (
	"encoding/json"
	"io"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
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

	// IBGPPeers and PeerFamilies describe the scenario by peer index.
	IBGPPeers    []bool
	PeerFamilies [][]string

	// ConvergenceDeadline enables logging the settings needed to replay the verdict.
	ConvergenceDeadline time.Duration

	// Properties is the selection of property checks used by the run.
	Properties string
}

// jsonHeader is the NDJSON header line (first line of event log).
type jsonHeader struct {
	RecordType          string        `json:"record-type"`
	Version             int           `json:"version"`
	Seed                uint64        `json:"seed"`
	Peers               int           `json:"peers"`
	ChaosRate           float64       `json:"chaos-rate"`
	StartTime           string        `json:"start-time"`
	IBGPPeers           []bool        `json:"ibgp-peers,omitempty"`
	PeerFamilies        [][]string    `json:"peer-families,omitempty"`
	ConvergenceDeadline time.Duration `json:"convergence-deadline-ns,omitempty"`
	Properties          string        `json:"properties,omitempty"`
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

// jsonObservationEnd marks the cutoff for the run's verdict.
type jsonObservationEnd struct {
	RecordType   string `json:"record-type"`
	Seq          uint64 `json:"seq"`
	TimeOffsetMS int64  `json:"time-offset-ms"`
	TimeOffsetNS int64  `json:"time-offset-ns"`
}

// jsonEvent is the NDJSON representation of a peer event.
// All keys use kebab-case per project JSON convention.
type jsonEvent struct {
	RecordType        string            `json:"record-type"`
	Seq               uint64            `json:"seq"`
	TimeOffsetMS      int64             `json:"time-offset-ms"`
	TimeOffsetNS      int64             `json:"time-offset-ns"`
	EventType         string            `json:"event-type"`
	PeerIndex         int               `json:"peer-index"`
	Prefix            string            `json:"prefix,omitempty"`
	Family            string            `json:"family,omitempty"`
	Count             int               `json:"count,omitempty"`
	ChaosAction       string            `json:"chaos-action,omitempty"`
	ChaosParams       map[string]string `json:"chaos-params,omitempty"`
	Error             string            `json:"error,omitempty"`
	PlannedShutdown   bool              `json:"planned-shutdown,omitempty"`
	WithdrawnPrefixes []netip.Prefix    `json:"withdrawn-prefixes,omitempty"`
}

// JSONLog writes events as NDJSON (one JSON object per line) to a writer.
// The first line is a header with scenario metadata; subsequent lines are events
// with monotonically increasing sequence numbers and relative time offsets.
// It implements the Consumer interface.
// Encode errors are tracked and returned from Close().
//
// JSONLog is safe for concurrent use: ProcessEvent (event loop goroutine)
// and LogControl (HTTP handler goroutines) share the same lock as EndObservation.
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
		RecordType:          "header",
		Version:             1,
		Seed:                cfg.Seed,
		Peers:               cfg.Peers,
		ChaosRate:           cfg.ChaosRate,
		StartTime:           cfg.Start.Format(time.RFC3339Nano),
		IBGPPeers:           cfg.IBGPPeers,
		PeerFamilies:        cfg.PeerFamilies,
		ConvergenceDeadline: cfg.ConvergenceDeadline,
		Properties:          cfg.Properties,
	}
	if cfg.ConvergenceDeadline > 0 {
		hdr.Version = 2
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

	offset := ev.Time.Sub(j.start)
	entry := jsonEvent{
		RecordType:        "event",
		Seq:               j.seq,
		TimeOffsetMS:      offset.Milliseconds(),
		TimeOffsetNS:      offset.Nanoseconds(),
		EventType:         ev.Type.String(),
		PeerIndex:         ev.PeerIndex,
		Family:            ev.Family,
		PlannedShutdown:   ev.PlannedShutdown,
		WithdrawnPrefixes: ev.WithdrawnPrefixes,
	}

	if ev.Prefix.IsValid() {
		entry.Prefix = ev.Prefix.String()
	}

	if ev.Count > 0 {
		entry.Count = ev.Count
	}

	if ev.ChaosAction != "" {
		entry.ChaosAction = ev.ChaosAction
		if len(ev.ChaosParams) > 0 {
			entry.ChaosParams = ev.ChaosParams
		}
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

// EndObservation records the end of the observation window for replay.
func (j *JSONLog) EndObservation(at time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.seq++
	offset := at.Sub(j.start)
	entry := jsonObservationEnd{
		RecordType:   "observation-end",
		Seq:          j.seq,
		TimeOffsetMS: offset.Milliseconds(),
		TimeOffsetNS: offset.Nanoseconds(),
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
