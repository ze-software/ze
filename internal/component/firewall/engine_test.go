// Design: docs/architecture/core-design.md -- Firewall plugin engine (SDK 5-stage)
// Related: engine.go -- validateBackendGate under test

package firewall

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// swapBackendGateSchema replaces the cached gate schema with the supplied
// Schema. The sync.Once guarding the lazy YANG load is reset and then marked
// Done so validateBackendGate bypasses the real load and uses the override.
// t.Cleanup clears the cache so subsequent tests re-load the real YANG.
//
// NOT safe for t.Parallel(): mutates package-level state (backendGateSchema,
// backendGateSchemaErr, backendGateSchemaOnce).
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

// VALIDATES: validateBackendGate rejects a firewall section when the active
// backend is "" (the non-Linux default). The walker's empty-backend guard
// fires regardless of YANG annotations and the rejection names the backend
// leaf path so operators know where to set it.
//
// PREVENTS: A non-Linux deployment silently accepting a firewall config that
// could never be programmed, with the Apply-time failure surfacing only after
// commit.
func TestFirewallBackendGateRejects_EmptyBackend(t *testing.T) {
	data := `{"firewall":{"table":{"t1":{"name":"t1","family":"ip"}}}}`
	sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: data}}

	err := validateBackendGate(sections, "")
	require.Error(t, err, "empty backend must be rejected")
	assert.Contains(t, err.Error(), backendLeafPath,
		"rejection must name /firewall/backend so operators know where to set it")
}

// VALIDATES: validateBackendGate aggregates diagnostics for every YANG node
// whose ze:backend annotation excludes the active backend. Synthetic schema
// annotates the `table` list `ze:backend "nft"` and the active backend is
// "vpp", so the walker emits a diagnostic naming the path and backends.
//
// PREVENTS: Wiring regression that disables the feature gate. If the walker
// were bypassed or the annotation stopped flowing onto the schema node, the
// annotated node would no longer produce a diagnostic and this test would
// catch it.
func TestFirewallBackendGateRejects_Synthetic(t *testing.T) {
	list := config.List(config.TypeString,
		config.Field("name", config.Leaf(config.TypeString)),
	)
	list.KeyName = "name"
	list.Backend = []string{"nft"}

	synthetic := config.NewSchema()
	synthetic.Define(configRootFirewall, config.Container(
		config.Field("backend", config.Leaf(config.TypeString)),
		config.Field("table", list),
	))

	swapBackendGateSchema(t, synthetic)

	data := `{"firewall":{"backend":"vpp","table":{"t1":{"name":"t1"}}}}`
	sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: data}}

	err := validateBackendGate(sections, "vpp")
	require.Error(t, err, "synthetic schema must reject vpp backend on nft-annotated list")
	msg := err.Error()
	assert.True(t, strings.Contains(msg, "/firewall/table"),
		"rejection must name the /firewall/table YANG path, got: %s", msg)
	assert.Contains(t, msg, `"vpp"`, "rejection must name the active backend")
	assert.Contains(t, msg, "nft", "rejection must list supporting backends")
}

// VALIDATES: validateBackendGate accepts a firewall section when the active
// backend matches every ze:backend annotation on the real schema. Today all
// seven annotations are `ze:backend "nft"`, so backend="nft" with a minimal
// firewall payload must return nil.
//
// PREVENTS: False positives if the walker ever learns to reject a config on
// an annotation that names the active backend (the "narrowest wins" and
// "absent annotation = unrestricted" rules must keep holding).
func TestFirewallBackendGateAccepts_NftBackend(t *testing.T) {
	data := `{"firewall":{"table":{"t1":{"name":"t1","family":"ip"}}}}`
	sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: data}}

	err := validateBackendGate(sections, "nft")
	require.NoError(t, err, "nft backend must satisfy every ze:backend \"nft\" annotation")
}
