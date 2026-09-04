// VALIDATES: Perfect Forward Secrecy on a Child SA rekey. The esp-group `pfs` leaf
// decides whether the CREATE_CHILD_SA request carries a KEi, and the shared secret of
// that exchange enters the replacement Child SA's keys through the PFS form of RFC 7296
// Section 2.17, "KEYMAT = prf+(SK_d, g^ir (new) | Ni | Nr)" (rfc/full/rfc7296.txt:3017).
//
// PREVENTS: the defect these tests were written for. `ESPGroup.PFS` was assigned by the
// parser and read by nothing, `initiateChildRekey` built no KE payload, and every rekey
// derived from SK_d alone through the non-PFS form, while the leaf defaulted to enable.
// An operator was told forward secrecy was on and every Child SA was rekeyed from the
// same keying material.
//
// These carry no `RFC requirement:` tag on purpose. Section 2.17 states the two KEYMAT
// forms as a definition rather than with an RFC 2119 keyword, and no id in
// `rfc/short/rfc7296.md` covers the PFS form: `RFC7296-2.17-1` and `-2` are the ordering
// and the split of the expanded KEYMAT. The extraction that would let these be tagged is
// owed and is named in the report that accompanied this change.
package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// pfsESPGroup returns an esp-group with PFS set explicitly, so a reader of the test does
// not have to know that ipsec.PFSEnable is the zero value of PFSMode.
func pfsESPGroup(mode ipsec.PFSMode) ipsec.ESPGroup {
	g := negESPGroupPair()
	g.PFS = mode
	return g
}

// pfsRekeyRequest sends one Child SA rekey and returns the pending state it recorded,
// the peer that can decrypt the request, and the request bytes.
func pfsRekeyRequest(t *testing.T, mode ipsec.PFSMode) (*SA, *SA, *ChildSA, *pendingRekey, []byte) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	group := pfsESPGroup(mode)
	ini.ESPGroup = group
	dp := &mockDP{}
	child, err := createFirstChildSA(ini, group, "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	reqBytes, pending, err := initiateChildRekey(ini, child)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	return ini, resp, child, pending, reqBytes
}

// VALIDATES: a Child SA rekey that carries a Diffie-Hellman value also offers that exact
// group in its SA payload. RFC7296-1.3-1 was proven only for the IKE SA rekey
// (TestRkyIKERekeyOffersTheKEiGroup); enabling PFS adds a SECOND site that sends a KEi,
// and this covers that site.
// RFC requirement: RFC7296-1.3-1 positive -- childRekeyDHGroup (rekey.go) takes the KEi group
// from the IKE SA's negotiated proposal when the esp-group enables PFS, and
// buildChildSAPayloads offers that same group as a Transform Type 4, so the offer set
// names the group of the KEi the request carries.
// RFC requirement: RFC7296-1.3-1 negative -- TestChildRekeyProposesNoDiffieHellmanWhenPFSIsDisabled
// is the discriminator: with pfs disable the request carries no KEi, so the obligation does
// not bind and no offer names a group. A build that always sent a KEi would fail it.
func TestChildRekeyProposesDiffieHellmanWhenPFSIsEnabled(t *testing.T) {
	sa, peer, _, pending, reqBytes := pfsRekeyRequest(t, ipsec.PFSEnable)
	if pending.dh == nil {
		t.Fatal("pfs enable sent a rekey with no Diffie-Hellman half: the KEi of RFC 7296 Section 1.3.3 is missing")
	}
	if pending.dh.GroupID != sa.Proposal.DHGroup.ID {
		t.Errorf("rekey proposed group %d, want the IKE SA's negotiated group %d",
			uint16(pending.dh.GroupID), uint16(sa.Proposal.DHGroup.ID))
	}
	if len(pending.dh.PublicKey) == 0 {
		t.Error("the Diffie-Hellman half carries no public key, so the peer receives nothing to answer")
	}

	// The requirement is about the WIRE, so read what the peer receives rather than the
	// state this node kept.
	inner, err := decryptAndParse(peer, parseMsg(t, reqBytes), reqBytes)
	if err != nil {
		t.Fatalf("the peer could not decrypt the child rekey request: %v", err)
	}
	kei := rkyFindKE(t, inner)
	offers := rkyFindSA(t, inner)
	if kei.DHGroup != uint16(sa.Proposal.DHGroup.ID) {
		t.Errorf("KEi group = %d, want the IKE SA's group %d", kei.DHGroup, uint16(sa.Proposal.DHGroup.ID))
	}
	if !rkyOffersGroup(offers, kei.DHGroup) {
		t.Errorf("no SA offer names the KEi group %d, which RFC 7296 Section 1.3 requires", kei.DHGroup)
	}
}

// TestChildRekeyProposesNoDiffieHellmanWhenPFSIsDisabled is the discriminator for the
// test above: without it, a build that always proposed a group would pass both.
// RFC requirement: RFC7296-1.3-1 negative -- with pfs disable the CREATE_CHILD_SA carries no
// KEi, so the obligation to offer its group does not bind. This is what makes the positive
// half a real search rather than a property of every request.
func TestChildRekeyProposesNoDiffieHellmanWhenPFSIsDisabled(t *testing.T) {
	_, _, _, pending, _ := pfsRekeyRequest(t, ipsec.PFSDisable)
	if pending.dh != nil {
		t.Fatalf("pfs disable sent a rekey proposing Diffie-Hellman group %d, want none",
			uint16(pending.dh.GroupID))
	}
}

// TestChildRekeyRefusesAPFSResponseThatCarriesNoKE proves the refusal rather than a
// silent downgrade. Keying from SK_d alone here would give an operator who asked for pfs
// enable the non-PFS form of Section 2.17, and nothing on the wire would say so.
func TestChildRekeyRefusesAPFSResponseThatCarriesNoKE(t *testing.T) {
	ini, _, child, pending, _ := pfsRekeyRequest(t, ipsec.PFSEnable)
	group := pfsESPGroup(ipsec.PFSEnable)
	props := buildWireESPProposals(group, 0x11223344, dhGroupNone)
	answer := append([]wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: props[1:2]}},
		{Payload: &wire.PayloadNonce{NonceData: negNonce(0x22)}},
	}, childRekeyAnswerTS(t, child)...)

	_, err := applyChildRekeyResponse(ini, pending, answer, &mockDP{}, slogutil.DiscardLogger())
	if err == nil {
		t.Fatal("a pfs rekey answered with no KE payload was accepted, and the Child SA was keyed without the exchange")
	}
	if !strings.Contains(err.Error(), "no KE payload") {
		t.Errorf("error = %q, want it to name the missing KE payload", err)
	}
}

// TestChildRekeyRefusesAKEItDidNotInvite is the opposite direction: a response carrying a
// KE for a request that proposed no group.
func TestChildRekeyRefusesAKEItDidNotInvite(t *testing.T) {
	ini, _, child, pending, _ := pfsRekeyRequest(t, ipsec.PFSDisable)
	group := pfsESPGroup(ipsec.PFSDisable)
	props := buildWireESPProposals(group, 0x11223344, dhGroupNone)
	peer, err := crypto.NewDHExchange(ini.Proposal.DHGroup.ID)
	if err != nil {
		t.Fatalf("NewDHExchange: %v", err)
	}
	answer := append([]wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: props[1:2]}},
		{Payload: &wire.PayloadNonce{NonceData: negNonce(0x22)}},
		{Payload: &wire.PayloadKE{DHGroup: uint16(ini.Proposal.DHGroup.ID), KeyExchangeData: peer.PublicKey}},
	}, childRekeyAnswerTS(t, child)...)

	if _, err := applyChildRekeyResponse(ini, pending, answer, &mockDP{}, slogutil.DiscardLogger()); err == nil {
		t.Fatal("a non-pfs rekey answered with a KE payload was accepted, so the two ends disagree about the keying")
	}
}

// TestChildRekeyRefusesAKEInAnotherGroup covers the third disagreement: the response
// answers a group the request never proposed, which leaves no private value to meet it.
func TestChildRekeyRefusesAKEInAnotherGroup(t *testing.T) {
	ini, _, child, pending, _ := pfsRekeyRequest(t, ipsec.PFSEnable)
	group := pfsESPGroup(ipsec.PFSEnable)
	other := crypto.DH_ECP_256
	if ini.Proposal.DHGroup.ID == other {
		other = crypto.DH_MODP_2048
	}
	props := buildWireESPProposals(group, 0x11223344, other)
	peer, err := crypto.NewDHExchange(other)
	if err != nil {
		t.Fatalf("NewDHExchange: %v", err)
	}
	answer := append([]wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: props[1:2]}},
		{Payload: &wire.PayloadNonce{NonceData: negNonce(0x22)}},
		{Payload: &wire.PayloadKE{DHGroup: uint16(other), KeyExchangeData: peer.PublicKey}},
	}, childRekeyAnswerTS(t, child)...)

	if _, err := applyChildRekeyResponse(ini, pending, answer, &mockDP{}, slogutil.DiscardLogger()); err == nil {
		t.Fatalf("a rekey that proposed group %d accepted an answer in group %d",
			uint16(ini.Proposal.DHGroup.ID), uint16(other))
	}
}

// TestPFSSharedSecretChangesTheChildKeys is the test that proves PFS DOES something. The
// four above prove a payload is sent and a disagreement is refused; none of them would
// fail if the shared secret were computed and then discarded. This one holds Ni, Nr and
// both transforms fixed and varies only the exchange, so a difference in the derived key
// can come from nothing else.
func TestPFSSharedSecretChangesTheChildKeys(t *testing.T) {
	sa, _, _, _, _ := pfsRekeyRequest(t, ipsec.PFSEnable)
	group := pfsESPGroup(ipsec.PFSEnable)
	enc, integ := espTransforms(group.Proposals[0])

	ni := negNonce(0x11)
	nr := negNonce(0x22)

	plain, err := childRekeyKeys(sa, nil, nil, nil, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("childRekeyKeys without an exchange: %v", err)
	}

	local, err := crypto.NewDHExchange(sa.Proposal.DHGroup.ID)
	if err != nil {
		t.Fatalf("NewDHExchange: %v", err)
	}
	peer, err := crypto.NewDHExchange(sa.Proposal.DHGroup.ID)
	if err != nil {
		t.Fatalf("NewDHExchange: %v", err)
	}
	props := buildWireESPProposals(group, 0x11223344, sa.Proposal.DHGroup.ID)
	accepted := &wire.PayloadSA{Proposals: props[0:1]}
	ke := &wire.PayloadKE{DHGroup: uint16(sa.Proposal.DHGroup.ID), KeyExchangeData: peer.PublicKey}

	withPFS, err := childRekeyKeys(sa, local, accepted, ke, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("childRekeyKeys with an exchange: %v", err)
	}

	if bytes.Equal(plain.EncryptKeyI, withPFS.EncryptKeyI) {
		t.Fatal("the PFS derivation produced the same encryption key as the non-PFS one," +
			" so the shared secret never reached KEYMAT (RFC 7296 Section 2.17)")
	}
}
