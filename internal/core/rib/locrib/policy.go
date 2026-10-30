// Design: docs/architecture/rib/unified-locrib.md -- the import policy gate on the Loc-RIB
// Related: internal/core/rib/policy/policy.go -- the compiled snapshot this applies
// Related: manager.go -- insert, which runs the gate on every path a source offers
// Related: distance.go -- Reselect, which re-runs the gate when a snapshot is published

package locrib

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// producerValues are a producer's own values of the Path fields an import
// chain may set. The Loc-RIB keeps them beside the values in effect, so a
// reload judges the path from what the producer offered, never from what an
// earlier chain made of it: a set action is never applied twice.
type producerValues struct {
	tag         uint32
	hasTag      bool
	distance    uint8
	hasDistance bool
}

// admit returns the Path the Loc-RIB stores for p, a path as its producer
// offered it, under snap: p unchanged and selectable when no chain is
// configured, p with the chains' set actions applied when they accept it, and p
// hidden when either chain rejects it. It records p's own values so a later
// Reselect judges from them.
//
// The nil check is the whole cost of the gate when no chain is configured: the
// BGP full-table insert path. admit allocates nothing.
func admit(snap *ribpolicy.Snapshot, fam family.Family, prefix netip.Prefix, p Path) Path { //nolint:gocritic // hugeParam: the Path is the value the caller stores, returned modified
	p.Hidden = false
	p.original = producerValues{
		tag:         p.Tag,
		hasTag:      p.HasTag,
		distance:    p.DistanceOverride,
		hasDistance: p.HasDistanceOverride,
	}
	// A nil snapshot is the no-policy state ribpolicy.Set publishes when no
	// chain is configured: every path is admitted unchanged.
	if snap == nil {
		return p
	}
	v := snap.Evaluate(routeView(fam, prefix, &p))
	if v.Action != ribpolicy.ActionAccept {
		p.Hidden = true
		return p
	}
	if v.Sets.HasDistance {
		p.DistanceOverride, p.HasDistanceOverride = v.Sets.Distance, true
	}
	if v.Sets.HasTag {
		p.Tag, p.HasTag = v.Sets.Tag, true
	}
	return p
}

// producerPath returns the stored path p with the producer's own values of
// every chain-settable field restored, as admit first received it.
func producerPath(p *Path) Path {
	q := *p
	q.Tag, q.HasTag = p.original.tag, p.original.hasTag
	q.DistanceOverride, q.HasDistanceOverride = p.original.distance, p.original.hasDistance
	q.Hidden = false
	return q
}

// routeView builds the read-only view a chain judges p through.
func routeView(fam family.Family, prefix netip.Prefix, p *Path) ribpolicy.Route {
	return ribpolicy.Route{
		Family:        fam,
		Prefix:        prefix,
		Source:        p.Source,
		IsEBGP:        p.IsEBGP,
		IsBGP:         p.IsBGP,
		NextHop:       p.NextHop,
		Interface:     p.Interface,
		Metric:        p.Metric,
		Tag:           p.Tag,
		HasTag:        p.HasTag,
		Level:         p.Level,
		OSPFRouteType: p.OSPFRouteType,
	}
}

// PublishPolicy publishes snap as the import policy and re-judges every path
// already stored in the process-wide Loc-RIB against it, hidden ones included,
// so a relaxed filter re-admits a path and a tightened one withdraws it with no
// producer event. A nil snap clears the policy. The config component that
// compiles the chains calls it on every configure and rollback, the way sysrib
// publishes a distance declaration and then calls Reselect.
//
// The caller MUST NOT hold a lock an OnChange subscriber takes: Reselect
// dispatches under the shard locks.
func PublishPolicy(snap *ribpolicy.Snapshot) {
	publishPolicy(snap, Default())
}

// publishPolicy is PublishPolicy for one RIB. A nil rib is a forked plugin
// process, which holds no Loc-RIB to re-judge (Default).
func publishPolicy(snap *ribpolicy.Snapshot, rib *RIB) {
	ribpolicy.Set(snap)
	if rib == nil {
		return
	}
	rib.Reselect()
}
