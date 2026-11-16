// Design: docs/architecture/testing/interop.md -- received marked policy output at FRR.
// Related: helper_atomic_policy.go -- public SDK answers, never direct reactor calls.
// FRR10.7.1: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.c
// bgp_attr_atomic accepts the zero-length marker. In bgp_route.c,
// bgp_nlri_parse_ip/bgp_withdraw apply withdrawals and route_vty_out_detail
// prints the path's metric, peer, valid flag and atomicAggregate marker.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkAtomicPolicyFRR requires a real baseline before any negative verdict.
// A later allowed prefix fences the final table; no silence interval proves
// absence. A guard bypass leaks A/25 or B, and omitted retirement leaves D.
func checkAtomicPolicyFRR(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("atomic policy scenario has no selected IPv4 network")
	}
	assertion := 0
	defer func() {
		if resultErr != nil {
			resultErr = checkerFailure(ctx, check.Lab, atomicPolicyScenario, assertion, resultErr)
		}
	}()
	for _, host := range []uint8{9, 3} {
		assertion++
		if err := waitAtomicPolicyZeSession(ctx, check, networkHostAddress(check.Network, host)); err != nil {
			return err
		}
	}
	ze, err := netip.ParseAddr(networkHostAddress(check.Network, 2))
	if err != nil {
		return err
	}
	neighbor := []string{cmdVtysh, "-c", "show bgp neighbor " + ze.String() + " json"}
	assertion++
	if err := waitJSONFields(ctx, check.Lab, peerFRR, neighbor, 60*time.Second,
		map[string]string{frrSessionStateKey: stateEstablished}, nil); err != nil {
		return err
	}
	initial, err := check.Lab.Query(ctx, peerFRR, neighbor, nil)
	if err != nil {
		return err
	}
	generation, err := parseFRRSessionGeneration(initial, ze.String())
	if err != nil {
		return err
	}
	if generation != 1 {
		return errors.New("atomic policy FRR baseline does not retain its first session")
	}
	assertion++
	if err := sendAtomicPolicyRelease(ctx, check, "baseline"); err != nil {
		return err
	}
	for _, ordinal := range []uint8{1, 2, 4, 5, 254} {
		assertion++
		if err := waitAtomicPolicyRoute(ctx, check.Lab, atomicPolicyPrefix(ordinal, 24), ze, true, 77); err != nil {
			return fmt.Errorf("atomic policy baseline: %w", err)
		}
	}
	assertion++
	if err := sendAtomicPolicyRelease(ctx, check, "mutation"); err != nil {
		return err
	}
	assertion++
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "FRR's fenced atomic policy result",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast json"}, nil)
	}, func(output string) bool {
		// refuse-by-default: guards Wait -- false retries the native state; the bounded wait returns a failure.
		return requireAtomicPolicyFinal(output) == nil
	})
	if err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("atomic policy final table: %w", err), last)
	}
	for _, expected := range []struct {
		ordinal uint8
		bits    int
		marked  bool
		metric  uint32
	}{{3, 25, false, 77}, {5, 24, true, 88}, {254, 24, true, 77}, {255, 24, false, 77}} {
		assertion++
		if err := waitAtomicPolicyRoute(ctx, check.Lab,
			atomicPolicyPrefix(expected.ordinal, expected.bits), ze, expected.marked, expected.metric); err != nil {
			return fmt.Errorf("atomic policy useful survivor: %w", err)
		}
	}
	assertion++
	source, err := netip.ParseAddr(networkHostAddress(check.Network, 9))
	if err != nil {
		return err
	}
	stored := zeCommand("show bgp adj-rib-in")
	last, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "Ze retains the original source keys after export policy",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", stored, queryEnvironment("ze", stored))
	}, func(output string) bool { return requireAtomicPolicySource(output, source) == nil })
	if err != nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("atomic policy source retention: %w", err), last)
	}
	assertion++
	final, err := check.Lab.Query(ctx, peerFRR, neighbor, nil)
	if err != nil {
		return err
	}
	after, err := parseFRRSessionGeneration(final, ze.String())
	if err != nil {
		return err
	}
	if after != generation {
		return errors.New("atomic policy handling replaced the FRR session")
	}
	for _, host := range []uint8{9, 3} {
		assertion++
		if err := waitAtomicPolicyZeSession(ctx, check, networkHostAddress(check.Network, host)); err != nil {
			return err
		}
	}
	return nil
}

func atomicPolicyPrefix(ordinal uint8, bits int) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 42, ordinal, 0}), bits)
}

func waitAtomicPolicyZeSession(ctx context.Context, check *interoplab.CheckContext, address string) error {
	command := zeCommand("show bgp peer " + address + " detail")
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second, Description: "atomic policy first Ze session and initial EOR",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", command, queryEnvironment("ze", command))
	}, func(output string) bool { return requireSRv6IdentityZeSession(output, address) == nil })
	if err != nil {
		return interoplab.WithSearchedOutput(err, last)
	}
	return nil
}

func sendAtomicPolicyRelease(ctx context.Context, check *interoplab.CheckContext, phase string) error {
	command := zeCommand(atomicPolicyRelease + " source " + networkHostAddress(check.Network, 9) +
		" next-hop " + networkHostAddress(check.Network, 2) + " phase " + phase)
	output, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return err
	}
	var receipt struct {
		Announced *uint32 `json:"announced"`
		Withdrawn *uint32 `json:"withdrawn"`
	}
	if err := json.Unmarshal([]byte(output), &receipt); err != nil {
		return err
	}
	if receipt.Announced == nil {
		return errors.New("atomic policy source release has no announcement receipt")
	}
	if receipt.Withdrawn == nil {
		return errors.New("atomic policy source release has no withdrawal receipt")
	}
	if *receipt.Announced != 1 {
		return errors.New("atomic policy source release did not announce exactly one token")
	}
	if *receipt.Withdrawn != 0 {
		return errors.New("atomic policy source release unexpectedly withdrew a route")
	}
	return nil
}

func waitAtomicPolicyRoute(ctx context.Context, lab interoplab.CheckerLab, prefix netip.Prefix, peer netip.Addr, marked bool, metric uint32) error {
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "FRR decodes atomic policy route " + prefix.String(),
	}, func(probe context.Context) (string, error) {
		return queryFRRIPv4Route(probe, lab, prefix.String())
	}, func(output string) bool { return requireAtomicPolicyRoute(output, prefix, peer, marked, metric) == nil })
	if err != nil {
		return interoplab.WithSearchedOutput(err, last)
	}
	return nil
}

// requireAtomicPolicyRoute reads fields from the same sole native path.
// FRR10.7.1 bgp_route.c:route_vty_out_detail emits these JSON fields.
func requireAtomicPolicyRoute(output string, prefix netip.Prefix, peer netip.Addr, marked bool, metric uint32) error {
	var route struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Valid           *bool   `json:"valid"`
			Metric          *uint32 `json:"metric"`
			AtomicAggregate bool    `json:"atomicAggregate"`
			ASPath          struct {
				String string `json:"string"`
			} `json:"aspath"`
			Peer struct {
				ID string `json:"peerId"`
			} `json:"peer"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &route); err != nil {
		return err
	}
	actual, err := netip.ParsePrefix(route.Prefix)
	if err != nil {
		return err
	}
	if actual != prefix {
		return fmt.Errorf("FRR answered prefix %s, want %s", actual, prefix)
	}
	if len(route.Paths) != 1 {
		return fmt.Errorf("FRR holds %d paths for %s, want one", len(route.Paths), prefix)
	}
	path := &route.Paths[0]
	if path.Valid == nil {
		return errors.New("FRR atomic policy path has no validity field")
	}
	if !*path.Valid {
		return errors.New("FRR atomic policy path is invalid")
	}
	if path.Metric == nil {
		return errors.New("FRR atomic policy path has no MED")
	}
	if *path.Metric != metric {
		return fmt.Errorf("FRR MED=%d, want the actual SDK output %d", *path.Metric, metric)
	}
	if path.AtomicAggregate != marked {
		return fmt.Errorf("FRR atomicAggregate=%t, want %t", path.AtomicAggregate, marked)
	}
	address, err := netip.ParseAddr(path.Peer.ID)
	if err != nil {
		return err
	}
	if address != peer {
		return fmt.Errorf("FRR path belongs to peer %s, want %s", address, peer)
	}
	asPath := strings.Fields(path.ASPath.String)
	if len(asPath) != 1 {
		return fmt.Errorf("FRR AS_PATH %q does not contain the sole source AS", path.ASPath.String)
	}
	sourceASN, err := asn.Parse(asPath[0])
	if err != nil {
		return err
	}
	if sourceASN != 65004 {
		return fmt.Errorf("FRR source AS=%d, want 65004", sourceASN)
	}
	return nil
}

// requireAtomicPolicyFinal never reads an unavailable/empty query as absence.
// Every negative shares a validated snapshot with useful routes and a later
// permitted source prefix. FRR10.7.1 bgp_show emits the native routes map.
func requireAtomicPolicyFinal(output string) error {
	for _, expected := range []struct {
		ordinal uint8
		bits    int
	}{{3, 25}, {5, 24}, {254, 24}, {255, 24}} {
		prefix := atomicPolicyPrefix(expected.ordinal, expected.bits)
		present, measured := jsonHasRoute(output, prefix.String())
		if !measured {
			return errors.New("FRR atomic policy table is not a measured native route snapshot")
		}
		if !present {
			return fmt.Errorf("FRR atomic policy table omits useful route %s", prefix)
		}
	}
	for _, excluded := range []struct {
		ordinal uint8
		bits    int
	}{{1, 24}, {1, 25}, {2, 24}, {3, 24}, {4, 24}} {
		prefix := atomicPolicyPrefix(excluded.ordinal, excluded.bits)
		present, measured := jsonHasRoute(output, prefix.String())
		if !measured {
			return errors.New("FRR atomic policy exclusion query is not a measured snapshot")
		}
		if present {
			return fmt.Errorf("FRR atomic policy table retains forbidden or excluded route %s", prefix)
		}
	}
	return nil
}

// requireAtomicPolicySource binds all seven original /24 identities to the
// actual source's retained table. Export policy must not rewrite or delete it.
// Producer: adj_rib_in/rib_commands.go:show emits family:route keys per peer.
func requireAtomicPolicySource(output string, source netip.Addr) error {
	var stored struct {
		Peers map[string][]struct {
			Key string `json:"key"`
		} `json:"adj-rib-in"`
	}
	if err := json.Unmarshal([]byte(output), &stored); err != nil {
		return err
	}
	if stored.Peers == nil {
		return errors.New("Ze source-retention query has no Adj-RIB-In table")
	}
	routes, found := stored.Peers[source.String()]
	if !found {
		return errors.New("Ze source-retention query omits the actual source peer")
	}
	if len(routes) != 7 {
		return fmt.Errorf("Ze retains %d source routes, want seven original keys", len(routes))
	}
	wanted := make(map[netip.Prefix]struct{}, 7)
	for _, ordinal := range []uint8{1, 2, 3, 4, 5, 254, 255} {
		wanted[atomicPolicyPrefix(ordinal, 24)] = struct{}{}
	}
	for _, route := range routes {
		separator := strings.LastIndexByte(route.Key, ':')
		if separator < 0 {
			return errors.New("Ze retained source key has no family separator")
		}
		prefix, err := netip.ParsePrefix(route.Key[separator+1:])
		if err != nil {
			return err
		}
		if _, found := wanted[prefix]; !found {
			return fmt.Errorf("Ze retained an unexpected or duplicate source key %q", route.Key)
		}
		delete(wanted, prefix)
	}
	if len(wanted) != 0 {
		return errors.New("Ze source-retention query is missing an original route")
	}
	return nil
}
