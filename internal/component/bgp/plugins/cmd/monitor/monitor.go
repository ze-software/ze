// Design: docs/architecture/api/commands.md — BGP monitor command handler
// Related: format.go — visual text line formatting
// Overview: doc.go — package doc + YANG import

package monitor

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"

	bgpevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
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
	pluginserver.RegisterMonitorEventFormatter(FormatMonitorLine)
	// Register "monitor event" streaming handler at engine level (verb-first: <action> <module>).
	pluginserver.RegisterStreamingHandler("monitor event", func(ctx context.Context, s *pluginserver.Server, w io.Writer, username string, args []string) error {
		return pluginserver.StreamEventMonitor(ctx, s, w, username, args)
	})

	// "monitor bgp" is the dashboard command, handled by the TUI model.
	// Follows verb-first convention: <action> <module>.
	// No streaming handler needed -- the CLI intercepts "monitor bgp" before
	// it reaches the streaming path.
}

// handleMonitor is the RPC handler for non-streaming callers (interactive CLI dispatch).
// Returns the parsed monitor configuration as JSON. Actual streaming is handled
// by StreamMonitor which is called from the SSH exec streaming path.
func handleMonitor(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	opts, err := parseMonitorArgs(args)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   err.Error(),
		}, err
	}

	// Return the parsed configuration. The SSH streaming path uses StreamMonitor instead.
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
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

	id := fmt.Sprintf("monitor-%d", nextMonitorID.Add(1))
	mc := pluginserver.NewMonitorClient(clientCtx, id, subs, monitorChanSize)
	mm.Add(mc)
	defer mm.Remove(id)

	// Write header line.
	header := formatHeader(opts)
	if _, err := fmt.Fprintln(w, header); err != nil {
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
				warning := fmt.Sprintf("--- WARNING: dropped %d events (slow reader)", d)
				if _, err := fmt.Fprintln(w, warning); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w, event); err != nil {
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
		parts = append(parts, "event="+strings.Join(opts.eventTypes, ","))
	}
	if opts.direction != "" {
		parts = append(parts, "direction="+opts.direction)
	}

	if len(parts) == 0 {
		return "monitoring: all events, all peers"
	}
	return "monitoring: " + strings.Join(parts, " ")
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
				return nil, fmt.Errorf("missing value for 'peer'")
			}
			i++
			peer := args[i]
			if peer == "" {
				return nil, fmt.Errorf("empty peer selector")
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
				return nil, fmt.Errorf("missing value for 'event'")
			}
			i++
			types := strings.Split(args[i], ",")
			for _, t := range types {
				if t == "" {
					return nil, fmt.Errorf("empty event type in list")
				}
				if !events.IsValidEvent(bgpevents.Namespace, t) {
					return nil, fmt.Errorf("invalid event type: %s (valid: %s)", t, events.ValidEventNames(bgpevents.Namespace))
				}
			}
			opts.eventTypes = types

		case "direction":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing value for 'direction'")
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
