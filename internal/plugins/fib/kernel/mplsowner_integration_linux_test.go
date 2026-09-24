//go:build integration && linux

// Design: docs/architecture/mpls/mpls-kernel.md -- native label ownership and next-hop devices.
package fibkernel

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

func requireOwnedSwapFrame(t *testing.T, bed *mplsTestbed, labels []uint32, nextHop net.HardwareAddr) {
	t.Helper()
	inner := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"), 4000, []byte("label-owner"))
	bed.inject([]uint32{100}, inner)
	out := bed.awaitForwarded("source-owned MPLS frame", isMPLS)
	if !slices.Equal(labelStack(out), labels) || !slices.Equal(out[:6], []byte(nextHop)) {
		t.Fatalf("forwarded labels %v toward %s, want %v toward %s", labelStack(out), net.HardwareAddr(out[:6]), labels, nextHop)
	}
}

// TestMPLSIntegration_LabelSourceOwnership rejects another producer's update and
// withdrawal, while a same-source RSVP repair changes the observed label stack.
func TestMPLSIntegration_LabelSourceOwnership(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)
		bed.enableInput()
		bus := newMPLSApplyBus(t, newFIBKernel(newTestBackend(h)))
		entry := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
			InLabel: 100, OutLabels: []uint32{200}, NextHop: mplsNextHop, Source: 1}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{200}, mplsNextMAC)
		foreign := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPop,
			InLabel: 100, NextHop: mplsBypassHop, Source: 5}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{foreign}); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("cross-source install error = %v, want EEXIST", err)
		}
		foreign.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{foreign}); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("cross-source removal error = %v, want EEXIST", err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{200}, mplsNextMAC)
		entry.OutLabels, entry.NextHop = []uint32{5000, 200}, mplsBypassHop
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{5000, 200}, mplsBypassMAC)
		entry.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		foreign.Action = mplsfibevents.ActionAdd
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{foreign}); err != nil {
			t.Fatal(err)
		}
		inner := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"), 4000, []byte("new-owner"))
		bed.inject([]uint32{100}, inner)
		out := bed.awaitForwarded("new owner's popped IPv4 frame", isIPv4)
		if !slices.Equal(out[:6], []byte(mplsBypassMAC)) {
			t.Fatalf("new owner forwarded toward %s, want %s", net.HardwareAddr(out[:6]), mplsBypassMAC)
		}
	})
}

// TestMPLSIntegration_ForeignLabelPreserved checks the real kernel's EEXIST
// refusal on first install and preserves foreign state during compensating remove.
func TestMPLSIntegration_ForeignLabelPreserved(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)
		bed.enableInput()
		link, err := h.LinkByName(mplsZeLink)
		if err != nil {
			t.Fatal(err)
		}
		label := 100
		foreign := &netlink.Route{Family: unix.AF_MPLS, MPLSDst: &label, Protocol: 100,
			LinkIndex: link.Attrs().Index, NewDst: &netlink.MPLSDestination{Labels: []int{900}},
			Via: &netlink.Via{AddrFamily: unix.AF_INET, Addr: mplsNextHop.AsSlice()}}
		if err := h.RouteAdd(foreign); err != nil {
			t.Fatal(err)
		}
		bus := newMPLSApplyBus(t, newFIBKernel(newTestBackend(h)))
		entry := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
			InLabel: 100, OutLabels: []uint32{200}, NextHop: mplsBypassHop, Source: 1}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("foreign-label install error = %v, want EEXIST", err)
		}
		entry.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{900}, mplsNextMAC)
		if err := h.RouteDel(foreign); err != nil {
			t.Fatal(err)
		}
		entry.Action = mplsfibevents.ActionAdd
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatal(err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{200}, mplsBypassMAC)
		if err := h.RouteReplace(foreign); err != nil {
			t.Fatal(err)
		}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("externally replaced label update error = %v, want EEXIST", err)
		}
		entry.Action = mplsfibevents.ActionRemove
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("externally replaced label removal error = %v, want EEXIST", err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{900}, mplsNextMAC)
	})
}

// TestMPLSIntegration_IPv6LabelNextHopZone installs the same link-local next-hop
// address on two devices. Named and numeric zones must choose the requested one.
func TestMPLSIntegration_IPv6LabelNextHopZone(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		var links [2]netlink.Link
		for i, name := range []string{"ze-zone-a", "ze-zone-b"} {
			attrs := netlink.NewLinkAttrs()
			attrs.Name = name
			if err := h.LinkAdd(&netlink.Dummy{LinkAttrs: attrs}); err != nil {
				t.Fatal(err)
			}
			link, err := h.LinkByName(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := h.LinkSetUp(link); err != nil {
				t.Fatal(err)
			}
			address := &netlink.Addr{IPNet: &net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
				Flags: unix.IFA_F_NODAD, Scope: unix.RT_SCOPE_LINK}
			if err := h.AddrAdd(link, address); err != nil {
				t.Fatal(err)
			}
			links[i] = link
		}
		bus := newMPLSApplyBus(t, newFIBKernel(newTestBackend(h)))
		entry := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPop,
			InLabel: 100, Source: 5}
		for _, zone := range []string{"ze-zone-a", "ze-zone-b", strconv.Itoa(links[0].Attrs().Index)} {
			entry.NextHop = netip.MustParseAddr("fe80::2").WithZone(zone)
			if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
				t.Fatalf("zone %s: %v", zone, err)
			}
			route, ok := mplsRoutes(t, h)[100]
			if !ok {
				t.Fatal("IPv6 peer label is absent")
			}
			want := links[0].Attrs().Index
			if zone == "ze-zone-b" {
				want = links[1].Attrs().Index
			}
			via, ok := route.Via.(*netlink.Via)
			if !ok || via.AddrFamily != unix.AF_INET6 || !via.Addr.Equal(net.ParseIP("fe80::2")) || route.LinkIndex != want {
				t.Fatalf("zone %s programmed device %d via %v, want device %d via fe80::2", zone, route.LinkIndex, route.Via, want)
			}
		}
		entry.NextHop = netip.MustParseAddr("fe80::2")
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err == nil {
			t.Fatal("ambiguous link-local next hop without a zone was accepted")
		}
		if route := mplsRoutes(t, h)[100]; route.LinkIndex != links[0].Attrs().Index {
			t.Fatal("failed zone-free update changed the installed device")
		}
	})
}

// TestMPLSIntegration_OwnerReadiness sends the first label operation as soon as
// run reports ready, then verifies shutdown removes the acknowledgment boundary.
func TestMPLSIntegration_OwnerReadiness(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)
		bus := newMPLSApplyBus(t, nil)
		previousBus := eventBusPtr.Load()
		setEventBus(bus)
		defer eventBusPtr.Store(previousBus)
		f := newFIBKernel(newTestBackend(h))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		ready := make(chan error, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			f.run(ctx, false, ready)
		}()
		defer func() {
			cancel()
			<-done
		}()
		select {
		case err := <-ready:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("forwarding owner never reported subscription readiness")
		}
		entry := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
			InLabel: 100, OutLabels: []uint32{200}, NextHop: mplsNextHop, Source: 1}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
			t.Fatalf("first operation after readiness was refused: %v", err)
		}
		if _, installed := mplsRoutes(t, h)[100]; !installed {
			t.Fatal("first acknowledged label is absent from the kernel")
		}
		cancel()
		<-done
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err == nil {
			t.Fatal("stopped forwarding owner still acknowledged a label operation")
		}
	})
}

// A restarted producer has no local label list. Source cleanup must use the
// native owner's retained claims and keep failed deletions claimed until retry.
func TestMPLSIntegration_SourceResetRetainsFailedDeletes(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)
		bed.enableInput()
		bus := newMPLSApplyBus(t, newFIBKernel(newTestBackend(h)))
		entries := []mplsfibevents.Entry{
			{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: 100, OutLabels: []uint32{200}, NextHop: mplsNextHop, Source: 7},
			{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: 102, OutLabels: []uint32{201}, NextHop: mplsNextHop, Source: 7},
			{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: 101, OutLabels: []uint32{300}, NextHop: mplsNextHop, Source: 1},
		}
		if err := mplsfibevents.Apply(bus, entries); err != nil {
			t.Fatal(err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{200}, mplsNextMAC)
		inner := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"), 4000, []byte("source-reset"))
		bed.inject([]uint32{102}, inner)
		out := bed.awaitForwarded("source label before reset", isMPLS)
		if !slices.Equal(labelStack(out), []uint32{201}) {
			t.Fatalf("source label did not forward before reset: %v", labelStack(out))
		}
		link, err := h.LinkByName(mplsZeLink)
		if err != nil {
			t.Fatal(err)
		}
		label := 100
		foreign := &netlink.Route{Family: unix.AF_MPLS, MPLSDst: &label, Protocol: 100,
			LinkIndex: link.Attrs().Index, NewDst: &netlink.MPLSDestination{Labels: []int{900}},
			Via: &netlink.Via{AddrFamily: unix.AF_INET, Addr: mplsBypassHop.AsSlice()}}
		if err := h.RouteReplace(foreign); err != nil {
			t.Fatal(err)
		}
		if err := mplsfibevents.RemoveLabelSource(bus, 7); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("source reset error = %v, want foreign-label refusal", err)
		}
		if _, present := mplsRoutes(t, h)[102]; present {
			t.Fatal("source reset retained its removable kernel label")
		}
		requireOwnedSwapFrame(t, bed, []uint32{900}, mplsBypassMAC)
		bed.inject([]uint32{102}, inner)
		bed.requireNothingForwarded("successfully removed source label still forwards")
		bed.inject([]uint32{101}, inner)
		out = bed.awaitForwarded("unrelated source label", isMPLS)
		if !slices.Equal(labelStack(out), []uint32{300}) {
			t.Fatalf("source reset changed another source's forwarding: %v", labelStack(out))
		}

		if err := h.RouteDel(foreign); err != nil {
			t.Fatal(err)
		}
		replacement := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
			InLabel: 100, OutLabels: []uint32{800}, NextHop: mplsNextHop, Source: 9}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{replacement}); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("failed deletion released the retained source claim: %v", err)
		}
		if err := mplsfibevents.RemoveLabelSource(bus, 7); err != nil {
			t.Fatalf("source reset retry failed: %v", err)
		}
		if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{replacement}); err != nil {
			t.Fatal(err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{800}, mplsNextMAC)
		if err := mplsfibevents.RemoveLabelSource(bus, 7); err != nil {
			t.Fatalf("completed source reset is not idempotent: %v", err)
		}
		requireOwnedSwapFrame(t, bed, []uint32{800}, mplsNextMAC)
	})
}
