// VALIDATES: an engine proposal builder never puts Transform ID 0 on the wire for a
// configured algorithm the crypto registry does not hold. RFC 7296 Section 3.3.2
// reserves ENCR 0, and a zero transform reads as a valid answer to every caller.
// PREVENTS: RULINGS R29 (second defect), fixed under R30. lookupEncryption, lookupPRF
// and lookupIntegrity (initiator.go) answered a failed registry lookup with the zero
// transform, so the IKE offer carried ENCR 0 and PRF 0 with no error.
//
// Method: each production builder is called with a group naming 3des, which the
// ipsec enum holds and the crypto registry does not, and must return an error wrapping
// crypto.ErrUnsupportedAlgorithm and no proposal. Config parse already refuses such a
// name (ipsec.EncryptionImplemented, TestParseRejectsUnimplementedEncryption), so the
// defect is reached only by a group that did not come through the parser; the engine is
// the second check the style guide's "pair the check" asks for.

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

func unresolvableIKEGroup() ipsec.IKEGroup {
	return ipsec.IKEGroup{Proposals: []ipsec.IKEProposal{{
		Number:     1,
		Encryption: ipsec.Encryption3DES,
		Hash:       ipsec.HashSHA256,
		DHGroup:    14,
	}}}
}

func unresolvableESPGroup() ipsec.ESPGroup {
	return ipsec.ESPGroup{Proposals: []ipsec.ESPProposal{{
		Number:     1,
		Encryption: ipsec.Encryption3DES,
		Hash:       ipsec.HashSHA256,
	}}}
}

func requireUnsupported(t *testing.T, builder string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s accepted %s, which the crypto registry does not hold", builder, ipsec.Encryption3DES)
	}
	if !errors.Is(err, crypto.ErrUnsupportedAlgorithm) {
		t.Fatalf("%s: error %v does not wrap crypto.ErrUnsupportedAlgorithm", builder, err)
	}
}

func TestEngineProposalBuilderNeverOffersAZeroTransform(t *testing.T) {
	props, err := wireIKEOffer(unresolvableIKEGroup())
	requireUnsupported(t, "wireIKEOffer", err)
	if props != nil {
		t.Fatalf("wireIKEOffer returned %d proposals beside its error", len(props))
	}

	local, err := localIKEProposals(unresolvableIKEGroup())
	requireUnsupported(t, "localIKEProposals", err)
	if local != nil {
		t.Fatalf("localIKEProposals returned %d proposals beside its error", len(local))
	}

	msg, err := encodeSAInitRequest(&SA{}, unresolvableIKEGroup())
	requireUnsupported(t, "encodeSAInitRequest", err)
	if msg != nil {
		t.Fatalf("encodeSAInitRequest returned %d octets beside its error", len(msg))
	}
}

func TestEngineESPBuilderNeverOffersAZeroTransform(t *testing.T) {
	group := unresolvableESPGroup()

	offer, err := wireESPOffer(group, 0x11223344, dhGroupNone)
	requireUnsupported(t, "wireESPOffer", err)
	if offer != nil {
		t.Fatalf("wireESPOffer returned %d proposals beside its error", len(offer))
	}

	_, err = espProposalWire(group.Proposals[0], 0x11223344, 1, dhGroupNone)
	requireUnsupported(t, "espProposalWire", err)

	_, err = localESPProposals(group)
	requireUnsupported(t, "localESPProposals", err)

	_, _, err = resolveESPTransforms(group.Proposals[0])
	requireUnsupported(t, "resolveESPTransforms", err)

	// A peer offering ENCR 0 must not match a configured proposal that does not
	// resolve: before the fix the zero transform matched exactly that offer.
	zeroOffer := &wire.PayloadSA{Proposals: []wire.Proposal{{
		Number: 1, ProtocolID: wire.ProtocolESP, SPISize: 4, SPI: []byte{1, 2, 3, 4},
		Transforms: []wire.Transform{
			{Type: wire.TransformTypeENCR, ID: 0},
			{Type: wire.TransformTypeINTG, ID: 12},
			{Type: wire.TransformTypeESN, ID: espESNNotExtended},
		},
	}}}
	_, ok, err := matchOfferedESP(zeroOffer, group.Proposals[0], espDHMatch{Unbound: true})
	requireUnsupported(t, "matchOfferedESP", err)
	if ok {
		t.Fatal("matchOfferedESP matched an ENCR 0 offer to an unresolvable proposal")
	}
}

// TestEngineResolvableGroupsStillBuild is the control: the same builders accept a
// group whose every algorithm is registered, so the refusals above are about 3des and
// not about the builders refusing everything.
func TestEngineResolvableGroupsStillBuild(t *testing.T) {
	ike := unresolvableIKEGroup()
	ike.Proposals[0].Encryption = ipsec.EncryptionAES256
	props, err := wireIKEOffer(ike)
	if err != nil {
		t.Fatalf("wireIKEOffer refused a registered suite: %v", err)
	}
	for _, p := range props {
		for _, tr := range p.Transforms {
			if tr.Type == wire.TransformTypeENCR && tr.ID == 0 {
				t.Fatalf("proposal %d offers ENCR 0", p.Number)
			}
		}
	}

	esp := unresolvableESPGroup()
	esp.Proposals[0].Encryption = ipsec.EncryptionAES256
	if _, err := wireESPOffer(esp, 0x11223344, dhGroupNone); err != nil {
		t.Fatalf("wireESPOffer refused a registered suite: %v", err)
	}
}
