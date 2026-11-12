// Design: docs/guide/ddos-mitigation.md -- what each end of the plugin does to a live FlowSpec rule
// Related: register.go (runEngine), responder.go (withdraw)

package flowspec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

// exitTestVictim is the victim every engine-pair test in this package
// announces against. A documentation prefix, so a rendered command that
// names it can only have come from the test.
const exitTestVictim = "192.0.2.9/32"

// exitTestBus is an in-process event bus: Emit runs every handler on the
// caller's goroutine, as the daemon's bus does for an in-process subscriber,
// so an event the test emits has been handled when Emit returns.
type exitTestBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func (b *exitTestBus) Emit(namespace, eventType string, payload any) (int, error) {
	key := namespace + "/" + eventType
	b.mu.Lock()
	handlers := slices.Clone(b.subs[key])
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return 0, nil
}

func (b *exitTestBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.subs[key] = append(b.subs[key], handler)
	b.mu.Unlock()
	return func() {}
}

var _ ze.EventBus = (*exitTestBus)(nil)

// stateCallRecord is one state RPC the plugin sent the engine.
type stateCallRecord struct {
	method string
	key    string
	data   []byte
}

// engineStub plays the daemon's half of the plugin connection: it answers
// every request the plugin sends and records the update-route commands and
// the state RPCs in arrival order. It records a request BEFORE it answers it,
// so a call the plugin has seen answered is always in the record.
//
// Safe for concurrent use: serve runs on its own goroutine while the test
// reads the record.
type engineStub struct {
	t   *testing.T
	mux *rpc.MuxConn

	mu     sync.Mutex
	routes []string
	state  []stateCallRecord
	store  *stubStore // what the state calls read and change
	// routeRefusal, when set, is the error every update-route is answered with,
	// as the daemon answers a send it refuses (reactor errSendNotPermitted).
	routeRefusal string

	// seen carries every request method, in arrival order, so the handshake
	// can wait for the stage the plugin reached instead of sleeping.
	seen chan string
	// done is closed when runEngine returns, and exitCode holds what it returned.
	done     chan struct{}
	exitCode int
}

// startEngineStub runs runEngine over a net.Pipe against an engineStub, drives
// the five-stage startup with a `ddos flowspec` section at response-level
// enforce with blackhole-fallback, and returns once the plugin reached ready.
// stored seeds what the plugin's state-get finds.
func startEngineStub(t *testing.T, stored map[string][]byte) (*engineStub, *exitTestBus) {
	t.Helper()
	return startEngineStubOn(t, newStubStore(stored), "")
}

// stubStore is the daemon's runtime state store as the engine stub keeps it:
// a put writes the key, a remove deletes it, and a get reads what is there
// now. Two stubs share one to play a plugin restart against one store.
//
// Safe for concurrent use.
type stubStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

// newStubStore returns a store holding a copy of seed.
func newStubStore(seed map[string][]byte) *stubStore {
	s := &stubStore{data: make(map[string][]byte, len(seed))}
	maps.Copy(s.data, seed)
	return s
}

// apply performs one state call and returns the daemon's answer to it.
func (s *stubStore) apply(method, key string, data []byte) *rpc.StateOutput {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch method {
	case rpc.MethodStatePut:
		s.data[key] = data
	case rpc.MethodStateRemove:
		delete(s.data, key)
	case rpc.MethodStateGet:
		value, found := s.data[key]
		if !found {
			return &rpc.StateOutput{Status: rpc.StateAbsent}
		}
		return &rpc.StateOutput{Status: rpc.StateOK, Data: value}
	}
	return &rpc.StateOutput{Status: rpc.StateOK}
}

// holds reports whether key is stored now.
func (s *stubStore) holds(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, found := s.data[key]
	return found
}

// startEngineStubOn is startEngineStub over store, answering every
// update-route with routeRefusal when it is not empty.
func startEngineStubOn(t *testing.T, store *stubStore, routeRefusal string) (*engineStub, *exitTestBus) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	bus := &exitTestBus{subs: make(map[string][]func(any))}
	var eb ze.EventBus = bus
	previousBus := eventBusPtr.Load()
	eventBusPtr.Store(&eb)
	t.Cleanup(func() { eventBusPtr.Store(previousBus) })

	pluginEnd, engineEnd := net.Pipe()
	e := &engineStub{
		t:            t,
		mux:          rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)),
		store:        store,
		routeRefusal: routeRefusal,
		seen:         make(chan string, 256),
		done:         make(chan struct{}),
	}
	go e.run(pluginEnd)
	go e.serve(ctx)
	t.Cleanup(func() {
		_ = e.mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-e.done:
		case <-time.After(5 * time.Second):
			t.Error("the ddos-flowspec engine did not exit after its transport closed")
		}
	})

	section := `{"ddos":{"flowspec":{"response-level":"enforce","action":"discard","blackhole-fallback":"true"}}}`
	e.waitRequest(ctx, rpc.MethodDeclareRegistration)
	e.callback(ctx, "ze-plugin-callback:configure", rpc.ConfigureInput{
		Sections: []rpc.ConfigSection{{Root: configRoot, Data: section}},
	})
	e.waitRequest(ctx, "ze-plugin-engine:declare-capabilities")
	e.callback(ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	e.waitRequest(ctx, "ze-plugin-engine:ready")
	return e, bus
}

// run is the plugin's side: the production engine function over its end of the pipe.
func (e *engineStub) run(conn net.Conn) {
	e.exitCode = runEngine(conn)
	close(e.done)
}

// serve answers the plugin's requests until the connection closes.
func (e *engineStub) serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-e.mux.Requests():
			if !ok {
				return
			}
			e.answer(ctx, req)
		}
	}
}

// answer records req and sends the answer the daemon would send.
func (e *engineStub) answer(ctx context.Context, req *rpc.Request) {
	var err error
	switch req.Method {
	case rpc.MethodDeclareRegistration:
		err = e.mux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion})
	case rpc.MethodUpdateRoute:
		var in rpc.UpdateRouteInput
		if jsonErr := json.Unmarshal(req.Params, &in); jsonErr != nil {
			e.t.Errorf("update-route params %s: %v", req.Params, jsonErr)
		}
		e.mu.Lock()
		e.routes = append(e.routes, in.Command)
		e.mu.Unlock()
		if e.routeRefusal != "" {
			err = e.mux.SendError(ctx, req.ID, e.routeRefusal)
			break
		}
		err = e.mux.SendResult(ctx, req.ID, &rpc.UpdateRouteOutput{})
	case rpc.MethodStateGet, rpc.MethodStatePut, rpc.MethodStateRemove:
		err = e.mux.SendResult(ctx, req.ID, e.recordState(req))
	default:
		err = e.mux.SendOK(ctx, req.ID)
	}
	if err != nil {
		e.t.Errorf("answering %s: %v", req.Method, err)
	}
	// Only the handshake waits on seen, and it sends a handful of requests, so a
	// full buffer means nobody waits any more and the method can be dropped.
	select {
	case e.seen <- req.Method:
	default:
	}
}

// recordState records one state RPC and computes the store's answer.
func (e *engineStub) recordState(req *rpc.Request) *rpc.StateOutput {
	var in rpc.StateInput
	if err := json.Unmarshal(req.Params, &in); err != nil {
		e.t.Errorf("%s params %s: %v", req.Method, req.Params, err)
	}
	e.mu.Lock()
	e.state = append(e.state, stateCallRecord{method: req.Method, key: in.Key, data: in.Data})
	e.mu.Unlock()
	return e.store.apply(req.Method, in.Key, in.Data)
}

// doctorCheck runs the plugin's declared runtime check through the
// doctor-check callback, as the daemon does for `ze doctor`, and returns the
// findings it answered.
func (e *engineStub) doctorCheck(ctx context.Context, name string) []rpc.DoctorCheckDiagnostic {
	e.t.Helper()
	raw, err := e.mux.CallRPC(ctx, "ze-plugin-callback:doctor-check", rpc.DoctorCheckInput{Name: name})
	if err != nil {
		e.t.Fatalf("doctor-check %s: %v", name, err)
	}
	var out rpc.DoctorCheckOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		e.t.Fatalf("doctor-check %s answer %s: %v", name, raw, err)
	}
	return out.Diagnostics
}

// stop ends the plugin the way a restart does: the daemon sends no `bye` and
// closes the connection (Process.Stop), then waits for the engine function to
// return.
func (e *engineStub) stop(ctx context.Context) {
	e.t.Helper()
	_ = e.mux.Close()
	select {
	case <-e.done:
	case <-ctx.Done():
		e.t.Fatalf("the ddos-flowspec engine did not stop after its connection closed: %v", ctx.Err())
	}
}

// waitRequest returns once the plugin has sent a request named method.
func (e *engineStub) waitRequest(ctx context.Context, method string) {
	e.t.Helper()
	for {
		select {
		case got := <-e.seen:
			if got == method {
				return
			}
		case <-ctx.Done():
			e.t.Fatalf("waiting for the plugin to send %s: %v", method, ctx.Err())
		}
	}
}

// callback sends one engine-to-plugin callback and fails the test on an error answer.
func (e *engineStub) callback(ctx context.Context, method string, input any) {
	e.t.Helper()
	if _, err := e.mux.CallRPC(ctx, method, input); err != nil {
		e.t.Fatalf("%s: %v", method, err)
	}
}

// bye sends the engine's `bye` with reason `removed`, as notifyPluginRemoval
// does (internal/component/plugin/server/startup_removal.go), and returns once
// the plugin answered it.
func (e *engineStub) bye(ctx context.Context) {
	e.t.Helper()
	e.callback(ctx, "ze-plugin-callback:bye", map[string]string{"reason": "removed"})
}

// routeCommands returns the update-route commands the engine received so far.
func (e *engineStub) routeCommands() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.routes...)
}

// stateCalls returns the state RPCs the engine received so far.
func (e *engineStub) stateCalls() []stateCallRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]stateCallRecord(nil), e.state...)
}

// waitExit returns once runEngine has returned.
func (e *engineStub) waitExit(ctx context.Context) {
	e.t.Helper()
	select {
	case <-e.done:
	case <-ctx.Done():
		e.t.Fatalf("the ddos-flowspec engine did not stop after bye: %v", ctx.Err())
	}
	if e.exitCode != 0 {
		e.t.Fatalf("runEngine returned %d after an answered bye, want 0", e.exitCode)
	}
}

// announceVictim drives one critical remote AttackDetected through the bus,
// which the blackhole fallback turns into one announcement, and fails the test
// unless the engine received it.
func announceVictim(t *testing.T, e *engineStub, bus ze.EventBus) {
	t.Helper()
	if _, err := ddosevent.Detected.Emit(bus, &ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17},
		Family:    ddosevent.FamilyUDPFlood,
		Severity:  ddosevent.SeverityCritical,
		Direction: ddosevent.DirectionRemote,
	}); err != nil {
		t.Fatalf("emit attack-detected: %v", err)
	}
	if countCommands(e.routeCommands(), "/flow add ") != 1 {
		t.Fatalf("setup: the engine did not receive one announcement for %s (commands: %v)",
			exitTestVictim, e.routeCommands())
	}
}

// countCommands counts the commands that hold mode, "/flow add " or
// "/flow del ", and name the test victim.
func countCommands(cmds []string, mode string) int {
	n := 0
	for _, cmd := range cmds {
		if strings.Contains(cmd, mode) && strings.Contains(cmd, exitTestVictim) {
			n++
		}
	}
	return n
}

// TestByeWithdrawsTheLiveAnnouncement proves that the engine's `bye` withdraws
// the rule the plugin announced, and that the withdrawal reaches the engine
// before the plugin answers the `bye`.
//
// VALIDATES: AC-11. The OnBye handler runs the stop function, whose withdrawal
// goes out over update-route while the connection is still open: the engine
// waits for the `bye` answer before it stops a removed plugin
// (notifyPluginRemoval), so a withdrawal sent after the answer reaches nobody.
// METHOD: runEngine over a real sdk.Plugin pair (A-11), one announcement through
// the bus, then `bye`; the engine stub's record is read once the answer arrived.
// PREVENTS: plan/journal/component-rebuilt-during-reload.md (2026-09-20): the
// plugin registered no `bye` handler, so a removal left the FlowSpec rule at
// every peer for the life of the daemon.
func TestByeWithdrawsTheLiveAnnouncement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e, bus := startEngineStub(t, nil)
	announceVictim(t, e, bus)

	e.bye(ctx)

	if got := countCommands(e.routeCommands(), "/flow del "); got != 1 {
		t.Fatalf("withdrawals for %s before the bye answer = %d, want 1 (commands: %v)",
			exitTestVictim, got, e.routeCommands())
	}
	e.waitExit(ctx)
}

// TestByeWithNothingAnnouncedSendsNothing proves that a `bye` reaching a plugin
// with no live rule sends the engine no route command.
//
// VALIDATES: AC-11's survivor: when the apply of the same reload already
// withdrew, the handler finds nothing live and sends nothing.
// METHOD: runEngine over a real sdk.Plugin pair, startup with no attack, then `bye`.
// PREVENTS: a stop function that withdraws unconditionally, sending an
// MP_UNREACH for an invalid prefix the plugin never announced.
func TestByeWithNothingAnnouncedSendsNothing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e, _ := startEngineStub(t, nil)

	e.bye(ctx)

	if cmds := e.routeCommands(); len(cmds) != 0 {
		t.Fatalf("route commands after a bye with nothing announced = %v, want none", cmds)
	}
	e.waitExit(ctx)
}

// ctxDispatcher refuses every call once its context ended, as the SDK's
// UpdateRoute and state calls do. WithContext hands back the same record under
// another context, so the test can tell which context a call was sent under.
type ctxDispatcher struct {
	ctx  context.Context
	cmds *[]string
}

func (d ctxDispatcher) Dispatch(cmd string) error {
	if err := d.ctx.Err(); err != nil {
		return err
	}
	*d.cmds = append(*d.cmds, cmd)
	return nil
}

func (d ctxDispatcher) WithContext(ctx context.Context) routeDispatcher {
	return ctxDispatcher{ctx: ctx, cmds: d.cmds}
}

func (d ctxDispatcher) Record(flowspecMatch) error { return d.ctx.Err() }
func (d ctxDispatcher) Forget() error              { return d.ctx.Err() }

func (d ctxDispatcher) Recorded() ([]byte, bool, error) { return nil, false, d.ctx.Err() }

// TestStopResponderWithdrawsUnderItsOwnContext proves that the stop function
// withdraws when the context the responder was built with has already ended.
//
// VALIDATES: AC-13. On the way out runEngine has canceled the signal context
// before stopResponder runs (the cap worker's defer), so a withdrawal sent under
// it would be refused before it left.
// METHOD: a responder announces through a dispatcher whose context the test then
// cancels; stopResponder must still dispatch the withdrawal.
// PREVENTS: a stop function that reuses the signal context and withdraws nothing.
func TestStopResponderWithdrawsUnderItsOwnContext(t *testing.T) {
	signalCtx, cancelSignal := context.WithCancel(context.Background())
	defer cancelSignal()
	var cmds []string
	r := newResponder(enforcingConfig(), ctxDispatcher{ctx: signalCtx, cmds: &cmds})
	announceOnce(t, r, exitTestVictim)

	cancelSignal()
	stopResponder(r)

	if got := countCommands(cmds, "/flow del "); got != 1 {
		t.Fatalf("withdrawals after the signal context ended = %d, want 1 (commands: %v)", got, cmds)
	}
}

// TestStopResponderOnAClosedTransportLogsOnce proves that a stop on a closed
// connection logs one INFO line and no error, and leaves the record.
//
// VALIDATES: AC-13 survivor and R-7. At a daemon stop the engine closes the
// connection without a `bye`, so no withdrawal can leave; the BGP sessions end
// with the daemon and the record stays for the next start (AC-15).
// METHOD: a responder announces through fakeDispatcher, the dispatcher then
// answers the closed-connection error the SDK returns, and the plugin logger
// writes to a buffer the test reads.
// PREVENTS: an ERROR line at every daemon stop with a live rule, and a record
// removed for a withdrawal that never left.
func TestStopResponderOnAClosedTransportLogsOnce(t *testing.T) {
	var logs bytes.Buffer
	previous := loggerPtr.Load()
	setLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { loggerPtr.Store(previous) })

	disp := &fakeDispatcher{}
	r := newResponder(enforcingConfig(), disp)
	announceOnce(t, r, exitTestVictim)
	disp.err = fmt.Errorf("update-route: %w", rpc.ErrMuxConnClosed)
	logs.Reset()

	stopResponder(r)

	var closedLines int
	for line := range strings.SplitSeq(strings.TrimSpace(logs.String()), "\n") {
		if strings.Contains(line, "level=ERROR") || strings.Contains(line, "level=WARN") {
			t.Errorf("a stop on a closed connection logged %q, want no warning or error", line)
		}
		if strings.Contains(line, "level=INFO") && strings.Contains(line, "the BGP sessions end with the daemon") {
			closedLines++
		}
	}
	if closedLines != 1 {
		t.Errorf("INFO lines saying the BGP sessions end with the daemon = %d, want 1 (log: %s)", closedLines, logs.String())
	}
	if disp.recorded == nil {
		t.Errorf("the record was removed although the withdrawal never left")
	}
}
