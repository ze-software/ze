// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- VPP NAT verify remains independent

//go:build ze_vpp

package firewallvpp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestVerifyConfigVPPNATBackendKeepsItsOwnPlacement(t *testing.T) {
	snat := vppNATPlacementSections("forward", "-200", `"snat":{"to":"192.0.2.10"}`)
	require.NoError(t, firewall.VerifyConfig(snat))

	// An accepted configuration alone could bypass the verifier altogether.
	// VPP's own refusal of redirect proves this fixture reaches that verifier.
	redirect := vppNATPlacementSections("prerouting", "0", `"redirect":{"to":"8080"}`)
	err := firewall.VerifyConfig(redirect)
	require.ErrorContains(t, err, "not supported in NAT chain by backend vpp")
}

func vppNATPlacementSections(hook, priority, action string) []sdk.ConfigSection {
	const section = `{"firewall":{"backend":"vpp","table":{"placement":{"family":"ip","chain":{"entry":{"type":"nat","hook":%q,"priority":%q,"policy":"accept","term":{"translate":{"then":{%s}}}}}}}}}`
	return []sdk.ConfigSection{{Root: "firewall", Data: fmt.Sprintf(section, hook, priority, action)}}
}
