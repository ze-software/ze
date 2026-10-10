package engine

import (
	"bytes"
	"slices"
	"testing"

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
