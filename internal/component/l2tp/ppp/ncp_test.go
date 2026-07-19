package ppp

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// VALIDATES: the generic FSM (LCPDoTransition) drives IPCP- and
//
//	IPv6CP-shaped events identically to LCP -- Closed + Open -> ReqSent
//	with [IRC, SCR]; ReqSent + RCA -> AckRcvd.
//
// PREVENTS: per-NCP FSM duplication drift (ze reuses the same function
//
//	for all three protocols per RFC 1661 §2).
func TestNCPFSMShared(t *testing.T) {
	cases := []struct {
		name    string
		state   LCPState
		event   LCPEvent
		wantNew LCPState
		wantAct []LCPAction
	}{
		{"closed+open", LCPStateClosed, LCPEventOpen, LCPStateReqSent, []LCPAction{LCPActIRC, LCPActSCR}},
		{"reqsent+rca", LCPStateReqSent, LCPEventRCA, LCPStateAckRcvd, []LCPAction{LCPActIRC}},
		{"acksent+rca", LCPStateAckSent, LCPEventRCA, LCPStateOpened, []LCPAction{LCPActIRC, LCPActTLU}},
		{"reqsent+rcr+", LCPStateReqSent, LCPEventRCRPlus, LCPStateAckSent, []LCPAction{LCPActSCA}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := LCPDoTransition(tc.state, tc.event)
			if tr.NewState != tc.wantNew {
				t.Errorf("new state = %s, want %s", tr.NewState, tc.wantNew)
			}
			if len(tr.Actions) != len(tc.wantAct) {
				t.Fatalf("actions = %v, want %v", tr.Actions, tc.wantAct)
			}
			for i := range tr.Actions {
				if tr.Actions[i] != tc.wantAct[i] {
					t.Errorf("action[%d] = %s, want %s", i, tr.Actions[i], tc.wantAct[i])
				}
			}
		})
	}
}

// VALIDATES: AC-1 -- after auth success, ze emits one EventIPRequest
//
//	on the IPEventsOut channel for the enabled family.
//
// PREVENTS: session-up without NCP phase.
func TestAuthSuccessStartsNCPs(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 9001)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := NewDriver(DriverConfig{
		Logger:  discardLogger(),
		Backend: &fakeBackend{},
		Ops:     ops,
	})
	go autoAcceptAuth(d)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- StartSession{
		TunnelID:            1,
		SessionID:           1,
		ChanFD:              9001,
		UnitFD:              9002,
		UnitNum:             1,
		LNSMode:             true,
		MaxMRU:              1500,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}

	select {
	case ev := <-d.IPEventsOut():
		req, ok := ev.(EventIPRequest)
		if !ok {
			t.Fatalf("ip event %T, want EventIPRequest", ev)
		}
		if req.Family != AddressFamilyIPv4 {
			t.Errorf("family = %s, want ipv4", req.Family)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventIPRequest")
	}
}

// VALIDATES: AC-4..AC-8 -- after IPResponse, the IPCP exchange reaches
//
//	Opened; ze programs pppN (AddAddressP2P + AddRoute + SetAdminUp)
//	and emits EventSessionIPAssigned{ipv4}.
//
// RFC requirement: RFC1332-2.1-1 positive -- IP is communicated only once IPCP has
// reached the Opened state: completeIPCP drives IPCP to Opened, and only then does ze
// program the pppN address (AddAddressP2P) and emit EventSessionIPAssigned. onNCPOpened
// (internal/component/l2tp/ppp/ncp.go), the sole AddAddressP2P caller, runs on the
// transition into Opened.
func TestIPResponseConfiguresInterface(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	td.completeIPCP(t)

	assigned, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("no EventSessionIPAssigned")
	}
	if assigned.Family != AddressFamilyIPv4 {
		t.Errorf("family = %s, want ipv4", assigned.Family)
	}
	if assigned.Local != ipcpTestLocal || assigned.Peer != ipcpTestPeer {
		t.Errorf("addresses = %v / %v, want %v / %v",
			assigned.Local, assigned.Peer, ipcpTestLocal, ipcpTestPeer)
	}

	p2p := td.backend.P2PCalls()
	if len(p2p) != 1 {
		t.Fatalf("AddAddressP2P calls = %d, want 1", len(p2p))
	}
	if p2p[0].name != "ppp42" || p2p[0].local != "10.0.0.1/32" || p2p[0].peer != "10.0.0.2/32" {
		t.Errorf("p2p[0] = %+v, want ppp42 10.0.0.1/32 10.0.0.2/32", p2p[0])
	}

	up := td.backend.UpCalls()
	if len(up) < 1 {
		t.Errorf("SetAdminUp calls = %v, want at least 1", up)
	}
}

// TestIPCPNoAddressBeforeOpened is the negative half of RFC 1332 Section 2.1: ze must not
// communicate IP -- program the pppN address or announce it via EventSessionIPAssigned --
// until IPCP reaches the Opened state.
//
// VALIDATES: with the IPCP exchange stalled short of Opened (ze's Configure-Request Acked,
// driving it to AckRcvd, but ze has not yet Acked the peer's Configure-Request because the
// peer never sends one), the backend AddAddressP2P is never called and no
// EventSessionIPAssigned is emitted.
// PREVENTS: programming the interface address at Configure-Request-sent or at AckRcvd,
// which would put IPv4 on the wire before IPCP Opened. onNCPOpened (ncp.go) is the only
// AddAddressP2P caller and runs only on the transition into Opened, so a regression that
// moved the programming earlier is exactly what this pins.
//
// RFC requirement: RFC1332-2.1-1 negative -- before IPCP reaches Opened, ze programs no
// address and emits no EventSessionIPAssigned, so no IP is communicated pre-Opened; the
// sole AddAddressP2P caller onNCPOpened (internal/component/l2tp/ppp/ncp.go) fires only on
// the transition into Opened.
func TestIPCPNoAddressBeforeOpened(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	// ze has sent its initial Configure-Request but is not Opened.
	cr := td.readPeerNCPPacket(t, ProtoIPCP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("got code %d, want Configure-Request", cr.Code)
	}
	if calls := td.backend.P2PCalls(); len(calls) != 0 {
		t.Fatalf("AddAddressP2P called at Configure-Request-sent, before Opened: %+v", calls)
	}

	// Ack ze's Configure-Request -> AckRcvd. Still not Opened: ze has not Acked
	// the peer's Configure-Request (the peer never sends one), so the exchange
	// stalls one step short of Opened.
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureAck, cr.Identifier, cr.Data)

	// No IP may be communicated while short of Opened.
	if _, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 300*time.Millisecond); ok {
		t.Fatal("EventSessionIPAssigned emitted before IPCP reached Opened")
	}
	if calls := td.backend.P2PCalls(); len(calls) != 0 {
		t.Errorf("AddAddressP2P called before IPCP reached Opened: %+v", calls)
	}
}

// VALIDATES: AC-8 (explicit) -- EventSessionIPAssigned{ipv4} carries
//
//	the DNS values supplied by the handler.
func TestIPCPOpenedEmitsAssigned(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	td.completeIPCP(t)

	assigned, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("no EventSessionIPAssigned")
	}
	if assigned.DNSPrimary != ipcpTestDNS1 {
		t.Errorf("DNSPrimary = %v, want %v", assigned.DNSPrimary, ipcpTestDNS1)
	}
}

// TestIPCPBackendAddressFailureFailsSession drives the iface.Backend error
// path, which onNCPOpened (ncp.go) reaches when AddAddressP2P returns non-nil.
//
// VALIDATES: when the kernel refuses the point-to-point address, the session is
// brought DOWN with the backend's reason, rather than being reported up with an
// address that was never installed.
// PREVENTS: the mock's addAddrP2PErr field existing but never being set by any
// test, leaving the fail() branch (`s.fail("iface AddAddressP2P: ...")`,
// ncp.go, immediately before the EventSessionIPAssigned send) unexercised. A
// regression that dropped the error check would install nothing, still emit
// IPAssigned, and every test would stay green.
func TestIPCPBackendAddressFailureFailsSession(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	// Armed before IPCP reaches Opened; AddAddressP2P is only called at
	// onNCPOpened, after the exchange completeIPCP plays below.
	td.backend.setAddAddrP2PErr(errors.New("rtnetlink: address already assigned"))

	td.completeIPCP(t)

	down, ok := waitForEventOfType[EventSessionDown](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("no EventSessionDown after the backend refused the address")
	}
	if !strings.Contains(down.Reason, "AddAddressP2P") {
		t.Errorf("reason = %q, want it to name the failing backend call", down.Reason)
	}
	if !strings.Contains(down.Reason, "address already assigned") {
		t.Errorf("reason = %q, want it to carry the backend's error", down.Reason)
	}
	// The address must have been ATTEMPTED: this pins that the failure came
	// from the backend call rather than from never reaching it.
	if calls := td.backend.P2PCalls(); len(calls) != 1 {
		t.Errorf("AddAddressP2P calls = %d, want 1 attempt; got %+v", len(calls), calls)
	}
}

// readIPCPUntil reads IPCP packets from the peer end until one carries the
// wanted code, skipping ze's own Configure-Request retransmissions (its CR is
// still unacknowledged during these exchanges, so it legitimately repeats).
func readIPCPUntil(t *testing.T, td *ncpTestDriver, want uint8) LCPPacket {
	t.Helper()
	for range 6 {
		pkt := td.readPeerNCPPacket(t, ProtoIPCP)
		if pkt.Code == want {
			return pkt
		}
	}
	t.Fatalf("no IPCP packet with code %d after 6 reads", want)
	return LCPPacket{}
}

// TestIPCPDNSRejectAbsorbed drives absorbIPCPReject's non-fatal branch.
//
// ze never OFFERS DNS in its own Configure-Request: writeNCPOptions (ncp.go)
// carries only IP-Address, because RFC 1877 DNS is communicated via Nak to the
// peer's Configure-Request (buildNakOrReject, ncp.go, fills PrimaryDNS from
// s.dnsPrimary). So the absorb branch defends against a peer that rejects
// options ze never sent, and its observable effect is on the NAK ze sends next.
//
// VALIDATES: after a peer Configure-Rejects the DNS options, the session
// survives (non-fatal) and ze's subsequent Nak offers NO DNS addresses, while
// still Nak-ing the peer's IP-Address.
// PREVENTS: the HasPrimary/HasSecondary clearing in absorbIPCPReject going
// unexercised. Without it ze would keep pushing DNS a peer explicitly refused.
// The control subtest proves the reject is what causes the difference, so the
// assertion cannot pass vacuously.
//
// RFC requirement: RFC1877-x-1 positive -- the link stays usable for IPv4 whether or
// not DNS is assigned (RFC 1877 Scope): when the peer Configure-Rejects the DNS
// options, absorbIPCPReject clears them and the session survives, still negotiating
// the IPv4 address, so IPv4 connectivity does not depend on DNS assignment.
func TestIPCPDNSRejectAbsorbed(t *testing.T) {
	t.Run("control: nak carries dns when nothing was rejected", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()

		cr := td.readPeerNCPPacket(t, ProtoIPCP)
		if cr.Code != LCPConfigureRequest {
			t.Fatalf("got code %d, want Configure-Request", cr.Code)
		}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureAck, cr.Identifier, cr.Data)

		// Peer asks for an address and both DNS servers.
		peerCR := []byte{IPCPOptIPAddress, 6, 0, 0, 0, 0, IPCPOptPrimaryDNS, 6, 0, 0, 0, 0, IPCPOptSecondaryDNS, 6, 0, 0, 0, 0}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x20, peerCR)

		nak := readIPCPUntil(t, td, LCPConfigureNak)
		opts, err := ParseIPCPOptions(nak.Data)
		if err != nil {
			t.Fatalf("parse nak: %v", err)
		}
		if !opts.HasPrimary || opts.PrimaryDNS != ipcpTestDNS1 {
			t.Errorf("control nak primary DNS = %v (has=%v), want %v", opts.PrimaryDNS, opts.HasPrimary, ipcpTestDNS1)
		}
		if !opts.HasSecondary || opts.SecondaryDNS != ipcpTestDNS2 {
			t.Errorf("control nak secondary DNS = %v (has=%v), want %v", opts.SecondaryDNS, opts.HasSecondary, ipcpTestDNS2)
		}
	})

	t.Run("dns cleared after configure-reject", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()

		cr := td.readPeerNCPPacket(t, ProtoIPCP)
		if cr.Code != LCPConfigureRequest {
			t.Fatalf("got code %d, want Configure-Request", cr.Code)
		}

		// Reject the DNS options, echoed verbatim per RFC 1661 5.4.
		reject := []byte{IPCPOptPrimaryDNS, 6, 0, 0, 0, 0, IPCPOptSecondaryDNS, 6, 0, 0, 0, 0}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureReject, cr.Identifier, reject)

		// Peer now asks for an address and both DNS servers.
		peerCR := []byte{IPCPOptIPAddress, 6, 0, 0, 0, 0, IPCPOptPrimaryDNS, 6, 0, 0, 0, 0, IPCPOptSecondaryDNS, 6, 0, 0, 0, 0}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x21, peerCR)

		nak := readIPCPUntil(t, td, LCPConfigureNak)
		opts, err := ParseIPCPOptions(nak.Data)
		if err != nil {
			t.Fatalf("parse nak: %v", err)
		}
		if opts.HasPrimary {
			t.Errorf("nak still offers primary DNS %v after the peer rejected it", opts.PrimaryDNS)
		}
		if opts.HasSecondary {
			t.Errorf("nak still offers secondary DNS %v after the peer rejected it", opts.SecondaryDNS)
		}
		// The address is NOT part of what was rejected: it must still be Nak-ed,
		// proving the session was absorbed rather than torn down.
		if !opts.HasIPAddress || opts.IPAddress != ipcpTestPeer {
			t.Errorf("nak IP-Address = %v (has=%v), want %v", opts.IPAddress, opts.HasIPAddress, ipcpTestPeer)
		}
	})
}

// TestIPCPIPAddressRejectIsFatal is absorbIPCPReject's other half.
//
// VALIDATES: rejecting the mandatory IP-Address option takes the session down
// instead of absorbing it (fatal=true).
// PREVENTS: treating IP-Address like DNS and continuing without an address.
//
// RFC requirement: RFC1877-x-1 negative -- the "usable with or without DNS" tolerance
// is specific to DNS, not the IPv4 address: rejecting the IP-Address option is fatal
// and tears the session down, so it is DNS (not IPv4 reachability) that is optional
// (RFC 1877 Scope).
func TestIPCPIPAddressRejectIsFatal(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	cr := td.readPeerNCPPacket(t, ProtoIPCP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("got code %d, want Configure-Request", cr.Code)
	}
	reject := []byte{IPCPOptIPAddress, 6, 0, 0, 0, 0}
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureReject, cr.Identifier, reject)

	if _, ok := waitForEventOfType[EventSessionDown](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionDown after the peer rejected the mandatory IP-Address option")
	}
}

// VALIDATES: AC-10, AC-11 -- IPv6CP reaching Opened emits
//
//	EventSessionIPAssigned{ipv6} with the peer's Interface-ID; NO
//	iface.Backend.AddAddressP2P call is made.
func TestIPv6CPOpenedEmitsAssigned(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	td.completeIPv6CP(t)

	assigned, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("no EventSessionIPAssigned")
	}
	if assigned.Family != AddressFamilyIPv6 {
		t.Errorf("family = %s, want ipv6", assigned.Family)
	}
	if assigned.InterfaceID != ipv6cpTestPeerID {
		t.Errorf("InterfaceID = %x, want %x", assigned.InterfaceID, ipv6cpTestPeerID)
	}
	if calls := td.backend.P2PCalls(); len(calls) != 0 {
		t.Errorf("AddAddressP2P should NOT be called for IPv6CP; got %+v", calls)
	}
}

// VALIDATES: AC-12 -- EventSessionUp fires after both NCPs reach
//
//	Opened.
func TestBothNCPsComplete(t *testing.T) {
	td := newNCPTestDriver(t)
	defer td.cleanup()

	peerDone := make(chan struct{})
	go runParallelNCPPeer(t, td.peer, true, true, peerDone)
	t.Cleanup(func() { <-peerDone })

	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 3*time.Second); !ok {
		t.Fatal("no EventSessionUp after both NCPs Opened")
	}
}

// VALIDATES: AC-13 -- DisableIPv6CP=true skips IPv6CP; EventSessionUp
//
//	fires on IPCP-Opened alone.
func TestSingleNCPCompletes(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	td.completeIPCP(t)

	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp")
	}
}

// VALIDATES: an IPv6CP rejection by the handler (e.g. an IPv4-only
//
//	static pool returning "IPv6 not supported by static pool") does NOT
//	tear the session down; IPCP/IPv4 still reaches EventSessionUp.
//	IPv6CP is an independent NCP (RFC 5072; RFC 1661 §2).
//
// PREVENTS: regression of the L2TP boot-evidence failure where the
//
//	IPv6CP-reject path called s.fail and flapped the whole session
//	(reason="ipv6cp: handler rejected: IPv6 not supported by static
//	pool"), even though IPCP had already negotiated 10.x addresses.
func TestIPv6CPRejectionKeepsIPv4Session(t *testing.T) {
	// Both NCPs enabled; the handler accepts IPv4 and rejects IPv6.
	td := newNCPTestDriverIP(t, &StartSession{}, autoAcceptIPv4RejectIPv6)
	defer td.cleanup()

	// IPv6 is declined, so the driver drops IPv6CP and never emits an
	// IPv6CP CONFREQ -- the sequential IPCP peer is sufficient.
	td.completeIPCP(t)

	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp: an IPv6CP rejection must not tear down the IPv4 session")
	}
}

// VALIDATES: AC-17 -- no IPResponse within ip-timeout fires
//
//	EventSessionDown.
//
// PREVENTS: session hangs when the IP handler crashed.
func TestIPTimeout(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 12001)
	defer closeConn(pair.peerEnd)

	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- StartSession{
		TunnelID:            77,
		SessionID:           88,
		ChanFD:              12001,
		UnitFD:              12002,
		UnitNum:             11,
		LNSMode:             true,
		MaxMRU:              1500,
		IPTimeout:           100 * time.Millisecond,
		DisableIPv6CP:       true,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}

	// Drain the parked EventIPRequest so the session goroutine is
	// inside the timeout select, not blocked on send.
	go func() {
		<-d.IPEventsOut()
	}()

	if _, ok := waitForEventOfType[EventSessionDown](t, d.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionDown after ip-timeout")
	}
}

// VALIDATES: AC-18 -- StopSession after IPCP-Opened triggers
//
//	RemoveAddress and RemoveRoute on the backend.
func TestSessionTeardownRemovesAddress(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	td.completeIPCP(t)
	drainEventsBest(t, td.driver.EventsOut(), 2, 500*time.Millisecond)

	if err := td.driver.StopSession(1, 1); err != nil {
		t.Fatalf("StopSession: %v", err)
	}

	removes := td.backend.AddrRemoveCalls()
	if len(removes) != 1 || removes[0].cidr != "10.0.0.1/32" {
		t.Errorf("addr removes = %+v, want one 10.0.0.1/32", removes)
	}
	routeRemoves := td.backend.RouteRemoveCalls()
	if len(routeRemoves) != 1 || routeRemoves[0].dest != "10.0.0.2/32" {
		t.Errorf("route removes = %+v, want one 10.0.0.2/32", routeRemoves)
	}
}

// VALIDATES: end-to-end IPCP via net.Pipe produces EventSessionUp.
func TestIPCPNetPipe(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	td.completeIPCP(t)

	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp")
	}
}

// VALIDATES: end-to-end IPv6CP via net.Pipe produces EventSessionUp.
func TestIPv6CPNetPipe(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	td.completeIPv6CP(t)

	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp")
	}
}

// VALIDATES: end-to-end both NCPs against same pipe produce
//
//	EventSessionUp.
func TestParallelNCPsNetPipe(t *testing.T) {
	td := newNCPTestDriver(t)
	defer td.cleanup()

	peerDone := make(chan struct{})
	go runParallelNCPPeer(t, td.peer, true, true, peerDone)
	t.Cleanup(func() { <-peerDone })

	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 3*time.Second); !ok {
		t.Fatal("no EventSessionUp after parallel NCPs")
	}
}
