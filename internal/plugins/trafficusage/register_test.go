// VALIDATES: the traffic-usage plugin is registered with the engine entrypoint,
// the "interface" dependency (so the iface rate tracker runs), its config root,
// metrics binding, and embedded YANG.
// PREVENTS: an unreachable plugin; a missing interface dependency leaving the
// rate tracker (and thus the snapshot-driven lifecycle) dormant.

package trafficusage

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func TestTrafficUsageRegistration(t *testing.T) {
	reg := registry.Lookup("traffic-usage")
	if reg == nil {
		t.Fatal("traffic-usage plugin not registered")
	}
	if reg.Name != "traffic-usage" {
		t.Errorf("Name = %q, want traffic-usage", reg.Name)
	}
	if reg.RunEngine == nil {
		t.Error("RunEngine is nil")
	}
	if reg.CLIHandler == nil {
		t.Error("CLIHandler is nil (registration would fail)")
	}
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "traffic-usage" {
		t.Errorf("ConfigRoots = %v, want [traffic-usage]", reg.ConfigRoots)
	}
	if len(reg.Dependencies) != 1 || reg.Dependencies[0] != "interface" {
		t.Errorf("Dependencies = %v, want [interface]", reg.Dependencies)
	}
	if reg.ConfigureMetrics == nil {
		t.Error("ConfigureMetrics is nil")
	}
	if reg.YANG == "" {
		t.Error("YANG schema is empty")
	}
}
