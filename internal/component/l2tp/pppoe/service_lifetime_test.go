// Design: docs/architecture/l2tp/followup-l2tp-call.md -- Service-Name call ownership
// Related: server.go -- real PADR/PADT; service_relay.go -- existing event consumer

package pppoe

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/callsink"
)

// A real PADT must cancel its accepted call both before and after bridge
// preparation, release its resources and preserve another admitted subscriber.
func TestServiceRelayPADTCancelsCall(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(map[bool]string{false: "before bridge", true: "after bridge"}[prepared], func(t *testing.T) {
			f := newServiceLifetimeFixture(t, false)
			pads := f.admit(t)
			request, call := f.sink.last()
			if prepared {
				if err := request.BeforeBridge.PrepareBridge(); err != nil {
					t.Fatal(err)
				}
			}
			other := f.addUnrelated(t, pads.SID+1)
			call.onCancel = func() { request.Handoff.Ended("subscriber left") }
			f.server.handlePADT(&Packet{Code: CodePADT, SID: pads.SID, SrcMAC: f.peer, DstMAC: f.server.hwAddr})
			if call.cancels.Load() != 1 {
				t.Fatalf("call cancellations = %d", call.cancels.Load())
			}
			if f.server.sessions.Lookup(pads.SID) != nil {
				t.Fatal("PADT kept its admission")
			}
			f.assertReleased(t, 0)
			if f.server.sessions.Lookup(pads.SID+1) == nil {
				t.Fatal("PADT removed another subscriber")
			}
			if err := other.WithDescriptor(func(int) error { return nil }); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Worker refusal and remote CDN must reach the existing event consumer, which
// sends PADT and releases the accepted access admission exactly once.
func TestServiceRelayEndedReleasesAdmission(t *testing.T) {
	for _, reason := range []string{"worker bridge failed", "remote CDN"} {
		t.Run(reason, func(t *testing.T) {
			f := newServiceLifetimeFixture(t, true)
			pads := f.admit(t)
			request, call := f.sink.last()
			request.Handoff.Ended(reason)
			padt := f.receive(t)
			if padt.Code != CodePADT {
				t.Fatalf("end emitted code %#x", padt.Code)
			}
			if padt.SID != pads.SID {
				t.Fatalf("PADT SID = %d, want %d", padt.SID, pads.SID)
			}
			// The outbound frame precedes ownership removal, so join that
			// same event consumer before inspecting its completed cleanup.
			if err := f.sub.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
			f.assertReleased(t, 0)
			if call.cancels.Load() != 1 {
				t.Fatalf("ended call cancellations = %d", call.cancels.Load())
			}
			request.Handoff.Ended(reason)
			if len(f.frames) != 0 {
				t.Fatal("repeated Ended emitted another discovery frame")
			}
		})
	}
}

// Ended may arrive inside Relay itself. The callback cannot publish PADT while
// the admission is still in discovery; PADS must be the first wire response.
func TestServiceRelayEarlyEndFollowsPADS(t *testing.T) {
	f := newServiceLifetimeFixture(t, true)
	f.sink.beforeReturn = func(request callsink.Request) {
		request.Handoff.Ended("remote call failed before Relay returned")
		f.sub.drainServiceRelayEnds()
		if len(f.frames) != 0 {
			t.Fatal("Ended published PADT before the accepting PADS")
		}
	}
	f.sendPADR(t)
	pads := f.receive(t)
	padt := f.receive(t)
	if pads.Code != CodePADS {
		t.Fatalf("first discovery response = %#x", pads.Code)
	}
	if pads.SID == 0 {
		t.Fatal("accepted call did not receive a session ID")
	}
	if padt.Code != CodePADT {
		t.Fatalf("second discovery response = %#x", padt.Code)
	}
	if padt.SID != pads.SID {
		t.Fatalf("end changed SID %d to %d", pads.SID, padt.SID)
	}
	if err := f.sub.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.assertReleased(t, 0)
}

// A delayed Ended callback keeps the old admission's identity and cannot end a
// later subscriber that obtained the same numeric access SID.
func TestServiceRelayStaleEndKeepsReplacement(t *testing.T) {
	f := newServiceLifetimeFixture(t, false)
	oldPADS := f.admit(t)
	oldRequest, oldCall := f.sink.last()
	f.server.closeAdmitted(oldPADS.SID)
	newPADS := f.admit(t)
	if newPADS.SID != oldPADS.SID {
		t.Fatalf("fixture did not reuse SID: old %d, new %d", oldPADS.SID, newPADS.SID)
	}
	_, newCall := f.sink.last()
	oldRequest.Handoff.Ended("delayed old call end")
	f.sub.drainServiceRelayEnds()
	if f.server.sessions.Lookup(newPADS.SID) == nil {
		t.Fatal("stale Ended removed the replacement")
	}
	if newCall.cancels.Load() != 0 {
		t.Fatal("stale Ended canceled the replacement call")
	}
	if f.owners[1].handle.closes.Load() != 0 {
		t.Fatal("stale Ended closed the replacement lease")
	}
	if err := f.owners[1].opened.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if oldCall.cancels.Load() != 1 {
		t.Fatal("original call was not canceled once")
	}
	if len(f.frames) != 0 {
		t.Fatal("stale Ended sent PADT to the replacement")
	}
}

// Stop must release Service-Name admissions that never started a local PPP
// worker. A lease transferred to PPP remains that driver's responsibility.
func TestServiceRelayStopReleasesAdmissions(t *testing.T) {
	f := newServiceLifetimeFixture(t, true)
	pads := f.admit(t)
	_, call := f.sink.last()
	other := f.addUnrelated(t, pads.SID+1)
	if f.server.sessions.takePPPLease(pads.SID+1) == nil {
		t.Fatal("unrelated PPP did not receive its lease")
	}
	if err := f.sub.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.assertReleased(t, 0)
	if call.cancels.Load() != 1 {
		t.Fatal("stop did not cancel its service call")
	}
	if f.server.sessions.Count() != 0 {
		t.Fatal("stop retained access admissions")
	}
	if f.owners[1].handle.closes.Load() != 0 {
		t.Fatal("stop closed a transferred PPP lease")
	}
	if err := other.WithDescriptor(func(int) error { return nil }); err == nil {
		t.Fatal("stop retained the unrelated access transport")
	}
	notified := make(map[uint16]bool, 2)
	for range 2 {
		padt := f.receive(t)
		if padt.Code != CodePADT {
			t.Fatalf("stop response = %#x", padt.Code)
		}
		if notified[padt.SID] {
			t.Fatal("stop emitted duplicate PADT")
		}
		notified[padt.SID] = true
	}
	if !notified[pads.SID] {
		t.Fatal("stop did not notify its service subscriber")
	}
	if !notified[pads.SID+1] {
		t.Fatal("stop did not notify the other published admission")
	}
	if len(f.frames) != 0 {
		t.Fatal("stop emitted an unexpected PADT")
	}
}

// A contained admission panic after PADS must cancel the accepted service call
// and release its access ownership even though there is no local PPP worker.
func TestServiceRelayAdmissionPanicCancelsCall(t *testing.T) {
	f := newServiceLifetimeFixture(t, false)
	pads := f.admit(t)
	_, call := f.sink.last()
	f.server.endAdmissionAfterPanic()
	padt := f.receive(t)
	if padt.Code != CodePADT {
		t.Fatalf("panic response code = %#x", padt.Code)
	}
	if padt.SID != pads.SID {
		t.Fatalf("panic response SID = %d, want %d", padt.SID, pads.SID)
	}
	if call.cancels.Load() != 1 {
		t.Fatal("contained panic retained its accepted call")
	}
	f.assertReleased(t, 0)
}

// If an admission disappears while Relay is placing its call, the returned
// token must be canceled without publishing a PADS for the removed owner.
func TestServiceRelayRemovedBeforeCallCaptureCancels(t *testing.T) {
	f := newServiceLifetimeFixture(t, false)
	f.sink.beforeReturn = func(request callsink.Request) {
		f.server.closeAdmitted(request.SessionID)
	}
	f.sendPADR(t)
	_, call := f.sink.last()
	if call.cancels.Load() != 1 {
		t.Fatal("vanished admission orphaned its returned call")
	}
	if f.server.sessions.Count() != 0 {
		t.Fatal("vanished admission was recreated")
	}
	if len(f.frames) != 0 {
		t.Fatal("vanished admission published an accepting PADS")
	}
	f.assertReleased(t, 0)
}

type serviceLifetimeFixture struct {
	sub    *Subsystem
	server *InterfaceServer
	sink   *serviceLifetimeSink
	peer   [EthALen]byte
	frames chan []byte
	owners []*serviceLifetimeOwner
}

func newServiceLifetimeFixture(t *testing.T, consume bool) *serviceLifetimeFixture {
	t.Helper()
	key, err := NewCookieKey()
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sub := NewSubsystem(Parameters{})
	sub.logger = log
	f := &serviceLifetimeFixture{sub: sub, sink: &serviceLifetimeSink{}, peer: [EthALen]byte{2, 0, 0, 0, 0, 2}, frames: make(chan []byte, 8)}
	server := &InterfaceServer{
		ifName: "access0", device: "access0", ifIndex: 9,
		hwAddr: [EthALen]byte{2, 0, 0, 0, 0, 1}, sessions: newSessionTable("access0", 8),
		cookieKey: key, cookieTimeout: DefaultCookieTimeout, maxSessionsPerMAC: 8,
		acName: "Ze", serviceNames: []string{"wholesale"}, logger: log, relayWake: sub.relayWake,
	}
	f.server = server
	server.sendFrameFn = func(frame []byte) { f.frames <- append([]byte(nil), frame...) }
	server.openSession = func(context.Context, dataplane.Request) (dataplane.OpenedSession, error) {
		owner := newServiceLifetimeOwner(t)
		f.owners = append(f.owners, owner)
		return owner.opened, nil
	}
	sub.servers = map[int]*InterfaceServer{9: server}
	prior := callsink.Lookup()
	callsink.Register(f.sink)
	t.Cleanup(func() {
		if err := sub.Stop(context.Background()); err != nil {
			t.Error(err)
		}
		for _, sid := range server.sessions.sids() {
			server.closeAdmitted(sid)
		}
		for _, owner := range f.owners {
			if err := owner.opened.Dataplane().Close(); err != nil {
				t.Error(err)
			}
			if err := owner.opened.CloseTransport(); err != nil {
				t.Error(err)
			}
		}
		callsink.Register(prior)
	})
	if consume {
		driver := ppp.NewProductionDriver(log)
		if err := driver.Start(); err != nil {
			t.Fatal(err)
		}
		sub.pppDriver = driver
		sub.eventDone = make(chan struct{})
		sub.stop = make(chan struct{})
		sub.started = true
		go sub.eventConsumer()
	}
	return f
}

func (f *serviceLifetimeFixture) sendPADR(t *testing.T) {
	t.Helper()
	packet := Packet{Code: CodePADR, SrcMAC: f.peer, DstMAC: f.server.hwAddr}
	packet.Tags = []Tag{
		{Type: TagACCookie, Value: GenerateCookie(f.server.cookieKey, f.server.hwAddr[:], f.peer[:], nil)},
		{Type: TagServiceName, Value: []byte("wholesale")},
	}
	f.server.handlePADR(&packet)
}

func (f *serviceLifetimeFixture) admit(t *testing.T) Packet {
	t.Helper()
	f.sendPADR(t)
	packet := f.receive(t)
	if packet.Code != CodePADS {
		t.Fatalf("admission response = %#x", packet.Code)
	}
	if packet.SID == 0 {
		t.Fatal("admission refused its service session")
	}
	return packet
}

func (f *serviceLifetimeFixture) receive(t *testing.T) Packet {
	t.Helper()
	select {
	case frame := <-f.frames:
		packet, err := ParseDiscovery(frame)
		if err != nil {
			t.Fatal(err)
		}
		return packet
	case <-time.After(time.Second):
		t.Fatal("discovery response did not reach the wire observer")
		return Packet{}
	}
}

func (f *serviceLifetimeFixture) assertReleased(t *testing.T, index int) {
	t.Helper()
	owner := f.owners[index]
	if owner.handle.closes.Load() != 1 {
		t.Fatalf("local lease releases = %d", owner.handle.closes.Load())
	}
	if owner.transport.closes.Load() != 1 {
		t.Fatalf("transport releases = %d", owner.transport.closes.Load())
	}
}

func (f *serviceLifetimeFixture) addUnrelated(t *testing.T, sid uint16) dataplane.OpenedSession {
	t.Helper()
	owner := newServiceLifetimeOwner(t)
	f.owners = append(f.owners, owner)
	allocated, err := f.server.sessions.AllocSID()
	if err != nil {
		t.Fatal(err)
	}
	if allocated != sid {
		t.Fatalf("unrelated SID = %d, want %d", allocated, sid)
	}
	if err := f.server.sessions.Add(&Session{SID: sid, State: StateSession}); err != nil {
		t.Fatal(err)
	}
	if !f.server.sessions.setOpened(sid, owner.opened) {
		t.Fatal("unrelated admission was not published")
	}
	return owner.opened
}

type serviceLifetimeOwner struct {
	opened    dataplane.OpenedSession
	handle    *serviceLifetimeHandle
	transport *serviceLifetimeTransport
}

func newServiceLifetimeOwner(t *testing.T) *serviceLifetimeOwner {
	t.Helper()
	owner := &serviceLifetimeOwner{handle: &serviceLifetimeHandle{}, transport: &serviceLifetimeTransport{}}
	var err error
	owner.opened, err = dataplane.NewOpenedSession(&fixtureFrames{}, owner.handle, owner.transport)
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

type serviceLifetimeHandle struct {
	ppp.SessionHandle
	once    sync.Once
	closes  atomic.Int32
	onClose func()
}

func (*serviceLifetimeHandle) Identity() ppp.SessionIdentity {
	return ppp.SessionIdentity{InterfaceName: "created-subscriber", NativeIndex: 901}
}

func (owner *serviceLifetimeHandle) Close() error {
	owner.once.Do(func() {
		if owner.onClose != nil {
			owner.onClose()
		}
		owner.closes.Add(1)
	})
	return nil
}

type serviceLifetimeTransport struct{ closes atomic.Int32 }

func (owner *serviceLifetimeTransport) RelayDescriptor() (int, error) {
	if owner.closes.Load() != 0 {
		return -1, errors.New("synthetic transport is closed")
	}
	return 37, nil // synthetic; no fixture performs an OS operation on it
}

func (owner *serviceLifetimeTransport) Close() error { owner.closes.Add(1); return nil }

type serviceLifetimeSink struct {
	mu           sync.Mutex
	requests     []callsink.Request
	calls        []*serviceLifetimeCall
	beforeReturn func(callsink.Request)
}

func (sink *serviceLifetimeSink) Relay(request callsink.Request) (callsink.RelayCall, bool, error) {
	call := &serviceLifetimeCall{}
	sink.mu.Lock()
	sink.requests = append(sink.requests, request)
	sink.calls = append(sink.calls, call)
	sink.mu.Unlock()
	if sink.beforeReturn != nil {
		sink.beforeReturn(request)
	}
	return call, true, nil
}

func (*serviceLifetimeSink) RelayRealm(callsink.RealmRequest) (callsink.RelayCall, bool, error) {
	return nil, false, nil
}
func (*serviceLifetimeSink) RealmRelayConfigured() bool { return false }

func (sink *serviceLifetimeSink) last() (callsink.Request, *serviceLifetimeCall) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	last := len(sink.requests) - 1
	return sink.requests[last], sink.calls[last]
}

type serviceLifetimeCall struct {
	once     sync.Once
	cancels  atomic.Int32
	onCancel func()
}

func (call *serviceLifetimeCall) Cancel() {
	call.once.Do(func() {
		call.cancels.Add(1)
		if call.onCancel != nil {
			call.onCancel()
		}
	})
}

// Reload must retain a removed access generation while the event consumer is
// completing its already-claimed relay end. Drive the real Reload path with a
// kept second access, so Stop's event-consumer join cannot conceal the race.
func TestServiceRelayReloadWaitsForClaimedEnd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newServiceLifetimeFixture(t, true)
		fixtureSelectBackend(t)
		previousResolve := resolveInterfaceFn
		resolveInterfaceFn = func(name string) (InterfaceBinding, error) {
			switch name {
			case "access0":
				return InterfaceBinding{Device: name, Ifindex: 9, HWAddr: f.server.hwAddr, MTU: 1500}, nil
			case "access1":
				return InterfaceBinding{Device: name, Ifindex: 10, HWAddr: [EthALen]byte{2, 0, 0, 0, 0, 4}, MTU: 1500}, nil
			}
			return InterfaceBinding{}, errors.New("unexpected relay fixture interface")
		}
		t.Cleanup(func() { resolveInterfaceFn = previousResolve })
		f.sub.params.Enabled = true
		f.sub.params.Interfaces = []InterfaceConfig{{Name: "access0"}, {Name: "access1"}}
		f.sub.backend, f.sub.accesses = fixturePrepareConfiguration(t, f.sub.params.Interfaces)
		accessOwner := &serviceLifetimeAccessOwner{}
		access, err := dataplane.NewAccess(dataplane.AccessInfo{
			Device: "access0", ControlDevice: "access0", DiscoveryIfIndex: 9, HWAddr: f.server.hwAddr, MTU: 1500,
		}, accessOwner)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.sub.accesses.accesses["access0"].Close(); err != nil {
			t.Fatal(err)
		}
		f.sub.accesses.accesses["access0"] = access
		kept, err := f.sub.newInterfaceServer(&f.sub.params.Interfaces[1], &f.sub.params, f.sub.backend, f.sub.accesses.accesses["access1"])
		if err != nil {
			t.Fatal(err)
		}
		f.sub.servers[10] = kept
		f.sub.readDone = make(chan struct{})
		close(f.sub.readDone)
		pads := f.admit(t)
		request, call := f.sink.last()
		entered := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		unblock := func() { releaseOnce.Do(func() { close(release) }) }
		defer unblock()
		f.owners[0].handle.onClose = func() { close(entered); <-release }
		request.Handoff.Ended("remote CDN")
		<-entered
		padt := f.receive(t)
		if padt.Code != CodePADT {
			t.Fatalf("end response = %#x", padt.Code)
		}
		if padt.SID != pads.SID {
			t.Fatalf("end PADT SID = %d", padt.SID)
		}
		cp := zeconfig.NewProvider()
		cp.SetRoot(SubsystemName, map[string]any{
			"enabled": "true", "interface": map[string]any{"access1": map[string]any{}},
		})
		reloaded := make(chan error, 1)
		go func() { reloaded <- f.sub.Reload(context.Background(), cp) }()
		synctest.Wait()
		select {
		case err := <-reloaded:
			t.Fatalf("Reload returned before claimed cleanup completed: %v", err)
		default:
		}
		if accessOwner.closes.Load() != 0 {
			t.Fatal("Reload closed access while its relay cleanup was active")
		}
		if f.server.sessions.Lookup(pads.SID) == nil {
			t.Fatal("cleanup freed the SID before its lease and call were released")
		}
		unblock()
		if err := <-reloaded; err != nil {
			t.Fatal(err)
		}
		if accessOwner.closes.Load() != 1 {
			t.Fatal("Reload did not release removed access after cleanup")
		}
		if call.cancels.Load() != 1 {
			t.Fatal("Reload left the old call uncanceled")
		}
		f.assertReleased(t, 0)
		if len(f.frames) != 0 {
			t.Fatal("Reload repeated PADT for the already-ended admission")
		}
		if err := f.sub.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

// A duplicate driver-down event is a drain operation, never an external join.
// It must return while another owner still releases this admission's lease.
func TestServiceRelayDuplicateDownDoesNotJoinCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newServiceLifetimeFixture(t, false)
		pads := f.admit(t)
		entered := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		unblock := func() { releaseOnce.Do(func() { close(release) }) }
		defer unblock()
		f.owners[0].handle.onClose = func() { close(entered); <-release }
		finished := make(chan struct{})
		go func() { f.server.handleSessionDown(pads.SID); close(finished) }()
		<-entered
		duplicate := make(chan struct{})
		go func() { f.server.handleSessionDown(pads.SID); close(duplicate) }()
		synctest.Wait()
		select {
		case <-duplicate:
		default:
			t.Error("duplicate event joined cleanup and blocked the event drain")
		}
		unblock()
		<-finished
		<-duplicate
		f.assertReleased(t, 0)
		if len(f.frames) != 1 {
			t.Fatalf("end responses = %d, want one", len(f.frames))
		}
	})
}

// A full real Driver.EventsOut queue must keep draining while discovery owns
// dispatchMu. The relay wake cannot take that mutex: a discovery PADT may hold
// it while waiting for the PPP finalizer that needs this same queue to drain.
func TestServiceRelayConsumerProgressWithDiscoveryLocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newServiceLifetimeFixture(t, false)
		pads := f.admit(t)
		request, _ := f.sink.last()
		driver := ppp.NewProductionDriver(f.sub.logger)
		if err := driver.Start(); err != nil {
			t.Fatal(err)
		}
		f.sub.pppDriver = driver
		f.sub.eventDone = make(chan struct{})
		f.sub.stop = make(chan struct{})
		f.sub.started = true
		submitted := make(chan struct{})
		go func() {
			for range 256 {
				// Invalid ownership emits a real refusal event, without a
				// kernel descriptor or a worker for the relayed subscriber.
				driver.SessionsIn() <- ppp.StartSession{TunnelID: 65534, SessionID: 1}
			}
			close(submitted)
		}()
		synctest.Wait()
		if len(driver.EventsOut()) != cap(driver.EventsOut()) {
			t.Fatal("fixture did not fill the real driver event queue")
		}
		f.sub.dispatchMu.Lock()
		var unlockOnce sync.Once
		unlock := func() { unlockOnce.Do(f.sub.dispatchMu.Unlock) }
		defer unlock()
		request.Handoff.Ended("remote CDN")
		go f.sub.eventConsumer()
		synctest.Wait()
		select {
		case <-submitted:
		default:
			t.Error("driver event queue stopped draining while discovery held dispatchMu")
		}
		select {
		case frame := <-f.frames:
			padt, err := ParseDiscovery(frame)
			if err != nil {
				t.Fatal(err)
			}
			if padt.Code != CodePADT {
				t.Fatalf("relay response = %#x", padt.Code)
			}
			if padt.SID != pads.SID {
				t.Fatalf("relay PADT SID = %d", padt.SID)
			}
		default:
			t.Error("relay wake waited for discovery instead of publishing its end")
		}
		unlock()
		<-submitted
		if err := f.sub.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		f.assertReleased(t, 0)
	})
}

// Stop notifies only published admissions, closes their transport before PADT,
// and keeps the access lease alive until both notification and cleanup finish.
func TestServiceRelayStopNotifiesSubscriber(t *testing.T) {
	f := newServiceLifetimeFixture(t, true)
	pads := f.admit(t)
	request, call := f.sink.last()
	// Create the same owned Discovery entry handlePADR holds before sendPADS.
	// No accepting PADS was sent for this owner.
	unpublished := newServiceLifetimeOwner(t)
	f.owners = append(f.owners, unpublished)
	pendingSID, err := f.server.sessions.AllocSID()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.server.sessions.Add(&Session{SID: pendingSID, State: StateDiscovery}); err != nil {
		t.Fatal(err)
	}
	if !f.server.sessions.setOpened(pendingSID, unpublished.opened) {
		t.Fatal("could not stage the unpublished admission")
	}
	accessOwner := &serviceLifetimeAccessOwner{}
	access, err := dataplane.NewAccess(dataplane.AccessInfo{
		Device: "access0", ControlDevice: "access0", DiscoveryIfIndex: 9, HWAddr: f.server.hwAddr, MTU: 1500,
	}, accessOwner)
	if err != nil {
		t.Fatal(err)
	}
	f.sub.accesses = &accessConfiguration{accesses: map[string]dataplane.Access{"access0": access}}
	f.server.sendFrameFn = func(frame []byte) {
		packet, err := ParseDiscovery(frame)
		if err != nil {
			t.Fatal(err)
		}
		if packet.Code != CodePADT {
			t.Fatalf("stop emitted code %#x", packet.Code)
		}
		if packet.SID != pads.SID {
			t.Fatal("Stop notified an unpublished discovery admission")
		}
		if f.owners[0].transport.closes.Load() != 1 {
			t.Fatal("Stop sent PADT before closing its PPP transport")
		}
		if accessOwner.closes.Load() != 0 {
			t.Fatal("Stop sent PADT after releasing the discovery access")
		}
		f.frames <- append([]byte(nil), frame...)
	}
	if err := f.sub.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	padt := f.receive(t)
	if padt.SID != pads.SID {
		t.Fatalf("Stop changed the published SID: %d", padt.SID)
	}
	if call.cancels.Load() != 1 {
		t.Fatal("Stop did not cancel its notified relay")
	}
	request.Handoff.Ended("late after Stop")
	if len(f.frames) != 0 {
		t.Fatal("Stop duplicated a PADT or notified an unpublished admission")
	}
	if accessOwner.closes.Load() != 1 {
		t.Fatal("Stop retained its access lease")
	}
	f.assertReleased(t, 0)
	f.assertReleased(t, 1)
}

type serviceLifetimeAccessOwner struct{ closes atomic.Int32 }

func (owner *serviceLifetimeAccessOwner) Close() error { owner.closes.Add(1); return nil }

// A contained cleanup panic still releases transport and Cancel and completes
// the exact generation join. The table cannot retain an unfinished close latch.
func TestServiceRelayCleanupPanicCompletesAdmission(t *testing.T) {
	f := newServiceLifetimeFixture(t, false)
	pads := f.admit(t)
	admission := f.server.sessions.Lookup(pads.SID)
	_, call := f.sink.last()
	cause := errors.New("synthetic lease cleanup panic")
	f.owners[0].handle.onClose = func() { panic(cause) }
	func() {
		defer func() {
			if recovered := recover(); recovered != cause { //nolint:errorlint // The exact original cleanup panic must survive; wrapping would violate the ownership contract.
				t.Fatalf("cleanup panic = %v", recovered)
			}
		}()
		f.server.handleSessionDown(pads.SID)
	}()
	select {
	case <-admission.closeDone:
	default:
		t.Fatal("cleanup panic left the access generation join incomplete")
	}
	if call.cancels.Load() != 1 {
		t.Fatal("cleanup panic retained its relay call")
	}
	if f.owners[0].transport.closes.Load() != 1 {
		t.Fatal("cleanup panic retained its transport")
	}
	if f.server.sessions.Lookup(pads.SID) != nil {
		t.Fatal("cleanup panic retained its SID")
	}
	f.sub.closeInterface(f.server)
}
