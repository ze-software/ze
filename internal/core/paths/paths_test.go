package paths_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/paths"
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

// VALIDATES: binary in unknown directory returns empty string.
// PREVENTS: guessing a config dir for unrecognized layouts.
func TestConfigDir_UnknownLocation(t *testing.T) {
	assert.Equal(t, "", paths.ConfigDirFromBinary("/tmp/ze"))
	assert.Equal(t, "", paths.ConfigDirFromBinary("/ze"))
	assert.Equal(t, "", paths.ConfigDirFromBinary("ze"))
}
