package bridge

import (
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

// TestWatchdogOwnerWithASpaceIsRefused verifies that a process name the
// plugin's command cannot carry as one word fails the line, naming why,
// rather than giving the route to an owner the exit cannot name.
//
// VALIDATES: watchdogGiveCommand refuses an owner containing whitespace.
// PREVENTS: an unparseable `request bgp watchdog give` command reaching the plugin.
func TestWatchdogOwnerWithASpaceIsRefused(t *testing.T) {
	translation, err := TranslateLine("announce route 10.0.4.0/24 next-hop 1.1.1.1 watchdog dnsr")
	if err != nil || len(translation.Commands) != 1 {
		t.Fatalf("translate watchdog route: %v %v", translation.Commands, err)
	}
	if _, err := watchdogGiveCommand("two words", "*", translation.Commands[0]); err == nil {
		t.Fatal("an owner holding a space was accepted")
	}
}
