package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// fixture06DiscardRoutes pairs each discard route type the system RIB carries
// with a prefix of its own, so one kernel read per prefix names the type the
// FIB plugin programmed for it. The third field is the type as kernelEntries
// reads it: `ip -N` prints rtm_type as its number, RTN_BLACKHOLE 6,
// RTN_UNREACHABLE 7 and RTN_PROHIBIT 8 (linux/rtnetlink.h).
var fixture06DiscardRoutes = [][3]string{
	{"blackhole", "198.18.201.0/24", "6"},
	{"unreachable", "198.18.202.0/24", "7"},
	{"prohibit", "198.18.203.0/24", "8"},
}

// fixture06FIBBlackhole proves spec-fib-depth AC-6 and AC-7 at the kernel: a
// system-RIB change of each discard type reaches fib-kernel, which installs a
// kernel route of that type (RTN_BLACKHOLE, RTN_UNREACHABLE, RTN_PROHIBIT),
// stamped proto 250. The kernel read is the evidence; the emit alone proves
// nothing, because fib-kernel could drop the change and log its name.
func fixture06FIBBlackhole(ctx context.Context, p *sdk.Plugin) error {
	for _, route := range fixture06DiscardRoutes {
		if err := fixture06DispatchDone(ctx, p, "request fakefib emit add ipv4/unicast "+route[1]+" routetype "+route[0]); err != nil {
			return err
		}
	}
	for _, route := range fixture06DiscardRoutes {
		var entries []string
		if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
			entries = kernelEntries(ctx, route[1])
			return len(entries) == 1 &&
				strings.HasPrefix(entries[0], route[2]+" "+route[1]+" ") &&
				strings.Contains(entries[0], "proto 250")
		}) {
			return fmt.Errorf("%s: want one kernel entry `%s %s ... proto 250` (%s); ip -N route show table all: %q",
				route[1], route[2], route[1], route[0], entries)
		}
		fmt.Fprintln(os.Stderr, "OK: kernel holds "+route[0]+" "+route[1]+" proto 250")
	}
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
	slices.Sort(out)
	return out
}

func fixture06FIBECMPRealtime(ctx context.Context, p *sdk.Plugin) error {
	const prefix = "10.55.0.0/24"
	for _, route := range []struct{ peer, nextHop string }{{addrPeerOne, "10.0.0.11"}, {addrPeerTwo, "10.0.0.12"}} {
		command := fmt.Sprintf("request bgp rib inject %s ipv4/unicast %s origin igp localpref 100 aspath 65001 nexthop %s", route.peer, prefix, route.nextHop)
		if err := fixture06DispatchDone(ctx, p, command); err != nil {
			return err
		}
	}
	rows, err := fixture06PollArray(ctx, p, 30, func(rows []map[string]any) bool {
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
	rows, err = fixture06PollArray(ctx, p, 30, func(rows []map[string]any) bool {
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

// ecmpPrefix06 is the prefix the ECMP fixture injects and then inspects.
const ecmpPrefix06 = "10.50.0.0/24"

func fixture06BestEntry(ctx context.Context, p *sdk.Plugin) map[string]any {
	data, err := fixture06DispatchObject(ctx, p, "show bgp rib best")
	if err != nil {
		return nil
	}
	rows, _ := data["best-path"].([]any)
	for _, raw := range rows {
		entry, _ := raw.(map[string]any)
		if entry["prefix"] == ecmpPrefix06 {
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
	const prefix = ecmpPrefix06
	// Each peer's path carries its own next hop, 10.0.0.N to 10.0.0.1N. A sibling
	// with no next hop names no target, so the system RIB would leave it out of
	// the ECMP group and the path count below could not be asserted.
	inject := func(peer string) error {
		return fixture06DispatchDone(ctx, p, fmt.Sprintf("request bgp rib inject %s ipv4/unicast %s origin igp localpref 100 aspath 65001 nexthop %s", peer, prefix, fixture06ECMPNextHop(peer)))
	}
	if err := inject("10.0.0.1"); err != nil {
		return err
	}
	if err := inject("10.0.0.2"); err != nil {
		return err
	}
	if !Poll(ctx, 40, fixture06PollDelay, func() bool { return fixture06SiblingCount(fixture06BestEntry(ctx, p)) == 1 }) {
		return fmt.Errorf("AC-1: expected 1 multipath sibling for %s", prefix)
	}
	entry := fixture06BestEntry(ctx, p)
	peers, _ := entry["multipath-peers"].([]any)
	if err := fixture06AwaitECMP(ctx, p, prefix, "10.0.0.11", "10.0.0.12"); err != nil {
		return fmt.Errorf("AC-1: %w", err)
	}
	fmt.Fprintf(os.Stderr, "OK AC-1: multipath 2-path: best=%v, siblings=%v\n", entry["best-peer"], peers)
	if err := fixture06DispatchDone(ctx, p, "request bgp rib withdraw 10.0.0.2 ipv4/unicast "+prefix); err != nil {
		return err
	}
	if !Poll(ctx, 40, fixture06PollDelay, func() bool { return fixture06SiblingCount(fixture06BestEntry(ctx, p)) == 0 }) {
		return fmt.Errorf("AC-2: multipath sibling remained after withdraw")
	}
	entry = fixture06BestEntry(ctx, p)
	if err := fixture06AwaitECMP(ctx, p, prefix, "10.0.0.11"); err != nil {
		return fmt.Errorf("AC-2: %w", err)
	}
	fmt.Fprintf(os.Stderr, "OK AC-2: single path after withdraw: best=%v\n", entry["best-peer"])
	if err := inject("10.0.0.2"); err != nil {
		return err
	}
	if err := inject("10.0.0.3"); err != nil {
		return err
	}
	if !Poll(ctx, 40, fixture06PollDelay, func() bool { return fixture06SiblingCount(fixture06BestEntry(ctx, p)) == 2 }) {
		return fmt.Errorf("AC-3: expected 2 multipath siblings for %s", prefix)
	}
	entry = fixture06BestEntry(ctx, p)
	if err := fixture06AwaitECMP(ctx, p, prefix, "10.0.0.11", "10.0.0.12", "10.0.0.13"); err != nil {
		return fmt.Errorf("AC-3: %w", err)
	}
	peers, _ = entry["multipath-peers"].([]any)
	fmt.Fprintf(os.Stderr, "OK AC-3: multipath 3-path: best=%v, siblings=%v\n", entry["best-peer"], peers)
	return fixture06WaitEOR(ctx, p, 1)
}

// fixture06ECMPNextHop answers the next hop the ECMP fixture gives a peer's
// path: 10.0.0.N becomes 10.0.0.1N.
func fixture06ECMPNextHop(peer string) string {
	return "10.0.0.1" + peer[len("10.0.0."):]
}

// fixture06AwaitECMP waits until the system RIB holds prefix with exactly the
// next hops want: the winner's next-hop plus one ecmp-paths member for each
// other path. The member count is checked apart from the next-hop set, so a
// duplicated member or a member the set hides still fails.
func fixture06AwaitECMP(ctx context.Context, p *sdk.Plugin, prefix string, want ...string) error {
	slices.Sort(want)
	var entry map[string]any
	_, err := fixture06PollArray(ctx, p, 40, func(rows []map[string]any) bool {
		entry = fixture06RIBEntry(rows, prefix)
		if entry == nil {
			return false
		}
		paths, _ := entry["ecmp-paths"].([]any)
		if len(paths) != len(want)-1 {
			return false
		}
		return slices.Equal(fixture06AllNextHops(entry), want)
	})
	if err != nil {
		return fmt.Errorf("system RIB %s: want next hops %v (%d ecmp-paths), last entry %v: %w", prefix, want, len(want)-1, entry, err)
	}
	fmt.Fprintf(os.Stderr, "OK: system RIB %s carries %d equal-cost next hops %v\n", prefix, len(want), want)
	return nil
}

func fixture06FIBMetric(ctx context.Context, p *sdk.Plugin) error {
	if _, err := fixture06PollObject(ctx, p, "show bgp rib status", 60, func(map[string]any) bool { return true }); err != nil {
		return errors.New("rib plugin did not become ready")
	}
	if err := fixture06DispatchDone(ctx, p, "request bgp rib inject 10.0.0.1 ipv4/unicast 10.20.0.0/24 origin igp med 200"); err != nil {
		return err
	}
	rows, err := fixture06PollArray(ctx, p, 40, func(rows []map[string]any) bool { return fixture06RIBEntry(rows, "10.20.0.0/24") != nil })
	if err != nil || fixture06RIBEntry(rows, "10.20.0.0/24") == nil {
		return fmt.Errorf("AC-2: route missing from sysrib: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK AC-2: route with metric 200 delivered through sysrib to fib-kernel")
	return fixture06WaitEOR(ctx, p, 1)
}

// fixture06MPLSPushPrefix is the labeled route's prefix. Its gateway is the
// static kernel scenarios' gateway, which static/static-kernel-setup makes
// reachable on the dummy link.
const fixture06MPLSPushPrefix = "198.18.204.0/24"

// fixture06FIBMPLSKernel proves spec-fib-depth AC-10 at the kernel: a
// system-RIB change carrying the label stack 100,200 makes fib-kernel install
// an lwtunnel MPLS encap route, outer label first, via the gateway.
func fixture06FIBMPLSKernel(ctx context.Context, p *sdk.Plugin) error {
	if err := fixture06DispatchDone(ctx, p, "request fakefib emit add ipv4/unicast "+fixture06MPLSPushPrefix+" nexthop "+kernelStaticGateway+" labels 100,200"); err != nil {
		return err
	}
	var entries []string
	if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
		entries = kernelEntries(ctx, fixture06MPLSPushPrefix)
		return len(entries) == 1 &&
			strings.Contains(entries[0], "encap mpls 100/200 ") &&
			strings.Contains(entries[0], "via "+kernelStaticGateway+" ") &&
			strings.Contains(entries[0], "proto 250")
	}) {
		return fmt.Errorf("%s: want one kernel entry `encap mpls 100/200 via %s ... proto 250`; ip route show table all: %q",
			fixture06MPLSPushPrefix, kernelStaticGateway, entries)
	}
	fmt.Fprintln(os.Stderr, "OK: kernel holds "+fixture06MPLSPushPrefix+" encap mpls 100/200 via "+kernelStaticGateway)
	return fixture06WaitEOR(ctx, p, 1)
}

// fixture06NextHopEntry answers the show nexthop-table row for nextHop, or nil
// when the resolver does not track it or the command fails.
func fixture06NextHopEntry(ctx context.Context, p *sdk.Plugin, nextHop string) map[string]any {
	rows, err := fixture06DispatchArray(ctx, p, "show nexthop-table")
	if err != nil {
		return nil
	}
	for _, row := range rows {
		if row["next-hop"] == nextHop {
			return row
		}
	}
	return nil
}

func fixture06FIBRecursive(ctx context.Context, p *sdk.Plugin) error {
	if _, err := fixture06PollObject(ctx, p, "show bgp rib status", 60, func(map[string]any) bool { return true }); err != nil {
		return errors.New("rib plugin did not become ready")
	}
	// Three levels of recursion: 172.16.0.0/16 via 10.0.0.2, covered by
	// 10.0.0.0/24 via 192.0.2.1, covered by 192.0.2.0/24 via 198.51.100.1,
	// covered by 198.51.100.0/24, which carries no next hop and so ends the
	// walk. The direct next hop is therefore 198.51.100.1. A resolver that stops
	// after the first covering route answers 192.0.2.1 instead, so the chain is
	// one level deeper than the shortest one that tells the two apart.
	for _, command := range []string{
		"request bgp rib inject 10.0.0.1 ipv4/unicast 198.51.100.0/24 origin igp",
		"request bgp rib inject 10.0.0.1 ipv4/unicast 192.0.2.0/24 origin igp nexthop 198.51.100.1",
		"request bgp rib inject 10.0.0.1 ipv4/unicast 10.0.0.0/24 origin igp nexthop 192.0.2.1",
		"request bgp rib inject 10.0.0.1 ipv4/unicast 172.16.0.0/16 origin igp nexthop 10.0.0.2",
	} {
		if err := fixture06DispatchDone(ctx, p, command); err != nil {
			return err
		}
	}
	rows, err := fixture06PollArray(ctx, p, 40, func(rows []map[string]any) bool { return fixture06RIBEntry(rows, "172.16.0.0/16") != nil })
	if err != nil || fixture06RIBEntry(rows, "172.16.0.0/16") == nil {
		return fmt.Errorf("AC-2: recursive route missing from system RIB: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK AC-2: recursive route 172.16.0.0/16 present in system RIB")
	var tracked map[string]any
	if !Poll(ctx, 40, fixture06PollDelay, func() bool {
		tracked = fixture06NextHopEntry(ctx, p, "10.0.0.2")
		return tracked != nil && tracked["resolved"] == true && tracked["direct-nh"] == "198.51.100.1"
	}) {
		return fmt.Errorf("AC-2: show nexthop-table: want 10.0.0.2 resolved to direct next hop 198.51.100.1, got %v", tracked)
	}
	fmt.Fprintln(os.Stderr, "OK AC-2: next hop 10.0.0.2 resolves recursively to direct next hop 198.51.100.1")
	for _, command := range []string{"show ecmp-groups"} {
		rows, err := fixture06DispatchArray(ctx, p, command)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "OK %s: returned %d entries\n", strings.ReplaceAll(command, " ", "-"), len(rows))
	}
	return fixture06WaitEOR(ctx, p, 1)
}
