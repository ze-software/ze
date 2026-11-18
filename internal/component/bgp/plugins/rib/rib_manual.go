// Design: docs/architecture/plugin/rib-storage-design.md -- synthetic forwarding origins.
// Related: rib_commands.go -- explicit inject/withdraw command boundaries.
package rib

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// appendManualCandidatesLocked reads only the existing BGP protocol's synthetic
// input population. These candidates can supply forwarding, never a received
// Peer owner or an ordinary source receipt. The caller MUST hold peerMu and
// later releaseCandidates for every returned snapshot.
func (r *RIBManager) appendManualCandidatesLocked(candidates []*Candidate, fam family.Family, prefix netip.Prefix, selfNextHops *[]netip.Addr) []*Candidate {
	var inline [4]storage.PrefixPath
	for address, routes := range r.ribInPool[bgpProtocolID] {
		peer := routes.PeerAddress()
		paths, addPath := routes.AppendPrefixPathsRetained(fam, prefix, inline[:0])
		for i := range paths {
			path := &paths[i]
			if !r.candidateAdmitted(fam, routes, path.Entry, selfNextHops) {
				path.Release()
				continue
			}
			candidate := r.extractCandidate(fam, 0, peer, address, path.Entry)
			candidate.origin = candidateOriginManual
			candidate.PathID, candidate.AddPath = path.PathID, addPath
			candidate.entry, candidate.labelHandle = path.Entry, path.Labels
			if !r.candidateNextHopResolved(fam, prefix, candidate) {
				path.Release()
				continue
			}
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// commitReceivedAdvertise commits the learned scope independently of the
// forwarding winner. Its caller MUST hold that native key's selection shard.
// A nil best is a known absence; a nil prepared route with best present is an
// unchanged selection or an already reported capture failure.
func (r *RIBManager) commitReceivedAdvertise(fam family.Family, prefix netip.Prefix, native []byte, best *Candidate, route *advertiseRoute) {
	if best == nil {
		r.removeAdvertise(fam, prefix, native)
		return
	}
	r.commitAdvertise(route)
}
