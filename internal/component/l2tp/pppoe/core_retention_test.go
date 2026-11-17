// Design: docs/architecture/l2tp/bng-5-pppoe.md -- exact core admission retention
// Related: server.go -- PADT/panic retain a transferred row until its real terminal event
package pppoe

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// The only fake boundary is the created native owner and physical wire gate.
// Admission, auth waiting, cancellation, Down/refusal and their capabilities
// are emitted by the actual production Driver.
type coreRetentionOwner struct {
	net.Conn
	identity     ppp.SessionIdentity
	mu           sync.Mutex
	failure      error
	writes       atomic.Int32
	closes       atomic.Int32
	panicFailure error
}

func (owner *coreRetentionOwner) Control() io.ReadWriter { return owner }
func (owner *coreRetentionOwner) Write(data []byte) (int, error) {
	owner.writes.Add(1)
	return owner.Conn.Write(data)
}
func (*coreRetentionOwner) Opened() io.Reader                   { return nil }
func (owner *coreRetentionOwner) InterruptRead() error          { return owner.SetReadDeadline(time.Now()) }
func (owner *coreRetentionOwner) Identity() ppp.SessionIdentity { return owner.identity }
func (owner *coreRetentionOwner) Close() error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.closes.Add(1)
	if owner.panicFailure != nil {
		panic(owner.panicFailure)
	}
	return errors.Join(owner.Conn.Close(), owner.failure)
}
func (owner *coreRetentionOwner) clearFailure() {
	owner.mu.Lock()
	owner.failure = nil
	owner.mu.Unlock()
}
func (*coreRetentionOwner) SetMTU(int) error                                            { return nil }
func (*coreRetentionOwner) Connect() error                                              { return nil }
func (*coreRetentionOwner) Disconnect() error                                           { return nil }
func (*coreRetentionOwner) SetReceiveMRU(uint16) error                                  { return nil }
func (*coreRetentionOwner) SetAdminUp() error                                           { return nil }
func (*coreRetentionOwner) AddAddressP2P(netip.Prefix, netip.Prefix) error              { return nil }
func (*coreRetentionOwner) RemoveAddress(netip.Prefix) error                            { return nil }
func (*coreRetentionOwner) AddRoute(netip.Prefix, netip.Addr, int, rtproto.Proto) error { return nil }
func (*coreRetentionOwner) RemoveRoute(netip.Prefix, netip.Addr, int, rtproto.Proto) error {
	return nil
}
func (*coreRetentionOwner) Counters() (ppp.SessionCounters, error) { return ppp.SessionCounters{}, nil }
func (*coreRetentionOwner) OpenIPv6Carrier(context.Context) (ppp.IPv6Carrier, error) {
	return nil, errors.New("core retention fixture has no IPv6 service")
}

type coreRetentionTransport struct {
	owner  *coreRetentionOwner
	once   sync.Once
	closed chan struct{}
	err    error
}

func (transport *coreRetentionTransport) Close() error {
	transport.once.Do(func() {
		close(transport.closed)
		transport.err = transport.owner.Conn.Close()
		if errors.Is(transport.err, io.ErrClosedPipe) || errors.Is(transport.err, net.ErrClosed) {
			transport.err = nil
		}
	})
	return transport.err
}

type coreRetentionFixture struct {
	sub     *Subsystem
	server  *InterfaceServer
	driver  *ppp.Driver
	frames  atomic.Int32
	started bool
}

func newCoreRetentionFixture(t *testing.T, ifindex int) *coreRetentionFixture {
	t.Helper()
	return newCoreRetentionFixtureStart(t, ifindex, true)
}
func newCoreRetentionFixtureStart(t *testing.T, ifindex int, start bool) *coreRetentionFixture {
	t.Helper()
	sub := newTestSubsystem(t, newRecordBus(), ifindex)
	server := sub.servers[ifindex]
	server.sessions = newSessionTable(server.ifName, 1)
	server.hwAddr = [EthALen]byte{2, 0, 0, 0, 0, 1}
	server.cookieTimeout = DefaultCookieTimeout
	fixture := &coreRetentionFixture{sub: sub, server: server, driver: ppp.NewProductionDriver(slog.Default())}
	server.sendFrameFn = func(frame []byte) {
		packet, err := ParseDiscovery(frame)
		if err != nil {
			t.Error(err)
			return
		}
		if packet.Code == CodePADS && packet.SID != 0 {
			return
		}
		if packet.Code != CodePADT {
			t.Error("retention cleanup emitted an unexpected discovery frame")
			return
		}
		fixture.frames.Add(1)
	}
	if start {
		if err := fixture.driver.Start(); err != nil {
			t.Fatal(err)
		}
		fixture.started = true
	}
	sub.pppDriver = fixture.driver
	server.pppDriver = fixture.driver
	t.Cleanup(func() {
		if !fixture.started {
			if err := fixture.driver.Start(); err != nil {
				t.Error(err)
				return
			}
		}
		fixture.driver.Stop()
	})
	return fixture
}
func (fixture *coreRetentionFixture) queue(t *testing.T, sid uint16, valid bool, failure error) (*Session, *coreRetentionTransport) {
	t.Helper()
	session, transport, start := fixture.prepare(t, sid, valid, failure)
	if fixture.server.sessions.takePPPLease(session) == nil {
		t.Fatal("created owner was not handed to PPP")
	}
	fixture.driver.SessionsIn() <- start
	return session, transport
}
func (fixture *coreRetentionFixture) prepare(t *testing.T, sid uint16, valid bool, failure error) (*Session, *coreRetentionTransport, ppp.StartSession) {
	t.Helper()
	transportSide, peer := net.Pipe()
	owner := &coreRetentionOwner{Conn: transportSide, identity: ppp.SessionIdentity{InterfaceName: "core-retention", NativeIndex: uint32(fixture.server.ifIndex)}, failure: failure}
	transport := &coreRetentionTransport{owner: owner, closed: make(chan struct{})}
	opened, err := dataplane.NewOpenedSession(owner, owner, transport)
	if err != nil {
		t.Fatal(err)
	}
	id := pppoeSessionID(fixture.server.ifIndex, sid)
	attempt, err := ppp.NewSessionAttempt(id, owner)
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{SubscriberID: id, AccessIfIndex: fixture.server.ifIndex, SID: sid, MAC: net.HardwareAddr{2, 0, 0, 0, 0, byte(sid)}, IfName: fixture.server.ifName, State: StateDiscovery}
	if err := fixture.server.sessions.Add(session); err != nil {
		t.Fatal(err)
	}
	if !fixture.server.sessions.setOpened(session, opened, attempt) {
		t.Fatal("created owner was not retained by its admission")
	}
	fixture.server.admitting = admission{session: session}
	request := Packet{DstMAC: fixture.server.hwAddr}
	copy(request.SrcMAC[:], session.MAC)
	var padsBuffer [EthMaxLen]byte
	pads := BuildPADS(padsBuffer[:], fixture.server.hwAddr, &request, fixture.server.acName, sid)
	if pads == nil {
		t.Fatal("prepared PADS was not encoded")
	}
	fixture.server.sendPADS(sid, pads)
	t.Cleanup(func() {
		owner.clearFailure()
		owner.mu.Lock()
		owner.panicFailure = nil
		owner.mu.Unlock()
		if err := opened.CloseTransport(); err != nil {
			t.Error(err)
		}
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		subscriber.DefaultRegistry.Remove(id)
	})
	var lcp ppp.LCPConfig
	if valid {
		lcp, err = ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
		if err != nil {
			t.Fatal(err)
		}
	}
	options := []byte{1, 4, 5, 220}
	start := ppp.StartSession{LCP: lcp, TunnelID: uint16(fixture.server.ifIndex), SessionID: sid, SubscriberID: id, Attempt: attempt, Frames: owner, Dataplane: owner, LNSMode: true, ProxyLCPInitialRecv: options, ProxyLCPLastSent: options, ProxyLCPLastRecv: options}
	return session, transport, start
}
func (fixture *coreRetentionFixture) next(t *testing.T) ppp.Event {
	t.Helper()
	select {
	case event, open := <-fixture.driver.EventsOut():
		if !open {
			t.Fatal("actual Driver events closed before the required terminal event")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("actual Driver omitted the required event")
		return nil
	}
}
func (fixture *coreRetentionFixture) admit(t *testing.T, sid uint16) (*Session, *coreRetentionTransport, ppp.SessionControl) {
	t.Helper()
	session, transport := fixture.queue(t, sid, true, nil)
	opened, ok := fixture.next(t).(ppp.EventLCPUp)
	if !ok {
		t.Fatal("actual Driver did not admit the retained attempt")
	}
	fixture.sub.handlePPPEvent(opened)
	select {
	case event := <-fixture.driver.AuthEventsOut():
		request, ok := event.(ppp.EventAuthRequest)
		if !ok {
			t.Fatal("actual Driver omitted its pending auth request")
		}
		if request.Control != opened.Control {
			t.Fatal("auth request changed admitted worker")
		}
	case <-time.After(time.Second):
		t.Fatal("actual Driver omitted authentication")
	}
	subscriber.DefaultRegistry.Add(&subscriber.Session{ID: session.SubscriberID, AccessType: subscriber.AccessPPPoE, State: subscriber.StateActive, AccessIfIndex: session.AccessIfIndex, PPPoESID: sid, Username: "original"})
	return session, transport, opened.Control
}
func assertCoreAdmissionHeld(t *testing.T, fixture *coreRetentionFixture, session *Session, opened dataplane.OpenedSession) {
	t.Helper()
	if fixture.server.sessions.Lookup(session.SID) != session {
		t.Fatal("wire teardown removed the admission before its genuine terminal consumer")
	}
	if session.Opened != opened {
		t.Fatal("wire teardown detached the original Opened borrower")
	}
	select {
	case <-session.closeDone:
		t.Fatal("wire teardown completed closeDone before its terminal consumer")
	default:
	}
	if _, err := fixture.server.sessions.AllocSID(); !errors.Is(err, ErrMaxSessions) {
		t.Fatal("held admission no longer reserves its configured slot", err)
	}
	if _, found := subscriber.DefaultRegistry.Get(session.SubscriberID); !found {
		t.Fatal("wire teardown withdrew shared state before the terminal consumer")
	}
}

// TestCorePPPoEPADTWaitsForRealDown keeps the actual Driver event reader held
// across PADT. It then consumes the original Down before admitting the same-ID
// successor and replays old Down against that new worker.
func TestCorePPPoEPADTWaitsForRealDown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newCoreRetentionFixture(t, 0x10001)
		session, transport, control := fixture.admit(t, 81)
		opened := session.Opened
		downs := 0
		subevents.SessionDown.Subscribe(fixture.sub.bus, func(event *subevents.SessionDownPayload) {
			downs++
			if event.Session.ID != session.SubscriberID || event.Session.AccessIfIndex != 0x10001 {
				t.Error("terminal consumer truncated the canonical access identity")
			}
			select {
			case <-session.closeDone:
				t.Error("closeDone preceded shared Down")
			default:
			}
		})
		packet := Packet{SID: session.SID, DstMAC: fixture.server.hwAddr}
		copy(packet.SrcMAC[:], session.MAC)
		fixture.server.handlePADT(&packet)
		select {
		case <-transport.closed:
		default:
			t.Fatal("PADT did not gate its exact physical wire")
		}
		synctest.Wait()
		assertCoreAdmissionHeld(t, fixture, session, opened)
		down, ok := fixture.next(t).(ppp.EventSessionDown)
		if !ok || down.Control != control {
			t.Fatal("cancellation did not emit the original worker's genuine Down")
		}
		if downs != 0 {
			t.Fatal("held event reader nevertheless published shared Down")
		}
		fixture.sub.handlePPPEvent(down)
		if fixture.server.sessions.Lookup(session.SID) != nil {
			t.Fatal("genuine Down did not finish its same admission")
		}
		select {
		case <-session.closeDone:
		default:
			t.Fatal("genuine Down did not complete closeDone")
		}
		if downs != 1 {
			t.Fatal("genuine Down did not publish shared teardown exactly once")
		}
		successor, _, nextControl := fixture.admit(t, 81)
		if nextControl == control {
			t.Fatal("same-ID successor reused an old worker capability")
		}
		fixture.sub.handlePPPEvent(down)
		if fixture.server.sessions.Lookup(81) != successor {
			t.Fatal("late original Down removed successor admission")
		}
		if !nextControl.Active() {
			t.Fatal("late original Down canceled successor worker")
		}
		if _, found := subscriber.DefaultRegistry.Get(successor.SubscriberID); !found {
			t.Fatal("late original shared Down released successor state")
		}
		if downs != 1 {
			t.Fatal("late original Down republished against successor")
		}
	})
}

// TestCorePPPoEReloadRetainsServerUntilRealDown drives the real disabled
// Reload while its original Driver event is deliberately not delivered.
func TestCorePPPoEReloadRetainsServerUntilRealDown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newCoreRetentionFixture(t, 7)
		session, transport, control := fixture.admit(t, 83)
		opened := session.Opened
		fixture.sub.started = true
		fixture.sub.params = Parameters{Enabled: true, Interfaces: []InterfaceConfig{{Name: fixture.server.ifName}}}
		fixture.sub.readDone = make(chan struct{})
		close(fixture.sub.readDone)
		provider := zeconfig.NewProvider()
		provider.SetRoot(SubsystemName, map[string]any{"enabled": false})
		result := make(chan error, 1)
		go func() { result <- fixture.sub.Reload(context.Background(), provider) }()
		<-transport.closed
		synctest.Wait()
		assertCoreAdmissionHeld(t, fixture, session, opened)
		fixture.sub.mu.Lock()
		retained := fixture.sub.servers[7] == fixture.server
		fixture.sub.mu.Unlock()
		if !retained {
			t.Fatal("reload detached the original server before its terminal event")
		}
		probePeer := [EthALen]byte{2, 0, 0, 0, 0, 99}
		pado := Packet{Code: CodePADO, SrcMAC: fixture.server.hwAddr, Tags: []Tag{{Type: TagACCookie, Value: GenerateCookie(fixture.server.cookieKey, fixture.server.hwAddr[:], probePeer[:], nil)}}}
		var buffer [EthMaxLen]byte
		probe := BuildPADR(buffer[:], probePeer, &pado, "", nil)
		if probe == nil {
			t.Fatal("valid drain-period PADR was not encoded")
		}
		fixture.sub.dispatchFrame(7, probe)
		select {
		case err := <-result:
			t.Fatal("reload finished before its original closeDone", err)
		default:
		}
		down, ok := fixture.next(t).(ppp.EventSessionDown)
		if !ok || down.Control != control {
			t.Fatal("reload lost the exact original Down")
		}
		fixture.sub.handlePPPEvent(down)
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("reload did not finish after terminal consumption")
		}
		select {
		case <-session.closeDone:
		default:
			t.Fatal("reload detached an incomplete admission")
		}
		if fixture.sub.servers != nil {
			t.Fatal("disabled reload retained its fully drained server map")
		}
	})
}

// TestCorePPPoERefusedCleanupErrorKeepsReloadOwner observes a genuine rejected
// transfer whose actual owner Close failed. Nil Control cannot authorize release.
func TestCorePPPoERefusedCleanupErrorKeepsReloadOwner(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	synctest.Test(t, func(t *testing.T) {
		fixture := newCoreRetentionFixture(t, 7)
		cause := errors.New("owned refused lease close failed")
		session, transport := fixture.queue(t, 84, false, cause)
		rejected, ok := fixture.next(t).(ppp.EventSessionRejected)
		if !ok || rejected.Attempt != session.Attempt {
			t.Fatal("actual rejection lost incoming attempt")
		}
		status, err := session.Attempt.CleanupStatus()
		if err != nil {
			t.Fatal(err)
		}
		if status.Admitted || !status.Finished || !status.LeaseAcquired || status.Released || !errors.Is(status.Err, cause) {
			t.Fatal("actual refused lease cleanup did not remain unconfirmed", status, err)
		}
		fixture.sub.handlePPPEvent(rejected)
		if fixture.server.sessions.Lookup(84) != session {
			t.Fatal("rejected Close failure dropped admission")
		}
		fixture.sub.started = true
		fixture.sub.params = Parameters{Enabled: true, Interfaces: []InterfaceConfig{{Name: fixture.server.ifName}}}
		fixture.sub.readDone = make(chan struct{})
		close(fixture.sub.readDone)
		provider := zeconfig.NewProvider()
		provider.SetRoot(SubsystemName, map[string]any{"enabled": false})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err = fixture.sub.Reload(ctx, provider)
		if !errors.Is(err, cause) {
			t.Fatal("reload hid the actual refused cleanup cause", err)
		}
		if ctx.Err() != nil {
			t.Fatal("reload waited for its deadline instead of returning the known cleanup error")
		}
		select {
		case <-transport.closed:
		default:
			t.Fatal("refused admission did not gate its own wire")
		}
		if fixture.sub.servers[7] != fixture.server || fixture.server.sessions.Lookup(84) != session {
			t.Fatal("failed reload dropped its old retry owner")
		}
		if session.Opened == nil {
			t.Fatal("failed reload detached its original borrower")
		}
		select {
		case <-session.closeDone:
			t.Fatal("failed refused cleanup closed its completion receipt")
		default:
		}
		transport.owner.clearFailure()
	})
}

// TestCorePPPoEQueuedPADTRetainsRealRefusal cancels the actual queued attempt
// before its Driver starts. No admitted Control or protocol I/O is invented.
func TestCorePPPoEQueuedPADTRetainsRealRefusal(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	synctest.Test(t, func(t *testing.T) {
		fixture := newCoreRetentionFixtureStart(t, 7, false)
		session, transport := fixture.queue(t, 85, true, nil)
		opened := session.Opened
		packet := Packet{SID: 85, DstMAC: fixture.server.hwAddr}
		copy(packet.SrcMAC[:], session.MAC)
		fixture.server.handlePADT(&packet)
		if fixture.server.sessions.Lookup(85) != session || session.Opened != opened {
			t.Fatal("queued PADT dropped its original admission or borrower")
		}
		select {
		case <-session.closeDone:
			t.Fatal("queued PADT completed before actual refusal")
		default:
		}
		if _, admitted := session.Attempt.Control(); admitted {
			t.Fatal("unstarted Driver fabricated an admitted Control")
		}
		if err := fixture.driver.Start(); err != nil {
			t.Fatal(err)
		}
		fixture.started = true
		rejected, ok := fixture.next(t).(ppp.EventSessionRejected)
		if !ok || rejected.Attempt != session.Attempt {
			t.Fatal("queued cancellation did not emit its actual incoming refusal")
		}
		status, err := session.Attempt.CleanupStatus()
		if err != nil {
			t.Fatal(err)
		}
		if status.Admitted || !status.Finished || !status.LeaseAcquired || !status.Released || status.Err != nil {
			t.Fatal("queued refusal did not confirm its own acquired lease", status)
		}
		if transport.owner.writes.Load() != 0 {
			t.Fatal("canceled queued attempt performed protocol I/O")
		}
		fixture.sub.handlePPPEvent(rejected)
		if fixture.server.sessions.Lookup(85) != nil {
			t.Fatal("confirmed queued refusal retained its completed row")
		}
		select {
		case <-session.closeDone:
		default:
			t.Fatal("confirmed queued refusal did not finish closeDone")
		}
		if fixture.frames.Load() != 0 {
			t.Fatal("queued PADT was followed by another wire termination")
		}
	})
}

// TestCorePPPoEStopReconcilesWithoutTerminalDelivery never delivers the genuine
// Driver Down/refusal to the NAS, matching Stop's suppression case. The finished
// original Attempt observer must reconcile its row without event-only waiting.
func TestCorePPPoEStopReconcilesWithoutTerminalDelivery(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	for _, admitted := range []bool{false, true} {
		t.Run(map[bool]string{false: "refused", true: "admitted"}[admitted], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := newCoreRetentionFixture(t, 7)
				var session *Session
				if admitted {
					session, _, _ = fixture.admit(t, 86)
				} else {
					session, _ = fixture.queue(t, 86, false, nil)
					rejected, ok := fixture.next(t).(ppp.EventSessionRejected)
					if !ok || rejected.Attempt != session.Attempt {
						t.Fatal("actual refusal lost its original attempt")
					}
				}
				fixture.driver.Stop()
				if fixture.server.sessions.Lookup(86) != session {
					t.Fatal("Driver stop independently deleted NAS ownership")
				}
				fixture.sub.started = true
				if err := fixture.sub.Stop(context.Background()); err != nil {
					t.Fatal(err)
				}
				select {
				case <-session.closeDone:
				default:
					t.Fatal("stopped Attempt observer never completed original admission")
				}
				if fixture.server.sessions.Lookup(86) != nil {
					t.Fatal("stopped original admission remained in its table")
				}
				if _, found := subscriber.DefaultRegistry.Get(session.SubscriberID); found {
					t.Fatal("stopped original admission retained subscriber state")
				}
			})
		})
	}
}

// TestCorePPPoERetainedStopRetriesRecoveredOwner holds an actual refused lease
// error through Stop, then repairs that same backend and calls the same Stop.
// The existing Driver retry seam must run; merely observing status cannot pass.
func TestCorePPPoERetainedStopRetriesRecoveredOwner(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	synctest.Test(t, func(t *testing.T) {
		fixture := newCoreRetentionFixture(t, 7)
		cause := errors.New("retained stop owner close failed")
		session, transport := fixture.queue(t, 87, false, cause)
		rejected, ok := fixture.next(t).(ppp.EventSessionRejected)
		if !ok || rejected.Attempt != session.Attempt {
			t.Fatal("actual failed refusal changed its incoming attempt")
		}
		// Do not deliver the refusal: shutdown must observe its finished lease.
		fixture.sub.started = true
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := fixture.sub.Stop(ctx); !errors.Is(err, cause) {
			t.Fatal("first Stop hid its exact owner failure", err)
		}
		if ctx.Err() != nil {
			t.Fatal("first Stop waited for a nonexistent terminal event")
		}
		if fixture.sub.servers[7] != fixture.server || fixture.server.sessions.Lookup(87) != session || session.Opened == nil {
			t.Fatal("failed Stop dropped its original retry owner")
		}
		select {
		case <-session.closeDone:
			t.Fatal("failed Stop claimed completed admission")
		default:
		}
		transport.owner.clearFailure()
		if err := fixture.sub.Stop(context.Background()); err != nil {
			t.Fatal("same Stop did not retry the recovered original owner", err)
		}
		status, err := session.Attempt.CleanupStatus()
		if err != nil || !status.Released || status.Err != nil {
			t.Fatal("same retained Stop did not confirm actual retry", status, err)
		}
		select {
		case <-session.closeDone:
		default:
			t.Fatal("successful retained Stop left its original completion open")
		}
		if fixture.sub.servers != nil {
			t.Fatal("successful retained Stop kept a completed server generation")
		}
	})
}
