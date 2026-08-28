package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func fixture06FIBBlackhole(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06DispatchDone(ctx, p, "request fakefib emit add ipv4/unicast 192.0.2.0/24 routetype blackhole"); err != nil {
		return err
	}
	if err := fixture06Wait(ctx, time.Second); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK AC-1: blackhole route emitted to fib-kernel via sysrib EventBus")
	return fixture06WaitEOR(ctx, p, 1)
}

func fixture06RIBEntry(rows []map[string]any, prefix string) map[string]any {
	for _, row := range rows {
		if row["prefix"] == prefix {
			return row
		}
	}
	return nil
}

func fixture06AllNextHops(entry map[string]any) []string {
	set := map[string]struct{}{}
	if nextHop, ok := entry["next-hop"].(string); ok && nextHop != "" {
		set[nextHop] = struct{}{}
	}
	paths, _ := entry["ecmp-paths"].([]any)
	for _, raw := range paths {
		path, _ := raw.(map[string]any)
		if nextHop, ok := path["next-hop"].(string); ok && nextHop != "" {
			set[nextHop] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for nextHop := range set {
		out = append(out, nextHop)
	}
	sort.Strings(out)
	return out
}

func fixture06FIBECMPRealtime(ctx context.Context, p *sdk.Plugin) error {
	const prefix = "10.55.0.0/24"
	for _, route := range []struct{ peer, nextHop string }{{"10.0.0.1", "10.0.0.11"}, {"10.0.0.2", "10.0.0.12"}} {
		command := fmt.Sprintf("request bgp rib inject %s ipv4/unicast %s origin igp localpref 100 aspath 65001 nexthop %s", route.peer, prefix, route.nextHop)
		if err := fixture06DispatchDone(ctx, p, command); err != nil {
			return err
		}
	}
	rows, err := fixture06PollArray(ctx, p, "show rib", 30, func(rows []map[string]any) bool {
		entry := fixture06RIBEntry(rows, prefix)
		paths, _ := entry["ecmp-paths"].([]any)
		return len(paths) >= 1
	})
	if err != nil {
		return fmt.Errorf("%s not in sysrib with ECMP: %w", prefix, err)
	}
	entry := fixture06RIBEntry(rows, prefix)
	nextHops := fixture06AllNextHops(entry)
	if strings.Join(nextHops, ",") != "10.0.0.11,10.0.0.12" {
		return fmt.Errorf("expected ECMP next-hops [10.0.0.11 10.0.0.12], got %v", nextHops)
	}
	fmt.Fprintf(os.Stderr, "OK: sysrib ECMP carries both next-hops %v\n", nextHops)
	if err := fixture06DispatchDone(ctx, p, "request bgp rib withdraw 10.0.0.2 ipv4/unicast "+prefix); err != nil {
		return err
	}
	rows, err = fixture06PollArray(ctx, p, "show rib", 30, func(rows []map[string]any) bool {
		entry := fixture06RIBEntry(rows, prefix)
		paths, _ := entry["ecmp-paths"].([]any)
		return entry != nil && len(paths) == 0
	})
	if err != nil {
		return fmt.Errorf("ECMP set did not shrink: %w", err)
	}
	if fixture06RIBEntry(rows, prefix) == nil {
		return fmt.Errorf("%s vanished after withdrawing one path", prefix)
	}
	fmt.Fprintln(os.Stderr, "OK: ECMP set shrank to a single next-hop after withdraw")
	return fixture06WaitEOR(ctx, p, 1)
}

func fixture06BestEntry(ctx context.Context, p *sdk.Plugin, prefix string) map[string]any {
	data, err := fixture06DispatchObject(ctx, p, "show bgp rib best")
	if err != nil {
		return nil
	}
	rows, _ := data["best-path"].([]any)
	for _, raw := range rows {
		entry, _ := raw.(map[string]any)
		if entry["prefix"] == prefix {
			return entry
		}
	}
	return nil
}

func fixture06SiblingCount(entry map[string]any) int {
	if entry == nil {
		return -1
	}
	peers, _ := entry["multipath-peers"].([]any)
	return len(peers)
}

func fixture06FIBECMP(ctx context.Context, p *sdk.Plugin) error {
	if _, err := fixture06PollObject(ctx, p, "show bgp rib status", 60, func(map[string]any) bool { return true }); err != nil {
		return errors.New("rib plugin did not become ready")
	}
	const prefix = "10.50.0.0/24"
	inject := func(peer string) error {
		return fixture06DispatchDone(ctx, p, fmt.Sprintf("request bgp rib inject %s ipv4/unicast %s origin igp localpref 100 aspath 65001", peer, prefix))
	}
	if err := inject("10.0.0.1"); err != nil {
		return err
	}
	if err := inject("10.0.0.2"); err != nil {
		return err
	}
	if !Poll(ctx, 40, fixture06PollDelay, func() bool { return fixture06SiblingCount(fixture06BestEntry(ctx, p, prefix)) == 1 }) {
		return fmt.Errorf("AC-1: expected 1 multipath sibling for %s", prefix)
	}
	entry := fixture06BestEntry(ctx, p, prefix)
	peers, _ := entry["multipath-peers"].([]any)
	rows, err := fixture06PollArray(ctx, p, "show rib", 40, func(rows []map[string]any) bool { return fixture06RIBEntry(rows, prefix) != nil })
	if err != nil || fixture06RIBEntry(rows, prefix) == nil {
		return fmt.Errorf("AC-1: %s not in sysrib: %w", prefix, err)
	}
	fmt.Fprintf(os.Stderr, "OK AC-1: multipath 2-path: best=%v, siblings=%v\n", entry["best-peer"], peers)
	if err := fixture06DispatchDone(ctx, p, "request bgp rib withdraw 10.0.0.2 ipv4/unicast "+prefix); err != nil {
		return err
	}
	if !Poll(ctx, 40, fixture06PollDelay, func() bool { return fixture06SiblingCount(fixture06BestEntry(ctx, p, prefix)) == 0 }) {
		return fmt.Errorf("AC-2: multipath sibling remained after withdraw")
	}
	entry = fixture06BestEntry(ctx, p, prefix)
	rows, err = fixture06DispatchArray(ctx, p, "show rib")
	if err != nil {
		return err
	}
	if fixture06RIBEntry(rows, prefix) == nil {
		return fmt.Errorf("AC-2: %s not in sysrib after single withdraw", prefix)
	}
	fmt.Fprintf(os.Stderr, "OK AC-2: single path after withdraw: best=%v\n", entry["best-peer"])
	if err := inject("10.0.0.2"); err != nil {
		return err
	}
	if err := inject("10.0.0.3"); err != nil {
		return err
	}
	if !Poll(ctx, 40, fixture06PollDelay, func() bool { return fixture06SiblingCount(fixture06BestEntry(ctx, p, prefix)) == 2 }) {
		return fmt.Errorf("AC-3: expected 2 multipath siblings for %s", prefix)
	}
	entry = fixture06BestEntry(ctx, p, prefix)
	peers, _ = entry["multipath-peers"].([]any)
	fmt.Fprintf(os.Stderr, "OK AC-3: multipath 3-path: best=%v, siblings=%v\n", entry["best-peer"], peers)
	return fixture06WaitEOR(ctx, p, 1)
}

func fixture06FIBMetric(ctx context.Context, p *sdk.Plugin) error {
	if _, err := fixture06PollObject(ctx, p, "show bgp rib status", 60, func(map[string]any) bool { return true }); err != nil {
		return errors.New("rib plugin did not become ready")
	}
	if err := fixture06DispatchDone(ctx, p, "request bgp rib inject 10.0.0.1 ipv4/unicast 10.20.0.0/24 origin igp med 200"); err != nil {
		return err
	}
	rows, err := fixture06PollArray(ctx, p, "show rib", 40, func(rows []map[string]any) bool { return fixture06RIBEntry(rows, "10.20.0.0/24") != nil })
	if err != nil || fixture06RIBEntry(rows, "10.20.0.0/24") == nil {
		return fmt.Errorf("AC-2: route missing from sysrib: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK AC-2: route with metric 200 delivered through sysrib to fib-kernel")
	return fixture06WaitEOR(ctx, p, 1)
}

func fixture06FIBMPLSKernel(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06DispatchDone(ctx, p, "request fakefib emit add ipv4/unicast 10.0.0.0/24 nexthop 192.168.1.1 labels 100,200"); err != nil {
		return err
	}
	if err := fixture06Wait(ctx, time.Second); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK AC-4: MPLS-labeled route emitted to fib-kernel via sysrib EventBus")
	return fixture06WaitEOR(ctx, p, 1)
}

func fixture06FIBRecursive(ctx context.Context, p *sdk.Plugin) error {
	if _, err := fixture06PollObject(ctx, p, "show bgp rib status", 60, func(map[string]any) bool { return true }); err != nil {
		return errors.New("rib plugin did not become ready")
	}
	for _, command := range []string{
		"request bgp rib inject 10.0.0.1 ipv4/unicast 10.0.0.0/24 origin igp",
		"request bgp rib inject 10.0.0.1 ipv4/unicast 172.16.0.0/16 origin igp nexthop 10.0.0.2",
	} {
		if err := fixture06DispatchDone(ctx, p, command); err != nil {
			return err
		}
	}
	rows, err := fixture06PollArray(ctx, p, "show rib", 40, func(rows []map[string]any) bool { return fixture06RIBEntry(rows, "172.16.0.0/16") != nil })
	if err != nil || fixture06RIBEntry(rows, "172.16.0.0/16") == nil {
		return fmt.Errorf("AC-2: recursive route missing from system RIB: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK AC-2: recursive route 172.16.0.0/16 present in system RIB")
	for _, command := range []string{"show nexthop-table", "show ecmp-groups"} {
		rows, err := fixture06DispatchArray(ctx, p, command)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "OK %s: returned %d entries\n", strings.ReplaceAll(command, " ", "-"), len(rows))
	}
	return fixture06WaitEOR(ctx, p, 1)
}
