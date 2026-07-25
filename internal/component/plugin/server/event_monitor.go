// Design: docs/architecture/api/process-protocol.md -- event monitoring
// Overview: handler.go -- streaming handler registry
// Related: monitor.go -- MonitorClient and MonitorManager

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errDuplicateKeywordInclude               = errors.New("duplicate keyword: include")
	errIncludeAndExcludeAreMutuallyExclusive = errors.New("include and exclude are mutually exclusive")
	errIncludeRequiresACommaSeparatedList    = errors.New("include requires a comma-separated list of event types")
	errDuplicateKeywordExclude               = errors.New("duplicate keyword: exclude")
	errExcludeRequiresACommaSeparatedList    = errors.New("exclude requires a comma-separated list of event types")
	errDuplicateKeywordPeer                  = errors.New("duplicate keyword: peer")
	errPeerRequiresASelectorIpName           = errors.New("peer requires a selector (IP, name, or !exclusion)")
	errDuplicateKeywordDirection             = errors.New("duplicate keyword: direction")
	errDirectionRequiresReceivedOrSent       = errors.New("direction requires 'received' or 'sent'")
	errEmptyEventTypeInListCheck             = errors.New("empty event type in list (check for trailing comma)")
)

// EventMonitorOpts holds parsed arguments for the event monitor command.
type EventMonitorOpts struct {
	IncludeTypes []string
	ExcludeTypes []string
	Peer         string
	Direction    string
}

var nextEventMonitorID uint64

// StreamEventMonitor is the streaming handler for the "event monitor" command.
// It parses arguments, creates subscriptions, registers a MonitorClient,
// and streams events until the context is canceled.
// Registration: called from internal/component/bgp/plugins/cmd/monitor/monitor.go init()
// via RegisterStreamingHandler("event monitor", StreamEventMonitor).
func StreamEventMonitor(ctx context.Context, s *Server, w io.Writer, _ string, args []string) error {
	opts, err := ParseEventMonitorArgs(args)
	if err != nil {
		return err
	}

	subs := BuildEventMonitorSubscriptions(opts)
	id := textbuf.StrUint("event-monitor-", atomic.AddUint64(&nextEventMonitorID, 1))

	clientCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	client := NewMonitorClient(clientCtx, id, subs, 64)

	monitors := s.Monitors()
	monitors.Add(client)
	defer monitors.Remove(id)

	// Write header line describing active filters.
	header := formatEventMonitorHeader(opts)
	if _, err := fmt.Fprintln(w, header); err != nil { //nolint:errcheck // output
		return err
	}

	for {
		select {
		case <-clientCtx.Done():
			return nil
		case event, ok := <-client.EventChan:
			if !ok {
				return nil
			}
			if _, err := fmt.Fprintln(w, event); err != nil { //nolint:errcheck // output
				return err
			}
		}
	}
}

// excludedFromMonitor contains entries in the BGP event namespace that are not
// event types but rather config flags (e.g., "sent" is a direction, not an event).
var excludedFromMonitor = map[string]bool{
	events.DirectionSent: true,
}

// ParseEventMonitorArgs parses keyword arguments for the event monitor command.
//
// Syntax: [include|exclude <types>] [peer <selector>] [direction received|sent]
// Keywords may appear in any order. Include and exclude are mutually exclusive.
func ParseEventMonitorArgs(args []string) (*EventMonitorOpts, error) {
	opts := &EventMonitorOpts{}
	seen := make(map[string]bool)

	for i := 0; i < len(args); i++ {
		kw := strings.ToLower(args[i])

		if kw == "include" {
			if seen["include"] {
				return nil, errDuplicateKeywordInclude
			}
			if seen["exclude"] {
				return nil, errIncludeAndExcludeAreMutuallyExclusive
			}
			seen["include"] = true
			i++
			if i >= len(args) {
				return nil, errIncludeRequiresACommaSeparatedList
			}
			types, err := parseEventTypeList(args[i])
			if err != nil {
				return nil, err
			}
			opts.IncludeTypes = types
			continue
		}

		if kw == "exclude" {
			if seen["exclude"] {
				return nil, errDuplicateKeywordExclude
			}
			if seen["include"] {
				return nil, errIncludeAndExcludeAreMutuallyExclusive
			}
			seen["exclude"] = true
			i++
			if i >= len(args) {
				return nil, errExcludeRequiresACommaSeparatedList
			}
			types, err := parseEventTypeList(args[i])
			if err != nil {
				return nil, err
			}
			opts.ExcludeTypes = types
			continue
		}

		if kw == kwPeer {
			if seen[kwPeer] {
				return nil, errDuplicateKeywordPeer
			}
			seen[kwPeer] = true
			i++
			if i >= len(args) {
				return nil, errPeerRequiresASelectorIpName
			}
			peer := args[i]
			if peer == "" || peer == "!" || peer == "!!" {
				return nil, fmt.Errorf("invalid peer selector %q", peer)
			}
			if len(peer) > 253 {
				return nil, fmt.Errorf("peer selector too long (%d chars, max 253)", len(peer))
			}
			opts.Peer = peer
			continue
		}

		if kw == kwDirection {
			if seen[kwDirection] {
				return nil, errDuplicateKeywordDirection
			}
			seen[kwDirection] = true
			i++
			if i >= len(args) {
				return nil, errDirectionRequiresReceivedOrSent
			}
			d := strings.ToLower(args[i])
			if d != events.DirectionReceived && d != events.DirectionSent {
				return nil, fmt.Errorf("invalid direction %q: must be 'received' or 'sent'", d)
			}
			opts.Direction = d
			continue
		}

		return nil, fmt.Errorf("unknown keyword %q (valid: include, exclude, peer, direction)", kw)
	}

	return opts, nil
}

// parseEventTypeList splits a comma-separated list of event types, trims whitespace,
// validates each type, and rejects empty entries (e.g., trailing comma).
func parseEventTypeList(raw string) ([]string, error) {
	parts, count := stringsx.SplitCount(raw, ",")
	result := make([]string, 0, count)
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			return nil, errEmptyEventTypeInListCheck
		}
		if err := validateEventTypeAnyNamespace(t); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, nil
}

// validateEventTypeAnyNamespace returns nil if the event type is valid in any
// namespace (BGP or RIB). Returns an error if unknown or excluded (e.g., "sent"
// is a direction flag, not an event type).
func validateEventTypeAnyNamespace(eventType string) error {
	if excludedFromMonitor[eventType] {
		return fmt.Errorf("invalid event type %q: %q is a direction, not an event type", eventType, eventType)
	}

	if events.IsValidEventAnyNamespace(eventType) {
		return nil
	}

	// Build a filtered list of valid types for the error message.
	all := events.AllEventTypes()
	seen := make(map[string]bool)
	for _, types := range all {
		for _, t := range types {
			if !excludedFromMonitor[t] {
				seen[t] = true
			}
		}
	}
	valid := make([]string, 0, len(seen))
	for k := range seen {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	return fmt.Errorf("invalid event type %q (valid: %s)", eventType, textbuf.Join(valid, ", "))
}

// allEventTypes returns all valid event types across all namespaces,
// excluding non-event entries like "sent".
func allEventTypes() map[string][]string {
	all := events.AllEventTypes()
	for ns, types := range all {
		filtered := types[:0]
		for _, et := range types {
			if !excludedFromMonitor[et] {
				filtered = append(filtered, et)
			}
		}
		all[ns] = filtered
	}
	return all
}

// BuildEventMonitorSubscriptions creates subscriptions from parsed options.
// With no include/exclude filter, subscribes to all event types across all namespaces.
// With include, subscribes only to those types (in whichever namespaces they exist).
// With exclude, subscribes to all types except those listed.
func BuildEventMonitorSubscriptions(opts *EventMonitorOpts) []*Subscription {
	var peerFilter *PeerFilter
	if opts.Peer != "" {
		peerFilter = &PeerFilter{Selector: opts.Peer}
	}

	dir := events.ParseDirection(opts.Direction)
	if dir == events.DirUnspecified {
		dir = events.DirBoth
	}

	all := allEventTypes()

	included := make(map[string]bool, len(opts.IncludeTypes))
	for _, t := range opts.IncludeTypes {
		included[t] = true
	}

	excluded := make(map[string]bool, len(opts.ExcludeTypes))
	for _, t := range opts.ExcludeTypes {
		excluded[t] = true
	}

	var subs []*Subscription
	for ns, types := range all {
		nsID := events.LookupNamespaceID(ns)
		for _, et := range types {
			if len(included) > 0 && !included[et] {
				continue
			}
			if excluded[et] {
				continue
			}
			subs = append(subs, &Subscription{
				Namespace:  nsID,
				EventType:  events.LookupEventTypeID(et),
				Direction:  dir,
				PeerFilter: peerFilter,
			})
		}
	}

	return subs
}

// formatEventMonitorHeader returns a human-readable header line for the monitor session.
func formatEventMonitorHeader(opts *EventMonitorOpts) string {
	var tb textbuf.Buffer
	var parts []string

	if len(opts.IncludeTypes) > 0 {
		parts = append(parts, tb.Str("include=").Join(opts.IncludeTypes, ",").String())
	}
	if len(opts.ExcludeTypes) > 0 {
		parts = append(parts, tb.Reset().Str("exclude=").Join(opts.ExcludeTypes, ",").String())
	}
	if opts.Peer != "" {
		parts = append(parts, tb.Reset().Str("peer=").Str(opts.Peer).String())
	}
	if opts.Direction != "" {
		parts = append(parts, tb.Reset().Str("direction=").Str(opts.Direction).String())
	}

	if len(parts) == 0 {
		return "monitoring: all events, all peers"
	}
	return tb.Reset().Str("monitoring: ").Join(parts, ", ").String()
}
