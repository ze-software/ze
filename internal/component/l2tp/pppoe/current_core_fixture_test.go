// Design: docs/architecture/l2tp/subscriber-session-model.md -- genuine current PPP fixture receipts
// Related: core_retention_test.go -- separate retained-admission proofs
package pppoe

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// fixtureCurrentOpened mints the transport's exact attempt before any transfer.
// It supplies no admitted Control: only the actual Driver may produce that.
func fixtureCurrentOpened(t *testing.T, server *InterfaceServer, session *Session, opened dataplane.OpenedSession) bool {
	t.Helper()
	if session == nil {
		return false
	}
	session.SubscriberID = pppoeSessionID(server.ifIndex, session.SID)
	session.AccessIfIndex = server.ifIndex
	attempt, err := ppp.NewSessionAttempt(session.SubscriberID, opened.Dataplane())
	if err != nil {
		t.Fatal(err)
	}
	return server.sessions.setOpened(session, opened, attempt)
}

// fixtureCurrentWire retains the old native and physical owner seams while
// the real Driver owns every admission, protocol wait and emitted capability.
// One bounded channel holds complete PPP frames; Close wakes the sole reader.
type fixtureCurrentWire struct {
	old        dataplane.OpenedSession
	input      chan []byte
	output     chan []byte
	done       chan struct{}
	closeCalls atomic.Int32
	once       sync.Once
}

func (wire *fixtureCurrentWire) Control() io.ReadWriter { return wire }
func (*fixtureCurrentWire) Opened() io.Reader           { return nil }
func (wire *fixtureCurrentWire) Read(buffer []byte) (int, error) {
	select {
	case frame := <-wire.input:
		return copy(buffer, frame), nil
	case <-wire.done:
		return 0, io.EOF
	}
}
func (wire *fixtureCurrentWire) Write(frame []byte) (int, error) {
	select {
	case wire.output <- append([]byte(nil), frame...):
		return len(frame), nil
	case <-wire.done:
		return 0, io.ErrClosedPipe
	}
}
func (wire *fixtureCurrentWire) InterruptRead() error {
	wire.once.Do(func() { close(wire.done) })
	return nil
}
func (wire *fixtureCurrentWire) Close() error {
	wire.closeCalls.Add(1)
	if err := wire.InterruptRead(); err != nil {
		return err
	}
	return wire.old.Dataplane().Close()
}
func (wire *fixtureCurrentWire) Identity() ppp.SessionIdentity {
	return wire.old.Dataplane().Identity()
}
func (*fixtureCurrentWire) SetMTU(int) error                                            { return nil }
func (*fixtureCurrentWire) Connect() error                                              { return nil }
func (*fixtureCurrentWire) Disconnect() error                                           { return nil }
func (*fixtureCurrentWire) SetReceiveMRU(uint16) error                                  { return nil }
func (*fixtureCurrentWire) SetAdminUp() error                                           { return nil }
func (*fixtureCurrentWire) AddAddressP2P(netip.Prefix, netip.Prefix) error              { return nil }
func (*fixtureCurrentWire) RemoveAddress(netip.Prefix) error                            { return nil }
func (*fixtureCurrentWire) AddRoute(netip.Prefix, netip.Addr, int, rtproto.Proto) error { return nil }
func (*fixtureCurrentWire) RemoveRoute(netip.Prefix, netip.Addr, int, rtproto.Proto) error {
	return nil
}
func (*fixtureCurrentWire) Counters() (ppp.SessionCounters, error) { return ppp.SessionCounters{}, nil }
func (*fixtureCurrentWire) OpenIPv6Carrier(context.Context) (ppp.IPv6Carrier, error) {
	return fixtureCurrentCarrier{}, nil
}

type fixtureCurrentCarrier struct{}

func (fixtureCurrentCarrier) ReadRS(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
func (fixtureCurrentCarrier) ReadDHCPv6(ctx context.Context, _ []byte) (int, netip.AddrPort, error) {
	<-ctx.Done()
	return 0, netip.AddrPort{}, ctx.Err()
}
func (fixtureCurrentCarrier) WriteRA([]byte, int) error { return nil }
func (fixtureCurrentCarrier) WriteDHCPv6(data []byte, _ netip.AddrPort) (int, error) {
	return len(data), nil
}
func (fixtureCurrentCarrier) Close() error { return nil }

type fixtureCurrentTransport struct{ opened dataplane.OpenedSession }

func (transport fixtureCurrentTransport) Close() error { return transport.opened.CloseTransport() }
func (transport fixtureCurrentTransport) RelayDescriptor() (int, error) {
	var descriptor int
	err := transport.opened.WithDescriptor(func(value int) error { descriptor = value; return nil })
	return descriptor, err
}

type fixtureCurrentPPP struct {
	driver  *ppp.Driver
	wire    *fixtureCurrentWire
	session *Session
	control ppp.SessionControl
	method  ppp.AuthMethod
	dual    bool
	packets map[uint16][]ppp.LCPPacket
}

type fixtureCurrentStartConfig struct {
	method ppp.AuthMethod
	dual   bool
}

type fixtureCurrentAuthConfig struct {
	username string
}

// fixtureCurrentStart attaches one exact existing transport row to a genuine
// Driver. Callers MUST retain this returned owner across teardown/replacement.
func fixtureCurrentStart(t *testing.T, server *InterfaceServer, session *Session, input *fixtureCurrentStartConfig) *fixtureCurrentPPP {
	t.Helper()
	config := *input
	method, dual := config.method, config.dual
	old := session.Opened
	if old == nil {
		old = fixtureOpened(t, "fixture-current")
	}
	wire := &fixtureCurrentWire{old: old, input: make(chan []byte, 32), output: make(chan []byte, 32), done: make(chan struct{})}
	opened, err := dataplane.NewOpenedSession(wire, wire, fixtureCurrentTransport{old})
	if err != nil {
		t.Fatal(err)
	}
	if !fixtureCurrentOpened(t, server, session, opened) {
		t.Fatal("fixture failed to bind exact created attempt")
	}
	fixture := &fixtureCurrentPPP{driver: ppp.NewProductionDriver(slog.Default()), wire: wire, session: session, method: method, dual: dual, packets: make(map[uint16][]ppp.LCPPacket)}
	if err := fixture.driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Attempt.Cancel(); fixture.driver.Stop() })
	options := []byte{1, 4, 5, 220}
	// Peer MRU stays 1500; the local proxy MRU must meet the 1492 session cap.
	local := []byte{1, 4, 5, 212}
	if method == ppp.AuthMethodPAP {
		local = append(local, 3, 4, 0xc0, 0x23)
	}
	if method == ppp.AuthMethodCHAPMD5 {
		local = append(local, 3, 5, 0xc2, 0x23, 5)
	}
	if server.sessions.takePPPLease(session) == nil {
		t.Fatal("fixture failed to transfer its exact lease")
	}
	lcp, err := ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	fixture.driver.SessionsIn() <- ppp.StartSession{LCP: lcp, SubscriberID: session.SubscriberID, Attempt: session.Attempt, TunnelID: uint16(server.ifIndex), SessionID: session.SID, Frames: wire, Dataplane: wire, LNSMode: true, MaxMRU: 1492, AuthMethod: method, AuthRequired: method != ppp.AuthMethodNone, AuthFallbackOrder: []ppp.AuthMethod{method}, DisableIPCP: !dual, DisableIPv6CP: !dual, AuthTimeout: time.Minute, IPTimeout: time.Minute, ProxyLCPInitialRecv: options, ProxyLCPLastRecv: options, ProxyLCPLastSent: local}
	openedEvent, ok := fixture.next(t).(ppp.EventLCPUp)
	if !ok {
		t.Fatal("fixture did not observe genuine LCPUp")
	}
	fixture.control = openedEvent.Control
	return fixture
}

// fixtureCurrentRealmRequest admits the original transport and obtains its
// realm credential through a real LCP exchange and the selected auth wire.
// The returned provider request is never rewritten after the Driver emits it.
func fixtureCurrentRealmRequest(t *testing.T, server *InterfaceServer, session *Session, method ppp.AuthMethod, username string) ppp.EventAuthRequest {
	t.Helper()
	old := session.Opened
	if old == nil {
		t.Fatal("realm fixture has no original transport")
	}
	wire := &fixtureCurrentWire{old: old, input: make(chan []byte, 32), output: make(chan []byte, 32), done: make(chan struct{})}
	opened, err := dataplane.NewOpenedSession(wire, wire, fixtureCurrentTransport{old})
	if err != nil {
		t.Fatal(err)
	}
	if !fixtureCurrentOpened(t, server, session, opened) {
		t.Fatal("realm fixture failed to bind exact created attempt")
	}
	fixture := &fixtureCurrentPPP{driver: ppp.NewProductionDriver(slog.Default()), wire: wire, session: session, method: method, packets: make(map[uint16][]ppp.LCPPacket)}
	if err := fixture.driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Attempt.Cancel(); fixture.driver.Stop() })
	if server.sessions.takePPPLease(session) == nil {
		t.Fatal("realm fixture failed to transfer its exact lease")
	}
	lcp, err := ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	fixture.driver.SessionsIn() <- ppp.StartSession{LCP: lcp, SubscriberID: session.SubscriberID, Attempt: session.Attempt, TunnelID: uint16(server.ifIndex), SessionID: session.SID, Frames: wire, Dataplane: wire, LNSMode: true, PPPoE: true, MaxMRU: 1492, AuthMethod: method, AuthRequired: true, AuthFallbackOrder: []ppp.AuthMethod{method}, DisableIPCP: true, DisableIPv6CP: true, AuthTimeout: time.Minute, IPTimeout: time.Minute, CaptureProxyLCP: true}
	local := fixture.read(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
	options, err := ppp.ParseLCPOptions(local.Data)
	if err != nil {
		t.Fatal(err)
	}
	wantAuth := []byte{0xc0, 0x23}
	if method == ppp.AuthMethodCHAPMD5 {
		wantAuth = []byte{0xc2, 0x23, 5}
	}
	mruSeen, authSeen := false, false
	for _, option := range options {
		if option.Type == ppp.LCPOptMRU {
			mruSeen = bytes.Equal(option.Data, []byte{5, 212})
		}
		if option.Type == ppp.LCPOptAuthProto {
			authSeen = bytes.Equal(option.Data, wantAuth)
		}
	}
	if !mruSeen {
		t.Fatal("realm fixture local LCP omitted MRU 1492")
	}
	if !authSeen {
		t.Fatal("realm fixture local LCP changed its authentication method")
	}
	fixture.send(t, ppp.ProtoLCP, ppp.LCPConfigureAck, local.Identifier, local.Data)
	initial := []byte{1, 4, 5, 220}
	fixture.send(t, ppp.ProtoLCP, ppp.LCPConfigureRequest, 0x31, initial)
	nak := fixture.read(t, ppp.ProtoLCP, ppp.LCPConfigureNak)
	if nak.Identifier != 0x31 {
		t.Fatal("realm fixture Nak did not answer the original peer request")
	}
	if !bytes.Equal(nak.Data, []byte{1, 4, 5, 212}) {
		t.Fatal("realm fixture Nak did not suggest MRU 1492")
	}
	fixture.send(t, ppp.ProtoLCP, ppp.LCPConfigureRequest, 0x32, nak.Data)
	ack := fixture.read(t, ppp.ProtoLCP, ppp.LCPConfigureAck)
	if ack.Identifier != 0x32 {
		t.Fatal("realm fixture Ack did not answer the final peer request")
	}
	if !bytes.Equal(ack.Data, nak.Data) {
		t.Fatal("realm fixture Ack changed the final peer options")
	}
	up, ok := fixture.next(t).(ppp.EventLCPUp)
	if !ok {
		t.Fatal("realm fixture did not observe genuine LCPUp")
	}
	fixture.control = up.Control
	request := fixture.request(t, username)
	if request.ProxyLCP == nil {
		t.Fatal("actual realm request omitted captured LCP exchange")
	}
	if !bytes.Equal(request.ProxyLCP.InitialReceived, initial) {
		t.Fatal("actual realm capture lost the original peer MRU 1500 request")
	}
	if !bytes.Equal(request.ProxyLCP.LastReceived, nak.Data) {
		t.Fatal("actual realm capture lost the final acknowledged peer request")
	}
	if !bytes.Equal(request.ProxyLCP.LastSent, local.Data) {
		t.Fatal("actual realm capture lost the acknowledged local request")
	}
	return request
}

func (fixture *fixtureCurrentPPP) next(t *testing.T) ppp.Event {
	t.Helper()
	select {
	case event := <-fixture.driver.EventsOut():
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("actual Driver lifecycle event missing")
		return nil
	}
}
func (fixture *fixtureCurrentPPP) send(t *testing.T, protocol uint16, code, id uint8, data []byte) {
	t.Helper()
	var buffer [4096]byte
	offset := ppp.WriteFrame(buffer[:], 0, protocol, nil)
	offset += ppp.WriteLCPPacket(buffer[:], offset, code, id, data)
	select {
	case fixture.wire.input <- append([]byte(nil), buffer[:offset]...):
	case <-time.After(5 * time.Second):
		t.Fatal("actual Driver frame input blocked")
	}
}
func (fixture *fixtureCurrentPPP) read(t *testing.T, protocol uint16, code uint8) ppp.LCPPacket {
	t.Helper()
	for range 32 {
		for index, packet := range fixture.packets[protocol] {
			if packet.Code == code {
				fixture.packets[protocol] = append(fixture.packets[protocol][:index], fixture.packets[protocol][index+1:]...)
				return packet
			}
		}
		select {
		case frame := <-fixture.wire.output:
			proto, payload, _, err := ppp.ParseFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := ppp.ParseLCPPacket(payload)
			if err != nil {
				t.Fatal(err)
			}
			fixture.packets[proto] = append(fixture.packets[proto], packet)
		case <-time.After(5 * time.Second):
			t.Fatal("actual Driver omitted required PPP reply")
		}
	}
	t.Fatal("too many unrelated PPP frames")
	return ppp.LCPPacket{}
}
func (fixture *fixtureCurrentPPP) request(t *testing.T, username string) ppp.EventAuthRequest {
	t.Helper()
	switch fixture.method {
	case ppp.AuthMethodNone:
	case ppp.AuthMethodPAP:
		data := append([]byte{byte(len(username))}, []byte(username)...)
		data = append(data, 1, 'p')
		fixture.send(t, ppp.ProtoPAP, ppp.PAPAuthenticateRequest, 7, data)
	case ppp.AuthMethodCHAPMD5:
		challenge := fixture.read(t, ppp.ProtoCHAP, 1)
		data := make([]byte, 17)
		data[0] = 16
		data = append(data, []byte(username)...)
		fixture.send(t, ppp.ProtoCHAP, 2, challenge.Identifier, data)
	case ppp.AuthMethodMSCHAPv2:
		t.Fatal("unsupported fixture authentication method")
	default:
		t.Fatal("unsupported fixture authentication method")
	}
	for range 8 {
		select {
		case event := <-fixture.driver.AuthEventsOut():
			if request, ok := event.(ppp.EventAuthRequest); ok {
				if request.Control != fixture.control || !request.Active() {
					t.Fatal("actual auth request lost its admitted owner")
				}
				return request
			}
		case <-time.After(5 * time.Second):
			t.Fatal("actual auth request missing")
		}
	}
	t.Fatal("too many non-request auth events")
	return ppp.EventAuthRequest{}
}
func (fixture *fixtureCurrentPPP) authenticate(t *testing.T, input *fixtureCurrentAuthConfig) {
	t.Helper()
	config := *input
	request := fixture.request(t, config.username)
	if err := fixture.driver.AuthResponse(request, true, "fixture accepted", nil); err != nil {
		t.Fatal(err)
	}
	if fixture.method == ppp.AuthMethodPAP {
		fixture.read(t, ppp.ProtoPAP, ppp.PAPAuthenticateAck)
	}
	if fixture.method == ppp.AuthMethodCHAPMD5 {
		fixture.read(t, ppp.ProtoCHAP, 3)
	}
}
func (fixture *fixtureCurrentPPP) up(t *testing.T) ppp.EventSessionUp {
	t.Helper()
	up, ok := fixture.next(t).(ppp.EventSessionUp)
	if !ok {
		t.Fatal("actual Driver omitted SessionUp")
	}
	return up
}
func (fixture *fixtureCurrentPPP) down(t *testing.T) ppp.EventSessionDown {
	t.Helper()
	fixture.session.Attempt.Cancel()
	down, ok := fixture.next(t).(ppp.EventSessionDown)
	if !ok {
		t.Fatal("actual Driver omitted terminal Down")
	}
	return down
}
