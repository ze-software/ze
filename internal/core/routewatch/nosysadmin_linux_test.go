// Design: docs/architecture/core-design.md -- shared route watcher without CAP_SYS_ADMIN
// Related: routewatch_linux.go -- openSubscription, namespaceToEnter, ListRoutes

//go:build linux

package routewatch

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// VALIDATES: a daemon granted CAP_NET_ADMIN but not CAP_SYS_ADMIN, as the
// interop lab's Ze containers are, subscribes to route changes and lists
// routes in the network namespace it runs in. The sequence is the one a PPPoE
// CE meets: the subscription opens first, the WAN device appears afterwards,
// and the IPv6 default route Router Discovery learns through it (::/0 via
// fe80::1, protocol ra, metric 1024) reaches the consumer with the device's
// index, both as an event and from ListRoutes.
// METHOD: in the test's own user and network namespace, each watcher call
// runs on a locked OS thread that dropped CAP_SYS_ADMIN, after proving that
// thread is refused a setns into its own namespace. The device and the route
// are added from another thread, which keeps every capability.
// PREVENTS: the watcher entering the namespace it already runs in, which the
// kernel refuses without CAP_SYS_ADMIN. Neither the subscription nor the
// listing then opened, no route reached the iface-ra sender, and a CE unit
// advertised Router Lifetime 0 on its LAN while its WAN held a default route
// (pppoe-dhcpv6-pd-ze-client).
func TestIntegration_SubscribesAndListsWithoutSysAdmin(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	w := New()
	rec := &eventRecorder{}
	w.Register(rec.record)
	w.captureNamespace()
	require.True(t, w.platform.ns.IsOpen(), "the watcher did not pin the test's network namespace")
	t.Cleanup(func() { _ = w.platform.ns.Close() })

	subscribed := onThreadWithoutSysAdmin(w.subscribeOnce)

	// The subscription is open once its listing delivered the loopback routes.
	opened := rec.waitFor(t, func(evs []RouteEvent) bool { return len(evs) > 0 })
	if len(opened) == 0 {
		w.Stop()
		t.Fatalf("no route event reached the watcher without CAP_SYS_ADMIN; the subscription answered: %v", <-subscribed)
	}

	h, err := netlink.NewHandle()
	require.NoError(t, err)
	defer h.Close()

	// The WAN device appears only now, after the subscription opened, as a
	// PPP session device appears after the RA sender started.
	require.NoError(t, h.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "wan0"}}))
	wan, err := h.LinkByName("wan0")
	require.NoError(t, err)
	require.NoError(t, h.LinkSetUp(wan))
	require.NoError(t, h.RouteAdd(&netlink.Route{
		LinkIndex: wan.Attrs().Index,
		Dst:       &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 8*net.IPv6len)},
		Gw:        net.ParseIP("fe80::1"),
		Protocol:  unix.RTPROT_RA,
		Priority:  1024,
		Table:     unix.RT_TABLE_MAIN,
	}))

	want := RouteEvent{
		Prefix:     netip.MustParsePrefix("::/0"),
		NextHop:    netip.MustParseAddr("fe80::1"),
		Protocol:   unix.RTPROT_RA,
		Metric:     1024,
		TableID:    unix.RT_TABLE_MAIN,
		OutIfindex: wan.Attrs().Index,
		Action:     ActionAdd,
	}
	events := rec.waitFor(t, func(evs []RouteEvent) bool { return slices.Contains(evs, want) })
	require.Contains(t, events, want, "the RA-learned default route through wan0 never reached the consumer")

	listed := <-onThreadWithoutSysAdmin(func() error {
		routes, err := w.ListRoutes(netip.MustParsePrefix("::/0"))
		if err != nil {
			return err
		}
		if !slices.Contains(routes, want) {
			return fmt.Errorf("ListRoutes answered %v, want %v among them", routes, want)
		}
		return nil
	})
	require.NoError(t, listed, "ListRoutes without CAP_SYS_ADMIN")

	w.Stop()
	require.NoError(t, <-subscribed, "the subscription ended with an error")
}

// onThreadWithoutSysAdmin runs fn on a locked OS thread that holds every
// capability of the test's namespace except CAP_SYS_ADMIN, and delivers fn's
// error once fn returns. The thread is never unlocked: it ends with its
// goroutine, so its reduced capability set never serves another goroutine.
func onThreadWithoutSysAdmin(fn func() error) <-chan error {
	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		if err := dropSysAdmin(); err != nil {
			result <- err
			return
		}
		result <- fn()
	}()
	return result
}

// dropSysAdmin removes CAP_SYS_ADMIN from the calling thread, then proves the
// condition the test needs: the thread is refused a setns into the network
// namespace it already runs in. Without that proof the test could pass on a
// host where the drop did nothing. The caller MUST hold its OS thread locked.
func dropSysAdmin() error {
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&header, &data[0]); err != nil {
		return fmt.Errorf("read the thread's capabilities: %w", err)
	}
	// CAP_SYS_ADMIN is 21, so its bit lives in the first 32-bit word.
	bit := uint32(1) << unix.CAP_SYS_ADMIN
	data[0].Effective &^= bit
	data[0].Permitted &^= bit
	data[0].Inheritable &^= bit
	if err := unix.Capset(&header, &data[0]); err != nil {
		return fmt.Errorf("drop CAP_SYS_ADMIN from the thread: %w", err)
	}

	self, err := netns.Get()
	if err != nil {
		return fmt.Errorf("read the thread's network namespace: %w", err)
	}
	defer func() { _ = self.Close() }()
	if err := netns.Set(self); !errors.Is(err, unix.EPERM) {
		return fmt.Errorf("setns into the thread's own namespace answered %v without CAP_SYS_ADMIN, want EPERM: the test cannot reproduce a daemon without CAP_SYS_ADMIN", err)
	}
	return nil
}
