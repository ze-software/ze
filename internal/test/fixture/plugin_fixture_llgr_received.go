// Design: docs/architecture/route-selection.md -- Step 0, stale depreference
// Related: plugin_fixture_02.go -- the registration, requireDone02, map02
// Related: ../../../test/plugin/llgr-stale-received-depreferenced.ci -- the scenario

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	llgrReceivedDevice   = "llgr-fixture0"
	llgrReceivedInstance = 9494
)

// llgrReceivedVerdict is what one prefix of the scenario must show.
type llgrReceivedVerdict struct {
	prefix string // the contested prefix
	winner string // the peer address that must be best
	step   string // the decision step that must decide it
}

// llgrStaleReceived02 checks RFC 9494 Section 4.3 through the daemon: a route
// carrying LLGR_STALE from the session whose OPEN carried the LLGR Capability
// loses Step 0 to a fresh route at a lower LOCAL_PREF, and the same community
// from the session whose OPEN did not carry it is not depreferenced, so its
// higher LOCAL_PREF wins.
func llgrStaleReceived02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := llgrReceivedTopology(ctx, plugin); err != nil {
		return err
	}
	if err := p12WaitPeersEOR(ctx, plugin, "llgr", "plain"); err != nil {
		return err
	}
	verdicts := []llgrReceivedVerdict{
		{prefix: "10.0.0.0/8", winner: addrLoopbackV6, step: "stale-level"},
		{prefix: "10.1.0.0/16", winner: addrLoopbackV6, step: "local-preference"},
	}
	entries := make(map[string]map[string]any, len(verdicts))
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp rib best reason")
		if err != nil {
			return false
		}
		data, err := map02(raw)
		if err != nil {
			return false
		}
		list, _ := data["best-path-reason"].([]any)
		for _, value := range list {
			entry, _ := value.(map[string]any)
			candidates, _ := entry["candidates"].([]any)
			prefix, _ := entry["prefix"].(string)
			if len(candidates) >= 2 {
				entries[prefix] = entry
			}
		}
		return len(entries) >= len(verdicts)
	}) {
		return fmt.Errorf("two candidates never stored for both prefixes, saw %v", entries)
	}
	for _, want := range verdicts {
		entry, ok := entries[want.prefix]
		if !ok {
			return fmt.Errorf("%s: no entry with two candidates, saw %v", want.prefix, entries)
		}
		if entry["winner-peer"] != want.winner {
			return fmt.Errorf("%s: best is %v, want %s: %v", want.prefix, entry["winner-peer"], want.winner, entry)
		}
		steps, _ := entry["steps"].([]any)
		if len(steps) == 0 {
			return fmt.Errorf("%s: no decision steps: %v", want.prefix, entry)
		}
		first, _ := steps[0].(map[string]any)
		if first["step"] != want.step {
			return fmt.Errorf("%s: decided by %v, want %s: %v", want.prefix, first["step"], want.step, entry)
		}
		fmt.Fprintf(os.Stderr, "OK: llgr received %s best %s by %s\n", want.prefix, want.winner, want.step)
	}
	return nil
}

// llgrReceivedTopology gives the unchanged wire next hops independent terminal
// paths in the real system RIB. The device is route metadata, not a claim that
// a kernel interface exists or that a kernel route was installed.
func llgrReceivedTopology(ctx context.Context, p *sdk.Plugin) error {
	addresses := []string{addrPeerOne, addrPeerTwo}
	entries := make([]rpc.RouteInstallEntry, len(addresses))
	for i, address := range addresses {
		entries[i] = rpc.RouteInstallEntry{Protocol: protocolStatic,
			Instance: llgrReceivedInstance, AFI: 1, SAFI: 1, Prefix: address + "/32",
			Interface: llgrReceivedDevice, RouteType: 1}
	}
	if err := sysribInstall(ctx, p, entries...); err != nil {
		return fmt.Errorf("install LLGR next-hop covers: %w", err)
	}
	last := errors.New("terminal LLGR covers have not been observed")
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		last = llgrReceivedTopologyState(ctx, p, addresses)
		return last == nil
	}) {
		return fmt.Errorf("LLGR next-hop covers were not ready: %w", last)
	}
	return nil
}

type llgrReceivedCoverRow struct {
	Prefix string                  `json:"prefix"`
	Family family.Family           `json:"family"`
	Paths  []llgrReceivedCoverPath `json:"paths"`
}

type llgrReceivedCoverPath struct {
	Source    string          `json:"source"`
	Instance  uint32          `json:"instance"`
	Interface string          `json:"interface"`
	NextHop   json.RawMessage `json:"next-hop"`
	Metric    *uint32         `json:"metric"`
	Selected  bool            `json:"selected"`
	Hidden    *bool           `json:"hidden"`
}

func llgrReceivedTopologyState(ctx context.Context, p *sdk.Plugin, addresses []string) error {
	for _, address := range addresses {
		prefix := address + "/32"
		var rows []llgrReceivedCoverRow
		status, err := Dispatch(ctx, p, "show rib candidates prefix "+prefix, &rows)
		if err != nil {
			return err
		}
		if status != rpc.StatusDone {
			return fmt.Errorf("LLGR cover %s inspection answered %q instead of done", prefix, status)
		}
		if len(rows) != 1 {
			return fmt.Errorf("LLGR cover %s inspection returned %d prefixes, want one", prefix, len(rows))
		}
		row := &rows[0]
		if row.Prefix != prefix {
			return fmt.Errorf("LLGR cover inspection returned prefix %q, want %s", row.Prefix, prefix)
		}
		if row.Family != family.IPv4Unicast {
			return fmt.Errorf("LLGR cover %s inspection returned family %s", prefix, row.Family)
		}
		if len(row.Paths) != 1 {
			return fmt.Errorf("LLGR cover %s inspection returned %d paths, want one", prefix, len(row.Paths))
		}
		path := &row.Paths[0]
		if path.Source != protocolStatic {
			return fmt.Errorf("LLGR cover %s source is %q, want static", prefix, path.Source)
		}
		if path.Instance != llgrReceivedInstance {
			return fmt.Errorf("LLGR cover %s instance is %d, want %d", prefix, path.Instance, llgrReceivedInstance)
		}
		if path.Interface != llgrReceivedDevice {
			return fmt.Errorf("LLGR cover %s device is %q, want %s", prefix, path.Interface, llgrReceivedDevice)
		}
		if len(path.NextHop) != 0 {
			return fmt.Errorf("LLGR terminal cover %s unexpectedly names a gateway", prefix)
		}
		if path.Metric == nil {
			return fmt.Errorf("LLGR cover %s inspection omitted its metric", prefix)
		}
		if *path.Metric != 0 {
			return fmt.Errorf("LLGR cover %s metric is %d, want zero", prefix, *path.Metric)
		}
		if !path.Selected {
			return fmt.Errorf("LLGR cover %s is not selected", prefix)
		}
		if path.Hidden == nil {
			return fmt.Errorf("LLGR cover %s inspection omitted its hidden state", prefix)
		}
		if *path.Hidden {
			return fmt.Errorf("LLGR cover %s is hidden", prefix)
		}
	}
	metrics, err := p.RouteMetrics(ctx, addresses)
	if err != nil {
		return err
	}
	if len(metrics.Distances) != len(addresses) {
		return fmt.Errorf("LLGR topology returned %d metrics, want %d", len(metrics.Distances), len(addresses))
	}
	for i, address := range addresses {
		if metrics.Distances[i] != (rpc.RouteMetric{Resolved: true}) {
			return fmt.Errorf("LLGR next hop %s has metric %+v, want resolved, nonrecursive and zero cost", address, metrics.Distances[i])
		}
	}
	return nil
}
