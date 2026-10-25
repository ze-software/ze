// Design: docs/architecture/testing/interop.md -- final policy judged by a foreign daemon.
package bgp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func checkPolicyFinalFRR(ctx context.Context, check *interoplab.CheckContext) error {
	assertion := 0
	run := func(operations ...operation) error {
		for i := range operations {
			assertion++
			if err := runOperation(ctx, check.Network, check.Lab, &operations[i]); err != nil {
				return checkerFailure(ctx, check.Lab, scenarioPolicyFinalFRR, assertion, err)
			}
		}
		return nil
	}
	routes := func() error {
		for _, prefix := range []string{"10.73.1.0/24", "10.73.2.0/24"} {
			if err := run(operation{kind: opFRRRoute, argument: prefix, timeout: 90 * time.Second}); err != nil {
				return err
			}
			assertion++
			output, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + prefix + " json"}, nil)
			if err == nil {
				err = requirePolicyFinalFRRRoute(output, prefix)
			}
			if err != nil {
				return checkerFailure(ctx, check.Lab, scenarioPolicyFinalFRR, assertion, interoplab.WithSearchedOutput(err, output))
			}
		}
		return nil
	}
	// The injector sends one EOR and one UPDATE, then only KEEPALIVEs.
	// Neither reconciliation phase may solicit another source announcement.
	sourceUnchanged := operation{kind: opRequireJSONFields, peer: "ze",
		command: zeCommand("show bgp peer 172.30.0.9 detail"),
		fields:  map[string]string{zeFieldConnectionsEstablished: "1", zeFieldConnectionsDropped: "0", "updates-received": "2"}}
	destinationUnchanged := operation{kind: opRequireJSONFields, peer: peerFRR,
		command: []string{cmdVtysh, "-c", frrShowZeNeighborJSON},
		fields:  map[string]string{"connectionsEstablished": "1", "connectionsDropped": "0", frrSessionStateKey: stateEstablished}}
	if err := run(operation{kind: opFRRSession, argument: zeLabAddress, timeout: 90 * time.Second}); err != nil {
		return err
	}
	if err := routes(); err != nil {
		return err
	}
	if err := run(sourceUnchanged, destinationUnchanged); err != nil {
		return err
	}
	assertion++
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return checkerFailure(ctx, check.Lab, scenarioPolicyFinalFRR, assertion, err)
	}
	if err := run(
		operation{kind: opWaitAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", showTableJSON},
			absent: []string{"10.73.1.0/24", "10.73.2.0/24"}, proof: []string{"10.73.254.0/24"}, timeout: 60 * time.Second},
		sourceUnchanged, destinationUnchanged); err != nil {
		return err
	}
	assertion++
	if err := applyZeReload(ctx, check.Lab, zeMountedConfig); err != nil {
		return checkerFailure(ctx, check.Lab, scenarioPolicyFinalFRR, assertion, err)
	}
	if err := routes(); err != nil {
		return err
	}
	return run(sourceUnchanged, destinationUnchanged)
}

// MED belongs to the route path, not its nested nexthop's IGP metric.
func requirePolicyFinalFRRRoute(output, prefix string) error {
	var route struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Metric uint32 `json:"metric"`
			Valid  bool   `json:"valid"`
			ASPath struct {
				String string `json:"string"`
			} `json:"aspath"`
			Community struct {
				List []string `json:"list"`
			} `json:"community"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &route); err != nil {
		return fmt.Errorf("decode FRR policy route: %w", err)
	}
	if route.Prefix != prefix {
		return fmt.Errorf("FRR policy route has prefix %q and %d paths, want %s with one path", route.Prefix, len(route.Paths), prefix)
	}
	if len(route.Paths) != 1 {
		return fmt.Errorf("FRR policy route has prefix %q and %d paths, want %s with one path", route.Prefix, len(route.Paths), prefix)
	}
	path := &route.Paths[0]
	if !path.Valid {
		return fmt.Errorf("FRR %s: valid=%t MED=%d AS_PATH=%q, want true, 77, and 65000 65004", prefix, path.Valid, path.Metric, path.ASPath.String)
	}
	if path.Metric != 77 {
		return fmt.Errorf("FRR %s: valid=%t MED=%d AS_PATH=%q, want true, 77, and 65000 65004", prefix, path.Valid, path.Metric, path.ASPath.String)
	}
	if path.ASPath.String != "65000 65004" {
		return fmt.Errorf("FRR %s: valid=%t MED=%d AS_PATH=%q, want true, 77, and 65000 65004", prefix, path.Valid, path.Metric, path.ASPath.String)
	}
	if len(path.Community.List) != 2 {
		return fmt.Errorf("FRR %s communities=%v, want 65000:73 and 65004:99", prefix, path.Community.List)
	}
	if !slices.Contains(path.Community.List, "65000:73") {
		return fmt.Errorf("FRR %s communities=%v, want 65000:73 and 65004:99", prefix, path.Community.List)
	}
	if !slices.Contains(path.Community.List, "65004:99") {
		return fmt.Errorf("FRR %s communities=%v, want 65000:73 and 65004:99", prefix, path.Community.List)
	}
	return nil
}
