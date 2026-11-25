// Design: docs/architecture/core-design.md -- N-1 selector value rename refusal proof
// Related: dataplane_verdict.go -- unknownBackendFinding and coveredByUnmetConfigNeed

package config

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestDataPlaneVerdictRefusesOldSelectorValues proves AC-19 and U-4:
// interface backend netlink, firewall backend nft, and traffic control backend
// tc are each refused and joined in one answer, with each refusal naming its
// registered backends and ending with `set <selector> kernel` read from the
// schema default; a value covered by an unmet ConfigNeed (vpp in a build
// without ze_vpp) is left to refuseAbsentOptionalProviders instead.
//
// VALIDATES: AC-19 refusal of retired selector values from the generic
// unknown-backend rule, and U-4 preservation of ConfigNeed build-tag refusals.
// PREVENTS: retired selector names being accepted, losing their schema-derived
// fix line, or shadowing ConfigNeed build-tag refusals.
func TestDataPlaneVerdictRefusesOldSelectorValues(t *testing.T) {
	isolateRegistry(t)
	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, []string{"kernel", "vpp"})

	schema := buildVerdictTestSchema("kernel", "kernel", "kernel")
	tree := mustParseWithSchema(t, schema, `
interface { backend netlink; }
firewall { backend nft; }
traffic { control { backend tc; } }
`)

	refusal := refuseDataPlaneMismatches(tree, schema, nil)
	if refusal == nil {
		t.Fatal("refuseDataPlaneMismatches = nil, want joined refusal for all three selectors")
	}

	msg := refusal.Error()
	wantBlocks := []string{
		"interface backend \"netlink\" names no backend this build registers\nthe backends are kernel, vpp\nfix: set interface backend kernel",
		"firewall backend \"nft\" names no backend this build registers\nthe backends are kernel, vpp\nfix: set firewall backend kernel",
		"traffic control backend \"tc\" names no backend this build registers\nthe backends are kernel, vpp\nfix: set traffic control backend kernel",
	}
	for _, want := range wantBlocks {
		if !strings.Contains(msg, want) {
			t.Errorf("joined refusal missing expected block:\nwant:\n%s\ngot:\n%s", want, msg)
		}
	}

	// U-4: in a build without ze_vpp, a ConfigNeed covers `interface backend vpp`
	// so the verdict leaves it to refuseAbsentOptionalProviders, while still
	// refusing an unknown retired value (`netlink`).
	isolateRegistry(t)
	if err := registry.Register(registry.Registration{
		Name:                 "interface",
		Description:          "test interface selector without vpp",
		ConfigRoots:          []string{"interface"},
		OptionalDependencies: []string{"vpp"},
		ConfigNeeds: []registry.ConfigNeed{{
			Path:      []string{"interface", "backend"},
			Value:     "vpp",
			Providers: []string{"vpp"},
		}},
		DataPlaneSelector: &registry.DataPlaneSelector{
			LeafPath:           "/interface/backend",
			Roots:              []string{"interface"},
			SelectsDataPlane:   true,
			RegisteredBackends: func() []string { return []string{"kernel"} },
		},
		RunEngine:  func(net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
	}); err != nil {
		t.Fatalf("register interface without vpp: %v", err)
	}

	vppTree := mustParseWithSchema(t, schema, `interface { backend vpp; }`)
	if findings := evaluateDataPlaneVerdict(vppTree, schema, nil); len(findings) != 0 {
		t.Errorf("value covered by unmet ConfigNeed produced verdict findings: %+v, want none", findings)
	}

	netlinkTree := mustParseWithSchema(t, schema, `interface { backend netlink; }`)
	netlinkRefusal := refuseDataPlaneMismatches(netlinkTree, schema, nil)
	if netlinkRefusal == nil {
		t.Fatal("netlinkTree refused nothing in kernel-only build, want AC-19 refusal")
	}
	if !strings.Contains(netlinkRefusal.Error(), "fix: set interface backend kernel") {
		t.Errorf("netlinkTree refusal missing schema-derived fix:\n%s", netlinkRefusal)
	}
}
