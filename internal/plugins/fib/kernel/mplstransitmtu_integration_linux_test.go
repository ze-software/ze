//go:build integration && linux

// Design: docs/architecture/mpls/mpls-kernel.md -- Path MTU and label overhead
// Related: mplstransitmtu_linux_test.go -- the same decision with the probe faked
// Related: mplsmtu_integration_linux_test.go -- enforcement on the patched kernel

package fibkernel

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/kernelcap"
	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

// TestMPLSIntegration_TransitPathMTUFollowsTheProbe installs an RSVP-TE transit
// swap and pop that carry a downstream path MTU, through the forwarding owner's
// acknowledged path, on whatever kernel runs the test.
//
// VALIDATES: on a kernel without CONFIG_MPLS_IP_MTU both routes install without
// the MTU and the swap forwards; on Ze's runtime kernel both carry it. The probe
// answers the question on this kernel and never answers "unknown" with
// CAP_NET_ADMIN and a label space.
// PREVENTS: the stock-kernel failure in plan/journal/kernel-refuses-what-the-
// installer-sends.md: EINVAL on every transit install carrying an ADSPEC MTU,
// so no Ze transit LSP came up.
func TestMPLSIntegration_TransitPathMTUFollowsTheProbe(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)

		probe := kernelcap.MPLSIPMTU()
		require.NotEqual(t, kernelcap.StateUnknown, probe.State, "the probe could not answer: %v", probe.Reason)
		want := 0
		if probe.State == kernelcap.StatePresent {
			want = 1400
		}
		t.Logf("kernel transit MTU support: %v (%v); routes must carry MTU %d", probe.State, probe.Reason, want)

		backend := &netlinkBackend{handle: h}
		bus := newMPLSApplyBus(t, newFIBKernel(backend))
		require.NoError(t, mplsfibevents.Apply(bus, []mplsfibevents.Entry{
			{
				Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
				InLabel: 100, OutLabels: []uint32{200}, NextHop: mplsNextHop, PathMTU: 1400,
			},
			{
				Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPop,
				InLabel: 101, NextHop: mplsNextHop, PathMTU: 1400,
			},
		}), "a transit route carrying a path MTU was refused")

		// The backend asked the kernel the routes went into, and got the
		// same answer: a probe run in another namespace would read that
		// namespace's label space and could disagree.
		require.Equal(t, probe.State, backend.transitMTU, "the forwarding owner's probe answer differs from this namespace's")

		routes := mplsRoutes(t, h)
		for _, label := range []int{100, 101} {
			route, ok := routes[label]
			require.True(t, ok, "transit in-label %d is not in the kernel", label)
			require.Equal(t, want, route.MTU, "transit in-label %d carries the wrong MTU", label)
		}

		bed.enableInput()
		inner := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.20"), 4000, []byte("transit"))
		bed.inject([]uint32{100}, inner)
		out := bed.awaitForwarded("swapped MPLS frame", isMPLS)
		require.Equal(t, []uint32{200}, labelStack(out), "the kernel did not swap the transit label")
		require.Equal(t, inner, out[ethernetHeaderLen+labelEntryLen:], "the kernel altered the inner packet")
	})
}
