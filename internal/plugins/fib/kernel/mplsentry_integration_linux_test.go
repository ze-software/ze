//go:build integration && linux

// Design: docs/architecture/mpls/mpls-kernel.md, plan/spec-mpls-2-ldp.md -- live-kernel MPLS
// dataplane verification. Exercises the real netlink backend (mplsentry_linux.go,
// nexthop_linux.go) against the QEMU Alpine kernel: program push (IP route + label
// encap), swap (AF_MPLS in->out via next-hop) and pop (AF_MPLS disposition), then
// read the entries back from the kernel. The swap and pop proofs go one step
// further and inject a labeled frame (mplsframe_integration_linux_test.go),
// because an AF_MPLS entry that reads back says nothing about whether the
// kernel forwards on it. This is handover item #1 (kernel push/swap/pop
// end-to-end), which could not be verified on darwin.
package fibkernel

import (
	"net"
	"net/netip"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

const mplsPlatformLabels = "/proc/sys/net/mpls/platform_labels"

// loadMPLSModules best-effort loads the kernel MPLS modules in the host (init)
// namespace before any netns switch. Modules are global; the per-netns sysctl
// tree (net.mpls) only appears once mpls_router is loaded.
func loadMPLSModules(t *testing.T) {
	t.Helper()
	for _, m := range []string{"mpls_router", "mpls_iptunnel"} {
		_ = exec.Command("modprobe", m).Run() //nolint:errcheck,noctx // best-effort module load; verified below
	}
	if _, err := os.Stat(mplsPlatformLabels); err != nil {
		t.Skipf("kernel MPLS unavailable (no %s): %v -- build a kernel with CONFIG_MPLS_ROUTING", mplsPlatformLabels, err)
	}
}

// enableNetnsMPLS raises the per-netns MPLS label space so the kernel accepts
// AF_MPLS routes. Must run after the netns switch (the sysctl is per-netns).
func enableNetnsMPLS(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(mplsPlatformLabels, []byte("1048575"), 0o644); err != nil {
		t.Skipf("cannot set net.mpls.platform_labels in netns: %v", err)
	}
}

// setupDummyLink brings loopback up (egress pop disposition routes out lo) and
// creates an up dummy interface with a connected /24 so next-hops used by
// swap/push routes resolve on-link.
func setupDummyLink(t *testing.T, h *netlink.Handle) {
	t.Helper()
	lo, err := h.LinkByName("lo")
	require.NoError(t, err)
	require.NoError(t, h.LinkSetUp(lo))

	la := netlink.NewLinkAttrs()
	la.Name = "ze-mpls0"
	require.NoError(t, h.LinkAdd(&netlink.Dummy{LinkAttrs: la}))
	link, err := h.LinkByName("ze-mpls0")
	require.NoError(t, err)
	addr, err := netlink.ParseAddr("10.0.0.1/24")
	require.NoError(t, err)
	require.NoError(t, h.AddrAdd(link, addr))
	require.NoError(t, h.LinkSetUp(link))
}

// mplsRoutes returns the fib-kernel-owned AF_MPLS routes keyed by in-label.
func mplsRoutes(t *testing.T, h *netlink.Handle) map[int]netlink.Route {
	t.Helper()
	routes, err := h.RouteList(nil, unix.AF_MPLS)
	require.NoError(t, err)
	out := make(map[int]netlink.Route)
	for i := range routes {
		r := routes[i]
		if r.Protocol != rtprotZE || r.MPLSDst == nil {
			continue
		}
		out[*r.MPLSDst] = r
	}
	return out
}

// VALIDATES: mpls-3 dataplane -- a swap entry installs an AF_MPLS route that the
// kernel accepts, keyed by in-label, carrying the out-label stack and next-hop,
// and the kernel then FORWARDS on it: a frame labeled 100 that arrives on an
// interface with net.mpls.conf.<iface>.input set leaves for next hop 10.0.0.2
// labeled 200, and leaves for nobody while input is unset or once the entry is
// withdrawn.
// PREVENTS: the netlink AF_MPLS encoding silently failing on a real kernel, and
// a read-back reading green over a plane that forwards nothing.
func TestMPLSIntegration_Swap(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)

		f := newFIBKernel(newTestBackend(h))
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:    mplsfibevents.ActionAdd,
			Op:        mplsfibevents.OpSwap,
			InLabel:   100,
			OutLabels: []uint32{200},
			NextHop:   mplsNextHop,
		}}})

		routes := mplsRoutes(t, h)
		swap, ok := routes[100]
		require.True(t, ok, "swap route for in-label 100 not found in kernel")
		require.NotNil(t, swap.NewDst, "swap route must carry an out-label stack")
		dst, ok := swap.NewDst.(*netlink.MPLSDestination)
		require.True(t, ok, "NewDst is *MPLSDestination")
		assert.Equal(t, []int{200}, dst.Labels)
		require.NotNil(t, swap.Via, "swap route must carry a via next-hop")

		// Control: the entry is in the table, and the interface does not accept
		// labeled frames, so the kernel forwards nothing.
		inner := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"), 4000, []byte("swap"))
		bed.inject([]uint32{100}, inner)
		bed.requireNothingForwarded("input is unset on " + mplsZeLink)

		bed.enableInput()
		bed.inject([]uint32{100}, inner)
		out := bed.awaitForwarded("MPLS frame", isMPLS)
		assert.Equal(t, []uint32{200}, labelStack(out), "the kernel swapped to another label stack")
		assert.Equal(t, mplsNextMAC, net.HardwareAddr(out[0:6]), "the kernel sent the swapped frame to another next hop")
		assert.Equal(t, inner, out[ethernetHeaderLen+labelEntryLen:], "the kernel altered the inner packet under the swap")

		// Withdraw removes it from the kernel, and the same frame is then dropped.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:  mplsfibevents.ActionRemove,
			Op:      mplsfibevents.OpSwap,
			InLabel: 100,
		}}})
		_, ok = mplsRoutes(t, h)[100]
		assert.False(t, ok, "swap route should be gone after withdraw")
		bed.inject([]uint32{100}, inner)
		bed.requireNothingForwarded("the swap entry for in-label 100 was withdrawn")
	})
}

// VALIDATES: mpls-4 AC-3 (RFC 4090 facility backup) -- a local-repair backup swap
// installs an AF_MPLS route carrying a TWO-label out-stack (the bypass label over
// the swapped protected label), exactly what rsvpte busFIB.ProgramBackup emits on
// local repair, and the kernel forwards on it: a frame labeled 100 leaves for the
// BYPASS next hop 10.0.0.5 carrying [5000, 200]. This is the live-kernel proof of
// spec assumption A-1: the kernel MPLS backend programs a 2-label facility-backup
// stack in one swap entry, so no new data-plane primitive is needed.
// PREVENTS: the facility-backup label stacking silently failing on a real kernel,
// and a repair that reads back replaced while the frame still takes the old path.
func TestMPLSIntegration_FacilityBackupSwap(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)
		bed.enableInput()

		f := newFIBKernel(newTestBackend(h))
		// First the protected LSP's ordinary single-label swap is installed (what
		// rsvpte handleResvTransit emits when the LSP comes up), and the frame
		// takes the protected path.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:    mplsfibevents.ActionAdd,
			Op:        mplsfibevents.OpSwap,
			InLabel:   100,
			OutLabels: []uint32{200},
			NextHop:   mplsNextHop,
		}}})
		inner := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"), 4000, []byte("backup"))
		bed.inject([]uint32{100}, inner)
		out := bed.awaitForwarded("MPLS frame on the protected path", isMPLS)
		require.Equal(t, []uint32{200}, labelStack(out), "the protected swap forwarded another label stack")
		require.Equal(t, mplsNextMAC, net.HardwareAddr(out[0:6]), "the protected swap forwarded to another next hop")

		// On local repair rsvpte busFIB.ProgramBackup re-programs the SAME in-label
		// with the 2-label backup stack [bypass, protected] via the bypass next hop
		// (here a different on-link neighbor, modeling the alternate-link bypass).
		// This must REPLACE the existing swap (RouteReplace), not fail EEXIST.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:    mplsfibevents.ActionAdd,
			Op:        mplsfibevents.OpSwap,
			InLabel:   100,
			OutLabels: []uint32{5000, 200}, // bypass label outermost, protected label under it
			NextHop:   mplsBypassHop,
		}}})

		swap, ok := mplsRoutes(t, h)[100]
		require.True(t, ok, "backup swap route for in-label 100 not found in kernel")
		require.NotNil(t, swap.NewDst, "backup swap must carry an out-label stack")
		dst, ok := swap.NewDst.(*netlink.MPLSDestination)
		require.True(t, ok, "NewDst is *MPLSDestination")
		assert.Equal(t, []int{5000, 200}, dst.Labels, "2-label facility-backup stack replaced the single-label swap")
		require.NotNil(t, swap.Via, "backup swap must carry a via next-hop")

		// The same frame now takes the bypass: two labels out, to the bypass neighbor.
		bed.inject([]uint32{100}, inner)
		out = bed.awaitForwarded("MPLS frame on the bypass", isMPLS)
		assert.Equal(t, []uint32{5000, 200}, labelStack(out), "the kernel forwarded another label stack after the repair")
		assert.Equal(t, mplsBypassMAC, net.HardwareAddr(out[0:6]), "the kernel forwarded to another next hop than the bypass")
		assert.Equal(t, inner, out[ethernetHeaderLen+2*labelEntryLen:], "the kernel altered the inner packet under the backup stack")

		// Withdraw removes it from the kernel, and the same frame is then dropped.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpSwap, InLabel: 100,
		}}})
		_, ok = mplsRoutes(t, h)[100]
		assert.False(t, ok, "backup swap should be gone after withdraw")
		bed.inject([]uint32{100}, inner)
		bed.requireNothingForwarded("the backup swap for in-label 100 was withdrawn")
	})
}

// VALIDATES: mpls-2 AC-3 / mpls-3 dataplane -- a pop entry with a next-hop
// (penultimate-style disposition) installs an AF_MPLS route with no out-labels,
// and the kernel forwards on it: a frame labeled 101 leaves for next hop 10.0.0.2
// as a bare IPv4 packet, and leaves for nobody while input is unset.
// PREVENTS: a pop that reads back correct while the interface discards every
// labeled frame.
func TestMPLSIntegration_PopWithNextHop(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)

		f := newFIBKernel(newTestBackend(h))
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:  mplsfibevents.ActionAdd,
			Op:      mplsfibevents.OpPop,
			InLabel: 101,
			NextHop: mplsNextHop,
		}}})

		pop, ok := mplsRoutes(t, h)[101]
		require.True(t, ok, "pop route for in-label 101 not found in kernel")
		assert.Nil(t, pop.NewDst, "pop route must have no out-label stack")

		// Control: the entry is in the table, and the interface does not accept
		// labeled frames, so the kernel forwards nothing.
		inner := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"), 4000, []byte("pop"))
		bed.inject([]uint32{101}, inner)
		bed.requireNothingForwarded("input is unset on " + mplsZeLink)

		bed.enableInput()
		bed.inject([]uint32{101}, inner)
		out := bed.awaitForwarded("IPv4 frame", isIPv4)
		assert.Equal(t, mplsNextMAC, net.HardwareAddr(out[0:6]), "the kernel sent the popped packet to another next hop")
		// The kernel propagates the decremented label TTL into the IPv4 header
		// (net.mpls.ip_ttl_propagate defaults to 1) and refreshes the header
		// checksum, so the popped packet is the inner one with those two fields
		// rewritten. Compare everything else, then the TTL on its own.
		popped := out[ethernetHeaderLen:]
		require.GreaterOrEqual(t, len(popped), len(inner), "the popped packet is shorter than the inner one")
		assert.Equal(t, inner[:8], popped[:8], "the kernel altered the IPv4 header before the TTL")
		assert.Equal(t, inner[12:], popped[12:len(inner)], "the kernel altered the inner packet after the checksum")
		assert.Equal(t, uint8(mplsTTL-1), popped[8], "the kernel wrote another TTL into the popped packet")
	})
}

// VALIDATES: mpls-2 AC-3 / mpls-3 egress -- the LDP and RSVP-TE egress-pop paths
// emit a pop entry with NO next-hop (ultimate-hop popping). The backend must give
// it an output device (loopback) so the kernel accepts it and routes the inner
// packet via a normal FIB lookup, and the kernel then does so: a frame labeled
// 102 whose inner packet is addressed to ze's own address is delivered to a UDP
// socket listening there, and is delivered to nobody while input is unset. Goes
// through the production handleMPLSEntry path.
// PREVENTS: regression of the live-kernel "no such device" rejection that the
// QEMU run surfaced for no-via pops, and a loopback disposition that reads back
// while the popped packet never re-enters the IP receive path.
func TestMPLSIntegration_EgressPopNoNextHop(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)

		f := newFIBKernel(newTestBackend(h))
		// Exactly what ldpFIB.ProgramPop / rsvpte busFIB.ProgramPop emit: pop, no
		// out-labels, no via.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:  mplsfibevents.ActionAdd,
			Op:      mplsfibevents.OpPop,
			InLabel: 102,
		}}})

		lo, err := h.LinkByName("lo")
		require.NoError(t, err)
		pop, ok := mplsRoutes(t, h)[102]
		require.True(t, ok, "egress pop route for in-label 102 not found in kernel (no-via pop rejected?)")
		assert.Nil(t, pop.NewDst, "egress pop must have no out-label stack")
		assert.Equal(t, lo.Attrs().Index, pop.LinkIndex, "egress pop must dispose via loopback")

		// The inner packet is from the on-link neighbor to ze's own address, so
		// once popped and re-injected through loopback the FIB delivers it locally.
		conn, port := listenUDP(t)
		inner := ipv4UDP(mplsNextHop, netip.MustParseAddr("10.0.0.1"), port, []byte("egress"))

		// Control: the entry is in the table, and the interface does not accept
		// labeled frames, so nothing reaches the socket.
		bed.inject([]uint32{102}, inner)
		requireNoDatagram(t, conn, "input is unset on "+mplsZeLink)

		bed.enableInput()
		bed.inject([]uint32{102}, inner)
		assert.Equal(t, []byte("egress"), awaitDatagram(t, conn), "the kernel delivered another payload than the popped one")
	})
}

// VALIDATES: mpls-3 dataplane -- a push entry installs an IP route with an MPLS
// label encap (ingress imposition), reusing the rich-route path.
func TestMPLSIntegration_Push(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)

		f := newFIBKernel(newTestBackend(h))
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:    mplsfibevents.ActionAdd,
			Op:        mplsfibevents.OpPush,
			FEC:       netip.MustParsePrefix("10.9.0.0/24"),
			OutLabels: []uint32{300},
			NextHop:   netip.MustParseAddr("10.0.0.2"),
		}}})

		routes, err := h.RouteList(nil, netlink.FAMILY_V4)
		require.NoError(t, err)
		var push *netlink.Route
		for i := range routes {
			if routes[i].Protocol == rtprotZE && routes[i].Dst != nil && routes[i].Dst.String() == "10.9.0.0/24" {
				push = &routes[i]
				break
			}
		}
		require.NotNil(t, push, "push route 10.9.0.0/24 not found in kernel")
		require.NotNil(t, push.Encap, "push route must carry an MPLS label encap")
		enc, ok := push.Encap.(*netlink.MPLSEncap)
		require.True(t, ok, "encap is *MPLSEncap")
		assert.Equal(t, []int{300}, enc.Labels)

		// Withdraw removes the push route.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action: mplsfibevents.ActionRemove,
			Op:     mplsfibevents.OpPush,
			FEC:    netip.MustParsePrefix("10.9.0.0/24"),
		}}})
		routes, err = h.RouteList(nil, netlink.FAMILY_V4)
		require.NoError(t, err)
		for i := range routes {
			if routes[i].Protocol == rtprotZE && routes[i].Dst != nil && routes[i].Dst.String() == "10.9.0.0/24" {
				t.Fatal("push route should be gone after withdraw")
			}
		}
	})
}

// pushEncapLabels returns the MPLS encap labels of the fib-kernel push route for
// dst, or nil if absent.
func pushEncapLabels(t *testing.T, h *netlink.Handle, dst string) []int { //nolint:unparam // dst kept explicit for call-site readability
	t.Helper()
	routes, err := h.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)
	for i := range routes {
		if routes[i].Protocol == rtprotZE && routes[i].Dst != nil && routes[i].Dst.String() == dst {
			enc, ok := routes[i].Encap.(*netlink.MPLSEncap)
			if !ok {
				return nil
			}
			return enc.Labels
		}
	}
	return nil
}

// VALIDATES: mpls-2 -- an MPLS push for a prefix another FIB writer already owns
// does NOT clobber that route (first install uses RouteAdd, which fails EEXIST).
// The MPLS push bypasses sysrib arbitration, so it must not stomp foreign routes.
func TestMPLSIntegration_PushNoClobberForeignRoute(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)

		const foreignProto = 100 // not rtprotZE
		addProtocolRoute(t, h, "10.9.0.0/24", "10.0.0.2", foreignProto)

		f := newFIBKernel(newTestBackend(h))
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
			FEC: netip.MustParsePrefix("10.9.0.0/24"), OutLabels: []uint32{300},
			NextHop: netip.MustParseAddr("10.0.0.2"),
		}}})

		routes, err := h.RouteList(nil, netlink.FAMILY_V4)
		require.NoError(t, err)
		var found *netlink.Route
		for i := range routes {
			if routes[i].Dst != nil && routes[i].Dst.String() == "10.9.0.0/24" {
				found = &routes[i]
			}
		}
		require.NotNil(t, found, "foreign route disappeared")
		assert.Equal(t, foreignProto, int(found.Protocol), "foreign route must be preserved, not taken over by ze")
		assert.Nil(t, found.Encap, "foreign route must not gain an MPLS label encap")
	})
}

// VALIDATES: mpls-2 -- re-advertising a FEC with a new label updates the kernel
// route (RouteReplace), rather than leaving the old label imposed (RouteAdd EEXIST).
func TestMPLSIntegration_PushRelabel(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)

		f := newFIBKernel(newTestBackend(h))
		fec := netip.MustParsePrefix("10.9.0.0/24")
		nh := netip.MustParseAddr("10.0.0.2")

		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush, FEC: fec, OutLabels: []uint32{300}, NextHop: nh,
		}}})
		require.Equal(t, []int{300}, pushEncapLabels(t, h, "10.9.0.0/24"), "initial label")

		// Relabel the same FEC.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush, FEC: fec, OutLabels: []uint32{400}, NextHop: nh,
		}}})
		require.Equal(t, []int{400}, pushEncapLabels(t, h, "10.9.0.0/24"), "relabel must update the kernel route")
	})
}
