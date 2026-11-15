// Design: docs/architecture/route-selection.md -- next-hop admission before ranking
// Related: rib_commands.go, rib_remote.go -- both candidate gathers and transports
// RFC: rfc/short/rfc4271.md -- Sections 9.1.2 and 9.1.2.1
package rib

import (
	"context"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// nextHopMetricTimeout bounds one engine response while admission holds peerMu.
// A missing reply must leave the path ineligible rather than stall every election.
const nextHopMetricTimeout = 5 * time.Second

// candidateNextHopResolved excludes paths before the Phase 2 ranking. Families
// without an address next hop do not acquire a forwarding requirement here.
func (r *RIBManager) candidateNextHopResolved(fam family.Family, prefix netip.Prefix, c *Candidate) bool {
	nextHop := entryNextHopAddr(fam, c.entry)
	if !nextHop.IsValid() {
		return true
	}
	// A non-CIDR family does not enter the shared prefix-keyed Loc-RIB, so it
	// cannot change this routing table. Its address next hop still must resolve.
	if !prefix.IsValid() {
		return r.igpDistance(nextHop).Resolved
	}
	// These are the policy/rank fields mirrorToLocRIB offers. Labels and the
	// forwarding handle neither rank paths nor change next-hop resolvability.
	path := locrib.Path{
		Source: bgpProtocolID, Instance: bgpLocRIBInstance,
		NextHop: nextHop, IsBGP: true,
		IsEBGP: r.protocolType(c) == routeaction.ProtocolEBGP,
		Metric: c.MED, AIGP: c.AIGP, AIGPPresent: c.HasAIGP,
	}
	// RFC 4271 Section 9.1.2: "If the NEXT_HOP attribute of a BGP route depicts
	// an address that is not resolvable, or if it would become unresolvable if
	// the route was installed in the routing table, the BGP route MUST be
	// excluded from the Phase 2 decision function."
	if remote := r.forkRIB; remote != nil {
		return remote.prospectiveDistance(fam, prefix, path).Resolved
	}
	return igpcost.ResolveProspective(r.locRIB.Load(), nextHop, fam, prefix, path).Resolved
}

// prospectiveMetricKey includes every varying field of the offer above. A route
// is not interchangeable with another route using the same next hop: either
// destination could capture the other's resolution chain when installed.
type prospectiveMetricKey struct {
	fam      family.Family
	prefix   netip.Prefix
	nextHop  netip.Addr
	metric   uint32
	aigp     uint64
	hasAIGP  bool
	external bool
}

func (remote *remoteRIB) prospectiveDistance(fam family.Family, prefix netip.Prefix, path locrib.Path) igpcost.Distance {
	key := prospectiveMetricKey{fam: fam, prefix: prefix, nextHop: path.NextHop,
		metric: path.Metric, aigp: path.AIGP, hasAIGP: path.AIGPPresent, external: path.IsEBGP}
	remote.mu.Lock()
	defer remote.mu.Unlock()
	if distance, found := remote.prospective[key]; found {
		return distance
	}
	ctx, cancel := context.WithTimeout(context.Background(), nextHopMetricTimeout)
	defer cancel()
	out, err := remote.client.RouteMetricsProspective(ctx, []string{path.NextHop.String()}, []rpc.RouteInstallEntry{{
		Protocol: protocolNameBGP, AFI: uint16(fam.AFI), SAFI: uint8(fam.SAFI), Prefix: prefix.String(),
		Instance: path.Instance, NextHop: path.NextHop.String(), IsBGP: true, IsEBGP: path.IsEBGP,
		Metric: path.Metric, AIGP: path.AIGP, AIGPPresent: path.AIGPPresent,
	}})
	if err != nil {
		remote.noteMetricFailure(true)
		logger().Error(remoteMetricProblem, "code", codeRemoteMetricFailure, "next-hop", path.NextHop, "prefix", prefix, "error", err)
		return igpcost.Distance{}
	}
	remote.noteMetricFailure(false)
	d := out.Distances[0]
	distance := igpcost.Distance{Cost: d.Cost, Resolved: d.Resolved, Recursive: d.Recursive, MissingAIGP: d.MissingAIGP}
	if remote.prospective == nil {
		remote.prospective = make(map[prospectiveMetricKey]igpcost.Distance)
	}
	if len(remote.prospective) >= rpc.RouteMetricsAddressMax {
		clear(remote.prospective)
	}
	remote.prospective[key] = distance
	return distance
}
