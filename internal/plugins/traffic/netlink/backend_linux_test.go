// Design: plan/deployment-readiness-deep-review.md -- tc original-qdisc restore regressions

//go:build linux

package trafficnetlink

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
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

func testBackend(t *testing.T, ops *fakeTCOps) (*backend, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state", "traffic-tc-snapshots.json")
	return newBackendWithOps(ops, path, nil, "boot-1", nil), path
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

func TestApplySnapshotsOriginalBeforeReplace(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	b, path := testBackend(t, ops)

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
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("snapshot file not written: %v", err)
	}
	if !strings.Contains(string(data), `"type": "fq"`) {
		t.Fatalf("snapshot file = %s, want fq qdisc", data)
	}
}

func TestRestoreOriginalUsesSnapshotNotFQCodelDefault(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{originalFQ(5)}
	b, path := testBackend(t, ops)

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
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("snapshot file still exists after restore: %v", err)
	}
}

func TestApplyRejectsUnrestorableOriginalBeforeReplace(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	ops.qdiscs["eth0"] = []netlink.Qdisc{
		&netlink.GenericQdisc{QdiscAttrs: rootAttrs(5), QdiscType: "cake"},
	}
	b, _ := testBackend(t, ops)

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
	b, _ := testBackend(t, ops)

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
	path := filepath.Join(t.TempDir(), "state", "traffic-tc-snapshots.json")
	snap, err := newInterfaceSnapshot(ops.links["eth0"], "boot-1", originalFQ(5))
	if err != nil {
		t.Fatalf("newInterfaceSnapshot: %v", err)
	}
	if err := saveTCSnapshots(path, map[string]tcInterfaceSnapshot{"eth0": snap}); err != nil {
		t.Fatalf("saveTCSnapshots: %v", err)
	}
	loaded, err := loadTCSnapshots(path)
	if err != nil {
		t.Fatalf("loadTCSnapshots: %v", err)
	}
	b := newBackendWithOps(ops, path, nil, "boot-1", loaded)

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
	b := newBackendWithOps(ops, filepath.Join(t.TempDir(), "snapshots.json"), nil, "boot-1", map[string]tcInterfaceSnapshot{"eth0": snap})

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
	b, _ := testBackend(t, ops)

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
	b, path := testBackend(t, ops)

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
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("snapshot file still exists after Close: %v", err)
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
	b, path := testBackend(t, ops)

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
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("snapshot file still exists after Close with missing interface: %v", statErr)
	}
}

func TestApplyRejectsWhenSnapshotNotReady(t *testing.T) {
	ops := newFakeTCOps()
	ops.links["eth0"] = testLink("eth0", 5)
	readyErr := fmt.Errorf("cannot resolve config directory")
	b := newBackendWithOps(ops, "", readyErr, "", nil)

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
