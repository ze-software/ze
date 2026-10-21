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

// The bgp-vpn-withheld-withdraw-frr scenario. A raw injector announces
// vpnWithheldPrefix at vpnWithheldRD under the one label vpnWithheldLabel,
// then re-announces it under three labels with NO_EXPORT, which ze withholds
// from FRR, an external peer, and converts into a withdrawal.
const (
	vpnWithheldScenario = "bgp-vpn-withheld-withdraw-frr"
	vpnWithheldRD       = "65004:40"
	vpnWithheldPrefix   = "10.20.0.0/24"
	vpnWithheldLabel    = 200
)

// checkVPNWithheldWithdraw proves end to end that the withdrawal ze's forward
// rail converts from a withheld labeled announcement carries the RFC 8277
// Section 2.4 framing, one Compatibility field, whatever label stack the
// source announced (spec-bgp-rs-replacement-on-withdrawal, AC-27).
//
// Method, in order, each step a separate observation:
//  1. FRR holds RD 65004:40 10.20.0.0/24, and its log holds one decode of that
//     announcement from ze under label 200, so the one-label route crossed ze.
//  2. FRR loses the route, and its session generation with ze is unchanged, so
//     the route left through an UPDATE, never a session reset.
//  3. FRR's log holds exactly one decode of a withdrawal of RD 65004:40
//     10.20.0.0/24 from ze, and still one announcement: ze never sent the
//     three-label revision.
//  4. ze still holds the injector session Established.
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
	if err := waitFRRVPNRoute(ctx, check.Lab, vpnWithheldRD, vpnWithheldPrefix, 180*time.Second, true); err != nil {
		return fmt.Errorf("FRR never held the one-label route ze relayed: %w", err)
	}
	if err := requireVPNWithheldDecodes(ctx, check.Lab, neighbor, false, 1); err != nil {
		return err
	}
	// RFC 1997: a NO_EXPORT route MUST NOT be advertised to an external peer,
	// so ze withdraws the route FRR holds instead.
	if err := waitFRRVPNRoute(ctx, check.Lab, vpnWithheldRD, vpnWithheldPrefix, 150*time.Second, false); err != nil {
		return fmt.Errorf("FRR still holds the route after the injector re-announced it with NO_EXPORT: %w", err)
	}
	now, err := queryFRRSessionGeneration(ctx, check.Lab, neighbor)
	if err != nil {
		return fmt.Errorf("FRR's session with ze after the withdrawal: %w", err)
	}
	if now != generation {
		return fmt.Errorf("FRR's session with ze moved from generation %d to %d: the withdrawal ze sent was refused, which a copied label stack causes", generation, now)
	}
	// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD
	// be set to 0x800000." One field, so FRR decodes the RD and the prefix.
	if err := requireVPNWithheldDecodes(ctx, check.Lab, neighbor, true, 1); err != nil {
		return err
	}
	if err := requireVPNWithheldDecodes(ctx, check.Lab, neighbor, false, 1); err != nil {
		return fmt.Errorf("after the withdrawal: %w", err)
	}
	state, err := zePeerState(ctx, check.Lab, injector)
	if err != nil {
		return err
	}
	if state != peerStateEstablished {
		return fmt.Errorf("ze's session with the injector %s is %q, so the route may have left FRR through the injector's departure rather than the converted withdrawal", injector, state)
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
