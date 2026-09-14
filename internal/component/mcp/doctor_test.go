// Design: docs/architecture/mcp/overview.md -- the readiness check this component owns
// Detail: doctor.go -- checkMCPTLSMaterial and its registration

package mcp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// mcpTLSTree builds an enabled mcp block naming the given certificate and key
// paths. An empty path leaves the leaf unset.
func mcpTLSTree(cert, key string) *config.Tree {
	tree := config.NewTree()
	mcp := tree.GetOrCreateContainer("environment").GetOrCreateContainer("mcp")
	mcp.Set("enabled", "true")
	tls := mcp.GetOrCreateContainer("tls")
	if cert != "" {
		tls.Set("cert", cert)
	}
	if key != "" {
		tls.Set("key", key)
	}
	return tree
}

// TestCheckMCPTLSMaterialReportsAMissingKey drives the check over a block that
// names a key file nothing wrote.
//
// VALIDATES: the diagnostic is doctor-tls-missing at error severity, resolved
// against the config directory, and names the mcp listener.
// PREVENTS: a listener that refuses to start at boot with nothing in the
// readiness report to say which file is absent.
func TestCheckMCPTLSMaterialReportsAMissingKey(t *testing.T) {
	dir := t.TempDir()

	diags := checkMCPTLSMaterial(diagnostic.DoctorCheckContext{Tree: mcpTLSTree("", "mcp.key"), ConfigDir: dir})

	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorTLSMissing, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "mcp: key not found")
	assert.Equal(t, filepath.Join(dir, "mcp.key"), diags[0].Path)
}

// TestCheckMCPTLSMaterialIsSilentWithoutAPair is the negative half.
//
// VALIDATES: no diagnostic for a block that names no pair, for a block that is
// not enabled, for a tree without the block, and for a nil tree.
// PREVENTS: a warning on every plaintext or disabled mcp listener.
func TestCheckMCPTLSMaterialIsSilentWithoutAPair(t *testing.T) {
	dir := t.TempDir()

	assert.Empty(t, checkMCPTLSMaterial(diagnostic.DoctorCheckContext{Tree: mcpTLSTree("", ""), ConfigDir: dir}), "no pair")

	disabled := mcpTLSTree("", "mcp.key")
	disabled.GetContainer("environment").GetContainer("mcp").Set("enabled", "false")
	assert.Empty(t, checkMCPTLSMaterial(diagnostic.DoctorCheckContext{Tree: disabled, ConfigDir: dir}), "not enabled")

	assert.Empty(t, checkMCPTLSMaterial(diagnostic.DoctorCheckContext{Tree: config.NewTree(), ConfigDir: dir}), "no block")
	assert.Empty(t, checkMCPTLSMaterial(diagnostic.DoctorCheckContext{}), "nil tree")
}

// TestMCPTLSDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this component's check, at the phase and order the
// declaration states, and whether every code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestMCPTLSDoctorCheckRegistered(t *testing.T) {
	want := mcpTLSDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	require.NotNil(t, found, "doctor check %q is not registered for phase %q", want.Name, want.Phase)
	assert.Equal(t, want.Order, found.Order)
	assert.Equal(t, want.Component, found.Component)

	diagnostic.RegisterBuiltinCodes()
	for _, code := range want.Codes {
		assert.NotNil(t, diagnostic.Lookup(code), "diagnostic code %q is not registered", code)
	}
}
