package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// gatewayAddr is where the gateway of the `initiate` peer answers: responderTestPeers
// gives Ze's initiate peer RemoteAddress 10.0.0.2.
var gatewayAddr = &net.UDPAddr{IP: net.ParseIP("10.0.0.2"), Port: 500}

// ownedInitiator returns an `initiate` PeerSession whose established IKE SA (old, Ze the
// initiator) is owned by the simulated owner loop, with a Child SA installed, its
// authenticated endpoint published as the gateway's address, and the session
// registered as the only active peer: the state a CPE tunnel is in just before the
// gateway reauthenticates. gwPeer is the gateway's mirrored configuration.
func ownedInitiator(t *testing.T) (ps *PeerSession, old *SA, table *SATable, gwPeer ipsec.SiteToSitePeer) {
	t.Helper()
	// The COOKIE challenge is the admission gate, proven by rfc7296_cookie_test.go.
	admitWithoutCookieChallenge(t)
	old, _, respPS := establishPSK(t)
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "rekey-psk")
	ps = &PeerSession{peerName: "ze", peerCfg: iniPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	ps.stopCh = make(chan struct{})
	ps.done = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, inboundQueueDepth)
	// A real Child SA from the same handshake, so the supersede arm has one to remove.
	ps.setChildSA(respPS.getChildSA())
	table = NewSATable()
	table.Insert(old)
	ps.setSA(old)
	ps.ownedSA.Store(old)
	ps.saEndpoint.Store(gatewayAddr)
	setActivePeers(map[string]*PeerSession{ps.peerName: ps})
	t.Cleanup(func() { setActivePeers(nil) })
	return ps, old, table, respPeer
}

// gatewayReauthInit builds the gateway's fresh IKE_SA_INIT request, a new IKE SA from
// scratch with no REKEY_SA (RFC 7296 Section 2.8.3), arriving from source.
func gatewayReauthInit(t *testing.T, gwPeer ipsec.SiteToSitePeer, source *net.UDPAddr) (*SA, transport.Packet) {
	t.Helper()
	gw, err := newInitiatorSA("ze", gwPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	req := buildSAInitRequest(gw, testIKEGroup())
	gw.InitiatorSAInitMsg = req
	gw.State = StateSAInitSent
	return gw, transport.Packet{Data: req, RemoteAddr: source}
}

// admitGatewayReauth feeds the gateway's IKE_SA_INIT to the dispatch path and returns
// the second-slot SA it created.
func admitGatewayReauth(t *testing.T, ps *PeerSession, gw *SA, pkt transport.Packet, table *SATable) *SA {
	t.Helper()
	var iSPI, rSPI [8]byte
	copy(iSPI[:], gw.InitiatorSPI[:])
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, slogutil.DiscardLogger()) {
		t.Fatal("the gateway's IKE_SA_INIT toward an established `initiate` peer was not accepted")
	}
	pending := ps.getPendingSA()
	if pending == nil {
		t.Fatal("the reauthentication did not create a second-slot SA")
	}
	return pending
}

// runGatewayAuth plays the gateway's half of the handshake: it reads Ze's IKE_SA_INIT
// response and sends IKE_AUTH, which Ze answers on the dispatch path.
func runGatewayAuth(t *testing.T, gw, pending *SA, table *SATable) {
	t.Helper()
	log := slogutil.DiscardLogger()
	gwTable := NewSATable()
	gwTable.Insert(gw)
	handleSAInitResponse(gw, parseMsg(t, pending.LastSentMsg), pending.LastSentMsg, gwTable, nil, nil, log)
	if gw.State != StateAuthSent {
		t.Fatalf("gateway state = %v, want auth-sent", gw.State)
	}
	handleInbound(pending, transport.Packet{Data: gw.LastSentMsg, RemoteAddr: gatewayAddr}, table, nil, log)
}

// mismatchedESPGroup is an esp-group whose only proposal the gateway does not offer
// (testESPGroup offers sha256), so Ze refuses the IKE_AUTH Child SA with
// NO_PROPOSAL_CHOSEN and keeps the IKE SA.
func mismatchedESPGroup() ipsec.ESPGroup {
	return ipsec.ESPGroup{
		Name:     "esp-mismatch",
		Lifetime: 3600,
		PFS:      ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{
			Number:     1,
			Encryption: ipsec.EncryptionAES256,
			Hash:       ipsec.HashSHA512,
		}},
	}
}

// VALIDATES: AC-1, AC-2 (the wiring test). The gateway of an `initiate` peer
// reauthenticates by sending a fresh IKE_SA_INIT from the established SA's address.
// It is admitted on the responder path (matchReauthenticatingPeer, tryResponderSAInit),
// authenticated with the same peer configuration, its Child SA staged beside the old
// one, the owner loop relinquishes on the supersede signal, and the session adopts the
// successor with Ze as its responder. Red before the fix: matchResponderPeer considered
// only `respond` peers, so tryResponderSAInit answered false.
func TestInitiatePeerAdoptsPeerStartedReauth(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, table, gwPeer := ownedInitiator(t)
	oldChild := ps.getChildSA()
	if oldChild == nil {
		t.Fatal("setup: the established initiate session has no Child SA")
	}

	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	pending := admitGatewayReauth(t, ps, gw, pkt, table)
	if ps.getSA() != old {
		t.Fatal("the unauthenticated IKE_SA_INIT replaced the established SA (RFC 7296 Section 2.4)")
	}
	runGatewayAuth(t, gw, pending, table)

	if pending.State != StateEstablished {
		t.Fatalf("successor state = %v, want established", pending.State)
	}
	newChild := ps.getPendingChild()
	if newChild == nil {
		t.Fatal("the successor's Child SA was not staged in the second slot")
	}
	if len(ps.supersede) != 1 {
		t.Fatal("the owner loop was not told to relinquish once the successor's Child SA was installed")
	}
	if ps.getChildSA() != oldChild {
		t.Fatal("the old Child SA was removed before the owner loop relinquished (make before break)")
	}

	done := make(chan error, 1)
	go func() {
		done <- ps.maintainSA(old, nil, newLifetimeState(3600), newLifetimeState(3600),
			testIKEGroup(), table, dataplane.Get(), nil, nil, log)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("maintainSA returned %v on supersede, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the owner loop did not relinquish on the supersede signal")
	}

	successor := ps.adoptPeerStartedSuccessor(table, dataplane.Get(), nil, log)
	if successor != pending {
		t.Fatal("the initiate session did not adopt the gateway's successor")
	}
	if ps.getSA() != pending {
		t.Error("the successor is not the session's SA")
	}
	if ps.getChildSA() != newChild {
		t.Error("the successor's Child SA is not the session's Child SA")
	}
	if ps.getPendingSA() != nil || ps.getPendingChild() != nil {
		t.Error("the second slot was not emptied by the adoption")
	}
	if ps.responderBusy.Load() {
		t.Error("the half-open gate stayed set after adoption, so the next reauthentication would be dropped")
	}
	if successor.IsInitiator {
		t.Error("Ze must be the responder of the adopted SA, the gateway created it")
	}
}

// VALIDATES: AC-5, AC-8. An `initiate` peer refuses an unsolicited IKE_SA_INIT while
// it holds no IKE SA, and once it holds one it refuses an initiation from any address
// but the one that SA talks to. Nothing enters the SATable either way.
func TestInitiatePeerRefusesUnsolicitedInit(t *testing.T) {
	ps, old, table, gwPeer := ownedInitiator(t)
	log := slogutil.DiscardLogger()

	// AC-8: the SA is established, the source is another address.
	stranger := &net.UDPAddr{IP: net.ParseIP("10.0.0.9"), Port: 500}
	gw, pkt := gatewayReauthInit(t, gwPeer, stranger)
	var iSPI, rSPI [8]byte
	copy(iSPI[:], gw.InitiatorSPI[:])
	if tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Error("an IKE_SA_INIT from an address the SA does not talk to was accepted")
	}

	// AC-5: no established SA, the source is the configured remote.
	ps.ownedSA.Store(nil)
	ps.saEndpoint.Store(nil)
	gw, pkt = gatewayReauthInit(t, gwPeer, gatewayAddr)
	copy(iSPI[:], gw.InitiatorSPI[:])
	if tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Error("an `initiate` peer with no established SA accepted an unsolicited IKE_SA_INIT")
	}

	if table.Len() != 1 {
		t.Errorf("table holds %d SAs, want only the old one", table.Len())
	}
	if ps.getPendingSA() != nil || ps.getSA() != old {
		t.Error("a refused initiation changed the session's SA slots")
	}
}

// VALIDATES: AC-6. A peer that authenticates with EAP refuses a reauthentication the
// gateway starts. RFC 7296 Section 2.8.3: with EAP, reauthentication "has to be
// initiated by the same party as the original IKE SA". Ze as initiator sends no
// CFG_REQUEST, so the Configuration payload half of the sentence has no case to test.
func TestInitiatePeerRefusesReauthAfterEAPOrCP(t *testing.T) {
	for _, mode := range []ipsec.AuthMode{ipsec.AuthEAPTLS, ipsec.AuthEAPMSCHAPv2, ipsec.AuthEAPMD5} {
		ps, _, table, gwPeer := ownedInitiator(t)
		ps.peerCfg.Auth.Mode = mode
		gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
		var iSPI, rSPI [8]byte
		copy(iSPI[:], gw.InitiatorSPI[:])
		if tryResponderSAInit(pkt, iSPI, rSPI, table, nil, slogutil.DiscardLogger()) {
			t.Errorf("%s: a reauthentication the gateway started was accepted", mode)
		}
		if ps.getPendingSA() != nil {
			t.Errorf("%s: the refused reauthentication created a second-slot SA", mode)
		}
	}
}

// VALIDATES: AC-4. The gateway's IKE_AUTH fails to authenticate (wrong pre-shared
// key): the old SA and its Child SA keep the session, nothing supersedes, and the
// failed successor is discarded when the owner loop ends.
func TestInitiatePeerReauthFailureKeepsOldSA(t *testing.T) {
	ps, old, table, gwPeer := ownedInitiator(t)
	oldChild := ps.getChildSA()
	gwPeer.Auth.PSK = "not-the-psk"
	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	pending := admitGatewayReauth(t, ps, gw, pkt, table)
	runGatewayAuth(t, gw, pending, table)

	if pending.State == StateEstablished {
		t.Fatal("an IKE_AUTH with the wrong key established the successor")
	}
	if len(ps.supersede) != 0 {
		t.Error("an unauthenticated successor signalled supersede")
	}
	if ps.getSA() != old || ps.getChildSA() != oldChild {
		t.Error("the failed reauthentication disturbed the established SA or its Child SA")
	}
	if got := ps.adoptPeerStartedSuccessor(table, nil, nil, slogutil.DiscardLogger()); got != nil {
		t.Error("a successor that failed to authenticate was adopted")
	}
	if ps.getPendingSA() != nil || table.Len() != 1 {
		t.Error("the failed successor was not discarded")
	}
}

// VALIDATES: AC-9. The successor authenticates but Ze refuses its Child SA
// (NO_PROPOSAL_CHOSEN, a refusal buildChildlessAuthResponse answers): it is held beside
// the owner and NOT adopted; the old IKE SA and Child SA keep the session.
// RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA to be
// deleted."
func TestInitiatePeerReauthChildRefusedKeepsOldSA(t *testing.T) {
	ps, old, table, gwPeer := ownedInitiator(t)
	oldChild := ps.getChildSA()
	ps.espGroup = mismatchedESPGroup()
	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	pending := admitGatewayReauth(t, ps, gw, pkt, table)
	runGatewayAuth(t, gw, pending, table)

	if pending.State != StateEstablished {
		t.Fatalf("successor state = %v, want established (only its Child SA is refused)", pending.State)
	}
	if ps.getPendingChild() != nil {
		t.Fatal("setup: the injected refusal still produced a Child SA")
	}
	if len(ps.supersede) != 0 {
		t.Error("a successor with no Child SA signalled supersede, which removes the working Child SA")
	}
	if ps.getSA() != old || ps.getChildSA() != oldChild {
		t.Error("the old IKE SA and its Child SA lost the session to a childless successor")
	}
	if ps.getPendingSA() != pending {
		t.Error("the authenticated childless successor was not held in the second slot")
	}
}

// VALIDATES: AC-12. The old IKE SA ends while the successor is still held childless:
// the session does not stay up childless (owner decision Q-5), so the successor is
// retired, it leaves the SATable, and the cycle ends so the initiator reconnects.
func TestInitiatePeerOldDeletedWhileSuccessorChildlessReconnects(t *testing.T) {
	ps, old, table, gwPeer := ownedInitiator(t)
	ps.espGroup = mismatchedESPGroup()
	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	pending := admitGatewayReauth(t, ps, gw, pkt, table)
	runGatewayAuth(t, gw, pending, table)
	if pending.State != StateEstablished {
		t.Fatalf("setup: successor state = %v, want established", pending.State)
	}

	// The owner loop ended (the gateway deleted the old IKE SA).
	if got := ps.adoptPeerStartedSuccessor(table, nil, nil, slogutil.DiscardLogger()); got != nil {
		t.Fatal("a childless successor was adopted when the old SA ended")
	}
	if ps.getPendingSA() != nil {
		t.Error("the childless successor was left in the second slot")
	}
	if table.Len() != 1 || table.Lookup(pending.InitiatorSPI, pending.ResponderSPI) != nil {
		t.Error("the retired successor is still in the SATable")
	}
	if ps.getSA() != old {
		t.Error("the retirement replaced the session's SA slot; runInitiator clears it itself")
	}
}

// VALIDATES: AC-15. A `respond` peer's re-initiation that authenticates with its Child
// SA refused is held, not adopted: the old IKE SA and Child SA keep ownership, and
// when the owner loop ends the held successor is retired rather than promoted. Red
// before the fix: finishResponderEstablish signalled supersede on authentication alone.
func TestRespondPeerReinitChildRefusedKeepsOldSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, table := ownedResponder(t)
	oldChild := ps.getChildSA()
	ps.espGroup = mismatchedESPGroup()

	ini, pkt := freshInitiatorInit(t)
	var iSPI, rSPI [8]byte
	copy(iSPI[:], ini.InitiatorSPI[:])
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("parallel init not accepted")
	}
	pending := ps.getPendingSA()
	if pending == nil {
		t.Fatal("no pending SA")
	}
	iniTable := NewSATable()
	iniTable.Insert(ini)
	handleSAInitResponse(ini, parseMsg(t, pending.LastSentMsg), pending.LastSentMsg, iniTable, nil, nil, log)
	handleInbound(pending, transport.Packet{Data: ini.LastSentMsg}, table, nil, log)

	if pending.State != StateEstablished {
		t.Fatalf("parallel SA state = %v, want established", pending.State)
	}
	if ps.getPendingChild() != nil {
		t.Fatal("setup: the injected refusal still produced a Child SA")
	}
	if len(ps.supersede) != 0 {
		t.Error("a childless re-initiation signalled supersede and would remove the working Child SA")
	}
	if ps.getSA() != old || ps.getChildSA() != oldChild {
		t.Error("the old IKE SA and its Child SA lost ownership to a childless re-initiation")
	}

	if got := ps.resolvePendingAfterOwnerLoop(table, nil, nil, log); got != pendingReturn {
		t.Fatalf("resolvePendingAfterOwnerLoop = %v, want pendingReturn (a childless successor is never promoted)", got)
	}
	if ps.getPendingSA() != nil || ps.getSA() == pending {
		t.Error("the held childless successor was promoted")
	}
	if table.Lookup(pending.InitiatorSPI, pending.ResponderSPI) != nil {
		t.Error("the retired successor is still in the SATable")
	}
}
