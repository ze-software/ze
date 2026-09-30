// VALIDATES: the sender half of RFC 7296 Section 3.3.5 for the transforms whose Key Length
// attribute is always required: every IKE and ESP offer ze builds for an AES cipher carries
// that attribute, with the configured key size.
// PREVENTS: an AES-CBC, AES-GCM or AES-CCM offer leaving ze without its Key Length
// attribute, which a conforming responder rejects.
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// keyLengthSenderCases is every AES cipher ze can be configured with, and its key size in
// bits. Each maps to a Transform ID whose Key Length attribute RFC 7296 Section 3.3.5
// (AES-CBC) or RFC 5282 Section 7.3 (AES GCM and CCM) makes mandatory.
var keyLengthSenderCases = []struct {
	algo ipsec.EncryptionAlgo
	bits uint16
}{
	{ipsec.EncryptionAES128, 128},
	{ipsec.EncryptionAES256, 256},
	{ipsec.EncryptionAES128GCM, 128},
	{ipsec.EncryptionAES256GCM, 256},
	{ipsec.EncryptionAES128CCM8, 128},
	{ipsec.EncryptionAES256CCM8, 256},
	{ipsec.EncryptionAES128CCM12, 128},
	{ipsec.EncryptionAES256CCM12, 256},
	{ipsec.EncryptionAES128CCM16, 128},
	{ipsec.EncryptionAES256CCM16, 256},
}

// keyLengthAttrs answers the Key Length attribute values of the ENCR transform in one wire
// proposal, and whether the proposal carries an ENCR transform at all.
func keyLengthAttrs(p wire.Proposal) ([]uint16, bool) {
	for _, tr := range p.Transforms {
		if tr.Type != wire.TransformTypeENCR {
			continue
		}
		var values []uint16
		for _, a := range tr.Attrs {
			if a.Type == wire.AttrTypeKeyLength {
				values = append(values, a.Value)
			}
		}
		return values, true
	}
	return nil, false
}

// RFC requirement: RFC7296-3.3.5-5 positive -- the sender clause: for every AES cipher ze
// can be configured with, the IKE offer built by buildWireIKEProposals and the ESP offer
// built by espProposalToWire each carry an ENCR transform with exactly one Key Length
// attribute, whose value is the configured key size (128 or 256). The receiver clause,
// rejection of an offer without it, is TestPropKeyLengthRequiredTransformRejectedWithoutIt
// in the crypto package.
//
// RFC 7296 Section 3.3.5: "Some transforms specify that the Key Length attribute MUST be
// always included (omitting the attribute is not allowed, and proposals not containing it
// MUST be rejected). For example, this includes ENCR_AES_CBC and ENCR_AES_CTR." Ze
// builds no AES-CTR offer, so AES-CBC, GCM and CCM are the ciphers under test.
func TestRFC7296OffersAlwaysCarryTheRequiredKeyLength(t *testing.T) {
	for _, tc := range keyLengthSenderCases {
		group := testIKEGroup()
		group.Proposals[0].Encryption = tc.algo
		ike := buildWireIKEProposals(group)
		if len(ike) != 1 {
			t.Fatalf("%s: buildWireIKEProposals built %d proposals, want 1", tc.algo, len(ike))
		}
		esp := espProposalToWire(ipsec.ESPProposal{
			Number:     1,
			Encryption: tc.algo,
			Hash:       ipsec.HashSHA256,
		}, 0x01020304, 1, dhGroupNone)

		for name, p := range map[string]wire.Proposal{"IKE": ike[0], "ESP": esp} {
			values, ok := keyLengthAttrs(p)
			if !ok {
				t.Fatalf("%s %s offer carries no ENCR transform", tc.algo, name)
			}
			if len(values) != 1 {
				t.Errorf("%s %s offer's ENCR transform carries %d Key Length attributes, "+
					"want exactly 1", tc.algo, name, len(values))
				continue
			}
			if values[0] != tc.bits {
				t.Errorf("%s %s offer's Key Length = %d, want the configured %d",
					tc.algo, name, values[0], tc.bits)
			}
		}
	}
}
