package pppoe

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/callsink"
)

// TestRealmFromUsername checks the realm split: the text after the last '@'.
func TestRealmFromUsername(t *testing.T) {
	cases := []struct {
		username, realm string
		ok              bool
	}{
		{"cpe@example.net", "example.net", true},
		{"a@b@Example.NET", "Example.NET", true}, // last '@'; the sink folds case
		{"cpe", "", false},
		{"cpe@", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		realm, ok := realmFromUsername(c.username)
		if realm != c.realm || ok != c.ok {
			t.Errorf("realmFromUsername(%q) = %q, %v; want %q, %v", c.username, realm, ok, c.realm, c.ok)
		}
	}
}

// fakeRelayDriver records the driver calls a handoff makes.
type fakeRelayDriver struct {
	mu         sync.Mutex
	quiesced   int
	released   int
	refusals   []string
	changed    chan struct{}
	quiesceErr error
}

func newFakeRelayDriver() *fakeRelayDriver {
	return &fakeRelayDriver{changed: make(chan struct{}, 16)}
}

func (f *fakeRelayDriver) note() { f.changed <- struct{}{} }

func (f *fakeRelayDriver) QuiesceSession(uint16, uint16) error {
	f.mu.Lock()
	f.quiesced++
	f.mu.Unlock()
	f.note()
	return f.quiesceErr
}

func (f *fakeRelayDriver) ReleaseSession(uint16, uint16) error {
	f.mu.Lock()
	f.released++
	f.mu.Unlock()
	f.note()
	return nil
}

func (f *fakeRelayDriver) RefuseAuth(_, _ uint16, message string) error {
	f.mu.Lock()
	f.refusals = append(f.refusals, message)
	f.mu.Unlock()
	f.note()
	return nil
}

func (f *fakeRelayDriver) counts() (released int, refusals []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.released, append([]string(nil), f.refusals...)
}

// fakeRealmSink accepts every realm and keeps the handoff it was given.
type fakeRealmSink struct {
	mu       sync.Mutex
	handoff  callsink.RealmHandoff
	canceled chan struct{}
	err      error
}

type fakeRealmCall struct{ sink *fakeRealmSink }

func (c fakeRealmCall) Cancel() {
	select {
	case c.sink.canceled <- struct{}{}:
	default:
	}
}

func (f *fakeRealmSink) Relay(callsink.Request) (bool, error) { return false, nil }
func (f *fakeRealmSink) RealmRelayConfigured() bool           { return true }
func (f *fakeRealmSink) RelayRealm(req callsink.RealmRequest) (callsink.RealmCall, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	f.mu.Lock()
	f.handoff = req.Handoff
	f.mu.Unlock()
	return fakeRealmCall{sink: f}, true, nil
}

type realmHarness struct {
	relay  *realmRelay
	driver *fakeRelayDriver
	sink   *fakeRealmSink
	ended  chan uint16
}

// newRealmHarness registers a fake sink and returns a relay that has
// accepted one CHAP request from cpe@example.net.
func newRealmHarness(t *testing.T) *realmHarness {
	t.Helper()
	callsink.Unregister()
	t.Cleanup(callsink.Unregister)
	h := &realmHarness{
		driver: newFakeRelayDriver(),
		sink:   &fakeRealmSink{canceled: make(chan struct{}, 1)},
		ended:  make(chan uint16, 1),
	}
	callsink.Register(h.sink)
	h.relay = newRealmRelay(slog.New(slog.NewTextHandler(io.Discard, nil)), h.driver,
		func(int, uint16) (realmSubscriber, bool) {
			return realmSubscriber{ifName: "eth1", mac: "02:00:00:00:00:01", pppoxFD: 42}, true
		},
		func(_ int, sid uint16) { h.ended <- sid },
	)
	t.Cleanup(h.relay.stopAll)
	req := ppp.EventAuthRequest{
		TunnelID: 3, SessionID: 5, Method: ppp.AuthMethodCHAPMD5, Identifier: 1,
		Username: "cpe@example.net", Challenge: []byte{1}, Response: []byte{2},
		ProxyLCP: &ppp.ProxyLCPCapture{InitialReceived: []byte{1, 4, 5, 0xdc}},
	}
	if !h.relay.decide(&req) {
		t.Fatal("decide did not handle a request whose realm the sink accepts")
	}
	return h
}

func (h *realmHarness) handoff() callsink.RealmHandoff {
	h.sink.mu.Lock()
	defer h.sink.mu.Unlock()
	return h.sink.handoff
}

func waitSignal[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
	var zero T
	return zero
}

// waitRefusal waits until the driver saw one refusal and returns it.
func (h *realmHarness) waitRefusal(t *testing.T) string {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if _, refusals := h.driver.counts(); len(refusals) > 0 {
			return refusals[0]
		}
		select {
		case <-h.driver.changed:
		case <-deadline:
			t.Fatal("no refusal reached the PPP driver")
		}
	}
}

// waitReleased waits until the driver released the PPP session into the
// bridge, so the subscriber-gone that follows meets the bridged handoff.
func waitReleased(t *testing.T, h *realmHarness) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if released, _ := h.driver.counts(); released > 0 {
			return
		}
		select {
		case <-h.driver.changed:
		case <-deadline:
			t.Fatal("the bridge did not release the PPP session")
		}
	}
}

// TestRelayRealmHandoffCancelledInEachPhase walks the rows of the spec's
// handoff table (AC-15) with a fake sink and driver: each cancellation
// leads to its row's outcome, and the handoff goroutine ends.
func TestRelayRealmHandoffCancelledInEachPhase(t *testing.T) {
	t.Run("placing, subscriber leaves", func(t *testing.T) {
		h := newRealmHarness(t)
		h.relay.subscriberGone(3, 5)
		waitSignal(t, h.sink.canceled, "the call cancel")
		h.relay.stopAll()
		if released, refusals := h.driver.counts(); released != 0 || len(refusals) != 0 {
			t.Fatalf("released=%d refusals=%v, want neither for a subscriber that left", released, refusals)
		}
	})
	t.Run("placing, LNS ends the call", func(t *testing.T) {
		h := newRealmHarness(t)
		h.handoff().Ended("CDN from the LNS")
		if msg := h.waitRefusal(t); !strings.Contains(msg, "example.net") {
			t.Fatalf("refusal %q does not name the realm", msg)
		}
		h.relay.stopAll()
		if released, _ := h.driver.counts(); released != 0 {
			t.Fatal("a call that ended before the bridge released the PPP session")
		}
	})
	t.Run("quiesced, subscriber leaves", func(t *testing.T) {
		h := newRealmHarness(t)
		if err := h.handoff().Quiesce(); err != nil {
			t.Fatalf("Quiesce: %v", err)
		}
		h.relay.subscriberGone(3, 5)
		waitSignal(t, h.sink.canceled, "the call cancel")
		h.relay.stopAll()
		if err := h.handoff().Quiesce(); !errors.Is(err, errRealmHandoffOver) {
			t.Fatalf("Quiesce after the subscriber left = %v, want errRealmHandoffOver", err)
		}
		h.handoff().Bridged(nil) // MUST NOT block after the handoff ended
		if released, _ := h.driver.counts(); released != 0 {
			t.Fatal("a bridge reported after the subscriber left released the PPP session")
		}
	})
	t.Run("bridged, LNS ends the call", func(t *testing.T) {
		h := newRealmHarness(t)
		h.handoff().Bridged(nil)
		h.handoff().Ended("StopCCN")
		if sid := waitSignal(t, h.ended, "the PPPoE session end"); sid != 5 {
			t.Fatalf("ended session %d, want 5", sid)
		}
		h.relay.stopAll()
		if released, refusals := h.driver.counts(); released != 1 || len(refusals) != 0 {
			t.Fatalf("released=%d refusals=%v, want one release and no refusal", released, refusals)
		}
	})
	t.Run("bridged, subscriber leaves", func(t *testing.T) {
		// The Bridged row: a PADT after the bridge MUST reach the sink,
		// which tears the L2TP session down with CDN (callsink.RealmCall).
		// Twice reported (PADT, then a PPP session-down) is one cancel.
		h := newRealmHarness(t)
		h.handoff().Bridged(nil)
		waitReleased(t, h)
		h.relay.subscriberGone(3, 5)
		h.relay.subscriberGone(3, 5)
		waitSignal(t, h.sink.canceled, "the call cancel after the bridge")
		h.relay.stopAll()
		if released, refusals := h.driver.counts(); released != 1 || len(refusals) != 0 {
			t.Fatalf("released=%d refusals=%v, want one release and no refusal", released, refusals)
		}
		select {
		case sid := <-h.ended:
			t.Fatalf("PPPoE session %d ended by the relay; the server already ends it on PADT", sid)
		default:
		}
	})
	t.Run("placing, daemon stops", func(t *testing.T) {
		h := newRealmHarness(t)
		h.relay.stopAll()
		waitSignal(t, h.sink.canceled, "the call cancel")
	})
}

// TestLACRealmBridgeFailureRefuses checks AC-16: a bridge error after the
// quiesce cancels the call and refuses the subscriber naming its realm; the
// PPP session is never released into a bridge that does not exist.
func TestLACRealmBridgeFailureRefuses(t *testing.T) {
	h := newRealmHarness(t)
	if err := h.handoff().Quiesce(); err != nil {
		t.Fatalf("Quiesce: %v", err)
	}
	h.handoff().Bridged(errors.New("PPPIOCBRIDGECHAN: operation not supported"))
	if msg := h.waitRefusal(t); !strings.Contains(msg, "example.net") {
		t.Fatalf("refusal %q does not name the realm", msg)
	}
	waitSignal(t, h.sink.canceled, "the call cancel")
	h.relay.stopAll()
	if released, _ := h.driver.counts(); released != 0 {
		t.Fatal("a failed bridge released the PPP session")
	}
}

// TestRelayRealmStartFailureRefuses checks AC-6 at the start: a matched realm
// whose call cannot start is refused, and never reaches the auth handler.
func TestRelayRealmStartFailureRefuses(t *testing.T) {
	callsink.Unregister()
	t.Cleanup(callsink.Unregister)
	sink := &fakeRealmSink{canceled: make(chan struct{}, 1), err: errors.New("remote lns-a unreachable")}
	callsink.Register(sink)
	driver := newFakeRelayDriver()
	relay := newRealmRelay(slog.New(slog.NewTextHandler(io.Discard, nil)), driver,
		func(int, uint16) (realmSubscriber, bool) { return realmSubscriber{pppoxFD: 42}, true },
		func(int, uint16) {})
	t.Cleanup(relay.stopAll)
	req := ppp.EventAuthRequest{TunnelID: 3, SessionID: 5, Method: ppp.AuthMethodPAP,
		Username: "cpe@example.net", ProxyLCP: &ppp.ProxyLCPCapture{}}
	if !relay.decide(&req) {
		t.Fatal("a matched realm whose call failed went on to local authentication")
	}
	if _, refusals := driver.counts(); len(refusals) != 1 || !strings.Contains(refusals[0], "example.net") {
		t.Fatalf("refusals = %v, want one naming the realm", refusals)
	}
}

// TestRelayRealmNoRealmAuthenticatesLocally checks AC-5: a username with no
// realm, or a session without the LCP capture, is not handled by the relay.
func TestRelayRealmNoRealmAuthenticatesLocally(t *testing.T) {
	callsink.Unregister()
	t.Cleanup(callsink.Unregister)
	callsink.Register(&fakeRealmSink{canceled: make(chan struct{}, 1)})
	relay := newRealmRelay(slog.New(slog.NewTextHandler(io.Discard, nil)), newFakeRelayDriver(),
		func(int, uint16) (realmSubscriber, bool) { return realmSubscriber{pppoxFD: 42}, true },
		func(int, uint16) {})
	t.Cleanup(relay.stopAll)
	noRealm := ppp.EventAuthRequest{Username: "cpe", ProxyLCP: &ppp.ProxyLCPCapture{}}
	if relay.decide(&noRealm) {
		t.Fatal("a username with no realm was relayed")
	}
	noCapture := ppp.EventAuthRequest{Username: "cpe@example.net"}
	if relay.decide(&noCapture) {
		t.Fatal("a session without LCP capture was relayed")
	}
}
