// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- show/clear command tests
//
// VALIDATES: AC-8 (show payload shapes + interface selector), AC-9 (clear
// resets counters without touching state).
// PREVENTS: the CommandDecl list and the dispatch switch drifting apart (a
// declared command that answers nothing), and a selector-less interface view
// silently returning every instance.

package vrrp

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// waitSettled returns the engine's instance views once none is still in
// Initialize, so a caller samples protocol state that has stopped moving.
//
// The engine drives an instance's startup transition on the instance's own
// goroutine, so `apply` returning says nothing about the FSM having left
// Initialize. Any test that compares two snapshots either side of an operation
// has to settle first, otherwise the startup transition lands between the
// samples and is attributed to whatever ran in between.
//
// Bounded and loud: a state that never settles is a real defect, and reporting
// it as "waited 2s, still initialize" is more use than a hang.
//
// The comparison goes through viewState, the same producer that FILLS the
// field. It read fsm.StateInitialize.String() until 2026-08-23, and those two
// spell one concept two ways -- "Initialize" against "initialize" -- so the
// guard matched nothing, waitSettled returned on its first poll, and the settle
// it exists to perform never happened. The symptom was the very failure this
// helper was written to remove, still arriving under load.
func waitSettled(t *testing.T, eng *engine) []instanceView {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var views []instanceView
	for {
		views = eng.snapshots()
		moving := false
		for i := range views {
			if views[i].State == viewState(fsm.StateInitialize) {
				moving = true
				break
			}
		}
		if !moving {
			return views
		}
		if time.Now().After(deadline) {
			t.Fatalf("instances never left %v: %+v", fsm.StateInitialize, views)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestWaitSettledActuallySettles proves the settle helper waits for the startup
// transition rather than returning on its first poll.
//
// The engine starts an instance's FSM on the instance's OWN goroutine, so
// `apply` returning says nothing about the state. An owner reaches Master, and
// reading Initialize here means waitSettled did not wait -- which is what a
// guard comparing the view's spelling against fsm.State.String() produced,
// silently, for every caller of this helper.
//
// VALIDATES: waitSettled's guard fires.
// PREVENTS: a settle helper that settles nothing, so every test built on it
// attributes the startup transition to whatever ran between its two samples.
func TestWaitSettledActuallySettles(t *testing.T) {
	eng, _ := newTestEngine(t)
	spec := testSpec()
	spec.IsOwner = true // reaches Master at startup, never stays in Initialize
	eng.apply([]GroupSpec{spec})

	views := waitSettled(t, eng)
	if len(views) != 1 {
		t.Fatalf("views = %d, want 1", len(views))
	}
	if views[0].State != viewState(fsm.StateMaster) {
		t.Fatalf("state after settling = %q, want %q: waitSettled returned before the FSM moved",
			views[0].State, viewState(fsm.StateMaster))
	}
}

// TestCommandDeclsMatchDispatch proves every declared command is answered.
//
// The SDK declaration and the handler switch are two lists of the same strings;
// nothing but a test stops one from growing without the other, and the symptom
// would be a command that exists in the CLI and errors at runtime.
func TestCommandDeclsMatchDispatch(t *testing.T) {
	eng, _ := newTestEngine(t)
	for _, decl := range commandDecls() {
		args := []string(nil)
		if decl.Name == cmdShowVRRPInterface {
			args = []string{"name", "eth0"} // this one requires a selector
		}
		status, payload, err := handleCommand(eng, decl.Name, args)
		if err != nil {
			t.Errorf("declared command %q returned an error: %v", decl.Name, err)
			continue
		}
		if status != rpc.StatusDone {
			t.Errorf("declared command %q returned status %q, want %q", decl.Name, status, rpc.StatusDone)
		}
		if payload == nil {
			t.Errorf("declared command %q returned no payload", decl.Name)
		}
	}
}

// TestHandleUnknownCommand proves an undeclared command is refused, not
// silently answered with an empty result.
func TestHandleUnknownCommand(t *testing.T) {
	eng, _ := newTestEngine(t)
	status, _, err := handleCommand(eng, "show vrrp nonsense", nil)
	if err == nil {
		t.Fatal("unknown command must return an error")
	}
	if status != rpc.StatusError {
		t.Errorf("status = %q, want %q", status, rpc.StatusError)
	}
}

// TestShowInterfaceRequiresSelector proves the typed selector is mandatory: a
// missing name must fail loudly rather than dumping every interface's state.
func TestShowInterfaceRequiresSelector(t *testing.T) {
	eng, _ := newTestEngine(t)
	if _, _, err := handleCommand(eng, cmdShowVRRPInterface, nil); err == nil {
		t.Fatal("show vrrp interface without a selector must error")
	}
}

// TestSelectorValue pins the grammar `show vrrp interface name <name>`
// (ai/rules/cli.md: a typed keyword before any free-form value).
func TestSelectorValue(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"typed selector", []string{"name", "eth0"}, "eth0"},
		{"bare value from a programmatic sender", []string{"eth0"}, "eth0"},
		{"no args", nil, ""},
		{"keyword with no value", []string{"name"}, ""},
		{"interface literally named name", []string{"name", "name"}, "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectorValue(tc.args); got != tc.want {
				t.Fatalf("selectorValue(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

// TestShowInterfaceFiltersToOneParent proves the selector actually filters.
func TestShowInterfaceFiltersToOneParent(t *testing.T) {
	eng, _ := newTestEngine(t)
	eth0 := testSpec()
	eth1 := testSpec()
	eth1.Interface = "eth1"
	eng.apply([]GroupSpec{eth0, eth1})

	views := eng.snapshotsForInterface("eth1")
	if len(views) != 1 || views[0].Interface != "eth1" {
		t.Fatalf("interface selector must return only eth1's instances, got %+v", views)
	}
}

// TestClearStatisticsPreservesState proves `clear vrrp statistics` zeroes
// counters without perturbing the protocol: an operator clearing counters must
// never trigger a failover.
func TestClearStatisticsPreservesState(t *testing.T) {
	eng, _ := newTestEngine(t)
	spec := testSpec()
	spec.IsOwner = true // becomes Master at startup
	eng.apply([]GroupSpec{spec})

	// Settle the startup transition BEFORE sampling, or this test accuses
	// clearStatistics of a state change it did not cause. An owner reaches
	// Master through the instance goroutine, so `before` could still read
	// "initialize" while `after` reads "master" -- the assertion below then
	// fires on scheduling rather than on behavior, which is what made this fail
	// only inside a loaded full run (ai/rules/completion.md: wait for the
	// condition, never for a duration).
	before := waitSettled(t, eng)
	if len(before) != 1 {
		t.Fatalf("instances = %d, want 1", len(before))
	}

	if cleared := eng.clearStatistics(); cleared != 1 {
		t.Errorf("cleared = %d, want 1", cleared)
	}

	after := eng.snapshots()
	if len(after) != 1 || after[0].State != before[0].State {
		t.Fatalf("clear changed state %q -> %q; counters only", before[0].State, after[0].State)
	}
	stats := eng.statistics()
	if len(stats) != 1 {
		t.Fatalf("statistics = %d, want 1", len(stats))
	}
	if stats[0].PriorityZeroSent != 0 || stats[0].PriorityZeroReceived != 0 {
		t.Errorf("priority-zero counters not cleared: %+v", stats[0])
	}
}
