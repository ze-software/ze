// Design: docs/architecture/testing/interop.md -- the IGP import-policy scenarios.
// Related: check_rib_candidates_predicate.go -- the pure predicates these bodies call.
// Related: register_import_policy.go -- the registration of both checkers and of the IS-IS links.
package bgp

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	isisImportPolicyScenario = "isis-import-policy-frr"
	ospfImportPolicyScenario = "ospf-import-policy-bird"
	importPolicyWait         = 90 * time.Second
	// isisImportAttachment is the chain the IS-IS plugin registers
	// (policy.RIBImport.Attachment in internal/plugins/isis/register.go).
	isisImportAttachment = "isis import"
	// hiddenBySource is ribpolicy.ChainRoleSource's name in the JSON.
	hiddenBySource = "source"
	sourceISIS     = "isis"
	sourceOSPF     = "ospf"
	sourceStatic   = "static"
)

// The two IS-IS links (register_import_policy.go): FRR-A (frr) meets Ze on
// VLAN 101, FRR-B (frr-sink) on VLAN 102, and the two FRRs never meet.
var (
	isisImportLinkA = netip.MustParsePrefix("10.101.0.0/24")
	isisImportLinkB = netip.MustParsePrefix("10.102.0.0/24")
	// isisImportZeOnLinkB is Ze's address on FRR-B's link: the next hop of
	// every route FRR-B learns, since Ze is its only neighbor.
	isisImportZeOnLinkB = netip.AddrFrom4([4]byte{10, 102, 0, zeHostNumber})
)

// checkISISImportPolicy proves that `isis filter import` decides what Ze
// installs and nothing else. Assertions 1 and 2 are the two adjacencies, so a
// dead link cannot pass what follows. Assertions 3 to 6 read Ze: the control
// prefix installed, both prefixes in IS-IS's own route table (its SPF result),
// the hidden one absent from `show rib` and the kernel FIB, each absence read
// beside the control's presence. Assertion 7 binds the hidden path's verdict
// (hidden by the IS-IS source chain) and the control path's (selected, not
// hidden) to their own paths. Assertion 8 is the flooding proof: FRR-B, whose only neighbor is Ze,
// learns BOTH prefixes through Ze, so Ze flooded on the LSP whose prefix it
// kept out of its own RIB.
func checkISISImportPolicy(ctx context.Context, check *interoplab.CheckContext) error {
	hidden := netip.MustParsePrefix(isisImportHiddenPrefix)
	control := netip.MustParsePrefix(isisImportControlPrefix)
	operations := []operation{
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: peerFRRSink, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: "ze", command: zeCommand("show rib"), contains: []string{isisImportControlPrefix}, timeout: importPolicyWait},
		// `show isis route` is the SPF result, in CIDR text, which the import chain
		// runs after and never filters. `show isis database detail` prints TLV
		// values as hex, so a prefix can never be found in it.
		{kind: opRequireContains, peer: "ze", command: zeCommand("show isis route"), contains: []string{isisImportHiddenPrefix, isisImportControlPrefix}},
		{kind: opRequireAbsent, peer: "ze", command: zeCommand("show rib"), absent: []string{isisImportHiddenPrefix}, proof: []string{isisImportControlPrefix}},
		{kind: opRequireAbsent, peer: "ze", command: []string{"ip", "-4", ipObjectRoute, ipActionShow}, absent: []string{isisImportHiddenPrefix}, proof: []string{isisImportControlPrefix}},
	}
	for index := range operations {
		if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
			return fmt.Errorf("assertion %d: %w", index+1, err)
		}
	}
	if err := waitRIBCandidates(ctx, check.Lab, hidden, []ribCandidateWant{
		{source: sourceISIS, hidden: true, hiddenBy: hiddenBySource, chain: isisImportAttachment},
	}); err != nil {
		return fmt.Errorf("assertion 7: %w", err)
	}
	if err := waitRIBCandidates(ctx, check.Lab, control, []ribCandidateWant{
		{source: sourceISIS, selected: true},
	}); err != nil {
		return fmt.Errorf("assertion 7: %w", err)
	}
	for _, prefix := range []netip.Prefix{hidden, control} {
		if err := waitFRRISISRoute(ctx, check.Lab, peerFRRSink, prefix, isisImportZeOnLinkB); err != nil {
			return fmt.Errorf("assertion 8: %w", err)
		}
	}
	return nil
}

// checkOSPFImportPolicy proves that `ospf filter import` can change a path's
// distance and so its selection. Assertion 1 is the adjacency. Assertion 2
// reads the tagged external: its OSPF path carries the distance 200 the modify
// set and loses to the static path at 150. Assertion 3 reads the untagged one:
// its OSPF path keeps 110 and wins. Each field is judged on its own path, so
// the static path's 150 cannot stand for the OSPF path's distance.
func checkOSPFImportPolicy(ctx context.Context, check *interoplab.CheckContext) error {
	adjacency := operation{kind: opWaitContains, peer: peerBIRD, command: []string{cmdBirdc, "show ospf neighbors ospf0"}, contains: []string{ospfStateFull}, timeout: 120 * time.Second}
	if err := runOperation(ctx, check.Network, check.Lab, &adjacency); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if err := waitRIBCandidates(ctx, check.Lab, netip.MustParsePrefix(ospfImportTaggedPrefix), []ribCandidateWant{
		{source: sourceOSPF, distanceJudged: true, distance: 200},
		{source: sourceStatic, selected: true, distanceJudged: true, distance: 150},
	}); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	if err := waitRIBCandidates(ctx, check.Lab, netip.MustParsePrefix(ospfImportUntaggedPrefix), []ribCandidateWant{
		{source: sourceOSPF, selected: true, distanceJudged: true, distance: 110},
		{source: sourceStatic, distanceJudged: true, distance: 150},
	}); err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	return nil
}

// waitRIBCandidates waits until Ze's `show rib candidates prefix <prefix>`
// holds exactly wants (ribCandidatesVerdict), and reports the last verdict
// and reply when it never does.
func waitRIBCandidates(ctx context.Context, lab interoplab.CheckerLab, prefix netip.Prefix, wants []ribCandidateWant) error {
	command := zeCommand("show rib candidates prefix " + prefix.String())
	var last string
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: importPolicyWait, Interval: time.Second, Description: "Ze's candidate paths for " + prefix.String(),
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, "ze", command, queryEnvironment("ze", command))
		last = output
		return output, queryErr
	}, func(output string) bool {
		verdict = ribCandidatesVerdict(output, prefix, wants)
		return verdict == nil
	})
	if err != nil {
		return fmt.Errorf("%w (last verdict: %w); last reply: %s", err, verdict, last)
	}
	return nil
}

// waitFRRISISRoute waits until peer's FRR holds a selected IS-IS route for
// prefix through nextHop (frrISISRouteVerdict).
func waitFRRISISRoute(ctx context.Context, lab interoplab.CheckerLab, peer string, prefix netip.Prefix, nextHop netip.Addr) error {
	command := []string{cmdVtysh, "-c", "show ip route " + prefix.String() + " json"}
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: importPolicyWait, Interval: time.Second, Description: peer + "'s IS-IS route for " + prefix.String(),
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peer, command, queryEnvironment(peer, command))
	}, func(output string) bool {
		verdict = frrISISRouteVerdict(output, prefix, nextHop)
		return verdict == nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w (last verdict: %w)", peer, err, verdict)
	}
	return nil
}
