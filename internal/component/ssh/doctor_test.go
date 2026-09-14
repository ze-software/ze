// Design: docs/architecture/system-architecture.md -- the readiness check this component owns
// Detail: doctor.go -- checkSSHHostKey and its registration
//
// The four host-key cases arrived from internal/component/doctor with the
// check they drive, case for case.

package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func sshEnabledTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("environment").GetOrCreateContainer("ssh").Set("enabled", "true")
	return tree
}

func TestCheckSSHHostKey_Missing(t *testing.T) {
	dir := t.TempDir()
	diags := checkSSHHostKey(diagnostic.DoctorCheckContext{Tree: sshEnabledTree(), ConfigDir: dir})
	require.Len(t, diags, 1)
	assert.Equal(t, codeSSHHostKeyMissing, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Equal(t, filepath.Join(dir, defaultHostKeyFile), diags[0].Path)
}

func TestCheckSSHHostKey_Present(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, defaultHostKeyFile), []byte("key"), 0o600))
	diags := checkSSHHostKey(diagnostic.DoctorCheckContext{Tree: sshEnabledTree(), ConfigDir: dir})
	assert.Empty(t, diags)
}

func TestCheckSSHHostKey_NotEnabled(t *testing.T) {
	dir := t.TempDir()
	diags := checkSSHHostKey(diagnostic.DoctorCheckContext{Tree: config.NewTree(), ConfigDir: dir})
	assert.Empty(t, diags, "SSH not enabled should skip host key check")
	assert.Empty(t, checkSSHHostKey(diagnostic.DoctorCheckContext{ConfigDir: dir}), "nil tree")
}

func TestCheckSSHHostKey_EmptyDir(t *testing.T) {
	diags := checkSSHHostKey(diagnostic.DoctorCheckContext{Tree: sshEnabledTree(), ConfigDir: ""})
	assert.Empty(t, diags)
}

// TestCheckSSHHostKey_CertificateMissing is the error half: a host
// certificate the config names and nothing generates.
func TestCheckSSHHostKey_CertificateMissing(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, defaultHostKeyFile), []byte("key"), 0o600))
	tree := sshEnabledTree()
	tree.GetContainer("environment").GetContainer("ssh").Set("host-certificate", "host-cert.pub")

	diags := checkSSHHostKey(diagnostic.DoctorCheckContext{Tree: tree, ConfigDir: dir})
	require.Len(t, diags, 1)
	assert.Equal(t, codeSSHHostKeyMissing, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Equal(t, filepath.Join(dir, "host-cert.pub"), diags[0].Path)
}

// TestSSHHostKeyDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this component's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestSSHHostKeyDoctorCheckRegistered(t *testing.T) {
	want := sshHostKeyDoctorCheck
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
	assert.NotNil(t, diagnostic.Lookup(codeSSHHostKeyMissing), "diagnostic code %q is not registered", codeSSHHostKeyMissing)
}
