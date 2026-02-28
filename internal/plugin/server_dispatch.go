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
		// Text-mode plugins have no post-startup RPC on Socket A.
		// Wait for server shutdown so cleanup runs at the right time.
		if proc.TextConnB() != nil {
			<-s.ctx.Done()
		} else {
			logger().Debug("rpc runtime: no connection (startup failed?)", "plugin", proc.Name())
		}
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
	case "ze-plugin-engine:dispatch-command":
		s.handleDispatchCommandRPC(proc, connA, req)
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

// handleDispatchCommandRPC handles ze-plugin-engine:dispatch-command from a plugin.
// Dispatches the command string through the standard command dispatcher and returns
// the full {status, data} response, enabling inter-plugin communication.
func (s *Server) handleDispatchCommandRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	var input rpc.DispatchCommandInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "invalid dispatch-command params: "+err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	cmdCtx := &CommandContext{
		Server:  s,
		Process: proc,
	}

	resp, err := s.dispatcher.Dispatch(cmdCtx, input.Command)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			if sendErr := connA.SendResult(s.ctx, req.ID, &rpc.DispatchCommandOutput{Status: StatusDone}); sendErr != nil {
				logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
			}
			return
		}
		if sendErr := connA.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	output := responseToDispatchOutput(resp)
	if sendErr := connA.SendResult(s.ctx, req.ID, output); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// responseToDispatchOutput converts a dispatcher Response to DispatchCommandOutput.
// The Data field is JSON-encoded if it's a structured type, or used as-is for strings.
func responseToDispatchOutput(resp *Response) *rpc.DispatchCommandOutput {
	output := &rpc.DispatchCommandOutput{}
	if resp == nil {
		output.Status = StatusDone
		return output
	}
	output.Status = resp.Status
	if resp.Data != nil {
		if s, ok := resp.Data.(string); ok {
			output.Data = s
		} else {
			encoded, err := json.Marshal(resp.Data)
			if err != nil {
				output.Status = StatusError
				output.Data = "marshal response data: " + err.Error()
			} else {
				output.Data = string(encoded)
			}
		}
	}
	return output
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
	if input.Encoding != "" {
		proc.SetEncoding(input.Encoding)
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

// dispatchPluginRPCDirect handles a plugin→engine RPC without socket I/O.
// Used by DirectBridge for internal plugins. Returns the result as a JSON-RPC
// response envelope (matching what callEngineRaw/CheckResponse/ParseResponse expect).
// Handlers return json.RawMessage only — all errors are encoded in the envelope,
// never as Go errors — so the second return is always nil.
func (s *Server) dispatchPluginRPCDirect(proc *Process, method string, params json.RawMessage) (json.RawMessage, error) {
	// Known plugin→engine RPCs
	switch method {
	case "ze-plugin-engine:update-route":
		return s.handleUpdateRouteDirect(proc, params), nil
	case "ze-plugin-engine:dispatch-command":
		return s.handleDispatchCommandDirect(proc, params), nil
	case "ze-plugin-engine:subscribe-events":
		return s.handleSubscribeEventsDirect(proc, params), nil
	case "ze-plugin-engine:unsubscribe-events":
		return s.handleUnsubscribeEventsDirect(proc), nil
	}

	// Try BGP codec hook for remaining methods
	if s.bgpHooks != nil && s.bgpHooks.CodecRPCHandler != nil {
		codec := s.bgpHooks.CodecRPCHandler(method)
		if codec != nil {
			return handleCodecRPCDirect(codec, params), nil
		}
	}

	// Unknown methods get an explicit error per ze's fail-on-unknown rule
	return directErrorResponse("unknown method: " + method), nil
}

// handleUpdateRouteDirect handles update-route without socket I/O.
// Returns a JSON-RPC envelope; errors are encoded in the envelope, not as Go errors.
func (s *Server) handleUpdateRouteDirect(proc *Process, params json.RawMessage) json.RawMessage {
	var input rpc.UpdateRouteInput
	if err := json.Unmarshal(params, &input); err != nil {
		return directErrorResponse("invalid update-route params: " + err.Error())
	}

	cmdCtx := &CommandContext{
		Server:  s,
		Process: proc,
		Peer:    input.PeerSelector,
	}
	if cmdCtx.Peer == "" {
		cmdCtx.Peer = "*"
	}

	var dispatchCmd string
	if strings.HasPrefix(strings.ToLower(input.Command), "bgp ") {
		dispatchCmd = input.Command
	} else {
		dispatchCmd = "bgp peer " + input.Command
	}

	resp, err := s.dispatcher.Dispatch(cmdCtx, dispatchCmd)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			return directResultResponse(&rpc.UpdateRouteOutput{})
		}
		return directErrorResponse(err.Error())
	}

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

	return directResultResponse(output)
}

// handleDispatchCommandDirect handles dispatch-command without socket I/O.
// Returns a JSON-RPC envelope; errors are encoded in the envelope, not as Go errors.
func (s *Server) handleDispatchCommandDirect(proc *Process, params json.RawMessage) json.RawMessage {
	var input rpc.DispatchCommandInput
	if err := json.Unmarshal(params, &input); err != nil {
		return directErrorResponse("invalid dispatch-command params: " + err.Error())
	}

	cmdCtx := &CommandContext{
		Server:  s,
		Process: proc,
	}

	resp, err := s.dispatcher.Dispatch(cmdCtx, input.Command)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			return directResultResponse(&rpc.DispatchCommandOutput{Status: StatusDone})
		}
		return directErrorResponse(err.Error())
	}

	return directResultResponse(responseToDispatchOutput(resp))
}

// handleSubscribeEventsDirect handles subscribe-events without socket I/O.
// Returns a JSON-RPC envelope; errors are encoded in the envelope, not as Go errors.
func (s *Server) handleSubscribeEventsDirect(proc *Process, params json.RawMessage) json.RawMessage {
	var input rpc.SubscribeEventsInput
	if err := json.Unmarshal(params, &input); err != nil {
		return directErrorResponse("invalid subscribe params: " + err.Error())
	}

	if s.subscriptions == nil {
		return directErrorResponse("subscription manager not available")
	}

	s.registerSubscriptions(proc, &input)
	return directResultResponse(nil)
}

// handleUnsubscribeEventsDirect handles unsubscribe-events without socket I/O.
// Returns a JSON-RPC envelope; errors are encoded in the envelope, not as Go errors.
func (s *Server) handleUnsubscribeEventsDirect(proc *Process) json.RawMessage {
	if s.subscriptions == nil {
		return directErrorResponse("subscription manager not available")
	}

	s.subscriptions.ClearProcess(proc)
	return directResultResponse(nil)
}

// handleCodecRPCDirect handles codec RPCs without socket I/O.
// Returns a JSON-RPC envelope; errors are encoded in the envelope, not as Go errors.
func handleCodecRPCDirect(codec func(json.RawMessage) (any, error), params json.RawMessage) json.RawMessage {
	result, err := codec(params)
	if err != nil {
		return directErrorResponse(err.Error())
	}
	return directResultResponse(result)
}

// directResultResponse builds a JSON-RPC result envelope.
func directResultResponse(data any) json.RawMessage {
	if data == nil {
		return json.RawMessage(`{"result":null}`)
	}
	result, err := json.Marshal(data)
	if err != nil {
		return json.RawMessage(`{"error":"marshal result: ` + err.Error() + `"}`)
	}
	return json.RawMessage(`{"result":` + string(result) + `}`)
}

// directErrorResponse builds a JSON-RPC error envelope.
func directErrorResponse(msg string) json.RawMessage {
	escaped, err := json.Marshal(msg)
	if err != nil {
		return json.RawMessage(`{"error":"internal error"}`)
	}
	return json.RawMessage(`{"error":` + string(escaped) + `}`)
}

// wireBridgeDispatch sets up the DirectBridge's DispatchRPC handler for an internal
// plugin's process. Called after the 5-stage startup completes for internal plugins.
func (s *Server) wireBridgeDispatch(proc *Process) {
	if proc.bridge == nil {
		return
	}
	proc.bridge.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		return s.dispatchPluginRPCDirect(proc, method, params)
	})
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
