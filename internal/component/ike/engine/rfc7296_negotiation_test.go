// VALIDATES: the two ends of an exchange agree on what was negotiated. Three rules meet
// here. A Diffie-Hellman group the responder did not select draws INVALID_KE_PAYLOAD. A
// simultaneous IKE rekey leaves one new IKE SA. An accepted offer that names a suite we
// never proposed stops the exchange.
// PREVENTS: an initiator and a responder that derive keys from different parameters and
// never find out.
package engine

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// negUnproposedKeyLength is an encryption key length our test config never offers.
// testIKEGroup and testESPGroup both name AES-256, so 128 names a suite we never sent.
const negUnproposedKeyLength uint16 = 128

// negPeerSPI is the new IKE SPI a peer proposes in its rekey request.
var negPeerSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}

// negNonce builds a nonce of nonceLen octets whose every byte is fill. An
// octet-by-octet comparison of two such nonces follows the fill byte alone. A test
// therefore chooses the winner of a rekey collision, rather than waits on random bytes.
func negNonce(fill byte) []byte { return bytes.Repeat([]byte{fill}, nonceLen) }

// negModpPublic returns a Diffie-Hellman public value of the MODP-2048 group.
func negModpPublic(t *testing.T) []byte {
	t.Helper()
	dh, err := crypto.NewDHExchange(crypto.DH_MODP_2048)
	if err != nil {
		t.Fatalf("MODP-2048 exchange: %v", err)
	}
	return dh.PublicKey
}

// negIKERekeyInner builds the decrypted payload chain of a peer IKE rekey request: SA,
// Ni, and KEi. The group argument names the Diffie-Hellman group the KE payload declares,
// which a caller sets apart from the group the SA proposals carry.
func negIKERekeyInner(t *testing.T, ni []byte, group uint16, public []byte) []wire.PayloadEntry {
	t.Helper()
	props := buildWireIKEProposals(testIKEGroup())
	spiBytes := make([]byte, 8)
	copy(spiBytes, negPeerSPI[:])
	for i := range props {
		props[i].SPISize = 8
		props[i].SPI = spiBytes
	}
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: props}},
		{Payload: &wire.PayloadNonce{NonceData: ni}},
		{Payload: &wire.PayloadKE{DHGroup: group, KeyExchangeData: public}},
	}
}

// negNotifyIn decrypts a message the peer built and returns the first Notify payload in
// it. It returns nil when the payload chain carries none.
func negNotifyIn(t *testing.T, reader *SA, raw []byte) *wire.PayloadNotify {
	t.Helper()
	inner, err := decryptAndParse(reader, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt the answer: %v", err)
	}
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok {
			return n
		}
	}
	return nil
}

// negFindSA returns the first SA payload of a decrypted payload chain.
func negFindSA(t *testing.T, inner []wire.PayloadEntry) *wire.PayloadSA {
	t.Helper()
	for i := range inner {
		if p, ok := inner[i].Payload.(*wire.PayloadSA); ok {
			return p
		}
	}
	t.Fatal("the payload chain carries no SA payload")
	return nil
}

// negSetKeyLength rewrites the encryption key length attribute of every proposal in an
// SA payload. It turns a real accepted offer into one that names a suite we never sent.
// A keyLen of zero leaves the payload as it was built.
func negSetKeyLength(sa *wire.PayloadSA, keyLen uint16) {
	if keyLen == 0 {
		return
	}
	for i := range sa.Proposals {
		for j := range sa.Proposals[i].Transforms {
			if sa.Proposals[i].Transforms[j].Type != wire.TransformTypeENCR {
				continue
			}
			sa.Proposals[i].Transforms[j].Attrs = []wire.TransformAttr{
				{Type: wire.AttrTypeKeyLength, Value: keyLen},
			}
		}
	}
}

// negESPRekeyInner builds the decrypted payload chain of a Child SA rekey response: SA
// and Nr. The keyLen argument sets the key length the accepted ESP offer names.
func negESPRekeyInner(t *testing.T, keyLen uint16) []wire.PayloadEntry {
	t.Helper()
	saPayload := &wire.PayloadSA{Proposals: buildWireESPProposals(testESPGroup(), 0x11223344)}
	negSetKeyLength(saPayload, keyLen)
	return []wire.PayloadEntry{
		{Payload: saPayload},
		{Payload: &wire.PayloadNonce{NonceData: negNonce(0x22)}},
	}
}

// negRekeySession builds the initiator side of a rekey: an established SA, its owner
// session, and a loopback link that receives what the session sends.
func negRekeySession(t *testing.T) (ini *SA, ps *PeerSession, peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	ini, _, _ = establishPSK(t)
	peerTr, myTr = rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	ps = &PeerSession{
		peerName: "ze",
		peerCfg:  ini.PeerCfg,
		ikeGroup: testIKEGroup(),
		espGroup: testESPGroup(),
	}
	return ini, ps, peerTr, myTr
}

// negStartIKERekey puts an IKE rekey of our own in flight and fixes its nonce. The
// collision comparison against a peer nonce is then decided by the test.
func negStartIKERekey(t *testing.T, ini *SA, ps *PeerSession, localNonce []byte) *pendingRekey {
	t.Helper()
	if !ini.reserveRequestWindow() {
		t.Fatal("the request window was already held before our rekey")
	}
	_, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	t.Cleanup(pending.clear)
	pending.localNonce = localNonce
	ps.pendingRekey = pending
	return pending
}

// negSAInitPair runs IKE_SA_INIT on both sides. It returns the initiator that waits for
// the response, its SA table, and the response bytes the responder built.
func negSAInitPair(t *testing.T) (*SA, *SATable, []byte) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "negotiation-psk")

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	return ini, table, resp.LastSentMsg
}

// negRewriteSAInit rewrites the accepted offer of an IKE_SA_INIT response so that it
// names a key length we never proposed. IKE_SA_INIT is not encrypted, so the message is
// parsed, changed in one field, and written out again.
func negRewriteSAInit(t *testing.T, raw []byte) []byte {
	t.Helper()
	msg := parseMsg(t, raw)
	found := false
	for i := range msg.Payloads {
		if p, ok := msg.Payloads[i].Payload.(*wire.PayloadSA); ok {
			negSetKeyLength(p, negUnproposedKeyLength)
			found = true
		}
	}
	if !found {
		t.Fatal("the IKE_SA_INIT response carries no SA payload")
	}
	buf := make([]byte, 4096)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		t.Fatalf("rewrite the IKE_SA_INIT response: %v", err)
	}
	return buf[:n]
}

// negAuthResponse runs the handshake up to the IKE_AUTH response and returns the
// initiator that waits for it, plus the response bytes. A keyLen above zero rewrites the
// accepted ESP offer and seals the chain again under the responder keys.
func negAuthResponse(t *testing.T, keyLen uint16) (*SA, []byte) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "negotiation-psk")

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	authResp := resp.LastSentMsg
	if keyLen == 0 {
		return ini, authResp
	}

	inner, err := decryptAndParse(ini, parseMsg(t, authResp), authResp)
	if err != nil {
		t.Fatalf("decrypt the IKE_AUTH response: %v", err)
	}
	negSetKeyLength(negFindSA(t, inner), keyLen)
	sealed, err := buildEncryptedMessageEx(resp, inner, 1, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		t.Fatalf("seal the rewritten IKE_AUTH response: %v", err)
	}
	return ini, sealed
}

// negIKERekeyExchange runs one IKE SA rekey up to the response. It returns the rekey
// initiator, its pending state, and the decrypted payload chain of the response.
func negIKERekeyExchange(t *testing.T) (*SA, *pendingRekey, []wire.PayloadEntry) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	req, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	t.Cleanup(pending.clear)
	reqInner, err := decryptAndParse(resp, parseMsg(t, req), req)
	if err != nil {
		t.Fatalf("responder decrypt of the rekey request: %v", err)
	}
	respBytes, _, err := respondIKERekey(resp, reqInner, pending.messageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	respInner, err := decryptAndParse(ini, parseMsg(t, respBytes), respBytes)
	if err != nil {
		t.Fatalf("initiator decrypt of the rekey response: %v", err)
	}
	return ini, pending, respInner
}

// RFC requirement: RFC7296-1.3-2 positive -- respondIKERekey (rekey.go) selects a proposal, then
// compares the group of that proposal against the group the KEi payload declares. A
// mismatch answers INVALID_KE_PAYLOAD naming the group we selected, and returns no new
// SA. No key is derived from a value computed over another group.
// RFC requirement: RFC7296-1.3-2 negative -- the same request with a matching KE group builds a
// new IKE SA and draws no Notify. The refusal therefore belongs to the group comparison,
// and not to a request the responder rejects for another reason.
func TestNegRekeyRejectsMismatchedKEGroup(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	public := negModpPublic(t)
	ni := negNonce(0x40)

	// Negative. The KE group equals the group the responder selects, so the exchange
	// completes and the answer carries no Notify.
	okInner := negIKERekeyInner(t, ni, uint16(crypto.DH_MODP_2048), public)
	okAnswer, okSA, err := respondIKERekey(resp, okInner, 2, log)
	if err != nil {
		t.Fatalf("a matching KE group returned %v, want a new IKE SA", err)
	}
	if okSA == nil {
		t.Fatal("a matching KE group produced no new IKE SA")
	}
	if n := negNotifyIn(t, ini, okAnswer); n != nil {
		t.Fatalf("a matching KE group drew Notify type %d, want none", n.NotifyMsgType)
	}

	// Positive. The KE payload names ECP-256 while the selected proposal names
	// MODP-2048.
	badInner := negIKERekeyInner(t, ni, uint16(crypto.DH_ECP_256), public)
	badAnswer, badSA, err := respondIKERekey(resp, badInner, 3, log)
	if err != nil {
		t.Fatalf("a mismatched KE group returned %v, want an INVALID_KE_PAYLOAD answer", err)
	}
	if badSA != nil {
		t.Fatal("a mismatched KE group still built an IKE SA, so keys were derived")
	}
	notify := negNotifyIn(t, ini, badAnswer)
	if notify == nil {
		t.Fatal("a mismatched KE group drew no Notify payload")
	}
	if notify.NotifyMsgType != wire.NotifyInvalidKEPayload {
		t.Fatalf("Notify type = %d, want INVALID_KE_PAYLOAD (%d)",
			notify.NotifyMsgType, wire.NotifyInvalidKEPayload)
	}
	// RFC 7296 Section 3.10.1: two octets that name the group the responder selected.
	want := []byte{0, byte(crypto.DH_MODP_2048)}
	if !bytes.Equal(notify.NotificationData, want) {
		t.Fatalf("Notify data = %v, want %v naming the group we selected",
			notify.NotificationData, want)
	}
}

// RFC requirement: RFC7296-1.3-2 positive -- crypto.SharedSecret (crypto/dh.go) refuses a MODP
// peer value whose length is not the length of the prime modulus. A value of another
// group reaches this primitive only when a caller forgot the group comparison. It fails
// closed there rather than produces a secret the peer never computes.
// RFC requirement: RFC7296-1.3-2 negative -- a value of the modulus length is accepted and the
// two sides agree on the secret. The refusal is therefore about the length of the value,
// and the primitive did not stop working.
func TestNegSharedSecretRefusesWrongLength(t *testing.T) {
	ours, err := crypto.NewDHExchange(crypto.DH_MODP_2048)
	if err != nil {
		t.Fatalf("our MODP-2048 exchange: %v", err)
	}
	theirs, err := crypto.NewDHExchange(crypto.DH_MODP_2048)
	if err != nil {
		t.Fatalf("peer MODP-2048 exchange: %v", err)
	}
	ec, err := crypto.NewDHExchange(crypto.DH_ECP_256)
	if err != nil {
		t.Fatalf("ECP-256 exchange: %v", err)
	}

	// Negative. A full-length value is accepted, and both sides derive one secret.
	full := theirs.PublicKey
	ourSecret, err := ours.SharedSecret(full)
	if err != nil {
		t.Fatalf("a full-length peer value returned %v, want a secret", err)
	}
	theirSecret, err := theirs.SharedSecret(ours.PublicKey)
	if err != nil {
		t.Fatalf("the peer refused our full-length value: %v", err)
	}
	if !bytes.Equal(ourSecret, theirSecret) {
		t.Fatal("the two sides derived different secrets from full-length values")
	}

	// Positive. Every value of another length is refused by name.
	cases := []struct {
		name  string
		value []byte
	}{
		{"one octet short", full[1:]},
		{"one octet long", append([]byte{0}, full...)},
		{"the natural encoding of the group generator", []byte{2}},
		{"an ECP-256 public value", ec.PublicKey},
	}
	for _, c := range cases {
		secret, err := ours.SharedSecret(c.value)
		if !errors.Is(err, crypto.ErrPublicKeyLength) {
			t.Errorf("%s (%d octets): error = %v, want ErrPublicKeyLength",
				c.name, len(c.value), err)
		}
		if secret != nil {
			t.Errorf("%s: a secret was returned beside the refusal", c.name)
		}
	}
}

// RFC requirement: RFC7296-2.8.2-1 positive -- handleCreateChildSAOwned (inbound.go) reads
// ps.pendingRekey on the IKE rekey branch, as the Child branch already does. One of the
// two exchanges is abandoned, so exactly one new IKE SA is left. Both peers run the same
// comparison, so they abandon opposite exchanges and agree on the survivor.
// RFC requirement: RFC7296-2.8.2-1 negative -- the same peer request with no rekey of our own in
// flight is answered as usual. The abandoned exchange and the silence therefore come from
// collision resolution, and not from the request itself.
func TestNegIKERekeyCollisionResolves(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, ps, peerTr, myTr := negRekeySession(t)
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}
	public := negModpPublic(t)
	high := negNonce(0xF0)
	low := negNonce(0x10)
	ours := negNonce(0x80)

	// Negative. No rekey of our own is in flight, so the peer request is answered.
	alone := negIKERekeyInner(t, high, uint16(crypto.DH_MODP_2048), public)
	out := ps.handleCreateChildSAOwned(ini, &wire.Message{Header: wire.Header{MessageID: 4}},
		alone, false, myTr, nil, log)
	if out.newSA != nil {
		t.Fatal("a peer IKE rekey returned a live SA instead of a pending swap")
	}
	if ps.pendingIKESwap == nil {
		t.Fatal("a peer IKE rekey with no collision built no new IKE SA")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("a peer IKE rekey with no collision drew no answer")
	}
	ps.setPendingIKESwap(nil)

	// Positive, first ordering. Our nonce is the lower one, so our exchange survives
	// and the peer request is left alone. The peer runs the same comparison and
	// abandons its own exchange.
	pending := negStartIKERekey(t, ini, ps, ours)
	collide := negIKERekeyInner(t, high, uint16(crypto.DH_MODP_2048), public)
	out = ps.handleCreateChildSAOwned(ini, &wire.Message{Header: wire.Header{MessageID: 5}},
		collide, false, myTr, nil, log)
	if out.newSA != nil {
		t.Error("the losing peer exchange still produced an SA")
	}
	if ps.pendingRekey != pending {
		t.Error("our surviving exchange was abandoned")
	}
	if ps.pendingIKESwap != nil {
		t.Error("the losing peer exchange still built a second new IKE SA")
	}
	if !ini.requestOutstanding {
		t.Error("our surviving exchange released the request window it holds")
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "peer IKE rekey that lost the collision")

	// Positive, second ordering. The peer nonce is the lower one, so we abandon our
	// exchange, free its window, and answer the peer. R-2 drives both orderings.
	ps.pendingRekey = nil
	ini.releaseRequestWindow()
	negStartIKERekey(t, ini, ps, ours)
	yield := negIKERekeyInner(t, low, uint16(crypto.DH_MODP_2048), public)
	ps.handleCreateChildSAOwned(ini, &wire.Message{Header: wire.Header{MessageID: 6}},
		yield, false, myTr, nil, log)
	if ps.pendingRekey != nil {
		t.Error("we kept our exchange after the peer exchange won the collision")
	}
	if ini.requestOutstanding {
		t.Error("the abandoned exchange still holds the request window")
	}
	if ps.pendingIKESwap == nil {
		t.Fatal("the surviving peer exchange built no new IKE SA")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the surviving peer exchange drew no answer")
	}
}

// RFC requirement: RFC7296-2.8.2-1 positive -- the IKE SA that survives a rekey collision inherits
// every Child SA. The Child SA hangs off the PeerSession, so the swap in the owner loop
// leaves it installed and it is never removed from the dataplane.
// RFC requirement: RFC7296-2.8.2-1 negative -- the Child SA is present before the rekey, and the
// dataplane records a removal when one really happens. An empty removal list is therefore
// evidence of inheritance, and not evidence of a dataplane that records nothing.
func TestNegSurvivingSAInheritsChildren(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, ps, peerTr, myTr := negRekeySession(t)
	dp := &mockDP{}
	child, err := createFirstChildSA(ini, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(child)
	if ps.getChildSA() != child {
		t.Fatal("the Child SA was not installed on the session before the rekey")
	}

	// Our rekey is in flight and the peer exchange wins, so the new IKE SA of the peer
	// is the survivor.
	negStartIKERekey(t, ini, ps, negNonce(0x80))
	inner := negIKERekeyInner(t, negNonce(0x10), uint16(crypto.DH_MODP_2048), negModpPublic(t))
	ps.handleCreateChildSAOwned(ini, &wire.Message{Header: wire.Header{MessageID: 7}},
		inner, false, myTr, dp, log)
	survivor := ps.pendingIKESwap
	if survivor == nil {
		t.Fatal("the surviving peer exchange built no new IKE SA")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the surviving peer exchange drew no answer")
	}
	// The survivor is the peer exchange because collision resolution chose it. Without
	// that step both exchanges would run. Two new IKE SAs would then exist.
	if ps.pendingRekey != nil {
		t.Fatal("our own exchange survived beside the peer exchange, so two new IKE SAs exist")
	}

	// The peer deletes the old IKE SA, which confirms the rekey and swaps to the
	// survivor. RFC 7296 Section 2.8.
	del := []wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}}
	out := ps.handleInformationalOwned(ini, &wire.Message{Header: wire.Header{MessageID: 8}},
		del, false, myTr, dp, log)
	if out.newSA != survivor {
		t.Fatal("the swap did not adopt the SA the surviving exchange built")
	}
	got := ps.getChildSA()
	if got != child {
		t.Fatalf("the surviving IKE SA holds Child SA %v, want the one it inherits", got)
	}
	if got.InboundSPI != child.InboundSPI || got.OutboundSPI != child.OutboundSPI {
		t.Error("the inherited Child SA changed its SPIs")
	}
	for _, spi := range dp.removed {
		if spi == child.InboundSPI || spi == child.OutboundSPI {
			t.Fatalf("the inherited Child SA SPI %d was removed from the dataplane", spi)
		}
	}
	// Negative half. removeChildSA really does record a removal, so the empty list
	// above is evidence.
	removeChildSA(child, dp, log)
	if len(dp.removed) == 0 {
		t.Fatal("the dataplane records no removal at all, so the check above proves nothing")
	}
}

// RFC requirement: RFC7296-3.3.6-3 positive -- one helper holds this rule. It is
// verifyAcceptedOffer (initiator.go), and all four initiator response paths call it. An
// accepted offer that names a suite we never proposed stops the exchange. That holds at
// IKE_SA_INIT, at IKE_AUTH, at a Child SA rekey, and at an IKE SA rekey.
// RFC requirement: RFC7296-3.3.6-3 negative -- the same four paths accept the real offer the
// responder sent. The refusal therefore comes from the consistency check, and the paths
// are not broken for every input.
func TestNegInitiatorRejectsUnproposedOffer(t *testing.T) {
	log := slogutil.DiscardLogger()

	// The helper itself. One producer serves both protocols, and it fails closed on an
	// SA payload that carries no proposal at all.
	t.Run("shared-helper", func(t *testing.T) {
		ike := &wire.PayloadSA{Proposals: buildWireIKEProposals(testIKEGroup())}
		got, err := verifyAcceptedOffer(ike, testIKEGroup(), testESPGroup())
		if err != nil {
			t.Fatalf("a consistent IKE offer returned %v", err)
		}
		if got.Protocol != wire.ProtocolIKE {
			t.Errorf("protocol = %d, want IKE (%d)", got.Protocol, wire.ProtocolIKE)
		}
		esp := &wire.PayloadSA{Proposals: buildWireESPProposals(testESPGroup(), 0x11223344)}
		got, err = verifyAcceptedOffer(esp, testIKEGroup(), testESPGroup())
		if err != nil {
			t.Fatalf("a consistent ESP offer returned %v", err)
		}
		if got.Protocol != wire.ProtocolESP {
			t.Errorf("protocol = %d, want ESP (%d)", got.Protocol, wire.ProtocolESP)
		}

		negSetKeyLength(ike, negUnproposedKeyLength)
		if _, err := verifyAcceptedOffer(ike, testIKEGroup(), testESPGroup()); err == nil {
			t.Error("an unproposed IKE offer was accepted")
		}
		negSetKeyLength(esp, negUnproposedKeyLength)
		if _, err := verifyAcceptedOffer(esp, testIKEGroup(), testESPGroup()); err == nil {
			t.Error("an unproposed ESP offer was accepted")
		}
		if _, err := verifyAcceptedOffer(nil, testIKEGroup(), testESPGroup()); err == nil {
			t.Error("a missing SA payload was accepted")
		}
		empty := &wire.PayloadSA{}
		if _, err := verifyAcceptedOffer(empty, testIKEGroup(), testESPGroup()); err == nil {
			t.Error("an SA payload with no proposal was accepted")
		}
	})

	// Site one: the IKE_SA_INIT response.
	t.Run("sa-init-response", func(t *testing.T) {
		ini, table, answer := negSAInitPair(t)
		handleSAInitResponse(ini, parseMsg(t, answer), answer, table, nil, nil, log)
		if ini.State == StateDead {
			t.Fatal("the initiator rejected the offer the responder really sent")
		}

		ini, table, answer = negSAInitPair(t)
		bad := negRewriteSAInit(t, answer)
		handleSAInitResponse(ini, parseMsg(t, bad), bad, table, nil, nil, log)
		if ini.State != StateDead {
			t.Fatalf("state = %v, want dead because the accepted offer is unproposed", ini.State)
		}
	})

	// Site two: the IKE_AUTH response, which carries the accepted ESP offer as SAr2.
	t.Run("ike-auth-response", func(t *testing.T) {
		ini, answer := negAuthResponse(t, 0)
		handleAuthResponse(ini, parseMsg(t, answer), answer, nil, nil, log)
		if ini.State != StateEstablished {
			t.Fatalf("state = %v, want established for the offer the responder really sent", ini.State)
		}

		ini, answer = negAuthResponse(t, negUnproposedKeyLength)
		handleAuthResponse(ini, parseMsg(t, answer), answer, nil, nil, log)
		if ini.State != StateDead {
			t.Fatalf("state = %v, want dead because the accepted ESP offer is unproposed", ini.State)
		}
	})

	// Site three: the Child SA rekey response.
	t.Run("child-rekey-response", func(t *testing.T) {
		ini, _, _ := establishPSK(t)
		child, err := createFirstChildSA(ini, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, nil, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		_, pending, err := initiateChildRekey(ini, child)
		if err != nil {
			t.Fatalf("initiateChildRekey: %v", err)
		}
		if _, err := applyChildRekeyResponse(ini, pending, negESPRekeyInner(t, 0), nil, log); err != nil {
			t.Fatalf("a consistent ESP offer returned %v, want a new Child SA", err)
		}
		bad := negESPRekeyInner(t, negUnproposedKeyLength)
		if _, err := applyChildRekeyResponse(ini, pending, bad, nil, log); err == nil {
			t.Fatal("an unproposed ESP offer was accepted, so the exchange did not stop")
		}
	})

	// Site four: the IKE SA rekey response.
	t.Run("ike-rekey-response", func(t *testing.T) {
		ini, pending, respInner := negIKERekeyExchange(t)
		if _, err := applyIKERekeyResponse(ini, pending, respInner, log); err != nil {
			t.Fatalf("a consistent IKE offer returned %v, want a new IKE SA", err)
		}
		ini, pending, respInner = negIKERekeyExchange(t)
		negSetKeyLength(negFindSA(t, respInner), negUnproposedKeyLength)
		if _, err := applyIKERekeyResponse(ini, pending, respInner, log); err == nil {
			t.Fatal("an unproposed IKE offer was accepted, so the exchange did not stop")
		}
	})
}
