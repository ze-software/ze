// Design: docs/features/interfaces.md -- original native VLAN resources
//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"maps"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

type nativeAccessRig struct {
	backend                         *netlinkBackend
	links                           map[string]netlink.Link
	adds, removes, modifies, closes int
	addErr                          error
	aliasTimeout                    string
	aliasFirst                      bool
	aliasTimedOut                   bool
	aliasNoWrite                    bool
	aliases                         map[string]int
	budget                          time.Duration
	readGate                        func()
	onAdd                           func(*netlink.Vlan)
}

func cloneNativeAccessLink(link netlink.Link) netlink.Link {
	switch link := link.(type) {
	case *netlink.Vlan:
		copy := *link
		copy.IngressQosMap = maps.Clone(link.IngressQosMap)
		copy.EgressQosMap = maps.Clone(link.EgressQosMap)
		return &copy
	case *netlink.Dummy:
		copy := *link
		return &copy
	default:
		panic("BUG: native access fixture received an undeclared link kind")
	}
}
func newNativeAccessRig(t *testing.T) *nativeAccessRig {
	t.Helper()
	rig := &nativeAccessRig{backend: &netlinkBackend{}, links: map[string]netlink.Link{"p0": &netlink.Dummy{Name: "p0", Index: 2}}, aliases: make(map[string]int)}
	ops := &nativeVLANOps{}
	ops.byName = func(name string) (netlink.Link, error) {
		link := rig.links[name]
		if link == nil {
			return nil, netlink.LinkNotFoundError{}
		}
		return cloneNativeAccessLink(link), nil
	}
	ops.byIndex = func(index int) (netlink.Link, error) {
		if rig.readGate != nil {
			rig.readGate()
		}
		for _, link := range rig.links {
			if link.Attrs().Index == index {
				return cloneNativeAccessLink(link), nil
			}
		}
		return nil, netlink.LinkNotFoundError{}
	}
	ops.add = func(link netlink.Link) (netlink.Link, error) {
		vlan, ok := link.(*netlink.Vlan)
		if !ok {
			t.Errorf("native VLAN create received %T", link)
			return nil, errors.New("native VLAN fixture create requires a VLAN")
		}
		if rig.onAdd != nil {
			rig.onAdd(vlan)
		}
		rig.adds++
		copy := cloneNativeAccessLink(link)
		copy.Attrs().Index = 100 + rig.adds
		copy.Attrs().Alias = ""
		rig.links[copy.Attrs().Name] = copy
		return cloneNativeAccessLink(copy), rig.addErr
	}
	ops.list = func() ([]netlink.Link, error) {
		var links []netlink.Link
		for _, link := range rig.links {
			links = append(links, cloneNativeAccessLink(link))
		}
		return links, nil
	}
	ops.up = func(netlink.Link) error { return nil }
	ops.modify = func(link netlink.Link) error {
		rig.modifies++
		wanted, ok := link.(*netlink.Vlan)
		if !ok {
			t.Errorf("native VLAN modify received %T", link)
			return errors.New("native VLAN fixture modify requires a VLAN")
		}
		actual, ok := rig.links[link.Attrs().Name].(*netlink.Vlan)
		if !ok {
			t.Errorf("native VLAN modify target is %T", rig.links[link.Attrs().Name])
			return errors.New("native VLAN fixture modify target requires a VLAN")
		}
		actual.IngressQosMap = maps.Clone(wanted.IngressQosMap)
		actual.EgressQosMap = maps.Clone(wanted.EgressQosMap)
		return nil
	}
	ops.remove = func(link netlink.Link) error {
		rig.removes++
		removed := map[int]bool{link.Attrs().Index: true}
		for range len(rig.links) {
			changed := false
			for _, candidate := range rig.links {
				if removed[candidate.Attrs().ParentIndex] {
					if !removed[candidate.Attrs().Index] {
						removed[candidate.Attrs().Index] = true
						changed = true
					}
				}
			}
			if !changed {
				break
			}
		}
		for name, candidate := range rig.links {
			if removed[candidate.Attrs().Index] {
				delete(rig.links, name)
			}
		}
		return nil
	}
	ops.alias = func(link netlink.Link, alias string) error {
		name := link.Attrs().Name
		if strings.HasPrefix(alias, iface.NativeVLANRetiredAliasPrefix) {
			rig.aliases[name]++
		}
		timeout := name == rig.aliasTimeout
		if rig.aliasFirst {
			if !rig.aliasTimedOut {
				timeout = true
				rig.aliasTimeout = name
			}
		}
		if timeout {
			rig.aliasTimedOut = true
			if !rig.aliasNoWrite {
				rig.links[name].Attrs().Alias = alias
			}
			// sleep(timeout-under-test): synctest consumes the fake socket's configured timeout to model an ACK-lost native operation.
			time.Sleep(rig.budget)
			return context.DeadlineExceeded
		}
		rig.links[name].Attrs().Alias = alias
		return nil
	}
	ops.timeout = func(budget time.Duration) error {
		if budget <= 0 {
			t.Fatalf("nonpositive native timeout: %s", budget)
		}
		if budget > nativeVLANOperationLimit {
			t.Fatalf("unbounded native timeout: %s", budget)
		}
		rig.budget = budget
		return nil
	}
	ops.close = func() error { rig.closes++; return nil }
	rig.backend.nativeVLANs.ops = ops
	t.Cleanup(func() {
		if err := rig.backend.Close(); err != nil {
			t.Errorf("native fixture cleanup: %v", err)
		}
	})
	return rig
}
func (rig *nativeAccessRig) create(t *testing.T, id int) iface.Binding {
	t.Helper()
	if err := rig.backend.CreateVLAN(iface.VLANSpec{Parent: "p0", VLANID: id, IngressQoSMap: map[uint32]uint32{3: 3}, EgressQoSMap: map[uint32]uint32{4: 4}}); err != nil {
		t.Fatal(err)
	}
	for name, link := range rig.links {
		vlan, ok := link.(*netlink.Vlan)
		if !ok {
			continue
		}
		if vlan.VlanId == id {
			return iface.Binding{OsName: name, Ifindex: vlan.Index, Alias: vlan.Alias, MTU: 1500, OperMAC: "02:00:00:00:00:01"}
		}
	}
	t.Fatal("created VLAN has no native readback")
	return iface.Binding{}
}
func (rig *nativeAccessRig) capture(t *testing.T, binding iface.Binding) iface.NativeVLANPreparation {
	t.Helper()
	lease, err := rig.backend.CaptureNativeVLAN(t.Context(), binding)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	return lease
}

// A held original MUST block deletion while unrelated unleased VLANs can leave.
// Use the real creator and delete path, then release and retry the original.
// MUTATION: dropping the held-resource deletion check permits native reuse.
func TestNativeAccessLeaseBlocksOnlyOriginalDeletion(t *testing.T) {
	rig := newNativeAccessRig(t)
	binding := rig.create(t, 101)
	lease := rig.capture(t, binding)
	if err := rig.backend.DeleteInterface(binding.OsName); !errors.Is(err, iface.ErrNativeAccessPending) {
		t.Fatalf("leased delete: %v", err)
	}
	if rig.removes != 0 {
		t.Fatal("leased access reached native delete")
	}
	second := rig.create(t, 102)
	if err := rig.backend.DeleteInterface(second.OsName); err != nil {
		t.Fatal(err)
	}
	if rig.removes != 1 {
		t.Fatal("unrelated unleased access was blocked")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rig.backend.DeleteInterface(binding.OsName); err != nil {
		t.Fatal(err)
	}
	if rig.removes != 2 {
		t.Fatal("original unleased access was not removed")
	}
}

// Deleting an owned child VLAN's parent MUST NOT bypass its retained lifetime.
// Exercise the same backend's ordinary parent delete while a real child is held.
// MUTATION: ignoring child custody lets native parent deletion destroy access.
func TestNativeAccessLeaseProtectsParentDeletion(t *testing.T) {
	rig := newNativeAccessRig(t)
	binding := rig.create(t, 114)
	rig.capture(t, binding)
	qosMapMu.Lock()
	handled, err := rig.backend.nativeVLANs.deleteLocked("p0")
	qosMapMu.Unlock()
	if !handled {
		t.Fatal("original parent escaped its native creation owner")
	}
	if !errors.Is(err, iface.ErrNativeAccessPending) {
		t.Fatalf("leased parent deletion: %v", err)
	}
	if rig.removes != 0 {
		t.Fatal("leased parent reached native deletion")
	}
}

// Every borrower MUST keep its creator's backend alive independently.
// Release the preparation, exercise its child writer and retry backend close.
// MUTATION: closing a backend with leases loses its original native transport.
func TestNativeAccessLeaseRetainsBackendUntilItsOwnRelease(t *testing.T) {
	rig := newNativeAccessRig(t)
	binding := rig.create(t, 103)
	lease := rig.capture(t, binding)
	if err := rig.backend.Close(); !errors.Is(err, iface.ErrNativeAccessPending) {
		t.Fatalf("leased backend close: %v", err)
	}
	if rig.closes != 0 {
		t.Fatal("creator transport closed while leased")
	}
	child, err := lease.Borrow(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := child.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := child.Replace(t.Context(), iface.NativeVLANMaps{Ingress: map[uint32]uint32{2: 6}}); err != nil {
		t.Fatal(err)
	}
	if err := rig.backend.Close(); !errors.Is(err, iface.ErrNativeAccessPending) {
		t.Fatalf("child backend close: %v", err)
	}
	if err := child.Close(); err != nil {
		t.Fatal(err)
	}
	if err := child.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rig.backend.Close(); err != nil {
		t.Fatal(err)
	}
	if rig.closes != 1 {
		t.Fatalf("creator transport closes: %d", rig.closes)
	}
	if _, err := lease.Identity(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed identity: %v", err)
	}
}

// Capture MUST bind the original birth as well as its name and native index.
// Present a distinct valid birth and a second backend against one real record.
// MUTATION: ignoring the alias in the initial binding accepts a different birth.
func TestNativeAccessCaptureRejectsForeignBirth(t *testing.T) {
	rig := newNativeAccessRig(t)
	binding := rig.create(t, 104)
	wrong := binding
	foreignBirth, err := iface.NewNativeVLANBirth()
	if err != nil {
		t.Fatal(err)
	}
	wrong.Alias, err = foreignBirth.Alias()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rig.backend.CaptureNativeVLAN(t.Context(), wrong); err == nil {
		t.Fatal("foreign binding birth captured")
	}
	foreign := &netlinkBackend{}
	if _, err := foreign.CaptureNativeVLAN(t.Context(), binding); !errors.Is(err, iface.ErrNativeAccessUnsupported) {
		t.Fatalf("foreign backend capture: %v", err)
	}
	lease := rig.capture(t, binding)
	got, err := lease.Identity()
	if err != nil {
		t.Fatal(err)
	}
	originalBirth, err := iface.ParseNativeVLANBirth(binding.Alias)
	if err != nil {
		t.Fatal(err)
	}
	if got.Birth != originalBirth {
		t.Fatalf("original birth changed: %+v", got)
	}
	if got.Index != binding.Ifindex {
		t.Fatalf("original index changed: %+v", got)
	}
}

// A replacement at the same native index MUST NOT receive an old map operation.
// Change only the returned birth, then exercise both read and write capabilities.
// MUTATION: accepting a same-index reply from another alias redirects the write.
func TestNativeAccessReadRejectsReusedNativeIndex(t *testing.T) {
	rig := newNativeAccessRig(t)
	binding := rig.create(t, 105)
	lease := rig.capture(t, binding)
	writer, err := lease.Borrow(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	})
	successor, ok := rig.links[binding.OsName].(*netlink.Vlan)
	if !ok {
		t.Fatalf("native VLAN successor is %T", rig.links[binding.OsName])
	}
	original := successor.Alias
	successor.Alias = nativeVLANAliasPrefix + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if successor.Alias == original {
		successor.Alias = nativeVLANAliasPrefix + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	}
	if _, err := writer.Snapshot(t.Context()); err == nil {
		t.Fatal("replacement supplied an old snapshot")
	}
	if err := writer.Replace(t.Context(), iface.NativeVLANMaps{Ingress: map[uint32]uint32{1: 7}}); err == nil {
		t.Fatal("replacement accepted an old write")
	}
	if rig.modifies != 0 {
		t.Fatal("wrong birth reached native modify")
	}
	successor.Alias = original
}

// A lost create ACK MUST retain intent before native mutation and prevent replay.
// Inspect the creator from its add callback, then require retained quarantine
// and refusal to replay or stamp after unmarked CREATE uncertainty.
// MUTATION: publishing the creation record after LinkAdd loses uncertain native ownership.
func TestNativeAccessRetainsUncertainCreationBeforeMutation(t *testing.T) {
	rig := newNativeAccessRig(t)
	cause := errors.New("create ACK was lost")
	rig.addErr = cause
	rig.onAdd = func(link *netlink.Vlan) {
		if rig.backend.nativeVLANs.byNameLocked(link.Name) == nil {
			t.Error("native create had no retained intent")
		}
	}
	spec := iface.VLANSpec{Parent: "p0", VLANID: 106}
	if err := rig.backend.CreateVLAN(spec); !errors.Is(err, cause) {
		t.Fatalf("create cause: %v", err)
	}
	if len(rig.backend.nativeVLANs.records) != 1 {
		t.Fatal("uncertain create lost its original receipt")
	}
	link := rig.links["p0.106"]
	if link.Attrs().Alias != "" {
		t.Fatal("fake minted the alias ignored by native CREATE")
	}
	binding := iface.Binding{OsName: "p0.106", Ifindex: link.Attrs().Index, Alias: link.Attrs().Alias}
	if _, err := rig.backend.CaptureNativeVLAN(t.Context(), binding); !errors.Is(err, iface.ErrNativeAccessUnsupported) {
		t.Fatalf("unmarked access granted mutation authority: %v", err)
	}
	rig.addErr = nil
	if err := rig.backend.CreateVLAN(spec); !errors.Is(err, cause) {
		t.Fatalf("retry lost original unconfirmed CREATE cause: %v", err)
	}
	if rig.adds != 1 {
		t.Fatal("uncertain create was resent")
	}
	if link.Attrs().Alias != "" {
		t.Fatal("unmarked uncertain CREATE was stamped by a later selector")
	}
	// Model an explicit external operator removal after the original worker
	// physically returned. This does not grant the old creator a later stamp.
	delete(rig.links, "p0.106")
	// The original receipt's exact echoed index is now absent, so Close can
	// confirm absence without a later mutation or a current-name adoption.
	if err := rig.backend.Close(); err != nil {
		t.Fatal(err)
	}
}

// A caller MUST NOT mutate native state through an owned map snapshot.
// Change a returned map and compare a fresh read through the same original owner.
func TestNativeAccessSnapshotsDoNotBorrowKernelMaps(t *testing.T) {
	rig := newNativeAccessRig(t)
	binding := rig.create(t, 107)
	preparation := rig.capture(t, binding)
	lease, err := preparation.Borrow(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	snapshot, err := lease.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Ingress[3] = 7
	again, err := lease.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if again.Ingress[3] != 3 {
		t.Fatal("consumer changed the original native snapshot")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := lease.Borrow(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled borrow cause: %v", err)
	}
}

// Close MUST return without joining native IO and a late read MUST stay retired.
// Withhold the original read under synctest, close its lease, then deliver it.
// MUTATION: returning the late snapshot after Close revives a retired lease.
func TestNativeAccessCloseDoesNotJoinWithheldRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig := newNativeAccessRig(t)
		binding := rig.create(t, 108)
		preparation := rig.capture(t, binding)
		lease, err := preparation.Borrow(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := lease.Close(); err != nil {
				t.Error(err)
			}
		})
		entered, resume := make(chan struct{}), make(chan struct{})
		rig.readGate = func() { close(entered); <-resume }
		done := make(chan error, 1)
		go func() { _, err := lease.Snapshot(t.Context()); done <- err }()
		<-entered
		if err := rig.backend.Close(); !errors.Is(err, iface.ErrNativeAccessPending) {
			t.Fatalf("close joined native read: %v", err)
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
		close(resume)
		if err := <-done; !errors.Is(err, os.ErrClosed) {
			t.Fatalf("retired read result: %v", err)
		}
		rig.readGate = nil
	})
}

// An ACK-lost retirement may already have changed the marker. Its next read
// must recognize only that exact marker, and a timeout cannot starve siblings.
// Lose one alias ACK under synctest, then retry both original creation records.
func TestNativeAccessRetirementRecoversLostACK(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig := newNativeAccessRig(t)
		first := rig.create(t, 109)
		second := rig.create(t, 110)
		rig.aliasFirst = true
		if err := rig.backend.Close(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("retirement cause: %v", err)
		}
		if rig.closes != 0 {
			t.Fatal("uncertain retirement discarded its owner")
		}
		synctest.Wait() // Join the fake native return before retrying its original worker.
		rig.aliasFirst = false
		rig.aliasTimeout = ""
		if err := rig.backend.Close(); err != nil {
			t.Fatal(err)
		}
		if rig.aliases[first.OsName] != 1 {
			t.Fatalf("first retirement replay/starvation: %+v", rig.aliases)
		}
		if rig.aliases[second.OsName] != 1 {
			t.Fatalf("second retirement replay/starvation: %+v", rig.aliases)
		}
		if rig.closes != 1 {
			t.Fatal("retired original handle did not close")
		}
	})
}

// A native resource that repeatedly consumes its deadline must not prevent
// another retained resource from confirming retirement on the next turn.
// Repeatedly withhold one alias ACK and observe the other original's attempt.
func TestNativeAccessPersistentTimeoutStillVisitsOtherResources(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig := newNativeAccessRig(t)
		first := rig.create(t, 112)
		second := rig.create(t, 113)
		rig.aliasFirst = true
		rig.aliasNoWrite = true
		if err := rig.backend.Close(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("first native timeout: %v", err)
		}
		synctest.Wait() // The bounded caller timed out; native completion is a separate event.
		blocked := rig.aliasTimeout
		sibling := first.OsName
		if sibling == blocked {
			sibling = second.OsName
		}
		if err := rig.backend.Close(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("persistent native timeout: %v", err)
		}
		if rig.aliases[sibling] != 1 {
			t.Fatalf("persistent native timeout starved its retained sibling: blocked=%s sibling=%s attempts=%v", blocked, sibling, rig.aliases)
		}
		if rig.closes != 0 {
			t.Fatal("backend closed before the blocked owner retired")
		}
		synctest.Wait() // Finish the second fake syscall before the cleanup retry.
		rig.aliasFirst = false
		rig.aliasTimeout = ""
		rig.aliasNoWrite = false
		if err := rig.backend.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

// Capacity exhaustion MUST refuse another creation before it reaches native IO.
// Fill the real bounded owner through its public creator, then count native adds.
func TestNativeAccessCreationCapacityRefusesBeforeMutation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig := newNativeAccessRig(t)
		rig.links["p1"] = &netlink.Dummy{Name: "p1", Index: 3}
		for i := range nativeVLANResourceLimit {
			parent, id := "p0", i+1
			if id > maxVLANID {
				parent, id = "p1", id-maxVLANID
			}
			if err := rig.backend.CreateVLAN(iface.VLANSpec{Parent: parent, VLANID: id}); err != nil {
				t.Fatal(err)
			}
		}
		before := rig.adds
		if err := rig.backend.CreateVLAN(iface.VLANSpec{Parent: "p1", VLANID: 3}); err == nil {
			t.Fatal("creation exceeded native receipt capacity")
		}
		if rig.adds != before {
			t.Fatal("exhausted creation capacity reached native mutation")
		}
	})
}

// Reference admission MUST stay bounded without losing original native custody.
// Borrow to the limit, reject one more child, and require backend close refusal.
func TestNativeAccessBorrowCapacityDoesNotWrapCustody(t *testing.T) {
	rig := newNativeAccessRig(t)
	binding := rig.create(t, 111)
	original := rig.capture(t, binding)
	held := make([]iface.NativeVLANLease, 0, nativeVLANLeaseLimit-1)
	t.Cleanup(func() {
		for _, lease := range held {
			if err := lease.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	for range nativeVLANLeaseLimit - 1 {
		lease, err := original.Borrow(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, lease)
	}
	extra, err := original.Borrow(t.Context())
	if extra != nil {
		if closeErr := extra.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}
	if err == nil {
		t.Fatal("native lease count exceeded its bound")
	}
	if err := rig.backend.Close(); !errors.Is(err, iface.ErrNativeAccessPending) {
		t.Fatalf("exhaustion discarded custody: %v", err)
	}
}
