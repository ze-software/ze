// Design: docs/architecture/api/process-protocol.md — plugin RPC dispatch
// Overview: server.go — Server struct and lifecycle
// Related: engine_event.go — engine-side stream pub/sub fans out from deliverEvent

package server

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"

	bgpevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/events"
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	plugipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// handleSingleProcessCommandsRPC handles runtime commands for an RPC-mode plugin.
// Reads plugin-initiated RPCs (update-route, subscribe, etc.) and dispatches them
// concurrently. Each request is dispatched in its own goroutine so that slow handlers
// (e.g., update-route) don't block the read loop and starve other requests.
//
// For bridge-mode plugins (internal plugins that negotiated Transport="bridge"
// during Stage 5), the SDK closes its end of the mux after the bridge switch,
// and all plugin->engine RPCs flow via DirectBridge (wired by wireBridgeDispatch).
// The mux read loop is skipped in that case -- reading it would immediately
// return ErrMuxConnClosed and incorrectly trigger cleanupProcess, causing
// Server.Wait to unblock and the daemon to shut down during startup.
func (s *Server) handleSingleProcessCommandsRPC(proc *process.Process) {
	defer s.cleanupProcess(proc)

	conn := proc.Conn()
	if conn == nil {
		logger().Debug("rpc runtime: no connection (startup failed?)", "plugin", proc.Name())
		return
	}

	// Bridge-mode plugins: no mux to read. Hold the WaitGroup entry until the
	// server is shutting down so Server.Wait() blocks until actual termination.
	// Plugin->engine RPCs still flow via DirectBridge independently of this.
	if conn.HasBridge() {
		<-s.ctx.Done()
		return
	}

	// WaitGroup tracks in-flight dispatches for clean shutdown.
	var wg sync.WaitGroup
	defer wg.Wait()

	// Plugin->engine RPC loop: read requests, dispatch in goroutines.
	for {
		req, err := conn.ReadRequest(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				return // Server shutting down
			}
			logger().Debug("rpc runtime: read failed", "plugin", proc.Name(), "error", err)
			return // Connection closed (plugin exited)
		}

		wg.Go(func() {
			s.dispatchPluginRPC(proc, conn, req)
		})
	}
}

// dispatchPluginRPC handles a single plugin->engine RPC request.
// Unknown or empty methods get an explicit error per ze's fail-on-unknown rule.
// Generic RPCs (update-route, subscribe, unsubscribe) are handled directly.
// Codec RPCs (decode-nlri, encode-nlri, etc.) are delegated via rpcFallback.
func (s *Server) dispatchPluginRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	switch req.Method {
	case "ze-plugin-engine:update-route":
		s.handleUpdateRouteRPC(proc, conn, req)
		return
	case "ze-plugin-engine:dispatch-command":
		s.handleDispatchCommandRPC(proc, conn, req)
		return
	case "ze-plugin-engine:subscribe-events":
		s.handleSubscribeEventsRPC(proc, conn, req)
		return
	case "ze-plugin-engine:unsubscribe-events":
		s.handleUnsubscribeEventsRPC(proc, conn, req)
		return
	case "ze-plugin-engine:emit-event":
		s.handleEmitEventRPC(proc, conn, req)
		return
	}

	// Try registered RPC handlers (codec RPCs, etc.)
	if codec, ok := s.getRPCHandlers()[req.Method]; ok {
		s.handleCodecRPC(proc, conn, req, codec)
		return
	}

	if err := conn.SendError(s.ctx, req.ID, "unknown method: "+req.Method); err != nil {
		logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", err)
	}
}

// handleUpdateRouteRPC handles ze-plugin-engine:update-route from a plugin.
// Dispatches the command string through the standard command dispatcher.
func (s *Server) handleUpdateRouteRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	var input rpc.UpdateRouteInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "invalid update-route params: "+err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}
	cmdCtx := &CommandContext{
		Server:  s,
		Process: proc,
		Peer:    input.PeerSelector,
		Meta:    input.Meta,
	}
	if cmdCtx.Peer == "" {
		cmdCtx.Peer = "*"
	}

	// Reconstruct the full command for the dispatcher.
	// Commands arrive in two forms:
	// 1. Peer subcommands: just "<cmd>" (e.g., "update text ...") -- need "peer <sel> " prepended
	// 2. Top-level commands: "cache ...", "peer ...", "commit ..." -- pass through directly
	//
	// Detect form by checking if the command matches a registered dispatch prefix.
	// If it does, pass through. If not, it's a peer subcommand.
	var dispatchCmd string
	if s.dispatcher.HasCommandPrefix(input.Command) {
		dispatchCmd = input.Command
	} else {
		dispatchCmd = "peer " + cmdCtx.Peer + " " + input.Command
	}

	resp, err := s.dispatcher.Dispatch(cmdCtx, dispatchCmd)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			if sendErr := conn.SendResult(s.ctx, req.ID, &rpc.UpdateRouteOutput{}); sendErr != nil {
				logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
			}
			return
		}
		if sendErr := conn.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
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

	if sendErr := conn.SendResult(s.ctx, req.ID, output); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleDispatchCommandRPC handles ze-plugin-engine:dispatch-command from a plugin.
// Dispatches the command string through the standard command dispatcher and returns
// the full {status, data} response, enabling inter-plugin communication.
func (s *Server) handleDispatchCommandRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	var input rpc.DispatchCommandInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "invalid dispatch-command params: "+err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	// Set plugin name as username so authorization rules apply to plugin-dispatched
	// commands. Without this, the empty username causes authz to return Allow for all
	// commands, bypassing any configured authorization profiles.
	cmdCtx := &CommandContext{
		Server:   s,
		Process:  proc,
		Username: "plugin:" + proc.Name(),
	}

	resp, err := s.dispatcher.Dispatch(cmdCtx, input.Command)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			if sendErr := conn.SendResult(s.ctx, req.ID, &rpc.DispatchCommandOutput{Status: plugin.StatusDone}); sendErr != nil {
				logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
			}
			return
		}
		if s.ctx.Err() != nil {
			logger().Debug("dispatch-command failed (shutting down)", "plugin", proc.Name(), "command", input.Command, "error", err)
		} else {
			logger().Error("dispatch-command failed", "plugin", proc.Name(), "command", input.Command, "error", err)
		}
		if sendErr := conn.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	output := responseToDispatchOutput(resp)
	if sendErr := conn.SendResult(s.ctx, req.ID, output); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// responseToDispatchOutput converts a dispatcher Response to DispatchCommandOutput.
// The Data field is JSON-encoded if it's a structured type, or used as-is for strings.
func responseToDispatchOutput(resp *plugin.Response) *rpc.DispatchCommandOutput {
	output := &rpc.DispatchCommandOutput{}
	if resp == nil {
		output.Status = plugin.StatusDone
		return output
	}
	output.Status = resp.Status
	if resp.Data != nil {
		if s, ok := resp.Data.(string); ok {
			output.Data = s
		} else {
			encoded, err := json.Marshal(resp.Data)
			if err != nil {
				output.Status = plugin.StatusError
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
	return event, events.DirectionBoth
}

// registerSubscriptions registers event subscriptions for a process.
// Parses event strings (e.g. "update direction sent") into EventType + Direction.
func (s *Server) registerSubscriptions(proc *process.Process, input *rpc.SubscribeEventsInput) {
	if input.Format != "" {
		proc.SetFormat(input.Format)
	}
	if input.Encoding != "" {
		proc.SetEncoding(input.Encoding)
	}

	for _, event := range input.Events {
		eventType, direction := parseEventString(event)
		sub := &Subscription{
			Namespace: bgpevents.Namespace,
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
func (s *Server) handleSubscribeEventsRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	var input rpc.SubscribeEventsInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "invalid subscribe params: "+err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	if s.subscriptions == nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "subscription manager not available"); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	s.registerSubscriptions(proc, &input)
	if sendErr := conn.SendResult(s.ctx, req.ID, nil); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleUnsubscribeEventsRPC handles ze-plugin-engine:unsubscribe-events from a plugin.
func (s *Server) handleUnsubscribeEventsRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	if s.subscriptions == nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "subscription manager not available"); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	s.subscriptions.ClearProcess(proc)
	if sendErr := conn.SendResult(s.ctx, req.ID, nil); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleEmitEventRPC handles ze-plugin-engine:emit-event from a plugin.
// Finds matching subscribers and delivers the event string to each.
func (s *Server) handleEmitEventRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	result, err := s.emitEvent(proc, req.Params)
	if err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}
	if sendErr := conn.SendResult(s.ctx, req.ID, result); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// emitEvent is the JSON wrapper for emit-event (RPC and Direct).
// Unmarshals params, delegates to deliverEvent, wraps result. The RPC
// payload arrives as a JSON string; deliverEvent handles the string->typed
// conversion for engine-side typed subscribers when the event has a
// registered payload type.
func (s *Server) emitEvent(emitter *process.Process, params json.RawMessage) (*rpc.EmitEventOutput, error) {
	var input rpc.EmitEventInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, &rpc.RPCCallError{Message: "invalid emit-event params: " + err.Error()}
	}
	delivered, err := s.deliverEvent(emitter, input.Namespace, input.EventType, input.Direction, input.PeerAddress, input.Event)
	if err != nil {
		return nil, err
	}
	return &rpc.EmitEventOutput{Delivered: delivered}, nil
}

// deliverEvent is the core emit-event logic shared by JSON and typed paths.
// Payload semantics:
//   - A nil payload is valid (signal events, registered via
//     events.RegisterSignal).
//   - An `any`-typed payload from engine code is passed through to engine
//     subscribers directly and marshaled to JSON lazily only when at least
//     one plugin-process subscriber exists.
//   - A `string` payload (emitted via plugin RPC) is the JSON form;
//     engine-side typed subscribers receive the unmarshaled Go value if the
//     event has a registered payload type, otherwise the raw string.
//
// The emitting process is excluded from plugin-process delivery to prevent
// self-delivery loops.
func (s *Server) deliverEvent(emitter *process.Process, namespace, eventType, direction, peerAddress string, payload any) (int, error) {
	if namespace == "" || eventType == "" {
		return 0, &rpc.RPCCallError{Message: "emit-event requires namespace and event-type"}
	}

	// Validate event type exists in the namespace (uses canonical registry).
	if !events.IsValidEvent(namespace, eventType) {
		return 0, &rpc.RPCCallError{Message: "unknown event: " + namespace + "/" + eventType}
	}

	// Compute the engine payload lazily. If the raw payload is a string,
	// the event has a registered typed payload, AND at least one engine
	// subscriber is listening, unmarshal once so typed engine subscribers
	// receive a native Go value. The hasSubscribers gate avoids decoding
	// for events that nobody on the engine side has registered for.
	//
	// The gate is best-effort: a subscriber that registers between this
	// check and the deferred dispatchEngineEvent below would receive the
	// undecoded raw string, and its typed-handle wrapper would log a
	// type-mismatch drop. Eliminating that race would require decoding
	// unconditionally whenever PayloadType != nil, losing most of the
	// lazy-decode benefit on events emitted only to external plugins.
	enginePayload := payload
	if raw, ok := payload.(string); ok && s.engineSubscribers != nil &&
		s.engineSubscribers.hasSubscribers(namespace, eventType) {
		if decoded, decodedOK := tryDecodeTypedPayload(namespace, eventType, raw); decodedOK {
			enginePayload = decoded
		}
	}

	// Engine-side subscribers fire regardless of whether the plugin
	// SubscriptionManager is initialized. They are a parallel registry.
	// Deferred so engine handlers run AFTER plugin process delivery, and so
	// they fire even if a plugin subscriber panics.
	defer s.dispatchEngineEvent(namespace, eventType, enginePayload)

	if s.subscriptions == nil {
		return 0, nil
	}

	procs := s.subscriptions.GetMatching(namespace, eventType, direction, peerAddress, "")
	if len(procs) == 0 {
		return 0, nil
	}

	// Lazy JSON: marshal once only when at least one external subscriber
	// exists. Producers that already have JSON (plugin RPC path, or
	// json.RawMessage) skip re-marshal.
	eventJSON, err := payloadToJSON(namespace, eventType, payload)
	if err != nil {
		return 0, &rpc.RPCCallError{Message: "marshal event payload: " + err.Error()}
	}

	delivered := 0
	for _, p := range procs {
		// Skip self-delivery to prevent loops.
		if p == emitter {
			continue
		}
		if p.Deliver(process.EventDelivery{Output: eventJSON}) {
			delivered++
		}
	}

	return delivered, nil
}

// payloadToJSON converts a bus payload into the JSON string delivered to
// plugin-process subscribers. Nil maps to "null" (signal events); an
// already-marshaled string or json.RawMessage passes through without a
// re-marshal; any other value is marshaled once.
//
// When payload is nil but the event has a registered non-signal payload
// type, this is a publisher bug (engine code emitted nil for a typed
// event); log a warn so external plugin processes do not silently
// receive "null" JSON the consumer cannot make sense of.
func payloadToJSON(namespace, eventType string, payload any) (string, error) {
	if payload == nil {
		if typ, isSignal := events.PayloadInfo(namespace, eventType); typ != nil && !isSignal {
			logger().Warn("eventbus: typed event emitted with nil payload, external subs will receive \"null\"",
				"namespace", namespace, "event-type", eventType, "want", typ.String())
		}
		return "null", nil
	}
	if s, ok := payload.(string); ok {
		return s, nil
	}
	if raw, ok := payload.(json.RawMessage); ok {
		return string(raw), nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// tryDecodeTypedPayload inspects the event registry and unmarshals raw JSON
// into the registered Go type when one exists. Returns (decoded, true) on
// success, (nil, false) otherwise (unknown event, signal event, empty
// payload, or unmarshal failure). Unmarshal failures and empty-payload
// arrivals on registered typed events log a warn so silent drops do not
// mask publisher / consumer drift; the caller forwards the raw string so
// the typed-handle wrapper can also log its drop and the engine
// subscriber still has a chance to handle the string if it registered for
// strings.
func tryDecodeTypedPayload(namespace, eventType, raw string) (any, bool) {
	typ, isSignal := events.PayloadInfo(namespace, eventType)
	if typ == nil {
		return nil, false
	}
	if isSignal {
		return nil, false
	}
	if raw == "" {
		logger().Warn("eventbus: typed event arrived with empty payload, dropping decode",
			"namespace", namespace, "event-type", eventType, "want", typ.String())
		return nil, false
	}
	// reflect.New(T) yields *T. For payloads declared as *S, typ is *S, so
	// reflect.New(typ) gives **S and Unmarshal populates a fresh *S inside.
	// Calling Elem() once returns the *S (or S for value-typed payloads)
	// that engine subscribers expect.
	ptr := reflect.New(typ)
	if err := json.Unmarshal([]byte(raw), ptr.Interface()); err != nil {
		logger().Warn("eventbus: typed event JSON unmarshal failed, dropping decode",
			"namespace", namespace, "event-type", eventType,
			"want", typ.String(), "error", err)
		return nil, false
	}
	return ptr.Elem().Interface(), true
}

// handleCodecRPC is a shared helper for plugin->engine codec RPCs (decode-nlri, encode-nlri).
// The codec callback unmarshals params and calls the registry; it returns the result to send
// or an error to relay back to the plugin.
func (s *Server) handleCodecRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request,
	codec func(json.RawMessage) (any, error),
) {
	result, err := codec(req.Params)
	if err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	if sendErr := conn.SendResult(s.ctx, req.ID, result); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// dispatchPluginRPCDirect handles a plugin→engine RPC without socket I/O.
// Used by DirectBridge for internal plugins. Returns the marshaled result JSON
// directly (not wrapped in a {"result":...} envelope). Errors are returned as
// *rpc.RPCCallError, matching the SDK's CallRPC protocol.
func (s *Server) dispatchPluginRPCDirect(proc *process.Process, method string, params json.RawMessage) (json.RawMessage, error) {
	// Known plugin→engine RPCs
	switch method {
	case "ze-plugin-engine:update-route":
		return s.handleUpdateRouteDirect(proc, params)
	case "ze-plugin-engine:dispatch-command":
		return s.handleDispatchCommandDirect(proc, params)
	case "ze-plugin-engine:subscribe-events":
		return s.handleSubscribeEventsDirect(proc, params)
	case "ze-plugin-engine:unsubscribe-events":
		return s.handleUnsubscribeEventsDirect(proc)
	case "ze-plugin-engine:emit-event":
		return s.handleEmitEventDirect(proc, params)
	}

	// Try registered RPC handlers (codec RPCs, etc.)
	if codec, ok := s.getRPCHandlers()[method]; ok {
		return handleCodecRPCDirect(codec, params)
	}

	// Unknown methods get an explicit error per ze's fail-on-unknown rule
	return nil, &rpc.RPCCallError{Message: "unknown method: " + method}
}

// handleUpdateRouteDirect handles update-route without socket I/O.
// Returns marshaled result JSON on success, or *rpc.RPCCallError on failure.
func (s *Server) handleUpdateRouteDirect(proc *process.Process, params json.RawMessage) (json.RawMessage, error) {
	var input rpc.UpdateRouteInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, &rpc.RPCCallError{Message: "invalid update-route params: " + err.Error()}
	}

	cmdCtx := &CommandContext{
		Server:  s,
		Process: proc,
		Peer:    input.PeerSelector,
		Meta:    input.Meta,
	}
	if cmdCtx.Peer == "" {
		cmdCtx.Peer = "*"
	}

	var dispatchCmd string
	if s.dispatcher.HasCommandPrefix(input.Command) {
		dispatchCmd = input.Command
	} else {
		dispatchCmd = "peer " + cmdCtx.Peer + " " + input.Command
	}

	resp, err := s.dispatcher.Dispatch(cmdCtx, dispatchCmd)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			return directResultResponse(&rpc.UpdateRouteOutput{})
		}
		return nil, &rpc.RPCCallError{Message: err.Error()}
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
// Unmarshals params, delegates to dispatchCommand, wraps result as JSON.
func (s *Server) handleDispatchCommandDirect(proc *process.Process, params json.RawMessage) (json.RawMessage, error) {
	var input rpc.DispatchCommandInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, &rpc.RPCCallError{Message: "invalid dispatch-command params: " + err.Error()}
	}
	status, data, err := s.dispatchCommand(proc, input.Command)
	if err != nil {
		return nil, &rpc.RPCCallError{Message: err.Error()}
	}
	return directResultResponse(&rpc.DispatchCommandOutput{Status: status, Data: data})
}

// dispatchCommand is the core dispatch-command logic shared by JSON and typed paths.
// Creates command context, dispatches through the command registry, and returns
// the status/data result. Logs failures with shutdown awareness.
func (s *Server) dispatchCommand(proc *process.Process, command string) (status, data string, err error) {
	cmdCtx := &CommandContext{
		Server:   s,
		Process:  proc,
		Username: "plugin:" + proc.Name(),
	}

	resp, dispatchErr := s.dispatcher.Dispatch(cmdCtx, command)
	if dispatchErr != nil {
		if errors.Is(dispatchErr, ErrSilent) {
			return plugin.StatusDone, "", nil
		}
		if s.ctx.Err() != nil {
			logger().Debug("dispatch-command failed (shutting down)", "plugin", proc.Name(), "command", command, "error", dispatchErr)
		} else {
			logger().Error("dispatch-command failed", "plugin", proc.Name(), "command", command, "error", dispatchErr)
		}
		return "", "", dispatchErr
	}

	out := responseToDispatchOutput(resp)
	return out.Status, out.Data, nil
}

// handleSubscribeEventsDirect handles subscribe-events without socket I/O.
// Returns marshaled result JSON on success, or *rpc.RPCCallError on failure.
func (s *Server) handleSubscribeEventsDirect(proc *process.Process, params json.RawMessage) (json.RawMessage, error) {
	var input rpc.SubscribeEventsInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, &rpc.RPCCallError{Message: "invalid subscribe params: " + err.Error()}
	}

	if s.subscriptions == nil {
		return nil, &rpc.RPCCallError{Message: "subscription manager not available"}
	}

	s.registerSubscriptions(proc, &input)
	return directResultResponse(nil)
}

// handleUnsubscribeEventsDirect handles unsubscribe-events without socket I/O.
// Returns marshaled result JSON on success, or *rpc.RPCCallError on failure.
func (s *Server) handleUnsubscribeEventsDirect(proc *process.Process) (json.RawMessage, error) {
	if s.subscriptions == nil {
		return nil, &rpc.RPCCallError{Message: "subscription manager not available"}
	}

	s.subscriptions.ClearProcess(proc)
	return directResultResponse(nil)
}

// handleEmitEventDirect handles emit-event without socket I/O.
func (s *Server) handleEmitEventDirect(proc *process.Process, params json.RawMessage) (json.RawMessage, error) {
	result, err := s.emitEvent(proc, params)
	if err != nil {
		return nil, err
	}
	return directResultResponse(result)
}

// handleCodecRPCDirect handles codec RPCs without socket I/O.
// Returns marshaled result JSON on success, or *rpc.RPCCallError on failure.
func handleCodecRPCDirect(codec func(json.RawMessage) (any, error), params json.RawMessage) (json.RawMessage, error) {
	result, err := codec(params)
	if err != nil {
		return nil, &rpc.RPCCallError{Message: err.Error()}
	}
	return directResultResponse(result)
}

// directResultResponse marshals data to JSON. Returns nil for nil data.
func directResultResponse(data any) (json.RawMessage, error) {
	if data == nil {
		return nil, nil
	}
	result, err := json.Marshal(data)
	if err != nil {
		return nil, &rpc.RPCCallError{Message: "marshal result: " + err.Error()}
	}
	return result, nil
}

// wireBridgeDispatch sets up the DirectBridge's DispatchRPC handler for an internal
// plugin's process. Called after the 5-stage startup completes for internal plugins.
func (s *Server) wireBridgeDispatch(proc *process.Process) {
	if proc.Bridge() == nil {
		return
	}
	proc.Bridge().SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		return s.dispatchPluginRPCDirect(proc, method, params)
	})

	// Typed fast paths: skip JSON marshal/unmarshal, delegate to shared core methods.
	proc.Bridge().SetEmitEvent(func(namespace, eventType, direction, peerAddress, event string) (int, error) {
		return s.deliverEvent(proc, namespace, eventType, direction, peerAddress, event)
	})
	proc.Bridge().SetDispatchCommand(func(command string) (status, data string, err error) {
		return s.dispatchCommand(proc, command)
	})
}

// cleanupProcess handles cleanup when a process exits.
func (s *Server) cleanupProcess(proc *process.Process) {
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
