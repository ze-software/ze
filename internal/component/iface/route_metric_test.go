// Design: docs/guide/configuration.md -- Route Priority
// Related: config.go -- defaultLearnedRouteMetric, parseUnits, parsePPPoEClientEntry
// Related: register.go -- reconcileDHCP, writtenRoutePriorities, handleLinkDown

package iface

import (
	"errors"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/pkg/ze"
)

// recordingDHCPFactory records the route metric each DHCP client is created
// with. The metric is the only thing the client can put in RTM_NEWROUTE, so it
// is where the learned-route default becomes a kernel route.
type recordingDHCPFactory struct {
	metrics map[string]int
}

type noopStopper struct{}

func (noopStopper) Stop() {}

func (f *recordingDHCPFactory) install(t *testing.T) {
	t.Helper()
	f.metrics = make(map[string]int)
	previous := dhcpClientFactory
	SetDHCPClientFactory(func(ifaceName, unit string, _ ze.EventBus, _, _ bool, _, _ string, _ int, _, _ string, _ bool, routeMetric int) (DHCPStopper, error) {
		f.metrics[ifaceName+"/"+unit] = routeMetric
		return noopStopper{}, nil
	})
	t.Cleanup(func() { dhcpClientFactory = previous })
}

func dhcpUnitJSON(t *testing.T, unitBody string) *ifaceConfig {
	t.Helper()
	return mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {`+unitBody+`
							"ipv4": {"dhcp": {"enabled": "true"}}
						}
					}
				}
			}
		}
	}`)
}

// TestLearnedDefaultRouteReachesTheDHCPClientAtItsOwnMetric proves the metric a
// DHCPv4 lease installs its default route with, for a unit that says nothing
// about route-priority.
//
// VALIDATES: a learned default route is ranked below an operator's static
// default rather than sharing metric 0 with it. Two defaults at different
// metrics coexist in the kernel and the lower one wins, so the operator's route
// keeps forwarding and keeps its gateway.
// PREVENTS: the metric-0 install this fix exists to remove. RouteReplace
// (internal/plugins/iface/netlink/manage_linux.go, (*netlinkBackend).AddRoute)
// matches on destination, metric and table and takes no protocol, so a learned
// default at metric 0 OVERWRITES an operator's static default at metric 0,
// gateway included, and re-stamps it proto 253.
func TestLearnedDefaultRouteReachesTheDHCPClientAtItsOwnMetric(t *testing.T) {
	factory := &recordingDHCPFactory{}
	factory.install(t)

	t.Run("no route-priority takes the learned metric", func(t *testing.T) {
		active := map[dhcpUnitKey]dhcpEntry{}
		reconcileDHCP(dhcpUnitJSON(t, ""), nil, active, slog.Default())

		require.Equal(t, 254, factory.metrics["eth0/0"],
			"the DHCP client must install its default route at defaultLearnedRouteMetric, not at 0")
		entry, ok := active[dhcpUnitKey{ifaceName: "eth0", unit: "0"}]
		require.True(t, ok, "the client is tracked for link failover")
		assert.Equal(t, defaultLearnedRouteMetric, entry.params.routePriority,
			"link down and link up shuffle the same metric the client installed")
		assert.False(t, entry.params.routePrioritySet,
			"an absent leaf must not read as an operator asking ze to own the routes")
	})

	t.Run("an explicit 0 restores the pre-2026-08-11 metric", func(t *testing.T) {
		active := map[dhcpUnitKey]dhcpEntry{}
		reconcileDHCP(dhcpUnitJSON(t, `"route-priority": "0",`), nil, active, slog.Default())

		assert.Equal(t, 0, factory.metrics["eth0/0"],
			"route-priority 0 is the documented way back to the kernel metric")
	})

	t.Run("a written priority still wins", func(t *testing.T) {
		active := map[dhcpUnitKey]dhcpEntry{}
		reconcileDHCP(dhcpUnitJSON(t, `"route-priority": "5",`), nil, active, slog.Default())

		assert.Equal(t, 5, factory.metrics["eth0/0"])
		assert.True(t, active[dhcpUnitKey{ifaceName: "eth0", unit: "0"}].params.routePrioritySet)
	})
}

// TestLearnedMetricSurvivesTheLinkBounce proves the metric the failover path
// removes and restores is the learned metric, not 0.
//
// VALIDATES: the link-down and link-up shuffle stays on the learned route and
// never reaches the metric an operator's static default occupies.
// PREVENTS: a carrier bounce re-installing the learned default at metric 0 and
// taking over the static route the lease install no longer touches.
func TestLearnedMetricSurvivesTheLinkBounce(t *testing.T) {
	factory := &recordingDHCPFactory{}
	factory.install(t)
	fb := setupFakeBackendForTest(t)

	active := map[dhcpUnitKey]dhcpEntry{}
	reconcileDHCP(dhcpUnitJSON(t, ""), nil, active, slog.Default())

	key := dhcpUnitKey{ifaceName: "eth0", unit: "0"}
	entry := active[key]
	entry.gateway = "192.0.2.1"
	active[key] = entry

	handleLinkDown("eth0", active, slog.Default())
	require.Len(t, fb.routeRemoves, 1)
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", 254, rtproto.Iface}, fb.routeRemoves[0])
	require.Len(t, fb.routeAdds, 1)
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", 254 + deprioritizedMetric, rtproto.Iface}, fb.routeAdds[0])

	handleLinkUp("eth0", active, slog.Default())
	require.Len(t, fb.routeAdds, 2)
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", 254, rtproto.Iface}, fb.routeAdds[1])
}

// TestRepeatedLinkEventsMoveTheRouteOnce proves a link event reporting a state
// ze has already acted on reaches no route call at all.
//
// VALIDATES: the link handlers move a default route between the base metric and
// the deprioritized one, and each is a no-op once the route is already there.
// A lease event ends the dedupe, because it says nothing reliable about which
// metric carries a route: the client publishes the lease whether or not its own
// install succeeded, and that install removes nothing a link handler moved.
// PREVENTS: the cost of the repeat. (*monitor).handleLinkUpdate
// (internal/plugins/iface/netlink/monitor_linux.go) emits up or down on EVERY
// RTM_NEWLINK for a link it already knows, with no comparison against the state
// it last reported, so an MTU change or a carrier bounce reaches these handlers
// with nothing to do. Each repeat deleted a route that had already moved, added
// the route that was already there, and made reportRemoveRouteMiss
// (internal/plugins/iface/netlink/manage_linux.go) dump the whole route table of
// the family to choose the log level of the delete the kernel had refused.
func TestRepeatedLinkEventsMoveTheRouteOnce(t *testing.T) {
	factory := &recordingDHCPFactory{}
	factory.install(t)
	fb := setupFakeBackendForTest(t)

	active := map[dhcpUnitKey]dhcpEntry{}
	reconcileDHCP(dhcpUnitJSON(t, ""), nil, active, slog.Default())

	key := dhcpUnitKey{ifaceName: "eth0", unit: "0"}
	entry := active[key]
	entry.gateway = "192.0.2.1"
	active[key] = entry

	handleLinkDown("eth0", active, slog.Default())
	handleLinkDown("eth0", active, slog.Default())
	require.Len(t, fb.routeRemoves, 1, "the second down event moved a route that had already moved")
	require.Len(t, fb.routeAdds, 1, "the second down event reinstalled a route that was already there")

	handleLinkUp("eth0", active, slog.Default())
	handleLinkUp("eth0", active, slog.Default())
	require.Len(t, fb.routeRemoves, 2, "the second up event removed a route that was no longer deprioritized")
	require.Len(t, fb.routeAdds, 2, "the second up event reinstalled a route that was already there")

	// A lease event leaves ze not knowing where the route is, so the next link
	// event runs the full remove-and-add again. That run is what clears a
	// deprioritized route the client's own install left beside its new one, and
	// what reinstalls a route whose install failed.
	handleDHCPLeaseEvent(`{"name":"eth0","unit":"0","router":"192.0.2.1"}`, active, slog.Default())
	assert.Equal(t, routeMetricUnknown, active[key].metricState,
		"a lease is not evidence about which metric carries a route")
	handleLinkUp("eth0", active, slog.Default())
	require.Len(t, fb.routeRemoves, 3, "the up event after a lease must remove any deprioritized leftover")
	require.Len(t, fb.routeAdds, 3, "and reinstall the base-metric route the lease may have failed to install")
	require.Equal(t, routeMetricBase, active[key].metricState)
}

// TestAFailedMetricMoveLeavesTheRouteRecoverable proves what a link handler
// records when its RemoveRoute landed and its AddRoute did not.
//
// VALIDATES: the entry is left at routeMetricUnknown, which is the one state
// the opposite event acts on. The route is then reinstalled by the next event
// the box sees, without waiting for a new DHCP lease.
// PREVENTS: the dedupe guard swallowing the repair. The handlers returned on an
// AddRoute error BEFORE recording anything, so the entry still named the metric
// it had just removed the route from: the kernel held no default route at
// either metric, and the next opposite event read the stale metric and returned
// having done nothing.
func TestAFailedMetricMoveLeavesTheRouteRecoverable(t *testing.T) {
	newEntry := func(t *testing.T, state routeMetricState) (*fakeBackend, map[dhcpUnitKey]dhcpEntry, dhcpUnitKey) {
		t.Helper()
		factory := &recordingDHCPFactory{}
		factory.install(t)
		fb := setupFakeBackendForTest(t)
		active := map[dhcpUnitKey]dhcpEntry{}
		reconcileDHCP(dhcpUnitJSON(t, ""), nil, active, slog.Default())
		key := dhcpUnitKey{ifaceName: "eth0", unit: "0"}
		entry := active[key]
		entry.gateway = "192.0.2.1"
		entry.metricState = state
		active[key] = entry
		return fb, active, key
	}
	const base = defaultLearnedRouteMetric
	down := base + deprioritizedMetric

	t.Run("a failed deprioritize is repaired by the up event", func(t *testing.T) {
		fb, active, key := newEntry(t, routeMetricBase)

		fb.addRouteErr = errors.New("netlink: operation not permitted")
		handleLinkDown("eth0", active, slog.Default())
		require.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", base, rtproto.Iface}, fb.routeRemoves[0],
			"the base-metric route was taken away before the add failed")
		assert.Equal(t, routeMetricUnknown, active[key].metricState,
			"the kernel holds this route at neither metric, so the entry must claim neither")

		fb.addRouteErr = nil
		handleLinkUp("eth0", active, slog.Default())
		require.Len(t, fb.routeAdds, 2, "the up event must reinstall the route the failed deprioritize took away")
		assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", base, rtproto.Iface}, fb.routeAdds[1])
		assert.Equal(t, routeMetricBase, active[key].metricState)
	})

	t.Run("a failed restore is repaired by the down event", func(t *testing.T) {
		fb, active, key := newEntry(t, routeMetricDeprioritized)

		fb.addRouteErr = errors.New("netlink: operation not permitted")
		handleLinkUp("eth0", active, slog.Default())
		require.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", down, rtproto.Iface}, fb.routeRemoves[0])
		assert.Equal(t, routeMetricUnknown, active[key].metricState)

		fb.addRouteErr = nil
		handleLinkDown("eth0", active, slog.Default())
		require.Len(t, fb.routeAdds, 2, "the down event must reinstall the route the failed restore took away")
		assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", down, rtproto.Iface}, fb.routeAdds[1])
		assert.Equal(t, routeMetricDeprioritized, active[key].metricState)
	})
}

// TestAFailedIPv6MetricMoveLeavesTheRouteRecoverable is the IPv6 twin of the
// test above.
//
// VALIDATES: handleLinkDownIPv6 and handleLinkUpIPv6 record routeMetricUnknown
// when their AddRoute fails after the RemoveRoute landed, so the next carrier
// event puts the ::/0 back.
// PREVENTS: an IPv6 outage that lasts until a full router-lost then
// router-discovered cycle. Nothing else reinstalls this route: it is not tied
// to a DHCP lease, and handleRouterDiscovered skips a router it already tracks.
func TestAFailedIPv6MetricMoveLeavesTheRouteRecoverable(t *testing.T) {
	key := routerKey{ifaceName: "eth0", routerIP: "fe80::1"}
	const base = 5
	down := base + deprioritizedMetric

	t.Run("a failed deprioritize is repaired by the up event", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		routers := map[routerKey]routerEntry{key: {metric: base, metricState: routeMetricBase}}

		fb.addRouteErr = errors.New("netlink: network is down")
		handleLinkDownIPv6("eth0", routers, slog.Default())
		require.Equal(t, routeCall{"eth0", "::/0", "fe80::1", base, rtproto.Iface}, fb.routeRemoves[0])
		assert.Equal(t, routeMetricUnknown, routers[key].metricState)

		fb.addRouteErr = nil
		handleLinkUpIPv6("eth0", routers, slog.Default())
		require.Len(t, fb.routeAdds, 2, "the up event must reinstall the ::/0 the failed deprioritize took away")
		assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", base, rtproto.Iface}, fb.routeAdds[1])
		assert.Equal(t, routeMetricBase, routers[key].metricState)
	})

	t.Run("a failed restore is repaired by the down event", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		routers := map[routerKey]routerEntry{key: {metric: base, metricState: routeMetricDeprioritized}}

		fb.addRouteErr = errors.New("netlink: network is down")
		handleLinkUpIPv6("eth0", routers, slog.Default())
		require.Equal(t, routeCall{"eth0", "::/0", "fe80::1", down, rtproto.Iface}, fb.routeRemoves[0])
		assert.Equal(t, routeMetricUnknown, routers[key].metricState)

		fb.addRouteErr = nil
		handleLinkDownIPv6("eth0", routers, slog.Default())
		require.Len(t, fb.routeAdds, 2, "the down event must reinstall the ::/0 the failed restore took away")
		assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", down, rtproto.Iface}, fb.routeAdds[1])
		assert.Equal(t, routeMetricDeprioritized, routers[key].metricState)
	})
}

// failingEventBus refuses every emission. It is the sysctl component being
// absent, or its handler returning an error: the two ways a sysctl ze asks for
// never reaches the kernel.
type failingEventBus struct {
	emissions int
}

func (b *failingEventBus) Emit(_, _ string, _ any) (int, error) {
	b.emissions++
	return 0, errors.New("sysctl: no subscriber")
}

func (b *failingEventBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// TestFailedRestoreKeepsTheInterfaceSuppressedAndRouted proves what ze holds
// after it fails to hand the IPv6 default routes back to the kernel.
//
// VALIDATES: restoreAcceptRaDefrtr emits before it removes anything, and keeps
// the interface in suppressed when that emission fails. The interface therefore
// still has ze's ::/0 while accept_ra_defrtr is still 0, and the next
// suppressRAForConfig retries the restore.
// PREVENTS: the IPv6 outage this spec has now met twice. The removal ran first
// and the interface was deleted from suppressed whatever the emission did, so a
// failed restore left the kernel suppressed, ze's route gone, and no record that
// anything was ever suppressed: no reconcile could retry, and the interface had
// no IPv6 default route until a reboot.
func TestFailedRestoreKeepsTheInterfaceSuppressedAndRouted(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	eb := &failingEventBus{}
	suppressed := map[string]bool{"eth0": true}
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 100, metricState: routeMetricBase},
	}

	restoreAcceptRaDefrtr("eth0", suppressed, routers, eb, slog.Default())

	require.Equal(t, 1, eb.emissions, "the restore must be attempted")
	assert.Empty(t, fb.routeRemoves,
		"the kernel is still suppressed, so ze's ::/0 is the only IPv6 default route left and MUST stay")
	assert.Len(t, routers, 1, "and ze must keep tracking the route it still owns")
	require.True(t, suppressed["eth0"],
		"forgetting the interface leaves accept_ra_defrtr at 0 with nothing that would ever restore it")

	// The retry: suppressRAForConfig republishes from a config that no longer
	// writes route-priority, so the interface is still due a restore.
	cfg := mustParseIfaceJSON(t, `{"interface": {"ethernet": {"eth0": {"unit": {"0": {}}}}}}`)
	priorities := map[string]int{}
	suppressRAForConfig(cfg, suppressed, routers, priorities, eb, slog.Default())
	assert.Equal(t, 2, eb.emissions, "the next reconcile must retry the restore")
	assert.True(t, suppressed["eth0"], "and the second failure keeps the interface for a third attempt")
}

// TestRepeatedLinkEventsMoveTheIPv6RouteOnce is the IPv6 twin of the test above,
// and it also proves a router-lost event names the metric the route is at.
//
// VALIDATES: handleLinkDownIPv6 and handleLinkUpIPv6 are no-ops once the route
// is at the metric they install, and handleRouterLost removes the route at the
// metric a link handler last put it at.
// PREVENTS: a router lost while the link is down leaking its ::/0 route. The
// remove named entry.metric while the route sat at entry.metric + 1024, so it
// matched nothing and the kernel kept forwarding on a route ze had dropped.
func TestRepeatedLinkEventsMoveTheIPv6RouteOnce(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	key := routerKey{ifaceName: "eth0", routerIP: "fe80::1"}
	routers := map[routerKey]routerEntry{key: {metric: 5, metricState: routeMetricBase}}

	handleLinkDownIPv6("eth0", routers, slog.Default())
	handleLinkDownIPv6("eth0", routers, slog.Default())
	require.Len(t, fb.routeRemoves, 1, "the second down event moved a route that had already moved")
	require.Len(t, fb.routeAdds, 1, "the second down event reinstalled a route that was already there")

	handleRouterLost(`{"name":"eth0","router-ip":"fe80::1"}`, routers, slog.Default())
	require.Len(t, fb.routeRemoves, 2)
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5 + deprioritizedMetric, rtproto.Iface}, fb.routeRemoves[1],
		"the router-lost remove must name the metric the route is installed at")
}

// TestRAOwnershipStillNeedsAWrittenRoutePriority proves the non-zero default
// did not hand ze the IPv6 default routes of every configured interface.
//
// VALIDATES: `route-priority` written above 0 is still what makes ze set
// accept_ra_defrtr=0 and install ::/0 itself. An RA route never landed at
// metric 0, so it carries none of the collision this fix removes.
// PREVENTS: a unit that says nothing about route-priority suppressing the
// kernel's own RA default route, which changes who owns IPv6 forwarding on
// every box that upgrades.
func TestRAOwnershipStillNeedsAWrittenRoutePriority(t *testing.T) {
	factory := &recordingDHCPFactory{}
	factory.install(t)
	fb := setupFakeBackendForTest(t)

	active := map[dhcpUnitKey]dhcpEntry{}
	cfg := dhcpUnitJSON(t, "")
	reconcileDHCP(cfg, nil, active, slog.Default())

	suppressed := make(map[string]bool)
	routers := make(map[routerKey]routerEntry)
	priorities := map[string]int{}
	suppressRAForConfig(cfg, suppressed, routers, priorities, newRecordingEventBus(), slog.Default())

	assert.Empty(t, suppressed, "accept_ra_defrtr stays as the kernel set it")
	assert.Empty(t, priorities,
		"an unwritten leaf must read as 'the kernel owns the RA default routes'")

	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::1"}`, routers, priorities, slog.Default())
	assert.Empty(t, routers, "no router is tracked while the kernel owns the RA routes")
	assert.Empty(t, fb.routeAdds, "and no ::/0 route is installed")
}

// slaacUnitJSON builds one ethernet unit with a static IPv6 address, a written
// route-priority and NO DHCP of either family. That is the config a SLAAC or
// static-IPv6 deployment writes when it wants ze to rank the default route the
// RAs on that link advertise.
func slaacUnitJSON(t *testing.T) *ifaceConfig {
	t.Helper()
	return mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"route-priority": "100",
							"ipv6": {"address": ["2001:db8::1/64"]}
						}
					}
				}
			}
		}
	}`)
}

// TestSLAACUnitWithRoutePriorityKeepsAnIPv6DefaultRoute proves that writing
// route-priority on a unit with no DHCP still leaves the interface with a
// usable IPv6 default route.
//
// VALIDATES: suppression and installation read ONE map (writtenRoutePriorities,
// published by suppressRAForConfig and read by handleRouterDiscovered), so ze
// either takes the kernel's RA default route AND installs its own, or does
// neither. The operator who writes the leaf is asking ze to own that route.
// PREVENTS: the two derivations that disagreed. The suppression read the config
// while the metric read the running DHCP clients, and reconcileDHCP starts no
// client for a unit that enables neither DHCPv4 nor DHCPv6 (register.go,
// collectDHCPUnits skips !v4 && !v6). A SLAAC-only unit carrying route-priority
// got accept_ra_defrtr=0, cleanupStaleIPv6DefaultRoutes deleted the kernel's RA
// default, and handleRouterDiscovered returned at metric <= 0: the interface
// ended with no IPv6 default route at all.
func TestSLAACUnitWithRoutePriorityKeepsAnIPv6DefaultRoute(t *testing.T) {
	factory := &recordingDHCPFactory{}
	factory.install(t)
	fb := setupFakeBackendForTest(t)

	cfg := slaacUnitJSON(t)
	active := map[dhcpUnitKey]dhcpEntry{}
	reconcileDHCP(cfg, nil, active, slog.Default())
	require.Empty(t, active,
		"the unit enables no DHCP, so no client runs: this is what the metric used to be read from")

	suppressed := make(map[string]bool)
	routers := make(map[routerKey]routerEntry)
	priorities := map[string]int{}
	suppressRAForConfig(cfg, suppressed, routers, priorities, &testEventBus{}, slog.Default())

	require.True(t, suppressed["eth0"],
		"a written route-priority is ze taking the IPv6 default route from the kernel")

	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::1"}`, routers, priorities, slog.Default())

	require.Len(t, fb.routeAdds, 1,
		"ze suppressed the kernel's RA default route, so it MUST install one of its own")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 100, rtproto.Iface}, fb.routeAdds[0])
	assert.Contains(t, routers, routerKey{ifaceName: "eth0", routerIP: "fe80::1"})
}

// pppoeTestSession is the session a fake dialer hands back: enough for
// runSession to reach the default-route install.
func pppoeTestSession(done <-chan struct{}) PPPoESession {
	return PPPoESession{
		SessionID: 7,
		UnitNum:   0,
		NegMTU:    1492,
		LocalIP:   netip.MustParseAddr("192.0.2.42"),
		PeerIP:    netip.MustParseAddr("192.0.2.1"),
		Done:      done,
		Cleanup:   func() {},
	}
}

// routeRecorder is a Backend that records AddRoute. It carries its own mutex
// because the PPPoE client installs its route from the session goroutine.
type routeRecorder struct {
	Backend
	mu   sync.Mutex
	adds []routeCall
}

func (r *routeRecorder) SetMTU(_ string, _ int) error       { return nil }
func (r *routeRecorder) SetAdminUp(_ string) error          { return nil }
func (r *routeRecorder) AddAddressP2P(_, _, _ string) error { return nil }
func (r *routeRecorder) RemoveRoute(_, _, _ string, _ int, _ rtproto.Proto) error {
	return nil
}

func (r *routeRecorder) AddRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adds = append(r.adds, routeCall{ifaceName, destCIDR, gateway, metric, proto})
	return nil
}

func (r *routeRecorder) waitForAdd(t *testing.T) routeCall {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		got := len(r.adds)
		var call routeCall
		if got > 0 {
			call = r.adds[0]
		}
		r.mu.Unlock()
		if got > 0 {
			return call
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the PPPoE session installed no default route")
	return routeCall{}
}

func pppoeClientJSON(t *testing.T, extra string) *ifaceConfig {
	t.Helper()
	return mustParseIfaceJSON(t, `{
		"interface": {
			"pppoe-client": {
				"pppoe0": {
					"source-interface": "eth0",`+extra+`
					"authentication": {"username": "user", "password": "pass"}
				}
			}
		}
	}`)
}

// TestPPPoERoutePriorityBounds walks the edges of the pppoe-client
// route-priority leaf.
//
// VALIDATES: the parser accepts the whole range its YANG leaf declares and
// refuses everything outside it, so a metric that reaches the kernel is one the
// netlink attribute can carry after the link-down 1024 is added to it.
// PREVENTS: a metric accepted here and truncated at the netlink boundary, where
// the route lands at the kernel's default instead of where the operator put it.
func TestPPPoERoutePriorityBounds(t *testing.T) {
	accepted := []struct {
		value string
		want  int
	}{
		{"0", 0},
		{"1", 1},
		{"4294966271", 4294966271}, // the last valid value
	}
	for _, tc := range accepted {
		cfg := pppoeClientJSON(t, `"route-priority": "`+tc.value+`",`)
		require.Len(t, cfg.PPPoEClient, 1, "route-priority %s must be accepted", tc.value)
		assert.Equal(t, tc.want, cfg.PPPoEClient[0].RoutePriority)
	}

	for _, value := range []string{"4294966272", "4294967296", "-1", "boot"} {
		_, err := parsePPPoEClientEntry("pppoe0", map[string]any{
			"source-interface": "eth0",
			"authentication":   map[string]any{"username": "user", "password": "pass"},
			"route-priority":   value,
		})
		require.Error(t, err, "route-priority %q must be refused", value)
		assert.Contains(t, err.Error(), "route-priority")
	}
}

// unitRoutePriorityJSON builds one ethernet unit carrying the route-priority
// value under test. The value is interpolated raw so a test can drive the
// parser with something the schema layer would have refused.
func unitRoutePriorityJSON(value string) string {
	return `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"route-priority": "` + value + `"
						}
					}
				}
			}
		}
	}`
}

// TestUnitRoutePriorityBounds walks the edges of the unit route-priority leaf.
//
// VALIDATES: parseUnits accepts the whole range its YANG leaf declares and
// refuses everything outside it, exactly as parsePPPoEClientEntry does for the
// pppoe-client leaf. The two leaves carry the same number to the same kernel
// attribute, so one parser rejecting what the other accepts is a hole rather
// than a difference.
// PREVENTS: the silently-landed collision metric. parseUnits read the leaf with
// `u.RoutePriority, _ = strconv.Atoi(rp)` and set RoutePrioritySet whatever came
// back, so any value the schema layer did not catch -- a negative, an overflow,
// a word -- put the learned route at metric 0. That is the metric a plain static
// route takes, and RouteReplace matching on destination, metric and table then
// overwrites the operator's static route with the learned one.
func TestUnitRoutePriorityBounds(t *testing.T) {
	accepted := []struct {
		value string
		want  int
	}{
		{"0", 0},
		{"1", 1},
		{"4294966271", 4294966271}, // the last valid value
	}
	for _, tc := range accepted {
		cfg := mustParseIfaceJSON(t, unitRoutePriorityJSON(tc.value))
		require.Len(t, cfg.Ethernet, 1, "route-priority %s must be accepted", tc.value)
		require.Len(t, cfg.Ethernet[0].Units, 1)
		assert.Equal(t, tc.want, cfg.Ethernet[0].Units[0].RoutePriority)
		assert.True(t, cfg.Ethernet[0].Units[0].RoutePrioritySet,
			"a written leaf is recorded as written")
	}

	// The message names the metric the value landed on, because that is the
	// defect: a value nobody refused reaches the kernel as 0 and collides with
	// the operator's static route.
	for _, value := range []string{"boot", "-1", "4294966272", "4294967296"} {
		cfg, err := parseIfaceConfig(unitRoutePriorityJSON(value))
		require.Errorf(t, err, "route-priority %q was accepted and put the learned route at metric %d",
			value, landedUnitRoutePriority(cfg))
		assert.Contains(t, err.Error(), "route-priority")
	}
}

// landedUnitRoutePriority reports the metric a parsed config puts on eth0 unit
// 0, or -1 when that unit is absent. A refusal that did not happen then reports
// the metric that would have reached the kernel.
func landedUnitRoutePriority(cfg *ifaceConfig) int {
	if cfg == nil || len(cfg.Ethernet) != 1 || len(cfg.Ethernet[0].Units) != 1 {
		return -1
	}
	return cfg.Ethernet[0].Units[0].RoutePriority
}

// TestPPPoEDefaultRouteTakesTheLearnedMetric proves a PPPoE session installs
// its default route at the learned metric, and that route-priority reaches it.
//
// VALIDATES: a PPPoE default route is ranked like a DHCP one. Before this,
// pppoe_client.go passed a literal 0 to AddRoute and PPPoEClientConfig had no
// priority field, so no configuration could move it off metric 0.
// PREVENTS: a PPPoE session overwriting an operator's static default route the
// moment IPCP completes.
func TestPPPoEDefaultRouteTakesTheLearnedMetric(t *testing.T) {
	cases := []struct {
		name  string
		extra string
		want  int
	}{
		{"no route-priority takes the learned metric", "", 254},
		{"an explicit 0 restores the kernel metric", `"route-priority": "0",`, 0},
		{"a written priority still wins", `"route-priority": "5",`, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := pppoeClientJSON(t, tc.extra)
			require.Len(t, cfg.PPPoEClient, 1)
			require.Equal(t, tc.want, cfg.PPPoEClient[0].RoutePriority)

			// Cleanups run last-registered-first, so the clients stop before
			// the session ends. The other order makes each client see a lost
			// session and start reconnecting while the test is tearing down.
			done := make(chan struct{})
			t.Cleanup(func() { close(done) })
			previous := pppoeDialerVar
			SetPPPoEDialer(&fakeDialer{sess: pppoeTestSession(done)})
			t.Cleanup(func() { pppoeDialerVar = previous })

			recorder := &routeRecorder{}
			activeClients := map[string]*PPPoEClient{}
			reconcilePPPoEClients(cfg, activeClients, recorder, slog.Default())
			t.Cleanup(func() {
				for _, c := range activeClients {
					c.Stop()
				}
			})

			call := recorder.waitForAdd(t)
			assert.Equal(t, routeCall{"ppp0", "0.0.0.0/0", "192.0.2.1", tc.want, rtproto.Iface}, call)
		})
	}
}

// TestRoutePriorityYANGDefaultMatchesTheConstant keeps the schema an operator
// reads and the value the parser applies on one number.
//
// VALIDATES: both route-priority leaves declare defaultLearnedRouteMetric.
// PREVENTS: the schema, `ze schema`, and the CLI completion advertising one
// metric while the parser installs another. The config tree delivers only what
// the operator wrote, so the YANG default is documentation the code has to
// repeat, and nothing but this test makes the two agree.
func TestRoutePriorityYANGDefaultMatchesTheConstant(t *testing.T) {
	source, err := os.ReadFile("yang/ze-iface-conf.yang")
	require.NoError(t, err)

	// One level of nesting: the leaf body holds a `type uint32 { range ... }`.
	leaf := regexp.MustCompile(`(?s)leaf route-priority \{(?:[^{}]|\{[^{}]*\})*\}`)
	blocks := leaf.FindAllString(string(source), -1)
	require.Len(t, blocks, 2, "one route-priority leaf on a unit, one on a pppoe-client")

	value := regexp.MustCompile(`default (\d+);`)
	for _, block := range blocks {
		m := value.FindStringSubmatch(block)
		require.NotNil(t, m, "leaf declares no default: %s", strings.TrimSpace(block))
		got, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		assert.Equal(t, defaultLearnedRouteMetric, got)
	}
}

// TestCoalescedUpRestoresBaseMetric drives the whole hand-off, from the push an
// event bus handler makes to the route the kernel is asked for.
//
// VALIDATES: AC-1 -- transitions arriving while the worker is blocked on the
// config-apply lock reach applyLinkEvent as ONE entry carrying the state the
// interface ENDED in, and the default route ends at the metric that state
// names. The superseded transitions cost no route move at all.
// PREVENTS: the loss this spec exists for, at the layer an operator sees it: an
// UP dropped after an applied DOWN left the route at base + 1024 with the link
// up, and handleLinkUp is idempotent by routeMetricState, so no later event
// repaired it. Both cases below fail against that design: the first ends
// deprioritized with a route move it should never have made, and the second
// ends at the base metric having moved nothing.
func TestCoalescedUpRestoresBaseMetric(t *testing.T) {
	// blockedRun replays transitions into a queue whose worker is stuck behind
	// the lock a config apply holds, then releases it and drains.
	blockedRun := func(t *testing.T, active map[dhcpUnitKey]dhcpEntry, transitions []bool) {
		t.Helper()
		var dhcpMu sync.Mutex
		dhcpMu.Lock()

		routers := map[routerKey]routerEntry{}
		priorities := map[string]int{}
		q := newLinkEventQueue(slog.Default())
		q.start(func(k linkEventKey, v linkEventValue) {
			dhcpMu.Lock()
			defer dhcpMu.Unlock()
			applyLinkEvent(k, v, active, routers, priorities, slog.Default())
		})
		for _, up := range transitions {
			q.pushCarrier("eth0", up)
		}
		dhcpMu.Unlock() // the commit finishes
		q.stop()
	}

	newActive := func(t *testing.T, state routeMetricState) (map[dhcpUnitKey]dhcpEntry, dhcpUnitKey) {
		t.Helper()
		factory := &recordingDHCPFactory{}
		factory.install(t)
		active := map[dhcpUnitKey]dhcpEntry{}
		reconcileDHCP(dhcpUnitJSON(t, ""), nil, active, slog.Default())
		key := dhcpUnitKey{ifaceName: "eth0", unit: "0"}
		entry := active[key]
		entry.gateway = "192.0.2.1"
		entry.metricState = state
		active[key] = entry
		return active, key
	}

	t.Run("a down superseded by an up leaves the route where it was", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		active, key := newActive(t, routeMetricBase)

		blockedRun(t, active, []bool{false, true})

		assert.Equal(t, routeMetricBase, active[key].metricState,
			"UP is the state the interface ended in, and the route is already at the base metric")
		assert.Empty(t, fb.routeAdds, "the superseded DOWN must never reach the kernel")
		assert.Empty(t, fb.routeRemoves)
	})

	t.Run("the last state of the burst is the one the kernel is asked for", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		active, key := newActive(t, routeMetricBase)

		blockedRun(t, active, []bool{false, true, false})

		assert.Equal(t, routeMetricDeprioritized, active[key].metricState)
		require.Len(t, fb.routeAdds, 1, "a burst that ends DOWN is one route move, not three")
		assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", 254 + deprioritizedMetric, rtproto.Iface}, fb.routeAdds[0])
		require.Len(t, fb.routeRemoves, 1)
		assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", 254, rtproto.Iface}, fb.routeRemoves[0])
	})

	t.Run("an up after an applied down restores the base metric", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		active, key := newActive(t, routeMetricDeprioritized)

		blockedRun(t, active, []bool{false, true})

		assert.Equal(t, routeMetricBase, active[key].metricState,
			"the UP that arrived while the worker was blocked is what brings the route back")
		require.Len(t, fb.routeAdds, 1)
		assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", 254, rtproto.Iface}, fb.routeAdds[0])
	})
}
