package engine

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// createChildRequestWithKE builds a peer CREATE_CHILD_SA request for a NEW Child SA
// with PFS: SA (ESP), Ni, KEi, TSi, TSr and no REKEY_SA notify, the RFC 7296 Section
// 1.3.1 shape "HDR, SK {SA, Ni, [KEi,] TSi, TSr}" with the optional KEi present.
func createChildRequestWithKE(t *testing.T, peerESPSPI uint32, ni []byte) []wire.PayloadEntry {
	t.Helper()
	return []wire.PayloadEntry{
		{Payload: espSAPayload(peerESPSPI)},
		{Payload: &wire.PayloadNonce{NonceData: ni}},
		{Payload: &wire.PayloadKE{DHGroup: 14, KeyExchangeData: make([]byte, 256)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "0.0.0.0/0")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")},
	}
}

// VALIDATES: AC-8 and AC-9. A peer request carrying KEi beside TSi and TSr and no
// REKEY_SA asks for a new Child SA, never for an IKE SA rekey, and while the session
// holds its one Child SA the answer is NO_ADDITIONAL_SAS with nothing changed.
// PREVENTS: the KE-first classifier that sent a PFS new-child request to
// respondIKERekey, which then answered it with IKE SA keys or an IKE error.
// Method: the request goes through handleCreateChildSAOwned on an established
// responder that holds a Child SA; the test reads the answer's notifies and checks
// that no IKE SA swap was staged and the Child SA is the same one.
func TestNewChildWithKEIsNotAnIKERekey(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ps := link.ps
	ps.peerName = "new-child-ke"
	ps.espGroup = testESPGroup()
	child := ps.getChildSA()
	if child == nil {
		t.Fatal("precondition: the established responder holds no Child SA")
	}

	inner := createChildRequestWithKE(t, 0x0badcafe, testNonce(41))
	msg := &wire.Message{Header: wire.Header{MessageID: link.resp.ExpectedMsgID}}
	out := ps.handleCreateChildSAOwned(link.resp, msg, inner, false, link.myTr, nil, log)

	got := rtxRecv(t, link.peerTr)
	if got == nil {
		t.Fatal("a new Child SA request with KEi drew no answer")
	}
	types := errNotifyIn(t, link.ini, got)
	if len(types) != 1 || types[0] != wire.NotifyNoAdditionalSAs {
		t.Errorf("the answer carries notifies %v, want exactly NO_ADDITIONAL_SAS", types)
	}
	if ps.pendingIKESwap != nil {
		t.Error("a new Child SA request staged an IKE SA swap")
	}
	if out.newSA != nil {
		t.Error("a new Child SA request produced a new IKE SA")
	}
	if ps.getChildSA() != child {
		t.Error("the live Child SA changed")
	}
}

// VALIDATES: AC-8 negative. A request with SA, Ni and KEi and no TS payload is still
// an IKE SA rekey (RFC 7296 Section 1.3.2, "HDR, SK {SA, Ni, KEi}").
// Method: Ze's own IKE SA rekey request, built by initiateIKERekey on the initiator,
// is decrypted by the responder and fed to handleCreateChildSAOwned; the responder
// stages the IKE SA swap.
func TestIKERekeyWithoutTSStaysAnIKERekey(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ps := link.ps
	ps.peerName = "ike-rekey-no-ts"

	reqBytes, pending, err := initiateIKERekey(link.ini, link.ini.IKEGroup)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	reqMsg := parseMsg(t, reqBytes)
	inner, err := decryptAndParse(link.resp, reqMsg, reqBytes)
	if err != nil {
		t.Fatalf("the responder could not decrypt the rekey request: %v", err)
	}
	for i := range inner {
		if _, ok := inner[i].Payload.(*wire.PayloadTS); ok {
			t.Fatal("precondition: an IKE SA rekey request carries a TS payload")
		}
	}
	ps.handleCreateChildSAOwned(link.resp, reqMsg, inner, false, link.myTr, nil, log)
	if ps.pendingIKESwap == nil {
		t.Fatal("an IKE SA rekey request staged no IKE SA swap")
	}
	ps.pendingIKESwap.forgetKeys()
}

// peerNewChildRequest builds a peer CREATE_CHILD_SA request for a new Child SA, the RFC 7296
// Section 1.3.1 shape "HDR, SK {SA, Ni, [KEi,] TSi, TSr}". A non-nil dh adds KEi in its
// group and offers that group in the SA payload, as Section 3.4 requires; transport adds
// USE_TRANSPORT_MODE; tsi and tsr are the proposed scopes.
func peerNewChildRequest(t *testing.T, espGroup ipsec.ESPGroup, peerESPSPI uint32, ni []byte,
	dh *crypto.DHExchange, transport bool, tsi, tsr string,
) []wire.PayloadEntry {
	t.Helper()
	group := dhGroupNone
	if dh != nil {
		group = dh.GroupID
	}
	inner := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: buildWireESPProposals(espGroup, peerESPSPI, group)}},
		{Payload: &wire.PayloadNonce{NonceData: ni}},
	}
	if dh != nil {
		inner = append(inner, wire.PayloadEntry{
			Payload: &wire.PayloadKE{DHGroup: uint16(group), KeyExchangeData: dh.PublicKey},
		})
	}
	if transport {
		inner = append(inner, wire.PayloadEntry{Payload: transportModeNotify()})
	}
	return append(inner,
		wire.PayloadEntry{Payload: tsPayload(t, wire.PayloadTypeTSi, tsi)},
		wire.PayloadEntry{Payload: tsPayload(t, wire.PayloadTypeTSr, tsr)},
	)
}

// VALIDATES: AC-7. A childless IKE SA answers a peer's new Child SA request with SA, Nr,
// [KEr], [USE_TRANSPORT_MODE], TSi and TSr, installs the Child SA keyed per RFC 7296
// Section 2.17 from THIS exchange's nonces (and the D-H secret when KEi was sent), and
// hands it to the owner loop as a created Child SA. A refused request answers the error
// notify and keeps the IKE SA childless.
// PREVENTS: the childless arm that answered every new Child SA request with
// NO_PROPOSAL_CHOSEN, which left a strongSwan initiator's second child unreachable.
// Method: an established PSK pair; the responder session's Child SA is dropped, the
// request is fed to handleCreateChildSAOwned, and the initiator SA decrypts the answer.
// The test derives the KEYMAT itself from the nonces and its own D-H half and compares.
func TestResponderCreatesChildOnChildlessSA(t *testing.T) {
	cases := []struct {
		name      string
		pfs       bool
		transport bool
	}{
		{name: "without KEi"},
		{name: "with KEi", pfs: true},
		{name: "transport mode echoed", transport: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := slogutil.DiscardLogger()
			link := errLink(t)
			ps := link.ps
			ps.peerName = "create-child"
			ps.espGroup = testESPGroup()
			tsi, tsr := "0.0.0.0/0", "0.0.0.0/0"
			if tc.pfs {
				ps.espGroup.PFS = ipsec.PFSEnable
			}
			if tc.transport {
				link.resp.PeerCfg.Mode = dataplane.ModeTransport
				tsi, tsr = "127.0.0.1/32", "127.0.0.1/32"
			}
			ps.setChildSA(nil)

			var dh *crypto.DHExchange
			if tc.pfs {
				var err error
				dh, err = crypto.NewDHExchange(link.resp.Proposal.DHGroup.ID)
				if err != nil {
					t.Fatalf("NewDHExchange: %v", err)
				}
				defer dh.Clear()
			}
			ni := testNonce(61)
			const peerSPI = 0x0c0ffee0
			inner := peerNewChildRequest(t, ps.espGroup, peerSPI, ni, dh, tc.transport, tsi, tsr)
			msg := &wire.Message{Header: wire.Header{MessageID: link.resp.ExpectedMsgID}}
			out := ps.handleCreateChildSAOwned(link.resp, msg, inner, false, link.myTr, nil, log)

			child := ps.getChildSA()
			if child == nil {
				t.Fatal("the childless SA installed no Child SA")
			}
			if out.createdChild != child {
				t.Error("the owner loop was not handed the created Child SA")
			}
			if out.newChild != nil {
				t.Error("a created Child SA was reported as a rekey")
			}
			if child.OutboundSPI != peerSPI {
				t.Errorf("outbound SPI %#x, want the peer's %#x", child.OutboundSPI, uint32(peerSPI))
			}
			if child.LocalIsInitiator {
				t.Error("the responder of the creation keyed itself as its initiator")
			}

			raw := rtxRecv(t, link.peerTr)
			if raw == nil {
				t.Fatal("the creation request drew no answer")
			}
			answer, err := decryptAndParse(link.ini, parseMsg(t, raw), raw)
			if err != nil {
				t.Fatalf("the peer could not decrypt the answer: %v", err)
			}
			var order []uint8
			var nr []byte
			var ker *wire.PayloadKE
			var sawTransport bool
			for i := range answer {
				switch p := answer[i].Payload.(type) {
				case *wire.PayloadSA:
					order = append(order, wire.PayloadTypeSA)
					spi, err := espSPIFromSA(p)
					if err != nil || spi != child.InboundSPI {
						t.Errorf("SA payload SPI %#x (%v), want the installed inbound %#x", spi, err, child.InboundSPI)
					}
				case *wire.PayloadNonce:
					order = append(order, wire.PayloadTypeNonce)
					nr = p.NonceData
				case *wire.PayloadKE:
					order = append(order, wire.PayloadTypeKE)
					ker = p
				case *wire.PayloadNotify:
					if p.NotifyMsgType == wire.NotifyUseTransportMode {
						sawTransport = true
						continue
					}
					t.Errorf("the answer carries notify %d", p.NotifyMsgType)
				case *wire.PayloadTS:
					order = append(order, p.TSPayloadType)
				}
			}
			want := []uint8{wire.PayloadTypeSA, wire.PayloadTypeNonce}
			if tc.pfs {
				want = append(want, wire.PayloadTypeKE)
			}
			want = append(want, wire.PayloadTypeTSi, wire.PayloadTypeTSr)
			if !slices.Equal(order, want) {
				t.Errorf("answer payloads %v, want %v", order, want)
			}
			if sawTransport != tc.transport {
				t.Errorf("USE_TRANSPORT_MODE in the answer = %v, want %v", sawTransport, tc.transport)
			}
			if tc.transport && child.Mode != modeTransport {
				t.Error("the created Child SA is not in transport mode")
			}

			enc, integ, err := resolveESPTransforms(ps.espGroup.Proposals[0])
			if err != nil {
				t.Fatalf("resolveESPTransforms: %v", err)
			}
			var keys *crypto.ChildSAKeys
			if tc.pfs {
				if ker == nil {
					t.Fatal("a creation with KEi was answered with no KEr")
				}
				shared, err := dh.SharedSecret(ker.KeyExchangeData)
				if err != nil {
					t.Fatalf("SharedSecret: %v", err)
				}
				keys, err = crypto.DeriveChildSAKeysPFS(link.ini.Proposal.PRF.ID, link.ini.SKKeys.SK_d,
					shared, ni, nr, enc, integ)
				if err != nil {
					t.Fatalf("DeriveChildSAKeysPFS: %v", err)
				}
			} else {
				keys, err = crypto.DeriveChildSAKeys(link.ini.Proposal.PRF.ID, link.ini.SKKeys.SK_d,
					ni, nr, enc, integ)
				if err != nil {
					t.Fatalf("DeriveChildSAKeys: %v", err)
				}
			}
			if !bytes.Equal(keys.EncryptKeyI, child.Keys.EncryptKeyI) {
				t.Error("the Child SA is not keyed from this exchange's KEYMAT (RFC 7296 Section 2.17)")
			}
		})
	}
}

// VALIDATES: AC-7 refusal. A new Child SA request the childless SA cannot accept is
// answered with the matching error notify (RFC 7296 Section 2.21.3) and the IKE SA stays
// up with no Child SA.
// Method: the request offers transforms the esp-group does not hold.
func TestResponderRefusesUnacceptableNewChild(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ps := link.ps
	ps.peerName = "create-child-refused"
	ps.espGroup = testESPGroup()
	ps.setChildSA(nil)

	inner := peerNewChildRequest(t, ps.espGroup, 0x0c0ffee1, testNonce(62), nil, false, "0.0.0.0/0", "0.0.0.0/0")
	for i := range inner {
		if sa, ok := inner[i].Payload.(*wire.PayloadSA); ok {
			for p := range sa.Proposals {
				for tf := range sa.Proposals[p].Transforms {
					// 9999 names no transform any RFC assigns.
					sa.Proposals[p].Transforms[tf].ID = 9999
				}
			}
		}
	}
	msg := &wire.Message{Header: wire.Header{MessageID: link.resp.ExpectedMsgID}}
	out := ps.handleCreateChildSAOwned(link.resp, msg, inner, false, link.myTr, nil, log)

	raw := rtxRecv(t, link.peerTr)
	if raw == nil {
		t.Fatal("the refused request drew no answer")
	}
	types := errNotifyIn(t, link.ini, raw)
	if len(types) != 1 || types[0] != wire.NotifyNoProposalChosen {
		t.Errorf("the answer carries notifies %v, want exactly NO_PROPOSAL_CHOSEN", types)
	}
	if ps.getChildSA() != nil || out.createdChild != nil {
		t.Error("a refused request installed a Child SA")
	}
	if link.resp.State != StateEstablished {
		t.Errorf("the IKE SA is %v after a refused Child SA, want established", link.resp.State)
	}
}

// createRig is one established PeerSession holding NO Child SA, with a UDP link to a
// stand-in peer whose end of the same IKE SA answers through respondNewChild, the
// phase 3 responder. It reuses newRefusedRig and drops the Child SA that rig builds.
func createRig(t *testing.T, pfs ipsec.PFSMode) *refusedRig {
	t.Helper()
	r := newRefusedRig(t)
	r.ps.setChildSA(nil)
	r.ps.espGroup = testESPGroup()
	r.ps.espGroup.PFS = pfs
	r.ps.childCreate.start(time.Now().Add(-childCreateFirst))
	return r
}

// relayCreate is relayToPeer in the order handleOwnedInbound keeps: the authenticated
// response frees the request window BEFORE the handler runs, so a Delete the handler
// sends finds the window free.
func (r *refusedRig) relayCreate(t *testing.T, respond func(req []wire.PayloadEntry, msgID uint32) []byte) ([]wire.PayloadEntry, ownedOutcome) {
	t.Helper()
	raw := rtxRecv(t, r.peerTr)
	if raw == nil {
		t.Fatal("the creation request never reached the peer")
	}
	if r.ps.pendingRekey == nil {
		t.Fatal("the creation left no outstanding exchange")
	}
	msgID := r.ps.pendingRekey.messageID
	req, err := decryptAndParse(r.peer, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("the peer could not decrypt the creation request: %v", err)
	}
	answer := respond(req, msgID)
	inner, err := decryptAndParse(r.sa, parseMsg(t, answer), answer)
	if err != nil {
		t.Fatalf("ze could not decrypt the peer's answer: %v", err)
	}
	r.sa.answerAuthenticatedResponse(msgID)
	respMsg := &wire.Message{Header: wire.Header{MessageID: msgID}}
	out := r.ps.handleCreateChildSAOwned(r.sa, respMsg, inner, true, r.myTr, r.dp, slogutil.DiscardLogger())
	return req, out
}

// peerAnswersCreate is a relayCreate responder: the peer's real respondNewChild.
func (r *refusedRig) peerAnswersCreate(t *testing.T, peerChild **ChildSA) func([]wire.PayloadEntry, uint32) []byte {
	return func(req []wire.PayloadEntry, msgID uint32) []byte {
		resp, child, err := respondNewChild(r.peer, req, r.ps.espGroup, msgID, nil, slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("the peer refused the creation request: %v", err)
		}
		*peerChild = child
		return resp
	}
}

// peerScripted answers with the given inner chain, encrypted by the peer's end.
func (r *refusedRig) peerScripted(t *testing.T, inner []wire.PayloadEntry) func([]wire.PayloadEntry, uint32) []byte {
	return func(_ []wire.PayloadEntry, msgID uint32) []byte {
		resp, err := buildEncryptedMessageEx(r.peer, inner, msgID, wire.ExchangeCreateChildSA,
			initiatorFlag(r.peer)|wire.FlagResponse)
		if err != nil {
			t.Fatalf("build scripted answer: %v", err)
		}
		return resp
	}
}

func payloadOrder(inner []wire.PayloadEntry) []uint8 {
	var order []uint8
	for i := range inner {
		switch p := inner[i].Payload.(type) {
		case *wire.PayloadTS:
			order = append(order, p.TSPayloadType)
		case *wire.PayloadNotify:
		default:
			order = append(order, p.Type())
		}
	}
	return order
}

// VALIDATES: AC-4 and AC-5. A childless IKE SA whose creation is due sends
// CREATE_CHILD_SA {SA, Ni, [KEi], TSi, TSr}; with pfs the KEi is in the IKE SA's group
// and the SA offer names that group (RFC7296-1.3-1), without pfs no KEi goes out. The
// answer installs a Child SA keyed from this exchange, identical to the peer's, with
// this node as the exchange initiator, and the schedule stops.
// Method: both ends of one IKE SA; the peer answers through its real respondNewChild.
// PREVENTS: a childless IKE SA that never asks for its Child SA.
func TestChildlessInitiatorCreatesChildSA(t *testing.T) {
	for _, pfs := range []ipsec.PFSMode{ipsec.PFSDisable, ipsec.PFSEnable} {
		t.Run(map[ipsec.PFSMode]string{ipsec.PFSDisable: "no pfs", ipsec.PFSEnable: "pfs"}[pfs], func(t *testing.T) {
			r := createRig(t, pfs)
			log := slogutil.DiscardLogger()
			r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
			if r.ps.pendingRekey == nil || r.ps.pendingRekey.kind != rekeyCreate {
				t.Fatal("a due creation sent no creation request")
			}
			var peerChild *ChildSA
			req, out := r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))

			want := []uint8{wire.PayloadTypeSA, wire.PayloadTypeNonce}
			if pfs == ipsec.PFSEnable {
				want = append(want, wire.PayloadTypeKE)
			}
			want = append(want, wire.PayloadTypeTSi, wire.PayloadTypeTSr)
			if got := payloadOrder(req); !slices.Equal(got, want) {
				t.Errorf("request payloads %v, want %v", got, want)
			}
			if pfs == ipsec.PFSEnable {
				ke := rkyFindKE(t, req)
				if crypto.DHGroupID(ke.DHGroup) != r.sa.Proposal.DHGroup.ID {
					t.Errorf("KEi group %d, want the IKE SA's %d", ke.DHGroup, r.sa.Proposal.DHGroup.ID)
				}
				for i := range req {
					if offer, ok := req[i].Payload.(*wire.PayloadSA); ok {
						if err := offer.ValidateKEGroup(ke); err != nil {
							t.Errorf("RFC7296-1.3-1: the offer does not name the KEi group: %v", err)
						}
					}
				}
			}

			child := r.ps.getChildSA()
			if child == nil || out.createdChild != child {
				t.Fatal("the answered creation installed no Child SA")
			}
			if peerChild == nil {
				t.Fatal("the peer installed no Child SA")
			}
			if !child.LocalIsInitiator {
				t.Error("the node that sent Ni keyed itself as the exchange responder")
			}
			if child.OutboundSPI != peerChild.InboundSPI || child.InboundSPI != peerChild.OutboundSPI {
				t.Error("the two ends disagree on the SPIs")
			}
			if !bytes.Equal(child.Keys.EncryptKeyI, peerChild.Keys.EncryptKeyI) {
				t.Error("the two ends derived different keys (RFC 7296 Section 2.17)")
			}
			if r.ps.childCreate.active {
				t.Error("the creation schedule still runs after a Child SA was installed")
			}
			if r.ps.pendingRekey != nil {
				t.Error("the creation exchange is still pending")
			}
		})
	}
}

// VALIDATES: AC-5 negatives. A pfs creation answered with no KEr is refused and nothing
// is installed; the peer's half is deleted. An INVALID_KE_PAYLOAD naming another
// configured group retries at once in that group.
func TestCreateChildInvalidKERetriesNamedGroup(t *testing.T) {
	r := createRig(t, ipsec.PFSEnable)
	extra := r.sa.IKEGroup.Proposals[0]
	extra.Number++
	extra.DHGroup = 19
	r.sa.IKEGroup.Proposals = append(r.sa.IKEGroup.Proposals, extra)
	log := slogutil.DiscardLogger()

	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	r.relayCreate(t, r.peerScripted(t, refusedNotify(wire.NotifyInvalidKEPayload, []byte{0, 19})))
	if !r.ps.childCreate.due(time.Now()) {
		t.Fatal("INVALID_KE_PAYLOAD naming a configured group did not retry at once")
	}
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	if r.ps.pendingRekey == nil {
		t.Fatal("the retry sent no request")
	}
	req, _ := r.relayCreate(t, r.peerScripted(t, refusedNotify(wire.NotifyNoProposalChosen, nil)))
	if got := rkyFindKE(t, req).DHGroup; got != 19 {
		t.Errorf("the retry carried KEi in group %d, want 19", got)
	}
}

// VALIDATES: AC-5. A creation with KEi answered by a success response without KEr is
// refused: no Child SA, a Delete for the pair the peer installed, the schedule backs off.
func TestCreateChildWithoutKErIsRefused(t *testing.T) {
	r := createRig(t, ipsec.PFSEnable)
	log := slogutil.DiscardLogger()
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	inSPI := r.ps.pendingRekey.newInboundSPI
	prop := r.ps.espGroup.Proposals[0]
	answer, err := espProposalWire(prop, 0x0abc0001, 1, r.sa.Proposal.DHGroup.ID)
	if err != nil {
		t.Fatalf("espProposalWire: %v", err)
	}
	_, out := r.relayCreate(t, r.peerScripted(t, []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: []wire.Proposal{answer}}},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(71)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "0.0.0.0/0")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")},
	}))
	if out.createdChild != nil || r.ps.getChildSA() != nil {
		t.Fatal("a pfs creation answered without KEr installed a Child SA")
	}
	expectDeleteOf(t, r, inSPI)
	if r.ps.childCreate.wait != 2*childCreateFirst {
		t.Errorf("the creation wait is %v after a refusal, want %v", r.ps.childCreate.wait, 2*childCreateFirst)
	}
}

// expectDeleteOf reads the next datagram at the peer and checks it is an INFORMATIONAL
// Delete naming the ESP SPI spi.
func expectDeleteOf(t *testing.T, r *refusedRig, spi uint32) {
	t.Helper()
	raw := rtxRecv(t, r.peerTr)
	if raw == nil {
		t.Fatal("no Delete reached the peer")
	}
	inner, err := decryptAndParse(r.peer, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("the peer could not decrypt the Delete: %v", err)
	}
	for i := range inner {
		del, ok := inner[i].Payload.(*wire.PayloadDelete)
		if !ok || del.ProtocolID != wire.ProtocolESP || len(del.SPIs) < 4 {
			continue
		}
		if binary.BigEndian.Uint32(del.SPIs) == spi {
			return
		}
	}
	t.Errorf("the peer received no ESP Delete for SPI %#x", spi)
}

// VALIDATES: AC-6. Each error answer to a creation is routed: TEMPORARY_FAILURE waits
// 60 s with the wait unchanged; NO_ADDITIONAL_SAS re-establishes; NO_PROPOSAL_CHOSEN,
// TS_UNACCEPTABLE and an unrecognized error keep the IKE SA and back off. None of them
// sets a Child or IKE rekey hold.
func TestCreateChildRefusalRouting(t *testing.T) {
	cases := []struct {
		name        string
		notify      uint16
		reestablish bool
		hold        time.Duration
		wait        time.Duration
	}{
		{name: "TEMPORARY_FAILURE", notify: wire.NotifyTemporaryFailure, hold: temporaryFailureBackoff, wait: childCreateFirst},
		{name: "NO_ADDITIONAL_SAS", notify: wire.NotifyNoAdditionalSAs, reestablish: true},
		{name: "NO_PROPOSAL_CHOSEN", notify: wire.NotifyNoProposalChosen, hold: 2 * childCreateFirst, wait: 2 * childCreateFirst},
		{name: "TS_UNACCEPTABLE", notify: wire.NotifyTSUnacceptable, hold: 2 * childCreateFirst, wait: 2 * childCreateFirst},
		{name: "unrecognized", notify: unrecognizedErrorType(t), hold: 2 * childCreateFirst, wait: 2 * childCreateFirst},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := createRig(t, ipsec.PFSDisable)
			log := slogutil.DiscardLogger()
			r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
			before := time.Now()
			_, out := r.relayCreate(t, r.peerScripted(t, refusedNotify(tc.notify, nil)))
			if out.reestablish != tc.reestablish {
				t.Fatalf("reestablish = %v, want %v", out.reestablish, tc.reestablish)
			}
			if r.ps.pendingRekey != nil {
				t.Error("the refused creation is still pending")
			}
			if r.ps.getChildSA() != nil {
				t.Error("a refused creation installed a Child SA")
			}
			if !r.ps.childRekeyHoldUntil.IsZero() || !r.ps.childRekeyRefusedUntil.IsZero() ||
				!r.ps.ikeRekeyHoldUntil.IsZero() || !r.ps.ikeRekeyRefusedUntil.IsZero() {
				t.Error("a refused creation set a rekey hold")
			}
			if tc.reestablish {
				return
			}
			if r.ps.childCreate.wait != tc.wait {
				t.Errorf("wait %v, want %v", r.ps.childCreate.wait, tc.wait)
			}
			if r.ps.childCreate.next.Before(before.Add(tc.hold)) || r.ps.childCreate.next.After(time.Now().Add(tc.hold)) {
				t.Errorf("next attempt in %v, want %v", r.ps.childCreate.next.Sub(before), tc.hold)
			}
		})
	}
}

// VALIDATES: Q-2. The first attempt is 30 s after establishment, each refusal doubles
// the wait up to 300 s, and a stop ends the schedule.
func TestCreateChildRetrySchedule(t *testing.T) {
	now := time.Unix(1000, 0)
	var s childCreateSchedule
	if s.due(now) {
		t.Fatal("the zero schedule is due")
	}
	s.start(now)
	if s.due(now.Add(childCreateFirst - time.Second)) {
		t.Error("due before 30 s")
	}
	if !s.due(now.Add(childCreateFirst)) {
		t.Error("not due at 30 s")
	}
	for _, want := range []time.Duration{60, 120, 240, 300, 300} {
		s.refused(now)
		if s.wait != want*time.Second || !s.next.Equal(now.Add(want*time.Second)) {
			t.Errorf("wait %v next +%v, want %v", s.wait, s.next.Sub(now), want*time.Second)
		}
	}
	s.stop()
	if s.due(now.Add(time.Hour)) {
		t.Error("a stopped schedule is due")
	}
}

// VALIDATES: AC-11. No creation request goes out while our IKE SA rekey is pending, a
// peer IKE SA rekey awaits its swap, or the request window is held; once those clear it
// does. The schedule survives (AC-10, an IKE SA rekey keeps creation scheduled).
func TestCreateChildWaitsForIKERekey(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	log := slogutil.DiscardLogger()
	ikeRekey := &pendingRekey{kind: rekeyIKE}
	r.ps.pendingRekey = ikeRekey
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	if r.ps.pendingRekey != ikeRekey {
		t.Fatal("a creation went out while an IKE SA rekey was pending")
	}
	r.ps.pendingRekey = nil
	r.ps.pendingIKESwap = &SA{}
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	if r.ps.pendingRekey != nil {
		t.Fatal("a creation went out while a peer IKE SA rekey awaited its swap")
	}
	r.ps.pendingIKESwap = nil
	if !r.sa.reserveRequestWindow() {
		t.Fatal("precondition: the window is held")
	}
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	if r.ps.pendingRekey != nil {
		t.Fatal("a creation went out while the request window was held")
	}
	r.sa.releaseRequestWindow()
	if !r.ps.childCreate.due(time.Now()) {
		t.Fatal("the deferred creation is no longer scheduled")
	}
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	if r.ps.pendingRekey == nil || r.ps.pendingRekey.kind != rekeyCreate {
		t.Fatal("the creation did not go out once the IKE SA was free")
	}
}

// VALIDATES: AC-12. A successful creation answer that arrives after the peer created the
// session's Child SA is not installed; ze deletes the new pair and the live Child SA is
// untouched.
func TestCreateChildCollisionDeletesNewPair(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	log := slogutil.DiscardLogger()
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	inSPI := r.ps.pendingRekey.newInboundSPI
	r.ps.setChildSA(r.old)
	var peerChild *ChildSA
	_, out := r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	if out.createdChild != nil {
		t.Error("the colliding pair was handed to the owner loop")
	}
	if r.ps.getChildSA() != r.old {
		t.Error("the live Child SA changed")
	}
	expectDeleteOf(t, r, inSPI)
}
