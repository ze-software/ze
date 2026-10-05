// Design: docs/architecture/testing/interop.md -- typed scenario registry.
// Related: check_special.go -- specialCheckers names this checker.
// Related: check_rfc_predicate.go -- frrDecodeFields reads FRR's decode.
// RFC: rfc/short/rfc8277.md
package bgp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The bgp-labeled-withdraw-compatibility-frr scenario. A raw injector announces
// labeledWithdrawPrefix under labeledWithdrawLabel and labeledKeptPrefix under
// labeledKeptLabel in ipv4/mpls-label, then withdraws labeledWithdrawPrefix
// alone with the RFC 8277 Section 2.4 Compatibility field 0x800000 in the label
// position. VPN families additionally exercise distinct RDs; see check_vpn_withdraw.go.
const (
	labeledWithdrawScenario = "bgp-labeled-withdraw-compatibility-frr"
	labeledWithdrawPrefix   = "10.10.0.0/24"
	labeledWithdrawLabel    = 100
	labeledKeptPrefix       = "10.11.0.0/24"
	labeledKeptLabel        = 101
	frrFamilyIPv4Labeled    = "ipv4 labeled-unicast"
)

// checkLabeledWithdrawCompatibility proves end to end that ze reads a labeled
// withdrawal whose label field holds the Compatibility value as the removal of
// its prefix.
//
// bgp-rs relays the withdrawal's bytes unchanged, and FRR parses them itself,
// so FRR losing 10.10.0.0/24 proves nothing about ze's own reading. ze's reading
// decides what bgp-rs keeps in its route inventory, and that inventory is what
// bgp-rs withdraws when the injector's session goes down. The scenario
// therefore ends the injector's session and counts what ze withdraws then.
//
// Method, in order, each step a separate observation:
//  1. FRR holds 10.10.0.0/24 with remote label 100 and 10.11.0.0/24 with
//     remote label 101, so both announcements crossed ze in the family under
//     test.
//  2. FRR loses 10.10.0.0/24 and still holds 10.11.0.0/24, and its log holds
//     exactly one decode of an UPDATE from ze withdrawing 10.10.0.0/24.
//  3. ze still holds the injector session Established, so that withdrawal was
//     the relayed UPDATE, never a session reset.
//  4. The checker stops the injector, and FRR loses 10.11.0.0/24: bgp-rs ran
//     its peer-down withdrawals, and FRR logged the withdrawal of 10.11.0.0/24.
//  5. FRR's log still holds exactly one withdrawal of 10.10.0.0/24, so bgp-rs
//     no longer held it when the session went down.
//
// The red: frame the withdrawal with the announcement reader and bgp-rs keeps
// 10.10.0.0/24 in its inventory, so step 5 counts a second withdrawal of it.
func checkLabeledWithdrawCompatibility(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return fmt.Errorf("%s has no selected IPv4 network", labeledWithdrawScenario)
	}
	neighbor := networkHostAddress(check.Network, 2)
	injector := networkHostAddress(check.Network, 9)
	if err := waitContains(ctx, check.Lab, peerFRR, frrNeighborCommand(neighbor), 90*time.Second, "BGP state = Established"); err != nil {
		return err
	}
	generation, err := queryFRRSessionGeneration(ctx, check.Lab, neighbor)
	if err != nil {
		return err
	}
	if err := waitFRRLabeledRoute(ctx, check.Lab, labeledWithdrawPrefix, labeledWithdrawLabel); err != nil {
		return err
	}
	if err := waitFRRLabeledRoute(ctx, check.Lab, labeledKeptPrefix, labeledKeptLabel); err != nil {
		return err
	}
	// RFC 4364 Section 4.1 and RFC 4659 Section 2: RD distinguishes equal prefixes.
	if err := waitVPNWithdrawState(ctx, check, vpnWithdrawAnnounced); err != nil {
		return err
	}
	if err := requireVPNAnnouncementLabels(ctx, check.Lab, neighbor); err != nil {
		return err
	}
	// RFC 8277 Section 2.4: the withdrawal carries 0x800000 in the label field.
	if err := waitFRRRoute(ctx, check.Lab, labeledWithdrawPrefix, frrFamilyIPv4Labeled, 150*time.Second, false); err != nil {
		return fmt.Errorf("FRR still holds %s after the injector withdrew it with Compatibility 0x800000: %w", labeledWithdrawPrefix, err)
	}
	if err := requireLabeledWithdrawals(ctx, check.Lab, neighbor, labeledWithdrawPrefix, 1); err != nil {
		return err
	}
	if err := waitFRRRoute(ctx, check.Lab, labeledKeptPrefix, frrFamilyIPv4Labeled, 10*time.Second, true); err != nil {
		return fmt.Errorf("FRR lost %s, which the injector never withdrew: %w", labeledKeptPrefix, err)
	}
	// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
	// MUST be ignored." Both values must remove only their own RD/prefix.
	if err := waitVPNWithdrawState(ctx, check, vpnWithdrawTargetsGone); err != nil {
		return err
	}
	if err := requireVPNWithdrawalCounts(ctx, check.Lab, neighbor, false); err != nil {
		return err
	}
	state, err := zePeerState(ctx, check.Lab, injector)
	if err != nil {
		return err
	}
	if state != peerStateEstablished {
		return fmt.Errorf("ze's session with the injector %s is %q, so the route may have left FRR through a session reset rather than the withdrawal", injector, state)
	}
	if err := check.Lab.Signal(ctx, peerInject, signalTERM); err != nil {
		return fmt.Errorf("stop the injector: %w", err)
	}
	if err := waitFRRRoute(ctx, check.Lab, labeledKeptPrefix, frrFamilyIPv4Labeled, 120*time.Second, false); err != nil {
		return fmt.Errorf("FRR still holds %s after the injector stopped, so bgp-rs ran no peer-down withdrawal: %w", labeledKeptPrefix, err)
	}
	if err := waitVPNWithdrawState(ctx, check, vpnWithdrawAllGone); err != nil {
		return err
	}
	// A later KEEPALIVE on the unchanged FRR session fences the final log read
	// behind the peer-down UPDATEs, rather than racing their decodes.
	if err := waitVPNWithdrawFence(ctx, check.Lab, neighbor, generation); err != nil {
		return err
	}
	if err := requireVPNWithdrawalCounts(ctx, check.Lab, neighbor, true); err != nil {
		return err
	}
	if err := requireLabeledWithdrawals(ctx, check.Lab, neighbor, labeledKeptPrefix, 1); err != nil {
		return err
	}
	if err := requireLabeledWithdrawals(ctx, check.Lab, neighbor, labeledWithdrawPrefix, 1); err != nil {
		return fmt.Errorf("after the injector stopped: %w", err)
	}
	return waitContains(ctx, check.Lab, peerFRR, frrNeighborCommand(neighbor), 30*time.Second, "BGP state = Established")
}

func frrNeighborCommand(neighbor string) []string {
	var command textbuf.Buffer
	return []string{cmdVtysh, "-c", command.Str("show bgp neighbor ").Str(neighbor).String()}
}

// requireLabeledWithdrawals reads FRR's log and requires exactly want decodes
// of an UPDATE from ze that withdrew prefix.
func requireLabeledWithdrawals(ctx context.Context, lab interoplab.CheckerLab, neighbor, prefix string, want int) error {
	log, err := lab.Query(ctx, peerFRR, []string{cmdCat, frrLogPath}, nil)
	if err != nil {
		return fmt.Errorf("read FRR log: %w", err)
	}
	got := frrWithdrawalDecodes(log, neighbor, prefix)
	if got != want {
		return fmt.Errorf("FRR logged %d withdrawals of %s from %s, want %d", got, prefix, neighbor, want)
	}
	return nil
}

// frrWithdrawalDecodes counts the lines of log that are FRR's own decode of an
// UPDATE from ze (neighbor) that withdrew prefix. The withdrawn marker is required on
// the same line as the prefix, so the decode of its announcement never counts.
func frrWithdrawalDecodes(log, neighbor, prefix string) int {
	count := 0
	for line := range strings.SplitSeq(log, "\n") {
		if slices.Contains(frrDecodeFields(line, neighbor, prefix), frrWithdrawnMarker) {
			count++
		}
	}
	return count
}

// waitFRRLabeledRoute waits until FRR's labeled-unicast table holds prefix with
// remote label label.
func waitFRRLabeledRoute(ctx context.Context, lab interoplab.CheckerLab, prefix string, label int) error {
	var command textbuf.Buffer
	show := []string{cmdVtysh, "-c", command.Str("show bgp ").Str(frrFamilyIPv4Labeled).Byte(' ').Str(prefix).Str(" json").String()}
	answer, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     180 * time.Second,
		Interval:    2 * time.Second,
		Description: "FRR labeled-unicast route " + prefix,
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peerFRR, show, nil)
	}, func(output string) bool {
		return frrRemoteLabel(output) == label
	})
	if err != nil {
		return fmt.Errorf("FRR never held %s with remote label %d: %w\n%s", prefix, label, err, strings.TrimSpace(answer))
	}
	return nil
}

// frrRemoteLabel answers the remoteLabel FRR prints for the route in output,
// or -1 when the answer is not JSON or carries no numeric remoteLabel.
func frrRemoteLabel(output string) int {
	var document any
	if json.Unmarshal([]byte(output), &document) != nil {
		return -1
	}
	value, ok := findJSONField(document, "remoteLabel")
	if !ok {
		return -1
	}
	label, isNumber := value.(float64)
	if !isNumber {
		return -1
	}
	return int(label)
}
