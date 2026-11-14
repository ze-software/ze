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
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
)

const (
	ospfSRAnycastScenario = "ospf-sr-anycast-nexthop-frr"
	ospfSRAnycastPrefix   = "10.255.255.100/32"
	// ospfSRAnycastInLabel is Ze's own label for index 100: SRGB 16000 + 100.
	ospfSRAnycastInLabel = "16100"
	// ospfPrefixSIDSubTLV is the Prefix-SID Sub-TLV type of RFC 8665 Section 5.
	ospfPrefixSIDSubTLV = 2
	ospfSRAnycastWait   = 90 * time.Second
	// ospfSRAnycastPoll is the interval every wait and the hold in this checker sample at.
	ospfSRAnycastPoll = time.Second
	// ospfSRAnycastSettle is how long assertion 3 holds the pop after its first passing
	// read. It bounds the time from the second advertiser's LSA entering Ze's LSDB, which
	// assertion 2 observed, to Ze's label route built from it. Ze builds its SR table only
	// in the SPF post-run hook (internal/plugins/ospf/spf_wiring.go), and the run that
	// reads the LSA starts at most DefaultSPFMaxHold after the LSA triggered it
	// (spf.Computer.TriggerArea, bumpDelayLocked doubles the hold up to that ceiling,
	// and a run already under way leaves the LSA to the next one). The scenario's ze.conf
	// sets no SPF timer, so the engine runs the config default ospf.DefaultSPFMaxHoldMS,
	// the same 5000 ms as the computer's own default read here. The entry then still
	// crosses the mpls-fib bus to fib-kernel before netlink writes it. The other two
	// ceilings cover that run, the bus and the kernel write on a loaded lab host. A single
	// read cannot work: the read that follows assertion 2 can see the pop Ze installed
	// before the LSA reached its SR table, while the next hop was the binding's only
	// advertiser, and a Ze that ignores a non-originating next hop's flags pops then too.
	// No `show` command exposes a fact that advances once the SR install consumed the
	// current LSDB, so the hold is bounded by time, not by state.
	ospfSRAnycastSettle = 3 * ospfspf.DefaultSPFMaxHold
	// ospfSRAnycastDatabase is the Ze command assertion 2 reads, and the hold re-reads.
	ospfSRAnycastDatabase = "show ospf database opaque-area"
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
// became the Originator fails rather than passing for the wrong reason. Assertion 3
// holds the pop for ospfSRAnycastSettle after its first passing read and then re-reads
// the condition, because the first pop can predate the condition (ospfSRAnycastSettle
// says why).
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
	if err := assertOSPFSRAnycastPop(ctx, check.Lab, nextHop, ospfSRAnycastSettle, ospfSRAnycastPoll); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	return nil
}

// assertOSPFSRAnycastPop is assertion 3. It waits for the first pop toward nextHop, holds
// it for settle, sampling every interval and failing on the first sample that is not
// that pop (a swap, no route, two routes, a pop elsewhere), and last re-reads Ze's LSDB,
// so every accepted pop was judged while the table still held the Originator first.
// settle and interval are parameters so a unit test can hold for milliseconds; the
// checker passes ospfSRAnycastSettle and ospfSRAnycastPoll.
func assertOSPFSRAnycastPop(ctx context.Context, lab interoplab.CheckerLab, nextHop netip.Addr, settle, interval time.Duration) error {
	if err := waitOSPFSRAnycastPop(ctx, lab, nextHop, interval); err != nil {
		return err
	}
	if err := holdOSPFSRAnycastPop(ctx, lab, nextHop, settle, interval); err != nil {
		return err
	}
	return recheckOSPFSRAnycastOriginator(ctx, lab)
}

func waitOSPFSRAnycastFull(ctx context.Context, lab interoplab.CheckerLab, peer string) error {
	command := []string{cmdVtysh, "-c", frrShowOSPFNeighbor}
	var last string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: ospfSRAnycastWait, Interval: ospfSRAnycastPoll, Description: "FRR Full with Ze",
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
	command := zeCommand(ospfSRAnycastDatabase)
	var last string
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: ospfSRAnycastWait, Interval: ospfSRAnycastPoll, Description: "both anycast Prefix-SIDs in Ze's LSDB",
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

// ospfSRAnycastLabelTable is the command that lists Ze's kernel label routes.
func ospfSRAnycastLabelTable() []string {
	return []string{"ip", "-f", ipFamilyMPLS, ipObjectRoute, "show"}
}

// waitOSPFSRAnycastPop returns at the first read that shows the pop. That read alone
// proves nothing: assertOSPFSRAnycastPop holds it.
func waitOSPFSRAnycastPop(ctx context.Context, lab interoplab.CheckerLab, nextHop netip.Addr, interval time.Duration) error {
	command := ospfSRAnycastLabelTable()
	var last string
	var verdict error
	answered := false
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: ospfSRAnycastWait, Interval: interval, Description: "Ze's label route for the anycast SID",
	}, func(ctx context.Context) (string, error) {
		output, err := lab.Query(ctx, "ze", command, nil)
		last = output
		return output, err
	}, func(output string) bool {
		answered = true
		verdict = ospfSRAnycastPopVerdict(output, nextHop)
		return verdict == nil
	})
	if err == nil {
		return nil
	}
	// A table that never answered holds no verdict on Ze, so it is a lab error.
	if !answered {
		return fmt.Errorf("lab error: `ip -f mpls route show` on Ze never answered, so no label route was judged: %w", err)
	}
	return fmt.Errorf("the next hop's own NP-Flag (RFC 8665 Section 5) was not applied by Ze: %w (last verdict: %s); last `ip -f mpls route show`: %s", err, ospfSRAnycastVerdictText(verdict), last)
}

// holdOSPFSRAnycastPop samples Ze's label table every interval until settle has passed
// since its start, and refuses the first sample that is not the pop toward nextHop. A
// sample that cannot be read is refused too, as a lab error rather than a verdict on
// Ze: an unread table is not a pop, and not a swap either. The last sample is taken
// at or after settle, so the hold covers the whole window.
func holdOSPFSRAnycastPop(ctx context.Context, lab interoplab.CheckerLab, nextHop netip.Addr, settle, interval time.Duration) error {
	command := ospfSRAnycastLabelTable()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	started := time.Now()
	for sample := 1; ; sample++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("the hold of the pop for in-label %s ended before %s, after %d samples: %w", ospfSRAnycastInLabel, settle, sample-1, ctx.Err())
		case <-ticker.C:
		}
		output, err := lab.Query(ctx, "ze", command, nil)
		if err != nil {
			return fmt.Errorf("lab error: sample %d of the hold could not run `ip -f mpls route show` on Ze, after the pop held for %s, so the hold judged no label route: %w", sample, time.Since(started).Round(time.Millisecond), err)
		}
		if verdict := ospfSRAnycastPopVerdict(output, nextHop); verdict != nil {
			return fmt.Errorf("the next hop's own NP-Flag (RFC 8665 Section 5) was not applied by Ze: the pop held for %s, then sample %d found %w; `ip -f mpls route show`: %s", time.Since(started).Round(time.Millisecond), sample, verdict, output)
		}
		if time.Since(started) >= settle {
			return nil
		}
	}
}

// recheckOSPFSRAnycastOriginator re-reads the condition assertion 2 established. A lab
// whose LSDB lost it during the hold (the next hop became the Originator, an advertiser
// withdrew) held its pops under a table that passes either way, so it fails.
func recheckOSPFSRAnycastOriginator(ctx context.Context, lab interoplab.CheckerLab) error {
	command := zeCommand(ospfSRAnycastDatabase)
	output, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return fmt.Errorf("re-read Ze's LSDB after the hold: %w", err)
	}
	if verdict := ospfSRAnycastOriginatorVerdict(output); verdict != nil {
		return fmt.Errorf("during the hold the Ze LSDB lost the condition the scenario tests, so its pops prove nothing: %w; last reply: %s", verdict, output)
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
