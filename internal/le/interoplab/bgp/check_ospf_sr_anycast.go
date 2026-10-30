// Design: docs/architecture/testing/interop.md -- the SR anycast next-hop scenario.
// Related: register_ospf_sr_anycast.go -- its registration.
// Related: prepare_ze_sysctls.go -- the MPLS label space the readback needs.
// RFC: rfc/short/rfc8665.md Section 5.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.3.1/ospfd/ospf_sr.c
// update_ext_prefix_sid and "segment-routing prefix" decide what the two FRRs send.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	ospfSRAnycastScenario = "ospf-sr-anycast-nexthop-frr"
	ospfSRAnycastPrefix   = "10.255.255.100/32"
	// ospfSRAnycastInLabel is Ze's own label for index 100: SRGB 16000 + 100.
	ospfSRAnycastInLabel = "16100"
	// ospfPrefixSIDSubTLV is the Prefix-SID Sub-TLV type of RFC 8665 Section 5.
	ospfPrefixSIDSubTLV = 2
	ospfSRAnycastWait   = 90 * time.Second
)

var (
	// ospfSRAnycastNextHop is frr.conf's router-id: Ze's only SPF next hop.
	ospfSRAnycastNextHop = netip.MustParseAddr("192.0.2.3")
	// ospfSRAnycastOriginator is frr-sink.conf's router-id: first in Ze's LSDB order.
	ospfSRAnycastOriginator = netip.MustParseAddr("192.0.2.1")
	ospfSRAnycastZeRouter   = netip.MustParseAddr("192.0.2.2")
)

// ospfSRAnycastPrefixLSA is the part of one decoded Extended Prefix Opaque LSA in
// `show ospf database opaque-area` the checker judges.
type ospfSRAnycastPrefixLSA struct {
	Router   netip.Addr `json:"advertising-router"`
	OpaqueID uint32     `json:"opaque-id"`
	Prefixes []struct {
		Prefix  string `json:"prefix"`
		SubTLVs []struct {
			Type uint16 `json:"type"`
		} `json:"sub-tlvs"`
	} `json:"prefixes"`
}

// checkOSPFSRAnycastNextHop proves RFC 8665 Section 5 at the router whose next hop
// advertised a Prefix-SID it is not the binding's Originator for. Assertion 3 is the
// discriminating one: Ze pops its own label for the anycast SID toward the next hop,
// as that router's NP-Flag asks, where a Ze reading the flags of the Originator alone
// swaps it. Assertions 1 and 2 establish the condition, so a lab where the next hop
// became the Originator fails rather than passing for the wrong reason.
func checkOSPFSRAnycastNextHop(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("assertion 1: the SR anycast lab has no selected IPv4 network")
	}
	for _, peer := range []string{peerFRR, peerFRRSink} {
		if err := waitOSPFSRAnycastFull(ctx, check.Lab, peer); err != nil {
			return fmt.Errorf("assertion 1: %s: %w", peer, err)
		}
	}
	if err := waitOSPFSRAnycastOriginator(ctx, check.Lab); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	nextHop, err := netip.ParseAddr(networkHostAddress(check.Network, frrHostNumber))
	if err != nil {
		return fmt.Errorf("assertion 3: FRR lab address: %w", err)
	}
	if err := waitOSPFSRAnycastPop(ctx, check.Lab, nextHop); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	return nil
}

func waitOSPFSRAnycastFull(ctx context.Context, lab interoplab.CheckerLab, peer string) error {
	command := []string{cmdVtysh, "-c", frrShowOSPFNeighbor}
	var last string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: ospfSRAnycastWait, Interval: time.Second, Description: "FRR Full with Ze",
	}, func(ctx context.Context) (string, error) {
		output, err := lab.Query(ctx, peer, command, queryEnvironment(peer, command))
		last = output
		return output, err
	}, func(output string) bool {
		return ospfSRAnycastFullWith(output, ospfSRAnycastZeRouter)
	})
	if err != nil {
		return fmt.Errorf("no Full adjacency with Ze %s: %w; last reply: %s", ospfSRAnycastZeRouter, err, last)
	}
	return nil
}

func waitOSPFSRAnycastOriginator(ctx context.Context, lab interoplab.CheckerLab) error {
	command := zeCommand("show ospf database opaque-area")
	var last string
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: ospfSRAnycastWait, Interval: time.Second, Description: "both anycast Prefix-SIDs in Ze's LSDB",
	}, func(ctx context.Context) (string, error) {
		output, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
		last = output
		return output, err
	}, func(output string) bool {
		verdict = ospfSRAnycastOriginatorVerdict(output)
		return verdict == nil
	})
	if err != nil {
		return fmt.Errorf("the Ze LSDB never held the condition the scenario tests: %w (last verdict: %s); last reply: %s", err, ospfSRAnycastVerdictText(verdict), last)
	}
	return nil
}

func waitOSPFSRAnycastPop(ctx context.Context, lab interoplab.CheckerLab, nextHop netip.Addr) error {
	command := []string{"ip", "-f", ipFamilyMPLS, ipObjectRoute, "show"}
	var last string
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: ospfSRAnycastWait, Interval: time.Second, Description: "Ze's label route for the anycast SID",
	}, func(ctx context.Context) (string, error) {
		output, err := lab.Query(ctx, "ze", command, nil)
		last = output
		return output, err
	}, func(output string) bool {
		verdict = ospfSRAnycastPopVerdict(output, nextHop)
		return verdict == nil
	})
	if err != nil {
		return fmt.Errorf("the next hop's own NP-Flag (RFC 8665 Section 5) was not applied by Ze: %w (last verdict: %s); last `ip -f mpls route show`: %s", err, ospfSRAnycastVerdictText(verdict), last)
	}
	return nil
}

// ospfSRAnycastFullWith reports whether one row of FRR's `show ip ospf neighbor`
// names router in state Full. The two tokens are judged on one row, so the Full of
// another neighbor cannot satisfy a row that names router in ExStart.
func ospfSRAnycastFullWith(output string, router netip.Addr) bool {
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] != router.String() {
			continue
		}
		if strings.HasPrefix(fields[2], ospfStateFull) {
			return true
		}
	}
	return false
}

// ospfSRAnycastOriginatorVerdict judges Ze's `show ospf database opaque-area` answer.
// Its extended-prefix rows come from the same LSDB walk, in the same order, as the
// table the SR installer builds (lsdb.OpaqueLSAsByType, then srPrefixSIDTable.add), and
// the first router to carry the SID becomes the binding's Originator
// (srPrefixSIDScope.resolve). The scenario tests something only when that first router
// is the one Ze does NOT route through, and the next hop advertised the same SID after
// it, so it is listed in Also. Each router MUST appear once: a router carrying the SID
// twice has every copy ignored (RFC 8665 Section 5), which is a different condition.
func ospfSRAnycastOriginatorVerdict(output string) error {
	var views []struct {
		ExtendedPrefix []ospfSRAnycastPrefixLSA `json:"extended-prefix"`
	}
	if err := json.Unmarshal([]byte(output), &views); err != nil {
		return fmt.Errorf("decode Ze's opaque-area database: %w", err)
	}
	var advertisers []netip.Addr
	for _, view := range views {
		for _, lsa := range view.ExtendedPrefix {
			if !ospfSRAnycastCarriesSID(&lsa) {
				continue
			}
			advertisers = append(advertisers, lsa.Router)
		}
	}
	if len(advertisers) != 2 {
		return fmt.Errorf("want the Prefix-SID for %s from exactly two routers, Ze holds it from %v", ospfSRAnycastPrefix, advertisers)
	}
	if advertisers[0] != ospfSRAnycastOriginator {
		return fmt.Errorf("the Prefix-SID for %s is read by Ze from %s first, so that router is the Originator; the scenario needs %s first, because the next hop %s being the Originator passes whether or not Ze applies a non-originating next hop's flags (compare the two FRRs' Extended Prefix Opaque IDs)",
			ospfSRAnycastPrefix, advertisers[0], ospfSRAnycastOriginator, ospfSRAnycastNextHop)
	}
	if advertisers[1] != ospfSRAnycastNextHop {
		return fmt.Errorf("the second advertiser of %s is %s, want the next hop %s", ospfSRAnycastPrefix, advertisers[1], ospfSRAnycastNextHop)
	}
	return nil
}

// ospfSRAnycastCarriesSID reports whether lsa binds a Prefix-SID to the anycast prefix.
func ospfSRAnycastCarriesSID(lsa *ospfSRAnycastPrefixLSA) bool {
	for _, prefix := range lsa.Prefixes {
		if prefix.Prefix != ospfSRAnycastPrefix {
			continue
		}
		for _, sub := range prefix.SubTLVs {
			if sub.Type == ospfPrefixSIDSubTLV {
				return true
			}
		}
	}
	return false
}

// ospfSRAnycastPopVerdict judges `ip -f mpls route show` in Ze's container: the route
// for Ze's own label ospfSRAnycastInLabel MUST be a pop toward nextHop. iproute2 prints a pop as
// "16100 via inet 172.30.0.3 dev eth0" and a swap as "16100 as to 17100 via inet ...".
// A Ze that treats the next hop as a transit router installs the swap, so the
// assertion is on a route that is present in both builds and differs between them.
func ospfSRAnycastPopVerdict(output string, nextHop netip.Addr) error {
	inLabel := ospfSRAnycastInLabel
	var route []string
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] != inLabel {
			continue
		}
		if route != nil {
			return fmt.Errorf("two label routes for in-label %s", inLabel)
		}
		route = fields
	}
	if route == nil {
		return fmt.Errorf("no label route for in-label %s", inLabel)
	}
	if slices.Contains(route[1:], "as") {
		return fmt.Errorf("in-label %s is swapped, not popped: %s", inLabel, strings.Join(route, " "))
	}
	if len(route) < 4 {
		return fmt.Errorf("in-label %s route names no next hop: %s", inLabel, strings.Join(route, " "))
	}
	if route[1] != "via" {
		return fmt.Errorf("in-label %s route names no next hop: %s", inLabel, strings.Join(route, " "))
	}
	if route[2] != "inet" {
		return fmt.Errorf("in-label %s route forwards to no IPv4 next hop: %s", inLabel, strings.Join(route, " "))
	}
	if route[3] != nextHop.String() {
		return fmt.Errorf("in-label %s is popped toward %s, want the SPF next hop %s", inLabel, route[3], nextHop)
	}
	return nil
}

// ospfSRAnycastVerdictText names the last judgement a wait made, or says that no
// probe ever answered, so a timeout message tells a refused state from no state.
func ospfSRAnycastVerdictText(verdict error) string {
	if verdict == nil {
		return "no probe answered"
	}
	return verdict.Error()
}
