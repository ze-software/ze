// Design: docs/architecture/api/process-protocol.md — plugin RPC dispatch
// Related: server.go — Server struct and lifecycle

package plugin

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/ipc"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// handleSingleProcessCommandsRPC handles runtime commands for an RPC-mode plugin.
// Reads from engineConnA and dispatches plugin->engine RPCs (update-route, subscribe, etc.)
// concurrently. Each request is dispatched in its own goroutine so that slow handlers
// (e.g., update-route) don't block the read loop and starve other requests.
// Event delivery to plugins is handled directly via engineConnB.SendDeliverEvent
// in OnMessageReceived, OnPeerStateChange, etc.
func (s *Server) handleSingleProcessCommandsRPC(proc *Process) {
	defer s.cleanupProcess(proc)

	connA := proc.ConnA()
	if connA == nil {
		logger().Debug("rpc runtime: no connection (startup failed?)", "plugin", proc.Name())
		return
	}

	// WaitGroup tracks in-flight dispatches for clean shutdown.
	var wg sync.WaitGroup
	defer wg.Wait()

	// Plugin->engine RPC loop: read from engineConnA, dispatch in goroutines.
	for {
		req, err := connA.ReadRequest(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				return // Server shutting down
			}
			logger().Debug("rpc runtime: read failed", "plugin", proc.Name(), "error", err)
			return // Connection closed (plugin exited)
		}

		wg.Go(func() {
			s.dispatchPluginRPC(proc, connA, req)
		})
	}
}

// dispatchPluginRPC handles a single plugin->engine RPC request.
// Unknown or empty methods get an explicit error per ze's fail-on-unknown rule.
// Generic RPCs (update-route, subscribe, unsubscribe) are handled directly.
// BGP codec RPCs (decode-nlri, encode-nlri, etc.) are delegated to BGPHooks.
func (s *Server) dispatchPluginRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	switch req.Method {
	case "ze-plugin-engine:update-route":
		s.handleUpdateRouteRPC(proc, connA, req)
		return
	case "ze-plugin-engine:subscribe-events":
		s.handleSubscribeEventsRPC(proc, connA, req)
		return
	case "ze-plugin-engine:unsubscribe-events":
		s.handleUnsubscribeEventsRPC(proc, connA, req)
		return
	}

	// Try BGP codec hook for remaining methods
	if s.bgpHooks != nil && s.bgpHooks.CodecRPCHandler != nil {
		codec := s.bgpHooks.CodecRPCHandler(req.Method)
		if codec != nil {
			s.handleCodecRPC(proc, connA, req, codec)
			return
		}
	}

	if err := connA.SendError(s.ctx, req.ID, "unknown method: "+req.Method); err != nil {
		logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", err)
	}
}

// handleUpdateRouteRPC handles ze-plugin-engine:update-route from a plugin.
// Dispatches the command string through the standard command dispatcher.
func (s *Server) handleUpdateRouteRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	var input rpc.UpdateRouteInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "invalid update-route params: "+err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	cmdCtx := &CommandContext{
		Server:  s,
		Process: proc,
		Peer:    input.PeerSelector,
	}
	if cmdCtx.Peer == "" {
		cmdCtx.Peer = "*"
	}

	// Reconstruct the full command for the dispatcher.
	// Commands from "bgp peer <sel> <cmd>" arrive with the peer selector stripped
	// and command as just "<cmd>" (e.g., "update text ..."). These need "bgp peer "
	// prepended for the dispatcher to match "bgp peer update", "bgp peer teardown", etc.
	//
	// Commands that aren't peer-targeted (e.g., "bgp watchdog announce dnsr",
	// "bgp cache list") arrive with the full "bgp ..." prefix intact and must be
	// passed through directly -- prepending "bgp peer " would create an unmatchable
	// "bgp peer bgp watchdog ..." command.
	var dispatchCmd string
	if strings.HasPrefix(strings.ToLower(input.Command), "bgp ") {
		dispatchCmd = input.Command
	} else {
		dispatchCmd = "bgp peer " + input.Command
	}

	resp, err := s.dispatcher.Dispatch(cmdCtx, dispatchCmd)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			if sendErr := connA.SendResult(s.ctx, req.ID, &rpc.UpdateRouteOutput{}); sendErr != nil {
				logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
			}
			return
		}
		if sendErr := connA.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	// Extract route counts from response if available
	output := &rpc.UpdateRouteOutput{}
	if resp != nil && resp.Data != nil {
		if m, ok := resp.Data.(map[string]any); ok {
			if v, ok := m["peers-affected"]; ok {
				if n, ok := v.(float64); ok {
					output.PeersAffected = uint32(n)
				}
			}
			if v, ok := m["routes-sent"]; ok {
				if n, ok := v.(float64); ok {
					output.RoutesSent = uint32(n)
				}
			}
		}
	}

	if sendErr := connA.SendResult(s.ctx, req.ID, output); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// parseEventString splits an event string like "update direction sent" into
// (eventType, direction). If no "direction" keyword is present, returns DirectionBoth.
// This mirrors the text protocol's ParseSubscription logic for RPC event strings.
func parseEventString(event string) (string, string) {
	parts := strings.Fields(event)
	if len(parts) >= 3 && parts[1] == "direction" {
		return parts[0], parts[2]
	}
	return event, DirectionBoth
}

// registerSubscriptions registers event subscriptions for a process.
// Parses event strings (e.g. "update direction sent") into EventType + Direction.
func (s *Server) registerSubscriptions(proc *Process, input *rpc.SubscribeEventsInput) {
	if input.Format != "" {
		proc.SetFormat(input.Format)
	}

	for _, event := range input.Events {
		eventType, direction := parseEventString(event)
		sub := &Subscription{
			Namespace: NamespaceBGP,
			EventType: eventType,
			Direction: direction,
		}
		if len(input.Peers) > 0 {
			sub.PeerFilter = &PeerFilter{Selector: input.Peers[0]}
		}
		s.subscriptions.Add(proc, sub)
	}
}

// handleSubscribeEventsRPC handles ze-plugin-engine:subscribe-events from a plugin.
func (s *Server) handleSubscribeEventsRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	var input rpc.SubscribeEventsInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "invalid subscribe params: "+err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	if s.subscriptions == nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "subscription manager not available"); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	s.registerSubscriptions(proc, &input)
	if sendErr := connA.SendResult(s.ctx, req.ID, nil); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleUnsubscribeEventsRPC handles ze-plugin-engine:unsubscribe-events from a plugin.
func (s *Server) handleUnsubscribeEventsRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	if s.subscriptions == nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "subscription manager not available"); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	s.subscriptions.ClearProcess(proc)
	if sendErr := connA.SendResult(s.ctx, req.ID, nil); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleCodecRPC is a shared helper for plugin->engine codec RPCs (decode-nlri, encode-nlri).
// The codec callback unmarshals params and calls the registry; it returns the result to send
// or an error to relay back to the plugin.
func (s *Server) handleCodecRPC(proc *Process, connA *PluginConn, req *ipc.Request,
	codec func(json.RawMessage) (any, error),
) {
	result, err := codec(req.Params)
	if err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	if sendErr := connA.SendResult(s.ctx, req.ID, result); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// cleanupProcess handles cleanup when a process exits.
func (s *Server) cleanupProcess(proc *Process) {
	// Unregister all commands from this process
	s.dispatcher.Registry().UnregisterAll(proc)

	// Cancel all pending requests
	s.dispatcher.Pending().CancelAll(proc)

	// Clear all subscriptions for this process
	if s.subscriptions != nil {
		s.subscriptions.ClearProcess(proc)
	}

	// Remove cache consumer tracking for this plugin.
	// UnregisterConsumer decrements pending counts for unacked entries
	// so they can be evicted instead of leaking.
	if proc.IsCacheConsumer() && s.reactor != nil {
		s.reactor.UnregisterCacheConsumer(proc.Name())
	}
}
