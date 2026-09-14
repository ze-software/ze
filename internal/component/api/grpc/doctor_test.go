// Design: docs/architecture/api/architecture.md -- the readiness check this transport owns
// Detail: doctor.go -- checkGRPCTLSMaterial and its registration

package grpc

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// grpcTLSTree builds an api-server block with gRPC enabled, naming the given
// certificate and key paths. An empty path leaves the leaf unset.
func grpcTLSTree(cert, key string) *config.Tree {
	tree := config.NewTree()
	api := tree.GetOrCreateContainer("environment").GetOrCreateContainer("api-server")
	block := api.GetOrCreateContainer("grpc")
	block.Set("enabled", "true")
	if cert != "" {
		block.Set("tls-cert", cert)
	}
	if key != "" {
		block.Set("tls-key", key)
	}
	return tree
}

// TestCheckGRPCTLSMaterialReportsAMissingKey drives the check over a block
// that names a key file nothing wrote.
//
// VALIDATES: the diagnostic is doctor-tls-missing at error severity, resolved
// against the config directory, and names the gRPC listener.
// PREVENTS: a transport that refuses to start at boot with nothing in the
// readiness report to say which file is absent.
func TestCheckGRPCTLSMaterialReportsAMissingKey(t *testing.T) {
	dir := t.TempDir()

	diags := checkGRPCTLSMaterial(diagnostic.DoctorCheckContext{Tree: grpcTLSTree("", "grpc.key"), ConfigDir: dir})

	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorTLSMissing, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "api-grpc: key not found")
	assert.Equal(t, filepath.Join(dir, "grpc.key"), diags[0].Path)
}

// TestCheckGRPCTLSMaterialIsSilentWithoutAPair is the negative half.
//
// VALIDATES: no diagnostic for a block that names no pair, for a transport
// that is not enabled, for a tree without the block, and for a nil tree.
// PREVENTS: a warning on every plaintext or disabled gRPC listener.
func TestCheckGRPCTLSMaterialIsSilentWithoutAPair(t *testing.T) {
	dir := t.TempDir()

	assert.Empty(t, checkGRPCTLSMaterial(diagnostic.DoctorCheckContext{Tree: grpcTLSTree("", ""), ConfigDir: dir}), "no pair")

	disabled := grpcTLSTree("", "grpc.key")
	disabled.GetContainer("environment").GetContainer("api-server").GetContainer("grpc").Set("enabled", "false")
	assert.Empty(t, checkGRPCTLSMaterial(diagnostic.DoctorCheckContext{Tree: disabled, ConfigDir: dir}), "not enabled")

	assert.Empty(t, checkGRPCTLSMaterial(diagnostic.DoctorCheckContext{Tree: config.NewTree(), ConfigDir: dir}), "no block")
	assert.Empty(t, checkGRPCTLSMaterial(diagnostic.DoctorCheckContext{}), "nil tree")
}

// TestGRPCTLSDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this transport's check, at the phase and order the
// declaration states, and whether every code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestGRPCTLSDoctorCheckRegistered(t *testing.T) {
	want := grpcTLSDoctorCheck
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
