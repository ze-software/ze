package engine

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: review Finding 1 (BLOCKER). reapStaleHandshake tears down a half-open
// responder handshake the peer abandoned (stuck pre-established past the timeout),
// freeing responderBusy + the SATable slot so the peer can reconnect; a fresh
// handshake is left alone.
// PREVENTS: a peer crash/restart after IKE_SA_INIT permanently wedging that peer.
func TestReapStaleHandshake(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()
	ps := &PeerSession{peerName: "ze"}
	ps.responderBusy.Store(true)

	sa := testSA()
	sa.IsInitiator = false
	sa.State = StateSAInitReceived
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	sa.ResponderSPI = [8]byte{9, 8, 7, 6, 5, 4, 3, 2}
	table.Insert(sa)
	ps.setSA(sa)

	// Fresh handshake: not reaped, state untouched.
	sa.CreatedAt = time.Now()
	if ps.reapStaleHandshake(sa, table, log) {
		t.Fatal("fresh handshake must not be reaped")
	}
	if table.Len() != 1 || !ps.responderBusy.Load() {
		t.Fatal("fresh handshake state disturbed")
	}

	// Stale but the dispatch goroutine just established it (race window): NOT reaped,
	// so the tunnel is not orphaned. runResponder adopts it next tick.
	sa.CreatedAt = time.Now().Add(-2 * responderHandshakeTimeout)
	sa.State = StateEstablished
	if ps.reapStaleHandshake(sa, table, log) {
		t.Fatal("a just-established (even if stale-timestamped) SA must not be reaped")
	}
	if table.Len() != 1 || !ps.responderBusy.Load() || ps.getSA() == nil {
		t.Fatal("established SA state disturbed by reap")
	}
	sa.State = StateSAInitReceived // back to a genuinely abandoned handshake

	// Stale handshake: reaped, table + responderBusy + ps.sa cleared.
	if !ps.reapStaleHandshake(sa, table, log) {
		t.Fatal("stale handshake must be reaped")
	}
	if table.Len() != 0 {
		t.Errorf("SATable not cleared: %d entries", table.Len())
	}
	if ps.responderBusy.Load() {
		t.Error("responderBusy not reset — peer would stay wedged")
	}
	if ps.getSA() != nil {
		t.Error("ps.sa not cleared")
	}
}

// VALIDATES: review Finding 2 (ISSUE). selectResponderESP narrows sa.ESPGroup to the
// single ESP proposal the peer offered that we accept (RFC 7296 Section 2.7), returns
// NO_PROPOSAL_CHOSEN when none match, and is a no-op for a nil SAi2 (EAP final).
// PREVENTS: the responder emitting multiple proposals in SAr2, or installing the
// wrong (Proposals[0]) algorithm, for a multi-proposal esp-group.
func TestResponderSelectsESPProposal(t *testing.T) {
	twoProp := ipsec.ESPGroup{Proposals: []ipsec.ESPProposal{
		{Number: 1, Encryption: ipsec.EncryptionAES128, Hash: ipsec.HashSHA256},
		{Number: 2, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256},
	}}

	// Peer offers only aes256 → narrow to the aes256 proposal.
	peerWire := buildWireESPProposals(ipsec.ESPGroup{Proposals: []ipsec.ESPProposal{
		{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256},
	}}, 0x11223344, dhGroupNone)
	sa := testSA()
	sa.IsInitiator = false
	sa.ESPGroup = twoProp
	if err := selectResponderESP(sa, &wire.PayloadSA{Proposals: peerWire}); err != nil {
		t.Fatalf("selectResponderESP: %v", err)
	}
	if len(sa.ESPGroup.Proposals) != 1 {
		t.Fatalf("esp-group not narrowed to one proposal: got %d", len(sa.ESPGroup.Proposals))
	}
	if sa.ESPGroup.Proposals[0].Encryption != ipsec.EncryptionAES256 {
		t.Errorf("narrowed to %v, want aes256", sa.ESPGroup.Proposals[0].Encryption)
	}

	// Peer offers only aes256gcm (AEAD) — our CBC group has no match.
	gcmWire := buildWireESPProposals(ipsec.ESPGroup{Proposals: []ipsec.ESPProposal{
		{Number: 1, Encryption: ipsec.EncryptionAES256GCM},
	}}, 0x55667788, dhGroupNone)
	saNo := testSA()
	saNo.IsInitiator = false
	saNo.ESPGroup = twoProp
	if err := selectResponderESP(saNo, &wire.PayloadSA{Proposals: gcmWire}); err == nil {
		t.Error("expected NO_PROPOSAL_CHOSEN for a non-matching ESP offer")
	}

	// nil SAi2 (EAP final): no-op, group unchanged.
	saNil := testSA()
	saNil.ESPGroup = twoProp
	if err := selectResponderESP(saNil, nil); err != nil || len(saNil.ESPGroup.Proposals) != 2 {
		t.Error("nil remoteSAi2 must be a no-op leaving the group unchanged")
	}
}

// VALIDATES: review Finding 3 (ISSUE). setSA/getSA are mutex-safe under concurrent
// access, so the terminate/reconcile readers (now routed through getSA) cannot race
// the dispatch goroutine's setSA. Run under `go test -race`.
func TestSetGetSAConcurrent(t *testing.T) {
	ps := &PeerSession{peerName: "ze"}
	sa := testSA()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 2000 {
			ps.setSA(sa)
			ps.setSA(nil)
		}
	}()
	go func() {
		defer wg.Done()
		for range 2000 {
			_ = ps.getSA()
		}
	}()
	wg.Wait()
}

// VALIDATES: review Finding 4 (ISSUE). setPendingIKESwap clears the key material of a
// prior unconfirmed pending IKE SA before replacing it, so a peer re-initiating an IKE
// rekey before Deleting the old SA cannot leak keys.
func TestSetPendingIKESwapClearsSuperseded(t *testing.T) {
	ps := &PeerSession{peerName: "ze"}
	old := &SA{SKKeys: &ikecrypto.SKKeys{SK_ei: []byte{1, 2, 3, 4}, SK_d: []byte{5, 6, 7, 8}}}
	ps.pendingIKESwap = old

	newSA := &SA{SKKeys: &ikecrypto.SKKeys{SK_ei: []byte{9, 9, 9, 9}}}
	ps.setPendingIKESwap(newSA)

	for _, b := range old.SKKeys.SK_ei {
		if b != 0 {
			t.Error("superseded SK_ei not cleared")
			break
		}
	}
	for _, b := range old.SKKeys.SK_d {
		if b != 0 {
			t.Error("superseded SK_d not cleared")
			break
		}
	}
	if ps.pendingIKESwap != newSA {
		t.Error("pendingIKESwap not updated to the new SA")
	}
}

// responderTestPeers returns matching initiator/responder peer configs for an
// in-process PSK handshake (each sees the other as "remote").
func responderTestPeers(mode ipsec.AuthMode, psk string) (ini, resp ipsec.SiteToSitePeer) {
	auth := ipsec.AuthConfig{Mode: mode, PSK: psk}
	ini = ipsec.SiteToSitePeer{
		Name: "ze", IKEGroup: "test-ike", ESPGroup: "test-esp",
		ConnectionType: ipsec.ConnectionInitiate,
		LocalAddress:   "10.0.0.1", RemoteAddress: "10.0.0.2", Auth: auth,
	}
	resp = ipsec.SiteToSitePeer{
		Name: "ze", IKEGroup: "test-ike", ESPGroup: "test-esp",
		ConnectionType: ipsec.ConnectionRespond,
		LocalAddress:   "10.0.0.2", RemoteAddress: "10.0.0.1", Auth: auth,
	}
	return ini, resp
}

func parseMsg(t *testing.T, raw []byte) *wire.Message {
	t.Helper()
	var m wire.Message
	if err := m.ReadFrom(raw); err != nil {
		t.Fatalf("parse message: %v", err)
	}
	return &m
}

// VALIDATES: an IP-literal identity is encoded as ID_IPV4_ADDR/ID_IPV6_ADDR (packed
// address bytes), and any other string as ID_FQDN. Peers constrain the remote id by
// type, so an IP value sent as FQDN is rejected ("constraint check failed").
func TestEncodeIKEID(t *testing.T) {
	cases := []struct {
		id       string
		wantType uint8
		wantLen  int
	}{
		{"172.28.0.2", wire.IDTypeIPv4Addr, 4},
		{"10.0.0.1", wire.IDTypeIPv4Addr, 4},
		{"2001:db8::1", wire.IDTypeIPv6Addr, 16},
		{"testuser", wire.IDTypeFQDN, len("testuser")},
		{"gw.example.com", wire.IDTypeFQDN, len("gw.example.com")},
	}
	for _, c := range cases {
		gotType, gotData := encodeIKEID(c.id)
		if gotType != c.wantType {
			t.Errorf("encodeIKEID(%q) type = %d, want %d", c.id, gotType, c.wantType)
		}
		if len(gotData) != c.wantLen {
			t.Errorf("encodeIKEID(%q) data len = %d, want %d", c.id, len(gotData), c.wantLen)
		}
	}
}

// establishPSK runs a full in-process PSK handshake and returns both established
// SAs (initiator and responder) sharing one SK hierarchy, plus the responder's ps.
func establishPSK(t *testing.T) (ini, resp *SA, ps *PeerSession) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "rekey-psk")

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	ps = &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	handleAuthResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, log)
	if ini.State != StateEstablished || resp.State != StateEstablished {
		t.Fatalf("establishPSK failed: ini=%v resp=%v", ini.State, resp.State)
	}
	return ini, resp, ps
}

// VALIDATES: AC-5. A peer-initiated IKE-SA rekey where Ze is the responder. The
// initiator (established SA) sends CREATE_CHILD_SA rekeying the IKE SA; respondIKERekey
// derives the new keys and replies; the initiator's applyIKERekeyResponse accepts it.
// Both new SAs must share one SK hierarchy with cross-matching SPIs and opposite
// roles (initiator=true / responder=false). This closes spec-ipsec-13's deferral.
// PREVENTS: the responder deriving wrong rekey keys or the wrong SK direction on the
// new IKE SA (the reason peer IKE rekeys were previously dropped).
// RFC requirement: RFC7296-1.3.3-1 positive -- rekeying the IKE SA carries a mandatory KE
// payload: initiateIKERekey (rekey.go:292) emits SA+Ni+KEi and respondIKERekey completes a
// real DH from it, so the decrypted rekey request contains a KE payload and the exchange
// derives fresh keys end to end.
func TestRespondIKERekey(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniSA, respSA, _ := establishPSK(t)
	ikeGroup := testIKEGroup()

	// Initiator initiates the IKE-SA rekey.
	reqBytes, pending, err := initiateIKERekey(iniSA, ikeGroup)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	// Responder decrypts and responds.
	reqInner, err := decryptAndParse(respSA, parseMsg(t, reqBytes), reqBytes)
	if err != nil {
		t.Fatalf("responder decrypt rekey request: %v", err)
	}
	// KE is mandatory when rekeying the IKE SA (RFC 7296 §1.3.3): the request must carry one.
	haveKE := false
	for i := range reqInner {
		if _, ok := reqInner[i].Payload.(*wire.PayloadKE); ok {
			haveKE = true
		}
	}
	if !haveKE {
		t.Fatal("IKE-SA rekey request carried no KE payload (KE is mandatory for IKE rekey)")
	}
	respBytes, newRespSA, err := respondIKERekey(respSA, reqInner, 2, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	// Initiator applies the response.
	respInner, err := decryptAndParse(iniSA, parseMsg(t, respBytes), respBytes)
	if err != nil {
		t.Fatalf("initiator decrypt rekey response: %v", err)
	}
	newIniSA, err := applyIKERekeyResponse(iniSA, pending, respInner, log)
	if err != nil {
		t.Fatalf("applyIKERekeyResponse: %v", err)
	}

	if !bytes.Equal(newIniSA.SKKeys.SK_ei, newRespSA.SKKeys.SK_ei) ||
		!bytes.Equal(newIniSA.SKKeys.SK_d, newRespSA.SKKeys.SK_d) {
		t.Fatal("rekeyed initiator and responder derived different SK keys")
	}
	if !newIniSA.IsInitiator {
		t.Error("rekey initiator's new SA must have IsInitiator=true (it sent Ni)")
	}
	if newRespSA.IsInitiator {
		t.Error("rekey responder's new SA must have IsInitiator=false")
	}
	if newIniSA.InitiatorSPI != newRespSA.InitiatorSPI || newIniSA.ResponderSPI != newRespSA.ResponderSPI {
		t.Error("rekeyed IKE SA SPIs do not match across peers")
	}

	// The two new SAs must interoperate: a message sealed by one decrypts on the other.
	marker := []byte("post-rekey-roundtrip-nonce-32byte")
	sealed, err := buildEncryptedMessageEx(newRespSA, []wire.PayloadEntry{{Payload: &wire.PayloadNonce{NonceData: marker}}}, 0, wire.ExchangeInformational, wire.FlagResponse)
	if err != nil {
		t.Fatalf("seal on rekeyed responder SA: %v", err)
	}
	got := decryptOneNonce(t, newIniSA, sealed)
	if !bytes.Equal(got, marker) {
		t.Error("rekeyed initiator could not decrypt a message from the rekeyed responder")
	}
}

// VALIDATES: AC-2, AC-3, and the A-3 computeSignedOctets fix. Runs a complete
// initiator<->responder IKE_SA_INIT + IKE_AUTH (PSK) handshake entirely in process,
// feeding each side's built message to the other. Both sides reaching
// StateEstablished proves: the responder decrypts the initiator's SK-encrypted
// IKE_AUTH (recv key = SK_ei), the responder verifies the initiator's AUTH (correct
// initiator signed octets from a responder SA), the initiator decrypts the
// responder's SK-encrypted response (responder send key = SK_er), and the initiator
// verifies the responder's AUTH (correct responder signed octets). It needs no
// network or XFRM, so it is the primary responder correctness gate on any host.
// PREVENTS: every direction bug the responder could have (SK send/recv, AUTH nonce
// order, AUTH ID selection, Child KEYMAT nonce order).
// RFC requirement: RFC7296-4-5 positive -- this is the four-message exchange Section 4
// makes mandatory, and nothing else runs here. buildSAInitRequest emits request 1,
// handleSAInitRequest emits response 2, handleSAInitResponse emits request 3, and
// ps.handleAuthRequest emits response 4, which handleAuthResponse consumes. Two SAs come
// out of it: the IKE SA (both sides at StateEstablished over one agreed SK hierarchy) and
// the ESP SA (the responder's installed Child SA, whose KEYMAT and ESP SPIs cross-match the
// initiator's). The negative half is TestFourmFirstPairEstablishesNeitherSA
// (rfc7296_fourmessage_test.go).
func TestResponderHandshakePSKEndToEnd(t *testing.T) {
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "shared-secret-123")

	table := NewSATable()

	// Initiator: build and "send" IKE_SA_INIT.
	iniSA, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(iniSA)
	saInitReq := buildSAInitRequest(iniSA, ikeGroup)
	iniSA.InitiatorSAInitMsg = saInitReq
	iniSA.State = StateSAInitSent

	// Responder: accept IKE_SA_INIT, produce the response.
	respSA, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, iniSA.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(respSA, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	if respSA.State != StateSAInitReceived {
		t.Fatalf("responder state after SA_INIT = %v, want sa-init-responded", respSA.State)
	}
	if respSA.NATDetected {
		t.Error("responder wrongly detected NAT for a direct exchange")
	}
	saInitResp := respSA.LastSentMsg
	if len(saInitResp) == 0 {
		t.Fatal("responder produced no IKE_SA_INIT response")
	}

	// Initiator: process the response, produce IKE_AUTH.
	handleSAInitResponse(iniSA, parseMsg(t, saInitResp), saInitResp, table, nil, nil, log)
	if iniSA.State != StateAuthSent {
		t.Fatalf("initiator state after SA_INIT response = %v, want auth-sent", iniSA.State)
	}
	if iniSA.NATDetected {
		t.Error("initiator wrongly detected NAT for a direct exchange")
	}
	authReq := iniSA.LastSentMsg

	// Responder: verify the initiator's AUTH, install child, produce IKE_AUTH response.
	ps := &PeerSession{peerName: "ze", espGroup: espGroup}
	ps.handleAuthRequest(respSA, parseMsg(t, authReq), authReq, nil, nil, log)
	if respSA.State != StateEstablished {
		t.Fatalf("responder state after IKE_AUTH = %v, want established (initiator AUTH must verify)", respSA.State)
	}
	authResp := respSA.LastSentMsg

	// Initiator: verify the responder's AUTH -> established.
	handleAuthResponse(iniSA, parseMsg(t, authResp), authResp, table, nil, log)
	if iniSA.State != StateEstablished {
		t.Fatalf("initiator state after IKE_AUTH response = %v, want established (responder AUTH must verify)", iniSA.State)
	}

	// Both sides derived the same SK hierarchy (shared DH + absolute Ni|Nr).
	if !bytes.Equal(iniSA.SKKeys.SK_ei, respSA.SKKeys.SK_ei) || !bytes.Equal(iniSA.SKKeys.SK_d, respSA.SKKeys.SK_d) {
		t.Fatal("initiator and responder derived different SK keys")
	}

	// Child SA keying agrees: the initiator's outbound (EncryptKeyI) equals the
	// responder's inbound key, and the negotiated ESP SPIs cross-match.
	respChild := ps.getChildSA()
	if respChild == nil {
		t.Fatal("responder did not install a child SA")
	}
	iniChild, err := createFirstChildSA(iniSA, espGroup, iniPeer.LocalAddress, iniPeer.RemoteAddress, 1, nil, log)
	if err != nil {
		t.Fatalf("initiator createFirstChildSA: %v", err)
	}
	if !bytes.Equal(iniChild.Keys.EncryptKeyI, respChild.Keys.EncryptKeyI) {
		t.Error("initiator and responder derived different Child KEYMAT")
	}
	if iniChild.InboundSPI != respChild.OutboundSPI || iniChild.OutboundSPI != respChild.InboundSPI {
		t.Errorf("child ESP SPIs do not cross-match: ini in=%d out=%d resp in=%d out=%d",
			iniChild.InboundSPI, iniChild.OutboundSPI, respChild.InboundSPI, respChild.OutboundSPI)
	}
	if respChild.LocalIsInitiator {
		t.Error("responder child must have LocalIsInitiator=false")
	}
	iniChild.Clear()
	respChild.Clear()
}

// VALIDATES: AC-2. handleSAInitRequest on a fresh responder SA negotiates a
// proposal, derives keys, and emits a well-formed IKE_SA_INIT response (FlagResponse
// set, FlagInitiator clear, our ResponderSPI present).
func TestResponderCreatesSAOnSAInit(t *testing.T) {
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "k")

	iniSA, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	saInitReq := buildSAInitRequest(iniSA, ikeGroup)

	respSA, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, iniSA.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	if respSA.IsInitiator {
		t.Fatal("responder SA must have IsInitiator=false")
	}
	handleSAInitRequest(respSA, parseMsg(t, saInitReq), saInitReq, nil, nil, log)

	if respSA.State != StateSAInitReceived {
		t.Fatalf("state = %v, want sa-init-responded", respSA.State)
	}
	if respSA.SKKeys == nil {
		t.Fatal("responder derived no SK keys")
	}
	if len(respSA.ResponderSAInitMsg) == 0 {
		t.Fatal("responder did not store its SA_INIT response")
	}
	resp := parseMsg(t, respSA.LastSentMsg)
	if resp.Header.ExchangeType != wire.ExchangeIKESAInit {
		t.Errorf("response exchange = %d, want IKE_SA_INIT", resp.Header.ExchangeType)
	}
	if resp.Header.Flags&wire.FlagResponse == 0 {
		t.Error("response must set FlagResponse")
	}
	if resp.Header.Flags&wire.FlagInitiator != 0 {
		t.Error("responder response must NOT set FlagInitiator")
	}
	if resp.Header.ResponderSPI == ([8]byte{}) {
		t.Error("response must carry a non-zero responder SPI")
	}
}

// VALIDATES: AC-4. A full EAP-MSCHAPv2 handshake with Ze as the EAP authenticator,
// driving the real eap.Session (server) against the real eap.PeerSession (client)
// through the IKE layer entirely in process. Reaching StateEstablished on both sides
// proves: sa.EAPSession holds a *eap.Session, Begin/Process drive the EAP rounds, the
// server's own AUTH is verified by the client, the MSK-derived AUTH is exchanged and
// verified both ways, and the first Child SA installs. No RADIUS, network, or XFRM.
// PREVENTS: EAP-server desync, MSK-AUTH direction bugs, and the "newEAPSession has
// zero callers" regression.
func TestResponderEAPSessionWired(t *testing.T) {
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	// Auth.PSK doubles as the EAP-MSCHAPv2 password (client + server) and the
	// server's long-term IKE credential for its first-message AUTH.
	// RFC 7296 Section 2.16: an EAP exchange authenticates the initiator, and the
	// responder authenticates back with a public-key signature. The responder
	// therefore needs a certificate, and the initiator needs the CA that signed
	// it. Before this fixture carried them, the responder answered with a
	// pre-shared-key AUTH, which is the violation computeServerAuth now refuses.
	eapTLSResponderPKI(t, "eap-wired-ca", "eap-wired-cert")
	iniPeer, respPeer := responderTestPeers(ipsec.AuthEAPMSCHAPv2, "s3cr3t-pass")
	iniPeer.Auth.Certificate = "eap-wired-cert"
	iniPeer.Auth.CACertificate = "eap-wired-ca"
	respPeer.Auth.Certificate = "eap-wired-cert"
	respPeer.Auth.CACertificate = "eap-wired-ca"

	table := NewSATable()
	iniSA, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(iniSA)
	saInitReq := buildSAInitRequest(iniSA, ikeGroup)
	iniSA.InitiatorSAInitMsg = saInitReq
	iniSA.State = StateSAInitSent

	respSA, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, iniSA.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.setSA(respSA)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	// SA_INIT exchange.
	handleSAInitRequest(respSA, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(iniSA, parseMsg(t, respSA.LastSentMsg), respSA.LastSentMsg, table, nil, nil, log)
	// The initiator's IKE_AUTH here is EAP-style (IDi, SAi2, TSi, TSr, no AUTH).

	// Drive the alternating IKE_AUTH/EAP rounds until both sides establish.
	cur := iniSA.LastSentMsg
	toResponder := true
	established := false
	for round := range 24 {
		if toResponder {
			handleInbound(respSA, transport.Packet{Data: cur}, table, nil, log)
			cur = respSA.LastSentMsg
		} else {
			handleInbound(iniSA, transport.Packet{Data: cur}, table, nil, log)
			cur = iniSA.LastSentMsg
		}
		toResponder = !toResponder
		if respSA.State == StateDead || iniSA.State == StateDead {
			t.Fatalf("EAP handshake died at round %d (ini=%v resp=%v)", round, iniSA.State, respSA.State)
		}
		if iniSA.State == StateEstablished && respSA.State == StateEstablished {
			established = true
			break
		}
	}
	if !established {
		t.Fatalf("EAP handshake did not establish (ini=%v resp=%v)", iniSA.State, respSA.State)
	}

	sess, ok := respSA.EAPSession.(*eap.Session)
	if !ok || sess == nil {
		t.Fatal("sa.EAPSession must hold a *eap.Session (newEAPSession wired)")
	}
	if !sess.Succeeded() {
		t.Error("EAP server session did not record success")
	}
	if respSA.EAPMSK == ([64]byte{}) {
		t.Error("responder did not derive an EAP MSK")
	}
	if ps.getChildSA() == nil {
		t.Error("responder did not install a child SA after EAP")
	}
}

// VALIDATES: AC-6, AC-7. tryResponderSAInit accepts an unsolicited IKE_SA_INIT from
// a configured `respond` peer (creating exactly one SA and advancing it), drops a
// concurrent second attempt while busy, and drops an IKE_SA_INIT from an
// unconfigured source.
func TestRunResponderAcceptsInboundAndBounds(t *testing.T) {
	admitWithoutCookieChallenge(t)
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "k")

	table := NewSATable()
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	iniSA, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	saInitReq := buildSAInitRequest(iniSA, ikeGroup)
	// The responder peer's configured RemoteAddress is 10.0.0.1.
	fromPeer := transport.Packet{Data: saInitReq, RemoteAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 500}}
	fromStranger := transport.Packet{Data: saInitReq, RemoteAddr: &net.UDPAddr{IP: net.ParseIP("203.0.113.9"), Port: 500}}

	var iSPI, rSPI [8]byte
	copy(iSPI[:], saInitReq[0:8])

	// Unconfigured source: dropped, no SA created.
	if tryResponderSAInit(fromStranger, iSPI, rSPI, table, nil, log) {
		t.Error("IKE_SA_INIT from an unconfigured source must not be consumed as a responder handshake")
	}
	if table.Len() != 0 {
		t.Fatalf("stranger created %d SAs, want 0", table.Len())
	}

	// Configured peer: accepted, SA created and advanced through the handshake.
	if !tryResponderSAInit(fromPeer, iSPI, rSPI, table, nil, log) {
		t.Fatal("IKE_SA_INIT from the configured peer must be accepted")
	}
	if table.Len() != 1 {
		t.Fatalf("accepted %d SAs, want 1", table.Len())
	}
	if sa := ps.getSA(); sa == nil || sa.State != StateSAInitReceived {
		t.Fatalf("responder SA not advanced to sa-init-responded")
	}

	// Concurrent second attempt while busy: dropped, still one SA.
	tryResponderSAInit(fromPeer, iSPI, rSPI, table, nil, log)
	if table.Len() != 1 {
		t.Errorf("busy responder created %d SAs, want 1", table.Len())
	}
}

// ownedResponder returns a PeerSession whose established responder SA (old) is owned
// by the (simulated) owner loop -- ownedSA set, responderBusy cleared, in the SATable,
// registered as the active `respond` peer -- exactly the state a live tunnel is in
// just before a fresh IKE_SA_INIT arrives from the peer.
func ownedResponder(t *testing.T) (ps *PeerSession, old *SA, table *SATable) {
	t.Helper()
	// These tests are about what the responder does AFTER an initiation is admitted.
	// The COOKIE challenge is the admission gate, and rfc7296_cookie_test.go proves it.
	admitWithoutCookieChallenge(t)
	_, old, ps = establishPSK(t)
	ps.stopCh = make(chan struct{})
	ps.done = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, inboundQueueDepth)
	table = NewSATable()
	table.Insert(old)
	ps.setSA(old)
	ps.ownedSA.Store(old)         // maintainSA owns this SA
	ps.responderBusy.Store(false) // cleared at adoption, so a parallel init is accepted
	setActivePeers(map[string]*PeerSession{ps.peerName: ps})
	t.Cleanup(func() { setActivePeers(nil) })
	return ps, old, table
}

// freshInitiatorInit builds a brand-new initiator SA and its IKE_SA_INIT request from
// the configured peer's source address (10.0.0.1), for feeding tryResponderSAInit.
func freshInitiatorInit(t *testing.T) (*SA, transport.Packet) {
	t.Helper()
	ini, err := newInitiatorSA("ze", func() ipsec.SiteToSitePeer {
		p, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "rekey-psk")
		return p
	}(), testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	req := buildSAInitRequest(ini, testIKEGroup())
	ini.InitiatorSAInitMsg = req
	ini.State = StateSAInitSent
	pkt := transport.Packet{Data: req, RemoteAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 500}}
	return ini, pkt
}

// VALIDATES: AC-3 (accept-in-parallel). A fresh IKE_SA_INIT (new initiator SPI) that
// arrives while an established SA owns the loop is accepted as a PARALLEL half-open SA
// in the second slot; the established SA and its Child SA are NOT touched (RFC 7296
// Section 2.4: never act on the unauthenticated message). Red before the fix: the old
// responderBusy gate stayed true for the SA's whole life and dropped the init.
// RFC requirement: RFC7296-2.4-3 positive -- a fresh (unauthenticated) IKE_SA_INIT arriving
// while an established SA owns the loop is accepted IN PARALLEL as a half-open second SA
// (tryResponderSAInit, register.go:541); the established SA and its Child SA are left intact,
// so the responder never concludes the peer failed from an unauthenticated message.
func TestResponderAcceptsReinitAfterStaleSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, table := ownedResponder(t)
	oldChild := ps.getChildSA()
	if oldChild == nil {
		t.Fatal("setup: established responder has no child SA")
	}

	ini, pkt := freshInitiatorInit(t)
	var iSPI, rSPI [8]byte
	copy(iSPI[:], ini.InitiatorSPI[:])

	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("a fresh IKE_SA_INIT alongside an established SA must be accepted, not dropped")
	}

	// Established SA untouched (coexist, not supersede -- the RFC Section 2.4 assertion).
	if ps.getSA() != old {
		t.Error("established SA slot was clobbered by the parallel init")
	}
	if old.State != StateEstablished {
		t.Errorf("established SA state changed to %v on an unauthenticated init", old.State)
	}
	if ps.getChildSA() != oldChild {
		t.Error("established Child SA was disturbed by the parallel init")
	}

	// New SA present in the second slot, advanced, and in the table alongside the old.
	pending := ps.getPendingSA()
	if pending == nil || pending == old {
		t.Fatal("parallel init did not create a distinct pending SA")
	}
	if pending.State != StateSAInitReceived {
		t.Errorf("pending SA state = %v, want sa-init-responded", pending.State)
	}
	if table.Len() != 2 {
		t.Errorf("table has %d SAs, want 2 (old + parallel)", table.Len())
	}
	if !ps.responderBusy.Load() {
		t.Error("responderBusy must be set while the parallel half-open handshake is in flight")
	}
}

// VALIDATES: AC-6 / R-1 (the DoS guard). An IKE_SA_INIT that is accepted in parallel
// but never authenticates leaves the established SA and its Child SA untouched, and the
// half-open SA is reaped after responderHandshakeTimeout -- freeing responderBusy so a
// later genuine re-init is not wedged. No unauthenticated message ever deletes an SA.
// RFC requirement: RFC7296-2.4-3 negative -- an IKE_SA_INIT that is accepted in parallel but
// never authenticates does NOT delete or supersede the established SA: it is reaped after the
// handshake timeout while the old SA and its Child SA survive untouched, proving no
// unauthenticated message ever tears down the established SA.
func TestResponderKeepsOldSAOnUnauthenticatedInit(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, table := ownedResponder(t)
	oldChild := ps.getChildSA()

	ini, pkt := freshInitiatorInit(t)
	var iSPI, rSPI [8]byte
	copy(iSPI[:], ini.InitiatorSPI[:])
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("parallel init not accepted")
	}
	pending := ps.getPendingSA()
	if pending == nil {
		t.Fatal("no pending SA created")
	}

	// The peer abandons the handshake (never sends IKE_AUTH). Age it past the timeout
	// and let the owner-loop reaper run.
	pending.CreatedAt = time.Now().Add(-2 * responderHandshakeTimeout)
	ps.reapStalePending(time.Now(), table, nil, log)

	if ps.getPendingSA() != nil {
		t.Error("abandoned parallel handshake was not reaped")
	}
	if ps.responderBusy.Load() {
		t.Error("responderBusy not cleared after reaping -- a future re-init would wedge")
	}
	if table.Len() != 1 {
		t.Errorf("table has %d SAs after reap, want 1 (only the established SA)", table.Len())
	}
	// The established SA and its Child SA survived the whole unauthenticated episode.
	if ps.getSA() != old || old.State != StateEstablished {
		t.Error("established SA did not survive an unauthenticated parallel init")
	}
	if ps.getChildSA() != oldChild {
		t.Error("established Child SA did not survive an unauthenticated parallel init")
	}
}

// VALIDATES: AC-3 (supersede-on-authentication) + INITIAL_CONTACT emit/honor. A full
// parallel handshake that reaches IKE_AUTH: the responder honors the initiator's
// INITIAL_CONTACT, stages the new Child SA in the second slot, and signals the owner
// loop to supersede -- but only now, on the AUTHENTICATED message, and still without
// touching the old SA until the owner loop relinquishes it. RFC 7296 Section 2.4.
func TestResponderSupersedesOnAuthenticatedInit(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, table := ownedResponder(t)
	oldChild := ps.getChildSA()

	ini, pkt := freshInitiatorInit(t)
	var iSPI, rSPI [8]byte
	copy(iSPI[:], ini.InitiatorSPI[:])
	table1 := NewSATable()
	table1.Insert(ini)
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("parallel init not accepted")
	}
	pending := ps.getPendingSA()
	if pending == nil {
		t.Fatal("no pending SA")
	}

	// Initiator processes the responder's IKE_SA_INIT reply and builds IKE_AUTH.
	handleSAInitResponse(ini, parseMsg(t, pending.LastSentMsg), pending.LastSentMsg, table1, nil, nil, log)
	if ini.State != StateAuthSent {
		t.Fatalf("initiator state = %v, want auth-sent", ini.State)
	}
	// Responder processes the parallel IKE_AUTH -> it authenticates and establishes.
	handleInbound(pending, transport.Packet{Data: ini.LastSentMsg}, table, nil, log)

	if pending.State != StateEstablished {
		t.Fatalf("parallel SA state = %v, want established (its IKE_AUTH must verify)", pending.State)
	}
	if !pending.InitialContact {
		t.Error("responder did not honor the initiator's INITIAL_CONTACT (RFC 7296 Section 2.4)")
	}
	if ps.getPendingChild() == nil {
		t.Error("new Child SA not staged in the pending slot (make-before-break)")
	}
	if len(ps.supersede) != 1 {
		t.Error("owner loop was not signaled to supersede on the authenticated init")
	}
	// Until the owner loop relinquishes, the old SA and its child are still intact.
	if ps.getSA() != old || old.State != StateEstablished {
		t.Error("old SA was torn down on the parallel establishment instead of by the owner loop")
	}
	if ps.getChildSA() != oldChild {
		t.Error("old Child SA was removed before the owner loop relinquished (breaks make-before-break)")
	}
}

// VALIDATES: INITIAL_CONTACT is emitted on the initiator's first IKE_AUTH request and
// honored by the responder. A normal PSK handshake round-trips it end to end (RFC 7296
// Section 2.4: "MUST be in the first IKE_AUTH request").
// RFC requirement: RFC7296-2.4-4 positive -- INITIAL_CONTACT is carried in the FIRST IKE_AUTH
// request: buildAuthRequest (auth.go:124-126) appends the notify to the initiator's first
// IKE_AUTH, and a full PSK handshake round-trips it so the responder parses/honors it there.
func TestInitiatorEmitsInitialContact(t *testing.T) {
	_, resp, _ := establishPSK(t)
	if !resp.InitialContact {
		t.Fatal("responder did not receive/parse INITIAL_CONTACT from the initiator's first IKE_AUTH")
	}
}
