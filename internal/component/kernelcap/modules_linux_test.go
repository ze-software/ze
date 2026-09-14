//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Detail: modules_linux.go -- LoadedModules, the shared module-list reader

package kernelcap

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
)

// withModulesFile stands in the /proc/modules read for the life of one test.
func withModulesFile(t *testing.T, data []byte, err error) {
	t.Helper()
	original := readFile
	readFile = func(string) ([]byte, error) { return data, err }
	t.Cleanup(func() { readFile = original })
}

// TestLoadedModulesDistinguishesEmptyFromUnreadable drives the reader over a
// /proc/modules-shaped file, an empty one, and one that cannot be read.
//
// VALIDATES: the parser keeps the first column of each row, an empty file is
// an empty set, and an unreadable path is nil rather than an empty set.
// PREVENTS: "file missing" and "no modules loaded" collapsing into one answer,
// which is what let the stub path in mpls-doctor.ci fail invisibly.
func TestLoadedModulesDistinguishesEmptyFromUnreadable(t *testing.T) {
	withModulesFile(t, []byte("mpls_router 32768 1 mpls_iptunnel, Live 0x0\nmpls_iptunnel 16384 0 - Live 0x0\n"), nil)
	loaded := LoadedModules()
	require.NotNil(t, loaded)
	assert.True(t, loaded["mpls_router"])
	assert.True(t, loaded["mpls_iptunnel"])

	withModulesFile(t, []byte(""), nil)
	empty := LoadedModules()
	require.NotNil(t, empty, "an empty file means no modules, which is a real answer")
	assert.Empty(t, empty)

	withModulesFile(t, nil, errors.New("permission denied"))
	assert.Nil(t, LoadedModules(), "an unreadable file must be nil, not an empty set")
}

// TestLoadedModulesPathHonorsTheTestOverride pins the file the reader opens:
// ze.test.doctor.modules-file when set, /proc/modules otherwise.
//
// VALIDATES: the override every doctor functional test sets reaches this reader.
// PREVENTS: a fixture module list that the checks reading through this package
// never see, so a test asserting on a missing module passes against the host.
func TestLoadedModulesPathHonorsTheTestOverride(t *testing.T) {
	require.NoError(t, env.Set(modulesFileEnv, ""))
	assert.Equal(t, ProcPath("modules"), loadedModulesPath())

	require.NoError(t, env.Set(modulesFileEnv, "/fixture/modules.empty"))
	t.Cleanup(func() { _ = env.Set(modulesFileEnv, "") })
	assert.Equal(t, "/fixture/modules.empty", loadedModulesPath())
}
