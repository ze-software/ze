package ospf

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13.3-2 positive -- an OSPFv2 interface configured with
// InfTransDelay (transmit-delay) 1, the smallest value above 0, is accepted by validateConfig,
// so the increment floodCopy adds is always a positive value the operator chose.
// RFC requirement: RFC2328-13.3-2 negative -- an OSPFv2 interface configured with
// transmit-delay 0 is rejected with ErrTransmitDelayZero, so an InfTransDelay that is not
// "> 0" never reaches the flooding engine (validateConfigAF over the OSPFv2 interfaces).
//
// Goal: prove the "(which must be > 0)" clause of RFC 2328 Section 13.3 on the OSPFv2 family.
// Method: validate one OSPFv2 config per case, each with a single backbone interface, and
// check the accept of 1 and the refusal of 0 independently.
func TestRFC2328TransmitDelayMustBePositive(t *testing.T) {
	mk := func(delay uint16) ospfConfig {
		return ospfConfig{
			present:  true,
			RouterID: ridOf("10.0.0.1"),
			Areas:    []areaConfig{{AreaID: types.BackboneArea, NSSATranslateRole: translateRoleCandidate}},
			Interfaces: []interfaceConfig{{
				Name: "eth0", AreaID: types.BackboneArea,
				HasTransmitDelay: true, TransmitDelay: delay,
			}},
		}
	}

	require.NoError(t, validateConfig(mk(1)), "an OSPFv2 InfTransDelay of 1 is the smallest valid value")
	require.ErrorIs(t, validateConfig(mk(0)), ErrTransmitDelayZero, "an OSPFv2 InfTransDelay of 0 must be rejected")
}
