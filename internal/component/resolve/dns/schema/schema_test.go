package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/resolve/dns/schema"
)

// TestSchema_ZeDNSModule verifies ze-dns-conf.yang content.
//
// VALIDATES: AC-1 -- ze-dns-conf module loaded and has expected structure.
// PREVENTS: Missing DNS configuration elements in YANG schema.
func TestSchema_ZeDNSModule(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-dns-conf")
	require.NotNil(t, mod, "ze-dns-conf module should exist")

	assert.Equal(t, "urn:ze:dns:conf", mod.Namespace.Name)

	var hasEnvironment bool
	for _, c := range mod.Container {
		if c.Name == "environment" {
			hasEnvironment = true
		}
	}
	assert.True(t, hasEnvironment, "environment container should exist")
}

// TestSchema_ZeDNSEntry verifies the YANG entry has expected children.
//
// VALIDATES: AC-1 -- config file with dns block parsed, all fields accessible.
// PREVENTS: Missing fields in DNS YANG schema.
func TestSchema_ZeDNSEntry(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-dns-conf")
	require.NotNil(t, entry, "ze-dns-conf entry should exist")

	environment := entry.Dir["environment"]
	require.NotNil(t, environment, "environment container should exist in entry")

	dns := environment.Dir["dns"]
	require.NotNil(t, dns, "dns container should exist inside environment")

	expectedLeaves := []string{"server", "timeout", "cache-size", "cache-ttl"}
	for _, name := range expectedLeaves {
		assert.NotNil(t, dns.Dir[name], "dns should have leaf %q", name)
	}
}
