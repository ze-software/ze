package ppp

import (
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
func TestIPResponseConfiguresInterface(t *testing.T) {
	td := newNCPTestDriverCfg(t, StartSession{DisableIPv6CP: true})
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

	route := td.backend.RouteAddCalls()
	if len(route) != 1 || route[0].dest != "10.0.0.2/32" {
		t.Errorf("AddRoute calls = %+v, want one 10.0.0.2/32", route)
	}
	up := td.backend.UpCalls()
	if len(up) < 2 {
		t.Errorf("SetAdminUp calls = %v, want at least 2 (post-LCP + post-IPCP)", up)
	}
}

// VALIDATES: AC-8 (explicit) -- EventSessionIPAssigned{ipv4} carries
//
//	the DNS values supplied by the handler.
func TestIPCPOpenedEmitsAssigned(t *testing.T) {
	td := newNCPTestDriverCfg(t, StartSession{DisableIPv6CP: true})
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

// VALIDATES: AC-10, AC-11 -- IPv6CP reaching Opened emits
//
//	EventSessionIPAssigned{ipv6} with the peer's Interface-ID; NO
//	iface.Backend.AddAddressP2P call is made.
func TestIPv6CPOpenedEmitsAssigned(t *testing.T) {
	td := newNCPTestDriverCfg(t, StartSession{DisableIPCP: true})
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
	td := newNCPTestDriverCfg(t, StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	td.completeIPCP(t)

	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp")
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
	td := newNCPTestDriverCfg(t, StartSession{DisableIPv6CP: true})
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
	td := newNCPTestDriverCfg(t, StartSession{DisableIPv6CP: true})
	defer td.cleanup()

	td.completeIPCP(t)

	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp")
	}
}

// VALIDATES: end-to-end IPv6CP via net.Pipe produces EventSessionUp.
func TestIPv6CPNetPipe(t *testing.T) {
	td := newNCPTestDriverCfg(t, StartSession{DisableIPCP: true})
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
