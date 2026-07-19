// VALIDATES: RFC 7296 (IKEv2) MUST-level obligations enrolled into `make ze-rfc-check`:
// the initial-exchange encryption boundary (§1.2), DPD empty-INFORMATIONAL echo (§2.4),
// mandatory KE on IKE-SA rekey (§1.3.3), AEAD/non-AEAD proposal separation and INTEG-NONE
// (§3.3), IKE proposal transform completeness and mandatory DH (§3.3.2/§3.3.6), NAT ESP
// port-floating scope (§2.23), non-negotiated lifetimes (§2.8), and INITIAL_CONTACT
// placement (§2.4). Each test carries an `RFC requirement:` tag binding it to its checklist id.
// PREVENTS: silent regressions in these wire/state-machine invariants going undetected by
// the RFC coverage gate.
package engine

import (
	"errors"
	"strings"
	"testing"

	ikecrypto "codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// hasPayload reports whether a parsed payload chain contains a payload of the given type.
func hasSKPayload(msg *wire.Message) bool {
	for i := range msg.Payloads {
		if _, ok := msg.Payloads[i].Payload.(*wire.PayloadSK); ok {
			return true
		}
	}
	return false
}

// RFC requirement: RFC7296-1.2-1 positive -- the second exchange pair (IKE_AUTH) is encrypted
// under the IKE SA keys: the initiator's IKE_AUTH request carries an SK payload and the peer
// decrypts it to recover the inner ID/AUTH/SA/TS payloads.
// RFC requirement: RFC7296-1.2-1 negative -- the first exchange pair (IKE_SA_INIT) is NOT
// encrypted: its message carries no SK payload and decryptAndParse rejects it as "no SK
// payload", so the SA/KE/Nonce are sent in the clear as the RFC requires.
func TestInitialExchangeEncryptionBoundary(t *testing.T) {
	ini, resp, _ := establishPSK(t)

	// First pair: IKE_SA_INIT must be unencrypted (no SK payload).
	initMsg := parseMsg(t, ini.InitiatorSAInitMsg)
	if initMsg.Header.ExchangeType != wire.ExchangeIKESAInit {
		t.Fatalf("first message exchange = %d, want IKE_SA_INIT (34)", initMsg.Header.ExchangeType)
	}
	if hasSKPayload(initMsg) {
		t.Error("IKE_SA_INIT must not carry an SK (encrypted) payload -- the first pair is unencrypted")
	}
	if _, err := decryptAndParse(ini, initMsg, ini.InitiatorSAInitMsg); err == nil {
		t.Error("decryptAndParse accepted an IKE_SA_INIT as encrypted; the first pair must be plaintext")
	}

	// Second pair: IKE_AUTH must be encrypted under the derived IKE SA keys.
	authMsg := parseMsg(t, ini.LastSentMsg)
	if authMsg.Header.ExchangeType != wire.ExchangeIKEAuth {
		t.Fatalf("second-pair message exchange = %d, want IKE_AUTH (35)", authMsg.Header.ExchangeType)
	}
	if !hasSKPayload(authMsg) {
		t.Error("IKE_AUTH must carry an SK (encrypted) payload -- the second pair is encrypted")
	}
	inner, err := decryptAndParse(resp, authMsg, ini.LastSentMsg)
	if err != nil {
		t.Fatalf("peer could not decrypt the IKE_AUTH (second pair must be SK-encrypted): %v", err)
	}
	if len(inner) == 0 {
		t.Error("decrypted IKE_AUTH carried no inner payloads")
	}
}

// RFC requirement: RFC7296-2.4-1 positive -- an empty INFORMATIONAL request (a DPD probe) is
// answered with an empty INFORMATIONAL response: handleInformationalOwned (inbound.go:270-276)
// builds an SK-encrypted response with no inner payloads that echoes the request message ID.
// RFC requirement: RFC7296-2.4-1 negative -- an INFORMATIONAL *response* (isResponse=true) is
// NOT answered, so a DPD ack never triggers another response (no infinite ping-pong).
func TestDPDEmptyInformationalGetsEmptyResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)

	const probeID = 42
	req := &wire.Message{Header: wire.Header{
		InitiatorSPI: resp.InitiatorSPI,
		ResponderSPI: resp.ResponderSPI,
		MajorVersion: 2,
		ExchangeType: wire.ExchangeInformational,
		Flags:        wire.FlagInitiator,
		MessageID:    probeID,
	}}

	resp.lastResponse = nil
	resp.lastResponseSet = false
	ps.handleInformationalOwned(resp, req, nil, false, nil, nil, log)

	if resp.lastResponse == nil {
		t.Fatal("empty INFORMATIONAL request produced no response (DPD probe unanswered)")
	}
	respMsg := parseMsg(t, resp.lastResponse)
	if respMsg.Header.ExchangeType != wire.ExchangeInformational {
		t.Errorf("response exchange = %d, want INFORMATIONAL", respMsg.Header.ExchangeType)
	}
	if respMsg.Header.Flags&wire.FlagResponse == 0 {
		t.Error("DPD response is missing the Response flag")
	}
	if respMsg.Header.MessageID != probeID {
		t.Errorf("response message ID = %d, want %d (echo the probe)", respMsg.Header.MessageID, probeID)
	}
	inner, err := decryptAndParse(ini, respMsg, resp.lastResponse)
	if err != nil {
		t.Fatalf("peer could not decrypt the DPD response: %v", err)
	}
	if len(inner) != 0 {
		t.Errorf("DPD response carried %d inner payloads, want 0 (empty INFORMATIONAL)", len(inner))
	}

	// Negative: a response must not itself be answered.
	resp.lastResponse = nil
	resp.lastResponseSet = false
	ps.handleInformationalOwned(resp, req, nil, true, nil, nil, log)
	if resp.lastResponse != nil {
		t.Error("an INFORMATIONAL response was itself answered -- DPD would ping-pong forever")
	}
}

// RFC requirement: RFC7296-1.3.3-1 negative -- an IKE-SA rekey request that omits the mandatory
// KE payload is rejected: respondIKERekey (rekey.go:454) returns an error naming the missing
// KEi rather than deriving keys without a fresh Diffie-Hellman exchange.
func TestRespondIKERekeyRejectsMissingKE(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa := testSAWithGCMKeys(t)
	sa.IKEGroup = testIKEGroup()

	inner := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: []wire.Proposal{{
			Number: 1, ProtocolID: wire.ProtocolIKE, SPISize: 8, SPI: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		}}}},
		{Payload: &wire.PayloadNonce{NonceData: make([]byte, nonceLen)}},
		// No KE payload: KE is mandatory for an IKE-SA rekey.
	}

	_, _, err := respondIKERekey(sa, inner, 2, log)
	if err == nil {
		t.Fatal("respondIKERekey accepted an IKE-SA rekey with no KE payload")
	}
	if !strings.Contains(err.Error(), "KEi") {
		t.Errorf("error = %q, want it to name the missing KEi", err.Error())
	}
}

// RFC requirement: RFC7296-3.3-2 positive -- an AEAD ESP proposal is emitted with INTEG NONE:
// buildWireESPProposals (initiator.go:303) omits the integrity transform entirely for an AEAD
// cipher, so no separate INTEG algorithm accompanies AEAD.
// RFC requirement: RFC7296-3.3-2 negative -- a non-AEAD ESP proposal DOES carry an integrity
// transform (initiator.go:304-307), so the AEAD rule never strips integrity from a cipher that
// needs it.
func TestESPWireProposalAEADIntegNone(t *testing.T) {
	grp := ipsec.ESPGroup{Proposals: []ipsec.ESPProposal{
		{Number: 1, Encryption: ipsec.EncryptionAES256GCM},                      // AEAD, no hash
		{Number: 2, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256}, // non-AEAD
	}}
	props := buildWireESPProposals(grp, 0x11223344)
	if len(props) != 2 {
		t.Fatalf("built %d wire proposals, want 2", len(props))
	}

	if countTransform(props[0], wire.TransformTypeINTG) != 0 {
		t.Error("AEAD ESP proposal carries an INTEG transform; INTEG must be NONE for AEAD")
	}
	if countTransform(props[1], wire.TransformTypeINTG) != 1 {
		t.Error("non-AEAD ESP proposal is missing its INTEG transform")
	}
}

// RFC requirement: RFC7296-3.3-1 positive -- AEAD and non-AEAD ciphers land in SEPARATE
// proposals: buildWireESPProposals (initiator.go:294) emits one wire proposal per configured
// proposal, so a group mixing an AEAD and a non-AEAD cipher yields two single-class proposals
// (the AEAD one with no INTEG, the non-AEAD one with INTEG), never one proposal mixing both.
func TestESPProposalsNeverMixAEADClass(t *testing.T) {
	grp := ipsec.ESPGroup{Proposals: []ipsec.ESPProposal{
		{Number: 1, Encryption: ipsec.EncryptionAES256GCM},
		{Number: 2, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256},
	}}
	props := buildWireESPProposals(grp, 0x55667788)
	if len(props) != 2 {
		t.Fatalf("built %d wire proposals, want 2 (one per configured proposal, never merged)", len(props))
	}
	// AEAD proposal: exactly one ENCR, zero INTEG (single-class AEAD).
	if countTransform(props[0], wire.TransformTypeENCR) != 1 || countTransform(props[0], wire.TransformTypeINTG) != 0 {
		t.Error("AEAD proposal is not single-class (want one ENCR, no INTEG)")
	}
	// non-AEAD proposal: one ENCR and one INTEG (single-class non-AEAD).
	if countTransform(props[1], wire.TransformTypeENCR) != 1 || countTransform(props[1], wire.TransformTypeINTG) != 1 {
		t.Error("non-AEAD proposal is not single-class (want one ENCR and one INTEG)")
	}
}

// RFC requirement: RFC7296-3.3.2-1 positive -- an IKE SA proposal carries all four transform
// types: buildWireIKEProposals (initiator.go:120-125) emits exactly one ENCR, PRF, INTEG and DH
// transform per proposal.
// RFC requirement: RFC7296-3.3.2-1 negative -- an IKE proposal missing a required transform (here
// the PRF) does not negotiate: NegotiateIKE (proposal.go:27) returns NO_PROPOSAL_CHOSEN because
// the incomplete proposal cannot match a complete local one.
func TestIKEWireProposalHasAllTransforms(t *testing.T) {
	props := buildWireIKEProposals(testIKEGroup())
	if len(props) == 0 {
		t.Fatal("buildWireIKEProposals returned no proposals")
	}
	full := props[0]
	for _, ty := range []uint8{wire.TransformTypeENCR, wire.TransformTypePRF, wire.TransformTypeINTG, wire.TransformTypeDH} {
		if countTransform(full, ty) != 1 {
			t.Errorf("IKE proposal has %d transforms of type %d, want exactly 1", countTransform(full, ty), ty)
		}
	}

	// Drop the PRF transform: the resulting proposal must fail negotiation.
	var noPRF []wire.Transform
	for _, tr := range full.Transforms {
		if tr.Type != wire.TransformTypePRF {
			noPRF = append(noPRF, tr)
		}
	}
	remote := []wire.Proposal{{Number: 1, ProtocolID: wire.ProtocolIKE, Transforms: noPRF}}
	if _, err := ikecrypto.NegotiateIKE(wireProposalsToIKE(remote), buildIKEProposals(testIKEGroup())); !errors.Is(err, ikecrypto.ErrNoProposalChosen) {
		t.Errorf("NegotiateIKE on a PRF-less IKE proposal = %v, want ErrNoProposalChosen", err)
	}
}

// RFC requirement: RFC7296-3.3.6-1 positive -- a DH group is mandatory for IKE SA negotiation:
// a valid IKE_SA_INIT carrying a KE payload (whose group matches the negotiated DH group) is
// accepted, advancing handleSAInitRequest (responder.go:193) to StateSAInitReceived.
// RFC requirement: RFC7296-3.3.6-1 negative -- an IKE_SA_INIT that omits the KE payload (and thus
// any DH group) is rejected: handleSAInitRequest (responder.go:115) marks the SA dead.
func TestResponderRequiresKEForDH(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "dh-psk")

	// Positive: a complete IKE_SA_INIT with a KE payload is processed.
	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	req := buildSAInitRequest(ini, testIKEGroup())
	resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, req), req, nil, nil, log)
	if resp.State != StateSAInitReceived {
		t.Fatalf("responder state = %v, want sa-init-received (a valid KE/DH must be accepted)", resp.State)
	}

	// Negative: the same exchange with the KE payload removed is rejected.
	noKE := &wire.Message{
		Header: wire.Header{
			InitiatorSPI: ini.InitiatorSPI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagInitiator,
		},
		Payloads: []wire.PayloadEntry{
			{Payload: &wire.PayloadSA{Proposals: buildWireIKEProposals(testIKEGroup())}},
			{Payload: &wire.PayloadNonce{NonceData: make([]byte, nonceLen)}},
		},
	}
	buf := make([]byte, 4096)
	n := noKE.WriteTo(buf, 0)
	raw := buf[:n]
	resp2, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp2, parseMsg(t, raw), raw, nil, nil, log)
	if resp2.State != StateDead {
		t.Errorf("responder state = %v, want dead (an IKE_SA_INIT without a KE/DH must be rejected)", resp2.State)
	}
}

// RFC requirement: RFC7296-2.23-2 negative -- when no NAT is detected, ESP traffic does NOT
// float to UDP 4500: installChildSA (child.go:235,263) leaves UDP encapsulation off, so the
// port float is confined to the NAT case exercised by TestChildSANATTEncapPorts.
func TestChildSANoNATNoEncap(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa := testSA() // NATDetected defaults to false
	dp := &mockDP{}

	if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2", len(dp.sas))
	}
	for i, s := range dp.sas {
		if s.UDPEncap {
			t.Errorf("SA[%d]: UDPEncap = true without NAT; ESP must stay on the raw ESP path", i)
		}
		if s.UDPEncapSPort != 0 || s.UDPEncapDPort != 0 {
			t.Errorf("SA[%d]: UDP encap ports set (%d/%d) without NAT", i, s.UDPEncapSPort, s.UDPEncapDPort)
		}
	}
}

// RFC requirement: RFC7296-2.8-2 negative -- SA lifetimes are never placed on the wire to be
// negotiated: the IKE and ESP proposals buildWireIKEProposals/buildWireESPProposals emit carry
// only the ENCR/PRF/INTEG/DH/ESN transform types and only the key-length attribute, so no
// lifetime attribute is exchanged.
func TestLifetimesNotNegotiatedOnWire(t *testing.T) {
	allowedTypes := map[uint8]bool{
		wire.TransformTypeENCR: true,
		wire.TransformTypePRF:  true,
		wire.TransformTypeINTG: true,
		wire.TransformTypeDH:   true,
		wire.TransformTypeESN:  true,
	}
	proposals := append(buildWireIKEProposals(testIKEGroup()), buildWireESPProposals(testESPGroup(), 0x1)...)
	if len(proposals) == 0 {
		t.Fatal("no proposals built")
	}
	for _, p := range proposals {
		for _, tr := range p.Transforms {
			if !allowedTypes[tr.Type] {
				t.Errorf("proposal %d carries transform type %d; no lifetime transform may be negotiated", p.Number, tr.Type)
			}
			for _, a := range tr.Attrs {
				if a.Type != wire.AttrTypeKeyLength {
					t.Errorf("proposal %d carries attribute type %d; only key-length is on the wire, lifetimes are local", p.Number, a.Type)
				}
			}
		}
	}
}

// RFC requirement: RFC7296-2.4-4 negative -- INITIAL_CONTACT is confined to the first IKE_AUTH,
// never a later exchange: an IKE-SA rekey CREATE_CHILD_SA request (initiateIKERekey) carries no
// INITIAL_CONTACT notify.
func TestInitialContactAbsentFromRekey(t *testing.T) {
	iniSA, respSA, _ := establishPSK(t)

	reqBytes, pending, err := initiateIKERekey(iniSA, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()

	inner, err := decryptAndParse(respSA, parseMsg(t, reqBytes), reqBytes)
	if err != nil {
		t.Fatalf("decrypt rekey request: %v", err)
	}
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == wire.NotifyInitialContact {
			t.Error("a CREATE_CHILD_SA rekey (a later exchange) carried INITIAL_CONTACT; it belongs only in the first IKE_AUTH")
		}
	}
}

// countTransform counts transforms of the given type in a wire proposal.
func countTransform(p wire.Proposal, ty uint8) int {
	n := 0
	for _, tr := range p.Transforms {
		if tr.Type == ty {
			n++
		}
	}
	return n
}
