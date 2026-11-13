//go:build linux

// Design: docs/guide/plugins.md -- installed-route repair replay.
// Related: backend_replay_linux.go -- bounded kernel operations.

package fibkernel

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// replayOperations records synchronous requests. Its zero value is a valid
// unexpected-ACK fixture; nil pointers are invalid. It never mutates requests.
type replayOperations struct {
	probeErr  error
	changeErr error
	calls     []string
	routes    []*netlink.Route
}

func (ops *replayOperations) RouteProbeIPv4(route *netlink.Route) error {
	ops.calls = append(ops.calls, "probe")
	ops.routes = append(ops.routes, route)
	return ops.probeErr
}

func (ops *replayOperations) RouteChange(route *netlink.Route) error {
	ops.calls = append(ops.calls, "change")
	ops.routes = append(ops.routes, route)
	return ops.changeErr
}

func replayOperationRoute(t *testing.T) *netlink.Route {
	t.Helper()
	_, destination, err := net.ParseCIDR("198.18.86.0/24")
	require.NoError(t, err)
	return &netlink.Route{
		Dst: destination, Protocol: rtprotZE, Table: 173, Priority: 71,
		Type: unix.RTN_UNICAST, Scope: netlink.SCOPE_UNIVERSE, MTU: 1400,
		MultiPath: []*netlink.NexthopInfo{
			{LinkIndex: 7, Gw: net.ParseIP("192.0.2.2"), Hops: 2, Flags: unix.RTNH_F_ONLINK},
			{LinkIndex: 8, Gw: net.ParseIP("192.0.2.3"), Hops: 3},
		},
	}
}

func TestReplayRouteOperationsExactIdentityChanges(t *testing.T) {
	route := replayOperationRoute(t)
	ops := &replayOperations{probeErr: fmt.Errorf("identity: %w", unix.EEXIST)}
	handled, err := reassertOwnedRoute(ops, route)
	assert.True(t, handled, "an exact live identity must perform a reassertion")
	require.NoError(t, err)
	assert.Equal(t, []string{"probe", "change"}, ops.calls)
	require.Len(t, ops.routes, 2)
	assert.Same(t, route, ops.routes[0], "the probe must receive the complete rendered forwarding group")
	assert.Same(t, route, ops.routes[1], "the actual change must use the exact probed description")
}

func TestReplayRouteOperationsFallback(t *testing.T) {
	for _, test := range []struct {
		name          string
		probeErr      error
		changeErr     error
		ipv6          bool
		noDestination bool
		calls         []string
	}{
		{name: "missing identity", probeErr: unix.ENOENT, calls: []string{"probe"}},
		{name: "disappeared before change", probeErr: unix.EEXIST, changeErr: unix.ENOENT, calls: []string{"probe", "change"}},
		{name: "removed before change", probeErr: unix.EEXIST, changeErr: unix.ESRCH, calls: []string{"probe", "change"}},
		{name: "IPv6", ipv6: true},
		{name: "unhandled destination", noDestination: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			route := replayOperationRoute(t)
			if test.ipv6 {
				_, destination, err := net.ParseCIDR("2001:db8:86::/64")
				require.NoError(t, err)
				route.Dst = destination
			}
			if test.noDestination {
				route.Dst = nil
			}
			ops := &replayOperations{probeErr: test.probeErr, changeErr: test.changeErr}
			handled, err := reassertOwnedRoute(ops, route)
			assert.False(t, handled, "the ordinary ownership path must repair this case")
			assert.NoError(t, err)
			assert.Equal(t, test.calls, ops.calls)
		})
	}
}

func TestReplayRouteOperationsFailure(t *testing.T) {
	for _, test := range []struct {
		name      string
		probeErr  error
		changeErr error
		wantErr   error
		calls     []string
	}{
		{name: "probe refused", probeErr: unix.EPERM, wantErr: unix.EPERM, calls: []string{"probe"}},
		{name: "unexpected positive ACK", wantErr: unix.EIO, calls: []string{"probe"}},
		{name: "foreign preferred alias", probeErr: unix.EEXIST, changeErr: unix.EEXIST, wantErr: unix.EEXIST, calls: []string{"probe", "change"}},
		{name: "change refused", probeErr: unix.EEXIST, changeErr: unix.EPERM, wantErr: unix.EPERM, calls: []string{"probe", "change"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ops := &replayOperations{probeErr: test.probeErr, changeErr: test.changeErr}
			handled, err := reassertOwnedRoute(ops, replayOperationRoute(t))
			assert.True(t, handled, "failure must not fall through to an ordinary replacement")
			assert.ErrorIs(t, err, test.wantErr)
			assert.True(t, reflect.DeepEqual(test.calls, ops.calls), "calls = %v, want %v", ops.calls, test.calls)
		})
	}
}

func TestReplayRichRouteRejectsInvalidDescription(t *testing.T) {
	backend := &netlinkBackend{}
	handled, err := backend.reassertRichRoute(RichRoute{})
	assert.True(t, handled, "an invalid description must fail before reaching a netlink handle")
	assert.Error(t, err)
}
