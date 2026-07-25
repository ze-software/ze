// Design: plan/learned/1051-ospf-ext-6-ti-lfa.md -- OSPF LFA / TI-LFA fast reroute.
// RFC 5286 base LFA selection: per-neighbor SPFs give D_opt(N,*), and each
// primary next-hop is protected by the loop-free alternate that best satisfies
// the Section 3.1 / 3.2 / 1.1 inequalities under the Section 3.6 preference
// order. TI-LFA (tilfa.go) is the SR-repair fallback when no directly-connected
// LFA exists. The whole pass reads the delivered SPF result and the ext-5 SR
// label maps; it never touches the LSDB, flooding or codec.
// RFC: rfc/short/rfc5286.md (Section 3); rfc/short/rfc8665.md (Section 5/6.1 repair labels)

package spf

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// Local aliases for the RFC 2328 Router-LSA link types used by the LFA pass.
const (
	rlP2P     = packet.RouterLinkTypeP2P
	rlVirtual = packet.RouterLinkTypeVirtual
	rlTransit = packet.RouterLinkTypeTransit
	rlStub    = packet.RouterLinkTypeStub
)

// maxMPLSLabel is the largest 20-bit MPLS label (2^20-1). A repair segment
// resolved past this is invalid and never pushed (RFC 8665 Section 5).
const maxMPLSLabel uint32 = 0x000f_ffff

// maxRepairDepth caps a TI-LFA repair label stack to the FIB MPLS stack limit.
const maxRepairDepth = 3

// FastRerouteMode selects base LFA only or LFA with a TI-LFA SR-repair fallback.
type FastRerouteMode uint8

const (
	// FastRerouteLFA installs only directly-connected loop-free alternates.
	FastRerouteLFA FastRerouteMode = iota
	// FastRerouteTILFA falls back to a TI-LFA SR repair list where no base LFA exists.
	FastRerouteTILFA
)

// FastRerouteConfig is the resolved `fast-reroute` container threaded into the
// Computer. Enabled false leaves the route set, install and snapshot byte-for-byte
// as a router without fast-reroute.
type FastRerouteConfig struct {
	Enabled bool
	Mode    FastRerouteMode
	// NodeProtection makes the Section 3.6 selection prefer node-protecting
	// alternates (rule 1). When false the node-protection classification is still
	// recorded for display but does not influence which alternate is chosen.
	NodeProtection bool
	// PreferPrimary makes another primary next-hop the preferred alternate
	// (RFC 5286 Section 3.6 rule 4), preserving ECMP patterns.
	PreferPrimary bool
}

// SRResolver resolves the ext-5 Segment Routing labels a TI-LFA repair list is
// built from (read-only). The engine implements it for IPv4 by reading the
// ext-5 Prefix-SID / Adj-SID maps and the advertised SRGB; it is nil for IPv6
// (RFC 8666 SR carriage is out of scope) so the v6 engine gets base-LFA
// selection only.
type SRResolver interface {
	// PrefixSIDLabel returns the resolved 20-bit MPLS label toward router's node
	// prefix, mapped through router's advertised SRGB (RFC 8665 Section 5). ok is
	// false when router is not SR-capable or advertises no usable node Prefix-SID.
	PrefixSIDLabel(router types.RouterID) (uint32, bool)
	// AdjSIDLabel returns the Adj-SID label for the adjacency from -> to, used for
	// a TI-LFA Q-segment that must cross a specific link (RFC 8665 Section 6.1).
	AdjSIDLabel(from, to types.RouterID) (uint32, bool)
}

// areaFRRStats accumulates the fast-reroute outcome for one area's routes, for
// the ze_ospf_fast_reroute_* metric series.
type areaFRRStats struct {
	protected    map[string]int // protection class -> primary count
	unprotected  map[string]int // reason (no-lfa/no-sr/suppressed) -> primary count
	installed    map[string]int // kind (lfa/ti-lfa) -> backup count
	repairLabels int            // total TI-LFA repair labels pushed
}

func newAreaFRRStats() areaFRRStats {
	return areaFRRStats{
		protected:   map[string]int{},
		unprotected: map[string]int{},
		installed:   map[string]int{},
	}
}

// fastRerouteInput carries the per-run context the LFA/TI-LFA pass needs.
type fastRerouteInput struct {
	root         types.RouterID
	maxPaths     int
	nh           NextHopSource
	resolver     InterfaceResolver
	results      map[types.AreaID]*Result
	graphs       map[types.AreaID]*Graph
	border       []BorderRouterEntry
	virtualLinks bool
	cfg          FastRerouteConfig
	sr           SRResolver // may be nil (base LFA only)
}

// candLink is one directly-connected neighbor link of the computing router S
// (a candidate alternate H_h in RFC 5286 Section 3.6 terms).
type candLink struct {
	neighbor    types.RouterID
	addr        netip.Addr
	iface       string
	forwardCost uint64            // cost S -> N on this link
	reverseCost uint64            // cost N -> S (RFC 5286 Section 3.5 gate)
	broadcast   bool              // primary crosses a broadcast/NBMA pseudo-node
	network     types.LinkStateID // pseudo-node network id when broadcast
}

// candResult is a scored loop-free candidate during Section 3.6 selection.
type candResult struct {
	addr        netip.Addr
	iface       string
	linkProtect bool
	nodeProtect bool
	downstream  bool
	isPrimary   bool
	dist        uint64 // D_opt(N,D), the alternate distance to the destination
}

// publishFRRMetrics updates the ze_ospf_fast_reroute_* series from one run's
// per-area outcome.
func (c *Computer) publishFRRMetrics(stats map[types.AreaID]areaFRRStats, dur time.Duration) {
	c.mu.Lock()
	mProt, mUnprot := c.mFRRProtected, c.mFRRUnprotected
	mInst, mCompute, mRepair := c.mFRRInstalled, c.mFRRCompute, c.mFRRRepairLabels
	c.mu.Unlock()
	classes := []string{"node", "link", "downstream", "loop-free"}
	reasons := []string{"no-lfa", "no-sr", "suppressed"}
	installedLFA, installedTILFA := 0, 0
	for area, st := range stats {
		label := area.String()
		for _, cl := range classes {
			mProt.With(label, cl).Set(float64(st.protected[cl]))
		}
		for _, rs := range reasons {
			mUnprot.With(label, rs).Set(float64(st.unprotected[rs]))
		}
		mRepair.With(label).Set(float64(st.repairLabels))
		mCompute.With(label).Observe(dur.Seconds())
		installedLFA += st.installed["lfa"]
		installedTILFA += st.installed["ti-lfa"]
	}
	mInst.With("lfa").Set(float64(installedLFA))
	mInst.With("ti-lfa").Set(float64(installedTILFA))
}

// attachAllBackups runs the RFC 5286 base LFA and TI-LFA fallback for every
// route in `routes`, mutating each RouteEntry's Backups slice in place. It is a
// no-op when fast-reroute is disabled, so the disabled route set is unchanged.
func attachAllBackups(routes []RouteEntry, in fastRerouteInput) map[types.AreaID]areaFRRStats {
	stats := make(map[types.AreaID]areaFRRStats)
	if !in.cfg.Enabled {
		return stats
	}
	byArea := make(map[types.AreaID][]int)
	for i := range routes {
		byArea[routes[i].AreaID] = append(byArea[routes[i].AreaID], i)
	}
	for area, idxs := range byArea {
		res := in.results[area]
		g := in.graphs[area]
		if res == nil || g == nil || g.Routers[in.root] == nil {
			continue
		}
		st := attachAreaBackups(routes, idxs, area, g, res, in)
		stats[area] = st
	}
	return stats
}

func attachAreaBackups(routes []RouteEntry, idxs []int, area types.AreaID, g *Graph, res *Result, in fastRerouteInput) areaFRRStats {
	st := newAreaFRRStats()
	cands := neighborLinks(g, in.root, in.nh, in.resolver)
	candByAddr := make(map[netip.Addr]candLink, len(cands))
	for _, c := range cands {
		candByAddr[c.addr] = c
	}
	sptByRouter := make(map[types.RouterID]*Result)
	ensureSPT := func(r types.RouterID) *Result {
		if r == (types.RouterID{}) {
			return nil
		}
		if s, ok := sptByRouter[r]; ok {
			return s
		}
		s := computeWithNextHop(g, r, in.maxPaths, in.nh)
		sptByRouter[r] = s
		return s
	}
	for _, c := range cands {
		ensureSPT(c.neighbor)
	}
	attach := attachmentVertices(res, g)
	abrCount := countBorder(in.border, area, BorderRouterABR)
	asbrCount := countBorder(in.border, area, BorderRouterASBR)

	for _, i := range idxs {
		r := &routes[i]
		if suppressLFA(r.Type, area, abrCount, asbrCount, in.virtualLinks) {
			st.unprotected["suppressed"] += len(r.NextHops)
			continue
		}
		v, ok := routeVertex(*r, attach)
		if !ok {
			st.unprotected["no-lfa"] += len(r.NextHops)
			continue
		}
		backups := make([]Backup, len(r.NextHops))
		any := false
		for pi := range r.NextHops {
			primary := r.NextHops[pi]
			ensureSPT(primary.Router)
			if b, found := selectLFA(v, primary, r.NextHops, cands, candByAddr, sptByRouter, res, in.cfg); found {
				backups[pi] = b
				any = true
				st.installed["lfa"]++
				st.protected[classKey(b)]++
				continue
			}
			if in.cfg.Mode == FastRerouteTILFA && in.sr != nil {
				if b, found := buildTILFA(g, res, v, primary, in, ensureSPT); found {
					backups[pi] = b
					any = true
					st.installed["ti-lfa"]++
					st.protected[classKey(b)]++
					st.repairLabels += len(b.RepairLabels)
					continue
				}
				st.unprotected["no-sr"]++
				continue
			}
			st.unprotected["no-lfa"]++
		}
		if any {
			r.Backups = backups
		}
	}
	return st
}

// selectLFA runs the RFC 5286 Section 3.6 per-primary selection for the primary
// next-hop protecting destination vertex v, returning the best loop-free
// alternate. It returns false when no directly-connected neighbor is loop-free.
func selectLFA(v VertexID, primary NextHop, primarySet []NextHop, cands []candLink, candByAddr map[netip.Addr]candLink, sptByRouter map[types.RouterID]*Result, res *Result, cfg FastRerouteConfig) (Backup, bool) {
	dSD := vertexDist(res, v) // D_opt(S,D)
	if dSD >= LSInfinity {
		return Backup{}, false
	}
	rootV := routerVertex(res.Root)
	e := primary.Router // the protected primary neighbor E
	sptE := sptByRouter[e]
	dEV := vertexDist(sptE, v) // D_opt(E,D)
	primaryLink, primaryKnown := candByAddr[primary.Addr]
	primaryBroadcast := primaryKnown && primaryLink.broadcast

	var best candResult
	found := false
	for _, c := range cands {
		// RFC 5286 Section 3.6 step 2: the primary next-hop itself is not an alternate.
		if c.addr == primary.Addr {
			continue
		}
		// RFC 5286 Section 3.5: MUST NOT use an alternate whose link cost or reverse
		// cost is LSInfinity (a neighbor reachable only over a costed-out link).
		if c.forwardCost >= LSInfinity || c.reverseCost >= LSInfinity {
			continue
		}
		sptN := sptByRouter[c.neighbor]
		if sptN == nil {
			continue
		}
		dND := vertexDist(sptN, v)     // D_opt(N,D)
		dNS := vertexDist(sptN, rootV) // D_opt(N,S)
		// RFC 5286 Section 3.1 Inequality 1 (STRICT): a neighbor is a loop-free
		// alternate iff D_opt(N,D) < D_opt(N,S) + D_opt(S,D). Equality is NOT loop-free.
		if dND >= clampSum(dNS, dSD) {
			continue
		}
		linkProtect := c.addr != primary.Addr
		// RFC 5286 Section 3.3: on a broadcast/NBMA primary link, an alternate over
		// the SAME pseudo-node is NOT link-protecting (S's own path to N crosses the
		// same PN); a different link is link-protecting only if it is loop-free wrt
		// the PN (Inequality 4).
		if primaryBroadcast {
			if c.broadcast && c.network == primaryLink.network {
				linkProtect = false
			} else {
				pn := networkVertex(primaryLink.network)
				dNPN := vertexDist(sptN, pn)
				dPND := pseudoNodeDist(candByAddr, primaryLink.network, v, sptByRouter, res)
				// RFC 5286 Section 3.3 Inequality 4: D_opt(N,D) < D_opt(N,PN) + D_opt(PN,D).
				if dND >= clampSum(dNPN, dPND) {
					linkProtect = false
				}
			}
		}
		nodeProtect := false
		if sptE != nil && e != (types.RouterID{}) {
			dNE := vertexDist(sptN, routerVertex(e)) // D_opt(N,E)
			// RFC 5286 Section 3.2 Inequality 3 (STRICT): node-protecting iff
			// D_opt(N,D) < D_opt(N,E) + D_opt(E,D). On equality assume NO node protection.
			nodeProtect = dND < clampSum(dNE, dEV)
		}
		// RFC 5286 Section 1.1 Inequality 2 + Errata 2323: downstream is measured
		// against D_opt(S,D), NOT D_opt(P_i.neighbor,D).
		downstream := dND < dSD
		cand := candResult{
			addr:        c.addr,
			iface:       c.iface,
			linkProtect: linkProtect,
			nodeProtect: nodeProtect,
			downstream:  downstream,
			isPrimary:   containsAddr(primarySet, c.addr),
			dist:        dND,
		}
		if !found || betterCand(cand, best, cfg) {
			best = cand
			found = true
		}
	}
	if !found {
		return Backup{}, false
	}
	return Backup{
		NextHop:     best.addr,
		Interface:   best.iface,
		LinkProtect: best.linkProtect,
		NodeProtect: best.nodeProtect,
		Downstream:  best.downstream,
		Kind:        BackupLFA,
	}, true
}

// betterCand implements the RFC 5286 Section 3.6 preference order: prefer a
// primary alternate (rule 4, when configured), then node-and-link > node > link
// protection (rules 1-2), then a downstream alternate (Errata 2323 step 16),
// then the closer alternate, then a deterministic address tie-break.
func betterCand(a, b candResult, cfg FastRerouteConfig) bool {
	if cfg.PreferPrimary && a.isPrimary != b.isPrimary {
		return a.isPrimary
	}
	if ar, br := protRank(a, cfg), protRank(b, cfg); ar != br {
		return ar > br
	}
	if a.downstream != b.downstream {
		return a.downstream
	}
	if a.dist != b.dist {
		return a.dist < b.dist
	}
	return a.addr.Compare(b.addr) < 0
}

func protRank(c candResult, cfg FastRerouteConfig) int {
	np := c.nodeProtect && cfg.NodeProtection
	switch {
	case np && c.linkProtect:
		return 3
	case np:
		return 2
	case c.linkProtect:
		return 1
	default:
		return 0
	}
}

// neighborLinks enumerates the computing router S's directly-connected neighbor
// links (the RFC 5286 candidate alternates), with the forward and reverse link
// costs for the Section 3.5 gate.
func neighborLinks(g *Graph, root types.RouterID, nh NextHopSource, resolver InterfaceResolver) []candLink {
	r := g.Routers[root]
	if r == nil {
		return nil
	}
	var out []candLink
	for _, l := range r.Links {
		switch l.Type {
		case rlP2P, rlVirtual:
			nb := routerIDFromLinkStateID(l.LinkID)
			if !twoWayRouterLink(g, root, nb) {
				continue
			}
			addr, ok := nh.P2PNextHop(g, nb, root)
			if !ok {
				continue
			}
			out = append(out, candLink{
				neighbor:    nb,
				addr:        addr,
				forwardCost: uint64(l.Metric),
				reverseCost: reverseP2PCost(g, nb, root),
			})
		case rlTransit:
			nw := l.LinkID
			n := g.Networks[nw]
			if n == nil {
				continue
			}
			for _, other := range n.AttachedRouters {
				if other == root || !twoWayRouterNetworkLink(g, other, nw) {
					continue
				}
				addr, ok := nh.TransitNextHop(g, other, nw)
				if !ok {
					continue
				}
				out = append(out, candLink{
					neighbor:    other,
					addr:        addr,
					forwardCost: uint64(l.Metric),
					reverseCost: reverseTransitCost(g, other, nw),
					broadcast:   true,
					network:     nw,
				})
			}
		}
	}
	if resolver != nil {
		for i := range out {
			if iface, ok := resolver.ResolveInterface(out[i].addr); ok {
				out[i].iface = iface
			}
		}
	}
	return out
}

func reverseP2PCost(g *Graph, nb, root types.RouterID) uint64 {
	r := g.Routers[nb]
	if r == nil {
		return LSInfinity
	}
	want := linkStateIDFromRouterID(root)
	best := LSInfinity
	for _, l := range r.Links {
		if (l.Type == rlP2P || l.Type == rlVirtual) && l.LinkID == want {
			if c := uint64(l.Metric); c < best {
				best = c
			}
		}
	}
	return best
}

func reverseTransitCost(g *Graph, nb types.RouterID, nw types.LinkStateID) uint64 {
	r := g.Routers[nb]
	if r == nil {
		return LSInfinity
	}
	for _, l := range r.Links {
		if l.Type == rlTransit && l.LinkID == nw {
			return uint64(l.Metric)
		}
	}
	return LSInfinity
}

// pseudoNodeDist is D_opt(PN,D): the shortest distance from the broadcast
// pseudo-node to destination v, taken as the minimum over the network's attached
// routers (the PN reaches each attached router at cost 0, RFC 2328).
func pseudoNodeDist(candByAddr map[netip.Addr]candLink, nw types.LinkStateID, v VertexID, sptByRouter map[types.RouterID]*Result, res *Result) uint64 {
	best := LSInfinity
	for _, c := range candByAddr {
		if !c.broadcast || c.network != nw {
			continue
		}
		if spt := sptByRouter[c.neighbor]; spt != nil {
			if d := vertexDist(spt, v); d < best {
				best = d
			}
		}
	}
	if d := vertexDist(res, v); d < best { // the root S is attached to the PN too
		best = d
	}
	return best
}

// attachmentVertices maps each intra-area prefix to the SPF vertex it attaches
// to (the stub-hosting router or the transit network), mirroring BuildRoutes so
// a route's backup is computed for the vertex behind it. The stub cost cancels in
// every RFC 5286 inequality, so the per-vertex classification is exact for the
// prefix behind the vertex.
func attachmentVertices(res *Result, g *Graph) map[netip.Prefix]VertexID {
	m := make(map[netip.Prefix]VertexID)
	for id, nr := range res.Nodes {
		if nr == nil || len(nr.NextHops) == 0 {
			continue
		}
		switch id.Kind {
		case VertexRouter:
			r := g.Routers[id.Router]
			if r == nil {
				continue
			}
			for _, link := range r.Links {
				if link.Type != rlStub {
					continue
				}
				if pfx, ok := stubPrefix(link.LinkID, link.LinkData); ok {
					m[pfx] = id
				}
			}
		case VertexNetwork:
			nv := g.Networks[id.Network]
			if nv == nil {
				continue
			}
			if pfx, ok := stubPrefix(nv.ID, nv.NetworkMask); ok {
				m[pfx] = id
			}
		}
	}
	return m
}

// routeVertex returns the SPF vertex a route's backup should be computed for:
// the intra-area attachment vertex, or (inter-area/external, or a fallback) the
// route's advertising border-router vertex reached intra-area.
func routeVertex(r RouteEntry, attach map[netip.Prefix]VertexID) (VertexID, bool) {
	if r.Type == RouteIntraArea {
		if v, ok := attach[r.Prefix]; ok {
			return v, true
		}
	}
	if r.Origin != (types.RouterID{}) {
		return routerVertex(r.Origin), true
	}
	return VertexID{}, false
}

func countBorder(border []BorderRouterEntry, area types.AreaID, kind BorderRouterKind) int {
	seen := make(map[types.RouterID]struct{})
	for _, b := range border {
		if b.AreaID == area && b.Kind == kind {
			seen[b.RouterID] = struct{}{}
		}
	}
	return len(seen)
}

func classKey(b Backup) string {
	switch {
	case b.NodeProtect:
		return "node"
	case b.LinkProtect:
		return "link"
	case b.Downstream:
		return "downstream"
	default:
		return "loop-free"
	}
}

func vertexDist(res *Result, v VertexID) uint64 {
	if res == nil {
		return LSInfinity
	}
	if n := res.Nodes[v]; n != nil {
		return n.Metric
	}
	return LSInfinity
}

func clampSum(a, b uint64) uint64 {
	if a >= LSInfinity || b >= LSInfinity {
		return LSInfinity
	}
	s := a + b
	if s < a || s >= LSInfinity {
		return LSInfinity
	}
	return s
}

func containsAddr(hops []NextHop, addr netip.Addr) bool {
	for _, h := range hops {
		if h.Addr == addr {
			return true
		}
	}
	return false
}
