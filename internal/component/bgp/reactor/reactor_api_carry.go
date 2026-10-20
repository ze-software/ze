// Design: docs/architecture/api/update-syntax.md -- which peers a route is for
// Related: session_negotiate.go -- offeredMultiprotocol, the families an OPEN offers
// Related: reactor_api_batch.go -- AnnounceNLRIBatch, which sends what this admits

package reactor

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// SelectedPeersCarry answers nil when at least one peer the selector matches is
// configured for the <afi, safi> family, route.ErrNoPeersMatch when it matches none, and
// route.ErrNoPeerConfiguredFamily when every match is configured without it.
//
// A peer is configured for a family when ze's OPEN to it offers the family
// (offeredMultiprotocol). A route for a family no selected peer is offered can
// reach no session, whatever state the sessions are in: a session that is down
// would otherwise queue it and answer it accepted, and none of them can ever
// negotiate it. Whether a configured family was NEGOTIATED is the announce's
// own question, answered per session when it sends.
func (a *reactorAPIAdapter) SelectedPeersCarry(sel *selector.Selector, afi uint16, safi uint8, sender plugin.Sender) error {
	fam := family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}
	a.r.mu.RLock()
	peers, err := a.getMatchingPeersSel(sel, announceOrigin(sender))
	a.r.mu.RUnlock()
	if err != nil {
		return err
	}
	if len(peers) == 0 {
		return route.ErrNoPeersMatch
	}
	for _, peer := range peers {
		if peerOffersFamily(peer, fam) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", route.ErrNoPeerConfiguredFamily, fam)
}

// peerOffersFamily reports whether ze's OPEN to peer offers fam.
//
// RFC 4760 Section 8: "To have a bi-directional exchange of routing
// information for a particular <AFI, SAFI> between a pair of BGP speakers,
// each such speaker MUST advertise to the other (via the Capability
// Advertisement mechanism) the capability to support that particular <AFI,
// SAFI> route."
func peerOffersFamily(peer *Peer, fam family.Family) bool {
	offered := offeredMultiprotocol(peer.configuredCapabilities(), peer.getPluginFamilies)
	if len(offered) == 0 {
		// An OPEN with no Multiprotocol capability speaks plain BGP-4, whose
		// UPDATE carries IPv4 unicast in its own NLRI field (RFC 4271 Section
		// 4.3); it is the family negotiation assumes when nothing is offered.
		return fam == family.IPv4Unicast
	}
	for _, c := range offered {
		if mp, ok := c.(*capability.Multiprotocol); ok && mp.AFI == fam.AFI && mp.SAFI == fam.SAFI {
			return true
		}
	}
	return false
}
