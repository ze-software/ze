package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"

	// Blank import triggers init() registration of the telemetry YANG module.
	_ "codeberg.org/thomas-mangin/ze/internal/component/telemetry/schema"
)

// TestSchema_ZeTelemetryModule verifies ze-telemetry-conf.yang content.
//
// VALIDATES: ze-telemetry-conf module defines expected namespace and containers.
// PREVENTS: Missing telemetry configuration elements.
func TestSchema_ZeTelemetryModule(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-telemetry-conf")
	require.NotNil(t, mod, "ze-telemetry-conf module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:telemetry:conf", mod.Namespace.Name)

	// Find telemetry container
	var telemetryContainer bool
	for _, c := range mod.Container {
		if c.Name == "telemetry" {
			telemetryContainer = true
			break
		}
	}
	assert.True(t, telemetryContainer, "telemetry container should exist")
}
