// Design: docs/architecture/api/process-protocol.md -- the engine side of one plugin connection, for tests
// Related: ../../../pkg/plugin/rpc/bridge.go -- the DirectBridge the bridge transport drives
// Related: ../../../pkg/plugin/rpc/mux.go -- the MuxConn the wire transport drives

// Package pluginengine is a test engine for one real plugin. It starts the
// plugin's own entry point on a connection, answers the five startup stages,
// and then lets a test deliver events, send source-policy requests and run
// commands through the plugin's callback loop, while it records every engine
// RPC the plugin makes.
//
// Both transports a plugin can run on are driven the way the daemon drives
// them. TransportWire hands the plugin a plain pipe, so the plugin finds no
// bridge and keeps every callback on the MuxConn, as a forked plugin does.
// TransportBridge hands it a pipe wrapped with a DirectBridge, so after stage 5
// the plugin moves its callbacks to the bridge, as an internal plugin does.
// Nothing here calls a plugin handler directly: every request is admitted by
// the plugin's own SDK loop.
package pluginengine

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Callback and startup method names. They are wire contract
// (ze-plugin-callback.yang, ze-plugin-engine.yang); the SDK keeps its own
// copies unexported.
const (
	methodConfigure      = "ze-plugin-callback:configure"
	methodShareRegistry  = "ze-plugin-callback:share-registry"
	methodDeliverEvent   = "ze-plugin-callback:deliver-event"
	methodSourcePolicy   = "ze-plugin-callback:source-policy"
	methodExecuteCommand = "ze-plugin-callback:execute-command"

	methodDeclareCapabilities = "ze-plugin-engine:declare-capabilities"
	methodReady               = "ze-plugin-engine:ready"
)

// startupTimeout bounds the five startup stages; exitTimeout bounds how long
// cleanup waits for the plugin's entry point to return.
const (
	startupTimeout = 10 * time.Second
	exitTimeout    = 5 * time.Second
)

// Transport is the connection a plugin runs on. The zero value is no
// transport, and Start refuses it.
type Transport uint8

const (
	TransportUnspecified Transport = iota
	// TransportWire is the MuxConn path a forked plugin takes.
	TransportWire
	// TransportBridge is the DirectBridge path an internal plugin takes.
	TransportBridge
)

// Transports is every transport, in the order a test runs a subtest for each.
func Transports() []Transport {
	return []Transport{TransportWire, TransportBridge}
}

// String names the transport for a subtest.
func (t Transport) String() string {
	switch t {
	case TransportWire:
		return "wire"
	case TransportBridge:
		return "bridge"
	case TransportUnspecified:
		return "unspecified"
	}
	return "transport-" + strconv.Itoa(int(t))
}

// EngineRPC is one plugin-to-engine RPC, as the engine received it.
type EngineRPC struct {
	Method string
	Params json.RawMessage
}

// Engine is the engine side of one running plugin. Its methods are safe for
// concurrent use; the plugin's own concurrency is whatever its SDK loop gives.
type Engine struct {
	mux    *rpc.MuxConn
	bridge *rpc.DirectBridge // nil on TransportWire

	mu         sync.Mutex
	rpcs       []EngineRPC
	nextSource uint64
	sources    map[uint64]*sourceLease
}

// sourceLease is one fixture-issued source Session and this connection's lease.
// It contains no route payload and confers no production engine authority.
type sourceLease struct {
	peer      string
	peerOwner uint64
	acquired  bool
	retained  bool
}

// IssueSource issues one source-session identity before a received event is
// delivered. Reuse its result for that Session's UPDATEs, not a successor's.
func (e *Engine) IssueSource(peer string, peerOwner uint64) uint64 {
	if peer == "" {
		panic("BUG: pluginengine source session has no peer or stable owner")
	}
	if peerOwner == 0 {
		panic("BUG: pluginengine source session has no peer or stable owner")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sources == nil {
		e.sources = make(map[uint64]*sourceLease)
	}
	e.nextSource++
	e.sources[e.nextSource] = &sourceLease{peer: peer, peerOwner: peerOwner}
	return e.nextSource
}

// sourceLeaseAnswer validates the real action/receipt/deadline envelope.
// Acquire and Retain require an issued Session; Release settles absent authority
// without issuing a receipt or retaining completed lease history.
func (e *Engine) sourceLeaseAnswer(params json.RawMessage) (*rpc.SourceLeaseOutput, error) {
	var input rpc.SourceLeaseInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	source := e.sources[input.SourceReceipt]
	if source == nil {
		if input.Action == rpc.SourceLeaseRelease {
			return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, nil
		}
		return nil, errors.New("pluginengine: source receipt was not issued; call IssueSource before delivering its UPDATE")
	}
	if input.PeerOwner != source.peerOwner {
		if input.Action != rpc.SourceLeaseRelease {
			return nil, errors.New("pluginengine: peer owner does not match the issued source receipt")
		}
		if input.PeerOwner != 0 {
			return nil, errors.New("pluginengine: peer owner does not match the issued source receipt")
		}
	}
	switch input.Action {
	case rpc.SourceLeaseAcquire:
		source.acquired = true
	case rpc.SourceLeaseRetain:
		if !source.acquired {
			return nil, errors.New("pluginengine: source lease for " + source.peer + " was not acquired before retention")
		}
		source.retained = true
	case rpc.SourceLeaseRelease:
		source.acquired = false
		source.retained = false
	case rpc.SourceLeaseUnspecified:
		return nil, errors.New("pluginengine: source lease action was not validated")
	default:
		return nil, errors.New("pluginengine: source lease action was not validated")
	}
	return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, nil
}

// Start runs the plugin entry point run on transport, completes its startup,
// and returns the engine that drives it. The configure and share-registry
// stages carry nothing. Cleanup closes the connection and waits for run to
// return. Tests that start a plugin with process-wide state MUST NOT run in
// parallel with each other.
func Start(t testing.TB, transport Transport, run func(net.Conn) int) *Engine {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	e := &Engine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))}
	conn := pluginEnd
	switch transport {
	case TransportBridge:
		e.bridge = rpc.NewDirectBridge()
		// Registered before the plugin starts: the plugin marks the bridge
		// ready after stage 5, and from then on calls the engine through it.
		e.bridge.SetDispatchRPC(e.dispatchBridged)
		conn = rpc.NewBridgedConn(pluginEnd, e.bridge)
	case TransportWire:
		// A plain pipe: the plugin finds no bridge and stays on the MuxConn.
	case TransportUnspecified:
		t.Fatal("pluginengine: Start needs TransportWire or TransportBridge")
	default:
		t.Fatal("pluginengine: Start needs TransportWire or TransportBridge, not " + transport.String())
	}

	exited := make(chan int, 1)
	go runPlugin(run, conn, exited)
	serveCtx, stopServe := context.WithCancel(context.Background())
	t.Cleanup(func() {
		stopServe()
		if e.bridge != nil {
			e.bridge.CloseCallbacks()
		}
		e.mux.Close()     //nolint:errcheck // test cleanup: the plugin may have closed it first
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
		select {
		case <-exited:
		case <-time.After(exitTimeout):
			t.Error("pluginengine: the plugin did not return after its connection closed")
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()
	if err := e.startup(ctx); err != nil {
		t.Fatal("pluginengine: startup: " + err.Error())
	}
	if transport == TransportWire {
		go e.serveWire(serveCtx)
	}
	// The Stage 5 acknowledgement lets the SDK activate its bridge and run
	// OnStarted; it does not mean its command callbacks are installed yet.
	// Join the existing generic receipt on the actual callback loop before
	// returning the engine. The wire reader above serves initializer calls.
	var readyErr error
	if e.bridge != nil {
		_, readyErr = e.bridge.SendCallback(ctx, rpc.MethodRuntimeReady, nil)
	} else {
		_, readyErr = e.mux.CallRPC(ctx, rpc.MethodRuntimeReady, nil)
	}
	if readyErr != nil {
		t.Fatal("pluginengine: runtime readiness: " + readyErr.Error())
	}
	return e
}

// runPlugin runs the entry point and reports its exit code.
func runPlugin(run func(net.Conn) int, conn net.Conn, exited chan<- int) {
	exited <- run(conn)
}

// startup answers the five stages over the MuxConn, which carries startup on
// both transports.
func (e *Engine) startup(ctx context.Context) error {
	if err := e.expectAndAnswer(ctx, rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}); err != nil {
		return err
	}
	if _, err := e.mux.CallRPC(ctx, methodConfigure, &rpc.ConfigureInput{Sections: []rpc.ConfigSection{}}); err != nil {
		return errors.New("stage 2 configure: " + err.Error())
	}
	if err := e.expectAndAnswer(ctx, methodDeclareCapabilities, nil); err != nil {
		return err
	}
	if _, err := e.mux.CallRPC(ctx, methodShareRegistry, &rpc.ShareRegistryInput{Commands: []rpc.RegistryCommand{}}); err != nil {
		return errors.New("stage 4 share-registry: " + err.Error())
	}
	return e.expectAndAnswer(ctx, methodReady, nil)
}

// expectAndAnswer reads the plugin's next startup request, refuses any other
// method than want, and answers it with result, or OK when result is nil.
func (e *Engine) expectAndAnswer(ctx context.Context, want string, result any) error {
	select {
	case req, ok := <-e.mux.Requests():
		if !ok {
			return errors.New("the plugin closed its connection before " + want)
		}
		if req.Method != want {
			return errors.New("the plugin sent " + req.Method + " where startup expects " + want)
		}
		if result == nil {
			return e.mux.SendOK(ctx, req.ID)
		}
		return e.mux.SendResult(ctx, req.ID, result)
	case <-ctx.Done():
		return errors.New("no " + want + " from the plugin: " + ctx.Err().Error())
	}
}

// serveWire answers the plugin's engine RPCs on the MuxConn until ctx ends or
// the connection closes. It is the wire transport's one engine worker.
func (e *Engine) serveWire(ctx context.Context) {
	for {
		select {
		case req, ok := <-e.mux.Requests():
			if !ok {
				return
			}
			result, err := e.answer(req.Method, req.Params)
			if err != nil {
				e.mux.SendError(ctx, req.ID, err.Error()) //nolint:errcheck // the plugin logs a refused RPC; a failed send means it is gone
				continue
			}
			e.mux.SendResult(ctx, req.ID, result) //nolint:errcheck // a failed send means the plugin is gone
		case <-ctx.Done():
			return
		}
	}
}

// dispatchBridged answers one engine RPC the plugin makes through the bridge.
func (e *Engine) dispatchBridged(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	result, err := e.answer(method, params)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

// answer records one engine RPC and builds its answer. UPDATE and event calls
// have empty fake answers. Source leases require fixture-issued identities.
// Every other method is refused, and stays visible in RPCs.
func (e *Engine) answer(method string, params json.RawMessage) (any, error) {
	e.mu.Lock()
	e.rpcs = append(e.rpcs, EngineRPC{Method: method, Params: append(json.RawMessage(nil), params...)})
	e.mu.Unlock()
	if method == rpc.MethodUpdateRoute {
		return &rpc.UpdateRouteOutput{}, nil
	}
	if method == rpc.MethodEmitEvent {
		return &rpc.EmitEventOutput{}, nil
	}
	if method == rpc.MethodSourceLease {
		return e.sourceLeaseAnswer(params)
	}
	return nil, errors.New("pluginengine: no fake engine answer for " + method)
}

// RPCs is every engine RPC the plugin made since startup, in arrival order.
func (e *Engine) RPCs() []EngineRPC {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]EngineRPC(nil), e.rpcs...)
}

// RouteUpdates decodes every update-route RPC the plugin made, in arrival
// order. A record that does not decode fails the test.
func (e *Engine) RouteUpdates(t testing.TB) []rpc.UpdateRouteInput {
	t.Helper()
	var updates []rpc.UpdateRouteInput
	for _, call := range e.RPCs() {
		if call.Method != rpc.MethodUpdateRoute {
			continue
		}
		var input rpc.UpdateRouteInput
		if err := json.Unmarshal(call.Params, &input); err != nil {
			t.Fatal("pluginengine: decode update-route: " + err.Error())
		}
		updates = append(updates, input)
	}
	return updates
}

// callback sends one engine-to-plugin callback and returns its result, over
// the bridge on TransportBridge and the MuxConn on TransportWire.
func (e *Engine) callback(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if e.bridge == nil {
		return e.mux.CallRPC(ctx, method, params)
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return e.bridge.SendCallback(ctx, method, body)
}

// Deliver delivers one JSON event through the plugin's callback loop and
// returns once the plugin's event handler has run.
func (e *Engine) Deliver(ctx context.Context, event string) error {
	_, err := e.callback(ctx, methodDeliverEvent, &rpc.DeliverEventInput{Event: event})
	return err
}

// SourcePolicy sends one source-policy request and returns the plugin's
// receipt. The receipt MUST echo the request's transaction, operation, source
// identity and target epoch, as the engine requires: a receipt that does not
// is an error, like a transport failure. An owner's refusal is a receipt with
// Status rpc.StatusError, not an error.
func (e *Engine) SourcePolicy(ctx context.Context, input rpc.SourcePolicyInput) (rpc.SourcePolicyOutput, error) {
	var out rpc.SourcePolicyOutput
	raw, err := e.callback(ctx, methodSourcePolicy, input)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, errors.New("decode source-policy receipt: " + err.Error())
	}
	if out.TransactionID != input.TransactionID {
		return out, errors.New("source-policy receipt names transaction " + out.TransactionID + ", not " + input.TransactionID)
	}
	if out.Operation != input.Operation {
		return out, errors.New("source-policy receipt names another operation")
	}
	if out.Source != input.Source {
		return out, errors.New("source-policy receipt names another source identity")
	}
	if out.NextPolicyEpoch != input.NextPolicyEpoch {
		return out, errors.New("source-policy receipt names another target epoch")
	}
	return out, nil
}

// Execute runs one plugin command and returns the document it answered. A
// command that answered an error is an error.
func (e *Engine) Execute(ctx context.Context, command string, args []string) (json.RawMessage, error) {
	return e.ExecutePeer(ctx, command, args, "")
}

// ExecutePeer reaches the real registered SDK callback with the supplied
// peer context. Execute retains its original empty-peer behavior. This fixture
// forwards context; it does not authenticate a caller or issue native authority.
func (e *Engine) ExecutePeer(ctx context.Context, command string, args []string, peer string) (json.RawMessage, error) {
	input := &rpc.ExecuteCommandInput{Serial: "1", Command: command, Args: args, Peer: peer}
	if e.bridge != nil {
		out, err := e.bridge.ExecuteCommand(ctx, input.Serial, input.Command, input.Args, input.Peer)
		if err != nil {
			return nil, err
		}
		if out.Status != rpc.StatusDone {
			return nil, errors.New(command + ": " + out.Status + ": " + string(out.Data))
		}
		return out.Data, nil
	}
	answer, err := e.mux.CallAnswer(ctx, methodExecuteCommand, input)
	if err != nil {
		return nil, err
	}
	document, collapseErr := rpc.CollapseAnswer(answer)
	// Read after the walk, which is what fills them.
	if err := answer.Err(); err != nil {
		return nil, err
	}
	if collapseErr != nil {
		return nil, collapseErr
	}
	if message := answer.Message(); message != "" {
		return nil, errors.New(command + ": " + message)
	}
	return document, nil
}
