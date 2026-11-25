// Design: docs/architecture/chaos-web-dashboard.md — event replay and diff
//
// Package replay provides event log replay and comparison for le chaos run.
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

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/report"
	"github.com/ze-software/ze/internal/chaos/validation"
)

// logHeader is the parsed header line of an event log.
type logHeader struct {
	RecordType          string        `json:"record-type"`
	Version             int           `json:"version"`
	Seed                uint64        `json:"seed"`
	Peers               int           `json:"peers"`
	ChaosRate           float64       `json:"chaos-rate"`
	StartTime           string        `json:"start-time"`
	IBGPPeers           []bool        `json:"ibgp-peers"`
	PeerFamilies        [][]string    `json:"peer-families"`
	ConvergenceDeadline time.Duration `json:"convergence-deadline-ns"`
	Properties          string        `json:"properties"`
}

// logEvent is a parsed event line from an event log.
type logEvent struct {
	RecordType        string            `json:"record-type"`
	Seq               uint64            `json:"seq"`
	TimeOffsetMS      int64             `json:"time-offset-ms"`
	TimeOffsetNS      *int64            `json:"time-offset-ns"`
	EventType         string            `json:"event-type"`
	PeerIndex         int               `json:"peer-index"`
	Prefix            string            `json:"prefix,omitempty"`
	Count             int               `json:"count,omitempty"`
	Family            string            `json:"family"`
	PlannedShutdown   bool              `json:"planned-shutdown"`
	WithdrawnPrefixes []netip.Prefix    `json:"withdrawn-prefixes"`
	ChaosAction       string            `json:"chaos-action,omitempty"`
	ChaosParams       map[string]string `json:"chaos-params,omitempty"`
	Error             string            `json:"error,omitempty"`
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
	"dropped-events":  peer.EventDroppedEvents,
	"route-action":    peer.EventRouteAction,
}

// writeErr writes an error message to w, ignoring write failures
// since we are already on an error path returning exit code 2.
func writeErr(w io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil { //nolint:errcheck // output
		// Best-effort error reporting; nothing more to do.
		return
	}
}

// Run replays an NDJSON event log through the validation model and prints
// a summary to w. Returns exit code: 0=pass, 1=fail, 2=error.
func Run(r io.Reader, w io.Writer) int {
	scanner := bufio.NewScanner(r)
	// A withdrawal record can carry every prefix from an extended UPDATE.
	scanner.Buffer(nil, 2*1024*1024)

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
	switch {
	case hdr.Peers < 1, hdr.Peers > 50:
		writeErr(w, "error: invalid peer count in header: %d\n", hdr.Peers)
		return 2
	}

	switch {
	case !validReplayProfiles(len(hdr.IBGPPeers), hdr.Peers), !validReplayProfiles(len(hdr.PeerFamilies), hdr.Peers):
		writeErr(w, "error: event log profile count does not match the header\n")
		return 2
	}
	if hdr.Version >= 2 && hdr.ConvergenceDeadline <= 0 {
		writeErr(w, "error: event log lacks a positive convergence deadline\n")
		return 2
	}

	startTime, err := time.Parse(time.RFC3339Nano, hdr.StartTime)
	if err != nil {
		if hdr.Version >= 2 {
			writeErr(w, "error: parsing event log start time: %v\n", err)
			return 2
		}
		startTime = time.Now()
	}
	deadline := hdr.ConvergenceDeadline
	if hdr.Version < 2 {
		deadline = 5 * time.Second
	}
	reachable := validation.ScenarioReachability(hdr.IBGPPeers, hdr.PeerFamilies)
	ep := validation.NewEventProcessor(hdr.Peers, deadline, reachable)
	properties, err := validation.ScenarioProperties(hdr.Properties, hdr.Peers, deadline, reachable)
	if err != nil {
		writeErr(w, "error: reading event log properties: %v\n", err)
		return 2
	}

	var observationEnd time.Time
	for scanner.Scan() {
		if hdr.Version < 2 && len(scanner.Bytes()) >= bufio.MaxScanTokenSize {
			writeErr(w, "error: reading event log: legacy record exceeds the line size limit\n")
			return 2
		}
		var record logEvent
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			if hdr.Version >= 2 {
				writeErr(w, "error: parsing event log record: %v\n", err)
				return 2
			}
			continue
		}
		if !observationEnd.IsZero() {
			continue
		}
		offset := time.Duration(record.TimeOffsetMS) * time.Millisecond
		if record.TimeOffsetNS != nil {
			offset = time.Duration(*record.TimeOffsetNS)
		}
		at := startTime.Add(offset)
		if record.RecordType == "observation-end" {
			observationEnd = at
			continue
		}
		if record.RecordType != "event" {
			continue
		}

		kind, ok := eventTypeFromString[record.EventType]
		if !ok {
			continue
		}
		switch {
		case record.PeerIndex < 0, record.PeerIndex >= hdr.Peers:
			writeErr(w, "error: event log peer index %d is outside the scenario\n", record.PeerIndex)
			return 2
		}
		var prefix netip.Prefix
		if record.Prefix != "" {
			prefix, err = netip.ParsePrefix(record.Prefix)
			if err != nil && hdr.Version >= 2 {
				writeErr(w, "error: parsing event log prefix: %v\n", err)
				return 2
			}
		}
		event := peer.Event{
			Type: kind, PeerIndex: record.PeerIndex, Time: at, Prefix: prefix, Family: record.Family,
			Count: record.Count, ChaosAction: record.ChaosAction, ChaosParams: record.ChaosParams,
			PlannedShutdown: record.PlannedShutdown, WithdrawnPrefixes: record.WithdrawnPrefixes,
		}
		ep.Process(event)
		if properties != nil {
			properties.ProcessEvent(event)
		}
	}

	// A log that stops early is not a log that ended. Scan returns false on
	// EOF, on a read error, and on a line above bufio.MaxScanTokenSize alike,
	// so validating here would judge a run by the events that happened to
	// arrive: the announcements after the cut read as routes never sent.
	if err := scanner.Err(); err != nil {
		writeErr(w, "error: reading event log: %v\n", err)
		return 2
	}

	if hdr.Version >= 2 && observationEnd.IsZero() {
		writeErr(w, "error: event log ended before the observation checkpoint\n")
		return 2
	}
	summary := report.Summary{Seed: hdr.Seed, PeerCount: hdr.Peers}
	if !observationEnd.IsZero() {
		summary.Duration = observationEnd.Sub(startTime)
	}
	summary.Observe(ep, properties, observationEnd)
	return summary.Write(w)
}

func validReplayProfiles(count, peers int) bool {
	return count == 0 || count == peers
}
