// Design: docs/architecture/plugin/plugin-system.md -- registration fields, data-plane declarations
// Related: registry.go -- Registration fields and Register validation calling validateDataPlaneDecl
// Related: registration_clone.go -- deepCopyInto used to isolate stored and returned declarations

package registry

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
)

// DataPlaneInUseKind states how the config verdict decides whether a
// DataPlaneFeature is active in a candidate configuration. Zero is invalid so
// an omitted field is refused at registration.
type DataPlaneInUseKind uint8

const (
	// DataPlaneInUseUnspecified is the invalid zero value.
	DataPlaneInUseUnspecified DataPlaneInUseKind = iota
	// DataPlaneInUseConfigRoot marks a feature in use whenever any of the
	// owning Registration.ConfigRoots is present in the candidate tree.
	DataPlaneInUseConfigRoot
	// DataPlaneInUsePredicate marks a feature in use when its InUse predicate
	// reports DataPlaneFeatureUse.InUse == true over the candidate tree.
	DataPlaneInUsePredicate
	// DataPlaneInUseSelectedPlugin marks a feature in use when the candidate's
	// plugin selection selects the owning registration's Name.
	DataPlaneInUseSelectedPlugin
)

// DataPlaneSelector declares a component's backend selector leaf and the
// config roots that selection governs. Each component with pluggable data-plane
// backends declares one on its Registration; at most one sets SelectsDataPlane
// to mark the primary data-plane selector.
type DataPlaneSelector struct {
	// LeafPath is the YANG path of the selector leaf, such as
	// "/interface/backend".
	LeafPath string
	// Roots are the config roots this selector governs (e.g., ["interface"]).
	Roots []string
	// SelectsDataPlane marks the single primary selector whose chosen backend
	// is the deployment's data plane. At most one registration may set it.
	SelectsDataPlane bool
	// RegisteredBackends returns the backend names currently registered with
	// this component.
	RegisteredBackends func() []string
}

// DataPlaneFeatureUse is the answer returned by a DataPlaneFeature.InUse
// predicate for one candidate configuration tree.
type DataPlaneFeatureUse struct {
	// InUse reports whether the feature is active in the candidate tree.
	InUse bool
	// ConfigPath optionally overrides DataPlaneFeature.ConfigPath with a more
	// specific operator path, such as the BGP peer holding a nonzero ttl leaf.
	ConfigPath string
	// SecondFix optionally supplies an alternative fix command beside changing
	// the data-plane selector (for example, deleting the peer's ttl leaf).
	SecondFix string
}

// DataPlaneFeature declares a feature's forwarding-plane requirement and how
// the config verdict determines whether a candidate configuration uses it.
//
// Features owned by a registered plugin live on Registration.DataPlaneFeatures
// (where Name is filled from Registration.Name if left empty). Subsystems with
// no plugin Registration (such as L2TP and GTSM) register through
// RegisterDataPlaneFeature from init().
type DataPlaneFeature struct {
	// Name identifies the feature's owner. On Registration.DataPlaneFeatures it
	// is populated from Registration.Name when empty; standalone registrations
	// MUST set it.
	Name string
	// ConfigPath is the default operator config path named in refusals (for
	// example, "vpn ipsec").
	ConfigPath string
	// Why explains in plain English why the feature requires its declared plane
	// or selector (for example, "IPsec programs kernel XFRM only").
	Why string
	// Planes lists the data planes this feature supports (e.g., ["kernel"]).
	// Mutually exclusive with FollowsSelector.
	Planes []string
	// FollowsSelector names the plugin whose DataPlaneSelector governs this
	// feature (e.g., "firewall"). Mutually exclusive with Planes.
	FollowsSelector string
	// InUseKind specifies how the verdict determines whether the feature is in
	// use in a candidate configuration.
	InUseKind DataPlaneInUseKind
	// InUse evaluates whether the feature is active in a lowered candidate
	// tree. Required iff InUseKind == DataPlaneInUsePredicate.
	InUse func(tree map[string]any) DataPlaneFeatureUse
	// DisablePaths lists concrete or wildcard ("[*]") config tree paths to
	// remove from the in-memory start tree when this predicate feature
	// mismatches the active data plane (for example, "flow-export/sampling" or
	// "bgp/peer[*]/connection/ttl").
	DisablePaths []string
	// DisableLeaves maps concrete config tree leaf paths to explicit disabled
	// values (for example, "l2tp/enabled": "false") applied at daemon start
	// when deleting the whole container would break a sibling consumer on the
	// active data plane.
	DisableLeaves map[string]string
}

var (
	// ErrInvalidDataPlaneDecl is returned when a registration or standalone
	// call supplies a malformed DataPlaneSelector, DataPlaneFeature,
	// HostSideDataPlane, or NoDataPlaneChoice declaration.
	ErrInvalidDataPlaneDecl = errors.New("registry: invalid data-plane declaration")
	// ErrDuplicatePrimaryDataPlaneSelector is returned when more than one
	// registration sets DataPlaneSelector.SelectsDataPlane.
	ErrDuplicatePrimaryDataPlaneSelector = errors.New("registry: duplicate primary data-plane selector")
	// ErrDuplicateStandaloneDataPlaneFeature is returned when a standalone
	// DataPlaneFeature repeats an already registered (Name, ConfigPath) pair.
	ErrDuplicateStandaloneDataPlaneFeature = errors.New("registry: duplicate standalone data-plane feature")
)

// standaloneDataPlaneFeatures holds DataPlaneFeature declarations registered
// by subsystems that have no plugin Registration (such as L2TP and GTSM).
// Guarded by mu.
var standaloneDataPlaneFeatures []DataPlaneFeature

// RegisterDataPlaneFeature registers a standalone data-plane feature
// declaration for a subsystem that has no plugin Registration.
// MUST be called from init() functions only: after Seal it refuses with
// ErrRegistrySealed and changes nothing. Safe for concurrent use.
func RegisterDataPlaneFeature(decl DataPlaneFeature) error {
	mu.Lock()
	defer mu.Unlock()

	if err := refuseSealedLocked("dataplane feature "+strconv.Quote(decl.Name), decl.ConfigPath); err != nil {
		return err
	}
	if decl.Name == "" {
		return fmt.Errorf("%w: standalone feature has empty name", ErrInvalidDataPlaneDecl)
	}
	if err := validateDataPlaneFeature(decl.Name, decl, nil, true); err != nil {
		return err
	}
	for _, existing := range standaloneDataPlaneFeatures {
		if existing.Name == decl.Name && existing.ConfigPath == decl.ConfigPath {
			return fmt.Errorf("%w: %q (%q) is already registered",
				ErrDuplicateStandaloneDataPlaneFeature, decl.Name, decl.ConfigPath)
		}
	}
	standaloneDataPlaneFeatures = append(standaloneDataPlaneFeatures, cloneDataPlaneFeature(&decl))
	return nil
}

// StandaloneDataPlaneFeatures returns deep clones of all registered standalone
// data-plane feature declarations, sorted by Name then ConfigPath. Safe for
// concurrent use.
func StandaloneDataPlaneFeatures() []DataPlaneFeature {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]DataPlaneFeature, len(standaloneDataPlaneFeatures))
	for i := range standaloneDataPlaneFeatures {
		out[i] = cloneDataPlaneFeature(&standaloneDataPlaneFeatures[i])
	}
	slices.SortFunc(out, func(a, b DataPlaneFeature) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(a.ConfigPath, b.ConfigPath)
	})
	return out
}

// cloneDataPlaneFeature answers a copy of feat that shares no slice with it.
func cloneDataPlaneFeature(feat *DataPlaneFeature) DataPlaneFeature {
	var clone DataPlaneFeature
	deepCopyInto(reflect.ValueOf(&clone).Elem(), reflect.ValueOf(feat).Elem())
	return clone
}

// cloneStandaloneDataPlaneFeatures copies a standalone feature slice deeply.
func cloneStandaloneDataPlaneFeatures(src []DataPlaneFeature) []DataPlaneFeature {
	if len(src) == 0 {
		return nil
	}
	out := make([]DataPlaneFeature, len(src))
	for i := range src {
		out[i] = cloneDataPlaneFeature(&src[i])
	}
	return out
}

// validateDataPlaneDecl validates DataPlaneSelector, DataPlaneFeatures,
// HostSideDataPlane, and NoDataPlaneChoice on reg. Caller MUST hold mu.
func validateDataPlaneDecl(reg *Registration) error {
	if reg.DataPlaneSelector != nil {
		if err := validateDataPlaneSelectorLocked(reg.Name, reg.DataPlaneSelector); err != nil {
			return err
		}
	}
	for _, feat := range reg.DataPlaneFeatures {
		if err := validateDataPlaneFeature(reg.Name, feat, reg.ConfigRoots, false); err != nil {
			return err
		}
	}
	if reg.HostSideDataPlane && !hasDataPlaneDeclaration(reg) {
		return fmt.Errorf("%w: plugin %q sets HostSideDataPlane without DataPlane or DataPlaneFeatures",
			ErrInvalidDataPlaneDecl, reg.Name)
	}
	if reg.NoDataPlaneChoice {
		if !reg.ProgramsFIB {
			return fmt.Errorf("%w: plugin %q sets NoDataPlaneChoice without ProgramsFIB",
				ErrInvalidDataPlaneDecl, reg.Name)
		}
		if reg.DataPlane != "" {
			return fmt.Errorf("%w: plugin %q sets NoDataPlaneChoice with DataPlane %q",
				ErrInvalidDataPlaneDecl, reg.Name, reg.DataPlane)
		}
		if reg.HostSideDataPlane {
			return fmt.Errorf("%w: plugin %q cannot set both NoDataPlaneChoice and HostSideDataPlane",
				ErrInvalidDataPlaneDecl, reg.Name)
		}
	}
	return nil
}

// hasDataPlaneDeclaration reports whether reg declares a FIB DataPlane or at
// least one DataPlaneFeatures entry.
func hasDataPlaneDeclaration(reg *Registration) bool {
	if reg.DataPlane != "" {
		return true
	}
	return len(reg.DataPlaneFeatures) > 0
}

// validateDataPlaneSelectorLocked validates a component's DataPlaneSelector
// and checks that at most one registration sets SelectsDataPlane.
// Caller MUST hold mu.
func validateDataPlaneSelectorLocked(owner string, sel *DataPlaneSelector) error {
	if sel.LeafPath == "" {
		return fmt.Errorf("%w: plugin %q selector has empty LeafPath", ErrInvalidDataPlaneDecl, owner)
	}
	if len(sel.Roots) == 0 {
		return fmt.Errorf("%w: plugin %q selector %q has no Roots", ErrInvalidDataPlaneDecl, owner, sel.LeafPath)
	}
	if slices.Contains(sel.Roots, "") {
		return fmt.Errorf("%w: plugin %q selector %q has an empty root element", ErrInvalidDataPlaneDecl, owner, sel.LeafPath)
	}
	if sel.RegisteredBackends == nil {
		return fmt.Errorf("%w: plugin %q selector %q has nil RegisteredBackends", ErrInvalidDataPlaneDecl, owner, sel.LeafPath)
	}
	if sel.SelectsDataPlane {
		for _, other := range plugins {
			if other.DataPlaneSelector != nil && other.DataPlaneSelector.SelectsDataPlane {
				return fmt.Errorf("%w: already claimed by %q, cannot also be set by %q",
					ErrDuplicatePrimaryDataPlaneSelector, other.Name, owner)
			}
		}
	}
	return nil
}

// validateDataPlaneFeature validates one DataPlaneFeature declaration.
func validateDataPlaneFeature(owner string, feat DataPlaneFeature, ownerConfigRoots []string, standalone bool) error {
	if feat.ConfigPath == "" {
		return fmt.Errorf("%w: %q feature has empty ConfigPath", ErrInvalidDataPlaneDecl, owner)
	}
	hasPlanes := len(feat.Planes) > 0
	hasFollows := feat.FollowsSelector != ""
	if !hasPlanes && !hasFollows {
		return fmt.Errorf("%w: %q feature %q must set Planes or FollowsSelector",
			ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
	}
	if hasPlanes && hasFollows {
		return fmt.Errorf("%w: %q feature %q cannot set both Planes and FollowsSelector",
			ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
	}
	if hasPlanes && slices.Contains(feat.Planes, "") {
		return fmt.Errorf("%w: %q feature %q has an empty plane name",
			ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
	}
	if slices.Contains(feat.DisablePaths, "") {
		return fmt.Errorf("%w: %q feature %q has an empty DisablePaths entry",
			ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
	}
	if _, hasEmpty := feat.DisableLeaves[""]; hasEmpty {
		return fmt.Errorf("%w: %q feature %q has an empty DisableLeaves key",
			ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
	}
	switch feat.InUseKind {
	case DataPlaneInUseUnspecified:
		return fmt.Errorf("%w: %q feature %q has unspecified InUseKind",
			ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
	case DataPlaneInUseConfigRoot:
		if standalone {
			return fmt.Errorf("%w: standalone feature %q (%q) must use DataPlaneInUsePredicate",
				ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
		}
		if len(ownerConfigRoots) == 0 {
			return fmt.Errorf("%w: %q feature %q uses DataPlaneInUseConfigRoot without ConfigRoots",
				ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
		}
		if feat.InUse != nil {
			return fmt.Errorf("%w: %q feature %q sets InUse predicate with DataPlaneInUseConfigRoot",
				ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
		}
	case DataPlaneInUsePredicate:
		if feat.InUse == nil {
			return fmt.Errorf("%w: %q feature %q uses DataPlaneInUsePredicate with nil InUse",
				ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
		}
	case DataPlaneInUseSelectedPlugin:
		if standalone {
			return fmt.Errorf("%w: standalone feature %q (%q) must use DataPlaneInUsePredicate",
				ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
		}
		if feat.InUse != nil {
			return fmt.Errorf("%w: %q feature %q sets InUse predicate with DataPlaneInUseSelectedPlugin",
				ErrInvalidDataPlaneDecl, owner, feat.ConfigPath)
		}
	default:
		// Open set: callers may cast an out-of-range uint8.
		return fmt.Errorf("%w: %q feature %q has unknown InUseKind %d",
			ErrInvalidDataPlaneDecl, owner, feat.ConfigPath, feat.InUseKind)
	}
	return nil
}

// fillDataPlaneFeatureNames fills any empty DataPlaneFeature.Name on reg from
// reg.Name. MUST only be called on a cloned Registration before storing it.
func fillDataPlaneFeatureNames(reg *Registration) {
	for i := range reg.DataPlaneFeatures {
		if reg.DataPlaneFeatures[i].Name == "" {
			reg.DataPlaneFeatures[i].Name = reg.Name
		}
	}
}
