package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// pnmIKEGroup returns an IKE group whose proposals carry the given config keys, in
// the order a parsed config holds them. The parser sorts ascending, so the slice
// order is the operator's priority order.
func pnmIKEGroup(numbers ...uint16) ipsec.IKEGroup {
	g := ipsec.IKEGroup{Name: "test-ike"}
	for _, n := range numbers {
		g.Proposals = append(g.Proposals, ipsec.IKEProposal{
			Number:     n,
			Encryption: ipsec.EncryptionAES256,
			Hash:       ipsec.HashSHA256,
			DHGroup:    14,
		})
	}
	return g
}

// pnmESPGroup returns an ESP group whose proposals carry the given config keys.
func pnmESPGroup(numbers ...uint16) ipsec.ESPGroup {
	g := ipsec.ESPGroup{Name: "test-esp", Lifetime: 3600}
	for _, n := range numbers {
		g.Proposals = append(g.Proposals, ipsec.ESPProposal{
			Number:     n,
			Encryption: ipsec.EncryptionAES256,
			Hash:       ipsec.HashSHA256,
		})
	}
	return g
}

// pnmIKEPair returns an IKE group of two proposals. The first names AES-128 and the
// second AES-256, so a responder configured for AES-256 alone must select the
// second one. That makes an echoed number of two a real observation.
func pnmIKEPair(first, second uint16) ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "test-ike",
		Proposals: []ipsec.IKEProposal{
			{Number: first, Encryption: ipsec.EncryptionAES128, Hash: ipsec.HashSHA256, DHGroup: 14},
			{Number: second, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 14},
		},
	}
}

// pnmESPPair mirrors pnmIKEPair for ESP.
func pnmESPPair(first, second uint16) ipsec.ESPGroup {
	return ipsec.ESPGroup{
		Name:     "test-esp",
		Lifetime: 3600,
		Proposals: []ipsec.ESPProposal{
			{Number: first, Encryption: ipsec.EncryptionAES128, Hash: ipsec.HashSHA256},
			{Number: second, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256},
		},
	}
}

// pnmSAPayload returns the single SA payload of a parsed message, or fails.
func pnmSAPayload(t *testing.T, payloads []wire.PayloadEntry) *wire.PayloadSA {
	t.Helper()
	for i := range payloads {
		if sa, ok := payloads[i].Payload.(*wire.PayloadSA); ok {
			return sa
		}
	}
	t.Fatal("the message carries no SA payload")
	return nil
}

// VALIDATES: every SA payload Ze offers numbers its proposals one, two, three, with
// no gap, whatever key the operator gave the proposal in the config.
// PREVENTS: the config key reaching the wire. The key is documented as a priority,
// and RFC 7296 Section 3.3 gives the wire field a different meaning. A peer that
// enforces the RFC refused every offer from a group not keyed one upward.
//
// RFC 7296 Section 3.3 states that each structure MUST have a proposal number one
// greater than the previous structure. It also states that the first Proposal in the
// initiator's SA payload MUST have a Proposal Num of one.
func TestPnmOfferNumbersStartAtOneAndStepByOne(t *testing.T) {
	for _, tc := range []struct {
		name    string
		configs []uint16
	}{
		{"one proposal keyed ten", []uint16{10}},
		{"two proposals keyed ten and twenty", []uint16{10, 20}},
		{"a gap between one and three", []uint16{1, 3}},
		{"already keyed one upward", []uint16{1, 2, 3}},
		{"a key above the octet the wire field holds", []uint16{300}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ike := &wire.PayloadSA{Proposals: buildWireIKEProposals(pnmIKEGroup(tc.configs...))}
			if err := ike.ValidateOfferNumbering(); err != nil {
				t.Errorf("the IKE offer is misnumbered: %v", err)
			}
			for i := range ike.Proposals {
				if got := ike.Proposals[i].Number; got != uint8(i+1) {
					t.Errorf("IKE proposal %d carries wire number %d, want %d", i, got, i+1)
				}
			}

			esp := &wire.PayloadSA{Proposals: buildWireESPProposals(pnmESPGroup(tc.configs...), 0x11223344)}
			if err := esp.ValidateOfferNumbering(); err != nil {
				t.Errorf("the ESP offer is misnumbered: %v", err)
			}
			for i := range esp.Proposals {
				if got := esp.Proposals[i].Number; got != uint8(i+1) {
					t.Errorf("ESP proposal %d carries wire number %d, want %d", i, got, i+1)
				}
			}
		})
	}
}

// VALIDATES: a Ze responder accepts a Ze initiator's IKE_SA_INIT when the operator
// keyed the proposals by priority rather than one upward. The exchange reaches the
// response, and the initiator accepts that response.
// PREVENTS: the Ze-to-Ze break the numbering check introduced. `proposal 10` is a
// documented config, and the responder logged "misnumbered proposals", set StateDead
// and sent nothing at all, not even NO_PROPOSAL_CHOSEN.
func TestPnmZeToZeSAInitAcceptsPriorityKeyedProposals(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "numbering-psk")

	for _, tc := range []struct {
		name    string
		configs []uint16
	}{
		{"proposal ten", []uint16{10}},
		{"proposals one and three", []uint16{1, 3}},
		{"proposals ten and twenty", []uint16{10, 20}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ikeGroup := pnmIKEGroup(tc.configs...)
			espGroup := pnmESPGroup(tc.configs...)

			table := NewSATable()
			ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
			if err != nil {
				t.Fatalf("newInitiatorSA: %v", err)
			}
			table.Insert(ini)
			req := buildSAInitRequest(ini, ikeGroup)
			ini.InitiatorSAInitMsg = req
			ini.State = StateSAInitSent

			resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
			if err != nil {
				t.Fatalf("newResponderSA: %v", err)
			}
			handleSAInitRequest(resp, parseMsg(t, req), req, nil, nil, log)
			if resp.State != StateSAInitReceived {
				t.Fatalf("the responder reached %v, want sa-init-responded", resp.State)
			}
			if len(resp.LastSentMsg) == 0 {
				t.Fatal("the responder sent nothing")
			}

			handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
			if ini.State != StateAuthSent {
				t.Fatalf("the initiator reached %v, want auth-sent", ini.State)
			}
		})
	}
}

// VALIDATES: the SA payload of a response carries the number of the proposal that was
// accepted, which is the number the peer put on that proposal.
// PREVENTS: a response numbered from local config. The peer matches the number
// against the offer it sent, so a local key names a proposal the peer never made.
//
// RFC 7296 Section 3.3.1 governs an accepted proposal. The proposal number in the SA
// payload MUST match the number on the proposal sent that was accepted.
func TestPnmResponseCarriesTheAcceptedProposalNumber(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "numbering-psk")

	// The initiator offers AES-128 then AES-256. The responder accepts AES-256 alone,
	// so the accepted proposal is the second one on the wire.
	iniIKE := pnmIKEPair(10, 20)
	iniESP := pnmESPPair(10, 20)
	respIKE := pnmIKEGroup(7)
	respESP := pnmESPGroup(5)

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, iniIKE, iniESP)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	req := buildSAInitRequest(ini, iniIKE)
	ini.InitiatorSAInitMsg = req
	ini.State = StateSAInitSent

	resp, err := newResponderSA("ze", respPeer, respIKE, respESP, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, req), req, nil, nil, log)
	if resp.State != StateSAInitReceived {
		t.Fatalf("the responder reached %v, want sa-init-responded", resp.State)
	}

	// The IKE_SA_INIT response names one proposal, and it is the second one offered.
	sar := pnmSAPayload(t, parseMsg(t, resp.LastSentMsg).Payloads)
	if len(sar.Proposals) != 1 {
		t.Fatalf("the response carries %d proposals, want exactly one", len(sar.Proposals))
	}
	if sar.Proposals[0].Number != 2 {
		t.Errorf("the accepted IKE proposal is numbered %d, want 2", sar.Proposals[0].Number)
	}

	// The IKE_AUTH response names the accepted ESP proposal by the same rule.
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: respIKE, espGroup: respESP}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	if resp.State != StateEstablished {
		t.Fatalf("the responder reached %v, want established", resp.State)
	}
	inner, err := decryptAndParse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg)
	if err != nil {
		t.Fatalf("the initiator could not read the IKE_AUTH response: %v", err)
	}
	sar2 := pnmSAPayload(t, inner)
	if len(sar2.Proposals) != 1 {
		t.Fatalf("SAr2 carries %d proposals, want exactly one", len(sar2.Proposals))
	}
	if sar2.Proposals[0].Number != 2 {
		t.Errorf("the accepted ESP proposal is numbered %d, want 2", sar2.Proposals[0].Number)
	}
}
