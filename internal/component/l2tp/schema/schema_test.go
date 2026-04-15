package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/l2tp/schema"
)

// TestSchema_ZeL2TPModule verifies ze-l2tp-conf.yang loads and has the
// expected top-level structure.
//
// VALIDATES: AC-1 -- minimal l2tp config parses via YANG schema.
// PREVENTS: Missing L2TP subsystem in YANG schema registry.
func TestSchema_ZeL2TPModule(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-l2tp-conf")
	require.NotNil(t, mod, "ze-l2tp-conf module should exist")
	assert.Equal(t, "urn:ze:l2tp:conf", mod.Namespace.Name)
}

// TestSchema_ZeL2TPEntry verifies the YANG entry has expected children.
//
// Protocol settings (enabled, max-tunnels, hello-interval, shared-secret)
// live under root l2tp{}. Listener endpoints live under environment{l2tp{}}.
//
// VALIDATES: AC-1 -- l2tp config leaves exist in schema at correct paths.
// PREVENTS: Config fields missing from YANG, causing parse rejection.
func TestSchema_ZeL2TPEntry(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-l2tp-conf")
	require.NotNil(t, entry, "ze-l2tp-conf entry should exist")

	// Root-level l2tp{} has protocol settings.
	l2tpRoot := entry.Dir["l2tp"]
	require.NotNil(t, l2tpRoot, "root l2tp container should exist")

	rootLeaves := []string{"enabled", "max-tunnels", "hello-interval", "shared-secret"}
	for _, name := range rootLeaves {
		assert.NotNil(t, l2tpRoot.Dir[name], "root l2tp should have child %q", name)
	}

	// environment{l2tp{}} has listener endpoints only.
	environment := entry.Dir["environment"]
	require.NotNil(t, environment, "environment container should exist")

	l2tpEnv := environment.Dir["l2tp"]
	require.NotNil(t, l2tpEnv, "l2tp container should exist under environment")

	server := l2tpEnv.Dir["server"]
	require.NotNil(t, server, "l2tp.server list should exist under environment")
	assert.Equal(t, "name", server.Key, "server list key should be 'name'")
}
