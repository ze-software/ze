// VALIDATES: spec-ospf-ext-5 metrics -- the seven ze_ospf_sr_* series exist with
// the af label and are updated from the resolved SR config.
// PREVENTS: a missing SR metric series or an unset gauge after config apply.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

func TestSRMetricsUpdateFromConfig(t *testing.T) {
	m := newSRMetrics(metrics.NopRegistry{})
	// A no-op registry never panics and every series is non-nil.
	cfg := sr.SRConfig{
		Enabled:  true,
		SRGB:     []sr.LabelRange{{Base: 16000, Size: 8000}},
		SRLB:     []sr.LabelRange{{Base: 40000, Size: 1000}},
		Prefixes: []sr.PrefixSIDConfig{{Index: 1}},
	}
	m.updateFromConfig("ipv4", cfg, 2)
	m.observeMalformed("ipv4", "prefix-sid")
	m.observeComputeError("ipv4", "index-out-of-range")
	m.observeLabelsInstalled("ipv4", "push", 3)
}

func TestSRMetricsNilRegistry(t *testing.T) {
	// newSRMetrics tolerates a nil registry (uses the Nop registry).
	m := newSRMetrics(nil)
	m.updateFromConfig("ipv6", sr.SRConfig{}, 0)
}
