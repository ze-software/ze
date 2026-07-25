// VALIDATES: the `show traffic-usage` RPC is registered with a handler, and the
// handler reports not-configured (rather than panicking) when the plugin is idle.
// PREVENTS: the command 404ing at dispatch; a nil-monitor panic on show.

package trafficusage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestShowTrafficUsageRegistered(t *testing.T) {
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:traffic-usage" {
			assert.NotNil(t, r.Handler, "ze-show:traffic-usage must have a handler")
			return
		}
	}
	require.Fail(t, "ze-show:traffic-usage RPC is not registered")
}

func TestShowTrafficUsageNotConfigured(t *testing.T) {
	activeMonitor.Store(nil)
	resp, err := handleShowTrafficUsage(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

func TestShowTrafficUsage(t *testing.T) {
	fa := newFakeAttacher()
	fa.result = counts{
		ingressPort: map[portProto]uint64{{port: 443, proto: 6}: 1000},
		egressPort:  map[portProto]uint64{{port: 53, proto: 17}: 500},
		ingressIP:   map[uint32]uint64{0x0100000a: 2000}, // 10.0.0.1
		mapEntries:  map[string]int{"port_ingress": 1},
	}
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, true, "eth0")})
	activeMonitor.Store(m)
	defer activeMonitor.Store(nil)

	// No args -> list of interface maps.
	resp, err := handleShowTrafficUsage(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	list, ok := resp.Data.(plugin.Slice[plugin.Map])
	require.True(t, ok, "data should be a slice of interface maps")
	require.Len(t, list, 1)
	assert.Equal(t, "eth0", list[0]["interface"])
	assert.NotNil(t, list[0]["ingress-ports"])
	assert.NotNil(t, list[0]["ingress-ips"]) // present because track-ip populated them

	// name filter -> single interface map.
	resp, err = handleShowTrafficUsage(nil, []string{"name", "eth0"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	one, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "eth0", one["interface"])

	// unknown interface -> error.
	resp, _ = handleShowTrafficUsage(nil, []string{"name", "missing"})
	assert.Equal(t, plugin.StatusError, resp.Status)
}
