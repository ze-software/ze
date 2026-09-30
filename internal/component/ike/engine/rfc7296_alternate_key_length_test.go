// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the SA offer ze builds
// Related: initiator.go -- buildWireIKEProposals, buildWireESPProposals

package engine

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// altkeyEncryptions offers one cipher family at both key sizes, CBC and GCM.
var altkeyEncryptions = []ipsec.EncryptionAlgo{
	ipsec.EncryptionAES128, ipsec.EncryptionAES256,
	ipsec.EncryptionAES128GCM, ipsec.EncryptionAES256GCM,
}

// altkeyOfferKeyLengths encodes an offer into an SA payload, decodes it off the wire, and
// returns every Key Length value its ENCR transforms carry, failing when one ENCR
// transform carries other than exactly one Key Length attribute.
func altkeyOfferKeyLengths(t *testing.T, where string, offer []wire.Proposal) []uint16 {
	t.Helper()
	sent := &wire.PayloadSA{Proposals: offer}
	buf := make([]byte, sent.Len())
	sent.WriteTo(buf, 0)
	var got wire.PayloadSA
	if err := got.ReadFrom(buf); err != nil {
		t.Fatalf("%s: the offer does not parse: %v", where, err)
	}
	var lengths []uint16
	for _, p := range got.Proposals {
		for _, tr := range p.Transforms {
			if tr.Type != wire.TransformTypeENCR {
				continue
			}
			count := 0
			for _, a := range tr.Attrs {
				if a.Type == wire.AttrTypeKeyLength {
					count++
					lengths = append(lengths, a.Value)
				}
			}
			if count != 1 {
				t.Errorf("%s: proposal %d ENCR transform %d carries %d Key Length attributes, want 1",
					where, p.Number, tr.ID, count)
			}
		}
	}
	return lengths
}

// VALIDATES: RFC 7296 Section 3.3: alternate values of an attribute are proposed as
// separate transforms of one Transform Type, each with a single attribute.
// PREVENTS: an offer that packs alternate key lengths into one transform.
//
// METHOD: an IKE and an ESP group configured with AES-CBC and AES-GCM at 128 and 256 bits
// are turned into the offers ze sends (buildWireIKEProposals, buildWireESPProposals),
// encoded and decoded off the wire. Both key lengths must be offered, and every ENCR
// transform must carry exactly one Key Length attribute. The receive-side refusal of a
// transform with two Key Length attributes is the wire package's
// TestPropAlternateKeyLengthsUseSeparateTransforms.
//
// RFC requirement: RFC7296-3.3-7 positive -- the IKE and the ESP offers ze sends for a group
// with AES-CBC and AES-GCM at 128 and 256 bits carry both key lengths, 128 and 256, each in
// its own ENCR transform holding exactly one Key Length attribute.
func TestRFC7296AlternateKeyLengthsAreOfferedAsSeparateTransforms(t *testing.T) {
	var ike ipsec.IKEGroup
	var esp ipsec.ESPGroup
	for i, enc := range altkeyEncryptions {
		ike.Proposals = append(ike.Proposals, ipsec.IKEProposal{
			Number: uint16(i + 1), Encryption: enc, Hash: ipsec.HashSHA256, DHGroup: 14,
		})
		esp.Proposals = append(esp.Proposals, ipsec.ESPProposal{
			Number: uint16(i + 1), Encryption: enc, Hash: ipsec.HashSHA256,
		})
	}
	offers := map[string][]wire.Proposal{
		"IKE offer": buildWireIKEProposals(ike),
		"ESP offer": buildWireESPProposals(esp, 0x01020304, 0),
	}
	for where, offer := range offers {
		lengths := altkeyOfferKeyLengths(t, where, offer)
		if len(lengths) != len(altkeyEncryptions) {
			t.Errorf("%s: %d Key Length attributes across the offer, want %d (one per ENCR transform)",
				where, len(lengths), len(altkeyEncryptions))
		}
		for _, want := range []uint16{128, 256} {
			if !slices.Contains(lengths, want) {
				t.Errorf("%s: key length %d is not offered (offered %v)", where, want, lengths)
			}
		}
	}
}
