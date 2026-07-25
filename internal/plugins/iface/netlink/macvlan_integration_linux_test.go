//go:build integration && linux

// Design: docs/features/interfaces.md -- real-kernel macvlan backend proof
// Overview: macvlan_linux.go -- CreateMacvlanDevice under test
//
// These run in the QEMU Alpine VM (ai/rules/qemu-testing.md): they create real
// macvlan devices via netlink in a throwaway network namespace and skip (never
// fail) without CAP_NET_ADMIN. Component-level owned-device lifecycle (registry
// -> reconcile -> orphan cleanup) is proven in the iface component's
// device_owner_integration_linux_test.go, which can drive the unexported
// reconcile pass; this file proves the backend building block.

package ifacenetlink

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
)

// withMacvlanNetNS runs fn inside a fresh named network namespace so real
// device creation cannot collide with host links. Skips (not fails) without
// CAP_NET_ADMIN per ai/rules/qemu-testing.md.
func withMacvlanNetNS(t *testing.T, fn func()) {
	t.Helper()
	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}
	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}
	nsName := macvlanNetNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}
	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		unlock()
	})
	fn()
}

func macvlanNetNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zemv_" + name
}

func addMacvlanParent(t *testing.T, name string, mtu int) {
	t.Helper()
	dummy := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(dummy); err != nil {
		t.Fatalf("add dummy parent %q: %v", name, err)
	}
	if err := netlink.LinkSetMTU(dummy, mtu); err != nil {
		t.Fatalf("set parent mtu: %v", err)
	}
	if err := netlink.LinkSetUp(dummy); err != nil {
		t.Fatalf("set parent up: %v", err)
	}
}

// TestIntegrationMacvlanCreate_ReadBackMACModeAliasMTU proves CreateMacvlanDevice
// lands a bridge-mode macvlan with the requested MAC, the ownership alias, and
// the parent's MTU, admin-up -- read back through netlink.
//
// VALIDATES: AC-1 + A-2 + A-3 against a real kernel.
func TestIntegrationMacvlanCreate_ReadBackMACModeAliasMTU(t *testing.T) {
	withMacvlanNetNS(t, func() {
		addMacvlanParent(t, "zvp0", 1400)
		b := &netlinkBackend{}
		spec := iface.MacvlanSpec{Name: "zvm0", Parent: "zvp0", MAC: "00:00:5e:00:01:0a", Alias: "ze:owned:test"}
		if err := b.CreateMacvlanDevice(spec); err != nil {
			t.Fatalf("CreateMacvlanDevice: %v", err)
		}
		link, err := netlink.LinkByName("zvm0")
		if err != nil {
			t.Fatalf("read back macvlan: %v", err)
		}
		mv, ok := link.(*netlink.Macvlan)
		if !ok {
			t.Fatalf("link type = %q, want macvlan", link.Type())
		}
		if mv.Mode != netlink.MACVLAN_MODE_BRIDGE {
			t.Errorf("mode = %v, want bridge", mv.Mode)
		}
		attrs := mv.Attrs()
		if attrs.HardwareAddr.String() != "00:00:5e:00:01:0a" {
			t.Errorf("mac = %q, want 00:00:5e:00:01:0a", attrs.HardwareAddr.String())
		}
		if attrs.Alias != "ze:owned:test" {
			t.Errorf("alias = %q, want ze:owned:test", attrs.Alias)
		}
		if attrs.MTU != 1400 {
			t.Errorf("mtu = %d, want 1400 (parent MTU)", attrs.MTU)
		}
		if attrs.Flags&1 == 0 { // net.FlagUp
			t.Errorf("macvlan not admin-up (flags=%x)", attrs.Flags)
		}
		// ListInterfaces (used by the orphan scan) surfaces the alias.
		infos, err := b.ListInterfaces()
		if err != nil {
			t.Fatalf("ListInterfaces: %v", err)
		}
		var seen bool
		for i := range infos {
			if infos[i].Name == "zvm0" {
				seen = true
				if infos[i].Alias != "ze:owned:test" {
					t.Errorf("ListInterfaces alias = %q, want ze:owned:test", infos[i].Alias)
				}
				if infos[i].Type != "macvlan" {
					t.Errorf("ListInterfaces type = %q, want macvlan", infos[i].Type)
				}
			}
		}
		if !seen {
			t.Error("macvlan not present in ListInterfaces")
		}
	})
}

// TestIntegrationMacvlanParentDown_DeviceSurvivesAndKeepsOperUp pins the
// kernel's ACTUAL parent-down contract for a macvlan, which is the opposite of
// what spec-vrrp-3 A-4 assumed.
//
// Measured in the QEMU VM (2026-07-15): after `ip link set <parent> down`, the
// macvlan keeps `state UP` and its LOWER_UP flag; the kernel only adds an
// M-DOWN flag to the macvlan's flag word. It does NOT drive the macvlan's
// operstate to LOWERLAYERDOWN, immediately or after 5s of linkwatch ticks. So
// A-4 ("parent-down propagates to macvlan oper-state") is BROKEN, and any
// readiness predicate keyed on the macvlan's own oper-state would never notice
// a dead parent.
//
// Consequence, enforced by spec-vrrp-5's engine: VRRP readiness keys on the
// PARENT's state (which the iface component already tracks and delivers via
// iface.Subscribe), never on the macvlan's oper-state. The macvlan contributes
// only existence.
//
// This test therefore asserts the two guarantees the kernel really makes: the
// device survives parent-down (so recovery needs no recreate), and its
// oper-state is NOT a usable liveness signal.
//
// VALIDATES: AC-5; pins A-4's corrected (broken-as-written) finding so a future
// kernel change that starts propagating is caught rather than silently relied on.
func TestIntegrationMacvlanParentDown_DeviceSurvivesAndKeepsOperUp(t *testing.T) {
	withMacvlanNetNS(t, func() {
		addMacvlanParent(t, "zvp0", 1500)
		b := &netlinkBackend{}
		spec := iface.MacvlanSpec{Name: "zvm0", Parent: "zvp0", MAC: "00:00:5e:00:01:0a", Alias: "ze:owned:test"}
		if err := b.CreateMacvlanDevice(spec); err != nil {
			t.Fatalf("CreateMacvlanDevice: %v", err)
		}
		parent, err := netlink.LinkByName("zvp0")
		if err != nil {
			t.Fatalf("get parent: %v", err)
		}
		if err := netlink.LinkSetDown(parent); err != nil {
			t.Fatalf("set parent down: %v", err)
		}
		// test-relax: the removed assertion (macvlan oper-state goes not-up on
		// parent-down) asserted a kernel behaviour that does not exist. Measured
		// in the QEMU VM 2026-07-15 with `ip -d link show mv0`: after
		// `ip link set p0 down` the macvlan reads
		// `<BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> ... state UP` -- the kernel
		// adds an M-DOWN flag but leaves oper-state UP and LOWER_UP set, both
		// immediately and after seconds of linkwatch ticks. The assertion was
		// therefore unsatisfiable, not red-because-the-code-is-wrong: no ze code
		// can make the kernel report LOWERLAYERDOWN here. Coverage is REPLACED,
		// not dropped: the inverted assertion below pins the real contract (so a
		// future kernel that starts propagating fails this test loudly), and the
		// behaviour the old assertion was proxying for -- VRRP noticing a dead
		// parent -- is covered where it actually lives, by spec-vrrp-5's
		// parent-keyed readiness predicate and its engine tests.
		//
		// Give linkwatch several ticks first, so "no propagation" is a measured
		// outcome rather than a race we won.
		time.Sleep(2 * time.Second)

		link, err := netlink.LinkByName("zvm0")
		if err != nil {
			// Guarantee 1: the device survives parent-down, so recovery when
			// the parent returns needs no recreate.
			t.Fatalf("macvlan was deleted on parent down: %v", err)
		}

		// Guarantee 2 (the corrected A-4): the kernel does NOT drive the
		// macvlan's oper-state from its parent.
		// netlink.OperUp is an untyped constant; convert so %s uses
		// LinkOperState.String() instead of printing a bare int.
		wantState := netlink.LinkOperState(netlink.OperUp)
		if got := link.Attrs().OperState; got != wantState {
			t.Fatalf("macvlan oper-state = %s after parent down, want %s: the kernel now propagates lower-layer state, so spec-vrrp-5's parent-keyed readiness predicate can be revisited",
				got, wantState)
		}
		t.Logf("kernel contract confirmed: macvlan oper-state stays %s after parent down (readiness must key on the parent)", link.Attrs().OperState)
	})
}
