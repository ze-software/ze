// Design: docs/architecture/l2tp/subscriber-session-model.md -- the reader contract of the L2TP session events

package l2tpshaper

import (
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/pkg/ze"
)

// reactorTripwire is an L2TP Service whose every method records that it was
// called. Published as the running service, it is the reactor a reader would
// reach through l2tp.LookupService. Safe for concurrent use.
type reactorTripwire struct {
	mu     sync.Mutex
	called []string
}

var _ l2tp.Service = (*reactorTripwire)(nil)

// installReactorTripwire publishes a tripwire as the running L2TP service and
// restores the previous one when the test ends.
func installReactorTripwire(t *testing.T) *reactorTripwire {
	t.Helper()
	tripwire := &reactorTripwire{}
	previous := l2tp.LookupService()
	l2tp.PublishService(tripwire)
	t.Cleanup(func() { l2tp.PublishService(previous) })
	return tripwire
}

func (w *reactorTripwire) trip(method string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.called = append(w.called, method)
}

func (w *reactorTripwire) calls() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.called)
}

func (w *reactorTripwire) Snapshot() l2tp.Snapshot { w.trip("Snapshot"); return l2tp.Snapshot{} }
func (w *reactorTripwire) LookupTunnel(uint16) (l2tp.TunnelSnapshot, bool) {
	w.trip("LookupTunnel")
	return l2tp.TunnelSnapshot{}, false
}
func (w *reactorTripwire) LookupSession(uint16) (l2tp.SessionSnapshot, bool) {
	w.trip("LookupSession")
	return l2tp.SessionSnapshot{}, false
}
func (w *reactorTripwire) Listeners() []l2tp.ListenerSnapshot { w.trip("Listeners"); return nil }
func (w *reactorTripwire) EffectiveConfig() l2tp.ConfigSnapshot {
	w.trip("EffectiveConfig")
	return l2tp.ConfigSnapshot{}
}
func (w *reactorTripwire) TeardownTunnel(uint16) error  { w.trip("TeardownTunnel"); return nil }
func (w *reactorTripwire) TeardownSession(uint16) error { w.trip("TeardownSession"); return nil }
func (w *reactorTripwire) TeardownAllTunnels() int      { w.trip("TeardownAllTunnels"); return 0 }
func (w *reactorTripwire) TeardownAllSessions() int     { w.trip("TeardownAllSessions"); return 0 }
func (w *reactorTripwire) SessionEvents(uint16) []l2tp.ObserverEvent {
	w.trip("SessionEvents")
	return nil
}
func (w *reactorTripwire) LoginSamples(string) []l2tp.CQMBucket { w.trip("LoginSamples"); return nil }
func (w *reactorTripwire) SessionSummaries() []l2tp.SessionSummary {
	w.trip("SessionSummaries")
	return nil
}
func (w *reactorTripwire) LoginSummaries() []l2tp.LoginSummary {
	w.trip("LoginSummaries")
	return nil
}
func (w *reactorTripwire) EchoState(string) *l2tp.LoginEchoState { w.trip("EchoState"); return nil }
func (w *reactorTripwire) ReliableStats(uint16) *l2tp.ReliableStats {
	w.trip("ReliableStats")
	return nil
}
func (w *reactorTripwire) TunnelFSMHistory(uint16) []l2tp.FSMTransition {
	w.trip("TunnelFSMHistory")
	return nil
}
func (w *reactorTripwire) SessionFSMHistory(uint16) []l2tp.FSMTransition {
	w.trip("SessionFSMHistory")
	return nil
}
func (w *reactorTripwire) CaptureSnapshot(int, uint16, string) []l2tp.CaptureEntry {
	w.trip("CaptureSnapshot")
	return nil
}
func (w *reactorTripwire) EnableRawCapture()  { w.trip("EnableRawCapture") }
func (w *reactorTripwire) DisableRawCapture() { w.trip("DisableRawCapture") }
func (w *reactorTripwire) RawCaptureSnapshot(int) []l2tp.RawCaptureEntry {
	w.trip("RawCaptureSnapshot")
	return nil
}
func (w *reactorTripwire) RecordDisconnect(uint16, string, string, uint32) {
	w.trip("RecordDisconnect")
}
func (w *reactorTripwire) PlaceOutgoingCall(string, string) (l2tp.OutgoingCallResult, error) {
	w.trip("PlaceOutgoingCall")
	return l2tp.OutgoingCallResult{}, nil
}

// sessionNestingBus is the engine's own event bus, wrapped to record each
// session event published while another one is still being delivered. The
// engine bus runs in-process handlers on the publishing goroutine, so such a
// publication comes from inside a reader. Safe for concurrent use.
type sessionNestingBus struct {
	ze.EventBus

	mu         sync.Mutex
	delivering int
	nested     []string
}

// newSessionNestingBus wraps a fresh engine bus, the one
// server.Server.Subscribe and server.Server.Emit implement for every plugin
// that runs in process.
func newSessionNestingBus(t *testing.T) *sessionNestingBus {
	t.Helper()
	engine, err := server.NewServer(&server.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("build the engine bus: %v", err)
	}
	return &sessionNestingBus{EventBus: engine}
}

func (b *sessionNestingBus) Emit(namespace, eventType string, payload any) (int, error) {
	if !isSessionEvent(namespace, eventType) {
		return b.EventBus.Emit(namespace, eventType, payload)
	}
	b.mu.Lock()
	if b.delivering > 0 {
		b.nested = append(b.nested, namespace+" "+eventType)
	}
	b.delivering++
	b.mu.Unlock()
	defer b.leave()
	return b.EventBus.Emit(namespace, eventType, payload)
}

func (b *sessionNestingBus) leave() {
	b.mu.Lock()
	b.delivering--
	b.mu.Unlock()
}

func (b *sessionNestingBus) nestedPublications() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.nested)
}

// isSessionEvent names the session lifecycle events of both namespaces: the
// L2TP ones the reactor publishes and the subscriber ones the bridge
// re-publishes from inside its L2TP handler.
func isSessionEvent(namespace, eventType string) bool {
	if namespace == l2tpevents.Namespace {
		return slices.Contains([]string{
			l2tpevents.SessionIPAssignedEvent, l2tpevents.SessionUpEvent, l2tpevents.SessionDownEvent,
		}, eventType)
	}
	if namespace == subevents.Namespace {
		return slices.Contains([]string{
			subevents.SessionIPAssignedEvent, subevents.SessionUpEvent, subevents.SessionDownEvent,
		}, eventType)
	}
	return false
}

// TestSessionEventReaderDoesNotCallBackIntoTheReactor validates A-3 and the
// reader contract for the shaper: shaperPlugin.onSessionUp, onSessionDown and
// onSubscriberSessionDown handle their events without calling into the
// reactor and without publishing a session event from inside the handler. A
// reader runs on the publishing goroutine, so either would wait behind the
// order ticket its own caller holds.
//
// Method: the shaper subscribes to the engine bus through setEventBus, as
// ConfigureEventBus subscribes it, with a configured rate and a recording
// traffic backend so each handler runs its whole body; a tripwire is
// published as the L2TP service. The test fails when any Service method is
// called or any session event is published from inside a handler. The
// session state the handlers store and remove shows that each one ran.
//
// It guards the contract and does not discriminate the reactor's order fix:
// it passes before and after it.
func TestSessionEventReaderDoesNotCallBackIntoTheReactor(t *testing.T) {
	tripwire := installReactorTripwire(t)
	backend := setupMockBackend(t)
	bus := newSessionNestingBus(t)
	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscTBF, DefaultRate: 10_000_000})
	s.setEventBus(bus)
	t.Cleanup(func() {
		s.busMu.Lock()
		defer s.busMu.Unlock()
		s.unsub()
	})

	const tunnelID, sessionID = uint16(11), uint16(7)
	key := sessionKey{tunnelID: tunnelID, sessionID: sessionID}
	if _, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		TunnelID: tunnelID, SessionID: sessionID, Interface: "ppp7",
	}); err != nil {
		t.Fatalf("publish SessionUp: %v", err)
	}
	if _, applied := backend.getApplied("ppp7"); !applied {
		t.Fatal("the SessionUp handler applied no shaping on ppp7: it did not run")
	}
	if _, stored := s.sessions.Load(key); !stored {
		t.Fatal("the SessionUp handler stored no session state: it did not run")
	}

	if _, err := l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{
		TunnelID: tunnelID, SessionID: sessionID, Username: "grace",
		Cause: l2tpevents.TerminateCauseAdminReset,
	}); err != nil {
		t.Fatalf("publish SessionDown: %v", err)
	}
	if _, stored := s.sessions.Load(key); stored {
		t.Fatal("the SessionDown handler left the session state: it did not run")
	}
	if _, err := subevents.SessionDown.Emit(bus, &subevents.SessionDownPayload{
		Session: subscriber.Session{
			ID: "l2tp-11-7", AccessType: subscriber.AccessL2TP,
			TunnelID: tunnelID, SessionID: sessionID, PppInterface: "ppp7",
		},
		Reason: "session-down",
	}); err != nil {
		t.Fatalf("publish the subscriber SessionDown: %v", err)
	}

	if calls := tripwire.calls(); len(calls) != 0 {
		t.Fatalf("a shaper reader of a session event called into the reactor from its handler: %v", calls)
	}
	if nested := bus.nestedPublications(); len(nested) != 0 {
		t.Fatalf("a shaper reader of a session event published a session event from its handler: %v", nested)
	}
}
