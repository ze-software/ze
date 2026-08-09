//go:build integration && linux

// Design: docs/architecture/mpls/mpls-kernel.md, plan/spec-mpls-2-ldp.md -- live-kernel MPLS
// dataplane verification. Exercises the real netlink backend (mplsentry_linux.go,
// nexthop_linux.go) against the QEMU Alpine kernel: program push (IP route + label
// encap), swap (AF_MPLS in->out via next-hop) and pop (AF_MPLS disposition), then
// read the entries back from the kernel. This is handover item #1 (kernel
// push/swap/pop end-to-end), which could not be verified on darwin.
package fibkernel

import (
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
// kernel accepts, keyed by in-label, carrying the out-label stack and next-hop.
// PREVENTS: the netlink AF_MPLS encoding silently failing on a real kernel.
func TestMPLSIntegration_Swap(t *testing.T) {
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
			Op:        mplsfibevents.OpSwap,
			InLabel:   100,
			OutLabels: []uint32{200},
			NextHop:   netip.MustParseAddr("10.0.0.2"),
		}}})

		routes := mplsRoutes(t, h)
		swap, ok := routes[100]
		require.True(t, ok, "swap route for in-label 100 not found in kernel")
		require.NotNil(t, swap.NewDst, "swap route must carry an out-label stack")
		dst, ok := swap.NewDst.(*netlink.MPLSDestination)
		require.True(t, ok, "NewDst is *MPLSDestination")
		assert.Equal(t, []int{200}, dst.Labels)
		require.NotNil(t, swap.Via, "swap route must carry a via next-hop")

		// Withdraw removes it from the kernel.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:  mplsfibevents.ActionRemove,
			Op:      mplsfibevents.OpSwap,
			InLabel: 100,
		}}})
		_, ok = mplsRoutes(t, h)[100]
		assert.False(t, ok, "swap route should be gone after withdraw")
	})
}

// VALIDATES: mpls-4 AC-3 (RFC 4090 facility backup) -- a local-repair backup swap
// installs an AF_MPLS route carrying a TWO-label out-stack (the bypass label over
// the swapped protected label), exactly what rsvpte busFIB.ProgramBackup emits on
// local repair. This is the live-kernel proof of spec assumption A-1: the kernel
// MPLS backend programs a 2-label facility-backup stack in one swap entry, so no
// new data-plane primitive is needed.
// PREVENTS: the facility-backup label stacking silently failing on a real kernel.
func TestMPLSIntegration_FacilityBackupSwap(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)

		f := newFIBKernel(newTestBackend(h))
		// First the protected LSP's ordinary single-label swap is installed (what
		// rsvpte handleResvTransit emits when the LSP comes up).
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:    mplsfibevents.ActionAdd,
			Op:        mplsfibevents.OpSwap,
			InLabel:   100,
			OutLabels: []uint32{200},
			NextHop:   netip.MustParseAddr("10.0.0.2"),
		}}})

		// On local repair rsvpte busFIB.ProgramBackup re-programs the SAME in-label
		// with the 2-label backup stack [bypass, protected] via the bypass next hop
		// (here a different on-link neighbor, modeling the alternate-link bypass).
		// This must REPLACE the existing swap (RouteReplace), not fail EEXIST.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:    mplsfibevents.ActionAdd,
			Op:        mplsfibevents.OpSwap,
			InLabel:   100,
			OutLabels: []uint32{5000, 200}, // bypass label outermost, protected label under it
			NextHop:   netip.MustParseAddr("10.0.0.5"),
		}}})

		swap, ok := mplsRoutes(t, h)[100]
		require.True(t, ok, "backup swap route for in-label 100 not found in kernel")
		require.NotNil(t, swap.NewDst, "backup swap must carry an out-label stack")
		dst, ok := swap.NewDst.(*netlink.MPLSDestination)
		require.True(t, ok, "NewDst is *MPLSDestination")
		assert.Equal(t, []int{5000, 200}, dst.Labels, "2-label facility-backup stack replaced the single-label swap")
		require.NotNil(t, swap.Via, "backup swap must carry a via next-hop")

		// Withdraw removes it from the kernel.
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpSwap, InLabel: 100,
		}}})
		_, ok = mplsRoutes(t, h)[100]
		assert.False(t, ok, "backup swap should be gone after withdraw")
	})
}

// VALIDATES: mpls-2 AC-3 / mpls-3 dataplane -- a pop entry with a next-hop
// (penultimate-style disposition) installs an AF_MPLS route with no out-labels.
func TestMPLSIntegration_PopWithNextHop(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)

		f := newFIBKernel(newTestBackend(h))
		f.handleMPLSEntry(&mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{{
			Action:  mplsfibevents.ActionAdd,
			Op:      mplsfibevents.OpPop,
			InLabel: 101,
			NextHop: netip.MustParseAddr("10.0.0.2"),
		}}})

		pop, ok := mplsRoutes(t, h)[101]
		require.True(t, ok, "pop route for in-label 101 not found in kernel")
		assert.Nil(t, pop.NewDst, "pop route must have no out-label stack")
	})
}

// VALIDATES: mpls-2 AC-3 / mpls-3 egress -- the LDP and RSVP-TE egress-pop paths
// emit a pop entry with NO next-hop (ultimate-hop popping). The backend must give
// it an output device (loopback) so the kernel accepts it and routes the inner
// packet via a normal FIB lookup. Goes through the production handleMPLSEntry path.
// PREVENTS: regression of the live-kernel "no such device" rejection that the
// QEMU run surfaced for no-via pops.
func TestMPLSIntegration_EgressPopNoNextHop(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		setupDummyLink(t, h)

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
