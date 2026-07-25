// VALIDATES: spec-ospf-13 AC-14 -- the config-sanity doctor checks flag a configured
// OSPF block with no derivable router-id (doctor-ospf-router-id-missing) and an enabled
// interface bound to an undeclared area (doctor-ospf-interface-area-unbound), and stay
// silent on a healthy config or when OSPF is not configured.
// PREVENTS: regressions where the doctor surfaces a spurious OSPF warning, or misses a
// missing router-id / dangling interface-area binding.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func diagCodes(cfg ospfConfig) []string {
	diags := ospfConfigDiagnostics(cfg)
	out := make([]string, 0, len(diags))
	for i := range diags {
		out = append(out, diags[i].Code)
	}
	return out
}

func TestOSPFDoctorConfigSanity(t *testing.T) {
	// Healthy config: router-id set, interface bound to a declared area -> no diagnostics.
	healthy := ospfConfig{
		present:    true,
		RouterID:   types.RouterID{10, 0, 0, 1},
		Areas:      []areaConfig{{AreaID: types.BackboneArea}},
		Interfaces: []interfaceConfig{{Name: "eth0", AreaID: types.BackboneArea}},
	}
	assert.Empty(t, diagCodes(healthy), "a healthy OSPF config produces no doctor diagnostics")

	// Missing router-id.
	noRID := healthy
	noRID.RouterID = types.RouterID{}
	assert.Contains(t, diagCodes(noRID), codeOSPFRouterIDMissing)

	// Interface bound to an undeclared area.
	unbound := ospfConfig{
		present:    true,
		RouterID:   types.RouterID{10, 0, 0, 1},
		Areas:      []areaConfig{{AreaID: types.BackboneArea}},
		Interfaces: []interfaceConfig{{Name: "eth9", AreaID: types.AreaID{1, 1, 1, 1}}},
	}
	assert.Contains(t, diagCodes(unbound), codeOSPFInterfaceAreaUnbound)

	// OSPF not present -> no diagnostics at all.
	assert.Empty(t, diagCodes(ospfConfig{}), "an absent OSPF config is silent")
}
