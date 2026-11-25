// VALIDATES: data-plane declaration types, Register validation, deep cloning,
// standalone RegisterDataPlaneFeature, and Snapshot/Restore/Reset isolation.
// PREVENTS: malformed selectors or features reaching the config verdict, two
// primary data-plane selectors competing by map order, or caller mutations
// leaking into registry state.

package registry

import (
	"errors"
	"slices"
	"testing"
)

func testBackends() []string {
	return []string{"kernel", "vpp"}
}

func testInUsePredicate(map[string]any) DataPlaneFeatureUse {
	return DataPlaneFeatureUse{InUse: true}
}

// TestDataPlaneSelectorValidation checks valid selectors, invalid selector
// fields, and duplicate SelectsDataPlane refusal.
//
// VALIDATES: validateDataPlaneSelectorLocked enforces LeafPath, Roots,
// RegisteredBackends, and at most one SelectsDataPlane registration.
// PREVENTS: an incomplete selector or two primary selectors entering the registry.
func TestDataPlaneSelectorValidation(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Reset()

	primary := validReg("iface-test")
	primary.DataPlaneSelector = &DataPlaneSelector{
		LeafPath:           "/interface/backend",
		Roots:              []string{"interface"},
		SelectsDataPlane:   true,
		RegisteredBackends: testBackends,
	}
	if err := Register(primary); err != nil {
		t.Fatalf("valid primary selector refused: %v", err)
	}

	secondary := validReg("firewall-test")
	secondary.DataPlaneSelector = &DataPlaneSelector{
		LeafPath:           "/firewall/backend",
		Roots:              []string{"firewall"},
		SelectsDataPlane:   false,
		RegisteredBackends: testBackends,
	}
	if err := Register(secondary); err != nil {
		t.Fatalf("valid secondary selector refused: %v", err)
	}

	dupPrimary := validReg("dup-primary")
	dupPrimary.DataPlaneSelector = &DataPlaneSelector{
		LeafPath:           "/other/backend",
		Roots:              []string{"other"},
		SelectsDataPlane:   true,
		RegisteredBackends: testBackends,
	}
	if err := Register(dupPrimary); !errors.Is(err, ErrDuplicatePrimaryDataPlaneSelector) {
		t.Fatalf("duplicate primary selector = %v, want ErrDuplicatePrimaryDataPlaneSelector", err)
	}

	invalidCases := []struct {
		name string
		sel  *DataPlaneSelector
	}{
		{
			name: "empty leaf path",
			sel: &DataPlaneSelector{
				Roots:              []string{"r"},
				RegisteredBackends: testBackends,
			},
		},
		{
			name: "no roots",
			sel: &DataPlaneSelector{
				LeafPath:           "/r/backend",
				RegisteredBackends: testBackends,
			},
		},
		{
			name: "empty root element",
			sel: &DataPlaneSelector{
				LeafPath:           "/r/backend",
				Roots:              []string{"r", ""},
				RegisteredBackends: testBackends,
			},
		},
		{
			name: "nil registered backends",
			sel: &DataPlaneSelector{
				LeafPath: "/r/backend",
				Roots:    []string{"r"},
			},
		},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			reg := validReg("invalid-sel-" + tc.name)
			reg.DataPlaneSelector = tc.sel
			if err := Register(reg); !errors.Is(err, ErrInvalidDataPlaneDecl) {
				t.Fatalf("Register(%s) = %v, want ErrInvalidDataPlaneDecl", tc.name, err)
			}
		})
	}
}

// TestDataPlaneFeatureValidation checks every DataPlaneFeature rule on
// Registration, including Name population and HostSideDataPlane rules.
//
// VALIDATES: validateDataPlaneFeature, fillDataPlaneFeatureNames, and
// HostSideDataPlane validation inside Register.
// PREVENTS: ambiguous Planes/FollowsSelector, invalid InUseKind/InUse pairs,
// or meaningless HostSideDataPlane flags.
func TestDataPlaneFeatureValidation(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Reset()

	// Valid config-root feature fills Name from reg.Name without mutating input.
	byRoot := validReg("copp-test")
	byRoot.ConfigRoots = []string{"control-plane-protection"}
	inputFeatures := []DataPlaneFeature{{
		ConfigPath:      "control-plane-protection",
		Why:             "CoPP programs firewall tables",
		FollowsSelector: "firewall",
		InUseKind:       DataPlaneInUseConfigRoot,
	}}
	byRoot.DataPlaneFeatures = inputFeatures
	if err := Register(byRoot); err != nil {
		t.Fatalf("valid config-root feature refused: %v", err)
	}
	if inputFeatures[0].Name != "" {
		t.Fatalf("Register mutated caller input slice Name = %q, want empty", inputFeatures[0].Name)
	}
	stored := Lookup("copp-test")
	if len(stored.DataPlaneFeatures) != 1 || stored.DataPlaneFeatures[0].Name != "copp-test" {
		t.Fatalf("stored feature Name = %+v, want copp-test", stored.DataPlaneFeatures)
	}

	// Valid predicate feature + HostSideDataPlane.
	byPred := validReg("sysctl-test")
	byPred.HostSideDataPlane = true
	byPred.DataPlaneFeatures = []DataPlaneFeature{{
		ConfigPath: "sysctl",
		Why:        "sysctl sets Linux kernel tunables",
		Planes:     []string{"kernel"},
		InUseKind:  DataPlaneInUsePredicate,
		InUse:      testInUsePredicate,
	}}
	if err := Register(byPred); err != nil {
		t.Fatalf("valid predicate + HostSideDataPlane refused: %v", err)
	}

	// Valid selected-plugin feature.
	bySel := validReg("flowspec-fw-test")
	bySel.DataPlaneFeatures = []DataPlaneFeature{{
		ConfigPath:      "plugin internal flowspec-firewall",
		Why:             "FlowSpec firewall writes firewall tables",
		FollowsSelector: "firewall",
		InUseKind:       DataPlaneInUseSelectedPlugin,
	}}
	if err := Register(bySel); err != nil {
		t.Fatalf("valid selected-plugin feature refused: %v", err)
	}

	// Valid HostSideDataPlane with FIB DataPlane.
	fibHost := writerReg("fib-kernel-test", "kernel")
	fibHost.HostSideDataPlane = true
	if err := Register(fibHost); err != nil {
		t.Fatalf("valid FIB HostSideDataPlane refused: %v", err)
	}

	// Invalid HostSideDataPlane with neither DataPlane nor DataPlaneFeatures.
	badHost := validReg("bad-host-side")
	badHost.HostSideDataPlane = true
	if err := Register(badHost); !errors.Is(err, ErrInvalidDataPlaneDecl) {
		t.Fatalf("HostSideDataPlane without declaration = %v, want ErrInvalidDataPlaneDecl", err)
	}

	invalidFeatures := []struct {
		name  string
		roots []string
		feat  DataPlaneFeature
	}{
		{
			name: "empty config path",
			feat: DataPlaneFeature{
				Planes:    []string{"kernel"},
				InUseKind: DataPlaneInUseSelectedPlugin,
			},
		},
		{
			name: "neither planes nor follows selector",
			feat: DataPlaneFeature{
				ConfigPath: "foo",
				InUseKind:  DataPlaneInUseSelectedPlugin,
			},
		},
		{
			name: "both planes and follows selector",
			feat: DataPlaneFeature{
				ConfigPath:      "foo",
				Planes:          []string{"kernel"},
				FollowsSelector: "firewall",
				InUseKind:       DataPlaneInUseSelectedPlugin,
			},
		},
		{
			name: "empty plane name",
			feat: DataPlaneFeature{
				ConfigPath: "foo",
				Planes:     []string{"kernel", ""},
				InUseKind:  DataPlaneInUseSelectedPlugin,
			},
		},
		{
			name: "unspecified in-use kind",
			feat: DataPlaneFeature{
				ConfigPath: "foo",
				Planes:     []string{"kernel"},
			},
		},
		{
			name: "unknown in-use kind",
			feat: DataPlaneFeature{
				ConfigPath: "foo",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUseKind(99),
			},
		},
		{
			name:  "config root without owner ConfigRoots",
			roots: nil,
			feat: DataPlaneFeature{
				ConfigPath: "foo",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUseConfigRoot,
			},
		},
		{
			name:  "config root with non-nil InUse predicate",
			roots: []string{"foo"},
			feat: DataPlaneFeature{
				ConfigPath: "foo",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUseConfigRoot,
				InUse:      testInUsePredicate,
			},
		},
		{
			name: "predicate with nil InUse",
			feat: DataPlaneFeature{
				ConfigPath: "foo",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUsePredicate,
			},
		},
		{
			name: "selected plugin with non-nil InUse",
			feat: DataPlaneFeature{
				ConfigPath: "foo",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUseSelectedPlugin,
				InUse:      testInUsePredicate,
			},
		},
	}
	for _, tc := range invalidFeatures {
		t.Run(tc.name, func(t *testing.T) {
			reg := validReg("bad-feat-" + tc.name)
			reg.ConfigRoots = tc.roots
			reg.DataPlaneFeatures = []DataPlaneFeature{tc.feat}
			if err := Register(reg); !errors.Is(err, ErrInvalidDataPlaneDecl) {
				t.Fatalf("Register(%s) = %v, want ErrInvalidDataPlaneDecl", tc.name, err)
			}
		})
	}
}

// TestStandaloneDataPlaneFeatures covers RegisterDataPlaneFeature validation,
// duplicate detection, cloning, sealing, and Snapshot/Restore/Reset.
//
// VALIDATES: RegisterDataPlaneFeature, StandaloneDataPlaneFeatures, Snapshot,
// Restore, Reset, and Seal enforcement for standalone declarations.
// PREVENTS: standalone registrations leaking across tests, mutating registry
// state through returned slices, or bypassing the post-init seal.
func TestStandaloneDataPlaneFeatures(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Reset()

	l2tp := DataPlaneFeature{
		Name:       "l2tp",
		ConfigPath: "environment l2tp server",
		Why:        "L2TP programs kernel sessions only",
		Planes:     []string{"kernel"},
		InUseKind:  DataPlaneInUsePredicate,
		InUse:      testInUsePredicate,
	}
	if err := RegisterDataPlaneFeature(l2tp); err != nil {
		t.Fatalf("RegisterDataPlaneFeature(l2tp) = %v, want nil", err)
	}
	// Mutating caller's slice must not affect stored declaration.
	l2tp.Planes[0] = "mutated"

	gtsm := DataPlaneFeature{
		Name:       "gtsm",
		ConfigPath: "bgp peer ttl",
		Why:        "GTSM programs kernel routes and nftables only",
		Planes:     []string{"kernel"},
		InUseKind:  DataPlaneInUsePredicate,
		InUse:      testInUsePredicate,
	}
	if err := RegisterDataPlaneFeature(gtsm); err != nil {
		t.Fatalf("RegisterDataPlaneFeature(gtsm) = %v, want nil", err)
	}

	// Duplicate (Name, ConfigPath) is refused.
	if err := RegisterDataPlaneFeature(gtsm); !errors.Is(err, ErrDuplicateStandaloneDataPlaneFeature) {
		t.Fatalf("duplicate standalone feature = %v, want ErrDuplicateStandaloneDataPlaneFeature", err)
	}

	// Different ConfigPath under the same Name is allowed.
	gtsmGroup := gtsm
	gtsmGroup.ConfigPath = "bgp group ttl"
	if err := RegisterDataPlaneFeature(gtsmGroup); err != nil {
		t.Fatalf("distinct ConfigPath under same Name refused: %v", err)
	}

	// Sorted order and clone isolation on read.
	got := StandaloneDataPlaneFeatures()
	if len(got) != 3 {
		t.Fatalf("StandaloneDataPlaneFeatures() len = %d, want 3", len(got))
	}
	if got[0].Name != "gtsm" || got[0].ConfigPath != "bgp group ttl" ||
		got[1].Name != "gtsm" || got[1].ConfigPath != "bgp peer ttl" ||
		got[2].Name != "l2tp" || !slices.Equal(got[2].Planes, []string{"kernel"}) {
		t.Fatalf("unexpected standalone features: %+v", got)
	}
	got[2].Planes[0] = "caller-write"
	if after := StandaloneDataPlaneFeatures(); !slices.Equal(after[2].Planes, []string{"kernel"}) {
		t.Fatalf("mutating returned slice changed registry: %v", after[2].Planes)
	}

	// Snapshot / Reset / Restore preserves standalone features cleanly.
	withThree := Snapshot()
	Reset()
	if len(StandaloneDataPlaneFeatures()) != 0 {
		t.Fatalf("Reset did not clear standalone features")
	}
	Restore(withThree)
	if len(StandaloneDataPlaneFeatures()) != 3 {
		t.Fatalf("Restore did not recover standalone features")
	}

	// Validation errors for standalone features.
	badStandalone := []struct {
		name string
		decl DataPlaneFeature
	}{
		{
			name: "empty name",
			decl: DataPlaneFeature{
				ConfigPath: "x",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUsePredicate,
				InUse:      testInUsePredicate,
			},
		},
		{
			name: "config root kind not allowed standalone",
			decl: DataPlaneFeature{
				Name:       "sub",
				ConfigPath: "x",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUseConfigRoot,
			},
		},
		{
			name: "selected plugin kind not allowed standalone",
			decl: DataPlaneFeature{
				Name:       "sub",
				ConfigPath: "x",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUseSelectedPlugin,
			},
		},
		{
			name: "nil predicate",
			decl: DataPlaneFeature{
				Name:       "sub",
				ConfigPath: "x",
				Planes:     []string{"kernel"},
				InUseKind:  DataPlaneInUsePredicate,
			},
		},
	}
	for _, tc := range badStandalone {
		t.Run(tc.name, func(t *testing.T) {
			if err := RegisterDataPlaneFeature(tc.decl); !errors.Is(err, ErrInvalidDataPlaneDecl) {
				t.Fatalf("RegisterDataPlaneFeature(%s) = %v, want ErrInvalidDataPlaneDecl", tc.name, err)
			}
		})
	}

	// Seal refuses further standalone registrations.
	Seal()
	err := RegisterDataPlaneFeature(DataPlaneFeature{
		Name:       "after-seal",
		ConfigPath: "after-seal path",
		Planes:     []string{"kernel"},
		InUseKind:  DataPlaneInUsePredicate,
		InUse:      testInUsePredicate,
	})
	if !errors.Is(err, ErrRegistrySealed) {
		t.Fatalf("RegisterDataPlaneFeature after Seal = %v, want ErrRegistrySealed", err)
	}
}

// TestRegisterRefusesNoPlaneMarkWithoutProgramsFIB verifies that
// Registration.NoDataPlaneChoice is refused unless ProgramsFIB is set,
// DataPlane is empty, and HostSideDataPlane is false.
//
// VALIDATES: validateDataPlaneDecl enforcement of NoDataPlaneChoice invariants.
// PREVENTS: a non-FIB plugin or a plane-declaring writer carrying the no-plane
// choice mark.
func TestRegisterRefusesNoPlaneMarkWithoutProgramsFIB(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Reset()

	// Refused without ProgramsFIB.
	noFIB := validReg("no-fib-no-plane")
	noFIB.NoDataPlaneChoice = true
	if err := Register(noFIB); !errors.Is(err, ErrInvalidDataPlaneDecl) {
		t.Fatalf("Register(NoDataPlaneChoice without ProgramsFIB) = %v, want ErrInvalidDataPlaneDecl", err)
	}

	// Refused with non-empty DataPlane.
	withPlane := writerReg("plane-and-no-plane", "kernel")
	withPlane.NoDataPlaneChoice = true
	if err := Register(withPlane); !errors.Is(err, ErrInvalidDataPlaneDecl) {
		t.Fatalf("Register(NoDataPlaneChoice with DataPlane) = %v, want ErrInvalidDataPlaneDecl", err)
	}

	// Refused with HostSideDataPlane (even if DataPlaneFeatures is present).
	withHost := validReg("host-and-no-plane")
	withHost.ProgramsFIB = true
	withHost.NoDataPlaneChoice = true
	withHost.HostSideDataPlane = true
	withHost.DataPlaneFeatures = []DataPlaneFeature{{
		ConfigPath: "x",
		Planes:     []string{"kernel"},
		InUseKind:  DataPlaneInUseSelectedPlugin,
	}}
	if err := Register(withHost); !errors.Is(err, ErrInvalidDataPlaneDecl) {
		t.Fatalf("Register(NoDataPlaneChoice with HostSideDataPlane) = %v, want ErrInvalidDataPlaneDecl", err)
	}

	// Accepted with ProgramsFIB and empty DataPlane.
	valid := validReg("fib-noop-choice")
	valid.ProgramsFIB = true
	valid.NoDataPlaneChoice = true
	if err := Register(valid); err != nil {
		t.Fatalf("Register(valid NoDataPlaneChoice) = %v, want nil", err)
	}
	stored := Lookup("fib-noop-choice")
	if stored == nil || !stored.NoDataPlaneChoice || !stored.ProgramsFIB || stored.DataPlane != "" {
		t.Fatalf("stored registration = %+v, want ProgramsFIB=true NoDataPlaneChoice=true DataPlane=\"\"", stored)
	}
}
