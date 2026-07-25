//go:build integration && linux

// VALIDATES: waitForInterface resolves a logical interface name through the
// shared iface resolver and returns the real *net.Interface the multicast
// discovery socket needs, against the real netlink backend. Replaces the host
// TestWaitForInterfaceFound, which could no longer run after the migration
// (iface resolution needs the Linux-only netlink backend). The os-name /
// mac-match remapping itself is proven centrally in the iface package.
// PREVENTS: a regression to net.InterfaceByName(name), which would fail for a
// logical name aliased via os-name/mac-match, and a hang when no backend loaded.

package ldp

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/slogutil"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // register the netlink backend for iface.LoadBackend
)

func TestWaitForInterfaceFoundResolves(t *testing.T) {
	withLDPNetNS(t, func() {
		if iface.GetBackend() == nil {
			if err := iface.LoadBackend("netlink"); err != nil {
				t.Fatalf("load iface backend: %v", err)
			}
		}
		const dev = "zeldp0"
		if err := iface.CreateDummy(dev); err != nil {
			t.Fatalf("create dummy %s: %v", dev, err)
		}
		t.Cleanup(func() { _ = iface.DeleteInterface(dev) })
		if err := iface.SetAdminUp(dev); err != nil {
			t.Fatalf("set %s up: %v", dev, err)
		}

		ifi := waitForInterface(context.Background(), slogutil.DiscardLogger(), dev, time.Second)
		if ifi == nil {
			t.Fatalf("waitForInterface(%q) returned nil for an existing interface", dev)
		}
		if ifi.Name != dev {
			t.Errorf("got interface %q, want %q (resolved through iface.Resolve)", ifi.Name, dev)
		}
	})
}

func withLDPNetNS(t *testing.T, fn func()) {
	t.Helper()
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}
	ns, err := netns.NewNamed("zeldpdisc")
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
		netns.DeleteNamed("zeldpdisc") //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})
	fn()
}
