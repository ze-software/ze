// Design: docs/architecture/testing/interop.md -- independent AIGP source-cost proof.
// Related: check_aigp_source_cost.go -- checker-driven metric changes, never source UPDATEs.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	scenarioAIGPSourceCostFRR = "bgp-nexthop-self-local-auto-frr"
	aigpCostCommand           = "request interop aigp-cost"
)

// runAIGPSourceCostProcess uses the same route-install RPC as a forked IGP plugin.
// The helper changes only a next-hop distance; it never advertises the test NLRI.
func runAIGPSourceCostProcess(name string) error {
	registration := sdk.Registration{Commands: []rpc.CommandDecl{{
		Name: aigpCostCommand, ShortHelp: "Install an AIGP interop next-hop distance",
	}}}
	var runErr error
	code := sdk.RunOrDeclare(registration, func() int {
		plugin, err := sdk.NewFromEnv(name)
		if err != nil {
			runErr = err
			return 1
		}
		plugin.OnExecuteCommand(func(_ string, command string, args []string, _ string) (string, any, error) {
			if command != aigpCostCommand {
				return rpc.StatusError, nil, errors.New("unexpected AIGP control command")
			}
			if len(args) != 2 {
				return rpc.StatusError, nil, errors.New("aigp-cost wants NEXT-HOP and METRIC")
			}
			hop, err := netip.ParseAddr(args[0])
			if err != nil {
				return rpc.StatusError, nil, err
			}
			if !hop.Is4() {
				return rpc.StatusError, nil, errors.New("AIGP control requires an IPv4 next hop")
			}
			metric, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return rpc.StatusError, nil, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			installed, err := plugin.RouteInstall(ctx, []rpc.RouteInstallEntry{{
				Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: netip.PrefixFrom(hop, 32).String(),
				AdminDistance: 110, Metric: uint32(metric),
			}})
			if err != nil {
				return rpc.StatusError, nil, err
			}
			if installed != 1 {
				return rpc.StatusError, nil, fmt.Errorf("installed %d next-hop metrics, want 1", installed)
			}
			return rpc.StatusDone, map[string]any{"next-hop": hop.String(), "metric": metric, "installed": installed}, nil
		})
		runErr = plugin.Run(context.Background(), registration)
		if runErr != nil {
			return 1
		}
		return 0
	})
	if code != 0 && runErr == nil {
		return errors.New("AIGP control process declaration failed")
	}
	return runErr
}
