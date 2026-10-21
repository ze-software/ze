package bridge

import (
	"context"
	"slices"
	"testing"
)

// TestOnExitDefaultByVersion holds exitPolicy to ExaBGP's Processes._on_exit
// (X-22): the leaf wins on any version; unset, API 4 keeps and every other
// version withdraws, each with the reason the exit log names.
func TestOnExitDefaultByVersion(t *testing.T) {
	cases := []struct {
		onExit   OnExit
		version  APIVersion
		withdraw bool
		by       string
	}{
		{OnExitByVersion, APIV4, false, "API 4"},
		{OnExitByVersion, APIV6, true, "API 6"},
		{OnExitByVersion, APIVersionAuto, true, "API undecided"},
		{OnExitKeep, APIV6, false, "on-exit"},
		{OnExitWithdraw, APIV4, true, "on-exit"},
	}
	for _, tc := range cases {
		withdraw, by := exitPolicy(tc.onExit, tc.version)
		if withdraw != tc.withdraw || by != tc.by {
			t.Errorf("exitPolicy(%d, %s) = %v, %q; want %v, %q", tc.onExit, tc.version, withdraw, by, tc.withdraw, tc.by)
		}
	}
}

// ownerTestRoute translates one route line into the command a runner notes.
func ownerTestRoute(t *testing.T, line string) Command {
	t.Helper()
	translation, err := TranslateLine(line)
	if err != nil || len(translation.Commands) != 1 {
		t.Fatalf("translate %q: %v %v", line, translation.Commands, err)
	}
	return translation.Commands[0]
}

// TestOwnershipTransfers holds the ledger to ExaBGP's owner rule (X-21, AC-25):
// the last announcer owns a route, and a withdraw from anywhere clears it.
func TestOwnershipTransfers(t *testing.T) {
	route := ownerTestRoute(t, "announce route 10.0.0.0/24 next-hop 1.2.3.4")
	withdraw := ownerTestRoute(t, "withdraw route 10.0.0.0/24 next-hop 1.2.3.4")

	ledger := NewOwnerLedger()
	ledger.Note("a", "*", route)
	ledger.Note("b", "*", route)
	if owned := ledger.release("a"); len(owned) != 0 {
		t.Fatalf("a no longer owns the route B announced after it, released %v", owned)
	}
	if owned := ledger.release("b"); len(owned) != 1 || owned[0].withdrawal != restoringWithdrawal(route.Withdrawal) {
		t.Fatalf("b owns the route: released %v, want its withdrawal %q", owned, route.Withdrawal)
	}

	ledger.Note("a", "*", route)
	ledger.Note("b", "*", withdraw)
	if owned := ledger.release("a"); len(owned) != 0 {
		t.Fatalf("a withdraw from b clears a's route, released %v", owned)
	}
}

// TestExitWithdrawalRestoresConfigured holds the exit withdrawal to ExaBGP's
// withdraw_owner (A-8): it asks ze to give back a configured route of the same
// prefix, so it carries `restore-configured` ahead of its NLRI section.
func TestExitWithdrawalRestoresConfigured(t *testing.T) {
	route := ownerTestRoute(t, "announce route 10.0.0.0/24 next-hop 1.2.3.4")
	ledger := NewOwnerLedger()
	ledger.Note("a", "*", route)

	owned := ledger.release("a")
	if len(owned) != 1 {
		t.Fatalf("a owns its route, released %v", owned)
	}
	want := "send bgp * update text restore-configured nhop 1.2.3.4 nlri del 10.0.0.0/24"
	if owned[0].withdrawal != want {
		t.Errorf("exit withdrawal %q, want %q", owned[0].withdrawal, want)
	}
}

// exitRecorder is an ExitRunner that records what an exit sent ze.
type exitRecorder struct {
	dispatched []string
	flushed    []string
}

func (r *exitRecorder) Dispatch(_ context.Context, command string) error {
	r.dispatched = append(r.dispatched, command)
	return nil
}

func (r *exitRecorder) Flush(_ context.Context, selector string) error {
	r.flushed = append(r.flushed, selector)
	return nil
}

// TestStandaloneExitWithdrawsOwnedRoutes holds the standalone runner
// (`ze exabgp plugin <command>`, Bridge) to the exit rule the fleet follows
// (X-21, X-22). Method: a route command passes through muxRunner.Dispatched,
// as ScriptLines.Batch hands it over once ze accepted it, then the child's
// exit is settled through a recorder. The runner has no process block, so
// `on-exit` is unset and the API version decides: undecided withdraws, held
// to API 4 keeps.
// PREVENTS: a script run by the standalone runner leaving its routes
// announced after it exited, where the plugin runner withdraws them.
func TestStandaloneExitWithdrawsOwnedRoutes(t *testing.T) {
	route := ownerTestRoute(t, "peer * announce route 10.6.0.0/24 next-hop 1.2.3.4")

	b := NewBridge([]string{"script"})
	muxRunner{b: b}.Dispatched("*", route)
	rec := &exitRecorder{}
	b.exited(context.Background(), rec)

	want := restoringWithdrawal(route.Withdrawal)
	if !slices.Equal(rec.dispatched, []string{want}) {
		t.Errorf("exit dispatched %q, want the route's withdrawal %q", rec.dispatched, want)
	}
	if !slices.Equal(rec.flushed, []string{"*"}) {
		t.Errorf("exit flushed %q, want the route's selector", rec.flushed)
	}

	held := NewBridge([]string{"script"})
	held.apiSetting.Store(uint32(APIV4))
	held.api.Reset()
	muxRunner{b: held}.Dispatched("*", route)
	kept := &exitRecorder{}
	held.exited(context.Background(), kept)
	if len(kept.dispatched) != 0 {
		t.Errorf("a script held to API 4 keeps its routes, exit dispatched %q", kept.dispatched)
	}
}
