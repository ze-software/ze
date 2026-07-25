// VALIDATES: parseLDPConfig handles the real config-delivery shape produced by
// config.Tree.ToMap + BuildPluginConfigSections: the subtree is wrapped under
// its root key ({"ldp": {...}}), every leaf is rendered as a JSON string (so
// numeric leaves arrive as "5", not 5), and a leaf-list renders as a bare scalar
// for one element and a []any for several. A parser reading the unwrapped /
// numeric / array shape silently produced an empty config, leaving the engine
// idle -- the defect this test pins down.
package ldp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestParseLDPConfigRealShape(t *testing.T) {
	// Exactly the JSON the engine receives at boot for a single-interface config.
	sections := []sdk.ConfigSection{{
		Root: "ldp",
		Data: `{"ldp":{"hello-hold-time":"15","hello-interval":"5","interfaces":"lo","keepalive-time":"60","lsr-id":"10.0.0.1","transport-address":"10.0.0.2"}}`,
	}}

	cfg, err := parseLDPConfig(sections)
	require.NoError(t, err)

	assert.True(t, cfg.LSRID.IsValid(), "lsr-id must parse from the wrapped tree")
	assert.Equal(t, "10.0.0.1", cfg.LSRID.String())
	assert.Equal(t, "10.0.0.2", cfg.TransportAddr.String())
	assert.Equal(t, 5*time.Second, cfg.HelloInterval, "string-typed numeric leaf must parse")
	assert.Equal(t, 15*time.Second, cfg.HelloHoldTime)
	assert.Equal(t, 60*time.Second, cfg.KeepaliveTime)
	assert.Equal(t, []string{"lo"}, cfg.Interfaces, "single leaf-list value is a scalar")
}

func TestParseLDPConfigMultiInterface(t *testing.T) {
	// A multi-element leaf-list renders as a JSON array.
	sections := []sdk.ConfigSection{{
		Root: "ldp",
		Data: `{"ldp":{"lsr-id":"10.0.0.1","interfaces":["eth0","eth1"]}}`,
	}}

	cfg, err := parseLDPConfig(sections)
	require.NoError(t, err)
	assert.Equal(t, []string{"eth0", "eth1"}, cfg.Interfaces)
}

func TestParseLDPConfigDefaultsAndEmpty(t *testing.T) {
	// No matching section: defaults stand and the engine has no lsr-id (idle).
	cfg, err := parseLDPConfig(nil)
	require.NoError(t, err)
	assert.False(t, cfg.LSRID.IsValid())
	assert.Equal(t, DefaultHelloInterval, cfg.HelloInterval)
	assert.Equal(t, DefaultHelloHoldTime, cfg.HelloHoldTime)
	assert.Equal(t, DefaultKeepaliveTime, cfg.KeepaliveTime)
}

func TestParseLDPConfigInvalidLSRID(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "ldp",
		Data: `{"ldp":{"lsr-id":"not-an-ip"}}`,
	}}
	_, err := parseLDPConfig(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid lsr-id")
}
