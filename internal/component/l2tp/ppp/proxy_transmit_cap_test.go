// Design: docs/guide/l2tp.md -- accepted proxy LCP retains peer MRU while local policy caps transmission.

package ppp

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

const proxyTransmitPeerMagic uint32 = 0x11223344

// The proxy is admitted before the test completes real PAP and IPCP exchanges.
// Observing EventSessionUp fences the backend MTU write after both phases;
// inspecting LCPUp alone would leave the defective installation untested.
func TestProxyTransmitMRUCap(t *testing.T) {
	for _, tc := range []struct {
		name       string
		peer       uint16
		negotiated uint16
		mtu        int
	}{
		{"smaller peer", 1400, 1400, 1400},
		{"equal peer", 1492, 1492, 1492},
		{"default peer", 1500, 1500, 1492},
		{"jumbo peer", 9000, 9000, 1492},
		{"omitted peer", 0, 1500, 1492},
	} {
		t.Run(tc.name, func(t *testing.T) {
			td := startProxyTransmitCapDriver(t, tc.peer)
			assertProxyTransmitNegotiation(t, td.driver, tc.negotiated)
			completeProxyTransmitNetwork(t, td)
			assertProxyTransmitInterface(t, td, tc.mtu, 1)
			assertProxyTransmitEcho(t, td, proxyTransmitPeerMagic)
		})
	}
}

// Once accepted proxy LCP ends, the same session runs normal LCP. An omitted
// replacement peer MRU still means 1500 in the event, but must not undo the
// admitted cap when authentication and IPCP configure the interface again.
func TestProxyTransmitCapAfterLCPRestart(t *testing.T) {
	td := startProxyTransmitCapDriver(t, 9000)
	assertProxyTransmitNegotiation(t, td.driver, 9000)
	completeProxyTransmitNetwork(t, td)
	assertProxyTransmitInterface(t, td, 1492, 1)

	writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureRequest, 0x41, nil)
	ev := waitForEvent(t, td.driver.EventsOut(), 5*time.Second)
	down, ok := ev.(EventLCPDown)
	if !ok || !down.NetworkPhase {
		t.Fatalf("restart event = %+v, want network lifetime withdrawal", ev)
	}
	down.Acknowledge()
	request := readLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureRequest)
	opts, err := ParseLCPOptions(request.Data)
	if err != nil {
		t.Fatal(err)
	}
	if mru, present := lookupMRUOption(opts); !present || mru != 1492 {
		t.Fatalf("replacement local MRU = %d, present=%t; want admitted cap 1492", mru, present)
	}
	auth, present := lookupOption(opts, LCPOptAuthProto)
	if !present || !bytes.Equal(auth, []byte{0xc0, 0x23}) {
		t.Fatalf("replacement Auth-Protocol = %x, want explicitly permitted PAP", auth)
	}
	ack := readLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck)
	if ack.Identifier != 0x41 || len(ack.Data) != 0 {
		t.Fatalf("replacement peer Ack = %+v, want its omitted-MRU request", ack)
	}
	writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
	assertProxyTransmitNegotiation(t, td.driver, 1500)
	completeProxyTransmitNetwork(t, td)
	assertProxyTransmitInterface(t, td, 1492, 2)
	removed := td.backend.AddrRemoveCalls()
	if len(removed) != 1 || removed[0].name != "ppp9" || removed[0].cidr != "10.0.0.1/32" {
		t.Fatalf("previous network lifetime cleanup = %+v", removed)
	}
	// The replacement peer request omitted Magic-Number as well as MRU.
	// RFC 1661 Section 6.4 requires this lifetime's Echo field to be zero.
	assertProxyTransmitEcho(t, td, 0)
}

func startProxyTransmitCapDriver(t *testing.T, peerMRU uint16) *ncpTestDriver {
	t.Helper()
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 24001)
	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(backend, ops)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeConn(pair.peerEnd)
		d.Stop()
	})
	peer := []LCPOption{magicOpt(proxyTransmitPeerMagic)}
	if peerMRU != 0 {
		peer = append(peer, mruOpt(peerMRU))
	}
	peerOptions := optStream(peer...)
	d.SessionsIn() <- StartSession{
		TunnelID:            24,
		SessionID:           1,
		ChanFD:              24001,
		UnitFD:              24002,
		UnitNum:             9,
		LNSMode:             true,
		MaxMRU:              1492,
		AuthMethod:          AuthMethodPAP,
		AuthFallbackOrder:   []AuthMethod{AuthMethodPAP},
		AuthRequired:        true,
		DisableIPv6CP:       true,
		AuthTimeout:         5 * time.Second,
		IPTimeout:           5 * time.Second,
		ProxyLCPInitialRecv: peerOptions,
		ProxyLCPLastRecv:    peerOptions,
		ProxyLCPLastSent:    optStream(mruOpt(1492), magicOpt(0x01020304), authProtoOpt(authProtoPAP)),
	}
	return &ncpTestDriver{t: t, driver: d, backend: backend, peer: pair.peerEnd, peerFD: 24001}
}

func assertProxyTransmitNegotiation(t *testing.T, d *Driver, peerMRU uint16) {
	t.Helper()
	ev := waitForEvent(t, d.EventsOut(), 5*time.Second)
	up, ok := ev.(EventLCPUp)
	if !ok || up.TunnelID != 24 || up.SessionID != 1 || up.NegotiatedMRU != peerMRU || up.NegotiatedAuthMethod != AuthMethodPAP {
		t.Fatalf("negotiation = %+v, want admitted proxy peer MRU %d and permitted PAP", ev, peerMRU)
	}
}

func completeProxyTransmitNetwork(t *testing.T, td *ncpTestDriver) {
	t.Helper()
	if err := td.peer.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := td.peer.Write(buildPAPRequestFrame(42, "proxy-subscriber", "proxy-secret")); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-td.driver.AuthEventsOut():
		req, ok := ev.(EventAuthRequest)
		if !ok || req.TunnelID != 24 || req.SessionID != 1 || req.Method != AuthMethodPAP || req.Username != "proxy-subscriber" || string(req.Response) != "proxy-secret" {
			t.Fatalf("proxy authentication = %+v, want the permitted PAP credential", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("accepted proxy did not reach credential verification")
	}
	select {
	case ev := <-td.driver.IPEventsOut():
		t.Fatalf("proxy reached network allocation before authentication: %+v", ev)
	default:
	}
	if err := td.driver.AuthResponse(24, 1, true, "verified proxy credential", nil); err != nil {
		t.Fatal(err)
	}
	ack := readLifecyclePacket(t, td.peer, ProtoPAP, PAPAuthenticateAck)
	if ack.Identifier != 42 {
		t.Fatalf("PAP Ack identifier = %d, want 42", ack.Identifier)
	}
	select {
	case ev := <-td.driver.AuthEventsOut():
		if _, ok := ev.(eventAuthSuccess); !ok {
			t.Fatalf("authentication result = %T, want success", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("permitted proxy authentication did not succeed")
	}
	select {
	case ev := <-td.driver.IPEventsOut():
		req, ok := ev.(EventIPRequest)
		if !ok || req.TunnelID != 24 || req.SessionID != 1 || req.Family != AddressFamilyIPv4 {
			t.Fatalf("network allocation = %+v, want the proxy's IPv4 request", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("authenticated proxy did not start IPCP")
	}
	if err := td.driver.IPResponse(24, 1, IPResponseArgs{
		Accept: true,
		Family: AddressFamilyIPv4,
		Local:  ipcpTestLocal,
		Peer:   ipcpTestPeer,
	}); err != nil {
		t.Fatal(err)
	}
	td.completeIPCP(t)
	ev := waitForEvent(t, td.driver.EventsOut(), 5*time.Second)
	assigned, ok := ev.(EventSessionIPAssigned)
	if !ok || assigned.Family != AddressFamilyIPv4 || assigned.Username != "proxy-subscriber" || assigned.Local != ipcpTestLocal || assigned.Peer != ipcpTestPeer {
		t.Fatalf("IPCP assignment = %+v, want the authenticated proxy's IPv4 pair", ev)
	}
	// The transport's acknowledgment is required before MTU/AdminUp are installed.
	assigned.Acknowledge()
	ev = waitForEvent(t, td.driver.EventsOut(), 5*time.Second)
	up, ok := ev.(EventSessionUp)
	if !ok || up.TunnelID != 24 || up.SessionID != 1 {
		t.Fatalf("post-NCP event = %+v, want the usable proxy session", ev)
	}
}

func assertProxyTransmitInterface(t *testing.T, td *ncpTestDriver, mtu, installs int) {
	t.Helper()
	calls := td.backend.MTUCalls()
	if len(calls) != installs {
		t.Fatalf("MTU calls = %+v, want %d installations", calls, installs)
	}
	for _, call := range calls {
		if call.name != "ppp9" || call.mtu != mtu {
			t.Fatalf("installed proxy MTU = %+v, want ppp9=%d", call, mtu)
		}
	}
	addresses := td.backend.P2PCalls()
	if len(addresses) != installs {
		t.Fatalf("IPCP address installations = %+v, want %d", addresses, installs)
	}
	for _, address := range addresses {
		if address.name != "ppp9" || address.local != "10.0.0.1/32" || address.peer != "10.0.0.2/32" {
			t.Fatalf("proxy IPv4 setup = %+v", address)
		}
	}
	if up := td.backend.UpCalls(); len(up) != installs {
		t.Fatalf("proxy AdminUp calls = %v, want %d", up, installs)
	}
}

func assertProxyTransmitEcho(t *testing.T, td *ncpTestDriver, peerMagic uint32) {
	t.Helper()
	data := make([]byte, 4+len("usable-proxy"))
	binary.BigEndian.PutUint32(data[:4], peerMagic)
	copy(data[4:], "usable-proxy")
	writeLifecyclePacket(t, td.peer, ProtoLCP, LCPEchoRequest, 77, data)
	reply := readLifecyclePacket(t, td.peer, ProtoLCP, LCPEchoReply)
	if reply.Identifier != 77 || len(reply.Data) != len(data) || !bytes.Equal(reply.Data[4:], data[4:]) {
		t.Fatalf("usable proxy Echo reply = %+v, want the correlated payload", reply)
	}
}
