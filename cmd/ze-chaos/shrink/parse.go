package shrink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// LogMeta holds metadata from the NDJSON event log header.
type LogMeta struct {
	Seed      uint64
	Peers     int
	ChaosRate float64
	StartTime time.Time
}

// logHeader mirrors the header line format from report.JSONLog.
type logHeader struct {
	RecordType string  `json:"record-type"`
	Version    int     `json:"version"`
	Seed       uint64  `json:"seed"`
	Peers      int     `json:"peers"`
	ChaosRate  float64 `json:"chaos-rate"`
	StartTime  string  `json:"start-time"`
}

// logEvent mirrors the event line format from report.JSONLog.
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
}

// ParseLog reads an NDJSON event log and returns the header metadata and
// the parsed event list. Events with unknown types are skipped.
func ParseLog(r io.Reader) (*LogMeta, []peer.Event, error) {
	scanner := bufio.NewScanner(r)

	// Parse header line.
	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("empty event log")
	}

	var hdr logHeader
	if err := json.Unmarshal(scanner.Bytes(), &hdr); err != nil {
		return nil, nil, fmt.Errorf("parsing header: %w", err)
	}
	if hdr.RecordType != "header" {
		return nil, nil, fmt.Errorf("first line must be a header (got %q)", hdr.RecordType)
	}
	if hdr.Peers < 1 {
		return nil, nil, fmt.Errorf("invalid peer count in header: %d", hdr.Peers)
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

	// Parse event lines.
	var events []peer.Event
	for scanner.Scan() {
		var ev logEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if ev.RecordType != "event" {
			continue
		}

		et, ok := eventTypeFromString[ev.EventType]
		if !ok {
			continue
		}

		evTime := startTime.Add(time.Duration(ev.TimeOffsetMS) * time.Millisecond)

		var prefix netip.Prefix
		if ev.Prefix != "" {
			var pfxErr error
			prefix, pfxErr = netip.ParsePrefix(ev.Prefix)
			if pfxErr != nil {
				return nil, nil, fmt.Errorf("parsing prefix %q at seq %d: %w", ev.Prefix, ev.Seq, pfxErr)
			}
		}

		events = append(events, peer.Event{
			Type:        et,
			PeerIndex:   ev.PeerIndex,
			Time:        evTime,
			Prefix:      prefix,
			Count:       ev.Count,
			ChaosAction: ev.ChaosAction,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading event log: %w", err)
	}

	return meta, events, nil
}
