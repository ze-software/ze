// VALIDATES: the `show traffic-usage` RPC is registered with a handler, and the
// handler reports not-configured (rather than panicking) when the plugin is idle.
// PREVENTS: the command 404ing at dispatch; a nil-monitor panic on show.

package trafficusage

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestShowTrafficUsageRegistered(t *testing.T) {
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-trafficusage:show-traffic-usage" {
			assert.NotNil(t, r.Handler, "ze-trafficusage:show-traffic-usage must have a handler")
			return
		}
	}
	require.Fail(t, "ze-trafficusage:show-traffic-usage RPC is not registered")
}

func TestShowTrafficUsageNotConfigured(t *testing.T) {
	activeMonitor.Store(nil)
	resp, err := handleShowTrafficUsage(nil, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

func TestShowTrafficUsage(t *testing.T) {
	fa := newFakeAttacher()
	fa.result = counts{
		ingressPort: map[portProto]uint64{{port: 443, proto: 6}: 1000},
		egressPort:  map[portProto]uint64{{port: 53, proto: 17}: 500},
		ingressIP:   map[netip.Addr]uint64{netip.MustParseAddr("10.0.0.1"): 2000},
		mapEntries:  map[string]int{"port_ingress": 1},
	}
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, true, "eth0")})
	activeMonitor.Store(m)
	defer activeMonitor.Store(nil)

	// No args -> list of interface maps.
	resp, err := handleShowTrafficUsage(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	list, ok := resp.Data.(plugin.Slice[plugin.Map])
	require.True(t, ok, "data should be a slice of interface maps")
	require.Len(t, list, 1)
	assert.Equal(t, "eth0", list[0]["interface"])
	assert.NotNil(t, list[0]["ingress-ports"])
	assert.NotNil(t, list[0]["ingress-ips"]) // present because track-ip populated them

	// name filter -> single interface map.
	resp, err = handleShowTrafficUsage(nil, commandtest.Args("name", "eth0"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	one, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "eth0", one["interface"])

	// unknown interface -> error.
	resp, _ = handleShowTrafficUsage(nil, commandtest.Args("name", "missing"))
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// VALIDATES: AC-6 — show traffic usage renders canonical RFC 5952 IPv6
// addresses in ingress-ips / egress-ips, icmpv6 in ingress-ports, and
// ip6_ingress / ip6_egress in map-entries.
func TestShowTrafficUsageIPv6(t *testing.T) {
	fa := newFakeAttacher()
	fa.result = counts{
		ingressPort: map[portProto]uint64{{port: 0, proto: 58}: 640},
		egressPort:  map[portProto]uint64{{port: 53, proto: 17}: 320},
		ingressIP: map[netip.Addr]uint64{
			netip.MustParseAddr("10.0.0.1"):                                1000,
			netip.MustParseAddr("2001:0db8:0000:0000:0000:0000:0000:0001"): 4096,
		},
		egressIP: map[netip.Addr]uint64{
			netip.MustParseAddr("2001:0db8:0000:0000:0000:0000:0000:0002"): 2048,
		},
		mapEntries: map[string]int{
			"port_ingress": 1,
			"ip_ingress":   1,
			"ip6_ingress":  1,
			"ip6_egress":   1,
		},
	}
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, true, "eth0")})
	activeMonitor.Store(m)
	defer activeMonitor.Store(nil)

	resp, err := handleShowTrafficUsage(nil, commandtest.Args("name", "eth0"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	one, ok := resp.Data.(plugin.Map)
	require.True(t, ok)

	inPorts, ok := one["ingress-ports"].(plugin.Slice[plugin.Map])
	require.True(t, ok)
	assert.Contains(t, inPorts, plugin.Map{"port": 0, "protocol": "icmpv6", "bytes": uint64(640)})

	inIPs, ok := one["ingress-ips"].(plugin.Slice[plugin.Map])
	require.True(t, ok)
	assert.Contains(t, inIPs, plugin.Map{"ip": "10.0.0.1", "bytes": uint64(1000)})
	assert.Contains(t, inIPs, plugin.Map{"ip": "2001:db8::1", "bytes": uint64(4096)})

	outIPs, ok := one["egress-ips"].(plugin.Slice[plugin.Map])
	require.True(t, ok)
	assert.Contains(t, outIPs, plugin.Map{"ip": "2001:db8::2", "bytes": uint64(2048)})

	entries, ok := one["map-entries"].(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 1, entries["ip6_ingress"])
	assert.Equal(t, 1, entries["ip6_egress"])
}
