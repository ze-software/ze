// Design: docs/architecture/testing/interop.md -- live capability replacement against FRR.
// Related: check_rpki_reload.go -- the applied-generation SIGHUP barrier.
// Upstream: https://github.com/FRRouting/frr/tree/frr-10.7.1/bgpd -- bgp_role_violation,
// bgp_show_peer and bgp_get_name_by_role define the acceptance and observations below.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkRoleReload adds and removes Ze's Role while FRR remains configured as
// Customer. FRR's non-strict mode accepts an absent Role; its neighbor JSON
// distinguishes that absence from the Provider Role received in the next OPEN.
// Each change must reach an applied reload and a new established session, so a
// successful reload that leaves the startup declaration in place cannot pass.
func checkRoleReload(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("role reload scenario has no selected IPv4 network")
	}
	neighbor := networkHostAddress(check.Network, 2)
	command := []string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}
	fields := map[string]string{
		"bgpState":   stateEstablished,
		"localRole":  "customer",
		"remoteRole": "undefined",
	}
	if err := waitJSONFields(ctx, check.Lab, peerFRR, command, 90*time.Second, fields, nil); err != nil {
		return checkerFailure(ctx, check.Lab, check.Source.Name, 1, fmt.Errorf("initial session without a Role: %w", err))
	}
	// Two reloads cover publication of a nonempty declaration and its removal.
	for index, phase := range []struct {
		config string
		role   string
	}{
		{config: zeMountedReloadConfig, role: "provider"},
		{config: zeMountedConfig, role: "undefined"},
	} {
		before, err := queryFRRSessionGeneration(ctx, check.Lab, neighbor)
		if err != nil {
			return checkerFailure(ctx, check.Lab, check.Source.Name, 2+index*4, err)
		}
		if err := applyZeReload(ctx, check.Lab, phase.config); err != nil {
			return checkerFailure(ctx, check.Lab, check.Source.Name, 3+index*4, err)
		}
		if err := waitFRRNewSession(ctx, check.Lab, neighbor, before); err != nil {
			return checkerFailure(ctx, check.Lab, check.Source.Name, 4+index*4, err)
		}
		fields["remoteRole"] = phase.role
		if err := waitJSONFields(ctx, check.Lab, peerFRR, command, 90*time.Second, fields, nil); err != nil {
			return checkerFailure(ctx, check.Lab, check.Source.Name, 5+index*4, fmt.Errorf("FRR receiving Role %s after reload: %w", phase.role, err))
		}
	}
	return nil
}
