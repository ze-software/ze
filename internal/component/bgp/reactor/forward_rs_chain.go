// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// Related: forward_rs.go -- reactorForwardRSSection, the rail these helpers serve
// Related: peer_forward_facts.go -- exportChainActive, the fact that selects the chain run

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rsBase is the UPDATE the route-server fast path edits for one destination,
// with the per-UPDATE reads the egress edits take from it. It is the received
// wire unless the destination's export chain rewrote it, and then
// every read is taken again over the rewrite, as the general rail does over
// its peerBaseWire (forwardUpdateCore).
type rsBase struct {
	wire         *wireu.WireUpdate
	nextHop      nextHopValue
	hasLocalPref bool
	hasPrefixSID bool
	med          medValue
}

// rsBaseOf answers the received wire with the reads the rail already took
// once for the whole fan-out.
func rsBaseOf(wire *wireu.WireUpdate, nextHop nextHopValue, hasLocalPref, hasPrefixSID bool, med medValue) rsBase {
	return rsBase{wire: wire, nextHop: nextHop, hasLocalPref: hasLocalPref, hasPrefixSID: hasPrefixSID, med: med}
}

// rsBaseOverride answers an export chain's rewrite of the received UPDATE,
// read again. A rewrite of the same received route keeps the RIB's verdict
// on its redirect next hop, as on the general rail.
func rsBaseOverride(override *wireu.WireUpdate, received nextHopValue) rsBase {
	payload := override.Payload()
	nextHop := payloadNextHop(payload)
	nextHop.mpRedirectRefused = nextHop.mpRedirect && received.mpRedirectRefused
	return rsBase{
		wire:         override,
		nextHop:      nextHop,
		hasLocalPref: payloadHasLocalPref(payload),
		hasPrefixSID: payloadHasAttr(payload, attribute.AttrPrefixSID),
		med:          payloadMED(payload),
	}
}

// runsExportChain reports whether any selected destination's export chain
// runs (exportChainActive), so the source facts the chain's session fields
// read are resolved once for the fan-out.
func runsExportChain(peers []*Peer) bool {
	for _, peer := range peers {
		if facts := peer.forwardFacts(); facts != nil && facts.exportChainActive {
			return true
		}
	}
	return false
}
