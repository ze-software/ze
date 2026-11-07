package engine

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
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
	ps, old, _, table, gwPeer = ownedInitiatorAndGateway(t)
	return ps, old, table, gwPeer
}

// ownedInitiatorAndGateway is ownedInitiator that also returns gwOld, the gateway's end
// of the old IKE SA, for a test that plays the gateway's half of an exchange on it.
func ownedInitiatorAndGateway(t *testing.T) (ps *PeerSession, old, gwOld *SA, table *SATable, gwPeer ipsec.SiteToSitePeer) {
	t.Helper()
	// The COOKIE challenge is the admission gate, proven by rfc7296_cookie_test.go.
	admitWithoutCookieChallenge(t)
	old, gwOld, respPS := establishPSK(t)
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
	return ps, old, gwOld, table, respPeer
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
		t.Error("an unauthenticated successor signaled supersede")
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
		t.Error("a successor with no Child SA signaled supersede, which removes the working Child SA")
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
// retired, it leaves the SATable, and the cycle ends so the initiator reconnects. The
// gateway is told: Ze's session socket (ps.ike) is a loopback stand-in for the gateway
// (rtxPeerLink), and the next datagram it reads is a Delete of the successor, protected
// under the successor's keys (RFC 7296 Section 1.4.1).
func TestInitiatePeerOldDeletedWhileSuccessorChildlessReconnects(t *testing.T) {
	ps, old, table, gwPeer := ownedInitiator(t)
	ps.espGroup = mismatchedESPGroup()
	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	pending := admitGatewayReauth(t, ps, gw, pkt, table)
	runGatewayAuth(t, gw, pending, table)
	if pending.State != StateEstablished {
		t.Fatalf("setup: successor state = %v, want established", pending.State)
	}
	gatewayReadsAuth(t, gw, pending)
	peerTr, myTr := rtxPeerLink(t, pending)
	ps.ike = myTr
	pending.peerEndpoint = loopbackOf(t, peerTr)

	// The owner loop ended (the gateway deleted the old IKE SA).
	if got := ps.adoptPeerStartedSuccessor(table, nil, nil, slogutil.DiscardLogger()); got != nil {
		t.Fatal("a childless successor was adopted when the old SA ended")
	}
	expectIKEDelete(t, peerTr, gw, "the childless successor")
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
// before the fix: finishResponderEstablish signaled supersede on authentication alone.
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
		t.Error("a childless re-initiation signaled supersede and would remove the working Child SA")
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

// gatewayReadsAuth plays the gateway reading Ze's IKE_AUTH response, so its IKE SA is
// established and its request counter sits past IKE_AUTH, as a real gateway's does
// before it sends anything else on the new SA.
func gatewayReadsAuth(t *testing.T, gw, held *SA) {
	t.Helper()
	handleAuthResponse(gw, parseMsg(t, held.LastSentMsg), held.LastSentMsg, nil, nil, slogutil.DiscardLogger())
	if gw.State != StateEstablished {
		t.Fatalf("setup: the gateway's end of the successor is %v, want established", gw.State)
	}
}

// heldInitiatorSuccessor drives a gateway reauthentication whose IKE_AUTH Child SA Ze
// refuses (NO_PROPOSAL_CHOSEN from mismatchedESPGroup), so the successor is held beside
// the owner, then restores the configured esp-group, as an operator's reload of it
// would (adoptESPGroup), so the gateway's later request can be accepted.
func heldInitiatorSuccessor(t *testing.T) (ps *PeerSession, old, gw, held *SA, table *SATable) {
	t.Helper()
	ps, old, table, gwPeer := ownedInitiator(t)
	configured := ps.espGroup
	ps.espGroup = mismatchedESPGroup()
	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	held = admitGatewayReauth(t, ps, gw, pkt, table)
	runGatewayAuth(t, gw, held, table)
	if held.State != StateEstablished || ps.getPendingChild() != nil || len(ps.supersede) != 0 {
		t.Fatal("setup: the successor is not held without a Child SA")
	}
	gatewayReadsAuth(t, gw, held)
	ps.espGroup = configured
	return ps, old, gw, held, table
}

// peerCreatesChildOnHeld sends the peer's CREATE_CHILD_SA for a new Child SA on the held
// successor through the dispatch path, checks the peer can read an accepting answer,
// and returns the Child SA Ze staged.
func peerCreatesChildOnHeld(t *testing.T, ps *PeerSession, peer, held *SA, table *SATable, source *net.UDPAddr) *ChildSA {
	t.Helper()
	group, err := childRekeyDHGroup(peer, testESPGroup())
	if err != nil {
		t.Fatalf("childRekeyDHGroup: %v", err)
	}
	req, _, err := initiateChildCreate(peer, testESPGroup(), group)
	if err != nil {
		t.Fatalf("initiateChildCreate: %v", err)
	}
	if id := parseMsg(t, req).Header.MessageID; id != held.ExpectedMsgID {
		t.Fatalf("setup: the peer's request carries Message ID %d, the held successor expects %d", id, held.ExpectedMsgID)
	}
	handleInbound(held, transport.Packet{Data: req, RemoteAddr: source}, table, nil, slogutil.DiscardLogger())
	if !held.lastResponseSet || held.lastResponseID != parseMsg(t, req).Header.MessageID {
		t.Fatal("the peer's CREATE_CHILD_SA on the held successor went unanswered (RFC 7296 Section 2.21.3)")
	}
	inner, err := decryptAndParse(peer, parseMsg(t, held.lastResponse), held.lastResponse)
	if err != nil {
		t.Fatalf("the peer cannot read Ze's answer: %v", err)
	}
	rkyFindSA(t, inner)
	child := ps.getPendingChild()
	if child == nil {
		t.Fatal("the Child SA the peer created on the held successor was not staged in the second slot")
	}
	return child
}

// relinquishOwner runs the owner loop of old until it returns on the supersede signal.
func relinquishOwner(t *testing.T, ps *PeerSession, old *SA, table *SATable) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- ps.maintainSA(old, nil, newLifetimeState(3600), newLifetimeState(3600),
			testIKEGroup(), table, dataplane.Get(), nil, nil, slogutil.DiscardLogger())
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("maintainSA returned %v on supersede, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the owner loop did not relinquish on the supersede signal")
	}
}

// VALIDATES: AC-11. The gateway's reauthentication is held without a Child SA, and the
// gateway then creates the Child SA on the successor with CREATE_CHILD_SA. Ze installs
// it, stages it beside the old Child SA (make before break), tells the owner loop to
// relinquish, and the session adopts the successor with that Child SA.
// RFC 7296 Section 2.8.3: reauthentication creates "new Child SAs within the new IKE SA
// (without REKEY_SA Notify payloads)". Red before the fix: handleResponderInbound only
// replayed cached responses on an established second-slot SA, so the request went
// unanswered and nothing was staged.
func TestInitiatePeerHeldSuccessorAdoptedOnPeerChildCreate(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, gw, held, table := heldInitiatorSuccessor(t)
	oldChild := ps.getChildSA()

	newChild := peerCreatesChildOnHeld(t, ps, gw, held, table, gatewayAddr)
	if len(ps.supersede) != 1 {
		t.Fatal("the owner loop was not told to relinquish once the successor's Child SA was installed")
	}
	if ps.getChildSA() != oldChild || ps.getSA() != old {
		t.Fatal("the old IKE SA or its Child SA lost the session before the owner loop relinquished (make before break)")
	}

	relinquishOwner(t, ps, old, table)
	successor := ps.adoptPeerStartedSuccessor(table, dataplane.Get(), nil, log)
	if successor != held {
		t.Fatal("the initiate session did not adopt the successor once its Child SA existed")
	}
	if ps.getChildSA() != newChild {
		t.Error("the Child SA created on the successor is not the session's Child SA")
	}
	if ps.getPendingSA() != nil || ps.getPendingChild() != nil {
		t.Error("the second slot was not emptied by the adoption")
	}
}

// VALIDATES: AC-16. The same as AC-11 on a `respond` peer: its re-initiation is held
// without a Child SA, the peer creates one on it, and runResponder's promotion adopts
// the successor with that Child SA.
func TestRespondPeerHeldSuccessorAdoptedOnPeerChildCreate(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, table := ownedResponder(t)
	oldChild := ps.getChildSA()
	configured := ps.espGroup
	ps.espGroup = mismatchedESPGroup()

	ini, pkt := freshInitiatorInit(t)
	var iSPI, rSPI [8]byte
	copy(iSPI[:], ini.InitiatorSPI[:])
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("parallel init not accepted")
	}
	held := ps.getPendingSA()
	if held == nil {
		t.Fatal("no pending SA")
	}
	iniTable := NewSATable()
	iniTable.Insert(ini)
	handleSAInitResponse(ini, parseMsg(t, held.LastSentMsg), held.LastSentMsg, iniTable, nil, nil, log)
	handleInbound(held, transport.Packet{Data: ini.LastSentMsg, RemoteAddr: pkt.RemoteAddr}, table, nil, log)
	if held.State != StateEstablished || ps.getPendingChild() != nil || len(ps.supersede) != 0 {
		t.Fatal("setup: the re-initiation is not held without a Child SA")
	}
	gatewayReadsAuth(t, ini, held)
	ps.espGroup = configured

	newChild := peerCreatesChildOnHeld(t, ps, ini, held, table, pkt.RemoteAddr)
	if len(ps.supersede) != 1 {
		t.Fatal("the owner loop was not told to relinquish once the successor's Child SA was installed")
	}
	if ps.getChildSA() != oldChild {
		t.Fatal("the old Child SA was replaced before the owner loop relinquished (make before break)")
	}
	relinquishOwner(t, ps, old, table)
	if got := ps.resolvePendingAfterOwnerLoop(table, dataplane.Get(), nil, log); got != pendingContinue {
		t.Fatalf("resolvePendingAfterOwnerLoop = %v, want pendingContinue", got)
	}
	if ps.getSA() != held || ps.getChildSA() != newChild {
		t.Error("the respond session did not adopt the successor and the Child SA created on it")
	}
}

// VALIDATES: the held successor answers the gateway's liveness probe and its Delete
// (RFC 7296 Section 1.4: every INFORMATIONAL request is answered). A probe leaves the
// successor held; a Delete of the successor ends it alone: it leaves the SATable and
// the second slot, the half-open gate frees for the gateway's next attempt, and the
// established IKE SA and its Child SA keep the session.
// RFC 7296 Section 1.4.1: "The response to a request that deletes the IKE SA is an
// empty INFORMATIONAL response."
//
// RFC requirement: RFC7296-1.4-4 positive -- an INFORMATIONAL request on an IKE SA held
// beside the owner, a liveness probe and a Delete, each draws a response cached at its
// Message ID that the requesting peer decrypts.
//
// RFC requirement: RFC7296-1.4.1-6 positive -- a Delete naming the held IKE SA closes
// it: it leaves the SATable and the session's second slot.
func TestHeldSuccessorAnswersLivenessAndDelete(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, gw, held, table := heldInitiatorSuccessor(t)
	oldChild := ps.getChildSA()

	send := func(inner []wire.PayloadEntry) {
		t.Helper()
		id := held.ExpectedMsgID
		req, err := buildEncryptedMessageEx(gw, inner, id, wire.ExchangeInformational, initiatorFlag(gw))
		if err != nil {
			t.Fatalf("build INFORMATIONAL: %v", err)
		}
		handleInbound(held, transport.Packet{Data: req, RemoteAddr: gatewayAddr}, table, nil, log)
		if !held.lastResponseSet || held.lastResponseID != id {
			t.Fatalf("the gateway's INFORMATIONAL request %d on the held successor went unanswered", id)
		}
	}

	send(nil)
	answer, err := decryptAndParse(gw, parseMsg(t, held.lastResponse), held.lastResponse)
	if err != nil {
		t.Fatalf("the gateway cannot read the liveness answer: %v", err)
	}
	if len(answer) != 0 {
		t.Errorf("the liveness answer carries %d payloads, want an empty INFORMATIONAL", len(answer))
	}
	if ps.getPendingSA() != held {
		t.Fatal("a liveness probe ended the held successor")
	}

	send([]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}})
	if ps.getPendingSA() != nil {
		t.Error("the gateway's Delete left the successor in the second slot")
	}
	if table.Lookup(held.InitiatorSPI, held.ResponderSPI) != nil {
		t.Error("the deleted successor is still in the SATable")
	}
	if ps.responderBusy.Load() {
		t.Error("the half-open gate stayed set, so the gateway's next reauthentication would be dropped")
	}
	if ps.getSA() != old || ps.getChildSA() != oldChild {
		t.Error("the Delete of the held successor disturbed the established IKE SA or its Child SA")
	}
}

// failingInstallBackend is a registered dataplane whose every SA install fails, for the
// install-failure injections of AC-10 and AC-15. The IKE_AUTH Child SA install reads
// the active backend (dataplane.Get), so the fake is loaded rather than passed.
var failingInstallBackend struct {
	sync.Once
	err    error
	active *rkyDP
}

func useFailingInstallDataplane(t *testing.T) *rkyDP {
	t.Helper()
	const name = "ike-engine-test-reauth-install-failure"
	failingInstallBackend.Do(func() {
		failingInstallBackend.err = dataplane.Register(name, func() (dataplane.Dataplane, error) { return failingInstallBackend.active, nil })
	})
	if failingInstallBackend.err != nil {
		t.Fatal(failingInstallBackend.err)
	}
	dp := &rkyDP{installErr: errors.New("injected SA install failure")}
	failingInstallBackend.active = dp
	if err := dataplane.Load(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Error(err)
		}
		failingInstallBackend.active = nil
	})
	return dp
}

// expectChildlessAuthAnswer checks that Ze's IKE_AUTH response gives the peer no Child
// SA (no SA payload) and names the refusal with an error notify, so the peer installs
// no half of a pair Ze does not hold and no Delete is owed for one.
func expectChildlessAuthAnswer(t *testing.T, peer, held *SA) {
	t.Helper()
	inner, err := decryptAndParse(peer, parseMsg(t, held.LastSentMsg), held.LastSentMsg)
	if err != nil {
		t.Fatalf("the peer cannot read Ze's IKE_AUTH response: %v", err)
	}
	refused := false
	for _, pe := range inner {
		if _, ok := pe.Payload.(*wire.PayloadSA); ok {
			t.Error("the IKE_AUTH response gave the peer an SA payload for a Child SA Ze could not install")
		}
		// RFC 7296 Section 3.10.1: error types are below 16384.
		if n, ok := pe.Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType < 16384 {
			refused = true
		}
	}
	if !refused {
		t.Error("the IKE_AUTH response names no refusal of the Child SA")
	}
}

// VALIDATES: AC-10. The successor authenticates and the dataplane refuses to install
// its Child SA (injected): the successor is held, not adopted, the old IKE SA and its
// Child SA keep the session, and the fake dataplane holds no state of the successor.
// The refusal answered the IKE_AUTH before the peer was given its half, so no Delete
// for an ESP pair is owed (expectChildlessAuthAnswer).
// RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA to be
// deleted."
func TestInitiatePeerReauthChildInstallFailureKeepsOldSA(t *testing.T) {
	ps, old, table, gwPeer := ownedInitiator(t)
	oldChild := ps.getChildSA()
	dp := useFailingInstallDataplane(t)
	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	held := admitGatewayReauth(t, ps, gw, pkt, table)
	runGatewayAuth(t, gw, held, table)

	if held.State != StateEstablished {
		t.Fatalf("successor state = %v, want established (only its Child SA failed)", held.State)
	}
	if ps.getPendingChild() != nil {
		t.Fatal("a Child SA whose install failed was staged")
	}
	if len(ps.supersede) != 0 {
		t.Error("a successor whose Child SA install failed signaled supersede")
	}
	if ps.getSA() != old || ps.getChildSA() != oldChild || ps.getPendingSA() != held {
		t.Error("the install failure moved the session off the old IKE SA, or the successor was not held")
	}
	if len(dp.installed) != 0 {
		t.Errorf("the fake dataplane holds %d SA states of the successor, want none", len(dp.installed))
	}
	expectChildlessAuthAnswer(t, gw, held)
}

// VALIDATES: AC-15, the install-failure injection on a `respond` peer's re-initiation.
func TestRespondPeerReinitChildInstallFailureKeepsOldSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, table := ownedResponder(t)
	oldChild := ps.getChildSA()
	dp := useFailingInstallDataplane(t)
	ini, pkt := freshInitiatorInit(t)
	var iSPI, rSPI [8]byte
	copy(iSPI[:], ini.InitiatorSPI[:])
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("parallel init not accepted")
	}
	held := ps.getPendingSA()
	if held == nil {
		t.Fatal("no pending SA")
	}
	iniTable := NewSATable()
	iniTable.Insert(ini)
	handleSAInitResponse(ini, parseMsg(t, held.LastSentMsg), held.LastSentMsg, iniTable, nil, nil, log)
	handleInbound(held, transport.Packet{Data: ini.LastSentMsg, RemoteAddr: pkt.RemoteAddr}, table, nil, log)

	if held.State != StateEstablished {
		t.Fatalf("parallel SA state = %v, want established", held.State)
	}
	if ps.getPendingChild() != nil || len(ps.supersede) != 0 {
		t.Fatal("a Child SA whose install failed was staged or signaled supersede")
	}
	if ps.getSA() != old || ps.getChildSA() != oldChild {
		t.Error("the old IKE SA and its Child SA lost ownership to a re-initiation whose install failed")
	}
	if len(dp.installed) != 0 {
		t.Errorf("the fake dataplane holds %d SA states of the successor, want none", len(dp.installed))
	}
	expectChildlessAuthAnswer(t, ini, held)
}

// VALIDATES: AC-7. After the session adopts the gateway's successor, Ze is the
// responder of its IKE SA, and a Child SA rekey works started by either side. Ze's own
// request carries the Initiator flag clear and its own counter at 0, and the gateway
// decrypts it, which proves the role-dependent keys and counters follow the SA's role,
// not the peer's connection type. The gateway's request is answered by the owner path.
// RFC 7296 Section 2.2: "Each endpoint in the IKE Security Association maintains two
// 'current' Message IDs: the next one to be used for a request it initiates and the
// next one it expects to see in a request from the other end."
func TestAdoptedResponderRoleSARekeys(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, table, gwPeer := ownedInitiator(t)
	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	pending := admitGatewayReauth(t, ps, gw, pkt, table)
	runGatewayAuth(t, gw, pending, table)
	gatewayReadsAuth(t, gw, pending)
	relinquishOwner(t, ps, old, table)
	successor := ps.adoptPeerStartedSuccessor(table, dataplane.Get(), nil, log)
	if successor != pending {
		t.Fatal("setup: the successor was not adopted")
	}
	zeChild := ps.getChildSA()
	gwChild, err := createFirstChildSA(gw, testESPGroup(), gwPeer.LocalAddress, gwPeer.RemoteAddress, 1, nil, log)
	if err != nil {
		t.Fatalf("the gateway's Child SA: %v", err)
	}
	t.Cleanup(gwChild.Clear)

	// Ze rekeys.
	req, p, err := initiateChildRekey(successor, zeChild)
	if err != nil {
		t.Fatalf("Ze could not build a Child SA rekey on the adopted SA: %v", err)
	}
	msg := parseMsg(t, req)
	if msg.Header.Flags&wire.FlagInitiator != 0 {
		t.Error("Ze set the Initiator flag on an IKE SA the gateway created (RFC 7296 Section 3.1)")
	}
	if msg.Header.MessageID != 0 {
		t.Errorf("Ze's first request on the adopted SA carries Message ID %d, want 0", msg.Header.MessageID)
	}
	reqInner, err := decryptAndParse(gw, msg, req)
	if err != nil {
		t.Fatalf("the gateway cannot decrypt Ze's rekey, the keys follow the wrong role: %v", err)
	}
	answer, _, err := respondChildRekey(gw, reqInner, gwChild, msg.Header.MessageID, nil, log)
	if err != nil {
		t.Fatalf("the gateway refused Ze's rekey: %v", err)
	}
	ansInner, err := decryptAndParse(successor, parseMsg(t, answer), answer)
	if err != nil {
		t.Fatalf("Ze cannot decrypt the gateway's answer: %v", err)
	}
	if newChild, err := applyChildRekeyResponse(successor, p, ansInner, nil, log); err != nil || newChild == nil {
		t.Fatalf("Ze did not complete its rekey on the adopted SA: %v", err)
	}
	successor.answerAuthenticatedResponse(msg.Header.MessageID)

	// The gateway rekeys.
	gwReq, _, err := initiateChildRekey(gw, gwChild)
	if err != nil {
		t.Fatalf("the gateway could not build its rekey: %v", err)
	}
	out := ps.handleOwnedInbound(successor, transport.Packet{Data: gwReq, RemoteAddr: gatewayAddr}, nil, nil, log)
	if out.newChild == nil {
		t.Fatal("Ze, responder of the adopted IKE SA, did not answer the gateway's Child SA rekey")
	}
}
