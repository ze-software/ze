package traffic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: init() registers the traffic plugin under Name="traffic" with
//
//	ConfigRoots=["traffic/control"] so the engine dispatches the
//	right config section to the plugin's runEngine.
//
// PREVENTS: A rename drift that stops the reactor from ever receiving the
//
//	traffic/control section. A broken ConfigRoots entry would
//	silently leave the traffic/control block parsed but unhandled.
func TestTrafficPluginRegistered(t *testing.T) {
	reg := registry.Lookup("traffic")
	require.NotNil(t, reg, "traffic plugin must be registered under Name=\"traffic\"")
	assert.Contains(t, reg.ConfigRoots, configRootTraffic,
		"traffic plugin must list %q in ConfigRoots", configRootTraffic)
	assert.NotNil(t, reg.RunEngine, "traffic plugin must have RunEngine")
	assert.NotEmpty(t, reg.YANG, "traffic plugin must ship its YANG schema")
}

// VALIDATES: AC-3 -- when the SDK delivers a payload with no traffic/control
//
//	section, parseTrafficSections returns the idle default
//	(backend=defaultBackendName, empty interfaces) and
//	hasTrafficSection returns false so OnConfigure can short-
//	circuit before LoadBackend.
//
// PREVENTS: regression where the reactor loads a backend and calls Apply for
//
//	configs that never mention traffic/control -- wasteful at best,
//	incorrect when the OS has no default backend.
func TestTrafficPayloadWithoutSection_IsIdle(t *testing.T) {
	sections := []sdk.ConfigSection{
		{Root: "bgp", Data: `{"bgp":{}}`},
		{Root: "interface", Data: `{"interface":{}}`},
	}
	assert.False(t, hasTrafficSection(sections),
		"payload without traffic/control must be idle")

	cfg, err := parseTrafficSections(sections)
	require.NoError(t, err, "idle payload must not produce a parse error")
	assert.Equal(t, defaultBackendName, cfg.Backend,
		"idle path must fall back to the OS default backend (never loaded)")
	assert.Empty(t, cfg.Interfaces,
		"idle path must produce an empty interface map")
}
