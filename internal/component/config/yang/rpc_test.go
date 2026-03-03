package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWireModule verifies YANG module name to wire method prefix conversion.
//
// VALIDATES: Module names are correctly stripped of -api/-conf suffixes.
// PREVENTS: Wrong method prefixes on the wire (e.g., "ze-bgp-api:peer-list" instead of "ze-bgp:peer-list").
func TestWireModule(t *testing.T) {
	tests := []struct {
		name   string
		module string
		want   string
	}{
		{"bgp-api", "ze-bgp-api", "ze-bgp"},
		{"system-api", "ze-system-api", "ze-system"},
		{"rib-api", "ze-rib-api", "ze-rib"},
		{"plugin-api", "ze-plugin-api", "ze-plugin"},
		{"bgp-conf", "ze-bgp-conf", "ze-bgp"},
		{"no-suffix", "ze-types", "ze-types"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, WireModule(tt.module))
		})
	}
}

// TestExtractRPCsNonexistentModule verifies graceful handling of missing modules.
//
// VALIDATES: Returns empty slice for nonexistent module.
// PREVENTS: Nil pointer panic when module doesn't exist.
func TestExtractRPCsNonexistentModule(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.Resolve())

	rpcs := ExtractRPCs(loader, "nonexistent-module")
	assert.Empty(t, rpcs, "should return empty for nonexistent module")
}

// TestExtractNotificationsNonexistentModule verifies graceful handling of missing modules.
//
// VALIDATES: Returns empty slice for nonexistent module.
// PREVENTS: Nil pointer panic when module doesn't exist.
func TestExtractNotificationsNonexistentModule(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.Resolve())

	notifs := ExtractNotifications(loader, "nonexistent-module")
	assert.Empty(t, notifs, "should return empty for nonexistent module")
}
