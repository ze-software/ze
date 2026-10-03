// Design: docs/architecture/testing/ci-format.md — the compiled observer API
// Overview: register_gr_sender_facts.go -- where these scenarios register.
// Detail: plugin_fixture_13.go — the command, poll and status helpers reused here
// Related: internal/test/peer/open_capability.go — the OPEN these observers read back
// RFC: rfc/short/rfc4724.md — the Restart Time and the family tuples a receiver acts on
// RFC: rfc/short/rfc9494.md — the Long-Lived Stale Time and its per-family timer
//
// Two observers for the graceful-restart facts ze-peer states about itself.
// Each one reads `show bgp rib status`, whose `gr-state` map is written by
// markStaleCommand (internal/component/bgp/plugins/rib/rib_commands.go) from the
// argument the bgp-gr plugin dispatched, and that argument is the Restart Time
// the PEER advertised. So the payload names the peer's own number, and a
// mirrored capability would show ze's instead.
//
// The row's presence is the second assertion. `mark-stale` is dispatched only
// after onSessionDown passes its empty-staleFamilies guard, and that set is
// built from the peer's <AFI, SAFI, Flags> tuples, so a row exists only if the
// peer's code-64 value carried at least one.
//
// These observers assert capability facts, not received-route retention.
// The LLGR lifecycle observers in plugin_fixture_llgr_lifecycle.go cover real
// received routes across GR, LLGR, reconnection, End-of-RIB and LLST expiry.

package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// grSenderFactsPeer is the address ze-peer binds in both conventional GR files.
const grSenderFactsPeer = "127.0.0.1"

// grSenderFactsAttempts and grSenderFactsDelay bound every poll below. The
// product is 15 seconds, which is under each file's own budget, so an assertion
// that never becomes true fails on its own words rather than on a test timeout.
const (
	grSenderFactsAttempts = 60
	grSenderFactsDelay    = 250 * time.Millisecond
)

func grSenderFactsDriver(scenario ObserverScenario) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("unexpected arguments: %v", args)
		}
		return Observe(ctx, "gr-sender-facts", sdk.Registration{}, scenario)
	}
}

// grStateRow is the `gr-state` entry for one peer, or nil while the peer has
// none. A nil row and a row of zeroes are different answers: the first says
// mark-stale was never dispatched, the second says it was dispatched with a
// restart time of zero.
func grStateRow(result commandResult13, peerAddr string) map[string]any {
	object, ok := result.object()["gr-state"].(map[string]any)
	if !ok {
		return nil
	}
	row, ok := object[peerAddr].(map[string]any)
	if !ok {
		return nil
	}
	return row
}

// awaitGRState polls until the peer has a gr-state row, which is the observable
// proof that the bgp-gr plugin dispatched `mark-stale` for it.
func awaitGRState(ctx context.Context, plugin *sdk.Plugin, peerAddr string) (map[string]any, error) {
	result := pollCommand13(ctx, plugin, "show bgp rib status", grSenderFactsAttempts, grSenderFactsDelay,
		func(result commandResult13) bool {
			return done13(result) && grStateRow(result, peerAddr) != nil
		})
	// The payload is printed whatever happens, because a failing observer's own
	// error line races the daemon shutdown and is often lost (ci-format.md, "The
	// sentinel is written where the scenario fails"). This line goes out before
	// the assertion, so a reader always sees what the RIB answered.
	fmt.Fprintln(os.Stderr, "gr-sender-facts: show bgp rib status =", result.status, string(result.raw))
	if err := requireStatus13("show bgp rib status", result, "done"); err != nil {
		return nil, err
	}
	row := grStateRow(result, peerAddr)
	if row == nil {
		return nil, fmt.Errorf(
			"no gr-state row for %s: the bgp-gr plugin never dispatched mark-stale, so the peer's "+
				"code-64 value carried no <AFI, SAFI, Flags> tuple and onSessionDown returned at "+
				"its empty-staleFamilies guard: %s", peerAddr, result.raw)
	}
	return row, nil
}

// requireRestartTime states which restart time reached the RIB.
func requireRestartTime(row map[string]any, want int) error {
	if got := number13(row["restart-time"]); got != want {
		return fmt.Errorf(
			"ze armed the peer's graceful restart on %d seconds, want the %d the peer advertised: %v",
			got, want, row)
	}
	return nil
}

// grPeerRestartTimeDrivesTimer is AC-1 of
// spec-test-peer-open-mirrors-five-more-sender-facts.
//
// The `.ci` states a Restart Time of 7 seconds and ze is configured for 120, so
// the number in gr-state is ze's own under a mirror and the peer's under the
// resolution.
func grPeerRestartTimeDrivesTimer(ctx context.Context, plugin *sdk.Plugin) error {
	row, err := awaitGRState(ctx, plugin, grSenderFactsPeer)
	if err != nil {
		return err
	}
	if err := requireRestartTime(row, 7); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: ze timed the peer's restart by the peer's own 7 seconds")
	return nil
}

// grPeerFamiliesDriveMarkStale is AC-2.
//
// Its Restart Time is 120, the SAME number ze is configured with, so the value
// cannot tell a mirror from a resolution here. What can is the family tuple: the
// gr-state row exists only if onSessionDown built a non-empty stale-family set
// from the peer's code-64 value (parseGRCapValue,
// internal/component/bgp/plugins/gr/gr_capability.go).
func grPeerFamiliesDriveMarkStale(ctx context.Context, plugin *sdk.Plugin) error {
	row, err := awaitGRState(ctx, plugin, grSenderFactsPeer)
	if err != nil {
		return err
	}
	if err := requireRestartTime(row, 120); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the peer's declared family made ze dispatch mark-stale")
	return nil
}

