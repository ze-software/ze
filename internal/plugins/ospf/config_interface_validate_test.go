// VALIDATES: spec-ospf-14 AC-6/AC-7 -- RFC 2328 App C.3 requires the interface output cost
// and InfTransDelay (transmit-delay) to be greater than 0; validateConfig rejects an explicit
// 0 that previously flowed through the parser.
// PREVENTS: a silently-accepted cost 0 (black-hole metric) or transmit-delay 0 (LS age never
// advanced on flood) reaching the engine.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-C.3-1 positive -- an interface output cost of 1 (the smallest positive value) is accepted, as is an unset cost that the engine defaults later (validateConfig, config.go:886-889).
// RFC requirement: RFC2328-C.3-1 negative -- an explicitly configured interface output cost of 0 is rejected with ErrInterfaceCostZero, so a non-positive metric never reaches the engine (validateConfig, config.go:886-889).
func TestInterfaceCostAndTransmitDelayBoundary(t *testing.T) {
	mk := func(mut func(*interfaceConfig)) ospfConfig {
		ic := interfaceConfig{Name: "eth0", AreaID: types.BackboneArea, TransmitDelay: 1}
		mut(&ic)
		return ospfConfig{
			present:    true,
			RouterID:   ridOf("1.1.1.1"),
			Areas:      []areaConfig{{AreaID: types.BackboneArea, NSSATranslateRole: translateRoleCandidate}},
			Interfaces: []interfaceConfig{ic},
		}
	}

	// Interface output cost: RFC 2328 App C.3 / YANG range 1..65535.
	require.NoError(t, validateConfig(mk(func(ic *interfaceConfig) { ic.HasCost = true; ic.Cost = 1 })), "cost 1 is the last valid lower bound")
	require.ErrorIs(t, validateConfig(mk(func(ic *interfaceConfig) { ic.HasCost = true; ic.Cost = 0 })), ErrInterfaceCostZero, "explicit cost 0 is rejected")
	require.NoError(t, validateConfig(mk(func(_ *interfaceConfig) {})), "an unset cost (defaulted later) is accepted")

	// transmit-delay (InfTransDelay): RFC 2328 App C.3 / YANG range 1..3600.
	require.NoError(t, validateConfig(mk(func(ic *interfaceConfig) { ic.HasTransmitDelay = true; ic.TransmitDelay = 1 })), "transmit-delay 1 is the last valid lower bound")
	require.ErrorIs(t, validateConfig(mk(func(ic *interfaceConfig) { ic.HasTransmitDelay = true; ic.TransmitDelay = 0 })), ErrTransmitDelayZero, "explicit transmit-delay 0 is rejected")
}
