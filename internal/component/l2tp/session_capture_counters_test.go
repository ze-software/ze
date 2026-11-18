// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact original counter read observations
// Related: session_capture.go -- original attempt/control/network checks; current_core_driver_fixture_test.go -- real admission

package l2tp

import (
	"errors"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestCapturedCountersKeepOriginalMeasurementAndError exercises the real
// admitted Control with explicit native-model measurements and a physical
// failure. A failure MUST preserve its cause and publish no partial sample.
func TestCapturedCountersKeepOriginalMeasurementAndError(t *testing.T) {
	probe := newCapturedCounterProbe(t)
	probe.owner.setCounterRead(func() (ppp.SessionCounters, error) { return capturedCounterSample(), nil })
	got, err := probe.captured.Counters()
	if err != nil {
		t.Fatal(err)
	}
	if got != capturedCounterSample() {
		t.Fatalf("original raw counters = %+v, want all eight original readings", got)
	}
	failure := errors.New("original physical counter read refused")
	probe.owner.setCounterRead(func() (ppp.SessionCounters, error) { return capturedCounterSample(), failure })
	got, err = probe.captured.Counters()
	if !errors.Is(err, failure) {
		t.Fatalf("physical read cause = %v, want original refusal", err)
	}
	if got != (ppp.SessionCounters{}) {
		t.Fatalf("failed read published partial counters: %+v", got)
	}
}

// TestAccountingNetworkCellSurvivesDualStackAndSessionUp observes the original
// cell inside both real assignment publications and the real SessionUp handler.
// The actual Driver negotiates PAP/IPCP/IPv6CP; only native I/O is modeled.
// Each boundary must preserve the first cell and return all eight raw readings.
// MUTATION: allocate unconditionally in either assignment or handleSessionUp.
func TestAccountingNetworkCellSurvivesDualStackAndSessionUp(t *testing.T) {
	reactor := newReactorForSnapshot(t)
	bus := newTestBus()
	reactor.SetEventBus(bus)
	session := &L2TPSession{localSID: 42, state: L2TPSessionEstablished, pppInterface: "ppp42"}
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"), session)
	wire := newCurrentCoreAdmission(t, reactor, &currentCoreAdmissionConfig{
		ids: currentCoreSessionIDs{tunnelID: 100, sessionID: 42}, ipv6: true,
	})
	reactor.handlePPPEvent(wire.opened)
	owner, ok := wire.startSession.Dataplane.(*currentCoreHandle)
	if !ok {
		t.Fatal("existing native-model fixture changed its owner type")
	}
	owner.setCounterRead(func() (ppp.SessionCounters, error) { return capturedCounterSample(), nil })
	root := &Subsystem{reactors: []*l2tpReactor{reactor}}
	selector := SessionSelector{SID: 42, TID: 100, Birth: session.Birth()}
	var first *sessionTimeoutLifetime
	assignments, ups := 0, 0
	observe := func(boundary string, firstAssignment bool) {
		reactor.tunnelsMu.Lock()
		current := session.timeoutLifetime
		reactor.tunnelsMu.Unlock()
		if firstAssignment {
			first = current
		}
		if first == nil {
			t.Errorf("%s has no original network cell from the first assignment", boundary)
		}
		if current != first {
			t.Errorf("%s replaced the first actual assignment's network cell", boundary)
		}
		captured, err := root.CaptureSession(selector)
		if err != nil {
			t.Errorf("%s cannot capture its original row: %v", boundary, err)
			return
		}
		got, err := captured.Counters()
		if err != nil {
			t.Errorf("%s cannot read its original counters: %v", boundary, err)
			return
		}
		if got != capturedCounterSample() {
			t.Errorf("%s counters = %+v, want all eight original readings", boundary, got)
		}
	}
	t.Cleanup(l2tpevents.SessionIPAssigned.Subscribe(bus, func(event *l2tpevents.SessionIPAssignedPayload) {
		if event.Birth != session.Birth() {
			return
		}
		assignments++
		observe("assignment", assignments == 1)
	}))
	t.Cleanup(l2tpevents.SessionUp.Subscribe(bus, func(event *l2tpevents.SessionUpPayload) {
		if event.Birth != session.Birth() {
			return
		}
		ups++
		observe("SessionUp", false)
	}))
	wire.prepareNetwork(t, "dual-stack-cell", ppp.IPResponseArgs{
		Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.42"),
	}, [8]byte{0, 0, 0, 0, 0, 0, 0, 0x42})
	reactor.handlePPPEvent(wire.assign(t, ppp.AddressFamilyIPv4))
	reactor.handlePPPEvent(wire.assign(t, ppp.AddressFamilyIPv6))
	reactor.handleSessionUp(wire.nextUp(t))
	if assignments != 2 {
		t.Fatalf("actual dual-stack assignments = %d, want two", assignments)
	}
	if ups != 1 {
		t.Fatalf("actual SessionUp publications = %d, want one", ups)
	}
}

// TestCapturedCountersRejectOriginalNetworkChangedDuringRead drives a genuine
// same-worker LCP restart while a native-model read is held. The new network
// remains usable; its pointer cannot validate a measurement begun for the old.
// MUTATION: remove the final timeoutLifetime pointer comparison in Counters.
func TestCapturedCountersRejectOriginalNetworkChangedDuringRead(t *testing.T) {
	probe := newCapturedCounterProbe(t)
	read := probe.hold(t)
	probe.reactor.tunnelsMu.Lock()
	original := probe.session.timeoutLifetime
	probe.reactor.tunnelsMu.Unlock()
	withdrawal := probe.wire.beginRestart(t)
	probe.reactor.handlePPPEvent(withdrawal)
	probe.wire.finishRestart(t)
	probe.reactor.handlePPPEvent(probe.wire.opened)
	probe.wire.prepareNetwork(t, "next-network", ppp.IPResponseArgs{
		Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.43"),
	}, [8]byte{})
	probe.reactor.handlePPPEvent(probe.wire.assign(t, ppp.AddressFamilyIPv4))
	probe.reactor.handleSessionUp(probe.wire.nextUp(t))
	probe.reactor.tunnelsMu.Lock()
	replacement := probe.session.timeoutLifetime
	probe.reactor.tunnelsMu.Unlock()
	if original == nil {
		t.Fatal("setup omitted the first actual network lifetime")
	}
	if replacement == nil {
		t.Fatal("actual LCP restart did not establish its replacement network")
	}
	if replacement == original {
		t.Fatal("actual LCP restart reused its ended network pointer")
	}
	read.rejected(t, ErrSessionNotFound)
	if !probe.wire.opened.Control.Active() {
		t.Fatal("old counter refusal ended the surviving same-worker network")
	}
	probe.owner.setCounterRead(func() (ppp.SessionCounters, error) { return capturedCounterSample(), nil })
	got, err := probe.captured.Counters()
	if err != nil {
		t.Fatalf("fresh read of the surviving logical session failed: %v", err)
	}
	if got != capturedCounterSample() {
		t.Fatalf("surviving network lost its original raw measurement: %+v", got)
	}
}

// TestCapturedCountersRejectReplacedOriginalRow uses a second genuine admission
// as an explicit actor-map replacement input. The original Attempt/Control and
// network cell stay live, isolating original row identity before and after I/O.
// MUTATION: remove the second currentLocked check in capturedSession.Counters.
func TestCapturedCountersRejectReplacedOriginalRow(t *testing.T) {
	probe := newCapturedCounterProbe(t)
	replacement := newCapturedCounterProbe(t)
	read := probe.hold(t)
	probe.reactor.tunnelsMu.Lock()
	tunnel := probe.reactor.tunnelsByLocalID[100]
	tunnel.sessions[42] = replacement.session
	probe.reactor.tunnelsMu.Unlock()
	t.Cleanup(func() {
		probe.reactor.tunnelsMu.Lock()
		tunnel.sessions[42] = probe.session
		probe.reactor.tunnelsMu.Unlock()
	})
	read.rejected(t, ErrSessionNotFound)
	probe.owner.setCounterRead(func() (ppp.SessionCounters, error) {
		t.Error("a stale original row reached its native reader before validation")
		return capturedCounterSample(), nil
	})
	if _, err := probe.captured.Counters(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("pre-read replaced row = %v, want session-not-found", err)
	}
	if !probe.wire.opened.Control.Active() {
		t.Fatal("rejecting the replaced row canceled its original Control")
	}
	if !replacement.wire.opened.Control.Active() {
		t.Fatal("rejecting the old row canceled the replacement Control")
	}
	replacement.measure(t)
}

// TestCapturedCountersRejectChangedAttemptAndControl supplies a second genuine
// Driver admission as an explicit actor-side replacement input while a read is
// held. No fake successful Control is constructed. The original row/Birth and
// original Control stay live, isolating the attempt/bound-control comparisons.
// MUTATION: remove both final pppAttempt and bound-Control equality checks.
func TestCapturedCountersRejectChangedAttemptAndControl(t *testing.T) {
	probe := newCapturedCounterProbe(t)
	replacement := newCapturedCounterProbe(t)
	read := probe.hold(t)
	probe.reactor.tunnelsMu.Lock()
	original := probe.session.pppAttempt
	probe.session.pppAttempt = replacement.wire.opened.Control.Attempt()
	probe.reactor.tunnelsMu.Unlock()
	t.Cleanup(func() {
		probe.reactor.tunnelsMu.Lock()
		probe.session.pppAttempt = original
		probe.reactor.tunnelsMu.Unlock()
	})
	read.rejected(t, ErrSessionNotFound)
	if !probe.wire.opened.Control.Active() {
		t.Fatal("rejecting the changed actor attempt canceled its original Control")
	}
	if !replacement.wire.opened.Control.Active() {
		t.Fatal("rejecting the old measurement canceled the replacement Control")
	}
	replacement.measure(t)
}

// TestCapturedCountersRejectCanceledOriginalControl cancels the actual admitted
// worker during a held native read. Neither a late raw value nor a distinct live
// admission can turn that canceled owner's result into a valid measurement.
// MUTATION: remove the Control's post-read Active check and actor Active check.
func TestCapturedCountersRejectCanceledOriginalControl(t *testing.T) {
	probe := newCapturedCounterProbe(t)
	survivor := newCapturedCounterProbe(t)
	read := probe.hold(t)
	if !probe.wire.opened.Control.Cancel() {
		t.Fatal("original admitted Control did not accept cancellation")
	}
	read.rejected(t, ppp.ErrSessionNotFound)
	if !survivor.wire.opened.Control.Active() {
		t.Fatal("old counter refusal affected a distinct admitted survivor")
	}
	survivor.measure(t)
}

// capturedCounterProbe uses the existing real Driver/LCP/PAP/IPCP fixture.
// Only the native handle's read is modeled; the capture and all worker/control
// relationships come from production admission. The fixture owns its cleanup.
type capturedCounterProbe struct {
	reactor  *l2tpReactor
	session  *L2TPSession
	wire     *currentCoreWire
	owner    *currentCoreHandle
	captured CapturedSession
}

func newCapturedCounterProbe(t *testing.T) *capturedCounterProbe {
	t.Helper()
	reactor := newReactorForSnapshot(t)
	reactor.SetEventBus(newTestBus())
	session := &L2TPSession{localSID: 42, state: L2TPSessionEstablished, pppInterface: "ppp42"}
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"), session)
	wire := newCurrentCoreAdmission(t, reactor, &currentCoreAdmissionConfig{
		ids: currentCoreSessionIDs{tunnelID: 100, sessionID: 42},
	})
	reactor.handlePPPEvent(wire.opened)
	wire.prepareNetwork(t, "counter-owner", ppp.IPResponseArgs{
		Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.42"),
	}, [8]byte{})
	reactor.handlePPPEvent(wire.assign(t, ppp.AddressFamilyIPv4))
	reactor.handleSessionUp(wire.nextUp(t))
	root := &Subsystem{reactors: []*l2tpReactor{reactor}}
	captured, err := root.CaptureSession(SessionSelector{SID: 42, TID: 100, Birth: session.Birth()})
	if err != nil {
		t.Fatal(err)
	}
	owner, ok := wire.startSession.Dataplane.(*currentCoreHandle)
	if !ok {
		t.Fatal("existing native-model fixture changed its owner type")
	}
	return &capturedCounterProbe{reactor: reactor, session: session, wire: wire, owner: owner, captured: captured}
}

// measure proves useful output from this surviving original admission.
func (probe *capturedCounterProbe) measure(t *testing.T) {
	t.Helper()
	probe.owner.setCounterRead(func() (ppp.SessionCounters, error) { return capturedCounterSample(), nil })
	got, err := probe.captured.Counters()
	if err != nil {
		t.Fatalf("surviving original admission could not read counters: %v", err)
	}
	if got != capturedCounterSample() {
		t.Fatalf("surviving original admission counters = %+v, want all eight original readings", got)
	}
}

func capturedCounterSample() ppp.SessionCounters {
	return ppp.SessionCounters{
		RxBytes: 101, TxBytes: 202, RxPackets: 3, TxPackets: 4,
		RxErrors: ppp.MeasuredCounter(5), TxErrors: ppp.MeasuredCounter(6),
		RxDropped: ppp.MeasuredCounter(7), TxDropped: ppp.MeasuredCounter(8),
	}
}

type capturedCounterReadResult struct {
	counters ppp.SessionCounters
	err      error
}

// capturedCounterRead owns one test-only observation goroutine. Both rejected
// and cleanup MUST release its gate and observe done before Driver cleanup.
// No production task, completion receipt or native owner is substituted.
type capturedCounterRead struct {
	release chan struct{}
	done    chan struct{}
	once    sync.Once
	result  capturedCounterReadResult
}

func (probe *capturedCounterProbe) hold(t *testing.T) *capturedCounterRead {
	t.Helper()
	entered := make(chan struct{})
	read := &capturedCounterRead{release: make(chan struct{}), done: make(chan struct{})}
	var held atomic.Bool
	probe.owner.setCounterRead(func() (ppp.SessionCounters, error) {
		if held.CompareAndSwap(false, true) {
			close(entered)
			<-read.release
		}
		return capturedCounterSample(), nil
	})
	go func() {
		defer close(read.done)
		read.result.counters, read.result.err = probe.captured.Counters()
	}()
	t.Cleanup(func() {
		read.finish(t)
		probe.owner.setCounterRead(nil)
	})
	select {
	case <-entered:
	case <-time.After(orderTestBound):
		t.Fatal("actual captured Control did not reach its original native read")
	}
	return read
}

// finish MUST follow hold and publishes no assertion about native release.
func (read *capturedCounterRead) finish(t *testing.T) {
	t.Helper()
	read.once.Do(func() { close(read.release) })
	select {
	case <-read.done:
	case <-time.After(orderTestBound):
		t.Fatal("test-owned captured counter observation did not join")
	}
}

func (read *capturedCounterRead) rejected(t *testing.T, expected error) {
	t.Helper()
	read.finish(t)
	if !errors.Is(read.result.err, expected) {
		t.Fatalf("changed original owner read = %+v, %v, want %v", read.result.counters, read.result.err, expected)
	}
	if read.result.counters != (ppp.SessionCounters{}) {
		t.Fatalf("changed original owner published a partial or replacement sample: %+v", read.result.counters)
	}
}
