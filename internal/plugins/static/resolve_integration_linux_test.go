//go:build integration && linux

// VALIDATES: a static route's nexthop interface is resolved through the shared
// iface resolver (iface.Resolve), not the static backend's own netlink handle,
// so a logical interface name maps to the right kernel device against the real
// netlink backend. The os-name / mac-match remapping itself is proven centrally
// in the iface package (TestResolveRemapsLogicalNameToOSDevice,
// TestResolveByMACBindsToDevice); this proves the static consumer is wired to it
// and returns the correct ifindex (and a clean error for an absent interface).
// PREVENTS: a regression to b.handle.LinkByName(name), which would fail for a
// logical name aliased via os-name/mac-match, or a "no backend loaded" surprise.

package static

import (
	"runtime"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register the netlink backend for iface.LoadBackend
)

func TestResolveNexthopIndexUsesResolver(t *testing.T) {
	withStaticNetNS(t, func() {
		if iface.GetBackend() == nil {
			if err := iface.LoadBackend("netlink"); err != nil {
				t.Fatalf("load iface backend: %v", err)
			}
		}
		const dev = "zestatic0"
		if err := iface.CreateDummy(dev); err != nil {
			t.Fatalf("create dummy %s: %v", dev, err)
		}
		t.Cleanup(func() { _ = iface.DeleteInterface(dev) })
		if err := iface.SetAdminUp(dev); err != nil {
			t.Fatalf("set %s up: %v", dev, err)
		}

		link, err := netlink.LinkByName(dev)
		if err != nil {
			t.Fatalf("lookup %s: %v", dev, err)
		}
		want := link.Attrs().Index

		idx, err := resolveNexthopIndex(dev)
		if err != nil {
			t.Fatalf("resolveNexthopIndex(%s): %v", dev, err)
		}
		if idx != want {
			t.Errorf("ifindex = %d, want real %d", idx, want)
		}

		if _, err := resolveNexthopIndex("zeghost0"); err == nil {
			t.Error("resolveNexthopIndex must error for an absent interface")
		}
	})
}

// withStaticNetNS runs fn in a fresh network namespace, skipping if the host
// lacks CAP_NET_ADMIN. Mirrors the per-package netns helper used by the isis /
// fib / traffic integration tests (there is no shared helper).
func withStaticNetNS(t *testing.T, fn func()) {
	t.Helper()
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}
	ns, err := netns.NewNamed("zestaticrt")
	if err != nil {
		orig.Close()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}
	t.Cleanup(func() {
		if rerr := netns.Set(orig); rerr != nil {
			t.Errorf("restore netns: %v", rerr)
		}
		orig.Close()
		ns.Close()
		netns.DeleteNamed("zestaticrt") //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})
	fn()
}
