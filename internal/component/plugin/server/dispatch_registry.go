// Design: docs/architecture/api/process-protocol.md — plugin RPC dispatch
// Related: dispatch.go — dispatchPluginRPC / dispatchPluginRPCDirect / wireBridgeDispatch
//          dispatch_cached.go, dispatch_route.go — op handlers for cached/route ops
//
// unify-rpc-dispatch: a single method registry for plugin->engine RPCs. Before
// this, every operation lived in up to three hand-maintained tables keyed by
// magic strings -- the JSON socket switch, the in-process Direct switch, and the
// wireBridgeDispatch Set* list -- plus a fourth branch table in the SDK. Nothing
// forced them to agree and coverage drifted (subscribe/unsubscribe had no typed
// slot; route-install/remove had no Direct arm; inject-wire-route/batch-validate
// had no JSON path). Here each operation is exactly ONE engineOp entry from which
// the JSON path, the Direct path, and the typed DirectBridge slot all derive, so
// adding an operation touches one place and the paths cannot drift.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// engineOp is one plugin->engine RPC operation. A single entry drives all three
// transports:
//   - the JSON socket path (dispatchPluginRPC -> serveEngineOpJSON),
//   - the in-process Direct path (dispatchPluginRPCDirect -> serveEngineOpDirect),
//   - and, when typedWire is non-nil, the typed DirectBridge fast-path slot
//     (wireBridgeDispatch iterates entries that declare one).
type engineOp struct {
	// method is the ze-plugin-engine:* wire method string (rpc.Method* constant).
	method string

	// handle unmarshals params, runs the shared CORE handler, and returns the
	// value to marshal (or nil). proc is PASSED, never captured, so the entry is
	// a single package-level value with zero per-request closure allocation (R-3).
	// On failure it returns an error: an *rpc.RPCCallError carries the exact
	// detail to relay, a plain error is relayed verbatim. Both serve wrappers
	// derive the sent message from the same error, so the JSON and Direct paths
	// cannot diverge (AC-2).
	handle func(s *Server, proc *process.Process, params json.RawMessage) (any, error)

	// typedWire, when non-nil, installs this op's typed DirectBridge slot on the
	// process bridge -- the hot-path escape hatch that skips JSON marshal and
	// passes native Go values. nil means "no typed slot" (subscribe/unsubscribe,
	// route-install/route-remove), exactly matching the pre-unification coverage.
	typedWire func(s *Server, proc *process.Process, b *rpc.DirectBridge)
}

// engineOps is the single source of truth for plugin->engine runtime RPCs. The
// JSON path, the Direct path, and the Bridge wiring all derive from this table;
// TestPluginRPCRegistryCoversAllPaths guards the method set and the typed set.
var engineOps = []engineOp{
	{
		method: rpc.MethodUpdateRoute,
		handle: (*Server).opUpdateRoute,
		typedWire: func(s *Server, proc *process.Process, b *rpc.DirectBridge) {
			b.SetUpdateRouteSel(func(sel *selector.Selector, command string, meta map[string]any) (uint32, uint32, error) {
				return s.handleUpdateRouteSelDirect(proc, sel, command, meta)
			})
		},
	},
	{
		method: rpc.MethodDispatchCommand,
		handle: (*Server).opDispatchCommand,
		// Two typed slots ride this one entry, because dispatch-command has one
		// wire method and two readings of its answer: the built value a caller
		// holds whole, and the records a caller walks. The registry is keyed by
		// method, so a second entry would be a duplicate method rather than a
		// second operation.
		typedWire: func(s *Server, proc *process.Process, b *rpc.DirectBridge) {
			b.SetDispatchCommand(func(command string) (*rpc.DispatchCommandOutput, error) {
				return s.dispatchCommand(proc, command)
			})
			b.SetDispatchCommandAnswer(func(command string) (*rpc.Answer, error) {
				return s.dispatchCommandAnswer(proc, command)
			})
		},
	},
	{
		method: rpc.MethodDispatchCommandArgs,
		handle: (*Server).opDispatchCommandArgs,
		typedWire: func(s *Server, proc *process.Process, b *rpc.DirectBridge) {
			b.SetDispatchCommandArgs(func(command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
				return s.dispatchCommandArgs(proc, command, args, peer)
			})
		},
	},
	{
		method: rpc.MethodSubscribeEvents,
		handle: (*Server).opSubscribeEvents,
		// No typed slot: low-frequency, string-only. Same as before unification.
	},
	{
		method: rpc.MethodUnsubscribeEvents,
		handle: (*Server).opUnsubscribeEvents,
	},
	{
		method: rpc.MethodEmitEvent,
		handle: (*Server).opEmitEvent,
		typedWire: func(s *Server, proc *process.Process, b *rpc.DirectBridge) {
			b.SetEmitEvent(func(namespace, eventType, direction, peerAddress, event string) (int, error) {
				return s.deliverEvent(proc, namespace, eventType, direction, peerAddress, event)
			})
		},
	},
	{
		method: rpc.MethodForwardCached,
		handle: (*Server).opForwardCached,
		typedWire: func(s *Server, proc *process.Process, b *rpc.DirectBridge) {
			b.SetForwardCached(func(_ context.Context, ids []uint64, destinations []string) error {
				return s.forwardCached(proc, ids, destinations)
			})
		},
	},
	{
		method: rpc.MethodReleaseCached,
		handle: (*Server).opReleaseCached,
		typedWire: func(s *Server, proc *process.Process, b *rpc.DirectBridge) {
			b.SetReleaseCached(func(_ context.Context, ids []uint64) error {
				return s.releaseCached(proc, ids)
			})
		},
	},
	{
		method: rpc.MethodRelayStoredRoute,
		handle: (*Server).opRelayStoredRoute,
		// proc names the sender here for the same reason opRelayStoredRoute does:
		// the Direct bridge is the in-process rail of the same RPC, so leaving it
		// unnamed would gate the forked plugin and let an in-process one through.
		typedWire: func(s *Server, proc *process.Process, b *rpc.DirectBridge) {
			b.SetRelayStoredRoute(func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
				return s.relayStoredRoute(procSender(proc), destination, routes)
			})
		},
	},
	{
		method: rpc.MethodRouteInstall,
		handle: (*Server).opRouteInstall,
		// No typed slot: forked-plugin batch path, socket/Direct only.
	},
	{
		method: rpc.MethodRouteRemove,
		handle: (*Server).opRouteRemove,
	},
	{
		method: rpc.MethodInjectWireRoute,
		handle: (*Server).opInjectWireRoute,
		typedWire: func(_ *Server, _ *process.Process, b *rpc.DirectBridge) {
			b.SetInjectWireRoute(func(protocol, peerKey string, updateBody []byte) error {
				fn := rpc.GetRouteInjector()
				if fn == nil {
					return errors.New("inject-wire-route: no route injector registered")
				}
				return fn(protocol, peerKey, updateBody)
			})
		},
	},
	{
		method: rpc.MethodBatchValidate,
		handle: (*Server).opBatchValidate,
		typedWire: func(_ *Server, _ *process.Process, b *rpc.DirectBridge) {
			b.SetBatchValidate(func(decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
				fn := rpc.GetBatchValidator()
				if fn == nil {
					return nil, errors.New("batch-validate: no batch validator registered")
				}
				return fn(decisions)
			})
		},
	},
}

// engineOpTable indexes engineOps by method for O(1) lookup. Built once at
// package init; the pointers into the engineOps backing array are stable because
// the slice is never reallocated after init.
var engineOpTable = buildEngineOpTable()

func buildEngineOpTable() map[string]*engineOp {
	m := make(map[string]*engineOp, len(engineOps))
	for i := range engineOps {
		m[engineOps[i].method] = &engineOps[i]
	}
	return m
}

// lookupEngineOp returns the registered op for method, or nil if none. A nil
// result is the fail-closed signal the dispatch paths turn into "unknown method".
func lookupEngineOp(method string) *engineOp {
	return engineOpTable[method]
}

// transportCompleteResponse is implemented by response envelopes that carry an
// accepted action to the boundary responsible for delivering them.
type transportCompleteResponse interface {
	TransportComplete()
}

// recordAnswer is the op result of a command answer: the response itself,
// unprojected, so the transport writes it as a head, its records and a
// terminator instead of marshaling it into one JSON result.
//
// Every peer reads this shape. The two readings of one answer are exclusive --
// a generator payload is walked once, so the {status, data, error} projection
// and the record sequence cannot both run over the same response (Records,
// internal/component/plugin/types.go) -- so the wire carries one of them and
// this is it.
type recordAnswer struct {
	resp *plugin.Response
}

// write puts the answer sequence for id on w, one framed line at a time.
func (a *recordAnswer) write(w io.Writer, id uint64) error {
	return plugin.WriteAnswer(w, id, a.resp)
}

// TransportComplete runs the accepted lifecycle action the response carries,
// after the transport has delivered the answer. Repeated calls are harmless.
func (a *recordAnswer) TransportComplete() {
	a.resp.TransportComplete()
}

// serveEngineOpJSON runs an op for the JSON socket path, writes its result, then
// completes any accepted lifecycle action. Error detail is the raw message
// (rpcErrMessage), matching what serveEngineOpDirect returns so external and
// in-process callers see identical error text (AC-2).
func (s *Server) serveEngineOpJSON(proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request, op *engineOp) {
	result, err := op.handle(s, proc, req.Params)
	reply := s.replyContext()
	if err != nil {
		if sendErr := conn.SendError(reply, req.ID, rpcErrMessage(err)); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}
	if completed, ok := result.(transportCompleteResponse); ok {
		defer completed.TransportComplete()
	}
	if answer, records := result.(*recordAnswer); records {
		if writeErr := answer.write(conn.AnswerWriter(reply), req.ID); writeErr != nil {
			logger().Debug("rpc runtime: write answer failed", "plugin", proc.Name(), "error", writeErr)
		}
		return
	}
	if sendErr := conn.SendResult(reply, req.ID, result); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// serveEngineOpDirect runs an op for the in-process Direct path and marshals its
// result before completing any accepted lifecycle action. Errors are returned
// as *rpc.RPCCallError, matching the SDK's CallRPC protocol; an
// existing RPCCallError passes through unwrapped so its detail is not
// double-prefixed.
func (s *Server) serveEngineOpDirect(proc *process.Process, op *engineOp, params json.RawMessage) (json.RawMessage, error) {
	result, err := op.handle(s, proc, params)
	if err != nil {
		if callErr, ok := errors.AsType[*rpc.RPCCallError](err); ok {
			return nil, callErr
		}
		return nil, &rpc.RPCCallError{Message: err.Error()}
	}
	if answer, records := result.(*recordAnswer); records {
		// This is the JSON-shaped Direct path, so its result is one marshaled
		// value and it has no line to carry a record on. Project the answer
		// instead, which is the JSON the record sequence reassembles to (AC-10
		// of spec-streaming-answer-protocol). Without this the marshal below
		// would answer `{}`: recordAnswer exports no field.
		//
		// An in-process caller that wants the records themselves takes the
		// typed answer slot rather than this path
		// (DirectBridge.DispatchCommandAnswer, pkg/plugin/rpc/bridge.go).
		result = responseToDispatchOutput(answer.resp)
	}
	return directResultResponse(result)
}

// rpcErrMessage returns the raw error detail to send on the wire. For an
// *rpc.RPCCallError it returns the bare Message (RPCCallError.Error() would
// prepend "rpc error: ", which the plugin adds again on receipt); for any other
// error it returns Error() verbatim.
func rpcErrMessage(err error) string {
	var callErr *rpc.RPCCallError
	if errors.As(err, &callErr) && callErr.Message != "" {
		return callErr.Message
	}
	return err.Error()
}

// opUpdateRoute is the shared handler for update-route (JSON + Direct). It
// dispatches the peer-scoped command string through the standard dispatcher.
func (s *Server) opUpdateRoute(proc *process.Process, params json.RawMessage) (any, error) {
	var tb textbuf.Buffer
	var input rpc.UpdateRouteInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid update-route params: ").Err(err).String()}
	}
	peer := input.PeerSelector
	if peer == "" {
		peer = "*"
	}
	cmdCtx := &CommandContext{
		Server:         s,
		Process:        proc,
		RequestContext: s.Context(),
		Peer:           peer,
		Meta:           input.Meta,
		// Inject the reserved trusted internal identity, same as the other internal
		// dispatch paths (spec-fixit-authz-admin-fallthrough O-4). Without it this
		// route-push RPC carries an empty username and, on a box with authorization
		// configured, hits the now-fail-closed "denied: empty identity" branch --
		// silently breaking route propagation (RS/OSPF/IS-IS `update text`).
		Username: internalPluginIdentity(proc.Name()),
		Sender:   plugin.ProcessSender(proc.Name()),
	}
	// Route injection commands are always peer-scoped subcommands
	// (e.g., "update text ...", "announce route ..."). Prepend unconditionally.
	dispatchCmd := tb.Str("peer ").Str(peer).Byte(' ').Str(input.Command).String()
	resp, err := s.dispatcher.Dispatch(cmdCtx, dispatchCmd)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			return &rpc.UpdateRouteOutput{}, nil
		}
		return nil, &rpc.RPCCallError{Message: err.Error()}
	}
	return extractUpdateRouteOutput(resp), nil
}

// opDispatchCommand is the shared handler for dispatch-command (JSON + Direct).
// Its output carries the accepted lifecycle action unchanged so the selected
// transport can complete it after response delivery.
func (s *Server) opDispatchCommand(proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.DispatchCommandInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid dispatch-command params: ").Err(err).String()}
	}
	resp, err := s.dispatchCommandResponse(proc, input.Command)
	if err != nil {
		return nil, err
	}
	return &recordAnswer{resp: resp}, nil
}

// opDispatchCommandArgs is the shared handler for dispatch-command-args.
func (s *Server) opDispatchCommandArgs(proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.DispatchCommandArgsInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid dispatch-command-args params: ").Err(err).String()}
	}
	resp, err := s.dispatchCommandArgsResponse(proc, input.Command, input.Args, input.Peer)
	if err != nil {
		return nil, err
	}
	return &recordAnswer{resp: resp}, nil
}

// opSubscribeEvents is the shared handler for subscribe-events.
func (s *Server) opSubscribeEvents(proc *process.Process, params json.RawMessage) (any, error) {
	var input rpc.SubscribeEventsInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid subscribe params: ").Err(err).String()}
	}
	if s.subscriptions == nil {
		return nil, &rpc.RPCCallError{Message: "subscription manager not available"}
	}
	s.registerSubscriptions(proc, &input)
	return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
}

// opUnsubscribeEvents is the shared handler for unsubscribe-events. It ignores
// params (there are none) and clears all of the process's subscriptions.
func (s *Server) opUnsubscribeEvents(proc *process.Process, _ json.RawMessage) (any, error) {
	if s.subscriptions == nil {
		return nil, &rpc.RPCCallError{Message: "subscription manager not available"}
	}
	s.subscriptions.clearProcess(proc)
	return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
}

// opEmitEvent is the shared handler for emit-event. s.emitEvent unmarshals its
// own params and returns *rpc.RPCCallError on failure.
func (s *Server) opEmitEvent(proc *process.Process, params json.RawMessage) (any, error) {
	return s.emitEvent(proc, params)
}

// opInjectWireRoute is the JSON-codec fallback for inject-wire-route (AC-6): a
// forked/external plugin with no typed slot reaches the process-wide route
// injector over the socket instead of erroring "bridge not available".
func (s *Server) opInjectWireRoute(_ *process.Process, params json.RawMessage) (any, error) {
	var input rpc.InjectWireRouteInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid inject-wire-route params: ").Err(err).String()}
	}
	fn := rpc.GetRouteInjector()
	if fn == nil {
		return nil, &rpc.RPCCallError{Message: "inject-wire-route: no route injector registered"}
	}
	if err := fn(input.Protocol, input.PeerKey, input.UpdateBody); err != nil {
		return nil, &rpc.RPCCallError{Message: err.Error()}
	}
	return nil, nil //nolint:nilnil // no result payload; (nil,nil) is success-with-no-content
}

// opBatchValidate is the JSON-codec fallback for batch-validate (AC-6): a
// forked/external plugin reaches the process-wide batch validator over the socket
// instead of hand-rolling a stride-6 string through dispatch-command-args.
func (s *Server) opBatchValidate(_ *process.Process, params json.RawMessage) (any, error) {
	var input rpc.BatchValidateInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid batch-validate params: ").Err(err).String()}
	}
	fn := rpc.GetBatchValidator()
	if fn == nil {
		return nil, &rpc.RPCCallError{Message: "batch-validate: no batch validator registered"}
	}
	return fn(input.Decisions)
}
