// Design: plan/learned/656-deployment-readiness-review.md -- tc original-qdisc restore regressions

//go:build linux

package trafficnetlink

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

type fakeTCOps struct {
	links    map[string]netlink.Link
	qdiscs   map[string][]netlink.Qdisc
	classes  map[string][]netlink.Class
	filters  map[string][]netlink.Filter
	calls    []string
	replaced []netlink.Qdisc
}

func newFakeTCOps() *fakeTCOps {
	return &fakeTCOps{
		links:   map[string]netlink.Link{},
		qdiscs:  map[string][]netlink.Qdisc{},
		classes: map[string][]netlink.Class{},
		filters: map[string][]netlink.Filter{},
	}
}

func (f *fakeTCOps) linkByName(name string) (netlink.Link, error) {
	f.calls = append(f.calls, "link:"+name)
	link, ok := f.links[name]
	if !ok {
		return nil, fmt.Errorf("link %q not found", name)
	}
	return link, nil
}

func (f *fakeTCOps) qdiscList(link netlink.Link) ([]netlink.Qdisc, error) {
	name := link.Attrs().Name
	f.calls = append(f.calls, "qdiscList:"+name)
	return append([]netlink.Qdisc(nil), f.qdiscs[name]...), nil
}

func (f *fakeTCOps) qdiscReplace(qdisc netlink.Qdisc) error {
	f.calls = append(f.calls, "replace:"+qdisc.Type())
	f.replaced = append(f.replaced, qdisc)
	return nil
}

func (f *fakeTCOps) classList(link netlink.Link, _ uint32) ([]netlink.Class, error) {
	name := link.Attrs().Name
	f.calls = append(f.calls, "classList:"+name)
	return append([]netlink.Class(nil), f.classes[name]...), nil
}

func (f *fakeTCOps) classAdd(class netlink.Class) error {
	f.calls = append(f.calls, "classAdd:"+class.Type())
	return nil
}

func (f *fakeTCOps) filterList(link netlink.Link, _ uint32) ([]netlink.Filter, error) {
	name := link.Attrs().Name
	f.calls = append(f.calls, "filterList:"+name)
	return append([]netlink.Filter(nil), f.filters[name]...), nil
}

func (f *fakeTCOps) filterAdd(filter netlink.Filter) error {
	f.calls = append(f.calls, "filterAdd:"+filter.Type())
	return nil
}

// registerSnapshotStore creates an empty database.zefs and registers it as the
// process-wide statestore so tc snapshot persistence round-trips through the real
// shared zefs store (not a loose file). statestore never creates the store, so
// tests must materialize it first. The store is kept open for the test's lifetime
// (Get/Put/Remove share this one handle) and reset to filesystem-fallback on
// cleanup so a later test that expects no store is not polluted.
func registerSnapshotStore(t *testing.T) {
	t.Helper()
	bs, err := zefs.Create(filepath.Join(t.TempDir(), "database.zefs"))
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	statestore.SetStore(bs)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		// The store stays open for the test's lifetime (shared handle) and is
		// closed best-effort at cleanup, so a close error no longer fails the test
		// as it did when each op reopened the file.
		bs.Close() //nolint:errcheck // best-effort cleanup
	})
}

func testBackend(t *testing.T, ops *fakeTCOps) *backend {
	t.Helper()
	registerSnapshotStore(t)
	return newBackendWithOps(ops, nil, "boot-1", nil)
}

func testLink(name string, index int) netlink.Link {
	hw, _ := net.ParseMAC("02:00:00:00:00:01")
	return &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name, Index: index, HardwareAddr: hw}}
}

func rootAttrs(index int) netlink.QdiscAttrs {
	return netlink.QdiscAttrs{LinkIndex: index, Handle: 0, Parent: netlink.HANDLE_ROOT}
}

func originalFQ(index int) netlink.Qdisc {
	return &netlink.Fq{QdiscAttrs: rootAttrs(index), Pacing: 1, Quantum: 1514}
}

func desiredHTB(iface string) traffic.InterfaceQoS {
	return traffic.InterfaceQoS{
		Interface: iface,
		Qdisc: traffic.Qdisc{
			Type: traffic.QdiscHTB,
			Classes: []traffic.TrafficClass{
				{Name: "default", Rate: 1_000_000, Ceil: 1_000_000},
			},
		},
	}
}

func TestTranslateHTBUsesKernelDefaults(t *testing.T) {
	qdisc, err := translateQdisc(traffic.Qdisc{
		Type:         traffic.QdiscHTB,
		DefaultClass: "default",
		Classes: []traffic.TrafficClass{
			{Name: "default", Rate: 1_000_000, Ceil: 1_000_000},
		},
	}, 5)
	if err != nil {
		t.Fatalf("translateQdisc: %v", err)
	}
	htb, ok := qdisc.(*netlink.Htb)
	if !ok {
		t.Fatalf("translateQdisc returned %T, want *netlink.Htb", qdisc)
	}
	if htb.Version != 3 || htb.Rate2Quantum != 10 {
		t.Fatalf("htb defaults = version %d rate2quantum %d, want version 3 rate2quantum 10", htb.Version, htb.Rate2Quantum)
	}
	if htb.Defcls != 1 {
		t.Fatalf("htb default class = %d, want 1", htb.Defcls)
	}
}

func TestApplySnapshotsOriginalBeforeReplace(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	b := testBackend(t, ops)

	err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := []string{"link:eth0", "qdiscList:eth0", "classList:eth0", "filterList:eth0", "replace:htb", "classAdd:htb"}
	if got := ops.calls; !equalStringSlices(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	if got := b.snapshots["eth0"].Qdisc.Type; got != "fq" {
		t.Fatalf("snapshot qdisc = %q, want fq", got)
	}
	persisted, err := loadTCSnapshots()
	if err != nil {
		t.Fatalf("load persisted snapshots: %v", err)
	}
	if got := persisted["eth0"].Qdisc.Type; got != "fq" {
		t.Fatalf("persisted snapshot qdisc = %q, want fq", got)
	}
}

func TestRestoreOriginalUsesSnapshotNotFQCodelDefault(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	b := testBackend(t, ops)

	if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	ops.calls = nil
	ops.replaced = nil

	if err := b.RestoreOriginal(context.Background(), "eth0"); err != nil {
		t.Fatalf("RestoreOriginal: %v", err)
	}

	want := []string{"link:eth0", "replace:fq"}
	if got := ops.calls; !equalStringSlices(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	if len(b.snapshots) != 0 {
		t.Fatalf("snapshots after restore = %v, want empty", b.snapshots)
	}
	persisted, err := loadTCSnapshots()
	if err != nil {
		t.Fatalf("load persisted snapshots after restore: %v", err)
	}
	if len(persisted) != 0 {
		t.Fatalf("persisted snapshots after restore = %v, want empty", persisted)
	}
}

func TestApplyRejectsUnrestorableOriginalBeforeReplace(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{
		&netlink.GenericQdisc{QdiscAttrs: rootAttrs(5), QdiscType: "cake"},
	}
	b := testBackend(t, ops)

	err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")})
	if err == nil {
		t.Fatal("Apply returned nil, want unrestorable qdisc error")
	}
	if !strings.Contains(err.Error(), "cannot be snapshotted exactly") {
		t.Fatalf("Apply error = %v, want exact snapshot rejection", err)
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "replace:") {
			t.Fatalf("Apply replaced qdisc before rejecting: calls=%v", ops.calls)
		}
	}
}

func TestApplyRejectsOriginalWithClassStateBeforeReplace(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	ops.classes["eth0"] = []netlink.Class{
		&netlink.GenericClass{ClassAttrs: netlink.ClassAttrs{LinkIndex: 5}, ClassType: "foreign"},
	}
	b := testBackend(t, ops)

	err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")})
	if err == nil {
		t.Fatal("Apply returned nil, want class-state rejection")
	}
	if !strings.Contains(err.Error(), "cannot snapshot class state exactly") {
		t.Fatalf("Apply error = %v, want class-state rejection", err)
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "replace:") {
			t.Fatalf("Apply replaced qdisc before rejecting: calls=%v", ops.calls)
		}
	}
}

func TestPersistedSnapshotSurvivesBackendRestart(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	registerSnapshotStore(t)
	snap, err := newInterfaceSnapshot(ops.links["eth0"], "boot-1", originalFQ(5))
	if err != nil {
		t.Fatalf("newInterfaceSnapshot: %v", err)
	}
	if err := saveTCSnapshots(map[string]tcInterfaceSnapshot{"eth0": snap}); err != nil {
		t.Fatalf("saveTCSnapshots: %v", err)
	}
	loaded, err := loadTCSnapshots()
	if err != nil {
		t.Fatalf("loadTCSnapshots: %v", err)
	}
	b := newBackendWithOps(ops, nil, "boot-1", loaded)

	if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")}); err != nil {
		t.Fatalf("Apply with persisted snapshot: %v", err)
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "qdiscList:") {
			t.Fatalf("Apply re-snapshotted despite persisted snapshot: calls=%v", ops.calls)
		}
	}
	ops.calls = nil

	if err := b.RestoreOriginal(context.Background(), "eth0"); err != nil {
		t.Fatalf("RestoreOriginal persisted snapshot: %v", err)
	}
	if got, want := ops.calls, []string{"link:eth0", "replace:fq"}; !equalStringSlices(got, want) {
		t.Fatalf("restore calls = %v, want %v", got, want)
	}
}

func TestPersistedSnapshotRejectsChangedLinkIdentity(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	snap, err := newInterfaceSnapshot(testLink("eth0", 6), "boot-1", originalFQ(6))
	if err != nil {
		t.Fatalf("newInterfaceSnapshot: %v", err)
	}
	registerSnapshotStore(t)
	b := newBackendWithOps(ops, nil, "boot-1", map[string]tcInterfaceSnapshot{"eth0": snap})

	err = b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")})
	if err == nil {
		t.Fatal("Apply returned nil, want link identity rejection")
	}
	if !strings.Contains(err.Error(), "ifindex") {
		t.Fatalf("Apply error = %v, want ifindex mismatch", err)
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "replace:") {
			t.Fatalf("Apply replaced qdisc despite identity mismatch: calls=%v", ops.calls)
		}
	}
}

func TestApplyRejectsOriginalWithFilterStateBeforeReplace(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	ops.filters["eth0"] = []netlink.Filter{
		&netlink.GenericFilter{FilterAttrs: netlink.FilterAttrs{LinkIndex: 5}, FilterType: "u32"},
	}
	b := testBackend(t, ops)

	err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")})
	if err == nil {
		t.Fatal("Apply returned nil, want filter-state rejection")
	}
	if !strings.Contains(err.Error(), "cannot snapshot filter state exactly") {
		t.Fatalf("Apply error = %v, want filter-state rejection", err)
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "replace:") {
			t.Fatalf("Apply replaced qdisc before rejecting: calls=%v", ops.calls)
		}
	}
}

func TestCloseRestoresAllOwnedInterfaces(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.links["eth1"] = testLink("eth1", 6)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	hw2, _ := net.ParseMAC("02:00:00:00:00:01")
	ops.links["eth1"] = &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "eth1", Index: 6, HardwareAddr: hw2}}
	ops.qdiscs["eth1"] = []netlink.Qdisc{originalFQ(6)}
	b := testBackend(t, ops)

	desired := map[string]traffic.InterfaceQoS{
		"eth0": desiredHTB("eth0"),
		"eth1": desiredHTB("eth1"),
	}
	if err := b.Apply(context.Background(), desired); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(b.snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(b.snapshots))
	}
	ops.calls = nil

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(b.snapshots) != 0 {
		t.Fatalf("snapshots after Close = %d, want 0", len(b.snapshots))
	}
	persisted, err := loadTCSnapshots()
	if err != nil {
		t.Fatalf("load persisted snapshots after Close: %v", err)
	}
	if len(persisted) != 0 {
		t.Fatalf("persisted snapshots after Close = %v, want empty", persisted)
	}
	restored := 0
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "replace:fq") {
			restored++
		}
	}
	if restored != 2 {
		t.Fatalf("Close restored %d interfaces, want 2; calls=%v", restored, ops.calls)
	}
}

func TestCloseDropsStaleSnapshotsWhenInterfaceGone(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	b := testBackend(t, ops)

	if err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	delete(ops.links, "eth0")

	err := b.Close()
	if err == nil {
		t.Fatal("Close returned nil, want error for missing interface")
	}
	if len(b.snapshots) != 0 {
		t.Fatalf("snapshots after Close = %d, want 0 (stale should be dropped)", len(b.snapshots))
	}
	persisted, loadErr := loadTCSnapshots()
	if loadErr != nil {
		t.Fatalf("load persisted snapshots after Close: %v", loadErr)
	}
	if len(persisted) != 0 {
		t.Fatalf("persisted snapshots after Close = %v, want empty (stale should be dropped)", persisted)
	}
}

func TestApplyRejectsWhenSnapshotNotReady(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	readyErr := fmt.Errorf("cannot resolve config directory")
	b := newBackendWithOps(ops, readyErr, "", nil)

	err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")})
	if err == nil {
		t.Fatal("Apply returned nil, want snapshotReadyErr")
	}
	if !strings.Contains(err.Error(), "cannot resolve config directory") {
		t.Fatalf("Apply error = %v, want config directory error", err)
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "replace:") {
			t.Fatalf("Apply replaced qdisc despite snapshot not ready: calls=%v", ops.calls)
		}
	}
}

// VALIDATES: the persistence interlock -- with NO state store registered
// (filesystem-fallback), Apply refuses to snapshot-and-replace the root qdisc,
// because saveTCSnapshots would silently no-op and strand the operator's original
// qdisc unrestorable across a restart. It must bail BEFORE the destructive
// qdiscReplace.
// PREVENTS: regression to the migration bug where a host with no usable store had
// its root qdisc destroyed while Apply reported success and persisted nothing.
func TestApplyRefusesWithoutPersistence(t *testing.T) {
	statestore.SetStore(nil) // filesystem-fallback: saveTCSnapshots would no-op

	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	b := newBackendWithOps(ops, nil, "boot-1", nil)

	err := b.Apply(context.Background(), map[string]traffic.InterfaceQoS{"eth0": desiredHTB("eth0")})
	if err == nil {
		t.Fatal("Apply returned nil, want persistence-unavailable rejection")
	}
	if !errors.Is(err, errSnapshotPersistUnavailable) {
		t.Fatalf("Apply error = %v, want errSnapshotPersistUnavailable", err)
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "replace:") {
			t.Fatalf("Apply replaced qdisc without persistence available: calls=%v", ops.calls)
		}
	}
	if len(b.snapshots) != 0 {
		t.Fatalf("snapshots after refused Apply = %d, want 0", len(b.snapshots))
	}
}

// VALIDATES: a corrupt or version-mismatched blob under the tc-snapshot key is
// rejected by loadTCSnapshots, so backend startup surfaces the error instead of
// silently discarding restore state.
// PREVENTS: a garbled database.zefs blob being read as an empty snapshot set,
// which would strand the original qdisc unrestorable.
func TestLoadTCSnapshotsRejectsCorruptAndVersion(t *testing.T) {
	registerSnapshotStore(t)

	if _, err := statestore.Put(zefs.KeyTrafficTCSnapshot.Pattern, []byte(`{not json`)); err != nil {
		t.Fatalf("seed corrupt blob: %v", err)
	}
	if _, err := loadTCSnapshots(); err == nil {
		t.Error("expected error for corrupt tc snapshot blob")
	}

	if _, err := statestore.Put(zefs.KeyTrafficTCSnapshot.Pattern,
		[]byte(`{"version":999,"interfaces":{}}`)); err != nil {
		t.Fatalf("seed version-mismatch blob: %v", err)
	}
	if _, err := loadTCSnapshots(); err == nil {
		t.Error("expected error for unsupported tc snapshot version")
	}
}

// VALIDATES: an unregistered store yields an empty snapshot set without error,
// preserving best-effort restore semantics.
// PREVENTS: a missing database.zefs failing backend startup.
func TestLoadTCSnapshotsAbsentStore(t *testing.T) {
	statestore.SetStore(nil) // filesystem-fallback: no blob store registered

	// test-relax: the old "empty path" and "missing file path" sub-cases are gone
	// because loadTCSnapshots no longer takes a path; store availability is now a
	// single process-wide fact (statestore.Store() == nil), covered by this case.
	absent, err := loadTCSnapshots()
	if err != nil {
		t.Fatalf("absent store should not error: %v", err)
	}
	if len(absent) != 0 {
		t.Fatalf("absent store snapshots = %v, want empty", absent)
	}
}

// VALIDATES: saveTCSnapshots is a best-effort no-op when no store is registered,
// never returning an error that would fail Apply/Close.
// PREVENTS: snapshot persistence turning a missing store into a hard failure.
func TestSaveTCSnapshotsAbsentStoreIsNoOp(t *testing.T) {
	statestore.SetStore(nil) // filesystem-fallback: no blob store registered

	snap, err := newInterfaceSnapshot(testLink("eth0", 5), "boot-1", originalFQ(5))
	if err != nil {
		t.Fatalf("newInterfaceSnapshot: %v", err)
	}
	// test-relax: the old "empty path" and "missing file path" no-op sub-cases are
	// gone because saveTCSnapshots no longer takes a path; the single no-store case
	// covers best-effort no-op semantics.
	if err := saveTCSnapshots(map[string]tcInterfaceSnapshot{"eth0": snap}); err != nil {
		t.Errorf("save to absent store should be a no-op, got %v", err)
	}
}

func equalStringSlices(a, b []string) bool {
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
