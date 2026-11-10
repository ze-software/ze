// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- per-peer SA options the backend must honor
// Related: unmatched.go -- verifyUnmatchedEnforceable, the same commit-time question for the catch-all
// Related: child.go -- isXFRMUnsupported, which never reads an option refusal as a missing dataplane

package engine

import (
	"maps"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/opererr"
)

// peerSAOptions is what one site-to-site peer asks every one of its Child SAs to carry.
func peerSAOptions(peer *ipsec.SiteToSitePeer) dataplane.SAOptions {
	return dataplane.SAOptions{
		DF:             peer.DFBit,
		OuterDSCP:      peer.OuterDSCP,
		ICMPErrorCheck: !peer.SkipICMPErrorCheck,
	}
}

// verifyPeerSAOptions is the config-verify half of the per-SA options: it refuses a
// peer whose df-bit, dscp-map or icmp-error-check the loaded dataplane cannot apply,
// so the operator's commit fails rather than the install after IKE has negotiated
// the Child SA. The install path refuses the same options with
// dataplane.ErrSAOptionRefused, from the backend's same predicate, for a
// configuration the daemon started with, which no verify judged.
//
// RFC 4301 Section 8.1, Section 5.1.2.1 and Section 6.2 each make an option
// configurable, and a configured value the backend cannot write would be a silent
// different one, so it is refused.
//
// A backend that does not implement dataplane.SAOptionChecker has declared nothing,
// and every option other than the leaves' defaults is refused on it. dpName is the
// name dp was loaded under (dataplane.GetNamed), which the refusal names.
//
// The refusal is an opererr.Error, because the config verifier hands it to the
// operator unchanged: it names the peer, why the commit cannot stand, and the edit
// that fixes it. A backend refusal stays its cause, so errors.Is still finds
// dataplane.ErrSAOptionRefused and the backend's own words reach the operator.
func verifyPeerSAOptions(dp dataplane.Dataplane, dpName string, cfg *ipsec.IPsecConfig) error {
	// refuse-by-default: guards installChildTolerant -- with no dataplane loaded every
	// Child SA is kept without ESP and reported so, and there is no backend to judge.
	if dp == nil {
		return nil
	}
	checker, declares := dp.(dataplane.SAOptionChecker)
	for _, name := range slices.Sorted(maps.Keys(cfg.Peers)) {
		peer := cfg.Peers[name]
		options := peerSAOptions(&peer)
		if declares {
			if err := checker.SAOptionsSupported(options); err != nil {
				return opererr.Wrap(err,
					"ipsec peer "+strconv.Quote(name)+": the "+strconv.Quote(dpName)+
						" dataplane backend cannot apply its df-bit, dscp-map or icmp-error-check",
					"an option the backend cannot write would install a different one in silence",
					"set df-bit, dscp-map and icmp-error-check on peer "+strconv.Quote(name)+
						" to values the backend applies, as the refusal above names")
			}
			continue
		}
		if options != (dataplane.SAOptions{ICMPErrorCheck: true}) {
			return opererr.New(
				"ipsec peer "+strconv.Quote(name)+": the "+strconv.Quote(dpName)+
					" dataplane backend does not say whether it can apply df-bit, dscp-map or icmp-error-check",
				"an option the backend cannot write would install a different one in silence, so only the leaves' defaults run on it",
				"remove df-bit, dscp-map and icmp-error-check from peer "+strconv.Quote(name)+" to run it with their defaults")
		}
	}
	return nil
}
