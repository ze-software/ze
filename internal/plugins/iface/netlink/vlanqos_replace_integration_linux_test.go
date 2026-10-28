//go:build linux

package ifacenetlink

import (
	"maps"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

// installedEgress reads the VLAN's egress map back from the kernel, dropping
// the PCP 0 entries the kernel keeps for a removed key (it does not dump
// them, so this is a guard should a kernel start to).
func installedEgress(t *testing.T, name string) map[uint32]uint32 {
	t.Helper()
	got := map[uint32]uint32{}
	for priority, pcp := range kernelVLAN(t, name).EgressQosMap {
		if pcp != 0 {
			got[priority] = pcp
		}
	}
	return got
}

// installedIngress reads the VLAN's ingress map back, dropping slots at the
// kernel default priority 0.
func installedIngress(t *testing.T, name string) map[uint32]uint32 {
	t.Helper()
	got := map[uint32]uint32{}
	for pcp, priority := range kernelVLAN(t, name).IngressQosMap {
		if priority != 0 {
			got[pcp] = priority
		}
	}
	return got
}

// TestNetlinkUpdateVLANQoSMapReplacesKernel proves AC-18b against the kernel:
// after UpdateVLANQoSMap the link reads back exactly the given maps, an
// omitted key and a nil map leave no mapping, and the minimal modify changes
// no other attribute (MTU, flags, txqlen, address, VLAN flags).
func TestNetlinkUpdateVLANQoSMapReplacesKernel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "zer0")
		b := &netlinkBackend{}
		if err := b.CreateVLAN(iface.VLANSpec{
			Parent:        "zer0",
			VLANID:        310,
			IngressQoSMap: map[uint32]uint32{3: 3, 6: 6},
			EgressQoSMap:  map[uint32]uint32{3: 3, 5: 5},
		}); err != nil {
			t.Fatalf("CreateVLAN: %v", err)
		}
		before := kernelVLAN(t, "zer0.310")

		if err := b.UpdateVLANQoSMap("zer0.310", map[uint32]uint32{6: 6}, map[uint32]uint32{3: 3}); err != nil {
			t.Fatalf("UpdateVLANQoSMap: %v", err)
		}
		if got := installedIngress(t, "zer0.310"); !maps.Equal(got, map[uint32]uint32{6: 6}) {
			t.Errorf("ingress after a slot was omitted: %v, want map[6:6]", got)
		}
		if got := installedEgress(t, "zer0.310"); !maps.Equal(got, map[uint32]uint32{3: 3}) {
			t.Errorf("egress after a key was omitted: %v, want map[3:3]", got)
		}

		if err := b.UpdateVLANQoSMap("zer0.310", nil, nil); err != nil {
			t.Fatalf("UpdateVLANQoSMap nil: %v", err)
		}
		if got := installedIngress(t, "zer0.310"); len(got) != 0 {
			t.Errorf("ingress after nil: %v, want no mapping", got)
		}
		if got := installedEgress(t, "zer0.310"); len(got) != 0 {
			t.Errorf("egress after nil: %v, want no mapping", got)
		}

		after := kernelVLAN(t, "zer0.310")
		if after.Attrs().MTU != before.Attrs().MTU || after.Attrs().Flags != before.Attrs().Flags ||
			after.Attrs().TxQLen != before.Attrs().TxQLen || after.Attrs().HardwareAddr.String() != before.Attrs().HardwareAddr.String() {
			t.Errorf("non-QoS attributes changed: before %+v, after %+v", before.Attrs(), after.Attrs())
		}
		if !sameVLANFlags(before, after) {
			t.Errorf("VLAN flags changed by the minimal modify")
		}
		t.Logf("kernel egress dump after nil: %v", kernelVLAN(t, "zer0.310").EgressQosMap)
	})
}

// sameVLANFlags compares the VLAN flag pointers the library parses.
func sameVLANFlags(a, b *netlink.Vlan) bool {
	eq := func(x, y *bool) bool { return (x == nil && y == nil) || (x != nil && y != nil && *x == *y) }
	return eq(a.ReorderHdr, b.ReorderHdr) && eq(a.Gvrp, b.Gvrp) && eq(a.Mvrp, b.Mvrp) &&
		eq(a.LooseBinding, b.LooseBinding) && eq(a.BridgeBinding, b.BridgeBinding)
}

// TestNetlinkUpdateVLANQoSMapSerialised proves R-17: two overlapping calls on
// one VLAN never compute their replace from one snapshot. Installed egress
// {3:3}; call A wants {3:3, 5:5}, call B wants {3:3}. The hook holds each call
// between its read and its write. With qosMapMu, B cannot reach the hook while
// A is held, so the bounded window expires, A is released, and B then reads
// A's result and zeroes key 5. Without the mutex B arrives inside the window
// with the pre-A snapshot and, released last, leaves key 5 installed.
func TestNetlinkUpdateVLANQoSMapSerialised(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "zes0")
		b := &netlinkBackend{}
		if err := b.CreateVLAN(iface.VLANSpec{Parent: "zes0", VLANID: 320, EgressQoSMap: map[uint32]uint32{3: 3}}); err != nil {
			t.Fatalf("CreateVLAN: %v", err)
		}

		arrived := make(chan struct{}, 2)
		release := make(chan struct{}, 2)
		afterQoSMapReadForTest = func() {
			arrived <- struct{}{}
			<-release
		}
		t.Cleanup(func() { afterQoSMapReadForTest = nil })

		// A namespace belongs to an OS thread, not to the process, so a new
		// goroutine starts outside the test namespace and finds no zes0.320.
		// Each call therefore locks its thread and enters the namespace. The
		// thread is never unlocked: Go ends a thread whose goroutine exits
		// locked, so the namespace change never reaches another goroutine.
		testNS, err := netns.Get()
		if err != nil {
			t.Fatalf("get the test namespace: %v", err)
		}
		t.Cleanup(func() { testNS.Close() }) //nolint:errcheck // best-effort cleanup
		updateInTestNS := func(egress map[uint32]uint32, done chan<- error) {
			runtime.LockOSThread()
			if err := netns.Set(testNS); err != nil {
				done <- err
				return
			}
			done <- b.UpdateVLANQoSMap("zes0.320", nil, egress)
		}

		doneA := make(chan error, 1)
		doneB := make(chan error, 1)
		go updateInTestNS(map[uint32]uint32{3: 3, 5: 5}, doneA)
		select {
		case <-arrived:
		case err := <-doneA:
			t.Fatalf("call A returned before reaching the hook: %v", err)
		}
		go updateInTestNS(map[uint32]uint32{3: 3}, doneB)

		bArrivedWhileAHeld := false
		select {
		case <-arrived:
			bArrivedWhileAHeld = true
		case <-time.After(500 * time.Millisecond):
		}
		release <- struct{}{}
		if err := <-doneA; err != nil {
			t.Fatalf("call A: %v", err)
		}
		if !bArrivedWhileAHeld {
			select {
			case <-arrived:
			case err := <-doneB:
				t.Fatalf("call B returned before reaching the hook: %v", err)
			}
		}
		release <- struct{}{}
		if err := <-doneB; err != nil {
			t.Fatalf("call B: %v", err)
		}

		if bArrivedWhileAHeld {
			t.Errorf("call B read the installed maps while call A was between its read and its write")
		}
		if got := installedEgress(t, "zes0.320"); !maps.Equal(got, map[uint32]uint32{3: 3}) {
			t.Errorf("egress after B returned last: %v, want exactly map[3:3] (key 5 must not survive)", got)
		}
	})
}
