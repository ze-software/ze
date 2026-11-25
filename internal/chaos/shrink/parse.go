// Design: docs/architecture/chaos-web-dashboard.md — test case shrinking

package shrink

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
)

var errEmptyEventLog = errors.New("empty event log")

// LogMeta holds metadata from the NDJSON event log header.
type LogMeta struct {
	Seed                uint64
	Peers               int
	ChaosRate           float64
	StartTime           time.Time
	IBGPPeers           []bool
	PeerFamilies        [][]string
	ConvergenceDeadline time.Duration
	Properties          string
}

// logHeader mirrors the header line format from report.JSONLog.
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

// logEvent mirrors the event line format from report.JSONLog.
type logEvent struct {
	RecordType        string            `json:"record-type"`
	Seq               uint64            `json:"seq"`
	TimeOffsetMS      int64             `json:"time-offset-ms"`
	TimeOffsetNS      *int64            `json:"time-offset-ns"`
	EventType         string            `json:"event-type"`
	PeerIndex         int               `json:"peer-index"`
	Prefix            string            `json:"prefix"`
	Count             int               `json:"count"`
	Family            string            `json:"family"`
	PlannedShutdown   bool              `json:"planned-shutdown"`
	WithdrawnPrefixes []netip.Prefix    `json:"withdrawn-prefixes"`
	ChaosAction       string            `json:"chaos-action"`
	ChaosParams       map[string]string `json:"chaos-params"`
}

// eventTypeFromString maps kebab-case event names to EventType constants.
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

// ParseLog reads an NDJSON event log and returns the header metadata and
// the parsed event list. Events with unknown types are skipped.
func ParseLog(r io.Reader) (*LogMeta, []peer.Event, error) {
	scanner := bufio.NewScanner(r)
	// A withdrawal record can carry every prefix from an extended UPDATE.
	scanner.Buffer(nil, 2*1024*1024)

	// Parse header line.
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, nil, fmt.Errorf("reading event log: %w", err)
		}
		return nil, nil, errEmptyEventLog
	}

	var hdr logHeader
	if err := json.Unmarshal(scanner.Bytes(), &hdr); err != nil {
		return nil, nil, fmt.Errorf("parsing header: %w", err)
	}
	if hdr.Version < 2 && len(scanner.Bytes()) >= bufio.MaxScanTokenSize {
		return nil, nil, errors.New("reading event log: legacy header exceeds the line size limit")
	}
	if hdr.RecordType != "header" {
		return nil, nil, fmt.Errorf("first line must be a header (got %q)", hdr.RecordType)
	}
	if hdr.Peers < 1 {
		return nil, nil, fmt.Errorf("invalid peer count in header: %d", hdr.Peers)
	}
	if hdr.Version >= 2 {
		if hdr.Peers > 50 {
			return nil, nil, fmt.Errorf("invalid peer count in header: %d", hdr.Peers)
		}
		if hdr.ConvergenceDeadline <= 0 {
			return nil, nil, errors.New("event log lacks a positive convergence deadline")
		}
		if len(hdr.IBGPPeers) != 0 && len(hdr.IBGPPeers) != hdr.Peers {
			return nil, nil, errors.New("iBGP profile count does not match the event log header")
		}
		if len(hdr.PeerFamilies) != 0 && len(hdr.PeerFamilies) != hdr.Peers {
			return nil, nil, errors.New("family profile count does not match the event log header")
		}
	}

	startTime, err := time.Parse(time.RFC3339Nano, hdr.StartTime)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing start-time %q: %w", hdr.StartTime, err)
	}

	meta := &LogMeta{
		Seed:      hdr.Seed,
		Peers:     hdr.Peers,
		ChaosRate: hdr.ChaosRate,
		StartTime: startTime,
	}
	if hdr.Version >= 2 {
		meta.IBGPPeers = hdr.IBGPPeers
		meta.PeerFamilies = hdr.PeerFamilies
		meta.ConvergenceDeadline = hdr.ConvergenceDeadline
		meta.Properties = hdr.Properties
	}

	// Parse event lines.
	var events []peer.Event
	observationEnded := false
	for scanner.Scan() {
		if hdr.Version < 2 && len(scanner.Bytes()) >= bufio.MaxScanTokenSize {
			return nil, nil, errors.New("reading event log: legacy record exceeds the line size limit")
		}
		var ev logEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			if hdr.Version >= 2 {
				return nil, nil, fmt.Errorf("parsing event log record: %w", err)
			}
			continue
		}
		if observationEnded {
			continue
		}
		if hdr.Version >= 2 && ev.RecordType == "observation-end" {
			observationEnded = true
			continue
		}
		if ev.RecordType != "event" {
			continue
		}

		et, ok := eventTypeFromString[ev.EventType]
		if !ok {
			continue
		}
		if hdr.Version < 2 && (et == peer.EventDroppedEvents || et == peer.EventRouteAction) {
			continue
		}

		offset := time.Duration(ev.TimeOffsetMS) * time.Millisecond
		if hdr.Version >= 2 {
			validPeer := ev.PeerIndex >= 0 && ev.PeerIndex < hdr.Peers
			if !validPeer {
				return nil, nil, fmt.Errorf("event log peer index %d is outside the scenario", ev.PeerIndex)
			}
			if ev.TimeOffsetNS != nil {
				offset = time.Duration(*ev.TimeOffsetNS)
			}
		}

		var prefix netip.Prefix
		if ev.Prefix != "" {
			var pfxErr error
			prefix, pfxErr = netip.ParsePrefix(ev.Prefix)
			if pfxErr != nil {
				return nil, nil, fmt.Errorf("parsing prefix %q at seq %d: %w", ev.Prefix, ev.Seq, pfxErr)
			}
		}

		event := peer.Event{
			Type:        et,
			PeerIndex:   ev.PeerIndex,
			Time:        startTime.Add(offset),
			Prefix:      prefix,
			Count:       ev.Count,
			ChaosAction: ev.ChaosAction,
		}
		if hdr.Version >= 2 {
			event.Family = ev.Family
			event.PlannedShutdown = ev.PlannedShutdown
			event.WithdrawnPrefixes = ev.WithdrawnPrefixes
			event.ChaosParams = ev.ChaosParams
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading event log: %w", err)
	}
	if hdr.Version >= 2 && !observationEnded {
		return nil, nil, errors.New("event log ended before the observation checkpoint")
	}

	return meta, events, nil
}
