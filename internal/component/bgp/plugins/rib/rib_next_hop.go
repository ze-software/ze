// Design: docs/architecture/route-selection.md -- next-hop admission before ranking
// Related: rib_commands.go, rib_remote.go -- both candidate gathers and transports
// RFC: rfc/short/rfc4271.md -- Sections 9.1.2 and 9.1.2.1
// RFC: rfc/short/rfc9252.md -- Sections 5 and 6 SRv6 Service SID resolvability before best-path
package rib

import (
	"context"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
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
	isEBGP := r.protocolType(c) == routeaction.ProtocolEBGP
	return r.nextHopResolved(fam, prefix, nextHop, isEBGP, c.MED, c.AIGP, c.HasAIGP)
}

// candidateSRv6SIDResolved evaluates SRv6 Service SID resolvability before
// Phase 2 BGP best-path selection on the reconstructed merged Service SID.
//
// RFC 9252 Section 5: "Therefore, the ingress PE MUST perform a resolvability
// check for the SRv6 Service SID before considering the received prefix for the
// BGP best path computation. The resolvability is evaluated as per [RFC4271]."
// Section 6 repeats this requirement verbatim for EVPN over SRv6. Section 5
// also allows local policy to ignore the resolvability verdict when an
// alternate steering mechanism reaches the egress PE (srv6-sid-resolvability).
//
// Returns (sid, true) when the route carries no valid SRv6 Service SID, when
// local policy ignores SRv6 SID resolvability for this peer/family, or when the
// merged SID resolves in the Loc-RIB. Returns (sid, false) when the merged SID
// is unresolvable and the path MUST be excluded from candidacy.
//
// Caller MUST hold peerMu.RLock (for peerGroupName lookup).
func (r *RIBManager) candidateSRv6SIDResolved(fam family.Family, prefix netip.Prefix, nlri []byte, c *Candidate) (netip.Addr, bool) {
	if !c.entry.GetBundle().HasOtherAttrs() {
		return netip.Addr{}, true
	}
	var sp storedPath
	if prefix.IsValid() {
		sp = storedPath{pfx: prefix, pathID: c.PathID, addPath: c.AddPath, byPrefix: true}
	} else {
		sp = storedPath{nlri: nlri, pathID: c.PathID, addPath: c.AddPath}
	}
	sid := entrySRv6SID(fam, sp, c.entry)
	if !sid.IsValid() {
		return netip.Addr{}, true
	}
	if r.srv6SIDResolvabilityIgnoredLocked(c.storeOwner, c.PeerAddr, fam) {
		return sid, true
	}
	isEBGP := r.protocolType(c) == routeaction.ProtocolEBGP
	if !r.nextHopResolved(fam, prefix, sid, isEBGP, c.MED, c.AIGP, c.HasAIGP) {
		return sid, false
	}
	return sid, true
}

// incSRv6UnresolvableExclusions increments the Prometheus exclusion counter for
// fam when a metrics registry is wired.
func (r *RIBManager) incSRv6UnresolvableExclusions(fam family.Family) {
	m := metricsPtr.Load()
	if m == nil {
		return
	}
	m.srv6SIDUnresolvableExclusions.With(fam.String()).Inc()
}

// nextHopResolved evaluates RFC 4271 Section 9.1.2 next-hop resolvability
// (including prospective self-capture for CIDR routes) using the active
// internal Loc-RIB or forked metric RPC transport.
func (r *RIBManager) nextHopResolved(fam family.Family, prefix netip.Prefix, nextHop netip.Addr, isEBGP bool, med uint32, aigp uint64, hasAIGP bool) bool {
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
		IsEBGP: isEBGP,
		Metric: med, AIGP: aigp, AIGPPresent: hasAIGP,
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

// nextHopResolvedCached evaluates next-hop resolvability from in-memory remote
// caches without issuing engine RPCs. An uncached entry returns false
// (fail-closed) while route-metrics lookups are failing.
func (r *RIBManager) nextHopResolvedCached(fam family.Family, prefix netip.Prefix, nextHop netip.Addr, isEBGP bool, med uint32, aigp uint64, hasAIGP bool) bool {
	if !nextHop.IsValid() {
		return true
	}
	remote := r.forkRIB
	if remote == nil {
		return r.nextHopResolved(fam, prefix, nextHop, isEBGP, med, aigp, hasAIGP)
	}
	if !prefix.IsValid() {
		return remote.igpDistanceCached(nextHop).Resolved
	}
	path := locrib.Path{
		Source: bgpProtocolID, Instance: bgpLocRIBInstance,
		NextHop: nextHop, IsBGP: true,
		IsEBGP: isEBGP,
		Metric: med, AIGP: aigp, AIGPPresent: hasAIGP,
	}
	return remote.prospectiveDistanceCached(fam, prefix, path).Resolved
}

// ownerClassAndGroup reads the eBGP classification and enclosing peer-group
// name for owner in one peerMu.RLock acquisition. Caller MUST NOT hold peerMu.
func (r *RIBManager) ownerClassAndGroup(owner uint64) (bool, string) {
	if owner == 0 {
		return true, ""
	}
	r.peerMu.RLock()
	meta := r.peerMeta[owner]
	isEBGP := true
	groupName := ""
	if meta != nil {
		if meta.LocalASN != 0 {
			isEBGP = meta.PeerASN != meta.LocalASN
		}
		groupName = meta.GroupName
	}
	r.peerMu.RUnlock()
	return isEBGP, groupName
}

// entryNextHopResolvedWithClass evaluates forwarding next-hop admission for one
// retained Adj-RIB-In entry using the same prospective resolver as
// candidateNextHopResolved.
func (r *RIBManager) entryNextHopResolvedWithClass(fam family.Family, prefix netip.Prefix, isEBGP bool, entry storage.RouteEntry) bool {
	return r.entryNextHopResolved(fam, prefix, isEBGP, entry, false)
}

// entryNextHopResolvedCached evaluates forwarding next-hop admission for one
// retained Adj-RIB-In entry from in-memory caches only, without issuing engine
// RPCs.
func (r *RIBManager) entryNextHopResolvedCached(fam family.Family, prefix netip.Prefix, isEBGP bool, entry storage.RouteEntry) bool {
	return r.entryNextHopResolved(fam, prefix, isEBGP, entry, true)
}

func (r *RIBManager) entryNextHopResolved(fam family.Family, prefix netip.Prefix, isEBGP bool, entry storage.RouteEntry, cacheOnly bool) bool {
	nextHop := entryNextHopAddr(fam, entry)
	if !nextHop.IsValid() {
		return true
	}
	var med uint32
	b := entry.GetBundle()
	if b.HasMED() {
		if data, err := pool.MED.Get(b.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				med = v
			}
		}
	}
	aigp, hasAIGP := entryAIGP(entry)
	if cacheOnly {
		return r.nextHopResolvedCached(fam, prefix, nextHop, isEBGP, med, aigp, hasAIGP)
	}
	return r.nextHopResolved(fam, prefix, nextHop, isEBGP, med, aigp, hasAIGP)
}

// entrySRv6SIDResolvedWithClass evaluates forwarding SRv6 Service SID admission
// for one retained Adj-RIB-In entry using the same merged-SID extraction,
// local-policy bypass, and prospective resolver as candidateSRv6SIDResolved.
func (r *RIBManager) entrySRv6SIDResolvedWithClass(fam family.Family, prefix netip.Prefix, nlri []byte, addPath bool, peerAddr, groupName string, isEBGP bool, entry storage.RouteEntry) bool {
	return r.entrySRv6SIDResolved(fam, prefix, nlri, addPath, peerAddr, groupName, isEBGP, entry, false)
}

// entrySRv6SIDResolvedCached evaluates forwarding SRv6 Service SID admission
// for one retained Adj-RIB-In entry from in-memory caches only, without
// issuing engine RPCs.
func (r *RIBManager) entrySRv6SIDResolvedCached(fam family.Family, prefix netip.Prefix, nlri []byte, addPath bool, peerAddr, groupName string, isEBGP bool, entry storage.RouteEntry) bool {
	return r.entrySRv6SIDResolved(fam, prefix, nlri, addPath, peerAddr, groupName, isEBGP, entry, true)
}

func (r *RIBManager) entrySRv6SIDResolved(fam family.Family, prefix netip.Prefix, nlri []byte, addPath bool, peerAddr, groupName string, isEBGP bool, entry storage.RouteEntry, cacheOnly bool) bool {
	b := entry.GetBundle()
	if !b.HasOtherAttrs() {
		return true
	}
	var sp storedPath
	if prefix.IsValid() {
		sp = storedPath{pfx: prefix, addPath: addPath, byPrefix: true}
	} else {
		sp = storedPath{nlri: nlri, addPath: addPath}
	}
	sid := entrySRv6SID(fam, sp, entry)
	if !sid.IsValid() {
		return true
	}
	if r.srv6SIDResolvabilityIgnoredWithGroup(peerAddr, groupName, fam) {
		return true
	}
	var med uint32
	if b.HasMED() {
		if data, err := pool.MED.Get(b.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				med = v
			}
		}
	}
	aigp, hasAIGP := entryAIGP(entry)
	if cacheOnly {
		return r.nextHopResolvedCached(fam, prefix, sid, isEBGP, med, aigp, hasAIGP)
	}
	return r.nextHopResolved(fam, prefix, sid, isEBGP, med, aigp, hasAIGP)
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

// prospectiveDistanceCached reads the last cached prospective distance for
// (fam, prefix, path) without issuing an engine RPC. A cache miss returns the
// zero unresolved Distance (fail-closed) so show commands never stall on a
// failing route-metrics RPC.
func (remote *remoteRIB) prospectiveDistanceCached(fam family.Family, prefix netip.Prefix, path locrib.Path) igpcost.Distance {
	key := prospectiveMetricKey{fam: fam, prefix: prefix, nextHop: path.NextHop,
		metric: path.Metric, aigp: path.AIGP, hasAIGP: path.AIGPPresent, external: path.IsEBGP}
	remote.mu.Lock()
	distance := remote.prospective[key]
	remote.mu.Unlock()
	return distance
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
	// refuse-by-default: guards candidateNextHopResolved -- Resolved=false excludes the candidate; the failed lookup is recorded, reported and not cached.
	if err != nil {
		if remote.noteMetricFailure(true) {
			logger().Error(remoteMetricProblem, "code", codeRemoteMetricFailure, "next-hop", path.NextHop, "prefix", prefix, "error", err)
		}
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
