// Design: plan/spec-fw-7b-backend-hardening.md -- Apply-path tests for vpp backend.

//go:build linux

package trafficvpp

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/policer"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
	vppcomp "codeberg.org/thomas-mangin/ze/internal/component/vpp"
)

// newTestBackend builds a backend whose connector returns an unconnected
// Connector. Apply reaches WaitConnected but never completes a real VPP call,
// which is what the ctx-cancellation tests need.
func newTestBackend() *backend {
	conn := vppcomp.NewConnector("/nonexistent/vpp.sock")
	return &backend{
		connector:               func() *vppcomp.Connector { return conn },
		interfaceOutputPolicers: make(map[string]map[string]uint32),
		interfaceQdiscTypes:     make(map[string]traffic.QdiscType),
	}
}

// newOpsBackend builds a backend with no connector (applyWithOps bypasses it)
// ready for scripted fakeOps injection.
func newOpsBackend() *backend {
	return &backend{
		interfaceOutputPolicers: make(map[string]map[string]uint32),
		interfaceQdiscTypes:     make(map[string]traffic.QdiscType),
	}
}

// VALIDATES: AC-3 "trafficvpp.Apply with a pre-canceled ctx: Returns ctx.Err()
// before WaitConnected tries to poll".
// PREVENTS: SIGTERM mid-Apply blocks for the full waitConnectedTimeout when the
// backend fabricates its own Background-derived ctx.
func TestApplyHonorsContextCancel(t *testing.T) {
	b := newTestBackend()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	err := b.Apply(ctx, map[string]traffic.InterfaceQoS{})
	if err == nil {
		t.Fatalf("Apply with canceled ctx returned nil, want ctx.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Apply err = %v, want wrapped context.Canceled", err)
	}
}

// VALIDATES: AC-4 "trafficvpp.Apply with ctx that cancels during WaitConnected:
// WaitConnected returns ctx.Canceled immediately; Apply propagates".
// PREVENTS: Apply ignoring caller cancellation during the WaitConnected loop.
func TestApplyContextCancelMidWait(t *testing.T) {
	b := newTestBackend()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Apply(ctx, map[string]traffic.InterfaceQoS{})
	}()

	// Give the goroutine a moment to enter WaitConnected, then cancel.
	time.Sleep(20 * time.Millisecond)
	start := time.Now()
	cancel()

	// The whole Apply must return well under waitConnectedTimeout (5s) --
	// a passing test that only succeeds because WaitConnected's natural
	// deadline expired would not prove cancellation is honored. Budget
	// 500ms for scheduling slack on slow CI; the real cancellation path
	// returns in microseconds.
	const cancelBudget = 500 * time.Millisecond
	select {
	case err := <-errCh:
		elapsed := time.Since(start)
		if err == nil {
			t.Fatalf("Apply with mid-wait cancel returned nil, want ctx.Canceled")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Apply err = %v, want wrapped context.Canceled", err)
		}
		if elapsed > cancelBudget {
			t.Fatalf("Apply took %v to honor ctx cancel, want <%v (cancel may not have reached WaitConnected's select)",
				elapsed, cancelBudget)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Apply did not return after ctx cancel within 2s")
	}
}

// VALIDATES: trafficvpp waits for the VPP component connector to appear during cold startup.
// PREVENTS: Config delivery racing ahead of the VPP plugin and failing with "component not initialized".
func TestApplyWaitsForConnector(t *testing.T) {
	conn := vppcomp.NewConnector("/nonexistent/vpp.sock")
	ready := make(chan struct{})
	polled := make(chan struct{}, 8)
	b := &backend{
		connector: func() *vppcomp.Connector {
			select {
			case <-ready:
				return conn
			default:
				select {
				case polled <- struct{}{}:
				default:
				}
				return nil
			}
		},
		interfaceOutputPolicers: make(map[string]map[string]uint32),
		interfaceQdiscTypes:     make(map[string]traffic.QdiscType),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Apply(ctx, map[string]traffic.InterfaceQoS{})
	}()

	<-polled
	close(ready)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Apply err = %v, want wrapped context.Canceled after connector appeared", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Apply did not return after connector appeared and ctx canceled")
	}
}

// VALIDATES: trafficvpp honors cancellation while waiting for a missing VPP connector.
// PREVENTS: daemon shutdown blocking until the connector wait deadline expires.
func TestApplyConnectorWaitHonorsContextCancel(t *testing.T) {
	polled := make(chan struct{}, 8)
	b := &backend{
		connector: func() *vppcomp.Connector {
			select {
			case polled <- struct{}{}:
			default:
			}
			return nil
		},
		interfaceOutputPolicers: make(map[string]map[string]uint32),
		interfaceQdiscTypes:     make(map[string]traffic.QdiscType),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Apply(ctx, map[string]traffic.InterfaceQoS{})
	}()

	<-polled
	start := time.Now()
	cancel()

	select {
	case err := <-errCh:
		elapsed := time.Since(start)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Apply err = %v, want wrapped context.Canceled", err)
		}
		if elapsed > 500*time.Millisecond {
			t.Fatalf("Apply took %v to honor ctx cancel while waiting for connector", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Apply did not return after ctx cancel while waiting for connector")
	}
}

// fakeOps is a scripted vppOps double used by the Apply-path tests. It records
// every call as a human-readable label and can be scripted to fail on a
// specific call by name or on the Nth policerAddDel. See `vppOps` in ops.go
// for the interface contract.
type fakeOps struct {
	ifaces         map[string]interface_types.InterfaceIndex
	policerNames   []string
	calls          []string
	dumpErr        error
	dumpPolicerErr error
	// addDelFailOn: policer.Name → error to return from policerAddDel.
	addDelFailOn map[string]error
	// delFailOn: policer index → error to return from policerDel. Used to
	// exercise reconcileRemovals' warn-on-delete-error branch.
	delFailOn map[uint32]error
	// deleteByNameFailOn: policer name → error to return from policerDeleteByName.
	deleteByNameFailOn map[string]error
	// outputFailOn: policer.Name → error to return from policerOutput(any
	// apply flag). Used to exercise the warn-on-unbind-error branch.
	outputFailOn map[string]error
	// failOnNthAddDel: 1-indexed count; when >0 and addDelCount == N, fail.
	// Used to force "the 2nd interface's addDel fails" tests where map
	// iteration order is not deterministic.
	failOnNthAddDel int
	addDelCount     int
	nextIdx         uint32
}

func newFakeOps(ifaces map[string]interface_types.InterfaceIndex) *fakeOps {
	return &fakeOps{
		ifaces:             ifaces,
		addDelFailOn:       map[string]error{},
		delFailOn:          map[uint32]error{},
		deleteByNameFailOn: map[string]error{},
		outputFailOn:       map[string]error{},
	}
}

func (f *fakeOps) dumpInterfaces() (map[string]interface_types.InterfaceIndex, error) {
	f.calls = append(f.calls, "dump")
	if f.dumpErr != nil {
		return nil, f.dumpErr
	}
	out := make(map[string]interface_types.InterfaceIndex, len(f.ifaces))
	maps.Copy(out, f.ifaces)
	return out, nil
}

func (f *fakeOps) policerAddDel(req *policer.PolicerAddDel) (uint32, error) {
	f.addDelCount++
	f.calls = append(f.calls, "addDel:"+req.Name)
	if err, ok := f.addDelFailOn[req.Name]; ok {
		return 0, err
	}
	if f.failOnNthAddDel > 0 && f.addDelCount == f.failOnNthAddDel {
		return 0, fmt.Errorf("scripted fail on addDel call #%d", f.addDelCount)
	}
	f.nextIdx++
	return f.nextIdx, nil
}

func (f *fakeOps) policerDel(idx uint32) error {
	f.calls = append(f.calls, fmt.Sprintf("del:%d", idx))
	return f.delFailOn[idx]
}

func (f *fakeOps) dumpPolicers() ([]string, error) {
	f.calls = append(f.calls, "dumpPolicers")
	if f.dumpPolicerErr != nil {
		return nil, f.dumpPolicerErr
	}
	return append([]string(nil), f.policerNames...), nil
}

func (f *fakeOps) policerDeleteByName(name string) error {
	f.calls = append(f.calls, "deleteByName:"+name)
	return f.deleteByNameFailOn[name]
}

func (f *fakeOps) policerOutput(name string, swIfIndex interface_types.InterfaceIndex, apply bool) error {
	state := "off"
	if apply {
		state = "on"
	}
	f.calls = append(f.calls, fmt.Sprintf("output:%s:%s:idx=%d", name, state, swIfIndex))
	return f.outputFailOn[name]
}

// countPrefix returns the number of recorded calls starting with prefix.
func (f *fakeOps) countPrefix(prefix string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// eth0OneClassHTB is a fixed minimal InterfaceQoS: eth0 with one HTB class
// "c1" at 1 Mbps. Passes the verifier (single class, HTB qdisc, rate > 0).
// Fixed shape keeps the tests terse and deterministic; the 2-iface
// TestApplyUndoOnPartialFailure builds its map inline when it needs variation.
func eth0OneClassHTB() map[string]traffic.InterfaceQoS {
	return map[string]traffic.InterfaceQoS{
		"eth0": {
			Interface: "eth0",
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "c1", Rate: 1_000_000},
				},
			},
		},
	}
}

// applyWithOpsLocked exercises applyWithOps under the backend's mutex, matching
// the contract the production Apply uses.
func applyWithOpsLocked(b *backend, ops vppOps, desired map[string]traffic.InterfaceQoS) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.applyWithOps(ops, desired)
}

// VALIDATES: AC-5 "trafficvpp vppOps interface defined and used by Apply path:
// api.Channel no longer referenced from applyAll / applyInterface /
// reconcileRemovals".
// Also validates AC-6 "Fresh Apply (no prior state) for 1 interface + 1 class:
// Records PolicerAddDel + PolicerOutput; undo list has 2 entries" (observable
// via call sequence).
// PREVENTS: Apply path regressing to direct api.Channel calls; create path
// forgetting to bind the policer to the interface output.
func TestApplyCreatesPolicer(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})

	desired := eth0OneClassHTB()

	if err := applyWithOpsLocked(b, fake, desired); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := []string{
		"dump",
		"dumpPolicers",
		"addDel:ze/eth0/c1",
		"output:ze/eth0/c1:on:idx=5",
	}
	if got := fake.calls; !equalSlices(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	if b.interfaceOutputPolicers["eth0"]["ze/eth0/c1"] != 1 {
		t.Fatalf("interfaceOutputPolicers[eth0][ze/eth0/c1] = %d, want 1",
			b.interfaceOutputPolicers["eth0"]["ze/eth0/c1"])
	}
	if b.interfaceQdiscTypes["eth0"] != traffic.QdiscHTB {
		t.Fatalf("interfaceQdiscTypes[eth0] = %v, want HTB", b.interfaceQdiscTypes["eth0"])
	}
}

// VALIDATES: startup orphan scan deletes ze-owned VPP policers that are absent
// from the desired config, while preserving desired ze policers and foreign
// policers.
// PREVENTS: old Ze process state continuing to police traffic after daemon restart.
func TestStartupOrphanScanDeletesUndesiredZePolicers(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake.policerNames = []string{
		"ze/eth0/old",
		"ze/eth0/c1",
		"foreign/policer",
	}

	if err := applyWithOpsLocked(b, fake, eth0OneClassHTB()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := []string{
		"dump",
		"dumpPolicers",
		"output:ze/eth0/old:off:idx=5",
		"deleteByName:ze/eth0/old",
		"output:ze/eth0/c1:off:idx=5",
		"addDel:ze/eth0/c1",
		"output:ze/eth0/c1:on:idx=5",
	}
	if got := fake.calls; !equalSlices(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
}

// VALIDATES: AC-7 "Second Apply with identical config: Records PolicerAddDel
// only (no PolicerOutput, no undo queued)". The "no undo queued" part is
// observed indirectly -- a later failure on the same UPDATE would not rewind
// the previous Apply's output binding, which this test demonstrates by
// confirming zero output calls during the upsert.
// PREVENTS: UPDATE path redundantly calling PolicerOutput on every reload or
// queueing undo that would tear down a live binding on a later iface error.
func TestApplyUpdatesPolicer(t *testing.T) {
	b := newOpsBackend()
	desired := eth0OneClassHTB()

	// First apply to establish prior state.
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, desired); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// Second apply with identical config on a fresh fake.
	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake2, desired); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	want := []string{"dump", "addDel:ze/eth0/c1"}
	if got := fake2.calls; !equalSlices(got, want) {
		t.Fatalf("second-apply calls = %v, want %v", got, want)
	}
}

// VALIDATES: AC-8 "Apply of iface2 fails after iface1 succeeded: Undo runs in
// reverse; fakeOps shows iface1 unbind + del called".
// PREVENTS: the undo list leaking partial state into VPP after a multi-iface
// Apply fails.
func TestApplyUndoOnPartialFailure(t *testing.T) {
	b := newOpsBackend()
	// Two interfaces, deterministic outcome via failOnNthAddDel=2: whichever
	// iface is processed first succeeds and gets undone; the second fails.
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{
		"eth0": 5,
		"eth1": 6,
	})
	fake.failOnNthAddDel = 2

	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Interface: "eth0",
			Qdisc: traffic.Qdisc{
				Type:    traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{{Name: "c1", Rate: 1_000_000}},
			},
		},
		"eth1": {
			Interface: "eth1",
			Qdisc: traffic.Qdisc{
				Type:    traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{{Name: "c1", Rate: 2_000_000}},
			},
		},
	}

	err := applyWithOpsLocked(b, fake, desired)
	if err == nil {
		t.Fatalf("Apply with scripted failure returned nil, want error")
	}

	// Expected sequence (order-dependent on map iteration of successful iface):
	//   dump, dumpPolicers, addDel:<first>, output:<first>:on, addDel:<second> (fails),
	//   output:<first>:off (undo 2), del:<first idx> (undo 1)
	if got := len(fake.calls); got != 7 {
		t.Fatalf("calls = %v (len=%d), want 7", fake.calls, got)
	}
	if fake.calls[0] != "dump" {
		t.Fatalf("calls[0] = %q, want dump", fake.calls[0])
	}
	if fake.calls[1] != "dumpPolicers" {
		t.Fatalf("calls[1] = %q, want dumpPolicers", fake.calls[1])
	}
	// Exactly 1 on-binding and 1 off-binding (the off from undo).
	if n := fake.countPrefix("output:"); n != 2 {
		t.Fatalf("output call count = %d, want 2 (1 on + 1 off from undo); calls=%v", n, fake.calls)
	}
	offCount := 0
	for _, c := range fake.calls {
		if strings.Contains(c, ":off:") {
			offCount++
		}
	}
	if offCount != 1 {
		t.Fatalf("expected exactly 1 off-binding in undo, got %d; calls=%v", offCount, fake.calls)
	}
	if n := fake.countPrefix("del:"); n != 1 {
		t.Fatalf("del call count = %d, want 1 (undo); calls=%v", n, fake.calls)
	}
	// After rollback, no iface state should remain recorded.
	if len(b.interfaceOutputPolicers) != 0 {
		t.Fatalf("interfaceOutputPolicers after failed apply = %v, want empty", b.interfaceOutputPolicers)
	}
}

// VALIDATES: AC-9 "Apply that drops iface1 from desired (previously had 1
// class): reconcileRemovals calls PolicerOutput(apply=false) + PolicerDel for
// iface1".
// PREVENTS: policer + binding leaks in VPP when the operator removes an
// interface from traffic-control config.
func TestReconcileRemovesDropped(t *testing.T) {
	b := newOpsBackend()
	desired := eth0OneClassHTB()

	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, desired); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// Second apply: empty desired, iface still present in VPP.
	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake2, map[string]traffic.InterfaceQoS{}); err != nil {
		t.Fatalf("reconcile apply: %v", err)
	}

	want := []string{
		"dump",
		"output:ze/eth0/c1:off:idx=5",
		"del:1",
	}
	if got := fake2.calls; !equalSlices(got, want) {
		t.Fatalf("reconcile calls = %v, want %v", got, want)
	}
	if len(b.interfaceOutputPolicers) != 0 {
		t.Fatalf("interfaceOutputPolicers after reconcile = %v, want empty", b.interfaceOutputPolicers)
	}
}

// VALIDATES: AC-10 "Apply where an iface present before is missing from VPP
// now: reconcileRemovals SKIPS unbind (no interface) but STILL calls
// PolicerDel".
// PREVENTS: policer leaks in VPP when an interface was deleted out-of-band
// between Apply calls (e.g., VPP restart with ephemeral interface).
func TestReconcileOrphanFixDeletesPolicer(t *testing.T) {
	b := newOpsBackend()
	desired := eth0OneClassHTB()

	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, desired); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// Second apply: empty desired, iface vanished from VPP dump.
	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{})
	if err := applyWithOpsLocked(b, fake2, map[string]traffic.InterfaceQoS{}); err != nil {
		t.Fatalf("reconcile apply: %v", err)
	}

	// No unbind call (interface absent from nameIndex), but PolicerDel still
	// runs so the named policer entity doesn't leak.
	if got := fake2.calls; !equalSlices(got, []string{"dump", "del:1"}) {
		t.Fatalf("orphan reconcile calls = %v, want [dump del:1]", got)
	}
	if n := fake2.countPrefix("output:"); n != 0 {
		t.Fatalf("orphan reconcile made %d output calls, want 0", n)
	}
}

// VALIDATES: reconcileRemovals tolerates PolicerDel/PolicerOutput errors
// (VPP-side staleness after a daemon restart). The backend logs a warn and
// continues instead of failing the whole Apply, so the newly-desired state
// still lands.
// PREVENTS: a single stale policer rejecting its deletion from aborting an
// entire reload and leaving the new config unapplied.
func TestReconcileWarnsOnVPPDeleteError(t *testing.T) {
	b := newOpsBackend()

	// First apply establishes one policer bound to eth0.
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, eth0OneClassHTB()); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// Second apply: empty desired (drop eth0's policer). Script VPP to fail
	// both the unbind AND the delete -- reconcileRemovals must log warns
	// and still return nil so the caller commits the new (empty) state.
	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake2.outputFailOn["ze/eth0/c1"] = errors.New("scripted vpp unbind error")
	fake2.delFailOn[1] = errors.New("scripted vpp delete error")

	if err := applyWithOpsLocked(b, fake2, map[string]traffic.InterfaceQoS{}); err != nil {
		t.Fatalf("reconcile apply returned %v, want nil (warn-path must not fail Apply)", err)
	}

	want := []string{
		"dump",
		"output:ze/eth0/c1:off:idx=5",
		"del:1",
	}
	if got := fake2.calls; !equalSlices(got, want) {
		t.Fatalf("reconcile calls = %v, want %v", got, want)
	}
	// Caller state cleared even though VPP rejected both ops: the backend
	// considers the policer gone from its tracker so it won't try again.
	if len(b.interfaceOutputPolicers) != 0 {
		t.Fatalf("interfaceOutputPolicers after warn-path reconcile = %v, want empty", b.interfaceOutputPolicers)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
