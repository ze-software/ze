// Design: plan/learned/629-fw-7b-backend-hardening.md -- Apply-path tests for vpp backend.

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

	"github.com/ze-software/ze/internal/component/traffic"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

// newTestBackend builds a backend whose connector returns an unconnected
// Connector. Apply reaches WaitConnected but never completes a real VPP call,
// which is what the ctx-cancellation tests need.
func newTestBackend() *backend {
	conn := vppcomp.NewConnector("/nonexistent/vpp.sock")
	return &backend{
		connector:                 func() *vppcomp.Connector { return conn },
		interfaceOutputPolicers:   make(map[string]map[string]uint32),
		interfaceQdiscTypes:       make(map[string]traffic.QdiscType),
		interfaceClassifyBindings: make(map[string]classifyBinding),
	}
}

// newOpsBackend builds a backend with no connector (applyWithOps bypasses it)
// ready for scripted fakeOps injection.
func newOpsBackend() *backend {
	return &backend{
		interfaceOutputPolicers:   make(map[string]map[string]uint32),
		interfaceQdiscTypes:       make(map[string]traffic.QdiscType),
		interfaceClassifyBindings: make(map[string]classifyBinding),
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

	// nextTableIdx is the next classify table index handed out by
	// classifyAddDelTable (mirrors VPP's auto-assignment on create).
	nextTableIdx uint32
	// classifyTableFail / classifySessionFail / policerClassifyFail script
	// failures on the classify pipeline calls to exercise undo paths.
	classifyTableFail   error
	classifySessionFail error
	policerClassifyFail error
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

func (f *fakeOps) classifyAddDelTable(tableIdx uint32, mask []byte, skipNVectors, nextTableIdx uint32, isAdd bool) (uint32, error) {
	if isAdd {
		f.calls = append(f.calls, fmt.Sprintf("clTable:add:skip=%d:masklen=%d:next=%d", skipNVectors, len(mask), int32(nextTableIdx)))
		if f.classifyTableFail != nil {
			return 0, f.classifyTableFail
		}
		idx := f.nextTableIdx
		f.nextTableIdx++
		return idx, nil
	}
	f.calls = append(f.calls, fmt.Sprintf("clTable:del:idx=%d", tableIdx))
	return tableIdx, nil
}

func (f *fakeOps) classifyAddDelSession(tableIdx, hitNextIndex uint32, match []byte, isAdd bool) error {
	state := "add"
	if !isAdd {
		state = "del"
	}
	f.calls = append(f.calls, fmt.Sprintf("clSession:%s:tbl=%d:hit=%d:matchlen=%d", state, tableIdx, hitNextIndex, len(match)))
	if isAdd {
		return f.classifySessionFail
	}
	return nil
}

func (f *fakeOps) policerClassifySetInterface(swIfIndex interface_types.InterfaceIndex, ip4TableIdx, ip6TableIdx uint32, isAdd bool) error {
	state := "off"
	if isAdd {
		state = "on"
	}
	f.calls = append(f.calls, fmt.Sprintf("polClassify:%s:ip4=%d:ip6=%d:idx=%d", state, int32(ip4TableIdx), int32(ip6TableIdx), swIfIndex))
	if isAdd {
		return f.policerClassifyFail
	}
	return nil
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

// eth0OneClassProtoHTB is eth0 with one HTB class carrying a protocol filter
// (TCP=6). The class's policer is bound via the classify pipeline (ip4+ip6
// tables + policer-classify), not the egress policer-output path.
func eth0OneClassProtoHTB() map[string]traffic.InterfaceQoS {
	return map[string]traffic.InterfaceQoS{
		"eth0": {
			Interface: "eth0",
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{
						Name:    "c1",
						Rate:    1_000_000,
						Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}},
					},
				},
			},
		},
	}
}

// VALIDATES: AC-1 / R-1 -- a class with a protocol filter creates one ip4 and
// one ip6 classify table (a session each, steering to the class policer),
// binds them to the interface policer-classify feature (the R-1 "table created
// but never attached" killer), and does NOT bind the egress policer-output.
func TestApplyFilterProtocolAttaches(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})

	if err := applyWithOpsLocked(b, fake, eth0OneClassProtoHTB()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if n := fake.countPrefix("addDel:ze/eth0/c1"); n != 1 {
		t.Fatalf("policer addDel count = %d, want 1; calls=%v", n, fake.calls)
	}
	if n := fake.countPrefix("clTable:add"); n != 2 {
		t.Fatalf("classify table add count = %d, want 2 (ip4+ip6); calls=%v", n, fake.calls)
	}
	if n := fake.countPrefix("clSession:add"); n != 2 {
		t.Fatalf("classify session add count = %d, want 2; calls=%v", n, fake.calls)
	}
	// R-1 killer: the tables MUST be attached to the interface.
	if n := fake.countPrefix("polClassify:on"); n != 1 {
		t.Fatalf("policer-classify bind count = %d, want 1 (R-1: table never attached); calls=%v", n, fake.calls)
	}
	// Steering: every session's hit index is the class policer index (fake
	// hands out index 1 for the first policer).
	for _, c := range fake.calls {
		if strings.HasPrefix(c, "clSession:add") && !strings.Contains(c, ":hit=1:") {
			t.Fatalf("session %q does not steer to policer idx 1; calls=%v", c, fake.calls)
		}
	}
	// A filtered class must NOT take the egress policer-output path.
	if n := fake.countPrefix("output:"); n != 0 {
		t.Fatalf("filtered class made %d policer-output calls, want 0; calls=%v", n, fake.calls)
	}
	bnd, ok := b.interfaceClassifyBindings["eth0"]
	if !ok {
		t.Fatalf("no classify binding recorded for eth0; bindings=%v", b.interfaceClassifyBindings)
	}
	if len(bnd.ip4Tables) == 0 || len(bnd.ip6Tables) == 0 {
		t.Fatalf("binding missing a family table chain: %+v", bnd)
	}
	if _, present := bnd.policers["ze/eth0/c1"]; !present {
		t.Fatalf("binding missing the filtered policer: %+v", bnd)
	}
	// The filtered policer is tracked in the binding, NOT in the output map.
	if _, present := b.interfaceOutputPolicers["eth0"]["ze/eth0/c1"]; present {
		t.Fatalf("filtered policer must not be tracked as an output policer")
	}
}

// VALIDATES: AC-1 undo -- a classify session failure unwinds the classify
// table(s) and the policer created in the same Apply (pre-Apply state).
func TestApplyFilterProtocolUndo(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake.classifySessionFail = errors.New("scripted session fail")

	err := applyWithOpsLocked(b, fake, eth0OneClassProtoHTB())
	if err == nil {
		t.Fatal("Apply with scripted classify-session failure returned nil, want error")
	}
	if n := fake.countPrefix("clTable:del"); n < 1 {
		t.Fatalf("undo classify table del count = %d, want >=1; calls=%v", n, fake.calls)
	}
	if n := fake.countPrefix("del:"); n != 1 {
		t.Fatalf("undo policer del count = %d, want 1; calls=%v", n, fake.calls)
	}
	if len(b.interfaceClassifyBindings) != 0 {
		t.Fatalf("classify bindings after failed apply = %v, want empty", b.interfaceClassifyBindings)
	}
	if len(b.interfaceOutputPolicers) != 0 {
		t.Fatalf("output policers after failed apply = %v, want empty", b.interfaceOutputPolicers)
	}
}

// VALIDATES: AC-1 reconcile -- dropping a filtered interface from desired tears
// down its classify tables (unbind + delete both families) and deletes the
// classify-bound policer, with no spurious policer-output unbind.
func TestApplyReconcileClassifyTeardown(t *testing.T) {
	b := newOpsBackend()
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, eth0OneClassProtoHTB()); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake2, map[string]traffic.InterfaceQoS{}); err != nil {
		t.Fatalf("reconcile apply: %v", err)
	}

	if n := fake2.countPrefix("polClassify:off"); n != 1 {
		t.Fatalf("classify unbind count = %d, want 1; calls=%v", n, fake2.calls)
	}
	if n := fake2.countPrefix("clTable:del"); n != 2 {
		t.Fatalf("classify table del count = %d, want 2 (ip4+ip6); calls=%v", n, fake2.calls)
	}
	if n := fake2.countPrefix("del:"); n != 1 {
		t.Fatalf("policer del count = %d, want 1 (classify policer); calls=%v", n, fake2.calls)
	}
	if n := fake2.countPrefix("output:"); n != 0 {
		t.Fatalf("reconcile made %d policer-output calls, want 0 (filtered policer never output-bound); calls=%v", n, fake2.calls)
	}
	if len(b.interfaceClassifyBindings) != 0 {
		t.Fatalf("classify bindings after teardown = %v, want empty", b.interfaceClassifyBindings)
	}
}

// applyWithOpsLocked exercises applyWithOps under the backend's mutex, matching
// the contract the production Apply uses.
func applyWithOpsLocked(b *backend, ops vppOps, desired map[string]traffic.InterfaceQoS) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.applyWithOps(ops, desired)
}

// eth0OneClassDscpHTB is eth0 with one HTB class carrying a dscp filter (cs6=48).
// Police-by-dscp: the class policer is bound via the classify pipeline (ip4+ip6
// tables matching the TOS/TC bits), not the egress policer-output path.
func eth0OneClassDscpHTB() map[string]traffic.InterfaceQoS {
	return map[string]traffic.InterfaceQoS{
		"eth0": {
			Interface: "eth0",
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "c1", Rate: 1_000_000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterDSCP, Value: 48}}},
				},
			},
		},
	}
}

// VALIDATES: AC-2 (police-by-dscp) / R-2 -- a class with a dscp filter creates
// one ip4 and one ip6 classify table (a session each, steering to the class
// policer), binds them to the interface policer-classify feature, and does NOT
// bind the egress policer-output. This is the R-2 "dscp classifies+steers, not
// remarks" proof: the pipeline is identical to protocol.
func TestApplyFilterDscpAttaches(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})

	if err := applyWithOpsLocked(b, fake, eth0OneClassDscpHTB()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := fake.countPrefix("clTable:add"); n != 2 {
		t.Fatalf("classify table add count = %d, want 2 (ip4+ip6); calls=%v", n, fake.calls)
	}
	if n := fake.countPrefix("clSession:add"); n != 2 {
		t.Fatalf("classify session add count = %d, want 2; calls=%v", n, fake.calls)
	}
	if n := fake.countPrefix("polClassify:on"); n != 1 {
		t.Fatalf("policer-classify bind count = %d, want 1; calls=%v", n, fake.calls)
	}
	for _, c := range fake.calls {
		if strings.HasPrefix(c, "clSession:add") && !strings.Contains(c, ":hit=1:") {
			t.Fatalf("dscp session %q does not steer to policer idx 1; calls=%v", c, fake.calls)
		}
	}
	if n := fake.countPrefix("output:"); n != 0 {
		t.Fatalf("dscp filtered class made %d policer-output calls, want 0; calls=%v", n, fake.calls)
	}
	bnd, ok := b.interfaceClassifyBindings["eth0"]
	if !ok || len(bnd.ip4Tables) == 0 || len(bnd.ip6Tables) == 0 {
		t.Fatalf("dscp classify binding missing a family chain: %+v", bnd)
	}
}

// twoClassProtoHTB is eth0 with two HTB classes, each carrying a DIFFERENT
// protocol filter (tcp=6, udp=17). Same field type -> one table per family with
// two sessions steering to two distinct policers (no chaining needed).
func twoClassProtoHTB() map[string]traffic.InterfaceQoS {
	return map[string]traffic.InterfaceQoS{
		"eth0": {
			Interface: "eth0",
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "web", Rate: 10_000_000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}}},
					{Name: "dns", Rate: 1_000_000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 17}}},
				},
			},
		},
	}
}

// VALIDATES: AC-5 -- multi-class HTB with a protocol filter on every class
// creates per-class policers and steers each class's traffic to its own policer.
// Same-field classes share ONE table per family (two sessions, distinct hit
// indices) -- no chaining. Real VPP v25.10 confirmed multi-session per table.
func TestApplyMultiClassSteering(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})

	if err := applyWithOpsLocked(b, fake, twoClassProtoHTB()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := fake.countPrefix("addDel:ze/eth0/"); n != 2 {
		t.Fatalf("policer addDel count = %d, want 2 (per-class); calls=%v", n, fake.calls)
	}
	// Same field mask across both classes -> ONE table per family (not 2).
	if n := fake.countPrefix("clTable:add"); n != 2 {
		t.Fatalf("classify table add count = %d, want 2 (ip4+ip6 single group); calls=%v", n, fake.calls)
	}
	if n := fake.countPrefix("clSession:add"); n != 4 {
		t.Fatalf("classify session add count = %d, want 4 (2 protos x 2 families); calls=%v", n, fake.calls)
	}
	if n := fake.countPrefix("polClassify:on"); n != 1 {
		t.Fatalf("policer-classify bind count = %d, want 1; calls=%v", n, fake.calls)
	}
	// Two distinct policer hit indices appear across the sessions (each class
	// steers to its own policer). fake hands out 1 and 2.
	hits := map[string]bool{}
	for _, c := range fake.calls {
		if strings.HasPrefix(c, "clSession:add") {
			if strings.Contains(c, ":hit=1:") {
				hits["1"] = true
			}
			if strings.Contains(c, ":hit=2:") {
				hits["2"] = true
			}
		}
	}
	if !hits["1"] || !hits["2"] {
		t.Fatalf("expected sessions steering to BOTH policers 1 and 2; calls=%v", fake.calls)
	}
	bnd := b.interfaceClassifyBindings["eth0"]
	if len(bnd.policers) != 2 {
		t.Fatalf("binding should track 2 filtered policers, got %d: %+v", len(bnd.policers), bnd)
	}
	// Single group per family -> chain length 1 (no NextTableIndex chaining).
	if len(bnd.ip4Tables) != 1 || len(bnd.ip6Tables) != 1 {
		t.Fatalf("same-field multi-class should yield 1 table per family, got ip4=%d ip6=%d", len(bnd.ip4Tables), len(bnd.ip6Tables))
	}
}

// VALIDATES: AC-5 (mixed-field) -- classes filtering on DIFFERENT fields
// (protocol vs dscp) need distinct masks, so their tables are CHAINED per family
// via NextTableIndex. Real VPP v25.10 confirmed the chain fall-through.
func TestApplyMultiClassMixedFieldChains(t *testing.T) {
	b := newOpsBackend()
	fake := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Interface: "eth0",
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "web", Rate: 10_000_000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}}},
					{Name: "voip", Rate: 1_000_000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterDSCP, Value: 46}}},
				},
			},
		},
	}
	if err := applyWithOpsLocked(b, fake, desired); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// Two distinct masks per family -> 2 tables per family -> 4 total.
	if n := fake.countPrefix("clTable:add"); n != 4 {
		t.Fatalf("classify table add count = %d, want 4 (2 masks x 2 families); calls=%v", n, fake.calls)
	}
	// Per family the head table chains to its successor: exactly 2 tables carry
	// a non-terminal next index (the two heads), 2 are chain tails (next=-1).
	heads, tails := 0, 0
	for _, c := range fake.calls {
		if !strings.HasPrefix(c, "clTable:add") {
			continue
		}
		if strings.Contains(c, ":next=-1") {
			tails++
		} else {
			heads++
		}
	}
	if heads != 2 || tails != 2 {
		t.Fatalf("chain shape wrong: heads=%d tails=%d (want 2/2); calls=%v", heads, tails, fake.calls)
	}
	bnd := b.interfaceClassifyBindings["eth0"]
	if len(bnd.ip4Tables) != 2 || len(bnd.ip6Tables) != 2 {
		t.Fatalf("mixed-field multi-class should yield 2 tables per family, got ip4=%d ip6=%d", len(bnd.ip4Tables), len(bnd.ip6Tables))
	}
}

// VALIDATES: AC-5 reconcile -- dropping one filtered class from a multi-class
// interface deletes that class's policer but keeps the surviving class's. Both
// families' old tables are replaced (deleted) as the fresh chain is rebound.
func TestApplyMultiClassReconcileDropsOneClass(t *testing.T) {
	b := newOpsBackend()
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, twoClassProtoHTB()); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	// Second apply: only the "web" class remains (dns dropped).
	oneClass := map[string]traffic.InterfaceQoS{
		"eth0": {
			Interface: "eth0",
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "web", Rate: 10_000_000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}}},
				},
			},
		},
	}
	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake2, oneClass); err != nil {
		t.Fatalf("reconcile apply: %v", err)
	}
	// The dropped "dns" policer must be deleted exactly once; "web" survives.
	if n := fake2.countPrefix("del:"); n != 1 {
		t.Fatalf("reconcile policer del count = %d, want 1 (dropped dns only); calls=%v", n, fake2.calls)
	}
	bnd := b.interfaceClassifyBindings["eth0"]
	if _, ok := bnd.policers["ze/eth0/web"]; !ok {
		t.Fatalf("surviving policer ze/eth0/web missing from binding: %+v", bnd)
	}
	if _, ok := bnd.policers["ze/eth0/dns"]; ok {
		t.Fatalf("dropped policer ze/eth0/dns still tracked: %+v", bnd)
	}
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

// VALIDATES: Second Apply with identical config replays PolicerOutput(apply=true)
// for the existing policer.
// PREVENTS: same-process drift where VPP loses the output feature binding
// out-of-band and Ze skips rebind because the policer is already tracked.
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

	want := []string{"dump", "addDel:ze/eth0/c1", "output:ze/eth0/c1:on:idx=5"}
	if got := fake2.calls; !equalSlices(got, want) {
		t.Fatalf("second-apply calls = %v, want %v", got, want)
	}
}

// VALIDATES: UPDATE rebind failure returns an error without queueing CREATE-style
// delete/unbind undo for an existing policer.
// PREVENTS: a failed same-process rebind tearing down previously-working traffic
// shaping before the component journal can retry the previous desired state.
func TestApplyUpdateRebindFailureDoesNotUndoExistingPolicer(t *testing.T) {
	b := newOpsBackend()
	desired := eth0OneClassHTB()

	// First apply to establish prior state.
	fake1 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	if err := applyWithOpsLocked(b, fake1, desired); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// Second apply updates the existing policer, then fails while rebinding it.
	fake2 := newFakeOps(map[string]interface_types.InterfaceIndex{"eth0": 5})
	fake2.nextIdx = 99
	fake2.outputFailOn["ze/eth0/c1"] = errors.New("scripted rebind failure")
	err := applyWithOpsLocked(b, fake2, desired)
	if err == nil {
		t.Fatalf("second apply returned nil, want rebind error")
	}

	want := []string{"dump", "addDel:ze/eth0/c1", "output:ze/eth0/c1:on:idx=5"}
	if got := fake2.calls; !equalSlices(got, want) {
		t.Fatalf("second-apply calls = %v, want %v", got, want)
	}
	if got := b.interfaceOutputPolicers["eth0"]["ze/eth0/c1"]; got != 1 {
		t.Fatalf("interfaceOutputPolicers[eth0][ze/eth0/c1] = %d, want prior idx 1", got)
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
