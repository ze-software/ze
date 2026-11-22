// Design: docs/architecture/behavior/peer-lifecycle.md -- RFC 4724 Section 4.1 Selection_Deferral_Timer
// Related: update_delay.go -- updateDelayHold, trackLocked, peerEndOfRIB, release
// RFC 4724 Section 4.1 -- see rfc/short/rfc4724.md

package reactor

import (
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC4724SelectionDeferralTimerAndExclusions proves the configurable
// Selection_Deferral_Timer upper bound and the multi-peer End-of-RIB deferral
// rules (including excluding R-bit peers and non-GR peers).
//
// VALIDATES:
//   - RFC4724-4.1-8: UpdateDelay.MaxDelay configures the upper-bound deferral
//     timer across distinct durations (45s and 90s); zero disables deferral.
//   - RFC4724-4.1-5: the hold defers initial updates until End-of-RIB is
//     received for every negotiated family from ALL expected GR peers (2 GR
//     peers), excluding peers without GR capability and peers with
//     RestartState=true, or until MaxDelay expires.
//
// PREVENTS: unbounded deferral with no configurable timer, releasing after only
// 1 of 2 GR peers sends EoR, waiting on non-GR or restarting peers, or releasing
// on duplicate/unnegotiated End-of-RIB markers.
//
// RFC requirement: RFC4724-4.1-8 positive -- configured UpdateDelay.MaxDelay (45s and 90s) arms the deferral timer and releases held initial updates at each exact configured upper bound.
// RFC requirement: RFC4724-4.1-8 negative -- zero UpdateDelay.MaxDelay does not arm the deferral timer.
// RFC requirement: RFC4724-4.1-5 positive -- updateDelayHold defers until End-of-RIB for every negotiated family of ALL expected GR peers (2 GR peers, excluding non-GR and RestartState=true peers) or MaxDelay expiry.
// RFC requirement: RFC4724-4.1-5 negative -- partial EoR (1 of 2 GR peers), duplicate EoR, or non-negotiated family EoR markers do not release the deferral hold prematurely.
func TestRFC4724SelectionDeferralTimerAndExclusions(t *testing.T) {
	t.Run("configurable MaxDelay bounds deferral across distinct durations and zero disables", func(t *testing.T) {
		for _, want := range []time.Duration{45 * time.Second, 90 * time.Second} {
			clk := updateDelayClock()
			r := &Reactor{}
			p := updateDelayGRPeer(r, 1)

			if armed := r.updateDelay.arm(context.Background(), clk, UpdateDelay{MaxDelay: 0}, []*Peer{p}); armed {
				t.Fatal("zero MaxDelay armed deferral hold; want false")
			}
			if armed := r.updateDelay.arm(context.Background(), clk, UpdateDelay{MaxDelay: want}, []*Peer{p}); !armed {
				t.Fatalf("MaxDelay %v did not arm deferral hold", want)
			}
			p.startInitialRoutes(p.testInitialSync())
			if !updateDelayHolding(r) {
				t.Fatalf("hold not active before %v expiry", want)
			}
			clk.Add(want - time.Second)
			if !updateDelayHolding(r) {
				t.Fatalf("hold released before %v expiry", want)
			}
			clk.Add(time.Second)
			if updateDelayHolding(r) {
				t.Fatalf("hold still active after %v expiry", want)
			}
			if got := updateDelayReason(r); got != "max-delay" {
				t.Fatalf("release reason = %q, want %q", got, "max-delay")
			}
		}
	})

	t.Run("defers until all 2 GR peers send EoR excluding non-GR and R-bit peers", func(t *testing.T) {
		clk := updateDelayClock()
		r := &Reactor{}
		grPeer1 := updateDelayGRPeer(r, 2)       // IPv4Unicast + IPv6Unicast
		grPeer2 := updateDelayGRPeer(r, 1)       // IPv4Unicast
		rBitPeer := updateDelayRestartingPeer(r) // R=1 -> excluded from EoR wait
		nonGRPeer := updateDelayTestPeer(r)      // no GR -> excluded from EoR wait

		if armed := r.updateDelay.arm(context.Background(), clk, UpdateDelay{MaxDelay: 120 * time.Second}, []*Peer{grPeer1, grPeer2, rBitPeer, nonGRPeer}); !armed {
			t.Fatal("failed to arm deferral hold")
		}

		grPeer1.startInitialRoutes(grPeer1.testInitialSync())
		grPeer2.startInitialRoutes(grPeer2.testInitialSync())
		rBitPeer.startInitialRoutes(rBitPeer.testInitialSync())
		nonGRPeer.startInitialRoutes(nonGRPeer.testInitialSync())
		if !updateDelayHolding(r) {
			t.Fatal("hold released before GR peers sent End-of-RIB")
		}

		// Negative: partial EoR (1 of 2 families on grPeer1), duplicate, and unnegotiated family do not release.
		grPeer1.updateDelayEndOfRIB(family.IPv4Unicast)
		grPeer1.updateDelayEndOfRIB(family.IPv4Unicast)
		grPeer1.updateDelayEndOfRIB(family.IPv4Multicast)
		if !updateDelayHolding(r) {
			t.Fatal("hold released before all negotiated families received End-of-RIB")
		}

		// Negative: completing grPeer1 alone must still hold waiting for grPeer2.
		grPeer1.updateDelayEndOfRIB(family.IPv6Unicast)
		if !updateDelayHolding(r) {
			t.Fatal("hold released after only 1 of 2 non-excluded GR peers completed End-of-RIB")
		}

		// Positive: completing grPeer2 converges immediately without waiting for rBitPeer or nonGRPeer.
		grPeer2.updateDelayEndOfRIB(family.IPv4Unicast)
		if updateDelayHolding(r) {
			t.Fatal("hold stayed active after all non-excluded GR peers completed End-of-RIB")
		}
		if got := updateDelayReason(r); got != "converged" {
			t.Fatalf("release reason = %q, want %q", got, "converged")
		}
	})
}
