// Design: docs/architecture/wire/capabilities.md — OPEN capability deduplication
// Related: session_negotiate.go — buildOpenFromSnapshot

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestBuildOpenDeduplicatesConfigAndPluginCapabilities verifies that
// buildOpenFromSnapshot deduplicates exact (code, value) capability TLVs across
// peer configuration and plugin declarations while keeping distinct values that
// share a capability code.
//
// VALIDATES: AC-43, S40, Q-E9(a) — Route Refresh (code 2) and identical
// ADD-PATH (code 69) declared by both config and plugins appear once in the
// OPEN message in first-seen order, while multiple Multiprotocol (code 1)
// families with distinct AFI/SAFI bytes are all preserved.
// PREVENTS: Duplicate Route Refresh or identical capability TLVs on the wire
// when both peer config and virtual-session plugins advertise the same
// capability, or over-broad per-code deduplication dropping distinct
// Multiprotocol families.
func TestBuildOpenDeduplicatesConfigAndPluginCapabilities(t *testing.T) {
	t.Parallel()

	s, settings := newNoFamilySession(t)
	configCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		&capability.RouteRefresh{},
		&capability.AddPath{
			Families: []capability.AddPathFamily{
				{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathSend},
			},
		},
	}

	plugins := pluginCapabilitySnapshot{
		capabilities: []capability.Capability{
			// Exact duplicate of config Route Refresh (code 2, empty value).
			capability.NewPlugin(uint8(capability.CodeRouteRefresh), nil),
			// Exact duplicate of another plugin entry (code 2, zero-length slice).
			capability.NewPlugin(uint8(capability.CodeRouteRefresh), []byte{}),
			// Exact duplicate of config ADD-PATH (IPv4 unicast send: 0001 01 02).
			capability.NewPlugin(uint8(capability.CodeAddPath), []byte{0x00, 0x01, 0x01, 0x02}),
			// Exact duplicate of config Multiprotocol IPv4 unicast.
			capability.NewPlugin(uint8(capability.CodeMultiprotocol), []byte{0x00, 0x01, 0x00, 0x01}),
			// Distinct Multiprotocol family (IPv4 FlowSpec: AFI=1, SAFI=133) must be kept.
			capability.NewPlugin(uint8(capability.CodeMultiprotocol), []byte{0x00, 0x01, 0x00, 0x85}),
			// Distinct plugin-only capability (Enhanced Route Refresh code 70) must be kept once.
			capability.NewPlugin(uint8(capability.CodeEnhancedRouteRefresh), nil),
			capability.NewPlugin(uint8(capability.CodeEnhancedRouteRefresh), nil),
		},
	}

	open, err := s.buildOpenFromSnapshot(settings, configCaps, plugins)
	require.NoError(t, err)

	got, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	require.NoError(t, err)

	// Expected order: MP(v4/uni), MP(v6/uni), ASN4, RouteRefresh, AddPath(v4/uni send),
	// MP(v4/flowspec), EnhancedRouteRefresh.
	require.Len(t, got, 7)

	var mpFamilies []capability.Family
	var rrCount, addPathCount, errCount int
	for _, c := range got {
		switch capVal := c.(type) {
		case *capability.Multiprotocol:
			mpFamilies = append(mpFamilies, capability.Family{AFI: capVal.AFI, SAFI: capVal.SAFI})
		case *capability.RouteRefresh:
			rrCount++
		case *capability.AddPath:
			addPathCount++
		case *capability.EnhancedRouteRefresh:
			errCount++
		}
	}

	assert.Equal(t, []capability.Family{
		{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		{AFI: capability.AFIIPv4, SAFI: capability.SAFIFlowSpec},
	}, mpFamilies, "distinct Multiprotocol families must be preserved in first-seen order without duplicating IPv4 unicast")
	assert.Equal(t, 1, rrCount, "Route Refresh declared by both config and plugin must emit once")
	assert.Equal(t, 1, addPathCount, "identical ADD-PATH declared by both config and plugin must emit once")
	assert.Equal(t, 1, errCount, "duplicate plugin capability must emit once")
}
