//go:build ze_anomaly && ze_ddos && ze_flowexport && ze_trafficusage

package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestDetectionEnabledPullsNoFlowSource resolves anomaly and DDoS detection
// against their real registrations in a build that carries both flow sources.
//
// VALIDATES: a need naming more than one provider is a set of alternatives:
// `anomaly detect enabled true` and `ddos detect enabled true` hold, both
// flow-export and traffic-usage are compiled in, and resolution pulls
// neither. Anomaly detection then pulls no interface either, which only flow
// export's hard dependency brought in; DDoS detection keeps interface, its own
// hard dependency.
// PREVENTS: enabling detection starting every flow source, so that boot on a
// host with no interface backend is refused through flow export's interface
// dependency (the anomaly-show and anomaly-observe-show functional tests on
// darwin).
// METHOD: config.ConfigNeedHolds over a lowered tree with detection enabled,
// handed to registry.ResolveDependencies as config.ExpandDependencies and boot
// auto-load hand it.
func TestDetectionEnabledPullsNoFlowSource(t *testing.T) {
	for _, name := range []string{"anomaly-detect", "ddos-detect", "flow-export", "traffic-usage"} {
		if !registry.Has(name) {
			t.Fatalf("composition root did not register %s", name)
		}
	}

	cases := []struct {
		owner      string
		configTree map[string]any
		absent     []string
	}{
		{
			owner:      "anomaly-detect",
			configTree: map[string]any{"anomaly": map[string]any{"detect": map[string]any{"enabled": "true"}}},
			absent:     []string{"flow-export", "traffic-usage", "interface"},
		},
		{
			owner:      "ddos-detect",
			configTree: map[string]any{"ddos": map[string]any{"detect": map[string]any{"enabled": "true"}}},
			absent:     []string{"flow-export", "traffic-usage"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.owner, func(t *testing.T) {
			resolved, err := registry.ResolveDependencies([]string{tc.owner}, nil, config.ConfigNeedHolds(tc.configTree))
			if err != nil {
				t.Fatalf("ResolveDependencies: %v", err)
			}
			if !slices.Contains(resolved, tc.owner) {
				t.Fatalf("owner %s missing from %v", tc.owner, resolved)
			}
			for _, source := range tc.absent {
				if slices.Contains(resolved, source) {
					t.Errorf("detection enabled pulled %s: %v", source, resolved)
				}
			}
		})
	}
}
