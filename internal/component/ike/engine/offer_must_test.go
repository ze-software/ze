// Design: docs/architecture/ike/ipsec-14-responder.md -- test wrappers over the offer builders
// Related: initiator.go -- wireIKEOffer, localIKEProposals, wireESPOffer, espProposalWire
// Related: responder.go -- matchOfferedESP

// The offer builders return an error for an algorithm the crypto registry does not
// hold (RULINGS R30: a failed lookup is never the zero transform). Every test fixture
// here names a registered algorithm, so these wrappers keep the one-value form the
// tests were written against and turn a lookup error into a panic that fails the test
// with the cause. They are test-only: no production path reaches them. The error path
// itself is proven by TestEngineProposalBuilderNeverOffersAZeroTransform and its
// siblings, which call the production builders directly.

package engine

import (
	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// mustResolve fails the calling test on a lookup error. A panic rather than t.Fatal,
// because the wrappers keep the signatures of the call sites that hold no *testing.T.
func mustResolve(err error) {
	if err != nil {
		panic("BUG: test fixture names an algorithm the crypto registry does not hold: " + err.Error())
	}
}

// buildSAInitRequest is the test-only one-value form of encodeSAInitRequest. It
// exists so the IKE_SA_INIT tests keep the signature they were written against,
// which leaves every RFC-tagged unit body unchanged. The fixtures name registered
// algorithms, so a lookup error is a broken fixture and panics.
func buildSAInitRequest(sa *SA, ikeGroup ipsec.IKEGroup) []byte {
	msg, err := encodeSAInitRequest(sa, ikeGroup)
	mustResolve(err)
	return msg
}

// buildWireIKEProposals is the test-only one-value form of wireIKEOffer. It exists
// so the IKE offer tests keep the signature they were written against, which leaves
// every RFC-tagged unit body unchanged. The fixtures name registered algorithms, so
// a lookup error is a broken fixture and panics.
func buildWireIKEProposals(ikeGroup ipsec.IKEGroup) []wire.Proposal {
	props, err := wireIKEOffer(ikeGroup)
	mustResolve(err)
	return props
}

// buildIKEProposals is the test-only one-value form of localIKEProposals. It exists
// so the IKE negotiation tests keep the signature they were written against, which
// leaves every RFC-tagged unit body unchanged. The fixtures name registered
// algorithms, so a lookup error is a broken fixture and panics.
func buildIKEProposals(ikeGroup ipsec.IKEGroup) []crypto.IKEProposal {
	props, err := localIKEProposals(ikeGroup)
	mustResolve(err)
	return props
}

// buildWireESPProposals is the test-only one-value form of wireESPOffer. It exists
// so the ESP offer tests keep the signature they were written against, which leaves
// every RFC-tagged unit body unchanged. The fixtures name registered algorithms, so
// a lookup error is a broken fixture and panics.
func buildWireESPProposals(espGroup ipsec.ESPGroup, spi uint32, dh crypto.DHGroupID) []wire.Proposal {
	props, err := wireESPOffer(espGroup, spi, dh)
	mustResolve(err)
	return props
}

// buildESPProposals is the test-only one-value form of localESPProposals. It exists
// so the ESP negotiation tests keep the signature they were written against, which
// leaves every RFC-tagged unit body unchanged. The fixtures name registered
// algorithms, so a lookup error is a broken fixture and panics.
func buildESPProposals(espGroup ipsec.ESPGroup) []crypto.ESPProposal {
	props, err := localESPProposals(espGroup)
	mustResolve(err)
	return props
}

// espProposalToWire is the test-only one-value form of espProposalWire. It exists so
// the per-proposal ESP encoding tests keep the signature they were written against,
// which leaves every RFC-tagged unit body unchanged. The fixtures name registered
// algorithms, so a lookup error is a broken fixture and panics.
//
//nolint:unparam // keeps espProposalWire's parameter list, so a test call reads as the production call.
func espProposalToWire(p ipsec.ESPProposal, spi uint32, number uint8, dh crypto.DHGroupID) wire.Proposal {
	prop, err := espProposalWire(p, spi, number, dh)
	mustResolve(err)
	return prop
}

// espTransforms is the test-only two-value form of resolveESPTransforms. It exists so
// the ESP transform tests keep the signature they were written against, which leaves
// every RFC-tagged unit body unchanged. The fixtures name registered algorithms, so a
// lookup error is a broken fixture and panics.
func espTransforms(p ipsec.ESPProposal) (crypto.EncryptionTransform, crypto.IntegrityTransform) {
	enc, integ, err := resolveESPTransforms(p)
	mustResolve(err)
	return enc, integ
}

// matchOfferedESPProposal is the test-only two-value form of matchOfferedESP. It
// exists so the responder ESP selection tests keep the signature they were written
// against, which leaves every RFC-tagged unit body unchanged. The fixtures name
// registered algorithms, so a lookup error is a broken fixture and panics.
func matchOfferedESPProposal(offer *wire.PayloadSA, our ipsec.ESPProposal, dh espDHMatch) (wire.Proposal, bool) {
	rp, ok, err := matchOfferedESP(offer, our, dh)
	mustResolve(err)
	return rp, ok
}
