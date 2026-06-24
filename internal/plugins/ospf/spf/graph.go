// Design: plan/learned/962-ospf-8-spf-rib.md -- OSPFv2 SPF graph build from the synced LSDB.
// RFC 2328 Section 16.1 builds the intra-area shortest-path tree over router
// vertices and transit-network vertices before stub networks are attached.

package spf

import (
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
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
