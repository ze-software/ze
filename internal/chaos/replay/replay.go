// Design: docs/architecture/chaos-web-dashboard.md — event replay and diff
//
// Package replay provides event log replay and comparison for ze-chaos.
//
// It reads NDJSON event logs produced by --event-log, feeds events through
// the validation model, and reports pass/fail with a summary.
package replay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
	"codeberg.org/thomas-mangin/ze/internal/chaos/report"
	"codeberg.org/thomas-mangin/ze/internal/chaos/validation"
)

// logHeader is the parsed header line of an event log.
type logHeader struct {
	RecordType string  `json:"record-type"`
	Version    int     `json:"version"`
	Seed       uint64  `json:"seed"`
	Peers      int     `json:"peers"`
	ChaosRate  float64 `json:"chaos-rate"`
	StartTime  string  `json:"start-time"`
}

// logEvent is a parsed event line from an event log.
type logEvent struct {
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

// eventTypeFromString maps kebab-case event type strings to EventType constants.
var eventTypeFromString = map[string]peer.EventType{
	"established":     peer.EventEstablished,
	"route-sent":      peer.EventRouteSent,
	"route-received":  peer.EventRouteReceived,
	"route-withdrawn": peer.EventRouteWithdrawn,
	"eor-sent":        peer.EventEORSent,
	"disconnected":    peer.EventDisconnected,
	"error":           peer.EventError,
	"chaos-executed":  peer.EventChaosExecuted,
	"reconnecting":    peer.EventReconnecting,
	"withdrawal-sent": peer.EventWithdrawalSent,
}

// writeErr writes an error message to w, ignoring write failures
// since we are already on an error path returning exit code 2.
func writeErr(w io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		// Best-effort error reporting; nothing more to do.
		return
	}
}

// Run replays an NDJSON event log through the validation model and prints
// a summary to w. Returns exit code: 0=pass, 1=fail, 2=error.
func Run(r io.Reader, w io.Writer) int {
	scanner := bufio.NewScanner(r)

	// Parse header line.
	if !scanner.Scan() {
		writeErr(w, "error: empty event log\n")
		return 2
	}

	var hdr logHeader
	if err := json.Unmarshal(scanner.Bytes(), &hdr); err != nil {
		writeErr(w, "error: parsing header: %v\n", err)
		return 2
	}
	if hdr.RecordType != "header" {
		writeErr(w, "error: first line must be a header (got %q)\n", hdr.RecordType)
		return 2
	}
	if hdr.Peers < 1 {
		writeErr(w, "error: invalid peer count in header: %d\n", hdr.Peers)
		return 2
	}

	// Create validation components.
	n := hdr.Peers
	model := validation.NewModel(n)
	tracker := validation.NewTracker(n)
	convergence := validation.NewConvergence(n, 5*time.Second)

	// Parse start time for convergence tracking.
	startTime, err := time.Parse(time.RFC3339Nano, hdr.StartTime)
	if err != nil {
		startTime = time.Now() // fallback
	}

	// Aggregate counters.
	var announced, received, chaosEvents, reconnections, withdrawn int

	// Process event lines.
	for scanner.Scan() {
		var ev logEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue // skip malformed lines
		}
		if ev.RecordType != "event" {
			continue
		}

		evTime := startTime.Add(time.Duration(ev.TimeOffsetMS) * time.Millisecond)

		et, ok := eventTypeFromString[ev.EventType]
		if !ok {
			continue // skip unknown event types
		}

		// Parse prefix if present.
		var prefix netip.Prefix
		if ev.Prefix != "" {
			prefix, _ = netip.ParsePrefix(ev.Prefix)
		}

		// Route event to validation components (mirrors EventProcessor.Process).
		switch et {
		case peer.EventEstablished:
			model.SetEstablished(ev.PeerIndex, true)
		case peer.EventRouteSent:
			model.Announce(ev.PeerIndex, prefix)
			convergence.RecordAnnounce(ev.PeerIndex, prefix, evTime)
			announced++
		case peer.EventRouteReceived:
			tracker.RecordReceive(ev.PeerIndex, prefix)
			convergence.RecordReceive(ev.PeerIndex, prefix, evTime)
			received++
		case peer.EventRouteWithdrawn:
			tracker.RecordWithdraw(ev.PeerIndex, prefix)
		case peer.EventDisconnected:
			model.Disconnect(ev.PeerIndex)
			tracker.ClearPeer(ev.PeerIndex)
		case peer.EventChaosExecuted:
			chaosEvents++
		case peer.EventReconnecting:
			reconnections++
		case peer.EventWithdrawalSent:
			withdrawn += ev.Count
		case peer.EventRouteAction:
			// Route dynamics — no validation action.
		case peer.EventEORSent, peer.EventError, peer.EventDroppedEvents:
			// Informational — no validation action.
		}
	}

	// Final validation.
	result := validation.Check(model, tracker)
	convStats := convergence.Stats()

	// Build per-peer failure details from check result.
	var peerFailures []report.PeerFailure
	var missingCount, extraCount int
	for i, pr := range result.Peers {
		if pr.Missing.Len() == 0 && pr.Extra.Len() == 0 {
			continue
		}
		pf := report.PeerFailure{
			PeerIndex:     i,
			ExpectedCount: pr.ExpectedCount,
			ActualCount:   pr.ActualCount,
		}
		pf.Missing = pr.Missing.SortedStrings()
		pf.Extra = pr.Extra.SortedStrings()
		missingCount += len(pf.Missing)
		extraCount += len(pf.Extra)
		peerFailures = append(peerFailures, pf)
	}

	summary := report.Summary{
		Seed:          hdr.Seed,
		PeerCount:     n,
		Announced:     announced,
		Received:      received,
		Missing:       missingCount,
		Extra:         extraCount,
		MinLatency:    convStats.Min,
		AvgLatency:    convStats.Avg,
		MaxLatency:    convStats.Max,
		P99Latency:    convStats.P99,
		ChaosEvents:   chaosEvents,
		Reconnections: reconnections,
		Withdrawn:     withdrawn,
		PeerFailures:  peerFailures,
	}

	return summary.Write(w)
}
