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

	"github.com/ze-software/ze/internal/component/aaa"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// dispatchPluginRPC handles a single plugin->engine RPC request over the socket
// (JSON) transport. It resolves the method through the unified engine-op registry
// (built-in ops), then the codec-RPC registry (decode-nlri, ...), and otherwise
// returns an explicit "unknown method" error per ze's fail-on-unknown rule. The
// serve wrapper (serveEngineOpJSON) owns the shared unmarshal/dispatch/SendResult
// plumbing so every built-in op is exactly one registry entry.
func (s *Server) dispatchPluginRPC(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) {
	if op := lookupEngineOp(req.Method); op != nil {
		s.serveEngineOpJSON(proc, conn, req, op)
		return
	}

	// Try registered RPC handlers (codec RPCs, etc.)
	if codec, ok := s.getRPCHandlers()[req.Method]; ok {
		s.handleCodecRPC(proc, conn, req, codec)
		return
	}

	var tb textbuf.Buffer
	if err := conn.SendError(s.replyContext(), req.ID, tb.Str("unknown method: ").Str(req.Method).String()); err != nil {
		logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", err)
	}
}

// responseToDispatchOutput converts a dispatcher Response to DispatchCommandOutput.
func responseToDispatchOutput(resp *plugin.Response) *rpc.DispatchCommandOutput {
	output := &rpc.DispatchCommandOutput{}
	if resp == nil {
		output.Status = plugin.StatusDone
		return output
	}
	output.Status = resp.Status
	if resp.Error != "" {
		output.Error = resp.Error
		return output
	}
	if resp.Data != nil {
		encoded, err := json.Marshal(resp.Data)
		if err != nil {
			output.Status = plugin.StatusError
			var tb textbuf.Buffer
			output.Error = tb.Str("marshal response data: ").Err(err).String()
		} else {
			output.Data = encoded
		}
	}
	return output
}

// parseEventString splits an event string like "update direction sent" into
// (eventType, direction). If no "direction" keyword is present, returns DirBoth.
// This mirrors the text protocol's ParseSubscription logic for RPC event strings.
func parseEventString(event string) (events.EventTypeID, events.Direction) {
	parts := strings.Fields(event)
	if len(parts) >= 3 && parts[1] == "direction" {
		return events.LookupEventTypeID(parts[0]), events.ParseDirection(parts[2])
	}
	return events.LookupEventTypeID(event), events.DirBoth
}

// registerSubscriptions registers event subscriptions for a process.
// Parses event strings (e.g. "update direction sent") into EventType + Direction.
func (s *Server) registerSubscriptions(proc *process.Process, input *rpc.SubscribeEventsInput) {
	// Resolve the namespace FIRST so a rejected block (unknown namespace) has
	// no side effects. Format/Encoding/Envelope are per-process state (last
	// writer wins); a skipped subscribe block must not silently reconfigure
	// delivery for subscriptions already registered on this process.
	nsID, namespace, ok := s.resolveSubscriptionNamespace(proc, input.Namespace)
	if !ok {
		return
	}

	if input.Format != "" {
		proc.SetFormat(input.Format)
	}
	if input.Encoding != "" {
		proc.SetEncoding(input.Encoding)
	}
	if input.Envelope {
		proc.SetEnvelope(true)
	}

	var peerFilter *PeerFilter
	if len(input.Peers) > 0 {
		peerFilter = &PeerFilter{Selector: input.Peers[0]}
	}

	for _, event := range input.Events {
		// Gap C: a "*" event expands at REGISTRATION time into one concrete
		// subscription per registered event type of the namespace (the
		// event_monitor precedent), instead of adding a wildcard branch to
		// Subscription.Matches on the per-event hot path. Without this "*"
		// resolves to EventTypeUnknown (0) and can never match an event whose
		// ID starts at 1.
		if event == "*" {
			for _, et := range events.AllEventTypes()[namespace] {
				s.subscriptions.Add(proc, &Subscription{
					Namespace:  nsID,
					EventType:  events.LookupEventTypeID(et),
					Direction:  events.DirBoth,
					PeerFilter: peerFilter,
				})
			}
			continue
		}
		eventType, direction := parseEventString(event)
		s.subscriptions.Add(proc, &Subscription{
			Namespace:  nsID,
			EventType:  eventType,
			Direction:  direction,
			PeerFilter: peerFilter,
		})
	}

	// Both halves of every edge this process sits on are now known, which is
	// the one moment they can be compared (delivery_reconcile.go). Config load
	// precedes a plugin's declaration, so this is the later of the two
	// moments for a process that starts after the index is published, and the
	// earlier one for a process that declares before it.
	s.reconcileDelivery(proc)
}

// resolveSubscriptionNamespace resolves the namespace for a subscribe block.
// An explicit namespace (Gap A) wins and is validated against the registry:
// an unknown one is logged and the whole block is skipped (ok=false) rather
// than registering silently-dead subscriptions under NamespaceUnknown. An
// empty namespace falls back to the default registered by the owning protocol
// component ("bgp" today) -- the exact pre-Gap-A behavior; a missing default
// is logged but still returns ok=true to preserve that legacy warn-and-continue.
func (s *Server) resolveSubscriptionNamespace(proc *process.Process, requested string) (events.NamespaceID, string, bool) {
	if requested != "" {
		if !events.IsValidNamespace(requested) {
			logger().Warn("rpc event subscription: unknown namespace, skipping subscriptions",
				"plugin", proc.Name(), "namespace", requested, "valid", events.ValidNamespaceNames())
			return events.NamespaceUnknown, "", false
		}
		return events.LookupNamespaceID(requested), requested, true
	}

	namespace := plugin.DefaultEventNamespace()
	if namespace == "" {
		logger().Warn("rpc event subscription: no default event namespace registered, subscriptions will not match (call plugin.RegisterDefaultEventNamespace from the protocol component's register.go)",
			"plugin", proc.Name())
	}
	return events.LookupNamespaceID(namespace), namespace, true
}

// emitEvent is the JSON wrapper for emit-event (RPC and Direct).
// Unmarshals params, delegates to deliverEvent, wraps result. The RPC
// payload arrives as a JSON string; deliverEvent handles the string->typed
// conversion for engine-side typed subscribers when the event has a
// registered payload type.
func (s *Server) emitEvent(emitter *process.Process, params json.RawMessage) (*rpc.EmitEventOutput, error) {
	var input rpc.EmitEventInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid emit-event params: ").Err(err).String()}
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
		var tb textbuf.Buffer
		return 0, &rpc.RPCCallError{Message: tb.Str("unknown event: ").Str(namespace).Byte('/').Str(eventType).String()}
	}

	if s.eventRing != nil {
		s.eventRing.Append(namespace, eventType)
	}

	// Convert strings to typed IDs once; all internal matching uses integers.
	nsID := events.LookupNamespaceID(namespace)
	etID := events.LookupEventTypeID(eventType)
	dirID := events.ParseDirection(direction)

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
		s.engineSubscribers.hasSubscribers(nsID, etID) {
		if decoded, decodedOK := tryDecodeTypedPayload(namespace, eventType, raw); decodedOK {
			enginePayload = decoded
		}
	}

	// Engine-side subscribers fire regardless of whether the plugin
	// SubscriptionManager is initialized. They are a parallel registry.
	// Deferred so engine handlers run AFTER plugin process delivery, and so
	// they fire even if a plugin subscriber panics.
	defer s.dispatchEngineEvent(nsID, etID, enginePayload)

	if s.subscriptions == nil {
		return 0, nil
	}

	// A peer-scoped event goes through the delivery graph, exactly as the seven
	// reactor entry points do (bgp/server/events.go). An emitted event is
	// peer-scoped when it carries a peer ADDRESS, which is the key the graph is
	// built on; the peer NAME is empty on this path and is not needed, because
	// PeerScopedProcs matches on the address.
	//
	// This path used to call getMatching directly, so `attach process <name>
	// { receive [ update-rpki ] }` decided nothing for the two bgp events that
	// travel it, bgp/update-rpki from bgp-rpki-decorator and bgp/rpki from
	// bgp-rpki. It was left unfiltered because the peer name is empty, which is
	// a true statement about a field the lookup does not read.
	//
	// An event with NO peer address is not peer-scoped, no attach block can
	// describe it, and it keeps flowing to everything subscribed. That branch is
	// written out rather than reached by accident, because a graph miss and "this
	// event has no peer" must never be the same answer (ai/rules/evidence.md).
	var procs []*process.Process
	if peerAddress == "" {
		procs = s.subscriptions.getMatching(nsID, etID, dirID, "", "")
	} else {
		procs = s.PeerScopedProcs(nsID, etID, dirID, peerAddress, "")
	}
	if len(procs) == 0 {
		return 0, nil
	}

	// Lazy JSON: marshal once only when at least one external subscriber
	// exists. Producers that already have JSON (plugin RPC path, or
	// json.RawMessage) skip re-marshal.
	eventJSON, err := payloadToJSON(namespace, eventType, payload)
	if err != nil {
		var tb textbuf.Buffer
		return 0, &rpc.RPCCallError{Message: tb.Str("marshal event payload: ").Err(err).String()}
	}

	// Gap B: subscribers that opted into enveloped delivery receive the bare
	// payload wrapped with its (namespace, event) identity. Render the envelope
	// AT MOST ONCE, and only if some matching proc actually opted in, so the
	// default (no opt-in) path keeps marshaling exactly once -- byte-identical
	// to before. Subscribers that did not opt in still get eventJSON verbatim.
	var envelopeJSON string
	var envelopeBuilt bool

	delivered := 0
	for _, p := range procs {
		// Skip self-delivery to prevent loops.
		if p == emitter {
			continue
		}
		output := eventJSON
		if p.Envelope() {
			if !envelopeBuilt {
				envelopeJSON, err = buildEventEnvelope(namespace, eventType, eventJSON)
				if err != nil {
					var tb textbuf.Buffer
					return 0, &rpc.RPCCallError{Message: tb.Str("marshal event envelope: ").Err(err).String()}
				}
				envelopeBuilt = true
			}
			output = envelopeJSON
		}
		if p.Deliver(process.EventDelivery{Output: output}) {
			delivered++
		}
	}

	return delivered, nil
}

// buildEventEnvelope wraps a bare event payload JSON string in an EventEnvelope
// carrying the (namespace, event) identity, marshaled to the JSON string that
// rides inside the delivered event (transparent to the deliver-event and
// deliver-batch string paths). bareJSON is a valid JSON document ("null", a
// string-passthrough, or a marshaled value), so embedding it as json.RawMessage
// re-marshals without a second decode.
func buildEventEnvelope(namespace, eventType, bareJSON string) (string, error) {
	b, err := json.Marshal(rpc.EventEnvelope{
		Namespace: namespace,
		Event:     eventType,
		Payload:   json.RawMessage(bareJSON),
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
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
	reply := s.replyContext()
	if err != nil {
		if sendErr := conn.SendError(reply, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	if sendErr := conn.SendResult(reply, req.ID, result); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// dispatchPluginRPCDirect handles a plugin→engine RPC without socket I/O.
// Used by DirectBridge for internal plugins. Returns the marshaled result JSON
// directly (not wrapped in a {"result":...} envelope). Errors are returned as
// *rpc.RPCCallError, matching the SDK's CallRPC protocol.
func (s *Server) dispatchPluginRPCDirect(proc *process.Process, method string, params json.RawMessage) (json.RawMessage, error) {
	if op := lookupEngineOp(method); op != nil {
		return s.serveEngineOpDirect(proc, op, params)
	}

	// Try registered RPC handlers (codec RPCs, etc.)
	if codec, ok := s.getRPCHandlers()[method]; ok {
		return handleCodecRPCDirect(codec, params)
	}

	// Unknown methods get an explicit error per ze's fail-on-unknown rule
	var tb textbuf.Buffer
	return nil, &rpc.RPCCallError{Message: tb.Str("unknown method: ").Str(method).String()}
}

// handleUpdateRouteSelDirect handles update-route with a typed *selector.Selector.
// Reuses the existing Dispatch path by stringifying the selector for the peer field.
func (s *Server) handleUpdateRouteSelDirect(proc *process.Process, sel *selector.Selector, command string, meta map[string]any) (uint32, uint32, error) {
	peer := sel.String()
	cmdCtx := &CommandContext{
		Server:         s,
		Process:        proc,
		RequestContext: s.Context(),
		Peer:           peer,
		Meta:           meta,
		// Reserved trusted internal identity (spec-fixit-authz-admin-fallthrough
		// O-4): the typed update-route-sel path is an internal route push too, so it
		// must authorize like opUpdateRoute rather than fail closed on an empty
		// username when authorization is configured.
		Username: internalPluginIdentity(proc.Name()),
		Sender:   plugin.ProcessSender(proc.Name()),
	}

	var tb textbuf.Buffer
	dispatchCmd := tb.Str("peer ").Str(peer).Byte(' ').Str(command).String()

	resp, err := s.dispatcher.Dispatch(cmdCtx, dispatchCmd)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	output := extractUpdateRouteOutput(resp)
	return output.Announced, output.Withdrawn, nil
}

// extractUpdateRouteOutput reads announced/withdrawn counts from a Dispatch response.
// DispatchNLRIGroups returns *plugin.RouteResult as resp.Data.
func extractUpdateRouteOutput(resp *plugin.Response) *rpc.UpdateRouteOutput {
	output := &rpc.UpdateRouteOutput{}
	if resp == nil || resp.Data == nil {
		return output
	}
	if r, ok := resp.Data.(*plugin.RouteResult); ok {
		output.Announced = r.Announced
		output.Withdrawn = r.Withdrawn
	}
	return output
}

// internalRPCIdentity is the reserved username wrapHandler injects for a trusted
// in-process RPC call that carries no plugin identity (spec-fixit-authz-admin-
// fallthrough O-4). It is un-typeable, so no authenticated user can present it;
// authz.Store.Authorize grants it as a trusted internal caller. Without an
// injected identity an empty username would reach the now-fail-closed Authorize
// and every RPC method would be denied.
const internalRPCIdentity = aaa.ReservedInternalPrefix + "rpc"

// internalPluginIdentity builds the reserved username the engine injects when a
// plugin dispatches a command in-process (spec O-4/F-6, regularizing the former
// bare `plugin:<name>` identity that fell through to the admin default). The
// plugin name is kept after the reserved prefix for accounting and audit; it
// never affects the authorization decision. The prefix is un-typeable, so no
// authenticated identity can spoof a trusted internal caller.
func internalPluginIdentity(pluginName string) string {
	var tb textbuf.Buffer
	return tb.Str(aaa.ReservedInternalPrefix).Str("plugin:").Str(pluginName).String()
}

// dispatchCommandArgs is the core dispatch-command-args logic shared by JSON and typed paths.
// It routes an exact registered plugin command with pre-tokenized args, avoiding
// command-string tokenization for runtime data while preserving dispatch-command output.
func (s *Server) dispatchCommandArgs(proc *process.Process, command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
	if peer == "" {
		peer = "*"
	}
	cmdCtx := &CommandContext{
		Server:         s,
		Process:        proc,
		RequestContext: s.Context(),
		Peer:           peer,
		Username:       internalPluginIdentity(proc.Name()),
		Sender:         plugin.ProcessSender(proc.Name()),
	}

	if s.dispatcher != nil && !s.dispatcher.isAuthorizedCommandArgs(cmdCtx, command, args, peer, false) {
		return &rpc.DispatchCommandOutput{
			Status: plugin.StatusError,
			Error:  unauthorizedError(aaa.CanonicalCommand(command, args, peer)),
		}, ErrUnauthorized
	}

	resp, dispatchErr := s.dispatcher.ForwardToPlugin(cmdCtx, command, args, peer)
	if dispatchErr != nil {
		if errors.Is(dispatchErr, ErrSilent) {
			return &rpc.DispatchCommandOutput{Status: plugin.StatusDone}, nil
		}
		authInput := aaa.CanonicalCommand(command, args, peer)
		if s.ctx.Err() != nil {
			logger().Debug("dispatch-command-args failed (shutting down)", "plugin", proc.Name(), "command", authInput, "error", dispatchErr)
		} else {
			logger().Error("dispatch-command-args failed", "plugin", proc.Name(), "command", authInput, "error", dispatchErr)
		}
		return nil, dispatchErr
	}

	return responseToDispatchOutput(resp), nil
}

// dispatchCommand is the core dispatch-command logic shared by JSON and typed paths.
// Creates command context, dispatches through the command registry, and returns
// the full DispatchCommandOutput. Logs failures with shutdown awareness.
func (s *Server) dispatchCommand(proc *process.Process, command string) (*rpc.DispatchCommandOutput, error) {
	cmdCtx := &CommandContext{
		Server:         s,
		Process:        proc,
		RequestContext: s.Context(),
		Username:       internalPluginIdentity(proc.Name()),
		Sender:         plugin.ProcessSender(proc.Name()),
	}

	resp, dispatchErr := s.dispatcher.Dispatch(cmdCtx, command)
	if dispatchErr != nil {
		if errors.Is(dispatchErr, ErrSilent) {
			return &rpc.DispatchCommandOutput{Status: plugin.StatusDone}, nil
		}
		if s.ctx.Err() != nil {
			logger().Debug("dispatch-command failed (shutting down)", "plugin", proc.Name(), "command", command, "error", dispatchErr)
		} else {
			logger().Error("dispatch-command failed", "plugin", proc.Name(), "command", command, "error", dispatchErr)
		}
		return nil, dispatchErr
	}

	return responseToDispatchOutput(resp), nil
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
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("marshal result: ").Err(err).String()}
	}
	return result, nil
}

// wireBridgeDispatch sets up the DirectBridge for an internal plugin's process
// after the 5-stage startup completes. The generic DispatchRPC handler covers
// every registered op via dispatchPluginRPCDirect (JSON-shaped fallback); the
// typed fast-path slots that skip JSON entirely are installed by iterating the
// engine-op registry and invoking each entry's typedWire descriptor, so the
// slot set derives from the same table as the JSON and Direct paths -- no
// hand-written Set* list to drift (AC-3).
func (s *Server) wireBridgeDispatch(proc *process.Process) {
	b := proc.Bridge()
	if b == nil {
		return
	}
	b.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		return s.dispatchPluginRPCDirect(proc, method, params)
	})

	// Typed fast paths: skip JSON marshal/unmarshal, delegate to shared core
	// methods. Only ops that declare a typedWire descriptor get a native slot;
	// the rest (subscribe/unsubscribe, route-install/route-remove) intentionally
	// have none and fall through to the generic DispatchRPC handler above.
	for i := range engineOps {
		if engineOps[i].typedWire != nil {
			engineOps[i].typedWire(s, proc, b)
		}
	}
}

// cleanupProcess handles cleanup when a process exits.
func (s *Server) cleanupProcess(proc *process.Process) {
	// Unregister all commands from this process
	s.dispatcher.Registry().UnregisterAll(proc)

	// Cancel all pending requests
	s.dispatcher.Pending().CancelAll(proc)

	// Clear all subscriptions for this process
	if s.subscriptions != nil {
		s.subscriptions.clearProcess(proc)
	}

	// Remove cache consumer tracking for this plugin.
	// UnregisterConsumer decrements pending counts for unacked entries
	// so they can be evicted instead of leaking.
	if proc.IsCacheConsumer() && s.reactor != nil {
		s.reactor.UnregisterCacheConsumer(proc.Name())
	}

	// Withdraw any routes this plugin installed into the engine Loc-RIB via the
	// route-install RPC (AC-8): a forked OSPF/IS-IS that dies without withdrawing
	// must not leave stale routes in the kernel.
	s.withdrawPluginRoutes(proc.Name())

	runProcessCleanupHooks(proc.Name())
}
