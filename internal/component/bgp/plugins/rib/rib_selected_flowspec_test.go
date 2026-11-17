// Design: docs/architecture/plugin/rib-storage-design.md -- selected FlowSpec authorization.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

func TestSelectedCaptureFlowSpecRedirectChangeGetsNewRevision(t *testing.T) {
	r, _ := flowValidationFixture(t)
	peer := netip.MustParseAddr("192.0.2.1")
	path := flowValidationPath(2, 65001)
	flowValidationReceive(t, r, peer, 65001, 1, family.IPv4Unicast,
		[]byte{8, 10}, flowValidationAttrs(path, netip.Addr{}, 0, nil), false)
	raw := flowspecNLRI(16, 10, 1)
	communities := []byte{0x80, 6, 0, 0, 0, 0, 0, 0, 0x08, 0, 0, 0, 0, 0, 0, 0}
	flowValidationReceiveNextHop(t, r, peer, 65001, 3, flowspecFamily, raw,
		flowValidationAttrs(path, netip.Addr{}, 100, communities), false, []byte{198, 51, 100, 1})
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	var before *advertiseRoute
	for i := range first {
		if first[i].key.family == flowspecFamily {
			before = &first[i]
		}
	}
	require.NotNil(t, before, "a useful eligible FlowSpec rule must be captured first")
	require.True(t, before.redirectRefused, "the rule has no route authorizing its redirect target")
	cfg, err := parseFlowSpecRedirectConfig(map[string]any{"flowspec-redirect-check": map[string]any{"enabled": "false"}})
	require.NoError(t, err)
	r.flowSpecRedirectCfg.Store(&cfg)
	r.reconcileFlowSpecs()
	after, err := r.advertiseSnapshot()
	require.NoError(t, err)
	var replacement *advertiseRoute
	for i := range after {
		if after[i].key == before.key {
			replacement = &after[i]
		}
	}
	require.NotNil(t, replacement)
	require.False(t, replacement.redirectRefused)
	require.Greater(t, replacement.revision, before.revision)
	require.Equal(t, before.route.MessageID, replacement.route.MessageID)
	require.Equal(t, before.route.Attributes, replacement.route.Attributes,
		"authorization changes the selected revision without changing received route bytes")
}
