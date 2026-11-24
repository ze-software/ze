// Design: docs/architecture/core-design.md -- BGP implied-filter provider seam for `| implied`
// Related: register.go -- registers collectBGPImpliedFilters via config.RegisterImpliedFilterProvider
// Related: filter_insertion.go -- plugin-declared FilterInsertions provenance
// Related: peers.go -- prependDefaultFilters and appendDefaultExportFilters
// Related: ../filterapi/policy.go -- Policy.Implied pure-value seam

package bgpconfig

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// collectBGPImpliedFilters reads every automatically inserted filter-chain
// member for BGP static peers and dynamic group templates from the unified
// filterapi.Policy.Implied() seam. It clones tree before resolution so the
// caller's configuration tree is never mutated, and names no filter itself.
func collectBGPImpliedFilters(tree *config.Tree) ([]config.ImpliedFilterMember, error) {
	if tree == nil {
		return nil, nil
	}
	if tree.GetContainer(attachmentOwnerBGP) == nil {
		return nil, nil
	}
	cloned := tree.Clone()
	peers, groups, err := buildPeerPopulations(cloned)
	if err != nil {
		return nil, err
	}
	return ImpliedFiltersFromPeers(peers, groups), nil
}

// ImpliedFiltersFromPeers converts the Policy.Implied() entries of compiled
// static peers and dynamic group templates into protocol-neutral
// config.ImpliedFilterMember records in chain order without naming any filter.
func ImpliedFiltersFromPeers(peers []*reactor.PeerSettings, groups []*reactor.DynamicGroupConfig) []config.ImpliedFilterMember {
	var out []config.ImpliedFilterMember
	for _, ps := range peers {
		if ps == nil {
			continue
		}
		target := impliedTargetPathForPeer(ps)
		out = appendImpliedPolicy(out, target, ps.ImportPolicy)
		out = appendImpliedPolicy(out, target, ps.ExportPolicy)
	}
	for _, dg := range groups {
		if dg == nil {
			continue
		}
		if dg.Settings == nil {
			continue
		}
		target := impliedTargetPathForGroup(dg)
		out = appendImpliedPolicy(out, target, dg.Settings.ImportPolicy)
		out = appendImpliedPolicy(out, target, dg.Settings.ExportPolicy)
	}
	return out
}

func appendImpliedPolicy(out []config.ImpliedFilterMember, targetPath string, pol filterapi.Policy) []config.ImpliedFilterMember {
	for _, f := range pol.Implied() {
		out = append(out, config.ImpliedFilterMember{
			TargetPath: targetPath,
			Direction:  f.Direction,
			Position:   impliedPositionLabel(f.Position),
			Ref:        f.Ref,
			ImpliedBy:  f.ImpliedBy,
			Required:   f.Required,
			Reason:     f.Reason,
		})
	}
	return out
}

func impliedPositionLabel(pos filterapi.ImpliedPosition) string {
	switch pos {
	case filterapi.ImpliedPositionHead:
		return "head"
	case filterapi.ImpliedPositionTail:
		return "tail"
	case filterapi.ImpliedPositionUnspecified:
		return ""
	}
	return ""
}

func impliedTargetPathForPeer(ps *reactor.PeerSettings) string {
	if ps.BusPath != "" {
		return ps.BusPath
	}
	if ps.Name != "" {
		return peerCanonicalPath(ps)
	}
	if ps.Address.IsValid() {
		var tb textbuf.Buffer
		return tb.Str(bgpSecondaryIPPrefix).Addr(ps.Address).String()
	}
	return ""
}

func impliedTargetPathForGroup(dg *reactor.DynamicGroupConfig) string {
	if dg.Settings.BusPath != "" {
		return dg.Settings.BusPath
	}
	var tb textbuf.Buffer
	return tb.Str("/bgp/group/").Str(dg.GroupName).String()
}
