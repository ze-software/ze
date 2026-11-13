// Design: docs/architecture/rib/unified-locrib.md -- authoritative forwarding membership.

package locrib

import (
	"slices"

	"github.com/ze-software/ze/internal/core/redistevents"
)

// ForwardingMember is a selected equal-cost target with its source ownership.
// The source distinguishes identical targets whose FIB permissions may differ.
// Slices follow Path's immutable producer-storage contract. Its zero value
// names no source or target.
type ForwardingMember struct {
	ECMPMember
	Source redistevents.ProtocolID
}

// ForwardingMembers is the authority's forwarding membership, distinct from
// the intra-source ECMP set carried by Change.ECMP. A consumer retaining these
// borrowed values MUST copy their slices before leaving the authority lock.
func (g PathGroup) ForwardingMembers(best Path) []ForwardingMember {
	return forwardingMembers(&g, &best)
}

func forwardingMembers(group *PathGroup, best *Path) []ForwardingMember {
	if !best.Valid() {
		return nil
	}
	if group == nil {
		group = &PathGroup{}
	}
	// Preserve the selected producer's explicit/intra-source set. Expand each
	// other selected source's complete set with that source's own metadata.
	var out []ForwardingMember
	if len(best.ECMP) > 0 {
		for _, target := range best.ECMP {
			out = appendForwardingTarget(out, best, target)
		}
	} else {
		for i := range group.Paths {
			path := &group.Paths[i]
			if path.Source != best.Source {
				continue
			}
			if !equalCostMember(path, best) {
				continue
			}
			out = appendForwardingTarget(out, path, NextHop{Addr: path.NextHop, Interface: path.Interface, OnLink: path.OnLink, Weight: path.Weight})
		}
	}
	for i := range group.Paths {
		path := &group.Paths[i]
		if path.Source == best.Source {
			continue
		}
		if !equalCostPath(path, best) {
			continue
		}
		hasPrimaryTarget := path.NextHop.IsValid() || path.Interface != ""
		if hasPrimaryTarget {
			out = appendForwardingTarget(out, path, NextHop{Addr: path.NextHop, Interface: path.Interface, OnLink: path.OnLink, Weight: path.Weight})
		}
		for _, target := range path.ECMP {
			out = appendForwardingTarget(out, path, target)
		}
	}
	return out
}

func appendForwardingTarget(members []ForwardingMember, path *Path, target NextHop) []ForwardingMember {
	member := ForwardingMember{Source: path.Source, ECMPMember: ECMPMember{
		NextHop: target, Labels: path.Labels, SRv6SID: path.SRv6SID,
	}}
	if containsForwardingTarget(members, &member) {
		return members
	}
	return append(members, member)
}

// equalCostMember selects a primary sibling target for producer observers.
// Path admission/rank does not require a primary: a forwarding contributor may
// instead carry every usable target in its explicit ECMP set.
func equalCostMember(path, best *Path) bool {
	if !equalCostPath(path, best) {
		return false
	}
	if !path.NextHop.IsValid() {
		if path.Interface == "" {
			return false
		}
	}
	if path.Source == best.Source {
		if path.NextHop == best.NextHop {
			if path.Interface == best.Interface {
				return false
			}
		}
	}
	return true
}

// equalCostPath is the shared path-level policy and rank rule, independent of
// whether the path supplies a primary target, explicit members, or neither.
func equalCostPath(path, best *Path) bool {
	if !path.Valid() {
		return false
	}
	if path.Hidden {
		return false
	}
	if path.AdminDistance != best.AdminDistance {
		return false
	}
	return path.Metric == best.Metric
}

func containsForwardingTarget(members []ForwardingMember, target *ForwardingMember) bool {
	for i := range members {
		if members[i].Source != target.Source {
			continue
		}
		if members[i].NextHop == target.NextHop {
			return true
		}
	}
	return false
}

func equalForwardingMemberSets(a, b []ForwardingMember) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !containsForwardingMember(b, &a[i]) {
			return false
		}
	}
	return true
}

func containsForwardingMember(members []ForwardingMember, target *ForwardingMember) bool {
	for i := range members {
		member := &members[i]
		if member.Source != target.Source {
			continue
		}
		if member.NextHop != target.NextHop {
			continue
		}
		if member.SRv6SID != target.SRv6SID {
			continue
		}
		if slices.Equal(member.Labels, target.Labels) {
			return true
		}
	}
	return false
}
