// Design: docs/architecture/rib/unified-locrib.md -- the import policy gate on the Loc-RIB
// Related: internal/core/rib/locrib/policy.go -- admit, the one consumer of the snapshot
// Related: internal/core/rib/distance/distance.go -- the seam shape this one follows

// Package policy carries the compiled route import policy from the config
// component that compiles it to the Loc-RIB, which applies it on every insert.
//
// The Loc-RIB evaluates two chains for every path a source offers: the chain
// configured for that source (`isis`, `ospf`, `static`, `connected`
// `filter import`), then the central chain (`rib filter import`), which every
// source passes, BGP's best included. A path either chain rejects is kept in
// the Loc-RIB as a hidden path: never selected, never programmed, and judged
// again whenever a new snapshot is published, so relaxing a filter re-admits
// the path without any producer re-sending it.
//
// WHY A SEAM RATHER THAN A DIRECT CALL. locrib sits in internal/core while the
// policy configuration lives in a component above it. A leaf package neither
// side owns lets the compiled chains reach the Loc-RIB without inverting the
// dependency, the same shape internal/core/rib/distance uses for the declared
// administrative distances.
//
// THE NO-POLICY STATE IS A NIL SNAPSHOT, AND THAT NIL IS A GUARD. Set stores
// nil for a snapshot that holds no chain, so the insert of a path when no chain
// is configured costs a pointer load and a nil check. That is the BGP
// full-table hot path.
package policy

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// ISISLevel is the IS-IS level of a route IS-IS has already selected. IS-IS
// resolves level preference inside SPF, so a filter that reads the level never
// changes which level IS-IS prefers.
type ISISLevel uint8

const (
	// ISISLevelUnspecified means the route carries no level: it is not an
	// IS-IS route, or its producer did not state one.
	ISISLevelUnspecified ISISLevel = 0
	ISISLevel1           ISISLevel = 1
	ISISLevel2           ISISLevel = 2
)

// OSPFRouteType is the OSPF path type of a route OSPF has already selected:
// the RFC 2328 Section 11 path types, plus the two NSSA external types of
// RFC 3101.
type OSPFRouteType uint8

const (
	// OSPFRouteTypeUnspecified means the route carries no OSPF path type: it is
	// not an OSPF route, or its producer did not state one.
	OSPFRouteTypeUnspecified OSPFRouteType = 0
	OSPFIntraArea            OSPFRouteType = 1
	OSPFInterArea            OSPFRouteType = 2
	OSPFExternal1            OSPFRouteType = 3
	OSPFExternal2            OSPFRouteType = 4
	OSPFNSSA1                OSPFRouteType = 5
	OSPFNSSA2                OSPFRouteType = 6
)

// Route is the read-only view of one path a chain judges: the (family, prefix)
// the path is offered for, and the path's own fields a filter may read. It is a
// value, copied for each evaluation, so a chain cannot change the stored path.
type Route struct {
	Family family.Family
	Prefix netip.Prefix
	Source redistevents.ProtocolID

	// IsEBGP and IsBGP give a BGP path's class, which the field vocabulary
	// names as protocol `ebgp` or `ibgp`.
	IsEBGP bool
	IsBGP  bool

	NextHop   netip.Addr
	Interface string
	Metric    uint32

	// Tag is meaningful only when HasTag is set, because 0 is a real tag.
	Tag    uint32
	HasTag bool

	Level         ISISLevel
	OSPFRouteType OSPFRouteType
}

// Action is a chain's verdict on a route. The zero value is ActionUnspecified,
// which Snapshot.Evaluate treats as a reject: a chain that cannot answer fails
// closed rather than installing the route.
type Action uint8

const (
	ActionUnspecified Action = 0
	ActionAccept      Action = 1
	ActionReject      Action = 2
)

// Sets are the set actions an accepting chain applies to the route it admits.
// Each value is meaningful only when its Has flag is set, because 0 is a real
// distance and a real tag. The Loc-RIB applies them to a copy of the producer's
// own values on every evaluation, never on top of an earlier evaluation's
// result.
type Sets struct {
	// Distance becomes the path's DistanceOverride: it wins over the distance
	// declared for the path's protocol.
	Distance    uint8
	HasDistance bool

	// Tag replaces the path's tag.
	Tag    uint32
	HasTag bool

	// Metric replaces the metric an IGP export advertises: the IS-IS prefix
	// metric, the OSPF AS-external metric. Only an export attachment allows it.
	Metric    uint32
	HasMetric bool

	// MetricType selects the OSPF external metric type. Only the OSPF export
	// attachment allows it; ExternalMetricTypeUnspecified means no filter set it.
	MetricType ExternalMetricType
}

// ExternalMetricType is the OSPF external metric type a filter sets (RFC 2328
// Section 2.3: a Type 1 metric is in the units of the link state metric, a
// Type 2 metric is larger than any path internal to the AS). The zero value
// is Unspecified: no filter set it, and the consumer keeps its default.
type ExternalMetricType uint8

// The external metric types a filter can set.
const (
	ExternalMetricTypeUnspecified ExternalMetricType = 0
	ExternalMetricType1           ExternalMetricType = 1
	ExternalMetricType2           ExternalMetricType = 2
)

// ChainRole names which of a snapshot's two chains judged a route: the chain of
// the route's own source, or the central chain every source passes. The zero
// value is ChainRoleUnspecified: no chain rejected the route.
type ChainRole uint8

const (
	ChainRoleUnspecified ChainRole = 0
	ChainRoleSource      ChainRole = 1
	ChainRoleCentral     ChainRole = 2
	// ChainRoleCount sizes an array indexed by ChainRole.
	ChainRoleCount = 3
)

// String names the role as `show rib candidates` and the
// ze_rib_policy_rejected_total chain label spell it.
func (c ChainRole) String() string {
	switch c {
	case ChainRoleSource:
		return "source"
	case ChainRoleCentral:
		return "central"
	case ChainRoleUnspecified:
		return "none"
	}
	return "unknown"
}

// Verdict is what a chain answers for one route. Sets are read only when
// Action is ActionAccept. RejectedBy is set by Snapshot.Evaluate on a reject
// and names the chain that rejected; a single Chain leaves it unspecified.
type Verdict struct {
	Action     Action
	Sets       Sets
	RejectedBy ChainRole
}

// Chain is one compiled import chain: the filters of one attachment, in order,
// and its `final`. The config component compiles each attachment into one.
//
// Evaluate MUST be safe for concurrent use: inserts on different shards call it
// at once. It MUST NOT block and MUST NOT call into the Loc-RIB, because the
// Loc-RIB may call it while it holds a shard lock (on a publish race, and in
// Reselect). It SHOULD NOT allocate: it runs on every insert from a source it
// covers.
type Chain interface {
	Evaluate(r Route) Verdict
}

// Snapshot is an immutable set of compiled chains: one optional chain per
// source and one optional central chain. It is published whole through Set, so
// a reader never sees one source's new chain beside another source's old one.
type Snapshot struct {
	// bySource is indexed by ProtocolID; a nil slot means the source has no
	// chain. A slice beats a map here: the lookup runs on every insert.
	bySource []Chain
	central  Chain
}

// errUnspecifiedSource is NewSnapshot's refusal of a chain keyed on no source.
var errUnspecifiedSource = snapshotError("route import policy: a chain is keyed on the unspecified protocol, which no path ever carries (a policy compiler defect: every chain names a registered source)")

type snapshotError string

func (e snapshotError) Error() string { return string(e) }

// NewSnapshot builds a snapshot from the per-source chains and the central
// chain. A nil chain, for a source or for central, means none is configured.
// The map is copied, so the caller keeps ownership of it.
//
// It refuses a chain keyed on ProtocolUnspecified: no valid path carries that
// source, so such a chain could only be a compiler defect, and silently
// dropping it would hide one.
func NewSnapshot(sources map[redistevents.ProtocolID]Chain, central Chain) (*Snapshot, error) {
	var slots int
	for id, chain := range sources {
		if id == redistevents.ProtocolUnspecified {
			return nil, errUnspecifiedSource
		}
		if chain == nil {
			continue
		}
		slots = max(slots, int(id)+1)
	}
	s := &Snapshot{central: central}
	if slots > 0 {
		s.bySource = make([]Chain, slots)
	}
	for id, chain := range sources {
		if chain == nil {
			continue
		}
		s.bySource[id] = chain
	}
	return s, nil
}

// empty reports whether the snapshot holds no chain at all.
func (s *Snapshot) empty() bool {
	if s.central != nil {
		return false
	}
	for _, chain := range s.bySource {
		if chain != nil {
			return false
		}
	}
	return true
}

// Evaluate judges r: the chain of r's source first, then the central chain.
// A reject from either, or an answer that is not an accept, rejects the route.
// The central chain sees the route as the source chain's set actions left it,
// and its own set actions win where both set the same field.
//
// Safe for concurrent use: a Snapshot never changes after NewSnapshot.
func (s *Snapshot) Evaluate(r Route) Verdict {
	var sets Sets
	if int(r.Source) < len(s.bySource) {
		if chain := s.bySource[r.Source]; chain != nil {
			v := chain.Evaluate(r)
			if v.Action != ActionAccept {
				return Verdict{Action: ActionReject, RejectedBy: ChainRoleSource}
			}
			sets = v.Sets
			if sets.HasTag {
				r.Tag, r.HasTag = sets.Tag, true
			}
		}
	}
	if s.central != nil {
		v := s.central.Evaluate(r)
		if v.Action != ActionAccept {
			return Verdict{Action: ActionReject, RejectedBy: ChainRoleCentral}
		}
		sets = sets.Override(v.Sets)
	}
	return Verdict{Action: ActionAccept, Sets: sets}
}

// Override returns first with every field later sets overriding it. A
// chain applies each accepting filter's sets this way, and a snapshot applies
// the central chain's sets over the source chain's.
func (first Sets) Override(later Sets) Sets {
	if later.HasDistance {
		first.Distance, first.HasDistance = later.Distance, true
	}
	if later.HasTag {
		first.Tag, first.HasTag = later.Tag, true
	}
	if later.HasMetric {
		first.Metric, first.HasMetric = later.Metric, true
	}
	if later.MetricType != ExternalMetricTypeUnspecified {
		first.MetricType = later.MetricType
	}
	return first
}

// current holds the published snapshot. Read on every Loc-RIB insert and
// written once per configure, so an atomic pointer beats a mutex here.
var current atomic.Pointer[Snapshot]

// Set publishes s as the import policy the Loc-RIB applies. A nil s, or one
// holding no chain, clears the policy: every path is then admitted unchanged.
//
// Set alone re-judges no stored path. A publisher calls locrib.PublishPolicy,
// which calls Set and then re-runs selection over the paths already stored.
//
// The seam is process-global, and so is the Loc-RIB that reads it.
func Set(s *Snapshot) {
	if s == nil {
		current.Store(nil)
		return
	}
	if s.empty() {
		current.Store(nil)
		return
	}
	current.Store(s)
}

// Current returns the published snapshot, or nil when no chain is configured.
// A nil answer is the no-policy state, not a failure.
func Current() *Snapshot {
	return current.Load()
}
