//go:build ze_bgp

package cli

import "testing"

// TestSweepRendersHomeCPE validates the same rendered CPE configuration that
// the topology lab boots, and proves that bypassing rendering exposes its
// invalid placeholder values rather than dropping the config from the sweep.
func TestSweepRendersHomeCPE(t *testing.T) {
	keep := sweepKeep{"test/interop-topology/cpe/ze-cpe.conf"}
	rendered := runSweep(t, ruleNone, keep)
	if rendered.zeConfigs != 1 {
		t.Fatalf("swept %d Ze configs, want the converted CPE", rendered.zeConfigs)
	}
	for _, finding := range rendered.findings {
		t.Error(finding)
	}
	unrendered := runSweep(t, rulePlaceholders, keep)
	if unrendered.zeConfigs != 1 || len(unrendered.findings) == 0 {
		t.Fatalf("without rendering: %d configs, %d findings; want one config and its invalid placeholders", unrendered.zeConfigs, len(unrendered.findings))
	}
}
