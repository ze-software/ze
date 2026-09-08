// Design: docs/architecture/testing/ci-format.md — the compiled observer API
// Overview: register_gr_sender_facts.go — where these three scenarios register
// Detail: plugin_fixture_13.go — the command, poll and status helpers reused here
// Related: internal/test/peer/open_capability.go — the OPEN these observers read back
// RFC: rfc/short/rfc4724.md — the Restart Time and the family tuples a receiver acts on
// RFC: rfc/short/rfc9494.md — the Long-Lived Stale Time and its per-family timer
//
// Three observers for the graceful-restart facts ze-peer states about itself.
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
// # What these observers deliberately do NOT assert
//
// They do not assert that any route was MARKED stale. Measured 2026-09-08: on
// peer-down, RIBManager.handleState releases the peer's Adj-RIB-In unless
// r.retainedPeers already holds the peer, and the flag is set by the
// `retain-routes` command the bgp-gr plugin dispatches after it receives the
// same peer-down event. The two plugins race on one event, bgp-rib wins, and
// mark-stale then runs over an empty RIB: two runs of this file showed
// `routes-in: 0` beside a gr-state row naming the peer's own restart time. That
// is a defect in ze's retention, recorded in
// plan/journal/announced-state-never-replayed.md, and it is not what these files
// are about. Asserting through it would make them fail for somebody else's
// reason.

package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// grSenderFactsPeer is the address ze-peer binds in all three `.ci` files.
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

// awaitLLGREntry holds until the peer's gr-state row states a restart time of
// zero, which is the observable proof that ze entered the Long-Lived Graceful
// Restart period for this peer.
//
// Only one caller writes that zero: onLLGREnter (internal/component/bgp/plugins/gr/gr.go)
// dispatches `request bgp rib mark-stale <peer> 0 2`, and its own comment states
// why -- "no new timer needed, LLST timer handles expiry". enterLLGRLocked calls
// it once per family that has an entry with a non-zero Long-Lived Stale Time in
// the PEER's code-71 capability; a family with no entry, or with a stale time of
// zero, is purged instead and never reaches this callback. So a zero here says
// the peer's own 7-octet tuple was decoded and acted on.
func awaitLLGREntry(ctx context.Context, plugin *sdk.Plugin, peerAddr string) error {
	entered := func(result commandResult13) bool {
		if !done13(result) {
			return false
		}
		row := grStateRow(result, peerAddr)
		return row != nil && number13(row["restart-time"]) == 0
	}
	result := pollCommand13(ctx, plugin, "show bgp rib status", grSenderFactsAttempts, grSenderFactsDelay, entered)
	if err := requireStatus13("show bgp rib status", result, "done"); err != nil {
		return err
	}
	if !entered(result) {
		return fmt.Errorf(
			"ze never entered LLGR for %s, so the peer's code-71 capability named no family with a "+
				"Long-Lived Stale Time it could act on: %s", peerAddr, result.raw)
	}
	return nil
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
// from the peer's code-64 value, and ze's own code 64 carries no tuples at all
// (parseGRCapValue, internal/component/bgp/plugins/gr/gr.go).
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

// llgrPeerStaleTimeDrivesTimer is AC-3.
//
// The `.ci` states a Restart Time of 1 second and a Long-Lived Stale Time of 2,
// against ze's own 120 and 3600. Two facts of the peer's are asserted: the
// gr-state row's restart time is the peer's 1 rather than ze's 120, and the row
// then reaches the zero that says ze entered LLGR, which needs the peer's own
// code-71 tuple to name a family with a non-zero stale time.
//
// The stale time's own NUMBER is not asserted here, and the reason is a product
// defect rather than a choice: every effect of the LLST timer -- purge-stale,
// release-routes -- acts on an Adj-RIB-In that RIBManager.handleState already
// released on peer-down, so none of them changes anything a command can read.
// The number is held by TestPeerOpenLLGRIsTheHarnessOwn instead, which reads the
// octets ze-peer writes. See the file header for the retention race.
func llgrPeerStaleTimeDrivesTimer(ctx context.Context, plugin *sdk.Plugin) error {
	row, err := awaitGRState(ctx, plugin, grSenderFactsPeer)
	if err != nil {
		return err
	}
	if err := requireRestartTime(row, 1); err != nil {
		return err
	}
	if err := awaitLLGREntry(ctx, plugin, grSenderFactsPeer); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: ze entered LLGR on the peer's own graceful-restart and stale times")
	return nil
}
