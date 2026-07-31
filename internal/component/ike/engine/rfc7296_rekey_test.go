// RFC 7296 rekey obligations. These tests cover four of them.
//
// Section 1.3 governs the Diffie-Hellman group a CREATE_CHILD_SA offers.
// Section 2.8 governs the close of a failed rekey, and the Delete that ends the old
// IKE SA. Section 2.8.1 governs the two Child SAs that receive during a rekey.
//
// Each test carries an `RFC requirement:` tag for the row it proves. Helpers here start with
// `rky`, so they cannot collide with the sibling `rfc7296_retransmit_test.go`. This
// file reuses that sibling's `rtx` loopback helpers.

package engine

import (
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// errRkyInstallRefused is the dataplane error that stands for a kernel that rejects
// an SA install. It avoids every string and errno that isXFRMUnsupported (child.go:22)
// treats as an absent XFRM stack, so installChildTolerant returns it to the caller.
var errRkyInstallRefused = errors.New("rky: the dataplane refused the install")

// rkyDP records what an SA install and an SA removal did. installErr makes every
// install fail, which is how a test proves what the engine does when the new Child
// SA cannot be installed.
type rkyDP struct {
	installed  []dataplane.SAParams
	removed    []uint32
	installErr error
}

func (d *rkyDP) InstallSA(p dataplane.SAParams) error {
	if d.installErr != nil {
		return d.installErr
	}
	d.installed = append(d.installed, p)
	return nil
}

func (d *rkyDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	d.removed = append(d.removed, spi)
	return nil
}

func (d *rkyDP) InstallPolicy(_ dataplane.SPParams) error              { return nil }
func (d *rkyDP) RemovePolicy(_, _ *net.IPNet, _ dataplane.SADir) error { return nil }
func (d *rkyDP) RemovePolicyParams(_ dataplane.SPParams) error         { return nil }
func (d *rkyDP) ListSAs(_ uint32) ([]dataplane.SAInfo, error)          { return nil, nil }
func (d *rkyDP) Close() error                                          { return nil }

// installedSA returns the parameters of the installed SA with this SPI, or nil.
func (d *rkyDP) installedSA(spi uint32) *dataplane.SAParams {
	for i := range d.installed {
		if d.installed[i].SPI == spi {
			return &d.installed[i]
		}
	}
	return nil
}

// wasRemoved reports whether this SPI was removed from the dataplane.
func (d *rkyDP) wasRemoved(spi uint32) bool {
	return slices.Contains(d.removed, spi)
}

// rkyFindSA returns the first SA payload of a decrypted payload chain.
func rkyFindSA(t *testing.T, inner []wire.PayloadEntry) *wire.PayloadSA {
	t.Helper()
	for i := range inner {
		if p, ok := inner[i].Payload.(*wire.PayloadSA); ok {
			return p
		}
	}
	t.Fatal("the message carries no SA payload")
	return nil
}

// rkyFindKE returns the first Key Exchange payload of a decrypted payload chain.
func rkyFindKE(t *testing.T, inner []wire.PayloadEntry) *wire.PayloadKE {
	t.Helper()
	for i := range inner {
		if p, ok := inner[i].Payload.(*wire.PayloadKE); ok {
			return p
		}
	}
	t.Fatal("the message carries no KE payload")
	return nil
}

// rkyOffersGroup reports whether one offer in the SA payload names this
// Diffie-Hellman group. This is the exact predicate RFC 7296 Section 1.3 states.
func rkyOffersGroup(sa *wire.PayloadSA, group uint16) bool {
	for _, prop := range sa.Proposals {
		for _, tr := range prop.Transforms {
			if tr.Type == wire.TransformTypeDH && tr.ID == group {
				return true
			}
		}
	}
	return false
}

// rkyDHGroups lists every Diffie-Hellman group the SA payload offers.
func rkyDHGroups(sa *wire.PayloadSA) []uint16 {
	var out []uint16
	for _, prop := range sa.Proposals {
		for _, tr := range prop.Transforms {
			if tr.Type == wire.TransformTypeDH {
				out = append(out, tr.ID)
			}
		}
	}
	return out
}

// rkyTwoGroupIKE is an IKE group whose two proposals name two different
// Diffie-Hellman groups. The first one is the group initiateIKERekey puts in KEi.
func rkyTwoGroupIKE() ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "rky-two-group",
		Proposals: []ipsec.IKEProposal{
			{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 14},
			{Number: 2, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 19},
		},
	}
}

// rkyChildRekeyRequest builds the payload chain of a peer CREATE_CHILD_SA that
// rekeys the Child SA behind oldSPI. RFC 7296 Section 1.3.2: N(REKEY_SA), SA, Ni.
func rkyChildRekeyRequest(oldSPI, peerESPSPI uint32, ni []byte) []wire.PayloadEntry {
	spiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(spiBytes, oldSPI)
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{
			ProtocolID:    wire.ProtocolESP,
			SPISize:       4,
			NotifyMsgType: wire.NotifyRekeySA,
			SPI:           spiBytes,
		}},
		{Payload: espSAPayload(peerESPSPI)},
		{Payload: &wire.PayloadNonce{NonceData: ni}},
	}
}

// VALIDATES: a CREATE_CHILD_SA that carries a Diffie-Hellman value also offers that
// exact group in its SA payload, on the request side and on the response side.
// RFC requirement: RFC7296-1.3-1 positive -- initiateIKERekey (rekey.go:300-321) reads the KEi
// group from the first configured proposal. buildWireIKEProposals (initiator.go:112-140)
// offers every proposal, so the KEi group is one of them. respondIKERekey
// (rekey.go:521-531) does the same for the KEr it returns.
// RFC requirement: RFC7296-1.3-1 negative -- the offer set names two different groups, so the
// match is a real search. The same search over an offer set without that group fails.
func TestRkyIKERekeyOffersTheKEiGroup(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	group := rkyTwoGroupIKE()

	reqBytes, pending, err := initiateIKERekey(ini, group)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()

	reqInner, err := decryptAndParse(resp, parseMsg(t, reqBytes), reqBytes)
	if err != nil {
		t.Fatalf("the peer could not decrypt the rekey request: %v", err)
	}
	kei := rkyFindKE(t, reqInner)
	offers := rkyFindSA(t, reqInner)

	// The request offers both configured groups, so "at least one" is a real search
	// over more than one candidate.
	groups := rkyDHGroups(offers)
	if len(groups) != 2 || groups[0] != 14 || groups[1] != 19 {
		t.Fatalf("the request offered groups %v, want [14 19]", groups)
	}
	if kei.DHGroup != 14 {
		t.Errorf("KEi group = %d, want 14 (the first configured proposal)", kei.DHGroup)
	}
	if !rkyOffersGroup(offers, kei.DHGroup) {
		t.Errorf("no offer names the KEi group %d", kei.DHGroup)
	}

	// Negative. The predicate discriminates: it rejects an offer set that names only
	// the other group, so the check above is not true of every payload.
	otherOnly := &wire.PayloadSA{Proposals: []wire.Proposal{{
		Number:     1,
		ProtocolID: wire.ProtocolIKE,
		Transforms: []wire.Transform{{Type: wire.TransformTypeDH, ID: 19}},
	}}}
	if rkyOffersGroup(otherOnly, kei.DHGroup) {
		t.Error("the group search accepted an offer set that lacks the KEi group")
	}

	// Second producer. The responder returns KEr with the group of the proposal it
	// chose, and its response carries that proposal.
	respBytes, peerNewSA, err := respondIKERekey(resp, reqInner, pending.messageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	defer peerNewSA.SKKeys.Clear()
	respInner, err := decryptAndParse(ini, parseMsg(t, respBytes), respBytes)
	if err != nil {
		t.Fatalf("the initiator could not decrypt the rekey response: %v", err)
	}
	ker := rkyFindKE(t, respInner)
	chosen := rkyFindSA(t, respInner)
	if ker.DHGroup != 14 {
		t.Errorf("KEr group = %d, want 14 (the group of the chosen proposal)", ker.DHGroup)
	}
	if !rkyOffersGroup(chosen, ker.DHGroup) {
		t.Errorf("the chosen proposal does not name the KEr group %d", ker.DHGroup)
	}
}

// VALIDATES: a rekey whose retransmissions are exhausted closes the Child SAs. It also
// ends the owner loop, instead of a tunnel that runs on keys about to expire.
// RFC requirement: RFC7296-2.8-5 positive -- serviceRekeyRetransmit (established.go:275-280)
// calls cleanupChild and returns errTimeout once the retransmissions are exhausted. The
// error ends maintainSA, and runResponder then drops the IKE SA (fsm.go:227-229).
// RFC requirement: RFC7296-2.8-5 negative -- one retransmission below the limit the same call
// keeps the Child SA installed and returns no error, so the close needs exhaustion.
func TestRkyExhaustedRekeyClosesTheChildSAs(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa := testSA()
	dp := &rkyDP{}
	ps := &PeerSession{peerName: "rky"}

	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)
	if dp.installedSA(old.InboundSPI) == nil {
		t.Fatal("the old Child SA was never installed, so a removal proves nothing")
	}

	// Negative. One below the limit: the engine sends the request again and the tunnel stands.
	ps.pendingRekey = &pendingRekey{
		kind:        rekeyChild,
		sentAt:      time.Now().Add(-time.Hour),
		sentMsg:     []byte("rky-request"),
		retransmits: maxRetransmissions - 1,
	}
	if err := ps.serviceRekeyRetransmit(sa, nil, time.Now(), dp, nil, log); err != nil {
		t.Fatalf("a retransmission below the limit returned %v, want nil", err)
	}
	if len(dp.removed) != 0 {
		t.Errorf("the Child SA was closed early: removed %v", dp.removed)
	}
	if ps.getChildSA() != old || ps.pendingRekey == nil {
		t.Error("a retransmission below the limit dropped the Child SA or the exchange")
	}

	// Positive. The limit is reached, so both halves of the Child SA go away and the
	// owner loop is told to end.
	ps.pendingRekey = &pendingRekey{
		kind:        rekeyChild,
		sentAt:      time.Now().Add(-time.Hour),
		sentMsg:     []byte("rky-request"),
		retransmits: maxRetransmissions,
	}
	err = ps.serviceRekeyRetransmit(sa, nil, time.Now(), dp, nil, log)
	if !errors.Is(err, errTimeout) {
		t.Fatalf("an exhausted rekey returned %v, want errTimeout", err)
	}
	if !dp.wasRemoved(old.InboundSPI) {
		t.Error("the inbound Child SA is still installed after the rekey failed")
	}
	if !dp.wasRemoved(old.OutboundSPI) {
		t.Error("the outbound Child SA is still installed after the rekey failed")
	}
	if ps.getChildSA() != nil {
		t.Error("the session still holds a Child SA after the rekey failed")
	}
	if ps.pendingRekey != nil {
		t.Error("the failed exchange is still outstanding")
	}
}

// VALIDATES: the Delete that retires the old IKE SA is the last request written on it.
// The engine writes it only when it accepts the rekey response.
// RFC requirement: RFC7296-2.8-6 positive -- handleCreateChildSAOwned (inbound.go:137-150) sends
// the IKE Delete on the old SA. No request follows it there, because maintainSA
// (established.go:152-164) adopts the new SA.
// RFC requirement: RFC7296-2.8-6 negative -- a rejected rekey response writes no Delete. The
// Delete needs a completed rekey, not an unconditional teardown.
func TestRkyIKERekeyDeleteIsTheLastRequestOnTheOldSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}
	ps := &PeerSession{peerName: "rky"}
	ikeGroup := testIKEGroup()

	reqBytes, pending, err := initiateIKERekey(ini, ikeGroup)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	ps.pendingRekey = pending
	reqInner, err := decryptAndParse(resp, parseMsg(t, reqBytes), reqBytes)
	if err != nil {
		t.Fatalf("the peer could not decrypt the rekey request: %v", err)
	}
	respBytes, peerNewSA, err := respondIKERekey(resp, reqInner, pending.messageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	defer peerNewSA.SKKeys.Clear()
	respInner, err := decryptAndParse(ini, parseMsg(t, respBytes), respBytes)
	if err != nil {
		t.Fatalf("the initiator could not decrypt the rekey response: %v", err)
	}

	deleteID := ini.NextMsgID
	out := ps.handleCreateChildSAOwned(ini, parseMsg(t, respBytes), respInner, true, myTr, nil, log)
	if out.newSA == nil {
		t.Fatal("the rekey response produced no replacement IKE SA")
	}
	defer out.newSA.SKKeys.Clear()

	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("the rekey wrote no Delete for the old IKE SA")
	}
	sent := parseMsg(t, got)
	if sent.Header.ExchangeType != wire.ExchangeInformational {
		t.Errorf("the Delete exchange = %d, want INFORMATIONAL", sent.Header.ExchangeType)
	}
	if sent.Header.Flags&wire.FlagResponse != 0 {
		t.Error("the Delete carries the Response flag, so it is not a request")
	}
	if sent.Header.MessageID != deleteID {
		t.Errorf("the Delete Message ID = %d, want %d", sent.Header.MessageID, deleteID)
	}

	// The Delete travels on the OLD IKE SA: the peer's old SA keys decrypt it.
	deleteInner, err := decryptAndParse(resp, sent, got)
	if err != nil {
		t.Fatalf("the Delete did not decrypt under the old IKE SA keys: %v", err)
	}
	sawIKEDelete := false
	for i := range deleteInner {
		if d, ok := deleteInner[i].Payload.(*wire.PayloadDelete); ok && d.ProtocolID == wire.ProtocolIKE {
			sawIKEDelete = true
		}
	}
	if !sawIKEDelete {
		t.Error("the message on the old IKE SA is not a Delete of the IKE SA")
	}

	// Nothing follows it on the old SA. The rekey wrote one datagram, and the SA the
	// loop switches to is a different SA with its own counters.
	rtxExpectSilence(t, peerTr, myTr, remote, "after the IKE rekey Delete")
	if ini.NextMsgID != deleteID+1 {
		t.Errorf("the old SA request counter = %d, want %d", ini.NextMsgID, deleteID+1)
	}
	if out.newSA.NextMsgID != 0 || out.newSA.ExpectedMsgID != 0 {
		t.Errorf("the new SA counters = next:%d expected:%d, want 0 and 0",
			out.newSA.NextMsgID, out.newSA.ExpectedMsgID)
	}
	if out.newSA.InitiatorSPI == ini.InitiatorSPI {
		t.Error("the new IKE SA reuses the old initiator SPI")
	}

	// Negative. The engine rejects a rekey response without KEr, and writes no Delete.
	_, pending2, err := initiateIKERekey(ini, ikeGroup)
	if err != nil {
		t.Fatalf("second initiateIKERekey: %v", err)
	}
	defer pending2.clear()
	ps.pendingRekey = pending2
	broken := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: []wire.Proposal{{
			Number: 1, ProtocolID: wire.ProtocolIKE, SPISize: 8, SPI: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		}}}},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(4)}},
	}
	out2 := ps.handleCreateChildSAOwned(ini, parseMsg(t, respBytes), broken, true, myTr, nil, log)
	if out2.newSA != nil {
		t.Error("a rekey response without KEr produced a replacement IKE SA")
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "after a rejected rekey response")
}

// VALIDATES: the rekey responder installs the Child SA it advertises before it answers,
// so the peer can send on that SPI as soon as the response arrives.
// RFC requirement: RFC7296-2.8-7 positive -- respondChildRekey installs the replacement at
// rekey.go:175 and builds the response at rekey.go:187, and handleCreateChildSAOwned
// sends it at inbound.go:181. The SPI in the answer is already a live inbound SA.
// RFC requirement: RFC7296-2.8-7 negative -- when the dataplane refuses the install the
// responder advertises no Child SA at all, so the answer follows the install and never
// leads it. The refusal now draws an error notify rather than silence, because RFC 7296
// Section 2.21.3 MUST answer every failing request on an authenticated SA. That notify
// carries no SA payload and no SPI, so nothing is advertised that is not installed.
// rfc-test-change-approved: 2026-07-31 owner standing approval for
// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only. The old assertion was
// "no datagram", a proxy for "no SPI advertised" that held only while silence was the
// only other outcome. The new assertion reads the datagram and proves the stronger
// claim directly.
func TestRkyResponderInstallsTheNewChildBeforeItAnswers(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	resp.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := resp.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the responder has no resolvable peer address")
	}

	dp := &rkyDP{}
	ps := &PeerSession{peerName: "rky", espGroup: testESPGroup()}
	old, err := createFirstChildSA(resp, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)

	inner := rkyChildRekeyRequest(old.InboundSPI, 0x0A0B0C0D, testNonce(11))
	msg := &wire.Message{Header: wire.Header{MessageID: resp.ExpectedMsgID}}
	out := ps.handleCreateChildSAOwned(resp, msg, inner, false, myTr, dp, log)
	if out.newChild == nil {
		t.Fatal("the responder installed no replacement Child SA")
	}

	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("the responder wrote no CREATE_CHILD_SA response")
	}
	answer, err := decryptAndParse(ini, parseMsg(t, got), got)
	if err != nil {
		t.Fatalf("the peer could not decrypt the rekey response: %v", err)
	}
	advertised, err := espSPIFromSA(rkyFindSA(t, answer))
	if err != nil {
		t.Fatalf("the response carries no ESP SPI: %v", err)
	}
	if advertised != out.newChild.InboundSPI {
		t.Errorf("the response advertised SPI %#x, want %#x", advertised, out.newChild.InboundSPI)
	}

	// The advertised SPI is a live receive SA: its destination is our own address.
	installed := dp.installedSA(advertised)
	if installed == nil {
		t.Fatal("the SPI in the response is not installed, so the peer's first packet drops")
	}
	if !installed.Dst.Equal(old.LocalAddr) {
		t.Errorf("the installed SA for %#x has destination %v, want the local address %v",
			advertised, installed.Dst, old.LocalAddr)
	}
	if dp.wasRemoved(advertised) {
		t.Error("the advertised SA was removed again before the response was sent")
	}

	// Negative. A dataplane that refuses the install draws no answer at all.
	ini2, resp2, _ := establishPSK(t)
	_ = ini2
	resp2.PeerCfg.RemoteAddress = "127.0.0.1"
	refuse := &rkyDP{installErr: errRkyInstallRefused}
	ps2 := &PeerSession{peerName: "rky-refuse", espGroup: testESPGroup()}
	old2, err := createFirstChildSA(resp2, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, nil, log)
	if err != nil {
		t.Fatalf("createFirstChildSA for the refused case: %v", err)
	}
	ps2.setChildSA(old2)
	beforeID := resp2.lastResponseID

	inner2 := rkyChildRekeyRequest(old2.InboundSPI, 0x0B0C0D0E, testNonce(13))
	msg2 := &wire.Message{Header: wire.Header{MessageID: resp2.ExpectedMsgID}}
	out2 := ps2.handleCreateChildSAOwned(resp2, msg2, inner2, false, myTr, refuse, log)
	if out2.newChild != nil {
		t.Error("a refused install still produced a replacement Child SA")
	}
	_ = beforeID

	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only.
	// The refusal is answered (RFC 7296 Section 2.21.3), and the answer advertises
	// nothing. Reading the datagram proves the install-before-answer ordering directly,
	// where the old "no datagram" assertion only implied it.
	refused := rtxRecv(t, peerTr)
	if refused == nil {
		t.Fatal("a refused Child SA rekey drew no answer, so the peer burns its request window")
	}
	answer2, err := decryptAndParse(ini2, parseMsg(t, refused), refused)
	if err != nil {
		t.Fatalf("the peer could not decrypt the refusal: %v", err)
	}
	sawNotify := false
	for i := range answer2 {
		switch p := answer2[i].Payload.(type) {
		case *wire.PayloadSA:
			t.Error("the refusal advertises an SA payload, so it names an SPI that was never installed")
		case *wire.PayloadNotify:
			sawNotify = true
			if p.NotifyMsgType != wire.NotifyNoProposalChosen {
				t.Errorf("the refusal carries notify %d, want NO_PROPOSAL_CHOSEN", p.NotifyMsgType)
			}
		}
	}
	if !sawNotify {
		t.Error("the refusal carries no notify, so the peer learns nothing")
	}
	if len(refuse.installed) != 0 {
		t.Error("the dataplane holds an installed SA even though the install was refused")
	}
}

// VALIDATES: during a peer Child SA rekey the old and the new inbound SA both stay
// installed. A packet on either one is accepted until the peer deletes the old.
// RFC requirement: RFC7296-2.8.1-1 positive -- handleCreateChildSAOwned holds the old Child SA
// in supersededChild (inbound.go:183). Both inbound SAs stay live at once, and
// handleDeletePayload (inbound.go:287-292) closes the old one later.
// RFC requirement: RFC7296-2.8.1-1 negative -- the peer's ESP Delete removes the old SA and
// keeps the new one. The two live SAs are a real window, not a blind observer.
func TestRkyOldAndNewChildBothReceiveUntilThePeerDeletes(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, resp, _ := establishPSK(t)
	dp := &rkyDP{}
	ps := &PeerSession{peerName: "rky", espGroup: testESPGroup()}

	old, err := createFirstChildSA(resp, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)

	inner := rkyChildRekeyRequest(old.InboundSPI, 0x0C0D0E0F, testNonce(17))
	msg := &wire.Message{Header: wire.Header{MessageID: resp.ExpectedMsgID}}
	out := ps.handleCreateChildSAOwned(resp, msg, inner, false, nil, dp, log)
	if out.newChild == nil {
		t.Fatal("the responder installed no replacement Child SA")
	}
	newChild := out.newChild

	// Both inbound SAs are eligible to receive.
	if dp.installedSA(old.InboundSPI) == nil || dp.wasRemoved(old.InboundSPI) {
		t.Error("the old inbound SA stopped receiving as soon as the new one appeared")
	}
	if dp.installedSA(newChild.InboundSPI) == nil || dp.wasRemoved(newChild.InboundSPI) {
		t.Error("the new inbound SA is not installed, so its first packet drops")
	}
	if ps.supersededChild != old {
		t.Error("the old Child SA is not held for the peer's Delete")
	}
	if ps.getChildSA() != newChild {
		t.Error("the session did not adopt the replacement Child SA")
	}

	// Negative. The peer's Delete ends the window: the old SA goes and the new stays.
	spiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(spiBytes, old.InboundSPI)
	del := []wire.PayloadEntry{{Payload: &wire.PayloadDelete{
		ProtocolID: wire.ProtocolESP, SPISize: 4, NumSPIs: 1, SPIs: spiBytes,
	}}}
	delMsg := &wire.Message{Header: wire.Header{MessageID: resp.ExpectedMsgID}}
	ps.handleInformationalOwned(resp, delMsg, del, false, nil, dp, log)
	if !dp.wasRemoved(old.InboundSPI) {
		t.Error("the peer's Delete left the old inbound SA installed")
	}
	if dp.wasRemoved(newChild.InboundSPI) {
		t.Error("the peer's Delete removed the new inbound SA")
	}
	if ps.supersededChild != nil {
		t.Error("the superseded Child SA is still held after the peer's Delete")
	}
}
