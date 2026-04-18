package traffic

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// swapBackendGateSchema replaces the cached gate schema with the supplied
// Schema. The sync.Once guarding the lazy YANG load is reset to a fresh
// zero value and then marked Done so validateBackendGate bypasses loading
// and uses the override. The t.Cleanup clears the cache so subsequent tests
// (or production callers) re-load the real YANG schema.
//
// NOT safe for t.Parallel(): mutates package-level state (backendGateSchema,
// backendGateSchemaErr, backendGateSchemaOnce). Tests using this helper must
// remain serial.
func swapBackendGateSchema(t *testing.T, s *config.Schema) {
	t.Helper()
	backendGateSchema = s
	backendGateSchemaErr = nil
	backendGateSchemaOnce = sync.Once{}
	backendGateSchemaOnce.Do(func() {})
	t.Cleanup(func() {
		backendGateSchema = nil
		backendGateSchemaErr = nil
		backendGateSchemaOnce = sync.Once{}
	})
}

// VALIDATES: init() registers the traffic plugin under Name="traffic" with
//
//	ConfigRoots=["traffic-control"] so the engine dispatches the
//	right config section to the plugin's runEngine.
//
// PREVENTS: A rename drift that stops the reactor from ever receiving the
//
//	traffic-control section. A broken ConfigRoots entry would
//	silently leave the traffic-control block parsed but unhandled.
func TestTrafficPluginRegistered(t *testing.T) {
	reg := registry.Lookup("traffic")
	require.NotNil(t, reg, "traffic plugin must be registered under Name=\"traffic\"")
	assert.Contains(t, reg.ConfigRoots, configRootTraffic,
		"traffic plugin must list %q in ConfigRoots", configRootTraffic)
	assert.NotNil(t, reg.RunEngine, "traffic plugin must have RunEngine")
	assert.NotEmpty(t, reg.YANG, "traffic plugin must ship its YANG schema")
}

// VALIDATES: OnConfigVerify rejects a traffic-control section when the active
//
//	backend is "" (non-Linux default). The walker's empty-backend
//	guard fires even though no YANG annotations exist today.
//
// PREVENTS: A non-Linux deployment silently installing a traffic-control
//
//	config that could never be programmed, with a runtime Apply
//	failure surfacing only after commit.
func TestTrafficBackendGateRejects_EmptyBackend(t *testing.T) {
	// Force-load the real schema via validateBackendGate so the cache has a
	// known-good value, then overwrite backend to "".
	data := `{"traffic-control":{"interface":{"eth0":{"name":"eth0"}}}}`
	sections := []sdk.ConfigSection{{Root: configRootTraffic, Data: data}}

	err := validateBackendGate(sections, "")
	require.Error(t, err, "empty backend must be rejected")
	assert.Contains(t, err.Error(), backendLeafPath,
		"rejection must name /traffic-control/backend so operators know where to set it")
}

// VALIDATES: OnConfigVerify returns the aggregated error when a node carries
//
//	a ze:backend annotation that excludes the active backend.
//	Uses a synthetic schema with the `interface` list annotated
//	`ze:backend "tc"` and the active backend set to `"vpp"`.
//
// PREVENTS: Silent drift that disables the backend feature gate -- if the
//
//	wiring regressed, an annotated node would no longer produce
//	a diagnostic and this test would catch it.
func TestTrafficBackendGateRejects_Synthetic(t *testing.T) {
	list := config.List(config.TypeString,
		config.Field("name", config.Leaf(config.TypeString)),
	)
	list.KeyName = "name"
	list.Backend = []string{"tc"}

	synthetic := config.NewSchema()
	synthetic.Define(configRootTraffic, config.Container(
		config.Field("backend", config.Leaf(config.TypeString)),
		config.Field("interface", list),
	))

	swapBackendGateSchema(t, synthetic)

	data := `{"traffic-control":{"backend":"vpp","interface":{"eth0":{"name":"eth0"}}}}`
	sections := []sdk.ConfigSection{{Root: configRootTraffic, Data: data}}

	err := validateBackendGate(sections, "vpp")
	require.Error(t, err, "synthetic schema must reject vpp backend on tc-annotated list")
	msg := err.Error()
	assert.True(t, strings.Contains(msg, "/traffic-control/interface"),
		"rejection must name the /traffic-control/interface YANG path, got: %s", msg)
	assert.Contains(t, msg, `"vpp"`, "rejection must name the active backend")
	assert.Contains(t, msg, "tc", "rejection must list supporting backends")
}

// VALIDATES: AC-3 -- when the SDK delivers a payload with no traffic-control
//
//	section, parseTrafficSections returns the idle default
//	(backend=defaultBackendName, empty interfaces) and
//	hasTrafficSection returns false so OnConfigure can short-
//	circuit before LoadBackend.
//
// PREVENTS: regression where the reactor loads a backend and calls Apply for
//
//	configs that never mention traffic-control -- wasteful at best,
//	incorrect when the OS has no default backend.
func TestTrafficPayloadWithoutSection_IsIdle(t *testing.T) {
	sections := []sdk.ConfigSection{
		{Root: "bgp", Data: `{"bgp":{}}`},
		{Root: "interface", Data: `{"interface":{}}`},
	}
	assert.False(t, hasTrafficSection(sections),
		"payload without traffic-control must be idle")

	cfg, err := parseTrafficSections(sections)
	require.NoError(t, err, "idle payload must not produce a parse error")
	assert.Equal(t, defaultBackendName, cfg.Backend,
		"idle path must fall back to the OS default backend (never loaded)")
	assert.Empty(t, cfg.Interfaces,
		"idle path must produce an empty interface map")
}
