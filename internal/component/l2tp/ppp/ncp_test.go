package ppp

import (
	"errors"
	"log/slog"
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

// readIPv6CPUntil is the IPv6CP counterpart of readIPCPUntil: it reads IPv6CP
// packets until one carries the wanted code, so an assertion on ze's response
// is not defeated by an interleaved (and legitimate) Configure-Request resend.
func readIPv6CPUntil(t *testing.T, td *ncpTestDriver, want uint8) LCPPacket {
	t.Helper()
	for range 6 {
		pkt := td.readPeerNCPPacket(t, ProtoIPv6CP)
		if pkt.Code == want {
			return pkt
		}
	}
	t.Fatalf("no IPv6CP packet with code %d after 6 reads", want)
	return LCPPacket{}
}

// ipv6cpInterfaceIDOption builds the wire bytes of an Interface-Identifier
// option (type 1, length 10) carrying id, the Data field of an IPv6CP
// Configure-* packet.
func ipv6cpInterfaceIDOption(id [ipv6cpInterfaceIDLen]byte) []byte {
	out := make([]byte, 0, ipv6cpInterfaceIDOptLen)
	out = append(out, IPv6CPOptInterfaceID, ipv6cpInterfaceIDOptLen)
	return append(out, id[:]...)
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
//
// RFC requirement: RFC5072-3-1 positive -- IPV6CP is exchanged only inside the
// network-layer protocol phase: completeIPv6CP runs the IPv6CP Configure exchange to the
// Opened state, which is only reachable after runNCPPhase (the network phase) has started
// the IPv6CP FSM (startNCP, internal/component/l2tp/ppp/ncp.go), and only then is the
// IPv6 assignment announced. The pre-network half is TestIPv6CPNoResponseBeforeNetworkPhase.
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

// TestIPv6ServiceStartsOnlyAfterIPv6CPOpened pins the guard at
// afterLCPOpen (internal/component/l2tp/ppp/session_run.go): startIPv6Service --
// the RA sender and DHCPv6 server that originate IPv6 packets on pppN -- is
// called only inside `if s.ipv6cpState == LCPStateOpened`. startIPv6Service
// cannot succeed against the fake ppp42 interface (no such interface / no raw
// socket), so it returns an error that afterLCPOpen logs once as
// "ppp: IPv6 service start failed (non-fatal)". That log is emitted if and only
// if the Opened guard passed, which makes it a faithful observable for "the IPv6
// service was invoked". afterLCPOpen runs the log before it sends EventSessionUp,
// so once the test observes EventSessionUp the attempt has already been recorded.
//
// VALIDATES: a session that drives IPV6CP to Opened attempts to start the IPv6
// service (the log appears); the control session with IPv6CP disabled -- which
// never reaches Opened -- never attempts it (the log is absent). The control run
// proves the assertion is not vacuous: ze does not start the IPv6 service
// unconditionally.
// PREVENTS: a regression that started RA/DHCPv6 origination before IPV6CP reached
// Opened (or regardless of it), putting IPv6 packets on the wire too early.
//
// RFC requirement: RFC5072-2-1 positive -- PPP MUST reach the network-layer protocol
// phase and IPV6CP MUST reach Opened before any IPv6 packet is sent (§2): ze's IPv6
// packet source, startIPv6Service (RA sender + DHCPv6 server), is invoked from
// afterLCPOpen (internal/component/l2tp/ppp/session_run.go) only under the
// s.ipv6cpState == LCPStateOpened guard, which afterLCPOpen reaches only after
// runNCPPhase (the network phase), so it runs after the network phase and only once
// IPV6CP has Opened.
func TestIPv6ServiceStartsOnlyAfterIPv6CPOpened(t *testing.T) {
	const startAttemptLog = "IPv6 service start failed"

	t.Run("opened: IPv6 service is started", func(t *testing.T) {
		w := &captureWriter{}
		logger := slog.New(slog.NewTextHandler(w, nil))
		td := newNCPTestDriverIPLogged(t, &StartSession{DisableIPCP: true}, autoAcceptIP, logger)
		defer td.cleanup()

		td.completeIPv6CP(t)

		if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
			t.Fatal("no EventSessionUp after IPv6CP reached Opened")
		}
		// Reaching EventSessionUp means afterLCPOpen passed the line-482 guard, so
		// the (failing) startIPv6Service attempt has already been logged.
		if got := w.String(); !strings.Contains(got, startAttemptLog) {
			t.Errorf("IPv6 service was not started after IPV6CP reached Opened; log = %q", got)
		}
	})

	t.Run("control: IPv6CP disabled, service is not started", func(t *testing.T) {
		w := &captureWriter{}
		logger := slog.New(slog.NewTextHandler(w, nil))
		td := newNCPTestDriverIPLogged(t, &StartSession{DisableIPv6CP: true}, autoAcceptIP, logger)
		defer td.cleanup()

		td.completeIPCP(t)

		if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
			t.Fatal("no EventSessionUp after IPCP reached Opened")
		}
		// ipv6cpState never left Initial, so the line-482 guard is false and
		// startIPv6Service is never called: the attempt log must be absent.
		if got := w.String(); strings.Contains(got, startAttemptLog) {
			t.Errorf("IPv6 service was started although IPV6CP never reached Opened; log = %q", got)
		}
	})
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

// TestIPv6CPNoResponseBeforeNetworkPhase is the pre-network half of RFC 5072 §3:
// "IPV6CP packets MUST NOT be exchanged until PPP has reached the network-layer protocol
// phase; earlier packets should be silently discarded." Before the network phase the
// per-session ipv6cpState is still LCPStateInitial (its zero value); handleFrame
// (internal/component/l2tp/ppp/session_run.go, ProtoIPv6CP branch) buffers such a frame
// into earlyNCPFrames and returns without writing anything to the chan fd -- ze never
// exchanges an IPV6CP packet before the phase begins.
//
// VALIDATES: an IPV6CP Configure-Request delivered while ipv6cpState==LCPStateInitial
// produces NO IPV6CP response on the wire (recordingChanFile stays empty) and is buffered
// (earlyNCPFrames grows), not answered.
// PREVENTS: a regression that answered (Ack/Nak/Reject) an IPV6CP packet arriving before
// the network phase, exchanging IPV6CP too early.
//
// RFC requirement: RFC5072-3-1 negative -- an IPV6CP Configure-Request arriving before the
// network-layer protocol phase (ipv6cpState==LCPStateInitial) draws no IPV6CP response;
// handleFrame (internal/component/l2tp/ppp/session_run.go) buffers it into earlyNCPFrames
// and writes nothing to the chan fd.
func TestIPv6CPNoResponseBeforeNetworkPhase(t *testing.T) {
	chanFile := &recordingChanFile{}
	// A pristine pppSession: ipv6cpState defaults to LCPStateInitial (0) and
	// disableIPv6CP defaults to false, i.e. IPv6CP is enabled but the network
	// phase has not started.
	s := &pppSession{
		logger:   discardLogger(),
		chanFile: chanFile,
	}
	if s.ipv6cpState != LCPStateInitial {
		t.Fatalf("precondition: ipv6cpState = %s, want Initial", s.ipv6cpState)
	}

	frame := make([]byte, MaxFrameLen)
	off := WriteFrame(frame, 0, ProtoIPv6CP, nil)
	off += WriteLCPPacket(frame, off, LCPConfigureRequest, 0x30,
		ipv6cpInterfaceIDOption(ipv6cpTestPeerID))

	if term := s.handleFrame(frame[:off]); term {
		t.Fatal("handleFrame terminated the session on a pre-network IPV6CP packet")
	}
	if chanFile.Len() != 0 {
		t.Fatalf("ze emitted %x in response to a pre-network IPV6CP Configure-Request; "+
			"RFC 5072 §3 forbids exchanging IPV6CP before the network phase", chanFile.Bytes())
	}
	if len(s.earlyNCPFrames) != 1 {
		t.Fatalf("pre-network IPV6CP frame not buffered: earlyNCPFrames = %d, want 1",
			len(s.earlyNCPFrames))
	}
}

// TestIPv6CPInterfaceIDsDiffer drives IPv6CP to Opened and reads back both ends' chosen
// interface identifiers: ze's own (carried in its initial Configure-Request) and the
// peer's (echoed in EventSessionIPAssigned once negotiation completes).
//
// VALIDATES: upon completion the two ends hold DIFFERENT interface identifiers.
// PREVENTS: a regression that let both ends settle on the same identifier.
//
// RFC requirement: RFC5072-4.1-2 positive -- "the interface identifier MUST be unique
// within the PPP link; ... different interface-identifier values are to be selected for
// the ends of the PPP link" (§4.1): ze's locally generated identifier
// (generateIPv6CPInterfaceID, internal/component/l2tp/ppp/ipv6cp.go) differs from the
// peer's accepted identifier at Opened.
func TestIPv6CPInterfaceIDsDiffer(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	cr := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("initial IPv6CP code = %d, want Configure-Request", cr.Code)
	}
	zeOpts, err := ParseIPv6CPOptions(cr.Data)
	if err != nil || !zeOpts.HasInterfaceID {
		t.Fatalf("ze's initial CR missing Interface-Identifier: %v", err)
	}
	localIID := zeOpts.InterfaceID

	// Complete with the peer proposing a different identifier.
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureAck, cr.Identifier, cr.Data)
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureRequest, 0x22,
		ipv6cpInterfaceIDOption(ipv6cpTestPeerID))
	if ack := readIPv6CPUntil(t, td, LCPConfigureAck); ack.Code != LCPConfigureAck {
		t.Fatalf("ze did not Ack the peer's Interface-Identifier")
	}

	assigned, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("no EventSessionIPAssigned after IPv6CP Opened")
	}
	if assigned.InterfaceID == localIID {
		t.Fatalf("both ends negotiated the same interface identifier %x; RFC 5072 §4.1 "+
			"requires the two ends to differ", localIID)
	}
	if assigned.InterfaceID != ipv6cpTestPeerID {
		t.Errorf("peer InterfaceID = %x, want %x", assigned.InterfaceID, ipv6cpTestPeerID)
	}
}

// TestIPv6CPNaksCollidingInterfaceID has the peer propose ze's OWN identifier. This is the
// uniqueness guard from the receive side: evalIPv6CPRequest
// (internal/component/l2tp/ppp/ncp.go) returns optsBad when the proposed identifier equals
// s.localInterfaceID, so the FSM answers Configure-Nak instead of Configure-Ack.
//
// VALIDATES: a peer Configure-Request whose Interface-Identifier equals ze's own is NOT
// acknowledged; ze replies Configure-Nak.
// PREVENTS: a regression that Acked a colliding identifier, leaving both ends identical.
//
// RFC requirement: RFC5072-4.1-2 negative -- the identifiers must be unique within the
// link (§4.1): when the peer proposes ze's own identifier, ze refuses to Ack it and sends
// a Configure-Nak so the peer must pick a different value.
func TestIPv6CPNaksCollidingInterfaceID(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	cr := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("initial IPv6CP code = %d, want Configure-Request", cr.Code)
	}
	zeOpts, err := ParseIPv6CPOptions(cr.Data)
	if err != nil || !zeOpts.HasInterfaceID {
		t.Fatalf("ze's initial CR missing Interface-Identifier: %v", err)
	}

	// Peer echoes ze's own identifier -> collision.
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureRequest, 0x21,
		ipv6cpInterfaceIDOption(zeOpts.InterfaceID))

	resp := readIPv6CPUntil(t, td, LCPConfigureNak)
	if resp.Code == LCPConfigureAck {
		t.Fatal("ze Acked a colliding Interface-Identifier equal to its own")
	}
	if resp.Code != LCPConfigureNak {
		t.Fatalf("response code = %d, want Configure-Nak for a colliding identifier", resp.Code)
	}
}

// TestIPv6CPAcksDifferentNonZeroInterfaceID has the peer propose a valid, non-zero
// identifier distinct from ze's. evalIPv6CPRequest (internal/component/l2tp/ppp/ncp.go)
// accepts it (optsBad=false), so the FSM emits Configure-Ack echoing the option.
//
// VALIDATES: a valid, different, non-zero peer identifier is answered with Configure-Ack
// that carries the peer's identifier verbatim.
// PREVENTS: a regression that Nak'd or rejected an acceptable identifier.
//
// RFC requirement: RFC5072-4.1-3 positive -- "if the two interface identifiers are
// different and the received interface identifier is not zero, the interface identifier
// MUST be acknowledged, i.e., a Configure-Ack is sent with the requested interface
// identifier" (§4.1).
func TestIPv6CPAcksDifferentNonZeroInterfaceID(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	cr := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("initial IPv6CP code = %d, want Configure-Request", cr.Code)
	}

	// ipv6cpTestPeerID is non-zero, non-all-ones, and (with overwhelming
	// probability) different from ze's crypto/rand identifier.
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureRequest, 0x21,
		ipv6cpInterfaceIDOption(ipv6cpTestPeerID))

	ack := readIPv6CPUntil(t, td, LCPConfigureAck)
	if ack.Code != LCPConfigureAck {
		t.Fatalf("response code = %d, want Configure-Ack for a valid non-zero identifier", ack.Code)
	}
	ackOpts, err := ParseIPv6CPOptions(ack.Data)
	if err != nil || !ackOpts.HasInterfaceID {
		t.Fatalf("Ack missing Interface-Identifier: %v", err)
	}
	if ackOpts.InterfaceID != ipv6cpTestPeerID {
		t.Errorf("Ack Interface-Identifier = %x, want the requested %x",
			ackOpts.InterfaceID, ipv6cpTestPeerID)
	}
}

// TestIPv6CPNaksZeroInterfaceID has the peer propose the all-zero identifier.
// isValidIPv6CPInterfaceID (internal/component/l2tp/ppp/ipv6cp.go) rejects all-zero, so
// evalIPv6CPRequest returns optsBad and the FSM answers Configure-Nak, never Configure-Ack.
//
// VALIDATES: an all-zero proposed identifier is NOT acknowledged; ze replies Configure-Nak.
// PREVENTS: a regression that Acked the reserved all-zero identifier.
//
// RFC requirement: RFC5072-4.1-3 negative -- the "MUST be acknowledged" rule is for a
// different, non-zero identifier (§4.1); a zero interface identifier is not a valid value
// to Ack, so ze answers Configure-Nak instead.
func TestIPv6CPNaksZeroInterfaceID(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	cr := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("initial IPv6CP code = %d, want Configure-Request", cr.Code)
	}

	var zero [ipv6cpInterfaceIDLen]byte
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureRequest, 0x21,
		ipv6cpInterfaceIDOption(zero))

	resp := readIPv6CPUntil(t, td, LCPConfigureNak)
	if resp.Code == LCPConfigureAck {
		t.Fatal("ze Acked an all-zero Interface-Identifier")
	}
	if resp.Code != LCPConfigureNak {
		t.Fatalf("response code = %d, want Configure-Nak for a zero identifier", resp.Code)
	}
}

// TestIPv6CPResendsCRWithNakSuggestedID feeds ze a Configure-Nak carrying a valid suggested
// identifier. absorbIPv6CPNak (internal/component/l2tp/ppp/ncp.go) adopts it into
// s.localInterfaceID, and the ReqSent+RCN transition (ppp_fsm.go) resends a
// Configure-Request; writeNCPOptions then emits the adopted identifier.
//
// VALIDATES: after a valid Configure-Nak, ze's resent Configure-Request carries the
// Nak-suggested Interface-Identifier.
// PREVENTS: a regression that ignored the peer's suggestion and re-proposed the old value.
//
// RFC requirement: RFC5072-4.1-7 positive -- "a new Configure-Request MUST be sent with
// the identifier value suggested in the last Configure-Nak from the peer" (§4.1).
func TestIPv6CPResendsCRWithNakSuggestedID(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	cr1 := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr1.Code != LCPConfigureRequest {
		t.Fatalf("initial IPv6CP code = %d, want Configure-Request", cr1.Code)
	}

	suggested := [ipv6cpInterfaceIDLen]byte{0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11}
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureNak, cr1.Identifier,
		ipv6cpInterfaceIDOption(suggested))

	cr2 := readIPv6CPUntil(t, td, LCPConfigureRequest)
	opts, err := ParseIPv6CPOptions(cr2.Data)
	if err != nil || !opts.HasInterfaceID {
		t.Fatalf("resent CR missing Interface-Identifier: %v", err)
	}
	if opts.InterfaceID != suggested {
		t.Errorf("resent CR Interface-Identifier = %x, want the Nak-suggested %x",
			opts.InterfaceID, suggested)
	}
}

// TestIPv6CPNakInvalidSuggestionNotAdopted feeds ze a Configure-Nak whose suggested
// identifier is the invalid all-zero value. absorbIPv6CPNak
// (internal/component/l2tp/ppp/ncp.go) guards adoption behind isValidIPv6CPInterfaceID, so
// the invalid suggestion is discarded and the resent Configure-Request keeps ze's original
// identifier.
//
// VALIDATES: an invalid Nak suggestion is NOT adopted; the resent Configure-Request carries
// ze's original identifier, not the invalid suggested value.
// PREVENTS: a regression that blindly copied any Nak-suggested value, including all-zero.
//
// RFC requirement: RFC5072-4.1-7 negative -- the "resend with the suggested value" rule
// (§4.1) applies to a usable suggestion; an invalid (all-zero) suggestion is not adopted,
// so the resent Configure-Request does not carry it.
func TestIPv6CPNakInvalidSuggestionNotAdopted(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	cr1 := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr1.Code != LCPConfigureRequest {
		t.Fatalf("initial IPv6CP code = %d, want Configure-Request", cr1.Code)
	}
	orig, err := ParseIPv6CPOptions(cr1.Data)
	if err != nil || !orig.HasInterfaceID {
		t.Fatalf("ze's initial CR missing Interface-Identifier: %v", err)
	}

	var zero [ipv6cpInterfaceIDLen]byte
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureNak, cr1.Identifier,
		ipv6cpInterfaceIDOption(zero))

	cr2 := readIPv6CPUntil(t, td, LCPConfigureRequest)
	opts, err := ParseIPv6CPOptions(cr2.Data)
	if err != nil || !opts.HasInterfaceID {
		t.Fatalf("resent CR missing Interface-Identifier: %v", err)
	}
	if opts.InterfaceID == zero {
		t.Fatal("resent CR adopted the invalid all-zero Nak suggestion")
	}
	if opts.InterfaceID != orig.InterfaceID {
		t.Errorf("resent CR Interface-Identifier = %x, want the original %x "+
			"(an invalid suggestion must be ignored)", opts.InterfaceID, orig.InterfaceID)
	}
}

// TestIPv6CPInterfaceIDRejectIsFatal is the IPv6CP counterpart of
// TestIPCPIPAddressRejectIsFatal. The Interface-Identifier is the sole, mandatory IPv6CP
// option; absorbIPv6CPReject (internal/component/l2tp/ppp/ncp.go) returns fatal when a
// Configure-Reject names it, so handleNCPPacket calls s.fail and the session is torn down.
// Because the session ends, ze never sends a further Configure-Request -- and so cannot
// send one still carrying the Interface-Identifier option.
//
// VALIDATES: a valid Configure-Reject of the Interface-Identifier option brings the session
// DOWN.
// PREVENTS: a regression that absorbed the reject and kept re-proposing the option.
//
// RFC requirement: RFC5072-4.1-12 positive -- "a new Configure-Request MUST NOT contain the
// interface-identifier option if a valid Interface-Identifier Configure-Reject is received"
// (§4.1): rejecting the mandatory option is fatal, so no subsequent Configure-Request
// carrying it is ever sent.
func TestIPv6CPInterfaceIDRejectIsFatal(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	cr := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("initial IPv6CP code = %d, want Configure-Request", cr.Code)
	}
	// RFC 1661 §5.4: the Configure-Reject echoes the rejected option verbatim -- here ze's
	// own Interface-Identifier option from the CR it just sent.
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureReject, cr.Identifier, cr.Data)

	if _, ok := waitForEventOfType[EventSessionDown](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionDown after the peer rejected the mandatory Interface-Identifier")
	}
}

// TestIPv6CPUnknownOptionRejectNotFatal contrasts with the mandatory-option reject: a
// Configure-Reject naming a NON-Interface-Identifier option (type 2) leaves
// absorbIPv6CPReject (internal/component/l2tp/ppp/ncp.go) returning non-fatal, so the
// session survives and the ReqSent+RCN transition (ppp_fsm.go) resends the
// Configure-Request -- which still legitimately carries the Interface-Identifier, because
// that option was NOT the one rejected.
//
// VALIDATES: a Configure-Reject of a non-Interface-Identifier option does NOT tear the
// session down; ze keeps negotiating and its resent Configure-Request still carries the
// Interface-Identifier.
// PREVENTS: a regression that treated every Configure-Reject as fatal.
//
// RFC requirement: RFC5072-4.1-12 negative -- the "MUST NOT re-include the option" rule
// (§4.1) is scoped to a reject OF the Interface-Identifier option; a reject of some other
// option is not fatal and does not strip the Interface-Identifier from the next
// Configure-Request.
func TestIPv6CPUnknownOptionRejectNotFatal(t *testing.T) {
	td := newNCPTestDriverCfg(t, &StartSession{DisableIPCP: true})
	defer td.cleanup()

	cr := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("initial IPv6CP code = %d, want Configure-Request", cr.Code)
	}
	// A well-formed option that is NOT Interface-Identifier (type 2, length 4).
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureReject, cr.Identifier,
		[]byte{2, 4, 0x00, 0x11})

	if _, ok := waitForEventOfType[EventSessionDown](t, td.driver.EventsOut(), 300*time.Millisecond); ok {
		t.Fatal("Configure-Reject of a non-Interface-Identifier option must not tear the session down")
	}

	// ze continues negotiating: the resent Configure-Request still carries the
	// Interface-Identifier (that option was not rejected).
	resent := readIPv6CPUntil(t, td, LCPConfigureRequest)
	opts, err := ParseIPv6CPOptions(resent.Data)
	if err != nil || !opts.HasInterfaceID {
		t.Fatalf("resent CR missing Interface-Identifier after a non-fatal reject: %v", err)
	}
}
