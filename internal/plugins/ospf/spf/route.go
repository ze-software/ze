// Design: plan/learned/962-ospf-8-spf-rib.md -- OSPF route table, preference, and diff.
// RFC 2328 Section 11 gives the route preference order. The OSPF package
// resolves that order before publishing one Loc-RIB path set per prefix.
// RFC: rfc/short/rfc5286.md (Section 3.6 fast-reroute backup protection classes)

package spf

import (
	"net/netip"
	"slices"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// RouteType is the OSPF-internal path type carried in snapshots and used for
// intra-protocol preference before a locrib.Path is inserted.
type RouteType uint8

const (
	RouteIntraArea RouteType = iota + 1
	RouteInterArea
	RouteExternalType1
	RouteExternalType2
)

// Display names for each RouteType, shared by String() and the tests and
// renderers that compare against them.
const (
	routeNameIntraArea     = "intra-area"
	routeNameInterArea     = "inter-area"
	routeNameExternalType1 = "external-type-1"
	routeNameExternalType2 = "external-type-2"
)

func (t RouteType) String() string {
	switch t {
	case RouteIntraArea:
		return routeNameIntraArea
	case RouteInterArea:
		return routeNameInterArea
	case RouteExternalType1:
		return routeNameExternalType1
	case RouteExternalType2:
		return routeNameExternalType2
	default:
		return "unknown"
	}
}

func routeTypeRank(t RouteType) int {
	switch t {
	case RouteIntraArea:
		return 0
	case RouteInterArea:
		return 1
	case RouteExternalType1:
		return 2
	case RouteExternalType2:
		return 3
	default:
		return 4
	}
}

// BackupKind distinguishes a directly-connected loop-free alternate (RFC 5286)
// from a Segment-Routing repair-tunnel backup (TI-LFA).
type BackupKind uint8

const (
	// BackupNone is the zero value: the primary next-hop has no fast-reroute backup.
	BackupNone BackupKind = iota
	// BackupLFA is a directly-connected loop-free alternate (RFC 5286 base LFA):
	// forward to the alternate neighbor, no label imposition.
	BackupLFA
	// BackupTILFA is a TI-LFA repair: forward to the first hop of the
	// post-convergence path and impose the SR repair label stack.
	BackupTILFA
)

// Backup is the pre-computed fast-reroute alternate protecting ONE primary
// next-hop of a prefix (RFC 5286 / TI-LFA). It is keyed per primary next-hop,
// never a route scalar and never an ECMP sibling: it is used only when the
// primary link goes down. RepairLabels is the TI-LFA SR label stack (outermost
// first, each a 20-bit MPLS label) built once per best-path change and shared,
// not mutated, exactly like the primary Labels contract; it is nil for a base
// LFA. LinkProtect / NodeProtect / Downstream are the non-mutually-exclusive
// RFC 5286 Section 3.6 protection characteristics.
type Backup struct {
	NextHop      netip.Addr
	Interface    string
	RepairLabels []uint32
	LinkProtect  bool
	NodeProtect  bool
	Downstream   bool
	Kind         BackupKind
}

// Valid reports whether the backup carries a usable next-hop.
func (b Backup) Valid() bool { return b.Kind != BackupNone && b.NextHop.IsValid() }

// Class renders the RFC 5286 Section 3.6 protection class for `show` output:
// node-and-link (NP+LP), node, link, or downstream. An unprotected backup
// renders the empty string.
func (b Backup) Class() string {
	if !b.Valid() {
		return ""
	}
	switch {
	case b.NodeProtect && b.LinkProtect:
		return "node+link"
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

// backupEqual reports whether two backups are identical for change detection so
// a backup-only delta re-installs (RFC 5286: a stale backup after a topology
// change that did not move the primary must be re-programmed).
func backupEqual(a, b Backup) bool {
	return a.NextHop == b.NextHop &&
		a.Interface == b.Interface &&
		a.LinkProtect == b.LinkProtect &&
		a.NodeProtect == b.NodeProtect &&
		a.Downstream == b.Downstream &&
		a.Kind == b.Kind &&
		slices.Equal(a.RepairLabels, b.RepairLabels)
}

// RouteEntry is one OSPF route selected for installation. It carries the path
// type because locrib.Path does not. Backups, when fast-reroute is enabled, is a
// per-primary parallel slice: Backups[i] protects NextHops[i]. It is nil (never
// allocated) when fast-reroute is disabled, so the route set, diff and snapshot
// are byte-for-byte identical to a router without fast-reroute.
type RouteEntry struct {
	AreaID   types.AreaID
	Prefix   netip.Prefix
	Metric   uint64
	Type     RouteType
	Origin   types.RouterID
	NextHops []NextHop
	Backups  []Backup
}

// BuildRoutes performs RFC 2328 Section 16.1 stage 2: attach stub-network links
// advertised by already-reached router vertices, retaining each router vertex's
// next-hop set. It also installs a route to each transit (broadcast LAN) network
// reached in stage 1 (RFC 2328 Section 16.1 step (4): the network's own prefix is
// the Network-LSA Link State ID masked with its network mask). The root's own
// connected prefixes have no next-hop and are skipped because the connected
// source owns directly connected routes.
func BuildRoutes(res *Result, maxPaths int, resolver InterfaceResolver) []RouteEntry {
	if res == nil || res.Graph == nil {
		return nil
	}
	if maxPaths <= 0 {
		maxPaths = DefaultMaxPaths
	}
	var candidates []RouteEntry
	for id, nr := range res.Nodes {
		switch id.Kind {
		case VertexRouter:
			if id.Router == res.Root || nr == nil || len(nr.NextHops) == 0 {
				continue
			}
			r := res.Graph.Routers[id.Router]
			if r == nil {
				continue
			}
			nextHops := decorateNextHops(nr.NextHops, resolver, maxPaths)
			if len(nextHops) == 0 {
				continue
			}
			for _, link := range r.Links {
				if link.Type != packet.RouterLinkTypeStub {
					continue
				}
				pfx, ok := stubPrefix(link.LinkID, link.LinkData)
				if !ok {
					continue
				}
				metric := clampMetric(nr.Metric, uint64(link.Metric))
				if metric >= LSInfinity {
					continue
				}
				candidates = append(candidates, RouteEntry{
					AreaID:   res.Area,
					Prefix:   pfx,
					Metric:   metric,
					Type:     RouteIntraArea,
					Origin:   id.Router,
					NextHops: nextHops,
				})
			}
		case VertexNetwork:
			// RFC 2328 Section 16.1 step (4): a transit network reached in stage 1
			// contributes a route to its own prefix. Directly-connected LANs have no
			// SPF next-hop (the connected source owns them) and are skipped.
			if nr == nil || len(nr.NextHops) == 0 || nr.Metric >= LSInfinity {
				continue
			}
			nv := res.Graph.Networks[id.Network]
			if nv == nil {
				continue
			}
			pfx, ok := stubPrefix(nv.ID, nv.NetworkMask)
			if !ok {
				continue
			}
			nextHops := decorateNextHops(nr.NextHops, resolver, maxPaths)
			if len(nextHops) == 0 {
				continue
			}
			candidates = append(candidates, RouteEntry{
				AreaID:   res.Area,
				Prefix:   pfx,
				Metric:   nr.Metric,
				Type:     RouteIntraArea,
				Origin:   nv.AdvertisingDR,
				NextHops: nextHops,
			})
		}
	}
	return selectBestRoutes(candidates, maxPaths)
}

// selectBestRoutes resolves the OSPF-internal route preference order and equal
// cost next-hop merge, returning exactly one winning RouteEntry per prefix.
func selectBestRoutes(candidates []RouteEntry, maxPaths int) []RouteEntry {
	if maxPaths <= 0 {
		maxPaths = DefaultMaxPaths
	}
	best := make(map[netip.Prefix]RouteEntry)
	for _, c := range candidates {
		if !c.Prefix.IsValid() || len(c.NextHops) == 0 || c.Metric >= LSInfinity {
			continue
		}
		c.Prefix = c.Prefix.Masked()
		c.NextHops = capNextHops(c.NextHops, maxPaths)
		sortNextHops(c.NextHops)
		if len(c.NextHops) == 0 {
			continue
		}
		cur, ok := best[c.Prefix]
		if !ok || routeBetter(c, cur) {
			best[c.Prefix] = c
			continue
		}
		if routeSamePreference(c, cur) {
			cur.NextHops, _ = mergeNextHops(cur.NextHops, c.NextHops, maxPaths)
			sortNextHops(cur.NextHops)
			if compare4(c.Origin, cur.Origin) < 0 {
				cur.Origin = c.Origin
			}
			best[c.Prefix] = cur
		}
	}
	out := make([]RouteEntry, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix.Compare(out[j].Prefix) < 0 })
	return out
}

func routeBetter(a, b RouteEntry) bool {
	ar, br := routeTypeRank(a.Type), routeTypeRank(b.Type)
	if ar != br {
		return ar < br
	}
	return a.Metric < b.Metric
}

func routeSamePreference(a, b RouteEntry) bool {
	return routeTypeRank(a.Type) == routeTypeRank(b.Type) && a.Metric == b.Metric
}

func decorateNextHops(in []NextHop, resolver InterfaceResolver, maxPaths int) []NextHop {
	out := capNextHops(in, maxPaths)
	if resolver != nil {
		for i := range out {
			if out[i].Interface != "" {
				continue
			}
			if iface, ok := resolver.ResolveInterface(out[i].Addr); ok {
				out[i].Interface = iface
			}
		}
	}
	sortNextHops(out)
	return out
}

func stubPrefix(id types.LinkStateID, mask [4]byte) (netip.Prefix, bool) {
	bits, ok := maskPrefixLen(mask)
	if !ok {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addrFrom4([4]byte(id)), bits).Masked(), true
}

func maskPrefixLen(mask [4]byte) (int, bool) {
	bits := 0
	seenZero := false
	for _, b := range mask {
		for bit := 7; bit >= 0; bit-- {
			one := b&(1<<uint(bit)) != 0
			if one {
				if seenZero {
					return 0, false
				}
				bits++
				continue
			}
			seenZero = true
		}
	}
	return bits, true
}

// RouteDelta is the add/change/remove diff between two OSPF route sets.
type RouteDelta struct {
	Added   []RouteEntry
	Changed []RouteEntry
	Removed []netip.Prefix
}

func (d RouteDelta) Empty() bool {
	return len(d.Added) == 0 && len(d.Changed) == 0 && len(d.Removed) == 0
}

// DiffRoutes computes the precise add/change/remove route delta for installation.
func DiffRoutes(prev, cur map[netip.Prefix]RouteEntry) RouteDelta {
	var d RouteDelta
	for pfx, c := range cur {
		p, ok := prev[pfx]
		if !ok {
			d.Added = append(d.Added, c)
			continue
		}
		if !routeEqual(p, c) {
			d.Changed = append(d.Changed, c)
		}
	}
	for pfx := range prev {
		if _, ok := cur[pfx]; !ok {
			d.Removed = append(d.Removed, pfx)
		}
	}
	sort.Slice(d.Added, func(i, j int) bool { return d.Added[i].Prefix.Compare(d.Added[j].Prefix) < 0 })
	sort.Slice(d.Changed, func(i, j int) bool { return d.Changed[i].Prefix.Compare(d.Changed[j].Prefix) < 0 })
	sort.Slice(d.Removed, func(i, j int) bool { return d.Removed[i].Compare(d.Removed[j]) < 0 })
	return d
}

func routeEqual(a, b RouteEntry) bool {
	if a.AreaID != b.AreaID || a.Prefix != b.Prefix || a.Metric != b.Metric || a.Type != b.Type || a.Origin != b.Origin {
		return false
	}
	if len(a.NextHops) != len(b.NextHops) {
		return false
	}
	for i := range a.NextHops {
		if a.NextHops[i] != b.NextHops[i] {
			return false
		}
	}
	// RFC 5286: a backup-only change (same primary, new backup) MUST re-install so
	// the forwarding plane never fails over onto a stale alternate.
	if len(a.Backups) != len(b.Backups) {
		return false
	}
	for i := range a.Backups {
		if !backupEqual(a.Backups[i], b.Backups[i]) {
			return false
		}
	}
	return true
}

// IndexByPrefix turns a route slice into a prefix-keyed map for diffing.
func IndexByPrefix(routes []RouteEntry) map[netip.Prefix]RouteEntry {
	m := make(map[netip.Prefix]RouteEntry, len(routes))
	for _, r := range routes {
		m[r.Prefix] = r
	}
	return m
}

// RouteSnapshotEntry is one `show ospf route` row.
type RouteSnapshotEntry struct {
	Area     string             `json:"area"`
	Prefix   string             `json:"prefix"`
	Metric   uint64             `json:"metric"`
	Type     string             `json:"type"`
	Origin   string             `json:"origin"`
	NextHops []RouteSnapshotHop `json:"next_hops"`
}

// RouteSnapshotHop is one next-hop in a route snapshot.
type RouteSnapshotHop struct {
	NextHop   string `json:"next_hop"`
	Interface string `json:"interface,omitempty"`
}

// Snapshot renders installed routes as stable value rows for the later CLI.
func Snapshot(routes []RouteEntry) []RouteSnapshotEntry {
	out := make([]RouteSnapshotEntry, 0, len(routes))
	for _, r := range routes {
		hops := make([]RouteSnapshotHop, 0, len(r.NextHops))
		for _, nh := range r.NextHops {
			hops = append(hops, RouteSnapshotHop{NextHop: nh.Addr.String(), Interface: nh.Interface})
		}
		out = append(out, RouteSnapshotEntry{
			Area:     r.AreaID.String(),
			Prefix:   r.Prefix.String(),
			Metric:   r.Metric,
			Type:     r.Type.String(),
			Origin:   r.Origin.String(),
			NextHops: hops,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Area != out[j].Area {
			return out[i].Area < out[j].Area
		}
		return out[i].Prefix < out[j].Prefix
	})
	return out
}

// FastRerouteSnapshotEntry is one `show ospf route fast-reroute` row. It is a
// dedicated type (not RouteSnapshotEntry) so the base `show ospf route` output
// is untouched by the fast-reroute surface.
type FastRerouteSnapshotEntry struct {
	Area     string                   `json:"area"`
	Prefix   string                   `json:"prefix"`
	Metric   uint64                   `json:"metric"`
	Type     string                   `json:"type"`
	Origin   string                   `json:"origin"`
	NextHops []FastRerouteSnapshotHop `json:"next-hops"`
}

// FastRerouteSnapshotHop is one primary next-hop with its RFC 5286 / TI-LFA
// backup. Protected is false and the backup fields empty when the primary has no
// loop-free alternate (RFC 5286 topology-dependent coverage) and no TI-LFA
// repair. BackupClass is the RFC 5286 Section 3.6 protection class
// (node+link/node/link/downstream); RepairLabels is the TI-LFA SR repair stack.
type FastRerouteSnapshotHop struct {
	NextHop      string   `json:"next-hop"`
	Interface    string   `json:"interface,omitempty"`
	Protected    bool     `json:"protected"`
	Backup       string   `json:"backup,omitempty"`
	BackupClass  string   `json:"backup-class,omitempty"`
	RepairLabels []uint32 `json:"repair-labels,omitempty"`
}

// FastRerouteSnapshot renders `show ospf route fast-reroute`: each primary
// next-hop with its backup next-hop, protection class and repair label stack.
func FastRerouteSnapshot(routes []RouteEntry) []FastRerouteSnapshotEntry {
	out := make([]FastRerouteSnapshotEntry, 0, len(routes))
	for _, r := range routes {
		hops := make([]FastRerouteSnapshotHop, 0, len(r.NextHops))
		for i, nh := range r.NextHops {
			hop := FastRerouteSnapshotHop{NextHop: nh.Addr.String(), Interface: nh.Interface}
			if i < len(r.Backups) && r.Backups[i].Valid() {
				b := r.Backups[i]
				hop.Protected = true
				hop.Backup = b.NextHop.String()
				hop.BackupClass = b.Class()
				hop.RepairLabels = b.RepairLabels
			}
			hops = append(hops, hop)
		}
		out = append(out, FastRerouteSnapshotEntry{
			Area:     r.AreaID.String(),
			Prefix:   r.Prefix.String(),
			Metric:   r.Metric,
			Type:     r.Type.String(),
			Origin:   r.Origin.String(),
			NextHops: hops,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Area != out[j].Area {
			return out[i].Area < out[j].Area
		}
		return out[i].Prefix < out[j].Prefix
	})
	return out
}
