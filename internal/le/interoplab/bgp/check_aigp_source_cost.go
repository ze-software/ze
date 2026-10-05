// Design: docs/architecture/testing/interop.md -- independent AIGP source-cost proof.
// Related: helper_aigp_source_cost.go -- real metric control, no UPDATE reinjection.
package bgp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkAIGPSourceCostFRR preserves the static/forwarded local-auto assertions,
// then observes RFC 7311 Section 3.4.3 through FRR and GoBGP's own decoders.
// MUTATION: forwardUpdateSection passes peer.Settings().AIGPLinkMetric instead
// of srcAIGPLinkMetric to applyFactsAIGP: FRR observes143 instead of107.
func checkAIGPSourceCostFRR(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkAIGPSelfNextHopBaseline(ctx, check); err != nil {
		return err
	}
	fail := func(err error) error {
		return checkerFailure(ctx, check.Lab, scenarioAIGPSourceCostFRR, 4, err)
	}
	zeAddress := networkHostAddress(check.Network, 2)
	sourceHop := networkHostAddress(check.Network, 9)
	thirdPartyHop := networkHostAddress(check.Network, 77)
	recipient, err := queryAIGPRecipientFence(ctx, check, zeAddress)
	if err != nil {
		return fail(err)
	}
	if err := waitAIGPControl(ctx, check, sourceHop, thirdPartyHop); err != nil {
		return fail(err)
	}
	if err := waitFRRAIGPRoute(ctx, check, aigpDirectPrefix, zeAddress); err != nil {
		return fail(err)
	}
	fence := operation{kind: opFRRRoute, argument: aigpFencePrefix, timeout: 45 * time.Second}
	if err := runOperation(ctx, check.Network, check.Lab, &fence); err != nil {
		return fail(err)
	}
	source, err := waitAIGPSourceFence(ctx, check, sourceHop)
	if err != nil {
		return fail(err)
	}
	// The sentinel follows every source announcement on this recipient's FIFO.
	// An attribute-free announcement is still a route and fails the inventory.
	inventory, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast json"}, nil)
	if err != nil {
		return fail(err)
	}
	if err := requireFRRAIGPInventory(inventory, false); err != nil {
		return fail(err)
	}
	if err := checkAIGPRecipientFence(ctx, check, zeAddress, recipient); err != nil {
		return fail(err)
	}
	for _, metric := range []uint64{7, 0, 7} {
		if err := installAIGPDistance(ctx, check, thirdPartyHop, metric); err != nil {
			return fail(err)
		}
		if metric != 0 {
			// RFC 7311 Section 3.4.3: recover from the retained received100,
			// not the prior advertised107, and without another source UPDATE.
			if err := waitFRRAIGPRoute(ctx, check, aigpRecoveryPrefix, zeAddress); err != nil {
				return fail(err)
			}
		}
		last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
			Timeout: 45 * time.Second, Interval: time.Second, Description: "FRR whole-route AIGP metric transition",
		}, func(probe context.Context) (string, error) {
			return check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast json"}, nil)
		}, func(output string) bool { return requireFRRAIGPInventory(output, metric != 0) == nil })
		if err != nil {
			return fail(withLastOutput(err, last))
		}
		if err := waitAIGPControl(ctx, check, sourceHop, thirdPartyHop); err != nil {
			return fail(err)
		}
		after, err := queryAIGPSourceFence(ctx, check, sourceHop)
		if err != nil {
			return fail(err)
		}
		if after != source {
			return fail(fmt.Errorf("metric recovery received new source traffic: before=%+v after=%+v", source, after))
		}
		if err := checkAIGPRecipientFence(ctx, check, zeAddress, recipient); err != nil {
			return fail(err)
		}
	}
	// The original local-auto next-hop proofs must survive every metric change.
	if err := checkAIGPSelfNextHopBaseline(ctx, check); err != nil {
		return err
	}
	if err := checkAIGPRecipientFence(ctx, check, zeAddress, recipient); err != nil {
		return fail(err)
	}
	return nil
}

func waitFRRAIGPRoute(ctx context.Context, check *interoplab.CheckContext, prefix, zeAddress string) error {
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "FRR decoded AIGP107 with next-hop self for " + prefix,
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + prefix + " json"}, nil)
	}, func(output string) bool { return requireFRRAIGPRoute(output, prefix, zeAddress, 107) == nil })
	return withLastOutput(err, last)
}

func waitAIGPControl(ctx context.Context, check *interoplab.CheckContext, sourceHop, thirdPartyHop string) error {
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "GoBGP unchanged-next-hop AIGP100 controls",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerGoBGP, []string{cmdGoBGP, gobgpGlobal, gobgpRIB, "-a", gobgpFamilyIPv4}, nil)
	}, func(output string) bool { return requireGoBGPAIGPControl(output, sourceHop, thirdPartyHop) == nil })
	return withLastOutput(err, last)
}

func installAIGPDistance(ctx context.Context, check *interoplab.CheckContext, hop string, metric uint64) error {
	var text textbuf.Buffer
	command := zeCommand(text.Str(aigpCostCommand).Byte(' ').Str(hop).Byte(' ').Uint(metric).String())
	answer, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return err
	}
	var receipt struct {
		Hop       string  `json:"next-hop"`
		Metric    *uint64 `json:"metric"`
		Installed uint32  `json:"installed"`
	}
	if err := json.Unmarshal([]byte(answer), &receipt); err != nil {
		return fmt.Errorf("decode metric control receipt: %w", err)
	}
	if receipt.Metric == nil {
		return fmt.Errorf("metric control returned no metric: %s", answer)
	}
	if receipt.Hop != hop {
		return fmt.Errorf("metric control installed a different next hop: %s", answer)
	}
	if *receipt.Metric != metric {
		return fmt.Errorf("metric control installed a different distance: %s", answer)
	}
	if receipt.Installed != 1 {
		return fmt.Errorf("metric control did not install one distance: %s", answer)
	}
	return nil
}

func queryAIGPSourceFence(ctx context.Context, check *interoplab.CheckContext, source string) (aigpSourceFence, error) {
	command := zeCommand("show bgp peer " + source + " detail")
	answer, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return aigpSourceFence{}, err
	}
	return readAIGPSourceFence(answer, source)
}

func waitAIGPSourceFence(ctx context.Context, check *interoplab.CheckContext, source string) (aigpSourceFence, error) {
	command := zeCommand("show bgp peer " + source + " detail")
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 45 * time.Second, Interval: time.Second, Description: "injector batch End-of-RIB before metric control",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze", command, queryEnvironment("ze", command))
	}, func(output string) bool { _, err := readAIGPSourceFence(output, source); return err == nil })
	if err != nil {
		return aigpSourceFence{}, withLastOutput(err, last)
	}
	return readAIGPSourceFence(last, source)
}

// checkAIGPSelfNextHopBaseline retains every assertion of the original scenario.
// FRR's "<next hop> from <peer>" text binds self to the connected local endpoint,
// rather than accidentally matching Ze only as the advertising peer.
func checkAIGPSelfNextHopBaseline(ctx context.Context, check *interoplab.CheckContext) error {
	operations := []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + injectPrefixSecond}, contains: []string{injectPrefixSecond, nextHopSelfFromZe}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + injectPrefixFirst}, contains: []string{injectPrefixFirst, nextHopSelfFromZe}, timeout: 60 * time.Second},
		{kind: opRequireAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + injectPrefixFirst}, absent: []string{injectorNextHopFromZe}, proof: []string{injectPrefixFirst, nextHopSelfFromZe}},
		{kind: opFRRSession, argument: zeLabAddress},
	}
	for index := range operations {
		if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
			return checkerFailure(ctx, check.Lab, scenarioAIGPSourceCostFRR, index+1, err)
		}
	}
	return nil
}

func queryAIGPRecipientFence(ctx context.Context, check *interoplab.CheckContext, address string) (aigpRecipientFence, error) {
	answer, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
	if err != nil {
		return aigpRecipientFence{}, err
	}
	return readFRRAIGPRecipientFence(answer, address)
}

func checkAIGPRecipientFence(ctx context.Context, check *interoplab.CheckContext, address string, want aigpRecipientFence) error {
	answer, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
	if err != nil {
		return err
	}
	return requireFRRAIGPRecipientFence(answer, address, want)
}
