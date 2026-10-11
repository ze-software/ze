// Tests for the attached up: a reload that attaches the watchdog to a session
// already established.

package watchdog

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestWatchdogSkipsResendOnAttachedUp proves that an up carrying reason
// attached records the peer up and neither resends its announced routes nor
// reports ready, on both event paths.
//
// VALIDATES: handlePeerUp with rpc.ReasonAttached marks the peer up, sends no
// route and calls no session ready; the text path parses "reason attached" and
// routes it the same way.
// PREVENTS: the watchdog resending routes into a session a reload only attached
// it to (its binding is receive-only there, so the engine refuses every one),
// and a ready report the engine refuses (an attached up carries no receipt).
//
// Method: TestReconnectResend's pool, where a normal up sends one route.
func TestWatchdogSkipsResendOnAttachedUp(t *testing.T) {
	const peer = "10.0.0.1"
	for _, event := range []string{"structured", "text"} {
		t.Run(event, func(t *testing.T) {
			sent, ready := 0, 0
			mgr := newWatchdogServer(
				func(string, string) { sent++ },
				func(string, uint64) { ready++ },
			)
			mgr.peerPools[peer] = newPoolSet()
			entry := newPoolEntry("10.0.0.0/24#0",
				"update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
				"update text nlri ipv4/unicast del 10.0.0.0/24")
			entry.initiallyAnnounced = true
			if err := mgr.peerPools[peer].AddRoute("dnsr", entry); err != nil {
				t.Fatal(err)
			}
			mgr.peerPools[peer].AnnouncePool("dnsr", peer)

			reason := rpc.ReasonAttached
			if event == "text" {
				addr, state, parsed, token := parseStateEvent("peer " + peer + " remote as 65001 state up reason " + rpc.ReasonAttached + "\n")
				if addr != peer || state != "up" || token != 0 {
					t.Fatalf("parsed %q %q token %d, want %s up token 0", addr, state, token, peer)
				}
				reason = parsed
			}
			// A nonzero receipt, so a path that resent would also report ready.
			mgr.handlePeerUp(peer, reason, 41)

			if !mgr.peerUp[peer] {
				t.Error("the peer is up for the watchdog")
			}
			if sent != 0 {
				t.Errorf("an attached up sent %d routes, want 0", sent)
			}
			if ready != 0 {
				t.Errorf("an attached up reported ready %d times, want 0", ready)
			}
		})
	}
}
