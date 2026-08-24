// VALIDATES: `show ddos local` surfaces the responder's live on-host mitigation
// state (installed / not, and the target vector) via the registered RPC handler.
// PREVENTS: the on-host drop state being unreachable from the CLI, and a
// regression that unregisters the handler.

package local

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

func TestShowDdosLocalNoResponder(t *testing.T) {
	activeResponder.Store(nil)

	resp, err := handleShowDdosLocal(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	if m["enabled"] != false || m["active"] != false {
		t.Errorf("got %v, want enabled=false active=false", m)
	}
}

func TestShowDdosLocalActive(t *testing.T) {
	r := newResponder(DefaultConfig(), nil)
	// setStatus is the responder's only writer of the mitigation state: it keeps
	// the mu-guarded fields and the lock-free snapshot the show handler reads in
	// step. Poking the fields would leave the snapshot idle.
	r.setStatus(true, ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 6, DstPort: 80})
	activeResponder.Store(r)
	t.Cleanup(func() { activeResponder.Store(nil) })

	resp, err := handleShowDdosLocal(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	if m["enabled"] != true || m["active"] != true {
		t.Errorf("got %v, want enabled=true active=true", m)
	}
	if _, ok := m["target"]; !ok {
		t.Error("active mitigation must report a target")
	}
}

// dispatchAnswerBudget is how long handleShowDdosLocal may take while a
// reconcile is wedged. It is a decision bound, not a latency measurement: the
// handler reads one atomic pointer, so a pass is microseconds and a failure is
// unbounded. 100ms sits far above the first and far below the second, so this
// test cannot flake on a loaded machine and cannot pass on a blocked one.
const dispatchAnswerBudget = 100 * time.Millisecond

// TestShowDdosLocalAnswersDuringWedgedReconcile is the deterministic
// reproduction of the 2026-07-12 dispatch stall, at the surface that stalled.
//
// VALIDATES: spec-fixit-firewall-concurrency-deadlock AC-2 / AC-3 -- a
// dispatch-command handler answers while a firewall reconcile is in flight and
// cannot return. It drives handleShowDdosLocal, the function the dispatcher
// resolves `show ddos local` to (show.go, ze-ddos-local-cmd.yang), rather than
// r.status() alone: TestResponderStatusDuringSlowApply proves the responder's
// half and stops one call short of the surface an operator reaches.
//
// PREVENTS: the head-of-line block returning. A wedged dataplane leaves
// applyAll inside a netlink round trip with r.mu held (responder.go
// applyMitigation), and before D-3 the handler took that same lock. Restore the
// lock in status() and this test hangs until the budget expires.
//
// The reconcile here NEVER returns, which is what makes the test a
// reproduction rather than a latency check: no deadline, no backend and no
// kernel are involved, so a pass says the handler is decoupled from the
// reconcile, not that the reconcile happened to be quick.
func TestShowDdosLocalAnswersDuringWedgedReconcile(t *testing.T) {
	origReg, origApply := registerTables, applyAll
	entered := make(chan struct{})
	release := make(chan struct{})
	registerTables = func(string, []firewall.Table) error { return nil }
	applyAll = func() error {
		close(entered)
		<-release
		return nil
	}
	t.Cleanup(func() {
		registerTables = origReg
		applyAll = origApply
	})

	r := newResponder(&Config{ResponseLevel: responseEnforce}, nil)
	activeResponder.Store(r)
	t.Cleanup(func() { activeResponder.Store(nil) })

	var wg sync.WaitGroup
	wg.Go(func() {
		r.onDetected(&ddosevent.AttackDetected{
			Interface: "xe0",
			Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
			Family:    ddosevent.FamilyUDPFlood,
			Direction: ddosevent.DirectionLocal,
		})
	})
	<-entered // the reconcile is in flight and r.mu is held for its duration

	type answer struct {
		resp *plugin.Response
		err  error
	}
	answered := make(chan answer, 1)
	go func() {
		resp, err := handleShowDdosLocal(nil, nil)
		answered <- answer{resp: resp, err: err}
	}()

	var got answer
	select {
	case got = <-answered:
	case <-time.After(dispatchAnswerBudget):
		close(release)
		wg.Wait()
		t.Fatal("show ddos local did not answer while a firewall reconcile was wedged: " +
			"dispatch-command is hostage to kernel latency again")
	}

	close(release)
	wg.Wait()

	if got.err != nil {
		t.Fatalf("handleShowDdosLocal returned %v, want an answer", got.err)
	}
	m, ok := got.resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", got.resp.Data)
	}
	// The mitigation is still being installed, so the honest answer is that no
	// drop is live yet. Reporting active here would claim a rule the kernel does
	// not hold: the snapshot must lag the reconcile, never lead it.
	if m["enabled"] != true || m["active"] != false {
		t.Errorf("mid-reconcile answer = %v, want enabled=true active=false", m)
	}
}
