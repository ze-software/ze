// VALIDATES: spec-bcp194-6-blackhole AC-2 and AC-8 -- a BLACKHOLE-honored best
// path becomes a real RTN_BLACKHOLE route in the kernel, and its withdraw
// removes it. Real netlink in an ephemeral namespace, so the kernel itself is
// the judge, not the message this build assembles.
// PREVENTS: the failure the shape test alone cannot see -- netlink returning
// EINVAL for a discard route that names a next-hop. Every path sysrib carries
// resolves one. So the route type reached the backend, and the kernel
// programmed nothing. It did so silently, on the only surface an operator can
// check.

//go:build integration && linux

package fibkernel

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

func TestNetlinkIntegration_BlackholeRouteWithNextHop(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)
		require.NotNil(t, f.asRichBackend(), "netlink backend must be a richRouteBackend")

		const prefix = "192.0.2.1/32"

		// The shape sysrib actually emits for an honored BLACKHOLE announcement:
		// the resolved BGP next-hop is present, and RouteType says discard.
		f.processEvent(makeSysribPayload([]incomingChange{{
			Action:    routeaction.Add,
			Prefix:    netip.MustParsePrefix(prefix),
			NextHop:   netip.MustParseAddr("127.0.0.1"),
			Protocol:  "bgp",
			RouteType: sysribevents.RouteTypeBlackhole,
		}}))

		routes := zeRoutes(t, h)
		require.Len(t, routes, 1, "a BLACKHOLE best path must program exactly one ze kernel route")
		assert.Equal(t, prefix, routes[0].Dst.String())
		assert.Equal(t, unix.RTN_BLACKHOLE, routes[0].Type, "the kernel route must discard, not forward")
		assert.Nil(t, routes[0].Gw, "the kernel must hold no gateway for a discard route")

		// AC-8: the withdraw removes it. sysrib's withdraw carries only
		// Action+Prefix, so the delete must match on prefix and protocol alone.
		f.processEvent(makeSysribPayload([]incomingChange{
			withdrawChange(prefix),
		}))
		assert.Empty(t, zeRoutes(t, h), "the discard route must leave the kernel on withdraw")
	})
}
