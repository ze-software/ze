// Design: docs/architecture/l2tp/subscriber-session-model.md -- genuine current-core PPP fixture receipts
// Related: session_event_order_test.go -- historical order/timeout assertions

package l2tp

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// Only native operations are substituted; Driver and LCP/PAP/NCP are real.
// The embedded pipe owner is idempotent; callers never close borrowed streams.
type currentCoreHandle struct {
	net.Conn
	identity    ppp.SessionIdentity
	counterMu   sync.Mutex
	counterRead func() (ppp.SessionCounters, error)
}

func (h *currentCoreHandle) Control() io.ReadWriter                                       { return h.Conn }
func (*currentCoreHandle) Opened() io.Reader                                              { return nil }
func (h *currentCoreHandle) InterruptRead() error                                         { return h.SetReadDeadline(time.Now()) }
func (h *currentCoreHandle) Identity() ppp.SessionIdentity                                { return h.identity }
func (*currentCoreHandle) SetMTU(int) error                                               { return nil }
func (*currentCoreHandle) Connect() error                                                 { return nil }
func (*currentCoreHandle) Disconnect() error                                              { return nil }
func (*currentCoreHandle) SetReceiveMRU(uint16) error                                     { return nil }
func (*currentCoreHandle) SetAdminUp() error                                              { return nil }
func (*currentCoreHandle) AddAddressP2P(netip.Prefix, netip.Prefix) error                 { return nil }
func (*currentCoreHandle) RemoveAddress(netip.Prefix) error                               { return nil }
func (*currentCoreHandle) AddRoute(netip.Prefix, netip.Addr, int, rtproto.Proto) error    { return nil }
func (*currentCoreHandle) RemoveRoute(netip.Prefix, netip.Addr, int, rtproto.Proto) error { return nil }

// Counters keeps the existing explicit modeled zero unless the question installs
// a controlled native read. The gate is copied under its own mutex and invoked
// outside it, so no fixture lock can hide the actor/Driver lock behavior.
func (handle *currentCoreHandle) Counters() (ppp.SessionCounters, error) {
	handle.counterMu.Lock()
	read := handle.counterRead
	handle.counterMu.Unlock()
	if read != nil {
		return read()
	}
	return ppp.SessionCounters{}, nil
}

// setCounterRead is test-only native I/O control. It grants no admission or
// physical authority; the existing Driver still mints the actual Control.
func (handle *currentCoreHandle) setCounterRead(read func() (ppp.SessionCounters, error)) {
	handle.counterMu.Lock()
	handle.counterRead = read
	handle.counterMu.Unlock()
}
func (*currentCoreHandle) OpenIPv6Carrier(context.Context) (ppp.IPv6Carrier, error) {
	return currentCoreIPv6Carrier{}, nil
}

// Native IPv6 receive workers obey their actual cancellation and join protocol.
// No extra reader borrows the PPP stream, and final RA writes remain available.
type currentCoreIPv6Carrier struct{}

func (currentCoreIPv6Carrier) ReadRS(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
func (currentCoreIPv6Carrier) WriteRA([]byte, int) error        { return nil }
func (currentCoreIPv6Carrier) ReadDHCPv6(ctx context.Context, _ []byte) (int, netip.AddrPort, error) {
	<-ctx.Done()
	return 0, netip.AddrPort{}, ctx.Err()
}
func (currentCoreIPv6Carrier) WriteDHCPv6(payload []byte, _ netip.AddrPort) (int, error) {
	return len(payload), nil
}
func (currentCoreIPv6Carrier) Close() error { return nil }

type currentCoreWire struct {
	driver         *ppp.Driver
	startSession   ppp.StartSession
	peer           net.Conn
	opened         ppp.EventLCPUp
	assignment     ppp.EventSessionIPAssigned
	auth           ppp.EventAuthRequest
	pool           ppp.EventIPRequest
	ipv6           bool
	configurations map[ppp.AddressFamily]ppp.LCPPacket
	answers        map[ppp.AddressFamily]ppp.IPResponseArgs
	release        func() // MUST open a held reader gate before cleanup joins paths.
	paths          []<-chan struct{}
}

type currentCoreSessionIDs struct {
	tunnelID, sessionID uint16
}

type currentCoreAdmissionConfig struct {
	ids          currentCoreSessionIDs
	ipv6         bool
	subscriberID string
}

// newCurrentCoreAdmission retains one real Attempt in the preexisting NAS
// session and returns at the actual LCP-up/authentication boundary. No setup
// notice or order ticket reaches the readers a consumer test is measuring.
// The test MUST consume/answer requests or close the peer; cleanup does the latter.
// With a nil reactor this is a protocol-only fixture: an explicitly supplied
// opaque ID remains opaque and carries no claim of an L2TP admission birth.
func newCurrentCoreAdmission(t *testing.T, r *l2tpReactor, input *currentCoreAdmissionConfig) *currentCoreWire {
	t.Helper()
	config := *input
	tid, sid := config.ids.tunnelID, config.ids.sessionID
	ipv6, id := config.ipv6, config.subscriberID
	protocolOnly := r == nil
	if r == nil {
		session := &L2TPSession{localSID: sid, subscriberID: id}
		if id == "" {
			newFixtureL2TPSession(t, session)
		}
		r = &l2tpReactor{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), tunnelsByLocalID: map[uint16]*L2TPTunnel{
			tid: {localTID: tid, sessions: map[uint16]*L2TPSession{sid: session}},
		}}
	}
	r.tunnelsMu.Lock()
	tunnel := r.tunnelsByLocalID[tid]
	if tunnel == nil {
		r.tunnelsMu.Unlock()
		t.Fatal("core fixture has no NAS tunnel")
	}
	session := tunnel.lookupSession(sid)
	if session == nil {
		r.tunnelsMu.Unlock()
		t.Fatal("core fixture has no NAS session")
	}
	if session.pppAttempt != (ppp.SessionAttempt{}) {
		r.tunnelsMu.Unlock()
		t.Fatal("core fixture cannot replace an existing attempt")
	}
	if !protocolOnly {
		if !session.birth.IsValid() {
			r.tunnelsMu.Unlock()
			t.Fatal("core fixture actor was published without an explicit valid birth")
		}
		if session.subscriberID == "" {
			r.tunnelsMu.Unlock()
			t.Fatal("core fixture actor was published without its cached subscriber ID")
		}
	}
	ifname := session.pppInterface
	birth := session.birth
	originalID := session.subscriberID
	r.tunnelsMu.Unlock()
	if id == "" {
		id = originalID
	}
	if !protocolOnly {
		if id != originalID {
			t.Fatal("core fixture cannot change an actor's original subscriber ID")
		}
	}
	lcp, err := ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	native, peer := net.Pipe()
	handle := &currentCoreHandle{Conn: native, identity: ppp.SessionIdentity{InterfaceName: ifname, NativeIndex: uint32(sid)}}
	driver := ppp.NewProductionDriver(r.logger)
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	wire := &currentCoreWire{driver: driver, peer: peer, ipv6: ipv6,
		configurations: make(map[ppp.AddressFamily]ppp.LCPPacket), answers: make(map[ppp.AddressFamily]ppp.IPResponseArgs)}
	t.Cleanup(func() {
		if wire.release != nil {
			wire.release()
		}
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		driver.Stop()
		for _, done := range wire.paths {
			awaitOrderEvent(t, done, "the controlled current-core path during cleanup")
		}
		r.tunnelsMu.Lock()
		cancelSessionTimeouts(session)
		r.tunnelsMu.Unlock()
		ClearSessionMetadata(id)
	})
	attempt, err := ppp.NewSessionAttempt(id, handle)
	if err != nil {
		t.Fatal(err)
	}
	r.tunnelsMu.Lock()
	session.pppAttempt = attempt
	r.tunnelsMu.Unlock()
	options := []byte{1, 4, 5, 220, 3, 4, 0xc0, 0x23}
	wire.startSession = ppp.StartSession{
		LCP: lcp, TunnelID: tid, SessionID: sid, L2TPBirth: birth, SubscriberID: id, Attempt: attempt,
		Frames: handle, Dataplane: handle, LNSMode: true, DisableIPv6CP: !ipv6,
		AuthMethod: ppp.AuthMethodPAP, AuthFallbackOrder: []ppp.AuthMethod{ppp.AuthMethodPAP},
		AuthTimeout: time.Hour, IPTimeout: time.Hour, EchoInterval: time.Hour,
		ProxyLCPInitialRecv: options, ProxyLCPLastSent: options, ProxyLCPLastRecv: options,
	}
	driver.SessionsIn() <- wire.startSession
	opened, valid := wire.event(t).(ppp.EventLCPUp)
	if !valid {
		t.Fatal("actual core Driver omitted LCP-up")
	}
	if opened.Control.Attempt() != attempt {
		t.Fatal("actual core Driver changed admission")
	}
	wire.opened = opened
	return wire
}

// Default ordering inputs remain the original grace / 192.0.2.7 IPCP values.
func newCurrentCoreWire(t *testing.T, r *l2tpReactor, input *currentCoreSessionIDs) *currentCoreWire {
	t.Helper()
	ids := *input
	tid, sid := ids.tunnelID, ids.sessionID
	wire := newCurrentCoreAdmission(t, r, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: tid, sessionID: sid}, ipv6: false, subscriberID: ""})
	wire.prepareNetwork(t, "grace", ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.7")}, [8]byte{})
	wire.assignment = wire.assign(t, ppp.AddressFamilyIPv4)
	return wire
}
func (w *currentCoreWire) event(t *testing.T) ppp.Event {
	t.Helper()
	select {
	case event, open := <-w.driver.EventsOut():
		if !open {
			t.Fatal("actual core Driver ended before its receipt")
		}
		return event
	case <-time.After(orderTestBound):
		t.Fatal("actual core Driver omitted lifecycle receipt")
		return nil
	}
}
func (w *currentCoreWire) authRequest(t *testing.T, username string) ppp.EventAuthRequest {
	t.Helper()
	payload := append([]byte{byte(len(username))}, username...)
	payload = append(payload, 2, 'p', 'w')
	w.write(t, ppp.ProtoPAP, ppp.PAPAuthenticateRequest, 0x61, payload)
	select {
	case event := <-w.driver.AuthEventsOut():
		request, ok := event.(ppp.EventAuthRequest)
		if !ok {
			t.Fatalf("actual core auth event=%T", event)
		}
		if request.Username != username {
			t.Fatal("actual PAP changed the test principal")
		}
		w.auth = request
		return request
	case <-time.After(orderTestBound):
		t.Fatal("actual core Driver omitted auth request")
		return ppp.EventAuthRequest{}
	}
}
func (w *currentCoreWire) poolRequest(t *testing.T) ppp.EventIPRequest {
	t.Helper()
	select {
	case event := <-w.driver.IPEventsOut():
		request, ok := event.(ppp.EventIPRequest)
		if !ok {
			t.Fatalf("actual core pool event=%T", event)
		}
		w.pool = request
		return request
	case <-time.After(orderTestBound):
		t.Fatal("actual core Driver omitted pool request")
		return ppp.EventIPRequest{}
	}
}
func (w *currentCoreWire) acceptAuth(t *testing.T, request ppp.EventAuthRequest) {
	t.Helper()
	if err := w.driver.AuthResponse(request, true, "", nil); err != nil {
		t.Fatal(err)
	}
	w.read(t, ppp.ProtoPAP, ppp.PAPAuthenticateAck)
	// AuthEventsOut also carries the actual consumed decision's private outcome.
	// PAP ACK plus this observation fences acceptance and leaves no stale result
	// ahead of the next lifetime's request on the same worker.
	select {
	case raw := <-w.driver.AuthEventsOut():
		if _, request := raw.(ppp.EventAuthRequest); request {
			t.Fatal("actual auth outcome was replaced by another request")
		}
	case <-time.After(orderTestBound):
		t.Fatal("actual Driver omitted consumed auth outcome")
	}
}
func (w *currentCoreWire) prepareNetwork(t *testing.T, username string, v4 ppp.IPResponseArgs, iid [8]byte) {
	t.Helper()
	w.acceptAuth(t, w.authRequest(t, username))
	request := w.poolRequest(t)
	if request.Family != ppp.AddressFamilyIPv4 {
		t.Fatal("actual core first pool request is not IPv4")
	}
	if err := w.driver.IPResponse(request, v4); err != nil {
		t.Fatal(err)
	}
	w.answers[ppp.AddressFamilyIPv4] = v4
	w.configurations[ppp.AddressFamilyIPv4] = w.read(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
	if w.ipv6 {
		request = w.poolRequest(t)
		if request.Family != ppp.AddressFamilyIPv6 {
			t.Fatal("actual core second pool request is not IPv6")
		}
		answer := ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv6, PeerInterfaceID: iid, HasPeerInterface: true}
		if err := w.driver.IPResponse(request, answer); err != nil {
			t.Fatal(err)
		}
		w.answers[ppp.AddressFamilyIPv6] = answer
		w.configurations[ppp.AddressFamilyIPv6] = w.read(t, ppp.ProtoIPv6CP, ppp.LCPConfigureRequest)
	}
}
func (w *currentCoreWire) assign(t *testing.T, family ppp.AddressFamily) ppp.EventSessionIPAssigned {
	t.Helper()
	protocol := ppp.ProtoIPCP
	answer := w.answers[family]
	var options []byte
	if family == ppp.AddressFamilyIPv4 {
		address := answer.Peer.As4()
		options = append([]byte{ppp.IPCPOptIPAddress, 6}, address[:]...)
	} else {
		protocol = ppp.ProtoIPv6CP
		options = append([]byte{ppp.IPv6CPOptInterfaceID, 10}, answer.PeerInterfaceID[:]...)
	}
	configure := w.configurations[family]
	w.write(t, protocol, ppp.LCPConfigureRequest, 0x62, options)
	w.read(t, protocol, ppp.LCPConfigureAck)
	w.write(t, protocol, ppp.LCPConfigureAck, configure.Identifier, configure.Data)
	assignment, ok := w.event(t).(ppp.EventSessionIPAssigned)
	if !ok {
		t.Fatal("actual core NCP omitted assignment")
	}
	if assignment.Family != family {
		t.Fatal("actual core assignment changed family")
	}
	return assignment
}
func (w *currentCoreWire) nextUp(t *testing.T) ppp.EventSessionUp {
	t.Helper()
	event, ok := w.event(t).(ppp.EventSessionUp)
	if !ok {
		t.Fatal("actual core network omitted Up")
	}
	return event
}
func (w *currentCoreWire) up(t *testing.T) ppp.EventSessionUp {
	t.Helper()
	// Protocol-only setup acknowledges the actual prior receipt without a NAS
	// setup publication or ticket that could satisfy the tested consumer path.
	w.assignment.Acknowledge()
	return w.nextUp(t)
}
func (w *currentCoreWire) withdraw(t *testing.T) ppp.EventLCPDown {
	t.Helper()
	w.up(t)
	return w.beginRestart(t)
}
func (w *currentCoreWire) beginRestart(t *testing.T) ppp.EventLCPDown {
	t.Helper()
	w.write(t, ppp.ProtoLCP, ppp.LCPConfigureRequest, 0x63, []byte{1, 4, 5, 220})
	for {
		raw := w.event(t)
		if _, up := raw.(ppp.EventSessionUp); up {
			continue
		} // protocol-only setup did not publish an earlier Up
		event, ok := raw.(ppp.EventLCPDown)
		if !ok {
			t.Fatal("actual core LCP restart omitted withdrawal")
		}
		return event
	}
}

// finishRestart MUST follow the actual consumer ACK of beginRestart. A single
// worker and immutable Attempt survive both protocol/auth/network lifetimes.
func (w *currentCoreWire) finishRestart(t *testing.T) {
	t.Helper()
	configure := w.read(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
	w.read(t, ppp.ProtoLCP, ppp.LCPConfigureAck)
	w.write(t, ppp.ProtoLCP, ppp.LCPConfigureAck, configure.Identifier, configure.Data)
	event, ok := w.event(t).(ppp.EventLCPUp)
	if !ok {
		t.Fatal("actual core LCP restart omitted reopened receipt")
	}
	if event.Control != w.opened.Control {
		t.Fatal("LCP restart replaced original worker")
	}
	w.opened = event
}
func (w *currentCoreWire) peerDown(t *testing.T) ppp.EventSessionDown {
	t.Helper()
	w.up(t)
	return w.finishPeerDown(t)
}
func (w *currentCoreWire) finishPeerDown(t *testing.T) ppp.EventSessionDown {
	t.Helper()
	w.write(t, ppp.ProtoLCP, ppp.LCPTerminateRequest, 0x64, nil)
	withdrawal, ok := w.event(t).(ppp.EventLCPDown)
	if !ok {
		t.Fatal("actual core peer termination omitted withdrawal")
	}
	// Protocol-only withdrawal is prerequisite to the one terminal notice being
	// tested, not another NAS teardown that could hide a missing Down handler.
	withdrawal.Acknowledge()
	w.read(t, ppp.ProtoLCP, ppp.LCPTerminateAck)
	event, ok := w.event(t).(ppp.EventSessionDown)
	if !ok {
		t.Fatal("actual core peer termination omitted Down")
	}
	if event.Cause != l2tpevents.TerminateCauseUserRequest {
		t.Fatal("actual peer termination changed cause")
	}
	return event
}
func (w *currentCoreWire) read(t *testing.T, protocol uint16, code uint8) ppp.LCPPacket {
	t.Helper()
	if err := w.peer.SetReadDeadline(time.Now().Add(orderTestBound)); err != nil {
		t.Fatal(err)
	}
	var frame [1504]byte
	n, err := w.peer.Read(frame[:])
	if err != nil {
		t.Fatal(err)
	}
	actual, payload, _, err := ppp.ParseFrame(frame[:n])
	if err != nil {
		t.Fatal(err)
	}
	if actual != protocol {
		t.Fatalf("actual core protocol=%x want%x", actual, protocol)
	}
	packet, err := ppp.ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Code != code {
		t.Fatalf("actual core code=%d want%d", packet.Code, code)
	}
	return packet
}
func (w *currentCoreWire) write(t *testing.T, protocol uint16, code, id uint8, data []byte) {
	t.Helper()
	if err := w.peer.SetWriteDeadline(time.Now().Add(orderTestBound)); err != nil {
		t.Fatal(err)
	}
	var frame [1504]byte
	n := ppp.WriteFrame(frame[:], 0, protocol, nil)
	n += ppp.WriteLCPPacket(frame[:], n, code, id, data)
	if _, err := w.peer.Write(frame[:n]); err != nil {
		t.Fatal(err)
	}
}
func (w *currentCoreWire) start(path func()) <-chan struct{} {
	done := startOrderPath(path)
	w.paths = append(w.paths, done)
	return done
}

// duplicateRejection requeues the SAME original handoff. The NAS still holds
// that Attempt, so removing the ExistingSession preservation branch would
// reach a matching Attempt and incorrectly tear down the admitted transport.
func (w *currentCoreWire) duplicateRejection(t *testing.T) ppp.EventSessionRejected {
	t.Helper()
	w.driver.SessionsIn() <- w.startSession
	event, ok := w.event(t).(ppp.EventSessionRejected)
	if !ok {
		t.Fatal("actual duplicate admission omitted rejection")
	}
	if !event.ExistingSession {
		t.Fatal("actual duplicate did not preserve the existing admission")
	}
	return event
}

// Reusing another canonical subscriber's live lease is the original new-call
// refusal class. The existing worker stays real and owns that lease throughout.
func currentCoreNewRejection(t *testing.T, r *l2tpReactor, tid, sid uint16) ppp.EventSessionRejected {
	t.Helper()
	wire := newCurrentCoreAdmission(t, nil, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: tid, sessionID: sid}, ipv6: false, subscriberID: "core-original-lease-owner"})
	r.tunnelsMu.Lock()
	session := r.tunnelsByLocalID[tid].lookupSession(sid)
	birth, id := session.Birth(), session.SubscriberID()
	r.tunnelsMu.Unlock()
	start := wire.startSession
	attempt, err := ppp.NewSessionAttempt(id, start.Dataplane)
	if err != nil {
		t.Fatal(err)
	}
	r.tunnelsMu.Lock()
	session.pppAttempt = attempt
	r.tunnelsMu.Unlock()
	start.L2TPBirth, start.SubscriberID, start.Attempt = birth, id, attempt
	wire.driver.SessionsIn() <- start
	event, ok := wire.event(t).(ppp.EventSessionRejected)
	if !ok {
		t.Fatal("actual reused-lease admission omitted rejection")
	}
	if event.ExistingSession {
		t.Fatal("fresh refused canonical ID was reported as an existing key")
	}
	if event.Attempt != attempt {
		t.Fatal("actual refusal lost original Attempt")
	}
	return event
}
