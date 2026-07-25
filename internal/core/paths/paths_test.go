package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
)

// VALIDATES: binary in standard system dirs resolves to /etc/ze.
// PREVENTS: wrong config dir for system-installed binaries.
func TestConfigDir_SystemBinaries(t *testing.T) {
	systemDirs := []string{
		"/bin",
		"/sbin",
		"/usr/bin",
		"/usr/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
	}
	for _, dir := range systemDirs {
		t.Run(dir, func(t *testing.T) {
			assert.Equal(t, "/etc/ze", paths.ConfigDirFromBinary(dir+"/ze"))
		})
	}
}

// VALIDATES: binary in ./bin resolves to ./etc/ze (relative).
// PREVENTS: wrong config dir for development/local builds.
func TestConfigDir_LocalBin(t *testing.T) {
	assert.Equal(t, "etc/ze", paths.ConfigDirFromBinary("bin/ze"))
	assert.Equal(t, "etc/ze", paths.ConfigDirFromBinary("./bin/ze"))
}

// VALIDATES: binary in /opt/<app>/bin resolves to /opt/<app>/etc/ze.
// PREVENTS: wrong config dir for /opt-style installs.
func TestConfigDir_OptPrefix(t *testing.T) {
	assert.Equal(t, "/opt/myapp/etc/ze", paths.ConfigDirFromBinary("/opt/myapp/bin/ze"))
	assert.Equal(t, "/opt/ze/etc/ze", paths.ConfigDirFromBinary("/opt/ze/bin/ze"))
}

// VALIDATES: binary in arbitrary prefix/<bin-like>/ze resolves relative to prefix.
// PREVENTS: only /opt handled, other prefixes ignored.
func TestConfigDir_ArbitraryPrefix(t *testing.T) {
	assert.Equal(t, "/home/user/app/etc/ze", paths.ConfigDirFromBinary("/home/user/app/bin/ze"))
	assert.Equal(t, "/srv/ze/etc/ze", paths.ConfigDirFromBinary("/srv/ze/sbin/ze"))
}

// VALIDATES: binary in gokrazy /user dir resolves to /perm/ze.
// PREVENTS: "cannot determine database location" on gokrazy appliances.
func TestConfigDir_Gokrazy(t *testing.T) {
	assert.Equal(t, "/perm/ze", paths.ConfigDirFromBinary("/user/ze"))
}

// VALIDATES: binary in unknown directory returns empty string.
// PREVENTS: guessing a config dir for unrecognized layouts.
func TestConfigDir_UnknownLocation(t *testing.T) {
	assert.Equal(t, "", paths.ConfigDirFromBinary("/tmp/ze"))
	assert.Equal(t, "", paths.ConfigDirFromBinary("/ze"))
	assert.Equal(t, "", paths.ConfigDirFromBinary("ze"))
}

// setConfigDirEnv sets ze.config.dir for the duration of the test.
func setConfigDirEnv(t *testing.T, value string) {
	t.Helper()
	orig := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", orig) })
	require.NoError(t, env.Set("ze.config.dir", value))
}

// VALIDATES: ze.config.dir overrides binary-relative resolution.
// PREVENTS: `ze data check` opening <prefix>/etc/ze/database.zefs while `ze init`
// writes $ZE_CONFIG_DIR/database.zefs. The override is registered in this package
// but DefaultConfigDir never read it, so `ze data` resolved a store nobody wrote.
func TestDefaultConfigDir_EnvOverride(t *testing.T) {
	setConfigDirEnv(t, "/custom/config")
	assert.Equal(t, "/custom/config", paths.DefaultConfigDir())
}

// VALIDATES: an unset ze.config.dir still falls back to the binary location.
// PREVENTS: the env override breaking system-installed and gokrazy layouts.
func TestDefaultConfigDir_EnvUnsetFallsBackToBinary(t *testing.T) {
	setConfigDirEnv(t, "")

	exe, err := os.Executable()
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(exe)
	require.NoError(t, err)

	assert.Equal(t, paths.ConfigDirFromBinary(resolved), paths.DefaultConfigDir())
}
