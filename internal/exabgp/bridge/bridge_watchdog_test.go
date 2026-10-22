package bridge

import (
	"context"
	"log/slog"
	"slices"
	"testing"
)

// TestWatchdogRouteReachesBgpWatchdog verifies that a script's announce naming
// a watchdog is handed to the bgp-watchdog plugin, with the script as its
// owner, its update text and its withdrawal, and the `withdraw` flag as
// `withdraw`.
//
// VALIDATES: ExaBGP's announce_route gives the route to add_to_rib_watchdog
// with its owner (c43f90f79, in the f9bfb853b pin): the watchdog holds it,
// sent now or held, and it stays the helper's.
// PREVENTS: the route reaching the peers directly, where no `announce
// watchdog` or `withdraw watchdog` reaches it and the exit does not release it.
// Method: feed one line to ScriptLines.Batch over a recording runner.
func TestWatchdogRouteReachesBgpWatchdog(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{
			"announce route 10.0.4.0/24 next-hop 1.1.1.1 watchdog dnsr",
			"request bgp watchdog give dnsr owner s selector * announce 7 update text nhop 1.1.1.1 nlri add 10.0.4.0/24" +
				" withdrawal update text nhop 1.1.1.1 nlri del 10.0.4.0/24",
		},
		{
			"announce route 10.0.5.0/24 next-hop 1.1.1.1 watchdog dnsr withdraw",
			"request bgp watchdog give dnsr owner s selector * withdraw announce 7 update text nhop 1.1.1.1 nlri add 10.0.5.0/24" +
				" withdrawal update text nhop 1.1.1.1 nlri del 10.0.5.0/24",
		},
	}
	for _, tc := range cases {
		g := newGroupScript()
		if got := g.send(t, tc.line); got != "done\n" {
			t.Errorf("%q answered %q, want done", tc.line, got)
		}
		if !slices.Equal(g.runner.dispatched, []string{tc.want}) {
			t.Errorf("%q dispatched %q, want %q", tc.line, g.runner.dispatched, tc.want)
		}
	}
}

func discardLog() *slog.Logger { return slog.New(slog.DiscardHandler) }

// TestWatchdogOwnerWithASpaceIsRefused verifies that a process name the
// plugin's command cannot carry as one word fails the line, naming why,
// rather than giving the route to an owner the exit cannot name.
func TestWatchdogOwnerWithASpaceIsRefused(t *testing.T) {
	command := ownerTestRoute(t, "announce route 10.0.4.0/24 next-hop 1.1.1.1 watchdog dnsr")
	if _, err := watchdogGiveCommand("two words", "*", command); err == nil {
		t.Fatal("an owner holding a space was accepted")
	}
}

// TestExitForgetsWatchdogRoutes verifies that the exit of a script that gave a
// watchdog a route asks bgp-watchdog to forget its routes, before it withdraws
// the routes the ledger holds, and that the ledger no longer owns a route once
// it is given to a watchdog.
//
// VALIDATES: ExaBGP's withdraw_owner calls _forget_watchdog_owner first: a
// helper which exits has its watchdog routes withdrawn and taken off the
// watchdog, so a later `announce watchdog` does not bring them back.
// PREVENTS: a held route coming back after its helper exited, and a route
// withdrawn twice, once by the ledger and once by the plugin.
func TestExitForgetsWatchdogRoutes(t *testing.T) {
	plain := ownerTestRoute(t, "announce route 10.0.0.0/24 next-hop 1.2.3.4")
	given := ownerTestRoute(t, "announce route 10.0.4.0/24 next-hop 1.2.3.4 watchdog dnsr")
	sameAsGiven := ownerTestRoute(t, "announce route 10.0.4.0/24 next-hop 1.2.3.4")

	ledger := NewOwnerLedger()
	ledger.Note("a", "*", sameAsGiven)
	ledger.Note("a", "*", given)
	ledger.Note("a", "*", plain)
	rec := &exitRecorder{}
	ledger.Exited(context.Background(), discardLog(), "a", OnExitWithdraw, APIV6, rec)

	want := []string{"request bgp watchdog forget a", restoringWithdrawal(plain.Withdrawal)}
	if !slices.Equal(rec.dispatched, want) {
		t.Errorf("exit dispatched %q, want %q", rec.dispatched, want)
	}

	// A script that gave no watchdog a route sends no forget, so a daemon
	// without bgp-watchdog is not asked for it.
	other := NewOwnerLedger()
	other.Note("b", "*", plain)
	quiet := &exitRecorder{}
	other.Exited(context.Background(), discardLog(), "b", OnExitWithdraw, APIV6, quiet)
	if slices.Contains(quiet.dispatched, "request bgp watchdog forget b") {
		t.Errorf("exit of a script with no watchdog route sent %q", quiet.dispatched)
	}

	// `on-exit keep` keeps the routes on their watchdog, as it keeps the rest.
	kept := NewOwnerLedger()
	kept.Note("c", "*", given)
	keep := &exitRecorder{}
	kept.Exited(context.Background(), discardLog(), "c", OnExitKeep, APIV6, keep)
	if len(keep.dispatched) != 0 {
		t.Errorf("on-exit keep dispatched %q, want nothing", keep.dispatched)
	}
}
