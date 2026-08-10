// Design: docs/architecture/api/commands.md — BGP monitor command handler
// Related: format.go — visual text line formatting
// Overview: doc.go — package doc + YANG import

package monitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errMissingValueForPeer      = errors.New("missing value for 'peer'")
	errEmptyPeerSelector        = errors.New("empty peer selector")
	errMissingValueForEvent     = errors.New("missing value for 'event'")
	errEmptyEventTypeInList     = errors.New("empty event type in list")
	errMissingValueForDirection = errors.New("missing value for 'direction'")
)

// WireMethod is the YANG RPC wire method for the monitor command.
const WireMethod = "ze-bgp:monitor"

// monitorChanSize is the buffered channel size for monitor event delivery.
const monitorChanSize = 256

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: WireMethod,
			Handler:    handleMonitor,
		},
	)
	// Register the compact one-liner formatter for monitor event display.
	pluginserver.RegisterMonitorEventFormatter(formatMonitorLine)
	// Register "monitor event" streaming handler at engine level (verb-first: <action> <module>).
	pluginserver.RegisterStreamingHandler("monitor event", pluginserver.StreamEventMonitor)

	// "monitor bgp" is the dashboard command, handled by the TUI model.
	// No streaming handler needed -- the CLI intercepts it before it
	// reaches the streaming path. (monitor traceroute is owned by the
	// dedicated traceroute feature module, internal/component/traceroute/cmd.)
}

// handleMonitor is the RPC handler for non-streaming callers (interactive CLI dispatch).
// Returns the parsed monitor configuration as JSON. Actual streaming is handled
// by StreamMonitor which is called from the SSH exec streaming path.
func handleMonitor(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	opts, err := parseMonitorArgs(args)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  err.Error(),
		}, err
	}

	// Return the parsed configuration. The SSH streaming path uses StreamMonitor instead.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"status":      "monitor-configured",
			"peer":        opts.peer,
			"event-types": opts.eventTypes,
			"direction":   opts.direction,
		},
	}, nil
}

// StreamMonitor is the entry point for SSH streaming monitor sessions.
// It registers a MonitorClient, writes a header line, then streams events
// to the writer until the context is canceled or a write error occurs.
// This function blocks until the monitor session ends.
func StreamMonitor(ctx context.Context, mm *pluginserver.MonitorManager, w io.Writer, args []string) error {
	opts, err := parseMonitorArgs(args)
	if err != nil {
		return err
	}

	// Build subscriptions from parsed options.
	subs := buildSubscriptions(opts)

	// Create monitor client with a unique ID.
	clientCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	id := textbuf.StrInt("monitor-", int64(nextMonitorID.Add(1)))
	mc := pluginserver.NewMonitorClient(clientCtx, id, subs, monitorChanSize)
	mm.Add(mc)
	defer mm.Remove(id)

	// Write header line.
	header := formatHeader(opts)
	if _, err := fmt.Fprintln(w, header); err != nil { //nolint:errcheck // output
		return err
	}

	// Stream events until disconnect or shutdown.
	for {
		select {
		case event, ok := <-mc.EventChan:
			if !ok {
				return nil
			}
			// Check for dropped events and prepend warning.
			if d := mc.Dropped.Swap(0); d > 0 {
				warning := textbuf.StrIntStr("--- WARNING: dropped ", int64(d), " events (slow reader)")
				if _, err := fmt.Fprintln(w, warning); err != nil { //nolint:errcheck // output
					return err
				}
			}
			if _, err := fmt.Fprintln(w, event); err != nil { //nolint:errcheck // output
				return err
			}
		case <-clientCtx.Done():
			return nil
		}
	}
}

// nextMonitorID generates unique monitor client IDs.
var nextMonitorID atomic.Uint64

// formatHeader builds the header line describing active filters.
func formatHeader(opts *monitorOpts) string {
	var parts []string
	if opts.peer != "" {
		parts = append(parts, "peer="+opts.peer)
	}
	if len(opts.eventTypes) > 0 {
		parts = append(parts, "event="+textbuf.Join(opts.eventTypes, ","))
	}
	if opts.direction != "" {
		parts = append(parts, "direction="+opts.direction)
	}

	if len(parts) == 0 {
		return "monitoring: all events, all peers"
	}
	return "monitoring: " + textbuf.Join(parts, " ")
}

// buildSubscriptions creates Subscription objects from parsed monitor options.
func buildSubscriptions(opts *monitorOpts) []*pluginserver.Subscription {
	eventTypes := opts.eventTypes
	if len(eventTypes) == 0 {
		// Subscribe to all BGP event types.
		eventTypes = allBGPEventTypes
	}

	dir := events.DirBoth
	if opts.direction != "" {
		dir = events.ParseDirection(opts.direction)
	}

	var peerFilter *pluginserver.PeerFilter
	if opts.peer != "" {
		peerFilter = &pluginserver.PeerFilter{Selector: opts.peer}
	}

	bgpNS := events.LookupNamespaceID(bgpevents.Namespace)
	subs := make([]*pluginserver.Subscription, len(eventTypes))
	for i, et := range eventTypes {
		subs[i] = &pluginserver.Subscription{
			Namespace:  bgpNS,
			EventType:  events.LookupEventTypeID(et),
			Direction:  dir,
			PeerFilter: peerFilter,
		}
	}
	return subs
}

var allBGPEventTypes = []string{
	bgpevents.EventUpdate,
	bgpevents.EventOpen,
	bgpevents.EventNotification,
	bgpevents.EventKeepalive,
	bgpevents.EventRefresh,
	bgpevents.EventState,
	bgpevents.EventNegotiated,
	bgpevents.EventEOR,
}

// monitorOpts holds parsed monitor command options.
type monitorOpts struct {
	peer       string   // Peer filter: IP, name, "!sel" (exclusion), or "*" (empty = all peers)
	eventTypes []string // Event type filter (nil = all events)
	direction  string   // Direction filter: "received", "sent" (empty = both)
}

// parseMonitorArgs parses keyword arguments for the monitor command.
// Supported keywords: peer <selector>, event <type>[,<type>], direction received|sent.
// Peer selector accepts IP addresses, peer names, "*" (all), or "!sel" (exclusion).
// Keywords may appear in any order. Each keyword may appear at most once.
func parseMonitorArgs(args []string) (*monitorOpts, error) {
	opts := &monitorOpts{}
	seen := make(map[string]bool)

	i := 0
	for i < len(args) {
		keyword := args[i]

		if seen[keyword] {
			return nil, fmt.Errorf("duplicate keyword: %s", keyword)
		}
		seen[keyword] = true

		switch keyword {
		case "peer":
			if i+1 >= len(args) {
				return nil, errMissingValueForPeer
			}
			i++
			peer := args[i]
			if peer == "" {
				return nil, errEmptyPeerSelector
			}
			if peer[0] == '!' {
				rest := peer[1:]
				if rest == "" {
					return nil, fmt.Errorf("invalid peer selector: %s (empty after exclusion)", peer)
				}
				if rest[0] == '!' {
					return nil, fmt.Errorf("invalid peer selector: %s (double exclusion)", peer)
				}
			}
			opts.peer = peer

		case "event":
			if i+1 >= len(args) {
				return nil, errMissingValueForEvent
			}
			i++
			parts, count := stringsx.SplitCount(args[i], ",")
			types := make([]string, 0, count)
			for _, t := range parts {
				if t == "" {
					return nil, errEmptyEventTypeInList
				}
				if !events.IsValidEvent(bgpevents.Namespace, t) {
					return nil, fmt.Errorf("invalid event type: %s (valid: %s)", t, events.ValidEventNames(bgpevents.Namespace))
				}
				types = append(types, t)
			}
			opts.eventTypes = types

		case "direction":
			if i+1 >= len(args) {
				return nil, errMissingValueForDirection
			}
			i++
			dir := args[i]
			if dir != events.DirectionReceived && dir != events.DirectionSent {
				return nil, fmt.Errorf("invalid direction: %s (valid: received, sent)", dir)
			}
			opts.direction = dir

		default:
			return nil, fmt.Errorf("unknown keyword: %s", keyword)
		}

		i++
	}

	return opts, nil
}
