// Design: docs/guide/rpki.md -- origin validation excludes routes only under an explicit policy.
// Related: plugin_fixture_15.go -- native observer startup and joined shutdown.
// Related: plugin_fixture_15_show.go -- waiting for the scripted BGP peer.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/rpki-default-invalid-eligible",
		plugin15Observe("rpki-default-policy-test", &sdk.Registration{}, rpkiDefaultPolicy), StopsDaemon)
}

// rpkiDefaultPolicy waits for synchronized VRPs and two completed received
// verdicts. The Invalid route and its Valid control must both remain eligible;
// an absent route, an unfinished verdict, or a missing field cannot pass.
// RFC 6811 Section 2: "An implementation MUST NOT exclude a route from the Adj-RIB-In or from
// consideration in the decision process as a side effect of its validation state, unless
// explicitly configured to do so."
func rpkiDefaultPolicy(ctx context.Context, plugin *sdk.Plugin) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := plugin15WaitEOR(ctx, plugin, "peer1"); err != nil {
		return err
	}
	last := errors.New("cache-only RPKI state has not been inspected")
	if !Poll(ctx, 80, 250*time.Millisecond, func() bool {
		last = rpkiDefaultPolicyState(ctx, plugin)
		return last == nil
	}) {
		return fmt.Errorf("cache-only RPKI never retained eligible Invalid and Valid routes: %w", last)
	}
	_, err := fmt.Fprintln(os.Stderr, "OK: cache-only RPKI retains the Invalid route eligible alongside its Valid control")
	return err
}

// rpkiDefaultPolicyState reads both live command answers on every attempt, so
// a cache that has not synchronized cannot pass through timeout acceptance.
func rpkiDefaultPolicyState(ctx context.Context, plugin *sdk.Plugin) error {
	var cache struct {
		Running *bool   `json:"running"`
		Synced  *bool   `json:"synced"`
		VRPs    *uint64 `json:"vrp-count-ipv4"`
	}
	status, err := Dispatch(ctx, plugin, "show bgp rpki status", &cache)
	if err != nil {
		return err
	}
	if status != rpc.StatusDone {
		return fmt.Errorf("RPKI status answered %q instead of done", status)
	}
	if cache.Running == nil {
		return errors.New("RPKI status omitted running")
	}
	if !*cache.Running {
		return errors.New("RPKI is not running")
	}
	if cache.Synced == nil {
		return errors.New("RPKI status omitted synced")
	}
	if !*cache.Synced {
		return errors.New("RPKI cache has not synchronized")
	}
	if cache.VRPs == nil {
		return errors.New("RPKI status omitted the IPv4 VRP count")
	}
	if *cache.VRPs != 1 {
		return fmt.Errorf("RPKI cache has %d IPv4 VRPs, want the one scripted covering VRP", *cache.VRPs)
	}
	var answer struct {
		Peers map[string][]rpkiDefaultPolicyRoute `json:"adj-rib-in"`
	}
	status, err = Dispatch(ctx, plugin, plugin14AdjRIBInCommand, &answer)
	if err != nil {
		return err
	}
	if status != rpc.StatusDone {
		return fmt.Errorf("received-route inspection answered %q instead of done", status)
	}
	if len(answer.Peers) != 1 {
		return fmt.Errorf("received-route inspection has %d peers, want only 127.0.0.1", len(answer.Peers))
	}
	routes, found := answer.Peers[addrLoopback]
	if !found {
		return errors.New("received-route inspection omitted the scripted peer 127.0.0.1")
	}
	if len(routes) != 2 {
		return fmt.Errorf("received-route inspection has %d routes, want Invalid and Valid controls", len(routes))
	}
	// The command's documented state values are 1 (Valid) and 3 (Invalid).
	expected := map[string]uint8{"ipv4/unicast:10.0.1.0/24": 3, "ipv4/unicast:10.0.2.0/24": 1}
	for _, route := range routes {
		state, found := expected[route.Key]
		if !found {
			return fmt.Errorf("received-route inspection returned unknown or duplicate route %q", route.Key)
		}
		if route.State == nil {
			return fmt.Errorf("route %s omitted its validation state", route.Key)
		}
		if *route.State != state {
			return fmt.Errorf("route %s has validation state %d, want %d", route.Key, *route.State, state)
		}
		if route.Ineligible == nil {
			return fmt.Errorf("route %s omitted its eligibility", route.Key)
		}
		if *route.Ineligible {
			return fmt.Errorf("route %s is ineligible without a configured rejection", route.Key)
		}
		if route.Attributes == "" {
			return fmt.Errorf("route %s omitted its received attributes", route.Key)
		}
		if route.NextHop == "" {
			return fmt.Errorf("route %s omitted its received next hop", route.Key)
		}
		if route.NLRI == "" {
			return fmt.Errorf("route %s omitted its received NLRI", route.Key)
		}
		delete(expected, route.Key)
	}
	return nil
}

type rpkiDefaultPolicyRoute struct {
	Key        string `json:"key"`
	State      *uint8 `json:"validation-state"`
	Ineligible *bool  `json:"ineligible"`
	Attributes string `json:"attr-hex"`
	NextHop    string `json:"nhop-hex"`
	NLRI       string `json:"nlri-hex"`
}
