package flowexport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: the `show flow export` RPC is registered with a handler, so the
// command is reachable rather than 404ing at dispatch time.
func TestShowFlowExportRPCRegistered(t *testing.T) {
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:flow-export" {
			assert.NotNil(t, r.Handler, "ze-show:flow-export must have a handler")
			return
		}
	}
	require.Fail(t, "ze-show:flow-export RPC is not registered")
}

// VALIDATES: with no exporter configured the handler reports not-configured
// rather than panicking on a nil exporter.
func TestShowFlowExportNotConfigured(t *testing.T) {
	resp, err := handleShowFlowExport(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// VALIDATES: the `show flow recent` RPC is registered with a handler, so the
// recent-flow query is reachable at dispatch time (Wiring Test row).
func TestShowFlowRecentRPCRegistered(t *testing.T) {
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if r.WireMethod == "ze-show:flow-recent" {
			assert.NotNil(t, r.Handler, "ze-show:flow-recent must have a handler")
			return
		}
	}
	require.Fail(t, "ze-show:flow-recent RPC is not registered")
}

// VALIDATES: with no exporter configured the handler reports not-configured
// rather than panicking on a nil exporter.
func TestShowFlowRecentNotConfigured(t *testing.T) {
	resp, err := handleShowFlowRecent(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// VALIDATES: the handler rejects malformed argument grammar rather than
// silently returning all flows.
func TestShowFlowRecentUsage(t *testing.T) {
	// A live exporter is required to reach the arg parsing (nil exporter
	// short-circuits to not-configured), so drive parseDstPrefix directly.
	if _, ok := parseDstPrefix("203.0.113.42"); !ok {
		t.Error("bare address 203.0.113.42 should parse as a host prefix")
	}
	if _, ok := parseDstPrefix("203.0.113.0/24"); !ok {
		t.Error("CIDR 203.0.113.0/24 should parse")
	}
	if _, ok := parseDstPrefix("not-an-ip"); ok {
		t.Error("garbage should not parse")
	}
}
