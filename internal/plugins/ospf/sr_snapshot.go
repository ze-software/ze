// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing show snapshot.
// srSnapshot renders this node's configured SR state (SRGB/SRLB ranges, node
// Prefix-SIDs, Adj-SIDs, advertised algorithm) for `show ospf segment-routing`
// and `show ospf ipv6 segment-routing`. It reads the process-global srWire keyed
// by the engine's Router ID (spec-ospf-ext-5 AC-17).
// RFC: rfc/short/rfc8665.md (§3 SRGB/SRLB); rfc/short/rfc8666.md (§4)

package ospf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

type srRangeView struct {
	LowerBound uint32 `json:"lower-bound"`
	UpperBound uint32 `json:"upper-bound"`
	Size       uint32 `json:"size"`
}

type srPrefixSIDView struct {
	Prefix       string `json:"prefix"`
	Index        uint32 `json:"index"`
	NodeSID      bool   `json:"node-sid"`
	NoPHP        bool   `json:"no-php"`
	ExplicitNull bool   `json:"explicit-null"`
}

type srAdjSIDView struct {
	Neighbor string `json:"neighbor,omitempty"`
	Label    uint32 `json:"label"`
	LAN      bool   `json:"lan"`
}

// srSnapshotView is the rendered SR state for one address family.
type srSnapshotView struct {
	Family     string            `json:"family"`
	Enabled    bool              `json:"enabled"`
	Algorithms []uint8           `json:"algorithms"`
	SRGB       []srRangeView     `json:"srgb"`
	SRLB       []srRangeView     `json:"srlb"`
	PrefixSIDs []srPrefixSIDView `json:"prefix-sids"`
	AdjSIDs    []srAdjSIDView    `json:"adj-sids"`
	SRMSPref   *uint8            `json:"srms-preference,omitempty"`
}

func srRangeViews(ranges []sr.LabelRange) []srRangeView {
	out := make([]srRangeView, 0, len(ranges))
	for _, r := range ranges {
		out = append(out, srRangeView{LowerBound: r.Base, UpperBound: r.Last(), Size: r.Size})
	}
	return out
}

// srSnapshot builds the SR show snapshot for the given address family from the
// SR config registered for this engine's Router ID.
func (e *engine) srSnapshot(family string) srSnapshotView {
	view := srSnapshotView{
		Family:     family,
		SRGB:       []srRangeView{},
		SRLB:       []srRangeView{},
		PrefixSIDs: []srPrefixSIDView{},
		AdjSIDs:    []srAdjSIDView{},
	}
	// Read this family's own config so the IPv6 view shows the IPv6 node Prefix-SIDs (and
	// the IPv4 view does not leak them) when both address families configured SR.
	cfg, ok := srWire.getAF(e.cfg.RouterID, family == interfaceFamilyIPv6)
	if !ok || !cfg.Enabled {
		return view
	}
	view.Enabled = true
	view.Algorithms = []uint8{0} // RFC 8665 §3.1: Algorithm 0 (SPF).
	view.SRGB = srRangeViews(cfg.SRGB)
	view.SRLB = srRangeViews(cfg.SRLB)
	for _, p := range cfg.Prefixes {
		view.PrefixSIDs = append(view.PrefixSIDs, srPrefixSIDView{
			Prefix:       p.Prefix.String(),
			Index:        p.Index,
			NodeSID:      p.NodeSID,
			NoPHP:        p.NoPHP,
			ExplicitNull: p.ExplicitNull,
		})
	}
	for _, a := range srWire.adjList(e.cfg.RouterID) {
		v := srAdjSIDView{Label: a.Label, LAN: a.IsLAN}
		if a.IsLAN {
			v.Neighbor = netip.AddrFrom4(a.NeighborID).String()
		}
		view.AdjSIDs = append(view.AdjSIDs, v)
	}
	if cfg.HasSRMS {
		pref := cfg.SRMSPreference
		view.SRMSPref = &pref
	}
	return view
}
