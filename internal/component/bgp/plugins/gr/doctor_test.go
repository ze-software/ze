// Design: docs/guide/graceful-restart.md -- graceful restart capability configuration diagnostics
// Detail: doctor.go -- checkGRCapabilityConfig and its registration

package gr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorCheckGRCapabilityIsRegistered verifies that bgp-gr registers its
// post-config capability doctor check and diagnostic metadata.
func TestDoctorCheckGRCapabilityIsRegistered(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorBGPGRCapability),
		"doctor code metadata must be registered in init()")

	for _, check := range registry.PluginDoctorChecks() {
		if check.Name != "bgp-gr-capability" {
			continue
		}
		assert.Equal(t, grPluginName, check.PluginName)
		assert.Equal(t, []string{codeDoctorBGPGRCapability}, check.Codes)
		require.NotNil(t, check.Check)
		return
	}
	t.Fatal("bgp-gr registers no bgp-gr-capability doctor check")
}

// TestDoctorCheckGRCapabilitySilentOnValidConfig verifies that a valid
// symmetric short-lived/long-lived config produces no diagnostics.
func TestDoctorCheckGRCapabilitySilentOnValidConfig(t *testing.T) {
	text := `
bgp {
	router-id 1.2.3.4;
	session {
		asn { local 65000 }
	}
	peer p1 {
		connection { remote { ip 10.0.0.1 } local { ip 10.0.0.2 } }
		session {
			asn { remote 65001 }
			family { ipv4/unicast; ipv6/unicast; }
			capability {
				graceful-restart {
					short-lived { restart-time 120; }
					long-lived { family { ipv4/unicast 3600; } }
				}
			}
		}
	}
}
`
	tree, err := config.ParseTreeWithYANG(text, nil)
	require.NoError(t, err)
	assert.Empty(t, checkGRCapabilityConfig(registry.DoctorCheckContext{Tree: tree}))
	assert.Empty(t, checkGRCapabilityConfig(registry.DoctorCheckContext{}))
}

// TestDoctorCheckGRCapabilityReportsAnomalies verifies that clamped timers and
// invalid/unknown families in short-lived and long-lived config are reported.
func TestDoctorCheckGRCapabilityReportsAnomalies(t *testing.T) {
	bgpSubtree := map[string]any{
		"peer": map[string]any{
			"p1": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "10.0.0.1"},
				},
				"session": map[string]any{
					"family": map[string]any{"ipv4/unicast": map[string]any{}},
					"capability": map[string]any{
						"graceful-restart": map[string]any{
							"short-lived": map[string]any{
								"restart-time": float64(5000),
							},
							"long-lived": map[string]any{
								"family": map[string]any{
									"ipv4/unicast": map[string]any{"stale-time": float64(16777216)},
									"unknown/safi": map[string]any{"stale-time": float64(3600)},
									"ipv6/unicast": map[string]any{"stale-time": float64(-1)},
									"ipv4/flow":    map[string]any{"stale-time": "not-an-int"},
								},
							},
						},
					},
				},
			},
		},
	}

	issues := grCapabilityIssues(bgpSubtree)
	require.Len(t, issues, 5)
	assert.Contains(t, issues[0], "restart-time 5000 exceeds 12-bit max 4095")
	assert.Contains(t, issues[1], "non-integer stale-time")
	assert.Contains(t, issues[2], "stale-time 16777216 exceeds 24-bit max 16777215")
	assert.Contains(t, issues[3], "negative stale-time -1")
	assert.Contains(t, issues[4], "unknown family unknown/safi")
}
