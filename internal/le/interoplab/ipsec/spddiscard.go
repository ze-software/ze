// Design: docs/architecture/testing/interop.md -- spd-discard-entry, an operator DISCARD entry beside a policy-based tunnel
// Related: vtispd.go -- xfrmStat and XfrmOutPolBlock, the BLOCK counter reads this reuses
// Related: helpers.go -- inner addresses, ping, the ESP counters and the IKE SA reads
// Related: checkers.go -- scenarioCheckers names the checker

package ipsec

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The spd-discard-entry scenario's inner addresses. The Child SA carries
// 10.2.0.0/24 (Ze) <-> 10.1.0.0/24 (strongSwan); ze.conf's entry drop-swan-upper
// discards 10.2.0.0/24 <-> 10.1.0.128/25.
const (
	discardScenario = "spd-discard-entry"
	discardZeInner  = "10.2.0.1"
	// discardSwanKept is inside the selectors and outside the entry, so the tunnel
	// carries it; discardSwanDropped is inside both, so the entry decides it.
	discardSwanKept    = "10.1.0.1"
	discardSwanDropped = "10.1.0.200"
	// discardCount is the echo requests each discarded probe sends.
	discardCount = 3
	// xfrmStatInPolBlock is the /proc/net/xfrm_stat counter __xfrm_policy_check raises
	// when a BLOCK policy decides an inbound packet (net/xfrm/xfrm_policy.c).
	xfrmStatInPolBlock = "XfrmInPolBlock"
)

// checkSPDDiscardEntry proves user story 1 and AC-1 of spec-rfc4301-architecture-gaps
// against strongSwan, on the default configuration: an operator DISCARD entry drops
// traffic the tunnel beside it would otherwise carry, and the tunnel stays up.
//
// RFC 4301 Section 4.4.1: "For any outbound or inbound datagram, three processing
// choices are possible: DISCARD, BYPASS IPsec, or PROTECT using IPsec.  The first
// choice refers to traffic that is not allowed to traverse the IPsec boundary (in the
// specified direction)." The entry discards both directions, so both are probed.
//
// Each assertion is numbered, so a red names the observation that failed:
//
//  1. The IKE SA and the Child SA are up on both peers.
//  2. The inner addresses are installed (setup, numbered so a red names it).
//  3. Ze's echo to 10.1.0.1, outside the entry, crosses the tunnel: no loss, and all
//     four simplex SAs move bytes.
//  4. Ze's echo to 10.1.0.200, inside the entry, is dropped by it: every echo lost,
//     Ze's XfrmOutPolBlock rises by at least the echoes sent, and Ze's outbound SA
//     moves no byte, so none of them left over the tunnel.
//  5. strongSwan's echo from 10.1.0.200 to Ze, which strongSwan carries because its
//     Child SA covers the whole /24, is dropped by the entry's inbound half: every
//     echo lost, strongSwan's outbound SA and Ze's inbound SA move bytes (it arrived and
//     decrypted), and Ze's XfrmInPolBlock rises by at least the echoes sent.
//  6. The tunnel stayed up: Ze and strongSwan still list the IKE SA established, the
//     ESP SPIs on both peers are the ones of assertion 3, and 10.1.0.1 still crosses.
//
// Red proof: ze.conf names the two breaks of spdPolicyHasDirection that turn 4 and 5
// red.
func checkSPDDiscardEntry(ctx context.Context, lab *scenarioLab) error {
	fail := func(assertion int, cause error) error {
		var tb textbuf.Buffer
		tb.Str("scenario ").Str(discardScenario).Str(" assertion ").Str(strconv.Itoa(assertion)).Str(": ").Err(cause)
		return errors.New(tb.String())
	}
	if err := establish(ctx, lab); err != nil {
		return fail(1, err)
	}
	for _, peer := range []string{zePeer, swanPeer} {
		if _, err := lab.waitXFRM(ctx, peer); err != nil {
			return fail(1, err)
		}
	}
	if err := lab.discardInnerAddresses(ctx); err != nil {
		return fail(2, err)
	}
	kept := pingProbe{peer: zePeer, target: discardSwanKept, source: discardZeInner}
	if err := lab.verifyESPDirectionsToward(ctx, "Ze to "+discardSwanKept+", outside the discard entry, did not cross the tunnel", kept, espBothDirections); err != nil {
		return fail(3, err)
	}
	spis := make(map[string]map[string]struct{}, 2)
	for _, peer := range []string{zePeer, swanPeer} {
		set, err := lab.espSPIs(ctx, peer)
		if err != nil {
			return fail(3, err)
		}
		spis[peer] = set
	}
	if err := lab.requireDiscarded(ctx, pingProbe{peer: zePeer, target: discardSwanDropped, source: discardZeInner},
		xfrmStatOutPolBlock, nil, []espDirection{zeEncrypts}); err != nil {
		return fail(4, err)
	}
	if err := lab.requireDiscarded(ctx, pingProbe{peer: swanPeer, target: discardZeInner, source: discardSwanDropped},
		xfrmStatInPolBlock, []espDirection{swanEncrypts, zeDecrypts}, nil); err != nil {
		return fail(5, err)
	}
	if err := lab.requireTunnelStayedUp(ctx, kept, spis); err != nil {
		return fail(6, err)
	}
	return nil
}

// discardInnerAddresses puts Ze's inner address and strongSwan's two on loopback, with
// the routes that send each toward the other side of the tunnel.
func (l *scenarioLab) discardInnerAddresses(ctx context.Context) error {
	for _, step := range []struct{ peer, local, remote string }{
		{zePeer, discardZeInner, discardSwanKept},
		{zePeer, discardZeInner, discardSwanDropped},
		{swanPeer, discardSwanDropped, discardZeInner},
		{swanPeer, discardSwanKept, discardZeInner},
	} {
		if err := l.natInnerAddress(ctx, step.peer, step.local, step.remote); err != nil {
			return err
		}
	}
	return nil
}

// requireDiscarded sends discardCount echoes along probe and requires every one lost to
// a BLOCK policy on Ze: Ze's counter block rises by at least the echoes sent. Each SA
// in moved must advance (the echo was carried that far) and each SA in still must not
// (the echo was never carried there).
func (l *scenarioLab) requireDiscarded(ctx context.Context, probe pingProbe, block string, moved, still []espDirection) error {
	observed := slices.Concat(moved, still)
	blockedBefore, err := l.xfrmStat(ctx, zePeer, block)
	if err != nil {
		return err
	}
	before, err := l.espCounters(ctx, observed)
	if err != nil {
		return err
	}
	output := l.execQuiet(ctx, probe.peer, pingCommand(probe, discardCount)...)
	loss, err := pingLoss(output)
	if err != nil {
		return fmt.Errorf("ping from %s %s to %s %w", probe.peer, probe.source, probe.target, err)
	}
	if loss != 100 {
		return fmt.Errorf("ping from %s %s to %s, inside the discard entry, lost %g%%, want every echo dropped by it: %s",
			probe.peer, probe.source, probe.target, loss, output)
	}
	blockedAfter, err := l.xfrmStat(ctx, zePeer, block)
	if err != nil {
		return err
	}
	after, err := l.espCounters(ctx, observed)
	if err != nil {
		return err
	}
	if blockedAfter < blockedBefore+discardCount {
		return fmt.Errorf("ze's %s rose from %d to %d for %d echoes from %s to %s, so no BLOCK entry decided them",
			block, blockedBefore, blockedAfter, discardCount, probe.source, probe.target)
	}
	for _, want := range moved {
		if err := assertESPAdvanced(before[want.peer], after[want.peer], want); err != nil {
			return fmt.Errorf("echoes from %s to %s: %w", probe.source, probe.target, err)
		}
	}
	for _, want := range still {
		if err := requireESPStill(before[want.peer], after[want.peer], want); err != nil {
			return fmt.Errorf("echoes from %s to %s: %w", probe.source, probe.target, err)
		}
	}
	return nil
}

// requireESPStill refuses unless the direction names at least one SA and none of its
// SAs moved a byte between the two snapshots. A direction with no SA is refused rather
// than read as still: an absent SA moves nothing whatever the dataplane did.
func requireESPStill(before, after map[saKey]uint64, want espDirection) error {
	beforeBytes := directionCounters(before, want)
	if len(beforeBytes) == 0 {
		return fmt.Errorf("%s reports no SA for src %s dst %s", want.peer, want.source, want.target)
	}
	afterBytes := directionCounters(after, want)
	if !maps.Equal(beforeBytes, afterBytes) {
		return fmt.Errorf("src %s dst %s at %s moved bytes (before=%s after=%s), so an echo the discard entry names crossed the tunnel",
			want.source, want.target, want.peer, sortedCounters(beforeBytes), sortedCounters(afterBytes))
	}
	return nil
}

// requireTunnelStayedUp refuses unless both peers still hold the IKE SA established,
// each peer's ESP SPIs are the ones it held before the discarded probes, and the kept
// probe still crosses the tunnel in all four directions.
func (l *scenarioLab) requireTunnelStayedUp(ctx context.Context, kept pingProbe, spis map[string]map[string]struct{}) error {
	sas, err := l.listSAs(ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(sas, "ESTABLISHED") {
		return fmt.Errorf("strongSwan lists no established IKE SA after the discarded probes: %s", sas)
	}
	if err := l.requireZeIKEEstablished(ctx, swanConfigPeer); err != nil {
		return err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		now, err := l.espSPIs(ctx, peer)
		if err != nil {
			return err
		}
		if !sameStrings(spis[peer], now) {
			return fmt.Errorf("%s's ESP SPIs changed across the discarded probes (before %v, after %v), so the tunnel did not stay up",
				peer, spis[peer], now)
		}
	}
	return l.verifyESPDirectionsToward(ctx, "Ze to "+discardSwanKept+" stopped crossing the tunnel after the discarded probes", kept, espBothDirections)
}
