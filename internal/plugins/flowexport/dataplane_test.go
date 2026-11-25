// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- flow-export data-plane predicates
// Related: register.go -- samplingDataPlaneInUse and conntrackDataPlaneInUse predicates

package flowexport

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestFlowExportDataPlaneInUse validates both flow-export data-plane predicates.
// VALIDATES: AC-5, AC-15 -- flow-export sampling is in use when the sampling
// container is present, flow-export conntrack is in use iff conntrack enabled
// is true (supplying the SecondFix to turn it off), and counter-only export
// triggers neither predicate.
// PREVENTS: refusing counter-only flow export under VPP, or missing sampling
// or conntrack flow records under interface backend vpp.
func TestFlowExportDataPlaneInUse(t *testing.T) {
	t.Parallel()

	reg := registry.Lookup("flow-export")
	if reg == nil {
		t.Fatal("flow-export registration missing from registry")
	}
	byPath := make(map[string]registry.DataPlaneFeature, len(reg.DataPlaneFeatures))
	for _, feat := range reg.DataPlaneFeatures {
		byPath[feat.ConfigPath] = feat
	}

	sampling, ok := byPath["flow-export sampling"]
	if !ok || sampling.InUse == nil {
		t.Fatalf("flow-export sampling DataPlaneFeature missing or nil predicate: %+v", reg.DataPlaneFeatures)
	}
	conntrack, ok := byPath["flow-export conntrack"]
	if !ok || conntrack.InUse == nil {
		t.Fatalf("flow-export conntrack DataPlaneFeature missing or nil predicate: %+v", reg.DataPlaneFeatures)
	}

	counterOnly := map[string]any{
		"flow-export": map[string]any{
			"collector": map[string]any{
				"c1": map[string]any{"address": "192.0.2.10", "protocol": "ipfix"},
			},
		},
	}
	if sampling.InUse(counterOnly).InUse {
		t.Fatal("counter-only config should not mark sampling in use")
	}
	if conntrack.InUse(counterOnly).InUse {
		t.Fatal("counter-only config should not mark conntrack in use")
	}

	withSampling := map[string]any{
		"flow-export": map[string]any{
			"sampling": map[string]any{
				"interface": map[string]any{
					"eth0": map[string]any{"rate": "100"},
				},
			},
		},
	}
	if !sampling.InUse(withSampling).InUse {
		t.Fatal("sampling container should mark sampling in use")
	}
	if conntrack.InUse(withSampling).InUse {
		t.Fatal("sampling-only config should not mark conntrack in use")
	}

	conntrackOff := map[string]any{
		"flow-export": map[string]any{
			"conntrack": map[string]any{
				"enabled":        "false",
				"active-timeout": "30",
			},
		},
	}
	if conntrack.InUse(conntrackOff).InUse {
		t.Fatal("conntrack enabled false should not mark conntrack in use")
	}

	conntrackDefault := map[string]any{
		"flow-export": map[string]any{
			"conntrack": map[string]any{
				"active-timeout": "30",
			},
		},
	}
	if conntrack.InUse(conntrackDefault).InUse {
		t.Fatal("conntrack without enabled leaf (default false) should not mark conntrack in use")
	}

	conntrackOn := map[string]any{
		"flow-export": map[string]any{
			"conntrack": map[string]any{
				"enabled": "true",
			},
		},
	}
	got := conntrack.InUse(conntrackOn)
	if !got.InUse {
		t.Fatal("conntrack enabled true should mark conntrack in use")
	}
	if got.SecondFix != "set flow-export conntrack enabled false" {
		t.Fatalf("SecondFix = %q, want %q", got.SecondFix, "set flow-export conntrack enabled false")
	}
	if sampling.InUse(conntrackOn).InUse {
		t.Fatal("conntrack-only config should not mark sampling in use")
	}
}
