// Design: docs/architecture/bgp/structural-forwarding.md -- default eBGP next-hop rewriting.
// Related: register_nexthop_auto_ebgp.go -- the standalone legal-address scenario.
// Upstream: FRR 10.7.1 bgpd/bgp_route.c::route_vty_out_detail, per-prefix JSON.
// https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const nextHopAutoEBGPScenario = "bgp-nexthop-auto-ebgp-frr"

// checkNextHopAutoEBGP observes a received route at real foreign receivers. The
// ordinary eBGP peer omits next-hop; its explicit local route is an independent
// useful-output control. Unchanged eBGP and default iBGP retain the source hop.
func checkNextHopAutoEBGP(ctx context.Context, check *interoplab.CheckContext) error {
	local := netip.MustParseAddr(networkHostAddress(check.Network, zeHostNumber))
	source := netip.MustParseAddr(networkHostAddress(check.Network, 9))
	for _, want := range []struct {
		peer   string
		prefix netip.Prefix
		hop    netip.Addr
		path   string
	}{
		{peerFRR, netip.MustParsePrefix(injectPrefixSecond), local, "65001"},
		{peerFRR, netip.MustParsePrefix(injectPrefixFirst), local, zeInjectorASPath},
		{peerFRRSink, netip.MustParsePrefix(injectPrefixFirst), source, zeInjectorASPath},
	} {
		if err := waitNextHopAutoFRR(ctx, check, want.peer, want.prefix, want.hop, local, want.path); err != nil {
			return fmt.Errorf("%s: %w", want.peer, err)
		}
	}
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second, Description: "iBGP keeps the received next hop",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerGoBGP, []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4}, nil)
	}, func(output string) bool {
		verdict = requireGoBGPSourceBlock(output, injectPrefixFirst, "65004", source.String())
		return verdict == nil
	})
	if err != nil {
		return fmt.Errorf("iBGP preservation: %w", errors.Join(err, verdict))
	}
	return nil
}

func waitNextHopAutoFRR(ctx context.Context, check *interoplab.CheckContext, receiver string, prefix netip.Prefix, hop, peer netip.Addr, path string) error {
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second, Description: receiver + " accepts " + prefix.String(),
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, receiver, []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + prefix.String() + " json"}, nil)
	}, func(output string) bool {
		verdict = requireNextHopAutoFRR(output, prefix, hop, peer, path)
		return verdict == nil
	})
	if err != nil {
		return fmt.Errorf("FRR acceptance: %w", errors.Join(err, verdict))
	}
	return nil
}

// requireNextHopAutoFRR reads FRR's detailed per-prefix JSON. A next hop in
// another field, an inaccessible route or an unselected path earns no pass.
// FRR 10.7.1 bgpd/bgp_route.c::route_vty_out_detail emits these fields from
// BGP_PATH_VALID and BGP_PATH_SELECTED, independently of Ze's sent-RIB claim.
func requireNextHopAutoFRR(output string, prefix netip.Prefix, hop, peer netip.Addr, asPath string) error {
	var document struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			Valid    bool `json:"valid"`
			BestPath struct {
				Overall bool `json:"overall"`
			} `json:"bestpath"`
			ASPath struct {
				String string `json:"string"`
			} `json:"aspath"`
			Peer struct {
				ID string `json:"peerId"`
			} `json:"peer"`
			NextHops []struct {
				IP         string `json:"ip"`
				Accessible bool   `json:"accessible"`
			} `json:"nexthops"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return fmt.Errorf("decode FRR route: %w", err)
	}
	gotPrefix, err := netip.ParsePrefix(document.Prefix)
	if err != nil {
		return fmt.Errorf("FRR prefix: %w", err)
	}
	if gotPrefix != prefix {
		return fmt.Errorf("FRR prefix %s, want %s", gotPrefix, prefix)
	}
	if len(document.Paths) != 1 {
		return fmt.Errorf("FRR has %d paths, want 1", len(document.Paths))
	}
	if !document.Paths[0].Valid {
		return fmt.Errorf("FRR route %s is not valid", prefix)
	}
	if !document.Paths[0].BestPath.Overall {
		return fmt.Errorf("FRR route %s is not selected", prefix)
	}
	if strings.Join(strings.Fields(document.Paths[0].ASPath.String), " ") != asPath {
		return fmt.Errorf("FRR AS_PATH %q, want %q", document.Paths[0].ASPath.String, asPath)
	}
	gotPeer, err := netip.ParseAddr(document.Paths[0].Peer.ID)
	if err != nil {
		return fmt.Errorf("FRR advertising peer: %w", err)
	}
	if gotPeer != peer {
		return fmt.Errorf("FRR advertising peer %s, want %s", gotPeer, peer)
	}
	if len(document.Paths[0].NextHops) != 1 {
		return fmt.Errorf("FRR has %d next hops, want 1", len(document.Paths[0].NextHops))
	}
	if !document.Paths[0].NextHops[0].Accessible {
		return fmt.Errorf("FRR next hop for %s is inaccessible", prefix)
	}
	gotHop, err := netip.ParseAddr(document.Paths[0].NextHops[0].IP)
	if err != nil {
		return fmt.Errorf("FRR next hop: %w", err)
	}
	if gotHop != hop {
		return fmt.Errorf("FRR next hop %s, want %s", gotHop, hop)
	}
	return nil
}
