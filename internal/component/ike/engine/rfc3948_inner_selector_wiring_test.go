// Design: docs/architecture/ike/ipsec-14-responder.md -- the cross-peer inner-selector guard
// Related: reconcile.go -- startPeerSession hands each session the table's registry
// Related: register.go -- tryResponderSAInit hands it to the responder SA
// Related: fsm.go -- runInitiator hands it to the initiator SA
// Related: rekey.go -- an IKE SA rekey hands it to the replacement SA
// VALIDATES: the sessions reconcilePeers builds from one SATable share its inner-selector
// registry on every IKE SA they hold: the responder SA of IKE_SA_INIT, the initiator SA
// of runInitiator, and the SA an IKE rekey replaces it with, in both directions.
// PREVENTS: a production path whose SA carries no registry, which serves another peer's
// inner address while every test that builds its SA by hand stays green.

package engine

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const wiringPSK = "wiring-psk"

// wiringResponder builds a respond-only peer session for the peer at remote through
// startPeerSession, the constructor reconcilePeers calls, over table. Its loop is
// stopped at once, so the test drives the handshake inline as the dispatch goroutine does.
func wiringResponder(t *testing.T, table *SATable, name, remote string) *PeerSession {
	t.Helper()
	peer := ipsec.SiteToSitePeer{
		Name: name, IKEGroup: "test-ike", ESPGroup: "test-esp",
		ConnectionType: ipsec.ConnectionRespond,
		LocalAddress:   "10.0.0.2", RemoteAddress: remote,
		Auth: ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: wiringPSK},
	}
	ps := startPeerSession(name, peer, testIKEGroup(), testESPGroup(), table, nil, nil, nil, slogutil.DiscardLogger())
	close(ps.stopCh)
	<-ps.done
	return ps
}

// wiringHandshake runs IKE_SA_INIT and IKE_AUTH from a remote initiator at remote that
// proposes TSi tsi and TSr 0.0.0.0/0, into ps through tryResponderSAInit and
// handleAuthRequest. It returns the remote's SA, Ze's SA, and the decrypted answer.
func wiringHandshake(t *testing.T, table *SATable, ps *PeerSession, remote, tsi string) (ini, resp *SA, answer []wire.PayloadEntry) {
	t.Helper()
	log := slogutil.DiscardLogger()
	peer := ipsec.SiteToSitePeer{
		Name: ps.peerName, IKEGroup: "test-ike", ESPGroup: "test-esp",
		ConnectionType: ipsec.ConnectionInitiate,
		LocalAddress:   remote, RemoteAddress: "10.0.0.2",
		Auth:             ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: wiringPSK},
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{selectorPolicy(t, "0.0.0.0/0", tsi, 0, 0)},
	}
	ini, err := newInitiatorSA(ps.peerName, peer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	req := buildSAInitRequest(ini, testIKEGroup())
	ini.InitiatorSAInitMsg = req
	ini.State = StateSAInitSent
	var iSPI, rSPI [8]byte
	copy(iSPI[:], ini.InitiatorSPI[:])
	pkt := transport.Packet{Data: req, RemoteAddr: &net.UDPAddr{IP: net.ParseIP(remote), Port: 500}}
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatalf("peer %s: IKE_SA_INIT not accepted", ps.peerName)
	}
	resp = ps.getSA()
	if resp == nil || resp.State != StateSAInitReceived {
		t.Fatalf("peer %s: no responder SA at sa-init-received", ps.peerName)
	}
	iniTable := NewSATable()
	iniTable.Insert(ini)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, iniTable, nil, nil, log)
	if ini.State != StateAuthSent {
		t.Fatalf("peer %s: initiator state %v, want auth-sent", ps.peerName, ini.State)
	}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	if resp.State != StateEstablished {
		t.Fatalf("peer %s: responder state %v, want established", ps.peerName, resp.State)
	}
	return ini, resp, mbDecrypt(t, ini, resp.LastSentMsg)
}

// carriesTSUnacceptable reports whether an answer carries the TS_UNACCEPTABLE notify.
func carriesTSUnacceptable(entries []wire.PayloadEntry) bool {
	for i := range entries {
		if n, ok := entries[i].Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == wire.NotifyTSUnacceptable {
			return true
		}
	}
	return false
}

// wiringChildRefused has peer b propose a's inner address 10.1.2.3/32 in a
// CREATE_CHILD_SA on sa, and requires the refusal with nothing installed.
func wiringChildRefused(t *testing.T, table *SATable, sa *SA, what string) {
	t.Helper()
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x01020304)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(4)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.1.2.3/32")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")},
	}
	dp := &mockDP{}
	_, child, err := respondNewChild(sa, inner, testESPGroup(), 7, dp, slogutil.DiscardLogger())
	if child != nil {
		t.Fatalf("%s: respondNewChild built a Child SA for peer b", what)
	}
	mustInnerRefused(t, err, what)
	if len(dp.sas) != 0 || len(dp.policies) != 0 {
		t.Fatalf("%s: peer b's dataplane got %d states and %d policies, want none", what, len(dp.sas), len(dp.policies))
	}
	if got := table.innerSelectors.holdings(); got != 1 {
		t.Fatalf("%s: registry holds %d Child SAs, want 1 (peer a)", what, got)
	}
}

// wiringIKERekey rekeys the IKE SA between Ze's sa and the remote's ini. Ze initiates
// it when zeStarts is set, and answers it otherwise. It returns Ze's replacement and
// the remote's.
func wiringIKERekey(t *testing.T, sa, ini *SA, zeStarts bool) (zeNext, iniNext *SA) {
	t.Helper()
	log := slogutil.DiscardLogger()
	starter, answerer := ini, sa
	if zeStarts {
		starter, answerer = sa, ini
	}
	reqBytes, pending, err := initiateIKERekey(starter, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	reqMsg := parseMsg(t, reqBytes)
	reqInner, err := decryptAndParse(answerer, reqMsg, reqBytes)
	if err != nil {
		t.Fatalf("decrypt the IKE rekey request: %v", err)
	}
	respBytes, answered, err := respondIKERekey(answerer, reqInner, reqMsg.Header.MessageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	respInner, err := decryptAndParse(starter, parseMsg(t, respBytes), respBytes)
	if err != nil {
		t.Fatalf("decrypt the IKE rekey response: %v", err)
	}
	started, err := applyIKERekeyResponse(starter, pending, respInner, log)
	if err != nil {
		t.Fatalf("applyIKERekeyResponse: %v", err)
	}
	if zeStarts {
		return started, answered
	}
	return answered, started
}

// TestRFC3948ResponderSessionsShareInnerRegistry: two respond-only peers built from one
// SATable by startPeerSession. Method: peer a's remote completes IKE_SA_INIT
// (tryResponderSAInit) and IKE_AUTH proposing 10.1.2.3/32 and is served; peer b's remote
// then proposes the same address and its IKE_AUTH answer carries TS_UNACCEPTABLE. b's IKE
// SA is then rekeyed, first by Ze (applyIKERekeyResponse) and then by the peer
// (respondIKERekey), and after each a CREATE_CHILD_SA of b proposing 10.1.2.3/32 on the
// replacement SA must be refused, the registry holding a's Child SA alone throughout.
func TestRFC3948ResponderSessionsShareInnerRegistry(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- two respond-only peer sessions built by startPeerSession over one SATable: once peer a's IKE_AUTH through tryResponderSAInit is served with inner address 10.1.2.3/32, peer b's IKE_AUTH proposing it is answered TS_UNACCEPTABLE, and after b's IKE SA is rekeyed by Ze and then by the peer, b's CREATE_CHILD_SA proposing it on each replacement SA is refused with nothing installed, the registry holding a's Child SA alone.
	admitWithoutCookieChallenge(t)
	table := NewSATable()
	psA := wiringResponder(t, table, "a", "10.0.0.1")
	psB := wiringResponder(t, table, "b", "10.0.0.3")
	setActivePeers(map[string]*PeerSession{"a": psA, "b": psB})
	t.Cleanup(func() { setActivePeers(nil) })

	_, _, answerA := wiringHandshake(t, table, psA, "10.0.0.1", "10.1.2.3/32")
	if carriesTSUnacceptable(answerA) {
		t.Fatal("peer a, the first to propose 10.1.2.3/32, was answered TS_UNACCEPTABLE")
	}
	if got := table.innerSelectors.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs after peer a's IKE_AUTH, want 1", got)
	}

	iniB, respB, answerB := wiringHandshake(t, table, psB, "10.0.0.3", "10.1.2.3/32")
	if !carriesTSUnacceptable(answerB) {
		t.Fatal("peer b proposing peer a's inner address 10.1.2.3/32 was not answered TS_UNACCEPTABLE")
	}
	if got := table.innerSelectors.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs after peer b's IKE_AUTH, want 1 (peer a)", got)
	}

	zeRekeyed, iniRekeyed := wiringIKERekey(t, respB, iniB, true)
	wiringChildRefused(t, table, zeRekeyed, "peer b after the IKE SA rekey Ze initiated")
	peerRekeyed, _ := wiringIKERekey(t, zeRekeyed, iniRekeyed, false)
	wiringChildRefused(t, table, peerRekeyed, "peer b after the IKE SA rekey the peer initiated")
}

// wiringHeldByA builds the SATable whose registry records peer a at 10.1.2.3/32, and
// peer b's respond-only session over it through startPeerSession. It then runs b's
// IKE_SA_INIT and IKE_AUTH proposing 10.1.2.3/32 and requires the TS_UNACCEPTABLE answer.
// It returns the table, the remote's SA and Ze's.
func wiringHeldByA(t *testing.T) (table *SATable, iniB, respB *SA) {
	t.Helper()
	admitWithoutCookieChallenge(t)
	table = NewSATable()
	mustInnerChild(t, innerPeerSA(t, table.innerSelectors, "a", "198.18.0.7"), &mockDP{},
		[]string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	psB := wiringResponder(t, table, "b", "10.0.0.3")
	setActivePeers(map[string]*PeerSession{"b": psB})
	t.Cleanup(func() { setActivePeers(nil) })
	iniB, respB, answer := wiringHandshake(t, table, psB, "10.0.0.3", "10.1.2.3/32")
	if !carriesTSUnacceptable(answer) {
		t.Fatal("peer b proposing peer a's inner address 10.1.2.3/32 was not answered TS_UNACCEPTABLE")
	}
	if got := table.innerSelectors.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs after peer b's IKE_AUTH, want 1 (peer a)", got)
	}
	return table, iniB, respB
}

// TestRFC3948ResponderSAInitRefusesHeldInnerAddress: the responder IKE SA that
// tryResponderSAInit creates for a session is compared with the table's registry.
// Method: peer a is recorded at 10.1.2.3/32, then peer b's remote completes IKE_SA_INIT
// and IKE_AUTH proposing it through b's startPeerSession session.
func TestRFC3948ResponderSAInitRefusesHeldInnerAddress(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- with peer a recorded at inner address 10.1.2.3/32, a respond-only peer b whose IKE SA tryResponderSAInit creates is answered TS_UNACCEPTABLE at IKE_AUTH when it proposes 10.1.2.3/32, the registry holding a's record alone.
	wiringHeldByA(t)
}

// TestRFC3948ZeIKERekeyKeepsInnerRegistry: the IKE SA Ze's own rekey builds
// (applyIKERekeyResponse) is compared with the table's registry. Method: after b's
// refused IKE_AUTH, Ze rekeys b's IKE SA, and b's CREATE_CHILD_SA proposing a's inner
// address on the replacement must be refused.
func TestRFC3948ZeIKERekeyKeepsInnerRegistry(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- with peer a recorded at inner address 10.1.2.3/32, after Ze rekeys peer b's IKE SA (applyIKERekeyResponse), b's CREATE_CHILD_SA proposing 10.1.2.3/32 on the replacement SA is answered TS_UNACCEPTABLE with nothing installed, the registry holding a's record alone.
	table, iniB, respB := wiringHeldByA(t)
	zeRekeyed, _ := wiringIKERekey(t, respB, iniB, true)
	wiringChildRefused(t, table, zeRekeyed, "peer b after the IKE SA rekey Ze initiated")
}

// TestRFC3948PeerIKERekeyKeepsInnerRegistry: the IKE SA Ze builds answering the peer's
// rekey (respondIKERekey) is compared with the table's registry. Method: after b's
// refused IKE_AUTH, b's remote rekeys the IKE SA, and b's CREATE_CHILD_SA proposing a's
// inner address on the replacement must be refused.
func TestRFC3948PeerIKERekeyKeepsInnerRegistry(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- with peer a recorded at inner address 10.1.2.3/32, after peer b's remote rekeys b's IKE SA (respondIKERekey), b's CREATE_CHILD_SA proposing 10.1.2.3/32 on the replacement SA is answered TS_UNACCEPTABLE with nothing installed, the registry holding a's record alone.
	table, iniB, respB := wiringHeldByA(t)
	peerRekeyed, _ := wiringIKERekey(t, respB, iniB, false)
	wiringChildRefused(t, table, peerRekeyed, "peer b after the IKE SA rekey the peer initiated")
}

// TestRFC3948InitiatorSessionRefusesHeldInnerAddress: an initiate peer built by
// startPeerSession over the SATable whose registry already records peer a at
// 10.1.2.3/32. Method: the session's own runInitiator drives IKE_SA_INIT and IKE_AUTH
// against a far end answered in process (icyFarEnd), proposing TSr 10.1.2.3/32; once the
// owner loop adopts the IKE SA, the session must hold no Child SA and the registry a's
// record alone.
func TestRFC3948InitiatorSessionRefusesHeldInnerAddress(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- an initiate peer session built by startPeerSession over an SATable whose registry records peer a at inner address 10.1.2.3/32 establishes its IKE SA through runInitiator proposing 10.1.2.3/32 and adopts no Child SA, the registry holding a's record alone.
	log := slogutil.DiscardLogger()
	ikeGroup, espGroup := testIKEGroup(), testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, wiringPSK)
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	iniPeer.TrafficSelectors = []ipsec.TrafficSelectorPolicy{selectorPolicy(t, "10.1.2.3/32", "0.0.0.0/0", 0, 0)}

	table := NewSATable()
	mustInnerChild(t, innerPeerSA(t, table.innerSelectors, "a", "198.18.0.7"), &mockDP{},
		[]string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})

	respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	far := &icyFarEnd{}
	var session atomic.Pointer[PeerSession]
	ready := make(chan struct{})
	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	afterFunc = func(time.Duration) <-chan time.Time {
		<-ready
		if cur := session.Load().getSA(); cur != nil {
			far.advance(cur, respPS, table, log)
		}
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}
	ps := startPeerSession("ze", iniPeer, ikeGroup, espGroup, table, nil, nil, nil, log)
	session.Store(ps)
	close(ready)

	icyWaitFor(t, "the owner loop adopting the established SA", func() bool {
		return ps.ownedSA.Load() != nil
	})
	child := ps.getChildSA()
	holdings := table.innerSelectors.holdings()
	close(ps.stopCh)
	<-ps.done
	if _, err := far.get(); err != nil {
		t.Fatalf("the driven handshake failed: %v", err)
	}
	if child != nil {
		t.Fatal("the initiator session adopted a Child SA for 10.1.2.3/32, which peer a holds")
	}
	if holdings != 1 {
		t.Fatalf("registry holds %d Child SAs, want 1 (peer a)", holdings)
	}
}
