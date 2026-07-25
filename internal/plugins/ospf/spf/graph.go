// Design: plan/learned/962-ospf-8-spf-rib.md -- OSPFv2 SPF graph build from the synced LSDB.
// RFC 2328 Section 16.1 builds the intra-area shortest-path tree over router
// vertices and transit-network vertices before stub networks are attached.
// RFC: rfc/short/rfc5286.md (post-convergence graph clone for TI-LFA repair)

package spf

import (
	"slices"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VertexKind identifies the two transit vertex kinds RFC 2328 Section 16.1 uses.
type VertexKind uint8

const (
	VertexRouter VertexKind = iota + 1
	VertexNetwork
)

// VertexID is a comparable SPF vertex key. Router vertices are keyed by RouterID;
// transit-network vertices are keyed by the Network-LSA Link State ID, which is
// the DR interface address, not the network prefix.
type VertexID struct {
	Kind    VertexKind
	Router  types.RouterID
	Network types.LinkStateID
}

func routerVertex(id types.RouterID) VertexID { return VertexID{Kind: VertexRouter, Router: id} }

func networkVertex(id types.LinkStateID) VertexID { return VertexID{Kind: VertexNetwork, Network: id} }

// RouterVertex is one Type 1 Router-LSA decoded into SPF-relevant data.
type RouterVertex struct {
	ID    types.RouterID
	Flags uint8
	Links []packet.RouterLink
}

// NetworkVertex is one Type 2 Network-LSA decoded into SPF-relevant data.
//
// ID is the vertex's graph-local key. In OSPFv2 it is the Network-LSA Link State ID (the
// DR's interface IP) and the join from a router transit link is by that IP. OSPFv3 identifies
// a transit network by (DR Router ID, DR Interface ID) -- 64 bits that do not fit one 32-bit
// LinkStateID -- so the v6 BuildGraph assigns ID a synthetic per-graph handle and records the
// real identity in AdvertisingDR (DR Router ID) + DRInterfaceID (DR Interface ID); the prefix
// model joins an Intra-Area-Prefix-LSA's Network reference by that pair. DRInterfaceID is zero
// for OSPFv2 (the IP-keyed join needs no separate Interface ID).
type NetworkVertex struct {
	ID              types.LinkStateID
	AdvertisingDR   types.RouterID
	DRInterfaceID   types.LinkStateID
	NetworkMask     [4]byte
	AttachedRouters []types.RouterID
}

// Graph is one area's transit graph plus the router stub links retained for the
// second SPF stage.
type Graph struct {
	Area     types.AreaID
	Routers  map[types.RouterID]*RouterVertex
	Networks map[types.LinkStateID]*NetworkVertex
}

// Source is the narrow LSDB read API SPF needs. The OSPF LSDB implements it;
// tests can provide a hand-built source.
type Source interface {
	Summary(types.AreaID) []packet.LSAHeader
	LookupLSA(types.AreaID, types.LSAKey) (packet.LSA, bool)
}

// NewGraph returns an empty per-area graph.
func NewGraph(area types.AreaID) *Graph {
	return &Graph{
		Area:     area,
		Routers:  make(map[types.RouterID]*RouterVertex),
		Networks: make(map[types.LinkStateID]*NetworkVertex),
	}
}

// BuildGraph reads one area's LSDB once and decodes only Router-LSAs and
// Network-LSAs. A malformed LSA excludes that vertex, not the whole run.
func BuildGraph(src Source, area types.AreaID) *Graph {
	g := NewGraph(area)
	if src == nil {
		return g
	}
	for _, h := range src.Summary(area) {
		if h.Age.IsMaxAge() {
			continue
		}
		switch h.Type {
		case types.LSTypeRouter:
			lsa, ok := src.LookupLSA(area, h.Key())
			if !ok {
				continue
			}
			body, err := routerBody(lsa)
			if err != nil {
				continue
			}
			links := make([]packet.RouterLink, len(body.Links))
			copy(links, body.Links)
			g.Routers[h.AdvertisingRouter] = &RouterVertex{ID: h.AdvertisingRouter, Flags: body.Flags, Links: links}
		case types.LSTypeNetwork:
			lsa, ok := src.LookupLSA(area, h.Key())
			if !ok {
				continue
			}
			body, err := networkBody(lsa)
			if err != nil {
				continue
			}
			attached := make([]types.RouterID, len(body.AttachedRouters))
			copy(attached, body.AttachedRouters)
			g.Networks[h.LinkStateID] = &NetworkVertex{
				ID:              h.LinkStateID,
				AdvertisingDR:   h.AdvertisingRouter,
				NetworkMask:     body.NetworkMask,
				AttachedRouters: attached,
			}
		default:
			continue
		}
	}
	return g
}

// Clone returns a deep copy of the graph: the router and network vertex maps and
// their per-vertex Links / AttachedRouters slices are copied, so a subsequent
// excludeLink / excludeRouter mutation for a TI-LFA post-convergence SPF never
// touches the live graph the Computer retains.
func (g *Graph) Clone() *Graph {
	out := NewGraph(g.Area)
	for id, r := range g.Routers {
		if r == nil {
			continue
		}
		links := make([]packet.RouterLink, len(r.Links))
		copy(links, r.Links)
		out.Routers[id] = &RouterVertex{ID: r.ID, Flags: r.Flags, Links: links}
	}
	for id, n := range g.Networks {
		if n == nil {
			continue
		}
		attached := make([]types.RouterID, len(n.AttachedRouters))
		copy(attached, n.AttachedRouters)
		out.Networks[id] = &NetworkVertex{
			ID:              n.ID,
			AdvertisingDR:   n.AdvertisingDR,
			DRInterfaceID:   n.DRInterfaceID,
			NetworkMask:     n.NetworkMask,
			AttachedRouters: attached,
		}
	}
	return out
}

// excludeLink removes the point-to-point / virtual adjacency between routers a
// and b in BOTH directions so a post-convergence SPF (TI-LFA link protection)
// cannot cross the protected link. The two-way check in Compute then treats the
// link as down. Mutates the receiver; call on a Clone.
func (g *Graph) excludeLink(a, b types.RouterID) {
	dropLink(g.Routers[a], b)
	dropLink(g.Routers[b], a)
}

func dropLink(v *RouterVertex, to types.RouterID) {
	if v == nil {
		return
	}
	want := linkStateIDFromRouterID(to)
	out := v.Links[:0]
	for _, l := range v.Links {
		if (l.Type == packet.RouterLinkTypeP2P || l.Type == packet.RouterLinkTypeVirtual) && l.LinkID == want {
			continue
		}
		out = append(out, l)
	}
	v.Links = out
}

// excludeRouter removes router r entirely: its vertex, every point-to-point link
// pointing at it, and its membership in every transit network, so a
// post-convergence SPF (TI-LFA node protection) routes around the protected
// node. Mutates the receiver; call on a Clone.
func (g *Graph) excludeRouter(r types.RouterID) {
	delete(g.Routers, r)
	want := linkStateIDFromRouterID(r)
	for _, v := range g.Routers {
		if v == nil {
			continue
		}
		out := v.Links[:0]
		for _, l := range v.Links {
			if (l.Type == packet.RouterLinkTypeP2P || l.Type == packet.RouterLinkTypeVirtual) && l.LinkID == want {
				continue
			}
			out = append(out, l)
		}
		v.Links = out
	}
	for _, n := range g.Networks {
		if n == nil {
			continue
		}
		n.AttachedRouters = slices.DeleteFunc(n.AttachedRouters, func(id types.RouterID) bool { return id == r })
	}
}

func routerBody(lsa packet.LSA) (packet.RouterLSA, error) {
	if lsa.Router != nil {
		return *lsa.Router, nil
	}
	return lsa.DecodeRouter()
}

func networkBody(lsa packet.LSA) (packet.NetworkLSA, error) {
	if lsa.Network != nil {
		return *lsa.Network, nil
	}
	return lsa.DecodeNetwork()
}
