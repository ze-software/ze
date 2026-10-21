// Design: docs/architecture/testing/interop.md -- typed scenario registry.
// Related: check_special.go -- specialCheckers names this checker.
// Related: check_vpn_withdraw.go -- frrVPNDecodes reads FRR's VPN decodes.
// RFC: rfc/short/rfc8277.md
package bgp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The bgp-vpn-withheld-withdraw-frr scenario. A raw injector announces, each
// under one label, vpnWithheldPrefix and vpnWithheldSibling at vpnWithheldRD
// in ipv4/mpls-vpn, and labeledWithheldPrefix and labeledWithheldSibling in
// ipv4/mpls-label. It then re-announces the two non-sibling routes under three
// labels with NO_EXPORT, which ze withholds from FRR, an external peer, and
// converts into withdrawals.
const (
	vpnWithheldScenario    = "bgp-vpn-withheld-withdraw-frr"
	vpnWithheldRD          = "65004:40"
	vpnWithheldPrefix      = "10.10.0.0/24"
	vpnWithheldSibling     = "10.11.0.0/24"
	vpnWithheldLabel       = 200
	labeledWithheldPrefix  = "10.20.0.0/24"
	labeledWithheldSibling = "10.21.0.0/24"
)

// checkVPNWithheldWithdraw proves end to end that the withdrawal ze's forward
// rail converts from a withheld labeled announcement carries the RFC 8277
// Section 2.4 framing, one Compatibility field, whatever label stack the
// source announced (spec-bgp-rs-replacement-on-withdrawal, AC-27).
//
// Method, in order, each step a separate observation:
//  1. FRR holds the four one-label routes, and its log holds one decode of the
//     VPN announcement under label 200, so the routes crossed ze.
//  2. FRR loses the VPN route, and its session generation with ze is
//     unchanged, so the route left through an UPDATE, never a session reset.
//  3. FRR's log holds exactly one decode of a withdrawal of RD 65004:40
//     10.10.0.0/24 from ze, and still one announcement: ze never sent the
//     three-label revision. FRR still holds the VPN sibling.
//  4. The ipv4/mpls-label leg: FRR loses 10.20.0.0/24 with one withdrawal
//     decode and keeps 10.21.0.0/24. This leg does not discriminate, because
//     FRR walks a copied stack to its bottom-of-stack bit.
//  5. FRR's session generation is still unchanged, and ze still holds the
//     injector session Established.
//
// The red: copy the announcement's NLRI into the withdrawal (forward_build.go,
// writeWithdrawalHalves bypassing the writer). Its Length 160 leaves FRR a
// 72-bit IPv4 prefix after the Compatibility field and the RD, so FRR refuses
// the UPDATE: step 2 sees a new session generation, or step 3 counts no
// withdrawal decode.
func checkVPNWithheldWithdraw(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return fmt.Errorf("%s has no selected IPv4 network", vpnWithheldScenario)
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
	for _, prefix := range []string{vpnWithheldPrefix, vpnWithheldSibling} {
		if err := waitFRRVPNRoute(ctx, check.Lab, vpnWithheldRD, prefix, 180*time.Second, true); err != nil {
			return fmt.Errorf("FRR never held the one-label VPN route %s ze relayed: %w", prefix, err)
		}
	}
	for _, prefix := range []string{labeledWithheldPrefix, labeledWithheldSibling} {
		if err := waitFRRRoute(ctx, check.Lab, prefix, frrFamilyIPv4Labeled, 60*time.Second, true); err != nil {
			return fmt.Errorf("FRR never held the one-label route %s ze relayed: %w", prefix, err)
		}
	}
	if err := requireVPNWithheldDecodes(ctx, check.Lab, neighbor, false, 1); err != nil {
		return err
	}
	// RFC 1997: a NO_EXPORT route MUST NOT be advertised to an external peer,
	// so ze withdraws the route FRR holds instead.
	if err := waitFRRVPNRoute(ctx, check.Lab, vpnWithheldRD, vpnWithheldPrefix, 150*time.Second, false); err != nil {
		return fmt.Errorf("FRR still holds the VPN route after the injector re-announced it with NO_EXPORT: %w", err)
	}
	if err := requireFRRSameSession(ctx, check.Lab, neighbor, generation); err != nil {
		return err
	}
	// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD
	// be set to 0x800000." One field, so FRR decodes the RD and the prefix.
	if err := requireVPNWithheldDecodes(ctx, check.Lab, neighbor, true, 1); err != nil {
		return err
	}
	if err := requireVPNWithheldDecodes(ctx, check.Lab, neighbor, false, 1); err != nil {
		return fmt.Errorf("after the withdrawal: %w", err)
	}
	if err := waitFRRVPNRoute(ctx, check.Lab, vpnWithheldRD, vpnWithheldSibling, 10*time.Second, true); err != nil {
		return fmt.Errorf("FRR lost the VPN sibling %s, which the injector never re-announced: %w", vpnWithheldSibling, err)
	}
	if err := waitFRRRoute(ctx, check.Lab, labeledWithheldPrefix, frrFamilyIPv4Labeled, 60*time.Second, false); err != nil {
		return fmt.Errorf("FRR still holds %s after the injector re-announced it with NO_EXPORT: %w", labeledWithheldPrefix, err)
	}
	if err := requireLabeledWithdrawals(ctx, check.Lab, neighbor, labeledWithheldPrefix, 1); err != nil {
		return err
	}
	if err := waitFRRRoute(ctx, check.Lab, labeledWithheldSibling, frrFamilyIPv4Labeled, 10*time.Second, true); err != nil {
		return fmt.Errorf("FRR lost %s, which the injector never re-announced: %w", labeledWithheldSibling, err)
	}
	if err := requireFRRSameSession(ctx, check.Lab, neighbor, generation); err != nil {
		return err
	}
	state, err := zePeerState(ctx, check.Lab, injector)
	if err != nil {
		return err
	}
	if state != peerStateEstablished {
		return fmt.Errorf("ze's session with the injector %s is %q, so the routes may have left FRR through the injector's departure rather than the converted withdrawals", injector, state)
	}
	return nil
}

// requireFRRSameSession requires FRR's session with ze to be the one counted
// as generation: a refused withdrawal resets it.
func requireFRRSameSession(ctx context.Context, lab interoplab.CheckerLab, neighbor string, generation uint64) error {
	now, err := queryFRRSessionGeneration(ctx, lab, neighbor)
	if err != nil {
		return fmt.Errorf("FRR's session with ze after the withdrawal: %w", err)
	}
	if now != generation {
		return fmt.Errorf("FRR's session with ze moved from generation %d to %d: the withdrawal ze sent was refused, which a copied label stack causes", generation, now)
	}
	return nil
}

// requireVPNWithheldDecodes reads FRR's log and requires exactly want decodes
// from ze of RD vpnWithheldRD prefix vpnWithheldPrefix: withdrawals when
// withdrawn is set, announcements under vpnWithheldLabel otherwise.
func requireVPNWithheldDecodes(ctx context.Context, lab interoplab.CheckerLab, neighbor string, withdrawn bool, want int) error {
	log, err := lab.Query(ctx, peerFRR, []string{cmdCat, frrLogPath}, nil)
	if err != nil {
		return fmt.Errorf("read FRR log: %w", err)
	}
	got := frrVPNDecodes(log, neighbor, vpnWithheldRD, vpnWithheldPrefix, withdrawn, vpnWithheldLabel)
	if got != want {
		kind := "announcements under label 200"
		if withdrawn {
			kind = "withdrawals"
		}
		return fmt.Errorf("FRR logged %d %s of RD %s %s from %s, want %d", got, kind, vpnWithheldRD, vpnWithheldPrefix, neighbor, want)
	}
	return nil
}

// waitFRRVPNRoute waits until FRR's VPN-IPv4 table at rd holds prefix
// (present) or no longer holds it.
func waitFRRVPNRoute(ctx context.Context, lab interoplab.CheckerLab, rd, prefix string, timeout time.Duration, present bool) error {
	show := []string{cmdVtysh, "-c", "show bgp ipv4 vpn rd " + rd}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     timeout,
		Interval:    2 * time.Second,
		Description: "FRR VPN route RD " + rd + " " + prefix,
	}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, peerFRR, show, nil)
	}, func(output string) bool {
		return strings.Contains(output, prefix) == present
	})
	return err
}
